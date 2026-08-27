awf has one tier: `./x gate` always runs its deterministic non-test checks before each commit, while independently selecting Go and Pi test lanes from staged paths. `./x gate timings` reports only executed stages; it is not a slower tier.

| Lane | Proves |
|---|---|
| Go | Unit, integration, regression, coverage, vet, lint, dead-code, cross-compile, and pin checks. |
| Pi host | Protocol-v2 profile-contract negotiation, strict TypeScript coverage, adapter policy, generated native skill routing, and effort association. |
| Pi runtime smoke | Generated adapter delivery through a contract double, retained awf effort integration, and narrow selected-checkout lifecycle composition through the test-only pinned pi-tools source. |

A successful capability handshake and final profile registration are the adopter `pi-tools` compatibility test; there is no awf fallback or adopter revision pin. External general context, handoff, subprocess supervision, and rendering mechanics belong to `pi-tools` assurance. The strict lane's test-only pi-tools v0.3.0 source pin records generic Pi API, event, context, UI, model-registry, command, tool, active-tool, and execution seams. It also composes `createSubagentToolkit` narrowly to prove selected prepared-CWD transport, scheduler callback traversal, and invocation isolation without claiming confinement or general pi-tools assurance. The retained 0.84.2 active-tool and file-mutation-queue floor applies only to awf-owned effort integration.

The release-only interactive Pi smoke is manual verification, not a deterministic gate lane.
