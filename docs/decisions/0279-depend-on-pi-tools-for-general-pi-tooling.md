---
format: current-state-v4
slug: depend-on-pi-tools-for-general-pi-tooling
status: Implemented
date: 2026-08-15
---
# ADR-0279: Depend on pi-tools for general Pi tooling

## Context

Awf currently renders and tests complete Pi implementations for transient context usage,
fresh-session handoff, and four governed subagent tools. The subagent implementation owns its
own subprocess runner, scheduler, output bounds, progress rendering, and model execution even
though those concerns are not specific to awf workflows. This makes awf responsible for general
Pi extension mechanics and prevents those mechanics from being patched independently of an awf
release.

The separately installed `hypnotox/pi-tools` package now owns the general context-usage and
handoff extensions. Its subagent toolkit exposes a versioned, load-order-tolerant event-bus
handshake through which consumers atomically register typed profiles. Protocol version 2 carries
active-tool prompt guidance, asynchronous model selection, pre- and post-run policy hooks,
bounded profile data, terminal policy failures, fail-closed exclusive batches, and correlated
final registration results. Those capabilities allow awf to retain its workflow policy without
retaining the general execution machinery.

Pi packages installed separately do not share a reliable runtime module root. The integration
therefore cannot depend on directly importing toolkit runtime code. Awf users also need to patch
`pi-tools` independently, so a pinned package revision would recreate the release coupling this
change is intended to remove.

## Decision

1. `decision: external-general-pi-tooling` Make `pi-tools` an independently installed prerequisite for awf's Pi harness. Let `pi-tools` exclusively own general context usage, fresh-session handoff, subagent process execution, scheduling, confinement, execution facts, and common presentation; awf will not render fallback implementations of those concerns.
2. `decision: handshake-profile-integration` Integrate awf's governed grounding, exploration, review, and implementation tools as one atomic consumer-owned subagent profile batch over protocol version 2. The consumer subscribes to capability and registration-result events before sending a correlated request, reuses one stable registration identity, and suppresses the generic default only when its complete batch registers successfully.
3. `decision: retained-awf-profile-policy` Retain the four closed tool schemas, active-tool guidance, rendered role contracts, role tool allowlists, asynchronous per-invocation model preference loading and validation, and the model preference wizard as awf policy. Exploration, grounding, and review each admit at most ten active calls; implementation admits one active call, requires parent-batch exclusivity, and retains caller-selected verification checkout snapshots and structured commit-policy failures.
4. `decision: compatibility-not-version-pinning` Treat successful handshake negotiation and profile registration as the compatibility boundary. Do not pin a `pi-tools` release or import its runtime directly; report a missing, incompatible, late, or rejected prerequisite once with actionable repair guidance and do not activate an awf fallback.
5. `decision: separated-assurance-ownership` Let `pi-tools` prove its general extension mechanics and let awf prove its rendered profile adapter, workflow policy, generated-output boundary, handshake behavior, and any overlapping compatibility contract needed at that boundary.

## State changes

- remove `rendering/pi-runtime:pi-child-process-safety`
- remove `rendering/pi-runtime:pi-child-tool-boundaries`
- remove `rendering/pi-runtime:pi-context-usage-injection`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-implementation-state-boundary`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`
- add `rendering/pi-runtime:pi-tools-integration-boundary`
- remove `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-dedicated-grounding-dispatch`
- update `rendering/pi-workflows:pi-extension-editor-quiet-strip`
- update `rendering/pi-workflows:pi-implementation-batch-exclusivity`
- remove `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/pi-workflows:pi-structured-exploration-contract`
- remove `rendering/pi-workflows:pi-subagent-failure-details`
- update `rendering/pi-workflows:pi-subagent-model-preferences`
- update `rendering/pi-workflows:pi-subagent-model-routing`
- update `rendering/pi-workflows:pi-implement-role-artifact`
- remove `rendering/pi-workflows:pi-subagent-progress-bounds`
- remove `rendering/pi-workflows:pi-subagent-progress-context-isolation`
- remove `rendering/pi-workflows:pi-subagent-progress-rendering`
- update `rendering/pi-workflows:pi-role-contract-loader`

