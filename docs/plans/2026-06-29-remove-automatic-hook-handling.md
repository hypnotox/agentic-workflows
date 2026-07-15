# Plan: Remove automatic git-hook handling

Links: [ADR-0031](../decisions/0031-invariant-retirement-via-successor-adr.md) (invariant-retirement mechanism), [ADR-0032](../decisions/0032-remove-automatic-hook-handling.md) (hook removal). The design lives in those ADRs; this plan is the execution record only.

## Goal

Remove awf's automatic git-hook handling end to end: the `hook` kind (rendering `.githooks/*`) and hook activation (`awf setup`, `--force-hooks`, the `core.hooksPath` unset in `uninstall`). Add the reusable invariant-retirement mechanism (ADR-0031) first, use it to retire `setup-guards-hookspath`, migrate the config schema (3→4, stripping `hooks:`), and dogfood by hand-maintaining awf's own `.githooks/`. Add an opt-in `local-hooks` workflow-doc section as the adopter-facing replacement.

## Architecture summary

- **Invariant retirement** (ADR-0031): `internal/adr` parses a new `retires_invariants:` frontmatter list; `internal/invariants.Check` collects retirements from Implemented ADRs, errors on a dangling retirement, and subtracts the rest from the required set before checking backing.
- **Hook removal** (ADR-0032): drop the `hook` entry from `internal/project`'s kind-descriptor table and render loop, the `Hooks` field from `internal/config` (`Config` + `Skeleton`) and `internal/catalog`, the `templates/hooks/` tree + catalog block + `embed.go` entry, the `.githooks` mode special-case in `Sync`, and the `if kind != "hook"` guard in `cmd/awf/list_add.go`. A schema migration (`internal/migrate`, To:4) strips `hooks:` via a new `config.RemoveKey` round-trip. The whole `cmd/awf/setup.go` and `cmd/awf/git.go` go away, along with `--force-hooks`, the `setup` subcommand, and `unsetAwfHooks`.

## Tech stack

- Go 1.26. Packages touched: `internal/adr`, `internal/invariants`, `internal/config`, `internal/catalog`, `internal/migrate`, `internal/project`, `cmd/awf`, `templates/`.
- Gate: `./x gate` (~15s, fast tier; `./x gate full` pre-push). 100% statement coverage is enforced (ADR-0012). `./x check` enforces render-drift and the schema-version gate.

## File structure

**Created**
- `internal/migrate/drophooks.go`: the To:4 `drop-hooks` migration.
- `.awf/docs/parts/workflow/local-hooks.md`: awf's override for the new doc section.
- `.awf/parts/adr-template/frontmatter.md`: ADR-template frontmatter override adding `retires_invariants: []`.

**Modified**
- `internal/adr/adr.go`, `internal/invariants/invariants.go` (+ tests): retirement mechanism.
- `internal/config/config.go`, `internal/config/edit.go` (+ `edit_test.go`): drop `Hooks`, add `RemoveKey`.
- `internal/catalog/catalog.go` (+ `catalog_test.go`): drop `Hooks`.
- `internal/migrate/migrate.go`, `internal/migrate/migrate_test.go`: register To:4; update applied-list + `Current()` assertions.
- `internal/project/kind.go`, `render.go`, `scaffold.go`, `project.go`, `check.go`: drop hook kind/loop/special-case + dead `hooks/` dead-ref guard.
- `cmd/awf/main.go`, `uninstall.go`, `list_add.go`: drop setup/`--force-hooks`/kind guard.
- `templates/catalog.yaml`, `templates/docs/workflow.md.tmpl`, `templates/agents-doc/AGENTS.md.tmpl`, `templates/embed.go`.
- `.awf/agents-doc.yaml`, `.awf/parts/agents-doc/identity.md`, `.awf/domains/parts/{tooling,config}/current-state.md`, `.awf/docs/parts/architecture/{overview,components}.md`, `.awf/domains/parts/invariants/current-state.md`.
- `README.md`.
- `.githooks/pre-commit`, `.githooks/pre-push`: become hand-maintained (banner stripped).
- ~17 `*_test.go` files carrying `hooks:` sample YAML (see Phase 3).
- ADR-0031, ADR-0032 frontmatter status flips; `docs/decisions/ACTIVE.md` + `docs/domains/*.md` regenerate.

