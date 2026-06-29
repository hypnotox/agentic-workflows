# Plan: Brownfield-adoption hardening (ADR-0034 + ADR-0035)

**Date:** 2026-06-29
**Branch:** `brownfield-adoption-hardening`
**Linked ADRs:**
- [ADR-0034: Convention Parts Are Raw Input](../decisions/0034-convention-parts-are-raw-input.md)
- [ADR-0035: Brownfield-Safe Sync Writes](../decisions/0035-brownfield-safe-sync-writes.md)

Design rationale lives in the ADRs — this plan is the execution record only.

## Goal

Close two awf-adoption traps reported by the first external adopter:
1. A literal `{{` in a convention part breaks `awf sync` (ADR-0034) — make convention parts raw,
   never run through `text/template`.
2. `awf sync` silently overwrites hand-written / externally-generated files (ADR-0035) — back up
   any foreign file before overwriting and surface an ADR-index ownership takeover.

## Architecture summary

- **ADR-0034 (render layer, `internal/render` + `internal/project`):** `Assemble` emits a brace-free
  NUL-delimited sentinel in each part slot and returns a `sentinel → raw body` map; `Execute`
  templates only the awf-owned skeleton (sentinels inert), then restores raw bodies verbatim, and is
  given the target name for error messages. The sentinel-form `assembled` string flows unchanged into
  `targetConfigHash`; because no existing part contains `.vars.` or `{{`, `render.ReferencedVars`
  yields an identical set → identical `ConfigHash` → zero drift (the golden re-render is the proof).
- **ADR-0035 (project + cmd layer, `internal/project` + `cmd/awf`):** `Sync` gains `SyncReport`,
  which loads the *prior* lock before the write loop and, for any target path that exists on disk but
  is absent from that lock, backs it up via the existing `BackupFile` before overwriting, returning
  the backups. `runSync` prints them and an ownership note when the file is the generated ADR/domain
  index. `init`'s now-redundant `--force` backup loop is removed — sync's backup subsumes it (one
  mechanism), avoiding a double-backup.

## Tech stack

- Go 1.26; standard `text/template`, `strings`, `os`, `errors`, `regexp`.
- Packages touched: `internal/render`, `internal/project`, `cmd/awf`, plus `.awf/` doc/domain parts.
- Gate: `./x gate` (100% coverage, golangci-lint, drift check) before every commit.

## File structure

**Modified:**
- `internal/render/render.go` — `Assemble`/`Execute` signatures + sentinel + restore (`inv: parts-raw`)
- `internal/render/render_test.go` — updated call sites + new raw-part test
- `internal/project/render.go` — `renderTarget` caller update
- `internal/project/frontmatter_test.go`, `spine_test.go`, `docs_sections_test.go` — call-site updates
- `internal/project/project.go` — `Backup` type, `SyncReport`, `isGeneratedIndex` (`inv: sync-backs-up-foreign`); `Sync` becomes a wrapper
- `internal/project/project_test.go` — new sync-backup + ownership-note + init-no-double-backup tests
- `cmd/awf/main.go` — `runSync` prints backups + note; init `--force` backup loop removed
- `cmd/awf/run_test.go` — adjust `TestInitGuardBlocksAndForceOverrides` (single-backup regression) + new `TestSyncReportsIndexOwnershipTakeover` (cmd-layer `if b.Index` note coverage)
- `.awf/docs/parts/architecture/data-flow.md` — render-flow note (parts protected from `text/template`)
- `.awf/domains/parts/rendering/current-state.md` — parts-are-raw note
- `.awf/domains/parts/tooling/current-state.md` — sync-backup + ownership note
- `docs/decisions/0034-*.md`, `docs/decisions/0035-*.md` — status flip Proposed → Implemented (final)
- Regenerated: `docs/architecture.md`, `docs/domains/rendering.md`, `docs/domains/tooling.md`,
  `docs/decisions/ACTIVE.md`, `.awf/awf.lock`

**Created:** none. **Deleted:** none.

---

## Phase 1 — ADR-0034: convention parts are raw

### Task 1.1 — Rework `render.Assemble` / `render.Execute`

In `internal/render/render.go`, replace the `Assemble` and `Execute` functions (lines 30-68) with:

