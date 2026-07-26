## Command runner

`./x` wraps every repo task: `gate` (tests + vet; `gate full` is identical), `test`,
and the awf verbs `render`, `check`, `audit`, `new`,
which run the release pinned in `.awf/bootstrap.sh`.
