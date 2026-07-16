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

hash_files() {
  sha256sum "$tool_dir/Dockerfile" "$tool_dir/docker-entrypoint.sh" \
    "$tool_dir/package.json" "$tool_dir/package-lock.json" | sha256sum | cut -d' ' -f1
}

dep_hash="$(hash_files)"
repo_hash="$(printf '%s' "$root" | sha256sum | cut -c1-12)"
short_dep="${dep_hash:0:12}"
repo_label="dev.awf.pi-test.repo=$repo_hash"
dep_label="dev.awf.pi-test.deps=$dep_hash"
image="awf-pi-extension-test:$short_dep"
container="awf-pi-extension-test-$repo_hash-$short_dep"
volume="awf-pi-extension-test-deps-$repo_hash-$short_dep"

matching_containers() {
  "$docker_cmd" ps -aq --filter "label=$repo_label"
}

stop_matching() {
  local ids
  ids="$(matching_containers)"
  if [ -n "$ids" ]; then
    # shellcheck disable=SC2086
    "$docker_cmd" stop $ids >/dev/null
  fi
}

reset_matching() {
  local ids volumes images
  ids="$(matching_containers)"
  if [ -n "$ids" ]; then
    # shellcheck disable=SC2086
    "$docker_cmd" rm -f $ids >/dev/null
  fi
  volumes="$("$docker_cmd" volume ls -q --filter "label=$repo_label")"
  if [ -n "$volumes" ]; then
    # shellcheck disable=SC2086
    "$docker_cmd" volume rm $volumes >/dev/null
  fi
  images="$("$docker_cmd" image ls -q --filter "label=$repo_label")"
  if [ -n "$images" ]; then
    # shellcheck disable=SC2086
    "$docker_cmd" image rm -f $images >/dev/null
  fi
}

case "$command_name" in
  stop)
    stop_matching
    exit 0
    ;;
  reset)
    reset_matching
    exit 0
    ;;
  run) ;;
  *)
    echo "usage: ./x pi-test <run|stop|reset>" >&2
    exit 2
    ;;
esac

setup_start=$SECONDS
if ! "$docker_cmd" image inspect "$image" >/dev/null 2>&1; then
  "$docker_cmd" build --label "$repo_label" --label "$dep_label" -t "$image" "$tool_dir"
fi

for stale in $(matching_containers); do
  if [ "$stale" != "$("$docker_cmd" ps -aq --filter "name=^/${container}$")" ]; then
    "$docker_cmd" rm -f "$stale" >/dev/null
  fi
done

if ! "$docker_cmd" volume inspect "$volume" >/dev/null 2>&1; then
  "$docker_cmd" volume create --label "$repo_label" --label "$dep_label" "$volume" >/dev/null
fi

if ! "$docker_cmd" container inspect "$container" >/dev/null 2>&1; then
  "$docker_cmd" create --name "$container" --label "$repo_label" --label "$dep_label" \
    -e "AWF_PI_TEST_FINGERPRINT=$dep_hash" \
    --mount "type=bind,src=$root,dst=/source,readonly" \
    --mount "type=volume,src=$volume,dst=/workspace/node_modules" \
    "$image" >/dev/null
fi

if [ "$("$docker_cmd" inspect -f '{{.State.Running}}' "$container")" != "true" ]; then
  "$docker_cmd" start "$container" >/dev/null
fi
ready=false
for _ in $(seq 1 300); do
  if "$docker_cmd" exec "$container" test -f "/workspace/node_modules/.awf-pi-test-$dep_hash"; then
    ready=true
    break
  fi
  sleep 0.1
done
if [ "$ready" != true ]; then
  echo "pi-extension-test: dependency volume did not become ready" >&2
  exit 1
fi
printf 'pi-extension-test: setup/start %ss\n' "$((SECONDS - setup_start))"

cleanup_ci() {
  "$docker_cmd" rm -f "$container" >/dev/null 2>&1 || true
  "$docker_cmd" volume rm "$volume" >/dev/null 2>&1 || true
}
if [ -n "${CI:-}" ]; then
  trap cleanup_ci EXIT
fi

test_start=$SECONDS
"$docker_cmd" exec --workdir /workspace/repo "$container" sh -lc \
  'find /workspace/repo -mindepth 1 -maxdepth 1 -exec rm -rf {} + && cp -a /source/. /workspace/repo/ && ln -s /workspace/node_modules /workspace/repo/node_modules && PATH=/workspace/node_modules/.bin:$PATH tsc -p tools/pi-extension-test/tsconfig.json && PATH=/workspace/node_modules/.bin:$PATH node --import tsx --test --experimental-test-coverage --test-coverage-lines=100 --test-coverage-functions=100 --test-coverage-branches=100 tools/pi-extension-test/fixture/*.test.ts'
printf 'pi-extension-test: tests %ss\n' "$((SECONDS - test_start))"