```go
// partSentinel is the brace-free, NUL-delimited placeholder emitted in a part's
// slot. NUL bytes cannot occur in template or markdown text, so the token can
// never collide with rendered content, and being brace-free it is inert to the
// template parser.
func partSentinel(name string) string {
	return "\x00awf:part:" + name + "\x00"
}

// Assemble applies the per-section plan to the parsed segments and returns the
// template skeleton plus a sentinel→raw-body map. Literal segments pass through
// verbatim; each non-dropped section is prefixed with its awf:edit pointer, then
// either a sentinel standing in for its part body (restored after Execute) or the
// template default. Section markers are consumed here and never written.
// invariant: no-section-marker-leak
func Assemble(segs []Segment, plan map[string]SectionPlan) (string, map[string]string) {
	var b strings.Builder
	parts := map[string]string{}
	for _, s := range segs {
		if !s.IsSection {
			b.WriteString(s.Text)
			continue
		}
		p := plan[s.Name]
		if p.Drop {
			continue
		}
		b.WriteString(editPointer(s.Name, p))
		if p.HasPart {
			sent := partSentinel(s.Name)
			parts[sent] = p.PartBody
			b.WriteString(sent)
		} else {
			b.WriteString(s.Text)
		}
	}
	return b.String(), parts
}

// Execute runs text/template over the awf-owned skeleton (part bodies stood in by
// sentinels) under missingkey=zero, then restores each raw part body verbatim — so
// a convention part is never parsed or executed as a template. name labels parse
// and execute errors with the target rather than a hardcoded literal.
// invariant: parts-raw
func Execute(assembled string, data map[string]any, parts map[string]string, name string) (string, error) {
	t, err := template.New(name).Option("missingkey=zero").Parse(assembled)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var out strings.Builder
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	rendered := out.String()
	for sent, body := range parts {
		rendered = strings.ReplaceAll(rendered, sent, body)
	}
	return rendered, nil
}
```

### Task 1.2 — Update the production caller

In `internal/project/render.go`, `renderTarget` (lines 243-244), change:

```go
	assembled := render.Assemble(render.ParseSections(string(src)), plan)
	content, err := render.Execute(assembled, data)
```

to:

```go
	assembled, parts := render.Assemble(render.ParseSections(string(src)), plan)
	content, err := render.Execute(assembled, data, parts, tid)
```

(`assembled` — the sentinel-form skeleton — continues to flow into `targetConfigHash` at line 252
unchanged; `tid` names errors, e.g. `docs/architecture.md.tmpl`.)

### Task 1.3 — Update `render_test.go` call sites and add the raw-part test

In `internal/render/render_test.go`, update each existing call. The nested one-liners at lines 23,
42, 56 take the form `out, err := Execute(Assemble(ParseSections(tmpl), <plan>), sampleData())` —
split each into two statements:

```go
	asm, parts := Assemble(ParseSections(tmpl), <plan>)
	out, err := Execute(asm, sampleData(), parts, "test")
```

(use the same `<plan>` argument each site already passes — `nil` at line 23, `plan` at 42 and 56).

At lines 69 and 80, the direct calls `Execute("{{ .prefix", sampleData())` and
`Execute("{{ range .prefix }}{{ end }}", sampleData())` become:

```go
	_, err := Execute("{{ .prefix", sampleData(), nil, "test")
```
```go
	_, err := Execute("{{ range .prefix }}{{ end }}", sampleData(), nil, "test")
```

Then fix `TestRenderConventionPart` (line 54), whose assertion encodes the *old* templated-part
behaviour: it sets `PartBody: "CUSTOM {{ .prefix }}"` and asserts `strings.Contains(out, "CUSTOM
example")`. Under the raw-part contract the body is no longer interpolated, so it renders verbatim —
change the assertion to expect the literal `CUSTOM {{ .prefix }}` (keep the `NOTE`-absent and
edit-pointer checks). Leaving the old assertion makes the test fail after Task 1.1.

Then append a new test proving the raw-part contract (`inv: parts-raw`):

