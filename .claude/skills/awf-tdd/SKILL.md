---
name: awf-tdd
description: Use to write the failing test before the implementation change in awf — for bug fixes (strongly recommended) and feature work where the failure mode is testable in isolation.
---

# awf-tdd

The test-first discipline as a project-owned task skill.

## Pick the right test surface
- **Package unit tests** → Go _test.go in `internal/<pkg>`
- **Template golden tests** → render assertions in `internal/project/spine_test.go`
- **CLI integration tests** → subprocess/temp-dir in `cmd/awf`


## Procedure

1. Write the failing test capturing the wrong (bug) or missing (feature) behaviour.
2. Run it and confirm it fails for the right reason: `go test ./...`.
3. Implement the minimal change to pass.
4. Run the gate: `./x gate`.

## Notes
- Coverage may never regress: a fix that breaks an existing passing test is itself a regression.
