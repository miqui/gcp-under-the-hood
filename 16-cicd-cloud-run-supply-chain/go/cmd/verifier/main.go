// Command verifier checks digest-pinned refs and attestation payload files.
//
//	verifier -image REF [-payload FILE] [-emit]
package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/supplychain/internal/supply"
)

func main() {
	image := flag.String("image", "", "image reference pinned by digest")
	payload := flag.String("payload", "", "attestation payload JSON file to validate")
	emit := flag.Bool("emit", false, "print the payload an attestor should sign")
	flag.Parse()

	ref, err := supply.ParseDigestRef(*image)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("OK digest-pinned:", ref)
	if *emit {
		b, _ := supply.BuildPayload(ref)
		fmt.Println(string(b))
	}
	if *payload != "" {
		data, err := os.ReadFile(*payload)
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if err := supply.ValidatePayload(data, ref); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		fmt.Println("OK payload matches image")
	}
}
