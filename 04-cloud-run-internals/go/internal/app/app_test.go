package app

import (
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestColdStartOnlyFirst(t *testing.T) {
	s := New()
	h := s.Handler()
	var infos [2]Info
	for i := range infos {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/info", nil))
		if err := json.Unmarshal(rec.Body.Bytes(), &infos[i]); err != nil {
			t.Fatal(err)
		}
	}
	if !infos[0].ColdStart || infos[1].ColdStart {
		t.Fatalf("cold flags = %v,%v", infos[0].ColdStart, infos[1].ColdStart)
	}
	if infos[0].InstanceID != s.ID || infos[1].TotalServed != 1 {
		t.Fatalf("bad info %+v", infos)
	}
}

func TestInflightCounting(t *testing.T) {
	s := New()
	h := s.Handler()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/work?latency_ms=300", nil))
		}()
	}
	time.Sleep(100 * time.Millisecond)
	if got := s.inflight.Load(); got != 5 {
		t.Fatalf("inflight = %d, want 5", got)
	}
	wg.Wait()
	if s.inflight.Load() != 0 || s.total.Load() != 5 {
		t.Fatalf("after: inflight=%d total=%d", s.inflight.Load(), s.total.Load())
	}
}

func TestClamp(t *testing.T) {
	for in, want := range map[string]int{"": 0, "x": 0, "-5": 0, "10": 10, "999999": 60000} {
		if got := clamp(in); got != want {
			t.Errorf("clamp(%q)=%d want %d", in, got, want)
		}
	}
}

func TestWorkLatency(t *testing.T) {
	s := New()
	start := time.Now()
	s.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/work?cpu_ms=20&latency_ms=50", nil))
	if d := time.Since(start); d < 65*time.Millisecond {
		t.Fatalf("too fast: %v", d)
	}
}
