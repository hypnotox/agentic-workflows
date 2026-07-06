# Plan: Project-local skills and agents

**Date:** 2026-07-06
**ADR:** [ADR-0068](../decisions/0068-project-local-skills-and-agents.md) — Project-local skills and agents

## Goal

Let an adopter define project-local skills/agents — names outside `catalog.Standard`,
declared by a sidecar and rendered from one awf-owned base template per kind — and scaffold
them with `awf new skill|agent <name> "<description>"`. Design rationale lives in ADR-0068;
this plan is the execution record.

## Architecture summary

- **Base templates.** Two awf-owned embedded templates — `templates/skills/_base/SKILL.md.tmpl`
  and `templates/agents/_base.md.tmpl` — each with frontmatter and a single `content` section,
  fully guarded so they degrade leak-free under empty data. Embedded via an `all:` `//go:embed`
  prefix (the bare walk skips `_`-prefixed names).
- **Base hook on the spec.** `catalog.SkillSpec`/`TargetSpec` gain a `Base bool` field. A
  synthesized local entry sets it; render resolves such an entry's template id to the base
  template instead of the name-derived path.
- **Effective catalog.** `project.Open` builds a per-project clone of `catalog.Standard` and
  inserts a synthesized `SkillSpec`/`TargetSpec` (`Sections: ["content"]`, `Base: true`) for each
  enabled non-Standard name that carries a declaring sidecar. The global is never mutated; an
  undeclared non-Standard name is left absent so existing catalog validation rejects it (typo
  protection); a Standard name is never overwritten (no shadowing). Merge runs before
  `validateAgainstCatalog`.
- **Author surface.** The body is the `content` convention part
  (`.awf/skills/parts/<name>/content.md`), spliced with `{{=awf:key}}` placeholder substitution.
  The base template sources `name`/`description` from synthesized+sidecar `data`, guarded against
  `<no value>`.
- **`local: true` unchanged**; no new sidecar flag. The discriminator is non-Standard-name +
  declaring sidecar.
- **Command.** `awf new skill|agent <name> "<description>"` validates the name, enables it, writes
  the sidecar (description) and a starter `content` part, then syncs.
- **Harness-agnostic** output is inherited from the existing multi-target render loop — no change.
- Local artifacts sit outside `catalog.Standard`, so the eval (ADR-0053), chain (ADR-0054), and
  Standard section/descriptor parity suites do not touch them; the base templates get their own
  publication-safety lock.

## Tech stack

- Go 1.26. Packages touched: `internal/catalog` (two struct fields), `internal/config`
  (`HasSidecar`, `ValidateArtifactName`, `SidecarPath`), `internal/project` (new `local.go`,
  `Open` rewire, two render tid methods), `cmd/awf` (`new.go`, `main.go`).
- Templates: `templates/skills/_base/`, `templates/agents/_base.md.tmpl`, `templates/embed.go`,
  `templates/docs/working-with-awf.md.tmpl`.
- Config: none of awf's own enable arrays change. Runner: `./x` (`gate`, `check`, `sync`).

## File structure

**Created**
- `templates/skills/_base/SKILL.md.tmpl`
- `templates/agents/_base.md.tmpl`
- `internal/project/local.go`
- `internal/project/local_test.go`

**Modified**
- `internal/catalog/catalog.go` (add `Base bool` to `SkillSpec` and `TargetSpec`)
- `templates/embed.go` (`all:skills all:agents`)
- `internal/project/spine_test.go` (two `TestUnsetFallbackRenders` cases + invariant marker)
- `internal/config/config.go` (`HasSidecar`, `SidecarPath`, `ValidateArtifactName`)
- `internal/config/config_test.go` (unit tests for the three new funcs)
- `internal/project/project.go` (rewire `Open` to merge before validate)
- `internal/project/render.go` (`skillTID`/`agentTID`; swap the two render tid closures)
- `cmd/awf/new.go` (dispatch skill/agent; scaffold local artifact)
- `cmd/awf/main.go` (`new` arg handling by kind; help text)
- `cmd/awf/new_test.go` (fix unknown-kind test; add skill/agent scaffolding tests)
- `templates/docs/working-with-awf.md.tmpl` (commands + local-artifacts prose)
- Rendered outputs via `./x sync` (`docs/working-with-awf.md`, its `.cursor` twin, `.awf/awf.lock`,
  `docs/decisions/ACTIVE.md`)
