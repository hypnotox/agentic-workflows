# Plan: single commit-scope knob (ADR-0051) + init/CLI UX cluster

**Goal:** Implement [ADR-0051](../decisions/0051-single-commit-scope-knob.md) — delete the
`commitScope` var, route a `commitScopes` init answer into `audit.allowedScopes`, feed the
reviewing-skill prose from the same storage the commit gate enforces, and fold the resolved
scope list into the drift signal — plus the approved init/CLI UX cluster: EOF-silent init
prompting, pre-prompt collision probe, post-init next steps, hook-command descriptor
de-trapping, bare `awf list` covering all seven kinds, `awf help <cmd>` dispatch,
`--answers`/README documentation gaps, an AGENTS.md Commands placeholder + dedupe, and
softened `domains` frontmatter guidance. The final phase flips ADR-0051 to Implemented and
adds the `## [0.6.0]` changelog entry for the whole batch.

**Architecture summary:** Commit scopes get a single storage (`audit.allowedScopes`), reached
three ways: at init via a new `audit-scopes` descriptor target (the ADR-0029 non-var routing
seam, mirroring `invariants-marker`/`catalog-skills`), at render time via a new `commitScopes`
render-context key computed through `audit.Resolve` (the same path `awf commit-gate` reads),
and in the drift oracle via a `render.ReferencesScopes` gate folding the resolved list into
`artifactConfigHash` (mirroring ADR-0046's `ReferencesSkills`). The init UX work reorders
`runInit` (probe → prompt → scaffold → check → sync → notes/next-steps) and adds an
EOF-latching prompt reader in `internal/initspec`. The CLI polish is confined to `cmd/awf`
dispatch and `templates/`. Design rationale lives in ADR-0051 — this plan cites, it does not
restate.

**Tech stack:** Go 1.26, module `github.com/hypnotox/agentic-workflows`. Packages touched:
`internal/initspec`, `internal/config`, `internal/project`, `internal/render`, `cmd/awf`,
`templates/` (embedded catalog + skill/agents-doc/adr-readme templates), `changelog/`.
Gate: `./x gate` (~15s, includes 100% statement coverage per ADR-0012); drift oracle:
`./x sync && ./x check`.

**File structure**

Created:
- `docs/plans/2026-07-02-scope-knob-and-init-ux.md` (this plan)

Modified:
- `templates/catalog.yaml`
- `templates/skills/reviewing-adr/SKILL.md.tmpl`
- `templates/skills/reviewing-plan/SKILL.md.tmpl`
- `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`
- `templates/skills/reviewing-impl/SKILL.md.tmpl`
- `templates/skills/proposing-adr/SKILL.md.tmpl`
- `templates/adr-readme/README.md.tmpl`
- `templates/agents-doc/AGENTS.md.tmpl`
- `internal/initspec/initspec.go`, `internal/initspec/initspec_test.go`
- `internal/config/edit.go`
- `internal/project/scaffold.go`, `internal/project/scaffold_test.go`
- `internal/project/render.go`, `internal/project/confighash.go`
- `internal/project/install.go`
- `internal/project/descriptor_parity_test.go`, `internal/project/templates_vars_test.go`,
  `internal/project/drift_test.go`, `internal/project/spine_test.go`,
  `internal/project/singleton_test.go`
- `internal/render/vars.go`, `internal/render/vars_test.go`
- `cmd/awf/init.go`, `cmd/awf/init_test.go`, `cmd/awf/main.go`, `cmd/awf/help_test.go`,
  `cmd/awf/list_add.go`, `cmd/awf/list_add_test.go`, `cmd/awf/run_test.go`
- `README.md`
- `.awf/config.yaml`, `.awf/awf.lock`
- `.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/rendering/current-state.md`
- rendered outputs under `.claude/skills/awf-reviewing-*/`, `.cursor/skills/awf-reviewing-*/`,
  `.claude/skills/awf-proposing-adr/`, `.cursor/skills/awf-proposing-adr/`,
  `docs/decisions/README.md`, `docs/decisions/ACTIVE.md`, `docs/domains/config.md`,
  `docs/domains/rendering.md`
- `docs/decisions/0051-single-commit-scope-knob.md` (status flip only — append-only otherwise)
- `changelog/CHANGELOG.md`

Deleted: none.

**Conventions for every phase:** run `./x gate` before the commit (expected: tests pass, 100%
coverage, vet+lint clean); stage exactly the listed files (never `git add -A`); Conventional
Commits subject ≤72 chars with scope `awf`, `adr`, or `plans`. Behavior fixes are test-first:
the failing-test task precedes the fix task.

---

## Phase 1 — route `audit-scopes` init answers into `audit.allowedScopes`

Plumbing only (ADR-0051 Decision 2, init side). No catalog descriptor yet — the descriptor
swap must land atomically with the template swap in Phase 2, because
`TestVarDescriptorParity` fails whenever the catalog descriptor set and the templates'
referenced-var set disagree. Phase-1 tests use synthetic descriptors, which the initspec
test fixtures already do.

- [ ] **Task 1.1 — `initspec.Resolve` gains a resolved-scopes return.** In
  `/home/hypno/Projects/agentic-workflows/internal/initspec/initspec.go`:

  Doc comment, old:
  ```go
  // Resolve maps descriptors + answers to a vars map, an optional invariants config,
  // and an optional catalog trim. For a string/enum descriptor the value is: the
  ```
  new:
  ```go
  // Resolve maps descriptors + answers to a vars map, an optional invariants config,
  // an optional catalog trim, and the resolved commit-scope list. For a string/enum
  // descriptor the value is: the
  ```

  Signature, old:
  ```go
  func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool) (map[string]string, *config.InvariantConfig, *config.CatalogTrim, error) {
  ```
  new:
  ```go
  func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool) (map[string]string, *config.InvariantConfig, *config.CatalogTrim, []string, error) {
  ```

  Local declaration, old: `	var marker, globs string` → new: `	var marker, globs, scopesRaw string`

  Target switch, old:
  ```go
		switch d.Target {
		case "invariants-marker":
			marker = val
		case "invariants-globs":
			globs = val
		default:
			vars[d.Key] = val
		}
  ```
  new:
  ```go
		switch d.Target {
		case "invariants-marker":
			marker = val
		case "invariants-globs":
			globs = val
		case "audit-scopes":
			scopesRaw = val
		default:
			vars[d.Key] = val
		}
  ```

  All three error returns in Resolve gain a `nil`: replace each
  `return nil, nil, nil, err` with `return nil, nil, nil, nil, err` (two sites: the
  multiselect error and the prompt error), and
  `return nil, nil, nil, errors.New("initspec: invariantsMarker and invariantsGlobs must be set together")`
  with
  `return nil, nil, nil, nil, errors.New("initspec: invariantsMarker and invariantsGlobs must be set together")`.

  Final return, old: `	return vars, inv, trim, nil` → new (comma-split, trim, drop empties —
  `splitNames` already does exactly this; empty answer yields nil):
  ```go
	return vars, inv, trim, splitNames(scopesRaw), nil
  ```

- [ ] **Task 1.2 — update the 13 `Resolve` call sites in
  `/home/hypno/Projects/agentic-workflows/internal/initspec/initspec_test.go`.** Three
  patterns, applied verbatim at every occurrence:
  - `vars, inv, _, err := Resolve(` → `vars, inv, _, _, err := Resolve(` (4 sites: lines 26, 40, 56, 71)
  - `_, _, trim, err := Resolve(` → `_, _, trim, _, err := Resolve(` (3 sites: lines 173, 186, 209)
  - `if _, _, _, err := Resolve(` → `if _, _, _, _, err := Resolve(` (6 sites: lines 84, 91, 100, 200, 223, 230)

- [ ] **Task 1.3 — routing tests (backs `inv: audit-scopes-descriptor-routed`).** Append to
  `internal/initspec/initspec_test.go`:
  ```go
  // An audit-scopes answer is comma-split, trimmed, empties dropped, and routed
  // out of the vars map (ADR-0051).
  func TestResolveAuditScopes(t *testing.T) {
  	ds := []catalog.VarDescriptor{{Key: "commitScopes", Kind: "string", Target: "audit-scopes"}}
  	vars, _, _, scopes, err := Resolve(ds, map[string]string{"commitScopes": " adr, awf ,,plans "}, strings.NewReader(""), &strings.Builder{}, false)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if _, ok := vars["commitScopes"]; ok {
  		t.Error("audit-scopes answer must not land in the vars map")
  	}
  	if !slices.Equal(scopes, []string{"adr", "awf", "plans"}) {
  		t.Errorf("scopes = %v, want [adr awf plans]", scopes)
  	}
  }

  // An empty (or absent) audit-scopes answer resolves to nil — accept-any
  // audit semantics, nothing written (ADR-0051, ADR-0017).
  func TestResolveAuditScopesEmptyIsNil(t *testing.T) {
  	ds := []catalog.VarDescriptor{{Key: "commitScopes", Kind: "string", Target: "audit-scopes"}}
  	_, _, _, scopes, err := Resolve(ds, nil, strings.NewReader(""), &strings.Builder{}, false)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if scopes != nil {
  		t.Errorf("empty answer must resolve to nil scopes, got %v", scopes)
  	}
  }
  ```
  (`slices` is already imported in this test file.)

- [ ] **Task 1.4 — `config.Skeleton` gains an audit block.** In
  `/home/hypno/Projects/agentic-workflows/internal/config/edit.go`, old:
  ```go
  type Skeleton struct {
  	Prefix     string            `yaml:"prefix"`
  	Vars       map[string]string `yaml:"vars"`
  	Skills     []string          `yaml:"skills"`
  	Agents     []string          `yaml:"agents"`
  	Docs       []string          `yaml:"docs"`
  	Invariants *InvariantConfig  `yaml:"invariants,omitempty"`
  	Bootstrap  *BootstrapConfig  `yaml:"bootstrap,omitempty"`
  	Hooks      *HooksConfig      `yaml:"hooks,omitempty"`
  }
  ```
  new:
  ```go
  type Skeleton struct {
  	Prefix     string            `yaml:"prefix"`
  	Vars       map[string]string `yaml:"vars"`
  	Skills     []string          `yaml:"skills"`
  	Agents     []string          `yaml:"agents"`
  	Docs       []string          `yaml:"docs"`
  	Audit      *SkeletonAudit    `yaml:"audit,omitempty"`
  	Invariants *InvariantConfig  `yaml:"invariants,omitempty"`
  	Bootstrap  *BootstrapConfig  `yaml:"bootstrap,omitempty"`
  	Hooks      *HooksConfig      `yaml:"hooks,omitempty"`
  }

  // SkeletonAudit is the audit block a scaffold can seed (ADR-0051): only
  // allowedScopes — the one audit field init collects. Deliberately not
  // *AuditConfig, whose zero-value fields would serialize as explicit settings.
  type SkeletonAudit struct {
  	AllowedScopes []string `yaml:"allowedScopes"`
  }
  ```
  (`config.Load` already strict-parses `audit.allowedScopes` via `AuditConfig`; no schema
  change — additive optional key.)

- [ ] **Task 1.5 — `ScaffoldConfig` writes the audit block.** In
  `/home/hypno/Projects/agentic-workflows/internal/project/scaffold.go`, signature old:
  ```go
  func ScaffoldConfig(prefix string, vars map[string]string, inv *config.InvariantConfig, trim *config.CatalogTrim) ([]byte, error) {
  ```
  new:
  ```go
  func ScaffoldConfig(prefix string, vars map[string]string, inv *config.InvariantConfig, trim *config.CatalogTrim, scopes []string) ([]byte, error) {
  ```
  Immediately before the `return config.MarshalSkeleton(config.Skeleton{` statement, insert:
  ```go
	// A non-empty resolved commitScopes answer becomes the audit block; an empty
	// answer writes nothing — nil audit.allowedScopes = accept any (ADR-0017,
	// ADR-0051 Decision 2).
	// invariant: audit-scopes-descriptor-routed
	var auditBlk *config.SkeletonAudit
	if len(scopes) > 0 {
		auditBlk = &config.SkeletonAudit{AllowedScopes: scopes}
	}
  ```
  and in the `config.Skeleton{...}` literal, after the `Docs:       docNames,` line, add:
  ```go
		Audit:      auditBlk,
  ```
  Also extend the ScaffoldConfig doc comment's final sentence, old:
  `// and the git-hook payloads (ADR-0048) enabled by default.` new:
  `// and the git-hook payloads (ADR-0048) enabled by default, and writes a resolved`
  `// commit-scope list to audit.allowedScopes (ADR-0051).`

- [ ] **Task 1.6 — update `ScaffoldConfig` call sites.**
  - `/home/hypno/Projects/agentic-workflows/cmd/awf/init.go` line 44, old:
    ```go
	vars, inv, trim, err := initspec.Resolve(descs, answers, stdin, stdout, isInteractive())
    ```
    new:
    ```go
	vars, inv, trim, scopes, err := initspec.Resolve(descs, answers, stdin, stdout, isInteractive())
    ```
    and line 55, old:
    ```go
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv, trim)
    ```
    new:
    ```go
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv, trim, scopes)
    ```
  - `/home/hypno/Projects/agentic-workflows/cmd/awf/list_add_test.go` line 20:
    `project.ScaffoldConfig("example", nil, nil, nil)` → `project.ScaffoldConfig("example", nil, nil, nil, nil)`
  - `/home/hypno/Projects/agentic-workflows/internal/project/scaffold_test.go`: append `, nil`
    to every existing call (8 sites at lines 23, 68, 116, 139, 158, 190, 232, 265 — the two
    trim sites become e.g. `ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Skills: &pickSkills}, nil)`).

- [ ] **Task 1.7 — scaffold test.** Append to `internal/project/scaffold_test.go`:
  ```go
  // A resolved scope list lands under audit.allowedScopes; an empty list writes
  // no audit key at all (ADR-0051).
  // invariant: audit-scopes-descriptor-routed
  func TestScaffoldWritesAuditScopes(t *testing.T) {
  	b, err := ScaffoldConfig("example", nil, nil, nil, []string{"adr", "awf"})
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	for _, want := range []string{"audit:", "allowedScopes:", "- adr", "- awf"} {
  		if !strings.Contains(string(b), want) {
  			t.Errorf("scaffold missing %q:\n%s", want, b)
  		}
  	}
  	b2, err := ScaffoldConfig("example", nil, nil, nil, nil)
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	if strings.Contains(string(b2), "audit:") {
  		t.Errorf("nil scopes must write no audit block:\n%s", b2)
  	}
  }
  ```
  (`strings` is already imported there; if not, add it.)

- [ ] **Task 1.8 — verify and commit.**
  ```
  go test ./internal/initspec/... ./internal/project/... ./internal/config/... ./cmd/awf/...
  ./x gate
  git add internal/initspec/initspec.go internal/initspec/initspec_test.go internal/config/edit.go internal/project/scaffold.go internal/project/scaffold_test.go cmd/awf/init.go cmd/awf/list_add_test.go
  git commit -m "feat(awf): route audit-scopes init answers into audit.allowedScopes"
  ```

---

## Phase 2 — render commit scopes from `audit.allowedScopes` (ADR-0051 core)

Atomic by necessity: the catalog descriptor swap, the eight template-site swaps, the render
context, and the confighash gate must land together or `TestVarDescriptorParity` /
`awf check` fail in between.

- [ ] **Task 2.1 — swap the catalog descriptor.** In
  `/home/hypno/Projects/agentic-workflows/templates/catalog.yaml`, old (lines 380–384):
  ```yaml
    - key: commitScope
      kind: string
      description: Conventional Commits scope for this project.
      default: ""
      options: ["awf"]
  ```
  new:
  ```yaml
    - key: commitScopes
      kind: string
      target: audit-scopes
      description: Comma-separated Conventional Commits scopes this project allows. Written to audit.allowedScopes — enforced by awf commit-gate/audit and quoted by the reviewing skills. Leave empty to accept any scope.
      default: ""
      options: ["adr,awf,plans"]
  ```

- [ ] **Task 2.2 — descriptor parity accommodates the new target.** In
  `/home/hypno/Projects/agentic-workflows/internal/project/descriptor_parity_test.go`, old:
  ```go
  var validTargets = []string{"", "var", "invariants-marker", "invariants-globs", "catalog-skills", "catalog-docs"}
  ```
  new:
  ```go
  var validTargets = []string{"", "var", "invariants-marker", "invariants-globs", "catalog-skills", "catalog-docs", "audit-scopes"}
  ```
  (The parity assertions themselves need no change: `commitScopes` has a non-var target, so
  it is exempt from the var-parity checks, and `commitScope` disappears from both sides.)

- [ ] **Task 2.3 — `render.ReferencesScopes`.** In
  `/home/hypno/Projects/agentic-workflows/internal/render/vars.go`, after the `skillsRE`
  declaration and `ReferencesSkills`, add:
  ```go
  var scopesRE = regexp.MustCompile(`\{\{[^{}]*[.$]commitScopes[^{}]*\}\}`)

  // ReferencesScopes reports whether src reads the resolved commit-scope render
  // context (any {{ … .commitScopes … }} action) — such templates fold the
  // resolved scope list into their config hash (ADR-0051, mirroring ADR-0046's
  // ReferencesSkills).
  func ReferencesScopes(src string) bool { return scopesRE.MatchString(src) }
  ```
  Append to `/home/hypno/Projects/agentic-workflows/internal/render/vars_test.go`:
  ```go
  func TestReferencesScopes(t *testing.T) {
  	if !ReferencesScopes("x {{ with .commitScopes }}y{{ end }} z") {
  		t.Error("expected a .commitScopes action to be detected")
  	}
  	if ReferencesScopes("prose mentioning .commitScopes outside an action") {
  		t.Error("a non-action mention must not match")
  	}
  }
  ```

- [ ] **Task 2.4 — render context key.** In
  `/home/hypno/Projects/agentic-workflows/internal/project/render.go`:
  add `"github.com/hypnotox/agentic-workflows/internal/audit"` to the imports; in
  `Project.data`, old:
  ```go
	return map[string]any{
		"prefix":  p.Cfg.Prefix,
		"vars":    nonNil(p.Cfg.Vars),
		"data":    nonNil(sc.Data),
		"layout":  p.layout().templateMap(),
		"version": Version,
		"skills":  p.effSkills,
	}
  ```
  new:
  ```go
	return map[string]any{
		"prefix":       p.Cfg.Prefix,
		"vars":         nonNil(p.Cfg.Vars),
		"data":         nonNil(sc.Data),
		"layout":       p.layout().templateMap(),
		"version":      Version,
		"skills":       p.effSkills,
		"commitScopes": p.commitScopesDisplay(),
	}
  ```
  and add below `data`:
  ```go
  // commitScopesDisplay returns the display-formatted allowed commit-scope list
  // (e.g. "`adr`, `awf`, `plans`") resolved from audit.allowedScopes — the same
  // audit.Resolve path awf commit-gate reads, so prose and gate agree by
  // construction — or "" when scopes are accept-any (ADR-0051).
  func (p *Project) commitScopesDisplay() string {
  	scopes := audit.Resolve(p.Cfg.Audit).AllowedScopes
  	if len(scopes) == 0 {
  		return ""
  	}
  	quoted := make([]string, len(scopes))
  	for i, s := range scopes {
  		quoted[i] = "`" + s + "`"
  	}
  	return strings.Join(quoted, ", ")
  }
  ```

- [ ] **Task 2.5 — confighash fold (backs `inv: scopes-in-confighash`).** In
  `/home/hypno/Projects/agentic-workflows/internal/project/confighash.go`: add
  `"github.com/hypnotox/agentic-workflows/internal/audit"` to the imports, and after the
  existing `ReferencesSkills` block, insert:
  ```go
	if render.ReferencesScopes(assembled) {
		// A template that reads .commitScopes re-renders when audit.allowedScopes
		// changes; folding the resolved list in flags it stale (ADR-0051).
		// invariant: scopes-in-confighash
		proj["commitScopes"] = audit.Resolve(p.Cfg.Audit).AllowedScopes
	}
  ```

- [ ] **Task 2.6 — swap the eight template sites.** In each file the swapped clause is
  identical; the surrounding sentences differ and are preserved exactly.

  `/home/hypno/Projects/agentic-workflows/templates/skills/reviewing-adr/SKILL.md.tmpl`:
  - line 32, old:
    ```
       - The commit convention: apply fixes as new commits (never `--amend`) using {{ with .vars.commitScope }}the `{{ . }}` scope{{ else }}the project's commit scope{{ end }}.
    ```
    new:
    ```
       - The commit convention: apply fixes as new commits (never `--amend`) {{ with .commitScopes }}using a Conventional-Commits scope from {{ . }}{{ else }}using the project's commit scope conventions{{ end }}.
    ```
  - line 45, old:
    ```
    5. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) using {{ with .vars.commitScope }}the `{{ . }}` scope{{ else }}the project's commit scope{{ end }}. The agent applies the edits; this skill ensures the commit convention is followed.
    ```
    new:
    ```
    5. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) {{ with .commitScopes }}using a Conventional-Commits scope from {{ . }}{{ else }}using the project's commit scope conventions{{ end }}. The agent applies the edits; this skill ensures the commit convention is followed.
    ```

  `/home/hypno/Projects/agentic-workflows/templates/skills/reviewing-plan/SKILL.md.tmpl`:
  - line 36: same old→new as reviewing-adr line 32.
  - line 49, old:
    ```
    5. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) using {{ with .vars.commitScope }}the `{{ . }}` scope{{ else }}the project's commit scope{{ end }}. The agent applies the edits; this skill ensures the commit convention is followed. Only the plan file is edited; no other repository files are touched.
    ```
    new:
    ```
    5. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) {{ with .commitScopes }}using a Conventional-Commits scope from {{ . }}{{ else }}using the project's commit scope conventions{{ end }}. The agent applies the edits; this skill ensures the commit convention is followed. Only the plan file is edited; no other repository files are touched.
    ```

  `/home/hypno/Projects/agentic-workflows/templates/skills/reviewing-plan-resync/SKILL.md.tmpl`:
  - line 27: same old→new as reviewing-adr line 32.
  - line 42, old:
    ```
    3. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) using {{ with .vars.commitScope }}the `{{ . }}` scope{{ else }}the project's commit scope{{ end }}. The agent applies the edits; this skill ensures the commit convention is followed. Resync fixes edit only the plan file; a finding that takes the return edge above routes its ADR amendment through the ADR's own review before this resync re-runs.
    ```
    new:
    ```
    3. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) {{ with .commitScopes }}using a Conventional-Commits scope from {{ . }}{{ else }}using the project's commit scope conventions{{ end }}. The agent applies the edits; this skill ensures the commit convention is followed. Resync fixes edit only the plan file; a finding that takes the return edge above routes its ADR amendment through the ADR's own review before this resync re-runs.
    ```

  `/home/hypno/Projects/agentic-workflows/templates/skills/reviewing-impl/SKILL.md.tmpl`:
  - line 37: same old→new as reviewing-adr line 32.
  - line 50 (also carries the gateCmd clause — preserve it), old:
    ```
    5. **Commit applied fixes.** Fixes land as new commits (never `--amend`) using {{ with .vars.commitScope }}the `{{ . }}` scope{{ else }}the project's commit scope{{ end }}; {{ with .vars.gateCmd }}`{{ . }}`{{ else }}the gate{{ end }} passes before each commit. The agent applies the edits; this skill ensures the commit convention is followed.
    ```
    new:
    ```
    5. **Commit applied fixes.** Fixes land as new commits (never `--amend`) {{ with .commitScopes }}using a Conventional-Commits scope from {{ . }}{{ else }}using the project's commit scope conventions{{ end }}; {{ with .vars.gateCmd }}`{{ . }}`{{ else }}the gate{{ end }} passes before each commit. The agent applies the edits; this skill ensures the commit convention is followed.
    ```

- [ ] **Task 2.7 — update the spine-test fixtures.** In
  `/home/hypno/Projects/agentic-workflows/internal/project/spine_test.go` the four reviewing
  templates are rendered directly; move the scope from the (now-unreferenced) var to the new
  top-level context key:
  - `TestReviewingPlanTemplate` (line ~567) and `TestReviewingPlanResyncTemplate` (line ~598), old:
    ```go
		"vars": map[string]any{
			"commitScope": "docs(plans)",
		},
    ```
    new (both):
    ```go
		"vars":         map[string]any{},
		"commitScopes": "`docs(plans)`",
    ```
  - `TestReviewingAdrTemplate` (line ~628), old:
    ```go
		"vars": map[string]any{
			"commitScope": "docs(adr)",
		},
    ```
    new:
    ```go
		"vars":         map[string]any{},
		"commitScopes": "`docs(adr)`",
    ```
  - `TestReviewingImplTemplate` (line ~659), old:
    ```go
		"vars": map[string]any{
			"commitScope": "feat",
			"gateCmd":     "./x gate",
		},
    ```
    new:
    ```go
		"vars": map[string]any{
			"gateCmd": "./x gate",
		},
		"commitScopes": "`feat`",
    ```

- [ ] **Task 2.8 — single-storage scan test (backs `inv: commit-scope-single-storage`).**
  Append to `/home/hypno/Projects/agentic-workflows/internal/project/templates_vars_test.go`
  (add `"github.com/hypnotox/agentic-workflows/internal/catalog"` to its imports):
  ```go
  // TestCommitScopeSingleStorage asserts commit scopes have one storage
  // (ADR-0051): no template references .vars.commitScope and the catalog vars
  // block carries no commitScope descriptor — every rendered scope mention
  // derives from audit.allowedScopes via the commitScopes render-context key.
  // invariant: commit-scope-single-storage
  func TestCommitScopeSingleStorage(t *testing.T) {
  	err := fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
  		if err != nil {
  			return err
  		}
  		if d.IsDir() {
  			return nil
  		}
  		b, err := fs.ReadFile(templates.FS, path)
  		if err != nil {
  			return err
  		}
  		if strings.Contains(string(b), ".vars.commitScope") {
  			t.Errorf("%s references .vars.commitScope — commit scopes live in audit.allowedScopes (ADR-0051)", path)
  		}
  		return nil
  	})
  	if err != nil {
  		t.Fatal(err)
  	}
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("catalog.Load: %v", err)
  	}
  	for _, d := range cat.Vars {
  		if d.Key == "commitScope" {
  			t.Error("catalog still carries a commitScope var descriptor (ADR-0051)")
  		}
  	}
  }
  ```

- [ ] **Task 2.9 — drift test: a scopes edit reflags exactly the referencing artifacts.**
  Append to `/home/hypno/Projects/agentic-workflows/internal/project/drift_test.go`
  (reuses `scaffold`, `testsupport.WriteAwfConfig`, and the `Open`/`Sync`/`Check` flow that
  `TestPerTargetDriftProjection` uses; add `os` and `strings` to the file's imports — it has
  neither today). The config enables the full reference-closed core chain plus all three
  reviewer agents: the rendered reviewing skills name the other chain skills unconditionally,
  so a thinner enable set makes `Check` return ADR-0046 `dead-skill-reference` findings that
  break the all-stale assertion, and `Open` hard-errors on an ADR-0050 unpaired reviewing
  skill. `tdd` (non-core, non-doc-gated, explicitly enabled here) never reads
  `.commitScopes`, so it is the control:
  ```go
  // Editing audit.allowedScopes reflags exactly the artifacts whose assembled
  // templates reference .commitScopes; non-referencing artifacts stay in sync,
  // and the rendered prose quotes the configured scopes (ADR-0051).
  // invariant: scopes-in-confighash
  func TestScopesEditReflagsReferencingArtifacts(t *testing.T) {
  	cfg := func(scope string) string {
  		return "prefix: example\nvars: {}\nskills:\n" +
  			"  - adr-lifecycle\n  - brainstorming\n  - executing-plans\n  - proposing-adr\n" +
  			"  - reviewing-adr\n  - reviewing-impl\n  - reviewing-plan\n  - reviewing-plan-resync\n" +
  			"  - subagent-driven-development\n  - tdd\n  - writing-plans\n" +
  			"agents:\n  - adr-reviewer\n  - code-reviewer\n  - plan-reviewer\n" +
  			"audit:\n  allowedScopes:\n    - " + scope + "\n"
  	}
  	root := scaffold(t, cfg("awf"))
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if err := p.Sync(); err != nil {
  		t.Fatal(err)
  	}
  	rendered, err := os.ReadFile(filepath.Join(root, ".claude/skills/example-reviewing-adr/SKILL.md"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !strings.Contains(string(rendered), "using a Conventional-Commits scope from `awf`") {
  		t.Errorf("rendered prose does not quote audit.allowedScopes:\n%s", rendered)
  	}
  	testsupport.WriteAwfConfig(t, root, cfg("core"))
  	p2, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	drift, err := p2.Check()
  	if err != nil {
  		t.Fatal(err)
  	}
  	flagged := map[string]bool{}
  	for _, d := range drift {
  		if d.Kind != "stale" {
  			t.Errorf("unexpected drift kind %q on %s", d.Kind, d.Path)
  		}
  		flagged[d.Path] = true
  	}
  	if !flagged[".claude/skills/example-reviewing-adr/SKILL.md"] {
  		t.Errorf("scopes edit did not reflag the referencing skill; drift = %v", drift)
  	}
  	if flagged[".claude/skills/example-tdd/SKILL.md"] {
  		t.Error("scopes edit reflagged the non-referencing tdd skill")
  	}
  }
  ```

- [ ] **Task 2.10 — end-to-end init test.** Append to
  `/home/hypno/Projects/agentic-workflows/cmd/awf/init_test.go`:
  ```go
  // A commitScopes answer lands in audit.allowedScopes, never in vars; the
  // silent default writes no audit block (ADR-0051).
  func TestInitCommitScopesAnswer(t *testing.T) {
  	root := t.TempDir()
  	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
  	forceNonInteractive(t)
  	var out, errb bytes.Buffer
  	if code := run([]string{"awf", "init", "--set", "commitScopes=adr, awf"}, &out, &errb); code != 0 {
  		t.Fatalf("init --set commitScopes: exit %d (%s)", code, errb.String())
  	}
  	cfg := readInitConfig(t, root)
  	for _, want := range []string{"audit:", "allowedScopes:", "- adr", "- awf"} {
  		if !strings.Contains(cfg, want) {
  			t.Errorf("config missing %q:\n%s", want, cfg)
  		}
  	}
  	if strings.Contains(cfg, "commitScopes:") {
  		t.Errorf("commitScopes must not be seeded as a var:\n%s", cfg)
  	}
  }
  ```
  (The existing `TestInitNonInteractiveDefault` plus Task 1.7's nil-scopes assertion cover
  the empty-answer-writes-nothing half.)

- [ ] **Task 2.11 — drop the var from this repo's config.** In
  `/home/hypno/Projects/agentic-workflows/.awf/config.yaml`, delete the line:
  ```yaml
    commitScope: awf
  ```
  (under `vars:`; the surrounding lines `commitGateCmd: ./x commit-gate` and
  `docCurrencyTargets: ...` stay).

- [ ] **Task 2.12 — sync, verify, commit.**
  ```
  ./x sync      # expected: "awf sync: done"
  ./x check     # expected: "awf check: clean"
  grep -c "using a Conventional-Commits scope from \`adr\`, \`awf\`, \`plans\`" .claude/skills/awf-reviewing-impl/SKILL.md
  ```
  Expected grep output: `2`. The sync rewrites the four reviewing skills under both
  `.claude/skills/` and `.cursor/skills/` (8 files) plus `.awf/awf.lock`.
  ```
  ./x gate
  git add templates/catalog.yaml templates/skills/reviewing-adr/SKILL.md.tmpl templates/skills/reviewing-plan/SKILL.md.tmpl templates/skills/reviewing-plan-resync/SKILL.md.tmpl templates/skills/reviewing-impl/SKILL.md.tmpl internal/render/vars.go internal/render/vars_test.go internal/project/render.go internal/project/confighash.go internal/project/descriptor_parity_test.go internal/project/templates_vars_test.go internal/project/drift_test.go internal/project/spine_test.go cmd/awf/init_test.go .awf/config.yaml .awf/awf.lock .claude/skills/awf-reviewing-adr/SKILL.md .claude/skills/awf-reviewing-plan/SKILL.md .claude/skills/awf-reviewing-plan-resync/SKILL.md .claude/skills/awf-reviewing-impl/SKILL.md .cursor/skills/awf-reviewing-adr/SKILL.md .cursor/skills/awf-reviewing-plan/SKILL.md .cursor/skills/awf-reviewing-plan-resync/SKILL.md .cursor/skills/awf-reviewing-impl/SKILL.md
  git commit -m "feat(awf): render commit-scope prose from audit.allowedScopes"
  ```

---

## Phase 3 — init prompting falls silent after stdin EOF (test-first)

`awf init < /dev/null` today: `/dev/null` is a char device, so `isInteractive()` is true and
every prompt streams to stdout while each read EOFs to the default
(`cmd/awf/main.go:20-23`, `internal/initspec/initspec.go` prompt loop). Fix at the EOF
layer: latch EOF in a small reader wrapper; once latched, all remaining descriptors take the
silent path and no further prompt text is emitted.

- [ ] **Task 3.1 — failing regression test.** Append to
  `/home/hypno/Projects/agentic-workflows/internal/initspec/initspec_test.go`:
  ```go
  // A prompt stream that hits EOF (e.g. /dev/null, which stats as a char device
  // and so counts as interactive) switches every remaining descriptor to the
  // silent path: the in-flight prompt keeps its default, no further prompt text
  // is emitted, and later values resolve empty.
  func TestResolveEOFFallsSilent(t *testing.T) {
  	ds := []catalog.VarDescriptor{
  		{Key: "first", Kind: "string", Default: "d1"},
  		{Key: "second", Kind: "string", Default: "d2"},
  	}
  	var out strings.Builder
  	vars, _, _, _, err := Resolve(ds, nil, strings.NewReader(""), &out, true)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !strings.Contains(out.String(), "first —") {
  		t.Errorf("the first prompt should have been emitted:\n%s", out.String())
  	}
  	if strings.Contains(out.String(), "second —") {
  		t.Errorf("prompt text emitted after EOF:\n%s", out.String())
  	}
  	if vars["first"] != "d1" {
  		t.Errorf(`vars["first"] = %q, want the prompted default "d1"`, vars["first"])
  	}
  	if vars["second"] != "" {
  		t.Errorf(`vars["second"] = %q, want "" (silent path)`, vars["second"])
  	}
  }
  ```
  Run `go test ./internal/initspec/ -run TestResolveEOFFallsSilent` — expected: FAIL with
  `prompt text emitted after EOF` (current code prompts every descriptor).

- [ ] **Task 3.2 — the fix.** In
  `/home/hypno/Projects/agentic-workflows/internal/initspec/initspec.go`:

  Add above `Resolve`:
  ```go
  // promptReader wraps the prompt input and latches EOF, so Resolve stops
  // prompting (and stops emitting prompt text) once the input is exhausted —
  // an init reading /dev/null or a closed stdin degrades to the silent path
  // instead of streaming every remaining prompt to nobody.
  type promptReader struct {
  	r   *bufio.Reader
  	eof bool
  }

  // line reads one line; EOF is latched, not returned — the partial line (or
  // empty string) read alongside it is still the answer.
  func (pr *promptReader) line() (string, error) {
  	s, err := pr.r.ReadString('\n')
  	if err == io.EOF {
  		pr.eof = true
  		return s, nil
  	}
  	if err != nil {
  		return "", fmt.Errorf("initspec: read input: %w", err)
  	}
  	return s, nil
  }
  ```

  In `Resolve`, old: `	r := bufio.NewReader(in)` → new: `	r := &promptReader{r: bufio.NewReader(in)}`

  Multiselect dispatch, old:
  ```go
			sel, selected, err := resolveMultiselect(r, out, d, answers, interactive)
  ```
  new:
  ```go
			sel, selected, err := resolveMultiselect(r, out, d, answers, interactive && !r.eof)
  ```

  String/enum dispatch, old:
  ```go
		if !ok {
			if interactive {
  ```
  new:
  ```go
		if !ok {
			if interactive && !r.eof {
  ```

  `resolveMultiselect` signature: `func resolveMultiselect(r *bufio.Reader, ...)` →
  `func resolveMultiselect(r *promptReader, ...)`.

  `promptMultiselect` signature: `func promptMultiselect(r *bufio.Reader, ...)` →
  `func promptMultiselect(r *promptReader, ...)`; its read, old:
  ```go
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, false, fmt.Errorf("initspec: read input: %w", err)
	}
  ```
  new:
  ```go
	line, err := r.line()
	if err != nil {
		return nil, false, err
	}
  ```

  `prompt` signature: `func prompt(r *bufio.Reader, ...)` → `func prompt(r *promptReader, ...)`;
  its read, old:
  ```go
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("initspec: read input: %w", err)
	}
  ```
  new:
  ```go
	line, err := r.line()
	if err != nil {
		return "", err
	}
  ```
  (`bufio`, `io`, `fmt` imports all stay; the read-error message is unchanged, so
  `TestResolvePromptReadError` and `TestResolveMultiselectPromptReadError` keep passing. The
  cmd-level `TestInitInteractivePromptWiring` also keeps passing — its prompted first value
  is consumed before EOF, and the later descriptors now resolve empty, which is what it
  asserts.)

- [ ] **Task 3.3 — verify and commit.**
  ```
  go test ./internal/initspec/... ./cmd/awf/...
  ./x gate
  git add internal/initspec/initspec.go internal/initspec/initspec_test.go
  git commit -m "fix(awf): fall silent after stdin EOF during init prompting"
  ```

---

## Phase 4 — probe init collisions before prompting (test-first)

`cmd/awf/init.go` today prompts fully, scaffolds the config, and only then lets
`InitCollisions` refuse — wasted prompts and a rollback. Reorder: a conservative pre-prompt
probe against the curated-core planned-output set (computed by scaffolding into a temp dir —
the paths are project-relative, so they transfer; the prefix is `filepath.Base(root)` either
way). The existing post-answer check + rollback stays as the accurate second line — a trim
answer (`--set skills=...`) can enable non-core artifacts the curated-core probe set does
not cover. `--force` skips the probe.

- [ ] **Task 4.1 — failing test.** Append to
  `/home/hypno/Projects/agentic-workflows/cmd/awf/run_test.go`:
  ```go
  // A collision refuses BEFORE any prompt: with a colliding AGENTS.md and an
  // interactive stdin, init exits without emitting a single prompt line and
  // without creating .awf/.
  func TestInitCollisionProbeRefusesBeforePrompts(t *testing.T) {
  	root := t.TempDir()
  	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("mine\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
  	testsupport.SwapVar(t, &isInteractive, func() bool { return true })
  	testsupport.SwapVar(t, &stdin, io.Reader(strings.NewReader("SHOULD-NOT-BE-CONSUMED\n")))
  	var out, errb bytes.Buffer
  	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
  		t.Fatal("expected init to refuse on collision")
  	}
  	if !strings.Contains(errb.String(), "refusing to overwrite") {
  		t.Fatalf("stderr = %q", errb.String())
  	}
  	if out.String() != "" {
  		t.Errorf("prompt text emitted before the collision refusal:\n%s", out.String())
  	}
  	if _, err := os.Stat(filepath.Join(root, ".awf")); !os.IsNotExist(err) {
  		t.Errorf(".awf/ should not exist after a probe refusal (err=%v)", err)
  	}
  }
  ```
  Run `go test ./cmd/awf/ -run TestInitCollisionProbeRefusesBeforePrompts` — expected: FAIL
  (`prompt text emitted before the collision refusal`).

- [ ] **Task 4.2 — extract `CollisionsAt`.** In
  `/home/hypno/Projects/agentic-workflows/internal/project/install.go`, replace the body of
  `InitCollisions` (keep its doc comment), old:
  ```go
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
  ```
  new:
  ```go
  func (p *Project) InitCollisions() ([]string, error) {
  	planned, err := p.PlannedOutputs()
  	if err != nil {
  		return nil, err
  	}
  	return CollisionsAt(p.Root, planned), nil
  }

  // CollisionsAt filters planned project-relative paths to those that already
  // exist under root and are not recorded in root's lock (not awf-managed).
  // Split from InitCollisions so init's pre-prompt probe can plan outputs in a
  // throwaway scaffold and test them against the real root; the ADR-0016
  // collision semantics are unchanged.
  func CollisionsAt(root string, planned []string) []string {
  	managed := map[string]bool{}
  	if lock, err := manifest.Load(config.LockPath(root)); err == nil {
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
  	return collisions
  }
  ```
  (`config`, `manifest`, `sort`, `os`, `filepath` are all already imported there.)

- [ ] **Task 4.3 — the probe in `runInit`.** In
  `/home/hypno/Projects/agentic-workflows/cmd/awf/init.go`:

  Insert between the `MergeSetFlags` block and the `initspec.Resolve` call:
  ```go
	// Pre-prompt probe (conservative): refuse collisions before asking a single
	// question or writing anything. The post-answer InitCollisions below stays
	// as the accurate second line — a trim answer can enable non-core artifacts
	// this curated-core probe set does not cover. --force skips the probe.
	if !force {
		collisions, err := probeCollisions(root)
		if err != nil {
			return err
		}
		if len(collisions) > 0 {
			return collisionRefusal(collisions)
		}
	}
  ```

  Replace the existing refusal, old:
  ```go
		return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
			strings.Join(collisions, "\n  "))
  ```
  new:
  ```go
		return collisionRefusal(collisions)
  ```

  Append to the file:
  ```go
  // collisionRefusal is the shared refusal for both collision checks, so the
  // probe and the post-answer check read identically.
  func collisionRefusal(collisions []string) error {
  	return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
  		strings.Join(collisions, "\n  "))
  }

  // probeCollisions computes the collision set before any prompt. With an
  // existing config tree it asks the real project; otherwise it scaffolds a
  // default (curated-core) config into a throwaway temp dir, plans that
  // project's outputs, and tests the project-relative paths against root.
  func probeCollisions(root string) ([]string, error) {
  	if _, err := os.Stat(config.ConfigPath(root)); err == nil {
  		p, err := project.Open(root)
  		if err != nil {
  			return nil, err
  		}
  		return p.InitCollisions()
  	}
  	tmp, err := os.MkdirTemp("", "awf-init-probe-*")
  	if err != nil { // coverage-ignore: MkdirTemp fails only on an unwritable TMPDIR, which a test cannot trigger portably
  		return nil, err
  	}
  	defer os.RemoveAll(tmp)
  	scaffold, err := project.ScaffoldConfig(filepath.Base(root), nil, nil, nil, nil)
  	if err != nil { // coverage-ignore: ScaffoldConfig over the embedded catalog cannot fail at runtime
  		return nil, err
  	}
  	cfgPath := config.ConfigPath(tmp)
  	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: a fresh temp dir's child MkdirAll fails only on a permission fault root bypasses
  		return nil, err
  	}
  	if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write into a fresh temp dir cannot fail in practice
  		return nil, err
  	}
  	tp, err := project.Open(tmp)
  	if err != nil { // coverage-ignore: a freshly-scaffolded default config always opens
  		return nil, err
  	}
  	planned, err := tp.PlannedOutputs()
  	if err != nil { // coverage-ignore: rendering the embedded catalog over a fresh scaffold in an empty tree cannot fail
  		return nil, err
  	}
  	return project.CollisionsAt(root, planned), nil
  }
  ```
  (All needed imports — `os`, `filepath`, `fmt`, `strings`, `config`, `project` — are already
  in `init.go`.)

  Existing tests keep passing: `TestInitGuardBlocksAndForceOverrides` and
  `TestInitRollbackPreservesExistingAwf` now refuse at the probe (no `.awf` was ever
  written, which is what they assert); `TestInitIdempotentReinitNoCollision` exercises the
  config-exists probe branch; `TestInitCollisionsOpenError` exercises the probe's
  `project.Open` error branch; `TestInitAbortsWhenInitCollisionsFails` passes the probe (the
  malformed ADR lives at the real root, not the temp scaffold) and still fails at the
  post-answer check.

- [ ] **Task 4.4 — keep the demoted branches covered.** The reorder strands two branches
  the pre-existing tests used to drive, and `./x gate` fails the 100% bar (ADR-0012)
  without replacements: the post-answer refusal + rollback in `runInit` (both collision
  tests now refuse at the probe instead) and `runInit`'s post-scaffold `project.Open`
  error forward (`TestInitCollisionsOpenError` now errors inside the probe).

  Append to `/home/hypno/Projects/agentic-workflows/cmd/awf/run_test.go`:
  ```go
  // A trim answer can enable a non-core artifact the curated-core probe set does
  // not cover: the probe passes, and the accurate post-answer check still
  // refuses and rolls the scaffolded config back.
  func TestInitPostAnswerCollisionAfterProbePasses(t *testing.T) {
  	root := t.TempDir()
  	skillPath := filepath.Join(root, ".claude", "skills", filepath.Base(root)+"-tdd", "SKILL.md")
  	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(skillPath, []byte("mine\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
  	forceNonInteractive(t)
  	var out, errb bytes.Buffer
  	if code := run([]string{"awf", "init", "--set", "skills=tdd"}, &out, &errb); code == 0 {
  		t.Fatal("expected init to refuse on the post-answer collision")
  	}
  	if !strings.Contains(errb.String(), "refusing to overwrite") {
  		t.Fatalf("stderr = %q", errb.String())
  	}
  	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
  		t.Error("scaffolded config should have been rolled back")
  	}
  }
  ```
  (`tdd` is non-core, so the curated-core probe set does not plan its output path;
  `forceNonInteractive` is defined in `init_test.go`, same package.)

  In `TestInitCollisionsOpenError` (same file), append after the existing failing run,
  inside the function:
  ```go
  	// --force skips the probe, so the same malformed config now fails at
  	// runInit's own post-scaffold project.Open — keeping that branch covered.
  	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code == 0 {
  		t.Fatal("expected init --force to fail when project.Open errors")
  	}
  ```

- [ ] **Task 4.5 — verify and commit.**
  ```
  go test ./cmd/awf/... ./internal/project/...
  ./x gate
  git add cmd/awf/init.go cmd/awf/run_test.go internal/project/install.go
  git commit -m "fix(awf): probe init collisions before prompting"
  ```

---

## Phase 5 — post-init unset-var notes and next steps (test-first)

- [ ] **Task 5.1 — failing test.** Append to
  `/home/hypno/Projects/agentic-workflows/cmd/awf/init_test.go`:
  ```go
  // After the chained sync succeeds, init prints the render-completeness notes
  // (same rendering as awf check, ADR-0045) and a fixed next-steps block.
  func TestInitPrintsNotesAndNextSteps(t *testing.T) {
  	root := t.TempDir()
  	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
  	forceNonInteractive(t)
  	var out, errb bytes.Buffer
  	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
  		t.Fatalf("init: exit %d (%s)", code, errb.String())
  	}
  	for _, want := range []string{
  		"references unset vars",
  		"next steps:",
  		".awf/parts/agents-doc/identity.md",
  		".awf/hooks/",
  	} {
  		if !strings.Contains(out.String(), want) {
  			t.Errorf("init output missing %q:\n%s", want, out.String())
  		}
  	}
  }
  ```
  Run `go test ./cmd/awf/ -run TestInitPrintsNotesAndNextSteps` — expected: FAIL (missing
  `next steps:`).

- [ ] **Task 5.2 — the feature.** In
  `/home/hypno/Projects/agentic-workflows/cmd/awf/init.go`, replace the tail of `runInit`, old:
  ```go
	if err := runSync(root, stdout); err != nil {
		return err
	}
	return nil
  ```
  new:
  ```go
	if err := runSync(root, stdout); err != nil {
		return err
	}
	// Post-init orientation: the same unset-var notes awf check prints
	// (ADR-0045), then a fixed next-steps block.
	np, err := project.Open(root)
	if err != nil { // coverage-ignore: the chained runSync just opened this same tree
		return err
	}
	notes, err := np.UnsetVarNotes()
	if err != nil { // coverage-ignore: RenderAll succeeded moments ago inside runSync
		return err
	}
	for _, n := range notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	fmt.Fprint(stdout, initNextSteps)
	return nil
  ```
  and append to the file:
  ```go
  // initNextSteps is the fixed orientation block init prints after a
  // successful render.
  const initNextSteps = `
  next steps:
    1. Fill the Identity section: edit .awf/parts/agents-doc/identity.md, then run awf sync.
    2. Set any still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf sync.
    3. Wire the rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section) — awf never activates hooks itself.
    4. Commit .awf/ and the rendered files together.
  `
  ```
  (Indentation inside the raw string is literal output — keep it exactly as written.)

