---
status: Implemented
date: 2026-07-15
tags: [audit-rules, doc-standard, commit-conformance]
related: [17, 107, 113, 115, 119]
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

Two existing pieces make a cheap, precise rule possible, and neither of the two needs new plumbing.
`FileChange` already populates `OldText`/`NewText` for `.md` files, because the ADR rules need
frontmatter. `Inputs` already carries `GeneratedPaths`, because other rules must distinguish
authored files from rendered ones. So a rule that reads changed markdown and skips generated output
is mostly assembled from parts already on the shelf: the one piece missing is a `docsDir` field on
`Inputs`, which Decision item 3 adds.

ADR-0107 is the governing precedent for severity: it downgraded the changelog conformance rule from
an error to an advisory warning rather than deleting it, establishing that a conformance concern
awf cannot mechanically prove belongs in `awf audit` as a `Warning`.

## Decision

1. **A new advisory audit rule, `plain-punctuation`.** It emits a `Warning` naming the file and the
   codepoints found. It never errors, and `awf audit` continues to exit zero on warnings.

2. **The trigger is a net increase, not a presence, measured per commit and per file.** For each
   changed markdown file in each commit of the range, the rule counts each banned codepoint in that
   commit's `OldText` and `NewText`, and warns only where the new count exceeds the old. The
   granularity is stated because it is observable: a branch that adds a glyph in one commit and
   removes it in a later one still warns on the first, and a file whose count rises across three
   commits warns three times. That matches every other rule in `evaluate`, which reports per commit,
   and it is the honest unit for a rule whose subject is what an author just wrote. Merge commits
   carry no changes and so are never counted twice.

   Grandfathering is therefore emergent rather than configured: a newly added file has an
   empty `OldText`, so every banned codepoint in it is new and is flagged; an edit that leaves a
   legacy file's existing glyphs untouched is silent, even when the file carries hundreds. There is
   no path allowlist, no cutoff date, and no exemption list to maintain. The 2344 authored ADR
   em-dashes and the 4347 in plans stay silent forever unless someone adds more.

3. **Scope is authored, non-generated markdown under the docs directory.** A changed `.md` file
   whose path lies under the configured `docsDir` qualifies. One prefix check suffices: `layout()`
   derives both `ADRDir` and `PlansDir` from `docsDir` (`internal/project/layout.go`), so ADRs,
   plans, and hand-written docs are all already beneath it, and enumerating the three separately
   would be redundant. `audit.Inputs` does not currently carry `docsDir`, so the rule's one piece of
   new plumbing is threading it through alongside the existing `ADRDir`/`PlansDir` fields.

   Any path in `Inputs.GeneratedPaths` is skipped: rendered output is not authored, its glyphs are
   its source's fault, and ADR-0115's gate already covers the shipped sources. That skip is
   load-bearing and deserves stating, because `GeneratedPaths` is built from the lock's file list
   (`internal/project/project.go`), not from a path convention: `ACTIVE.md`, `docs/decisions/README.md`,
   `docs/plans/README.md` and every rendered doc are entries in `.awf/awf.lock`, which is precisely
   what stops a commit proposing a new ADR from warning about the em-dash rows its own `ACTIVE.md`
   regeneration adds. A deleted file is skipped.

4. **The rule ships to adopters, enabled by default,** via `audit.plainPunctuation` (bool, default
   `true`), matching the shape of every other advisory knob (`audit.domainDocStaleness`,
   `audit.domainCodeStaleness`, `audit.undocumentedDomain`, `audit.uncommittedChanges`). An adopter
   who disagrees sets it to `false`, and an adopter who wants different prose guidance overrides the
   documentation standard's section with a convention part.

