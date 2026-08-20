---
format: current-state-v4
slug: separate-blocking-checks-from-advisories
status: Proposed
date: 2026-08-20
---
# ADR-separate-blocking-checks-from-advisories: Separate blocking checks from advisories


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

The current blocking families protect concrete properties:

| Family | Protected property |
|---|---|
| Configuration, lock, sidecar, record, and reference validation | Correctness and authority |
| Generated and staged drift, tracked membership, and residue | Reproducibility |
| Current-state handshakes, transitions, coverage, and declared policy | Authority |
| Binary and schema compatibility, build, test, coverage, and verification availability | Correctness and reproducibility |
| Confined writes, lifecycle ownership, memory citations, stale merge authorization, and dependency pins | Safety |
| Defect-oriented static analysis and unreachable production code | Correctness and required verification |

The non-blocking families do not establish those properties. Punctuation, glossary length, tag
health, plan-detail prompts, topic fan-out, guide size, and preference-oriented static analysis are
judgement or review signals. Unused vocabulary, unset inputs, stub content, marker suggestions,
context expansion, tracking-unavailable notices, and non-blocking compatibility notices are optional
information. Existing audit Warning findings and successful informational command notes already
have the intended zero-exit behavior.

## Decision

1. `decision: errors-protect-validity` A blocking Error must protect a named correctness, safety,
   authority, or reproducibility property. Invalid or corrupt inputs, broken declared references,
   generated or staged drift, unsafe or ambiguous writes, active authority violations, incompatible
   binary or schema state, verification loss, confinement or ownership violations, and destructive
   lifecycle failures remain blocking.

2. `decision: judgement-warns` Style and heuristic judgements are Warning findings or equivalent
   visible zero-exit advisories. This includes prose punctuation, glossary length, tag distribution,
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

5. `decision: aggregate-remains-actionable` Aggregate and direct check commands may keep their
   existing composition, ordering, and serious-failure behavior. Output labels each severity class,
   retains the source and remediation needed to act, and tests cover error-only, warning-only,
   information-only, and mixed exit behavior.

## State changes

- add `tooling/cli:check-severity-by-protected-property`
- add `tooling/quality-gates:gate-severity-by-protected-property`
- update `tooling/cli:repo-check-capability-plan`
- update `tooling/cli:terseness-advisory-nonfailing`
- update `tooling/audit-commands:severity-single-spelling`
- update `tooling/quality-gates:prose-gate-tracked-file-scan`

## Consequences

A valid change no longer fails solely because of prose style, unused vocabulary, or a heuristic
preference. Serious invalidity, authority breaches, unsafe behavior, and lost reproducibility keep
their nonzero exits. Warning and information output becomes distinguishable without changing the
shared rank type or adding configuration.

The repository gate must separate defect-oriented static analysis from advisory style and heuristic
analysis while preserving deterministic order and visibility. Mixed analyzers are advisory unless
the configured rule is demonstrably defect-oriented; this avoids letting a preference-bearing
analyzer block valid work at the cost of moving some possible-defect hints into review output.

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
