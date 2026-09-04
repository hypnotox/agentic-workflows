The migrate package executes supported live schema advances, while audit decodes represented historical schemas read-only. These claims capture the live migration seam and lock contracts.

## Claims

### `invariant: retired-keys-forward-ported`

Audit alone owns historical config decoding and any required read-only translation within its managed history horizon. Live and staged source operations parse only supported live authority and never forward-port historical bytes.
Backing: unbacked
Verify: Inspect live config entry points for direct supported-schema decoding and audit history loading for isolated historical translation; no live operation may call the historical decoder.

### `invariant: corrupt-lock-refuses`

A present live .awf/awf.lock must be one complete closed JSON object with unique fields and a nonempty, well-formed permanent file inventory. Every live lock reader treats a violation as a hard error; in particular Publisher refuses before writing any file, so a corrupt lock cannot authorize overwrite, prune, or lock replacement. Lock absence remains the distinct first-adoption case.
Backing: test

### `invariant: lock-atomic-save`

Ordinary render completely replaces the lock through the selected tracked root-confined handle. Each file replacement is atomic, but a Git-backed multi-file mutation is not cross-file transactional. A supported schema advance publishes its replacement lock last after earlier ordered effects succeed.
Backing: test

### `invariant: migration-ordering`

awf upgrade applies only registered migrations for supported live sources whose target generation exceeds the source generation, in ascending target order. The registry begins with the schema-50 no-op seam and advances through schema 53; re-running at the current schema applies nothing and exits zero.
Backing: test

### `invariant: migration-mutation-safe`

Migration planning uses retained no-follow entries and proves the minimum ownership needed for every replacement or removal. Before the first write it validates the original authority-lock image and completely preflights every collision and destructive path the plan can derive. Regular-file changes preserve content and mode, creation is exclusive, final symlinks refuse, and planned directories are removed only when empty. Application proceeds in order, stops on the first failure, leaves earlier successful effects visible, and reports affected paths for inspection and ordinary rerun. A multi-step planning failure reports completed step descriptions as planned, not applied changes.
Backing: test

### `invariant: context-skill-source-migration`

The schema-51 migration renames authored `repository-context` skill sidecars and declared section parts to `context`, preserving content and mode. It removes an equivalent duplicate target but refuses conflicting old and new sources before mutation.
Backing: test

### `invariant: skill-extraction-source-migration`

The schema-52 migration renames retained AWF skill sidecars and declared section parts to their fixed `awf-*` identities, preserving content and mode and collapsing equivalent destinations. It refuses conflicting sources and removes retired generic-skill and generic-role authored sources only after complete preflight proves each destructive source is a stage-0 tracked regular file unchanged from HEAD; otherwise it refuses before mutation. The final sync publishes only the fixed AWF skills and prunes retired managed generic outputs.
Backing: test

### `invariant: workflow-surface-source-migration`

The schema-53 migration retires AWF's duplicate maintainable-code-design singleton and renames the working-with-awf section identity from `model-selection` to `advanced-workflow`. Obsolete model-policy content is never applied to the new section. It removes retired authored sources only after complete preflight proves each source is a stage-0 tracked regular file unchanged from HEAD; otherwise it refuses before mutation, and obsolete ancestors are removed only when empty.
Backing: test

### `invariant: schema-min-version`

Every supported live config-schema generation is paired with a minimum binary version in a lookup table, and the current schema generation always has an entry. Retired historical schemas have no live minimum-version authority. The binary's own version is never below the minimum recorded for the current schema generation.
Backing: test

### `invariant: schema-version-lock`

The lock file carries an integer schemaVersion, and sync stamps the current highest registered supported migration target. awfVersion remains an independent tool release string.
Backing: test

### `invariant: upgrade-gate`

Live operations refuse a source below schema 50, a retired layout, partial authority, or a future schema before decoding, dispatch, or mutation with the supported floor and upgrade direction. Only upgrade may execute an ordered supported migration within the temporary schema-50-through-53 bridge; ordinary render, check, and staged operations never use historical decoding.
Backing: test

### `rule: live-source-compatibility-floor`

Live project authority begins at schema generation 50. The temporary schema-50-through-53 bridge remains until the external managed-adopter rollout permits removal, and its retention does not advance the floor. A below-floor, retired-layout, partial, or future authority refuses before decoding or dispatch; historical parsing is audit-only and cannot authorize live migration.
