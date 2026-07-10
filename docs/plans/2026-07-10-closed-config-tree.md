# 2026-07-10 — Closed config tree and configuration-consumption checks

**Goal:** implement [ADR-0086](../decisions/0086-closed-config-tree-and-configuration-consumption-checks.md)
(open-time rejection of inert sidecar fields; unused-var and unused-data drift in `awf check`;
the closed-tree sweep subsuming `orphans()` with `.awf-bak` flagging and `memory/**` exemption;
interactive init prompting only for enabled-set vars). Design rationale lives in the ADR — not
duplicated here.

**Architecture summary:** the checks land smallest-first so each phase gates green alone.
Phase 1 adds pure validations at project open (`internal/project/validate.go`). Phase 2 adds
the consumption extractors to `internal/render/vars.go` and two drift producers to
`Project.Check`; the var-consumption union needs two bookkeeping extensions verified during
design: `RenderedFile` must carry the artifact identity (`kind`/`artifact`) and the
part-placeholder var refs (parts are sentinel-substituted raw bodies, invisible to the
assembled-source scan), and `generateDomainDocs` must stop dropping `assembled`/part refs
(domain docs render outside `RenderAll`). Phase 3 replaces `orphans()` with the allowlist
sweep (`internal/project/sweep.go`); ADR-0011's two invariant slugs (`drift-source-set`,
`section-orphan-flagged`) move onto the new sweep in the same commit, and the pre-existing
orphan detail strings are preserved byte-identical (pinned by the existing tests in
`drift_test.go`, which must pass unchanged). Phase 4 makes `initspec.Resolve` two-pass
(multiselects first, then vars filtered by a `needed` callback backed by a new
`project.NeededVars` that reuses `ScaffoldConfig`'s selection derivation, extracted as
`scaffoldSelection`). Phase 5 is docs + the flip. Changelog bullets ride their phases
(docs-travel invariant); the guide/working-with-awf prose rides Phase 5.

**Tech stack:** Go 1.26, stdlib only. Packages touched: `internal/render` (`vars.go`,
`render.go`), `internal/project` (`validate.go`, `check.go`, `render.go`, `scaffold.go`,
new `sweep.go`), `internal/initspec`, `cmd/awf` (`init.go`), `templates/` (one prose edit),
`changelog/`, `.awf/` (agents-doc data, domain current-state parts).

**File structure:**

- Created: `internal/project/sweep.go`, `internal/project/sweep_test.go`,
  `internal/project/inertfields_test.go`, `internal/project/unused_test.go`,
  `docs/plans/2026-07-10-closed-config-tree.md` (this plan)
