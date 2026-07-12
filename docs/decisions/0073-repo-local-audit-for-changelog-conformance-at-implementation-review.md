---
status: Implemented
date: 2026-07-07
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, audit, changelog, dispatch, adoption]
related: [17, 41, 67, 72]
domains: [tooling]
---
# ADR-0073: Repo-local audit for changelog conformance at implementation review

## Context

This repo's `changelog/CHANGELOG.md` keeps its per-effort entries under a standing
`## [Unreleased]` section (ADR-0041), and entries are supposed to land with the change.
They repeatedly do not: they get missed during the effort and backfilled at release from
commit archaeology (v0.10.0 was backfilled in `19aed1c`; the ADR-0072 effort missed its
entry entirely and it was caught only by the `awf-retrospective` catch-net, despite the
prose reminder that step already carries). A prose reminder recorded the failure mode and
did not prevent its recurrence — the signal to move it onto a deterministic rung.

The obvious deterministic home — a rule in `awf audit` — is wrong. `awf audit` is part of
the **shipped awf standard**: every adopter runs it (ADR-0017). Changelog management is not
part of awf's scope, and defaulting a changelog rule into `awf audit` would impose this
repo's release process on projects that structure changelogs differently or not at all. The
guard must be **repo-specific dev tooling** — the same category as `cmd/covercheck`,
`cmd/deadcodecheck`, `cmd/mutants`, `./x`, and the CI workflows: hand-maintained, outside the
render/lock set, and never built into the shipped `./cmd/awf` binary (GoReleaser builds only
`./cmd/awf`).

The enforcement *point* also matters. The gate (`./x gate`) is deliberately a pure
working-tree analysis with no git-history dependency; a per-commit changelog check would give
it a diff-base dependency and false-positive on every intermediate commit of a multi-commit
effort (entries legitimately batch — one entry per effort, not per commit). The natural point
is the one where `awf-reviewing-impl` *already* runs a conformance audit (`awf audit`, its
`run-audit` step): after all of an effort's commits have landed, over the effort's own SHA
range, advisory in the same Error-blocks / Warning-informs sense.

## Decision

1. **A repo-local audit command, mirroring `awf audit`'s contract, not its code or its gate
   coupling.** Add a Go `cmd/repoaudit` package (name may be refined in the plan) — a repo
   helper in the `cmd/covercheck` / `cmd/deadcodecheck` / `cmd/mutants` family: **not**
   imported by `./cmd/awf`, excluded from the GoReleaser binary, wired as a new `./x
   audit-local` case. It emits findings with a Warning/Error severity and exits non-zero only
   when an Error finding is present — mirroring `internal/audit`'s reporting contract. It does
   **not** run the gate (unlike `awf audit`, which runs the gate first); it only audits. It
   carries a small rules registry so a second repo-local rule is cheap later; the first and
   only rule now is changelog conformance.

2. **The changelog-conformance rule.** Over a commit range, if any commit touches an
   adopter-facing path **and** the `[Unreleased]` section of `changelog/CHANGELOG.md` is
   unchanged across the range, emit one Error finding. "Adopter-facing" is a conservative,
   logged path allowlist — `templates/`, `cmd/awf/`, and the config/lock schema — deliberately
   a heuristic net, not exhaustive (a render-logic-only change under `internal/` can alter
   adopter output without matching; the `awf-retrospective` step remains the judgment
   backstop). The "`[Unreleased]` unchanged" test is by **section extraction and comparison**,
   not a file-level path check: extract the section body (the lines between `## [Unreleased]`
   and the next `## [` header) from the file at the range base (`git show <base>:…`) and at the
   range head, and compare — a file-level "CHANGELOG.md appears in the diff" test would pass
   when only an older release section changed. The extractor is a small repo-local helper, not a
   reuse of the shipped `internal/changelog` parser: that parser's header regex requires a dated
   `## [X.Y.Z] - YYYY-MM-DD` line and discards the `## [Unreleased]` body outright (ADR-0041), so
   it cannot supply this check — a deliberate, contained duplication rather than widening the
   shipped parser to model an unreleased section it has no other reason to understand.

3. **Range: the effort's own SHA range, supplied by the caller.** `./x audit-local` accepts an
   explicit `<base>..<head>` range and defaults to `origin/main..HEAD` (this repo's unpushed
   work, since it pushes straight to `main`). `awf-reviewing-impl` already computes
   `baseSha`/`headSha` for its review; awf's own copy of that skill passes that range, so the
   rule judges precisely "did *this effort* add adopter-facing changes without a changelog
   entry."

