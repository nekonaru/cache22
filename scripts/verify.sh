#!/bin/sh
# Local verification mirror of CI. Run before tagging or pushing a release.
set -eu

out=$(gofmt -l .)
if [ -n "$out" ]; then
  echo "not gofmt-clean:"
  echo "$out"
  exit 1
fi

go vet ./...
go test -race ./...

go test -coverprofile=coverage.out ./...
total=$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $3); print $3}')
if awk -v v="$total" 'BEGIN {exit !(v >= 60)}'; then
  echo "coverage $total% >= 60%"
else
  echo "coverage $total% < 60%"
  exit 1
fi

go build -trimpath -o bin/cache22 ./cmd/cache22
go build -trimpath -o bin/deadrop ./cmd/deadrop

if command -v docker >/dev/null 2>&1; then
  docker build -t cache22:verify .
fi
