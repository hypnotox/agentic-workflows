awf has one tier: `./x gate` always runs its deterministic non-test checks before each commit, while independently selecting Go and Pi test lanes from staged paths. `./x gate timings` reports only executed stages; it is not a slower tier.

| Lane | Proves |
|---|---|
| Go | Unit, integration, regression, coverage, vet, lint, dead-code, cross-compile, and pin checks. |
| Pi container | Protocol-v2 profile-contract negotiation, strict TypeScript coverage, adapter policy, generated native skill routing, and effort association. |
| Pi runtime smoke | Generated adapter delivery through a contract double plus retained awf effort integration on the pinned in-memory Pi seam where applicable. |

A successful capability handshake and final profile registration are the `pi-tools` compatibility test; there is no awf fallback and no revision pin. External general context, handoff, scheduling, execution, and rendering mechanics belong to `pi-tools` assurance. The retained 0.81.1 active-tool and file-mutation-queue floor applies only to awf-owned effort integration.

The release-only interactive Pi smoke is manual verification, not a deterministic gate lane.
