The migrate package executes supported live schema advances, while audit decodes represented historical schemas read-only. These claims capture the live migration seam and lock contracts.

## Claims

### `invariant: retired-keys-forward-ported`

Audit alone owns historical config decoding and any required read-only translation within its managed history horizon. Live and staged source operations parse only supported live authority and never forward-port historical bytes.
Backing: unbacked
Verify: Inspect live config entry points for direct supported-schema decoding and audit history loading for isolated historical translation; no live operation may call the historical decoder.


### `invariant: corrupt-lock-refuses`

A present live .awf/awf.lock must be one complete closed JSON object with unique fields and a nonempty, well-formed permanent file inventory. Every live lock reader treats a violation as a hard error; in particular Publisher refuses before writing any file, so a corrupt lock cannot create a backup, skip a prune, or be overwritten. Lock absence remains the distinct first-adoption case.
Backing: test


### `invariant: lock-atomic-save`

Ordinary render loads and completely replaces the lock through the selected tracked root-confined handle. A supported schema advance retains journaled rollback and publishes its lock replacement last, so recovery never exposes partial authority.
Backing: test


### `invariant: migration-ordering`

awf upgrade applies only registered migrations for supported live sources whose target generation exceeds the source generation, in ascending target order. The registry begins with the schema-50 no-op seam and advances through supported migrations; re-running at the current schema applies nothing and exits zero.
Backing: test


### `invariant: context-skill-source-migration`

The schema-51 migration renames authored `repository-context` skill sidecars and declared section parts to `context`, preserving content and mode. It removes an equivalent duplicate target but refuses conflicting old and new sources before mutation.
Backing: test


### `invariant: skill-extraction-source-migration`

The schema-52 migration renames retained AWF skill sidecars and declared section parts to their fixed `awf-*` identities, preserving content and mode, collapsing equivalent destinations, and refusing conflicts. Authored overrides for extracted generic skills and removed generic roles move to collision-safe adjacent `.awf-bak` files rather than being discarded or installed externally. The final sync publishes only the fixed AWF skills and prunes retired managed generic outputs.
Backing: test


### `invariant: schema-min-version`

Every supported live config-schema generation is paired with a minimum binary version in a lookup table, and the current schema generation always has an entry. Retired historical schemas have no live minimum-version authority. The binary's own version is never below the minimum recorded for the current schema generation.
Backing: test


### `invariant: schema-version-lock`

The lock file carries an integer schemaVersion, and sync stamps the current highest registered supported migration target. awfVersion remains an independent tool release string.
Backing: test


### `invariant: upgrade-gate`

Live operations refuse a source below schema 50, a retired layout, or partial authority before decoding, dispatch, or mutation with the supported floor and recovery direction. Only upgrade may execute a future ordered supported migration from that floor; ordinary render, check, and staged operations never use historical decoding.
Backing: test


### `rule: live-source-compatibility-floor`

Live project authority begins at schema generation 50. A below-floor, retired-layout, or partial authority refuses before decoding or dispatch with recovery direction; historical parsing is audit-only and cannot authorize live migration.
