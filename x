#!/usr/bin/env bash
# Small command runner for developing awf from this checkout.
set -euo pipefail

command="${1:-}"
shift || true

case "$command" in
  gate)
    [ "$#" -eq 0 ] || { echo "usage: ./x gate" >&2; exit 2; }
    unformatted="$(find . -type f -name '*.go' -not -path './.git/*' -not -path './.awf/efforts/*' -not -path './.awf/worktrees/*' -not -path './.awf/effort-archive/*' -print0 | xargs -0 gofmt -l)"
    [ -z "$unformatted" ] || { printf 'unformatted Go files:\n%s\n' "$unformatted" >&2; exit 1; }
    go test ./...
    go build ./...
    ;;
  test)
    go test ./... "$@"
    ;;
  lint)
    go vet ./... "$@"
    ;;
  fmt)
    find . -type f -name '*.go' \
      -not -path './.git/*' \
      -not -path './.awf/efforts/*' \
      -not -path './.awf/worktrees/*' \
      -not -path './.awf/effort-archive/*' \
      -print0 | xargs -0 gofmt -w
    ;;
  render|check|resolve|effort|adr|plan|version)
    go run ./cmd/awf "$command" "$@"
    ;;
  build)
    [ "$#" -eq 0 ] || { echo "usage: ./x build" >&2; exit 2; }
    mkdir -p bin
    go build -o bin/awf ./cmd/awf
    ;;
  install)
    go install ./cmd/awf
    ;;
  *)
    echo "usage: ./x <gate|test|lint|fmt|render|check|resolve|effort|adr|plan|version|build|install> [args]" >&2
    exit 2
    ;;
esac
