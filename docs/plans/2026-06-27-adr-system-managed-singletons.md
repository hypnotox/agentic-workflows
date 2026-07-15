# Plan: ADR-system managed singletons (ADR-0021)

Implements [ADR-0021](../decisions/0021-adr-system-managed-singletons.md). Design lives there.

## Goal

Render `docs/decisions/README.md` and `docs/decisions/template.md` as always-on, section-overridable,
`local`-suppressible singletons mirroring the `agents-doc`/AGENTS.md pattern, so the ADR workflow's
`.layout.adrReadme` / `.layout.adrTemplate` references resolve for adopters. awf dogfoods them.

## Architecture summary

Mirror the agents-doc singleton. Generalize the agents-doc special case in `config.Sidecar` /
`config.PartPath` / `project.partRel` to a singleton set `{agents-doc, adr-readme, adr-template}`
(new exported `config.IsSingletonKind`). Add catalog `adrReadme`/`adrTemplate` specs, two embedded
templates, two `RenderAll` `renderTarget` calls (out-path from `layout`), and two
`validateAgainstCatalog` blocks. awf overrides the README's two command-citing sections via `./x`
convention parts; `template.md` ships generic. Phasing keeps each commit compiling under the 100%
gate: Phase 1 adds catalog struct fields + templates + embed (unrendered); Phase 2 wires the engine
live (singletons render, awf adopts); Phase 3 flips the ADR + refreshes domain narratives.

## Tech stack

- Go 1.26. Packages: `internal/catalog`, `internal/config`, `internal/project`, `templates`.
- Pre-commit hook runs `./x check` then `./x gate` (full suite + 100% coverage) on every commit.

## File structure

**Created:**
- `templates/adr-readme/README.md.tmpl`
- `templates/adr-template/template.md.tmpl`
- `.awf/parts/adr-readme/invariants.md` (awf `./x` override)
- `.awf/parts/adr-readme/active-md.md` (awf `./x` override)

**Modified:**
- `internal/catalog/catalog.go` (two struct fields)
- `templates/embed.go` (embed the two dirs)
- `templates/catalog.yaml` (two singleton specs)
- `internal/config/config.go` (`IsSingletonKind` + Sidecar/PartPath generalization)
- `internal/project/project.go` (partRel + validateAgainstCatalog + RenderAll)
- test files: `internal/project/docs_sections_test.go` (parity), `internal/project/project_test.go`
  (render/local), `internal/project/coverage_test.go` (four error-branch cases)
- `docs/decisions/README.md`, `docs/decisions/template.md` (become awf-rendered, Phase 2)
- `docs/decisions/0021-...md` (flip, Phase 3), `docs/domains/{rendering,adr-system}.md` + their parts, `.awf/awf.lock`

**Deleted:** none.

---

## Phase 1: Catalog fields, templates, embed (compiles, unrendered)

### Task 1.1: Add catalog struct fields

In `internal/catalog/catalog.go`, replace:

```
	AgentsDoc SkillSpec            `yaml:"agentsDoc"`
	DomainDoc SkillSpec            `yaml:"domainDoc"`
	Docs      map[string]DocSpec   `yaml:"docs"`
}
```

with:

```
	AgentsDoc   SkillSpec          `yaml:"agentsDoc"`
	DomainDoc   SkillSpec          `yaml:"domainDoc"`
	AdrReadme   SkillSpec          `yaml:"adrReadme"`
	AdrTemplate SkillSpec          `yaml:"adrTemplate"`
	Docs        map[string]DocSpec `yaml:"docs"`
}
```

(run `gofmt -w` after. `AdrTemplate` is the longest field name, so gofmt re-aligns the whole struct
block: the committed diff touches the `Skills`/`Agents`/`Hooks`/`AgentsDoc`/`DomainDoc`/`Docs`
lines too; that is expected, not drift.)

### Task 1.2: Embed the two template dirs

In `templates/embed.go`, replace:

