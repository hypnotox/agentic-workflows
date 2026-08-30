#!/usr/bin/env bash
set -euo pipefail

root="${AWF_PI_TEST_ROOT:-$(git rev-parse --show-toplevel)}"
tool_dir="$root/tools/pi-extension-test"
pinned_node="$(tr -d '[:space:]' <"$root/.nvmrc")"

# The helper holds a kernel advisory lock for the complete worker. Its inherited
# descriptor remains locked in any surviving descendant, so no user-space owner
# record, stale deletion, or nested acquisition is needed.
if [ "${AWF_PI_TEST_WORKER:-0}" != "1" ]; then
  exec go run "$tool_dir/lockrun" "$tool_dir/.host-lane.lock" env AWF_PI_TEST_WORKER=1 bash "$0"
fi

# Local runs select the repository pin through NVM when it is present. CI sets
# AWF_PI_TEST_SKIP_NVM=1 after setup-node has installed that exact pin; bypassing
# selection never bypasses the exact node --version check below.
if [ "${AWF_PI_TEST_SKIP_NVM:-0}" != "1" ]; then
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
fi
if [ "$(node --version)" != "$pinned_node" ]; then
  echo "pi-extension-test: Node $(node --version) does not match $pinned_node; install it with: nvm install $pinned_node" >&2
  exit 1
fi


workspace=""
tmp_marker=""
cleanup_worker() {
  [ -z "$workspace" ] || rm -rf "$workspace"
  [ -z "$tmp_marker" ] || rm -f "$tmp_marker"
}
trap cleanup_worker EXIT
trap 'cleanup_worker; exit 1' INT TERM HUP

fingerprint="$(node - "$root/.nvmrc" "$tool_dir/package.json" "$tool_dir/package-lock.json" <<'NODE'
const { createHash } = require("node:crypto");
const { readFileSync } = require("node:fs");
const { spawnSync } = require("node:child_process");
const os = require("node:os");
const hash = createHash("sha256");
const field = (name, value) => {
  const bytes = Buffer.isBuffer(value) ? value : Buffer.from(value);
  hash.update(Buffer.from(`${name}:${bytes.length}:`));
  hash.update(bytes);
  hash.update(Buffer.from("\\n"));
};
for (const path of process.argv.slice(2)) field(`file:${path.split("/").pop()}`, readFileSync(path));
const npm = spawnSync("npm", ["--version"], { encoding: "utf8" });
if (npm.status !== 0) process.exit(npm.status || 1);
field("node", process.version);
field("npm", npm.stdout.trim());
field("os", os.platform());
field("arch", os.arch());
process.stdout.write(hash.digest("hex"));
NODE
)"
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
  tmp_marker=""
fi

workspace="$(mktemp -d "${TMPDIR:-/tmp}/awf-pi-extension-test.XXXXXX")"
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
  node - <<'NODE'
const fs = require("node:fs");
const path = require("node:path");
const root = ".pi/extensions";
const files = [];
const visit = (dir) => {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
    const target = path.join(dir, entry.name);
    if (entry.isDirectory()) visit(target);
    else if (entry.isFile() && target.endsWith(".ts")) files.push(target);
  }
};
visit(root);
for (const target of files.sort()) {
  const lines = fs.readFileSync(target, "utf8").split("\n");
  fs.writeFileSync(target, lines.filter((line) => line !== "// @ts-nocheck").join("\n"));
}
NODE
}
(
  cd "$workspace"
  strip_ts_nocheck
  node_modules/.bin/tsc -p tools/pi-extension-test/tsconfig.json
  node_modules/.bin/c8 --all \
    --include='.pi/extensions/awf-subagents/index.ts' \
    --include='.pi/extensions/awf-subagents/model-routing.ts' \
    --exclude='tools/pi-extension-test/tests/*.ts' \
    node --import tsx --test --experimental-test-isolation=none tools/pi-extension-test/tests/*.test.ts
)
