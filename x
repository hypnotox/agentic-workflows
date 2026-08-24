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

select_gate_tests() {
  # One rename-disabled, NUL-delimited index diff is the selection snapshot.
  # Any Git or parsing uncertainty deliberately retains every test lane.
  local LC_ALL=C diff path size consumed=0 saw=false
  diff="$(mktemp)"
  cleanup_paths+=("$diff")
  if ! git diff --cached --name-only -z --no-renames >"$diff"; then
    return 1
  fi
  gate_go_tests=false
  gate_pi_tests=false
  while IFS= read -r -d '' path; do
    if [ -z "$path" ]; then
      return 1
    fi
    saw=true
    consumed=$((consumed + ${#path} + 1))
    # Each recognized category explicitly selects its dependent suites. New or
    # uncertain paths deliberately select both rather than inheriting a lane.
    case "$path" in
      # Exact test-free data and documentation inputs select neither suite.
      docs/*|README.md|changelog/CHANGELOG.md|.awf/docs/parts/*|templates/docs/*|internal/project/VERSION|.awf/awf.lock) ;;
      # Pi templates and generated guidance are consumed by Go tests as well.
      templates/pi/*|templates/embed.go|.pi/agents/*|.pi/skills/*|x|internal/project/*|internal/render/*|internal/config/*|internal/catalog/*|.awf/*|go.mod|go.sum) gate_go_tests=true; gate_pi_tests=true ;;
      # These Pi harness proving inputs have direct Go-test consumers.
      .nvmrc|tools/pi-extension-test/run.sh|tools/pi-extension-test/lockrun/*|tools/pi-extension-test/tests/index.test.ts|tools/pi-extension-test/tests/handoff.test.ts) gate_go_tests=true; gate_pi_tests=true ;;
      # Pi extension and standalone harness inputs have no Go-test consumer.
      .pi/extensions/*|tools/pi-extension-test/*manifest*|tools/pi-extension-test/*lock*|tools/pi-extension-test/tsconfig*.json|tools/pi-extension-test/fixtures/*|tools/pi-extension-test/tests/*.ts|tools/pi-extension-test/package.json|tools/pi-extension-test/package-lock.json) gate_pi_tests=true ;;
      # Ordinary Go and Claude-only inputs do not affect the Pi runtime suite.
      *.go|.claude/*) gate_go_tests=true ;;
      *) gate_go_tests=true; gate_pi_tests=true ;;
    esac
  done <"$diff"
  size="$(wc -c <"$diff")" || return 1
  "$saw" && [ "$consumed" -eq "$size" ] || return 1
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

covercheck_mutants_path_owned() {
  case "$1" in cmd/covercheck|cmd/covercheck/*) return 0;; *) return 1;; esac
}

covercheck_mutants_selected() {
  # The staged and explicit-range callers deliberately share this one
  # rename-disabled NUL stream parser. Any unread byte or Git failure is
  # uncertainty, and therefore selects the blocker.
  local mode="$1" path stream size consumed=0 saw=false
  shift
  stream="$(mktemp)"
  cleanup_paths+=("$stream")
  if [ "$mode" = staged ]; then
    git diff --cached --name-only -z --no-renames >"$stream" || return 2
  else
    [ "$#" -eq 2 ] || return 2
    git cat-file -e "$1^{commit}" && git cat-file -e "$2^{commit}" || return 2
    git diff --name-only -z --no-renames "$1" "$2" >"$stream" || return 2
  fi
  while IFS= read -r -d '' path; do
    [ -n "$path" ] || return 2
    saw=true
    consumed=$((consumed + ${#path} + 1))
    covercheck_mutants_path_owned "$path" && return 0
  done <"$stream"
  size="$(wc -c <"$stream")" || return 2
  [ "$consumed" -eq "$size" ] || return 2
  "$saw" && return 1
  return 1
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
  timeout 900s bash "$0" __covercheck-mutants-inner "$root" "$evidence" "$baseline"
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
  tmp="$(mktemp -d /tmp/covercheck-mutants.XXXXXX)" || return 1
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
  run_mutation_segment "$segments" preflight go test -count=1 ./...
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
    if [ "$#" -eq 1 ] && [ "$1" = timings ]; then
      gate_timings=true
    elif [ "$#" -ne 0 ]; then
      echo "usage: ./x gate [timings]" >&2
      exit 2
    fi
    unset AWF_PI_RUNTIME_SMOKE
    # Sequential gate: profiled tests + raw-identity coverage policy + the
    # explicitly enabled uncached Pi runtime smoke + vet + lint. The policy
    # evaluates one whole-module profile against its canonical baseline;
    # -coverpkg=./... makes every package contribute. Ordinary Go runs skip the host Pi lane;
    # this gate invokes both proving units below while their shared helper runs
    # the host lane exactly once.
    # The profile is durable (gitignored via *.out) so CI can upload it to
    # Codecov without rerunning the suite, and an interrupted run leaks no
    # tmpfs file (ADR-0196).
    # The index determines only which test lanes run; all commands continue to
    # test the working tree. No staged set or any read uncertainty fails closed.
    if ! select_gate_tests; then
      gate_go_tests=true
      gate_pi_tests=true
    fi
    run_gate_step versioncheck go run ./cmd/versioncheck
    prof="coverage.out"
    if "$gate_go_tests"; then
      run_gate_step go-test env -u AWF_PI_RUNTIME_SMOKE go test ./... -coverpkg=./... -coverprofile="$prof"
      run_gate_step covercheck go run ./cmd/covercheck --policy "$prof" coverage-baseline.json
    elif "$gate_pi_tests"; then
      echo "gate: skipping Go tests and coverage for Pi-only staged changes" >&2
    else
      echo "gate: skipping Go tests and coverage for test-free staged changes" >&2
    fi
    if "$gate_pi_tests"; then
      run_gate_step pi-runtime-smoke run_pi_runtime_smoke
    elif "$gate_go_tests"; then
      echo "gate: skipping Pi runtime smoke for Go-only staged changes" >&2
    else
      echo "gate: skipping Pi runtime smoke for test-free staged changes" >&2
    fi
    run_gate_step vet go vet ./...
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
        run_gate_step "build-${target//\//-}" env GOOS="${target%/*}" GOARCH="${target#*/}" go build ./...
      fi
    done
    run_gate_step lint go tool golangci-lint run
    run_gate_step advisory-lint run_advisory_lint
    run_gate_step deadcode run_deadcode_gate
    run_gate_step pincheck go run ./cmd/pincheck
    run_gate_step covercheck-mutation-regression run_covercheck_mutants --select-staged
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
    echo "test: Pi host lane skipped; run './x pi-test run' alone or './x gate' to include it" >&2
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
    echo "usage: ./x <gate [timings]|lint|fmt|test|clean-test-tmp [--all]|deadcode|render|check|context|pi-test <run>|build|install|mutants|covercheck-mutants [--select-staged|--select-range <base> <head>]|audit-local>" >&2
    exit 2
    ;;
esac
