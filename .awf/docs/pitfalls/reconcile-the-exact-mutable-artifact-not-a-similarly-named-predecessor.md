---
title: "Reconcile the exact mutable artifact, not a similarly named predecessor"
domains: ["rendering", "tooling"]
---
A helper once edited an older, similarly themed plan instead of the active effort's scratch
plan. Earlier in the same effort, authored artifacts also landed in the wrong checkout before
immediate cleanup. Repeated path plausibility is not identity proof, and a green render or gate
does not prove that a valid edit reached the intended artifact.

Before mutating or accepting a report about an effort plan, memory, or managed-worktree file,
resolve the exact path from the active effort, verify its identity and selected checkout, and
inspect the actual changed-path list.
