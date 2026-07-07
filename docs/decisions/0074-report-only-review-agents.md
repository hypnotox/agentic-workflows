---
status: Proposed
date: 2026-07-07
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [workflow, review, agents]
related: [0050, 0052, 0054, 0017, 0067]
domains: [tooling]
---
# ADR-0074: Report-only review agents

## Context

The three chain review subagents — `adr-reviewer`, `plan-reviewer`, `code-reviewer` — currently
both *judge* and *fix*. Each reads its artifact, runs its lenses, classifies every finding as
`mechanical / reasoned / user-decision`, then **applies the mechanical and reasoned fixes itself**
inside its own fresh-context subagent, re-reviews the result under a 3-round soft cap, and returns a
digest counting fixes applied. The dispatching `awf-reviewing-*` skill only surfaces that digest,
commits whatever the subagent already edited on disk, and gates on `user-decision` findings.

This conflates two roles in one agent. A reviewer's value is its independence: it is dispatched in
fresh context precisely so it judges the work without the implementer's assumptions. When that same
agent then edits the artifact it just judged, the independence it was spawned for is spent — it is
now invested in its own edits, and the next thing it "re-reviews" is its own work. The edits also
happen invisibly, inside the subagent's context: the main thread (and the user watching it) sees a
digest of changes already written to disk, not the changes as they are proposed and made.

The mechanism to change this is already centralised. ADR-0052 deduplicated the review-discipline
spine into two awf-owned partials — `templates/partials/review-spine-head.md` (Finding schema,
Classification rules) and `review-spine-tail.md` (Dedup, Review procedure, Digest format) — spliced
into all three reviewer templates via `awf:include`. The fix-application instructions live in the
spine (`review-spine-tail.md` Review procedure step 5, the re-review loop step 6, the digest's
"fixes applied" counts; `review-spine-head.md`'s "apply the fix directly" classification
imperatives), so a single spine edit changes all three reviewers at once. The per-agent
`.data.fixesAsCommits` flag — set `true` only for `code-reviewer` (`internal/catalog/standard.go`)
and read only by the spine's commit clause — is the spine's only fix-application conditional
(other per-reviewer data — `readStep`, `digestLabel`, `focusItems` — only vary rendered text), and
becomes dead once the apply clause leaves the spine. ADR-0052's own invariants govern the include *mechanism*,
not the spine *content*, so the spine body is free to change; ADR-0050's reviewing-skill/agent
pairing is about *enablement* (a reviewing skill requires its agent enabled) and is untouched — the
agent stays enabled, only its role narrows.

## Decision

1. **Review subagents are report-only judges.** The shared spine and all three reviewer agent
   templates instruct the agent to read, run its lenses, dedup, classify each finding
   `mechanical / reasoned / user-decision`, and emit a findings digest — and nothing more. No
   reviewer agent edits a file, commits, or runs an internal re-review loop. The agent is
   single-pass.

2. **The classification vocabulary is unchanged; only its subject moves.** `mechanical / reasoned /
   user-decision` still partition findings by what acting on them requires. The spine's
   Classification rules describe what each class *requires* (unambiguous / judgment-with-rationale /
   genuine fork) rather than instructing the reviewer to apply anything. Routing by classification
   kind — not severity — is retained.

3. **The dispatching `awf-reviewing-*` skill owns application.** After the reviewer returns
   findings, the main-thread skill applies `mechanical` fixes directly, applies `reasoned` fixes
   with a one-line rationale, and escalates `user-decision` findings to the user. It edits the
   artifact (the plan or ADR file in place; the code diff for the impl reviewer) and commits the
   fixes as new commits — never `--amend` — with the project's gate passing before each commit.
   Application is thus visible on the main thread.

4. **Verification is exactly one fresh verify-pass dispatch.** After applying fixes and passing the
   gate, the skill dispatches **one** fresh reviewer to confirm the fixes resolved the findings
   without introducing new ones. Residual structural findings from that verify pass are escalated as
   `user-decision`; the skill does not loop further without explicit user direction. This replaces
   the agent-side 3-round soft cap.