- `docs/decisions/0068-...md` (status flip to Implemented in the final commit)

---

## Phase 1 — Base templates + spec field + publication-safety lock

Self-contained: adds the base templates and the `Base` field, and locks the templates'
leak-free degradation. Nothing renders them in production yet, so the suite stays green.

- [ ] **Add `Base bool` to both catalog specs.** In `internal/catalog/catalog.go`, in
  `SkillSpec`, add after the `Core` field:
  ```go
  	Core          bool           `yaml:"core"`
  	// Base marks a synthesized project-local entry (ADR-0068): render resolves its
  	// template id to the shared base template, not the name-derived catalog path.
  	// Standard skills never set it.
  	Base          bool           `yaml:"base"`
  	Data          map[string]any `yaml:"data"`
  ```
  In `TargetSpec`, add after `Sections`:
  ```go
  	Sections []string       `yaml:"sections"`
  	// Base marks a synthesized project-local agent (ADR-0068); see SkillSpec.Base.
  	Base     bool           `yaml:"base"`
  	Data     map[string]any `yaml:"data"`
  ```

- [ ] **Widen the embed glob** so `_`-prefixed base templates are included. In
  `templates/embed.go`, change the directive line to:
  ```go
  //go:embed all:skills all:agents agents-doc docs domains claude adr-readme adr-template plans-readme bootstrap hooks partials
  ```

- [ ] **Create `templates/skills/_base/SKILL.md.tmpl`** with exactly:
  ```
  ---
  name: {{ .prefix }}-{{ with .data.slug }}{{ . }}{{ else }}local-skill{{ end }}
  description: >
    {{ with .data.description }}{{ . }}{{ else }}A project-local {{ $.prefix }} skill.{{ end }}
  ---

  # {{ .prefix }}-{{ with .data.slug }}{{ . }}{{ else }}local-skill{{ end }}

  <!-- awf:section content -->
  {{ with .data.description }}{{ . }}

  {{ end }}Describe when to use this skill and the procedure it follows. Replace this by editing the `content` convention part.
  <!-- awf:end -->
  ```

- [ ] **Create `templates/agents/_base.md.tmpl`** with exactly (agents are unprefixed):
  ```
  ---
  name: {{ with .data.slug }}{{ . }}{{ else }}local-agent{{ end }}
  description: >
    {{ with .data.description }}{{ . }}{{ else }}A project-local {{ $.prefix }} agent.{{ end }}
  ---

  # {{ with .data.slug }}{{ . }}{{ else }}local-agent{{ end }}

  <!-- awf:section content -->
  {{ with .data.description }}{{ . }}

  {{ end }}Describe this agent's role, its inputs, and how it reports back. Replace this by editing the `content` convention part.
  <!-- awf:end -->
  ```

- [ ] **Lock publication-safety.** In `internal/project/spine_test.go`, inside the
  `TestUnsetFallbackRenders` `cases` slice, add these two entries (place the invariant-marker
  comment on the line directly above the first added case):
  ```go
  		// invariant: local-base-publication-safe
  		{
  			tmpl: "skills/_base/SKILL.md.tmpl",
  			want: []string{
  				"example-local-skill",
  				"A project-local example skill.",
  				"Describe when to use this skill",
  			},
  			ban: []string{"<no value>", "``"},
  		},
  		{
  			tmpl: "agents/_base.md.tmpl",
  			want: []string{
  				"name: local-agent",
  				"A project-local example agent.",
  				"Describe this agent's role",
  			},
  			ban: []string{"<no value>"},
  		},
  ```

