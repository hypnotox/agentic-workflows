# Workflow

## Principles

You own the project's long-term health, not just the task in front of you: bugs you notice in passing are yours, coverage gaps are yours, and documentation drift is yours to fix in the same commit that caused it. Three rules bind every change — reality and its docs move together, the gate is green before every commit, and each commit carries exactly one concern.

## The chain

Non-trivial work follows one canonical chain:

```
brainstorming → planning (if warranted) → ADR (if warranted) → review → implementation → review
```

Brainstorming is the hard prerequisite. **Planning** is warranted by *complexity* — multi-commit or interdependent steps. An **ADR** is warranted by *load-bearing-ness* — a design decision the project must remember. Many tasks need neither; few need both. Reviews are lightweight: the grounding-check inside brainstorming subsumes plan/ADR review, and implementation review is the single terminal gate.

For the detailed criteria of when a decision is load-bearing enough to warrant an ADR — and the ADR format itself — see [`docs/decisions/README.md`](docs/decisions/README.md).

## Commit discipline

Use Conventional Commits, one concern per commit. Stage files explicitly rather than `git add -A`, so each commit is a deliberate, reviewable unit. The gate runs before every commit; a commit that cannot pass the gate is not ready to land.

## Documentation currency

Documentation travels with the change that makes it true. When you change behaviour, update the affected docs — this file, the agent guide, ADRs, and any reference tables — in the same commit. A separate "docs later" commit is drift waiting to happen.
