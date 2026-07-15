# Plan: Docs Default Content and Per-Doc Section Taxonomy

Implements **ADR-0011** (`docs/decisions/0011-docs-default-content-and-section-taxonomy.md`,
status `Accepted`). The design, rationale, and invariants live in the ADR; this plan is the
execution record only. Do not duplicate rationale here; link.

## Goal

Replace the eight empty `templates/docs/*.tmpl` placeholders with real hybrid default content
(canonical prose where awf is authoritative; a visible `###` skeleton for project-specific docs),
decomposed into a named per-doc section taxonomy declared in `templates/catalog.yaml`; add an
always-on `awf-setup` ("Working with awf") section to the AGENTS.md template; add section-level
orphan detection to `awf check`; back both tagged invariants with tests; migrate this repo's own
`architecture` doc override onto the new section parts; and flip ADR-0011 to `Implemented`.

## Architecture summary

- **Engine is unchanged.** Docs sections already render identically to skill sections
  (`renderTarget` → `overlaySections` → `render.Assemble`); `catalog.DocSpec.Sections []string`
  already exists. The work is catalog content, template content, one Go change to `orphans()`,
  and tests. (ADR-0011 Context.)
- **Convention parts include their own `##` heading** and replace the entire marker block (verified
  against `.claude/awf/parts/agents-doc/identity.md`). So each template section marker carries its
  heading + default content, and each migrated architecture part carries its heading. The doc `# H1`
  lives outside all markers and is always present.
- **Section taxonomy** (ADR-0011 Decision 2): workflow→`principles,chain,commit-discipline,doc-currency`;
  testing→`gate,tiers,layout`; development→`setup,command-runner,dependencies`;
  architecture→`overview,components,data-flow,dependencies`; debugging→`surfaces,recipes`;
  roadmap→`ideas,deferred`; glossary→`terms`; pitfalls→`entries`.
- **Section-orphan detection** (ADR-0011 Decision 4) is generic over kinds in `orphans()`; only the
  convention-part-*file* gap is new (undeclared sidecar `sections` keys are already a hard render
  error via `checkSectionsAllowed`).

## Tech stack

- Go 1.26; module `github.com/hypnotox/agentic-workflows`.
- Packages touched: `internal/project` (orphans + tests), `templates/` (catalog + doc/agents-doc
  templates). No new dependency.
- Gate: `./x gate` (~15s) runs on every commit via the pre-commit hook, plus `./x check`.

## Execution mode

Execute **inline** (`awf-executing-plans`): the tasks are tightly coupled (catalog ↔ template ↔
parity test; the architecture migration spans config + parts + re-sync) and must each leave the gate
green, so a single ordered session is appropriate. One commit per phase.

## File structure

- **Created:**
  - `internal/project/docs_sections_test.go`: parity test + section-orphan test.
  - `.claude/awf/docs/parts/architecture/{overview,components,data-flow,dependencies}.md`: migrated repo override.
- **Modified:**
  - `internal/project/project.go`: section-level orphan detection + `declaredSections` helper.
  - `templates/catalog.yaml`: per-doc `sections`; `awf-setup` added to `agentsDoc.sections`.
  - `templates/docs/{architecture,workflow,testing,development,debugging,pitfalls,glossary,roadmap}.md.tmpl`: default content.
  - `templates/agents-doc/AGENTS.md.tmpl`: `awf-setup` marker block.
  - `docs/decisions/0011-docs-default-content-and-section-taxonomy.md`: status flip (final phase).
  - Re-synced: `docs/architecture.md`, `AGENTS.md`, `.claude/awf/awf.lock`, `docs/decisions/ACTIVE.md`.
- **Deleted:**
  - `.claude/awf/docs/parts/architecture/body.md`: replaced by the four section parts.

---

## Phase 1: Section-orphan detection + parity/orphan tests

Lands the enforcement first. Current templates (single `body`) satisfy parity, so the parity test
is green from the start; the orphan test derives its valid section from the live catalog, so it
stays green across every later phase.

