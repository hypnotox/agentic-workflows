---
title: "An ordered-phrase assertion cannot reach a site ahead of its first anchor"
domains: ["rendering", "invariants"]
---
`assertOrderedBody` advances a cursor past each matched phrase, so every phrase must
occur after the previous one and nothing before the FIRST anchor is scanned at all.
Appending a phrase to an ordered list therefore proves nothing about prose that renders
ahead of that list's opening anchor: the match is satisfied by a later copy of the same
sentence, and deleting the early one fails no test. In ADR-0160 the brainstorming skill
carries the effort-identity rule in its own procedure prose near the top of the body,
while the shared approval partial repeats it far below, and the ordered list opens at
"**Mandatory approval check-in.**". Two successive rounds recorded the early site as
covered by that list; it never was. A site ahead of the anchor needs its own assertion
that resolves both offsets and compares them. The general form: before claiming an
ordered-list entry covers a site, check where that site renders relative to the list's
first phrase.