**Deleted**
- `templates/hooks/` (`pre-commit.tmpl`, `pre-push.tmpl`).
- `cmd/awf/setup.go`, `cmd/awf/git.go`, `cmd/awf/setup_test.go`.

---

## Phase 1: ADR-0031 invariant-retirement mechanism

Ordering: this phase must land (and flip ADR-0031 to Implemented) before Phase 4, where ADR-0032's `retires_invariants` retirement of `setup-guards-hookspath` takes effect.

- [ ] **Task 1.1: Parse `retires_invariants` in `internal/adr`.** In `internal/adr/adr.go`:
  - Add to the `ADR` struct (after `SupersededBy`): `RetiresInvariants []string // retires_invariants: frontmatter (ADR-0031)`.
  - Add to `adrFrontmatter`: `RetiresInvariants []string `yaml:"retires_invariants"``.
  - In `parse`, extend the `ADR{...}` literal to set `RetiresInvariants: fm.RetiresInvariants`.

- [ ] **Task 1.2: Subtract retirements in `internal/invariants.Check`.** In `internal/invariants/invariants.go`, after the loop that builds `required` (and before the `if len(required) == 0` check), insert retirement handling:
  ```go
  	// Retirements: an Implemented ADR may retire an inv slug it removed the
  	// backing for (ADR-0031). A retired slug is dropped from required; a slug
  	// retired but declared by no Implemented ADR is a dangling retirement.
  	for _, a := range adrs {
  		if a.Status != "Implemented" {
  			continue
  		}
  		for _, slug := range a.RetiresInvariants {
  			if _, ok := required[slug]; !ok {
  				return nil, fmt.Errorf("dangling retirement: ADR %s retires %q, which no Implemented ADR declares as inv:", a.Filename, slug)
  			}
  			delete(required, slug)
  		}
  	}
  ```
  Note the existing `required` map is `slug -> declaring ADR filename`; `delete` is safe to call while the slug may also be retired by another ADR; a second retirement of the same slug then hits the dangling branch (which is correct: after the first delete it is no longer required). If two ADRs must retire the same slug, that is a genuine authoring error worth surfacing.

- [ ] **Task 1.3: Back the three ADR-0031 invariant slugs with tests.** In `internal/invariants/invariants_test.go`, add table/cases (use the existing test helpers for writing ADR files + a tagged source file) that assert:
  - `// invariant: inv-retirement-drops-slug`: an Implemented ADR `A` declares `inv: x`, no source backs `x`, and an Implemented ADR `B` has `retires_invariants: [x]` → `Check` returns no finding for `x`.
  - `// invariant: inv-retirement-implemented-only`: same as above but `B` is `Proposed` → `x` is still reported (Unbacked, with sources configured / Unchecked otherwise).
  - `// invariant: inv-retirement-dangling-errors`: an Implemented ADR has `retires_invariants: [ghost]` that no Implemented ADR declares `inv:` → `Check` returns an error mentioning `ghost`.

  Place each `// invariant: <slug>` marker comment on the test that exercises it (markers in `*_test.go` are honoured: `internal/project/drift_test.go:114` is precedent). Verify all three slugs are spelled exactly as in ADR-0031.

- [ ] **Task 1.4: Add `retires_invariants: []` to the ADR template frontmatter.** Create `.awf/parts/adr-template/frontmatter.md` containing the current template frontmatter block plus the new field (matches `docs/decisions/template.md` lines 3-11, with `retires_invariants: []` added after `supersedes`):
  ```
  ---
  status: Proposed
  date: YYYY-MM-DD
  supersedes: []
  retires_invariants: []
  superseded_by: ""
  tags: []
  related: []
  domains: []
  ---
  ```

