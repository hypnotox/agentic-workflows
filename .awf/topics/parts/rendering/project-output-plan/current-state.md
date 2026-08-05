The project package assembles the full render set, computes the output plan and config hash, checks drift, and prunes stale outputs. The claims below capture the current output-plan and render-orchestration contracts.

The Pi target descriptor is the sole declaration of the five Pi TypeScript outputs: context usage, handoff, and subagent index, model-routing, and runner; non-Pi target sets render and prune none of them.

## Claims

### `invariant: bridge-render-identity`

Every target-declared bridge renders through the neutral `target-bridge` identity while its descriptor remains the sole owner of bridge path and template. Input observation does not derive a target-specific sidecar or template from that neutral identity, so a future bridge target cannot inherit Claude-specific inputs accidentally.
Origin: ADR-0214
Backing: test

### `invariant: catalog-trim-applied`

A non-nil catalog-trim selection passed to ScaffoldConfig replaces the curated-core skills and docs enable arrays verbatim before closure completion, while a nil selection keeps exactly the curated core.
Origin: ADR-0029
Backing: test

### `invariant: curated-init-skill-refs-clean`

A default curated awf init render passes awf check with zero dead-skill-reference findings.
Origin: ADR-0046
Backing: test

### `invariant: inert-sidecar-field-rejected`

Every gated command fails at project open when a non-domain sidecar carries a non-empty paths field, or a domain sidecar carries any non-paths field such as data, sections, or local, with a message naming the file and the required edit.
Origin: ADR-0086
Backing: test

### `invariant: kind-dispatch-single-table`

Every per-kind facet - the config enable array, catalog pool, declared sections, output path, singular and plural labels, graph membership, and freeform-domain membership - is defined once in the single ordered kind-descriptor table in the project package, and cmd/awf decides no kind fact outside the table's exported accessors; a test asserts the table's kind set equals the catalog's kinds plus the freeform domains kind, and a source-scanning test over the cmd/awf sources asserts no kind-name equality or switch-case comparison remains there.
Origin: ADR-0027
Revised-by: ADR-0195
Backing: test

### `invariant: multi-target-render`

With multiple targets enabled, every enabled catalog skill and agent renders once per target to that target's descriptor-derived path, while neutral artifacts such as AGENTS.md render exactly once regardless of target count. A target-owned skill or other output renders only for its declaring target when its closed catalog-selection predicate is satisfied; configured-prefix path derivation, declaration, rendering, coalescing, hashing, pruning, provenance, and policy all use the same resolved descriptor.
With multiple targets enabled, each adapter artifact renders once per target at that descriptor's declared paths - including Claude Code and Pi skills and agents - while neutral artifacts such as `AGENTS.md` render exactly once regardless of target count. Descriptor-specific wording, bridges, capabilities, encodings, and additional outputs remain independently customizable.
Origin: ADR-0037
Revised-by: ADR-0214, ADR-0218
Backing: test

### `invariant: output-plan-complete`

The deterministic output plan contains catalog and local artifacts, bridge files, generated documentation, reservations, and exactly two resident-root self-ignoring outputs: efforts and worktrees. Its conditional config-tree units share their declaration facts with render dispatch. Resident dynamic descendants are not plan nodes and resolve at the primary root while tracked authority remains invoking-checkout authority.
Origin: ADR-0124
Revised-by: ADR-0164, ADR-0167, ADR-0175, ADR-derive-render-completeness-from-output-authority
Backing: test

### `invariant: check-report-single-plan`

Project.CheckReport constructs one operation-owned OutputPlan after deriving its current state and parsed plans, threads that same plan to both drift and advisory projections, and never regenerates domain documents or the config reference inside either projection. Standalone Check, AdvisoryNotes, OutputPlan, and other direct project operations continue to derive their own operation-scoped inputs without a persistent cache.
Origin: ADR-0223
Backing: test

### `invariant: output-policy-explicit`

Post-processing of each output, frontmatter validation, link scanning, and skill-reference scanning, is selected by that output's declared policy rather than its file suffix. A non-Markdown path with a Markdown policy is still validated and scanned, a Markdown-looking path with a plain policy is not, and the zero-value policy scans nothing.
Origin: ADR-0124
Backing: test

### `invariant: resident-policy-single-home`

The resident-root table, the resident-path predicate, and anchored output-path resolution have exactly one production home in internal/resident; core consumes them through the Roots value constructed once at project open, and no file under internal/project or cmd redeclares or re-derives the table or predicate (internal/git's seam-owned ResidentName spelling is the recorded tolerated parallel).
Origin: ADR-0195
Backing: test

### `invariant: reviewing-skill-agent-pairing`

Opening a project fails when an enabled non-local skill declares a required agent that is absent from the agents enable array, with an error naming both the skill and the agent.
Origin: ADR-0050
Backing: test

### `invariant: scaffold-core-only`

The config generated by ScaffoldConfig enables exactly the catalog's core skills and core docs plus all agents and all hooks, and omits every non-core skill and doc.
Origin: ADR-0022
Backing: test

### `invariant: scaffold-seeds-all-vars`

ScaffoldConfig seeds a value for every var referenced by any catalog skill, agent, hook, or doc template, whether or not that target is core, so opting a target in later renders without an unresolved value.
Origin: ADR-0022
Backing: test

### `invariant: shared-output-coalesced`

An output produced by more than one target at the same path with an identical recipe is coalesced into a single plan node whose declarer set unions the contributing target names, and its drift hash folds in every declarer's projection. Two targets that declare the same path with conflicting recipes fail with a conflicting-output-recipes error.
Origin: ADR-0124
Backing: test

### `invariant: sidecar-key-overrides-default`

When merging an artifact's catalog default data with its sidecar, a top-level key present in the sidecar - even when set to null or empty - fully replaces the catalog default for that key, while a key absent from the sidecar falls through to the catalog default; there is no deep merge.
Origin: ADR-0045
Backing: test

### `invariant: skills-context-effective-set`

The skills set exposed to templates equals the enabled skills minus those suppressed by a doc gate, while skills declared local are always kept.
Origin: ADR-0046
Backing: test

### `invariant: target-capabilities-closed`

A target descriptor is validated against closed sets: unknown capabilities, unknown agent dialects, unknown output encoders, out-of-set provenance values, path traversal in output paths, and undeclared or inconsistent output policies are all rejected, both when the descriptor is validated and again when the output plan is built.
Origin: ADR-0124
Backing: test

### `invariant: template-id-single-derivation`

Template identity derives from the catalog, the kind-descriptor table, and the singleton and target declaration tables alone; no production file outside those declaration files spells a full template-ID path literal, and internal/topic receives template identity and content from its caller rather than re-reading the embedded tree. Live identity resolution derives from those same authorities, while the co-owned runner remains recognition-only.
Origin: ADR-0195
Revised-by: ADR-derive-render-completeness-from-output-authority
Backing: test

### `invariant: conditional-unit-single-source`

Each enabled config-tree render unit derives its enablement, path, template identity, render kind,
and fixed sections from one bounded descriptor consumed by output declarations and render dispatch.
Unit-specific data construction, policy, encoding, and lifecycle behavior remain at their owning
render seams.
Origin: ADR-derive-render-completeness-from-output-authority
Backing: test