5. **This reverses ADR-0113's house-style disclaimer, deliberately and on the record.** ADR-0113
   said awf "enforces its own shipped prose, not an adopter's house style", and used that to
   exclude adopter content. ADR-0115 kept that boundary for its *gate*, which is why it warns
   rather than rewrites an adopter's ADR title.

   This ADR draws the line differently for *advice*, and the honest framing is that it **adds** an
   opinion rather than enforcing one awf already held. The shipped documentation standard currently
   scopes its punctuation rule to "shipped prose", and ADR-0115 item 9 widens only that rule's
   codepoint list, not its reach, so nothing awf ships today tells an adopter how to punctuate
   their own ADRs. Warning them against an unstated rule would be the same defect ADR-0113 item 4
   was written to prevent. Decision item 8 therefore widens the standard first; this rule then
   enforces what the standard states, and the ordering is the whole argument.

   The distinction that keeps this coherent with ADR-0115 item 6: awf never mutates adopter content
   and never fails an adopter's build over house style, but it does state the convention in a
   standard they render, and tell them once, in a warning they can switch off.

6. **Advisory severity is the depiction escape hatch.** ADR-0115 Decision item 7 keeps a
   no-escape-hatch posture for the gate, which it can afford because its scope boundary leaves
   sidecar data free to depict a glyph. This rule has no such boundary to hide behind: a doc that
   documents the em-dash will trip it. That is acceptable precisely because the finding is a
   `Warning`. A warning an author reads and knowingly ignores is the escape hatch, and it costs no
   exemption list, no marker comment, and no config surface.

7. **The rule flags new ADRs and plans, and that is the point.** ADR-0113 Decision item 2 had
   promised that hand-authored ADRs and plans "keep em-dashes freely". That promise is withdrawn
   here for *new* prose only, which is the entire reason the rule exists.

8. **The documentation authoring standard widens from shipped prose to all awf-managed prose.**
   The plain-punctuation rule in `templates/docs/doc-standard.md.tmpl` scopes itself to "shipped
   prose". Its scope clause widens to cover authored prose as well, so an adopter reads the
   convention in a standard they render before any warning cites it.

   **This item amends ADR-0115 item 9's rendering of that line and depends on ADR-0115 landing
   first.** The two are complementary and strictly ordered, not conflicting: item 9 widens the
   rule's *codepoint list* from one to seven, and this item then widens the *scope clause* of the
   text item 9 leaves behind. The baseline to edit is therefore the post-ADR-0115 line, not the
   line as it reads today. If the order ever reversed, an implementer following item 9 literally
   would restore "Shipped prose" and silently undo this widening; if both land in one effort, the
   line is written once, carrying both, under a single changelog entry.

   Two constraints bind the replacement text, and both come from guards over `templates.FS`. It
   must not type any banned glyph, because ADR-0115's gate scans that template. It also must not
   cite an ADR number, because the source-level residue guard fails any shipped template carrying a
   concrete `ADR-NNNN` citation: this rule is therefore stated in the standard without attribution,
   which is why the line cites nothing today.

## Invariants

- `` `invariant: audit-plain-punctuation` ``: with `audit.plainPunctuation` enabled, `awf audit`
  emits a `Warning` for each commit in which a non-generated markdown file under `docsDir` has a
  rising banned-codepoint count, and emits nothing when the count is unchanged or falls, when the
  path is generated, when the file lies outside `docsDir`, or when the knob is `false`. Backed by
  `TestRulePlainPunctuation` in `internal/audit/audit_test.go`.

## Consequences

- **The rule that motivated the effort finally reaches the prose that motivated it.** Every ADR and
  plan an agent writes from now on is checked, without touching a line of settled rationale.