- [ ] **Task 1.5: Verify and commit (ADR-0031 still Proposed).**
  - Run `./x sync` (regenerates `docs/decisions/template.md` from the new part). Expect `awf sync: done`.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.` (ADR-0031 is still Proposed, so its `inv:` slugs are not yet enforced; the markers are present but inert.)
  - Run `./x check`. Expect `awf check: clean`.
  - Stage: `internal/adr/adr.go internal/invariants/invariants.go internal/invariants/invariants_test.go .awf/parts/adr-template/frontmatter.md docs/decisions/template.md` and any regenerated lock/index. Commit: `feat(awf): add invariant-retirement mechanism (ADR-0031)`.

- [ ] **Task 1.6: Update invariants-domain narrative and the agent-guide "Backed invariants" rule.**
  - In `.awf/domains/parts/invariants/current-state.md`, append a sentence to the narrative: enforcement now has a retirement escape; a successor Implemented ADR's `retires_invariants:` drops a slug from enforcement (ADR-0031), with a dangling-retirement guard.
  - In `.awf/agents-doc.yaml`, edit the ADR-0008 invariant `text` to: `'**Backed invariants.** Every `inv: <slug>` tag in an Implemented ADR is backed by a matching `<marker> invariant: <slug>` comment in source, unless retired by an Implemented successor ADR (ADR-0031).'`

- [ ] **Task 1.7: Flip ADR-0031 to Implemented; verify; commit.**
  - In `docs/decisions/0031-invariant-retirement-via-successor-adr.md` frontmatter, set `status: Implemented`.
  - Run `./x sync` (regenerates `ACTIVE.md`, `docs/domains/invariants.md`, `docs/domains/adr-system.md`, and the AGENTS.md "Backed invariants" line).
  - Run `./x gate` then `./x check`. Expect 100% coverage, `0 issues.`, `awf check: clean`. (ADR-0031 is now Implemented → its three slugs are enforced and backed by Task 1.3.)
  - Stage the ADR, the edited parts/sidecar, `AGENTS.md`, and all regenerated `docs/` + lock. Commit: `docs(adr): implement 0031 invariant-retirement mechanism`.

## Phase 2: Opt-in `local-hooks` workflow-doc section

Additive and independent; safe while ADR-0032 is Proposed.

- [ ] **Task 2.1: Declare the section in the catalog.** In `templates/catalog.yaml`, change the `workflow` doc `sections:` (line 217) to add `local-hooks`:
  ```yaml
      sections: [principles, chain, commit-discipline, doc-currency, local-hooks]
  ```

- [ ] **Task 2.2: Add the section to the workflow template with a publication-safe default body.** In `templates/docs/workflow.md.tmpl`, after the `doc-currency` section's `<!-- awf:end -->` (line 33), append:
  ```
  
  <!-- awf:section local-hooks -->
  ## Local git hooks

  awf does not install or manage git hooks. To run the gate automatically before a commit or push, install a hook yourself: point `core.hooksPath` at a tracked hook directory (`git config core.hooksPath .githooks`) or drop a script under `.git/hooks/`. A pre-commit hook that runs your check and gate commands keeps the green-gate rule enforced locally; `core.hooksPath` is local, uncommitted config, so each clone wires it once.
  <!-- awf:end -->
  ```
  (No `{{ }}` interpolation, so it is publication-safe with no conditional. ADR-0001.)

- [ ] **Task 2.3: Add awf's override part describing its hand-maintained hooks.** Create `.awf/docs/parts/workflow/local-hooks.md`:
  ```
  ## Local git hooks

  This repository keeps hand-maintained hooks under `.githooks/` (not awf-rendered): `pre-commit` runs `./x check` then `./x gate`, and `pre-push` runs `./x gate full`. Wire them once per clone with `git config core.hooksPath .githooks`. They are plain checked-in scripts; edit them directly.
  ```

- [ ] **Task 2.4: Sync, verify, commit.**
  - `./x sync`; expect the section to render into `docs/workflow.md` (awf's override body) with no drift.
  - `./x gate` then `./x check`. Expect 100% coverage, `awf check: clean`.
  - Stage `templates/catalog.yaml templates/docs/workflow.md.tmpl .awf/docs/parts/workflow/local-hooks.md docs/workflow.md` + lock. Commit: `feat(awf): add opt-in local-hooks workflow doc section (ADR-0032)`.

## Phase 3: Remove the hook kind + schema migration + dogfood (ADR-0032 stays Proposed)

All of this lands in one commit: removing the `Hooks` config field is strict-decoder-breaking, so the field removal, every `hooks:` test-YAML cleanup, the migration, and awf's own config migration must be atomic to keep `go test`/gate green. ADR-0032 stays Proposed (its `inv: hooks-config-dropped` is not yet enforced; `setup-guards-hookspath` stays backed because `setup.go` still exists in this phase).

- [ ] **Task 3.1: Add `config.RemoveKey`.** In `internal/config/edit.go`, add after `SetArrayMember`:
  ```go
  // RemoveKey deletes the top-level mapping entry under key from a config.yaml
  // source via a yaml.Node round-trip that preserves comments and every untouched
  // key (ADR-0026). Removing an absent key is a no-op (returns src unchanged), so a
  // schema migration can re-run safely.
  func RemoveKey(src []byte, key string) ([]byte, error) {
  	var doc yaml.Node
  	if err := yaml.Unmarshal(src, &doc); err != nil {
  		return nil, fmt.Errorf("config: parse: %w", err)
  	}
  	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
  		return nil, errors.New("config: not a YAML mapping")
  	}
  	root := doc.Content[0]
  	for i := 0; i+1 < len(root.Content); i += 2 {
  		if root.Content[i].Value == key {
  			root.Content = append(root.Content[:i], root.Content[i+2:]...)
  			return encode(&doc)
  		}
  	}
  	return src, nil
  }
  ```
  In `internal/config/edit_test.go`, add cases covering all four branches: key present (removed, other keys + comments preserved), key absent (src returned verbatim), non-mapping input (error), unparseable YAML (error).

- [ ] **Task 3.2: Write the To:4 migration.** Create `internal/migrate/drophooks.go`:
  ```go
  package migrate

  import (
  	"os"
  	"path/filepath"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  )

  // applyDropHooks ports schema 3 → 4: the hook kind is removed (ADR-0032), so the
  // `hooks:` enable array is stripped from .awf/config.yaml. A config with no
  // `hooks:` key is left unchanged (idempotent). The edit routes through
  // config.RemoveKey so config.yaml serialization stays owned by internal/config
  // (ADR-0026).
  func applyDropHooks(root string) error {
  	cfgPath := filepath.Join(root, ".awf", "config.yaml")
  	src, err := os.ReadFile(cfgPath)
  	if os.IsNotExist(err) {
  		return nil
  	}
  	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
  		return err
  	}
  	out, err := config.RemoveKey(src, "hooks")
  	if err != nil {
  		return err
  	}
  	return os.WriteFile(cfgPath, out, 0o644)
  }
  ```
  In `internal/migrate/migrate.go`, append to `registry`: `{To: 4, Name: "drop-hooks", Apply: applyDropHooks},`.

- [ ] **Task 3.3: Test the migration and the schema bump.** In `internal/migrate/migrate_test.go`:
  - Add a direct `applyDropHooks` success case (mirroring the `TestDropReplaceWith*` precedents): a `.awf/config.yaml` containing `hooks: [pre-commit, pre-push]` plus other keys → after `applyDropHooks`, the file has no `hooks:` key and the other keys/comments are preserved.
  - Add an idempotent case: `applyDropHooks` on a config with no `hooks:` key leaves it byte-identical (no-op).
  - Add the **absent-config** case: `applyDropHooks(t.TempDir())` (no `.awf/config.yaml`) returns nil; this covers the `os.IsNotExist` arm, without which the 100% gate strands that branch. (The non-NotExist `if err != nil` arm stays `// coverage-ignore`: a permission fault a writable test root cannot reach, matching the precedent migrations.)
  - Add a malformed-YAML case: a `.awf/config.yaml` whose contents do not parse surfaces the `RemoveKey` error through `applyDropHooks`.
  - Place the `// invariant: hooks-config-dropped` marker comment (ADR-0032's slug, spelled exactly) on the success-case test, so the slug is backed when ADR-0032 flips to Implemented in Phase 4. (ADR-0032's second Invariants bullet carries no `inv:` slug, so `hooks-config-dropped` is the only marker this change needs.)
  - Bump `Current()` to 4: update `TestCurrentIsThree` (line 587) to assert `Current() == 4` (rename it `TestCurrentIsFour`).
  - **Update the two applied-list assertions the new To:4 lengthens** (a `hooks:` grep does not catch these): `TestUpgradeAppliesInOrderIdempotent` (line 146) → `tree-layout,drop-replacewith,awf-dir-relocation,drop-hooks`; `TestUpgradeStampsTreeLock` (line 753) → `drop-replacewith,awf-dir-relocation,drop-hooks`.
  - **Leave** the `fixtureMonolith` `hooks:` block (lines 47-49) and `legacy.go`'s `Hooks` field in place: the legacy single-file format and the To:1 `applyTreeLayout` port are historical and faithful (it uses the legacy struct, not `config.Config`, so it still compiles), the downstream To:4 strips the carried-through `hooks:`, and the fixture's non-empty hooks keep `applyTreeLayout`'s `if len(lc.Hooks) > 0` branch covered. `TestTreeLayoutPortsMonolith` declares but does not assert `skel.Hooks`, so it needs no change.

