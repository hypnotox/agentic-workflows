# Plan: Unify render loops + move init/uninstall logic out of cmd

**Date:** 2026-06-28
**ADR:** none — both items are plan-only refactors with no behaviour change and no new
architectural decision. Item 1 is the DRY follow-up ADR-0027 named but deferred; it deliberately
does **not** fold the render loops into the `kindDescriptor` (that would overscope ADR-0027 and
doesn't fit docs' neutral output path), using self-contained local closures instead. Item 2
applies the existing "cmd is thin dispatch, logic lives in internal" principle.

## Goal

1. **Item 1** — collapse the three near-identical `RenderAll` per-kind loops (skills/agents/docs)
   into one local render-driver helper in `render.go`.
2. **Item 2** — move init collision/backup planning and the uninstall removal algorithm from
   `cmd/awf` into `internal/project`, re-homing the `init-force-backs-up` and
   `uninstall-removes-lock-tracked` invariant markers, leaving the git/hooks bits in `cmd`.

No change to rendered output, lock format, config schema, or CLI behaviour. Coverage is an
**aggregate** 100% floor across the merged profile (confirmed: this repo's per-package numbers are
well below 100% while the total is 100% under `-coverpkg=./...`), so cmd tests that call `run(...)`
transitively cover moved `internal/project` code; direct project tests are added where cleaner.

## Architecture summary

- **Item 1**: `renderTarget` is untouched, so ConfigHash stays byte-identical and drift is
  unaffected. `RenderAll` output is consumed map-keyed by `.Path` (verified: `Check`/`Sync` build
  maps), so grouping skills/agents/docs together — moving the hooks block after docs — is
  order-safe. The skills doc-gate (`inv: doc-gated-skill-suppressed`) becomes an optional per-spec
  predicate (nil for agents/docs). Hooks (no sidecar), the agents-doc + CLAUDE bridge, and the
  adr-readme/adr-template/plans-readme singletons stay bespoke.
- **Item 2**: `initCollisions`→`(*Project).InitCollisions`; the `--force` backup becomes
  `(*Project).BackupFile`; `freeBackupPath`/`copyFile`/`fileExists` move to a new
  `internal/project/install.go`. Uninstall removal becomes a free function `project.Uninstall(root)`
  (a free function, not a method, so a broken config.yaml does not block uninstall — it only needs
  the lock + root). The git helpers (`unsetAwfHooks`, `openWorktree`, …) stay in `cmd/awf`.

## Tech stack

- Go 1.26; packages touched: `internal/project`, `cmd/awf`. Gate: `./x gate` (aggregate 100%
  coverage, golangci-lint) before each commit; `./x check` for drift.

## File structure

**Created:** `internal/project/install.go`, `internal/project/install_test.go`
**Modified:** `internal/project/render.go`, `cmd/awf/main.go`, `cmd/awf/uninstall.go`,
`cmd/awf/run_test.go`

---

## Phase 1 — Unify the RenderAll render loops

### Task 1.1 — Extract a local render-driver helper
- [ ] In `internal/project/render.go`, add the spec type and driver immediately above
  `func (p *Project) RenderAll()`:
  ```go
  // renderKindSpec drives one catalog-backed render loop (skills/agents/docs): the
  // kinds that share the sort → sidecar → skip-local → render → append shape. tid,
  // sections, and outPath derive from the target name; gate (optional, nil = always
  // render) suppresses a target — used for the skills doc-gate.
  type renderKindSpec struct {
  	kind     string
  	names    []string
  	tid      func(name string) string
  	sections func(name string) []string
  	outPath  func(name string) string
  	gate     func(name string) bool
  }

  func (p *Project) renderKind(spec renderKindSpec) ([]RenderedFile, error) {
  	var out []RenderedFile
  	for _, name := range slices.Sorted(slices.Values(spec.names)) {
  		sc, err := p.Cfg.Sidecar(spec.kind, name)
  		if err != nil {
  			return nil, err
  		}
  		if sc.Local {
  			continue
  		}
  		if spec.gate != nil && !spec.gate(name) {
  			continue
  		}
  		rf, err := p.renderTarget(spec.kind, name, spec.tid(name), spec.sections(name), sc, p.data(sc), spec.outPath(name))
  		if err != nil {
  			return nil, err
  		}
  		out = append(out, rf)
  	}
  	return out, nil
  }
  ```
