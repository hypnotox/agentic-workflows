# 2026-07-02: Reviewing-skill/agent pairing (ADR-0050) + template sweep

**Goal:** implement [ADR-0050](../decisions/0050-reviewing-skill-and-agent-pairing.md)
(review-settled at 13c4447): a `requiresAgent` catalog field on the four reviewing-skill specs,
hard pairing validation at project open, an upfront `awf remove agent` refusal, and an
`awf add skill` auto-pair; then land the user-approved template sweep: chain-prose seams,
generality residue, guarded task-skill references with AGENTS.md task-skill gating, lifecycle-state
alignment, and the resync→ADR return edge. Final phase flips ADR-0050 to Implemented.

**Architecture summary:** no package moves. `internal/catalog` gains one struct field;
`internal/project/validate.go` gains the pairing check inside `checkKindAgainstCatalog` plus an
exported `SkillsRequiringAgent` predicate the CLI shares; `cmd/awf/list_add.go` reshapes
`rewriteConfig` into a multi-edit single-write so a paired skill+agent add lands in one config
rewrite (ADR-0050 Decision 5). Phases 4-7 are template-prose-only: every changed template
re-renders this repo's copies under `.claude/` and `.cursor/` (targets: claude, cursor) via
`./x sync`; rendered files and `.awf/awf.lock` are staged with their templates. Design rationale
lives in ADR-0050; cite, don't restate.

**Tech stack:** Go 1.26; packages touched: `internal/catalog`, `internal/project`, `cmd/awf`,
`templates` (catalog.yaml, skills, agents, agents-doc, docs); plus `.awf/` config tree and
rendered artifacts.

**File structure:**
- Created: none.
- Modified:
  - Go: `internal/catalog/catalog.go`, `internal/catalog/catalog_test.go`,
    `internal/project/validate.go`, `internal/project/project_test.go`,
    `internal/project/spine_test.go`, `internal/project/skillrefs_test.go`,
    `cmd/awf/list_add.go`, `cmd/awf/list_add_test.go`
  - Templates: `templates/catalog.yaml`, `templates/agents-doc/AGENTS.md.tmpl`,
    `templates/agents/code-reviewer.md.tmpl`, `templates/agents/plan-reviewer.md.tmpl`,
    `templates/docs/doc-standard.md.tmpl`, and `templates/skills/<name>/SKILL.md.tmpl` for
    `writing-plans`, `proposing-adr`, `executing-plans`, `refactor-coupling-audit`, `debugging`,
    `brainstorming`, `reviewing-impl`, `bugfix`, `subagent-driven-development`, `adr-lifecycle`,
    `reviewing-plan-resync`, `roadmap-graduation`
  - Config tree: `.awf/agents-doc.yaml`, `.awf/domains/parts/config/current-state.md`,
    `docs/decisions/0050-reviewing-skill-and-agent-pairing.md` (status flip)
  - Re-rendered by `./x sync`: `AGENTS.md`, `docs/doc-standard.md`, `docs/decisions/ACTIVE.md`,
    `docs/domains/config.md`, `docs/domains/rendering.md`, `.awf/awf.lock`,
    `.claude/skills/awf-<name>/SKILL.md` + `.cursor/skills/awf-<name>/SKILL.md` for the skills
    listed per phase, `.claude/agents/{code-reviewer,plan-reviewer}.md` +
    `.cursor/agents/{code-reviewer,plan-reviewer}.md`
    (`roadmap-graduation` is enabled but doc-gate-suppressed in this repo (the `roadmap` doc is
    off), so its template change has no rendered counterpart here)
- Deleted: none.

Conventions for every phase: run `./x gate` immediately before the commit (expected: `ok` lines
for all packages, `coverage: 100.0% of non-ignored statements`, vet and lint silent, exit 0).
Stage explicitly; never `git add -A`.

---

## Phase 1: catalog: `requiresAgent` on reviewing-skill specs (ADR-0050 Decisions 1, 6)

- [ ] **Task 1.1: add the field to `internal/catalog/catalog.go`.** Replace the `SkillSpec`
  doc comment and struct (currently lines 20-30):

  Old:
  ```go
  // SkillSpec declares a skill's render sections plus its optional doc dependency:
  // a non-empty RequiresDoc gates the skill on that doc being enabled. Core marks a
  // skill as part of the workflow-core set awf init scaffolds by default (ADR-0022).
  // Data carries the artifact's default render data; sidecars override it per
  // top-level key (ADR-0045).
  type SkillSpec struct {
  	Sections    []string       `yaml:"sections"`
  	RequiresDoc string         `yaml:"requiresDoc"`
  	Core        bool           `yaml:"core"`
  	Data        map[string]any `yaml:"data"`
  }
  ```

  New:
  ```go
  // SkillSpec declares a skill's render sections plus its optional gating fields.
  // RequiresDoc is *suppression* (ADR-0013): a non-empty value gates the skill on
  // that doc being enabled: with the doc off, the skill silently drops out of
  // the effective render set. RequiresAgent is *hard validation* (ADR-0050): a
  // non-empty value names the reviewer agent the skill dispatches, and enabling
  // the skill without that agent fails every gated command at project open: a
  // silently-dropped reviewing skill would sever the workflow chain, so the
  // pairing must be loud. Core marks a skill as part of the workflow-core set
  // awf init scaffolds by default (ADR-0022). Data carries the artifact's
  // default render data; sidecars override it per top-level key (ADR-0045).
  type SkillSpec struct {
  	Sections      []string       `yaml:"sections"`
  	RequiresDoc   string         `yaml:"requiresDoc"`
  	RequiresAgent string         `yaml:"requiresAgent"`
  	Core          bool           `yaml:"core"`
  	Data          map[string]any `yaml:"data"`
  }
  ```

- [ ] **Task 1.2: set the field on the four reviewing specs in `templates/catalog.yaml`.**
  Four two-line edits, inserting `requiresAgent:` directly under each `core: true`:

  ```yaml
    reviewing-plan:
      core: true
      requiresAgent: plan-reviewer
      sections:
  ```
  ```yaml
    reviewing-plan-resync:
      core: true
      requiresAgent: plan-reviewer
      sections:
  ```
  ```yaml
    reviewing-adr:
      core: true
      requiresAgent: adr-reviewer
      sections:
  ```
  ```yaml
    reviewing-impl:
      core: true
      requiresAgent: code-reviewer
      sections:
  ```

- [ ] **Task 1.3: catalog-parity test.** Append to `internal/catalog/catalog_test.go`:

  ```go
  // Every reviewing skill is a thin dispatcher around one reviewer agent; the
  // catalog must pair them so the ADR-0050 validation can enforce it: the
  // prefix anchor keeps a future reviewing skill from reopening the blind spot.
  // invariant: reviewing-skill-specs-paired
  func TestReviewingSkillSpecsArePaired(t *testing.T) {
  	cat, err := Load(templates.FS)
  	if err != nil {
  		t.Fatalf("Load: %v", err)
  	}
  	for name, spec := range cat.Skills {
  		if !strings.HasPrefix(name, "reviewing-") {
  			if spec.RequiresAgent != "" {
  				t.Errorf("skill %q: requiresAgent %q on a non-reviewing skill (ADR-0050 scopes the field to dispatchers)", name, spec.RequiresAgent)
  			}
  			continue
  		}
  		if spec.RequiresAgent == "" {
  			t.Errorf("reviewing skill %q carries no requiresAgent", name)
  			continue
  		}
  		if _, ok := cat.Agents[spec.RequiresAgent]; !ok {
  			t.Errorf("skill %q requires agent %q, which is not in the catalog agents map", name, spec.RequiresAgent)
  		}
  	}
  }
  ```

- [ ] **Task 1.4: verify.** `go test ./internal/catalog/` → `ok`. Then `./x sync && ./x check` →
  no drift, exit 0, and `git status --short` shows only the three edited files (the field is not
  render data, so no rendered file or lock changes).

- [ ] **Task 1.5: gate + commit.** `./x gate`, then:
  ```
  git add internal/catalog/catalog.go internal/catalog/catalog_test.go templates/catalog.yaml
  git commit -m "feat(awf): declare requiresAgent on reviewing-skill specs"
  ```
  Body: names ADR-0050 Decision 1 and the prefix-anchored parity invariant (Decision 6).