- [ ] **Task 1.1: Add `declaredSections` helper and section-level orphan detection in `internal/project/project.go`.**

  Replace the parts loop inside `orphans()` (currently):
  ```go
  		for _, t := range targets {
  			if !t.IsDir() {
  				continue
  			}
  			if !enabled[kind][t.Name()] {
  				drift = append(drift, manifest.Drift{
  					Path: filepath.Join(".claude", "awf", kind, "parts", t.Name()),
  					Kind: "orphaned", Detail: "convention parts for a target not in the enable list",
  				})
  			}
  		}
  ```
  with:
  ```go
  		for _, t := range targets {
  			if !t.IsDir() {
  				continue
  			}
  			if !enabled[kind][t.Name()] {
  				drift = append(drift, manifest.Drift{
  					Path: filepath.Join(".claude", "awf", kind, "parts", t.Name()),
  					Kind: "orphaned", Detail: "convention parts for a target not in the enable list",
  				})
  				continue
  			}
  			// Enabled target: flag part files whose section is not catalog-declared.
  			declared := sliceSet(p.declaredSections(kind, t.Name()))
  			sections, err := os.ReadDir(filepath.Join(partsDir, t.Name()))
  			if err != nil {
  				continue
  			}
  			for _, sf := range sections {
  				if sf.IsDir() || !strings.HasSuffix(sf.Name(), ".md") {
  					continue
  				}
  				if section := strings.TrimSuffix(sf.Name(), ".md"); !declared[section] {
  					drift = append(drift, manifest.Drift{
  						Path: filepath.Join(".claude", "awf", kind, "parts", t.Name(), sf.Name()),
  						Kind: "orphaned", Detail: "convention part for a section not in the target's declared set",
  					})
  				}
  			}
  		}
  ```

  Update the `orphans()` doc comment from:
  ```go
  // orphans reports sidecar and convention-part files whose target is not in the
  // matching enable list (the second clause of inv: drift-source-set).
  ```
  to:
  ```go
  // orphans reports sidecar and convention-part files whose target is not in the
  // matching enable list, plus convention-part files of an enabled target whose
  // section is not catalog-declared (inv: drift-source-set; ADR-0011 section-orphan-flagged).
  ```

  Add this helper immediately after the closing brace of `orphans()` (before `func sliceSet`):
  ```go

  // declaredSections returns the catalog-declared section names for a target.
  func (p *Project) declaredSections(kind, name string) []string {
  	switch kind {
  	case "skills":
  		return p.Cat.Skills[name].Sections
  	case "agents":
  		return p.Cat.Agents[name].Sections
  	case "docs":
  		return p.Cat.Docs[name].Sections
  	}
  	return nil
  }
  ```