- [ ] **Task 3.4: Drop the `Hooks` config + catalog fields.**
  - `internal/config/config.go`: delete `Hooks []string `yaml:"hooks"`` (line 42). Update the `Config` doc comment "Skills/Agents/Docs/Hooks" → "Skills/Agents/Docs".
  - `internal/config/edit.go`: delete `Hooks []string `yaml:"hooks"`` from `Skeleton` (line 22).
  - `internal/catalog/catalog.go`: delete `Hooks []string `yaml:"hooks"`` (line 53); update the package doc comment (line 1) "skills, agents, and hooks" → "skills, agents, and docs". (The `catalog_test.go` `cat.Hooks` assertion is removed in Task 3.8.)

- [ ] **Task 3.5: Drop the hook kind, render loop, scaffold, and Sync special-case.**
  - `internal/project/kind.go`: delete the entire `hooks`/`hook` descriptor block (lines 50-56).
  - `internal/project/render.go`: delete the `// Hooks.` loop (lines 148-156).
  - `internal/project/scaffold.go`: delete the `for _, hook := range cat.Hooks { ... }` loop (lines 44-49); delete the `hookList` block (lines 85-87); remove `Hooks: hookList,` from the `Skeleton` literal (line 98); change the comment "agents and hooks are all enabled (every one is workflow-essential)" → "agents are all enabled (every one is workflow-essential)"; update the package doc comment (lines 16-17) "every agent, and every hook in the embedded catalog" → "and every agent in the embedded catalog".
  - `internal/project/check.go`: simplify `isManagedMarkdown` (line 144) to `return tid != "claude/CLAUDE.md.tmpl"`; with no `hooks/` template id ever produced, the `&& !strings.HasPrefix(tid, "hooks/")` clause is dead. Update its doc comment (line 142, "and the .githooks scripts") and the `checkDeadRefs` comment (line 276, "and hooks are not.") to drop the hook reference. (`strings` stays imported; it is used elsewhere in the file.)
  - `internal/project/project.go` `Sync`: delete the `.githooks` mode special-case:
    ```go
  		mode := os.FileMode(0o644)
  		if filepath.Dir(f.Path) == ".githooks" {
  			mode = 0o755
  		}
    ```
    becomes `mode := os.FileMode(0o644)`.

