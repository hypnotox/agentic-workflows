---
title: "Do not infer linked-plan freshness from file activity"
domains: ["rendering", "tooling"]
tags: ["plan-artifact", "verification-discipline"]
---
Modification time, filename similarity, and session context do not identify every plan
affected by an ADR correction. Use the parsed plan-level `adrs:` association exposed by
explicit ADR context, settle ADR review first, and run ordinary full review for every linked
Proposed plan. If implementation has started, inventory completed affected phases and renew
assurance wherever the changed decision can affect landed work before progression.