- [ ] **Task 1.2: Create `internal/project/docs_sections_test.go` with the parity and orphan tests.**

  Exact file content:
  ```go
  package project

  import (
  	"fmt"
  	"io/fs"
  	"sort"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  	"github.com/hypnotox/agentic-workflows/internal/render"
  	"github.com/hypnotox/agentic-workflows/templates"
  )

  // TestDocsSectionParity asserts that for every catalog doc the declared section
  // set equals the template's marker-block set, and that each doc renders from
  // template defaults with no leaked <no value> token.
  // invariant: docs-section-parity
  func TestDocsSectionParity(t *testing.T) {
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("load catalog: %v", err)
  	}
  	for name, spec := range cat.Docs {
  		tid := fmt.Sprintf("docs/%s.md.tmpl", name)
  		src, err := fs.ReadFile(templates.FS, tid)
  		if err != nil {
  			t.Fatalf("read %s: %v", tid, err)
  		}
  		var markers []string
  		for _, s := range render.ParseSections(string(src)) {
  			if s.IsSection {
  				markers = append(markers, s.Name)
  			}
  		}
  		want := append([]string(nil), spec.Sections...)
  		got := append([]string(nil), markers...)
  		sort.Strings(want)
  		sort.Strings(got)
  		if strings.Join(want, ",") != strings.Join(got, ",") {
  			t.Errorf("%s: section mismatch: catalog %v vs template markers %v", name, want, got)
  		}
  		out, err := render.Render(string(src), nil,
  			func(string) (string, error) { return "", nil },
  			map[string]any{"prefix": "awf", "vars": map[string]any{}, "data": map[string]any{}})
  		if err != nil {
  			t.Fatalf("render %s: %v", tid, err)
  		}
  		if strings.Contains(out, "<no value>") {
  			t.Errorf("%s: <no value> leaked into rendered doc", name)
  		}
  	}
  }

  // TestSectionOrphanDetection asserts that a convention part whose section is not
  // in the target's catalog-declared set is reported as drift, while a part at a
  // genuinely declared section is not. The valid section is read from the live
  // catalog so the test stays correct as the taxonomy evolves.
  // invariant: section-orphan-flagged
  func TestSectionOrphanDetection(t *testing.T) {
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("load catalog: %v", err)
  	}
  	valid := cat.Docs["architecture"].Sections[0]
  	const orphan = "definitely-not-a-section"
  	cfg := "prefix: example\n" + sprintfVars("") +
  		"skills: []\nagents: []\nhooks: []\ndocs:\n  - architecture\n"
  	root := scaffoldFiles(t, cfg, map[string]string{
  		"docs/parts/architecture/" + valid + ".md":  "## Valid\n\noverride body\n",
  		"docs/parts/architecture/" + orphan + ".md": "## Bogus\n\nstray\n",
  	})
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if err := p.Sync(); err != nil {
  		t.Fatal(err)
  	}
  	drift, err := p.Check()
  	if err != nil {
  		t.Fatal(err)
  	}
  	var sawOrphan, sawValid bool
  	for _, d := range drift {
  		if d.Kind != "orphaned" {
  			continue
  		}
  		switch d.Path {
  		case ".claude/awf/docs/parts/architecture/" + orphan + ".md":
  			sawOrphan = true
  		case ".claude/awf/docs/parts/architecture/" + valid + ".md":
  			sawValid = true
  		}
  	}
  	if !sawOrphan {
  		t.Errorf("expected orphan drift for undeclared section part %q, got %#v", orphan, drift)
  	}
  	if sawValid {
  		t.Errorf("declared section part %q must not be flagged as orphan, got %#v", valid, drift)
  	}
  }
  ```

- [ ] **Task 1.3: Verify and commit Phase 1.**
  ```
  ./x gate
  ```
  Expected: `0 issues.` and every `internal/...` package line `ok` (including `internal/project`).
  ```
  ./x check
  ```
  Expected: `awf check: clean`.

  Commit (stage explicitly):
  ```
  git add internal/project/project.go internal/project/docs_sections_test.go
  git commit -m "feat(awf): add section-level orphan detection and docs section-parity test"
  ```
  Commit body should note: closes the convention-part-file gap in `orphans()` and backs ADR-0011's
  `section-orphan-flagged` and `docs-section-parity` invariants (current single-`body` docs satisfy
  parity).

---

## Phase 2: Canonical-prose docs (workflow, testing, development)

These docs are not enabled in this repo, so no re-sync is needed; the parity test (Phase 1) gates
the catalog↔template match.

- [ ] **Task 2.1: Rewrite `templates/docs/workflow.md.tmpl`.** Full file content:
  ```
  # Workflow

  <!-- awf:section principles -->
  ## Principles

  You own the project's long-term health, not just the task in front of you: bugs you notice in passing are yours, coverage gaps are yours, and documentation drift is yours to fix in the same commit that caused it. Three rules bind every change: reality and its docs move together, the gate is green before every commit, and each commit carries exactly one concern.
  <!-- awf:end -->

  <!-- awf:section chain -->
  ## The chain

  Non-trivial work follows one canonical chain:

  ```
  brainstorming → planning (if warranted) → ADR (if warranted) → review → implementation → review
  ```

  Brainstorming is the hard prerequisite. **Planning** is warranted by *complexity* (multi-commit or interdependent steps). An **ADR** is warranted by *load-bearing-ness* (a design decision the project must remember). Many tasks need neither; few need both. Reviews are lightweight: the grounding-check inside brainstorming subsumes plan/ADR review, and implementation review is the single terminal gate.
  <!-- awf:end -->

  <!-- awf:section commit-discipline -->
  ## Commit discipline

  Use Conventional Commits, one concern per commit. Stage files explicitly rather than `git add -A`, so each commit is a deliberate, reviewable unit. The gate runs before every commit; a commit that cannot pass the gate is not ready to land.
  <!-- awf:end -->

  <!-- awf:section doc-currency -->
  ## Documentation currency

  Documentation travels with the change that makes it true. When you change behaviour, update the affected docs (this file, the agent guide, ADRs, and any reference tables) in the same commit. A separate "docs later" commit is drift waiting to happen.
  <!-- awf:end -->
  ```

