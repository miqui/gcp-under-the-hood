// Command lbdemo is a tiny backend that reveals how a load balancer treats a request.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync/atomic"
	"time"
)

// server holds the toggleable health state.
type server struct {
	healthy atomic.Bool
	sleep   func(time.Duration)
}

func newServer() *server {
	s := &server{sleep: time.Sleep}
	s.healthy.Store(true)
	return s
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/healthz/fail", s.setHealth(false))
	mux.HandleFunc("/healthz/ok", s.setHealth(true))
	mux.HandleFunc("/slow", s.slow)
	mux.HandleFunc("/", s.echo)
	return mux
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	if !s.healthy.Load() {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "ok")
}

func (s *server) setHealth(v bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		s.healthy.Store(v)
		fmt.Fprintf(w, "healthy=%t\n", v)
	}
}

func (s *server) slow(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("s"))
	if err != nil || n < 0 || n > 300 {
		http.Error(w, "s must be an integer 0..300", http.StatusBadRequest)
		return
	}
	s.sleep(time.Duration(n) * time.Second)
	host, _ := os.Hostname()
	fmt.Fprintf(w, "slept %ds on %s\n", n, host)
}

func (s *server) echo(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	fmt.Fprintf(w, "hostname: %s\n", host)
	fmt.Fprintf(w, "remote_addr: %s\n", r.RemoteAddr)
	fmt.Fprintf(w, "proto: %s\n\n", r.Proto)
	keys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range r.Header[k] {
			fmt.Fprintf(w, "%s: %s\n", k, v)
		}
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newServer().routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
