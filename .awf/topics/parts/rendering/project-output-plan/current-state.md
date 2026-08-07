The project package assembles the full render set, computes the output plan and config hash, checks drift, and prunes stale outputs. The claims below capture the current output-plan and render-orchestration contracts.

The Pi target descriptor is the sole declaration of the five Pi TypeScript outputs: context usage, handoff, and subagent index, model-routing, and runner; non-Pi target sets render and prune none of them.

## Claims

### `invariant: bridge-render-identity`

Every target-declared bridge renders through the neutral `target-bridge` identity while its descriptor remains the sole owner of bridge path and template. Input observation does not derive a target-specific sidecar or template from that neutral identity, so a future bridge target cannot inherit Claude-specific inputs accidentally.
Origin: ADR-0214
Backing: test

### `invariant: kind-dispatch-single-table`

Every per-kind facet - the catalog collection, declared sections, output path, singular and plural labels, and freeform-domain membership - is defined once in the single ordered kind-descriptor table in the project package, and cmd/awf decides no kind fact outside the table's exported accessors; a test asserts the table's kind set equals the catalog's kinds plus the freeform domains kind, and a source-scanning test over the cmd/awf sources asserts no kind-name equality or switch-case comparison remains there.
Origin: ADR-0027
Revised-by: ADR-0195, ADR-0251
Backing: test

### `invariant: multi-target-render`

For both built-in targets, every catalog skill and agent renders once at that target's descriptor-derived path, while neutral artifacts such as AGENTS.md render exactly once. A target-owned skill or other output renders only for its declaring target when its declared predicate is satisfied; configured-prefix path derivation, declaration, rendering, coalescing, hashing, pruning, provenance, and policy all use the same resolved descriptor.
Each adapter artifact renders once for Claude Code and Pi at its descriptor's declared paths - including Claude Code and Pi skills and agents - while neutral artifacts such as `AGENTS.md` render exactly once. Descriptor-specific wording, bridges, capabilities, encodings, and additional outputs remain independently customizable.
Origin: ADR-0037
Revised-by: ADR-0214, ADR-0218, ADR-0251
Backing: test

### `invariant: output-plan-complete`

The deterministic output plan contains every catalog artifact, bridge files, generated documentation, reservations, and exactly two resident-root self-ignoring outputs: efforts and worktrees. Its conditional config-tree units share their declaration facts with render dispatch. Resident dynamic descendants are not plan nodes and resolve at the primary root while tracked authority remains invoking-checkout authority.
Origin: ADR-0124
Revised-by: ADR-0164, ADR-0167, ADR-0175, ADR-0235, ADR-0251
Backing: test

### `invariant: full-catalog-render`

Every output plan includes every catalog skill, agent, and document without consulting a config-derived enable selection, requirement closure, per-document suppression, or local reservation.
Origin: ADR-0251
Backing: test

### `invariant: inert-sidecar-field-rejected`

A skill, agent, document, or singleton sidecar rejects paths, and a domain sidecar rejects data, dataDefaults, and sections, so no accepted sidecar field is inert for its artifact kind.
Origin: ADR-0086
Revised-by: ADR-0251
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

### `invariant: scaffold-seeds-all-vars`

ScaffoldConfig seeds a value for every var referenced by any catalog skill, agent, hook, or doc template, so every unconditional catalog render starts without an unresolved value.
Origin: ADR-0022
Revised-by: ADR-0251
Backing: test

### `invariant: shared-output-coalesced`

An output produced by more than one target at the same path with an identical recipe is coalesced into a single plan node whose declarer set unions the contributing target names, and its drift hash folds in every declarer's projection. Two targets that declare the same path with conflicting recipes fail with a conflicting-output-recipes error.
Origin: ADR-0124
Backing: test

### `invariant: sidecar-key-overrides-default`

When merging an artifact's catalog default data with its sidecar, a non-list top-level key present in the sidecar - even when set to null or empty - fully replaces the catalog default for that key, while a key absent from the sidecar falls through to the catalog default; there is no deep merge.
Origin: ADR-0045
Revised-by: ADR-0236
Backing: test

### `invariant: catalog-list-data-layering`

A same-key catalog list and project list compose shallowly as catalog entries followed by authored entries, preserving both orders without generic deduplication or identity merging. An absent or empty project list keeps the catalog list; dataDefaults false suppresses that default and yields only authored entries or an empty list, while differently keyed specialized transforms such as glossary standardTerms and terms stay outside this generic path.
Origin: ADR-0236
Backing: test

### `invariant: target-capabilities-closed`

A target descriptor is validated against closed sets: unknown capabilities, unknown agent dialects, unknown output encoders, out-of-set provenance values, path traversal in output paths, and undeclared or inconsistent output policies are all rejected, both when the descriptor is validated and again when the output plan is built.
Origin: ADR-0124
Backing: test

### `invariant: template-id-single-derivation`

Template identity derives from the catalog, the kind-descriptor table, and the singleton and target declaration tables alone; no production file outside those declaration files spells a full template-ID path literal, and internal/topic receives template identity and content from its caller rather than re-reading the embedded tree. Live identity resolution derives from those same authorities, while the co-owned runner remains recognition-only.
Origin: ADR-0195
Revised-by: ADR-0235
Backing: test

### `invariant: conditional-unit-single-source`

Each config-tree render unit derives its enablement, path, template identity, render kind, and fixed
sections from one bounded descriptor consumed by output declarations and render dispatch. Hook
payloads and the runner are unconditional members, while bootstrap is the only member whose
enablement is conditional. Unit-specific data construction, policy, encoding, and lifecycle behavior
remain at their owning render seams.
Origin: ADR-0235
Revised-by: ADR-0253
Backing: test