---

## Phase 2: hard pairing validation at project open (ADR-0050 Decision 2)

- [ ] **Task 2.1: failing tests first.** Append to `internal/project/project_test.go`:

  ```go
  // A reviewing skill enabled without its dispatched agent fails project open:
  // the error names both sides and the fix (ADR-0050).
  // invariant: reviewing-skill-agent-pairing
  func TestOpenRejectsPairedSkillWithoutAgent(t *testing.T) {
  	root := scaffold(t, "prefix: example\nskills: [reviewing-impl]\nagents: []\n")
  	_, err := Open(root)
  	if err == nil {
  		t.Fatal("expected pairing error for reviewing-impl without code-reviewer")
  	}
  	want := `skill "reviewing-impl" requires agent "code-reviewer"; enable the agent or disable the skill`
  	if err.Error() != want {
  		t.Errorf("error = %q, want %q", err.Error(), want)
  	}
  }

  func TestOpenAllowsPairedSkillWithAgent(t *testing.T) {
  	root := scaffold(t, "prefix: example\nskills: [reviewing-impl]\nagents: [code-reviewer]\n")
  	if _, err := Open(root); err != nil {
  		t.Fatalf("paired skill with its agent must open cleanly, got: %v", err)
  	}
  }

  func TestOpenAllowsLocalPairedSkillWithoutAgent(t *testing.T) {
  	root := scaffoldFiles(t, "prefix: example\nskills: [reviewing-impl]\nagents: []\n",
  		map[string]string{"skills/reviewing-impl.yaml": "local: true\n"})
  	if _, err := Open(root); err != nil {
  		t.Fatalf("local skill sidecar must skip the pairing check, got: %v", err)
  	}
  }
  ```

  Run `go test ./internal/project/ -run 'TestOpenRejectsPairedSkillWithoutAgent'` → expect
  `FAIL` (`expected pairing error ...`): Open currently succeeds.

- [ ] **Task 2.2: the check.** In `internal/project/validate.go`, inside
  `checkKindAgainstCatalog`, after the catalog-membership check. Old (lines 58-63):

  ```go
  		if sc.Local {
  			continue
  		}
  		if !slices.Contains(pool, name) {
  			return fmt.Errorf("%s %q is not in the catalog", d.Singular, name)
  		}
  ```

  New:

  ```go
  		if sc.Local {
  			continue
  		}
  		if !slices.Contains(pool, name) {
  			return fmt.Errorf("%s %q is not in the catalog", d.Singular, name)
  		}
  		// Pairing validation (ADR-0050): a reviewing skill may never be enabled
  		// without the agent it dispatches. Unlike requiresDoc suppression, this
  		// is a hard error: a silently-thinner chain is the failure mode the
  		// workflow exists to prevent.
  		// invariant: reviewing-skill-agent-pairing
  		if d.Plural == "skills" {
  			if req := p.Cat.Skills[name].RequiresAgent; req != "" && !slices.Contains(p.Cfg.Agents, req) {
  				return fmt.Errorf("skill %q requires agent %q; enable the agent or disable the skill", name, req)
  			}
  		}
  ```

  (`sc` is already non-local here and the name is catalog-confirmed, so the check adds no error
  branches beyond the covered one; the required agent may itself be catalog or local; membership
  in `p.Cfg.Agents` is the whole contract.)

- [ ] **Task 2.3: verify.** `go test ./internal/project/ ./cmd/...` → all `ok` (this repo's
  fixtures that enable reviewing skills (`scaffoldedProject`, curated init) enable all agents,
  so nothing else trips). Then `./x sync && ./x check` → clean (this repo enables all three
  agents).

- [ ] **Task 2.4: gate + commit.** `./x gate`, then:
  ```
  git add internal/project/validate.go internal/project/project_test.go
  git commit -m "feat(awf): fail gated commands on an unpaired reviewing skill"
  ```

---

## Phase 3: CLI guards + AGENTS.md invariant (ADR-0050 Decisions 4, 5, Consequences)

- [ ] **Task 3.1: failing tests first.** Append to `cmd/awf/list_add_test.go`:

  ```go
  // `awf remove agent` refuses upfront (before any config rewrite) while an
  // enabled, non-local skill requires the agent (ADR-0050).
  // invariant: remove-agent-pairing-guard
  func TestRunRemoveAgentPairingGuard(t *testing.T) {
  	root := scaffoldedProject(t) // 10 core skills incl. the reviewing four; all 3 agents
  	before := readConfig(t, root)
  	err := runRemove(root, "agent", "code-reviewer", io.Discard)
  	if err == nil || !strings.Contains(err.Error(), `skill "reviewing-impl" requires agent "code-reviewer"`) {
  		t.Fatalf("expected pairing refusal, got %v", err)
  	}
  	if got := readConfig(t, root); got != before {
  		t.Errorf("config must be untouched on refusal:\n%s", got)
  	}

  	// A local sidecar takes the requiring skill out of the pairing's scope,
  	// mirroring the validator exactly.
  	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills"), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "reviewing-adr.yaml"), []byte("local: true\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := runRemove(root, "agent", "adr-reviewer", io.Discard); err != nil {
  		t.Fatalf("remove agent with only a local requirer: %v", err)
  	}

  	// Disabling the requiring skill unblocks the removal.
  	if err := runRemove(root, "skill", "reviewing-impl", io.Discard); err != nil {
  		t.Fatalf("remove skill reviewing-impl: %v", err)
  	}
  	if err := runRemove(root, "agent", "code-reviewer", io.Discard); err != nil {
  		t.Fatalf("remove agent after disabling its skill: %v", err)
  	}
  }

  // `awf add skill` enables the skill's required agent in the same config
  // rewrite, announced by a note (ADR-0050).
  // invariant: add-skill-pairs-agent
  func TestRunAddSkillPairsAgent(t *testing.T) {
  	root := scaffoldProject(t) // minimalYAML: skills [tdd], agents []
  	var out bytes.Buffer
  	if err := runAdd(root, "skill", "reviewing-impl", &out); err != nil {
  		t.Fatalf("add skill reviewing-impl: %v", err)
  	}
  	if !strings.Contains(out.String(), `note: also enabled agent "code-reviewer" (required by skill "reviewing-impl")`) {
  		t.Errorf("missing pairing note, got %q", out.String())
  	}
  	cfg := readConfig(t, root)
  	if !strings.Contains(cfg, "- reviewing-impl") || !strings.Contains(cfg, "- code-reviewer") {
  		t.Errorf("expected both skill and agent enabled:\n%s", cfg)
  	}
  	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "code-reviewer.md")); err != nil {
  		t.Errorf("code-reviewer not rendered after paired add: %v", err)
  	}

  	// Second paired add enables the shared plan-reviewer once; a skill whose
  	// agent is already enabled adds without a note.
  	out.Reset()
  	if err := runAdd(root, "skill", "reviewing-plan", &out); err != nil {
  		t.Fatalf("add skill reviewing-plan: %v", err)
  	}
  	if !strings.Contains(out.String(), `note: also enabled agent "plan-reviewer"`) {
  		t.Errorf("expected plan-reviewer note, got %q", out.String())
  	}
  	out.Reset()
  	if err := runAdd(root, "skill", "reviewing-plan-resync", &out); err != nil {
  		t.Fatalf("add skill reviewing-plan-resync: %v", err)
  	}
  	if strings.Contains(out.String(), "also enabled agent") {
  		t.Errorf("no note expected when the agent is already enabled, got %q", out.String())
  	}
  }
  ```

  Run `go test ./cmd/awf/ -run 'PairingGuard|PairsAgent'` → expect both `FAIL`.

- [ ] **Task 3.2: shared predicate.** Append to `internal/project/validate.go`:

  ```go
  // SkillsRequiringAgent returns the enabled, non-local skills whose catalog
  // spec requires agent: exactly the set the pairing validation would fail on
  // if the agent left the enable array. `awf remove agent` refuses while it is
  // non-empty (ADR-0050).
  func (p *Project) SkillsRequiringAgent(agent string) []string {
  	var out []string
  	for _, name := range p.Cfg.Skills {
  		sc, err := p.Cfg.Sidecar("skills", name)
  		if err != nil || sc.Local { // err: unreachable; Open pre-validated every enabled sidecar
  			continue
  		}
  		if p.Cat.Skills[name].RequiresAgent == agent {
  			out = append(out, name)
  		}
  	}
  	return out
  }
  ```