- [ ] **Verify.** Run `./x gate`. Expected tail: `coverage: 100.0% (…)`, `0 issues.`,
  `deadcodecheck: no production dead code`. (The new `Base` field is an unreferenced struct field,
  not a function, so `deadcode` does not flag it.)

- [ ] **Commit** (stage exactly these paths):
  ```
  git add internal/catalog/catalog.go templates/embed.go templates/skills/_base/SKILL.md.tmpl templates/agents/_base.md.tmpl internal/project/spine_test.go
  git commit -m "feat(rendering): add base templates for project-local skills and agents"
  ```

---

## Phase 2 — Effective-catalog merge, discovery, validation, tid hook

Wires the base templates into production: a declared non-Standard name renders from the base
template, once per target.

- [ ] **Add config helpers.** In `internal/config/config.go`, add after `ConfigPath`:
  ```go
  // SidecarPath returns the sidecar file path for an artifact of a project root:
  // <root>/.awf/<kind>/<name>.yaml, or <root>/.awf/<kind>.yaml for a singleton kind.
  func SidecarPath(root, kind, name string) string {
  	if IsSingletonKind(kind) {
  		return filepath.Join(RootDir(root), kind+".yaml")
  	}
  	return filepath.Join(RootDir(root), kind, name+".yaml")
  }
  ```
  Add a `HasSidecar` method after `Sidecar`:
  ```go
  // HasSidecar reports whether a declaring sidecar file exists for an artifact —
  // the presence signal that marks a non-catalog name as an intentional local
  // artifact rather than a typo (ADR-0068).
  func (c *Config) HasSidecar(kind, name string) (bool, error) {
  	var rel string
  	if IsSingletonKind(kind) {
  		rel = kind + ".yaml"
  	} else {
  		rel = filepath.Join(kind, name+".yaml")
  	}
  	_, err := os.Stat(filepath.Join(c.root, rel))
  	if err == nil {
  		return true, nil
  	}
  	if errors.Is(err, os.ErrNotExist) {
  		return false, nil
  	}
  	return false, fmt.Errorf("stat sidecar %s: %w", rel, err) // coverage-ignore: Stat fails here only on a permission fault a test cannot trigger
  }
  ```
  Add, next to `ValidateDomainName`:
  ```go
  // ValidateArtifactName reports whether name is usable as a local skill/agent
  // name (ADR-0068): non-empty, free of path separators or "..", and not in awf's
  // reserved "_"-prefixed namespace (which the base templates occupy).
  // invariant: local-name-validated
  func ValidateArtifactName(kind, name string) error {
  	if name == "" {
  		return fmt.Errorf("%s name must not be empty", kind)
  	}
  	if hasPathSep(name) {
  		return fmt.Errorf("%s %q must not contain path separators or \"..\"", kind, name)
  	}
  	if strings.HasPrefix(name, "_") {
  		return fmt.Errorf("%s %q must not start with \"_\" (reserved)", kind, name)
  	}
  	return nil
  }
  ```

