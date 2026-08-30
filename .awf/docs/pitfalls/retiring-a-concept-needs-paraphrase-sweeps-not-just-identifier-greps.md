---
title: "Retiring a concept needs paraphrase sweeps, not just identifier greps"
domains: ["rendering", "adr-system"]
---
Grepping for a retired concept's identifier misses the prose that teaches the concept
without naming it. Retiring `state-sequence` left "sequences are consecutive", "global
sequence order", and "a contiguous global sequence" untouched in the reviewer-agent
lenses, skill notes, and the glossary through a full plan review; a resync pass and a
spot-check found them only by grepping the concept's behavioral vocabulary ("global
sequence", "consecutive"). When a change retires or redefines a concept, derive a
paraphrase list from how the docs actually describe its behavior and sweep templates,
agent configs, glossary, domain parts, and pitfalls with those terms too; the review
catalog's own lens prose is governed text and drifts like any other doc.

Related decisions: [ADR-0191](../decisions/0191-replace-the-global-state-sequence-with-adr-number-provenance-order.md)