```
//go:embed catalog.yaml skills hooks agents agents-doc docs domains claude
```

with:

```
//go:embed catalog.yaml skills hooks agents agents-doc docs domains claude adr-readme adr-template
```

### Task 1.3: Create `templates/adr-readme/README.md.tmpl`

Create the file with exactly this content:

```
# Architecture Decision Records

<!-- awf:section intro -->
An ADR captures a significant decision made about the design of this project: what was decided,
why, and what the consequences are. Write one when the decision is hard to reverse, affects
multiple components, or would otherwise be rediscovered from scratch months later.
<!-- awf:end -->

<!-- awf:section when -->
## When to write an ADR

- Choosing between two technically viable approaches
- Establishing a constraint that binds future work (an "invariant")
- Superseding an existing decision with a changed one
- Recording why something was explicitly *not* done
<!-- awf:end -->

<!-- awf:section naming -->
## Naming & location

Files live in `{{ .layout.adrDir }}/` and follow this pattern:

```
NNNN-kebab-title.md
```

where `NNNN` is a zero-padded sequence number (next available). Example:
`0003-drift-detection-strategy.md`.
<!-- awf:end -->

<!-- awf:section frontmatter -->
## Frontmatter

Every ADR starts with YAML frontmatter:

```yaml
---
status: Proposed | Accepted | Implemented | Superseded
date: YYYY-MM-DD
supersedes: []        # list of ADR numbers this replaces, e.g. [0001]
superseded_by: ""     # ADR number that replaced this (empty if still active)
tags: [tooling]
related: []           # related ADR numbers
domains: [area]       # coarse domain keys driving the per-domain decision indexes
---
# ADR-NNNN: Title
```

`domains:` lists the coarse domains this decision belongs to; each one's generated
`## Decisions` index under `{{ .layout.domainsDir }}/` is built from this field, so set it on every
ADR (use the project's existing domain names).
<!-- awf:end -->

<!-- awf:section invariants -->
## Invariant tagging

Give each machine-enforceable Invariants bullet an explicit slug
(``- `inv: <slug>`: ...``) and add a matching `// invariant: <slug>` comment to a test that
exercises it. `awf check` and the standalone `awf invariants` fail when an **Implemented** ADR has
a tagged slug with no backing test. Proposed/Accepted ADRs are not yet enforced (tests land with
implementation); Superseded ADRs are skipped. Bullets without a slug are textual contracts, not
machine-checked.
<!-- awf:end -->

<!-- awf:section active-md -->
## ACTIVE.md

`ACTIVE.md` is a generated index: **do not edit it by hand**. It is regenerated by `awf sync` from
the ADR frontmatter, and `awf check` (run by the pre-commit hook) fails if it is stale. After
adding an ADR or changing a `status:` field, run `awf sync` and stage the regenerated `ACTIVE.md`
alongside your change.
<!-- awf:end -->
```

### Task 1.4: Create `templates/adr-template/template.md.tmpl`

Create the file with exactly this content:

```
<!-- awf:section frontmatter -->
---
status: Proposed
date: YYYY-MM-DD
supersedes: []
superseded_by: ""
tags: []
related: []
domains: []
---
# ADR-NNNN: Title
<!-- awf:end -->

<!-- awf:section body -->
## Context

What situation prompted this decision? What constraints, forces, or prior art shaped the
problem space? Include any measurements or observations that are verifiable.

## Decision

The chosen approach. Be precise enough that a reader can implement it correctly without
further consultation.

## Invariants

Checkable constraints that must hold as long as this decision stands: conditions that
should trigger a new ADR if violated. Tag each machine-enforceable bullet with a slug and back it
with a comment tag (`<your marker> invariant: <slug>`, e.g. `// invariant: <slug>` or
`# invariant: <slug>`) in a source matching your `.awf/config.yaml` `invariants.sources`; `awf
check` enforces tagged slugs once the ADR is `Implemented`. Untagged bullets are textual contracts.

- `inv: <slug>`: ...

## Consequences

What becomes easier, what becomes harder, what is explicitly ruled out by this choice.
Include known risks and how they are mitigated.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Option A | ... |
| Option B | ... |
<!-- awf:end -->
```

### Task 1.5: Verify and commit Phase 1

Run:

```
go build ./...
./x check
./x gate
```

Expected: build succeeds (the embed finds the two new dirs); `./x check` clean (no rendered file
changes yet: RenderAll does not reference the new templates); `./x gate` passes at 100% (no new Go
statements; the templates are embedded but unexercised).

Stage and commit:

```
git add internal/catalog/catalog.go templates/embed.go templates/adr-readme/README.md.tmpl templates/adr-template/template.md.tmpl
git commit -m "feat(awf): add ADR-system singleton templates and catalog fields"
```

---

## Phase 2: Wire the engine live (singletons render, awf adopts)

### Task 2.1: `config.IsSingletonKind` + generalize Sidecar/PartPath

In `internal/config/config.go`, replace the `Sidecar` singleton branch:

```
	var rel string
	if kind == "agents-doc" {
		rel = "agents-doc.yaml"
	} else {
		rel = filepath.Join(kind, name+".yaml")
	}
```

with:

```
	var rel string
	if IsSingletonKind(kind) {
		rel = kind + ".yaml"
	} else {
		rel = filepath.Join(kind, name+".yaml")
	}
```

Replace the `PartPath` function **including its preceding doc comment**:

```
// PartPath returns the convention part path for a section of a target.
func (c *Config) PartPath(kind, target, section string) string {
	if kind == "agents-doc" {
		return filepath.Join(c.root, "parts", "agents-doc", section+".md")
	}
	return filepath.Join(c.root, kind, "parts", target, section+".md")
}
```

with (IsSingletonKind first with its own comment, then the re-documented PartPath):

```
// IsSingletonKind reports whether kind is an always-on singleton whose sidecar lives at
// <root>/<kind>.yaml and whose parts live under <root>/parts/<kind>/ (ADR-0021).
func IsSingletonKind(kind string) bool {
	switch kind {
	case "agents-doc", "adr-readme", "adr-template":
		return true
	}
	return false
}

// PartPath returns the convention part path for a section of a target.
func (c *Config) PartPath(kind, target, section string) string {
	if IsSingletonKind(kind) {
		return filepath.Join(c.root, "parts", kind, section+".md")
	}
	return filepath.Join(c.root, kind, "parts", target, section+".md")
}
```

### Task 2.2: `project.partRel`

In `internal/project/project.go`, replace:

```
func partRel(kind, target, section string) string {
	if kind == "agents-doc" {
		return ".awf/parts/agents-doc/" + section + ".md"
	}
	return ".awf/" + kind + "/parts/" + target + "/" + section + ".md"
}
```

with:

```
func partRel(kind, target, section string) string {
	if config.IsSingletonKind(kind) {
		return ".awf/parts/" + kind + "/" + section + ".md"
	}
	return ".awf/" + kind + "/parts/" + target + "/" + section + ".md"
}
```

### Task 2.3: `validateAgainstCatalog` blocks

In `internal/project/project.go`, replace the agents-doc validation block's trailing `return nil`:

```
	if !ad.Local {
		if err := checkSectionsAllowed("agents-doc", "", p.Cat.AgentsDoc.Sections, ad.Sections); err != nil {
			return err
		}
	}
	return nil
}
```

with:

```
	if !ad.Local {
		if err := checkSectionsAllowed("agents-doc", "", p.Cat.AgentsDoc.Sections, ad.Sections); err != nil {
			return err
		}
	}
	for _, sg := range []struct {
		kind     string
		sections []string
	}{
		{"adr-readme", p.Cat.AdrReadme.Sections},
		{"adr-template", p.Cat.AdrTemplate.Sections},
	} {
		sc, err := p.Cfg.Sidecar(sg.kind, "")
		if err != nil {
			return err
		}
		if !sc.Local {
			if err := checkSectionsAllowed(sg.kind, "", sg.sections, sc.Sections); err != nil {
				return err
			}
		}
	}
	return nil
}
```

### Task 2.4: `RenderAll` singleton renders

In `internal/project/project.go`, replace the end of `RenderAll`:

```
			out = append(out, brf)
		}
	}
	return out, nil
}
```

with:

```
			out = append(out, brf)
		}
	}
	// adr-readme + adr-template (always-on singletons unless local; ADR-0021).
	lay := p.layout()
	for _, sg := range []struct {
		kind, tid, out string
		sections       []string
	}{
		{"adr-readme", "adr-readme/README.md.tmpl", lay["adrReadme"].(string), p.Cat.AdrReadme.Sections},
		{"adr-template", "adr-template/template.md.tmpl", lay["adrTemplate"].(string), p.Cat.AdrTemplate.Sections},
	} {
		sc, err := p.Cfg.Sidecar(sg.kind, "")
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		rf, err := p.renderTarget(sg.kind, "", sg.tid, sg.sections, sc, p.data(sc), sg.out)
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, nil
}
```

### Task 2.5: Catalog specs

In `templates/catalog.yaml`, immediately after the `agentsDoc:` block (the `document-map` line) and
before `domainDoc:`, insert:

```yaml
adrReadme:
  sections:
    - intro
    - when
    - naming
    - frontmatter
    - invariants
    - active-md
