`./x gate` must pass before every commit. It runs the profiled tests, coverage checks, Pi extension type and coverage checks, `go vet`, `golangci-lint`, dead-code checking, and workflow-pin checking. See [testing](testing.md) for details.

`./x check` separately checks rendered output and repository policy. The pre-commit hook and CI run both. Use `./x gate timings` for per-stage timing.

For saved output, preserve the command status: `./x gate > /tmp/gate.log 2>&1; gate_status=$?; exit "$gate_status"`.
