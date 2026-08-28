#!/usr/bin/env bash
# Command runner for the awf repo - project verbs only; awf verbs go through ./awf.
# Usage: ./x <command> [args]
set -euo pipefail

# Per-checkout lint cache: the default shared cache is content-keyed but stores
# absolute file positions, so a byte-identical package linted in another checkout
# (a managed .awf/worktrees/ tree) leaks that checkout's paths into this one's
# findings, pointing at files that vanish when the worktree is removed.
export GOLANGCI_LINT_CACHE="${PWD}/.cache/golangci-lint"
# Go's tests isolate HOME, but the nested real Pi smoke still needs the NVM
# location discovered by this host process. CI has no NVM and falls through to
# its setup-node runtime when this path does not exist.
: "${AWF_PI_TEST_NVM_DIR:=${NVM_DIR:-$HOME/.nvm}}"
export AWF_PI_TEST_NVM_DIR

cleanup_paths=()
cleanup() {
  if [ "${#cleanup_paths[@]}" -gt 0 ]; then
    rm -rf -- "${cleanup_paths[@]}"
  fi
}
trap cleanup EXIT

gate_timings=false
run_gate_step() {
  local label="$1"
  shift
  local started=$SECONDS status
  if "$@"; then
    status=0
  else
    status=$?
  fi
  if "$gate_timings"; then
    printf 'gate timing: %s %ss\n' "$label" "$((SECONDS - started))" >&2
  fi
  return "$status"
}

