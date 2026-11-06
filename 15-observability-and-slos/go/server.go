package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// Faults is the injectable failure configuration.
type Faults struct {
	ErrorRate float64 `json:"error_rate"` // 0..1 probability of HTTP 500
	LatencyMS int     `json:"latency_ms"` // extra sleep per request
}

// Server is the demo service. Randomness and sleeping are injectable for tests.
type Server struct {
	mu     sync.RWMutex
	faults Faults
	log    *JSONLogger
	rnd    func() float64
	sleep  func(context.Context, time.Duration)
}

func NewServer(log *JSONLogger, f Faults) *Server {
	return &Server{
		log:    log,
		faults: f,
		rnd:    rand.Float64,
		sleep: func(ctx context.Context, d time.Duration) {
			select {
			case <-time.After(d):
			case <-ctx.Done():
			}
		},
	}
}

func (s *Server) Faults() Faults {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.faults
}

func (s *Server) SetFaults(f Faults) error {
	if f.ErrorRate < 0 || f.ErrorRate > 1 || f.LatencyMS < 0 || f.LatencyMS > 30000 {
		return errInvalidFaults
	}
	s.mu.Lock()
	s.faults = f
	s.mu.Unlock()
	return nil
}

type constErr string

func (e constErr) Error() string { return string(e) }

const errInvalidFaults = constErr("error_rate must be in [0,1], latency_ms in [0,30000]")

// Handler wires routes. Extract propagates W3C traceparent from callers.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Deliberately not logged: health checks are noise (see log exclusion).
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/work", s.work)
	mux.HandleFunc("/admin/faults", s.adminFaults)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("slo-demo").Start(ctx, r.Method+" "+r.URL.Path)
		defer span.End()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) work(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := s.Faults()
	_, span := otel.Tracer("slo-demo").Start(ctx, "do-work")
	defer span.End()
	span.SetAttributes(attribute.Float64("fault.error_rate", f.ErrorRate), attribute.Int("fault.latency_ms", f.LatencyMS))

	if f.LatencyMS > 0 {
		s.sleep(ctx, time.Duration(f.LatencyMS)*time.Millisecond)
	}
	if s.rnd() < f.ErrorRate {
		span.SetStatus(codes.Error, "injected failure")
		s.log.Log(ctx, "ERROR", "injected failure", map[string]any{"event": "injected_error", "latency_ms": f.LatencyMS})
		http.Error(w, "injected failure", http.StatusInternalServerError)
		return
	}
	s.log.Log(ctx, "INFO", "request ok", map[string]any{"event": "request_ok", "latency_ms": f.LatencyMS})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) adminFaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		var f Faults
		if q := r.URL.Query(); q.Get("error_rate") != "" || q.Get("latency_ms") != "" {
			f = s.Faults()
			if v := q.Get("error_rate"); v != "" {
				p, err := strconv.ParseFloat(v, 64)
				if err != nil {
					http.Error(w, "bad error_rate", http.StatusBadRequest)
					return
				}
				f.ErrorRate = p
			}
			if v := q.Get("latency_ms"); v != "" {
				p, err := strconv.Atoi(v)
				if err != nil {
					http.Error(w, "bad latency_ms", http.StatusBadRequest)
					return
				}
				f.LatencyMS = p
			}
		} else if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.SetFaults(f); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.log.Log(r.Context(), "NOTICE", "faults updated", map[string]any{"event": "faults_updated", "error_rate": f.ErrorRate, "latency_ms": f.LatencyMS})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.Faults())
}
