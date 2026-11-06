package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	t.Setenv("K_REVISION", "rev-1")
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), `"revision":"rev-1"`) {
		t.Fatalf("body: %s", rec.Body.String())
	}
}