```go
func TestPartBodyIsRawNeverTemplated(t *testing.T) {
	tmpl := "<!-- awf:section body -->DEFAULT {{ .prefix }}<!-- awf:end -->\n"
	plan := map[string]SectionPlan{"body": {
		HasPart:  true,
		PartBody: "Literal braces survive: {{ .vars.x }} {{ if }} }} and a mustache {{name}}.",
		EditPath: ".awf/x/parts/y/body.md",
	}}
	asm, parts := Assemble(ParseSections(tmpl), plan)
	out, err := Execute(asm, sampleData(), parts, "raw-test")
	if err != nil {
		t.Fatalf("Execute over a part with literal braces must not error: %v", err)
	}
	want := "Literal braces survive: {{ .vars.x }} {{ if }} }} and a mustache {{name}}."
	if !strings.Contains(out, want) {
		t.Fatalf("part body must render verbatim (not interpolated)\n got: %q\nwant substring: %q", out, want)
	}
	if strings.Contains(out, "<no value>") || strings.Contains(out, "\x00") {
		t.Fatalf("part body was interpolated or a sentinel leaked: %q", out)
	}
}
```

(If `sampleData()` lacks a `vars` key, the test still passes — the point is the part text is *not*
evaluated, so an undefined `.vars.x` inside it can never produce `<no value>`.)

### Task 1.4 — Update the project-layer test call sites

These three tests call `render.Execute(render.Assemble(...), data)` directly. Split each:

- `internal/project/frontmatter_test.go:54`
- `internal/project/spine_test.go:19`
- `internal/project/docs_sections_test.go:43` and `:132`

For each, replace the single `out, err := render.Execute(render.Assemble(render.ParseSections(string(src)), nil), <data>)`
line with:

```go
	asm, parts := render.Assemble(render.ParseSections(string(src)), nil)
	out, err := render.Execute(asm, <data>, parts, "test")
```

preserving each site's existing `<data>` argument verbatim (the `map[string]any{...}` literal at
`docs_sections_test.go:132` spans multiple lines — keep it as the second argument).

### Task 1.5 — Update the rendering doc-currency parts

In `.awf/docs/parts/architecture/data-flow.md`, add to the render-flow description that
convention-part bodies are protected from `text/template` by sentinel substitution and restored
verbatim after execution (so a part is never templated). Keep the existing prose; extend it.

In `.awf/domains/parts/rendering/current-state.md`, note that convention parts are raw input — awf
templates only its own embedded defaults; a part renders byte-for-byte (ADR-0034).

### Task 1.6 — Flip ADR-0034, regenerate, verify, commit

1. In `docs/decisions/0034-convention-parts-are-raw-input.md`, change `status: Proposed` to
   `status: Implemented`.
2. Run `./x sync` (regenerates `docs/architecture.md`, `docs/domains/rendering.md`,
   `docs/decisions/ACTIVE.md`, `.awf/awf.lock`).
3. Run `./x check` — expect `awf check: clean` (the zero-drift proof: awf's own rendered tree is
   byte-identical despite the sentinel rework).
4. Run `./x gate` — expect `coverage: 100.0%` and `0 issues.`.
5. Stage explicitly and commit:

```
git add internal/render/render.go internal/render/render_test.go internal/project/render.go \
  internal/project/frontmatter_test.go internal/project/spine_test.go internal/project/docs_sections_test.go \
  .awf/docs/parts/architecture/data-flow.md .awf/domains/parts/rendering/current-state.md \
  docs/decisions/0034-convention-parts-are-raw-input.md docs/decisions/ACTIVE.md \
  docs/architecture.md docs/domains/rendering.md .awf/awf.lock
git commit -m "feat(awf): make convention parts raw input (ADR-0034)"
```

Commit message body: summarise that parts are no longer templated; sentinel-protected single pass;
error name fix; zero drift.

---

## Phase 2 — ADR-0035: brownfield-safe sync writes

### Task 2.1 — Add `Backup`, `SyncReport`, `isGeneratedIndex`; make `Sync` a wrapper

In `internal/project/project.go`:

1. Add `"errors"` and `"strings"` to the import block.
2. Replace the existing `func (p *Project) Sync() error { ... }` (lines 54-115) with the wrapper plus
   the reporting variant. The body is the current Sync body with the write loop guarded and the
   prior lock hoisted:

