`./x gate` must pass before every commit; [testing](testing.md) owns its checks and `./x gate timings`. `./x check` separately checks rendered output and repository policy; the pre-commit hook and CI run both.

For saved output, preserve the command status: `./x gate > /tmp/gate.log 2>&1; gate_status=$?; exit "$gate_status"`.
