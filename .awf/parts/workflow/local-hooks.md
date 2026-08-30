This repository tracks five optional client-side preflight stubs under `.githooks/`: pre-commit checks `./x check` and the fast `./x gate`; commit-msg checks the final message; pre-merge-commit checks staged state; reference-transaction and pre-push apply commit policy and the fast `./x gate`. `./x render` keeps their payloads current.

The stubs resolve the invoking worktree before delegation. A clone may activate them with `git config core.hooksPath .githooks`. Preview policy with `./awf check commit-policy <revision-or-range>...`. These checks do not gate remote updates by themselves.

The active GitHub repository ruleset `main` (ID `18766557`) is the final remote control for publishing `main`: with no bypass actors, it requires signed commits and blocks non-fast-forward updates and deletion, but has no required-status rule. `CI / gate` is therefore definitive post-push assurance rather than a pre-update requirement on `main`. The active `release tags` ruleset (ID `21631403`) requires only `CI / gate` before `refs/tags/v*` can be created or updated. Reverify both live rules with `gh api repos/hypnotox/agentic-workflows/rulesets/<id>` whenever remote policy changes.

`awf check staged commit` validates Conventional Commits and stale-format ADR merge trailers. Correct a refusal and run `git commit`; do not repeat the merge or retrofit an ADR.
