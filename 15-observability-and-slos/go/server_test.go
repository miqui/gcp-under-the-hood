package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func newTest(t *testing.T, f Faults, rnd float64) (*Server, *bytes.Buffer) {
	t.Helper()
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	buf := &bytes.Buffer{}
	s := NewServer(NewJSONLogger(buf, "test-proj"), f)
	s.rnd = func() float64 { return rnd }
	s.sleep = func(context.Context, time.Duration) {}
	return s, buf
}

func do(s *Server, method, path, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rr
}

func TestWorkOK(t *testing.T) {
	s, _ := newTest(t, Faults{}, 0.5)
	if rr := do(s, "GET", "/work", ""); rr.Code != 200 {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestWorkInjectedError(t *testing.T) {
	s, _ := newTest(t, Faults{ErrorRate: 0.3}, 0.1)
	if rr := do(s, "GET", "/work", ""); rr.Code != 500 {
		t.Fatalf("got %d", rr.Code)
	}
	s.rnd = func() float64 { return 0.9 }
	if rr := do(s, "GET", "/work", ""); rr.Code != 200 {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestLatencyInjected(t *testing.T) {
	s, _ := newTest(t, Faults{LatencyMS: 250}, 0.9)
	var got time.Duration
	s.sleep = func(_ context.Context, d time.Duration) { got = d }
	do(s, "GET", "/work", "")
	if got != 250*time.Millisecond {
		t.Fatalf("slept %v", got)
	}
}

func TestLogHasTraceCorrelation(t *testing.T) {
	s, buf := newTest(t, Faults{}, 0.9)
	do(s, "GET", "/work", "")
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("not json: %v: %s", err, buf.String())
	}
	tr, _ := m["logging.googleapis.com/trace"].(string)
	if !strings.HasPrefix(tr, "projects/test-proj/traces/") || len(tr) != len("projects/test-proj/traces/")+32 {
		t.Fatalf("bad trace field %q", tr)
	}
	if _, ok := m["logging.googleapis.com/spanId"]; !ok {
		t.Fatal("missing spanId")
	}
	if m["severity"] != "INFO" {
		t.Fatalf("severity %v", m["severity"])
	}
}

func TestErrorLogSeverity(t *testing.T) {
	s, buf := newTest(t, Faults{ErrorRate: 1}, 0)
	do(s, "GET", "/work", "")
	if !strings.Contains(buf.String(), `"severity":"ERROR"`) {
		t.Fatalf("no ERROR log: %s", buf.String())
	}
}

func TestHealthzNotLogged(t *testing.T) {
	s, buf := newTest(t, Faults{}, 0)
	if rr := do(s, "GET", "/healthz", ""); rr.Code != 200 || buf.Len() != 0 {
		t.Fatalf("code=%d log=%q", rr.Code, buf.String())
	}
}

func TestAdminFaults(t *testing.T) {
	s, _ := newTest(t, Faults{}, 0)
	if rr := do(s, "POST", "/admin/faults", `{"error_rate":0.5,"latency_ms":100}`); rr.Code != 200 {
		t.Fatalf("got %d", rr.Code)
	}
	if f := s.Faults(); f.ErrorRate != 0.5 || f.LatencyMS != 100 {
		t.Fatalf("%+v", f)
	}
	if rr := do(s, "POST", "/admin/faults?error_rate=0.9", ""); rr.Code != 200 || s.Faults().ErrorRate != 0.9 || s.Faults().LatencyMS != 100 {
		t.Fatalf("query update failed: %d %+v", rr.Code, s.Faults())
	}
	for _, bad := range []string{`{"error_rate":2}`, `{"latency_ms":-1}`, `nope`} {
		if rr := do(s, "POST", "/admin/faults", bad); rr.Code != http.StatusBadRequest {
			t.Fatalf("%s -> %d", bad, rr.Code)
		}
	}
	if rr := do(s, "DELETE", "/admin/faults", ""); rr.Code != 405 {
		t.Fatalf("got %d", rr.Code)
	}
}
