# Plan: Tool-Agnostic Target Seam, `.awf/` Relocation, and the Claude Adapter

Implements [ADR-0016](../decisions/0016-tool-agnostic-target-seam-and-awf-relocation.md). Design
rationale lives in the ADR; this file is the execution record. Do not duplicate rationale — link.

## Goal

Make awf a tool-agnostic renderer shipping one adapter (Claude Code): route adapter output paths
through a named `Target`, relocate awf's config tree `.claude/awf/` → `.awf/`, emit a whole-file
root `CLAUDE.md` (`@AGENTS.md` bridge), and guard `awf init` against clobbering existing files
(`--force` to override).

## Architecture summary

- **Target seam** (`internal/project/target.go`): a `Target` value holds the placement rules for
  *adapter* artifacts (skills, agents, the bridge file). `claudeTarget` is the sole built-in.
  `RenderAll`/`localOutPath` ask the target for paths instead of using `.claude/...` literals.
- **Relocation**: every site naming `.claude/awf/` (config load, lock path, provenance banner text,
  `awf:edit` pointer paths, orphan scanner, `awf add`, doc comments) moves to `.awf/`. Adapter
  output (`.claude/skills/`, `.claude/agents/`) and the frozen legacy/intermediate migration paths
  (`migrate.go` legacy, `treelayout.go`, `dropreplacewith.go`, `legacy.go`) deliberately stay.
- **Migration `{To:3}`** (`internal/migrate`): `applyAwfRelocation` `os.Rename`s the finished
  `.claude/awf/` tree to `.awf/`; `Generation` keys on directory presence (new `.awf/` tree → its
  lock; old `.claude/awf/` tree → sub-`Current()` sentinel that gates; legacy → 0); `stampLockSchema`
  targets `.awf/awf.lock`.
- **CLAUDE.md bridge**: new template `templates/claude/CLAUDE.md.tmpl` (`@AGENTS.md`), emitted by the
  Claude adapter alongside `AGENTS.md`, carrying the ADR-0015 banner.
- **init guard**: `Project.PlannedOutputs()` enumerates everything `Sync` writes; `awf init` aborts
  (writing nothing) on any planned path that exists on disk and is not in the prior lock, unless
  `--force`.

## Tech stack

- Go 1.26; `gopkg.in/yaml.v3`; stdlib `os`/`path/filepath`/`embed`.
- Packages touched: `internal/project`, `internal/config`, `internal/migrate`, `internal/manifest`
  (read only), `cmd/awf`, `templates`.
- Gate: `./x gate` (`go test ./... -coverpkg=./...` at 100%, `go vet`, golangci-lint) on every
  code-touching commit; `./x check` (drift + invariants) via the pre-commit hook.

## File structure

Created:
- `internal/project/target.go` — the `Target` type + `claudeTarget`.
- `internal/project/target_test.go` — backs `inv: target-output-paths`.
- `templates/claude/CLAUDE.md.tmpl` — the bridge template (`@AGENTS.md`).
- `internal/migrate/relocation.go` — `applyAwfRelocation`.

Modified (production):
- `internal/project/project.go` — Target field + path routing; CLAUDE.md emit; `PlannedOutputs`;
  banner text; `awf:edit` pointer paths; orphan scanner; `config.Load`/lock paths.
- `internal/config/config.go` — doc comment + `root` field comment (path text only).
- `internal/migrate/migrate.go` — register `{To:3}`; rewrite `Generation`; `stampLockSchema` path.
- `templates/embed.go` — add `claude` to the embed list.
- `cmd/awf/main.go` — `--force` parse; `runInit` collision guard + `.awf` scaffold path; doc comment.
- `cmd/awf/list_add.go` — `awf add` config path.
- `internal/manifest/manifest.go` — doc comment (path text only).
- `internal/project/scaffold.go` — doc comment (path text only).

Modified (tests, Phase 4): `internal/project/{project_test,spine_test,drift_test,coverage_test,docs_sections_test,domains_test,scaffold_test}.go`, `cmd/awf/{run_test,main_test,check_test,invariants_test,list_add_test}.go`, `internal/config/config_test.go`, `internal/migrate/migrate_test.go`, `internal/render/render_test.go` — fixture paths `.claude/awf` → `.awf`; predecessor backing-test updates. This list is indicative; the `grep -rln` sweep in Phase 4 is authoritative — update every `*_test.go` that constructs the config tree.

Modified (prose, Phase 5): `templates/agents-doc/AGENTS.md.tmpl`, `templates/skills/proposing-adr/SKILL.md.tmpl`, `.awf/` convention parts/sidecars carrying `.claude/awf` prose, `docs/decisions/template.md`, `README.md`, and re-synced current-guidance docs.