- [ ] **Task 5.3 — verify and commit.**
  ```
  go test ./cmd/awf/...
  ./x gate
  git add cmd/awf/init.go cmd/awf/init_test.go
  git commit -m "feat(awf): print unset-var notes and next steps after init"
  ```

---

## Phase 6 — hook-command descriptors stop suggesting unpinned awf

The rendered hook payloads carry a bootstrap shim that runs the *pinned* awf when the
command var is empty (`templates/hooks/*.sh.tmpl`); offering `"awf check"` /
`"awf commit-gate"` as prompt options steers adopters into bypassing that pin with whatever
PATH awf is installed. `activeMdRegenCmd` keeps its `awf sync` option deliberately: it is
quoted in skill prose for an agent to run in a session, not executed by a hook payload, so
the bootstrap-shim pinning story does not apply to it.

- [ ] **Task 6.1 — catalog edits.** In
  `/home/hypno/Projects/agentic-workflows/templates/catalog.yaml`:

  old:
  ```yaml
    - key: checkCmd
      kind: string
      description: Command that checks rendered output for drift.
      default: ""
      options: ["./x check", "awf check"]
  ```
  new:
  ```yaml
    - key: checkCmd
      kind: string
      description: Command that checks rendered output for drift. Leave empty to have the rendered hook payloads run the pinned awf via the bootstrap shim.
      default: ""
      options: ["./x check"]
  ```

  old:
  ```yaml
    - key: commitGateCmd
      kind: string
      description: Command that validates one commit message (the commit-msg hook payload appends the message-file argument).
      default: ""
      options: ["./x commit-gate", "awf commit-gate"]
  ```
  new:
  ```yaml
    - key: commitGateCmd
      kind: string
      description: Command that validates one commit message (the commit-msg hook payload appends the message-file argument). Leave empty to have the payload run the pinned awf via the bootstrap shim.
      default: ""
      options: ["./x commit-gate"]
  ```