```go
// Backup records a foreign file preserved before sync overwrote its path.
type Backup struct {
	Path  string // project-relative file that was overwritten
	Bak   string // project-relative backup copy (.awf-bak[.N])
	Index bool   // the file is the generated ADR/domain index (ownership-takeover note)
}

func (p *Project) Sync() error {
	_, err := p.SyncReport()
	return err
}

// SyncReport renders and writes the project like Sync, additionally backing up any
// foreign file (on disk but absent from the start-of-sync lock) before overwriting
// it, and returning those backups (ADR-0035).
func (p *Project) SyncReport() ([]Backup, error) {
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if isSkillOrAgent(f.TemplateID) {
			if err := validateFrontmatter([]byte(f.Content)); err != nil { // coverage-ignore: rendered catalog skill/agent frontmatter is template-fixed (non-empty name/description guaranteed by inv templates-valid-frontmatter); it cannot be invalid at sync time
				return nil, fmt.Errorf("invalid frontmatter in %s: %w", f.Path, err)
			}
		}
	}
	var localErr error
	if err := p.checkLocalFrontmatter(func(path string, e error) {
		if localErr == nil {
			localErr = fmt.Errorf("local target %s: %w", path, e)
		}
	}); err != nil { // coverage-ignore: checkLocalFrontmatter only errors on a malformed local-target sidecar, which RenderAll above already surfaces earlier in Sync
		return nil, err
	}
	if localErr != nil {
		return nil, localErr
	}
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	files = append(files, amd)
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — generateActiveMD above parses the same decisions dir and fails first on a malformed ADR
		return nil, err
	}
	files = append(files, dds...)

	// Prior lock, read before any write: membership decides foreign (back up) vs
	// awf-managed (overwrite silently), and drives pruning below.
	old, _ := manifest.Load(p.lockPath())
	prior := map[string]bool{}
	if old != nil {
		for path := range old.Files {
			prior[path] = true
		}
	}

	var backups []Backup
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	want := map[string]bool{}
	for _, f := range files {
		abs := filepath.Join(p.Root, f.Path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		if !prior[f.Path] {
			if _, statErr := os.Stat(abs); statErr == nil {
				// invariant: sync-backs-up-foreign
				bak, err := p.BackupFile(f.Path)
				if err != nil { // coverage-ignore: BackupFile only fails on a copyFile permission fault that root bypasses
					return nil, fmt.Errorf("back up %s: %w", f.Path, err)
				}
				backups = append(backups, Backup{Path: f.Path, Bak: bak, Index: p.isGeneratedIndex(f.Path)})
			} else if !errors.Is(statErr, os.ErrNotExist) { // coverage-ignore: os.Stat returns a non-NotExist error only on a permission/IO fault that root bypasses
				return nil, statErr
			}
		}
		if err := os.WriteFile(abs, []byte(f.Content), 0o644); err != nil {
			return nil, err
		}
		lock.Files[f.Path] = manifest.Entry{
			TemplateID: f.TemplateID, TemplateHash: f.TemplateHash,
			ConfigHash: f.ConfigHash, OutputHash: manifest.Hash([]byte(f.Content)),
		}
		want[f.Path] = true
	}
	// Prune files from the previous lock that are no longer produced.
	if old != nil {
		for path := range old.Files {
			if !want[path] {
				file := filepath.Join(p.Root, path)
				_ = os.Remove(file)
				_ = os.Remove(filepath.Dir(file)) // only succeeds if now empty
			}
		}
	}
	if err := lock.Save(p.lockPath()); err != nil {
		return nil, err
	}
	return backups, nil
}

// isGeneratedIndex reports whether rel is the generated ADR index or a per-domain
// index — the awf-owned generated docs whose first-time takeover warrants a note.
func (p *Project) isGeneratedIndex(rel string) bool {
	lay := p.layout()
	return rel == lay.ActiveMd || strings.HasPrefix(rel, lay.DomainsDir+"/")
}
```

Note: the prior-lock load replaces the old line-105 reload; `manifest.Load` returning an error
(no lock on first sync) yields `old == nil` → empty `prior` → every existing output path is foreign,
matching `init`'s adoption behaviour.