- [ ] **Create `internal/project/local.go`** with:
  ```go
  package project

  import (
  	"maps"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  )

  // Base template ids shared by every synthesized project-local artifact (ADR-0068).
  const (
  	baseSkillTID = "skills/_base/SKILL.md.tmpl"
  	baseAgentTID = "agents/_base.md.tmpl"
  )

  // effectiveCatalog returns a per-project clone of catalog.Standard augmented with
  // a synthesized entry for every enabled local (non-Standard) skill/agent — a name
  // outside the standard pool that carries a declaring sidecar. The package global
  // is never mutated: the maps are cloned before any insert, and existing values
  // are only read (ADR-0068).
  // invariant: local-catalog-clone
  func (p *Project) effectiveCatalog() (*catalog.Catalog, error) {
  	cat := &catalog.Catalog{
  		Skills:    maps.Clone(catalog.Standard.Skills),
  		Agents:    maps.Clone(catalog.Standard.Agents),
  		DomainDoc: catalog.Standard.DomainDoc,
  		Docs:      catalog.Standard.Docs,
  		Vars:      catalog.Standard.Vars,
  	}
  	if err := synthesizeLocals(p, cat.Skills, p.Cfg.Skills, "skills", func(n string) catalog.SkillSpec {
  		return catalog.SkillSpec{Base: true, Sections: []string{"content"}, Data: localData(n)}
  	}); err != nil {
  		return nil, err
  	}
  	if err := synthesizeLocals(p, cat.Agents, p.Cfg.Agents, "agents", func(n string) catalog.TargetSpec {
  		return catalog.TargetSpec{Base: true, Sections: []string{"content"}, Data: localData(n)}
  	}); err != nil {
  		return nil, err
  	}
  	return cat, nil
  }

  // synthesizeLocals inserts a base-rendered entry into pool for each enabled name
  // that is absent from the standard pool and carries a non-local declaring sidecar.
  func synthesizeLocals[T any](p *Project, pool map[string]T, enabled []string, kind string, mk func(string) T) error {
  	for _, name := range enabled {
  		if _, ok := pool[name]; ok {
  			// A standard entry is never overwritten by a local synthesis.
  			// invariant: local-no-shadow
  			continue
  		}
  		has, err := p.Cfg.HasSidecar(kind, name)
  		if err != nil { // coverage-ignore: HasSidecar only errors on a permission fault a test cannot trigger
  			return err
  		}
  		if !has {
  			// Undeclared non-standard name: leave it absent so validateAgainstCatalog
  			// rejects it as a typo.
  			// invariant: local-requires-declaration
  			continue
  		}
  		sc, err := p.Cfg.Sidecar(kind, name)
  		if err != nil {
  			return err
  		}
  		if sc.Local {
  			continue // hand-authored opt-out — render and validate already skip it.
  		}
  		if err := config.ValidateArtifactName(kind, name); err != nil {
  			return err
  		}
  		pool[name] = mk(name)
  	}
  	return nil
  }

  // localData is a synthesized local artifact's default render data: its slug (the
  // frontmatter name stem). The description falls through from the sidecar, guarded
  // by the base template.
  func localData(name string) map[string]any {
  	return map[string]any{"slug": name}
  }
  ```
  Then add the `config` import (the file references `config.ValidateArtifactName`): update the
  import block to:
  ```go
  import (
  	"maps"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  	"github.com/hypnotox/agentic-workflows/internal/config"
  )
  ```

- [ ] **Rewire `Open`** in `internal/project/project.go` so the merge runs before validation.
  Replace the body from `cat := catalog.Standard` through the `return p, nil`:
  ```go
  	targets, err := resolveTargets(cfg.Targets)
  	if err != nil {
  		return nil, err
  	}
  	p := &Project{Root: root, Cfg: cfg, Targets: targets}
  	cat, err := p.effectiveCatalog()
  	if err != nil {
  		return nil, err
  	}
  	p.Cat = cat
  	if err := p.validateAgainstCatalog(); err != nil {
  		return nil, err
  	}
  	return p, nil
  ```
  (The `catalog` import stays used by `effectiveCatalog`; confirm `goimports`/`./x fmt` leaves it.)

- [ ] **Add the tid hook** in `internal/project/render.go`. Add two methods near `renderKind`:
  ```go
  // skillTID resolves a skill's template id: the shared base template for a
  // synthesized local entry, else the name-derived catalog path (ADR-0068).
  // invariant: local-renders-from-base
  func (p *Project) skillTID(n string) string {
  	if p.Cat.Skills[n].Base {
  		return baseSkillTID
  	}
  	return mustDescriptor("skills").tid(n)
  }

  // agentTID mirrors skillTID for agents.
  func (p *Project) agentTID(n string) string {
  	if p.Cat.Agents[n].Base {
  		return baseAgentTID
  	}
  	return mustDescriptor("agents").tid(n)
  }
  ```
  In `RenderAll`, change the skills render spec line `tid: mustDescriptor("skills").tid,` to
  `tid: p.skillTID,` and the agents spec line `tid: mustDescriptor("agents").tid,` to
  `tid: p.agentTID,`.