- [ ] **Task 3.3: reshape `rewriteConfig` into a multi-edit single write.** In
  `cmd/awf/list_add.go`, replace the whole `rewriteConfig` function (currently lines 208-224):

  ```go
  // enableEdit is one enable-array edit for rewriteConfig: the config key and
  // the member name to add or remove.
  type enableEdit struct{ key, name string }

  // rewriteConfig applies one or more enable-array edits to .awf/config.yaml in
  // a single read-modify-write, so a paired skill+agent add lands in the same
  // config rewrite (ADR-0050).
  func rewriteConfig(root string, add bool, edits ...enableEdit) error {
  	cfgPath := config.ConfigPath(root)
  	b, err := os.ReadFile(cfgPath)
  	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
  		return err
  	}
  	for _, e := range edits {
  		b, err = config.SetArrayMember(b, e.key, e.name, add)
  		if err != nil { // coverage-ignore: callers guard add-present / remove-absent before this, and config.Load already rejected a malformed config, so SetArrayMember cannot error here
  			return err
  		}
  	}
  	if err := os.WriteFile(cfgPath, b, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault that root bypasses
  		return err
  	}
  	return nil
  }
  ```

- [ ] **Task 3.4: `runAdd`: pair the agent.** Replace, in `runAdd`:

  Old:
  ```go
  	if err := rewriteConfig(root, key, name, true); err != nil { // coverage-ignore: rewriteConfig only errors on an unreachable SetArrayMember/write failure (the already-enabled guard and config.Load preclude it)
  		return err
  	}
  ```

  New:
  ```go
  	edits := []enableEdit{{key: key, name: name}}
  	// Pairing (ADR-0050): adding a skill that dispatches a reviewer agent
  	// enables the missing agent in the same config rewrite: the additive fix
  	// is safe to apply silently, announced by a note.
  	// invariant: add-skill-pairs-agent
  	var pairedAgent string
  	if kind == "skill" {
  		if req := p.Cat.Skills[name].RequiresAgent; req != "" && !slices.Contains(p.Cfg.Agents, req) {
  			pairedAgent = req
  			edits = append(edits, enableEdit{key: "agents", name: req})
  		}
  	}
  	if err := rewriteConfig(root, true, edits...); err != nil { // coverage-ignore: rewriteConfig only errors on an unreachable SetArrayMember/write failure (the already-enabled guard and config.Load preclude it)
  		return err
  	}
  	if pairedAgent != "" {
  		fmt.Fprintf(stdout, "note: also enabled agent %q (required by skill %q)\n", pairedAgent, name)
  	}
  ```

- [ ] **Task 3.5: `runRemove`: refuse upfront.** Replace, in `runRemove`:

  Old:
  ```go
  	if !slices.Contains(enabledNames(p.Cfg, kind), name) {
  		return fmt.Errorf("%s %q is not enabled", kind, name)
  	}
  	if err := rewriteConfig(root, key, name, false); err != nil { // coverage-ignore: rewriteConfig only errors on an unreachable SetArrayMember/write failure (the not-enabled guard and config.Load preclude it)
  		return err
  	}
  ```

  New:
  ```go
  	if !slices.Contains(enabledNames(p.Cfg, kind), name) {
  		return fmt.Errorf("%s %q is not enabled", kind, name)
  	}
  	// Pairing guard (ADR-0050): refuse BEFORE the config rewrite; the
  	// rewrite-then-sync order would otherwise strand a half-broken tree that
  	// every gated command rejects. The predicate is the validator's exactly.
  	// invariant: remove-agent-pairing-guard
  	if kind == "agent" {
  		if paired := p.SkillsRequiringAgent(name); len(paired) > 0 {
  			return fmt.Errorf("skill %q requires agent %q; enable the agent or disable the skill", paired[0], name)
  		}
  	}
  	if err := rewriteConfig(root, false, enableEdit{key: key, name: name}); err != nil { // coverage-ignore: rewriteConfig only errors on an unreachable SetArrayMember/write failure (the not-enabled guard and config.Load preclude it)
  		return err
  	}
  ```

- [ ] **Task 3.6: AGENTS.md invariants bullet (ADR-0050 Consequences, ADR-0046 precedent:
  the new failing check documents itself in the same change).** In `.awf/agents-doc.yaml`,
  append after the second ADR-0049 bullet (the `Bootstrap output contract` entry, end of the
  `invariants` list):

  ```yaml
        - ref: ADR-0050
          text: '**Reviewing-skill/agent pairing.** A reviewing skill enabled without its dispatched agent fails every gated command; `awf remove agent` refuses upfront and `awf add skill` auto-enables the pair.'
  ```
  (Match the file's existing 4-space/6-space indentation. The bullet describes all three pairing
  behaviours, so it deliberately lands with the CLI half here rather than with the Phase 2
  validation commit: one commit later than the strict ADR-0046 same-commit precedent, so the
  documented contract is fully true the moment it lands.)

- [ ] **Task 3.7: verify + render.** `go test ./cmd/awf/ ./internal/project/` → `ok`. Then
  `./x sync && ./x check` → exit 0; `git status --short` shows exactly `AGENTS.md` and
  `.awf/awf.lock` as the sync products.

- [ ] **Task 3.8: gate + commit.** `./x gate`, then:
  ```
  git add cmd/awf/list_add.go cmd/awf/list_add_test.go internal/project/validate.go .awf/agents-doc.yaml AGENTS.md .awf/awf.lock
  git commit -m "feat(awf): guard agent removal and pair skill adds (ADR-0050)"
  ```

---

## Phase 4: template sweep: chain-prose seams (B1)

All edits below are template prose; section keys never change. After the edits, `./x sync`
re-renders this repo's copies.

- [ ] **Task 4.1: `templates/skills/writing-plans/SKILL.md.tmpl`: commit-before-review.**
  Three edits:

  (a) `procedure-write-plan`: old:
  ```
  2. **Write the plan file in one go.** Do not commit yet. The plan must be self-contained: every step executable by an agent with no prior conversation context.
  ```
  new:
  ```
  2. **Write the plan file in one go.** The plan must be self-contained: every step executable by an agent with no prior conversation context.
  ```

  (b) fill the empty `plan-commit-step` section: old:
  ```
  <!-- awf:section plan-commit-step -->
  <!-- awf:end -->
  ```
  new:
  ```
  <!-- awf:section plan-commit-step -->
  3. **Commit the plan** as soon as it is written: `docs(plans): add YYYY-MM-DD-<topic>`. The review step expects a committed plan; review fixes land as new commits, never `--amend`.
  <!-- awf:end -->
  ```

  (c) `terminal-step`: old:
  ```
  3. **Terminal step: invoke `{{ .prefix }}-reviewing-plan`** via the project's skill-invocation mechanism, passing the plan path. The reviewer applies its lenses and reports findings; route them per the reviewing skill's procedure. After review findings are resolved, commit the plan: `docs(plans): add YYYY-MM-DD-<topic>`.
  ```
  new:
  ```
  4. **Terminal step: invoke `{{ .prefix }}-reviewing-plan`** via the project's skill-invocation mechanism, passing the plan path. The reviewer applies its lenses and reports findings; route them per the reviewing skill's procedure; fixes land as new commits on top of the committed plan.
  ```
  (This matches `reviewing-plan`'s existing "written and committed" premise at its
  `when-fires`/`apply-fixes-commit` sections: those need no edit.)