Modified (status, Phase 6): `docs/decisions/0016-*.md` (Accepted → Implemented), regenerated `ACTIVE.md` + domain indexes.

---

## Phase 1 — Target seam (byte-identical output)

- [ ] **Create `internal/project/target.go`** with exactly:
  ```go
  package project

  import "fmt"

  // Target places adapter (tool-specific) artifacts for one runtime. Neutral
  // artifacts (AGENTS.md, docs, domains, hooks) are not target-scoped (ADR-0016).
  type Target struct {
  	Name       string
  	SkillDir   string // dir holding rendered skills, e.g. ".claude/skills"
  	AgentDir   string // dir holding rendered agents, e.g. ".claude/agents"
  	BridgeFile string // adapter bridge file at repo root, "" if none
  }

  // SkillPath is the output path for a rendered skill under this target.
  func (t Target) SkillPath(prefix, name string) string {
  	return fmt.Sprintf("%s/%s-%s/SKILL.md", t.SkillDir, prefix, name)
  }

  // AgentPath is the output path for a rendered agent under this target.
  func (t Target) AgentPath(name string) string {
  	return fmt.Sprintf("%s/%s.md", t.AgentDir, name)
  }

  // claudeTarget is the sole built-in target. Adding a second runtime is a new
  // Target value plus its placement, not a render-loop change.
  var claudeTarget = Target{
  	Name:       "claude",
  	SkillDir:   ".claude/skills",
  	AgentDir:   ".claude/agents",
  	BridgeFile: "CLAUDE.md",
  }
  ```

- [ ] **Add the `Target` field to `Project`** in `internal/project/project.go`. Change the struct:
  ```go
  type Project struct {
  	Root   string
  	Cfg    *config.Config
  	Cat    *catalog.Catalog
  	Target Target
  }
  ```
  and in `Open`, set it when constructing `p`:
  ```go
  	p := &Project{Root: root, Cfg: cfg, Cat: cat, Target: claudeTarget}
  ```

- [ ] **Route skill/agent paths through the target.** In `RenderAll`, replace the skill output-path
  arg `fmt.Sprintf(".claude/skills/%s-%s/SKILL.md", p.Cfg.Prefix, name)` with
  `p.Target.SkillPath(p.Cfg.Prefix, name)`, and the agent output-path arg
  `fmt.Sprintf(".claude/agents/%s.md", name)` with `p.Target.AgentPath(name)`. In `localOutPath`,
  replace the two `fmt.Sprintf` bodies:
  ```go
  	case "skills":
  		return p.Target.SkillPath(p.Cfg.Prefix, name)
  	case "agents":
  		return p.Target.AgentPath(name)
  ```
  Remove the now-unused `fmt` import only if `go build` reports it unused (it is still used elsewhere
  in `project.go`, so leave the import).

- [ ] **Create `internal/project/target_test.go`** backing `inv: target-output-paths`:
  ```go
  package project

  import "testing"

  // invariant: target-output-paths
  func TestClaudeTargetPaths(t *testing.T) {
  	if got := claudeTarget.SkillPath("awf", "tdd"); got != ".claude/skills/awf-tdd/SKILL.md" {
  		t.Fatalf("SkillPath = %q", got)
  	}
  	if got := claudeTarget.AgentPath("code-reviewer"); got != ".claude/agents/code-reviewer.md" {
  		t.Fatalf("AgentPath = %q", got)
  	}
  	if claudeTarget.BridgeFile != "CLAUDE.md" {
  		t.Fatalf("BridgeFile = %q", claudeTarget.BridgeFile)
  	}
  }
  ```

- [ ] **Verify byte-identical output and gate.** Run `./x sync` — expect `git status --short` to show
  no changes to any rendered file or the lock (paths are unchanged; this is a pure refactor). Run
  `./x gate` — expect `0 issues.` and `coverage: 100.0%`. If coverage drops, the new `Target` methods
  are covered by `TestClaudeTargetPaths` plus the existing render path; add assertions if a method
  shows uncovered.

- [ ] **Commit:** `feat(awf): route adapter output paths through a Target seam (ADR-0016)`.

---

## Phase 2 — CLAUDE.md bridge

- [ ] **Create `templates/claude/CLAUDE.md.tmpl`** with exactly this content (one line + trailing
  newline, no frontmatter, no section markers):
  ```
  @AGENTS.md
  ```

- [ ] **Add `claude` to the embed list** in `templates/embed.go`:
  ```go
  //go:embed catalog.yaml skills hooks agents agents-doc docs domains claude
  var FS embed.FS
  ```

