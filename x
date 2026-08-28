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

go_shard_0='changelog cmd/awf cmd/deadcodecheck cmd/repoaudit internal/audit internal/checkresult internal/commitpolicy internal/configspec internal/contextq internal/currentstatecoord internal/frontmatter internal/initop internal/memorycite internal/pitfall internal/presentation internal/prosegate internal/render internal/snapshot internal/testsupport/fsfixture internal/upgrade tools/pi-extension-test/lockrun'
go_shard_1='cmd/mutants cmd/testperformance internal/catalog internal/clispec internal/config internal/contextdelivery internal/contextspill internal/domainop internal/execution internal/generatedcheck internal/initspec internal/migrate internal/pitfallcheck internal/project internal/repositorycheck internal/testperformance internal/testsupport/gitfixture internal/vocabularycheck'
go_shard_2='cmd/contextspilllog cmd/pincheck cmd/versioncheck internal/changelog internal/commitgateop internal/configcheck internal/contextinput internal/coverage internal/effort internal/filepublication internal/git internal/localdocop internal/outputplan internal/plan internal/projectlicense internal/publisher internal/referencecheck internal/resident internal/testsupport internal/topic internal/worktree'
go_shard_3='cmd/covercheck cmd/releasecheck cmd/testselection internal/adr internal/checkop internal/commitmsg internal/configop internal/contextop internal/currentstate internal/effortop internal/evals internal/filesystem internal/glossary internal/manifest internal/pathglob internal/plancheck internal/projectstate internal/refs internal/severity internal/testselection internal/testsupport/cmd/testtmpclean internal/topicop templates'

go_shard_index() {
  local package="$1"
  case " $go_shard_0 " in *" $package "*) printf '0\n'; return;; esac
  case " $go_shard_1 " in *" $package "*) printf '1\n'; return;; esac
  case " $go_shard_2 " in *" $package "*) printf '2\n'; return;; esac
  case " $go_shard_3 " in *" $package "*) printf '3\n'; return;; esac
  echo "gate: Go package has no qualified shard: $package" >&2
  return 1
}