### Task 2.2 — Print backups in `runSync`; remove init's `--force` backup loop

In `cmd/awf/main.go`, `runSync` (lines 320-333), change the `p.Sync()` call to:

```go
	backups, err := p.SyncReport()
	if err != nil {
		return err
	}
	for _, b := range backups {
		fmt.Fprintf(stdout, "backed up %s → %s\n", b.Path, b.Bak)
		if b.Index {
			fmt.Fprintf(stdout, "  note: awf now generates %s; retire any external generator for it\n", b.Path)
		}
	}
	fmt.Fprintln(stdout, "awf sync: done")
	return nil
```

Then remove init's now-redundant explicit backup loop. The current block (lines ~285-302):

```go
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

becomes (refuse-without-force kept; backup delegated to the chained sync):

```go
	if len(collisions) > 0 && !force {
		if scaffolded {
			_ = os.Remove(cfgPath)               // remove the config we scaffolded
			_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
		}
		return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
			strings.Join(collisions, "\n  "))
	}
	// Under --force, the chained runSync backs up every foreign file via the shared
	// BackupFile mechanism (ADR-0035) — one backup path for init and sync alike.
```

### Task 2.3 — Tests: sync backup, ownership note, init no-double-backup

In `internal/project/project_test.go`, add two concrete tests (the test is in `package project`,
so the unexported `layout`, `isGeneratedIndex`, `SyncReport`, and `Backup` are all in scope; reuse
the `scaffold` helper and `sampleYAML` at the top of the file):

```go
func TestSyncReportBacksUpForeignIndexNotManaged(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	lay := p.layout()
	// Plant a foreign ADR index with hand content before the first sync (no lock yet),
	// so its path is absent from the prior lock and therefore foreign.
	foreign := filepath.Join(root, lay.ActiveMd)
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	backups, err := p.SyncReport()
	if err != nil {
		t.Fatalf("SyncReport: %v", err)
	}
	var got *Backup
	for i := range backups {
		if backups[i].Path == lay.ActiveMd {
			got = &backups[i]
		}
	}
	if got == nil {
		t.Fatalf("foreign ACTIVE.md not backed up; backups=%#v", backups)
	}
	if !got.Index {
		t.Errorf("ACTIVE.md backup must be flagged Index=true")
	}
	if b, _ := os.ReadFile(filepath.Join(root, got.Bak)); string(b) != "hand index\n" {
		t.Errorf("backup = %q, want original hand content", b)
	}
	// A path recorded in the prior lock is awf-managed: a second sync backs up nothing.
	again, err := p.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("re-sync of awf-managed output must not back up, got %#v", again)
	}
}