- [ ] **Emit the bridge in `RenderAll`.** In `internal/project/project.go`, inside the agents-doc
  block (`if !ad.Local { ... }`), after `out = append(out, rf)` for the agents-doc render and before
  the block closes, add the bridge emit guarded by the target having a bridge file:
  ```go
  		if p.Target.BridgeFile != "" {
  			brf, err := p.renderTarget("claude", "", "claude/CLAUDE.md.tmpl",
  				nil, config.Sidecar{}, p.data(config.Sidecar{}), p.Target.BridgeFile)
  			if err != nil { // coverage-ignore: the bridge template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
  				return nil, err
  			}
  			out = append(out, brf)
  		}
  ```
  (`renderTarget` with `nil` declared sections yields an empty plan; `injectBanner` adds the HTML
  banner as line 1, leaving `@AGENTS.md` as the body.)

- [ ] **Add a bridge render test** backing `inv: claude-md-bridge`. Append to
  `internal/project/target_test.go`:
  ```go
  // invariant: claude-md-bridge
  func TestClaudeMdBridgeRendered(t *testing.T) {
  	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\nhooks: []\ndocs: []\n")
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	files, err := p.RenderAll()
  	if err != nil {
  		t.Fatal(err)
  	}
  	var got *RenderedFile
  	for i := range files {
  		if files[i].Path == "CLAUDE.md" {
  			got = &files[i]
  		}
  	}
  	if got == nil {
  		t.Fatal("CLAUDE.md not rendered")
  	}
  	if !strings.Contains(got.Content, "@AGENTS.md") {
  		t.Fatalf("CLAUDE.md missing @AGENTS.md import:\n%s", got.Content)
  	}
  	if !strings.HasPrefix(got.Content, "<!-- ") {
  		t.Fatalf("CLAUDE.md missing provenance banner:\n%s", got.Content)
  	}
  }
  ```
  Add `"strings"` to the test file's imports. (Uses the existing `scaffold` helper in
  `project_test.go`.)

- [ ] **Re-sync the dogfood + update golden fixtures.** Run `./x sync`. Expect a NEW file
  `CLAUDE.md` at the repo root containing the banner + `@AGENTS.md`, plus a new lock entry. If
  `spine_test.go` or `docs_sections_test.go` enumerate the full rendered-file set and now miss
  `CLAUDE.md`, update those fixtures to include it. Run `./x gate` — expect `0 issues.`, 100%.

- [ ] **Verify the import is intact.** `cat CLAUDE.md` — expect exactly:
  ```
  <!-- GENERATED by awf — do not edit; change .claude/awf/ and run `awf sync` -->
  @AGENTS.md
  ```
  (Banner still names `.claude/awf/`; relocated in Phase 4.)

- [ ] **Commit:** `feat(awf): add CLAUDE.md @AGENTS.md bridge to Claude adapter (ADR-0016)`.

---

## Phase 3 — `awf init` collision guard + `--force`

- [ ] **Add `PlannedOutputs` to `internal/project/project.go`** (after `RenderAll`):
  ```go
  // PlannedOutputs returns the project-relative paths Sync would write: every
  // RenderAll output plus the generated ACTIVE.md and domain docs. Used by
  // awf init to detect collisions before writing (ADR-0016).
  func (p *Project) PlannedOutputs() ([]string, error) {
  	files, err := p.RenderAll()
  	if err != nil {
  		return nil, err
  	}
  	var paths []string
  	for _, f := range files {
  		paths = append(paths, f.Path)
  	}
  	if amd, ok, err := p.generateActiveMD(); err != nil {
  		return nil, err
  	} else if ok {
  		paths = append(paths, amd.Path)
  	}
  	dds, err := p.generateDomainDocs()
  	if err != nil { // coverage-ignore: unreachable — generateActiveMD above parses the same decisions dir and fails first on a malformed ADR
  		return nil, err
  	}
  	for _, dd := range dds {
  		paths = append(paths, dd.Path)
  	}
  	return paths, nil
  }
  ```

- [ ] **Parse `--force`** in `cmd/awf/main.go`. Change the `init` dispatch:
  ```go
  	case "init":
  		cmdErr = runInit(cwd, hasFlag(args, "--force"), stdout, stderr)
  ```
  and add a helper near the top of the file:
  ```go
  // hasFlag reports whether flag appears anywhere in args[2:].
  func hasFlag(args []string, flag string) bool {
  	for _, a := range args[2:] {
  		if a == flag {
  			return true
  		}
  	}
  	return false
  }
  ```

