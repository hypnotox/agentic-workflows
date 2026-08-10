---
format: plan-v2
date: 2026-08-10
adrs: [archive-finished-efforts-and-permit-effort-scratch-data]
status: Proposed
---
# Plan: Archive Finished Efforts and Add Effort Scratch Space

## Goal

Make `awf effort finish` preserve each complete valid effort in an ignored machine-local archive and
allow one optional opaque effort-local `scratch/` directory, while retaining strict protocol-file
validation, safe failed-creation rollback, and no archive management surface.

## Architecture summary

`internal/resident` and `internal/git` remain the closed authorities for repository-wide resident
roots; output planning and rendering derive the third archive marker from that registry. A registered
config generation gates older projects until ordinary upgrade sync publishes the marker and current
lock. `internal/effort` owns scratch-boundary validation, archive destination identity, no-replace
namespace mutation, durability results, and presentation semantics. `internal/worktree` consumes a
narrow identity-bound deletion rollback for a just-created resident instead of routing failed creation
through public finish. Two independently green transactions land the archive substrate and upgrade
boundary first, then the complete finish, scratch, rollback, CLI, and documentation behavior.

## Phase 1: Add the governed archive resident root and upgrade boundary

**Execution mode: subagent-driven.**

Advances: ["archived-finish", "authority-and-docs"]
Completes: ["archive-root-upgrade"]

### Task 1.1: Extend the closed resident-root registry and marker pipeline
Kind: batch
Latitude: exact
Applying: ["archive-finished-efforts-and-permit-effort-scratch-data:third-resident-root", "archive-finished-efforts-and-permit-effort-scratch-data:archive-ignore-precondition"]
Paths: ["internal/git/controlroot.go", "internal/git/controlroot_test.go", "internal/git/topology_test.go", "internal/git/git_test.go", "internal/git/status_test.go", "internal/resident/resident.go", "internal/resident/resident_test.go", "internal/resident/singlehome_test.go", "internal/effort/paths.go", "templates/embed.go", "templates/embed_test.go", "templates/effort-archive/gitignore.tmpl", "internal/project/render.go", "internal/project/banner.go", "internal/project/output_plan_test.go", "internal/project/project_test.go", "internal/project/install_test.go", "internal/project/memory_test.go", "internal/project/banner_test.go", "internal/project/sweep_test.go", "internal/testsupport/deps_test.go", "cmd/awf/effort_test.go", "internal/worktree/manager_test.go"]
Representative: "Register `effort-archive` once, resolve it to `.awf/effort-archive` under the primary control root, embed its self-ignoring marker template, and let output planning, render, preservation, confinement, and uninstall derive the third root from the shared table."
Edge: "Reject unknown resident names and unsafe archive-root ancestors exactly like the existing roots; preserve dynamic archive descendants without turning them into output-plan nodes, lock-file descendants, source inputs, or recursive uninstall targets."
Post-check: "`go test ./internal/git ./internal/resident ./internal/project ./internal/testsupport ./templates ./internal/worktree ./cmd/awf` exits zero, and `go test ./internal/resident -run '^TestResidentPolicyHasOneHome$' -count=1` exits zero after its tracked-source census rejects every resident-root policy outside the declared single-home exemptions."

Add `ResidentEffortArchive` to the existing closed Git resident-name mapping and `effort-archive` to
the single `internal/resident` root table. Extend effort path composition with the resolved archive root
but do not add archive enumeration or recursive reads. Add the new top-level embedded template with
the exact publication-safe `*` and `!.gitignore` marker contract. Extract the existing resident-marker
rendering path, including template execution and generated hash-comment banner injection, into one
`internal/project` function that the ordinary render loop consumes immediately and that command
composition can later call to obtain the byte-identical planned marker. Keep output-plan and render
behavior derived from `resident.RootNames`; change production consumers only where a direct two-root
assumption prevents that derivation.

