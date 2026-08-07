---
format: current-state-v4
slug: scope-memory-diff-truncation-to-bounding-loss
status: Proposed
date: 2026-08-07
---
# ADR-0247: Scope Memory Diff Truncation To Bounding Loss

## Context

ADR-0244 gave effort-memory mutations a bounded Pi-compatible display diff carrying a
`truncated` flag, which the generated Pi tools render as the warning `Diff truncated for
display.`. Its decision 2 closes with "Any content or row omission sets `truncated` to true",
and the claim it revised, `tooling/effort-management:memory-skeleton-purpose-partition`, ends
"whole changed rows or ranges use Pi's exact omission row, and every omission sets
`truncated:true`".

Two different things omit rows. Bounding drops or elides rows when the diff would exceed the
50-KiB protocol bound, which is real content loss. Hunk separation emits the same omission row
whenever `GetGroupedOpCodes(4)` splits regions more than eight unchanged lines apart, which
loses nothing: it is ordinary four-context-line windowing that Pi's own diff performs. The
implementation of ADR-0244 read the sentence as covering both and set `truncated` for either,
so a complete two-hunk diff of a twelve-line memory displayed the truncation warning while
having dropped nothing. Implementation review found that behaviour and it was narrowed to
bounding loss alone.

Both readings of the frozen sentence are defensible and the ambiguity is worth recording,
because the sentence is the only prose an adopter reads. Supporting the narrow reading: "Any
content or row omission" is a two-item enumeration that maps one to one onto the two sentences
immediately before it, one about eliding content inside a row and one about replacing whole
rows or ranges; the surrounding run of four sentences all concern bounding, opened by "The
50-KiB protocol bound remains"; and the wide reading compels a truncation warning on a diff
that lost nothing. Supporting the wide reading: "Any" is an unrestricted universal where
"bounding" was available as a qualifier and was used two sentences earlier; ADR-0244's same
paragraph already applies the word omission to hunk separation in "separated regions use the
same omission convention"; the claim sentence is a flat coordinate list with none of that
paragraph structure; and the original implementer, working from the frozen text, read it
the wide way.

The distinction matters beyond wording. `truncated` is the only signal a consumer has that
content was lost, and a flag that also fires on complete diffs cannot carry that meaning. The
narrow behaviour is already shipped; what remains is that the claim sentence still reads as the
wide rule, so the corpus does not record which rule is in force.

## Decision

1. `decision: truncation-means-bounding-loss` `truncated` reports only that bounding lost
   content: a changed row whose content was elided to fit, or a row or range dropped by the
   bounded selection. Hunk separation never sets it, and the omission row it emits remains a
   rendering convention rather than a loss signal. Discarded leading and trailing context is
   likewise not a loss signal, because four-context-line windowing is the diff's defined shape
   rather than an omission from it. Consumers may therefore treat `truncated` as a faithful
   answer to "is displayed content missing", and a presentation surface may warn on it.

## State changes

- update `tooling/effort-management:memory-skeleton-purpose-partition`

## Consequences

The truncation warning now appears only when content was actually lost, so it carries
information rather than firing on most multi-hunk edits. A consumer that wants to know whether
regions were separated reads the omission rows in the diff text, which is where Pi itself
carries that fact; no separate protocol signal is introduced, so the envelope is unchanged.

The claim sentence is reworded to say bounding omission rather than every omission. That
retires the wide reading as a live interpretation, and any future decision wanting a
separation signal must add its own field rather than overloading this one.

Because the narrowed behaviour and its tests already shipped under ADR-0244, this record
changes no code. It exists to make the governing rule explicit in the corpus and to leave the
two-reading analysis on the record, so the question is settled once rather than re-argued from
the ambiguous sentence.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Restore the wide behaviour so the existing sentence is literally true | Keeps the claim unchanged, but requires the truncation warning on complete diffs that lost nothing, which drains the flag of meaning at its only consumer |
| Leave the narrowed behaviour and the wide sentence as they are | Costs nothing now, but leaves prose that reads false to an adopter and no record that the reading was ever decided |
| Add a separate `contextOmitted` field to the protocol diff object | Most faithful to both facts, but changes the Go envelope, the generated client validator, and both Pi renderers to expose a fact already visible in the diff text |

## Status history

- 2026-08-07: Proposed
