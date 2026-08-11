---
title: "`adr.ADR.Title` already carries the `ADR-NNNN: ` prefix"
domains: ["adr-system"]
tags: ["adr-parsing", "context-query"]
---
`adr.ParseDir` reads `Title` verbatim from the `# ADR-NNNN: ...` heading, so it includes the
`ADR-NNNN: ` prefix while `Number` carries the digits separately. A new consumer that prints
both, as `awf context`'s `ADRRef` did in its first draft (2026-07-11), double-prints the
number (`ADR-0092 ... ADR-0092: Title`); plan review caught it. Strip the prefix
(`strings.TrimPrefix(a.Title, "ADR-"+a.Number+": ")`) when surfacing Title alongside Number.
`awf context` was the first `adr.ParseDir` consumer outside `internal/{adr,invariants,audit}`,
so the gotcha only surfaces as awf grows ADR-aware tooling.