run_go_shards() {
  local profile_dir="$1" requested_workload="${2:-}" requested_profile="${3:-}" prefix='github.com/hypnotox/agentic-workflows/' import_path package index group slice slices name bucket ordinal job job_status gmp wave
  local shared_gocache shared_modcache status=0
  local -a caches=() shard0=() shard1=() shard2=() shard3=() packages=() coverargs=()
  local -a job_groups=() job_slices=() job_regexes=() job_names=() job_tmps=() pids=() logs=() durations=() statuses=()
  while IFS= read -r cache; do
    caches+=("$cache")
  done < <(go env GOCACHE GOMODCACHE)
  [ "${#caches[@]}" -eq 2 ] || { echo "gate: Go cache locations unavailable" >&2; return 1; }
  shared_gocache="${caches[0]}"; shared_modcache="${caches[1]}"
  while IFS= read -r import_path; do
    case "$import_path" in "$prefix"*) package="${import_path#"$prefix"}";; *) echo "gate: package outside module: $import_path" >&2; return 1;; esac
    index="$(go_shard_index "$package")" || return
    case "$index" in
      0) shard0+=("./$package");;
      1) shard1+=("./$package");;
      2) shard2+=("./$package");;
      3) shard3+=("./$package");;
      *) echo "gate: invalid Go shard index for $package" >&2; return 1;;
    esac
  done < <(go list -f '{{.ImportPath}}' ./... | LC_ALL=C sort)
  [ "${#shard0[@]}" -gt 0 ] && [ "${#shard1[@]}" -gt 0 ] && [ "${#shard2[@]}" -gt 0 ] && [ "${#shard3[@]}" -gt 0 ] || {
    echo "gate: every qualified Go shard must contain at least one package" >&2
    return 1
  }

  # The three measured dominant package groups are divided by complete top-level
  # proving-unit names. A name occurs in exactly one slice; subtests remain with
  # their owning top-level test. The fourth group is already below the slice cost.
  for group in 0 1 2; do
    case "$group" in
      0) packages=("${shard0[@]}"); slices=4;;
      1) packages=("${shard1[@]}"); slices=4;;
      2) packages=("${shard2[@]}"); slices=3;;
    esac
    if ! go test -list '^(Test|Example|Fuzz)' "${packages[@]}" | grep -E '^(Test|Example|Fuzz)' | LC_ALL=C sort -u >"$profile_dir/group$group.names"; then
      echo "gate: cannot enumerate proving units for Go shard group $group" >&2
      return 1
    fi
    [ -s "$profile_dir/group$group.names" ] || { echo "gate: Go shard group $group has no proving units" >&2; return 1; }
    for ((slice=0; slice<slices; slice++)); do : >"$profile_dir/group$group-$slice.names"; done
    ordinal=0
    while IFS= read -r name; do
      bucket="$((ordinal % slices))"
      printf '%s\n' "$name" >>"$profile_dir/group$group-$bucket.names"
      ordinal="$((ordinal + 1))"
    done <"$profile_dir/group$group.names"
    for ((slice=0; slice<slices; slice++)); do
      [ -s "$profile_dir/group$group-$slice.names" ] || { echo "gate: Go shard $group-$slice has no proving units" >&2; return 1; }
      job_groups+=("$group")
      job_slices+=("$slice")
      job_regexes+=("^($(paste -sd'|' "$profile_dir/group$group-$slice.names"))$")
      job_names+=("shard$group-$slice")
    done
  done
  job_groups+=("3"); job_slices+=("0"); job_regexes+=(""); job_names+=("shard3-0")

  # The already-small fourth group runs in a second wave. Giving its eval-heavy
  # proving units two processors without concurrent shard contention keeps their
  # qualified component below the landed serial maximum while retaining headroom.
  for wave in 0 1; do
    for job in "${!job_names[@]}"; do
      group="${job_groups[job]}"; slice="${job_slices[job]}"
      if { [ "$wave" -eq 0 ] && [ "$group" -eq 3 ]; } || { [ "$wave" -eq 1 ] && [ "$group" -ne 3 ]; }; then
        continue
      fi
      case "$group" in
        0) packages=("${shard0[@]}");;
        1) packages=("${shard1[@]}");;
        2) packages=("${shard2[@]}");;
        3) packages=("${shard3[@]}");;
      esac
      gmp=1
      [ "$group" -ne 3 ] || gmp=2
      [ -z "$requested_workload" ] || [ "${job_groups[job]}-${job_slices[job]}" = "$requested_workload" ] || continue
      job_tmps[job]="$(mktemp -d "/tmp/j${job}XXX")"
      job_tmps[job]="$(cd "${job_tmps[job]}" && pwd -P)"
      cleanup_paths+=("${job_tmps[job]}")
      logs[job]="$profile_dir/${job_names[job]}.log"
      durations[job]="$profile_dir/${job_names[job]}.duration"
      (
        shard_started=$SECONDS
        args=()
        coverargs=()
        [ -z "${job_regexes[job]}" ] || args=(-run "${job_regexes[job]}")
        if [ -n "$requested_workload" ]; then
          [ -z "$requested_profile" ] || coverargs=(-coverpkg=./... -coverprofile="$requested_profile")
        else
          coverargs=(-coverpkg=./... -coverprofile="$profile_dir/${job_names[job]}.out")
        fi
        if env -u AWF_PI_RUNTIME_SMOKE HOME="${job_tmps[job]}" TMPDIR="${job_tmps[job]}" GOTMPDIR="${job_tmps[job]}" \
          GOCACHE="$shared_gocache" GOMODCACHE="$shared_modcache" GOMAXPROCS="$gmp" \
          go test -p=1 -timeout=20m -count=1 "${args[@]+"${args[@]}"}" "${packages[@]}" "${coverargs[@]+"${coverargs[@]}"}"; then
          job_status=0
        else
          job_status=$?
        fi
        printf '%s\n' "$((SECONDS - shard_started))" >"${durations[job]}"
        exit "$job_status"
      ) >"${logs[job]}" 2>&1 &
      pids[job]=$!
    done
    for job in "${!job_names[@]}"; do
      group="${job_groups[job]}"
      if { [ "$wave" -eq 0 ] && [ "$group" -eq 3 ]; } || { [ "$wave" -eq 1 ] && [ "$group" -ne 3 ]; }; then
        continue
      fi
      [ -n "${pids[job]:-}" ] || continue
      if wait "${pids[job]}"; then job_status=0; else job_status=$?; fi
      statuses[job]="$job_status"
    done
  done
  for job in "${!job_names[@]}"; do
    [ -n "${pids[job]:-}" ] || continue
    cat "${logs[job]}"
    if "$gate_timings"; then printf 'gate shard timing: %s %ss\n' "${job_names[job]}" "$(cat "${durations[job]}")" >&2; fi
    if [ "${statuses[job]}" -ne 0 ]; then
      echo "gate: Go shard ${job_names[job]#shard} failed with status ${statuses[job]}" >&2
      [ "$status" -ne 0 ] || status="${statuses[job]}"
    fi
  done
  return "$status"
}

