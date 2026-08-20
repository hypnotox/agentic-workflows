---
format: current-state-v4
slug: separate-blocking-checks-from-advisories
status: Implemented
date: 2026-08-20
---
# ADR-0295: Separate blocking checks from advisories


## Context

`awf check` and the repository gate currently mix objective failures with style and heuristic
judgements. A tracked punctuation finding fails `awf check`, although correcting the prose is a
matter of editorial judgement. Unused render variables and data are emitted as drift, so their
presence also fails. At the same time, every unranked project note is presented under `warnings`,
even when the note is only optional information such as an unset variable, a stub section, or a
non-blocking compatibility notice.

The existing model already has useful boundaries. Produced findings share exactly two fixed ranks,
`error` and `warn`, and only an Error makes a completed report fail. Several command results also
carry unranked notes. ADR-0183 deliberately removed configurable severity and a third finding rank.
This decision preserves that model and gives the unranked notes an honest presentation category
rather than recreating rank configuration.

The inventory below names every current check and advisory by the property it protects and its exit
behavior. Entries classified Information are unranked notes, not a third finding rank.

| `awf check` check or advisory | Property | Class and exit |
|---|---|---|
| Configuration, lock, sidecar, ADR, plan, topic, pitfall, glossary, and frontmatter parsing | Valid input and authority | Error, nonzero |
| Generated output membership, missing, unsynced, orphaned, stale, hand-edited, invalid-frontmatter, stale backup, config residue, and staged drift | Reproducibility | Error, nonzero |
| Broken ADR, plan, pitfall, glossary, Markdown, skill, domain, tag, and current-state references | Correctness and authority | Error, nonzero |
| Current-state handshake, transition, static coverage, claim provenance, plan structure, and a pending ADR on the integration branch | Authority and ADR lifecycle consistency | Error, nonzero |
| Memory citations in durable decisions, plans, and commit messages | Safety and authority lifetime | Error, nonzero |
| Tracked-file punctuation restraint | Prose style | Warning, zero |
| Topic fan-out and Proposed-plan assignment or detail prompts | Potential cohesion and plan-detail quality | Warning, zero |
| Glossary length, tag frequency, untagged artifacts, and generated guide size | Heuristic readability and retrieval quality | Warning, zero |
| Unused render variables and sidecar data | Optional vocabulary cleanup | Information, zero |
| Unset variables, stub content, marker-shaped part lines, and unknown planned commit scope | Optional improvement | Information, zero |
| Generated-artifact tracking unavailable, staged universe unavailable outside Git, older-format provisional introductions, and binary ahead of the rendered lock | Non-blocking verification or compatibility notice | Information, zero |
| Scanner, index, Git, and project preparation failures | Verification unavailable | Error, nonzero |

| Audit or direct policy rule | Property | Class and exit |
|---|---|---|
| Commit subject shape, type, scope at commit time, and length | Declared commit authority | Error, nonzero |
| ADR status and index cochange, current-state transition replay, and stale merge authorization | Authority consistency | Error, nonzero |
| Uncommitted audit state, commit identity or signature policy, and audit infrastructure failure | Reproducible or available verification | Error, nonzero |
| Dependency change without an ADR, large change without a plan, historical ADR frontmatter, non-context historical transition-load failure, punctuation increase, changelog heuristic, and added coverage-ignore review prompt | Review judgement, stale or unavailable non-authoritative history, style, or coverage-change review | Warning, zero |
| Disabled commit policy, empty audit range, and successful policy or audit notices | Operation state | Information, zero |
| Mutation report missing, malformed, or timed out | Required verification unavailable | Error, nonzero |
| Surviving mutants and a successful mutation run | Optional test-strength improvement or operation note | Information, zero |

| Repository gate check | Property | Class and exit |
|---|---|---|
| Version and schema authority | Compatible reproducible execution | Error, nonzero |
| Selected Go tests, Pi runtime smoke, 100 percent coverage, and valid coverage-ignore reasons | Required verification | Error, nonzero |
| `go vet`, released-platform builds, defect-oriented lint, production dead code, and workflow pin checks | Correctness, reachability, and supply-chain reproducibility | Error, nonzero |
| Style, wording, formatting, preferred idiom, speculative performance, and maintainability lint | Heuristic quality | Warning, zero |
| Context-spill residue and inspection failure | Optional context cleanup | Information, zero |
| Skipped-suite, successful render, migration, and gate notes | Operation state | Information, zero |

The enabled lint rules are classified at their configured rule boundary. `govet`; Staticcheck `SA0*`
through `SA5*` and `SA9*`; `errcheck`; `ineffassign`; `nilerr`; `bodyclose`; `errorlint`;
`durationcheck`; `asasalint`;
`nilnesserr`; `gocheckcompilerdirectives`; `makezero`; `exhaustive`; and `wastedassign` are blocking
because they report a concrete defect, lost result, resource error, impossible value flow, invalid
directive, incomplete closed case set, or ineffective assignment. Staticcheck `SA6*`, `S*`, `ST*`,
and `QF*`; `nilnil`; `unused`; `unconvert`; `unparam`; `predeclared`; `gocritic`; `dupword`;
`perfsprint`; `intrange`; `usestdlibvars`; `usetesting`; `misspell`; `revive`; `whitespace`;
`gofmt`; and `goimports` warn because their configured rule sets include preference, style,
formatting, possible package cohesion, or heuristic maintainability judgements. A linter execution
or configuration failure is an Error. Every newly enabled lint rule requires the same concrete
protected-property classification before it enters either lane.

