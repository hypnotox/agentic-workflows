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

archive="$candidate_dist/awf_${expected_version}_${expected_os}_${expected_arch}.tar.gz"
[ -f "$archive" ] || { echo "native-release-test: missing $archive" >&2; exit 1; }

root="$(mktemp -d "${TMPDIR:-/tmp}/awf-native-release.XXXXXX")"
trap 'rm -rf "$root"' EXIT HUP INT TERM
mkdir -p "$root/bin" "$root/cache" "$root/home" "$root/repo" "$root/tmp"
export HOME="$root/home"
export XDG_CACHE_HOME="$root/cache"
export TMPDIR="$root/tmp"

tar -xzf "$archive" -C "$root/bin"
candidate="$root/bin/awf"
[ -x "$candidate" ]
[ "$($candidate version)" = "version: $expected_version" ]
"$candidate" --help | grep '^Usage:' >/dev/null

# The release is not published yet. Seed the cache with the exact candidate so
# the generated wrapper exercises its normal pinned-cache path without network.
cache_binary="$XDG_CACHE_HOME/awf/$expected_version/awf"
mkdir -p "$(dirname "$cache_binary")"
cp "$candidate" "$cache_binary"
chmod 0755 "$cache_binary"

cd "$root/repo"
"$candidate" init
[ "$(bash .awf/bootstrap.sh)" = "$cache_binary" ]
./awf check
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

printf 'native-release-test: verified %s/%s candidate version %s\n' "$expected_os" "$expected_arch" "$expected_version"