- [ ] Replace the three loops in `RenderAll` — the skills loop (the `// Skills.` block:
  `enabledDocs := sliceSet(p.Cfg.Docs)` through the skills `for`/append), the `// Agents.` loop,
  and the `// Docs.` loop — with a single spec-driven block. The exact replacement for those three
  blocks:
  ```go
  	// Skills / agents / docs share one driver (order-independent: consumers are map-keyed by path).
  	enabledDocs := sliceSet(p.Cfg.Docs)
  	for _, spec := range []renderKindSpec{
  		{
  			kind: "skills", names: p.Cfg.Skills,
  			tid:      func(n string) string { return fmt.Sprintf("skills/%s/SKILL.md.tmpl", n) },
  			sections: func(n string) []string { return p.Cat.Skills[n].Sections },
  			outPath:  func(n string) string { return p.Target.SkillPath(p.Cfg.Prefix, n) },
  			// Doc-gated skill: omit from the render set when its required doc is not
  			// enabled (inv: doc-gated-skill-suppressed).
  			// invariant: doc-gated-skill-suppressed
  			gate: func(n string) bool {
  				req := p.Cat.Skills[n].RequiresDoc
  				return req == "" || enabledDocs[req]
  			},
  		},
  		{
  			kind: "agents", names: p.Cfg.Agents,
  			tid:      func(n string) string { return fmt.Sprintf("agents/%s.md.tmpl", n) },
  			sections: func(n string) []string { return p.Cat.Agents[n].Sections },
  			outPath:  func(n string) string { return p.Target.AgentPath(n) },
  		},
  		{
  			kind: "docs", names: p.Cfg.Docs,
  			tid:      func(n string) string { return fmt.Sprintf("docs/%s.md.tmpl", n) },
  			sections: func(n string) []string { return p.Cat.Docs[n].Sections },
  			outPath:  func(n string) string { return p.docOutPath(n) },
  		},
  	} {
  		rfs, err := p.renderKind(spec)
  		if err != nil {
  			return nil, err
  		}
  		out = append(out, rfs...)
  	}
  ```
  Leave the `// Hooks.` loop, the `// agents-doc …` block, the CLAUDE bridge, and the
  adr-readme/adr-template/plans-readme singleton block exactly as they are (the hooks block now
  follows the driver block). Note: the old skills doc-gate in `render.go` carries only the prose
  `(inv: doc-gated-skill-suppressed)` reference — there is no standalone `// invariant:` marker
  line there to delete (the backing marker for this slug lives on the test at
  `internal/project/project_test.go:556` and stays). The new `// invariant: doc-gated-skill-suppressed`
  line on the spec's `gate` adds a source-side backing, which is a net improvement.
- [ ] `go build ./... ` then `gofmt -w internal/project/render.go`.
- [ ] Verify drift unchanged (renderTarget args identical, ConfigHash byte-stable):
  `./x check` → `awf check: clean`.
- [ ] Verify the doc-gate still fires: `go test ./internal/project/ -run DocGated` → PASS
  (`TestRenderAllSuppressesDocGatedSkill`).
- [ ] `./x gate` → green (aggregate 100%). If any new branch is uncovered, the offender is the
  `spec.gate != nil` false arm or a gate predicate arm — all are exercised by existing tests
  (agents/docs have nil gate; skills cover req=="" and the suppressed case). Add no coverage-ignore
  without first confirming the arm is genuinely unreachable.
- [ ] Commit: `refactor(awf): unify the RenderAll skills/agents/docs loops`.

---

## Phase 2 — Move init/uninstall logic into internal/project

