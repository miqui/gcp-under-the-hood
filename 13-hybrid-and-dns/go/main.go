// Command dnsprobe resolves a name via a chosen DNS server and optionally GETs a URL.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"example.com/gcp-under-the-hood/hybrid-dns-probe/probe"
)

func main() {
	resolver := flag.String("resolver", "", "DNS server IP[:port] (default: system resolver)")
	name := flag.String("name", "", "hostname to resolve")
	url := flag.String("http", "", "optional URL to GET (resolved through -resolver)")
	timeout := flag.Duration("timeout", 3*time.Second, "per-operation timeout")
	flag.Parse()
	if *name == "" && *url == "" {
		flag.Usage()
		os.Exit(2)
	}

	var r *net.Resolver
	if *resolver != "" {
		var err error
		r, err = probe.NewResolver(*resolver, *timeout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	ctx := context.Background()
	failed := false
	if *name != "" {
		res, err := probe.Lookup(ctx, r, *name)
		if err != nil {
			fmt.Printf("DNS FAIL %s: %v (%s)\n", *name, err, res.Elapsed)
			failed = true
		} else {
			fmt.Printf("DNS OK   %s -> %v (%s)\n", *name, res.Addrs, res.Elapsed)
		}
	}
	if *url != "" {
		res, err := probe.HTTPGet(ctx, r, *url, *timeout)
		if err != nil {
			fmt.Printf("HTTP FAIL %s: %v\n", *url, err)
			failed = true
		} else {
			fmt.Printf("HTTP OK  %s -> %d, %d bytes (%s)\n", *url, res.Status, res.Bytes, res.Elapsed)
		}
	}
	if failed {
		os.Exit(1)
	}
}