run_native_shard() {
  local workload= profile= workdir
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --workload) [ "$#" -ge 2 ] || return 2; workload="$2"; shift ;;
      --coverprofile) [ "$#" -ge 2 ] || return 2; profile="$2"; shift ;;
      *) echo "usage: ./x native-shard --workload <0-0|...|3-0> [--coverprofile <path>]" >&2; return 2 ;;
    esac
    shift
  done
  case "$workload" in 0-[0-3]|1-[0-3]|2-[0-2]|3-0) ;; *) echo "native-shard: invalid workload $workload" >&2; return 2;; esac
  workdir="$(mktemp -d /tmp/nXXXXXX)"; cleanup_paths+=("$workdir")
  run_go_shards "$workdir" "$workload" "$profile"
}

write_coverage_manifest() {
  local workload="$1" directory="$2" candidate="$3" profile toolchain digest
  profile="$directory/profile-$workload.out"
  [ "$(git rev-parse HEAD)" = "$candidate" ] || { echo "coverage-producer: checkout SHA does not match candidate" >&2; return 1; }
  [ -f "$profile" ] || { echo "coverage-producer: profile missing" >&2; return 1; }
  toolchain="$(go version)"; digest="$(sha256sum "$profile" | awk '{print $1}')"
  printf 'schema=1\nsha=%s\nworkload=%s\nos=%s\narch=%s\ntoolchain=%s\nprofile=%s\ndigest=%s\n' "$candidate" "$workload" "$(go env GOOS)" "$(go env GOARCH)" "$toolchain" "$(basename "$profile")" "$digest" >"$directory/manifest-$workload"
}