func TestIsGeneratedIndex(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	lay := p.layout()
	if !p.isGeneratedIndex(lay.ActiveMd) {
		t.Errorf("ActiveMd must be a generated index (true via ==)")
	}
	if !p.isGeneratedIndex(lay.DomainsDir + "/rendering.md") {
		t.Errorf("a per-domain index must be a generated index (true via prefix)")
	}
	if p.isGeneratedIndex(lay.DocsDir + "/architecture.md") {
		t.Errorf("an ordinary doc must not be a generated index (false)")
	}
}
```

The first test exercises the `SyncReport` backup branch and the `Index==true` flag; the second pins
both `||` arms of `isGeneratedIndex` and its false arm — together satisfying the 100% coverage gate
for the new project-layer branches.

In `cmd/awf/main.go`, `runSync` gains an `if b.Index` note-print branch (Task 2.2) that neither the
project tests above nor `TestInitGuardBlocksAndForceOverrides` (which backs up a non-index
`CLAUDE.md`, `Index==false`) reach. The 100% coverage gate (ADR-0012) therefore requires a
cmd-layer test that drives it. Add to `cmd/awf/run_test.go` (default `docsDir` is `docs`, so the ADR
index lands at `docs/decisions/ACTIVE.md`):

```go
func TestSyncReportsIndexOwnershipTakeover(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Foreign ADR index present before any sync (no lock yet).
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "ACTIVE.md"), []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swapGetwd(t, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 0 {
		t.Fatalf("sync: %s", errb.String())
	}
	if !strings.Contains(out.String(), "backed up docs/decisions/ACTIVE.md") {
		t.Errorf("missing backup line: %q", out.String())
	}
	if !strings.Contains(out.String(), "note: awf now generates") {
		t.Errorf("missing ownership-takeover note: %q", out.String())
	}
}
```

Finally, in `cmd/awf/run_test.go`, keep `TestInitGuardBlocksAndForceOverrides` green: under
`--force` the colliding `CLAUDE.md` must still end up at `CLAUDE.md.awf-bak` (now created by the
chained sync) and the "backed up CLAUDE.md" line must still print (now from `runSync`, before
"awf sync: done" — same observable order, so an order-sensitive assertion still holds). Add a
regression assertion that exactly one `.awf-bak` (no `.awf-bak.1`) exists for the colliding file,
proving init no longer double-backs-up. Do not weaken the `.awf-bak` content assertion.

### Task 2.4 — Update the tooling doc-currency part

In `.awf/domains/parts/tooling/current-state.md`, note that `awf sync` backs up any foreign
(not-in-prior-lock) file before overwriting — alongside `init --force` — and surfaces an
ADR-index ownership-takeover note (ADR-0035), and that init delegates its backup to the shared
sync mechanism.

### Task 2.5 — Flip ADR-0035, regenerate, verify, commit

1. In `docs/decisions/0035-brownfield-safe-sync-writes.md`, change `status: Proposed` to
   `status: Implemented`.
2. `./x sync` (regenerates `docs/domains/tooling.md`, `docs/decisions/ACTIVE.md`, `.awf/awf.lock`).
3. `./x check` — expect `awf check: clean`.
4. `./x gate` — expect `coverage: 100.0%` and `0 issues.`.
5. Stage explicitly and commit:

```
git add internal/project/project.go internal/project/project_test.go cmd/awf/main.go cmd/awf/run_test.go \
  .awf/domains/parts/tooling/current-state.md \
  docs/decisions/0035-brownfield-safe-sync-writes.md docs/decisions/ACTIVE.md \
  docs/domains/tooling.md .awf/awf.lock
git commit -m "feat(awf): back up foreign files on sync (ADR-0035)"
```

Commit message body: summarise that sync backs up foreign files via the shared BackupFile, surfaces
ADR-index ownership takeover, and that init's standalone backup loop is subsumed.

---

## Verification (whole plan)

After both phases:
- `./x gate full` — full tier including e2e; expect green.
- `git log --oneline -2` shows the two feature commits.
- Manual spot-check: in a scratch dir, `awf init --force` over a repo containing a hand-written
  colliding file produces exactly one `.awf-bak`; a part containing `{{ .x }}` renders verbatim.
- Both ADRs show `Implemented` in `docs/decisions/ACTIVE.md`; `docs/domains/rendering.md` and
  `docs/domains/tooling.md` reflect the new behaviour.

## Notes / risks

- **Zero-drift is the ADR-0034 proof.** If `./x check` reports drift after Task 1.6, the sentinel
  rework changed rendered output — stop and investigate before flipping status. Most likely cause: a
  sentinel leaking into output (the new test guards this) or `targetConfigHash` seeing a different
  var set (it must not, since no part contains `.vars.`).
- **`<no value>` check (`render.go:248`)** now sees restored part bodies. A part containing the
  literal text `<no value>` would falsely trip it — accepted as an absurd, pre-existing edge.
- **ADR-0034's second (untagged) invariant** — "template control-flow never spans a section-marker
  boundary" — defers to the plan whether to add a guard test. Decision: no guard test is added. The
  sentinel single-pass is robust to a spanning action regardless (the sentinel passes through the
  parser inertly, so it can never land inside an open control-flow block in a way that changes
  parsing), and the contract carries no `inv:` slug requiring backing. Revisit only if a future
  default introduces control flow across a section boundary.
- **init→sync unification (Task 2.2)** is the plan's reading of ADR-0035's "one mechanism"
  consequence. The plan↔ADR resync step must confirm ADR-0035 covers removing init's standalone
  loop; if it only implies it, amend ADR-0035 (still Proposed at resync time) to state it.
