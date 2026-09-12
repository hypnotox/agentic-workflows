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
for guide in overview integration topics effort; do
  [ -f "$root/bin/internal/docs/$guide.md" ] || { echo "native-release-test: missing internal/docs/$guide.md" >&2; exit 1; }
done
candidate="$root/bin/awf"
[ -x "$candidate" ]
[ "$("$candidate" version)" = "version: $expected_version" ]
"$candidate" --help | grep '^Usage:' >/dev/null

# Start with an empty cache and use the public launcher as a first-use docs path.
cd "$root/repo"
AWF_VERSION=0.0.0 bash "$launcher" docs integration > "$root/integration.md"
cmp "$root/bin/internal/docs/integration.md" "$root/integration.md"
bash "$launcher" docs > "$root/overview.md"
cmp "$root/bin/internal/docs/overview.md" "$root/overview.md"
bash "$launcher" docs effort > "$root/effort.md"
cmp "$root/bin/internal/docs/effort.md" "$root/effort.md"
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
override_version=9.8.7
override_binary="$XDG_CACHE_HOME/awf/$override_version/awf"
mkdir -p "$(dirname "$override_binary")"
printf '#!/usr/bin/env bash\nexit 0\n' > "$override_binary"
chmod 0755 "$override_binary"
[ "$(AWF_VERSION="$override_version" bash .awf/bootstrap.sh)" = "$override_binary" ]
printf '#!/usr/bin/env bash\nexit 99\n' > "$root/fake-bin/awf"
chmod 0755 "$root/fake-bin/awf"
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
  bash "$launcher" docs topics > "$root/topics.md"
)
cmp "$root/bin/internal/docs/topics.md" "$root/topics.md"
cmp "$root/malformed/before" "$root/malformed/.awf/project.md"
[ ! -e "$root/malformed/AGENTS.md" ]

mkdir -p docs/topics/code
cat > docs/topics/global.md <<'EOF'
---
paths: ['**']
---
Global smoke guidance.
EOF
cat > docs/topics/code/go.md <<'EOF'
---
paths: ['src/**/*.go']
---
Go smoke guidance.
EOF
[ "$("$candidate" resolve)" = $'global\tdocs/topics/global.md' ]
[ "$("$candidate" resolve src/future/main.go)" = $'code/go\tdocs/topics/code/go.md\nglobal\tdocs/topics/global.md' ]
[ "$("$candidate" resolve --coverage src/future/main.go missing/file)" = $'globals:\n  global\tdocs/topics/global.md\npath: "missing/file"\n  none\npath: "src/future/main.go"\n  code/go\tdocs/topics/code/go.md' ]
printf '\nNative smoke guidance.\n' >> .awf/project.md
"$candidate" render
"$candidate" check

"$candidate" new effort smoke
printf 'opaque effort memory\000\377\n' > .awf/efforts/smoke/memory.md
cp .awf/efforts/smoke/memory.md "$root/expected-memory"
"$candidate" effort show smoke > "$root/effort-show"
[ "$(head -n 1 "$root/effort-show")" = "memory: .awf/efforts/smoke/memory.md" ]
[ -z "$(sed -n '2p' "$root/effort-show")" ]
tail -n +3 "$root/effort-show" > "$root/shown-memory"
cmp "$root/expected-memory" "$root/shown-memory"
[ "$("$candidate" new intent smoke)" = "intent: docs/changes/smoke/intent.md" ]
[ "$("$candidate" new spec smoke)" = "spec: docs/changes/smoke/spec.md" ]
[ "$("$candidate" new plan smoke)" = "plan: docs/plans/smoke.md" ]
[ "$("$candidate" new adr smoke-choice)" = "adr: docs/decisions/smoke-choice.md" ]
grep '^status: pending$' docs/decisions/smoke-choice.md >/dev/null
[ "$("$candidate" new topic generated/smoke 'generated/**')" = "topic: docs/topics/generated/smoke.md" ]
[ "$("$candidate" resolve generated/future.txt)" = $'generated/smoke\tdocs/topics/generated/smoke.md\nglobal\tdocs/topics/global.md' ]
"$candidate" check
"$candidate" effort finish smoke
cmp "$root/expected-memory" .awf/effort-archive/smoke/memory.md
[ -f docs/changes/smoke/intent.md ]
[ -f docs/changes/smoke/spec.md ]
[ -f docs/plans/smoke.md ]
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