Update root-table, primary-control-root, output-plan, render, sweep, install, uninstall, nested-adopter,
repository-walk, and source-discovery tests. Test that only the marker is governed and that arbitrary
archive descendants remain ignored and preserved. Extend the missing-key-zero and empty-variable
template fixtures so the new marker cannot emit unresolved or no-value tokens.

### Task 1.2: Register and prove the archive-root upgrade generation
Kind: batch
Latitude: exact
Applying: ["archive-finished-efforts-and-permit-effort-scratch-data:archive-ignore-precondition", "archive-finished-efforts-and-permit-effort-scratch-data:upgrade-boundary-proof"]
Paths: ["internal/migrate/migrate.go", "internal/migrate/migrate_test.go", "internal/upgrade/upgrade_test.go", "cmd/awf/upgrade_test.go", "cmd/awf/gate_test.go", "internal/project/project.go", "internal/project/version_test.go", "internal/project/project_test.go", "internal/project/output_plan_test.go", "internal/manifest/manifest_test.go"]
Representative: "A project stamped at the predecessor generation is command-gated, `awf upgrade` applies the registered generation and ordinary sync publishes `.awf/effort-archive/.gitignore` plus a current-generation `awf.lock` entry, and current-generation render repairs a missing or stale marker."
Edge: "A current project with a safe correct marker remains unchanged; an older project cannot bypass upgrade through an effort command; interrupted or failed upgrade follows existing journal and lock atomicity instead of claiming the marker or generation was published."
Post-check: "`go test ./internal/migrate ./internal/upgrade ./internal/project ./internal/manifest ./cmd/awf` exits zero; the named proof test under `internal/` bears `config/migrations-and-locks:archive-root-upgrade-boundary` and reaches the terminal gated, upgraded, and current-generation repair states without count-based assertions."

Allocate the next migration generation after rebasing against the integration branch and give it a
stable archive-root name. The migration need not invent authored configuration: its purpose is to
create an effective-generation boundary so command gating forces the ordinary upgrade flow that
renders the new governed output and atomically republishes the lock manifest. Preserve migration
ordering, `ConfigForCurrentSchema` behavior, and current-schema no-op behavior. Add the new generation
to `minVersionBySchema` at the current binary version and extend its invariant proof so schema and
binary compatibility remain closed. Do not add a new lock format, archive protocol, or finish-specific
implicit upgrade.

Prove the complete invariant in one named `internal/` test and retain focused registry, gate, upgrade,
render-repair, and lock-publication tests in their owning packages. Read back the marker and lock after
compound upgrade or render commands before asserting success.

