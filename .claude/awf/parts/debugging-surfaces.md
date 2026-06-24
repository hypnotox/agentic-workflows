1. **Enumerate observable surfaces and validate the hypothesis.** Pick the cheapest oracle that can confirm or refute it. Go-specific surfaces to consider:

   - **`go test -run TestX -v ./pkg/...`** — run a single test verbosely; the output shows which assertion failed and what was expected vs. actual.
   - **`go test -v ./... 2>&1 | grep -A5 FAIL`** — skim all failures with context lines.
   - **`awf check`** — catches rendered no-value tokens and manifest drift; run after any sync to confirm the output is clean.
   - **Golden-test diffs** (`internal/project/spine_test.go`) — if a template render changed unexpectedly, the golden diff shows the exact line-level divergence; update goldens only when the change is intentional.
   - **`go vet ./...`** — catches type-system violations, printf format mismatches, and unreachable code the compiler accepts.
   - **`dlv test ./pkg/... -- -test.run TestX`** — attach Delve to a test binary for breakpoint-level inspection when log output is insufficient.
   - **Print/log debugging** — add `t.Logf(...)` or `fmt.Fprintf(os.Stderr, ...)` inline; remove before committing. For template render bugs, log the `data` map before `tmpl.Execute` to confirm what the renderer sees.
   - **`.claude/awf.lock`** — the lock file records the rendered content hash; `awf check` compares on-disk files against it. Inspect the lock directly if `awf check` reports false drift.

   Inspect the surface that the hypothesis predicts will be wrong. Update the hypothesis if the evidence refutes it and loop.
