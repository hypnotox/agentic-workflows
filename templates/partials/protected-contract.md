**The protected contract.**

The workflow governs a change's protected contract, not its execution route. Protected: the requested outcome, the explicitly settled durable choices, the material scope, the externally observable behaviour, the compatibility and safety constraints, the required verification strength, the prohibited shortcuts, and every constraint an active project rule places on one of these, which includes generated-source ownership, drift detection, and path and worktree confinement{{if ne .profile "core"}}, and current-state authority{{end}}.

Everything else about how the change is carried out is the route: phase and task boundaries, their order, local names, file and symbol inventories, helper allocation, execution mode, exact command sequence, commit decomposition, and non-load-bearing mechanism choice. An implementation owner chooses and revises the route while the protected contract holds.

Precedence is decided per constraint, not per rule. A clause that bears only on how a change is carried out is subordinate to the protected contract, so one rule may be protected in its protected clauses and subordinate in its route clauses. A route detail binds only when a settled decision states that it is load-bearing.
