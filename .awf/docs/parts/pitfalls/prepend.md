## Parity checks need independent observations

_Domains: rendering, tooling_

A parity check is tautological when one side is copied from the declaration it claims to verify.
ADR-0144 initially copied `OutputDeclaration.Inputs` onto each `OutputPlan` node immediately before
comparing them, so missing or misclassified inputs changed both sides together and the check stayed
green. Keep the declared set and the consumed set independent: observe render-time project inputs
and semantic recipe inputs at their owning seams, normalize both sets, then compare them in both
directions. Mutation tests must remove, add, and reclassify a declaration input rather than only
mutating the already-copied plan representation.

## Non-null navigation is not useful navigation

_Domains: tooling_

Initializing a closed result field to an empty slice proves JSON shape, not behavior. ADR-0144's
first artifact-navigation proof asserted role presence and non-null `Navigation` arrays while every
array was empty, even though the claim promised direct drilldowns. For navigation or attribution
contracts, test the semantic payload for every closed role: identity, sources, outputs, navigation
labels and paths, ordering, relocated roots, disabled artifacts, reservations, and unmanaged
lookalikes. A structural non-null assertion is only the baseline.
