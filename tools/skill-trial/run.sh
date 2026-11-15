#!/usr/bin/env bash
# Run the AWF skill observation trial. See README.md in this directory.
#
# Usage: tools/skill-trial/run.sh [-r <git-ref>] [-l <label>] [<task>...]
#   <task> is a file name under tasks/ without .md (default: all tasks).
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(git -C "$here" rev-parse --show-toplevel)"

ref="HEAD"
label=""
while getopts "r:l:h" opt; do
  case "$opt" in
    r) ref="$OPTARG" ;;
    l) label="$OPTARG" ;;
    h) sed -n '2,5p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) exit 2 ;;
  esac
done
shift $((OPTIND - 1))

model="${TRIAL_MODEL:-claude-sonnet-5-5}"
max_turns="${TRIAL_MAX_TURNS:-80}"
time_cap="${TRIAL_TIMEOUT:-900}"
results_root="${TRIAL_RESULTS:-${XDG_CACHE_HOME:-$HOME/.cache}/awf-skill-trial}"

commit="$(git -C "$repo" rev-parse --verify "${ref}^{commit}")"
[ -n "$label" ] || label="$(printf '%s' "$ref" | tr '/' '-')-${commit:0:9}"

if [ "$#" -eq 0 ]; then
  set -- $(cd "$here/tasks" && ls ./*.md | sed 's|^\./||; s|\.md$||')
fi
for task in "$@"; do
  [ -f "$here/tasks/$task.md" ] || { echo "skill-trial: unknown task: $task" >&2; exit 2; }
done

for tool in claude go git jq timeout; do
  command -v "$tool" >/dev/null || { echo "skill-trial: missing $tool" >&2; exit 1; }
done

out="$results_root/$label"
mkdir -p "$out/bin"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/awf-skill-trial.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# Build awf from the ref without touching the checkout's working tree.
echo "skill-trial: building awf from $ref ($commit)" >&2
mkdir "$scratch/src"
git -C "$repo" archive "$commit" | tar -x -C "$scratch/src"
(cd "$scratch/src" && go build -o "$out/bin/awf" ./cmd/awf)
awf="$out/bin/awf"

# Prepare the fixture template once; each task gets a fresh copy.
template="$scratch/template/tally"
mkdir -p "$template"
(cd "$here/fixture" && find . -type f -name '*.in') | while read -r src; do
  dst="$template/${src%.in}"
  mkdir -p "$(dirname "$dst")"
  cp "$here/fixture/$src" "$dst"
done
# The generated ./awf wrapper runs .awf/bootstrap.sh, which uses a cached binary
# at $XDG_CACHE_HOME/awf/<version>/awf before downloading anything. Seed a
# private cache with the build so ./awf runs it offline and the generated files
# stay unmodified (awf check stays green). Go keeps the user's own build cache.
version="$("$awf" version | sed -n 's/^version: //p')"
cache="$scratch/xdg-cache"
mkdir -p "$cache/awf/$version"
cp "$awf" "$cache/awf/$version/awf"
export GOCACHE="$(go env GOCACHE)"
export XDG_CACHE_HOME="$cache"

(
  cd "$template"
  git init -q -b main
  git config user.name "Trial User"
  git config user.email "trial@example.invalid"
  "$awf" init >/dev/null
  [ "$(bash .awf/bootstrap.sh)" = "$cache/awf/$version/awf" ] || { echo "skill-trial: bootstrap does not use the seeded binary" >&2; exit 1; }
  go test ./... >/dev/null
  git add -A
  git commit -q -m "chore: initial tally project"
  ./awf check >/dev/null || { ./awf check >&2; exit 1; }
)

{
  echo "ref: $ref"
  echo "commit: $commit"
  echo "awf: $("$awf" version | tr '\n' ' ')"
  echo "claude: $(claude --version)"
  echo "model: $model"
  echo "max_turns: $max_turns"
  echo "timeout_s: $time_cap"
  echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "plugins:"
  claude plugin list 2>/dev/null | sed 's/^/  /' || true
} > "$out/run-info.txt"

allowed=(
  Read Edit Write Glob Grep Skill Agent Task TodoWrite
  "Bash(git:*)" "Bash(go:*)" "Bash(gofmt:*)" "Bash(./awf:*)"
  "Bash(ls:*)" "Bash(cat:*)" "Bash(head:*)" "Bash(tail:*)" "Bash(wc:*)"
  "Bash(grep:*)" "Bash(find:*)" "Bash(diff:*)" "Bash(printf:*)" "Bash(echo:*)"
  "Bash(mkdir:*)" "Bash(mv:*)" "Bash(cp:*)" "Bash(pwd)" "Bash(cd:*)"
)

for task in "$@"; do
  dest="$out/$task"
  rm -rf "$dest"
  mkdir -p "$dest"
  work="$scratch/runs/$task"
  mkdir -p "$work"
  cp -a "$template" "$work/tally"
  cp "$here/tasks/$task.md" "$dest/prompt.md"
  echo "skill-trial: running $task (cap ${time_cap}s, ${max_turns} turns)" >&2

  status=0
  (
    cd "$work/tally"
    timeout --kill-after=30 "$time_cap" claude -p "$(cat "$here/tasks/$task.md")" \
      --model "$model" \
      --max-turns "$max_turns" \
      --output-format stream-json --verbose \
      --permission-mode acceptEdits \
      --permission-prompts none \
      --allowed-tools "${allowed[@]}" \
      --add-dir "$work" \
      --strict-mcp-config \
      --no-session-persistence \
      < /dev/null
  ) > "$dest/transcript.jsonl" 2> "$dest/stderr.log" || status=$?
  echo "$status" > "$dest/exit-status"
  [ "$status" -eq 124 ] && echo "skill-trial: $task hit the time cap" >&2

  (
    cd "$work/tally"
    {
      echo "## git log --all --graph"
      git log --all --graph --oneline --decorate
      echo
      echo "## git status --short"
      git status --short --untracked-files=all
      echo
      echo "## git worktree list"
      git worktree list
      echo
      echo "## diff stat (initial commit .. main)"
      git diff --stat "$(git rev-list --max-parents=0 HEAD)" main
      echo
      echo "## diff stat (working tree vs HEAD)"
      git diff --stat HEAD
    } > "$dest/git-state.txt" 2>&1
    git diff "$(git rev-list --max-parents=0 HEAD)" > "$dest/diff.patch" 2>&1 || true
  )
  tar -C "$work" -czf "$dest/final-fixture.tar.gz" .
  "$here/summarize.sh" "$dest" > "$dest/summary.md" || true
  rm -rf "$work"
done

echo "skill-trial: results in $out" >&2
