package probe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startFakeDNS answers every A query with ip and every other type with empty NOERROR.
func startFakeDNS(t *testing.T, ip net.IP) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if n < 12 {
				continue
			}
			i := 12
			for i < n && buf[i] != 0 {
				i += int(buf[i]) + 1
			}
			qEnd := i + 5 // zero label + type(2) + class(2)
			if qEnd > n {
				continue
			}
			qtype := int(buf[i+1])<<8 | int(buf[i+2])
			resp := append([]byte{}, buf[:qEnd]...)
			resp[2], resp[3] = 0x81, 0x80
			for k := 6; k < 12; k++ {
				resp[k] = 0
			}
			if qtype == 1 {
				resp[7] = 1
				resp = append(resp, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4)
				resp = append(resp, ip.To4()...)
			}
			_, _ = pc.WriteTo(resp, addr)
		}
	}()
	return pc.LocalAddr().String()
}

func TestNewResolverRejectsHostname(t *testing.T) {
	if _, err := NewResolver("dns.example.com", time.Second); err == nil {
		t.Fatal("expected error for non-IP resolver")
	}
}

func TestNewResolverDefaultPort(t *testing.T) {
	if _, err := NewResolver("10.0.0.2", time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestLookupUsesCustomResolver(t *testing.T) {
	srv := startFakeDNS(t, net.ParseIP("10.1.2.3"))
	r, err := NewResolver(srv, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := Lookup(ctx, r, "www.split.example.internal")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Addrs) != 1 || res.Addrs[0] != "10.1.2.3" {
		t.Fatalf("got %v", res.Addrs)
	}
}

func TestLookupFailure(t *testing.T) {
	r, _ := NewResolver("127.0.0.1:1", 200*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Lookup(ctx, r, "nope.example.internal"); err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPGetResolvesViaCustomResolver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	srv := startFakeDNS(t, net.ParseIP("127.0.0.1"))
	r, _ := NewResolver(srv, time.Second)
	res, err := HTTPGet(context.Background(), r, "http://app.fake.internal:"+port+"/", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 200 || res.Bytes != 5 {
		t.Fatalf("got %+v", res)
	}
}
