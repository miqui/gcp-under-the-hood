// Package app implements the instrumented demo service.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// Server tracks instance-level state: identity, in-flight requests, age.
type Server struct {
	ID       string
	Started  time.Time
	inflight atomic.Int64
	total    atomic.Int64
	first    atomic.Bool // set once the first request has been served
}

// New creates a Server. Instance ID is random per process (== per instance).
func New() *Server {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return &Server{ID: hex.EncodeToString(b), Started: time.Now()}
}

// Info is the JSON body returned by /info and /work.
type Info struct {
	InstanceID  string  `json:"instance_id"`
	Revision    string  `json:"revision"`
	Inflight    int64   `json:"inflight"`
	TotalServed int64   `json:"total_served"`
	UptimeSec   float64 `json:"uptime_seconds"`
	ColdStart   bool    `json:"cold_start"` // true only for the first request this instance serves
}

// Handler returns the HTTP mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/info", s.track(s.info))
	mux.HandleFunc("/work", s.track(s.work))
	return mux
}

// track counts in-flight requests and detects the first request (cold start).
func (s *Server) track(h func(http.ResponseWriter, *http.Request, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cold := !s.first.Swap(true)
		s.inflight.Add(1)
		defer func() { s.inflight.Add(-1); s.total.Add(1) }()
		h(w, r, cold)
	}
}

func (s *Server) snapshot(cold bool) Info {
	return Info{
		InstanceID:  s.ID,
		Revision:    os.Getenv("K_REVISION"),
		Inflight:    s.inflight.Load(),
		TotalServed: s.total.Load(),
		UptimeSec:   time.Since(s.Started).Seconds(),
		ColdStart:   cold,
	}
}

func (s *Server) info(w http.ResponseWriter, _ *http.Request, cold bool) {
	writeJSON(w, s.snapshot(cold))
}

// work handles /work?cpu_ms=N&latency_ms=M: busy-loops N ms, then sleeps M ms.
func (s *Server) work(w http.ResponseWriter, r *http.Request, cold bool) {
	cpu := clamp(r.URL.Query().Get("cpu_ms"))
	lat := clamp(r.URL.Query().Get("latency_ms"))
	end := time.Now().Add(time.Duration(cpu) * time.Millisecond)
	for time.Now().Before(end) {
	}
	time.Sleep(time.Duration(lat) * time.Millisecond)
	writeJSON(w, s.snapshot(cold))
}

func clamp(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	if n > 60000 {
		return 60000
	}
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