## Decision

1. `decision: errors-protect-validity` A blocking Error must protect a named correctness, safety,
   authority, or reproducibility property. Invalid or corrupt inputs, broken declared references,
   generated or staged drift, unsafe or ambiguous writes, active authority violations, incompatible
   binary or schema state, verification loss, confinement or ownership violations, and destructive
   lifecycle failures remain blocking.

2. `decision: judgement-warns` Style and heuristic judgements use the existing Warning rank.
   This includes prose punctuation, glossary length, tag distribution,
   plan-detail quality, topic fan-out, guide size, and static-analysis rules that express preference,
   maintainability opportunity, wording, formatting, or speculative performance rather than a
   concrete defect. Operational inability to run their check remains an Error.

3. `decision: optional-notes-inform` Optional improvements, unused vocabulary, unset inputs, stub
   content, context suggestions, non-blocking compatibility notices, and successful operation notes
   remain unranked. Readable output presents them under a distinct `information` category, separate
   from ranked `warnings`, and they never change exit status.

4. `decision: fixed-ranks-preserved` The shared finding rank remains exactly Error and Warn. Severity
   is not configurable, information is not a third rank, and check aggregation is not decomposed by
   this decision.

5. `decision: aggregate-remains-actionable` Direct and aggregate outputs preserve serious-failure
   behavior and visibly separate Error, Warning, and unranked Information. They exit nonzero exactly
   when an Error is present.

## State changes

- add `tooling/cli:check-severity-by-protected-property`
- add `tooling/quality-gates:gate-severity-by-protected-property`
- update `tooling/cli:repo-check-capability-plan`
- update `tooling/cli:terseness-advisory-nonfailing`
- update `tooling/audit-commands:severity-single-spelling`
- update `tooling/quality-gates:prose-gate-tracked-file-scan`
- update `rendering/inplace-and-placeholders:unused-var-drift`
- update `rendering/inplace-and-placeholders:unused-data-drift`
- update `rendering/doc-outputs:glossary-terseness-advisory`
- update `config/configuration:tag-coverage-note`
- update `config/configuration:tag-frequency-note`
- update `adr-system/plan-artifacts:plan-v2-assignment-advisories`
- update `rendering/sync-and-drift:agent-guide-size-advisory`

## Consequences

A valid change no longer fails solely because of prose style, unused vocabulary, or a heuristic
preference. Serious invalidity, authority breaches, unsafe behavior, and lost reproducibility keep
their nonzero exits. Warning and information output becomes distinguishable without changing the
shared rank type or adding configuration.

The repository gate must separate defect-oriented static analysis from advisory style and heuristic
analysis while preserving deterministic order and visibility. Classifying mixed analyzer rule sets
as advisory avoids letting preference-bearing rules block valid work, at the cost of moving some
possible-defect hints into review output.

Existing byte-exact output tests and current-state claims must change. Generated documentation moves
with its authoring sources. RF-004 remains responsible for later checker ownership and aggregation
decomposition.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add an Information finding rank | Contradicts the approved boundary and makes optional notes part of a rank model that already represents every produced finding. |
| Make severity configurable per check | Reopens the configuration surface ADR-0183 removed and lets adopters suppress protected properties. |
| Keep all lint findings blocking | Allows style, preferred idiom, and heuristic maintainability findings to fail otherwise valid work. |
| Decompose checkers while classifying them | Expands this policy correction into RF-004's separate ownership and architecture work. |

## Status history

- 2026-08-20: Proposed
- 2026-08-20: Accepted; content-sha256: 1908a27d2ea0e97c534adbd67a376b8be22a43d1e4c2ea50da9d7979aedbc721
- 2026-08-20: Implementing; content-sha256: 1908a27d2ea0e97c534adbd67a376b8be22a43d1e4c2ea50da9d7979aedbc721
- 2026-08-20: Applied; operations: add `tooling/cli:check-severity-by-protected-property`, add `tooling/quality-gates:gate-severity-by-protected-property`, update `tooling/cli:repo-check-capability-plan`, update `tooling/cli:terseness-advisory-nonfailing`, update `tooling/audit-commands:severity-single-spelling`, update `tooling/quality-gates:prose-gate-tracked-file-scan`, update `rendering/inplace-and-placeholders:unused-var-drift`, update `rendering/inplace-and-placeholders:unused-data-drift`, update `rendering/doc-outputs:glossary-terseness-advisory`, update `config/configuration:tag-coverage-note`, update `config/configuration:tag-frequency-note`, update `adr-system/plan-artifacts:plan-v2-assignment-advisories`, update `rendering/sync-and-drift:agent-guide-size-advisory`
- 2026-08-20: Implemented; content-sha256: 1908a27d2ea0e97c534adbd67a376b8be22a43d1e4c2ea50da9d7979aedbc721