run_deadcode_gate() {
  go tool deadcode -json ./... | go run ./cmd/deadcodecheck
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

covercheck_mutants_path_owned() {
  case "$1" in cmd/covercheck|cmd/covercheck/*) return 0;; *) return 1;; esac
}

# Return selected (0), not selected (1), or uncertain (2).  The full gate
# treats uncertainty as selected under ADR-0302's conservative trust contract.
covercheck_mutants_selected() {
  local mode="$1" path stream size consumed=0 saw=false
  shift
  stream="$(mktemp)"; cleanup_paths+=("$stream")
  case "$mode" in
    staged) git diff --cached --name-only -z --no-renames >"$stream" || return 2 ;;
    ranges)
      [ "$#" -gt 0 ] || return 2
      while [ "$#" -gt 0 ]; do
        [ "$#" -ge 2 ] && git cat-file -e "$1^{commit}" && git cat-file -e "$2^{commit}" || return 2
        git diff --name-only -z --no-renames "$1" "$2" -- >>"$stream" || return 2
        shift 2
      done ;;
    *) return 2 ;;
  esac
  while IFS= read -r -d '' path; do
    [ -n "$path" ] || return 2
    saw=true; consumed=$((consumed + ${#path} + 1))
    covercheck_mutants_path_owned "$path" && return 0
  done <"$stream"
  size="$(wc -c <"$stream")" || return 2
  [ "$consumed" -eq "$size" ] || return 2
  return 1
}

run_pi_runtime_smoke() {
  local output status
  if output="$(env AWF_PI_RUNTIME_SMOKE=1 go test -json ./internal/publisher -run '^TestPi(EffortMemoryToolContract|RealRuntimeSmoke)$' -count=1)"; then
    status=0
  else
    status=$?
  fi
  printf '%s\n' "$output"
  if [ "$status" -ne 0 ]; then
    return "$status"
  fi
  for proving_unit in TestPiEffortMemoryToolContract TestPiRealRuntimeSmoke; do
    if ! grep -q '"Action":"pass".*"Test":"'"$proving_unit"'"' <<<"$output"; then
      echo "gate: Pi runtime smoke proving units did not run and pass" >&2
      return 1
    fi
  done
}

run_covercheck_mutants() {
  local root evidence baseline selection=always base= head= arg tmp config dry actual status
  root="$(git rev-parse --show-toplevel)" || { echo "covercheck-mutants: repository root unavailable" >&2; return 1; }
  evidence="$root/.cache/covercheck-mutants-evidence"
  baseline="$root/coverage-baseline.json"
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --select-staged) [ "$selection" = always ] || { echo "covercheck-mutants: select flags conflict" >&2; return 2; }; selection=staged ;;
      --select-range) [ "$selection" = always ] && [ "$#" -ge 3 ] || { echo "usage: ./x covercheck-mutants [--select-staged|--select-range <base> <head>] [--evidence <dir>] [--baseline <path>]" >&2; return 2; }; selection=range; base="$2"; head="$3"; shift 2 ;;
      --evidence) [ "$#" -ge 2 ] || return 2; evidence="$2"; shift ;;
      --baseline) [ "$#" -ge 2 ] || return 2; baseline="$2"; shift ;;
      *) echo "usage: ./x covercheck-mutants [--select-staged|--select-range <base> <head>] [--evidence <dir>] [--baseline <path>]" >&2; return 2 ;;
    esac
    shift
  done
  case "$selection" in
    staged) if covercheck_mutants_selected staged; then status=0; else status=$?; fi ;;
    range) if covercheck_mutants_selected range "$base" "$head"; then status=0; else status=$?; fi ;;
    *) status=0 ;;
  esac
  if [ "$status" -eq 1 ]; then echo "covercheck-mutants: no cmd/covercheck changes" >&2; return 0; fi
  if [ "$status" -eq 2 ]; then echo "covercheck-mutants: uncertain change selection; running blocker" >&2; fi
  mkdir -p -- "$evidence" || return 1
  # Enclose every expensive trust step in one aggregate deadline.
  timeout 1800s bash "$0" __covercheck-mutants-inner "$root" "$evidence" "$baseline"
}

run_mutation_segment() {
  local segments="$1" name="$2" started ended status
  shift 2
  started="$(date +%s%N)"
  if "$@"; then status=0; else status=$?; fi
  ended="$(date +%s%N)"
  printf '%s\t%s\t%s\t%s\n' "$name" "$started" "$ended" "$status" >>"$segments"
  return "$status"
}

cleanup_covercheck_mutation_tmp() {
  local owned="$1" evidence="$2" status
  if lsof +D "$owned" >"$evidence/cleanup-lsof.txt" 2>&1; then status=0; else status=$?; fi
  if [ "$status" -ne 1 ]; then
    echo "covercheck-mutants: temporary root still has an owner or cannot be inspected: $owned" >&2
    printf 'preserved=%s lsof_status=%s\n' "$owned" "$status" >"$evidence/cleanup.txt"
    return 1
  fi
  rm -rf -- "$owned"
  printf 'removed=%s lsof_status=1\n' "$owned" >"$evidence/cleanup.txt"
}

covercheck_mutation_tmp=
covercheck_mutation_evidence=
cleanup_covercheck_mutation_exit() {
  local status=$?
  trap - EXIT TERM
  cleanup_covercheck_mutation_tmp "$covercheck_mutation_tmp" "$covercheck_mutation_evidence" || exit 1
  exit "$status"
}

run_covercheck_mutants_inner() {
  local root="$1" evidence="$2" baseline="$3" name value tmp config dry actual imports deps tool segments
  local census=() expected=() operators=()
  # Preserve the command transcript alongside both machine reports for later
  # qualification review without placing evidence in the source tree.
  exec > >(tee -a "$evidence/runner.log") 2>&1
  for name in $(compgen -v); do
    case "$name" in GREMLINS_*) echo "covercheck-mutants: GREMLINS_* overrides are forbidden" >&2; return 1;; esac
  done
  printf '(none)\n' >"$evidence/gremlins-environment.txt"
  [ "$(go list -m -f '{{.Version}}' github.com/go-gremlins/gremlins)" = v0.6.0 ] || { echo "covercheck-mutants: gremlins module must be v0.6.0" >&2; return 1; }
  tool="$(go tool -n gremlins)" || return 1
  go version -m "$tool" >"$evidence/tool-version.txt"
  grep -F $'mod\tgithub.com/go-gremlins/gremlins\tv0.6.0\t' "$evidence/tool-version.txt" >/dev/null || { echo "covercheck-mutants: gremlins tool must be v0.6.0" >&2; return 1; }
  tmp="$(mktemp -d /tmp/cXXXXXX)" || return 1
  covercheck_mutation_tmp="$tmp"
  covercheck_mutation_evidence="$evidence"
  trap cleanup_covercheck_mutation_exit EXIT TERM
  export TMPDIR="$tmp" GOTMPDIR="$tmp" TMP="$tmp" TEMP="$tmp"
  df -B1 /tmp >"$evidence/capacity-before.txt"
  [ "$(df -Pk /tmp | awk 'END { print $4 }')" -ge 1048576 ] || { echo "covercheck-mutants: /tmp capacity below 1 GiB" >&2; return 1; }
  if git -C "$tmp" rev-parse --show-toplevel >"$evidence/git-isolation.stdout" 2>"$evidence/git-isolation.stderr"; then echo "covercheck-mutants: temporary root is inside Git discovery" >&2; return 1; fi
  printf 'outside-git=%s\n' "$tmp" >"$evidence/git-isolation.txt"
  config="$tmp/gremlins.yaml"; printf 'config: {}\n' >"$config"
  sha256sum "$config" >"$evidence/config.sha256"
  git rev-parse HEAD >"$evidence/base-head.txt"
  git write-tree >"$evidence/staged-tree.txt"
  mapfile -t census < <(find "$root/cmd/covercheck" -maxdepth 1 -type f -name '*_test.go' -printf '%f\n' | LC_ALL=C sort)
  mapfile -t expected < <(go list -f '{{join .TestGoFiles "\n"}}{{"\n"}}{{join .XTestGoFiles "\n"}}' ./cmd/covercheck | sed '/^$/d' | LC_ALL=C sort)
  [ "${census[*]}" = "${expected[*]}" ] || { echo "covercheck-mutants: test census differs from no-tag go list" >&2; return 1; }
  printf '%s\n' "${census[@]}" >"$evidence/test-files.txt"
  imports="$(go list -f '{{join .Imports "\n"}}' ./cmd/covercheck)"
  grep -Fxq 'github.com/hypnotox/agentic-workflows/internal/coverage' <<<"$imports" || { echo "covercheck-mutants: cmd/covercheck lacks direct internal/coverage import" >&2; return 1; }
  deps="$(go list -deps -test ./cmd/covercheck)"
  grep -Fxq 'github.com/hypnotox/agentic-workflows/internal/coverage' <<<"$deps" || { echo "covercheck-mutants: compiled covercheck tests lack internal/coverage dependency" >&2; return 1; }
  printf 'direct-import=true\ncompiled-test-dependency=true\n' >"$evidence/dependency-contract.txt"
  while IFS= read -r value; do operators+=("$value"); done < <(go run ./cmd/mutants operators)
  [ "${#operators[@]}" -eq 11 ] || { echo "covercheck-mutants: operator inventory is incomplete" >&2; return 1; }
  printf '%s\n' "${operators[@]}" >"$evidence/operators.txt"
  dry="$evidence/dry.json"; actual="$evidence/actual.json"; segments="$evidence/segments.tsv"
  rm -f -- "$dry" "$actual"
  : >"$segments"
  run_mutation_segment "$segments" preflight go test -p=1 -timeout=20m -count=1 ./...
  run_mutation_segment "$segments" discovery go tool gremlins --config "$config" unleash --integration=false --workers=1 --test-cpu=1 --timeout-coefficient=20 --threshold-efficacy=0 --threshold-mcover=0 --tags= --coverpkg= --diff= --dry-run "${operators[@]}" --output "$dry" ./cmd/covercheck
  run_mutation_segment "$segments" mutation go tool gremlins --config "$config" unleash --integration=false --workers=1 --test-cpu=1 --timeout-coefficient=20 --threshold-efficacy=0 --threshold-mcover=0 --tags= --coverpkg= --diff= "${operators[@]}" --output "$actual" ./cmd/covercheck
  run_mutation_segment "$segments" validation sh -c 'go run ./cmd/mutants validate "$1" "$2" "$3" cmd/covercheck >"$4"' sh "$dry" "$actual" "$baseline" "$evidence/validation.txt"
  cat "$evidence/validation.txt"
  printf 'covercheck mutation blocker completed\n' >"$evidence/summary.txt"
}

cmd="${1:-}"
shift || true

case "$cmd" in
  __covercheck-mutants-inner)
    run_covercheck_mutants_inner "$@"
    ;;
  covercheck-mutants)
    run_covercheck_mutants "$@"
    ;;

  gate)
    full=false
    ranges=()
    while [ "$#" -gt 0 ]; do
      case "$1" in
        full) [ "$full" = false ] || { echo "usage: ./x gate [full] [timings] [--range <base> <head>]" >&2; exit 2; }; full=true ;;
        timings) gate_timings=true ;;
        --range) [ "$full" = true ] && [ "$#" -ge 3 ] || { echo "usage: ./x gate [full] [timings] [--range <base> <head>]" >&2; exit 2; }; ranges+=("$2" "$3"); shift 2 ;;
        *) echo "usage: ./x gate [full] [timings] [--range <base> <head>]" >&2; exit 2 ;;
      esac
      shift
    done
    run_gate_step versioncheck go run ./cmd/versioncheck
    run_gate_step build go build ./...
    run_gate_step lint go tool golangci-lint run
    run_gate_step pincheck go run ./cmd/pincheck
    if [ "$full" = true ]; then
      # Full assurance always runs complete native behavioural lanes; it never
      # derives execution from the staged path set.
      prof="coverage.out"
      run_gate_step go-test env -u AWF_PI_RUNTIME_SMOKE go test -p=1 -timeout=20m ./... -coverpkg=./... -coverprofile="$prof"
      run_gate_step covercheck go run ./cmd/covercheck --policy "$prof" coverage-baseline.json
      run_gate_step pi-runtime-smoke run_pi_runtime_smoke
      run_gate_step vet go vet ./...
      run_gate_step advisory-lint run_advisory_lint
      run_gate_step deadcode run_deadcode_gate
      for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
        run_gate_step "build-${target//\//-}" env GOOS="${target%/*}" GOARCH="${target#*/}" go build ./...
      done
      if [ "${#ranges[@]}" -eq 0 ]; then
        if covercheck_mutants_selected staged; then mutation=0; else mutation=$?; fi
      else
        if covercheck_mutants_selected ranges "${ranges[@]}"; then mutation=0; else mutation=$?; fi
      fi
      case "$mutation" in
        0) run_gate_step covercheck-mutation-regression run_covercheck_mutants ;;
        1) echo "gate full: mutation skipped; exact change universe does not own cmd/covercheck" >&2 ;;
        *) echo "gate full: mutation selection uncertain; running blocker conservatively" >&2; run_gate_step covercheck-mutation-regression run_covercheck_mutants ;;
      esac
    fi
    ;;
  lint)
    go tool golangci-lint run "$@"
    run_advisory_lint "$@"
    ;;
  deadcode)
    # Whole-program dead-code gate (ADR-0063): fails on any production func
    # unreachable from a main outside internal/testsupport/. Run without -test.
    go tool deadcode -json ./... | go run ./cmd/deadcodecheck
    ;;
  fmt)
    go tool golangci-lint fmt --config .golangci-advisory.yml "$@"
    ;;
  test)
    echo "test: Pi host lane skipped; run './x pi-test run' for focused verification or './x gate full' for terminal verification" >&2
    env -u AWF_PI_RUNTIME_SMOKE go test ./... "$@"
    ;;
  clean-test-tmp)
    go run ./internal/testsupport/cmd/testtmpclean "$@"
    ;;
  render)
    # The repository runner-body convention part runs awf from source so the
    # dogfooded render always matches the tree.
    ./awf render "$@"
    ;;
  check)
    ./awf check "$@"
    if ! go run ./cmd/contextspilllog --check-log --root "$PWD"; then
      echo "check: warning: context spill advisory inspection failed; resolve or promote the issue before removing the log" >&2
    fi
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
    echo "usage: ./x <gate [full] [timings] [--range <base> <head>]|lint|fmt|test|clean-test-tmp [--all]|deadcode|render|check|context|pi-test <run>|build|install|mutants|covercheck-mutants [--select-staged|--select-range <base> <head>]|audit-local>" >&2
    exit 2
    ;;
esac