- **Flip-commit obligations.** The commit flipping this ADR to `Implemented` ships the rule and its
  backing test, regenerates `docs/decisions/ACTIVE.md` for the status change, regenerates both
  config-reference docs for the new key, and carries a changelog entry: a default-on rule and a new
  config key are adopter-facing. The documentation-standard re-render (`docs/doc-standard.md` and
  `examples/sundial`'s copy) belongs to this commit **only when ADR-0115 landed in a separate
  effort**. When both land in one effort, Decision item 8 has already folded that line into a single
  edit under a single changelog entry, so it re-renders in the ADR-0115 flip commit and this commit
  does not touch the template at all. Reading this bullet as unconditional would produce exactly the
  second edit item 8 forbids. `cmd/repoaudit`'s
  changelog rule would flag an unchanged `[Unreleased]` section, though only as a `warning` under
  the ADR-0107 downgrade this ADR takes its severity precedent from, so the obligation is this
  ADR's rather than the tooling's.
- **The backlog stays silent without a carve-out.** Net-increase semantics mean the 6691 existing
  occurrences never warn. "Lands green" is a claim about noise, not about exit codes: `awf audit`
  never exits non-zero on a warning, and `./x gate` does not run it at all (it is a separate `./x
  audit` target), so a `Warning` could never have broken a build. The property that matters is that
  the rule does not flood an author with findings about prose they did not write, which is what
  makes shipping it default-on to adopters safe: an adopter's first `awf audit` after upgrading
  warns about nothing they have already written.
- **A net-zero swap slips through.** Replacing one em-dashed sentence with a different em-dashed
  sentence leaves the count unchanged and is not flagged. Accepted: the rule is advisory, and
  precise hunk attribution would cost diff parsing for a warning.
- **Adopters inherit an opinion they must actively decline.** Default-`true` means a project that
  likes em-dashes sees warnings until it sets `audit.plainPunctuation: false`. That is the cost of
  Decision item 5, and it is the same bargain every other default-on advisory rule already makes.
- **A doc that depicts a banned glyph warns, forever.** There is no way to silence it short of
  disabling the rule. Decision item 6 accepts this rather than building an exemption mechanism for
  a case measured at one file.
- **The config key costs five coordinated touchpoints, plus one for the plumbing.** Traced against
  `audit.undocumentedDomain`: an `internal/config` `AuditConfig` field, the
  `internal/audit/settings.go` `Resolve` default, an `internal/configspec` descriptor, the
  `internal/project/configreference.go` live-state case, and a regenerated `docs/config-reference.md`
  together with `examples/sundial/docs/config-reference.md`. Missing any one trips the
  closed-config-tree checks (ADR-0086) or leaves the reference stale. There is no separate
  "promote the setting into `Inputs`" step: `Inputs` *embeds* `Settings` and the audit inputs are
  constructed with `Settings: s` unconditionally, so a new knob is readable as
  `in.PlainPunctuation` for free; only the knob list in the `Inputs` doc comment needs refreshing.
  The descriptor list is not alphabetical but a declaration order mirrored across all four files,
  and the generated reference's row order derives from it, so the new key goes in at the same
  relative position in each.
- **The rule itself costs a separate group of edits, which is not the key's cost.** Threading
  `DocsDir` into `audit.Inputs` and populating it where the inputs are built (Decision item 3),
  registering the rule in `evaluate`, the rule function, its backing test, and the `Resolve`
  override branch's coverage. That last one is not optional: the 100% coverage gate (ADR-0012)
  fails if the new knob's override branch is uncovered, and the existing settings tests enumerate
  every toggle, so a default-on knob joins those assertions.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Flag presence, not net increase | Simpler to implement, but touching any legacy ADR or plan would flood the author with warnings about prose they did not write, which trains them to ignore the rule or switch it off. Net-increase makes grandfathering emergent instead of configured. |
| Flag only newly added files (`Action == Added`) | Grandfathers just as cleanly and is simpler still, but it lets an author add an em-dash to an existing doc without a word, which is the most likely way new glyphs enter a mature repository. |
| Put the rule in `cmd/repoaudit` (repo-local) | Honours ADR-0113's house-style boundary exactly, needs no config key, and drops both the five config touchpoints and the `docsDir` plumbing. Rejected because Decision item 8 makes awf ship the documentation standard that states this rule; shipping that standard without the check is the inconsistency worth fixing, not the other way round. |
| Make it an `Error` | Would give the rule teeth, but it is a style opinion awf cannot mechanically prove is right, and a doc legitimately depicting a glyph would be unfixable. ADR-0107's downgrade of the changelog rule is the governing precedent. |
| Default the knob to `false` | Respects adopter autonomy maximally, but a default-off check is a check nobody runs, and it would leave the shipped documentation standard stating a rule nothing enforces, which is the exact gap ADR-0113 left. |
| Extend the scope to `.awf/` sidecar data and convention parts | Would close ADR-0115's stated sidecar blind spot, but `FileChange` loads text only for `.md`, so YAML sidecars need new plumbing, and ADR-0115 Decision item 7 depends on sidecar data staying free to depict a glyph. Deferred as a separate question. |
