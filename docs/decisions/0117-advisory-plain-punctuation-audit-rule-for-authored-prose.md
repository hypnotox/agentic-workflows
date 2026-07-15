---
status: Proposed
date: 2026-07-15
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [audit-rules, doc-standard, commit-conformance]
related: [17, 107, 113, 115]
domains: [tooling, adr-system]
---
# ADR-0117: Advisory plain-punctuation audit rule for authored prose

## Context

ADR-0115 bans seven typographic punctuation substitutes (U+2014, U+2013, U+2026, U+2018, U+2019,
U+201C, U+201D) across the three surfaces awf *ships*: the embedded `templates.FS`, the embedded
`changelog.FS`, and production Go string literals. That invariant is a hard gate, and it is
deliberately silent about prose a project *authors*: ADRs, plans, and hand-written docs. ADR-0115
Decision item 6 named this gap and assigned it here, promising that authored prose is "warned
about, never rewritten".

The gap is the one that motivated the whole effort. The rule's origin is a maintainer observation
that agents overuse the em-dash, and agents author ADRs and plans. A ban reaching only awf's own
shipped strings never touches the prose that prompted it.

The measured backlog is why the rule cannot simply be a gate: this repository carries 2344
em-dashes in authored ADR bodies and 4347 under the plans directory. ADRs are append-only
historical records, so a repository-wide scan would fail on landing and demand a rewrite of settled
rationale, which ADR-0115 Decision item 5 explicitly forbids.

Two existing pieces make a cheap, precise rule possible, and neither needs new plumbing.
`FileChange` already populates `OldText`/`NewText` for `.md` files, because the ADR rules need
frontmatter. `Inputs` already carries `GeneratedPaths`, because other rules must distinguish
authored files from rendered ones. A rule that reads changed markdown and skips generated output is
assembled from parts already on the shelf.

ADR-0107 is the governing precedent for severity: it downgraded the changelog conformance rule from
an error to an advisory warning rather than deleting it, establishing that a conformance concern
awf cannot mechanically prove belongs in `awf audit` as a `Warning`.

## Decision

1. **A new advisory audit rule, `plain-punctuation`.** It emits a `Warning` naming the file and the
   codepoints found. It never errors, and `awf audit` continues to exit zero on warnings.

2. **The trigger is a net increase, not a presence.** For each changed markdown file the rule counts
   each banned codepoint in `OldText` and in `NewText`, and warns only where the new count exceeds
   the old. Grandfathering is therefore emergent rather than configured: a newly added file has an
   empty `OldText`, so every banned codepoint in it is new and is flagged; an edit that leaves a
   legacy file's existing glyphs untouched is silent, even when the file carries hundreds. There is
   no path allowlist, no cutoff date, and no exemption list to maintain. The 2344 authored ADR
   em-dashes and the 4347 in plans stay silent forever unless someone adds more.

3. **Scope is authored, non-generated markdown.** A changed `.md` file under the configured docs
   directory, ADR directory, or plans directory qualifies. Any path in `Inputs.GeneratedPaths` is
   skipped: rendered output is not authored, its glyphs are its source's fault, and ADR-0115's gate
   already covers the shipped sources. A deleted file is skipped.

4. **The rule ships to adopters, enabled by default,** via `audit.plainPunctuation` (bool, default
   `true`), matching the shape of every other advisory knob (`audit.domainDocStaleness`,
   `audit.domainCodeStaleness`, `audit.undocumentedDomain`, `audit.uncommittedChanges`). An adopter
   who disagrees sets it to `false`, and an adopter who wants different prose guidance overrides the
   documentation standard's section with a convention part.

5. **This reverses ADR-0113's house-style disclaimer, deliberately and on the record.** ADR-0113
   said awf "enforces its own shipped prose, not an adopter's house style", and used that to
   exclude adopter content. ADR-0115 kept that boundary for its *gate*, which is why it warns
   rather than rewrites an adopter's ADR title. This ADR draws the line differently for *advice*:
   awf already ships an opinionated documentation standard telling adopters how to write, so
   shipping the check that matches that standard is consistency, not overreach. The distinction
   that makes both coherent: awf never mutates adopter content and never fails an adopter's build
   over house style, but it does tell them, once, in a warning they can switch off.