- [ ] **Task 3.6: Drop the templates and embed entry.**
  - Delete `templates/hooks/pre-commit.tmpl` and `templates/hooks/pre-push.tmpl` (and the now-empty `templates/hooks/` dir).
  - `templates/catalog.yaml`: delete the `hooks:` block (lines 205-207).
  - `templates/embed.go`: remove `hooks` from the `//go:embed` directive and from the package doc comment ("skills, agents, hooks, docs" → "skills, agents, docs").

- [ ] **Task 3.7: Simplify the `list_add.go` kind guard and message.**
  - `cmd/awf/list_add.go` line 16: change the unknown-kind message to `unknown kind %q (want: skill, agent, doc, domain)`.
  - In `targetState` (around lines 148-154), remove the now-always-true `if kind != "hook"` wrapper: dedent its body so the sidecar-state logic always runs, and update the comment that references hooks carrying no sidecar.

- [ ] **Task 3.8: Sweep `hooks:` from test YAML and hook-specific tests.** The strict decoder rejects an unknown `hooks:` key, so remove every `hooks:` line/segment from embedded YAML and inline cfg strings across the test suite. Files (from `grep -rn "hooks:" --include=*_test.go`): `internal/project/coverage_test.go` (28), `internal/project/project_test.go` (19), `internal/project/render_tree_test.go` (4), `internal/project/{target,domains,drift}_test.go`, `internal/project/{docs_sections,install}_test.go`, `internal/project/kind_test.go`, `cmd/awf/{run,invariants,check,list_add,audit}_test.go`, `internal/config/{edit,config}_test.go`, `internal/migrate/migrate_test.go` (handled in 3.3). Also:
  - `internal/project/golden_test.go`: delete the `.githooks/pre-commit`/`.githooks/pre-push` assertions and the `TestPreCommitCheckCmdOverride`-style test that renders the hook with a `checkCmd` override (golden_test.go lines ~36-58, ~69-86).
  - `internal/project/project_test.go`: drop `.githooks/pre-commit` and `.githooks/pre-push` from the lock-tracked render assertion (line 85).
  - `internal/project/kind_test.go`: drop any assertion enumerating `hook`/`hooks` among the kinds.
  - `internal/catalog/catalog_test.go`: delete the `if len(cat.Hooks) != 2 { ... }` assertion (lines ~25-26). It references the removed `Catalog.Hooks` field and would otherwise fail to compile; a bare `hooks:` grep misses this `cat.Hooks` reference.
  - `cmd/awf/list_add_test.go`: drop the `add/remove/list hook` dispatch case, **and** drop `"hooks:"` from the `runList`-output `want` list (line 172); with the kind gone, `awf list` no longer prints a `hooks:` header.
  - Verify with `grep -rn "hooks:\|\.githooks\|pre-commit\|pre-push\|\.Hooks" --include=*_test.go .` returning nothing hook-related **except** the intentionally-retained legacy single-file fixtures in `internal/migrate/migrate_test.go` (the `fixtureMonolith` `hooks:` block, kept per Task 3.3) and any legacy-format `hooks:` in `cmd/awf/run_test.go`'s monolith string. Phase 4 removes the remaining `setup`/`force-hooks` test refs.