- [ ] **Wire the guard into `runInit`** in `cmd/awf/main.go`. Replace the whole `runInit` body with:
  ```go
  func runInit(root string, force bool, stdout, stderr io.Writer) error {
  	cfgPath := filepath.Join(root, ".claude", "awf", "config.yaml")
  	scaffolded := false
  	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
  		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
  			return err
  		}
  		scaffold, err := project.ScaffoldConfig(filepath.Base(root))
  		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
  			return err
  		}
  		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
  			return err
  		}
  		scaffolded = true
  		fmt.Fprintf(stdout, "scaffolded %s\n", cfgPath)
  	}
  	if !force {
  		if collisions, err := initCollisions(root); err != nil {
  			return err
  		} else if len(collisions) > 0 {
  			if scaffolded {
  				os.RemoveAll(filepath.Dir(cfgPath)) // writes nothing on abort
  			}
  			return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
  				strings.Join(collisions, "\n  "))
  		}
  	}
  	if err := runSync(root, stdout); err != nil {
  		return err
  	}
  	if err := runSetup(root, stdout, stderr); err != nil {
  		fmt.Fprintln(stderr, "awf init: hook setup skipped:", err)
  	}
  	return nil
  }

  // initCollisions returns planned output paths that already exist on disk and are
  // not recorded in the prior lock (i.e. not awf-managed). An awf-managed path that
  // already exists is not a collision — re-init is idempotent.
  func initCollisions(root string) ([]string, error) {
  	p, err := project.Open(root)
  	if err != nil {
  		return nil, err
  	}
  	planned, err := p.PlannedOutputs()
  	if err != nil {
  		return nil, err
  	}
  	managed := map[string]bool{}
  	if lock, err := manifest.Load(filepath.Join(root, ".claude", "awf", "awf.lock")); err == nil {
  		for path := range lock.Files {
  			managed[path] = true
  		}
  	}
  	var collisions []string
  	for _, rel := range planned {
  		if managed[rel] {
  			continue
  		}
  		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
  			collisions = append(collisions, rel)
  		}
  	}
  	sort.Strings(collisions)
  	return collisions, nil
  }
  ```
  Add `"sort"`, `"strings"`, and `"github.com/hypnotox/agentic-workflows/internal/manifest"` to the
  `cmd/awf/main.go` imports.

- [ ] **Add tests** in `cmd/awf/run_test.go` backing `inv: init-collision-guard` (mirror the
  existing `scaffoldProject`/`run` helpers in that file; the `run` signature is
  `run(args []string, stdout, stderr io.Writer) int`):
  ```go
  // invariant: init-collision-guard
  func TestInitGuardBlocksAndForceOverrides(t *testing.T) {
  	root := t.TempDir()
  	// A pre-existing, non-awf CLAUDE.md is a collision.
  	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	swapGetwd(t, func() (string, error) { return root, nil })
  	var out, errb bytes.Buffer
  	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
  		t.Fatal("expected init to fail on collision")
  	}
  	if !strings.Contains(errb.String(), "refusing to overwrite") {
  		t.Fatalf("stderr = %q", errb.String())
  	}
  	// Nothing written: the scaffolded config tree was rolled back.
  	if _, err := os.Stat(filepath.Join(root, ".claude", "awf", "config.yaml")); !os.IsNotExist(err) {
  		t.Fatal("expected .claude/awf to be rolled back")
  	}
  	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) != "mine\n" {
  		t.Fatalf("CLAUDE.md clobbered: %q", b)
  	}
  	// --force overwrites and completes.
  	out.Reset()
  	errb.Reset()
  	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
  		t.Fatalf("init --force failed: %s", errb.String())
  	}
  }
  ```
  Ensure `bytes`, `os`, `path/filepath`, `strings` are imported in `run_test.go` (add any missing).

- [ ] **Fix the existing `runInit` callers** broken by the new `force` parameter (compile failure
  otherwise — they pass three args to a now-four-arg function). Add `false` as the second argument
  to each direct call (all three init into fresh or already-managed trees, so no collision is
  expected):
  - `cmd/awf/main_test.go:18` — `runInit(proj, io.Discard, io.Discard)` → `runInit(proj, false, io.Discard, io.Discard)`.
  - `cmd/awf/run_test.go` (`TestRunInitSyncError`, ~line 285) — add `false`.
  - `cmd/awf/run_test.go` (`TestRunInitOnExistingConfigSkipsScaffold`, ~line 391) — add `false`.
  (The `run([]string{"awf","init"}, …)` dispatch tests are unaffected — they exercise the new
  signature through `run`.)

- [ ] **Gate:** `./x gate` — expect `0 issues.`, 100%. The `os.RemoveAll` rollback branch is covered
  by the collision test; the `--force` path by its second half.

- [ ] **Commit:** `feat(awf): add awf init collision guard and --force (ADR-0016)`.

---

## Phase 4 — Relocation: code paths + migration + dogfood port (atomic)

This phase keeps the repo green only as a whole: the code starts reading `.awf/`, the migration moves
the tree, and the dogfood port runs `awf upgrade` — all in one commit.

