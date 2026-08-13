This repository wires five rendered payloads through `.githooks/`: pre-commit checks `./x check` and `./x gate`; commit-msg checks the final message; pre-merge-commit checks staged state; reference-transaction and pre-push apply commit policy. `./x render` keeps payloads current.

The stubs resolve the invoking worktree before delegation. Enable them once per clone with `git config core.hooksPath .githooks`. Preview policy with `./awf check commit-policy <revision-or-range>...`. GitHub branch protection is the final publication boundary.

`awf check staged commit` validates Conventional Commits and stale-format ADR merge trailers. Correct a refusal and run `git commit`; do not repeat the merge or retrofit an ADR.
