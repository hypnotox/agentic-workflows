---
format: current-state-v4
slug: replace-plain-punctuation-ban-with-punctuation-restraint
status: Implementing
date: 2026-08-20
---
# ADR-0290: Replace plain punctuation ban with punctuation restraint

## Context

The prose gate currently treats seven typographic codepoints as substitutes for ordinary punctuation
and rejects every occurrence outside a configured exemption. A separate test prohibits the same
codepoints in embedded templates, the changelog, and production Go string literals, while the audit
command warns when a documentation commit raises any of their counts. Together these mechanisms ban
all em dashes, en dashes, ellipses, and curly quotes.

That policy is broader than the protected property. Ordinary punctuation and sentence structure
remain preferable, but an occasional em dash does not justify blocking adopter work, ellipses have
legitimate prose uses, and curly quotes are normal text. In particular, `gofmt` may produce curly
quotes in doc comments, so the old restriction conflicts with the language's standard formatter.
The retained guard must remain language-agnostic: it scans tracked text rather than attempting to
classify prose through language-specific comment or literal parsing.

Existing adopter configuration may contain path-and-codepoint exemptions for any formerly banned
character. Rejecting those entries immediately would turn a policy relaxation into a compatibility
failure. The approved audit-remediation boundary reserves removal of proven obsolete Program A and
compatibility machinery for RF-014A and RF-014B rather than mixing that cleanup into this change.

## Decision

1. `decision: adopt-punctuation-restraint` Prefer ordinary punctuation and sentence structure, while
   replacing the seven-codepoint ban with one language-agnostic tracked-text policy. Skip binary
   files. Reject every en dash. Within each blank-line-delimited paragraph, permit zero, one, or two
   em-dash codepoints and reject three or more. Permit the single-codepoint ellipsis, three-period
   ellipses, and all curly quotes.
2. `decision: retain-guarded-character-exemptions` Retain path-and-codepoint exemptions, including an
   optional exact occurrence count, for quotations, frozen records, and text that discusses a
   guarded character. An exemption suppresses the matching guarded character for its path before
   the paragraph restraint is evaluated.
3. `decision: tolerate-retired-exemptions` During the compatibility period, continue accepting
   existing exemption entries for formerly guarded ellipses and curly quotes as inert configuration.
   Do not require new exemptions for those permitted characters. Removal of proven obsolete policy
   or compatibility machinery remains owned by RF-014A or RF-014B.
4. `decision: align-advisory-punctuation-audit` Keep the existing non-failing documentation audit
   advisory aligned with punctuation restraint. For each non-generated Markdown file under the
   documentation root, compare old and new text and warn when either the en-dash occurrence count or
   the total em-dash excess rises. Em-dash excess is the sum, across blank-line-delimited paragraphs,
   of each paragraph's em-dash count beyond two. Do not warn about permitted ellipses, curly quotes,
   or compliant em-dash use.

## State changes

- remove `tooling/quality-gates:emitted-prose-no-typographic-substitutes`
- update `tooling/quality-gates:prose-gate-tracked-file-scan`
- update `tooling/audit-and-snapshots:audit-advisories-always-run`
- update `tooling/audit-and-snapshots:audit-plain-punctuation`

## Consequences

Adopter work remains subject to a deterministic tracked-text guard, but normal ellipses, curly
quotes, and restrained em-dash use no longer fail repository checks. Every en dash remains a finding,
and dense em-dash prose becomes a finding at the paragraph boundary rather than by whole-file count.
The scan continues to cover non-prose tracked text without needing language-aware exclusions, so
formatter-produced doc comments are judged by the same policy as every other text block.

Diagnostics and exemptions become slightly more contextual because em-dash findings arise from a
paragraph threshold. Path-and-codepoint exemptions still provide a narrow escape hatch for frozen or
self-describing text. Former exemptions remain parseable but inert until the compatibility support
floor permits their removal.

The specialized seven-codepoint emitted-prose invariant is retired because it would contradict the
new policy and depends on language-specific Go literal detection. The ordinary tracked-text scan
continues to cover those source files. The audit advisory remains warning-only and covers its existing
documentation universe; it does not replace the repository check or widen historical blob loading.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Remove punctuation enforcement entirely | The approved boundary retains a language-agnostic tracked-text guard and a narrower restraint policy. |
| Keep the seven-codepoint ban but make it advisory | It would still discourage explicitly permitted punctuation and would not implement the approved paragraph threshold. |
| Parse comments or prose regions by language | It would make policy coverage language-dependent and recreate the formatter conflict the new boundary removes. |
| Reject obsolete exemption codepoints immediately | A policy relaxation would unexpectedly break existing adopter configuration instead of remaining compatible. |
| Expand historical audit collection to every tracked text file | That changes the historical snapshot boundary and is unnecessary because the repository check owns complete tracked-text enforcement. |

## Status history

- 2026-08-20: Proposed
- 2026-08-20: Accepted; content-sha256: 29644868fab3ddb7b01dfd997cab6f1494dafd1ead3c8c7ac198c152d083ab7d
- 2026-08-20: Implementing; content-sha256: 29644868fab3ddb7b01dfd997cab6f1494dafd1ead3c8c7ac198c152d083ab7d
- 2026-08-20: Applied; operations: remove `tooling/quality-gates:emitted-prose-no-typographic-substitutes`, update `tooling/quality-gates:prose-gate-tracked-file-scan`, update `tooling/audit-and-snapshots:audit-advisories-always-run`, update `tooling/audit-and-snapshots:audit-plain-punctuation`
