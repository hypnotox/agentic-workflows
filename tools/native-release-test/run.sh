#!/usr/bin/env bash
set -euo pipefail

[ "$#" -eq 4 ] || { echo "usage: run.sh <candidate-dist> <goos> <goarch> <version>" >&2; exit 2; }
candidate_dist="$(cd "$1" && pwd -P)"
expected_os="$2"
expected_arch="$3"
expected_version="${4#v}"

case "$expected_os/$expected_arch" in
  linux/amd64|darwin/arm64) ;;
  *) echo "native-release-test: unsupported target $expected_os/$expected_arch" >&2; exit 2 ;;
esac

asset="awf_${expected_version}_${expected_os}_${expected_arch}.tar.gz"
archive="$candidate_dist/$asset"
launcher="$candidate_dist/awf.sh"
[ -f "$archive" ] || { echo "native-release-test: missing $archive" >&2; exit 1; }
[ -f "$candidate_dist/checksums.txt" ] || { echo "native-release-test: missing checksums.txt" >&2; exit 1; }
[ -f "$launcher" ] || { echo "native-release-test: missing awf.sh" >&2; exit 1; }

root="$(mktemp -d "${TMPDIR:-/tmp}/awf-native-release.XXXXXX")"
trap 'rm -rf "$root"' EXIT HUP INT TERM
mkdir -p "$root/bin" "$root/cache" "$root/fake-bin" "$root/home" "$root/malformed/.awf" "$root/repo" "$root/tmp"
export HOME="$root/home"
export XDG_CACHE_HOME="$root/cache"
export TMPDIR="$root/tmp"
export AWF_DOWNLOAD_FIXTURE="$candidate_dist"
export PATH="$root/fake-bin:$PATH"

cat > "$root/fake-bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[ "${AWF_FAKE_OFFLINE:-0}" != 1 ] || { echo "fixture curl: network disabled" >&2; exit 90; }
url=""
output=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) output="$2"; shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
[ -n "$url" ] && [ -n "$output" ] || { echo "fixture curl: unsupported arguments" >&2; exit 2; }
name="${url##*/}"
if [ "$name" = checksums.txt ] && [ -n "${AWF_CHECKSUM_FILE:-}" ]; then
  cp "$AWF_CHECKSUM_FILE" "$output"
else
  cp "$AWF_DOWNLOAD_FIXTURE/$name" "$output"
fi
EOF
chmod 0755 "$root/fake-bin/curl"

tar -xzf "$archive" -C "$root/bin"
candidate="$root/bin/awf"
[ -x "$candidate" ]
[ "$("$candidate" version)" = "version: $expected_version" ]
"$candidate" --help | grep '^Usage:' >/dev/null

# Start with an empty cache and use the public launcher as a first-use docs path.
cd "$root/repo"
bash "$launcher" docs integration > "$root/integration.md"
grep '^# Integrating AWF$' "$root/integration.md" >/dev/null
[ ! -e .awf ]
cache_binary="$XDG_CACHE_HOME/awf/$expected_version/awf"
[ -x "$cache_binary" ]
cmp "$candidate" "$cache_binary"
[ "$("$cache_binary" version)" = "version: $expected_version" ]
[ -z "$(ls -A "$root/tmp")" ]

# Once cached, both public and repository entrypoints must work without downloading.
export AWF_FAKE_OFFLINE=1
bash "$launcher" init
[ "$(bash .awf/bootstrap.sh)" = "$cache_binary" ]
./awf check
if bash "$launcher" docs unknown > "$root/usage.out" 2> "$root/usage.err"; then
  echo "native-release-test: invalid docs page unexpectedly succeeded" >&2
  exit 1
else
  status=$?
fi
[ "$status" -eq 2 ]
[ ! -s "$root/usage.out" ]
grep '^awf:' "$root/usage.err" >/dev/null

# Embedded docs remain available around missing or malformed repository sources.
printf 'malformed source\n' > "$root/malformed/.awf/project.md"
cp "$root/malformed/.awf/project.md" "$root/malformed/before"
(
  cd "$root/malformed"
  bash "$launcher" docs topics | grep '^# Working with topics$' >/dev/null
)
cmp "$root/malformed/before" "$root/malformed/.awf/project.md"
[ ! -e "$root/malformed/AGENTS.md" ]

mkdir -p .awf/topics/code
cat > .awf/topics/global.md <<'EOF'
---
paths: ['**']
---
Global smoke guidance.
EOF
cat > .awf/topics/code/go.md <<'EOF'
---
paths: ['src/**/*.go']
---
Go smoke guidance.
EOF
[ "$("$candidate" resolve)" = $'global\t.awf/topics/global.md' ]
[ "$("$candidate" resolve src/future/main.go)" = $'code/go\t.awf/topics/code/go.md\nglobal\t.awf/topics/global.md' ]
printf '\nNative smoke guidance.\n' >> .awf/project.md
"$candidate" render
"$candidate" check

"$candidate" effort new smoke
"$candidate" effort show smoke | grep '# Effort: smoke' >/dev/null
[ "$("$candidate" plan new smoke)" = "plan: .awf/efforts/smoke/plan.md" ]
[ "$("$candidate" adr new smoke-choice)" = "adr: docs/decisions/smoke-choice.md" ]
"$candidate" check
"$candidate" effort finish smoke
[ -f .awf/effort-archive/smoke/memory.md ]
[ -f .awf/effort-archive/smoke/plan.md ]
[ -f docs/decisions/smoke-choice.md ]
[ ! -e .awf/efforts/smoke ]

# A bad checksum must fail before an executable enters a fresh cache.
unset AWF_FAKE_OFFLINE
bad_checksums="$root/bad-checksums.txt"
printf '%064d  %s\n' 0 "$asset" > "$bad_checksums"
if XDG_CACHE_HOME="$root/bad-cache" AWF_CHECKSUM_FILE="$bad_checksums" bash "$launcher" docs > "$root/bad.out" 2> "$root/bad.err"; then
  echo "native-release-test: checksum mismatch unexpectedly succeeded" >&2
  exit 1
fi
[ ! -e "$root/bad-cache/awf/$expected_version/awf" ]
[ ! -s "$root/bad.out" ]

printf 'native-release-test: verified %s/%s candidate version %s\n' "$expected_os" "$expected_arch" "$expected_version"