- [ ] **Add the relocation migration.** Create `internal/migrate/relocation.go`:
  ```go
  package migrate

  import (
  	"errors"
  	"fmt"
  	"os"
  	"path/filepath"
  )

  // applyAwfRelocation moves a finished .claude/awf/ config tree (and its lock) to
  // .awf/ (ADR-0016). Idempotent: a no-op when .claude/awf/ is absent. Fails rather
  // than overwrite if .awf/ already exists.
  func applyAwfRelocation(root string) error {
  	oldDir := filepath.Join(root, ".claude", "awf")
  	newDir := filepath.Join(root, ".awf")
  	if _, err := os.Stat(oldDir); errors.Is(err, os.ErrNotExist) {
  		return nil
  	}
  	if _, err := os.Stat(newDir); err == nil {
  		return fmt.Errorf("cannot relocate: %s already exists", newDir)
  	}
  	return os.Rename(oldDir, newDir)
  }
  ```

- [ ] **Register `{To:3}`** in `internal/migrate/migrate.go`:
  ```go
  var registry = []Migration{
  	{To: 1, Name: "tree-layout", Apply: applyTreeLayout},
  	{To: 2, Name: "drop-replacewith", Apply: applyDropReplaceWith},
  	{To: 3, Name: "awf-dir-relocation", Apply: applyAwfRelocation},
  }
  ```

- [ ] **Rewrite `Generation`** in `internal/migrate/migrate.go` to key on directory presence:
  ```go
  // Generation reports the project's schema generation. Detection is by layout:
  // a .awf/ tree reports its lock's SchemaVersion (or Current() when no lock yet —
  // fresh init / just-upgraded); a pre-relocation .claude/awf/ tree reports its
  // lock's schema, or Current()-1 when no lock, so the To:3 relocation gates; the
  // legacy single file reports 0; nothing present reports Current().
  func Generation(root string) int {
  	newTree := filepath.Join(root, ".awf", "config.yaml")
  	oldTree := filepath.Join(root, ".claude", "awf", "config.yaml")
  	legacy := filepath.Join(root, ".claude", "awf.yaml")
  	if _, err := os.Stat(newTree); err == nil {
  		l, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
  		if err != nil {
  			return Current()
  		}
  		return l.SchemaVersion
  	}
  	if _, err := os.Stat(oldTree); err == nil {
  		l, err := manifest.Load(filepath.Join(root, ".claude", "awf", "awf.lock"))
  		if err != nil {
  			return Current() - 1
  		}
  		return l.SchemaVersion
  	}
  	if _, err := os.Stat(legacy); err == nil {
  		return 0
  	}
  	return Current()
  }
  ```

- [ ] **Point `stampLockSchema` at the relocated lock** in `internal/migrate/migrate.go`:
  ```go
  	lockPath := filepath.Join(root, ".awf", "awf.lock")
  ```
  (the only line changed in that function).

- [ ] **Relocate the production path literals** (NOT adapter or legacy-migration paths):
  - `internal/project/project.go`:
    - `Open`: `config.Load(filepath.Join(root, ".claude", "awf"))` → `config.Load(filepath.Join(root, ".awf"))`.
    - `lockPath()`: `filepath.Join(p.Root, ".claude", "awf", "awf.lock")` → `filepath.Join(p.Root, ".awf", "awf.lock")`.
    - `partRel`: `".claude/awf/parts/agents-doc/"` → `".awf/parts/agents-doc/"`; `".claude/awf/" + kind + "/parts/"` → `".awf/" + kind + "/parts/"`.
    - `bannerText`: `change .claude/awf/ and run` → `change .awf/ and run`.
    - `orphans()`: the four `filepath.Join(p.Root, ".claude", "awf", kind)` /
      `filepath.Join(".claude", "awf", ...)` (scan base + three `Drift.Path`s) → drop the
      `".claude", "awf"` pair to `".awf"`, e.g. `filepath.Join(p.Root, ".awf", kind)` and
      `filepath.Join(".awf", kind, e.Name())` etc.
    - Comment at the `data` helper: `<root>/.claude/awf` → `<root>/.awf`.
  - `cmd/awf/main.go`: `runInit` cfgPath and the `initCollisions` lock path → `.awf`; package doc
    comment `.claude/awf/ config tree` → `.awf/ config tree`. (The `os.RemoveAll(filepath.Dir(cfgPath))`
    rollback now targets `.awf` automatically.)
  - `cmd/awf/list_add.go`: `runAdd` cfgPath `filepath.Join(root, ".claude", "awf", "config.yaml")` → `.awf`.
  - `internal/config/config.go`: package doc comment and the `root` field comment text `.claude/awf` → `.awf`.
  - `internal/manifest/manifest.go`: package doc comment `.claude/awf/awf.lock` → `.awf/awf.lock`.
  - `internal/project/scaffold.go`: doc comment `.claude/awf/config.yaml` → `.awf/config.yaml`.

  Verify no production code keeps an `.claude/awf` path except the frozen migration readers:
  ```
  grep -rn "claude/awf\|\"awf\"" --include="*.go" internal cmd | grep -v "_test.go" \
    | grep -v "internal/migrate/legacy.go" | grep -v "internal/migrate/treelayout.go" \
    | grep -v "internal/migrate/dropreplacewith.go"
  ```
  Expect only: `migrate.go` legacy/old-tree detection lines in `Generation` (intended), and the
  `applyTreeLayout`/`portSidecar` builders (intended — they construct the pre-relocation tree).

