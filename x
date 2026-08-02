#!/usr/bin/env bash
# Command runner for the awf repo - project verbs only; awf verbs go through ./awf.
# Usage: ./x <command> [args]
set -euo pipefail

# Per-checkout lint cache: the default shared cache is content-keyed but stores
# absolute file positions, so a byte-identical package linted in another checkout
# (a managed .awf/worktrees/ tree) leaks that checkout's paths into this one's
# findings, pointing at files that vanish when the worktree is removed.
export GOLANGCI_LINT_CACHE="${PWD}/.cache/golangci-lint"

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
    # `full` arg is accepted as a no-op legacy argument; no rendered artifact
    # passes it, and awf has no slower tier. The coverage step (ADR-0012) fails
    # below 100% of non-ignored statements; -coverpkg=./... so every package
    # contributes.
    # The profile is durable (gitignored via *.out) so CI can upload it to
    # Codecov without rerunning the suite, and an interrupted run leaks no
    # tmpfs file (ADR-0196).
    prof="coverage.out"
    go test ./... -coverpkg=./... -coverprofile="$prof"
    go run ./cmd/covercheck "$prof"
    tools/pi-extension-test/container.sh run
    go vet ./...
    # Cross-compile gate: the suite only ever runs on the host platform, so a
    # package that stops building for a contributor's platform is otherwise
    # invisible until they clone. The matrix is the released set
    # (.goreleaser.yaml: linux, darwin, and windows for amd64 and arm64) minus
    # the host, which the steps above already build. Deriving it from the host
    # rather than hardcoding non-linux targets keeps a linux-only break visible
    # to a contributor gating on macOS.
    host="$(go env GOOS)/$(go env GOARCH)"
    for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
      if [ "$target" != "$host" ]; then
        GOOS="${target%/*}" GOARCH="${target#*/}" go build ./...
      fi
    done
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
  clean-test-tmp)
    go run ./cmd/testtmpclean "$@"
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
    if ! go run ./cmd/contextspilllog --check-log --root "$PWD"; then
      echo "check: warning: context spill advisory inspection failed; resolve or promote the issue before removing the log" >&2
    fi
    # ADR-0090: the example adopter must be drift-free, authority-clean, free of
    # advisory notes (the model adopter has zero smells), and its scenery green.
    bindir="$(mktemp -d)"
    cleanup_paths+=("$bindir")
    go build -o "$bindir/awf" ./cmd/awf
    if ! out="$(cd examples/sundial && "$bindir/awf" check repo)"; then
      printf '%s\n' "$out"
      exit 1
    fi
    printf '%s\n' "$out"
    if grep -q '^note: ' <<<"$out"; then
      echo "check: the example adopter has advisory notes - author the missing content or clear the smell (ADR-0090)" >&2
      exit 1
    fi
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
      echo "usage: ./x pi-test <run|reset>" >&2
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
    echo "usage: ./x <gate [full]|lint|fmt|test|clean-test-tmp [--all]|deadcode|render|check|context|pi-test <run|reset>|build|install|mutants|audit-local>" >&2
    exit 2
    ;;
esac
