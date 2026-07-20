The project package assembles the full render set, computes the output plan and config hash, checks drift, and prunes stale outputs. The claims below capture the current output-plan and render-orchestration contracts.

## Claims

### `invariant: absent-var-acknowledged`

An absent vars key never produces an unset-var completeness note, while a present key with an empty or null value does.
Origin: ADR-0087
Backing: test

### `invariant: adr-system-singletons-rendered`

A full render emits docs/decisions/README.md and docs/decisions/template.md from their always-on singletons, and omits either one when its sidecar sets local: true.
Origin: ADR-0021
Backing: test

### `invariant: authoring-comment-inplace-inert`

An in-place section body read back from rendered output is never subject to the authoring-comment strip: a directive-shaped line inside an in-place region survives re-render byte for byte.
Origin: ADR-0121
Backing: test

### `invariant: authoring-comment-stripped`

A whole-line awf:comment directive in a template source, an include partial, or a convention part never appears in rendered output, across every render unit.
Origin: ADR-0121
Backing: test

### `invariant: awf-bak-flagged`

A collision-backup file under .awf whose name ends in .awf-bak or .awf-bak.<N>, outside the memory directory, is reported by awf check as drift with a distinct stale-backup detail rather than passing silently.
Origin: ADR-0086
Backing: test

### `invariant: bootstrap-config-tree-path`

When the bootstrap singleton is enabled it renders at .awf/bootstrap.sh, and no rendered output path is the retired repo-root awf-bootstrap.sh location.
Origin: ADR-0047
Backing: test

### `invariant: bootstrap-two-files`

With the bootstrap singleton enabled, exactly two files render under it, `.awf/bootstrap.sh` and `.awf/upgrade.sh`, and no third file joins the unit.
Origin: ADR-0085
Backing: test

### `invariant: catalog-data-in-confighash`

A change to an artifact's catalog default data changes that artifact's lock configHash, so `awf check` reports the artifact stale exactly as it would for a template change.
Origin: ADR-0045
Backing: test

### `invariant: catalog-trim-applied`

A non-nil catalog-trim selection passed to ScaffoldConfig replaces the curated-core skills and docs enable arrays verbatim before closure completion, while a nil selection keeps exactly the curated core.
Origin: ADR-0029
Backing: test

### `invariant: check-active-md-stale`

awf check regenerates the ADR status index at docs/decisions/INDEX.md from the current ADR frontmatter and reports it as stale drift when the on-disk file differs, for example after an ADR's status changes without a re-sync; a synced, unchanged index produces no drift.
Origin: ADR-0005
Backing: test

### `invariant: check-invalid-frontmatter`

awf check reports an invalid-frontmatter drift entry for an on-disk skill or agent file that is otherwise in sync but whose frontmatter is missing, unparseable, or has an empty name or description; a clean synced tree reports no such entry, and at most one drift entry is reported per path.
Origin: ADR-0006
Backing: test

### `invariant: closed-config-tree`

Every filesystem entry under .awf that falls outside the claimed-path model, with the memory directory exempt, is reported by awf check as failing orphaned drift.
Origin: ADR-0086
Backing: test

### `invariant: curated-init-skill-refs-clean`

A default curated awf init render passes awf check with zero dead-skill-reference findings.
Origin: ADR-0046
Backing: test

### `invariant: cursor-no-bridge`

The cursor target has an empty bridge-file path and emits no bridge file; its rendered skill and agent files are byte-identical in body and frontmatter to the claude target's files at their respective paths.
Origin: ADR-0037
Backing: test

### `invariant: domain-doc-regenerated`

awf check regenerates each enabled domain document from current state and reports it stale when the on-disk copy diverges, so adding a topic to a domain without re-syncing is detected rather than passing silently.
Origin: ADR-0014
Backing: test

### `invariant: domains-dir-given`

The layout's domains directory is computed as <docsDir>/domains.
Origin: ADR-0013
Backing: test

### `invariant: drift-source-set`

Each rendered file's stored ConfigHash is a per-target projection over only that file's own effective inputs (the skeleton fields it reads, its sidecar, and its consumed parts), so awf check reports a file stale only when one of its own inputs changed since the last sync and never flags unrelated targets; a sidecar or part file matching no enabled or declared target is reported as an orphan.
Origin: ADR-0009
Backing: test

### `invariant: escaped-placeholder-literal`

A backslash placed immediately before an awf placeholder-token opener in a convention part renders the literal token with the backslash consumed, triggering neither placeholder substitution nor the residual-token guard error.
Origin: ADR-0058
Backing: test

### `invariant: hook-payloads-rendered`

