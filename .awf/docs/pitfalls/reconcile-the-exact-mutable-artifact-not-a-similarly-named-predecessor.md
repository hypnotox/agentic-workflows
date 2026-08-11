---
title: "Reconcile the exact mutable artifact, not a similarly named predecessor"
domains: ["rendering", "tooling"]
tags: ["plan-artifact", "verification-discipline"]
---
An implementation-review settlement was told to reconcile one mutable plan but edited an
older, similarly themed Implemented plan instead. The change both misplaced the new Notes
entry and regressed the historical plan to Proposed; render and the full gate stayed green
because each file remained structurally valid. Earlier in the same effort, ADR and plan
scaffolds had also landed in the wrong checkout before immediate cleanup. Repeated path
plausibility is not identity proof.

Before mutating or accepting a report about an ADR, plan, memory, or managed-worktree file,
resolve the exact path from the active effort and review evidence, verify its current identity
and lifecycle state, and inspect the actual changed-path list. For terminal settlements,
confirm that the intended mutable plan received the Notes update and that unrelated terminal
artifacts retained their status. A green render or gate does not prove that a valid edit
reached the intended artifact.
