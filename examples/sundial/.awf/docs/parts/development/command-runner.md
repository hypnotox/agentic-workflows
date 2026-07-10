## Command runner

`./x` wraps every repo task: `gate` (tests + vet; `gate full` is identical), `test`,
and the awf verbs `sync`, `check`, `invariants`, `audit`, `commit-gate`, `new`,
which run the release pinned in `.awf/bootstrap.sh`.