4. **Invoked at implementation review, without polluting the shipped skill.** `awf-reviewing-impl`
   is a rendered standard artifact; its `run-audit` section default must keep naming only
   `awf audit`, because adopters have no `./x audit-local`. awf overrides that section in its
   own config at `.awf/skills/parts/reviewing-impl/run-audit.md`, extending it to also run the
   repo-local audit and route its findings the same way (Error blocks the review from
   concluding, Warning is advisory).

5. **The override extends, not replaces — dogfooding ADR-0072.** The override re-injects the
   standard `run-audit` default via the `{{=awf:sectionDefault}}` placeholder and appends the
   repo-audit instruction, rather than copy-pasting the default body (which would fork and
   rot). This makes awf the first real adopter of the ADR-0072 re-injection feature. (Safe:
   `run-audit` is a plain, non-stub section, so re-injection renders rather than hard-erroring.)

## Invariants

- `inv: repo-audit-error-exit` — the repo-local audit exits non-zero when and only when it
  reports at least one Error finding; Warning-only and clean runs exit zero.

The "repo-audit is never part of the shipped standard" property is **not** a tagged invariant:
it holds by construction (see Consequences), not by a check. Its auditable logic lives in the
`cmd/repoaudit` `package main` — which Go cannot import — so `./cmd/awf` *cannot* reach it, and
GoReleaser builds only `./cmd/awf`. An import-graph test would assert a property the language
already guarantees; there is nothing meaningful to back.

## Consequences

- **The recurring miss gets a deterministic catch at the right moment.** At implementation
  review — after the effort's commits land, before the retrospective — a missing changelog
  entry for adopter-facing work becomes an Error the reviewer must resolve or escalate, exactly
  as it treats an `awf audit` Error. The `awf-retrospective` changelog step (ADR-0041/0067)
  stays as the prose backstop for what the heuristic path-allowlist misses.
- **The scope boundary is structural, not merely conventional.** Repo-specific conformance
  has a home (`cmd/repoaudit` + `./x audit-local`) separated from the shipped `awf audit` by
  construction: the audit logic sits in a `package main` that nothing can import and that
  GoReleaser never builds into the released binary, so no changelog rule can reach `awf audit`
  or an adopter — no test needed to hold the line. A future repo-only rule adds a registry
  entry rather than tempting a standard edit. The cost is a second audit surface to keep in
  mind, mitigated by both mirroring the same finding contract.
- **A heuristic, not a proof.** The path allowlist can miss adopter-facing changes that touch
  only render logic; the rule logs what it considered so the gap is visible, and does not claim
  completeness. It is a net under a known-recurring miss, not a total guarantee. The section
  comparison also fails **open**: the rule fires only when the `[Unreleased]` body is byte-identical
  across the range, so anything that changes that body for a non-entry reason — a release-promotion
  commit that empties `[Unreleased]` into a dated section, or an unrelated concurrent edit inside
  the range — reads as "an entry was added" and silently suppresses the check. This is acceptable
  for the push-straight-to-`main`, one-effort-at-a-time flow the range assumes (a release promotion
  is not normally mid-effort), but the rule does not attempt to distinguish an entry from any other
  `[Unreleased]` edit; if that assumption ever breaks the retrospective backstop remains.
- **awf dogfoods ADR-0072 immediately**, validating the re-injection feature on a real
  override and giving the `run-audit` extension a single source of truth (the shipped default).
- **No gate or CI behavior changes.** `./x gate` stays a pure working-tree analysis; the new
  command is advisory and invoked by the review step, not wired into the gate. (A CI
  invocation is possible later but is out of scope here.)

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| **A rule inside `awf audit`.** | `awf audit` is the shipped standard every adopter runs; changelog management is not in awf's scope, so a default changelog rule there would impose this repo's process on all adopters. |
| **A step in `./x gate`.** | The gate is a pure working-tree analysis by design; a changelog check adds a git-history dependency and false-positives on every intermediate commit of a multi-commit effort (entries batch per effort, not per commit). |
| **A CI-only check (fail the build).** | Post-push for a push-straight-to-`main` flow, so it flags after the fact rather than at the natural review point; and it does not integrate with the impl-review finding-routing the way an audit does. Viable as a later addition, not the primary mechanism. |
| **A release-workflow guard only.** | Blocks the release, not the effort — too late to avoid the backfill archaeology the miss causes, and does not surface per-effort. |
| **Override `run-audit` as a plain replacement** (copy the default + append). | Forks the shipped default; it rots when the standard `run-audit` guidance changes. Re-injecting via `{{=awf:sectionDefault}}` keeps one source of truth (and dogfoods ADR-0072). |