With the hooks singleton enabled, exactly three payloads render at .awf/hooks/pre-commit.sh, .awf/hooks/commit-msg.sh, and .awf/hooks/pre-push.sh; with it absent or disabled, no path under .awf/hooks/ renders.
Origin: ADR-0048
Backing: test

### `invariant: in-place-readback`

On both sync and check, an in-place-editable section's body is read back from the existing output file between its awf:edit-in-place pointer and awf's next registered section pointer, matched by that pointer's exact expected string rather than any pointer-shaped adopter line, or to end-of-file when it is the last section; when the output is absent or the pointer is missing, the body falls back to the template default.
Origin: ADR-0100
Backing: test

### `invariant: in-place-spacing-owned`

An in-place region's interior, including its internal blank lines, is spliced back verbatim while only the leading and trailing blank framing is regenerated to a fixed form, so a sync followed by a check is an idempotent fixpoint that reports no drift on unedited whitespace.
Origin: ADR-0100
Backing: test

### `invariant: in-place-tamper-drift`

A file with an in-place-editable section is drift-checked by regenerating every awf-owned section and the file structure from the template, so an edit to an awf-owned region or the structure surfaces as drift and is overwritten, while an edit confined to an in-place section's content lines is preserved and reports clean.
Origin: ADR-0100
Backing: test

### `invariant: inert-sidecar-field-rejected`

Every gated command fails at project open when a non-domain sidecar carries a non-empty paths field, or a domain sidecar carries any non-paths field such as data, sections, or local, with a message naming the file and the required edit.
Origin: ADR-0086
Backing: test

### `invariant: kind-dispatch-single-table`

Every per-kind facet - the config enable array, catalog pool, declared sections, output path, and singular and plural labels - resolves through a single ordered kind-descriptor table in the project package, and a test asserts that table's kind set equals the catalog's kinds plus the freeform domains kind, so adding a catalog kind without a descriptor entry fails.
Origin: ADR-0027
Backing: test

### `invariant: layout-derivation`

The decisions directory, ADR index file, and plans directory are derived structurally from the configured docsDir rather than being independently configurable, so setting docsDir to documentation resolves them under documentation/decisions, documentation/decisions/INDEX.md, and documentation/plans.
Origin: ADR-0005
Backing: test

### `invariant: layout-docs-enabled-only`

The layout docs map contains exactly the enabled doc names, each mapping to <docsDir>/<name>.md, and no other keys.
Origin: ADR-0013
Backing: test

### `invariant: local-catalog-clone`

Opening a project synthesizes its declared local skill and agent entries into a clone of the standard catalog, never mutating the shared standard catalog's skills or agents maps.
Origin: ADR-0068
Backing: test

### `invariant: local-doc-catalog-clone`

Local doc entries are synthesized into a clone of the standard catalog's Docs map, so opening a project never mutates the shared package-global catalog.
Origin: ADR-0091
Backing: test

### `invariant: local-doc-map-fields`

A synthesized local doc entry always carries a non-empty Title and Desc, lifted from its declaring sidecar or defaulted to a name-derived title and a generic description when the sidecar omits them, so the document map lists it with a non-empty title and description.
Origin: ADR-0091
Backing: test

### `invariant: local-doc-no-shadow`

A local, non-standard doc name equal to a name in the standard catalog's Docs is rejected rather than shadowing the standard doc.
Origin: ADR-0091
Backing: test

### `invariant: local-doc-renders-from-base`

A rendered local doc resolves its template id through the effective catalog to the shared base doc template, not to a name-derived or empty path.
Origin: ADR-0091
Backing: test

### `invariant: local-doc-requires-declaration`

An enabled doc name that is neither a standard catalog doc nor a local: true doc, and that has no declaring sidecar, is a hard error when the project is opened.
Origin: ADR-0091
Backing: test

### `invariant: local-frontmatter`

A declared local skill or agent has its on-disk frontmatter validated by sync and check at its conventional output path: a missing or empty name or description fails exactly as a rendered target would, and an absent file for a declared local target is an error.
Origin: ADR-0009
Backing: test

### `invariant: local-no-shadow`

A local (non-standard) skill or agent whose name equals a standard-catalog name is rejected.
Origin: ADR-0068
Backing: test

### `invariant: local-renders-from-base`

A rendered local artifact resolves its template id to the shared base template for its kind rather than to a name-derived template path.
Origin: ADR-0068
Backing: test

### `invariant: local-requires-declaration`

An enabled skill or agent name that is neither in the standard catalog nor declared with local: true by a sidecar is a hard error when the project is opened.
Origin: ADR-0068
Backing: test

### `invariant: memory-gitignore-always-on`

Every `awf sync` unconditionally renders `.awf/memory/.gitignore` with no config gate, lock-tracked, whose content ignores everything in the directory except the gitignore itself and carries a hash-comment provenance banner.
Origin: ADR-0069
Backing: test

