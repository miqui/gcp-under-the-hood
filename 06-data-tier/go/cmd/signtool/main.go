// signtool: generates a V4 signed URL without a key file (IAM signBlob), then
// demonstrates GCS strong read-after-write and read-after-overwrite consistency.
// Env: GCP_PROJECT_ID (informational), BUCKET, SIGNER_SA (email to sign as).
// Run with ADC that may call iamcredentials.signBlob on SIGNER_SA.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	iam "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"cloud.google.com/go/storage"
	"example.com/gcp-under-the-hood/datatier/internal/logic"
)

func must(k string) string {
	v, err := logic.Env(os.Getenv, k)
	if err != nil {
		log.Fatal(err)
	}
	return v
}

func main() {
	ctx := context.Background()
	bucket, sa := must("BUCKET"), must("SIGNER_SA")
	expiry := 15 * time.Minute
	if err := logic.ValidateExpiry(expiry); err != nil {
		log.Fatal(err)
	}

	ic, err := iam.NewIamCredentialsClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer ic.Close()
	sc, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer sc.Close()

	obj := fmt.Sprintf("demo/%d.txt", time.Now().UnixNano())
	// 1) Write v1 and immediately read: strong consistency => always sees v1.
	write := func(body string) {
		w := sc.Bucket(bucket).Object(obj).NewWriter(ctx)
		io.WriteString(w, body)
		if err := w.Close(); err != nil {
			log.Fatal(err)
		}
	}
	read := func() string {
		r, err := sc.Bucket(bucket).Object(obj).NewReader(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer r.Close()
		b, _ := io.ReadAll(r)
		return string(b)
	}
	write("v1")
	fmt.Println("read after create:   ", read())
	write("v2")
	fmt.Println("read after overwrite:", read())

	// 2) V4 signed GET URL; the signature is computed remotely by IAM.
	u, err := sc.Bucket(bucket).SignedURL(obj, &storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,
		Method:         "GET",
		Expires:        time.Now().Add(expiry),
		GoogleAccessID: sa,
		SignBytes: func(b []byte) ([]byte, error) {
			resp, err := ic.SignBlob(ctx, &credentialspb.SignBlobRequest{
				Name:    "projects/-/serviceAccounts/" + sa,
				Payload: b,
			})
			if err != nil {
				return nil, err
			}
			return resp.SignedBlob, nil
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("signed URL host/path:", strings.SplitN(u, "?", 2)[0])

	// 3) Use it with a plain HTTP client: no credentials.
	resp, err := http.Get(u)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("anonymous GET via signed URL: %d body=%q\n", resp.StatusCode, b)
}
