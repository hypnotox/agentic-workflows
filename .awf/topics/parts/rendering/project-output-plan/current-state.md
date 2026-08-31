A project render assembles a deterministic output set from catalog, target, configuration, topic, pitfall, and local-document declarations. Output planning rejects conflicting or invalid declarations before mutation, and repository checks preserve completed owner evidence and explicit presentation order.

The Pi target descriptor is the sole declaration of the two Pi-specific outputs: the subagent index and model-routing module. No target descriptor retains an effort index, effort client, or `using-effort` output.

## Claims

### `invariant: bridge-render-identity`

Every target-declared bridge renders through the neutral `target-bridge` identity while its descriptor remains the sole owner of bridge path and template. Input observation does not derive a target-specific sidecar or template from that neutral identity, so a future bridge target cannot inherit Claude-specific inputs accidentally.
Backing: test

### `invariant: multi-target-render`

For both built-in targets, every selected catalog skill and agent renders once at its descriptor-derived path, while neutral artifacts such as AGENTS.md render once. Target-owned outputs render only for their declaring target when their predicate and selected view include them.
Backing: test

### `invariant: output-plan-complete`

The deterministic output plan contains every standard catalog artifact, applicable bridge file, generated document, reservation, configured local-document output, and the three resident-root markers. Historical decision and plan leaves remain outside managed output ownership.
Backing: test

### `invariant: inert-sidecar-field-rejected`

A skill, agent, document, or singleton sidecar rejects paths, and a domain sidecar rejects data, dataDefaults, and sections, so no accepted sidecar field is inert for its artifact kind.
Backing: test

### `invariant: check-report-single-plan`

Repository check composition preserves each completed owner's immutable evidence and explicit slot order. It rejects owner results placed in advisory-only or information-only slots rather than silently reclassifying them.
Backing: test

### `invariant: output-policy-explicit`

Post-processing of each output, frontmatter validation, link scanning, and skill-reference scanning, is selected by that output's declared policy rather than its file suffix. A non-Markdown path with a Markdown policy is still validated and scanned, a Markdown-looking path with a plain policy is not, and the zero-value policy scans nothing.
Backing: test

### `invariant: scaffold-seeds-all-vars`

ScaffoldConfig seeds every var referenced by templates in the selected catalog view, so each selected unconditional render starts without an unresolved value.
Backing: test

### `invariant: shared-output-coalesced`

An output produced by more than one target at the same path with an identical recipe is coalesced into a single plan node whose declarer set unions the contributing target names, and its drift hash folds in every declarer's projection. Two targets that declare the same path with conflicting recipes fail with a conflicting-output-recipes error.
Backing: test

### `invariant: sidecar-key-overrides-default`

When merging an artifact's catalog default data with its sidecar, a non-list top-level key present in the sidecar - even when set to null or empty - fully replaces the catalog default for that key, while a key absent from the sidecar falls through to the catalog default; there is no deep merge.
Backing: test

### `invariant: catalog-list-data-layering`

A same-key catalog list and project list compose shallowly as catalog entries followed by authored entries, preserving both orders without generic deduplication or identity merging. An absent or empty project list keeps the catalog list; dataDefaults false suppresses that default and yields only authored entries or an empty list, while differently keyed specialized transforms such as glossary standardTerms and terms stay outside this generic path.
Backing: test

### `invariant: target-capabilities-closed`

A target descriptor is validated against closed sets: unknown capabilities, unknown agent dialects, unknown output encoders, out-of-set provenance values, path traversal in output paths, and undeclared or inconsistent output policies are all rejected, both when the descriptor is validated and again when the output plan is built.
Backing: test


### `invariant: conditional-unit-single-source`

Each config-tree render unit derives its enablement, path, template identity, render kind, and fixed
sections from one bounded descriptor consumed by output declarations and render dispatch. Hook
payloads and the runner are unconditional members, while bootstrap is the only member whose
enablement is conditional. Unit-specific data construction, policy, encoding, and lifecycle behavior
remain at their owning render seams.
Backing: test
