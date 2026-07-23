## Footer parity must reuse public accounting boundaries

_Domains: rendering_

Do not maintain dashboard-local token counters, add tool-result usage after Pi has folded top-level usage into an assistant message, or charge summaries and compactions as new historical work. Traverse the active branch once by stable entry ID, sum public assistant usage including restored and nested-subagent totals, and use `getContextUsage()` only for current context. Keep subscription and automatic-context labels absent without a public signal. Likewise, assign canonical refresh generations before launch and reject stale completions; asynchronous refresh must not overwrite a newer validated local badge.

## Provisional and finding failures are authority boundaries

_Domains: rendering, tooling_

A fresh Pi session may lose only its uncommitted process-local window of 256 observations and 1 MiB; do not imply a durable spool or silently reassign overflow history. A repair or waiver is not authorized by the overlay selection alone: re-resolve the finding under its owning `effortId`, exact evidence and scope, eligible reason, and current nonempty causal frontier, and append nothing on any mismatch.

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

## Print-mode teardown does not wait for late settlement

_Domains: rendering_

Pi print mode ends the process when the final turn's awaited path returns: `agent_settled` and
other late hooks may never fire, and an asynchronous handler the runtime does not await races
shutdown. Land any durable terminal lifecycle write inside the final successful turn's awaited
path, and give every settlement entry point one shared in-flight promise to join rather than a
second run. ADR-0149's retrospective completion took three fix rounds to learn this: success
authority first hung on a retired signal, then on an event that teardown outran, and finally two
invokers could double-append the same terminal event with differing timestamps, which the causal
append rejects as conflicting identity.
