Use the failing output to choose the next inspection:

| Need | Command |
|---|---|
| Transaction state | `git status --short` and `git diff` |
| Generated drift | `./awf check repo drift` |
| Current-state authority | `./awf check repo state`, then `./awf context <affected-path>` and `./awf topic <domain>/<topic>` |
| Code failure | `./x test`; use `./x gate full` for terminal verification |

Take `<affected-path>` from the refusal and the qualified topic from the context report. See [Working with awf](working-with-awf.md), [Testing](testing.md), and [Development](development.md) for owned procedure.
