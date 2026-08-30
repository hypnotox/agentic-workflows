#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: run.sh <candidate-dist> <goos> <goarch> <version>" >&2
  exit 2
}

[ "$#" -eq 4 ] || usage
candidate_dist="$1"
expected_os="$2"
expected_arch="$3"
expected_version="${4#v}"

case "$expected_os/$expected_arch" in
  linux/amd64|darwin/arm64) ;;
  *) echo "native-release-test: unsupported target $expected_os/$expected_arch" >&2; exit 2 ;;
esac

candidate_dist="$(cd "$candidate_dist" && pwd -P)"
archive="$candidate_dist/awf_${expected_version}_${expected_os}_${expected_arch}.tar.gz"
[ -f "$archive" ] || {
  echo "native-release-test: missing candidate archive $archive" >&2
  exit 1
}

root="$(mktemp -d "${TMPDIR:-/tmp}/awf-native-release.XXXXXX")"
cleanup() { rm -rf "$root"; }
trap cleanup EXIT HUP INT TERM

mkdir -p "$root/home" "$root/cache" "$root/tmp" "$root/bin" "$root/repo"
export HOME="$root/home"
export XDG_CACHE_HOME="$root/cache"
export TMPDIR="$root/tmp"
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
export GIT_CONFIG_NOSYSTEM=1

tar -xzf "$archive" -C "$root/bin"
awf="$root/bin/awf"
[ -x "$awf" ] || {
  echo "native-release-test: extracted awf is not executable" >&2
  exit 1
}

version_output="$($awf version)"
version_count="$(printf '%s\n' "$version_output" | awk '/^version: / { count++ } END { print count + 0 }')"
actual_version="$(printf '%s\n' "$version_output" | awk '
  /^version: [^[:space:]()]+( \([^[:cntrl:]]+\))?$/ {
    value = substr($0, 10)
    sub(/ \(.*/, "", value)
    print value
  }
')"
[ "$version_count" -eq 1 ] && [ "$actual_version" = "$expected_version" ] || {
  echo "native-release-test: shipped version is '$version_output', want $expected_version" >&2
  exit 1
}
help_output="$("$awf" --help)"
printf '%s\n' "$help_output" | grep '^[[:space:]]*usage:' >/dev/null

# Restrict bootstrap lookup to the extracted candidate ahead of system tools.
export PATH="$root/bin:/usr/local/bin:/usr/bin:/bin"
cd "$root/repo"
git init -b main >/dev/null
"$awf" init --set gateCmd=true
resolved="$(AWF_VERSION="$expected_version" bash .awf/bootstrap.sh)"
[ "$resolved" = "$awf" ] || {
  echo "native-release-test: bootstrap resolved $resolved, want $awf" >&2
  exit 1
}
"$resolved" render
git add .
"$resolved" check

git -c user.name='awf native smoke' \
  -c user.email='awf-native-smoke@example.invalid' \
  -c commit.gpgSign=false commit --no-verify -m 'test: initialize native smoke repository' >/dev/null

"$resolved" effort new --slug smoke-cycle 'Native release smoke lifecycle'
worktree="$root/repo/.awf/worktrees/smoke-cycle"
[ -d "$worktree" ]
(
  cd "$worktree"
  printf 'native release smoke\n' > native-smoke.txt
  git add native-smoke.txt
  git -c user.name='awf native smoke' \
    -c user.email='awf-native-smoke@example.invalid' \
    -c commit.gpgSign=false commit --no-verify -m 'test: exercise native effort integration' >/dev/null
)
"$resolved" effort show smoke-cycle >/dev/null
"$resolved" effort integrate smoke-cycle
"$resolved" effort worktree remove smoke-cycle
"$resolved" effort finish smoke-cycle

[ ! -e "$worktree" ]
[ ! -e "$root/repo/.awf/efforts/smoke-cycle" ]
archive_count=0
for archived in "$root/repo/.awf/effort-archive/"*-smoke-cycle; do
  [ -d "$archived" ] || continue
  archive_count=$((archive_count + 1))
done
[ "$archive_count" -eq 1 ]
printf 'native-release-test: verified %s/%s candidate version %s\n' "$expected_os" "$expected_arch" "$expected_version"