- [ ] **Task 4.2: dead workflow.md authority citations, five sites.**

  (a) `templates/skills/proposing-adr/SKILL.md.tmpl` (`autonomous-rule`): old:
  ```
  8. **Autonomous continuation.** After the commit, continue to the next chain step without waiting for further approval, per the project's autonomous post-brainstorm rule.
  ```
  new:
  ```
  8. **Autonomous continuation.** After the commit, continue to the next chain step without waiting for further approval: once the brainstorm is agreed, the chain runs autonomously until a review surfaces a user-decision finding.
  ```

  (b) `templates/skills/executing-plans/SKILL.md.tmpl` (`notes-auto-commit`): old:
  ```
  - Auto-commit when green: tests pass + lint clean → commit without asking (per `{{ .layout.workflowRef }}`).
  ```
  new:
  ```
  - Auto-commit when green: tests pass + lint clean → commit without asking.
  ```

  (c) `templates/skills/refactor-coupling-audit/SKILL.md.tmpl` intro (line 12): old:
  ```
  A task skill for refactor ADRs. Runs (or dispatches) the 6-category coupling audit that `{{ .layout.workflowRef }}` mandates before the ADR scope is finalised. The audit's output is a structured listing that lands in the ADR's Context section so scope reflects the real coupling surface, not the assumed one.
  ```
  new:
  ```
  A task skill for refactor ADRs. Runs (or dispatches) the 6-category coupling audit before the ADR scope is finalised. The audit's output is a structured listing that lands in the ADR's Context section so scope reflects the real coupling surface, not the assumed one.
  ```
  Same file, `audit-shape-selection`: old ending: `...to absorb the grep transcript noise per
  `{{ .layout.workflowRef }}`.` → new ending: `...to absorb the grep transcript noise.`
  Same file, `notes` item 1: old:
  ```
  1. Authoritative source for the audit categories: `{{ .layout.workflowRef }}`. This skill is a procedural pointer, not a contract restatement: when the playbook prose evolves, follow the prose.
  ```
  new:
  ```
  1. This skill is the authoritative source for the audit categories.
  ```

  (d) `templates/skills/debugging/SKILL.md.tmpl` notes: old:
  ```
  - Environment problems (infrastructure down, containers missing, dependencies unavailable) are not bugs to work around: stop and report per `{{ .layout.workflowRef }}`.
  ```
  new:
  ```
  - Environment problems (infrastructure down, containers missing, dependencies unavailable) are not bugs to work around: stop and report to the user.
  ```

