package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const dummy = "dummy-value"

func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestWhoami(t *testing.T) {
	s := NewServer()
	s.Env = fakeEnv(map[string]string{"POD_IP": "10.20.0.5", "NODE_NAME": "n1"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "10.20.0.5") || !strings.Contains(rec.Body.String(), "n1") {
		t.Fatalf("unexpected: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGCSFlow(t *testing.T) {
	md := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			w.WriteHeader(403)
			return
		}
		w.Write([]byte(`{"access_token":"` + dummy + `","expires_in":3600}`))
	}))
	defer md.Close()
	st := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+dummy {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer st.Close()
	s := NewServer()
	s.MetadataURL, s.StorageURL = md.URL, st.URL
	s.Env = fakeEnv(map[string]string{"BUCKET": "b"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/gcs", nil))
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"gcs_status":200`) || strings.Contains(body, dummy) {
		t.Fatalf("unexpected: %d %s", rec.Code, body)
	}
}

func TestGCSNoBucket(t *testing.T) {
	s := NewServer()
	s.Env = fakeEnv(nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/gcs", nil))
	if rec.Code != 400 {
		t.Fatalf("got %d", rec.Code)
	}
}
