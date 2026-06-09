#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

GO_FILES=$(find ./cmd ./internal -name '*.go' -type f | sort)

echo "==> Checking Go formatting"
if [ -n "$GO_FILES" ]; then
  UNFORMATTED=$(gofmt -l $GO_FILES)
  if [ -n "$UNFORMATTED" ]; then
    echo "These files need gofmt:"
    printf '%s\n' "$UNFORMATTED"
    exit 1
  fi
fi

echo "==> Running go vet"
go vet ./...

echo "==> Running tests"
go test -v ./...

echo "==> Running agent/doc consistency checks"
sh ./scripts/docs_sync_check.sh

echo "Review checks passed."
