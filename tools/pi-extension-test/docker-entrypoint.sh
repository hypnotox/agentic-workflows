#!/bin/sh
set -eu

marker="/workspace/node_modules/.awf-pi-test-${AWF_PI_TEST_FINGERPRINT:?}"
if [ ! -f "$marker" ]; then
  find /workspace/node_modules -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  cp -a /opt/awf-pi-test/node_modules/. /workspace/node_modules/
  touch "$marker"
fi
exec tail -f /dev/null
