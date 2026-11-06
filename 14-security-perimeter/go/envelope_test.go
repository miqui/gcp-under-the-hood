package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeKMS "wraps" by XOR with a versioned key and tags AAD + version. It models
// key versions: after Rotate(), new wraps use the new version but old ones still unwrap.
type fakeKMS struct {
	keys    [][]byte
	primary int
	fail    error
}

func newFake() *fakeKMS { return &fakeKMS{keys: [][]byte{bytes.Repeat([]byte{0x5a}, 64)}} }

func (f *fakeKMS) Rotate() {
	f.keys = append(f.keys, bytes.Repeat([]byte{byte(0x10 + len(f.keys))}, 64))
	f.primary = len(f.keys) - 1
}

func (f *fakeKMS) Wrap(_ context.Context, p, aad []byte) ([]byte, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	out := []byte{byte(f.primary)}
	out = append(out, aad...)
	out = append(out, 0)
	for i, b := range p {
		out = append(out, b^f.keys[f.primary][i%64])
	}
	return out, nil
}

func (f *fakeKMS) Unwrap(_ context.Context, w, aad []byte) ([]byte, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	if len(w) < 2 || int(w[0]) >= len(f.keys) {
		return nil, errors.New("unknown key version")
	}
	v := int(w[0])
	rest := w[1:]
	n := bytes.IndexByte(rest, 0)
	if n < 0 || !bytes.Equal(rest[:n], aad) {
		return nil, errors.New("aad mismatch")
	}
	body := rest[n+1:]
	out := make([]byte, len(body))
	for i, b := range body {
		out[i] = b ^ f.keys[v][i%64]
	}
	return out, nil
}

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	k := newFake()
	e, err := Seal(ctx, k, "k", []byte("hello"), []byte("ctx"), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(e.Ciphertext, []byte("hello")) {
		t.Fatal("plaintext leaked")
	}
	pt, err := Open(ctx, k, e, []byte("ctx"))
	if err != nil || string(pt) != "hello" {
		t.Fatalf("got %q %v", pt, err)
	}
}

func TestFreshDEKPerSeal(t *testing.T) {
	k := newFake()
	a, _ := Seal(context.Background(), k, "k", []byte("x"), nil, rand.Reader)
	b, _ := Seal(context.Background(), k, "k", []byte("x"), nil, rand.Reader)
	if bytes.Equal(a.WrappedDEK, b.WrappedDEK) || bytes.Equal(a.Ciphertext, b.Ciphertext) {
		t.Fatal("DEK/nonce reused")
	}
}

func TestWrongAAD(t *testing.T) {
	ctx := context.Background()
	k := newFake()
	e, _ := Seal(ctx, k, "k", []byte("x"), []byte("a"), rand.Reader)
	if _, err := Open(ctx, k, e, []byte("b")); err == nil {
		t.Fatal("expected failure")
	}
}

func TestTamperDetected(t *testing.T) {
	ctx := context.Background()
	k := newFake()
	e, _ := Seal(ctx, k, "k", []byte("secret"), nil, rand.Reader)
	e.Ciphertext[0] ^= 1
	if _, err := Open(ctx, k, e, nil); err == nil || !strings.Contains(err.Error(), "decrypt data") {
		t.Fatalf("err=%v", err)
	}
}

func TestKMSDownOrKeyDisabled(t *testing.T) {
	ctx := context.Background()
	k := newFake()
	e, _ := Seal(ctx, k, "k", []byte("x"), nil, rand.Reader)
	k.fail = errors.New("FAILED_PRECONDITION: key version destroyed")
	if _, err := Open(ctx, k, e, nil); err == nil || !strings.Contains(err.Error(), "unwrap dek") {
		t.Fatalf("err=%v", err)
	}
	if _, err := Seal(ctx, k, "k", []byte("x"), nil, rand.Reader); err == nil {
		t.Fatal("expected wrap failure")
	}
}

func TestRotationAndRewrap(t *testing.T) {
	ctx := context.Background()
	k := newFake()
	e, _ := Seal(ctx, k, "k", []byte("data"), nil, rand.Reader)
	k.Rotate()
	if pt, err := Open(ctx, k, e, nil); err != nil || string(pt) != "data" {
		t.Fatal("old version must still decrypt", err)
	}
	e2, err := Rewrap(ctx, k, e, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e2.WrappedDEK[0] != 1 || e.WrappedDEK[0] != 0 {
		t.Fatal("rewrap should use new primary and not mutate original")
	}
	if !bytes.Equal(e.Ciphertext, e2.Ciphertext) {
		t.Fatal("data ciphertext must be untouched by rewrap")
	}
	if pt, _ := Open(ctx, k, e2, nil); string(pt) != "data" {
		t.Fatal("rewrapped open failed")
	}
}

func TestOpenNil(t *testing.T) {
	if _, err := Open(context.Background(), newFake(), nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunSealOpen(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "p.txt")
	env := filepath.Join(dir, "e.json")
	if err := os.WriteFile(in, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	k := newFake()
	ctx := context.Background()
	if err := run(ctx, "seal", k, "k", in, env, []byte("c")); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "o.txt")
	if err := run(ctx, "open", k, "k", env, out, []byte("c")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != "payload" {
		t.Fatalf("got %q", b)
	}
	if err := run(ctx, "bogus", k, "k", in, "", nil); err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestRESTKMSWrap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || !strings.HasSuffix(r.URL.Path, "/keys/kek:encrypt") {
			http.Error(w, "bad", 400)
			return
		}
		_, _ = w.Write([]byte(`{"ciphertext":"AQID"}`))
	}))
	defer srv.Close()
	r := &RESTKMS{KeyName: "keys/kek", Token: "tok", Base: srv.URL}
	out, err := r.Wrap(context.Background(), []byte("x"), nil)
	if err != nil || !bytes.Equal(out, []byte{1, 2, 3}) {
		t.Fatalf("%v %v", out, err)
	}
}

func TestRESTKMSError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", 403)
	}))
	defer srv.Close()
	r := &RESTKMS{KeyName: "k", Token: "t", Base: srv.URL}
	if _, err := r.Unwrap(context.Background(), []byte("x"), nil); err == nil {
		t.Fatal("expected error")
	}
}
