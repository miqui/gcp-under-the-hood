package gcpauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetadataToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "missing flavor", 403)
			return
		}
		if r.URL.Path != "/computeMetadata/v1/instance/service-accounts/default/token" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"access_token":"tok-md","expires_in":3599,"token_type":"Bearer"}`))
	}))
	defer srv.Close()
	c := New()
	c.MetadataBase = srv.URL
	tok, err := c.MetadataToken(context.Background(), "")
	if err != nil || tok.AccessToken != "tok-md" || tok.ExpiresIn != 3599 {
		t.Fatalf("got %+v, %v", tok, err)
	}
}

func TestSTSExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]string
		json.NewDecoder(r.Body).Decode(&b)
		if b["grantType"] != "urn:ietf:params:oauth:grant-type:token-exchange" || b["subjectToken"] != "jwt" || b["audience"] != "//iam/aud" {
			http.Error(w, `{"error":"invalid_request"}`, 400)
			return
		}
		w.Write([]byte(`{"access_token":"tok-fed","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()
	c := New()
	c.STSURL = srv.URL
	tok, err := c.STSExchange(context.Background(), "//iam/aud", "jwt")
	if err != nil || tok.AccessToken != "tok-fed" {
		t.Fatalf("got %+v, %v", tok, err)
	}
	if _, err := c.STSExchange(context.Background(), "bad", "jwt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer caller" {
			http.Error(w, "denied", 403)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/serviceAccounts/t@p.iam.gserviceaccount.com:generateAccessToken") {
			http.NotFound(w, r)
			return
		}
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		if b["lifetime"] != "300s" {
			http.Error(w, "bad lifetime", 400)
			return
		}
		w.Write([]byte(`{"accessToken":"minted","expireTime":"2030-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c := New()
	c.IAMCredBase = srv.URL
	tok, err := c.GenerateAccessToken(context.Background(), "caller", "t@p.iam.gserviceaccount.com", 300)
	if err != nil || tok.AccessToken != "minted" {
		t.Fatalf("got %+v, %v", tok, err)
	}
	if _, err := c.GenerateAccessToken(context.Background(), "nope", "t@p.iam.gserviceaccount.com", 300); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestRedact(t *testing.T) {
	if Redact("short") != "****" || strings.Contains(Redact("abcdef-verylongsecrettoken"), "secrettoken") {
		t.Fatal("redaction leaked")
	}
}
