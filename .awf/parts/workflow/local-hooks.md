This repository tracks five optional client-side preflight stubs under `.githooks/`: pre-commit checks `./x check` and `./x gate`; commit-msg checks the final message; pre-merge-commit checks staged state; reference-transaction and pre-push apply commit policy. `./x render` keeps their payloads current.

The stubs resolve the invoking worktree before delegation. A clone may activate them with `git config core.hooksPath .githooks`. Preview policy with `./awf check commit-policy <revision-or-range>...`. These checks do not gate remote updates.

The active GitHub repository ruleset `main` (ID `18766557`) is the final remote control for publishing `main`: with no bypass actors, it requires signed commits and blocks non-fast-forward updates and deletion. It does not require CI status checks before accepting an update. Reverify the live rule with `gh api repos/hypnotox/agentic-workflows/rulesets/18766557` whenever the remote policy changes.

`awf check staged commit` validates Conventional Commits and stale-format ADR merge trailers. Correct a refusal and run `git commit`; do not repeat the merge or retrofit an ADR.