adrTemplate:
  sections:
    - frontmatter
    - body
```

### Task 2.6: awf's `./x` README override parts

Create `.awf/parts/adr-readme/invariants.md` with exactly:

```
## Invariant tagging

Give each machine-enforceable Invariants bullet an explicit slug
(``- `inv: <slug>`: ...``) and add a matching `// invariant: <slug>` comment to a test that
exercises it. `awf check` (here `./x check`) and the standalone `awf invariants` (`./x invariants`)
fail when an **Implemented** ADR has a tagged slug with no backing test. Proposed/Accepted ADRs
are not yet enforced (tests land with implementation); Superseded ADRs are skipped. Bullets
without a slug are textual contracts, not machine-checked.
```

Create `.awf/parts/adr-readme/active-md.md` with exactly:

```
## ACTIVE.md

`ACTIVE.md` is a generated index: **do not edit it by hand**. It is regenerated by `awf sync`
(here, `./x sync`) from the ADR frontmatter, and `awf check` (`./x check`, run by the pre-commit
hook) fails if it is stale. After adding an ADR or changing a `status:` field, run `./x sync` and
stage the regenerated `ACTIVE.md` alongside your change.
```

### Task 2.7: Section-parity + render tests

In `internal/project/docs_sections_test.go`, append a parity test mirroring the domain-doc one:

```
func TestAdrSingletonSectionParity(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		tid      string
		sections []string
	}{
		{"adr-readme/README.md.tmpl", cat.AdrReadme.Sections},
		{"adr-template/template.md.tmpl", cat.AdrTemplate.Sections},
	} {
		src, err := fs.ReadFile(templates.FS, c.tid)
		if err != nil {
			t.Fatalf("read %s: %v", c.tid, err)
		}
		var markers []string
		for _, s := range render.ParseSections(string(src)) {
			if s.IsSection {
				markers = append(markers, s.Name)
			}
		}
		if strings.Join(markers, ",") != strings.Join(c.sections, ",") {
			t.Errorf("%s markers %v != catalog sections %v", c.tid, markers, c.sections)
		}
	}
}
```

In `internal/project/project_test.go`, add a render + local-suppression test:

```
func TestAdrSingletonsRenderedAndSuppressible(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"docs/decisions/README.md": false, "docs/decisions/template.md": false}
	for _, f := range files {
		if _, ok := want[f.Path]; ok {
			want[f.Path] = true
		}
	}
	for path, seen := range want {
		if !seen {
			t.Errorf("%s not rendered", path)
		}
	}
	// local: true suppresses the README singleton.
	if err := os.WriteFile(filepath.Join(root, ".awf", "adr-readme.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files2, err := p2.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files2 {
		if f.Path == "docs/decisions/README.md" {
			t.Error("README should be suppressed by local: true")
		}
	}
}
```

Ensure the test file imports `os`, `path/filepath`, `io/fs`, `strings`, `internal/catalog`,
`internal/render`, and `templates` as needed (add any missing import).

### Task 2.7b: Error-branch coverage for the singleton paths

The two RenderAll error arms and the two `validateAgainstCatalog` error arms must be covered (the
100% gate; the agents-doc equivalents are all tested). In `internal/project/coverage_test.go`:

Add an `adr-readme` row to the `TestRenderAllSurfacesMalformedSidecars` table; replace:

```
		{"agents-doc", "prefix: example\nskills: []\nagents: []\nhooks: []\n", "agents-doc.yaml"},
	}
