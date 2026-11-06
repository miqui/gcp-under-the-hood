// Command app reports pod/node identity and demonstrates the Workload
// Identity token flow against the GKE metadata server.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const defaultMetadata = "http://metadata.google.internal"

// Server holds injectable endpoints so tests can use httptest.
type Server struct {
	MetadataURL string
	StorageURL  string
	Client      *http.Client
	Env         func(string) string
}

func NewServer() *Server {
	return &Server{
		MetadataURL: defaultMetadata,
		StorageURL:  "https://storage.googleapis.com",
		Client:      &http.Client{Timeout: 5 * time.Second},
		Env:         os.Getenv,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.whoami)
	mux.HandleFunc("/metadata", s.metadata)
	mux.HandleFunc("/gcs", s.gcs)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// whoami: values come from the Downward API (see k8s/deployment.yaml).
func (s *Server) whoami(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	writeJSON(w, 200, map[string]string{
		"pod":         host,
		"pod_ip":      s.Env("POD_IP"),
		"node":        s.Env("NODE_NAME"),
		"namespace":   s.Env("POD_NAMESPACE"),
		"ksa":         s.Env("KSA_NAME"),
		"remote_addr": r.RemoteAddr,
	})
}

func (s *Server) mdGet(path string) (string, int, error) {
	req, err := http.NewRequest("GET", s.MetadataURL+path, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return string(b), resp.StatusCode, nil
}

// metadata: with Workload Identity the GKE metadata server answers these calls
// (not the node's real GCE metadata server).
func (s *Server) metadata(w http.ResponseWriter, _ *http.Request) {
	out := map[string]string{}
	for k, p := range map[string]string{
		"project_id": "/computeMetadata/v1/project/project-id",
		"sa_email":   "/computeMetadata/v1/instance/service-accounts/default/email",
	} {
		v, code, err := s.mdGet(p)
		if err != nil {
			writeJSON(w, 502, map[string]string{"error": err.Error()})
			return
		}
		out[k] = fmt.Sprintf("%d:%s", code, v)
	}
	writeJSON(w, 200, out)
}

// gcs: fetches a token from the metadata server, then lists the bucket.
func (s *Server) gcs(w http.ResponseWriter, _ *http.Request) {
	bucket := s.Env("BUCKET")
	if bucket == "" {
		writeJSON(w, 400, map[string]string{"error": "BUCKET env not set"})
		return
	}
	body, code, err := s.mdGet("/computeMetadata/v1/instance/service-accounts/default/token")
	if err != nil || code != 200 {
		writeJSON(w, 502, map[string]any{"error": "token fetch failed", "status": code, "detail": fmt.Sprint(err)})
		return
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(body), &tok); err != nil || tok.AccessToken == "" {
		writeJSON(w, 502, map[string]string{"error": "bad token response"})
		return
	}
	req, err := http.NewRequest("GET", s.StorageURL+"/storage/v1/b/"+bucket+"/o", nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := s.Client.Do(req)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	// Never echo the token; only report status.
	writeJSON(w, 200, map[string]any{"token_obtained": true, "gcs_status": resp.StatusCode})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, NewServer().Routes()))
}