- [ ] **Create `internal/project/local_test.go`** covering every new branch. It must exercise: a
  local skill and a local agent rendering from the base template once per target; the clone leaving
  `catalog.Standard` unmutated; an undeclared non-Standard name failing `Open`; a `_`-prefixed /
  path-separator name failing `Open`; a malformed local sidecar failing `Open`; a Standard-name
  collision staying the Standard spec. Use the existing `scaffoldProject`-style test root helper in
  the `project` package (see `project_test.go` for the local fixture builder) — enable a local name
  in `.awf/config.yaml`, write `.awf/skills/<name>.yaml` (with `data.description`) and
  `.awf/skills/parts/<name>/content.md`, call `Open` + `RenderAll`, and assert:
  - a `RenderedFile` exists with `TemplateID == "skills/_base/SKILL.md.tmpl"` for each target, its
    `Content` contains `name: <prefix>-<name>` and the content-part body, and no `<no value>`;
  - `len(catalog.Standard.Skills)` is unchanged across `Open`;
  - `Open` returns an error containing `is not in the catalog` when the name has no sidecar;
  - `Open` returns an error from `ValidateArtifactName` for a `_x`/`a/b` name;
  - `Open` returns a parse error for a malformed `.awf/skills/<name>.yaml`;
  - enabling a name equal to a Standard skill (e.g. `tdd`) with a `content` part leaves
    `p.Cat.Skills["tdd"].Base == false` and its Sections unchanged.

- [ ] **Verify.** Run `./x gate`. Expected: `coverage: 100.0% (…)`, `0 issues.`,
  `deadcodecheck: no production dead code`. Then `./x check` — expected `awf check: clean` (no
  drift: awf's own tree enables no local artifacts, and the `Base` field is not serialized into any
  ConfigHash).

- [ ] **Commit:**
  ```
  git add internal/config/config.go internal/config/config_test.go internal/project/local.go internal/project/local_test.go internal/project/project.go internal/project/render.go
  git commit -m "feat(rendering): render project-local skills and agents from base templates"
  ```

---

## Phase 3 — `awf new skill|agent` command