## Consequences

General Pi behavior can evolve independently in `pi-tools`, while awf releases concentrate on
workflow policy and generated integration. Pi adopters must install and maintain an additional
package, and missing or incompatible handshakes leave the awf subagent profiles unavailable rather
than silently falling back. The generic `pi-tools` subagent may remain available after an awf
registration failure, but it is not governed as an awf workflow tool.

Awf adopts the externally owned handoff, context-usage, progress, bounds, and presentation
semantics rather than preserving implementation-specific copies. Model preferences still reload
for every invocation, but the external toolkit owns queueing after selection, so awf no longer
revalidates them a second time after a queue wait. The effort association and memory extension
remain rendered by awf because their protocol and policy are repository-specific.

The handshake is a public compatibility dependency even without a revision pin. Protocol changes
must either preserve a mutually supported handshake or produce the actionable incompatibility
path. Contract tests on both sides reduce drift without coupling awf's gate to a particular
`pi-tools` checkout.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep all Pi extensions in awf | Continues duplicate ownership and requires an awf release for general Pi tooling fixes. |
| Import or vendor the `pi-tools` runtime | Separately installed packages do not share a reliable runtime module root, and vendoring recreates duplicate implementation ownership. |
| Pin a `pi-tools` tag or commit | Prevents independent patching and makes package release cadence part of awf compatibility. |
| Retain awf implementations as a registration fallback | Preserves duplicate ownership, permits the implementations to diverge, and weakens the prerequisite failure boundary. |
| Move awf role and verification policy into `pi-tools` | Couples a general Pi package to repository-specific workflow semantics. |

## Status history

- 2026-08-15: Proposed
- 2026-08-15: Accepted; content-sha256: 42f2da3bc64001924042a9599f68c9a2bee88bb8b36326542b5bd0b79a7d104a
- 2026-08-15: Implementing; content-sha256: 42f2da3bc64001924042a9599f68c9a2bee88bb8b36326542b5bd0b79a7d104a
- 2026-08-15: Applied; operations: remove `rendering/pi-runtime:pi-child-process-safety`, remove `rendering/pi-runtime:pi-child-tool-boundaries`, remove `rendering/pi-runtime:pi-context-usage-injection`, update `rendering/pi-runtime:pi-extension-target-render`, update `rendering/pi-runtime:pi-implementation-state-boundary`, update `rendering/pi-runtime:pi-minimum-runtime`, update `rendering/pi-runtime:pi-real-runtime-smoke`, add `rendering/pi-runtime:pi-tools-integration-boundary`, remove `rendering/pi-workflows:pi-session-handoff-lifecycle`, update `rendering/pi-workflows:pi-dedicated-grounding-dispatch`, update `rendering/pi-workflows:pi-extension-editor-quiet-strip`, update `rendering/pi-workflows:pi-implementation-batch-exclusivity`, remove `rendering/pi-workflows:pi-session-handoff-public-contract`, update `rendering/pi-workflows:pi-structured-exploration-contract`, remove `rendering/pi-workflows:pi-subagent-failure-details`, update `rendering/pi-workflows:pi-subagent-model-preferences`, update `rendering/pi-workflows:pi-subagent-model-routing`, update `rendering/pi-workflows:pi-implement-role-artifact`, remove `rendering/pi-workflows:pi-subagent-progress-bounds`, remove `rendering/pi-workflows:pi-subagent-progress-context-isolation`, remove `rendering/pi-workflows:pi-subagent-progress-rendering`, update `rendering/pi-workflows:pi-role-contract-loader`
- 2026-08-15: Implemented; content-sha256: 42f2da3bc64001924042a9599f68c9a2bee88bb8b36326542b5bd0b79a7d104a