- [ ] **Task 6.2 — verify and commit.** Descriptor metadata renders nothing, so no sync is
  needed (confirm: `./x check` → `awf check: clean`).
  ```
  go run ./cmd/awf init --describe | grep -c '"awf check"\|"awf commit-gate"'
  ```
  Expected output: `0` (note `grep -c` exits 1 when it counts 0 matches — that nonzero
  exit is the pass condition here, not a failure).
  ```
  ./x gate
  git add templates/catalog.yaml
  git commit -m "fix(awf): drop unpinned awf from hook-command descriptor options"
  ```

---

## Phase 7 — bare `awf list` shows all seven kinds (test-first)

- [ ] **Task 7.1 — failing test.** Append to
  `/home/hypno/Projects/agentic-workflows/cmd/awf/list_add_test.go`:
  ```go
  // Bare `awf list` covers every kind — the four catalog/domain kinds plus
  // target, bootstrap, and hooks — and an empty kind prints (none) under its
  // header. A single-kind filter still prints only that kind.
  func TestRunListBareShowsAllKinds(t *testing.T) {
  	root := scaffoldedProject(t)
  	var out bytes.Buffer
  	if err := runList(root, "", &out); err != nil {
  		t.Fatalf("list: %v", err)
  	}
  	for _, want := range []string{
  		"skills:", "agents:", "docs:", "domains:\n  (none)",
  		"targets:", "bootstrap:", ".awf/bootstrap.sh", "hooks:", ".awf/hooks/pre-commit.sh",
  	} {
  		if !strings.Contains(out.String(), want) {
  			t.Errorf("bare list missing %q:\n%s", want, out.String())
  		}
  	}
  	out.Reset()
  	if err := runList(root, "skill", &out); err != nil {
  		t.Fatalf("list skill: %v", err)
  	}
  	if strings.Contains(out.String(), "targets:") || strings.Contains(out.String(), "hooks:") {
  		t.Errorf("filtered list must not append the singleton kinds:\n%s", out.String())
  	}
  }
  ```
  Run `go test ./cmd/awf/ -run TestRunListBareShowsAllKinds` — expected: FAIL (missing
  `targets:`).