```

with:

```
		{"agents-doc", "prefix: example\nskills: []\nagents: []\nhooks: []\n", "agents-doc.yaml"},
		{"adr-readme", "prefix: example\nskills: []\nagents: []\nhooks: []\n", "adr-readme.yaml"},
	}
```

Add an `adr-readme` row to the `TestRenderAllAssembleErrorOnUnreadablePart` table; replace:

```
		{"agents-doc", "prefix: example\nskills: []\nagents: []\nhooks: []\n", ".awf/parts/agents-doc/identity.md"},
	}
```

with:

```
		{"agents-doc", "prefix: example\nskills: []\nagents: []\nhooks: []\n", ".awf/parts/agents-doc/identity.md"},
		{"adr-readme", "prefix: example\nskills: []\nagents: []\nhooks: []\n", ".awf/parts/adr-readme/intro.md"},
	}
```

Then append two standalone `validateAgainstCatalog` tests (mirroring the agents-doc ones at
`coverage_test.go:51,65`):

```
func TestOpenRejectsMalformedAdrReadmeSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
		"adr-readme.yaml": "bogusUnknownField: true\n",
	})
	if _, err := Open(root); err == nil {
		t.Fatal("expected Open to fail on a malformed adr-readme sidecar")
	}
}

func TestOpenRejectsUnknownAdrReadmeSection(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
		"adr-readme.yaml": "sections:\n  not-a-real-section:\n    drop: true\n",
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected Open to reject an undeclared adr-readme section")
	}
	if !strings.Contains(err.Error(), "not-a-real-section") {
		t.Errorf("error should mention the offending section, got: %v", err)
	}
}
```

These cover, respectively: the RenderAll singleton Sidecar-error arm, the RenderAll singleton
renderTarget-error arm, the `validateAgainstCatalog` Sidecar-error arm, and its
`checkSectionsAllowed`-error arm (the loop body runs for `adr-readme`, covering the shared arms for
both singletons).

### Task 2.8: Render, verify, commit Phase 2

Run:

```
./x sync
./x check
./x gate
```

Expected: `./x sync` renders `docs/decisions/README.md` and `docs/decisions/template.md` (awf's
hand-authored files become awf-rendered: content preserved via the template + the two `./x` parts,
now carrying the `GENERATED by awf` banner + `awf:edit` pointers); `./x check` exits 0 (clean: the
render matches the freshly-stamped lock); `./x gate` passes at 100%. If the gate reports an
uncovered branch in the new RenderAll/validate code, add the covering case (e.g. a malformed
`adr-readme.yaml` to exercise the Sidecar-error path); do not weaken the logic.

Confirm the README links now resolve and content is preserved:

```
grep -n "GENERATED by awf" docs/decisions/README.md docs/decisions/template.md
grep -n "./x check" docs/decisions/README.md
```

Expected: both files carry the banner; the README's invariant section shows `./x check` (awf's part
applied).

Stage the wiring, catalog, parts, tests, and the now-rendered files:

```
git add internal/config/config.go internal/project/project.go internal/project/docs_sections_test.go internal/project/project_test.go internal/project/coverage_test.go templates/catalog.yaml .awf/parts/adr-readme/invariants.md .awf/parts/adr-readme/active-md.md docs/decisions/README.md docs/decisions/template.md .awf/awf.lock
git commit -m "feat(awf): render ADR README and template as managed singletons"
```

---

## Phase 3: Refresh domain narratives and mark ADR-0021 Implemented

### Task 3.1: Refresh the `rendering` and `adr-system` current-state narratives

In `.awf/domains/parts/rendering/current-state.md`, append a sentence at the end of the existing
final paragraph noting the new always-on singletons (read the file first; add after its last line):

```
Two always-on neutral singletons render the ADR-system static files (`<docsDir>/decisions/README.md` and `template.md`) via the same `renderTarget` machinery as the agent guide, suppressible per-file with a `local: true` sidecar (ADR-0021).
```

In `.awf/domains/parts/adr-system/current-state.md`, append (read the file first; add after its last line):

```
The ADR guide (`README.md`) and the ADR skeleton (`template.md`) are awf-managed singletons (ADR-0021), so the `.layout.adrReadme` / `.layout.adrTemplate` references resolve in any adopter project, not only awf's own.
```

### Task 3.2: Flip ADR-0021 to Implemented

In `docs/decisions/0021-adr-system-managed-singletons.md`, change frontmatter `status: Accepted` to
`status: Implemented`. Both tagged slugs (`adr-system-singletons-rendered`,
`adr-singleton-section-parity`) are backed by the Phase-2 tests' `// invariant:` comments; add
those comments now if not already present (one on `TestAdrSingletonsRenderedAndSuppressible`, one on
`TestAdrSingletonSectionParity`).