- [ ] **Task 3.9: Kind-related doc edits.**
  - `.awf/docs/parts/architecture/overview.md`: line 4 "skills, agents, hooks, docs, and agent guide" → "skills, agents, docs, and agent guide"; line 13 "(`skills`, `agents`, `docs`, `hooks`: ...)" → "(`skills`, `agents`, `docs`: ...)"; line 24 "the ADR index, and git hooks under `.githooks/`." → "and the ADR index." (drop the hooks clause).
  - `.awf/docs/parts/architecture/components.md`: line 9 "hooks, docs, and their sections." → "docs, and their sections."; line 24 "embedded skill, agent, hook, doc, and agent-guide templates" → "embedded skill, agent, doc, and agent-guide templates". (Line 3's CLI list keeps `setup` until Phase 4.)
  - `.awf/domains/parts/config/current-state.md`: "flat enable arrays for skills/agents/docs/domains/hooks" → "flat enable arrays for skills/agents/docs/domains"; "plus all agents and hooks" → "plus all agents"; "The five enable arrays" → "The four enable arrays"; add a sentence that schema migration `{To:4}` dropped the `hooks:` array with the hook kind (ADR-0032).
  - `templates/agents-doc/AGENTS.md.tmpl` `awf-setup` section: "skills, agents, and git hooks (and this guide) are rendered by awf" → "skills, agents, and docs (and this guide) are rendered by awf"; the kind list "(kinds: `skill`, `agent`, `doc`, `hook`, `domain`)" → "(kinds: `skill`, `agent`, `doc`, `domain`)".
  - `.awf/parts/agents-doc/identity.md` line 3 (renders into AGENTS.md's Identity section: edit the part, never the rendered `AGENTS.md`): drop `git hooks` from the rendered-suite list ("skills, review agents, git hooks, docs, and this agent guide" → "skills, review agents, docs, and this agent guide"); reword the trailing "hooks under `.githooks/` enforce the gate" clause: awf no longer renders hooks, so phrase it as a hand-maintained/local hook (e.g. "a local git hook enforces the gate"), consistent with the new `local-hooks` story.
  - `cmd/awf/main.go` line 1 package doc comment: "renders standardised .claude skills, review agents, and git hooks into a project" → drop "and git hooks". Also drop `hook` from the `helpText` kind enumeration on the `add` line (line 48: `kind ∈ {skill, agent, doc, hook, domain}` → `kind ∈ {skill, agent, doc, domain}`); the user-facing `--help` kind list must match the kind removal landing in this phase (ADR-0032). (The `setup`/`--force-hooks` `helpText` lines are Phase 4.)
  - `README.md` line 94: `<kind>` ∈ `skill`, `agent`, `doc`, `hook`, `domain` → drop `hook`. (Lines 49/76/90/96/100/111-114 (setup/uninstall/init-activation) and line 17's rendered-output list are Phase 4.)

- [ ] **Task 3.10: Migrate awf's own tree and hand-maintain `.githooks/`.**
  - Run `go run ./cmd/awf upgrade`. Expect `awf upgrade: applied drop-hooks` then `awf sync: done`. This strips `hooks:` from `.awf/config.yaml`, stamps `.awf/awf.lock` to schema 4, and re-renders; the sync prunes the no-longer-rendered `.githooks/pre-commit` and `.githooks/pre-push` from disk.
  - Recreate `.githooks/pre-commit` (hand-maintained, banner removed):
    ```bash
    #!/usr/bin/env bash
    set -euo pipefail
    ./x check
    ./x gate
    ```
  - Recreate `.githooks/pre-push` (hand-maintained, banner removed):
    ```bash
    #!/usr/bin/env bash
    set -euo pipefail
    ./x gate full
    ```
  - `chmod +x .githooks/pre-commit .githooks/pre-push` (awf no longer sets their mode; git hooks must be executable).

- [ ] **Task 3.11: Verify and commit.**
  - `go test ./...`: expect all green (confirms no stray `hooks:` YAML and no dead code).
  - `./x gate`: expect `coverage: 100.0%` and `0 issues.`
  - `./x check`: expect `awf check: clean` (the hand-maintained `.githooks/*` are untracked by the lock, so check ignores them).
  - Confirm `.githooks/pre-commit` and `.githooks/pre-push` exist, are executable, and carry no `GENERATED by awf` banner; confirm `.awf/config.yaml` has no `hooks:` key and `.awf/awf.lock` `schemaVersion` is 4 with no `.githooks/*` entries.
  - Stage all changed source, templates, parts, tests, the migrated `.awf/config.yaml` + lock, regenerated `docs/`, and the two `.githooks/*` files. Commit: `feat(awf)!: remove the hook kind and migrate schema to 4 (ADR-0032)`.

## Phase 4: Remove hook activation + flip ADR-0032 to Implemented

The `setup.go` deletion unbacks `setup-guards-hookspath`; flipping ADR-0032 to Implemented in this same commit activates its `retires_invariants` so the slug is retired (mechanism from Phase 1), keeping the gate green.

- [ ] **Task 4.1: Delete the activation code.**
  - Delete `cmd/awf/setup.go` and `cmd/awf/git.go` (after removing `unsetAwfHooks` in Task 4.2, every function in `git.go` (`openWorktree`, `localHooksPath`, `writeLocalHooksPath`) is unreferenced; `setup.go` also carries `awfHooksRel`, whose only callers were `runSetup` and `unsetAwfHooks`). `git.go` is the only `cmd/awf` *non-test* user of the `go-git` import; `cmd/awf/audit_test.go` also imports it, but a test does not keep the binary's dependency; `go-git` stays in `go.mod` via `internal/audit` regardless.
  - Delete `cmd/awf/setup_test.go`.

- [ ] **Task 4.2: Strip activation from `uninstall.go`.** In `cmd/awf/uninstall.go`: delete the `unsetAwfHooks(root, stdout)` call (line 21) and the entire `unsetAwfHooks` function (lines 26-41). Update `runUninstall`'s doc comment to drop the `core.hooksPath` clause. The command now only removes lock-tracked files + the lock and reports leaving `.awf/` in place. In `cmd/awf/uninstall_test.go`, remove the hook-unset assertions/cases (keep the lock-tracked-removal coverage that backs `inv: uninstall-removes-lock-tracked`).

- [ ] **Task 4.3: Strip `setup`/`--force-hooks` from the dispatcher.** In `cmd/awf/main.go`:
  - `helpText`: from `init`, drop the trailing "and activate git hooks" and the `--force-hooks` line (lines 39, 41); delete the two `setup` lines (50-51); change the `uninstall` line (56) to "Remove awf's generated files (keeps `.awf/`)".
  - Usage string (line 62): remove `setup` from the `<init|sync|...>` list.
  - `runInit`: drop the `forceHooks` parameter, the `--force-hooks` arg at the call site (lines 84-86), and the trailing `runSetup(...)` block (lines 311-313). `stderr` is referenced **only** by that setup-skip warning, so it becomes unused; remove it too: the signature becomes `runInit(root string, force, describe bool, sets []string, answersFile string, stdout io.Writer)` and the call site (lines 84-86) passes only `stdout`. (Leaving an unused `stderr` parameter risks an `unparam` lint flag under `./x gate`.)
  - Delete the `case "setup":` dispatch (lines 117-118).
  - `argSpecs`: remove `--force-hooks` from the `init` entry (line 154) and delete the `"setup"` entry (line 162).
  - In `cmd/awf/run_test.go`, remove the two `setup`/`--force-hooks` references the grep found (drop the `setup` dispatch test and any `--force-hooks` flag assertion).

- [ ] **Task 4.4: Activation-related doc edits.**
  - `README.md`: line 17 drop `.githooks/` from the rendered-output list ("(`.claude/`, `AGENTS.md`, `docs/`, `.githooks/`)" → "(`.claude/`, `AGENTS.md`, `docs/`)"): awf no longer renders `.githooks/`; line 49 drop the `.githooks/... gate hooks` tree line; line 76 "scaffold .awf/, render the workflow-core set, activate git hooks" → "scaffold .awf/, render the workflow-core set"; line 90 (`awf init` row) drop "and activate git hooks" + the `--force-hooks` sentence; delete the `awf setup` row (96); line 100 (`awf uninstall` row) → "Remove awf's generated files (keeps your `.awf/` config)."; delete the `awf setup` safe-adoption bullet (111-112) and drop "and unsets its hook path" from the back-out bullet (114). Add a short "Local hooks" note pointing readers at installing their own hook (mirroring the new workflow-doc section / `.githooks/` example), and (per ADR-0032's prior-adopter consequence and its README doc-currency obligation) document the migration step for adopters who previously ran `awf setup`: their `core.hooksPath` still points at the no-longer-rendered `.githooks/`, so they should run `git config --unset core.hooksPath` (or keep the now hand-owned hook files).
  - `.awf/docs/parts/architecture/components.md` line 3: drop `setup` from the CLI subcommand list.
  - `.awf/domains/parts/tooling/current-state.md`: from the `./x` runner list drop `setup`; rewrite the ADR-0023 sentence to drop the `awf setup`/`core.hooksPath` clauses (keep the `--force`/`.awf-bak` backup and the `awf uninstall` lock-tracked removal, dropping its `core.hooksPath` clause); reword the "all five config enable arrays ... `skill`/`agent`/`doc`/`hook`" sentence to four arrays and drop `hook`; note hook handling was removed (ADR-0032). Drop "and hooks" from the curated-core sentence ("alongside all agents and hooks" → "alongside all agents"). Rewrite the go-git sentence ("All git interaction (toplevel detection, the repo-local `core.hooksPath` read/write, and the audit's history and working-tree reads) goes through go-git") to drop the now-removed toplevel-detection and `core.hooksPath` read/write (only the audit's history/working-tree reads use go-git after `git.go` is deleted). Also reword the line-3 "git hooks under `.githooks/` run the gate on commit/push" clause: the `.githooks/` here are now hand-maintained, not awf-rendered.

- [ ] **Task 4.5: Flip ADR-0032 to Implemented; verify; commit.**
  - In `docs/decisions/0032-remove-automatic-hook-handling.md`, set `status: Implemented`.
  - `./x sync` (regenerates `ACTIVE.md`, `docs/domains/{tooling,rendering}.md`, and the doc edits above).
  - `./x gate`: expect 100% coverage, `0 issues.` (`setup-guards-hookspath` is now retired via ADR-0032's `retires_invariants`; `hooks-config-dropped` is backed by the Phase 3 migration test; the dangling-retirement guard passes because ADR-0023 still declares the slug).
  - `./x check`: expect `awf check: clean`.
  - `./x gate full`: run the pre-push tier once to confirm nothing in a slower surface regressed.
  - Stage all changed source, tests, README, parts/sidecar, the ADR, and regenerated `docs/` + lock. Commit: `feat(awf)!: remove hook activation and implement ADR-0032`.

## Verification (whole change)

- `git grep -nE "githooks|hooksPath|force-hooks|runSetup|awf setup|cat\.Hooks|c\.Hooks" -- ':!docs/decisions' ':!docs/plans' ':!.githooks'` returns nothing (all references gone except the historical ADRs/plan and the hand-maintained hook files).
- `./x gate full` green; `./x check` clean; `awf list` shows no `hook` kind; `awf` help shows no `setup`.
- ADR-0031 and ADR-0032 are Implemented; ADR-0003 is Superseded by ADR-0032; ADR-0023 stays Implemented.

## Execution

Tasks are tightly coupled and ordered (Phase 1 before Phase 4; the strict-decoder and retirement constraints force the atomic Phase 3 / Phase 4 commits). Execute inline with `awf-executing-plans` (one task at a time, `./x gate` per commit), not subagent dispatch.
