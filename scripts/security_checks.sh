#!/usr/bin/env bash

set -euo pipefail

echo "Running go vet..."
go vet ./...

echo "Running govulncheck..."
go run golang.org/x/vuln/cmd/govulncheck@latest ./...

echo "Running gosec..."
go run github.com/securego/gosec/v2/cmd/gosec@latest ./...