- [ ] **Update predecessor backing tests + all test fixtures** (`.claude/awf` → `.awf`):
  - `internal/config/config_test.go`: update the `config-root` test (line ~78) so its setup writes
    `.awf/config.yaml` and it asserts loading from `.awf`; add a second marker line directly above
    the existing one so the single test backs both slugs:
    ```go
    // invariant: config-root
    // invariant: awf-config-root
    ```
  - `internal/project/project_test.go`: in `scaffold`/`scaffoldFiles` (and `writeLocalSkill` if it
    writes under the config tree), change `.claude/awf` join segments to `.awf`.
  - `cmd/awf/run_test.go`: `scaffoldProject` and any fixture writing `.claude/awf` → `.awf`.
  - `internal/migrate/migrate_test.go`: the legacy-port fixtures keep `.claude/awf.yaml` /
    `.claude/awf/` (they exercise To:1/To:2 against the old layout — unchanged). Add the relocation
    test below. Several existing migrate tests **break** once `{To:3}` lands and must be updated in
    this same commit (they assert the old `Current()`, the old applied-migration chain, or a
    post-upgrade tree still at `.claude/awf/`):
    - `TestCurrentIsTwo` (asserts `Current() == 2`) → assert `3` (rename to `TestCurrentIsThree`).
    - `TestUpgradeAppliesInOrderIdempotent` — a gen-0 `Upgrade` now applies
      `tree-layout,drop-replacewith,awf-dir-relocation`, and the ported tree ends at `.awf/config.yaml`
      (not `.claude/awf/config.yaml`). Update both the `strings.Join(applied, ",")` expectation and
      the post-upgrade stat path; `stampLock` after the first upgrade must target `.awf/awf.lock`.
    - `TestUpgradeStampsTreeLock` — a tree at `.claude/awf/` with a schema-1 lock now upgrades through
      `drop-replacewith,awf-dir-relocation` and the restamped lock lives at `.awf/awf.lock`. Update the
      applied list and the `manifest.Load` path.
    - `TestGateBlocksWhenBehind` — semantic change, not just a value bump: under the rewritten
      `Generation`, a no-lock tree at `.claude/awf/` is now a **pre-relocation** tree that returns
      `Current()-1` and gates. Move the fresh-init/no-lock "must not gate, returns `Current()`"
      sub-case to a `.awf/config.yaml` fixture, and add (or convert) a `.claude/awf/` no-lock sub-case
      asserting `Generation == Current()-1` and `GateState == "gate"`. Keep the `upgraded` (lock at
      `Current()`) sub-case at `.awf/` so it represents a reachable state.
    - Coverage: the rewritten `Generation` adds six branches (`.awf/`+lock, `.awf/`+no-lock→`Current()`,
      `.claude/awf/`+lock, `.claude/awf/`+no-lock→`Current()-1`, legacy→0, none→`Current()`). Ensure
      the migrate tests collectively hit all six so `./x gate` stays at 100%.
    `TestNoopGapAutoBumps` uses `gateStateFor` with literals and needs no change.
  - Sweep the remaining test files for fixture paths:
    ```
    grep -rln "claude/awf\|\".claude\", \"awf\"" --include="*_test.go"
    ```
    Update every hit that constructs the *config tree* from `.claude/awf` → `.awf`. Known hits beyond
    the files named above: `internal/project/scaffold_test.go`, `cmd/awf/main_test.go` (also flips the
    `.claude/awf/config.yaml` + `.claude/awf/awf.lock` *assertions* in `TestRunInitScaffoldsAndSyncs`
    to `.awf/`), `cmd/awf/check_test.go`, `cmd/awf/invariants_test.go`, `cmd/awf/list_add_test.go`,
    plus `drift_test`, `coverage_test`, `docs_sections_test`, `domains_test`, `render_test`. Leave
    `migrate_test`'s legacy-layout fixtures and all adapter-output paths (`.claude/skills`,
    `.claude/agents`) untouched.

