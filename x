#!/usr/bin/env bash
# Command runner for the awf repo — the single entry point for repo interactions.
# Usage: ./x <command> [args]
set -euo pipefail

cmd="${1:-}"
shift || true

case "$cmd" in
  gate)
    # Full gate: profiled tests + 100% coverage check + vet + lint. The optional
    # `full` arg is accepted for hook compatibility (pre-push runs `./x gate full`);
    # awf has no slower tier. The coverage step (ADR-0012) fails below 100% of
    # non-ignored statements; -coverpkg=./... so every package contributes.
    prof="$(mktemp)"
    trap 'rm -f "$prof"' EXIT
    go test ./... -coverpkg=./... -coverprofile="$prof"
    go run ./cmd/covercheck "$prof"
    go vet ./...
    go tool golangci-lint run
    ;;
  lint)
    go tool golangci-lint run "$@"
    ;;
  fmt)
    go tool golangci-lint fmt "$@"
    ;;
  test)
    go test ./... "$@"
    ;;
  sync)
    # Run awf from source so the dogfooded render always matches the tree.
    go run ./cmd/awf sync "$@"
    ;;
  check)
    go run ./cmd/awf check "$@"
    ;;
  invariants)
    go run ./cmd/awf invariants "$@"
    ;;
  audit)
    go run ./cmd/awf audit "$@"
    ;;
  commit-gate)
    go run ./cmd/awf commit-gate "$@"
    ;;
  build)
    go build -o awf ./cmd/awf
    ;;
  install)
    go install ./cmd/awf
    ;;
  *)
    echo "usage: ./x <gate [full]|lint|fmt|test|sync|check|invariants|audit|commit-gate|build|install>" >&2
    exit 2
    ;;
esac
