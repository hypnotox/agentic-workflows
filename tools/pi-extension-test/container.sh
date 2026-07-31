#!/usr/bin/env bash
set -euo pipefail

command_name="${1:-run}"
docker_cmd="${AWF_PI_TEST_DOCKER:-docker}"
root="$(git rev-parse --show-toplevel)"
tool_dir="$root/tools/pi-extension-test"

if ! "$docker_cmd" info >/dev/null 2>&1; then
  echo "pi-extension-test: Docker is required by ./x gate" >&2
  exit 1
fi

# Hash the file CONTENTS rather than sha256sum's output. sha256sum prints each
# file's path beside its digest, so hashing that output made the fingerprint
# vary with the absolute path of the checkout: no two paths ever shared an
# image, and every worktree ran its own full npm ci build (ADR-0195).
hash_files() {
  cat "$tool_dir/Dockerfile" "$tool_dir/package.json" "$tool_dir/package-lock.json" |
    sha256sum | cut -d' ' -f1
}

image_repo="awf-pi-extension-test"
dep_hash="$(hash_files)"
image="$image_repo:${dep_hash:0:12}"

# The superseded design keyed a long-lived container, a dependency volume, and
# an image on the repository path, labelling each with this key. Nothing creates
# them any more; reset sweeps up what they left behind.
legacy_label="dev.awf.pi-test.repo"

legacy_source_path() {
  "$docker_cmd" inspect -f \
    '{{range .Mounts}}{{if eq .Destination "/source"}}{{.Source}}{{end}}{{end}}' "$1" 2>/dev/null
}

# A legacy container is removed only when it is provably unused: either it is
# not running, or the source path recorded in its bind mount no longer exists,
# which proves no new gate can be started against it. A running container whose
# source path still exists belongs to a checkout that has not yet adopted
# ADR-0195, and removing it would kill that checkout's in-flight gate.
reap_legacy_containers() {
  local id running source
  for id in $("$docker_cmd" ps -aq --filter "label=$legacy_label"); do
    running="$("$docker_cmd" inspect -f '{{.State.Running}}' "$id" 2>/dev/null || echo true)"
    source="$(legacy_source_path "$id")"
    if [ "$running" != true ] || [ -z "$source" ] || [ ! -d "$source" ]; then
      "$docker_cmd" rm -f "$id" >/dev/null 2>&1 || true
    fi
  done
}

# Docker refuses to remove a volume that a surviving container still mounts, so
# the per-volume failure is itself the guard here.
reap_legacy_volumes() {
  local volume
  for volume in $("$docker_cmd" volume ls -q --filter "label=$legacy_label"); do
    "$docker_cmd" volume rm "$volume" >/dev/null 2>&1 || true
  done
}

# Untagging an image whose layers a running container still holds does not
# disturb that container; the next run rebuilds.
reap_images() {
  local images
  images="$("$docker_cmd" image ls -q "$image_repo")"
  if [ -n "$images" ]; then
    # shellcheck disable=SC2086
    "$docker_cmd" image rm -f $images >/dev/null 2>&1 || true
  fi
}

case "$command_name" in
  reset)
    reap_legacy_containers
    reap_legacy_volumes
    reap_images
    exit 0
    ;;
  run) ;;
  *)
    echo "usage: pi-extension-test <run|reset>" >&2
    exit 2
    ;;
esac

setup_start=$SECONDS
if ! "$docker_cmd" image inspect "$image" >/dev/null 2>&1; then
  "$docker_cmd" build -t "$image" "$tool_dir"
fi
printf 'pi-extension-test: setup/start %ss\n' "$((SECONDS - setup_start))"

bin_path=/opt/awf-pi-test/node_modules/.bin
test_command="c8 --all --include='.pi/extensions/awf-subagents/runner.ts' --include='.pi/extensions/awf-subagents/model-routing.ts' --include='.pi/extensions/awf-handoff/index.ts' --exclude='tools/pi-extension-test/tests/*.ts' --check-coverage --lines=100 --functions=100 --branches=100 node --import tsx --test --experimental-test-isolation=none tools/pi-extension-test/tests/*.test.ts"

# Copy only what the suite compiles and runs, about 470 KB. The superseded
# `cp -a /source/. /workspace/repo/` moved 376 MB, raced concurrent git activity
# over .git/index.lock, and copied a host node_modules that then shadowed the
# image's pinned tree during module resolution (ADR-0195). The ts-nocheck strip
# still runs after the source copy and before the TypeScript compiler.
prepare_command="$(cat <<'COMMAND'
find /workspace/repo -mindepth 1 -maxdepth 1 -exec rm -rf {} + && mkdir -p /workspace/repo/tools/pi-extension-test && cp -a /source/.pi /workspace/repo/.pi && cp -a /source/tools/pi-extension-test/tests /source/tools/pi-extension-test/fixtures /source/tools/pi-extension-test/tsconfig.json /workspace/repo/tools/pi-extension-test/ && printf '%s\n' '{"type":"module"}' > /workspace/repo/package.json && ln -s /opt/awf-pi-test/node_modules /workspace/repo/node_modules && find .pi/extensions -type f -name '*.ts' -print0 | sort -z | xargs -0 sed -i "s|^// @ts-nocheck$||" && tsc -p tools/pi-extension-test/tsconfig.json
COMMAND
)"

test_start=$SECONDS
test_log="$(mktemp)"
cleanup_test_log() {
  rm -f "$test_log" || true
}
trap cleanup_test_log EXIT

if "$docker_cmd" run --rm --entrypoint sh --workdir /workspace/repo \
  --mount "type=bind,src=$root,dst=/source,readonly" \
  "$image" -lc "export PATH=$bin_path:\$PATH; $prepare_command && $test_command" \
  >"$test_log" 2>&1; then
  cleanup_test_log
  printf 'pi-extension-test: tests %ss\n' "$((SECONDS - test_start))"
else
  test_status=$?
  cat "$test_log" >&2
  cleanup_test_log
  exit "$test_status"
fi
