#!/usr/bin/env bash
# Command runner for the awf repo - project verbs only; awf verbs go through ./awf.
# Usage: ./x <command> [args]
set -euo pipefail

cleanup_paths=()
cleanup() {
  if [ "${#cleanup_paths[@]}" -gt 0 ]; then
    rm -rf -- "${cleanup_paths[@]}"
  fi
}
trap cleanup EXIT

cmd="${1:-}"
shift || true

case "$cmd" in
  gate)
    # Full gate: profiled tests + 100% coverage check + vet + lint. The optional
    # `full` arg is accepted for hook compatibility (pre-push runs `./x gate full`);
    # awf has no slower tier. The coverage step (ADR-0012) fails below 100% of
    # non-ignored statements; -coverpkg=./... so every package contributes.
    prof="$(mktemp)"
    cleanup_paths+=("$prof")
    go test ./... -coverpkg=./... -coverprofile="$prof"
    go run ./cmd/covercheck "$prof"
    tools/pi-extension-test/container.sh run
    go vet ./...
    go tool golangci-lint run
    go tool deadcode -json ./... | go run ./cmd/deadcodecheck
    go run ./cmd/pincheck
    ./awf check prose
    ./awf check memory
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
  render)
    # The rendered ./awf wrapper runs awf from source (awfInvokeCmd) so the
    # dogfooded render always matches the tree.
    ./awf render "$@"
    # ADR-0090: re-render the example adopter with the same source. The example
    # is its own Go module, so build once and run with the example as cwd.
    bindir="$(mktemp -d)"
    cleanup_paths+=("$bindir")
    go build -o "$bindir/awf" ./cmd/awf
    (cd examples/sundial && "$bindir/awf" render)
    ;;
  check)
    ./awf check "$@"
    spill_log=".awf/local/context-spills.log"
    if [ -d .awf/local ] && [ ! -L .awf/local ] && [ -f "$spill_log" ] && [ ! -L "$spill_log" ] && [ -s "$spill_log" ]; then
      echo "check: advisory: context spills were observed; resolve or promote the issue, then remove $spill_log" >&2
    fi
    # ADR-0090: the example adopter must be drift-free, invariant-clean, free of
    # advisory notes (the model adopter has zero smells), and its scenery green.
    bindir="$(mktemp -d)"
    cleanup_paths+=("$bindir")
    go build -o "$bindir/awf" ./cmd/awf
    if ! out="$(cd examples/sundial && "$bindir/awf" check)"; then
      printf '%s\n' "$out"
      exit 1
    fi
    printf '%s\n' "$out"
    if grep -q '^note: ' <<<"$out"; then
      echo "check: the example adopter has advisory notes - author the missing content or clear the smell (ADR-0090)" >&2
      exit 1
    fi
    (cd examples/sundial && "$bindir/awf" check invariants)
    (cd examples/sundial && go test ./...)
    ;;
  context)
    capture="$(mktemp)"
    cleanup_paths+=("$capture")
    if ./awf context "$@" >"$capture"; then
      status=0
    else
      status=$?
    fi
    cat "$capture"
    if [ "$status" -eq 0 ]; then
      if ! go run ./cmd/contextspilllog --root "$PWD" --notice-file "$capture" -- ./x context "$@"; then
        echo "context: warning: spill delivered but local observability logging failed" >&2
      fi
    fi
    exit "$status"
    ;;
  pi-test)
    action="${1:-run}"
    if [ "$#" -gt 1 ]; then
      echo "usage: ./x pi-test <run|stop|reset>" >&2
      exit 2
    fi
    tools/pi-extension-test/container.sh "$action"
    ;;
  build)
    go build -o bin/awf ./cmd/awf
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
    cleanup_paths+=("$tmp")
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
    # Repo-local conformance audit (ADR-0073) - repo-specific, NOT part of the shipped
    # awf audit. Requires an explicit <base>..<head> range (ADR-0127: no default range,
    # so a call never reports over commits nobody named; the reviewing-impl override
    # passes the review's session range). Never wired into ./x gate.
    go run ./cmd/repoaudit "$@"
    ;;
  *)
    echo "usage: ./x <gate [full]|lint|fmt|test|deadcode|render|check|context|pi-test <run|stop|reset>|build|install|mutants|audit-local>" >&2
    exit 2
    ;;
esac
