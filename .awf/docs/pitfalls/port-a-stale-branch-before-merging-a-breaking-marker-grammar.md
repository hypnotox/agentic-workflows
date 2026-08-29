---
title: "Port a stale branch before merging a breaking marker grammar"
domains: ["invariants", "adr-system", "tooling"]
related: [205, 206]
---
Before merging a checker that cannot read the old marker grammar, land compatibility parsing
and migrate branch-only markers first. Staged validation reads the stale first parent as well
as the result, so a correct merged tree alone is insufficient.
