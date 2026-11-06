package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// Usage:
//
//	export GCP_ACCESS_TOKEN=$(gcloud auth print-access-token)
//	envelope -key <kek_id> -in plain.txt -out env.json -aad ctx seal
//	envelope -key <kek_id> -in env.json -aad ctx open
//	envelope -key <kek_id> -in env.json -out env2.json -aad ctx rewrap
func main() {
	key := flag.String("key", "", "KEK resource name (terraform output kek_id)")
	in := flag.String("in", "", "input file")
	out := flag.String("out", "", "output file (default stdout)")
	aad := flag.String("aad", "", "additional authenticated data (context binding)")
	flag.Parse()
	if flag.NArg() != 1 || *key == "" || *in == "" {
		fmt.Fprintln(os.Stderr, "usage: envelope -key KEK -in FILE [-out FILE] [-aad CTX] seal|open|rewrap")
		os.Exit(2)
	}
	tok := os.Getenv("GCP_ACCESS_TOKEN")
	if tok == "" {
		fmt.Fprintln(os.Stderr, "set GCP_ACCESS_TOKEN (gcloud auth print-access-token)")
		os.Exit(2)
	}
	if err := run(context.Background(), flag.Arg(0), &RESTKMS{KeyName: *key, Token: tok}, *key, *in, *out, []byte(*aad)); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd string, k KMS, key, in, out string, aad []byte) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	var res []byte
	switch cmd {
	case "seal":
		e, err := Seal(ctx, k, key, data, aad, rand.Reader)
		if err != nil {
			return err
		}
		res, _ = json.MarshalIndent(e, "", "  ")
	case "open", "rewrap":
		var e Envelope
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		if cmd == "open" {
			res, err = Open(ctx, k, &e, aad)
		} else {
			var e2 *Envelope
			if e2, err = Rewrap(ctx, k, &e, aad); err == nil {
				res, _ = json.MarshalIndent(e2, "", "  ")
			}
		}
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	if out == "" {
		_, err = os.Stdout.Write(append(res, '\n'))
		return err
	}
	return os.WriteFile(out, res, 0o600)
}
