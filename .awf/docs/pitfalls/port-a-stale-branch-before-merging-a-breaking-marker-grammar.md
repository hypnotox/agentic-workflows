---
title: "Port a stale branch before merging a breaking marker grammar"
domains: ["invariants", "adr-system", "tooling"]
---
Before merging a checker that cannot read the old marker grammar, land compatibility parsing
and migrate branch-only markers first. Staged validation reads the stale first parent as well
as the result, so a correct merged tree alone is insufficient.

Related decisions: [ADR-0205](../decisions/0205-proof-markers-name-the-unit-that-proves-them.md), [ADR-0206](../decisions/0206-sanction-the-seal-crossing-integration-transition.md)
