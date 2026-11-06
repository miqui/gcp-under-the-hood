// Package probe provides a DNS lookup and HTTP check using a custom resolver.
package probe

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"time"
)

// NewResolver returns a pure-Go resolver that sends every query to server
// ("host:port"; port defaults to 53), ignoring /etc/resolv.conf.
func NewResolver(server string, timeout time.Duration) (*net.Resolver, error) {
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		host, port = server, "53"
	}
	if net.ParseIP(host) == nil {
		return nil, fmt.Errorf("resolver %q must be an IP address", host)
	}
	addr := net.JoinHostPort(host, port)
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, network, addr)
		},
	}, nil
}

// LookupResult is the outcome of a DNS lookup.
type LookupResult struct {
	Name    string
	Addrs   []string
	Elapsed time.Duration
}

// Lookup resolves name's A/AAAA records with r (nil means system resolver).
func Lookup(ctx context.Context, r *net.Resolver, name string) (LookupResult, error) {
	if r == nil {
		r = net.DefaultResolver
	}
	start := time.Now()
	addrs, err := r.LookupHost(ctx, name)
	res := LookupResult{Name: name, Elapsed: time.Since(start)}
	if err != nil {
		return res, err
	}
	sort.Strings(addrs)
	res.Addrs = addrs
	return res, nil
}

// HTTPResult is the outcome of an HTTP GET.
type HTTPResult struct {
	URL     string
	Status  int
	Bytes   int64
	Elapsed time.Duration
}

// HTTPGet fetches url, resolving hostnames through r.
func HTTPGet(ctx context.Context, r *net.Resolver, url string, timeout time.Duration) (HTTPResult, error) {
	if r == nil {
		r = net.DefaultResolver
	}
	d := &net.Dialer{Timeout: timeout, Resolver: r}
	client := &http.Client{Timeout: timeout, Transport: &http.Transport{DialContext: d.DialContext}}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HTTPResult{URL: url}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return HTTPResult{URL: url, Elapsed: time.Since(start)}, err
	}
	defer resp.Body.Close()
	n, _ := io.Copy(io.Discard, resp.Body)
	return HTTPResult{URL: url, Status: resp.StatusCode, Bytes: n, Elapsed: time.Since(start)}, nil
}