run_coverage_aggregate() {
  local root="$1" candidate workload manifest profile value key matched duplicate field_count
  local schema sha os arch toolchain digest profile_name
  local schema_set sha_set workload_set os_set arch_set toolchain_set profile_set digest_set
  local -a manifests=() profiles=() validated=() artifact_files=() seen_workloads=()
  candidate="$(git rev-parse HEAD)" || return
  while IFS= read -r value; do artifact_files+=("$value"); done < <(find "$root" -type f -print | LC_ALL=C sort)
  [ "${#artifact_files[@]}" -eq 24 ] || { echo "coverage-aggregate: unexpected artifact files" >&2; return 1; }
  [ -z "$(find "$root" -type l -print -quit)" ] || { echo "coverage-aggregate: symbolic-link evidence is forbidden" >&2; return 1; }
  while IFS= read -r value; do manifests+=("$value"); done < <(find "$root" -type f -name 'manifest-*' -print | LC_ALL=C sort)
  [ "${#manifests[@]}" -eq 12 ] || { echo "coverage-aggregate: expected exactly 12 manifests" >&2; return 1; }
  while IFS= read -r value; do profiles+=("$value"); done < <(find "$root" -type f -name 'profile-*.out' -print | LC_ALL=C sort)
  [ "${#profiles[@]}" -eq 12 ] || { echo "coverage-aggregate: expected exactly 12 profiles" >&2; return 1; }
  for manifest in "${manifests[@]}"; do
    schema= sha= workload= os= arch= toolchain= profile_name= digest= field_count=0
    schema_set=false; sha_set=false; workload_set=false; os_set=false; arch_set=false; toolchain_set=false; profile_set=false; digest_set=false
    while IFS='=' read -r key value; do
      [ -n "$key" ] || { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }
      case "$key" in
        schema) "$schema_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; schema="$value"; schema_set=true ;;
        sha) "$sha_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; sha="$value"; sha_set=true ;;
        workload) "$workload_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; workload="$value"; workload_set=true ;;
        os) "$os_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; os="$value"; os_set=true ;;
        arch) "$arch_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; arch="$value"; arch_set=true ;;
        toolchain) "$toolchain_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; toolchain="$value"; toolchain_set=true ;;
        profile) "$profile_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; profile_name="$value"; profile_set=true ;;
        digest) "$digest_set" && { echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1; }; digest="$value"; digest_set=true ;;
        *) echo "coverage-aggregate: malformed manifest $manifest" >&2; return 1 ;;
      esac
      field_count=$((field_count + 1))
    done <"$manifest"
    [ "$field_count" -eq 8 ] || { echo "coverage-aggregate: unexpected manifest evidence" >&2; return 1; }
    for key in schema sha workload os arch toolchain profile digest; do
      case "$key" in
        schema) value="$schema" ;; sha) value="$sha" ;; workload) value="$workload" ;; os) value="$os" ;;
        arch) value="$arch" ;; toolchain) value="$toolchain" ;; profile) value="$profile_name" ;; digest) value="$digest" ;;
      esac
      [ -n "$value" ] || { echo "coverage-aggregate: missing $key evidence" >&2; return 1; }
    done
    case "$workload" in 0-[0-3]|1-[0-3]|2-[0-2]|3-0) ;; *) echo "coverage-aggregate: foreign workload" >&2; return 1;; esac
    [ "$schema" = 1 ] && [ "$sha" = "$candidate" ] && [ "$os" = linux ] && [ "$arch" = amd64 ] && [ "$toolchain" = "$(go version)" ] || { echo "coverage-aggregate: foreign SHA, platform, or toolchain evidence" >&2; return 1; }
    duplicate=false
    for value in "${seen_workloads[@]+"${seen_workloads[@]}"}"; do [ "$value" != "$workload" ] || duplicate=true; done
    "$duplicate" && { echo "coverage-aggregate: duplicate workload $workload" >&2; return 1; }
    seen_workloads+=("$workload")
    profile="$(dirname "$manifest")/$profile_name"; [ "$(basename "$manifest")" = "manifest-$workload" ] && [ "$profile_name" = "profile-$workload.out" ] && [ -f "$profile" ] && [ "$(sha256sum "$profile" | awk '{print $1}')" = "$digest" ] || { echo "coverage-aggregate: invalid profile evidence for $workload" >&2; return 1; }
    validated+=("$profile")
  done
  for workload in 0-0 0-1 0-2 0-3 1-0 1-1 1-2 1-3 2-0 2-1 2-2 3-0; do
    matched=false
    for value in "${seen_workloads[@]}"; do [ "$value" != "$workload" ] || matched=true; done
    "$matched" || { echo "coverage-aggregate: missing workload $workload" >&2; return 1; }
  done
  [ "${#validated[@]}" -eq "${#profiles[@]}" ] || { echo "coverage-aggregate: extra profile evidence" >&2; return 1; }
  for profile in "${profiles[@]}"; do
    matched=false
    for validated_profile in "${validated[@]}"; do [ "$profile" != "$validated_profile" ] || { matched=true; break; }; done
    "$matched" || { echo "coverage-aggregate: unreferenced profile evidence" >&2; return 1; }
  done
  go run ./cmd/covercheck --merge "${validated[@]}" > coverage.out
  go run ./cmd/covercheck --policy coverage.out coverage-baseline.json
  go run ./cmd/covercheck --emit-filtered coverage.out > coverage.covered.out
}

run_platform_builds() {
  local target target_status status=0
  for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
    if env GOOS="${target%/*}" GOARCH="${target#*/}" go build ./...; then
      target_status=0
    else
      target_status=$?
    fi
    [ "$status" -ne 0 ] || status="$target_status"
  done
  return "$status"
}

