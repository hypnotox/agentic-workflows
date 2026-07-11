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
    go tool deadcode -json ./... | go run ./cmd/deadcodecheck
    go run ./cmd/pincheck
    ;;
  lint)
    go tool golangci-lint run "$@"
    ;;
  deadcode)
    # Whole-program dead-code gate (ADR-0063): fails on any production func
    # unreachable from a main outside internal/testsupport/. Run without -test.
    go tool deadcode -json ./... | go run ./cmd/deadcodecheck
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
    # ADR-0090: re-render the example adopter with the same source. The example
    # is its own Go module, so build once and run with the example as cwd.
    bindir="$(mktemp -d)"
    trap 'rm -rf "$bindir"' EXIT
    go build -o "$bindir/awf" ./cmd/awf
    (cd examples/sundial && "$bindir/awf" sync)
    ;;
  check)
    go run ./cmd/awf check "$@"
    # ADR-0090: the example adopter must be drift-free, invariant-clean, free of
    # advisory notes (the model adopter has zero smells), and its scenery green.
    bindir="$(mktemp -d)"
    trap 'rm -rf "$bindir"' EXIT
    go build -o "$bindir/awf" ./cmd/awf
    if ! out="$(cd examples/sundial && "$bindir/awf" check)"; then
      printf '%s\n' "$out"
      exit 1
    fi
    printf '%s\n' "$out"
    if grep -q '^note: ' <<<"$out"; then
      echo "check: the example adopter has advisory notes — author the missing content or clear the smell (ADR-0090)" >&2
      exit 1
    fi
    (cd examples/sundial && "$bindir/awf" invariants)
    (cd examples/sundial && go test ./...)
    ;;
  invariants)
    go run ./cmd/awf invariants "$@"
    ;;
  audit)
    go run ./cmd/awf audit "$@"
    ;;
  context)
    go run ./cmd/awf context "$@"
    ;;
  commit-gate)
    go run ./cmd/awf commit-gate "$@"
    ;;
  new)
    # Scaffold a new ADR (or other awf artifact) from source, e.g. ./x new adr "<title>".
    go run ./cmd/awf new "$@"
    ;;
  build)
    go build -o awf ./cmd/awf
    ;;
  install)
    go install ./cmd/awf
    ;;
  mutants)
    # Advisory mutation triage (ADR-0066). No args: mutate production code changed
    # vs main. A path arg (e.g. ./internal/refs): mutate that package. Never gated.
    # Under .gremlins.yaml the efficacy/coverage thresholds stay 0, so gremlins exits
    # 0 even with survivors and set -e does not abort before cmd/mutants runs.
    tmp="$(mktemp)"
    trap 'rm -f "$tmp"' EXIT
    if [ "$#" -gt 0 ]; then
      go tool gremlins unleash -o "$tmp" "$@"
    else
      base="$(git merge-base HEAD main)" || {
        echo "mutants: no merge-base with 'main' (detached HEAD or missing branch); pass a package path, e.g. ./x mutants ./internal/refs" >&2
        exit 2
      }
      go tool gremlins unleash -D "$base" -o "$tmp" ./...
    fi
    go run ./cmd/mutants "$tmp"
    ;;
  audit-local)
    # Repo-local conformance audit (ADR-0073) — repo-specific, NOT part of the shipped
    # awf audit. Default range origin/main..HEAD; pass <base>..<head> to scope it (the
    # reviewing-impl override passes the review's session range). Never wired into ./x gate.
    go run ./cmd/repoaudit "$@"
    ;;
  *)
    echo "usage: ./x <gate [full]|lint|fmt|test|deadcode|sync|check|invariants|audit|commit-gate|new|build|install|mutants|audit-local>" >&2
    exit 2
    ;;
esac