- [ ] **Dispatch by kind** in `cmd/awf/new.go`. Replace the whole file with:
  ```go
  package main

  import (
  	"fmt"
  	"io"
  	"os"
  	"path/filepath"
  	"slices"
  	"strings"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"github.com/hypnotox/agentic-workflows/internal/project"
  	"gopkg.in/yaml.v3"
  )

  // runNew scaffolds a new templated artifact: an ADR, or a project-local skill/agent
  // (ADR-0068). ADR takes a single joined title; skill/agent take a name and a
  // separate quoted description.
  // invariant: adr-new-version-gated
  func runNew(root, kind string, args []string, stdout io.Writer) error {
  	switch kind {
  	case "adr":
  		return newADR(root, args, stdout)
  	case "skill", "agent":
  		return newLocalArtifact(root, kind, args, stdout)
  	default:
  		return &usageErr{fmt.Sprintf("unknown kind %q (want: adr, skill, agent)", kind)}
  	}
  }

  func newADR(root string, titleWords []string, stdout io.Writer) error {
  	if err := gate(root); err != nil {
  		return err
  	}
  	p, err := project.Open(root)
  	if err != nil {
  		return err
  	}
  	path, err := p.NewADR(strings.Join(titleWords, " "))
  	if err != nil {
  		return err
  	}
  	fmt.Fprintln(stdout, path)
  	return nil
  }

  // newLocalArtifact scaffolds a project-local skill/agent: validates the name,
  // writes a declaring sidecar carrying the description and a starter content part,
  // enables the name in config, and re-renders (ADR-0068).
  func newLocalArtifact(root, kind string, args []string, stdout io.Writer) error {
  	if len(args) < 2 {
  		return &usageErr{fmt.Sprintf("usage: awf new %s <name> \"<description>\"", kind)}
  	}
  	name := args[0]
  	desc := strings.Join(strings.Fields(strings.Join(args[1:], " ")), " ")
  	if desc == "" {
  		return &usageErr{"description must not be empty"}
  	}
  	if err := config.ValidateArtifactName(kind, name); err != nil {
  		return err
  	}
  	if err := gate(root); err != nil {
  		return err
  	}
  	p, err := project.Open(root)
  	if err != nil {
  		return err
  	}
  	pl, _ := project.PluralKind(kind) // "skills" / "agents"
  	if pool, _ := project.CatalogNames(p.Cat, kind); slices.Contains(pool, name) {
  		return fmt.Errorf("%s %q already exists (catalog or local) — pick another name", kind, name)
  	}
  	// Declaring sidecar: data.description feeds the base template's frontmatter.
  	scBytes, err := yaml.Marshal(map[string]any{"data": map[string]any{"description": desc}})
  	if err != nil { // coverage-ignore: a string map always marshals
  		return err
  	}
  	scPath := config.SidecarPath(root, pl, name)
  	if err := os.MkdirAll(filepath.Dir(scPath), 0o755); err != nil { // coverage-ignore: parent is the just-opened .awf tree; fails only on a permission fault a test cannot trigger
  		return err
  	}
  	if err := os.WriteFile(scPath, scBytes, 0o644); err != nil { // coverage-ignore: post-mkdir write; fails only on a permission fault a test cannot trigger
  		return err
  	}
  	partPath := p.Cfg.PartPath(pl, name, "content")
  	if err := os.MkdirAll(filepath.Dir(partPath), 0o755); err != nil { // coverage-ignore: as above
  		return err
  	}
  	if err := os.WriteFile(partPath, []byte(localPartStub), 0o644); err != nil { // coverage-ignore: as above
  		return err
  	}
  	updated, err := config.SetArrayMember(p.Cfg.Source(), pl, name, true)
  	if err != nil { // coverage-ignore: config.Load already parsed this config, so SetArrayMember cannot fail here
  		return err
  	}
  	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault a test cannot trigger
  		return err
  	}
  	return runSync(root, stdout)
  }

  // localPartStub is the starter body for a new local artifact's content part —
  // plain prose only (no live {{=awf:…}} placeholder, which would hard-error if its
  // value were unset this render).
  const localPartStub = "Replace this with the artifact's body. This file is a convention part: edit it to author the content, and see docs/working-with-awf.md for the placeholder syntax.\n"
  ```

- [ ] **Update dispatch and help** in `cmd/awf/main.go`. The `new` case already forwards
  `args[3:]`; leave it, but the `len(args) < 4` guard is correct for all kinds (kind + ≥1 arg).
  Update the `"new"` help entry to:
  ```go
  	"new": {
  		maxPos: -1, summary: "Scaffold a new artifact — kind ∈ {adr, skill, agent}",
  		help: `Usage: awf new <kind> <args>

  Scaffold a new artifact. <kind> is adr, skill, or agent.

  - awf new adr "Some Decision Title"
  - awf new skill <name> "<description>"   (a project-local skill)
  - awf new agent <name> "<description>"   (a project-local agent)
  `,
  	},
  ```

- [ ] **Fix and extend the command tests** in `cmd/awf/new_test.go`:
  - Change `TestRunNewUnknownKind` to use a genuinely unknown kind:
    ```go
    func TestRunNewUnknownKind(t *testing.T) {
    	root := scaffoldProject(t)
    	if err := runNew(root, "doc", []string{"x"}, os.Stdout); err == nil {
    		t.Fatal("expected error for unknown kind")
    	}
    }
    ```
  - Add tests that scaffold a local skill and a local agent, asserting the sidecar, the content
    part, the enable-array entry, and that a second `./x sync` stays clean. Assert the rendered
    `.claude/skills/<prefix>-<name>/SKILL.md` exists after the call, and that a reserved `_x` name
    and an already-existing catalog name (`tdd`) are rejected. Add a `TestRunNewMissingDescription`
    (skill with only a name → usage error).

