## Local git hooks

This repository keeps hand-maintained hooks under `.githooks/` (not awf-rendered): `pre-commit` runs `./x check` then `./x gate`, and `pre-push` runs `./x gate full`. Wire them once per clone with `git config core.hooksPath .githooks`. They are plain checked-in scripts — edit them directly.
