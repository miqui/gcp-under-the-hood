// Command loadgen drives concurrent load and reports distinct instances seen.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

func main() {
	url := flag.String("url", "", "target URL, e.g. https://HOST/work?latency_ms=500 (required)")
	conc := flag.Int("c", 20, "concurrent workers")
	dur := flag.Duration("d", 30*time.Second, "test duration")
	flag.Parse()
	if *url == "" {
		flag.Usage()
		return
	}
	var mu sync.Mutex
	inst := map[string]int{}
	cold, ok, bad := 0, 0, 0
	var lats []time.Duration
	end := time.Now().Add(*dur)
	cl := &http.Client{Timeout: 60 * time.Second}
	var wg sync.WaitGroup
	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(end) {
				t0 := time.Now()
				resp, err := cl.Get(*url)
				d := time.Since(t0)
				mu.Lock()
				if err != nil || resp.StatusCode != 200 {
					bad++
				} else {
					var v struct {
						ID   string `json:"instance_id"`
						Cold bool   `json:"cold_start"`
					}
					_ = json.NewDecoder(resp.Body).Decode(&v)
					inst[v.ID]++
					if v.Cold {
						cold++
					}
					ok++
					lats = append(lats, d)
				}
				mu.Unlock()
				if resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}
	wg.Wait()
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	pct := func(p float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		return lats[int(float64(len(lats)-1)*p)]
	}
	fmt.Printf("ok=%d errors=%d cold_starts=%d distinct_instances=%d\n", ok, bad, cold, len(inst))
	fmt.Printf("latency p50=%s p95=%s max=%s\n", pct(.5), pct(.95), pct(1))
	for id, n := range inst {
		fmt.Printf("  instance %s: %d requests\n", id, n)
	}
}