- [ ] **Task 2.2: Rewrite `templates/docs/testing.md.tmpl`.** Full file content:
  ```
  # Testing

  <!-- awf:section gate -->
  ## The gate

  A single gate command runs the project's checks (tests, vet/lint, and any drift verification) and must be green before every commit. Treat a red gate as a blocker, never a warning: fix the cause or revert, do not commit around it.
  <!-- awf:end -->

  <!-- awf:section tiers -->
  ## Tiers

  The gate has tiers. A fast tier runs on every commit and covers the common path cheaply; a fuller tier runs the slower, broader checks before merging or releasing. Reach for the fuller tier when a change is risky or cross-cutting, and always before integrating.
  <!-- awf:end -->

  <!-- awf:section layout -->
  ## Test layout

  _Describe where tests live and how they map to the code: the directory convention, how unit, integration, and regression tests are named and separated, and where a new test for a given change belongs._
  <!-- awf:end -->
  ```

- [ ] **Task 2.3: Rewrite `templates/docs/development.md.tmpl`.** Full file content:
  ```
  # Development

  <!-- awf:section setup -->
  ## Setup

  _List the steps to a working local checkout: the toolchain and version, how to install dependencies, and any services, environment variables, or credentials a contributor needs before the first build._
  <!-- awf:end -->

  <!-- awf:section command-runner -->
  ## Command runner

  This project drives common tasks through a single command runner so humans and agents share one interface: build, test, the gate, and any sync/check steps run the same way for everyone. _Name the entry point and list its subcommands, so a newcomer never has to reconstruct a raw command by hand._
  <!-- awf:end -->

  <!-- awf:section dependencies -->
  ## Dependencies

  _List the key dependencies and why each is here: the runtime and library deps that shape the build, and any developer tools pinned for reproducibility._
  <!-- awf:end -->
  ```

- [ ] **Task 2.4: Update `templates/catalog.yaml` `sections` for these three docs.** Apply three edits:

  Under `workflow:`, replace `    sections: [body]` with:
  ```
      sections: [principles, chain, commit-discipline, doc-currency]
  ```
  Under `testing:`, replace `    sections: [body]` with:
  ```
      sections: [gate, tiers, layout]
  ```
  Under `development:`, replace `    sections: [body]` with:
  ```
      sections: [setup, command-runner, dependencies]
  ```
  (Each `sections: [body]` is disambiguated by the unique `title:`/`desc:` lines directly above it.)

- [ ] **Task 2.5: Verify and commit Phase 2.**
  ```
  ./x gate
  ```
  Expected: `0 issues.` and `internal/project` `ok` (the parity test now validates workflow/testing/development).
  ```
  ./x check
  ```
  Expected: `awf check: clean` (these docs are not enabled here; nothing re-renders).

  Commit:
  ```
  git add templates/docs/workflow.md.tmpl templates/docs/testing.md.tmpl templates/docs/development.md.tmpl templates/catalog.yaml
  git commit -m "feat(awf): ship default content for the workflow, testing, and development docs"
  ```

---

## Phase 3: Skeleton docs (debugging, pitfalls, glossary, roadmap)

Also not enabled in this repo; gated by the parity test.

