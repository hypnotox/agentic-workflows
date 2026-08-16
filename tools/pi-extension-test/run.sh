#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
tool_dir="$root/tools/pi-extension-test"
pinned_node="$(tr -d '[:space:]' <"$root/.nvmrc")"

# A local NVM installation is authoritative for selecting the repository pin.
# CI deliberately has no NVM: setup-node has already supplied the exact runtime.
nvm_home="${AWF_PI_TEST_NVM_DIR:-${NVM_DIR:-$HOME/.nvm}}"
if [ ! -s "$nvm_home/nvm.sh" ] && [ -s "$HOME/.nvm/nvm.sh" ]; then nvm_home="$HOME/.nvm"; fi
if [ -s "$nvm_home/nvm.sh" ]; then
  export NVM_DIR="$nvm_home"
  # shellcheck source=/dev/null
  . "$NVM_DIR/nvm.sh" --no-use
  if ! nvm use "$pinned_node" >/dev/null 2>&1; then
    echo "pi-extension-test: install the pinned runtime with: nvm install $pinned_node" >&2
    exit 1
  fi
fi
if [ "$(node --version)" != "$pinned_node" ]; then
  echo "pi-extension-test: Node $(node --version) does not match $pinned_node; install it with: nvm install $pinned_node" >&2
  exit 1
fi

lock_dir="$tool_dir/.host-lane.lock"
owner="$lock_dir/pid"
acquire_lock() {
  while ! mkdir "$lock_dir" 2>/dev/null; do
    if [ -r "$owner" ] && ! kill -0 "$(cat "$owner")" 2>/dev/null; then
      stale_lock="$lock_dir.stale.$$"
      if mv "$lock_dir" "$stale_lock" 2>/dev/null; then
        rm -rf "$stale_lock"
      fi
      continue
    fi
    sleep 1
  done
  printf '%s\n' "$$" >"$owner"
}
release_lock() { rm -rf "$lock_dir"; }
acquire_lock
trap release_lock EXIT

fingerprint="$({
  node -e '
const { createHash } = require("node:crypto");
const { readFileSync } = require("node:fs");
const os = require("node:os");
const hash = createHash("sha256");
for (const file of process.argv.slice(1)) hash.update(readFileSync(file));
hash.update(`node=${process.version}\nnpm=${process.env.npm_config_user_agent || ""}\nos=${os.platform()}\narch=${os.arch()}\n`);
process.stdout.write(hash.digest("hex"));
' "$root/.nvmrc" "$tool_dir/package.json" "$tool_dir/package-lock.json"
  npm --version
} | sha256sum | cut -d' ' -f1)"
marker="$tool_dir/.host-deps-${fingerprint}.ok"
required_bins=(c8 tsc tsx)
deps_ready=true
[ -f "$marker" ] || deps_ready=false
for bin in "${required_bins[@]}"; do [ -x "$tool_dir/node_modules/.bin/$bin" ] || deps_ready=false; done
if ! "$deps_ready"; then
  rm -f "$tool_dir"/.host-deps-*.ok
  rm -rf "$tool_dir/node_modules"
  (cd "$tool_dir" && npm ci --ignore-scripts)
  for bin in "${required_bins[@]}"; do
    if [ ! -x "$tool_dir/node_modules/.bin/$bin" ]; then
      echo "pi-extension-test: npm ci did not provide required local binary $bin" >&2
      exit 1
    fi
  done
  tmp_marker="$(mktemp "$tool_dir/.host-deps.XXXXXX")"
  printf '%s\n' "$fingerprint" >"$tmp_marker"
  mv "$tmp_marker" "$marker"
fi

workspace="$(mktemp -d "${TMPDIR:-/tmp}/awf-pi-extension-test.XXXXXX")"
cleanup_workspace() { rm -rf "$workspace"; }
trap 'cleanup_workspace; release_lock' EXIT
copy_workspace() {
  mkdir -p "$workspace/.pi" "$workspace/tools/pi-extension-test"
  cp -a "$root/.pi/extensions" "$workspace/.pi/extensions"
  cp -a "$root/.pi/agents" "$workspace/.pi/agents"
  cp -a "$root/.pi/skills" "$workspace/.pi/skills"
  cp -a "$tool_dir/tests" "$tool_dir/fixtures" "$tool_dir/tsconfig.json" "$tool_dir/package.json" "$tool_dir/package-lock.json" "$workspace/tools/pi-extension-test/"
  printf '%s\n' '{"type":"module"}' >"$workspace/package.json"
  ln -s "$tool_dir/node_modules" "$workspace/node_modules"
}
copy_workspace

strip_ts_nocheck() {
  find .pi/extensions -type f -name '*.ts' -print0 | sort -z | xargs -0 node -e '
const fs = require("node:fs");
for (const path of process.argv.slice(1)) {
  const lines = fs.readFileSync(path, "utf8").split("\n");
  fs.writeFileSync(path, lines.filter((line) => line !== "// @ts-nocheck").join("\n"));
}
'
}
(
  cd "$workspace"
  strip_ts_nocheck
  node_modules/.bin/tsc -p tools/pi-extension-test/tsconfig.json
  node_modules/.bin/c8 --all \
    --include='.pi/extensions/awf-subagents/index.ts' \
    --include='.pi/extensions/awf-subagents/model-routing.ts' \
    --include='.pi/extensions/awf-effort/index.ts' \
    --include='.pi/extensions/awf-effort/client.ts' \
    --exclude='tools/pi-extension-test/tests/*.ts' \
    --check-coverage --lines=100 --functions=100 --branches=100 \
    node --import tsx --test --experimental-test-isolation=none tools/pi-extension-test/tests/*.test.ts
)
