---
format: current-state-v2
status: Proposed
date: 2026-07-22
---
# ADR-0147: Topic-Grouped Context Output and Selectors-Only Rendered Applicability

## Context

ADR-0144 (Implemented 2026-07-21) established the agent-oriented context projection: per effective path, `awf context` reports classification, owning domains, applicable topics, directly marked claims, and drilldowns, with `--full` emitting the complete authority packet per path. Measurement against real usage on 2026-07-22 (HEAD 5eda8646) found the presentation defeats its consumers:

- Multi-path output repeats each topic's entire authority block once per effective path. `awf context --full internal/migrate` (30 files, one topic) emitted 310 KB, the same 17-claim authority block 30 times. `awf context --full --range HEAD~5..HEAD` emitted 477 KB. The `awf-reviewing-impl` and `awf-reviewing-plan` skills instruct the dispatcher to paste exactly that invocation's output into reviewer subagent prompts, which is unusable at real diff sizes.
- The `Matched paths:` census renders unconditionally in every topic block (a single line of roughly 3.5 KB for a 93-file topic), and the rendered `docs/topics/*.md` Applicability paragraph bakes the full census plus every marker site into committed docs (6,868 of `docs/topics/tooling/cli.md`'s 21,509 bytes). Every file added to a matched package rewrites every affected topic doc, a recurring drift-churn class with no reader benefit.
- Full mode prints both the concise omission line ("N topic-wide claim(s) omitted; drill down") and the full authority list that follows it, a visible contradiction.
- Concise context surfaces zero claims for most production files, because proof markers overwhelmingly live in test files; the block carries only a bare omitted count, so an agent cannot judge whether the drilldown is worth the tokens.
- An `eligible-unowned` path prints bare, with the remediation hint available only in `--uncovered`; the intent of `--json` (whose concise serialization is larger than the full text projection) is documented nowhere.

The applicability evidence model has exactly three consumers: the context projection (`internal/project/context_projection.go`), `awf topic --coverage` (`internal/topic/query.go`), and the topic-doc renderer (`internal/topic/render.go`). No production code consumes `MatchedPaths` elsewhere, no external consumer parses context JSON, and no current-state claim pins the JSON schema or the rendered Applicability census. `ContextResult` additionally carries never-serialized invocation-level aggregate fields with no non-test consumer. ADR-0144 is frozen, so its presentation model is revised here rather than amended.

## Decision

1. `awf context` output is grouped by topic. Every projection (concise and full), universe (working, staged, range), and serialization assembles one topic entry per applicable topic per invocation, ordered by topic ID, in a `## Topics` section preceding `## Effective paths`. A topic's authority renders exactly once per invocation regardless of how many effective paths select it. This is presentation grouping, never truncation: `--full` still emits every applicable current claim.

2. Each `## Topics` entry carries: the topic ID and title; the owning-domain selectors and topic selectors with the both-must-match statement; the matched-path count with a drilldown pointer to `awf topic <id> --coverage` in place of the census; and the complete roster of the topic's current claim IDs as a compact list, uncapped. In the concise projection the entry additionally renders the full detail (prose, backing, `Verify:` text, marker sites) of the union of the invocation's directly marker-selected claims, deduplicated across paths, and retains an explicit detail-omission line stating how many of the rostered claims have no detail rendered, with the topic drilldown command; the line is absent when every rostered claim's detail is shown. In the full projection the entry renders the full detail of every current claim plus pending operations, once, and no omission line appears alongside a full authority block. Uncapped rosters are an accepted cost: a topic over the claim budget produces a long roster, and the remedy is splitting the topic, not display truncation.

3. `## Effective paths` entries shrink to attribution: the path, its classification, its requests, its owning domains, the IDs of its applicable topics, and the IDs of its own directly marker-selected claims. ADR-navigation blocks (lifecycle-derived operation progress) and known-artifact navigation blocks remain per-path, outside the topic group. An `eligible-unowned` path gains one remediation line pointing at domain configuration and `awf context --uncovered`. Per-path attribution lists are sorted for determinism; a path selected by topics in several domains attributes each by its domain-qualified topic ID.

4. `--json` serializes the same grouped semantic model, preserving text/JSON parity. The existing never-serialized invocation-level aggregate fields on `ContextResult` are replaced by the grouped model rather than kept alongside it. Documentation states the intent of `--json`: machine consumption by tooling; agents reading output prefer the text projection. The JSON shape change ships without a compatibility bridge, consistent with the pre-1.0 posture and the absence of consumers.

5. The rendered topic document's Applicability paragraph becomes selectors-only: the owning-domain selectors, the topic selectors, the both-must-match statement, and a pointer to `awf topic <id> --coverage` for live matched paths and marker sites. No census, no marker sites, no count. The global-topic variant sentence is retained, and the wording degrades to coherent generic prose when selector lists are empty. `awf topic --coverage` remains the census's home, unchanged, reporting sorted matched paths and marker sites. The new claim covering this rendered form is a test-backed invariant, proven by an annotation on an `internal/topic` render test.

6. Relative to ADR-0144's presentation model, this decision revises: the per-path reporting rule (point 1), which now applies to attribution while authority is unioned per invocation into the Topics section; the full-packet-per-path shape (point 10), which becomes full-packet-per-topic-once with the never-truncated guarantee intact; and the applicability presentation (point 14), where context reports a count with a coverage drilldown and rendered docs report selectors only, while `--coverage` keeps the concrete evidence. Path classification semantics, coverage eligibility, read-only operation, and the non-recursive semantic model are unchanged.

7. Documentation travels with the change: the `working-with-awf` command reference and its shipped template default, the agent-guide commands section, the tooling domain narrative, the glossary entries that define the concise projection by omitted counts and the full packet by matched paths, and the `awf context` help text. Every lifecycle transition of this ADR runs `./x sync` so `docs/decisions/INDEX.md` and all re-rendered topic docs land in the same commit as the status change.

## State changes

- update `tooling/cli:context-full-authority-packet`
- update `tooling/cli:context-default-excludes-history`
- update `tooling/cli:context-path-attribution`
- update `tooling/cli:context-applicability-navigation`
- add `invariants/topics-and-markers:rendered-applicability-selectors-only`

## Consequences

- The flagship consumers become viable: pasting `awf context --full` over a multi-file diff into a reviewer prompt costs one authority block per topic instead of one per file, collapsing the measured 310 KB directory case to roughly the size of its topic drilldown.
- The rendered-doc churn class disappears: adding a file to a matched package no longer rewrites topic docs, because no rendered artifact carries the census. The cost is that a doc reader must run `awf topic <id> --coverage` for concrete evidence; the docs keep the stable selectors. The switch itself causes a one-time re-render of every topic doc in this repository (and in `examples/sundial`), and every adopter's topic docs report drift after upgrading until they sync, standard template-change behavior guarded by the lock's `awfVersion` gate.
- Concise output becomes claim-visible: agents see every claim ID and the full detail of exactly the marker-selected claims, and can price the drilldown before paying it. The dominant bulk of concise output shifts from the census to claim-ID rosters; for topics over the claim budget the roster is long by design, and the pressure lands where it belongs, on splitting overgrown topics.
- The JSON schema changes shape once, unannounced, which is acceptable pre-1.0 with no known consumers; anything that later parses context JSON binds to the grouped model.
- Tests pinning the current shape must move with the model, including the context projection, path attribution, parity, and applicability tests, the topic-doc render tests, and the managed-caller projection guard; the four revised claims and the new claim are re-authored at implementation time under the V2 batch rules: each update preserves the claim's `Origin` and existing `Revised-by` entries, appends this ADR to `Revised-by`, changes a canonical field, and keeps its current backing mode (`context-default-excludes-history` stays unbacked with a `Verify:` line and no proof marker); the add lands with this ADR as `Origin`, each operation paired with its Applied event.
- ADR-navigation and artifact navigation staying per-path preserves the ADR-path projection unchanged.
- Deferred follow-ups this decision does not cover: implementer-side grounding guidance in the workflow skills, and topic hygiene for over-budget topics.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| First-occurrence printing (authority under the first path that touches a topic, later paths reference it) | Ordering-dependent, awkward to cite, and leaves single-topic multi-path output shaped differently from multi-topic output. |
| Trailing authority section after the path blocks | Buries the payload agents came for beneath the attribution list. |
| Per-path direct-claim detail retained | Reintroduces repetition whenever several paths share a marker-selected claim, the exact defect being fixed. |
| Capped claim-ID roster with an explicit remainder | Introduces a display constant and a silent-feeling boundary; uncapped keeps output deterministic and pushes size pressure onto topic splitting. |
| Rendered docs keep a matched-path count | A live count still rewrites affected topic docs on every file add, preserving the churn class for one integer. |
| Rendered docs keep the census, drop only marker sites | Halves the bloat but keeps most churn and keeps stale-by-next-commit evidence in committed docs. |
| Amending ADR-0144 | ADR-0144 is Implemented and frozen; a frozen record's meaning is never edited. |

## Status history

- 2026-07-22: Proposed