### Task 2.1 — Move init collision + backup planning to internal/project
- [ ] Create `internal/project/install.go`:
  ```go
  package project

  import (
  	"fmt"
  	"os"
  	"path/filepath"
  	"sort"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  )

  // InitCollisions returns planned output paths that already exist on disk and are
  // not recorded in the prior lock (i.e. not awf-managed). An awf-managed path that
  // already exists is not a collision — re-init is idempotent.
  func (p *Project) InitCollisions() ([]string, error) {
  	planned, err := p.PlannedOutputs()
  	if err != nil {
  		return nil, err
  	}
  	managed := map[string]bool{}
  	if lock, err := manifest.Load(p.lockPath()); err == nil {
  		for path := range lock.Files {
  			managed[path] = true
  		}
  	}
  	var collisions []string
  	for _, rel := range planned {
  		if managed[rel] {
  			continue
  		}
  		if _, err := os.Stat(filepath.Join(p.Root, rel)); err == nil {
  			collisions = append(collisions, rel)
  		}
  	}
  	sort.Strings(collisions)
  	return collisions, nil
  }

  // BackupFile copies a colliding project-relative file to a free <path>.awf-bak[.N]
  // sibling (never clobbering a prior backup) and returns the backup's
  // project-relative path.
  // invariant: init-force-backs-up
  func (p *Project) BackupFile(rel string) (string, error) {
  	src := filepath.Join(p.Root, rel)
  	bak := freeBackupPath(src)
  	if err := copyFile(src, bak); err != nil { // coverage-ignore: rel is a known-existing collision and bak is a free sibling path; copyFile fails only on a permission fault root bypasses
  		return "", err
  	}
  	bakRel, _ := filepath.Rel(p.Root, bak)
  	return bakRel, nil
  }

  // freeBackupPath returns base+".awf-bak", or "...awf-bak.N" with the lowest N
  // that does not yet exist, so a forced backup never overwrites a prior one.
  func freeBackupPath(base string) string {
  	p := base + ".awf-bak"
  	for i := 1; fileExists(p); i++ {
  		p = fmt.Sprintf("%s.awf-bak.%d", base, i)
  	}
  	return p
  }

  func fileExists(p string) bool {
  	_, err := os.Stat(p)
  	return err == nil
  }

  // copyFile copies src to dst, preserving the source file's permission bits.
  func copyFile(src, dst string) error {
  	info, err := os.Stat(src)
  	if err != nil { // coverage-ignore: src is a known-existing collision path
  		return err
  	}
  	data, err := os.ReadFile(src)
  	if err != nil { // coverage-ignore: src was just stat'd and is readable
  		return err
  	}
  	return os.WriteFile(dst, data, info.Mode().Perm())
  }
  ```
- [ ] In `cmd/awf/main.go`, delete `initCollisions` (the whole func), `freeBackupPath`,
  `fileExists`, and `copyFile`. Rewrite `runInit` to open the project once and delegate. Replace
  the body from `collisions, err := initCollisions(root)` through the end of the `--force` backup
  loop so it reads:
  ```go
  	p, err := project.Open(root)
  	if err != nil {
  		return err
  	}
  	collisions, err := p.InitCollisions()
  	if err != nil {
  		return err
  	}
  	if len(collisions) > 0 {
  		if !force {
  			if scaffolded {
  				_ = os.Remove(cfgPath)               // remove the config we scaffolded
  				_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
  			}
  			return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
  				strings.Join(collisions, "\n  "))
  		}
  		// --force: back up each colliding non-managed file before sync overwrites it.
  		for _, rel := range collisions {
  			bakRel, err := p.BackupFile(rel)
  			if err != nil { // coverage-ignore: p.BackupFile only fails on a copyFile permission fault that root bypasses
  				return fmt.Errorf("awf init: back up %s: %w", rel, err)
  			}
  			fmt.Fprintf(stdout, "backed up %s → %s\n", rel, bakRel)
  		}
  	}
  ```
  (The `// invariant: init-force-backs-up` marker is removed here — it re-homes onto
  `BackupFile` in `install.go`. The scaffold block above `p, err := project.Open` is unchanged.)