run_parallel_gate_steps() {
  local workspace="$1" index stage_status status=0
  shift
  local -a labels=() commands=() pids=() logs=() durations=() statuses=()
  while [ "$#" -gt 0 ]; do
    [ "$#" -ge 2 ] || { echo "gate: incomplete parallel stage declaration" >&2; return 2; }
    labels+=("$1"); commands+=("$2"); shift 2
  done
  for index in "${!labels[@]}"; do
    logs[index]="$workspace/parallel-$index.log"
    durations[index]="$workspace/parallel-$index.duration"
    (
      stage_started=$SECONDS
      if eval "${commands[index]}"; then stage_status=0; else stage_status=$?; fi
      printf '%s\n' "$((SECONDS - stage_started))" >"${durations[index]}"
      exit "$stage_status"
    ) >"${logs[index]}" 2>&1 &
    pids[index]=$!
  done
  for index in "${!labels[@]}"; do
    if wait "${pids[index]}"; then stage_status=0; else stage_status=$?; fi
    statuses[index]="$stage_status"
  done
  for index in "${!labels[@]}"; do
    cat "${logs[index]}"
    if "$gate_timings"; then printf 'gate timing: %s %ss\n' "${labels[index]}" "$(cat "${durations[index]}")" >&2; fi
    if [ "${statuses[index]}" -ne 0 ]; then
      echo "gate: stage ${labels[index]} failed with status ${statuses[index]}" >&2
      [ "$status" -ne 0 ] || status="${statuses[index]}"
    fi
  done
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
  if output="$(env AWF_PI_RUNTIME_SMOKE=1 go test -json ./internal/publisher -run '^TestPiRealRuntimeSmoke$' -count=1)"; then
    status=0
  else
    status=$?
  fi
  printf '%s\n' "$output"
  if [ "$status" -ne 0 ]; then
    return "$status"
  fi
  for proving_unit in TestPiRealRuntimeSmoke; do
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
    range) if covercheck_mutants_selected ranges "$base" "$head"; then status=0; else status=$?; fi ;;
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
  while IFS= read -r value; do census+=("$value"); done < <(find "$root/cmd/covercheck" -maxdepth 1 -type f -name '*_test.go' -exec basename {} \; | LC_ALL=C sort)
  while IFS= read -r value; do expected+=("$value"); done < <(go list -f '{{join .TestGoFiles "\n"}}{{"\n"}}{{join .XTestGoFiles "\n"}}' ./cmd/covercheck | sed '/^$/d' | LC_ALL=C sort)
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

  native-shard)
    run_native_shard "$@"
    ;;
  coverage-produce)
    [ "$#" -eq 3 ] || { echo "usage: ./x coverage-produce <workload> <directory> <candidate-sha>" >&2; exit 2; }
    mkdir -p -- "$2"
    run_native_shard --workload "$1" --coverprofile "$2/profile-$1.out"
    write_coverage_manifest "$1" "$2" "$3"
    ;;
  coverage-aggregate)
    [ "$#" -eq 1 ] || { echo "usage: ./x coverage-aggregate <artifact-directory>" >&2; exit 2; }
    run_coverage_aggregate "$1"
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
      profile_dir="$(mktemp -d /tmp/gXXXXXX)"
      cleanup_paths+=("$profile_dir")
      rm -f -- "$prof"
      run_gate_step go-test run_go_shards "$profile_dir"
      run_gate_step coverage-merge sh -c 'dir="$1"; shift; go run ./cmd/covercheck --merge "$@" >"$dir/merged.out"' sh "$profile_dir" "$profile_dir"/shard*.out
      mv "$profile_dir/merged.out" "$prof"
      run_gate_step covercheck go run ./cmd/covercheck --policy "$prof" coverage-baseline.json
      # Pi owns the measured component ceiling and runs without analysis-stage
      # contention. Capture its status, then still terminate every analysis stage.
      if run_gate_step pi-runtime-smoke run_pi_runtime_smoke; then pi_status=0; else pi_status=$?; fi
      if run_parallel_gate_steps "$profile_dir" \
        vet 'go vet ./...' \
        advisory-lint 'run_advisory_lint' \
        deadcode 'run_deadcode_gate' \
        platform-builds 'run_platform_builds'; then
        parallel_status=0
      else
        parallel_status=$?
      fi
      [ "$pi_status" -eq 0 ] || exit "$pi_status"
      [ "$parallel_status" -eq 0 ] || exit "$parallel_status"
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
  test-affected)
    go run ./cmd/testselection --execute "$@"
    ;;
  test-performance)
    go run ./cmd/testperformance "$@"
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
    echo "usage: ./x <gate [full] [timings] [--range <base> <head>]|lint|fmt|test|test-affected [--staged|--range <base>..<head>]|test-performance <validate|report> [--machine] [record]|clean-test-tmp [--all]|deadcode|render|check|context|pi-test <run>|build|install|mutants|covercheck-mutants [--select-staged|--select-range <base> <head>]|audit-local>" >&2
    exit 2
    ;;
esac
