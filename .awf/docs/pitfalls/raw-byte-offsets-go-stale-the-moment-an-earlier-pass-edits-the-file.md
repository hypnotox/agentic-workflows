---
title: "Raw-byte offsets go stale the moment an earlier pass edits the file"
domains: ["config"]
related: [128]
---
`ADR.DecisionStart`/`DecisionEnd` are byte offsets into the bytes that were *parsed*. A
migration that rewrites the body in one pass and appends at `DecisionEnd` in a later pass
is appending at an offset that no longer means what it did. The generation-12
supersession-keys migration downgrades `supersedes:` to `refines:` (three bytes shorter
per token) before appending its bookkeeping item, and on a token-dense ADR the append
landed far enough early to open inside the following heading, corrupting
`## Invariants` into `## Inv13. **Supersedence bookkeeping...` - which then silently
deleted every slug that ADR declared, surfacing as unrelated `adr-token-ref` drift.
Track a per-file delta from every editing pass and add it to any later offset use, the
way the same migration already tracked `removed` for stripped key lines. The tell is
drift about *declarations going missing*, which points at a mangled section heading
rather than at the tokens the error names.