- [ ] Fix `cmd/awf/main.go` imports: remove `"sort"` and the `internal/manifest` import **iff**
  no longer used (`rg -n 'sort\.|manifest\.' cmd/awf/main.go` — both were only used by the deleted
  `initCollisions`; remove whichever has no remaining use). `go build ./...` confirms.
- [ ] Rewrite the two tests that called `initCollisions` directly:
  - `cmd/awf/run_test.go` `TestInitCollisionsOpenError` (the bad-config case): the Open error now
    surfaces inside `runInit`, so drive it through `run`:
    ```go
    func TestInitCollisionsOpenError(t *testing.T) {
    	root := t.TempDir()
    	dir := filepath.Join(root, ".awf")
    	if err := os.MkdirAll(dir, 0o755); err != nil {
    		t.Fatal(err)
    	}
    	// Unknown field → strict config.Load fails → project.Open errors inside runInit.
    	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("bogusField: true\n"), 0o644); err != nil {
    		t.Fatal(err)
    	}
    	swapGetwd(t, func() (string, error) { return root, nil })
    	var out, errb bytes.Buffer
    	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
    		t.Fatal("expected init to fail when project.Open errors")
    	}
    }
    ```
  - Move `TestInitCollisionsPlannedOutputsError` out of `cmd/awf/run_test.go` and into a new
    `internal/project/install_test.go` as a direct method test (it tests the PlannedOutputs error
    path of `InitCollisions`):
    ```go
    package project

    import (
    	"os"
    	"path/filepath"
    	"testing"
    )

    func TestInitCollisionsSurfacesPlannedOutputsError(t *testing.T) {
    	root := t.TempDir()
    	awf := filepath.Join(root, ".awf")
    	if err := os.MkdirAll(awf, 0o755); err != nil {
    		t.Fatal(err)
    	}
    	if err := os.WriteFile(filepath.Join(awf, "config.yaml"),
    		[]byte("prefix: awf\nskills: []\nagents: []\nhooks: []\ndocs: []\n"), 0o644); err != nil {
    		t.Fatal(err)
    	}
    	// A malformed ADR makes generateActiveMD (inside PlannedOutputs) fail.
    	dd := filepath.Join(root, "docs", "decisions")
    	if err := os.MkdirAll(dd, 0o755); err != nil {
    		t.Fatal(err)
    	}
    	if err := os.WriteFile(filepath.Join(dd, "0099-bad.md"), []byte("---\nstatus: [unclosed\n---\n# Bad\n"), 0o644); err != nil {
    		t.Fatal(err)
    	}
    	p, err := Open(root)
    	if err != nil {
    		t.Fatal(err)
    	}
    	if _, err := p.InitCollisions(); err == nil {
    		t.Fatal("expected InitCollisions to surface the PlannedOutputs error")
    	}
    }
    ```
    Delete the original `TestInitCollisionsPlannedOutputsError` from `run_test.go`.
  - A third test, `TestInitAbortsWhenInitCollisionsFails` (in `run_test.go`), drives `run` with a
    malformed ADR and still passes unchanged (the error now surfaces via `p.InitCollisions()` inside
    `runInit`). Refresh its comment, which currently says "PlannedOutputs (inside `initCollisions`)",
    to name `p.InitCollisions` so it does not reference the deleted free function.
- [ ] `go build ./...`; `gofmt -w` the touched files.
- [ ] Verify: `go test ./internal/project/ ./cmd/awf/` → PASS; `./x gate` → green (aggregate
  100% — the force-backup branches stay covered by `TestInitGuardBlocksAndForceOverrides` /
  `TestInitForceBackupDoesNotClobberPriorBak` via `run`, now transitively into `project`).
- [ ] `./x check` → clean. Commit:
  `refactor(awf): move init collision/backup planning into internal/project`.

