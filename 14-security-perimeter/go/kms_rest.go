package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// RESTKMS calls the Cloud KMS REST API using a bearer token
// (e.g. from `gcloud auth print-access-token`). Stdlib only.
type RESTKMS struct {
	KeyName string // projects/P/locations/L/keyRings/R/cryptoKeys/K
	Token   string
	Base    string // defaults to https://cloudkms.googleapis.com/v1
	HTTP    *http.Client
}

func (r *RESTKMS) call(ctx context.Context, op string, in map[string]string) (string, error) {
	base := r.Base
	if base == "" {
		base = "https://cloudkms.googleapis.com/v1"
	}
	cl := r.HTTP
	if cl == nil {
		cl = http.DefaultClient
	}
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s:%s", base, r.KeyName, op), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kms %s: %s: %s", op, resp.Status, string(b))
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	field := "ciphertext"
	if op == "decrypt" {
		field = "plaintext"
	}
	s, _ := out[field].(string)
	return s, nil
}

func (r *RESTKMS) Wrap(ctx context.Context, p, aad []byte) ([]byte, error) {
	s, err := r.call(ctx, "encrypt", map[string]string{
		"plaintext":                   base64.StdEncoding.EncodeToString(p),
		"additionalAuthenticatedData": base64.StdEncoding.EncodeToString(aad),
	})
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(s)
}

func (r *RESTKMS) Unwrap(ctx context.Context, w, aad []byte) ([]byte, error) {
	s, err := r.call(ctx, "decrypt", map[string]string{
		"ciphertext":                  base64.StdEncoding.EncodeToString(w),
		"additionalAuthenticatedData": base64.StdEncoding.EncodeToString(aad),
	})
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(s)
}