### `invariant: multi-target-render`

With multiple targets enabled, each adapter artifact (skill, agent) renders once per target to that target's descriptor-derived paths (for example .claude/skills/<prefix>-<name>/SKILL.md and .cursor/skills/<prefix>-<name>/SKILL.md), while neutral artifacts such as AGENTS.md render exactly once regardless of target count.
Origin: ADR-0037
Backing: test

### `invariant: output-plan-complete`

The single deterministic output plan contains every producer class: catalog and local skills, target-owned bridge files, neutral singletons such as the memory-directory ignore file, generated index and domain docs, and the generated config-reference with its non-self dependencies, plus pre-render reservation nodes for skills. Reservation nodes are excluded from the files actually written.
Origin: ADR-0124
Backing: test

### `invariant: output-policy-explicit`

Post-processing of each output, frontmatter validation, link scanning, and skill-reference scanning, is selected by that output's declared policy rather than its file suffix. A non-Markdown path with a Markdown policy is still validated and scanned, a Markdown-looking path with a plain policy is not, and the zero-value policy scans nothing.
Origin: ADR-0124
Backing: test

### `invariant: part-placeholder-sandboxed`

A `{{=awf:key}}` placeholder in a convention part is resolved by literal substitution against a closed registry of config-derived values, never through the template engine; an unknown or empty key, or any residual `{{=awf` token surviving substitution, is a hard render error that fails both `awf sync` and `awf check`.
Origin: ADR-0057
Backing: test

### `invariant: part-scopes-in-confighash`

A raw convention-part body referencing a `{{=awf:commitScope...}}` placeholder folds the resolved scope data into its artifact's config hash, so editing `audit.allowedScopes` flags that artifact stale in `awf check` while a non-referencing part stays in sync.
Origin: ADR-0057
Backing: test

### `invariant: pitfall-adr-link-resolved`

check fails a pitfall entry whose related list names an ADR number with no matching file under docs/decisions/.
Origin: ADR-0099
Backing: test

### `invariant: pitfall-data-validated`

check fails on unparseable docs/pitfalls.yaml data and on any entry with a non-string or empty or newline-bearing title, a missing or non-string or empty body, or a malformed domains, related, or tags field; the transform that renders docs/pitfalls.md is a hard error on the same malformed data.
Origin: ADR-0099
Backing: test

### `invariant: pitfall-domains-resolved`

check fails a pitfall entry whose domains list names a domain not configured in the project; an entry with no domains is valid and never surfaces through context.
Origin: ADR-0099
Backing: test

### `invariant: placeholder-value-token-free`

Building the placeholder registry returns a hard error naming the offending key when any registry value itself contains an awf placeholder token.
Origin: ADR-0058
Backing: test

### `invariant: plain-singleton-via-renderkind`

The always-on plain singletons (adr-readme, adr-template, plans-readme, workflow, doc-standard, agents-md-standard, and working-with-awf) each render to their fixed output path and content through the shared plainSingletons table and the common renderKind path rather than a hand-rolled per-kind loop.
Origin: ADR-0043
Backing: test

### `invariant: provenance-banner`

Every rendered file begins with the awf generated-by banner as its first line, except that it follows a leading construct where one exists: the closing frontmatter delimiter for targets carrying frontmatter, and the shebang line for shell hooks.
Origin: ADR-0015
Backing: test

### `invariant: regeneration-checked-attribute`

The files excluded from the frozen-output-hash comparison are exactly those a first-class RegenChecked attribute marks on the rendered-file model; the generated index, the config reference, and the domain docs carry it, as does every file containing an in-place-editable section, replacing the former hardcoded path list.
Origin: ADR-0100
Backing: test

### `invariant: residue-exemptions-pinned-three`

The identity-exemption list for the rendered-output residue scan contains exactly three entries: the bootstrap template, the upgrade-script template, and the agents-doc template; extending it requires a successor decision.
Origin: ADR-0131
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

### `invariant: scopes-in-confighash`

The resolved commit-scope list folds into the config hash of every artifact whose assembled template references `.commitScopes`, so editing `audit.allowedScopes` flags exactly those artifacts stale in `awf check` while non-referencing artifacts stay in sync.
Origin: ADR-0051
Backing: test

### `invariant: section-orphan-flagged`

A convention part at docs/parts/<name>/<section>.md whose section is not among the enabled target's catalog-declared sections is reported by check as orphaned drift, while a part at a genuinely declared section is not.
Origin: ADR-0011
Backing: test

### `invariant: section-source-exclusive`

No section is both part-overridable and in-place-editable; a template section declared in-place that also has a matching convention part is a render error rather than a precedence resolution.
Origin: ADR-0100
Backing: test

