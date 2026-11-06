// Command loadgen sends steady traffic to the service and prints a summary.
// For an authenticated Cloud Run service pass an identity token via TOKEN env:
//
//	TOKEN=$(gcloud auth print-identity-token) go run ./cmd/loadgen -url $URL -rps 20 -duration 10m
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	OK, ServerErr, Other, NetErr atomic.Int64
}

// Classify updates stats from a status code (0 = transport error).
func (s *Stats) Classify(code int) {
	switch {
	case code == 0:
		s.NetErr.Add(1)
	case code >= 500:
		s.ServerErr.Add(1)
	case code >= 200 && code < 400:
		s.OK.Add(1)
	default:
		s.Other.Add(1)
	}
}

func (s *Stats) String() string {
	ok, e5, o, ne := s.OK.Load(), s.ServerErr.Load(), s.Other.Load(), s.NetErr.Load()
	total := ok + e5 + o + ne
	rate := 0.0
	if total > 0 {
		rate = float64(e5+ne) / float64(total) * 100
	}
	return fmt.Sprintf("total=%d ok=%d 5xx=%d 4xx=%d neterr=%d bad%%=%.2f", total, ok, e5, o, ne, rate)
}

func main() {
	url := flag.String("url", "", "base URL of the service (required)")
	rps := flag.Int("rps", 10, "requests per second")
	dur := flag.Duration("duration", time.Minute, "how long to run")
	flag.Parse()
	if *url == "" || *rps <= 0 {
		fmt.Fprintln(os.Stderr, "usage: loadgen -url URL [-rps N] [-duration D]")
		os.Exit(2)
	}
	token := os.Getenv("TOKEN")
	client := &http.Client{Timeout: 15 * time.Second}
	var st Stats
	var wg sync.WaitGroup
	tick := time.NewTicker(time.Second / time.Duration(*rps))
	defer tick.Stop()
	end := time.After(*dur)
	report := time.NewTicker(10 * time.Second)
	defer report.Stop()
loop:
	for {
		select {
		case <-end:
			break loop
		case <-report.C:
			fmt.Println(st.String())
		case <-tick.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				req, _ := http.NewRequest(http.MethodGet, *url+"/work", nil)
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				resp, err := client.Do(req)
				if err != nil {
					st.Classify(0)
					return
				}
				resp.Body.Close()
				st.Classify(resp.StatusCode)
			}()
		}
	}
	wg.Wait()
	fmt.Println("final:", st.String())
}
