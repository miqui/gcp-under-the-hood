// Package gcpauth implements the three GCP token flows over plain HTTP so the
// wire protocol is visible. Stdlib only.
package gcpauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	DefaultMetadataBase = "http://metadata.google.internal"
	DefaultSTSURL       = "https://sts.googleapis.com/v1/token"
	DefaultIAMCredBase  = "https://iamcredentials.googleapis.com"
)

// Token is an OAuth2 access token.
type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// Client holds endpoints (overridable for tests) and an HTTP client.
type Client struct {
	HTTP         *http.Client
	MetadataBase string
	STSURL       string
	IAMCredBase  string
	Log          func(format string, args ...any) // optional wire logging
}

func New() *Client {
	return &Client{HTTP: http.DefaultClient, MetadataBase: DefaultMetadataBase, STSURL: DefaultSTSURL, IAMCredBase: DefaultIAMCredBase}
}

func (c *Client) logf(f string, a ...any) {
	if c.Log != nil {
		c.Log(f, a...)
	}
}

func (c *Client) do(req *http.Request, out any) error {
	c.logf("> %s %s", req.Method, req.URL)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	c.logf("< %d (%d bytes)", resp.StatusCode, len(body))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: HTTP %d: %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, out)
}

// MetadataToken fetches the attached service account's token from the metadata
// server. The Metadata-Flavor header is mandatory (SSRF defence).
func (c *Client) MetadataToken(ctx context.Context, account string) (*Token, error) {
	if account == "" {
		account = "default"
	}
	u := fmt.Sprintf("%s/computeMetadata/v1/instance/service-accounts/%s/token", c.MetadataBase, url.PathEscape(account))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	var t Token
	if err := c.do(req, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// STSExchange trades an external subject token (e.g. OIDC JWT) for a federated
// access token (RFC 8693 token exchange).
func (c *Client) STSExchange(ctx context.Context, audience, subjectToken string) (*Token, error) {
	body, _ := json.Marshal(map[string]string{
		"grantType":          "urn:ietf:params:oauth:grant-type:token-exchange",
		"audience":           audience,
		"scope":              "https://www.googleapis.com/auth/cloud-platform",
		"requestedTokenType": "urn:ietf:params:oauth:token-type:access_token",
		"subjectToken":       subjectToken,
		"subjectTokenType":   "urn:ietf:params:oauth:token-type:jwt",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.STSURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var t Token
	if err := c.do(req, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// GenerateAccessToken calls iamcredentials.generateAccessToken using callerToken
// to mint a short-lived token for targetSA (requires serviceAccountTokenCreator).
func (c *Client) GenerateAccessToken(ctx context.Context, callerToken, targetSA string, lifetimeSeconds int) (*Token, error) {
	u := fmt.Sprintf("%s/v1/projects/-/serviceAccounts/%s:generateAccessToken", c.IAMCredBase, url.PathEscape(targetSA))
	body, _ := json.Marshal(map[string]any{
		"scope":    []string{"https://www.googleapis.com/auth/cloud-platform"},
		"lifetime": fmt.Sprintf("%ds", lifetimeSeconds),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+callerToken)
	req.Header.Set("Content-Type", "application/json")
	var raw struct {
		AccessToken string `json:"accessToken"`
		ExpireTime  string `json:"expireTime"`
	}
	if err := c.do(req, &raw); err != nil {
		return nil, err
	}
	return &Token{AccessToken: raw.AccessToken, ExpiresIn: lifetimeSeconds, TokenType: "Bearer"}, nil
}

// Redact hides all but the prefix of a secret for safe printing.
func Redact(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:6] + "...(redacted, " + fmt.Sprint(len(s)) + " chars)"
}