- [ ] **Task 4.3: `templates/skills/brainstorming/SKILL.md.tmpl`: terminal pointers on the
  direct-implementation branches** (`terminal-handoff` section). Old:
  ```
     - **Load-bearing + simple** → invoke `{{ .prefix }}-proposing-adr` only; implement directly after the ADR is committed.
  ```
  new:
  ```
     - **Load-bearing + simple** → invoke `{{ .prefix }}-proposing-adr` only; implement directly after the ADR is committed, then invoke `{{ .prefix }}-reviewing-impl`.
  ```
  Old:
  ```
     - **Neither** → implement directly without a plan or ADR.
  ```
  new:
  ```
     - **Neither** → implement directly without a plan or ADR, then invoke `{{ .prefix }}-reviewing-impl`.
  ```
  (Unguarded chain-to-chain reference: the chain is disabled as a unit, matching this
  template's existing refs to `proposing-adr`/`writing-plans`.)

- [ ] **Task 4.4: `templates/skills/reviewing-impl/SKILL.md.tmpl`: layout-driven paths + the
  audit contradiction.** Four edits:

  (a) `when-fires`: old: ``Exception: `docs/decisions/` changes always proceed so the
  `code-reviewer`'s doc-currency lens can confirm any ADR status-flip drift.`` → new:
  ``Exception: `{{ .layout.adrDir }}/` changes always proceed so the `code-reviewer`'s
  doc-currency lens can confirm any ADR status-flip drift.``

  (b) `sha-range-detection` planPath bullet: old:
  ```
     - `planPath` auto-detection: `git log ${baseSha}..${headSha} --name-only --pretty=format: | grep -E '^docs/plans/[0-9]{4}-[0-9]{2}-[0-9]{2}-.*\.md$' | sort -u | tail -1`. Use if non-empty; otherwise `null`.
  ```
  new:
  ```
     - `planPath` auto-detection: `git log ${baseSha}..${headSha} --name-only --pretty=format: | grep -E '^{{ .layout.plansDir }}/[0-9]{4}-[0-9]{2}-[0-9]{2}-.*\.md$' | sort -u | tail -1`. Use if non-empty; otherwise `null`.
  ```

  (c) `docs-only-check`: old: ``Exception: `docs/decisions/` changes always proceed. If every
  changed file is docs-only (outside `docs/decisions/`), surface a `Skipped (docs-only)` note
  and return.`` → new: ``Exception: `{{ .layout.adrDir }}/` changes always proceed. If every
  changed file is docs-only (outside `{{ .layout.adrDir }}/`), surface a `Skipped (docs-only)`
  note and return.``

  (d) `run-audit`: old:
  ```
  6. **Run the process-conformance audit.** After the code-review findings are routed, run
     `awf audit` (or this project's runner alias for it) over the branch. Treat `Error` findings as
     blocking and `Warning` findings as advisory; surface both in the digest. The audit is advisory
     and never gates; it does not replace the gate or the drift check.
  ```
  new:
  ```
  6. **Run the process-conformance audit.** After the code-review findings are routed, run
     `awf audit` (or this project's runner alias for it) over the branch. `Error` findings block
     this review from concluding: resolve them or escalate them as user-decision items before
     closing; `Warning` findings are advisory. Surface both in the digest. The audit itself never
     gates commits; it does not replace the gate or the drift check.
  ```

- [ ] **Task 4.5: `templates/skills/bugfix/SKILL.md.tmpl`: coherent numbering.** The
  `{{ if .layout.docs.pitfalls }}`-gated step 3 breaks the sequence when the doc is disabled.
  Old (the `pitfalls-check` section plus the three following steps):
  ```
  <!-- awf:section pitfalls-check -->
  {{ if .layout.docs.pitfalls }}3. **Check `{{ .layout.docs.pitfalls }}` for known-tricky areas.** The pitfalls list catalogues recurring traps; verify the fix is not re-introducing one that bit before.
  {{ end }}<!-- awf:end -->

  4. **Verify via the gates.** {{ with .vars.gateCmd }}`{{ . }}`{{ else }}The project's gate{{ end }} (fast tier) is the default.{{ if .vars.gateCmdFull }} Run `{{ .vars.gateCmdFull }}` when regression-test placement warrants the full tier.{{ end }}

  5. **Commit** with Conventional Commits, typically `fix(<scope>): ...`, body explains the *why*. Per `{{ .layout.workflowRef }}`, fixes ship with a regression test.

  6. **Invoke `{{ .prefix }}-reviewing-impl` as the terminal step.**
  ```
  New (the pitfalls check becomes an unnumbered note attached to step 2, so the numbered list is
  1-5 whether or not the pitfalls doc is enabled):
  ```
  <!-- awf:section pitfalls-check -->
  {{ if .layout.docs.pitfalls }}   Before writing the fix, check `{{ .layout.docs.pitfalls }}` for known-tricky areas: the pitfalls list catalogues recurring traps; verify the fix is not re-introducing one that bit before.
  {{ end }}<!-- awf:end -->

  3. **Verify via the gates.** {{ with .vars.gateCmd }}`{{ . }}`{{ else }}The project's gate{{ end }} (fast tier) is the default.{{ if .vars.gateCmdFull }} Run `{{ .vars.gateCmdFull }}` when regression-test placement warrants the full tier.{{ end }}

  4. **Commit** with Conventional Commits, typically `fix(<scope>): ...`, body explains the *why*. Per `{{ .layout.workflowRef }}`, fixes ship with a regression test.

  5. **Invoke `{{ .prefix }}-reviewing-impl` as the terminal step.**
  ```

- [ ] **Task 4.6: `templates/skills/subagent-driven-development/SKILL.md.tmpl`: drop the
  maintainer meta-sentence** (`per-task-review-note`). Old:
  ```
  **Per-task review is the recommended discipline.** After each implementer subagent reports `DONE`, dispatch one review subagent (spec-adherence + code quality combined) before advancing to the next task. A project that relies solely on the terminal `{{ .prefix }}-reviewing-impl` review can drop this section; otherwise keep it: catching issues per task is cheaper than catching them in the final pass. Dropping it leaves the terminal `{{ .prefix }}-reviewing-impl` as the only quality gate: no other review stands behind it, so the whole-branch review absorbs everything per-task review would have caught.
  ```
  New:
  ```
  **Per-task review is the recommended discipline.** After each implementer subagent reports `DONE`, dispatch one review subagent (spec-adherence + code quality combined) before advancing to the next task: catching issues per task is cheaper than catching them in the final pass. Skipping it leaves the terminal `{{ .prefix }}-reviewing-impl` as the only quality gate: no other review stands behind it, so the whole-branch review absorbs everything per-task review would have caught.
  ```

- [ ] **Task 4.7: `templates/agents/code-reviewer.md.tmpl`: gate rule in the agent's own
  fix-application procedure** (Review procedure, step 5: spine prose outside the sections).
  Old:
  ```
  1. Apply mechanical and reasoned fixes directly as new commits; note rationale for reasoned fixes. Never `--amend` prior commits.
  ```
  New:
  ```
  1. Apply mechanical and reasoned fixes directly as new commits; run {{ with .vars.gateCmd }}`{{ . }}`{{ else }}the project's gate{{ end }} before each fix commit; note rationale for reasoned fixes. Never `--amend` prior commits.
  ```

- [ ] **Task 4.8: `internal/project/spine_test.go`: give `TestReviewingImplTemplate` a
  layout.** The template now interpolates `.layout.adrDir`/`.layout.plansDir`; without them the
  golden render leaks `<no value>`. Old (in `TestReviewingImplTemplate`):
  ```go
  		"data": map[string]any{},
  ```
  New:
  ```go
  		"layout": map[string]any{"adrDir": "docs/decisions", "plansDir": "docs/plans"},
  		"data":   map[string]any{},
  ```
  The existing `"docs/decisions/"` load-bearing assertion stays green via the rendered layout
  value.

- [ ] **Task 4.9: render + verify.** `go test ./internal/project/ -run 'Template|Golden|Fallback'`
  → `ok`. Then `./x sync && ./x check` → exit 0; `git status --short` shows exactly the 9 skills
  × 2 targets, code-reviewer × 2 targets, and the lock:
  `.claude/skills/awf-{writing-plans,proposing-adr,executing-plans,refactor-coupling-audit,debugging,brainstorming,reviewing-impl,bugfix,subagent-driven-development}/SKILL.md`,
  the same nine under `.cursor/skills/`, `.claude/agents/code-reviewer.md`,
  `.cursor/agents/code-reviewer.md`, `.awf/awf.lock`. Spot-check
  `.claude/skills/awf-reviewing-impl/SKILL.md` renders `docs/decisions/` (from layout) and the
  new audit-blocking wording.

- [ ] **Task 4.10: gate + commit.** `./x gate`, then:
  ```
  git add templates/skills/writing-plans/SKILL.md.tmpl templates/skills/proposing-adr/SKILL.md.tmpl templates/skills/executing-plans/SKILL.md.tmpl templates/skills/refactor-coupling-audit/SKILL.md.tmpl templates/skills/debugging/SKILL.md.tmpl templates/skills/brainstorming/SKILL.md.tmpl templates/skills/reviewing-impl/SKILL.md.tmpl templates/skills/bugfix/SKILL.md.tmpl templates/skills/subagent-driven-development/SKILL.md.tmpl templates/agents/code-reviewer.md.tmpl internal/project/spine_test.go .claude/skills/awf-writing-plans/SKILL.md .claude/skills/awf-proposing-adr/SKILL.md .claude/skills/awf-executing-plans/SKILL.md .claude/skills/awf-refactor-coupling-audit/SKILL.md .claude/skills/awf-debugging/SKILL.md .claude/skills/awf-brainstorming/SKILL.md .claude/skills/awf-reviewing-impl/SKILL.md .claude/skills/awf-bugfix/SKILL.md .claude/skills/awf-subagent-driven-development/SKILL.md .cursor/skills/awf-writing-plans/SKILL.md .cursor/skills/awf-proposing-adr/SKILL.md .cursor/skills/awf-executing-plans/SKILL.md .cursor/skills/awf-refactor-coupling-audit/SKILL.md .cursor/skills/awf-debugging/SKILL.md .cursor/skills/awf-brainstorming/SKILL.md .cursor/skills/awf-reviewing-impl/SKILL.md .cursor/skills/awf-bugfix/SKILL.md .cursor/skills/awf-subagent-driven-development/SKILL.md .claude/agents/code-reviewer.md .cursor/agents/code-reviewer.md .awf/awf.lock
  git commit -m "docs(awf): seal chain-prose seams across skill templates"
  ```

---

## Phase 5: template sweep: generality residue (B2)

- [ ] **Task 5.1: `templates/agents/code-reviewer.md.tmpl`: de-Go the universal lenses.**
  Three edits inside `universal-lenses`/spine:

  (a) old:
  ```
  1. **correctness**: logic errors, edge cases, nil/null dereferences, type-coercion bugs, off-by-one errors, unchecked error paths, race conditions, missing locks; error handling must preserve information (e.g. `%w` wrapping); SQL concurrency issues (wrong index, missing row lock).
  ```
  new:
  ```
  1. **correctness**: logic errors, edge cases, nil/null dereferences, type-coercion bugs, off-by-one errors, unchecked error paths, race conditions, missing locks; error handling must preserve information (wrapping or context propagation, per the language's idiom); storage-layer concurrency (locking, transaction boundaries).
  ```

  (b) old:
  ```
  1. **testing-discipline**: every behaviour-changing change has a regression test; test placement in the correct tier (runtime bugs → unit tests; codegen bugs → fixture tests); test-first ordering for bug fixes (failing test before or in the same commit as the fix); no bypassed gate (coverage regression, skipped test without `SKIP: reason`).
  ```
  new:
  ```
  1. **testing-discipline**: every behaviour-changing change has a regression test; test placement in the tier that exercises the bug's surface; test-first ordering for bug fixes (failing test before or in the same commit as the fix); no bypassed gate (coverage regression, skipped test without `SKIP: reason`).
  ```

  (c) old:
  ```
  1. **doc-currency (impl-level)**: ADR status flips (Accepted→Implemented); state doc updates when domain shifts; workflow/convention doc updates when a rule changes; CLAUDE.md shape preserved.
  ```
  new:
  ```
  1. **doc-currency (impl-level)**: ADR status flips (Accepted→Implemented); state doc updates when domain shifts; workflow/convention doc updates when a rule changes; the rendered agent guide and any runtime bridge files kept current.
  ```

- [ ] **Task 5.2: `templates/agents/plan-reviewer.md.tmpl`: same codegen parenthetical**
  (`universal-lenses`, testing-discipline lens). Old:
  ```
  1. **testing-discipline**: behaviour-changing tasks have regression tests; test placement in the correct tier (runtime bugs → unit tests; codegen bugs → fixture tests); test-first ordering for bug fixes (failing test before or in the same commit as the fix); new invariants extend the invariant test suite where one exists.
  ```
  new:
  ```
  1. **testing-discipline**: behaviour-changing tasks have regression tests; test placement in the tier that exercises the bug's surface; test-first ordering for bug fixes (failing test before or in the same commit as the fix); new invariants extend the invariant test suite where one exists.
  ```

- [ ] **Task 5.3: `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`: language-neutral
  category prose.** Two edits:

  (a) `category-2-sibling-tests`: old:
  ```
  Grep `*_test.go` separately from production code. Test files often use moved symbols as helpers in **unrelated** tests; the test-coupling profile differs from production coupling and is routinely larger.
  ```
  new:
  ```
  Grep your language's test files (e.g. `*_test.go`, `*.spec.ts`) separately from production code. Test files often use moved symbols as helpers in **unrelated** tests; the test-coupling profile differs from production coupling and is routinely larger.
  ```

  (b) `category-6-init-visibility` (section KEY stays `category-6-init-visibility`): old
  heading and final paragraph:
  ```
  ### 6. `init()` ordering and cross-package method visibility
  ```
  ```
  `init()` ordering across packages is hard to reason about. Flag any cross-package `init()` chains the move would break: registry seeding and global-state setup are common load-bearing sites.
  ```
  new:
  ```
  ### 6. Initialization ordering and cross-module visibility
  ```
  ```
  Initialization ordering across modules is hard to reason about. Flag any cross-module initialization chains the move would break (e.g. Go `init()` functions): registry seeding and global-state setup are common load-bearing sites.
  ```
  The rest of the section body (its self-labeled Go examples) stays as is.

- [ ] **Task 5.4: `templates/docs/doc-standard.md.tmpl`: drop the two leaked ADR numbers**
  (`rules` section). Old rule endings:
  ```
  ...the dead-reference gate enforces this mechanically for markdown links (ADR-0020).
  ```
  ```
  ...using the full value from a file already under `docsDir` doubles the path (ADR-0020).
  ```
  new endings (citation removed, both lines):
  ```
  ...the dead-reference gate enforces this mechanically for markdown links.
  ```
  ```
  ...using the full value from a file already under `docsDir` doubles the path.
  ```

- [ ] **Task 5.5: `templates/catalog.yaml`: two data/descriptor touch-ups.**

  (a) code-reviewer `correctnessTraps`: old:
  ```yaml
        - description: boundary conditions at empty, zero, and nil inputs
  ```
  new:
  ```yaml
        - description: boundary conditions at empty, zero, and null/nil inputs
  ```

  (b) `gateDuration` descriptor: old:
  ```yaml
    - key: gateDuration
      kind: string
      description: 'Approximate gate runtime, quoted in docs (e.g. "~15s").'
      default: ""
      options: ["~15s"]
  ```
  new:
  ```yaml
    - key: gateDuration
      kind: string
      description: 'Gate runtime as quoted in docs; the value carries its own approximation marker (e.g. "~15s").'
      default: ""
      options: ["~15s"]
  ```
  This repo's `.awf/config.yaml` `gateDuration: ~15s` stays unchanged.

- [ ] **Task 5.6: `templates/skills/writing-plans/SKILL.md.tmpl`: stop double-tildeing the
  duration** (`gate-tier-note`; the current render shows `(~~15s)`). Old fragment:
  ```
  {{ with .vars.gateDuration }} (~{{ . }}){{ end }}
  ```
  new fragment:
  ```
  {{ with .vars.gateDuration }} ({{ . }}){{ end }}
  ```

- [ ] **Task 5.7: render + verify.** `go test ./internal/catalog/ ./internal/project/` → `ok`
  (this repo's code-reviewer sidecar overrides `correctnessTraps`, so the catalog-data tweak
  changes no rendered file here; the lens prose does). `./x sync && ./x check` → exit 0;
  `git status --short` shows exactly: `docs/doc-standard.md`,
  `.claude/agents/{code-reviewer,plan-reviewer}.md`, `.cursor/agents/{code-reviewer,plan-reviewer}.md`,
  `.claude/skills/awf-{refactor-coupling-audit,writing-plans}/SKILL.md`, the same two under
  `.cursor/skills/`, `.awf/awf.lock`. Confirm `.claude/skills/awf-writing-plans/SKILL.md` now
  reads `(~15s)`.

- [ ] **Task 5.8: gate + commit.** `./x gate`, then:
  ```
  git add templates/agents/code-reviewer.md.tmpl templates/agents/plan-reviewer.md.tmpl templates/skills/refactor-coupling-audit/SKILL.md.tmpl templates/docs/doc-standard.md.tmpl templates/catalog.yaml templates/skills/writing-plans/SKILL.md.tmpl docs/doc-standard.md .claude/agents/code-reviewer.md .claude/agents/plan-reviewer.md .cursor/agents/code-reviewer.md .cursor/agents/plan-reviewer.md .claude/skills/awf-refactor-coupling-audit/SKILL.md .claude/skills/awf-writing-plans/SKILL.md .cursor/skills/awf-refactor-coupling-audit/SKILL.md .cursor/skills/awf-writing-plans/SKILL.md .awf/awf.lock
  git commit -m "docs(awf): sweep language and repo residue from templates"
  ```

---

## Phase 6: template sweep: reference guards + AGENTS.md task-skill gating (B3)

- [ ] **Task 6.1: failing render test first.** Append to
  `internal/project/skillrefs_test.go`:

  ```go
  // A chain-less config that enables only task skills renders with zero dead
  // skill references: every chain-skill mention in a task skill is guarded with
  // generic fallback prose (ADR-0045, ADR-0046).
  func TestTaskSkillsOnlyConfigHasNoDeadRefs(t *testing.T) {
  	got := deadSkillRefs(t,
  		"prefix: example\nvars: {}\nskills: [tdd, bugfix, debugging, refactor-coupling-audit, roadmap-graduation]\ndocs: [roadmap]\nagents: []\n",
  		nil)
  	if len(got) != 0 {
  		t.Fatalf("expected no dead skill references, got %v", got)
  	}
  }
  ```

  Run `go test ./internal/project/ -run TestTaskSkillsOnlyConfigHasNoDeadRefs` → expect `FAIL`
  listing `example-reviewing-impl` (bugfix), `example-brainstorming` (debugging), and
  `example-proposing-adr` (refactor-coupling-audit, roadmap-graduation).

- [ ] **Task 6.2: guard `templates/skills/bugfix/SKILL.md.tmpl`'s two `reviewing-impl` refs**
  (mirroring the file's existing `{{ if .skills.debugging }}` / `{{ if .skills.tdd }}` idiom;
  hyphenated keys need `index`). Old ("When to invoke" second bullet):
  ```
  - **Non-trivial bugfix that ran the chain upstream**: `brainstorming → ... → implementation`. This skill IS that terminal implementation + `{{ .prefix }}-reviewing-impl` pair.
  ```
  new:
  ```
  - **Non-trivial bugfix that ran the chain upstream**: `brainstorming → ... → implementation`. This skill IS that terminal implementation + review pair{{ if index .skills "reviewing-impl" }} (`{{ .prefix }}-reviewing-impl`){{ end }}.
  ```
  Old (final procedure step, renumbered to 5 in Phase 4):
  ```
  5. **Invoke `{{ .prefix }}-reviewing-impl` as the terminal step.**
  ```
  new:
  ```
  5. {{ if index .skills "reviewing-impl" }}**Invoke `{{ .prefix }}-reviewing-impl` as the terminal step.**{{ else }}**Run the project's review step as the terminal step.**{{ end }}
  ```

- [ ] **Task 6.3: guard `templates/skills/debugging/SKILL.md.tmpl`'s two `brainstorming`
  refs.** Old ("When to invoke", final sentence):
  ```
  If the bug reveals a load-bearing design gap rather than a straightforward defect, escalate to `{{ .prefix }}-brainstorming` after confirming the root cause.
  ```
  new:
  ```
  If the bug reveals a load-bearing design gap rather than a straightforward defect, escalate to {{ if .skills.brainstorming }}`{{ .prefix }}-brainstorming`{{ else }}a design discussion before changing behaviour{{ end }} after confirming the root cause.
  ```
  Old (step 6, final sentence):
  ```
  If investigation reveals a design gap rather than a defect, invoke `{{ .prefix }}-brainstorming` instead.
  ```
  new:
  ```
  If investigation reveals a design gap rather than a defect, {{ if .skills.brainstorming }}invoke `{{ .prefix }}-brainstorming`{{ else }}start a design discussion before changing behaviour{{ end }} instead.
  ```

- [ ] **Task 6.4: guard the `proposing-adr` refs in the two audit/graduation task skills.**

  (a) `templates/skills/refactor-coupling-audit/SKILL.md.tmpl` (`when-to-invoke`, last
  sentence): old:
  ```
  This is a **task skill**: it sits off the workflow chain and does not gate it. Its output feeds the ADR's Context section before `{{ .prefix }}-proposing-adr` finalises the Decision.
  ```
  new:
  ```
  This is a **task skill**: it sits off the workflow chain and does not gate it. Its output feeds the ADR's Context section before {{ if index .skills "proposing-adr" }}`{{ .prefix }}-proposing-adr`{{ else }}the project's decision process{{ end }} finalises the Decision.
  ```

  (b) `templates/skills/roadmap-graduation/SKILL.md.tmpl` (`graduate-single-commit`): old:
  ```
  - **Architectural → ADR:** Invoke `{{ .prefix }}-proposing-adr`. Remove the roadmap entry in the same commit as the ADR introduction (or, if the ADR ships across multiple commits, in the final implementation commit).
  ```
  new:
  ```
  - **Architectural → ADR:** {{ if index .skills "proposing-adr" }}Invoke `{{ .prefix }}-proposing-adr`.{{ else }}Write the ADR per the project's decision process.{{ end }} Remove the roadmap entry in the same commit as the ADR introduction (or, if the ADR ships across multiple commits, in the final implementation commit).
  ```

- [ ] **Task 6.5: `templates/agents-doc/AGENTS.md.tmpl`: regate the task-skills sentence.**
  The sentence is currently gated on `adr-lifecycle` alone; regate so each of the four task
  skills appears iff enabled and the sentence renders if ANY is enabled. The line stays inside
  the `{{ if .skills.brainstorming }}` Workflow block (ADR-0046's chain gate), so the guarantee
  is scoped to chain-enabled configs: a chain-less task-skills-only config still renders no
  Workflow prose at all, which is that gate's existing, deliberate behaviour. Old (one line,
  inside the `{{ if .skills.brainstorming }}` block):
  ```
  **Chain skills** (invoke in order): `{{ .prefix }}-brainstorming`, `{{ .prefix }}-proposing-adr`, `{{ .prefix }}-reviewing-adr`, `{{ .prefix }}-writing-plans`, `{{ .prefix }}-reviewing-plan`, `{{ .prefix }}-reviewing-plan-resync`, `{{ .prefix }}-executing-plans` / `{{ .prefix }}-subagent-driven-development`, `{{ .prefix }}-reviewing-impl`.{{ if index .skills "adr-lifecycle" }} **Task skills** (as needed):{{ if .skills.tdd }} `{{ .prefix }}-tdd`,{{ end }}{{ if .skills.bugfix }} `{{ .prefix }}-bugfix`,{{ end }}{{ if .skills.debugging }} `{{ .prefix }}-debugging`,{{ end }} `{{ .prefix }}-adr-lifecycle`.{{ end }}
  ```
  New (one line; `$tasks` accumulates `", `<prefix>-<name>`"` per enabled skill and
  `slice . 1` strips the leading comma; output is byte-identical to today's when all four are
  enabled):
  ```
  **Chain skills** (invoke in order): `{{ .prefix }}-brainstorming`, `{{ .prefix }}-proposing-adr`, `{{ .prefix }}-reviewing-adr`, `{{ .prefix }}-writing-plans`, `{{ .prefix }}-reviewing-plan`, `{{ .prefix }}-reviewing-plan-resync`, `{{ .prefix }}-executing-plans` / `{{ .prefix }}-subagent-driven-development`, `{{ .prefix }}-reviewing-impl`.{{ $tasks := "" }}{{ if .skills.tdd }}{{ $tasks = printf "%s, `%s-tdd`" $tasks $.prefix }}{{ end }}{{ if .skills.bugfix }}{{ $tasks = printf "%s, `%s-bugfix`" $tasks $.prefix }}{{ end }}{{ if .skills.debugging }}{{ $tasks = printf "%s, `%s-debugging`" $tasks $.prefix }}{{ end }}{{ if index .skills "adr-lifecycle" }}{{ $tasks = printf "%s, `%s-adr-lifecycle`" $tasks $.prefix }}{{ end }}{{ with $tasks }} **Task skills** (as needed):{{ slice . 1 }}.{{ end }}
  ```

- [ ] **Task 6.6: spine-test updates for the new guards.** In
  `internal/project/spine_test.go`:

  (a) `TestBugfixTemplate`: old:
  ```go
  		"skills": map[string]bool{"tdd": true, "debugging": true},
  ```
  new:
  ```go
  		"skills": map[string]bool{"tdd": true, "debugging": true, "reviewing-impl": true},
  ```
  (keeps the existing `"example-reviewing-impl"` load-bearing assertion green).

  (b) `TestDebuggingTemplate`: old:
  ```go
  		"skills": map[string]bool{"tdd": true, "bugfix": true},
  ```
  new:
  ```go
  		"skills": map[string]bool{"tdd": true, "bugfix": true, "brainstorming": true},
  ```
  and extend its `loadBearing` list: old:
  ```go
  		"example-bugfix",
  	}
  ```
  new:
  ```go
  		"example-bugfix",
  		"example-brainstorming",
  	}
  ```

  (c) `TestUnsetFallbackRenders`: pin the new fallbacks. Old bugfix case:
  ```go
  			ban: []string{"example-tdd", "example-debugging", "``"},
  ```
  new:
  ```go
  			ban: []string{"example-tdd", "example-debugging", "example-reviewing-impl", "``"},
  ```
  and add `"Run the project's review step as the terminal step."` to that case's `want` list.
  Old debugging case:
  ```go
  			ban: []string{"example-bugfix", "example-tdd", "``"},
  ```
  new:
  ```go
  			ban: []string{"example-bugfix", "example-tdd", "example-brainstorming", "``"},
  ```
  and add `"a design discussion before changing behaviour"` to that case's `want` list.
  Old refactor-coupling-audit case:
  ```go
  		{
  			tmpl: "skills/refactor-coupling-audit/SKILL.md.tmpl",
  			want: []string{"<module-prefix>/"},
  		},
  ```
  new:
  ```go
  		{
  			tmpl: "skills/refactor-coupling-audit/SKILL.md.tmpl",
  			want: []string{"<module-prefix>/", "the project's decision process"},
  			ban:  []string{"example-proposing-adr"},
  		},
  ```

  (d) Append a partial-enablement gating test for the regated AGENTS.md sentence:
  ```go
  // Each task skill appears in the AGENTS.md task-skills sentence iff enabled;
  // the sentence renders when any of the four is (ADR-0046 follow-up sweep).
  func TestAgentsDocTaskSkillsGating(t *testing.T) {
  	data := map[string]any{
  		"prefix": "example",
  		"vars":   map[string]any{"gateCmd": "make gate"},
  		"layout": testLayout(),
  		"data":   map[string]any{},
  		"skills": map[string]bool{"brainstorming": true, "bugfix": true},
  	}
  	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
  	if !strings.Contains(out, "**Task skills** (as needed): `example-bugfix`.") {
  		t.Errorf("expected a bugfix-only task-skills sentence:\n%s", out)
  	}
  	for _, banned := range []string{"example-tdd", "example-debugging", "example-adr-lifecycle"} {
  		if strings.Contains(out, banned) {
  			t.Errorf("disabled task skill %q must not render:\n%s", banned, out)
  		}
  	}
  }
  ```

- [ ] **Task 6.7: render + verify.** `go test ./internal/project/` → `ok`, including the
  Task 6.1 test now green (the ADR-0046 dead-ref scan over the chain-less-with-task-skills
  config finds nothing). `./x sync && ./x check` → exit 0; `git status --short` shows exactly
  `.claude/skills/awf-{bugfix,debugging,refactor-coupling-audit}/SKILL.md`, the same three under
  `.cursor/skills/`, and `.awf/awf.lock`: `AGENTS.md` must NOT appear (this repo enables all
  four task skills, and the regated sentence renders byte-identically);
  `roadmap-graduation` is doc-gate-suppressed here (the `roadmap` doc is off), so its template
  change renders nothing.

- [ ] **Task 6.8: gate + commit.** `./x gate`, then:
  ```
  git add templates/skills/bugfix/SKILL.md.tmpl templates/skills/debugging/SKILL.md.tmpl templates/skills/refactor-coupling-audit/SKILL.md.tmpl templates/skills/roadmap-graduation/SKILL.md.tmpl templates/agents-doc/AGENTS.md.tmpl internal/project/skillrefs_test.go internal/project/spine_test.go .claude/skills/awf-bugfix/SKILL.md .claude/skills/awf-debugging/SKILL.md .claude/skills/awf-refactor-coupling-audit/SKILL.md .cursor/skills/awf-bugfix/SKILL.md .cursor/skills/awf-debugging/SKILL.md .cursor/skills/awf-refactor-coupling-audit/SKILL.md .awf/awf.lock
  git commit -m "docs(awf): guard task-skill chain refs, regate AGENTS.md list"
  ```

---

## Phase 7: template sweep: lifecycle states + resync return edge (B4)

- [ ] **Task 7.1: `templates/skills/adr-lifecycle/SKILL.md.tmpl`: drop the two phantom
  states.** The default `adrStates` table has exactly four states; `Deferred`/`Declined` are
  not among them. In `commit-templates`, delete these two lines (the section keeps its other
  three bullets):
  ```
  - `docs(adr): defer NNNN; <one-line reason>`: `Proposed → Deferred`
  - `docs(adr): decline NNNN; <one-line reason>`: `Proposed → Declined`
  ```
  In `amendment-while-proposed`, keep the mechanism but reword deferral as a scope-shrink
  amendment, not a transition: old:
  ```
  - **Deferral.** When scope shrinks mid-flight, open a `docs(adr): defer <title>; <reason>` commit that updates the ADR Context with what was learned. Deferred work lands in a follow-up ADR or the roadmap.
  ```
  new:
  ```
  - **Deferral.** When scope shrinks mid-flight, amend the still-`Proposed` ADR's Context with what was deferred and why; commit as `docs(adr): amend NNNN, defer <part>; <reason>`. Deferral is a Context edit on a `Proposed` ADR, not a lifecycle state; deferred work lands in a follow-up ADR or the roadmap.
  ```

- [ ] **Task 7.2: `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`: match the
  amendment form** (`scope-shrink-rule`). Old:
  ```
  If the audit reveals the refactor is larger than the ADR's originally proposed scope, **shrink the scope** with a `docs(adr): defer X` amendment to the Context section before implementation starts. Do not proceed with an underscoped ADR.
  ```
  new:
  ```
  If the audit reveals the refactor is larger than the ADR's originally proposed scope, **shrink the scope** with a Context-section amendment to the still-`Proposed` ADR (`docs(adr): amend NNNN, defer X`) recording what was deferred and why, before implementation starts. Do not proceed with an underscoped ADR.
  ```
  (`roadmap-graduation` carries no defer reference (verified); leave it alone.)

- [ ] **Task 7.3: `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`: the ADR return
  edge.** Three edits:

  (a) `classify-route-findings`: old:
  ```
  2. **Surface the digest, then route the findings.** Display the digest the `plan-reviewer` agent returns to the user. Then route the classified findings by classification kind, not severity:
     - **mechanical**: agent applies directly.
     - **reasoned**: agent applies with one-line rationale.
     - **user-decision**: present to the user and wait.
  ```
  new:
  ```
  2. **Surface the digest, then route the findings.** Display the digest the `plan-reviewer` agent returns to the user. Then route the classified findings by classification kind, not severity:
     - **mechanical**: agent applies directly.
     - **reasoned**: agent applies with one-line rationale.
     - **user-decision**: present to the user and wait.

     **Return edge:** when a finding implicates the ADR itself (the plan is right and the still-`Proposed` decision text is wrong), do not bend the plan to stale decision text. Amend the ADR ({{ if index .skills "adr-lifecycle" }}via `{{ .prefix }}-adr-lifecycle`'s amendment-while-Proposed procedure{{ else }}an amendment-while-Proposed edit{{ end }}), re-run `{{ .prefix }}-reviewing-adr` on the amended ADR, then re-run this resync, looping until plan and ADR(s) converge.
  ```

  (b) `apply-fixes-commit`: old final sentence:
  ```
  Only the plan file is edited; no other repository files are touched.
  ```
  new final sentence:
  ```
  Resync fixes edit only the plan file; a finding that takes the return edge above routes its ADR amendment through the ADR's own review before this resync re-runs.
  ```

  (c) `notes` first bullet: old:
  ```
  - This skill never edits the user's repository other than the plan file itself.
  ```
  new:
  ```
  - Resync fixes never edit the repository beyond the plan file; ADR-implicating findings route through the ADR amendment + review skills instead (return edge, step 2).
  ```
  The narrowed-lenses note (next bullet) stands (the return edge re-enters through the ADR
  review, not through extra resync lenses), so it needs no edit. The AGENTS.md/workflow.md
  "looping until they converge" prose is untouched; this edge is what makes it true.

- [ ] **Task 7.4: render + verify.** `go test ./internal/project/ -run 'Template|Fallback'` →
  `ok` (`TestReviewingPlanResyncTemplate` has no skills key, so the golden render exercises the
  `amendment-while-Proposed edit` fallback; `TestAdrLifecycleTemplate`'s assertions all
  survive). `./x sync && ./x check` → exit 0; `git status --short` shows exactly
  `.claude/skills/awf-{adr-lifecycle,refactor-coupling-audit,reviewing-plan-resync}/SKILL.md`,
  the same three under `.cursor/skills/`, and `.awf/awf.lock`.

- [ ] **Task 7.5: gate + commit.** `./x gate`, then:
  ```
  git add templates/skills/adr-lifecycle/SKILL.md.tmpl templates/skills/refactor-coupling-audit/SKILL.md.tmpl templates/skills/reviewing-plan-resync/SKILL.md.tmpl .claude/skills/awf-adr-lifecycle/SKILL.md .claude/skills/awf-refactor-coupling-audit/SKILL.md .claude/skills/awf-reviewing-plan-resync/SKILL.md .cursor/skills/awf-adr-lifecycle/SKILL.md .cursor/skills/awf-refactor-coupling-audit/SKILL.md .cursor/skills/awf-reviewing-plan-resync/SKILL.md .awf/awf.lock
  git commit -m "docs(awf): align lifecycle states and add resync return edge"
  ```

---

## Phase 8: flip ADR-0050 to Implemented

- [ ] **Task 8.1: status flip.** In
  `docs/decisions/0050-reviewing-skill-and-agent-pairing.md`, change the frontmatter line
  `status: Proposed` → `status: Implemented`. All four `inv:` slugs
  (`reviewing-skill-agent-pairing`, `remove-agent-pairing-guard`, `add-skill-pairs-agent`,
  `reviewing-skill-specs-paired`) are backed by the `// invariant:` comments landed in Phases
  1-3.

- [ ] **Task 8.2: config-domain narrative.** ADR-0050's domains are `[config, rendering]`.
  The pairing rule is a config-layer contract, so append this sentence at the end of the single
  paragraph in `.awf/domains/parts/config/current-state.md` (after "...no schema bump, the
  tree's shape is unchanged."):
  ```
  Reviewing skills are pair-validated with the reviewer agents they dispatch (ADR-0050): a catalog `requiresAgent` on each `reviewing-*` spec fails every gated command at project open when an enabled, non-local reviewing skill's agent is missing from the `agents:` array, `awf remove agent` refuses upfront while an enabled skill requires the agent, and `awf add skill` enables a missing required agent in the same config rewrite.
  ```
  The rendering-domain narrative (`.awf/domains/parts/rendering/current-state.md`) is left
  unchanged: the render path is untouched by ADR-0050 (the domain tag reflects where the blind
  spot surfaced, not where the fix lives); its `## Decisions` index picks up ADR-0050's new
  status from frontmatter automatically.

- [ ] **Task 8.3: regenerate + full verification.** Run:
  ```
  ./x sync && ./x check && ./x invariants && ./x gate
  ```
  Expected: sync regenerates `docs/decisions/ACTIVE.md` (ADR-0050 moves to the Implemented
  listing), `docs/domains/config.md` (new narrative sentence + status), and
  `docs/domains/rendering.md` (status only); check reports no drift; `invariants` exits 0 with
  all ADR-0050 slugs backed; gate green. `git status --short` shows exactly:
  `docs/decisions/0050-reviewing-skill-and-agent-pairing.md`, `docs/decisions/ACTIVE.md`,
  `docs/domains/config.md`, `docs/domains/rendering.md`,
  `.awf/domains/parts/config/current-state.md`, `.awf/awf.lock`.

- [ ] **Task 8.4: commit.**
  ```
  git add docs/decisions/0050-reviewing-skill-and-agent-pairing.md docs/decisions/ACTIVE.md docs/domains/config.md docs/domains/rendering.md .awf/domains/parts/config/current-state.md .awf/awf.lock
  git commit -m "docs(adr): flip 0050 Implemented (skill/agent pairing shipped)"
  ```
  Body: names the four backed invariants and the seven preceding commits.

- [ ] **Task 8.5: advisory audit.** Run `./x audit` over the branch; surface findings to the
  user (advisory only: `Warning` findings do not block, and the plan-file commits carry the
  `plans` scope the audit config allows).
