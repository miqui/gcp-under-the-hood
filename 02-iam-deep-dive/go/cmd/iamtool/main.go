// iamtool: show GCP token flows on the wire.
//
//	iamtool metadata                 # token from metadata server (GCE/Cloud Run/GKE only)
//	iamtool impersonate TARGET_SA    # needs CALLER_TOKEN env
//	iamtool sts                      # needs WIF_AUDIENCE + SUBJECT_JWT env
//	iamtool call-gcs BUCKET OBJECT   # needs ACCESS_TOKEN env
//
// Tokens are redacted unless PRINT_TOKEN=1. Wire log goes to stderr.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"example.com/gcp-under-the-hood/iam-deep-dive/internal/gcpauth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: iamtool metadata|impersonate SA|sts|call-gcs BUCKET OBJECT")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := gcpauth.New()
	c.Log = func(f string, a ...any) { fmt.Fprintf(os.Stderr, "[wire] "+f+"\n", a...) }
	var tok *gcpauth.Token
	var err error
	switch os.Args[1] {
	case "metadata":
		tok, err = c.MetadataToken(ctx, "default")
	case "impersonate":
		if len(os.Args) < 3 || os.Getenv("CALLER_TOKEN") == "" {
			die("need TARGET_SA arg and CALLER_TOKEN env")
		}
		tok, err = c.GenerateAccessToken(ctx, os.Getenv("CALLER_TOKEN"), os.Args[2], 300)
	case "sts":
		if os.Getenv("WIF_AUDIENCE") == "" || os.Getenv("SUBJECT_JWT") == "" {
			die("need WIF_AUDIENCE and SUBJECT_JWT env")
		}
		tok, err = c.STSExchange(ctx, os.Getenv("WIF_AUDIENCE"), os.Getenv("SUBJECT_JWT"))
	case "call-gcs":
		if len(os.Args) < 4 || os.Getenv("ACCESS_TOKEN") == "" {
			die("need BUCKET OBJECT args and ACCESS_TOKEN env")
		}
		u := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/%s?alt=media", url.PathEscape(os.Args[2]), url.PathEscape(os.Args[3]))
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		req.Header.Set("Authorization", "Bearer "+os.Getenv("ACCESS_TOKEN"))
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			die(e.Error())
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("HTTP %d\n%s\n", resp.StatusCode, b)
		return
	default:
		die("unknown command " + os.Args[1])
	}
	if err != nil {
		die(err.Error())
	}
	fmt.Printf("token_type=%s expires_in=%ds token=%s\n", tok.TokenType, tok.ExpiresIn, gcpauth.Redact(tok.AccessToken))
	if os.Getenv("PRINT_TOKEN") == "1" {
		fmt.Println(tok.AccessToken)
	}
}

func die(m string) {
	fmt.Fprintln(os.Stderr, "error:", m)
	os.Exit(1)
}
