package main

import (
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	for path, want := range map[string]string{"/": "host=vm-x", "/healthz": "ok"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		Handler("vm-x").ServeHTTP(rec, req)
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: got %q want %q", path, rec.Body.String(), want)
		}
	}
}

func TestCheck(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if _, err := Check(addr, time.Second); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	l.Close()
	if _, err := Check(addr, 200*time.Millisecond); err == nil {
		t.Fatal("expected failure on closed port")
	}
}
