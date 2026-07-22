These packages load, validate, and describe the .awf config tree and the anchored path-glob dialect it uses. The claims below capture the current configuration contracts.

## Claims

### `invariant: audit-no-base-branch-config`

No config field, configspec entry, or resolved audit setting supplies an audit base branch; the audit range reaches the audit only from the command line.
Origin: ADR-0127
Backing: test

### `invariant: awf-config-root`

Configuration loads from `.awf/config.yaml` and the lock is read from and written to `.awf/awf.lock`; no ordinary load, render, sync, or check path reads or writes under `.claude/awf/`, and only the migrate package reads the legacy `.claude/awf.yaml` when porting an older tree forward.
Origin: ADR-0016
Backing: test

### `invariant: config-mutation-roundtrip`

SetArrayMember edits config.yaml through a yaml.Node round-trip rather than line or string surgery, preserving comments and unrelated formatting, and accepts both block-style and flow-style input arrays while normalizing the edited sequence to block style.
Origin: ADR-0026
Backing: test

### `invariant: config-serialization-owned`

The live .awf/config.yaml is constructed and mutated only through internal/config via MarshalSkeleton, SetArrayMember, SetArray, SetMappingScalar, and the typed nested-integer SetMappingInteger editor, which share one encoding funnel at a two-space indent, so no other package hand-rolls config.yaml serialization.
Origin: ADR-0026
Revised-by: ADR-0144
Backing: test

### `invariant: docsdir-default`

The config carries a docsDir field; loading a config file that omits it defaults docsDir to docs, and setting an explicit value relocates the documentation tree and every path derived from it to that root.
Origin: ADR-0005
Backing: test

### `invariant: enable-arrays`

The skills, agents, and docs keys in config.yaml are plain string arrays whose entries enable targets by presence, and a data, sections, or local key placed at the root of config.yaml is rejected at load.
Origin: ADR-0009
Backing: test

### `invariant: topic-claim-budget-configured`

The positive currentState.maxClaimsPerTopic setting has an effective default of 20, is explicitly serialized by scaffold and schema migration, and is exposed consistently through strict config parsing, configspec, generated reference state, render hashing, and lock inputs.
Origin: ADR-0144
Backing: test

### `invariant: no-replacewith`

A section-override sidecar exposes no replaceWith field: the strict config decoder rejects a sections entry carrying replaceWith, so a convention part is the only mechanism that replaces a section body.
Origin: ADR-0015
Backing: test

### `invariant: remove-block-scoped`

Removing a member from a mapping key affects only that key's own block sequence. When two keys each hold an identically named item, removing the item from one key empties or shortens that key's sequence and leaves the item under the other key untouched.
Origin: ADR-0024
Backing: test

### `invariant: scope-config-dual-form`

The audit.allowedScopes list decodes both a bare-string element and a {name, meaning} mapping element in the same list; resolution yields the name for gating regardless of form, and the meaning is empty for the bare-string form and for a mapping that omits it.
Origin: ADR-0056
Backing: test

### `invariant: tag-coverage-note`

Under a non-empty tag vocabulary, awf check emits a non-failing note for each ADR and each pitfall that carries zero tags and for no tagged artifact, never changing the exit code; an empty or absent vocabulary is inert.
Origin: ADR-0109
Backing: test

### `invariant: tag-frequency-note`

Under a non-empty tag vocabulary, awf check emits a non-failing note for each vocabulary tag carried by strictly more than 25 percent of the artifacts that carry at least one vocabulary tag, and for no tag at or below that share, without changing the exit code.
Origin: ADR-0109
Backing: test

### `invariant: tag-vocabulary-governed`

With a non-empty tag vocabulary, awf check fails on any tag used by an ADR or a pitfall that is not a declared vocabulary member and on any vocabulary entry whose meaning is empty; with an empty or absent vocabulary the membership rule is inert.
Origin: ADR-0103
Backing: test

### `invariant: targets-default-claude`

A config with no targets key loads with targets defaulting to [claude]; Validate rejects an empty targets list and any path-separator name, while an unknown adapter name is rejected later by project open, keeping config free of the adapter registry.
Origin: ADR-0037
Backing: test

### `invariant: workflow-telemetry-settings`

The strict tracked workflowTelemetry block carries retention, widget, heuristic baseline, and threshold settings with complete scaffolded defaults; omission receives effective defaults, explicit valid leaves are preserved, retention zero disables its dimension, and all other numeric bounds follow the documented contract.
Origin: ADR-0146
Backing: test
