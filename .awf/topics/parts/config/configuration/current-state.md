These packages load, validate, and describe the .awf config tree and the anchored path-glob dialect it uses. The claims below capture the current configuration contracts.

## Claims

### `invariant: sidecar-authoring-roundtrip`

Sidecar leaf authoring mutates YAML nodes through configuration-owned two-space encoding, preserves unrelated ordering and comments, compares list entries structurally, and removes empty ancestor mappings and a final empty sidecar. Scalar modes write strings while JSON modes retain one complete structured JSON value.
Backing: test

### `rule: config-expresses-repo-facts-only`

Configuration records repository facts, including the selected closed governance footprint under the required `profile` key. Other awf behavior preferences and transitional adoption differences remain fixed in awf; additive `localDocs` records repository-specific document metadata without selecting standard artifacts.

### `invariant: no-artifact-selection-surface`

The live config schema exposes no selection of individual skills, agents, docs, targets, or docsDir fields and no sidecar local field; strict parsing rejects every retired field. The required profile selects one closed footprint, while `localDocs` only declares additive outputs.
Backing: test

### `invariant: local-doc-declarations`

`localDocs` accepts only unique lowercase kebab-case docs-relative names, titles, and one-line descriptions; its normalized projection sorts by name without rewriting authored list order, reserves generated documentation roots, and leaves an absent list valid.
Backing: test

### `invariant: root-sidecar-keys-rejected`

Working-tree and snapshot config and every sidecar contain exactly one complete known-field YAML document. Strict decoding rejects a second document, trailing non-comment content, and data or sections at the root of config.yaml because those keys belong only in sidecars.
Backing: test

### `invariant: audit-no-base-branch-config`

No config field, configspec entry, or resolved audit setting supplies an audit base branch; the audit range reaches the audit only from the command line.
Backing: test

### `invariant: awf-config-root`

Configuration loads from `.awf/config.yaml` and the lock is read from and written to `.awf/awf.lock`; no live operation reads or writes either retired `.claude/awf/` layout. Retired layouts are recognized only to refuse them before decoding.
Backing: test

### `invariant: config-mutation-roundtrip`

SetArrayMember edits config.yaml through a yaml.Node round-trip rather than line or string surgery, preserving comments and unrelated formatting, and accepts both block-style and flow-style input arrays while normalizing the edited sequence to block style.
Backing: test

### `invariant: config-serialization-owned`

The live .awf/config.yaml is constructed through internal/config via MarshalSkeleton and mutated through its scalar and mapping editors, while frozen migrations retain the array editors for historical shapes; all share one encoding funnel at a two-space indent, so no other package hand-rolls config.yaml serialization.
Backing: test

### `invariant: integration-branch-explicit`

The config carries a required integrationBranch key with no in-code default: validation rejects an absent or empty value, a value containing whitespace, and a value starting with a hyphen while accepting a slashed branch name, its schema migration writes integrationBranch: main visibly into a config that lacks it and leaves a config that already carries one byte-identical, and a freshly scaffolded config writes the key so it validates against its own rules.
Backing: test

### `invariant: no-replacewith`

A section-override sidecar exposes no replaceWith field: the strict config decoder rejects a sections entry carrying replaceWith, so a convention part is the only mechanism that replaces a section body.
Backing: test

### `invariant: remove-block-scoped`

Removing a member from a mapping key affects only that key's own block sequence. When two keys each hold an identically named item, removing the item from one key empties or shortens that key's sequence and leaves the item under the other key untouched.
Backing: test

### `invariant: scope-config-dual-form`

The audit.allowedScopes list decodes both a bare-string element and a {name, meaning} mapping element in the same list; resolution yields the name for gating regardless of form, and the meaning is empty for the bare-string form and for a mapping that omits it.
Backing: test

### `invariant: severity-not-configurable`

The currentState configuration exposes no severity setting: no configuration value ranks or suppresses a produced topic coverage or topic fan-out finding, a requested coverage finding always reports at error and a requested fan-out finding always at warn, and a tree carrying a currentState.topicCoverage or currentState.topicFanout key is rejected by strict parsing rather than honoured.
Backing: test

### `invariant: sidecar-data-defaults-control`

A sidecar dataDefaults map accepts only boolean controls whose keys name same-key list defaults declared by that catalog artifact. Absence or true keeps the default and false suppresses it; unknown, non-list, differently keyed specialized, and non-boolean entries are rejected, and a present catalog-backed project list value must be a list rather than null or another type.
Backing: test

### `invariant: template-source-root`

The optional `render.templateSourceRoot` is a normalized repository-relative directory fact. When present it enables Markdown template-source symbols only after every emitted root or included source resolves as a regular file in the selected working or staged repository tree; when absent scaffolded configuration and generated adopter bytes remain unchanged.
Backing: test

### `invariant: no-active-tag-system`

The live config schema exposes no `tags` key, and strict decoding rejects the retired field after migration.
Backing: test