6. **Advisory severity is the depiction escape hatch.** ADR-0115 Decision item 7 keeps a
   no-escape-hatch posture for the gate, which it can afford because its scope boundary leaves
   sidecar data free to depict a glyph. This rule has no such boundary to hide behind: a doc that
   documents the em-dash will trip it. That is acceptable precisely because the finding is a
   `Warning`. A warning an author reads and knowingly ignores is the escape hatch, and it costs no
   exemption list, no marker comment, and no config surface.

7. **The rule flags new ADRs and plans, and that is the point.** ADR-0113 Decision item 2 had
   promised that hand-authored ADRs and plans "keep em-dashes freely". That promise is withdrawn
   here for *new* prose only, which is the entire reason the rule exists.

## Invariants

- `` `invariant: audit-plain-punctuation` ``: with `audit.plainPunctuation` enabled, `awf audit`
  emits a `Warning` for a changed non-generated markdown file under the docs, ADR, or plans
  directory whose banned-codepoint count rises, and emits nothing when the count is unchanged or
  falls, when the path is generated, or when the knob is `false`. Backed by
  `TestPlainPunctuationRule` in `internal/audit/audit_test.go`.

## Consequences

- **The rule that motivated the effort finally reaches the prose that motivated it.** Every ADR and
  plan an agent writes from now on is checked, without touching a line of settled rationale.
- **The backlog stays silent without a carve-out.** Net-increase semantics mean the 6691 existing
  occurrences never warn, so the rule lands green on a repository that violates it thousands of
  times. This property is what makes shipping it to adopters safe: an adopter's first `awf audit`
  after upgrading warns about nothing they have already written.
- **A net-zero swap slips through.** Replacing one em-dashed sentence with a different em-dashed
  sentence leaves the count unchanged and is not flagged. Accepted: the rule is advisory, and
  precise hunk attribution would cost diff parsing for a warning.
- **Adopters inherit an opinion they must actively decline.** Default-`true` means a project that
  likes em-dashes sees warnings until it sets `audit.plainPunctuation: false`. That is the cost of
  Decision item 5, and it is the same bargain every other default-on advisory rule already makes.
- **A doc that depicts a banned glyph warns, forever.** There is no way to silence it short of
  disabling the rule. Decision item 6 accepts this rather than building an exemption mechanism for
  a case measured at one file.
- **The config key costs six coordinated touchpoints.** An `internal/config` field, the
  `internal/audit/settings.go` default, the `Settings` promotion into `Inputs`, an
  `internal/configspec` descriptor, the `internal/project/configreference.go` live-state case, and
  a regenerated `docs/config-reference.md` plus `examples/sundial/docs/config-reference.md`.
  Missing any one trips the closed-config-tree checks or leaves the reference stale.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Flag presence, not net increase | Simpler to implement, but touching any legacy ADR or plan would flood the author with warnings about prose they did not write, which trains them to ignore the rule or switch it off. Net-increase makes grandfathering emergent instead of configured. |
| Flag only newly added files (`Action == Added`) | Grandfathers just as cleanly and is simpler still, but it lets an author add an em-dash to an existing doc without a word, which is the most likely way new glyphs enter a mature repository. |
| Put the rule in `cmd/repoaudit` (repo-local) | Honours ADR-0113's house-style boundary exactly, needs no config key, and drops the six touchpoints. Rejected because awf already ships the documentation standard that states this rule; shipping the standard without the check is the inconsistency worth fixing, not the other way round. |
| Make it an `Error` | Would give the rule teeth, but it is a style opinion awf cannot mechanically prove is right, and a doc legitimately depicting a glyph would be unfixable. ADR-0107's downgrade of the changelog rule is the governing precedent. |
| Default the knob to `false` | Respects adopter autonomy maximally, but a default-off check is a check nobody runs, and it would leave the shipped documentation standard stating a rule nothing enforces, which is the exact gap ADR-0113 left. |
| Extend the scope to `.awf/` sidecar data and convention parts | Would close ADR-0115's stated sidecar blind spot, but `FileChange` loads text only for `.md`, so YAML sidecars need new plumbing, and ADR-0115 Decision item 7 depends on sidecar data staying free to depict a glyph. Deferred as a separate question. |
