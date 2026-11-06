package bq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bigquery/v2/projects/p1/queries" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("auth header missing")
		}
		var q queryReq
		_ = json.NewDecoder(r.Body).Decode(&q)
		if !q.DryRun || q.UseLegacySQL {
			t.Errorf("bad req %+v", q)
		}
		_, _ = w.Write([]byte(`{"totalBytesProcessed":"12345","cacheHit":false}`))
	}))
	defer srv.Close()
	c := &Client{Project: "p1", Token: "tok", BaseURL: srv.URL}
	got, err := c.DryRun(context.Background(), "SELECT 1", "US")
	if err != nil || got.TotalBytesProcessed != 12345 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestDryRunError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()
	c := &Client{Project: "p1", Token: "tok", BaseURL: srv.URL}
	if _, err := c.DryRun(context.Background(), "x", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestExec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"numDmlAffectedRows":"42","jobComplete":true}`))
	}))
	defer srv.Close()
	c := &Client{Project: "p1", Token: "tok", BaseURL: srv.URL}
	n, err := c.Exec(context.Background(), "INSERT ...", "US")
	if err != nil || n != 42 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