### Task 1.3: Apply archive-root authority and document the substrate
Kind: batch
Latitude: exact
Applying: ["archive-finished-efforts-and-permit-effort-scratch-data:third-resident-root", "archive-finished-efforts-and-permit-effort-scratch-data:archive-ignore-precondition", "archive-finished-efforts-and-permit-effort-scratch-data:upgrade-boundary-proof"]
Paths: ["docs/decisions/archive-finished-efforts-and-permit-effort-scratch-data.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/docs/parts/architecture/overview.md", "templates/docs/working-with-awf.md.tmpl", "docs/topics/config/migrations-and-locks.md", "docs/topics/rendering/singletons-and-payloads.md", "docs/topics/rendering/project-output-plan.md", "docs/working-with-awf.md", "docs/architecture.md", "README.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Enter Implementing and apply the added upgrade invariant plus the three rendering claim updates in the same transaction as the live root, marker, generation, and proof."
Edge: "Do not claim finish archives yet, expose archive commands, describe archived descendants as durable authority, or apply the pending tooling claims before Phase 2 behavior exists."
Post-check: "After `./x render`, `./x check` exits zero and `./awf context --show pending docs/decisions/archive-finished-efforts-and-permit-effort-scratch-data.md internal/resident/resident.go` reports the config add and three rendering updates Applied while all three tooling operations remain pending. Read `README.md`, `docs/working-with-awf.md`, `docs/architecture.md`, and the three rendered destination topics at their changed paragraphs; record that they consistently describe the third self-ignored unmanaged root and older-project upgrade gate, contain no contradictory two-root current-state fragment, and show no unintended unresolved or no-value placeholder."

Use `awf-adr-lifecycle` to move the Proposed ADR through the required nonterminal transition into
Implementing and append one Applied event naming exactly:

- add `config/migrations-and-locks:archive-root-upgrade-boundary`
- update `rendering/singletons-and-payloads:memory-gitignore-always-on`
- update `rendering/singletons-and-payloads:resident-output-preservation`
- update `rendering/project-output-plan:output-plan-complete`

Author the invariant and revised claims in their `.awf/topics/parts/**` sources with the ADR as Origin
or Revised-by and the named test proof. Update generated-root, architecture, and working-with-awf prose
through their owning sources. State that the archive is a third self-ignored local resident root with
no recursive management, and that older generations must upgrade before effort commands run. Do not
hand-edit generated topics or singleton documentation.

### Phase close

Land the root registry, template, generation, proof, current-state transaction, and generated outputs
as one coherent green commit:

```commit
feat(rendering): add effort archive root (applies archive root batch)
```

## Phase 2: Archive valid finished efforts and admit opaque scratch

**Execution mode: subagent-driven.**

Completes: ["archived-finish", "opaque-scratch", "creation-rollback", "authority-and-docs"]

### Task 2.1: Admit exactly one opaque scratch boundary
Kind: batch
Latitude: exact
Applying: ["archive-finished-efforts-and-permit-effort-scratch-data:opaque-effort-scratch"]
Paths: ["internal/effort/store.go", "internal/effort/store_test.go", "internal/effort/safety_test.go", "internal/effort/service_test.go"]
Representative: "An otherwise valid effort may contain an optional direct `scratch/` owned real directory, and show, list, memory, activity, and finish validation accept it without reading any descendant."
Edge: "Reject a direct scratch symlink, regular file, unsafe owner, or renamed foreign leaf; continue rejecting every other unknown direct child, while nested files, directories, symlinks, hard links, and unreadable content remain opaque because production never traverses them."
Post-check: "`go test ./internal/effort` exits zero; focused tests prove acceptance of absent and valid scratch, refusal of every unsafe direct boundary shape without mutation, continued strict protocol-file validation, and successful operations with deliberately uninspectable nested scratch content."

Extend `store.loadDirectory`'s closed direct-child model rather than adding a generic foreign-file
escape. Validate only the `scratch` directory inode with the existing confinement, no-follow,
file-type, and current-owner rules. Never call `ReadDir`, walk, size, parse, clean, or remove a scratch
descendant during ordinary effort validation. Creation does not scaffold scratch and no CLI command
manages it.

### Task 2.2: Replace finishing deletion with no-clobber archival
Kind: batch
Latitude: exact
Applying: ["archive-finished-efforts-and-permit-effort-scratch-data:archived-finish", "archive-finished-efforts-and-permit-effort-scratch-data:stable-archive-identity", "archive-finished-efforts-and-permit-effort-scratch-data:restartable-archive-transition", "archive-finished-efforts-and-permit-effort-scratch-data:archive-ignore-precondition"]
Paths: ["internal/effort/paths.go", "internal/effort/service.go", "internal/effort/store.go", "internal/effort/types.go", "internal/effort/presentation.go", "internal/effort/publication_linux.go", "internal/effort/publication_darwin.go", "internal/effort/publication_windows.go", "internal/effort/publication_other.go", "internal/effort/safeio.go", "internal/effort/service_test.go", "internal/effort/durability_test.go", "internal/effort/platform_test.go", "internal/effort/platform_windows_test.go", "internal/effort/safety_test.go", "internal/effort/types_test.go", "internal/effort/presentation_test.go", "internal/effort/wiring_test.go", "internal/project/render.go", "internal/project/banner.go", "internal/project/banner_test.go", "cmd/awf/effort.go", "cmd/awf/effort_test.go", "templates/embed.go", "templates/effort-archive/gitignore.tmpl"]
Representative: "Finish validates topology, resident, archive root, and exact marker; renames active to its existing finishing reservation; atomically moves that complete directory without replacement to `.awf/effort-archive/<uuid>-<slug>`; syncs both parents under the platform durability contract; and reports the exact archived path."
Edge: "Before the archive move, retry by slug resumes the finishing reservation. A destination collision, cross-device refusal, unsafe or stale marker, identity race, source-parent fault, or destination-parent fault preserves every provable byte and reports whether the effort is active, reserved, or archived plus exact inspection actions. After the archive move, no blind retry is prescribed and the slug is reusable."
Post-check: "`go test ./internal/effort ./cmd/awf` exits zero across normal archive, scratch preservation, slug reuse, finishing restart, existing empty and nonempty destination collisions, unsafe marker and root, namespace races, platform no-replace behavior, and every injected pre-move, post-move, destination-sync, and source-sync failure; tests assert resident bytes and typed facts before presentation prose."

Keep the existing active-to-`.finishing-<uuid>-<slug>` transition as the pre-completion reservation.
Construct the exact archive destination from the validated persisted UUID and slug. Add the smallest
platform publication seam that atomically renames a directory without replacing any existing
entry; reuse existing no-replace primitives where their contract already covers directories, and
refuse rather than copy-and-delete or traverse scratch. Preserve platform honesty: POSIX parent sync
and Windows' documented namespace/write-through limits must be reflected in typed partial outcomes
rather than hidden behind a false universal durability claim.

At command composition, call the Phase 1 `internal/project` resident-marker renderer over the embedded
`templates/effort-archive/gitignore.tmpl` so template execution and provenance-banner injection are
identical to ordinary rendering, then inject those exact fully rendered bytes through
`effort.Dependencies`. `internal/effort` compares supplied bytes without importing rendering or
duplicating template or banner policy. Tests inject explicit marker bytes through this seam, while
`internal/project` parity tests prove the composition result equals the actual planned output.
Before active mutation, prove the archive root and marker against the registered resident root and
those supplied bytes. After the archive move, synchronize both changed parents in a documented order.
Extend `FinishResult`, partial errors, and effort-owned presentation mappings only with facts needed
to distinguish active rename, archive move, parent durability, exact destination, and next actions.
Remove deletion-oriented `Cleaned` claims and activity consumption; activity and scratch move as bytes
inside the validated directory. Keep list/show/select blind to archive residents and add no archive
subcommand or protocol.

### Task 2.3: Preserve deletion only for failed default creation
Kind: batch
Latitude: exact
Applying: ["archive-finished-efforts-and-permit-effort-scratch-data:creation-rollback-is-not-finish"]
Paths: ["internal/effort/service.go", "internal/effort/types.go", "internal/effort/service_test.go", "internal/effort/durability_test.go", "internal/worktree/manager.go", "internal/worktree/manager_test.go", "internal/worktree/wiring_test.go", "cmd/awf/effort.go", "cmd/awf/effort_test.go"]
Representative: "When default worktree Add fails and actual managed topology is proven absent, orchestration deletes only the exact just-created UUID/slug resident through an identity-bound rollback and reports no archive; successful public finish always archives."
Edge: "If topology is present or uncertain, the resident identity changed, rollback publication is interrupted, or deletion cannot be proven safe, retain the effort or reservation with concrete recovery actions and never delete an unrelated reused slug or archive the unsuccessful creation."
Post-check: "`go test ./internal/worktree ./internal/effort ./cmd/awf` exits zero; combined-creation fault tests establish the created identity, actual topology classification, deletion-or-retention result, archive absence, and restart behavior without parsing error prose."

Replace `internal/worktree.Manager.rollback`'s call to public `Finish` with a narrow service operation
that accepts the just-created immutable identity, revalidates it, and uses the existing safe
reservation/removal machinery only for this transaction. Do not expose a general delete command,
force flag, or caller-selected archive bypass. Keep structured rollback outcomes sufficient for
`NewEffort` composition and preserve the current rule that ambiguous topology retains data.

### Task 2.4: Apply tooling authority and update adopter-facing lifecycle prose
Kind: batch
Latitude: exact
Applying: ["archive-finished-efforts-and-permit-effort-scratch-data:archived-finish", "archive-finished-efforts-and-permit-effort-scratch-data:stable-archive-identity", "archive-finished-efforts-and-permit-effort-scratch-data:restartable-archive-transition", "archive-finished-efforts-and-permit-effort-scratch-data:archive-ignore-precondition", "archive-finished-efforts-and-permit-effort-scratch-data:opaque-effort-scratch", "archive-finished-efforts-and-permit-effort-scratch-data:creation-rollback-is-not-finish"]
Paths: ["docs/decisions/archive-finished-efforts-and-permit-effort-scratch-data.md", ".awf/topics/parts/tooling/effort-management/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/parts/working-with-awf/commands.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/docs/parts/architecture/overview.md", ".awf/docs/glossary.yaml", "templates/docs/working-with-awf.md.tmpl", "templates/skills/effort-workflow/SKILL.md.tmpl", "changelog/CHANGELOG.md", "docs/topics/tooling/effort-management.md", "docs/topics/tooling/cli.md", "docs/working-with-awf.md", "docs/architecture.md", "docs/glossary.md", "README.md", "glob:.pi/skills/awf-effort-workflow/**", "glob:.claude/skills/awf-effort-workflow/**", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Apply all three tooling claim updates with the live finish, scratch, rollback, CLI, and proof behavior; generated guidance says finish archives locally and scratch is optional and opaque, without treating either as project authority or offering archive management."
Edge: "Remove every current-state claim that finish deletes or consumes activity and every guide statement that only two roots exist, but preserve the memory-citation ban, one-writer contract, managed-worktree finalization order, manual archive deletion, and absence of archive list, restore, prune, analysis, or retention commands."
Post-check: "After `./x render`, `./x check` exits zero and `./awf context --show pending docs/decisions/archive-finished-efforts-and-permit-effort-scratch-data.md internal/effort/service.go` reports every declared operation Applied. `! rg -n 'finish is restartable deletion|restartable finish deletes|Finish consumes activity only inside proven tombstone deletion|exactly two repository-wide resident roots' README.md AGENTS.md docs/working-with-awf.md docs/architecture.md docs/glossary.md docs/topics .pi/skills/awf-effort-workflow .claude/skills/awf-effort-workflow` exits zero because `rg` finds no match; the confined path set excludes historical ADRs and changelog entries. Read every changed paragraph in those outputs and record that complete archived finish, unchanged opaque scratch, absence of archive management, partial-recovery actions, and intentional placeholder examples retain consistent meanings without contradictory fragments or unintended literal tokens."

Use `awf-adr-lifecycle` to append one Applied event naming exactly:

- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:default-worktree-creation`
- update `tooling/cli:effort-command-contract`

Revise the source claims in the same transaction as their implemented behavior and retain claim history
through `Revised-by`. Update command help and guidance through their owning sources. Define `scratch`
as optional disposable effort-local data and the archive as ignored, unmanaged, non-authoritative,
manually disposable bytes that can still appear in backups or local disclosure. State the exact
finish path and partial-result recovery model. Update the Unreleased changelog for the behavioral and
upgrade compatibility change. Keep the ADR Implementing and the plan Proposed; terminal assurance and
effort finalization own their later status-only closure.

### Phase close

Land scratch validation, platform publication, archived finish, failed-creation rollback, presentation,
current-state claims, generated guidance, and changelog as one coherent green commit:

```commit
feat(tooling): archive finished effort residents (applies finish batch)
```

## Definition of done

- `dod: archive-root-upgrade` Older-generation projects cannot run effort commands until upgrade publishes the third self-ignored archive root and current lock, while current-generation render repairs its governed marker.
- `dod: archived-finish` Finishing a valid topology-free effort preserves its complete unchanged resident at `.awf/effort-archive/<uuid>-<slug>`, releases the slug, refuses collisions without replacement, and reports restart or inspection actions honestly across every mutation boundary.
- `dod: opaque-scratch` A valid effort may contain one optional owned real `scratch/` directory whose descendants are never interpreted or traversed by awf and move unchanged into the archive.
- `dod: creation-rollback` Failed default worktree creation still deletes only its identity-matched just-created resident on proven-absent topology and never archives it or exposes a general deletion API.
- `dod: authority-and-docs` All seven ADR State changes are Applied with named proof backing, generated outputs are drift-free, public guidance and Unreleased notes describe the local disposable semantics, and no archive management surface exists.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated
owners may report rather than edit; the parent supplies the report to phase review and reconciles it
with findings in one focused post-review settlement commit before checkpointing or later execution.
Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- 2026-08-10 plan review: A reasoned finding identified that marker validation had no named single
  source. The first correction used raw embedded bytes, but the verify pass found that it omitted the
  renderer-owned generated banner. Final disposition: Phase 1 extracts one `internal/project`
  resident-marker renderer used by the ordinary render loop; command composition calls that same
  function over the embedded template and injects its fully rendered bytes into `internal/effort`.
  This preserves dependency direction and prevents duplicated template or banner policy. Mechanical
  scope, minimum-version, exact-test-name, subject-length, semantic-review, deterministic-scan, and
  operation-count corrections were applied directly.
- 2026-08-10 Phase 1 implementation deviation: added `internal/effort/paths_test.go` to prove archive-root resolution and unsafe-root refusal required by the phase safety boundary and the 100% coverage gate. The rendered transaction also added the governed `.awf/effort-archive/.gitignore` output required by the phase marker contract.
- 2026-08-10 Phase 1 review settlement: strengthened the named upgrade-boundary proof to execute the real effort-command gate, compound upgrade, lock and marker read-back, unchanged-marker case, and missing and stale repair cases. Derived repository-source exclusions from the resident registry, added archive-descendant preservation coverage across migration, sync, sweep, lock pruning, and uninstall, restored the generation-41 pin beside a dedicated generation-42 pin, and documented the already-live schema and output compatibility change in Unreleased. These corrections preserve the approved outcome and Phase 2 boundary.
- 2026-08-10 Phase 1 renewed-review settlement: completed the named proof by comparing the full published marker manifest entry with the planned rendered entry and retaining adversarial archive bytes across correct, missing, and stale marker renders. The lock-pruning proof now also creates and preserves the archive descendant whose stale manifest entry is removed.
- 2026-08-10 Phase 2 review settlement: made parent-sync availability explicit so Windows reports its namespace and write-through limits without claiming POSIX directory sync, and added Windows compile and behavior proofs. Active and reserved collision or cross-filesystem outcomes now retain typed resident state and exact source/destination inspection plus manual-resolution actions. Failed-creation rollback now checks removal before reservation, reports both identity-bound paths absent with durability uncertainty after a post-delete parent-sync failure, and has an orchestration proof for that boundary. Added `internal/effort/archive_test.go`, `internal/effort/platform_windows_test.go`, `internal/effort/service_test.go`, `internal/effort/types_test.go`, and `internal/worktree/manager_test.go` to the focused settlement paths required by these review findings.
- 2026-08-10 Phase 2 renewed-review settlement: preserved the proven source-parent sync fact while the resident remains reserved, then clears it when a successful archive move dirties that parent. Collision proofs now compare the returned and error-carried typed results and every exact source, destination, and manual-resolution action for active and reserved publication paths.
- 2026-08-10 Phase 2 verify-pass settlement: completed the restarted-reservation collision proof with every resident, archive, sync-availability, sync-completion, path, error-carried result, and exact ordered recovery-action fact. This mechanical residual closes the renewed review without another review dispatch.
