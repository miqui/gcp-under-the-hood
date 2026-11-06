package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func do(h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthToggle(t *testing.T) {
	h := newServer().routes()
	if c := do(h, "/healthz", nil).Code; c != 200 {
		t.Fatalf("want 200, got %d", c)
	}
	do(h, "/healthz/fail", nil)
	if c := do(h, "/healthz", nil).Code; c != 503 {
		t.Fatalf("want 503, got %d", c)
	}
	do(h, "/healthz/ok", nil)
	if c := do(h, "/healthz", nil).Code; c != 200 {
		t.Fatalf("want 200 after ok, got %d", c)
	}
}

func TestEchoHeaders(t *testing.T) {
	h := newServer().routes()
	rec := do(h, "/", map[string]string{"X-Forwarded-For": "1.2.3.4, 5.6.7.8", "Via": "1.1 google"})
	body := rec.Body.String()
	for _, want := range []string{"hostname:", "X-Forwarded-For: 1.2.3.4, 5.6.7.8", "Via: 1.1 google"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

func TestSlow(t *testing.T) {
	s := newServer()
	var got time.Duration
	s.sleep = func(d time.Duration) { got = d }
	h := s.routes()
	if c := do(h, "/slow?s=3", nil).Code; c != 200 || got != 3*time.Second {
		t.Fatalf("code=%d slept=%v", c, got)
	}
	for _, p := range []string{"/slow", "/slow?s=abc", "/slow?s=-1", "/slow?s=9999"} {
		if c := do(h, p, nil).Code; c != 400 {
			t.Errorf("%s: want 400, got %d", p, c)
		}
	}
}