- [ ] **Verify.** `./x gate` → `coverage: 100.0% (…)`, `0 issues.`, no dead code.

- [ ] **Commit:**
  ```
  git add cmd/awf/new.go cmd/awf/main.go cmd/awf/new_test.go
  git commit -m "feat(awf): scaffold project-local skills and agents via awf new"
  ```

---

## Phase 4 — Document the feature

- [ ] **Extend the usage doc.** In `templates/docs/working-with-awf.md.tmpl`:
  - In the `commands` section, add after the `awf new adr` bullet:
    ```
    - `awf new skill <name> "<description>"` / `awf new agent <name> "<description>"` — scaffold a project-local skill/agent (rendered from awf's base template plus a `content` part you author).
    ```
  - At the end of the `config-and-overrides` section body (before `<!-- awf:end -->`), add a
    paragraph:
    ```
    **Project-local skills and agents.** Beyond overriding catalog artifacts, you can define your own. `awf new skill <name> "<description>"` enables a name that is not in awf's standard catalog, writes a declaring sidecar, and drops a `content` convention part for you to author. awf renders it from a built-in base template — once per enabled target — so a skill you define once is emitted for every agent harness you target. Edit its body at `.awf/skills/parts/<name>/content.md` (agents: `.awf/agents/parts/<name>/content.md`); the sidecar's `data.description` becomes its frontmatter description.
    ```

- [ ] **Re-render and verify no unexpected drift.** Run `./x sync`, then `git status --short`.
  Expected changed paths: `templates/docs/working-with-awf.md.tmpl`, `docs/working-with-awf.md`,
  `.cursor/…/working-with-awf.md` (if the cursor target renders it), and `.awf/awf.lock`. Run
  `./x gate` → green.

- [ ] **Commit:**
  ```
  git add templates/docs/working-with-awf.md.tmpl docs/working-with-awf.md .awf/awf.lock
  # include the .cursor twin if `git status` shows it changed
  git commit -m "docs(rendering): document awf new skill/agent and local artifacts"
  ```

---

## Phase 5 — Flip ADR-0068 to Implemented

The final commit; every `inv:` slug from ADR-0068 is now backed in source, so `awf check`
enforces them once the status is `Implemented`.

- [ ] **Confirm invariant backing.** Run:
  ```
  grep -rn "invariant: local-" internal/ | sort
  ```
  Expected — all six slugs present: `local-catalog-clone` (local.go), `local-requires-declaration`
  (local.go), `local-no-shadow` (local.go), `local-renders-from-base` (render.go),
  `local-name-validated` (config.go), `local-base-publication-safe` (spine_test.go).

- [ ] **Flip the status.** In `docs/decisions/0068-project-local-skills-and-agents.md`, change the
  frontmatter `status: Proposed` to `status: Implemented`.

- [ ] **Regenerate and gate.** Run `./x sync` (updates `docs/decisions/ACTIVE.md` + `.awf/awf.lock`),
  then `./x gate` and `./x check`. Expected: gate green; `awf check: clean`; the invariants gate
  passes (all six slugs backed).

- [ ] **Commit:**
  ```
  git add docs/decisions/0068-project-local-skills-and-agents.md docs/decisions/ACTIVE.md .awf/awf.lock
  git commit -m "docs(adr): mark 0068 implemented"
  ```

- [ ] **Terminal handoff:** invoke `awf-reviewing-impl` over the phase commit range.

## Notes

- The design lives in ADR-0068; this plan does not restate rationale.
- `ValidateArtifactName(kind, name)` is one shared validator (mirroring `ValidateDomainName`)
  rather than the separate `ValidateSkillName`/`ValidateAgentName` the ADR names in Decision item 8
  — the observable contract (`inv: local-name-validated`) is unchanged. Flagged for the plan↔ADR
  resync.
- No awf-own enable arrays change, so awf does not dogfood a local artifact; the feature is covered
  by `internal/project` and `cmd/awf` tests, not by awf's own rendered tree.
