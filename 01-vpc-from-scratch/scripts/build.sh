#!/usr/bin/env bash
# Test, then cross-compile the probe for the lab VMs (linux/amd64, e2 machines).
set -euo pipefail
cd "$(dirname "$0")/../go"
go test ./...
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/probe ./probe
echo "built go/bin/probe"
