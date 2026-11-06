// Command service is a tiny HTTP app that reports its Cloud Run revision.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

var version = "v1" // override with -ldflags "-X main.version=v2"

func handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version":  version,
		"revision": os.Getenv("K_REVISION"),
		"service":  os.Getenv("K_SERVICE"),
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
