#!/usr/bin/env bash

set -euo pipefail

run_with_fallback() {
    local binary="$1"
    local module="$2"
    shift 2

    if command -v "$binary" >/dev/null 2>&1; then
        if "$binary" "$@"; then
            return
        fi
        echo "$binary failed; retrying via 'go run ${module}@latest'." >&2
    fi

    if ! command -v "$binary" >/dev/null 2>&1; then
        echo "$binary not found in PATH; using 'go run ${module}@latest' (slower)." >&2
    fi
    go run "${module}@latest" "$@"
}

echo "Running go vet..."
go vet ./...

echo "Running govulncheck..."
run_with_fallback govulncheck golang.org/x/vuln/cmd/govulncheck ./...

echo "Running gosec..."
run_with_fallback gosec github.com/securego/gosec/v2/cmd/gosec ./...
