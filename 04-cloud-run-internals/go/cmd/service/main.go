// Command service is the Cloud Run demo workload.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/gcp-under-the-hood/cloudrun/internal/app"
)

func main() {
	port := os.Getenv("PORT") // container contract: Cloud Run injects PORT
	if port == "" {
		port = "8080"
	}
	s := app.New()
	srv := &http.Server{Addr: ":" + port, Handler: s.Handler()}

	// Optional startup delay (env STARTUP_DELAY, e.g. "3s") to exaggerate cold starts.
	if d, err := time.ParseDuration(os.Getenv("STARTUP_DELAY")); err == nil {
		log.Printf("simulating slow startup: %s", d)
		time.Sleep(d)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	done := make(chan struct{})
	go func() {
		got := <-sig
		t0 := time.Now()
		log.Printf("SIGTERM-HANDLER: received %v on instance %s; draining (Cloud Run allows ~10s)", got, s.ID)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		log.Printf("SIGTERM-HANDLER: shutdown complete err=%v after %s", err, time.Since(t0))
		close(done)
	}()

	log.Printf("instance %s listening on :%s (K_REVISION=%q)", s.ID, port, os.Getenv("K_REVISION"))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-done
}
