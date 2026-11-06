// Command probe is a tiny TCP/HTTP echo + client probe (stdlib only).
//
//	probe serve [-addr :8080]          HTTP server: reports hostname and caller address
//	probe check -target host:port      TCP dial, prints latency; exit 1 on failure
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// Handler returns the HTTP handler used by "serve".
func Handler(hostname string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "host=%s remote=%s\n", hostname, r.RemoteAddr)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	return mux
}

// Check dials target over TCP and returns the connect latency.
func Check(target string, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	c, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return 0, err
	}
	defer c.Close()
	return time.Since(start), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: probe serve|check [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		addr := fs.String("addr", ":8080", "listen address")
		fs.Parse(os.Args[2:])
		h, _ := os.Hostname()
		fmt.Println("listening on", *addr)
		if err := http.ListenAndServe(*addr, Handler(h)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		t := fs.String("target", "", "host:port")
		to := fs.Duration("timeout", 3*time.Second, "dial timeout")
		fs.Parse(os.Args[2:])
		d, err := Check(*t, *to)
		if err != nil {
			fmt.Println("FAIL", *t, err)
			os.Exit(1)
		}
		fmt.Println("OK", *t, d)
	default:
		fmt.Fprintln(os.Stderr, "unknown command")
		os.Exit(2)
	}
}