- Modified: `internal/project/validate.go`, `internal/project/check.go` (delete `orphans()`,
  add two producers, rewire `Check`), `internal/project/render.go` (`RenderedFile`,
  `planSections`, `renderTarget`, `generateDomainDocs`), `internal/project/scaffold.go`,
  `internal/render/vars.go`, `internal/render/render.go` (`SectionPlan`),
  `internal/render/vars_test.go`, `internal/initspec/initspec.go` (+ its tests' call sites),
  `cmd/awf/init.go`, `templates/docs/working-with-awf.md.tmpl` (sync-and-drift section),
  `changelog/CHANGELOG.md`, `.awf/agents-doc.yaml`,
  `.awf/domains/parts/config/current-state.md`,
  `.awf/domains/parts/rendering/current-state.md`,
  `docs/decisions/0086-*.md` (status flip), `docs/decisions/0011-*.md` / `0022-*.md` /
  `0077-*.md` (`related:` back-pointers), plus rendered files refreshed by `./x sync`
- Deleted: none (`orphans()` is removed in place)

**Phase → ADR Decision map:** P1→D5; P2→D3+D4+D7; P3→D1+D2+D7; P4→D6; P5→doc/flip
obligations. New invariant slugs land with their code: `inert-sidecar-field-rejected` (P1),
`unused-var-drift`+`unused-data-drift` (P2), `closed-config-tree`+`awf-bak-flagged` (P3),
`init-prompts-enabled-vars` (P4).

**Fixture-fallout rule (applies to every phase):** the new checks are error-tier, so any test
fixture carrying decorative config (`data: {k: v}` on an artifact whose template never reads
`.data.k`, non-empty vars nothing references, stray files under a fixture `.awf/`) will start
flagging. Repair the fixture to be hygienic (remove the dead key / empty the var / use a key
the template reads, e.g. tdd's `testSurfaces`) unless the test's very purpose is the flagged
state. Do not weaken assertions to tolerate dirty fixtures. awf's own tree and the evals
fixture were verified clean during design.

---

## Phase 1 — open-time inert-field rejections (ADR-0086 Decision 5)

- [ ] In `internal/project/validate.go`, inside `checkKindAgainstCatalog`, insert between
      the sidecar read (`sc, err := p.Cfg.Sidecar(d.Plural, name)` block) and the
      `if sc.Local {` skip:

      ```go
      		// Inert-field rejection (ADR-0086 Decision 5): paths: is read only from
      		// domain sidecars (ADR-0077), so on any other kind it is configuration
      		// that silently does nothing. Checked before the local: skip — a local
      		// sidecar cannot carry it either.
      		// invariant: inert-sidecar-field-rejected
      		if len(sc.Paths) > 0 {
      			return fmt.Errorf("%s %q: paths: is read only from domain sidecars; remove it from .awf/%s/%s.yaml", d.Singular, name, d.Plural, name)
      		}
      ```

- [ ] In `validateAgainstCatalog`, immediately after the kind loop (the
      `for _, d := range kindDescriptors` block) and before the agents-doc sidecar read,
      add:

      ```go
      	// Domain sidecars are paths-only (ADR-0086 Decision 5): domain rendering
      	// passes an empty sidecar and injects its own data map, so an authored
      	// data:, sections:, or local: entry silently does nothing — and the
      	// domain template's own .data.domain reference would mask a data: block
      	// from the consumption check.
      	for _, name := range p.Cfg.Domains {
      		sc, err := p.Cfg.Sidecar("domains", name)
      		if err != nil {
      			return err
      		}
      		if len(sc.Data) > 0 || len(sc.Sections) > 0 || sc.Local {
      			return fmt.Errorf("domain %q: a domain sidecar is paths-only — nothing reads data:, sections:, or local: on it; remove them from .awf/domains/%s.yaml", name, name)
      		}
      	}
      ```

- [ ] In the same function, after `ad, err := p.Cfg.Sidecar("agents-doc", "")`'s error
      check, add:

      ```go
      	if len(ad.Paths) > 0 {
      		return errors.New("agents-doc: paths: is read only from domain sidecars; remove it from .awf/agents-doc.yaml")
      	}
      ```

      and in the `plainSingletons` loop after `sc, err := p.Cfg.Sidecar(sg.kind, "")`'s
      error check:

      ```go
      		if len(sc.Paths) > 0 {
      			return fmt.Errorf("%s: paths: is read only from domain sidecars; remove it from .awf/%s.yaml", sg.kind, sg.kind)
      		}
      ```

- [ ] Create `internal/project/inertfields_test.go` with one test per rejection branch,
      using the package's existing `scaffoldFiles(t, cfg, files)` helper and asserting
      `Open(root)` fails with the expected message fragment:
      - skill sidecar with `paths:\n  - '**/*.go'\n` → error containing
        `paths: is read only from domain sidecars`;
      - the same sidecar with `local: true` added → still errors (the check precedes the
        local skip);
      - config with `domains:\n  - config\n` and `domains/config.yaml` carrying
        `data:\n  k: v\n` → error containing `a domain sidecar is paths-only`;
      - same with `sections:\n  current-state:\n    drop: true\n` instead → same error;
      - same with `local: true` instead → same error (nothing reads `local:` on a
        domain sidecar — ADR-0086 Decision 5 as amended);
      - `domains/config.yaml` carrying only `paths:\n  - internal/config/**\n` → `Open`
        succeeds;
      - `agents-doc.yaml` with `paths:` → error naming `.awf/agents-doc.yaml`;
      - `workflow.yaml` (a plain singleton) with `paths:` → error naming
        `.awf/workflow.yaml`.
- [ ] Add to `changelog/CHANGELOG.md` under `## [Unreleased]` / `### Breaking changes`:

      ```markdown
      - Inert sidecar fields now refuse at project open (ADR-0086): `paths:` on a
        non-domain sidecar, and anything but `paths:` on a domain sidecar (`data:`,
        `sections:`, `local: true`), fail every gated command with the exact file
        and fix named. These fields were silently ignored before — delete them (or
        move `paths:` to a domain sidecar) and re-run.
      ```

- [ ] Run `go test ./internal/project/` then `./x gate` — green. Commit:
      `feat(awf): reject inert sidecar fields at project open (ADR-0086)`.

## Phase 2 — unused-var and unused-data drift (Decisions 3, 4, 7)

- [ ] In `internal/render/vars.go`, append (mirroring the `ReferencedVars` shape):

      ```go
      var dataRE = regexp.MustCompile(`\.data\.([A-Za-z_][A-Za-z0-9_]*)`)

      // ReferencedDataKeys returns the sorted, de-duplicated list of top-level
      // sidecar data keys referenced via {{ .data.K }} patterns in src (ADR-0086).
      // Nested access (.data.a.b) claims its top-level key.
      func ReferencedDataKeys(src string) []string {
      	matches := dataRE.FindAllStringSubmatch(src, -1)
      	seen := map[string]bool{}
      	for _, m := range matches {
      		seen[m[1]] = true
      	}
      	out := make([]string, 0, len(seen))
      	for name := range seen {
      		out = append(out, name)
      	}
      	sort.Strings(out)
      	return out
      }

      var bareDataRE = regexp.MustCompile(`\.data(?:[^.A-Za-z0-9_]|$)`)

      // ReferencesBareData reports whether src reads .data without a key selector
      // (range/with/index or a whole-map reference). Key-level extraction cannot
      // see through such access, so it conservatively marks every data key
      // consumed (ADR-0086 Decision 4). No shipped template uses the form; this
      // is the future-proofing escape.
      func ReferencesBareData(src string) bool { return bareDataRE.MatchString(src) }

      var bareVarsRE = regexp.MustCompile(`\.vars(?:[^.A-Za-z0-9_]|$)`)

      // ReferencesBareVars mirrors ReferencesBareData for the vars namespace
      // (ADR-0086 Decision 3).
      func ReferencesBareVars(src string) bool { return bareVarsRE.MatchString(src) }

      var varPlaceholderRefRE = regexp.MustCompile(`\{\{=awf:(gateCmd|checkCmd)\}\}`)

      // PlaceholderVarRefs returns the config vars a raw convention-part body
      // consumes through {{=awf:key}} placeholders — gateCmd and checkCmd are the
      // only registry keys that read vars (see project.placeholderRegistry).
      // Scanned on the on-disk bytes: substitution has already replaced the
      // tokens in the assembled output, so this is the one consumption channel
      // the assembled-source scan cannot see (ADR-0086 Decision 3).
      func PlaceholderVarRefs(body string) []string {
      	matches := varPlaceholderRefRE.FindAllStringSubmatch(body, -1)
      	seen := map[string]bool{}
      	for _, m := range matches {
      		seen[m[1]] = true
      	}
      	out := make([]string, 0, len(seen))
      	for name := range seen {
      		out = append(out, name)
      	}
      	sort.Strings(out)
      	return out
      }
      ```

- [ ] In `internal/render/render.go`, add to `SectionPlan` (after `PartMarker`):

      ```go
      	// PartVarRefs lists the config vars the raw part body consumes via
      	// {{=awf:key}} placeholders (ADR-0086). Set by the project layer over
      	// the on-disk bytes; consumed by the unused-var union, which cannot see
      	// part bodies in the assembled source (they are sentinel-substituted raw).
      	PartVarRefs []string
      ```

- [ ] In `internal/project/render.go` `planSections`, after the `sp.PartMarker = …` line,
      add:

      ```go
      			sp.PartVarRefs = render.PlaceholderVarRefs(string(b))
      ```

- [ ] In `internal/project/render.go`, extend `RenderedFile` (after `markerParts`):

      ```go
      	// kind/artifact identify the rendered artifact for the per-artifact
      	// unused-data check; partVarRefs carries the part-placeholder var
      	// consumption the assembled source cannot show (both ADR-0086).
      	kind, artifact string
      	partVarRefs    []string
      ```

      In `renderTarget`, after the `markerParts` collection loop, add:

      ```go
      	var partVarRefs []string
      	for _, name := range slices.Sorted(maps.Keys(plan)) {
      		partVarRefs = append(partVarRefs, plan[name].PartVarRefs...)
      	}
      ```

      and extend the returned literal's last line to:

      ```go
      		markerParts: markerParts, kind: kind, artifact: artifact, partVarRefs: partVarRefs,
      ```

      In `generateDomainDocs`, extend the stripped copy to keep the consumption inputs:

      ```go
      		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content,
      			stubDefaults: rf.stubDefaults, stubParts: rf.stubParts,
      			markerParts: rf.markerParts, assembled: rf.assembled,
      			partVarRefs: rf.partVarRefs})
      ```

- [ ] In `internal/project/check.go`, add the two producers (below `markerNotes`):

      ```go
      // unusedVarDrift reports each non-empty vars: key referenced by no rendered
      // artifact — neither a .vars.X reference in any assembled source (RenderAll
      // output and the generated domain docs, passed concatenated) nor a
      // gateCmd/checkCmd part placeholder (ADR-0086 Decision 3). Empty values are
      // exempt: they mirror the ADR-0045 unset definition, keeping the ADR-0022
      // seed-all-vars scaffold legal. A bare .vars reference conservatively
      // consumes every key.
      // invariant: unused-var-drift
      func (p *Project) unusedVarDrift(files []RenderedFile) []manifest.Drift {
      	used := map[string]bool{}
      	for _, f := range files {
      		if render.ReferencesBareVars(f.assembled) {
      			return nil
      		}
      		for _, r := range render.ReferencedVars(f.assembled) {
      			used[r] = true
      		}
      		for _, r := range f.partVarRefs {
      			used[r] = true
      		}
      	}
      	var drift []manifest.Drift
      	for _, k := range slices.Sorted(maps.Keys(p.Cfg.Vars)) {
      		if v := p.Cfg.Vars[k]; v == nil || v == "" || used[k] {
      			continue
      		}
      		drift = append(drift, manifest.Drift{
      			Path: config.DirName + "/config.yaml", Kind: "unused-var",
      			Detail: fmt.Sprintf("var %q is set but referenced by no rendered artifact; delete it from vars: or enable an artifact that consumes it", k),
      		})
      	}
      	return drift
      }

      // unusedDataDrift reports, per enabled artifact, the sidecar data: keys its
      // assembled sources reference nowhere, unioned across enabled targets
      // (ADR-0086 Decision 4). Domains are excluded — their sidecars are rejected
      // as paths-only at open. A local: true sidecar renders nothing, so every
      // key reports. A key referenced only inside a dropped section counts as
      // unused: the drop makes it configuration that does nothing.
      // invariant: unused-data-drift
      func (p *Project) unusedDataDrift(files []RenderedFile) ([]manifest.Drift, error) {
      	type refset struct {
      		keys map[string]bool
      		bare bool
      	}
      	refs := map[string]*refset{}
      	for _, f := range files {
      		key := f.kind + "\x00" + f.artifact
      		rs := refs[key]
      		if rs == nil {
      			rs = &refset{keys: map[string]bool{}}
      			refs[key] = rs
      		}
      		for _, k := range render.ReferencedDataKeys(f.assembled) {
      			rs.keys[k] = true
      		}
      		rs.bare = rs.bare || render.ReferencesBareData(f.assembled)
      	}
      	var drift []manifest.Drift
      	check := func(kind, name, sidecarRel string) error {
      		sc, err := p.Cfg.Sidecar(kind, name)
      		if err != nil { // coverage-ignore: this sidecar was already read by RenderAll (or validation) in the same Check pass
      			return err
      		}
      		if len(sc.Data) == 0 {
      			return nil
      		}
      		rs := refs[kind+"\x00"+name]
      		if rs != nil && rs.bare {
      			return nil
      		}
      		var unused []string
      		for _, k := range slices.Sorted(maps.Keys(sc.Data)) {
      			if rs == nil || !rs.keys[k] {
      				unused = append(unused, k)
      			}
      		}
      		if len(unused) == 0 {
      			return nil
      		}
      		detail := "data keys referenced by no rendered section: " + strings.Join(unused, ", ") + " — a key referenced only inside a dropped section counts as unused; remove the key or the drop"
      		if sc.Local {
      			detail = "local: true renders nothing, so no data key is consumed; remove the data block: " + strings.Join(unused, ", ")
      		}
      		drift = append(drift, manifest.Drift{Path: sidecarRel, Kind: "unused-data", Detail: detail})
      		return nil
      	}
      	for _, d := range kindDescriptors {
      		if d.Plural == "domains" {
      			continue
      		}
      		for _, name := range d.enable(p.Cfg) {
      			if err := check(d.Plural, name, config.DirName+"/"+d.Plural+"/"+name+".yaml"); err != nil { // coverage-ignore: see check's coverage-ignore
      				return nil, err
      			}
      		}
      	}
      	for _, kind := range catalog.SingletonKinds() {
      		if err := check(kind, "", config.DirName+"/"+kind+".yaml"); err != nil { // coverage-ignore: see check's coverage-ignore
      			return nil, err
      		}
      	}
      	return drift, nil
      }
      ```

- [ ] Wire both into `Check` in `internal/project/check.go`, after the
      `drift = append(drift, p.checkDomainDocs(…)…)` line:

      ```go
      	drift = append(drift, p.unusedVarDrift(slices.Concat(files, dds))...)
      	ud, err := p.unusedDataDrift(files)
      	if err != nil { // coverage-ignore: unusedDataDrift re-reads sidecars RenderAll already read
      		return nil, err
      	}
      	drift = append(drift, ud...)
      ```

- [ ] Add extractor tests to `internal/render/vars_test.go`: `ReferencedDataKeys` on a
      source with `{{ .data.a }} {{ with .data.b }}{{ .data.a.c }}{{ end }}` returns
      `[a, b]`; `ReferencesBareData` true for `{{ range .data }}` / `{{ index .data "k" }}`
      and false for `{{ .data.a }}` / `.database`; `ReferencesBareVars` mirrored;
      `PlaceholderVarRefs` returns `[checkCmd, gateCmd]` for a body with both tokens and
      nothing for `{{=awf:prefix}}`.
- [ ] Create `internal/project/unused_test.go` covering, via `scaffoldFiles` + `Sync` +
      `Check` and filtering drift by Kind:
      - a non-empty var nothing references → one `unused-var` entry at
        `.awf/config.yaml` naming the key; the same var empty → no entry;
      - a var consumed ONLY via a part placeholder — the fixture must isolate the
        `PartVarRefs` channel or the test passes even with the plumbing unwired
        (`gateCmd` is referenced via `.vars.gateCmd` by tdd's own template and the
        always-on agents-doc/workflow templates): enable only
        `refactor-coupling-audit` (its template has no `.vars.gateCmd`), set
        `agents-doc.yaml` and `workflow.yaml` to `local: true`, keep hooks/bootstrap
        off, add a part under `skills/parts/refactor-coupling-audit/` for one of its
        declared sections containing `{{=awf:gateCmd}}`, set `gateCmd` non-empty →
        no `unused-var` entry for `gateCmd`; negative control: the same fixture
        without the part → `unused-var` entry for `gateCmd` (proves the channel);
      - sidecar `data:` key the template never reads (`skills/tdd.yaml` with
        `data: {testSurfaces: […valid…], dead: v}`) → one `unused-data` entry at
        `.awf/skills/tdd.yaml` naming only `dead`;
      - `local: true` sidecar with a data key → `unused-data` entry with the
        `local: true renders nothing` detail;
      - a data key referenced only in a section the sidecar drops → flagged (use
        tdd: drop the section that references `testSurfaces`, keep the key);
      - a singleton sidecar (`agents-doc.yaml`) with a dead data key → entry at
        `.awf/agents-doc.yaml`.
- [ ] Apply the fixture-fallout rule: run `go test ./...`; repair fixtures the new drift
      kinds flag (typical: `data:\n  k: v\n` filler sidecars in `internal/project` and
      `cmd/awf` tests whose Check-based assertions now see `unused-data` — swap `k` for a
      template-read key or drop the data block).
- [ ] Add to `changelog/CHANGELOG.md` under `### Breaking changes`:

      ```markdown
      - `awf check` now fails on authored-but-unconsumed configuration (ADR-0086): a
        non-empty `vars:` key no rendered artifact references (`unused-var`), and a
        sidecar `data:` key the artifact's template never reads (`unused-data`) — the
        typo that publication-safe degradation used to hide. Empty vars stay legal
        (the init scaffold is unchanged), but note that leftover keys from removed
        catalog vars (e.g. ADR-0084's) are now flagged when non-empty, and disabling
        a render unit (`awf remove hooks`) can strand the var only it consumed —
        delete the key in the same change.
      ```

- [ ] Run `./x gate` — green (this also proves awf's own tree clean under the new
      checks). Commit: `feat(awf): flag unused vars and data keys in awf check (ADR-0086)`
      (subject kept under the 72-char commit-gate limit).

## Phase 3 — the closed-tree sweep (Decisions 1, 2, 7)

- [ ] Create `internal/project/sweep.go`:

      ```go
      package project

      import (
      	"errors"
      	"io/fs"
      	"os"
      	"path/filepath"
      	"regexp"
      	"sort"
      	"strings"

      	"github.com/hypnotox/agentic-workflows/internal/catalog"
      	"github.com/hypnotox/agentic-workflows/internal/config"
      	"github.com/hypnotox/agentic-workflows/internal/manifest"
      )

      // claimedModel is the ADR-0086 Decision 1 allowlist: every path under .awf/
      // is either claimed here or drift. files holds claimed file paths
      // (project-relative, slash-separated); dirs holds structural directories
      // legal even when empty; enabled/singletons index the artifact facts the
      // classifier needs to keep the pre-ADR-0086 detail strings — locality is
      // never stored, because an enabled-but-unclaimed parts dir already implies
      // local: true (buildClaimedModel claims every non-local artifact's parts).
      type claimedModel struct {
      	files      map[string]bool
      	dirs       map[string]bool
      	enabled    map[string]map[string]bool // kind → name → enabled
      	singletons map[string]bool            // known singleton kinds
      }

      // claimedDir reports whether dir may exist: a structural dir or an ancestor
      // of a claimed file.
      func (m *claimedModel) claimedDir(dir string) bool {
      	if m.dirs[dir] {
      		return true
      	}
      	pre := dir + "/"
      	for f := range m.files {
      		if strings.HasPrefix(f, pre) {
      			return true
      		}
      	}
      	return false
      }

      // buildClaimedModel computes the claimed-path model from config, catalog,
      // and the RenderAll output (whose .awf/-prefixed paths are exactly the
      // enabled config-tree render units — the model derives from the same code
      // path that writes them, per the ADR's dual-bookkeeping consequence).
      func (p *Project) buildClaimedModel(files []RenderedFile) (*claimedModel, error) {
      	m := &claimedModel{
      		files: map[string]bool{
      			config.DirName + "/config.yaml": true,
      			config.DirName + "/awf.lock":    true,
      		},
      		dirs: map[string]bool{
      			config.DirName:             true,
      			config.DirName + "/parts":  true,
      			config.DirName + "/memory": true,
      		},
      		enabled:    map[string]map[string]bool{},
      		singletons: map[string]bool{},
      	}
      	for _, f := range files {
      		if strings.HasPrefix(f.Path, config.DirName+"/") {
      			m.files[f.Path] = true
      		}
      	}
      	for _, d := range kindDescriptors {
      		kind := d.Plural
      		m.dirs[config.DirName+"/"+kind] = true
      		m.dirs[config.DirName+"/"+kind+"/parts"] = true
      		m.enabled[kind] = map[string]bool{}
      		for _, name := range d.enable(p.Cfg) {
      			m.enabled[kind][name] = true
      			m.files[config.DirName+"/"+kind+"/"+name+".yaml"] = true
      			sc, err := p.Cfg.Sidecar(kind, name)
      			if err != nil { // coverage-ignore: RenderAll read this sidecar earlier in the same Check pass
      				return nil, err
      			}
      			// A local: true artifact renders nothing, so its parts are
      			// dead weight — deliberately unclaimed (ADR-0086 Decision 1).
      			// A local: true domain sidecar cannot reach here: open-time
      			// validation rejects any non-paths: domain field (Decision 5).
      			if sc.Local {
      				continue
      			}
      			m.dirs[config.DirName+"/"+kind+"/parts/"+name] = true
      			for _, sec := range p.declaredSections(kind, name) {
      				m.files[config.DirName+"/"+kind+"/parts/"+name+"/"+sec+".md"] = true
      			}
      		}
      	}
      	for _, kind := range catalog.SingletonKinds() {
      		m.files[config.DirName+"/"+kind+".yaml"] = true
      		m.singletons[kind] = true
      		sc, err := p.Cfg.Sidecar(kind, "")
      		if err != nil { // coverage-ignore: RenderAll read the singleton sidecars earlier in the same Check pass
      			return nil, err
      		}
      		if sc.Local {
      			continue
      		}
      		m.dirs[config.DirName+"/parts/"+kind] = true
      		for _, sec := range p.Cat.Docs[kind].Sections {
      			m.files[config.DirName+"/parts/"+kind+"/"+sec+".md"] = true
      		}
      	}
      	return m, nil
      }

      var awfBakRE = regexp.MustCompile(`\.awf-bak(\.\d+)?$`)

      // classify labels one unclaimed entry: the pre-ADR-0086 orphan shapes keep
      // their ADR-0011 detail strings byte-identical, sync-written backups get
      // the stale-backup detail (inv: awf-bak-flagged), local-managed artifacts'
      // parts their own, and everything else is unclaimed.
      // invariant: awf-bak-flagged
      func (m *claimedModel) classify(rel string, isDir bool) manifest.Drift {
      	const localDetail = "convention parts for a local-managed artifact (local: true renders nothing)"
      	d := manifest.Drift{Path: rel, Kind: "orphaned"}
      	segs := strings.Split(rel, "/") // segs[0] is always ".awf"
      	switch {
      	case !isDir && awfBakRE.MatchString(rel):
      		d.Detail = "stale awf-bak backup — review and delete"
      	// Singleton parts tree: .awf/parts/<kind>[/<section>.md].
      	case len(segs) == 3 && segs[1] == "parts" && isDir && !m.singletons[segs[2]]:
      		d.Detail = "convention parts for an unknown singleton kind"
      	case len(segs) == 3 && segs[1] == "parts" && isDir:
      		d.Detail = localDetail // known singleton, unclaimed dir ⇒ local: true
      	case len(segs) == 4 && segs[1] == "parts" && !isDir && strings.HasSuffix(segs[3], ".md"):
      		d.Detail = "convention part for a section not in the singleton's declared set"
      	// Kind trees: .awf/<kind>/<name>.yaml and .awf/<kind>/parts/<name>[/<sec>.md].
      	case len(segs) == 3 && !isDir && strings.HasSuffix(segs[2], ".yaml") && m.enabled[segs[1]] != nil:
      		d.Detail = "sidecar for an artifact not in the enable list"
      	case len(segs) == 4 && segs[2] == "parts" && isDir && m.enabled[segs[1]] != nil && !m.enabled[segs[1]][segs[3]]:
      		d.Detail = "convention parts for an artifact not in the enable list"
      	case len(segs) == 4 && segs[2] == "parts" && isDir && m.enabled[segs[1]] != nil:
      		d.Detail = localDetail // enabled name, unclaimed dir ⇒ local: true
      	case len(segs) == 5 && segs[2] == "parts" && !isDir && strings.HasSuffix(segs[4], ".md") && m.enabled[segs[1]] != nil && m.enabled[segs[1]][segs[3]]:
      		d.Detail = "convention part for a section not in the target's declared set"
      	default:
      		d.Detail = "unclaimed file or directory — not part of the .awf config tree; delete it or move it out"
      	}
      	return d
      }

      // sweepConfigTree walks .awf/ and reports every entry outside the
      // claimed-path model (ADR-0086 Decision 1), collapsing to the highest
      // fully-unclaimed directory. memory/** is session scratch and wholly exempt
      // (ADR-0069). It subsumes the pre-ADR-0086 orphan sweep: wrong-name
      // sidecars/parts and undeclared sections keep their detail strings
      // (inv: drift-source-set; ADR-0011 section-orphan-flagged).
      // invariant: closed-config-tree
      // invariant: drift-source-set
      // invariant: section-orphan-flagged
      func (p *Project) sweepConfigTree(files []RenderedFile) ([]manifest.Drift, error) {
      	m, err := p.buildClaimedModel(files)
      	if err != nil { // coverage-ignore: see buildClaimedModel's sidecar coverage-ignores
      		return nil, err
      	}
      	var drift []manifest.Drift
      	walkErr := filepath.WalkDir(filepath.Join(p.Root, config.DirName), func(path string, de fs.DirEntry, err error) error {
      		if err != nil { // coverage-ignore: Check requires the lock inside .awf, so the tree exists; a mid-walk error is a permission fault a test cannot trigger
      			if errors.Is(err, os.ErrNotExist) {
      				return filepath.SkipAll
      			}
      			return err
      		}
      		rel, rerr := filepath.Rel(p.Root, path)
      		if rerr != nil { // coverage-ignore: path is always under p.Root
      			return rerr
      		}
      		rel = filepath.ToSlash(rel)
      		if rel == config.DirName {
      			return nil
      		}
      		if de.IsDir() {
      			if rel == config.DirName+"/memory" {
      				return filepath.SkipDir
      			}
      			if m.claimedDir(rel) {
      				return nil
      			}
      			drift = append(drift, m.classify(rel, true))
      			return filepath.SkipDir
      		}
      		if m.files[rel] {
      			return nil
      		}
      		drift = append(drift, m.classify(rel, false))
      		return nil
      	})
      	if walkErr != nil { // coverage-ignore: the callback only returns permission-fault errors (above)
      		return nil, walkErr
      	}
      	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
      	return drift, nil
      }
      ```

- [ ] In `internal/project/check.go`: delete the whole `orphans()` function and its
      doc comment (its `inv: drift-source-set` / ADR-0011 comment and behavior move to
      `sweepConfigTree` above — same commit, so the invariant backing never gaps), and
      change the call site in `Check` from

      ```go
      	// Orphan sidecars/parts (second clause of inv: drift-source-set).
      	od, err := p.orphans()
      ```

      to

      ```go
      	// Closed-tree sweep: orphans, strays, backups (ADR-0086 Decision 1).
      	od, err := p.sweepConfigTree(files)
      ```

      Remove imports `check.go` no longer needs if the compiler flags any (it still uses
      `os`/`filepath` elsewhere; verify with `go build ./...`).
- [ ] Run the two pinned existing tests unchanged — they lock the preserved drift
      *paths* (Detail preservation is pinned by sweep_test.go below):
      `go test ./internal/project/ -run 'TestPerTargetDriftProjection|TestCheckFlagsOrphanedSingletonParts' -v`
      — both PASS without edits.
- [ ] Create `internal/project/sweep_test.go` covering, via `scaffoldFiles` + `Sync` +
      `Check` (filter drift to `Kind == "orphaned"`, assert exact Path+Detail):
      - `.awf/notes.md` → unclaimed detail;
      - `.awf/scratch/a.txt` + `.awf/scratch/b/c.txt` → exactly one entry,
        `.awf/scratch`, unclaimed detail (collapse);
      - `.awf/skills/readme.txt` (non-yaml in a kind dir) → unclaimed detail;
      - `.awf/skills/parts/tdd/notes.txt` (non-md in an enabled artifact's parts dir,
        with `notes` NOT a declared section — use a name like `stray.txt`) → unclaimed
        detail; and `.awf/skills/parts/tdd/bogus.md` → the byte-identical
        `convention part for a section not in the target's declared set`;
      - `.awf/memory/anything.md` and `.awf/memory/deep/file.awf-bak` → no drift at all;
      - `.awf/hooks/pre-commit.sh.awf-bak` (hooks enabled) and
        `.awf/config.yaml.awf-bak.2` → each one entry with
        `stale awf-bak backup — review and delete`;
      - an enabled skill with `local: true` sidecar and a part file →
        `.awf/skills/parts/<name>` flagged with the local-managed detail;
      - a local `workflow.yaml` singleton (`local: true`) with `.awf/parts/workflow/x.md`
        → `.awf/parts/workflow` flagged with the local-managed detail;
      - the ADR-0068 effective-catalog claim, pinned so a future declaredSections
        change to `catalog.Standard` cannot silently flag every synthesized local
        artifact's parts: an enabled skill name absent from `catalog.Standard` with a
        non-local declaring sidecar and `.awf/skills/parts/<name>/content.md` → zero
        `orphaned` entries; plus `.awf/skills/parts/<name>/bogus.md` → the
        declared-set detail;
      - the four remaining preserved ADR-0011 detail strings, each asserted with
        exact Path+Detail (the ADR's byte-identical textual invariant):
        `.awf/skills/debugging.yaml` (not enabled) →
        `sidecar for an artifact not in the enable list`;
        `.awf/skills/parts/orphan-target` (not enabled) →
        `convention parts for an artifact not in the enable list`;
        `.awf/parts/bogus-kind` →
        `convention parts for an unknown singleton kind`;
        `.awf/parts/workflow/bogus.md` →
        `convention part for a section not in the singleton's declared set`;
      - baseline hygiene: a scaffold with hooks+bootstrap enabled and no strays →
        zero `orphaned` entries (proves the render units, `memory/.gitignore`, and
        structural dirs are all claimed).
- [ ] Apply the fixture-fallout rule: run `go test ./...`; repair any fixture that
      leaves stray files under its `.awf/` (typical: helper-written scratch files or
      backups in `cmd/awf` init/e2e tests — `awf init --force` tests now see the
      stale-backup drift *by design*; assert it rather than remove it where the test
      exercises the backup path).
- [ ] Add to `changelog/CHANGELOG.md` under `### Breaking changes`:

      ```markdown
      - The `.awf/` tree is now closed (ADR-0086): `awf check` fails on any file or
        directory it cannot claim — strays like `.awf/notes.md`, files with the wrong
        extension in kind/parts dirs, parts of a `local: true` artifact — with a
        repair hint per entry, collapsing to the topmost unclaimed directory.
        Sync-written `<path>.awf-bak[.N]` collision backups are flagged as stale
        backups to review and delete (a brownfield adopt is therefore red on its
        first check until the backups are cleared — intended to-do surfacing).
        `.awf/memory/` stays exempt session scratch.
      ```

- [ ] Run `./x gate` — green (awf's own tree passes the sweep). Commit:
      `feat(awf): close the .awf config tree in awf check (ADR-0086)`.

## Phase 4 — init prompts only enabled-set vars (Decision 6)

- [ ] In `internal/project/scaffold.go`, extract the selection derivation from
      `ScaffoldConfig` into a helper it then calls — move the block from the
      `var skillNames, docNames []string` declaration through the four `slices.Sort`
      calls (the `Core` loop, `agentNames := …`, the whole `trim != nil` closure block)
      verbatim into:

      ```go
      // scaffoldSelection derives the scaffolded enable arrays from an optional
      // trim: the curated-core default, or the closure-completed trim with agents
      // derived from the selection's requirements (ADR-0081 Decision 9). added
      // lists closure additions beyond the explicit selection.
      func scaffoldSelection(cat *catalog.Catalog, trim *config.CatalogTrim) (skillNames, agentNames, docNames, added []string) {
      	// …moved body, unchanged, ending after the slices.Sort calls…
      	return skillNames, agentNames, docNames, added
      }
      ```

      and in `ScaffoldConfig` replace the moved block with
      `skillNames, agentNames, docNames, added := scaffoldSelection(cat, trim)`.
      Keep the `invariant: scaffold-core-only`, `invariant: catalog-trim-applied`, and
      `invariant: init-set-closed` comment lines with the moved code.
- [ ] Below `ScaffoldConfig`, add:

      ```go
      // NeededVars returns the var names referenced by the templates the
      // scaffolded enabled set will render: the enable arrays scaffoldSelection
      // derives, the always-on singletons (agents-doc + plain), and the
      // default-enabled hook payloads. Init's interactive path prompts only for
      // these (ADR-0086 Decision 6); the scaffold still seeds the full catalog
      // union as empty keys (ADR-0022 unchanged), and an explicit --set/answers
      // value is honored regardless.
      func NeededVars(trim *config.CatalogTrim) (map[string]bool, error) {
      	cat := catalog.Standard
      	skills, agents, docs, _ := scaffoldSelection(cat, trim)
      	varSet := map[string]bool{}
      	for _, c := range []struct {
      		kind  string
      		names []string
      	}{{"skills", skills}, {"agents", agents}, {"docs", docs}} {
      		d, _ := descriptorByPlural(c.kind)
      		for _, n := range c.names {
      			if err := collectVars(templates.FS, d.tid(n), varSet); err != nil { // coverage-ignore: every scaffoldSelection name has a backing embedded template
      				return nil, err
      			}
      		}
      	}
      	if err := collectVars(templates.FS, "agents-doc/AGENTS.md.tmpl", varSet); err != nil { // coverage-ignore: the agents-doc template is always embedded
      		return nil, err
      	}
      	for _, sg := range plainSingletons {
      		if err := collectVars(templates.FS, sg.tid, varSet); err != nil { // coverage-ignore: every plainSingletons entry has a backing embedded template
      			return nil, err
      		}
      	}
      	for _, name := range hookNames {
      		if err := collectVars(templates.FS, "hooks/"+name+".sh.tmpl", varSet); err != nil { // coverage-ignore: every hookNames entry has a backing embedded template
      			return nil, err
      		}
      	}
      	return varSet, nil
      }
      ```

- [ ] In `internal/initspec/initspec.go`, change `Resolve` to two-pass with a `needed`
      callback. New signature:

      ```go
      func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool, needed func(*config.CatalogTrim) (map[string]bool, error)) (map[string]string, *config.CatalogTrim, []string, error) {
      ```

      Restructure the single descriptor loop into: **pass 1** over multiselect
      descriptors only (the existing `d.Kind == "multiselect"` branch, verbatim), then
      build `trim` (move the existing trailing `trim` construction up), then

      ```go
      	// The trim decides which var prompts are worth asking (ADR-0086
      	// Decision 6), so artifact selection now precedes var entry.
      	var neededVars map[string]bool
      	if needed != nil {
      		nv, err := needed(trim)
      		if err != nil {
      			return nil, nil, nil, err
      		}
      		neededVars = nv
      	}
      ```

      then **pass 2** over the remaining descriptors (skip `d.Kind == "multiselect"`),
      the existing string/enum body with one change — the prompt condition becomes:

      ```go
      		val, ok := answers[d.Key]
      		if !ok {
      			// A var no template of the scaffolded enabled set references is
      			// seeded empty, never prompted (ADR-0086 Decision 6): a typed
      			// answer for it could only become unused-var drift. Explicit
      			// answers (the ok branch above) stay honored.
      			// invariant: init-prompts-enabled-vars
      			skip := neededVars != nil && d.Target == "" && !neededVars[d.Key]
      			if interactive && !r.eof && !skip {
      				p, err := prompt(r, out, d)
      				if err != nil {
      					return nil, nil, nil, err
      				}
      				val = p
      			} else {
      				val = ""
      			}
      		}
      ```

      Return `vars, trim, splitNames(scopesRaw), nil` at the end (the trim was already
      built before pass 2).
- [ ] Update every `Resolve` call site (compiler-guided): `cmd/awf/init.go` passes
      `project.NeededVars`; every test call site in `internal/initspec` passes `nil`
      (existing behavior: prompt for everything).
- [ ] Fix the one caller the compiler cannot surface: `TestInitInteractivePromptWiring`
      (`cmd/awf/init_test.go`) scripts stdin as `"make gate\n"` assuming `gateCmd` is
      the first prompt — with multiselects now prompting first, the skills multiselect
      would consume `make gate` and error on the non-numeric token. Change the stdin to
      `"\n\nmake gate\n"` (two empty lines keep both multiselects' core defaults, then
      `gateCmd` reads `make gate`) and update the comment calling `gateCmd` "the first
      descriptor" to note multiselects prompt first.
- [ ] Tests:
      - `internal/initspec`: a new test drives `Resolve` interactively (bytes.Buffer
        in/out) with descs `[{Key: "a", Kind: "string"}, {Key: "b", Kind: "string"}]`
        and `needed` returning `{"a": true}`: the transcript contains a prompt for `a`
        and none for `b`, and resolved `vars["b"] == ""`; a second case passes
        `answers: {"b": "x"}` and asserts `vars["b"] == "x"` (explicit answers
        honored); a third asserts a `needed` error propagates. The
        multiselect-before-vars ordering assertion is a separate case using
        trimDescs-style descriptors (mirror the existing multiselect fixture in the
        initspec tests): one multiselect + one string descriptor, with the string
        listed FIRST — the transcript still shows the multiselect prompt first.
      - `internal/project`: `TestNeededVars` — `NeededVars(nil)` (untrimmed default)
        contains `commitGateCmd` (hook payloads) and `gateCmd` (agents-doc/workflow);
        with `trim = &config.CatalogTrim{Skills: &[]string{"tdd"}, Docs: &[]string{}}`
        the result contains `gateCmd` and `commitGateCmd` but NOT `invariantTestPath`
        (referenced only by the adr-reviewer agent and retrospective skill — both
        outside tdd's closure; verified during design via
        `grep -rl '\.vars\.invariantTestPath' templates/` →
        `templates/agents/adr-reviewer.md.tmpl`,
        `templates/skills/retrospective/SKILL.md.tmpl`).
- [ ] Add to `changelog/CHANGELOG.md` under `### Features`:

      ```markdown
      - Interactive `awf init` now asks for the skill/doc selection first and then
        prompts only for the vars that selection's templates (plus the always-on
        singletons and hook payloads) actually reference (ADR-0086); every other
        catalog var is seeded empty as before. `--set`/answers-file values are
        honored for any var either way.
      ```

- [ ] Run `./x gate` — green. Commit:
      `feat(awf): prompt only enabled-set vars in interactive init (ADR-0086)`.

## Phase 5 — docs, guide, flip

- [ ] In `templates/docs/working-with-awf.md.tmpl`, inside the `sync-and-drift` section,
      append to the existing paragraph (before the closing `<!-- awf:end -->`):

      ```markdown

      `awf check` also enforces config-tree hygiene: every entry under `.awf/` must be claimed —
      an enabled artifact's sidecar or declared-section parts, a rendered unit, or the skeleton —
      and anything else is failing drift with a repair hint, including sync-written `*.awf-bak`
      collision backups (review and delete them) and the parts of a `local: true` artifact
      (`.awf/memory/` is exempt session scratch). Configuration must be consumed, too: a non-empty
      `vars:` key or a sidecar `data:` key that no rendered artifact references is flagged, so a
      typo'd override can no longer degrade silently. A `paths:` key outside a domain sidecar, or
      `data:`/`sections:` on a domain sidecar, refuses at project open with the fix named.
      ```

- [ ] In `.awf/agents-doc.yaml`, append to `data: → invariants:` (matching the existing
      `- ref:`/`text:` entry style):

      ```yaml
              - ref: ADR-0086
                text: '**Closed config tree.** Every `.awf/` entry outside the claimed-path model — enabled artifacts'' sidecars and declared-section parts, the rendered units, the skeleton — is failing `awf check` drift, collapsed to the topmost unclaimed directory (`memory/**` exempt; `*.awf-bak` backups flagged for review; a `local: true` artifact''s parts unclaimed), and authored-but-unconsumed configuration fails too: non-empty `vars:` keys and sidecar `data:` keys no rendered artifact references.'
              - ref: ADR-0086
                text: '**Inert sidecar fields refuse at open.** `paths:` on a non-domain sidecar and `data:`/`sections:` on a domain sidecar fail every gated command at project open; interactive `awf init` prompts only for vars the scaffolded enabled set''s templates reference.'
      ```

- [ ] Append one sentence to `.awf/domains/parts/config/current-state.md` (the config
      domain gained the open-time rejections and the closed-tree contract) and one to
      `.awf/domains/parts/rendering/current-state.md` (the render layer gained the
      consumption extractors and the RenderedFile bookkeeping), each citing ADR-0086 in
      prose consistent with the file's existing style.
- [ ] Add `86` to the `related:` frontmatter arrays of
      `docs/decisions/0011-docs-default-content-and-section-taxonomy.md`,
      `docs/decisions/0022-curated-init-default.md`, and
      `docs/decisions/0077-anchored-path-globs-and-domain-code-staleness.md` (exact 0077
      filename per `ls docs/decisions/0077-*.md`).
- [ ] Run `./x sync` (re-renders AGENTS.md, working-with-awf.md, domain docs), then
      `./x check` — clean. Run `./x gate` — green. Commit everything above:
      `docs(rendering): document the ADR-0086 hygiene checks (ADR-0086)`.
- [ ] Flip `docs/decisions/0086-closed-config-tree-and-configuration-consumption-checks.md`
      frontmatter `status: Proposed` → `status: Implemented`. Run `./x sync` (ACTIVE.md
      regen) and `./x gate` — the invariants check now enforces the six ADR-0086 slugs
      against the markers landed in Phases 1–4; green. Commit:
      `docs(adr): flip 0086 to Implemented`.
- [ ] Freeze this plan: it needs no completion marker — the ADR flip freezes it per the
      plan lifecycle.