### `invariant: shared-output-coalesced`

An output produced by more than one target at the same path with an identical recipe is coalesced into a single plan node whose declarer set unions the contributing target names, and its drift hash folds in every declarer's projection. Two targets that declare the same path with conflicting recipes fail with a conflicting-output-recipes error.
Origin: ADR-0124
Backing: test

### `invariant: shebang-rendered-executable`

A rendered file whose content begins with a shebang is written with executable mode 0755 and every other rendered file with 0644; the mode follows the shebang predicate and is re-enforced on every sync, correcting a pre-existing file's mode rather than only setting it at creation.
Origin: ADR-0100
Backing: test

### `invariant: sidecar-key-overrides-default`

When merging an artifact's catalog default data with its sidecar, a top-level key present in the sidecar - even when set to null or empty - fully replaces the catalog default for that key, while a key absent from the sidecar falls through to the catalog default; there is no deep merge.
Origin: ADR-0045
Backing: test

### `invariant: singleton-kinds-complete`

The runner is a dedicated config-tree render block rather than a catalog docs entry, so it is excluded from the singleton-kind set, and the unified-doc-model completeness check asserts that set equals exactly the mandatory doc entries.
Origin: ADR-0101
Backing: test

### `invariant: skill-ref-dead-fails`

awf check fails when a managed rendered artifact references a known skill name via its prefix-anchored token while that skill is not in the effective rendered set.
Origin: ADR-0046
Backing: test

### `invariant: skill-ref-unknown-ignored`

A prefix-anchored token whose trailing word matches no catalog or local skill name produces no dead-skill-reference finding.
Origin: ADR-0046
Backing: test

### `invariant: skills-context-effective-set`

The skills set exposed to templates equals the enabled skills minus those suppressed by a doc gate, while skills declared local are always kept.
Origin: ADR-0046
Backing: test

### `invariant: skills-set-in-confighash`

Changing the skills enable array changes the lock config hash of every artifact whose assembled template references the skills set, so awf check flags those artifacts stale.
Origin: ADR-0046
Backing: test

### `invariant: stub-notes-path-keyed`

The unauthored-content advisory reports one entry per rendered output path, so artifacts that share a template id, including per-target local artifacts and domain docs, each report independently.
Origin: ADR-0070
Backing: test

### `invariant: sync-always-writes-active-md`

awf sync writes the ADR status index at docs/decisions/INDEX.md for every decisions directory, recording it in the lock when the directory holds ADRs and rendering a placeholder index when it holds none.
Origin: ADR-0131
Backing: test

### `invariant: sync-backs-up-foreign`

During `awf sync`, a target path that already exists on disk but is not recorded as awf-written in the lock at the start of the sync is copied to a free `.awf-bak` sibling and reported before being overwritten, while a path recorded in that lock is overwritten with no backup.
Origin: ADR-0035
Backing: test

### `invariant: target-capabilities-closed`

A target descriptor is validated against closed sets: unknown capabilities, unknown agent dialects, unknown output encoders, out-of-set provenance values, path traversal in output paths, and undeclared or inconsistent output policies are all rejected, both when the descriptor is validated and again when the output plan is built.
Origin: ADR-0124
Backing: test

### `invariant: target-prune-ancestors`

Removing a target from the config and re-syncing deletes that target's rendered files and every resulting empty ancestor directory, not only the immediate parent.
Origin: ADR-0037
Backing: test

### `invariant: topic-output-complete`

Every valid topic input has one rendered topic document and participates in its domain's generated topic index, output plan, lock manifest, drift check, and prune behaviour.
Origin: ADR-0134
Backing: unbacked
Verify: Creating and removing a topic in a render fixture changes awf sync, awf check, the output plan, the lock, the index, and stale-output pruning consistently.

### `invariant: uninstall-removes-lock-entries`

awf uninstall removes the in-tree files recorded in the lock and no file outside it, reporting the count it removed.
Origin: ADR-0131
Backing: test

### `invariant: unused-data-drift`

A sidecar data key with no matching .data reference in its artifact's assembled sources, unioned across all enabled targets, is reported by awf check as failing drift keyed to the sidecar file.
Origin: ADR-0086
Backing: test

### `invariant: unused-var-drift`

A non-empty vars key referenced by no assembled template source and by no gate- or check-command placeholder in any consumed convention part is reported by awf check as failing drift; empty-valued vars keys never are.
Origin: ADR-0086
Backing: test

### `invariant: working-with-awf-mandatory`

The working-with-awf doc renders as an always-on singleton for every project, present in the plain-singleton set and the catalog's singleton kinds, and is suppressible only by a local: true sidecar.
Origin: ADR-0059
Backing: test