### Task 3.3: Render, verify, commit Phase 3

Run:

```
./x sync
./x check
./x gate
./x invariants
```

Expected: `./x sync` regenerates `ACTIVE.md` (0021 Implemented) + the two domain docs; `./x check`
clean; `./x gate` passes; `./x invariants` reports both new slugs backed.

Stage `.awf/domains/parts/rendering/current-state.md`,
`.awf/domains/parts/adr-system/current-state.md`,
`docs/decisions/0021-adr-system-managed-singletons.md`, `docs/decisions/ACTIVE.md`,
`docs/domains/rendering.md`, `docs/domains/adr-system.md`, and `.awf/awf.lock`, then:

```
git commit -m "feat(awf): mark ADR-0021 Implemented; refresh domain narratives"
```

---

## Verification (whole plan)

```
./x check       # clean
./x gate        # 100% coverage, 0 lint
./x invariants  # adr-system-singletons-rendered + adr-singleton-section-parity backed
ls docs/decisions/README.md docs/decisions/template.md   # both present, awf-managed
```

## Terminal step

The ADR flip lands in Phase 3. Invoke `awf-reviewing-impl` against the Phase 1-3 commit range.

## Notes

- ADR-driven plan: the `Accepted → Implemented` flip is the final-commit action (Task 3.2); no
  `# Implementation complete` header.
- The `// invariant:` comments backing the two slugs go on the Phase-2 tests (placed in Phase 3
  alongside the flip so the gate enforces them only once Implemented).
- awf's hand-authored README content is preserved, not lost: `intro`/`when` ship verbatim, `naming`
  ships verbatim after the `.layout.adrDir` substitution, and the two `./x`-citing sections
  (`invariants`, `active-md`) are reproduced by the convention parts in Task 2.6. The `frontmatter`
  section is intentionally generalized (example domain `[rendering]`→`[area]`, index-path phrasing
  via `.layout.domainsDir`): a content change to awf's own README, not a loss. `template.md` is
  reproduced verbatim (frontmatter + body). All rendered files gain the `GENERATED by awf` banner +
  `awf:edit` pointers (expected, since they become managed).
