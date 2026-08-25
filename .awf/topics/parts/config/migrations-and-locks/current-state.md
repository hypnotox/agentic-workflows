The migrate package executes supported live schema advances, while audit decodes represented historical schemas read-only. These claims capture the live migration seam and lock contracts.

## Claims

### `invariant: retired-keys-forward-ported`

Audit alone owns historical config decoding and any required read-only translation within its managed history horizon. Live and staged source operations parse only supported live authority and never forward-port historical bytes.
Origin: ADR-0251
Revised-by: ADR-0270, ADR-separate-live-upgrade-support-from-historical-audit-decoding
Backing: test


### `invariant: corrupt-lock-refuses`

A present-but-unreadable .awf/awf.lock causes a hard error in every lock reader; in particular the sync report refuses before writing any file, so a corrupt lock can never create a backup, skip a prune, or be overwritten.
Origin: ADR-0076
Backing: test


### `invariant: lock-atomic-save`

Ordinary render loads and completely replaces the lock through the selected tracked root-confined handle. A supported schema advance retains journaled rollback and publishes its lock replacement last, so recovery never exposes partial authority.
Origin: ADR-0076
Revised-by: ADR-0269, ADR-separate-live-upgrade-support-from-historical-audit-decoding
Backing: test


### `invariant: migration-ordering`

awf upgrade applies only registered migrations for supported live sources whose target generation exceeds the source generation, in ascending target order. The explicit ordered seam begins at schema 46; re-running at the current schema applies nothing and exits zero.
Origin: ADR-0010
Revised-by: ADR-separate-live-upgrade-support-from-historical-audit-decoding
Backing: test


### `invariant: schema-min-version`

Every supported live config-schema generation is paired with a minimum binary version in a lookup table, and the current schema generation always has an entry. Retired historical schemas have no live minimum-version authority. The binary's own version is never below the minimum recorded for the current schema generation.
Origin: ADR-0049
Revised-by: ADR-separate-live-upgrade-support-from-historical-audit-decoding
Backing: test


### `invariant: schema-version-lock`

The lock file carries an integer schemaVersion, and sync stamps the current highest registered supported migration target. awfVersion remains an independent tool release string.
Origin: ADR-0010
Revised-by: ADR-0278, ADR-separate-live-upgrade-support-from-historical-audit-decoding
Backing: test


### `invariant: upgrade-gate`

Live operations refuse a source below schema 46, a retired layout, or partial authority before decoding, dispatch, or mutation with the supported floor and recovery direction. Only upgrade may execute an ordered supported migration from that floor; ordinary render, check, and staged operations never use historical decoding.
Origin: ADR-0010
Revised-by: ADR-0159, ADR-separate-live-upgrade-support-from-historical-audit-decoding
Backing: test


### `rule: live-source-compatibility-floor`

Live project authority begins at schema generation 46. A below-floor, retired-layout, or partial authority refuses before decoding or dispatch with recovery direction; historical parsing is audit-only and cannot authorize live migration.
Origin: ADR-0297
Revised-by: ADR-separate-live-upgrade-support-from-historical-audit-decoding