- [ ] **Task 3.1: Rewrite `templates/docs/debugging.md.tmpl`.** Full file content:
  ```
  # Debugging

  <!-- awf:section surfaces -->
  ## Inspection surfaces

  _The state you can inspect and how: log locations and verbosity flags, debug endpoints or dumps, and the commands that reveal what the system is actually doing._
  <!-- awf:end -->

  <!-- awf:section recipes -->
  ## Recipes

  _Step-by-step recipes for the most common failure modes. One subsection per symptom: how to reproduce it, where to look, and the usual cause._
  <!-- awf:end -->
  ```

- [ ] **Task 3.2: Rewrite `templates/docs/pitfalls.md.tmpl`.** Full file content (single section, body directly under the H1, no redundant `##`):
  ```
  # Pitfalls

  <!-- awf:section entries -->
  Recurring bugs and tricky areas worth a warning before you touch them. _One subsection per pitfall: the symptom, the underlying cause, and how to avoid or fix it._
  <!-- awf:end -->
  ```

- [ ] **Task 3.3: Rewrite `templates/docs/glossary.md.tmpl`.** Full file content:
  ```
  # Glossary

  <!-- awf:section terms -->
  Project jargon and what each term means; start here when a term is unfamiliar. _Fill in the table and keep it sorted._

  | Term | Meaning |
  |---|---|
  | _term_ | _what it means in this project, and who owns it_ |
  <!-- awf:end -->
  ```

- [ ] **Task 3.4: Rewrite `templates/docs/roadmap.md.tmpl`.** Full file content:
  ```
  # Roadmap

  <!-- awf:section ideas -->
  ## Ideas

  _Uncommitted ideas and directions under consideration: things that are not yet planned work but are worth not forgetting._
  <!-- awf:end -->

  <!-- awf:section deferred -->
  ## Deferred

  _Work that was explicitly postponed, each with the reason it was deferred and any trigger that should revive it._
  <!-- awf:end -->
  ```

- [ ] **Task 3.5: Update `templates/catalog.yaml` `sections` for these four docs.** Apply four edits:

  Under `debugging:`, replace `    sections: [body]` with:
  ```
      sections: [surfaces, recipes]
  ```
  Under `pitfalls:`, replace `    sections: [body]` with:
  ```
      sections: [entries]
  ```
  Under `glossary:`, replace `    sections: [body]` with:
  ```
      sections: [terms]
  ```
  Under `roadmap:`, replace `    sections: [body]` with:
  ```
      sections: [ideas, deferred]
  ```

- [ ] **Task 3.6: Verify and commit Phase 3.**
  ```
  ./x gate
  ```
  Expected: `0 issues.` and `internal/project` `ok`.
  ```
  ./x check
  ```
  Expected: `awf check: clean`.

  Commit:
  ```
  git add templates/docs/debugging.md.tmpl templates/docs/pitfalls.md.tmpl templates/docs/glossary.md.tmpl templates/docs/roadmap.md.tmpl templates/catalog.yaml
  git commit -m "feat(awf): ship default skeleton content for debugging, pitfalls, glossary, and roadmap docs"
  ```

---

## Phase 4: Architecture doc + repo self-migration

This repo enables `architecture` and overrides it via `.claude/awf/docs/parts/architecture/body.md`.
The catalog change, the template rewrite, the new parts, the deletion of `body.md`, and the re-sync
must land in **one commit** so `./x check` stays clean (after the catalog change, `body` is no
longer a declared section, so leaving `body.md` would itself be flagged by the new orphan check).

- [ ] **Task 4.1: Rewrite `templates/docs/architecture.md.tmpl`.** Full file content:
  ```
  # Architecture

  <!-- awf:section overview -->
  ## Overview

  _One paragraph: what this system is and its shape at a glance: the problem it solves and how its major pieces fit together._
  <!-- awf:end -->

  <!-- awf:section components -->
  ## Components

  _The top-level packages or modules and what each one owns. One bullet per component: its responsibility and what it deliberately does not do._
  <!-- awf:end -->

  <!-- awf:section data-flow -->
  ## Data flow

  _How a request, build, or command moves through the system end to end: the path from entry point to result, and where the important transformations happen._
  <!-- awf:end -->

  <!-- awf:section dependencies -->
  ## Key dependencies

  _The external dependencies that shape the architecture and the role each plays. Note anything load-bearing enough that replacing it would be a significant change._
  <!-- awf:end -->
  ```

