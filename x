#!/usr/bin/env bash
# Command runner for the awf repo - project verbs only; awf verbs go through ./awf.
# Usage: ./x <command> [args]
set -euo pipefail

# Keep lint diagnostics scoped to this checkout. The shared cache records
# absolute positions and otherwise leaks paths from managed worktrees.
export GOLANGCI_LINT_CACHE="${PWD}/.cache/golangci-lint"
# Nested Pi runtime tests need the host's NVM location when available. CI uses
# its setup-node runtime when this path is absent.
: "${AWF_PI_TEST_NVM_DIR:=${NVM_DIR:-$HOME/.nvm}}"
export AWF_PI_TEST_NVM_DIR

gate_timings=false
run_gate_step() {
  local label="$1"
  shift
  local started=$SECONDS status
  if "$@"; then status=0; else status=$?; fi
  if "$gate_timings"; then
    printf 'gate timing: %s %ss\n' "$label" "$((SECONDS - started))" >&2
  fi
  return "$status"
}

run_advisory_lint() {
  local output status
  if output="$(go tool golangci-lint run --config .golangci-advisory.yml --issues-exit-code 0 "$@" 2>&1)"; then
    status=0
  else
    status=$?
    printf '%s\n' "$output" >&2
    return "$status"
  fi
  if [ -n "$output" ]; then
    echo "warning: advisory lint findings" >&2
    printf '%s\n' "$output" >&2
  fi
}

cmd="${1:-}"
shift || true

case "$cmd" in
  gate)
    while [ "$#" -gt 0 ]; do
      case "$1" in
        timings) gate_timings=true ;;
        *) echo "usage: ./x gate [timings]" >&2; exit 2 ;;
      esac
      shift
    done
    run_gate_step versioncheck go run ./cmd/versioncheck
    run_gate_step build go build ./...
    run_gate_step lint go tool golangci-lint run
    run_gate_step pincheck go run ./cmd/pincheck
    ;;
  lint)
    go tool golangci-lint run "$@"
    run_advisory_lint "$@"
    ;;
  deadcode)
    # Optional whole-program analysis. It is intentionally outside hooks and CI.
    go tool deadcode -json ./... | go run ./cmd/deadcodecheck
    ;;
  fmt)
    go tool golangci-lint fmt --config .golangci-advisory.yml "$@"
    ;;
  test)
    echo "test: Pi host lane skipped; run './x pi-test run' separately" >&2
    env -u AWF_PI_RUNTIME_SMOKE go test ./... "$@"
    ;;
  test-affected)
    go run ./cmd/testselection --execute "$@"
    ;;
  test-full-linux)
    mode="${1:-}"
    shift || true
    case "$mode" in
      calibrate|budget) ;;
      *) echo "usage: ./x test-full-linux <calibrate|budget> [--artifact FILE]" >&2; exit 2 ;;
    esac
    artifact="${AWF_FULL_LINUX_TIMING_ARTIFACT:-$PWD/.cache/full-linux-timing.json}"
    if [ "${1:-}" = "--artifact" ]; then
      [ "$#" -ge 2 ] || { echo "missing artifact path" >&2; exit 2; }
      artifact="$2"
      shift 2
    fi
    [ "$#" -eq 0 ] || { echo "usage: ./x test-full-linux <calibrate|budget> [--artifact FILE]" >&2; exit 2; }
    if [ "$mode" = budget ] && [ -z "${AWF_FULL_LINUX_CEILING:-}" ]; then
      echo "test-full-linux: budget requires AWF_FULL_LINUX_CEILING from reviewed hosted evidence" >&2
      exit 2
    fi
    timing_args=(full-linux --mode "$mode" --artifact "$artifact")
    if [ -n "${AWF_FULL_LINUX_CEILING:-}" ]; then
      timing_args+=(--ceiling "$AWF_FULL_LINUX_CEILING")
    fi
    go run ./cmd/testselection "${timing_args[@]}"
    ;;
  clean-test-tmp)
    go run ./internal/testsupport/cmd/testtmpclean "$@"
    ;;
  render)
    # Run from source so dogfooded rendering always matches this tree.
    ./awf render "$@"
    ;;
  check)
    ./awf check "$@"
    ;;
  pi-test)
    action="${1:-run}"
    if [ "$#" -gt 1 ] || [ "$action" != run ]; then
      echo "usage: ./x pi-test <run>" >&2
      exit 2
    fi
    tools/pi-extension-test/run.sh "$action"
    ;;
  build)
    go build -o bin/awf ./cmd/awf
    ;;
  install)
    go install ./cmd/awf
    ;;
  audit-local)
    # Repository-specific conformance audit; never part of the shipped awf audit
    # or the ordinary gate. It requires an explicit <base>..<head> range.
    go run ./cmd/repoaudit "$@"
    ;;
  *)
    echo "usage: ./x <gate [timings]|lint|fmt|test|test-affected [--staged|--range <base>..<head>]|test-full-linux <calibrate|budget> [--artifact FILE]|clean-test-tmp [--all]|deadcode|render|check|pi-test <run>|build|install|audit-local>" >&2
    exit 2
    ;;
esac