### Task 2.2 — Move the uninstall removal algorithm to internal/project
- [ ] In `internal/project/install.go`, add (imports: add `"maps"`, `"slices"` to the file's
  import block):
  ```go
  // Uninstall removes awf's generated footprint from root: every file recorded in
  // the lock, the directories left empty by their removal, and the now-stale lock
  // itself. It leaves the authored .awf/ config in place and returns the count of
  // files removed. It is a free function (not a *Project method) so a broken
  // config.yaml does not block uninstall — only the lock and root are needed.
  // invariant: uninstall-removes-lock-tracked
  func Uninstall(root string) (int, error) {
  	lockPath := filepath.Join(root, ".awf", "awf.lock")
  	lock, err := manifest.Load(lockPath)
  	if err != nil {
  		return 0, fmt.Errorf("no %s — nothing to uninstall", filepath.Join(".awf", "awf.lock"))
  	}
  	removed := 0
  	dirs := map[string]bool{}
  	for path := range lock.Files {
  		abs := filepath.Join(root, path)
  		if err := os.Remove(abs); err == nil {
  			removed++
  		}
  		for d := filepath.Dir(abs); d != root; d = filepath.Dir(d) {
  			dirs[d] = true
  		}
  	}
  	// Remove now-empty directories deepest-first (a child path is always longer
  	// than its parent, so descending-length order attempts children first).
  	dirList := slices.Collect(maps.Keys(dirs))
  	slices.SortFunc(dirList, func(a, b string) int { return len(b) - len(a) })
  	for _, d := range dirList {
  		_ = os.Remove(d) // removes only if now empty
  	}
  	if err := os.Remove(lockPath); err != nil { // coverage-ignore: lock was just loaded, so removal fails only on a permission fault root bypasses
  		return removed, fmt.Errorf("remove lock: %w", err)
  	}
  	return removed, nil
  }
  ```
  Update `install.go`'s import block to include `"maps"` and `"slices"` alongside the existing
  `fmt`/`os`/`path/filepath`/`sort`/`manifest`.
- [ ] In `cmd/awf/uninstall.go`, replace the removal body of `runUninstall` (the lock load through
  the `os.Remove(lockPath)` block, lines from `lockPath := …` to the `removed %d` Fprintf) with a
  delegation, keeping the git/hooks teardown:
  ```go
  func runUninstall(root string, stdout io.Writer) error {
  	removed, err := project.Uninstall(root)
  	if err != nil {
  		return err
  	}
  	fmt.Fprintf(stdout, "awf uninstall: removed %d generated file(s) and the lock\n", removed)
  	unsetAwfHooks(root, stdout)
  	fmt.Fprintln(stdout, "awf uninstall: left the .awf/ config in place (delete it to fully remove)")
  	return nil
  }
  ```
  Remove the now-unused `// invariant: uninstall-removes-lock-tracked` marker from `uninstall.go`
  (re-homed onto `project.Uninstall`). Fix `cmd/awf/uninstall.go` imports: after delegation the
  only remaining users of `"os"`, `"path/filepath"`, `"maps"`, `"slices"`, and `internal/manifest`
  are gone — `runUninstall` no longer touches them and `unsetAwfHooks` uses none of them (its git
  helpers live in another file) — so drop all five; the file then imports only `"fmt"`, `"io"`, and
  `internal/project`. `go build ./...` confirms the exact set.
- [ ] Verify: `go test ./cmd/awf/ ./internal/project/` → PASS — the existing
  `TestUninstallRemovesGeneratedFilesAndLock` / `TestUninstallNoLockErrors` drive `runUninstall`
  and now transitively cover `project.Uninstall` (aggregate coverage). If a removal branch shows
  uncovered, add a direct `project.Uninstall` test in `install_test.go`.
- [ ] `./x gate` → green; `./x check` → clean. Commit:
  `refactor(awf): move uninstall removal into internal/project`.

---

## Final verification
- [ ] `./x gate full` → green (aggregate 100% coverage, lint clean).
- [ ] `./x check` → `awf check: clean`; `./x invariants` → `awf invariants: clean` (both re-homed
  markers resolve at their new `internal/project` locations).
- [ ] Add `# Implementation complete (YYYY-MM-DD)` header to this plan (freezes it — non-ADR).
- [ ] Invoke `awf-reviewing-impl` (terminal review of the series).