- [ ] **Add the relocation migration test** in `internal/migrate/migrate_test.go` backing
  `inv: awf-relocation-migration` (mirror the existing `t.TempDir()` + `os.WriteFile` fixture style):
  ```go
  // invariant: awf-relocation-migration
  func TestAwfRelocationGatesAndMoves(t *testing.T) {
  	root := t.TempDir()
  	old := filepath.Join(root, ".claude", "awf")
  	if err := os.MkdirAll(old, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(old, "config.yaml"), []byte("prefix: awf\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(old, "awf.lock"),
  		[]byte(`{"awfVersion":"0.1.0","schemaVersion":2,"files":{}}`+"\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if got := Generation(root); got != 2 {
  		t.Fatalf("pre-relocation Generation = %d, want 2", got)
  	}
  	if GateState(root) != "gate" {
  		t.Fatalf("expected gate state, got %q", GateState(root))
  	}
  	if _, err := Upgrade(root); err != nil {
  		t.Fatal(err)
  	}
  	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); err != nil {
  		t.Fatalf("config not relocated: %v", err)
  	}
  	if _, err := os.Stat(old); !os.IsNotExist(err) {
  		t.Fatal("old .claude/awf not removed")
  	}
  	if got := Generation(root); got != Current() {
  		t.Fatalf("post-relocation Generation = %d, want %d", got, Current())
  	}
  }
  ```
  Also add a no-op assertion (covers the `errors.Is ... ErrNotExist` branch):
  ```go
  func TestAwfRelocationNoopWhenAbsent(t *testing.T) {
  	if err := applyAwfRelocation(t.TempDir()); err != nil {
  		t.Fatal(err)
  	}
  }
  ```
  and the already-relocated guard (covers the `.awf exists` branch):
  ```go
  func TestAwfRelocationRefusesExistingTarget(t *testing.T) {
  	root := t.TempDir()
  	if err := os.MkdirAll(filepath.Join(root, ".claude", "awf"), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := applyAwfRelocation(root); err == nil {
  		t.Fatal("expected error when .awf already exists")
  	}
  }
  ```
  (The breaking `migrate_test.go` tests — `TestCurrentIsTwo`, `TestUpgradeAppliesInOrderIdempotent`,
  `TestUpgradeStampsTreeLock`, `TestGateBlocksWhenBehind` — are itemised in the predecessor-tests step
  above.)

- [ ] **Port this repo's config tree.** Run the relocation through the freshly-built binary:
  ```
  go run ./cmd/awf upgrade
  ```
  Expect `awf upgrade: applied awf-dir-relocation` then `awf sync: done`. Confirm `.awf/` now holds
  `config.yaml`, `awf.lock`, `skills/ agents/ docs/ domains/ parts/ agents-doc.yaml`, and
  `.claude/awf/` is gone:
  ```
  test -d .awf && test ! -d .claude/awf && echo RELOCATED
  ```

- [ ] **Re-sync and confirm the banner/pointer relocation.** `./x sync` (now reading `.awf`). Every
  rendered file's banner now reads `change .awf/`, and skill/agent `awf:edit` pointers read
  `.awf/...`. Stage all. Sanity:
  ```
  grep -rl "change .claude/awf/ and run" .claude docs AGENTS.md CLAUDE.md | head   # expect: no output
  ```

- [ ] **Gate.** `./x gate` — expect `0 issues.`, 100%. `./x check` — expect `awf check: clean`
  (Generation reads `.awf/awf.lock` at schema 3 → gate ok). If the invariant checker complains that
  ADR-0016 slugs are unbacked, that is expected to remain until Phase 6 (ADR-0016 is still
  `Accepted`, so its slugs are not yet enforced — confirm by re-reading the failure: it must NOT
  mention 0016 while Accepted).

- [ ] **Commit:** `feat(awf): relocate config tree to .awf, migration {To:3} (ADR-0016)`. The
  commit body notes the dogfood port ran via `awf upgrade` and rendered output is byte-identical
  except the relocated banner/pointer strings.

---

## Phase 5 — Prose & template sweep

Update the *source* of every current-guidance surface that names `.claude/awf/`, then re-sync so the
rendered docs regenerate. Leave historical ADRs (0002–0015) and historical plans as-is (they narrate
the pre-relocation world).

- [ ] **Templates** (`templates/`):
  - `templates/agents-doc/AGENTS.md.tmpl`: change `.claude/awf/` references (the "Working with awf"
    prose) → `.awf/`.
  - `templates/skills/proposing-adr/SKILL.md.tmpl`: the invariant-backing step's
    `.claude/awf/config.yaml` → `.awf/config.yaml`.