5. **The reviewer digest reports findings, not fixes.** Because the reviewer applies nothing, its
   digest reports findings by classification (`Findings: N (mechanical K, reasoned L, user-decision
   P)`) with no "rounds" or "fixes applied" counts. What was applied is reported by the skill that
   applied it.

6. **`.data.fixesAsCommits` is retired.** With the spine's apply/commit clause gone, the flag has no
   reader; it is removed from the catalog. The commit-as-new-commits and gate-before-commit
   discipline lives in each skill's `apply-fixes-commit` section, where the skill now does the
   applying.

## Invariants

- `inv: reviewers-report-only` — no rendered reviewer agent (the shared spine or any of the three
  reviewer templates) contains a fix-application, commit, or re-review imperative: the reviewer is
  instructed to report findings, never to edit, commit, or loop. Backed by a golden-render test in
  `internal/project` over the `awf:include`-expanded reviewer renders asserting the rendered output
  contains none of the ban phrases — e.g. `Apply mechanical and reasoned fixes directly`, `apply the
  fix directly`, `as new commits`, `3-round soft cap` — and carrying a `// invariant:
  reviewers-report-only` marker so the ADR-0008/0031 backing lands in the same commit that flips this
  ADR to Implemented.
- The `mechanical / reasoned / user-decision` classification vocabulary remains the single routing
  axis for review findings across the spine and all four `awf-reviewing-*` skills (textual).
- Every reviewer dispatch by an `awf-reviewing-*` skill is followed by at most one verify-pass
  dispatch; no skill instructs an unbounded re-review loop (textual).

## Consequences

- **Independence is restored.** A reviewer judges and never edits what it judged; the verify pass is
  a *fresh* reviewer with no stake in the fixes, so its judgment is genuinely independent — the
  property the fresh-context dispatch was always meant to provide.
- **Application is visible and auditable.** Fixes are proposed by the reviewer and made by the
  main-thread skill, where the user sees each one, rather than appearing pre-made on disk out of a
  subagent's context.
- **The spine change lands once for all three reviewers** (ADR-0052), keeping the reviewers'
  discipline uniform; the alternative of diverging one reviewer is avoided.
- **Cost: one extra fresh dispatch per review that applied fixes.** The verify pass has no prior
  review context and must re-read the artifact — that is the price of, and the point of,
  independence. Reviews that surface no fixable findings incur no verify pass. Net dispatch count is
  comparable to today's worst case (a 2–3-round in-context loop) while being simpler to reason about.
- **The main-thread skill carries more responsibility** (apply → gate → commit → verify), moving
  work from the opaque subagent into the visible chain. The `awf-reviewing-*` skills grow their
  `apply-fixes-commit` and (renamed-in-body) verify-pass sections accordingly.
- **Downstream work (a plan):** edit the two spine partials, the three agent-template headers, the
  four `awf-reviewing-*` skills (`dispatch-subagent` — `dispatch-subagent-narrowed` in the resync
  skill — `classify-route-findings`, `apply-fixes-commit`, `re-review-loop` bodies — marker names kept stable so
  `skill-section-parity` needs no catalog edit), remove the catalog `fixesAsCommits` key, and update
  the reviewer render assertions (drop the "3-round soft cap" phrase; add the apply-imperative ban)
  — re-rendered across both the `.claude/` and `.cursor/` trees.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep reviewers applying fixes (status quo) | Conflates judge and fixer; the agent re-reviews its own edits; changes are made invisibly inside the subagent — the independence the fresh-context dispatch exists to provide is spent. |
| Make only `code-reviewer` report-only | Diverges the shared spine per-reviewer, splitting one uniform discipline into two mental models for marginal benefit; the independence argument applies equally to ADR and plan reviewers. |
| Report-only, but a full 3-round loop driven from the main thread | Up to three fresh verify dispatches per review — each re-reading the artifact from scratch — for little gain over one; the common failure (a fix that regressed or under-resolved) is caught by a single verify pass. |
| Report-only with no automatic re-review | Loses confirmation that applied fixes actually resolved the findings; the gate catches compile/test breakage but not an unaddressed or half-addressed review finding. |