- [ ] **Task 7.2 — the feature.** In
  `/home/hypno/Projects/agentic-workflows/cmd/awf/list_add.go`, replace the whole `runList`
  function (currently lines 272–333) with:
  ```go
  // listTargets, listBootstrap, and listHooks print the three non-catalog kind
  // blocks; runList shares them between the single-kind filters and the bare
  // all-kinds listing.
  func listTargets(p *project.Project, stdout io.Writer) {
  	fmt.Fprintln(stdout, "targets:")
  	for _, n := range project.KnownTargets() {
  		state := "available"
  		if slices.Contains(p.Cfg.Targets, n) {
  			state = "enabled"
  		}
  		fmt.Fprintf(stdout, "  %-28s %s\n", n, state)
  	}
  }

  func listBootstrap(p *project.Project, stdout io.Writer) {
  	state := "available"
  	if p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled {
  		state = "enabled"
  	}
  	fmt.Fprintln(stdout, "bootstrap:")
  	fmt.Fprintf(stdout, "  %-28s %s\n", ".awf/bootstrap.sh", state)
  }

  func listHooks(p *project.Project, stdout io.Writer) {
  	state := "available"
  	if p.Cfg.Hooks != nil && p.Cfg.Hooks.Enabled {
  		state = "enabled"
  	}
  	fmt.Fprintln(stdout, "hooks:")
  	for _, n := range project.HookNames() {
  		fmt.Fprintf(stdout, "  %-28s %s\n", ".awf/hooks/"+n+".sh", state)
  	}
  }

  func runList(root, kindFilter string, stdout io.Writer) error {
  	if err := gate(root); err != nil {
  		return err
  	}
  	p, err := project.Open(root)
  	if err != nil {
  		return err
  	}
  	switch kindFilter {
  	case "target":
  		listTargets(p, stdout)
  		return nil
  	case "bootstrap":
  		listBootstrap(p, stdout)
  		return nil
  	case "hooks":
  		listHooks(p, stdout)
  		return nil
  	}
  	kinds := project.Kinds()
  	if kindFilter != "" {
  		if _, ok := project.PluralKind(kindFilter); !ok {
  			return unknownKind(kindFilter)
  		}
  		kinds = []string{kindFilter}
  	}
  	for _, kind := range kinds {
  		pl, _ := project.PluralKind(kind)
  		fmt.Fprintf(stdout, "%s:\n", pl)
  		pool, catalogBacked := catalogNames(p.Cat, kind)
  		if !catalogBacked { // domains: configured set only
  			names := slices.Sorted(slices.Values(p.Cfg.Domains))
  			if len(names) == 0 {
  				fmt.Fprintln(stdout, "  (none)")
  			}
  			for _, n := range names {
  				fmt.Fprintf(stdout, "  %-28s %s\n", n, "configured")
  			}
  			continue
  		}
  		for _, n := range pool {
  			fmt.Fprintf(stdout, "  %-28s %s\n", n, artifactState(p, kind, n))
  		}
  	}
  	// Bare list covers every kind: append the non-catalog blocks last.
  	if kindFilter == "" {
  		listTargets(p, stdout)
  		listBootstrap(p, stdout)
  		listHooks(p, stdout)
  	}
  	return nil
  }
  ```
  (This preserves the previous per-filter output byte-for-byte, so
  `TestRunTargetCLI`/`TestRunBootstrapCLI`/`TestRunHooksCLI` assertions keep passing.)

