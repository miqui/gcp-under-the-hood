package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func envFloat(k string, d float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return d
}

func main() {
	project := os.Getenv("GCP_PROJECT_ID")
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	log := NewJSONLogger(os.Stdout, project)
	ctx := context.Background()

	shutdown, err := InitTracing(ctx, project)
	if err != nil {
		log.Log(ctx, "ERROR", "tracing init failed: "+err.Error(), nil)
		os.Exit(1)
	}

	srv := NewServer(log, Faults{
		ErrorRate: envFloat("FAULT_ERROR_RATE", 0),
		LatencyMS: envInt("FAULT_LATENCY_MS", 0),
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	hs := &http.Server{Addr: ":" + port, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		c, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = hs.Shutdown(c)
		_ = shutdown(c) // flush pending spans
	}()

	log.Log(ctx, "INFO", "listening on :"+port, map[string]any{"event": "startup"})
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Log(ctx, "CRITICAL", err.Error(), nil)
		os.Exit(1)
	}
}
