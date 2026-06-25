#!/usr/bin/env bash
# Command runner for the awf repo — the single entry point for repo interactions.
# Usage: ./x <command> [args]
set -euo pipefail

cmd="${1:-}"
shift || true

case "$cmd" in
  gate)
    # Full gate: tests + vet + lint. The optional `full` arg is accepted for
    # hook compatibility (pre-push runs `./x gate full`); awf has no slower tier.
    go test ./... && go vet ./... && go tool golangci-lint run
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
  setup)
    go run ./cmd/awf setup "$@"
    ;;
  build)
    go build -o awf ./cmd/awf
    ;;
  install)
    go install ./cmd/awf
    ;;
  *)
    echo "usage: ./x <gate [full]|lint|fmt|test|sync|check|setup|build|install>" >&2
    exit 2
    ;;
esac
