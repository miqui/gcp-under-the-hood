// Package bq is a minimal stdlib-only BigQuery REST client (jobs.query dry-run
// and DDL/DML jobs). Auth: bearer token from GCP_ACCESS_TOKEN
// (e.g. `gcloud auth print-access-token`).
package bq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Client calls the BigQuery v2 REST API.
type Client struct {
	Project string
	Token   string
	BaseURL string // default https://bigquery.googleapis.com
	HTTP    *http.Client
}

// DryRunResult is the subset of jobs.query dry-run response we use.
type DryRunResult struct {
	TotalBytesProcessed int64
	CacheHit            bool
}

type queryReq struct {
	Query        string `json:"query"`
	UseLegacySQL bool   `json:"useLegacySql"`
	DryRun       bool   `json:"dryRun"`
	UseCache     bool   `json:"useQueryCache"`
	Location     string `json:"location,omitempty"`
}

type queryResp struct {
	TotalBytesProcessed string `json:"totalBytesProcessed"`
	CacheHit            bool   `json:"cacheHit"`
	JobComplete         bool   `json:"jobComplete"`
	NumDMLAffectedRows  string `json:"numDmlAffectedRows"`
}

func (c *Client) post(ctx context.Context, body queryReq) (*queryResp, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://bigquery.googleapis.com"
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/bigquery/v2/projects/%s/queries", base, c.Project)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bigquery %d: %s", resp.StatusCode, string(data))
	}
	var qr queryResp
	if err := json.Unmarshal(data, &qr); err != nil {
		return nil, err
	}
	return &qr, nil
}

// DryRun returns bytes the query would process, without running it (free).
func (c *Client) DryRun(ctx context.Context, sql, location string) (DryRunResult, error) {
	qr, err := c.post(ctx, queryReq{Query: sql, DryRun: true, UseCache: true, Location: location})
	if err != nil {
		return DryRunResult{}, err
	}
	n, err := strconv.ParseInt(qr.TotalBytesProcessed, 10, 64)
	if err != nil {
		return DryRunResult{}, fmt.Errorf("parse totalBytesProcessed %q: %w", qr.TotalBytesProcessed, err)
	}
	return DryRunResult{TotalBytesProcessed: n, CacheHit: qr.CacheHit}, nil
}

// Exec runs a DML/DDL statement (billed!) and returns affected rows.
func (c *Client) Exec(ctx context.Context, sql, location string) (int64, error) {
	qr, err := c.post(ctx, queryReq{Query: sql, UseCache: false, Location: location})
	if err != nil {
		return 0, err
	}
	if qr.NumDMLAffectedRows == "" {
		return 0, nil
	}
	return strconv.ParseInt(qr.NumDMLAffectedRows, 10, 64)
}
