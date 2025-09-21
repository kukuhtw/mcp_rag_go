#!/usr/bin/env bash
# tools/lint/precommit.sh
# Hook lint sederhana sebelum commit

set -e

echo "Running go fmt..."
go fmt ./...

echo "Running go vet..."
go vet ./...

echo "Running staticcheck (if available)..."
if command -v staticcheck &> /dev/null; then
  staticcheck ./...
fi

echo "Pre-commit checks done."
