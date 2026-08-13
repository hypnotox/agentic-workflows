awf has one tier: `./x gate` always runs its deterministic non-test checks before each commit, while selecting test lanes from staged paths. `./x gate timings` reports only executed stages; it is not a slower tier.

| Lane | Proves |
|---|---|
| Go | Unit, integration, regression, coverage, vet, lint, dead-code, cross-compile, and pin checks. |
| Pi container | Protocol parity, strict TypeScript coverage, runtime guards, and the selection-gated effort association. |
| Pi runtime smoke | The pinned in-memory Pi 0.81.1 seam refreshes request context after active-branch compaction. |

The companion capability is final authority for degradation above the retained 0.81.1 floor; do not infer it from foreign package publication or installation topology. The guarded active-tool, prompt-guidance, and file-mutation-queue floor is also tested against the retained fork.

The release-only interactive Pi smoke is manual verification, not a deterministic gate lane.
