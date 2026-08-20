---
format: current-state-v4
slug: pitfalls-as-the-current-tag-vocabulary-carrier
status: Implementing
date: 2026-08-20
---
# ADR-pitfalls-as-the-current-tag-vocabulary-carrier: Pitfalls as the Current Tag Vocabulary Carrier


## Context

The governed tag vocabulary was introduced as a relevance currency for ADR and pitfall context
surfacing. The current system no longer retrieves artifacts by tag. Its live tag consumers are the
vocabulary and domain checks, the coverage and frequency advisories, and the pitfall index and leaf
renderers. Governed current-state ADR formats cannot carry tags, while older ADR frontmatter still
parses `tags:` and `related:` for historical compatibility.

A complete census at this boundary found 98 configured members. Thirty-seven label current authored
pitfalls, sixty occur only in append-only legacy ADR frontmatter, and `topic-navigation` occurs on no
current carrier. The thirty-seven pitfall-backed members were then assessed individually rather than
retained by count. Each accurately labels current actionable pitfall knowledge and is projected into
both the generated pitfall index and leaf, while vocabulary validation protects each label. No
pitfall-backed member describes a retired concern, so all thirty-seven have a demonstrated current
display and validation consumer. The sixty ADR-only members have no current retrieval, display, or
ownership consumer; validating their historical occurrence against the same vocabulary is circular
and makes settled history dictate current vocabulary growth.

ADR-0103 item 4 deliberately validated tags on both ADRs and pitfalls. ADR-0109 item 4 later made
both artifact kinds the population for coverage and frequency advisories. The current
`tag-vocabulary-governed`, `tag-coverage-note`, and `tag-frequency-note` claims preserve those
contracts, so narrowing them is a durable policy decision. It must not erase append-only ADR
frontmatter, remove the legacy parser, disturb `related:` behavior, or become compatibility cleanup
reserved for RF-008B and RF-014B.

The owner settled the conflict as follows, verbatim:

~~~text
Owner disposition: choose B. The approved RF-012 boundary already requires removal of tags whose only role is classifying historical artifacts or retired mechanisms; treating circular generic validation as a sufficient live consumer would not satisfy that outcome. Author a successor ADR within RF-012 that preserves legacy ADR frontmatter bytes and parsing as append-only/history compatibility, but retires legacy ADR tags from current vocabulary membership validation and coverage/frequency advisories. Supersede only ADR-0103 item 4 and its current-state claims to the extent necessary; keep `related:` behavior and all unrelated parsing intact. Make pitfalls the current tag carrier only where each retained tag has a demonstrated retrieval, validation, display, or ownership use. Do not automatically retain all 37: census each pitfall-backed member and remove any lacking such a real consumer, with no arbitrary target count. Remove `topic-navigation` and every ADR-only/retired live vocabulary member from current config and generated reference; do not rewrite frozen ADR history. Interpret external `removed tags have no references` under repository precedence as no live/config/current-metadata reference; explicitly report frozen historical occurrences retained. Close the known tag-coverage issue and correct its claim in the same governed transaction. Preserve this ruling verbatim in the effort decision log and every ADR/review brief. Continue through ADR/plan/implementation assurance, but do not number, terminally close, integrate, remove topology, finish, or start another issue. Stop integration-ready or on a further material choice outside this direction.
~~~

## Decision

1. `decision: pitfalls-are-current-tag-carriers` Govern the configured tag vocabulary against
   authored pitfall metadata only. This replaces only ADR-0103 item 4's legacy-ADR membership arm
   and refines ADR-0109 item 4's coverage and frequency populations from legacy ADRs plus pitfalls
   to pitfalls alone. A non-empty vocabulary validates pitfall membership and meanings, and its
   coverage and frequency advisories evaluate pitfalls only. Legacy ADR tags remain accepted and
   parsed historical metadata but neither require current vocabulary membership nor contribute to
   current tag-health advisories. Preserve ADR-0109's frequency threshold, advisory rank, empty-
   vocabulary behavior, and all unrelated decisions. ADR `related:` parsing and validation remain
   unchanged.
2. `decision: vocabulary-requires-current-consumer` Retain a self-hosted vocabulary member only when
   a current pitfall carrier demonstrates a retrieval, validation, display, or ownership use for an
   accurate current concern. Remove members supported only by historical ADR classification, retired
   mechanisms, circular validation, or no carrier. The vocabulary has no target count.
3. `decision: historical-tags-remain-append-only` Preserve legacy ADR bytes, including removed tag
   names, as append-only decision history. A historical occurrence is not a live vocabulary
   reference and does not authorize rewriting the record or deleting compatibility parsing.

## State changes

- update `config/configuration:tag-vocabulary-governed`
- update `config/configuration:tag-coverage-note`
- update `config/configuration:tag-frequency-note`

## Consequences

The current vocabulary becomes a compact index over actionable pitfall knowledge instead of a mirror
of historical ADR topics. The thirty-seven individually justified pitfall tags remain; sixty
ADR-only members and the unused `topic-navigation` member leave live configuration. Generated config
reference data follows the live set without a hand-maintained target count.

Legacy ADR frontmatter continues to expose historical tag names to parsers and audit history, but
those names no longer fail current checks when absent from configuration. Reports must distinguish
those frozen occurrences from live config and current metadata references. This accepts that legacy
ADR tag typos are no longer current drift; append-only history and the lack of a current ADR tag
carrier make that the smaller cost.

Pitfall tags keep closed-set membership validation, the tag-versus-domain guard, health advisories,
and visible generated output. The stale claim that every ADR can receive a coverage advisory is
corrected to pitfalls, which closes the corresponding known issue. No ADR bytes, pitfall metadata,
parser compatibility, `related:` behavior, architecture boundary, or adopter repository changes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Remove only the one unused member | Circular validation would preserve sixty historical-only members and fail RF-012's approved outcome. |
| Delete or retag legacy ADR frontmatter | It would rewrite append-only decision history merely to erase old tag text. |
| Stop parsing legacy ADR tags | Parser removal is unnecessary for current governance and belongs behind the separate managed compatibility gate. |

## Status history

- 2026-08-20: Proposed
- 2026-08-20: Accepted; content-sha256: 998599ee522c66318072600530f1ba8137942b3d30019302634c1b7aa65f7f6c
- 2026-08-20: Implementing; content-sha256: 998599ee522c66318072600530f1ba8137942b3d30019302634c1b7aa65f7f6c
- 2026-08-20: Applied; operations: update `config/configuration:tag-vocabulary-governed`, update `config/configuration:tag-coverage-note`, update `config/configuration:tag-frequency-note`