- [ ] **Task 4.2: Update `templates/catalog.yaml` `sections` for architecture.** Under `architecture:`, replace `    sections: [body]` with:
  ```
      sections: [overview, components, data-flow, dependencies]
  ```

- [ ] **Task 4.3: Create the four migrated repo parts** (carrying this repo's real architecture content, redistributed from the old `body.md`; each includes its `##` heading).

  `.claude/awf/docs/parts/architecture/overview.md`:
  ```
  ## Overview

  awf ties a per-project `.claude/awf/` config tree to an embedded template catalog, renders the
  standard's skills, agents, hooks, docs, and agent guide, and drift-checks the rendered output
  against a lock file. awf is both the tool that publishes the standard and its own first adopter,
  so this repo's `.claude/` is rendered by the same engine it ships.

  The config tree (ADR-0009) lives under a single `.claude/awf/` root:

  - **`config.yaml`**: the skeleton: `prefix`, `vars`, `invariants`, `docsDir`, and flat enable
    arrays (`skills`, `agents`, `docs`, `hooks`: a name's presence enables that target).
  - **`<kind>/<target>.yaml`**: optional per-target sidecars holding a target's structured `data`,
    its `sections` overrides (`drop` / `replaceWith`), and its `local` flag.
  - **`<kind>/parts/<target>/<section>.md`**: convention parts: if present, the file replaces that
    section's body, no `replaceWith` pointer needed. Per-section precedence is
    `drop > explicit replaceWith > convention part > template default`.
  - **`agents-doc.yaml`** + **`parts/agents-doc/<section>.md`**: the always-on agent-guide singleton.
  - **`awf.lock`**: the relocated, schema-versioned lock; each entry's `ConfigHash` is a per-target
    projection over exactly that file's inputs, so a sidecar or part edit reflags only that target.
  ```

  `.claude/awf/docs/parts/architecture/components.md`:
  ```
  ## Components

  - **`cmd/awf/`**: CLI entry point; `init`, `sync`, `check`, `list`, `add`, `setup`, `upgrade`
    subcommands. `sync`/`check` gate on the schema generation before opening the project.
  - **`internal/config/`**: loads `.claude/awf/config.yaml` plus keyed sidecars; owns the config schema.
  - **`internal/catalog/`**: reads `templates/catalog.yaml`; declares the available skills, agents,
    hooks, docs, and their sections.
  - **`internal/render/`**: Go `text/template` rendering with `missingkey=zero`; assembles section
    overlays (sidecar overrides + convention parts) then executes the template.
  - **`internal/manifest/`**: reads and writes `.claude/awf/awf.lock` (schema-versioned); drives
    drift detection for `awf check`.
  - **`internal/migrate/`**: ordered schema-migration registry (ADR-0010); the `tree-layout`
    migration and the frozen legacy reader; powers `awf upgrade` and the sync/check version gate.
  - **`internal/project/`**: orchestrates config + catalog + render + manifest into `Sync()` and
    `Check()`; golden tests live here.
  - **`internal/frontmatter/`**: the single parser for `---`-delimited YAML frontmatter; used by
    `internal/adr` and skill/agent validation.
  - **`internal/adr/`**: parses ADRs and regenerates `docs/decisions/ACTIVE.md` from their
    frontmatter; invoked by `awf sync` (`./x sync`).
  - **`templates/`**: embedded skill, agent, hook, doc, and agent-guide templates; the catalog
    lives at `templates/catalog.yaml`.
  ```

  `.claude/awf/docs/parts/architecture/data-flow.md`:
  ```
  ## Data flow

  A `sync` loads the config tree, resolves each enabled target's sections (sidecar overrides and
  convention parts layered over template defaults, precedence
  `drop > explicit replaceWith > convention part > template default`), executes `text/template`
  under `missingkey=zero`, rejects any `<no value>` output, writes the rendered files, and stamps
  each one's per-target `ConfigHash` into `.claude/awf/awf.lock`. A `check` re-renders in memory and
  compares against the lock (reporting drift, orphaned sidecars/parts, and stale `ACTIVE.md`) while
  a stale schema generation hard-fails with a "run `awf upgrade`" gate; `awf upgrade` runs the
  registered migrations up to current and re-syncs.
  ```

  `.claude/awf/docs/parts/architecture/dependencies.md`:
  ```
  ## Key dependencies

  - **`gopkg.in/yaml.v3`**: strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
    unknown keys fail fast rather than rendering silently wrong output.
  - **`text/template`** (standard library): the rendering engine, always executed with
    `missingkey=zero` so an unset optional var collapses to empty instead of leaking a token.
  - **`golangci-lint`**: pinned as a `go tool` dependency and run by the gate (`./x gate`); this
    repo only, not part of the rendered standard.
  ```

- [ ] **Task 4.4: Delete the old override part.**
  ```
  git rm .claude/awf/docs/parts/architecture/body.md
  ```

- [ ] **Task 4.5: Re-sync and verify.**
  ```
  ./x sync
  ```
  Expected: `awf sync: done`. This re-renders `docs/architecture.md` from the new template + the four
  new parts and updates `.claude/awf/awf.lock`.
  ```
  ./x gate
  ```
  Expected: `0 issues.` and all `ok` (incl. `TestDocArchitectureTemplate`, which only asserts the
  `# Architecture` heading, and `TestSyncAutoLinksDocsInAgentsDoc`, which asserts the unchanged
  catalog `desc`).
  ```
  ./x check
  ```
  Expected: `awf check: clean` (no orphan for the removed `body.md`; the four new parts are all
  declared sections).

  Sanity-check the rendered doc carries the migrated content under the new headings:
  ```
  grep -c '^## ' docs/architecture.md
  ```
  Expected: `4`.

- [ ] **Task 4.6: Commit Phase 4.**
  ```
  git add templates/docs/architecture.md.tmpl templates/catalog.yaml \
    .claude/awf/docs/parts/architecture/overview.md \
    .claude/awf/docs/parts/architecture/components.md \
    .claude/awf/docs/parts/architecture/data-flow.md \
    .claude/awf/docs/parts/architecture/dependencies.md \
    .claude/awf/docs/parts/architecture/body.md \
    docs/architecture.md .claude/awf/awf.lock
  git commit -m "feat(awf): decompose architecture doc and migrate repo override to section parts"
  ```
  (`git add` of the removed `body.md` path stages its deletion.)

---

## Phase 5: Always-on `awf-setup` AGENTS.md section

- [ ] **Task 5.1: Add the `awf-setup` marker block to `templates/agents-doc/AGENTS.md.tmpl`.**
  Insert it as the **first** section, between the intro paragraph and the existing
  `<!-- awf:section you-and-this-project -->` line. Insert exactly:
  ```
  <!-- awf:section awf-setup -->
  ## Working with awf

  This project's `.claude/` skills, agents, and git hooks (and this guide) are rendered by [awf](https://github.com/hypnotox/agentic-workflows) from the `.claude/awf/` config tree. Every rendered file is generated: never hand-edit one; change the config and re-render.

  - **Toggle a target**: add or remove its name in the enable arrays (`skills`, `agents`, `docs`, `hooks`) in `.claude/awf/config.yaml`.
  - **Set a variable**: edit `vars` in `.claude/awf/config.yaml`.
  - **Override one section of a target**: drop a convention part at `.claude/awf/<kind>/parts/<target>/<section>.md`; it replaces that section's body and inherits the rest of the template default. For a doc that path is `.claude/awf/docs/parts/<name>/<section>.md`.
  - **After any config or part edit**: run `awf sync` to re-render, then `awf check` to confirm there is no drift, and commit the rendered files alongside the config change.
  <!-- awf:end -->

  ```
  (Keep one blank line after the new `<!-- awf:end -->` and before
  `<!-- awf:section you-and-this-project -->`.)

- [ ] **Task 5.2: Add `awf-setup` to `agentsDoc.sections` in `templates/catalog.yaml`.**
  `agentsDoc.sections` already holds six entries (`you-and-this-project`, `identity`,
  `invariants`, `workflow`, `commands`, `document-map`); insert `awf-setup` as the new first
  list item, leaving the rest in place. Replace the three lines:
  ```
  agentsDoc:
    sections:
      - you-and-this-project
  ```
  with:
  ```
  agentsDoc:
    sections:
      - awf-setup
      - you-and-this-project
  ```
  (The `- you-and-this-project` line is the first under `agentsDoc.sections:`; the remaining five
  entries follow unchanged.)

- [ ] **Task 5.3: Re-sync and verify.**
  ```
  ./x sync
  ```
  Expected: `awf sync: done` (re-renders this repo's `AGENTS.md` with the new first section).
  ```
  ./x gate
  ```
  Expected: `0 issues.` and all `ok` (the AGENTS.md golden test asserts phrases that remain present;
  the static `awf-setup` block introduces no template tokens, so the `]()` empty-link guard is unaffected).
  ```
  ./x check
  ```
  Expected: `awf check: clean`.

  Confirm the section rendered into the repo guide:
  ```
  grep -n '## Working with awf' AGENTS.md
  ```
  Expected: one match.

- [ ] **Task 5.4: Commit Phase 5.**
  ```
  git add templates/agents-doc/AGENTS.md.tmpl templates/catalog.yaml AGENTS.md .claude/awf/awf.lock
  git commit -m "feat(awf): add always-on \"Working with awf\" AGENTS.md section"
  ```

---

## Phase 6: Flip ADR-0011 to Implemented

The implementation (templates, taxonomy, orphan detection) and its backing tests now exist, so the
tagged invariants `docs-section-parity` and `section-orphan-flagged` are satisfied and may be
enforced.

- [ ] **Task 6.1: Flip the ADR status.** In
  `docs/decisions/0011-docs-default-content-and-section-taxonomy.md`, change the frontmatter line
  `status: Accepted` to `status: Implemented`.

- [ ] **Task 6.2: Regenerate ACTIVE.md and verify invariant backing.**
  ```
  ./x sync
  ```
  Expected: `awf sync: done` (moves ADR-0011 from `## Proposed`/`## Accepted` grouping into
  `## Implemented` in `docs/decisions/ACTIVE.md`).
  ```
  ./x check
  ```
  Expected: `awf check: clean`, confirms both tagged slugs are backed (the two `// invariant:`
  comments in `internal/project/docs_sections_test.go`). If `awf check` reports an unbacked slug,
  the backing comment is missing or misspelled; fix it before committing.
  ```
  ./x gate
  ```
  Expected: `0 issues.` and all `ok`.

- [ ] **Task 6.3: Commit Phase 6 (final).**
  ```
  git add docs/decisions/0011-docs-default-content-and-section-taxonomy.md docs/decisions/ACTIVE.md .claude/awf/awf.lock
  git commit -m "docs(adr): mark 0011 docs default content and section taxonomy implemented"
  ```

---

## Terminal step

After Phase 6 commits, invoke **`awf-reviewing-impl`** over the SHA range of this plan's commits
(Phase 1 through Phase 6) for the single terminal implementation review.

## Notes

- **Gate cost / batching:** doc-content phases (2, 3) batch same-shape template+catalog changes that
  share one rationale into one commit each; the architecture migration (4), the AGENTS.md section
  (5), and the status flip (6) are genuinely separate concerns and stay separate. Enforcement (1)
  lands before any content so the parity test guards every later template edit.
- **No new vars:** all doc default content and the `awf-setup` block are static (no `.vars.X` /
  `.data.X`), satisfying ADR-0011 Decision 5 and publication-safety under `missingkey=zero`.
- **Repo command wrappers:** the generic `awf-setup` prose names `awf sync` / `awf check`; this repo
  uses the `./x` wrappers, which its AGENTS.md `## Commands` section already documents.