- [ ] **Task 7.3 — verify and commit.**
  ```
  go test ./cmd/awf/...
  ./x gate
  git add cmd/awf/list_add.go cmd/awf/list_add_test.go
  git commit -m "feat(awf): show all seven kinds in bare awf list"
  ```

---

## Phase 8 — `awf help <cmd>` dispatches to per-command help (test-first)

- [ ] **Task 8.1 — failing test.** Append to
  `/home/hypno/Projects/agentic-workflows/cmd/awf/help_test.go`:
  ```go
  // `awf help <cmd>` prints that command's --help text; an unknown command
  // falls back to the top-level overview and exits 0, like bare `awf help`.
  func TestHelpSubcommandDispatch(t *testing.T) {
  	var out, errb bytes.Buffer
  	if code := run([]string{"awf", "help", "sync"}, &out, &errb); code != 0 {
  		t.Fatalf("help sync: exit %d (%s)", code, errb.String())
  	}
  	if out.String() != argSpecs["sync"].help {
  		t.Errorf("awf help sync = %q, want the sync --help text", out.String())
  	}
  	out.Reset()
  	if code := run([]string{"awf", "help", "bogus"}, &out, &errb); code != 0 {
  		t.Fatalf("help bogus: exit %d (%s)", code, errb.String())
  	}
  	if !strings.Contains(out.String(), "Commands:") {
  		t.Errorf("unknown command should fall back to the overview:\n%s", out.String())
  	}
  }
  ```
  (Match this file's existing imports — add `bytes`/`strings` if absent.) Run
  `go test ./cmd/awf/ -run TestHelpSubcommandDispatch` — expected: FAIL (overview printed for
  `help sync`).

- [ ] **Task 8.2 — the feature.** In
  `/home/hypno/Projects/agentic-workflows/cmd/awf/main.go`, old:
  ```go
	if a := args[1]; a == "help" || a == "--help" || a == "-h" {
		fmt.Fprint(stdout, globalHelp())
		return 0
	}
  ```
  new:
  ```go
	if a := args[1]; a == "help" || a == "--help" || a == "-h" {
		if a == "help" && len(args) >= 3 {
			if spec, ok := argSpecs[args[2]]; ok {
				fmt.Fprint(stdout, spec.help)
				return 0
			}
		}
		fmt.Fprint(stdout, globalHelp())
		return 0
	}
  ```

- [ ] **Task 8.3 — verify and commit.**
  ```
  go test ./cmd/awf/...
  ./x gate
  git add cmd/awf/main.go cmd/awf/help_test.go
  git commit -m "feat(awf): dispatch awf help <cmd> to per-command help"
  ```

---

## Phase 9 — document the answers schema, prefix derivation, and Windows pinning

- [ ] **Task 9.1 — `--answers` help text.** In
  `/home/hypno/Projects/agentic-workflows/cmd/awf/main.go`, in the `init` argSpec help, old:
  ```
	  --set k=v      set a value non-interactively (repeatable)
	  --answers FILE read values from a JSON/YAML answers file
  ```
  new:
  ```
	  --set k=v      set a value non-interactively (repeatable)
	  --answers FILE read values from a JSON/YAML answers file: a flat key→value map
	                 of descriptor keys (see --describe); multiselect answers
	                 (skills, docs) are comma-joined name lists
  ```
  (Keep the surrounding backtick-string indentation exactly — these lines are inside the
  raw help literal.)

- [ ] **Task 9.2 — README prefix derivation.** In
  `/home/hypno/Projects/agentic-workflows/README.md`, in the "Adopting into an existing
  repo" bullet list, old:
  ```
  - **Trim to taste** — the curated default is small; grow or shrink it with `awf add`/`remove <kind> <name>` (or edit `.awf/config.yaml` directly).
  ```
  new:
  ```
  - **Trim to taste** — the curated default is small; grow or shrink it with `awf add`/`remove <kind> <name>` (or edit `.awf/config.yaml` directly).
  - **Prefix** — rendered skills are named `<prefix>-<skill>`; `awf init` derives `prefix` from the repo directory's basename. Change it via the `prefix` key in `.awf/config.yaml`, then `awf sync`.
  ```

- [ ] **Task 9.3 — README Windows note.** In the "Pinning with `.awf/bootstrap.sh`"
  subsection, old:
  ```
  It touches nothing outside its cache directory, and `awf remove bootstrap` deletes it if
  you'd rather manage the binary yourself.
  ```
  new:
  ```
  It touches nothing outside its cache directory, and `awf remove bootstrap` deletes it if
  you'd rather manage the binary yourself. The bootstrap and the rendered hook payloads are
  bash scripts and the bootstrap targets the linux/darwin archives — on Windows, install awf
  on `PATH` and run it directly instead of through the pin.
  ```

- [ ] **Task 9.4 — verify and commit.** (README is hand-maintained — outside the render/lock
  set — so no sync.)
  ```
  ./x gate
  git add cmd/awf/main.go README.md
  git commit -m "docs(awf): document answers schema, prefix, and Windows pinning"
  ```

---

## Phase 10 — AGENTS.md Commands placeholder + command dedupe (test-first)

- [ ] **Task 10.1 — failing test.** Append to
  `/home/hypno/Projects/agentic-workflows/internal/project/singleton_test.go` (uses the
  package's `scaffold` helper; add `os`/`filepath`/`strings` imports only if missing):
  ```go
  // agentsDocContent renders the tree and returns AGENTS.md's content.
  func agentsDocContent(t *testing.T, configYAML string) string {
  	t.Helper()
  	p, err := Open(scaffold(t, configYAML))
  	if err != nil {
  		t.Fatal(err)
  	}
  	files, err := p.RenderAll()
  	if err != nil {
  		t.Fatal(err)
  	}
  	for _, f := range files {
  		if f.Path == "AGENTS.md" {
  			return f.Content
  		}
  	}
  	t.Fatal("AGENTS.md not rendered")
  	return ""
  }

  // With no commands data and no command vars, the Commands section renders a
  // self-describing placeholder; identical command values render once.
  func TestAgentsDocCommandsPlaceholderAndDedupe(t *testing.T) {
  	empty := agentsDocContent(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n")
  	if !strings.Contains(empty, "<!-- No commands configured") {
  		t.Errorf("empty Commands section missing the placeholder:\n%s", empty)
  	}
  	dup := agentsDocContent(t, "prefix: example\nvars:\n  testCmd: make test\n  gateCmd: make test\n  checkCmd: make check\nskills: []\nagents: []\n")
  	if got := strings.Count(dup, "- `make test` — run the test suite"); got != 1 {
  		t.Errorf("testCmd line rendered %d times, want 1:\n%s", got, dup)
  	}
  	if strings.Contains(dup, "— run the gate before committing") {
  		t.Errorf("gateCmd identical to testCmd must not render its own Commands line:\n%s", dup)
  	}
  	if !strings.Contains(dup, "`make check` — check rendered files for drift") {
  		t.Errorf("distinct checkCmd line missing:\n%s", dup)
  	}
  }
  ```
  (Assert on the Commands-section line texts, not on a bare `` `make test` `` count —
  `.vars.gateCmd` also renders in the Invariants bullet and the Workflow paragraph of
  `AGENTS.md.tmpl`, so the whole-document count is 3 even after a correct dedupe.)
  Run `go test ./internal/project/ -run TestAgentsDocCommandsPlaceholderAndDedupe` —
  expected: FAIL (no placeholder; a duplicate "— run the gate before committing" line
  renders).

- [ ] **Task 10.2 — the template.** In
  `/home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl`, Commands
  section (the inner triple-backtick lines are literal template content), old:
  ````
  {{ if .data.commands -}}
  ```
  {{ range .data.commands }}{{ .cmd }} — {{ .desc }}
  {{ end -}}
  ```
  {{- else -}}
  {{ with .vars.testCmd }}- `{{ . }}` — run the test suite
  {{ end }}{{ with .vars.gateCmd }}- `{{ . }}` — run the gate before committing
  {{ end }}{{ with .vars.checkCmd }}- `{{ . }}` — check rendered files for drift
  {{ end }}{{- end }}
  ````
  new (the `.data.commands` branch is byte-identical — only the else branch changes; `or X ""`
  normalizes a missing/nil var to a comparable empty string under `missingkey=zero`):
  ````
  {{ if .data.commands -}}
  ```
  {{ range .data.commands }}{{ .cmd }} — {{ .desc }}
  {{ end -}}
  ```
  {{- else -}}
  {{- $test := or .vars.testCmd "" }}{{ $gate := or .vars.gateCmd "" }}{{ $check := or .vars.checkCmd "" -}}
  {{- if or $test $gate $check -}}
  {{ with $test }}- `{{ . }}` — run the test suite
  {{ end }}{{ if and $gate (ne $gate $test) }}- `{{ $gate }}` — run the gate before committing
  {{ end }}{{ if and $check (ne $check $test) (ne $check $gate) }}- `{{ $check }}` — check rendered files for drift
  {{ end }}{{- else }}
  <!-- No commands configured. Set the testCmd/gateCmd/checkCmd vars in .awf/config.yaml (or supply data.commands via an agents-doc sidecar), then run awf sync. -->
  {{ end -}}
  {{- end }}
  ````
  (The placeholder mirrors the Identity section's pattern: self-describing, names the exact
  config surface to edit. If the new unit test flags stray blank lines, adjust only the `-`
  trim markers — never the emitted text.)

- [ ] **Task 10.3 — sync, byte-identical verify, commit.** This repo's AGENTS.md supplies
  `data.commands` via `.awf/agents-doc.yaml`, so its content must not change — only the lock
  re-stamps the template hash:
  ```
  ./x sync && ./x check
  git status --porcelain
  ```
  Expected `git status` output (plus this plan file if not yet committed): exactly
  ```
   M .awf/awf.lock
   M internal/project/singleton_test.go
   M templates/agents-doc/AGENTS.md.tmpl
  ```
  — in particular, `AGENTS.md` must NOT appear (byte-identical). Then:
  ```
  ./x gate
  git add templates/agents-doc/AGENTS.md.tmpl internal/project/singleton_test.go .awf/awf.lock
  git commit -m "feat(awf): agents-doc Commands placeholder and command dedupe"
  ```

---

## Phase 11 — soften the `domains` frontmatter guidance

`awf new adr` scaffolds `domains: []` while the proposing-adr skill calls the field
"≥1 … required" unconditionally — wrong for projects with no configured domains. Align the
prose: fill ≥1 domain key before committing *when the project configures domain docs*;
otherwise leave it empty.

- [ ] **Task 11.1 —
  `/home/hypno/Projects/agentic-workflows/templates/skills/proposing-adr/SKILL.md.tmpl`.**

  Line 31, old (one line — the tail only, shown from `` `domains` ``):
  ```
  `domains` (≥1 coarse domain key — drives the per-domain `{{ .layout.domainsDir }}/<domain>.md` index).
  ```
  new:
  ```
  `domains` (coarse domain keys driving the per-domain `{{ .layout.domainsDir }}/<domain>.md` index — fill ≥1 before committing when the project configures domain docs; otherwise leave `[]`).
  ```

  Line 49, old:
  ```
     - Also fill in every remaining frontmatter array (`supersedes`, `tags`, `related`, `domains`) that `awf new adr` left empty.
  ```
  new:
  ```
     - Also fill in every remaining frontmatter array (`supersedes`, `tags`, `related`, `domains`) that `awf new adr` left empty — `domains` stays `[]` when the project configures no domain docs.
  ```

- [ ] **Task 11.2 —
  `/home/hypno/Projects/agentic-workflows/templates/adr-readme/README.md.tmpl`** (lines
  49–51), old:
  ```
  `domains:` lists the coarse domains this decision belongs to; each one's generated
  `## Decisions` index under `{{ .layout.domainsDir }}/` is built from this field, so set it on every
  ADR (use the project's existing domain names).
  ```
  new:
  ```
  `domains:` lists the coarse domains this decision belongs to; each one's generated
  `## Decisions` index under `{{ .layout.domainsDir }}/` is built from this field. Fill it on every
  ADR when the project configures domain docs (use the existing domain names); otherwise
  leave it `[]`.
  ```
  (The `adr-template` scaffold itself already ships `domains: []` — no change there.)

- [ ] **Task 11.3 — sync, verify, commit.**
  ```
  ./x sync && ./x check
  ```
  Expected re-rendered files: `.claude/skills/awf-proposing-adr/SKILL.md`,
  `.cursor/skills/awf-proposing-adr/SKILL.md`, `docs/decisions/README.md`, `.awf/awf.lock`.
  ```
  ./x gate
  git add templates/skills/proposing-adr/SKILL.md.tmpl templates/adr-readme/README.md.tmpl .claude/skills/awf-proposing-adr/SKILL.md .cursor/skills/awf-proposing-adr/SKILL.md docs/decisions/README.md .awf/awf.lock
  git commit -m "docs(awf): scope domains frontmatter guidance to configured domains"
  ```

---

## Phase 12 — flip ADR-0051 Implemented, domain narrative, `## [0.6.0]` changelog

- [ ] **Task 12.1 — status flip.** In
  `/home/hypno/Projects/agentic-workflows/docs/decisions/0051-single-commit-scope-knob.md`,
  frontmatter, old: `status: Proposed` → new: `status: Implemented`. No other edit
  (append-only ADRs).

- [ ] **Task 12.2 — domain narratives.** Append to the end of
  `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/config/current-state.md` (same
  paragraph, one added sentence at the very end):
  ```
   Commit scopes have one storage (ADR-0051): the `commitScope` var is gone from the catalog; a `commitScopes` init descriptor (`target: audit-scopes`) routes a comma-separated answer into `audit.allowedScopes`, which `awf commit-gate`/`awf audit` and the rendered reviewing-skill prose both consume through `audit.Resolve`, and `awf init` writes the block only for a non-empty answer (nil = accept any).
  ```
  Append to the end of
  `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`:
  ```
   The render context also exposes `.commitScopes` — the display-formatted `audit.allowedScopes` list, empty when accept-any — folded into the config hash of referencing artifacts like `.skills`, so an `audit.allowedScopes` edit reflags exactly the templates that quote it (ADR-0051).
  ```
  (Each is one sentence appended to the existing final paragraph with a leading space, not a
  new paragraph.)

- [ ] **Task 12.3 — changelog entry.** In
  `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`, insert directly above the
  `## [0.5.1] - 2026-07-01` line (grouping per the header convention and ADR-0041 — by
  adopter-facing effect; the batch is everything in `git log v0.5.1..HEAD`; the tag push
  itself remains a user action per docs/releasing.md):
  ```markdown
  ## [0.6.0] - 2026-07-02
  ### Breaking changes
  - The three standard docs (`workflow`, `doc-standard`, `agents-md-standard`) are now mandatory
    always-on singletons instead of toggleable catalog docs; config schema migrates to
    generation 6 (ADR-0043). Run `awf upgrade` after updating.
  - The rendered bootstrap moves off the repo root into the config tree at `.awf/bootstrap.sh`
    (ADR-0047); update any hook or CI reference to the old `awf-bootstrap.sh` path.
  - The `commitScope` var is removed: commit scopes now live only in `audit.allowedScopes`, set
    at init via the comma-separated `commitScopes` answer, quoted by the reviewing skills from
    the same storage `awf commit-gate` enforces, and folded into the drift signal (ADR-0051).
    A leftover var entry is inert; set `audit.allowedScopes` and re-sync to keep the prose.
  ### Features
  - Render three inert git-hook payload scripts (`pre-commit`/`commit-msg`/`pre-push`) under
    `.awf/hooks/` via a `hooks` singleton — enabled by default at init, toggled with
    `awf add/remove hooks`; awf still never touches git config (ADR-0048).
  - Add `awf new adr`, scaffolding the next sequential ADR from the rendered template (ADR-0042).
  - Add `awf changelog` with `--version`/`--since`/`--range` filters over an embedded changelog
    (ADR-0041).
  - `awf add domain` scaffolds the domain's `current-state.md` convention part alongside the
    config edit.
  - The rendered workflow doc gains gate-composition and CI-backstop sections.
  - Every var/data interpolation degrades to coherent generic prose when unset — an empty
    `awf init` renders publication-safe output — and `awf check`/`awf init` print advisory
    notes for referenced-but-unset vars (ADR-0045).
  - `awf check` fails on a rendered reference to a catalog skill outside the enabled set, and
    templates can read the enabled-skill set to conditionalize prose (ADR-0046).
  - Reviewing skills and their reviewer agents are pair-validated: `awf add skill` enables the
    missing agent, `awf remove agent` refuses while an enabled skill requires it, and gated
    commands fail on an unpaired config (ADR-0050).
  - `awf init` refuses collisions before asking a single prompt, prints unset-var notes and a
    next-steps block after rendering, and falls silent when stdin hits EOF instead of
    streaming the remaining prompts.
  - Bare `awf list` shows all seven kinds (including targets, bootstrap, and hooks), and
    `awf help <command>` prints that command's help text.
  - The rendered `AGENTS.md` Commands section shows a self-describing placeholder when no
    commands are configured and de-duplicates identical command values.
  ### Bug fixes
  - Single-source the binary version on `project.Version` so the version gate, lock stamp, and
    bootstrap pin cannot disagree; the bootstrap prefers a matching local binary, prints only
    the binary path on stdout, and falls back to `shasum` where `sha256sum` is missing
    (ADR-0049).
  - Anchor the rendered skill-reference scanner on a token boundary, so prose like
    `example-bootstrap.sh` no longer trips the dead-skill-reference check.
  - Restore the ADR title heading dropped when a project overrides the ADR template's
    sections, and route the generated ACTIVE.md through the canonical generated-by banner.
  ### Others
  - Sweep chain-prose seams, tool-specific vocabulary, and repo residue from the rendered
    templates; hook-command descriptor options no longer suggest unpinned `awf` invocations;
    the `domains` frontmatter guidance now scopes itself to projects with configured domains.

  ```
  (Keep one blank line between this entry and `## [0.5.1]`. `project.Version` is already
  `0.6.0` — bumped by the ADR-0049 work — so no version-const change; docs/releasing.md step
  2's version check is satisfied.)

- [ ] **Task 12.4 — regenerate, full verify, commit.**
  ```
  ./x sync      # regenerates docs/decisions/ACTIVE.md + docs/domains/{config,rendering}.md; expected "awf sync: done"
  ./x check     # expected: "awf check: clean"
  ./x invariants # expected: exit 0, no unbacked-slug findings (0051's three slugs are backed by Phases 1-2 markers)
  ./x gate      # expected: PASS, 100% coverage
  ./x audit     # advisory — review findings; the ADR flip + domain-index regen land in this same commit, satisfying the co-change rule
  git add docs/decisions/0051-single-commit-scope-knob.md docs/decisions/ACTIVE.md docs/domains/config.md docs/domains/rendering.md .awf/domains/parts/config/current-state.md .awf/domains/parts/rendering/current-state.md changelog/CHANGELOG.md .awf/awf.lock
  git commit -m "docs(adr): flip 0051 Implemented and add the 0.6.0 changelog"
  ```
  (If `./x check` re-stamps additional generated files after the narrative edits, stage
  exactly those files too — they will be `docs/domains/config.md`/`rendering.md` content
  changes, already listed.)

---

## Completion checklist

- [ ] `git log --oneline` shows the 12 commits above, each gate-green.
- [ ] `grep -rn "commitScope\b" templates/` → no hits, exit 1 (only `commitScopes` remains,
  which `\b` does not match).
- [ ] `awf init --describe` shows the `commitScopes` descriptor with `"target": "audit-scopes"`.
- [ ] `.awf/config.yaml` has no `vars.commitScope`; rendered reviewing skills read
  "using a Conventional-Commits scope from `adr`, `awf`, `plans`".
- [ ] ADR-0051 status is Implemented and `docs/decisions/ACTIVE.md` reflects it.
- [ ] `changelog/CHANGELOG.md` opens with `## [0.6.0] - 2026-07-02`. Tagging/pushing `v0.6.0`
  is the user's action (docs/releasing.md steps 3–4) — not part of this plan.