- [ ] **Convention parts / sidecars** now under `.awf/` that carry `.claude/awf` prose (from the
  Phase-4 survey): `.awf/agents-doc.yaml`, `.awf/parts/agents-doc/identity.md`,
  `.awf/docs/parts/architecture/{overview,components,data-flow}.md`,
  `.awf/skills/parts/debugging/debugging-surfaces.md`, `.awf/domains/parts/config/current-state.md` —
  update each `.claude/awf` → `.awf`. Re-list to be exact:
  ```
  grep -rln "claude/awf" .awf/
  ```

- [ ] **Satisfy the ADR's doc-currency content obligations** (Consequences §"Doc-currency
  obligations" — not just path strings; new narrative). Edit the source convention parts, then re-sync:
  - `.awf/docs/parts/architecture/{overview,components}.md` (and a new `target-seam` part if the
    architecture doc declares that section): document the **target seam**, the **neutral/adapter
    artifact taxonomy**, the `.awf/` config root as awf's home (vs `.claude/` holding only adapter
    output), and the **`CLAUDE.md` bridge**. These are new ADR-0016 concepts the current
    architecture body does not mention.
  - `.awf/domains/parts/config/current-state.md`: refresh the narrative to say config now lives at
    `.awf/` (relocated from `.claude/awf/` via migration `{To:3}`), beyond the path-string swap.
  - `tooling` domain current-state: refresh its narrative for the target seam + relocation migration
    (add the convention part under `.awf/domains/parts/tooling/` if absent, or edit the existing one).
  If the architecture doc has no section to host the target-seam narrative, escalate — adding a
  catalog section is outside this plan's scope.

- [ ] **Hand-maintained docs:** `docs/decisions/template.md` (`.claude/awf/config.yaml` →
  `.awf/config.yaml`) and `README.md` (the `.claude/awf/` config-tree + `.claude/awf/awf.lock`
  references → `.awf/`).

- [ ] **Re-sync** (`./x sync`) so `docs/architecture.md`, `docs/workflow.md`, `docs/development.md`,
  `docs/testing.md`, `docs/glossary.md`, `docs/pitfalls.md`, and `AGENTS.md` regenerate from the
  updated templates/parts. Stage all regenerated files.

- [ ] **Verify the sweep** is complete for current surfaces (historical ADRs/plans excepted):
  ```
  grep -rln "claude/awf" templates .awf docs/architecture.md docs/workflow.md docs/development.md \
    docs/testing.md docs/glossary.md docs/pitfalls.md docs/decisions/template.md AGENTS.md README.md
  ```
  Expect: no output.

- [ ] **Gate + check.** `./x gate` (100%, 0 issues) and `./x check` (clean).

- [ ] **Commit:** `docs(awf): relocate .claude/awf references to .awf (ADR-0016)`.

---

## Phase 6 — Flip ADR-0016 to Implemented

- [ ] **Update the ADR-0015 banner/pointer backing assertions** if any test asserts the banner *text*
  or an `awf:edit` pointer *path* literally (Phase 4 already changed the strings; this confirms the
  `provenance-banner` / `section-edit-pointer` backings still pass with `.awf/`). Search:
  ```
  grep -rn "change .claude/awf\|change .awf\|awf/parts" --include="*_test.go"
  ```
  Update any literal to `.awf/`. (If none assert the literal, no change.)

- [ ] **Flip status** in `docs/decisions/0016-tool-agnostic-target-seam-and-awf-relocation.md`
  frontmatter: `status: Accepted` → `status: Implemented`.

- [ ] **Regenerate the indexes.** `./x sync` — ADR-0016 moves to the `## Implemented` section of
  `ACTIVE.md` and the config/tooling/rendering domain indexes. Stage `ACTIVE.md` and the three
  `docs/domains/*.md`.

- [ ] **Enforce invariants.** `./x check` — now that 0016 is `Implemented`, the checker requires all
  five tagged slugs backed: `awf-config-root` (config_test.go), `target-output-paths`
  (target_test.go), `claude-md-bridge` (target_test.go), `init-collision-guard` (run_test.go),
  `awf-relocation-migration` (migrate_test.go). Expect `awf check: clean`. If any slug reports
  `Unbacked`, add the missing `// invariant: <slug>` marker to its test from the earlier phase.

- [ ] **Gate.** `./x gate` — `0 issues.`, 100%.

- [ ] **Commit:** `feat(awf): mark ADR-0016 Implemented; back tool-agnostic invariants`.

---

## Done criteria

- `.awf/` holds awf's config; `.claude/` holds only adapter output (`skills/`, `agents/`) plus the
  root `CLAUDE.md` bridge and `AGENTS.md`.
- `awf init` aborts on collisions and `--force` overrides; `awf upgrade` relocates a pre-existing
  `.claude/awf/` tree.
- ADR-0016 is `Implemented`; `./x gate` and `./x check` are clean; coverage 100%.
- Every rendered file's banner reads `change .awf/`; no current-guidance surface names `.claude/awf/`.
