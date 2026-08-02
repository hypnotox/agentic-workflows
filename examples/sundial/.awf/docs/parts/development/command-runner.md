## Command runner

`./x` wraps every repo task: `gate` (tests + vet, with no arguments) and `test` (forwarding Go test arguments),
and the awf verbs `render`, `check`, `audit`, `new`,
which run the release pinned in `.awf/bootstrap.sh`.
