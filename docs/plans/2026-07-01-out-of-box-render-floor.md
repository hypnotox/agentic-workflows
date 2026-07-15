# 2026-07-01: Out-of-box render floor (ADR-0045 + ADR-0046)

**Goal:** implement [ADR-0045](../decisions/0045-out-of-box-render-completeness.md)
(catalog default data, graceful-fallback contract, render-completeness advisory) and
[ADR-0046](../decisions/0046-skill-reference-integrity.md) (enabled-skills render context,
conditional non-core skill references, dead-skill-reference check), plus one ADR-less prose fix
(the `reviewing-adr` terminal-handoff misroute). Design rationale lives in the ADRs; this plan
is execution detail only.

**Architecture summary:** catalog specs gain `Data map[string]any`; a merge helper overlays
sidecar data onto catalog defaults (present key wins, even null) *before* `renderTarget` and
`artifactConfigHash`, so defaults participate in drift. Templates get `{{ with }}` fallbacks at
every unguarded var/data site. `p.data()` gains a `skills` key (effective rendered set);
`artifactConfigHash` folds that set in when the assembled template references `.skills`.
`awf check` gains non-failing unset-var notes and a failing `dead-skill-reference` drift kind.

**Tech stack:** Go 1.26. Packages touched: `internal/catalog`, `internal/config` (read-only),
`internal/project`, `internal/render`, `internal/refs`, `cmd/awf`, `templates/`.

**File structure:**
- Created: `internal/project/datamerge.go`, `internal/project/datamerge_test.go`,
  `internal/project/skillrefs_test.go`, `cmd/awf/initrender_test.go`.
- Modified: `internal/catalog/catalog.go` (+ `catalog_test.go`), `templates/catalog.yaml`,
  `internal/project/render.go`, `internal/project/confighash.go`, `internal/project/check.go`
  (+ its tests), `internal/render/vars.go` (+ test), `internal/refs/refs.go` (+ test),
  `cmd/awf/check.go` (+ test), 13 skill templates, 3 agent templates,
  `templates/agents-doc/AGENTS.md.tmpl`, `.awf/skills/adr-lifecycle.yaml`,
  `.awf/skills/proposing-adr.yaml`, `.awf/agents-doc.yaml`,
  `.awf/domains/parts/rendering/current-state.md`, `.awf/domains/parts/config/current-state.md`,
  both ADRs (status flip), rendered outputs via `./x sync`.
- Deleted: none.

Every phase ends with `./x gate` + `./x check` green and one commit. Run `./x sync` before
`check` whenever a template or `.awf/` file changed.

**Snippet notation (whole plan):** in inline replacement snippets, `\`` denotes a literal
backtick character in the target file; the backslash is markdown-escaping plan notation
only and is never typed into the file.

---

## Phase 1: catalog default-data mechanism (ADR-0045 items 1-2, mechanism only)

- [ ] **1.1 Add `Data` to catalog specs.** In `internal/catalog/catalog.go` add to `SkillSpec`:
  `Data map[string]any \`yaml:"data"\`` (after `Core`), and the same field to `TargetSpec`
  (after `Sections`). Update both doc comments: SkillSpec's gains "Data carries the
  artifact's default render data; sidecars override it per top-level key (ADR-0045).";
  TargetSpec's gains the same sentence.

- [ ] **1.2 Write the merge helper.** New file `internal/project/datamerge.go`:

  ```go
  package project

  import "github.com/hypnotox/agentic-workflows/internal/config"

  // withDefaultData overlays a sidecar onto an artifact's catalog default data:
  // a key absent from the sidecar falls through to the default; a key present in
  // the sidecar (even null or empty) replaces it (the explicit off-switch,
  // ADR-0045). The merged sidecar feeds renderTarget AND artifactConfigHash, so
  // catalog default data participates in the drift signal.
  // invariant: sidecar-key-overrides-default
  func withDefaultData(sc config.Sidecar, defaults map[string]any) config.Sidecar {
      if len(defaults) == 0 {
          return sc
      }
      merged := make(map[string]any, len(defaults)+len(sc.Data))
      for k, v := range defaults {
          merged[k] = v
      }
      for k, v := range sc.Data {
          merged[k] = v
      }
      sc.Data = merged
      return sc
  }
  ```

- [ ] **1.3 Apply the merge at both sidecar-resolution sites.** In
  `internal/project/render.go`:
  - Add a field to `renderKindSpec` (after `gate`): `defaults func(name string) map[string]any`
    with comment `// defaults returns the artifact's catalog default data (nil = none).`
  - In `renderKind`, after the `spec.gate` check and before the `renderTarget` call, insert:

    ```go
    if spec.defaults != nil {
        sc = withDefaultData(sc, spec.defaults(name))
    }
    ```
  - In `RenderAll`, set `defaults` on the three catalog-backed specs:
    - docs spec: no `defaults` field; `DocSpec` carries no `Data` (docs ship full default
      prose already, ADR-0011).
    - skills spec: `defaults: func(n string) map[string]any { return p.Cat.Skills[n].Data }`
    - agents spec: `defaults: func(n string) map[string]any { return p.Cat.Agents[n].Data }`
    - plain-singletons loop: add to the `renderKindSpec` literal:
      `defaults: func(string) map[string]any { return p.Cat.Singletons[sg.kind].Data },`
  - agents-doc direct path: change `data := p.data(ad)` to

    ```go
    ad = withDefaultData(ad, p.Cat.Singletons["agents-doc"].Data)
    data := p.data(ad)
    ```

    and pass the merged `ad` to the existing `renderTarget` call (it already receives `ad`).

- [ ] **1.4 Tests.** New file `internal/project/datamerge_test.go` with table tests for
  `withDefaultData`: (a) nil defaults → sc returned unchanged; (b) key only in defaults →
  present in merged; (c) key in both → sidecar value wins; (d) key present in sidecar with
  nil value → merged value is nil (default suppressed); (e) empty-list sidecar value wins;
  (f) sc.Data nil + non-empty defaults → merged equals defaults. Then a config-hash test
  (in the same file or `confighash_test.go` style): build a `Project` via the existing test
  scaffolding (see `internal/project/project_test.go` helpers), render once, mutate
  `p.Cat.Skills["tdd"].Data` (or any enabled skill) to a different value, render again,
  assert the artifact's `ConfigHash` changed. Tag it:
  `// invariant: catalog-data-in-confighash`.

- [ ] **1.5 Gate + commit.**
  `./x gate && ./x check` → both clean (no catalog data exists yet, so no render changes).
  `git add internal/catalog/catalog.go internal/project/datamerge.go internal/project/datamerge_test.go internal/project/render.go`
  (plus any test file touched) →
  `git commit -m "feat(awf): merge catalog default data under sidecars"` with a body citing
  ADR-0045 items 1-2.

## Phase 2: default content + data-range guards (ADR-0045 items 1, 3-data-half)

- [ ] **2.1 Ship universal defaults in `templates/catalog.yaml`.** Add `data:` blocks:
  - Under `skills: adr-lifecycle:`, the four universal lifecycle states, copied verbatim
    from `.awf/skills/adr-lifecycle.yaml` (`adrStates`: Proposed / Accepted / Implemented /
    Superseded with their exact `meaning`/`mutability` strings; they describe the standard,
    not this repo).
  - Under `skills: proposing-adr:`, `adrSections: [Context, Decision, Invariants,
    Consequences, Alternatives Considered]` (verbatim from `.awf/skills/proposing-adr.yaml`)
    and a *generic* `adrTriggers` list:

    ```yaml
    adrTriggers:
      - Introducing or moving a module/package boundary
      - Adopting a new external dependency
      - Changing a persisted format (config, lock file, schema, API contract)
      - Changing the development workflow's rules
      - Any decision a future maintainer would need to know the "why" for
    ```
  - Under `skills: tdd:`, generic `testSurfaces`:

    ```yaml
    testSurfaces:
      - {name: Unit, kind: fast isolated test, location: beside the code under test}
      - {name: Integration, kind: cross-component test, location: the project's integration suite}
      - {name: End-to-end, kind: full-system test, location: the project's e2e suite}
    ```
  - Under each of `agents: adr-reviewer / plan-reviewer / code-reviewer:`, generic
    `focusItems` (2-3 per agent, e.g. adr-reviewer: decision-clarity ("each Decision item is
    a discrete, implementable commitment"); consequences-honesty ("trade-offs name real
    costs, not straw men")), generic `docCurrencyItems` (e.g. "the change updates every doc
    that states the old behaviour, same commit"), and for code-reviewer generic
    `correctnessTraps` (e.g. "error paths: every returned error is checked or explicitly
    ignored with a reason", "boundary conditions at empty/zero/nil inputs"). **Item shape:**
    exactly what the templates address; `focusItems` entries are `{name, description}` maps,
    `docCurrencyItems` entries are `{check}` maps, `correctnessTraps` entries are
    `{description}` maps (the templates render `.name`/`.description`/`.check`; a plain
    string entry fails render). **Rule:** no
    default may name an awf-repo path or command (`./x`, `docs/decisions/README.md`, Go
    idioms), ADR-0045's generic-defaults contract.
- [ ] **2.2 Shrink awf's own sidecars where they now equal the defaults.** Delete the
  `adrStates` key from `.awf/skills/adr-lifecycle.yaml` (file keeps other keys if any;
  delete the file if `data:` becomes empty) and the `adrSections` key from
  `.awf/skills/proposing-adr.yaml` (keep `adrTriggers`: awf's list is intentionally
  awf-specific). Reviewer sidecars stay untouched (intentional overrides).
- [ ] **2.3 Guard the eleven unguarded data ranges** (the ADR sweep's "nine" undercounted;
  the enumeration below is authoritative) so an explicit sidecar suppression (or a
  yet-unshipped default) still renders coherent prose. Wrap each range *and its
  intro/table-header* in `{{ with }}`, using `{{ range . }}` inside. Exact edits:
  - `templates/skills/tdd/SKILL.md.tmpl:12-13` →

    ```
    {{ with .data.testSurfaces }}{{ range . }}- **{{ .name }}** → {{ .kind }} in `{{ .location }}`
    {{ end }}{{ else }}Pick the smallest surface that can prove the behaviour: unit test beside the code when possible.
    {{ end }}
    ```
  - `templates/skills/adr-lifecycle/SKILL.md.tmpl:16-19`: replace the state table (header
    rows + range) inside the `states` section markers with (byte-identical output when
    `adrStates` is set):

    ```
    {{ with .data.adrStates }}| State | Meaning | Mutability |
    |---|---|---|
    {{ range . }}| `{{ .name }}` | {{ .meaning }} | {{ .mutability }} |
    {{ end }}{{ else }}The lifecycle states are defined in the ADR index (`{{ .layout.adrReadme }}`).
    {{ end }}
    ```
  - `templates/skills/proposing-adr/SKILL.md.tmpl:21-22` (`adrTriggers`): wrap range *and*
    the "Load-bearing triggers include:" intro line directly above it in one `{{ with }}`;
    no else-branch (the preceding sentence "load-bearing decisions only" stands alone).
  - `templates/skills/proposing-adr/SKILL.md.tmpl:32` →
    `- **Required sections:** {{ with .data.adrSections }}{{ range . }}{{ . }}, {{ end }}in that order. {{ else }}follow the ADR template's section order. {{ end }}Delete the authoring checklist before committing.`
  - `templates/agents/adr-reviewer.md.tmpl:58-61`, `plan-reviewer.md.tmpl:58-61`,
    `code-reviewer.md.tmpl:56-58` (`correctnessTraps`), `:59-62` (`focusItems`): wrap each
    `{{ range .data.X }}` block in `{{ with .data.X }}{{ range . }}...{{ end }}{{ end }}` (no
    else; the surrounding lens prose stands alone).
  - `adr-reviewer.md.tmpl:69-70`, `plan-reviewer.md.tmpl:69-70`, `code-reviewer.md.tmpl:70-71`
    (`docCurrencyItems`): same `with`-wrap; wrap the checklist intro sentence ending in
    "AND...:" inside the `with` so no dangling colon renders when suppressed.
- [ ] **2.4 Update golden/spine tests.** `go test ./internal/project/` will fail where
  goldens rendered empty ranges; re-derive expected content (the spine tests' fixtures gain
  the default data output). Follow existing `spine_test.go` / `docs_sections_test.go`
  update conventions; adjust fixtures, never weaken assertions.
- [ ] **2.5 Generic-denylist backstop test (ADR-0045 `inv: catalog-defaults-generic-denylist`).**
  In `internal/catalog/catalog_test.go` add a test that loads the embedded catalog
  (`catalog.Load(templates.FS)`), walks every spec's `Data` value (skills, agents,
  singletons) recursively through nested maps and lists down to the strings, and asserts
  no string contains `./x` or `hypnotox/agentic-workflows`.
  Tag: `// invariant: catalog-defaults-generic-denylist`.
- [ ] **2.6 Sync + commit.** `./x sync` (awf's own adr-lifecycle/proposing-adr renders now
  come from catalog defaults: byte-identical for adrStates/adrSections since the content
  matches; reviewer renders unchanged; lock hashes change). `./x gate && ./x check` clean →
  commit rendered files + catalog + sidecars + templates + tests:
  `feat(awf): ship universal catalog default data`.

## Phase 3: var fallback sweep (ADR-0045 item 3)

Guard every unguarded `{{ .vars.* }}` prose site; the ADR's sweep counted 22; the
enumeration below is authoritative and includes two sites that sweep missed (`bugfix:29`,
`debugging:39`). Fallback conventions: `checkCmd` → literal
`awf check`; `activeMdRegenCmd` → literal `awf sync`; `gateCmd` → "the project's gate";
`testCmd` → "the project's test suite"; `commitScope` → drop the scope clause;
`gateDuration`/`gateCmdFull` → drop the clause. **Backticks move inside the `with` branch**
so no empty code span can render (see the snippet-notation rule in the plan header for
the `\`` convention).

- [ ] **3.1 tdd** `:19` → `2. Run it and confirm it fails for the right reason{{ with .vars.testCmd }}: \`{{ . }}\`{{ end }}.`
  `:21` → `4. Run the gate{{ with .vars.gateCmd }}: \`{{ . }}\`{{ end }}.`
- [ ] **3.2 bugfix + debugging** `bugfix:29` → `4. **Verify via the gates.** {{ with .vars.gateCmd }}\`{{ . }}\`{{ else }}The project's gate{{ end }} (fast tier) is the default.{{ if .vars.gateCmdFull }} Run \`{{ .vars.gateCmdFull }}\` when regression-test placement warrants the full tier.{{ end }}`
  `bugfix:52` → `- Doc-currency rule: if the fix invalidates anything documented in {{ with .vars.docCurrencyTargets }}any of {{ . }}{{ else }}the project's docs{{ end }}, update it in the same commit.`
  `debugging:39`: replace `Then verify: \`{{ .vars.gateCmd }}\` (fast tier).` with
  `Then verify with {{ with .vars.gateCmd }}\`{{ . }}\`{{ else }}the project's gate{{ end }} (fast tier).`
  (the `gateCmdFull` clause on that line is already `if`-guarded).
- [ ] **3.3 adr-lifecycle** `:71` and `:106`: replace `` `{{ .vars.activeMdRegenCmd }}` `` with
  `` `{{ with .vars.activeMdRegenCmd }}{{ . }}{{ else }}awf sync{{ end }}` ``. `:75` →
  `5. **Run the gate{{ with .vars.gateCmd }} (\`{{ . }}\`){{ end }}.** The gate's drift test validates ...` (rest of sentence unchanged).
- [ ] **3.4 proposing-adr** `:61`: the first `` `{{ .vars.checkCmd }}` `` occurrence becomes
  `` `{{ with .vars.checkCmd }}{{ . }}{{ else }}awf check{{ end }}` ``; the final clause
  `` Run `{{ .vars.gateCmd }}` and `{{ .vars.checkCmd }}` to confirm. `` becomes
  `` Run {{ with .vars.gateCmd }}`{{ . }}` and {{ end }}`{{ with .vars.checkCmd }}{{ . }}{{ else }}awf check{{ end }}` to confirm. ``
  (net: "Run `gate` and `check` to confirm." degrades to "Run `awf check` to
  confirm."). `:65` and `:84`: same `activeMdRegenCmd` treatment as 3.3. `:69` →
  `7. **Commit everything in one commit.** Format: \`{{ with .vars.adrProposeCommitFmt }}{{ . }}{{ else }}docs(adr): propose NNNN <short title>{{ end }}\`. ...` (rest unchanged).
- [ ] **3.5 writing-plans** `:37` → `- **Gate cost:** {{ with .vars.gateCmd }}\`{{ . }}\`{{ else }}the project's gate{{ end }}{{ with .vars.gateDuration }} (~{{ . }}){{ end }} runs before every commit. ...` (rest unchanged).
- [ ] **3.6 executing-plans** description `:3`: `... one commit per task, {{ with .vars.gateCmd }}{{ . }} per commit{{ else }}gate-per-commit{{ end }}, ADR status flip ...`.
  `:38` → `   - **Verify** with {{ with .vars.gateCmd }}\`{{ . }}\`{{ else }}the project's gate{{ end }} (fast tier). ...`.
  `:46` (already `if`-guarded block): inside it, apply the same `gateCmd`/`gateCmdFull`
  with-guards. `:50`: `activeMdRegenCmd` treatment as 3.3.
- [ ] **3.7 subagent-driven-development** `:50` →
  `   - **{{ with .vars.gateCmd }}\`{{ . }}\`{{ else }}Gate{{ end }} per commit.** Fast tier by default{{ with .vars.gateCmdFull }}; \`{{ . }}\` for the pre-push tier when a pre-push-only surface is involved{{ end }}. See \`{{ .layout.workflowRef }}\`.`
  `:69`: `activeMdRegenCmd` treatment as 3.3.
- [ ] **3.8 The four reviewing-* skills** (`reviewing-adr:32,45`, `reviewing-plan:36,49`,
  `reviewing-plan-resync:27,40`, `reviewing-impl:37,50`): replace
  `` the `{{ .vars.commitScope }}` scope `` / `` `{{ .vars.commitScope }}` scope `` with
  `{{ with .vars.commitScope }}the \`{{ . }}\` scope{{ else }}the project's commit scope{{ end }}`.
  In `reviewing-impl:50` also guard `gateCmd`:
  `; {{ with .vars.gateCmd }}\`{{ . }}\`{{ else }}the gate{{ end }} passes before each commit`.
- [ ] **3.9 refactor-coupling-audit** `:70` → `grep -rn "{{ with .vars.modulePrefix }}{{ . }}{{ else }}<module-prefix>{{ end }}/<original-package-path>" <original-package-path>/`
- [ ] **3.10 adr-reviewer** `:46`: wrap the final sentence: `{{ with .vars.invariantTestPath }}When a project-level invariant test path is configured, note that each Invariant requires a corresponding test annotation at \`{{ . }}\`.{{ end }}`.
  `:71`: `Regen command: \`{{ with .vars.activeMdRegenCmd }}{{ . }}{{ else }}awf sync{{ end }}\`.`
- [ ] **3.11 AGENTS.md invariants entry (ADR-0045 same-commit obligation).** In
  `.awf/agents-doc.yaml` `data.invariants`, update the publication-safety line to:
  `**Publication-safe templates.** Every interpolation degrades to coherent generic prose when its var/data is unset; no unresolved-value token ever renders. (ADR-0001, ADR-0045)`.
- [ ] **3.12 Update goldens, sync, commit.** Fix `internal/project` golden/spine expectations
  (rendered output for THIS repo is unchanged where vars are set; only templates whose
  goldens render with empty fixture vars change). `./x sync && ./x gate && ./x check` →
  commit templates + `.awf/agents-doc.yaml` + rendered files + tests:
  `feat(awf): graceful fallbacks for every var interpolation`.

## Phase 4: completeness advisory + empty-init regression (ADR-0045 item 4)

- [x] **4.1 Unset-var report.** New method in `internal/project/check.go`:
  `func (p *Project) UnsetVarNotes() ([]string, error)`. As-built (amended during
  implementation): instead of re-assembling per artifact, `RenderedFile` carries the
  assembled template in an unexported `assembled` field set by `renderTarget`, and
  `UnsetVarNotes` walks `RenderAll()` output, the same artifact set by construction,
  with no duplicated enumeration and no second assembly pass. Adapter duplicates are
  collapsed by template id. Compute `render.ReferencedVars(f.assembled)`, keep refs
  whose config value is nil or `""` (`Vars` is `map[string]any`, so a *missing* key
  reads as nil; plain `== ""` misses it), and emit one line per artifact with ≥1 hit:
  `skill proposing-adr references unset vars: activeMdRegenCmd, adrProposeCommitFmt`,
  sorted for determinism.
- [ ] **4.2 Wire into `cmd/awf/check.go`.** After the version-note block, before `p.Check()`:

  ```go
  notes, err := p.UnsetVarNotes()
  if err != nil {
      return err
  }
  for _, n := range notes {
      fmt.Fprintf(stdout, "note: %s\n", n)
  }
  ```

  (No reordering needed: `project.Open` already sits between the version-note block and
  `p.Check()`; insert there. Keep the existing behaviour that notes never affect the
  return value.)
  Tag the non-failing property in the cmd test (4.3):
  `// invariant: completeness-advisory-nonfailing`.
- [ ] **4.3 Tests.** In `cmd/awf` (follow `init_test.go`/`run_test.go` fixtures): a project
  with an enabled skill and an empty `gateCmd` → `runCheck` output contains
  `note: skill ...
  references unset vars: ... gateCmd` AND returns nil (exit 0). A fully-set project → no
  `note:` unset-var lines. Unit-test `UnsetVarNotes` in `internal/project` for the empty-vs-
  missing-var equivalence.
- [ ] **4.4 Empty-init regression test.** New `cmd/awf/initrender_test.go`: drive
  `run([]string{"awf", "init"})` non-interactively in a temp git-less dir (pattern:
  `init_test.go`; `forceNonInteractive`, `getwd` swap), then walk every rendered `.md` under
  the temp root and assert, per file: (a) no broken code span: tokenize each line's
  backtick runs (skipping lines whose trimmed form starts with ```` ``` ````, fences) and
  flag a run of exactly two backticks with no later ≥2-backtick run on the same line: an
  unpaired double-backtick run is the residue of an empty-var span, while legitimate
  *paired* double-backtick spans pass (proposing-adr's rendered invariants rule contains
  one, so a naive substring check for two adjacent backticks would false-positive on it
  and on every fence line); (b) no table
  separator row (`^\|[ :-]+\|`-ish, i.e. a line starting with `|` containing only `-`, `:`,
  `|`, spaces) whose next line does not start with `|`; (c) no line ending in `include:` or
  `sections:** in that order`. Tag: `// invariant: empty-init-coherent-render`. This test
  MUST fail if any Phase-2/3 guard is missing; verify by temporarily reverting one guard
  locally (do not commit the revert).
- [ ] **4.5 Gate + commit.** `./x gate && ./x check` →
  `feat(awf): render-completeness advisory and empty-init regression gate`.

## Phase 5: ADR-0046: skills context, conditional non-core refs, dead-skill-reference check

- [ ] **5.1 Effective-skills set.** In `internal/project` (new small func beside `RenderAll`):

  ```go
  // effectiveSkills returns the skill names whose files exist on disk under awf's
  // model: enabled minus doc-gate-suppressed, keeping local-declared ones.
  // invariant: skills-context-effective-set
  func (p *Project) effectiveSkills() (map[string]bool, error)
  ```

  Implementation: for each name in `p.Cfg.Skills`: read the sidecar; if `sc.Local` →
  include; else if `RequiresDoc` set and doc not enabled → exclude; else include. Return
  `map[string]bool`. Refactor the skills `gate:` closure in `RenderAll` to share the same
  logic (one source of truth for suppression).
- [ ] **5.2 Expose in the template context.** Give `Project` an unexported field
  `effSkills map[string]bool`, populated at the top of `RenderAll` via `effectiveSkills()`;
  in `p.data()` add `"skills": p.effSkills`. No signature changes anywhere: a nil
  `map[string]bool` is lookup-safe in text/template (every key reads false), so the
  domains/bridge/bootstrap call sites that render before/outside `RenderAll` need no
  change. Templates address it as `{{ if .skills.tdd }}` (map key access; missing key →
  false under `missingkey=zero`).
- [ ] **5.3 Hash participation.** In `internal/render/vars.go` add
  ``var skillsRE = regexp.MustCompile(`\{\{[^{}]*[.$]skills[^{}]*\}\}`)`` and
  `func ReferencesSkills(src string) bool { return skillsRE.MatchString(src) }` (+ table
  test in the vars test file). In `artifactConfigHash` (a `*Project` method: no signature
  change; it reads `p.effSkills`), after `proj["vars"] = vs` insert:

  ```go
  if render.ReferencesSkills(assembled) {
      proj["skills"] = sortedSkillNames // sorted []string keys of p.effSkills
      // invariant: skills-set-in-confighash
  }
  ```

  Test: toggle a non-core skill in a fixture project → the AGENTS.md
  artifact's ConfigHash changes; a skill artifact that never references `.skills` keeps its
  hash. (The AGENTS.md assertion goes green only after task 5.4 makes the template
  reference `.skills`; fine, the phase gates once at 5.8.)
- [x] **5.4 Conditionalize non-core references.** *(As-built amendment, recorded in
  ADR-0046 Decision 2: the agent guide's chain prose is additionally gated on
  `.skills.brainstorming` and the task-skills clause anchors on core `adr-lifecycle`;
  the original unconditional chain list made every chain-less config, including
  every minimal test fixture, an illegal state. A partial chain still hard-fails.)*
  - `templates/agents-doc/AGENTS.md.tmpl:55`: replace the trailing task-skills clause with
    (exact replacement; plain conditionals: text/template has no `list`/`append` builtins):

    ```
    **Task skills** (as needed):{{ if .skills.tdd }} `{{ .prefix }}-tdd`,{{ end }}{{ if .skills.bugfix }} `{{ .prefix }}-bugfix`,{{ end }}{{ if .skills.debugging }} `{{ .prefix }}-debugging`,{{ end }} `{{ .prefix }}-adr-lifecycle`.
    ```

    (`adr-lifecycle` is core and unconditional; the clause never renders empty. Trailing
    comma placement keeps the list well-formed for any subset.)
  - `templates/skills/bugfix/SKILL.md.tmpl` `:8`: wrap the companion sentence:
    `{{ if .skills.debugging }}Companion to \`{{ .prefix }}-debugging\`: ...{{ end }}` (keep the
    first sentence unconditional). `:17` →
    `{{ if .skills.debugging }}If the root cause is not yet known, invoke \`{{ .prefix }}-debugging\` first.{{ else }}Confirm the root cause with a falsifiable check before touching code.{{ end }}`
    `:21` → `1. **Ensure a regression test exists that fails for the right reason.** {{ if .skills.tdd }}Invoke \`{{ .prefix }}-tdd\` for the project's test-first discipline: it picks the right surface, writes the failing test, and verifies it fails for the right reason before the fix.{{ else }}Write the failing test first and confirm it fails for the right reason before the fix.{{ end }}`
  - `templates/skills/debugging/SKILL.md.tmpl` `:21` → the parenthetical becomes
    `{{ if .skills.bugfix }}(invoke \`{{ .prefix }}-bugfix\` directly in that case){{ else }}(fix it directly with a regression test in that case){{ end }}`; `:35` →
    `{{ if .skills.tdd }}Invoke \`{{ .prefix }}-tdd\` for the project's test-first discipline.{{ else }}Write it test-first.{{ end }}`; `:41` →
    `{{ if .skills.bugfix }}invoke \`{{ .prefix }}-bugfix\` for the fix + commit + review discipline{{ else }}apply the fix with its regression test and the project's review discipline{{ end }}` (keep the brainstorming clause unconditional: brainstorming is core).
- [ ] **5.5 `reviewing-adr` handoff fix (ADR-less prose).**
  `templates/skills/reviewing-adr/SKILL.md.tmpl:57`, replace the final sentence
  `If no plan exists, the chain proceeds directly to implementation.` with:
  `If no plan exists yet, route by the brainstorm's terminal decision (the ADR settles before the plan is written): invoke \`{{ .prefix }}-writing-plans\` when planning is warranted, else proceed directly to implementation.`
  (No awf ADR number in the rendered prose: templates ship to adopters whose decision
  logs do not contain awf's ADRs; no existing template cites one.)
- [ ] **5.6 Fence-skipping scanner.** In `internal/refs` add
  `func WithoutFences(content string) string`. There is no existing standalone masking
  func: the fence-skipping logic lives inline in `Links`'s line loop; factor that loop
  into the new helper so `WithoutFences` returns the content minus fenced-block lines
  (```` ``` ````/`~~~` delimited, delimiter lines included) and `Links` iterates the helper's
  surviving lines (behaviour-preserving; `Links` keeps applying `stripCodeSpans` per line).
  `WithoutFences` deliberately keeps inline code spans; skill references legitimately
  render inside single-backtick spans. + test. In
  `internal/project/check.go` add:

  ```go
  // checkDeadSkillRefs scans managed rendered markdown for <prefix>-<name> tokens
  // whose <name> is a catalog-known skill outside the effective rendered set
  // (inv: skill-ref-dead-fails). Unknown names are ignored (inv: skill-ref-unknown-ignored).
  // invariant: skill-ref-dead-fails
  // invariant: skill-ref-unknown-ignored
  func (p *Project) checkDeadSkillRefs(files []RenderedFile, amd RenderedFile, dds []RenderedFile, effective map[string]bool) []manifest.Drift
  ```

  Implementation mirrors `checkDeadRefs` (same scan-set assembly). Token scan: regex
  ``regexp.MustCompile(regexp.QuoteMeta(p.Cfg.Prefix) + `-([a-z0-9]+(?:-[a-z0-9]+)*)`)`` over
  `refs.WithoutFences(f.Content)`; for each captured `<name>` (the maximal word-run:
  regex greediness gives whole-token matching per ADR-0046 item 3), if
  `p.Cat.Skills[name]` exists and `!effective[name]` → append
  `manifest.Drift{Path: f.Path, Kind: "dead-skill-reference", Detail: p.Cfg.Prefix + "-" + name}`.
  Dedup per (path, name). Wire into `Check()` right after the `checkDeadRefs` call, passing
  the same `files, amd, dds`.
- [ ] **5.7 Tests.** New `internal/project/skillrefs_test.go`: fixture project where AGENTS.md
  (via a convention part or template) references `<prefix>-tdd` with tdd not enabled →
  exactly one `dead-skill-reference` drift; enable tdd → clean; content `awf-specific` and a
  fenced block containing `awf-tdd` → no findings; `<prefix>-reviewing-plan-resync` with
  resync enabled but `reviewing-plan` disabled → no false substring hit. Also unit-test
  `effectiveSkills` directly (backing `skills-context-effective-set` behaviourally): a
  `local`-declared skill is kept even when a doc gate would suppress it; a doc-gated
  non-local skill with its doc disabled is excluded; a plain enabled skill is included.
  Extend the Phase-4
  init regression test: after init, run `runCheck` → exit nil and zero `dead-skill-reference`
  lines. Tag there: `// invariant: curated-init-skill-refs-clean`.
- [ ] **5.8 Sync, gate, commit.** `./x sync` (AGENTS.md task-skills line changes for THIS repo
  only if tdd/bugfix/debugging are disabled here; they are enabled, so the rendered guide is
  unchanged except hash re-stamps). `./x gate && ./x check` →
  `feat(awf): skill-reference integrity check and skills render context`.

## Phase 6: dogfood narratives, doc currency, status flips

- [ ] **6.1 Domain narratives.** Refresh `.awf/domains/parts/rendering/current-state.md`
  (catalog default-data layer, graceful-fallback contract, skills context) and
  `.awf/domains/parts/config/current-state.md` (sidecar-overrides-default semantics; the
  enable array now feeds the render context and config hash). Keep each to 2-3 added
  sentences, cite ADR-0045/ADR-0046; reference, don't restate (doc-standard).
- [ ] **6.2 AGENTS.md invariants entry for the new check.** In `.awf/agents-doc.yaml`
  `data.invariants` add: `**No dead skill references.** \`awf check\` fails on a rendered
  reference to a catalog skill outside the effective enabled set. (ADR-0046)`.
- [ ] **6.3 Verify invariant backing.** All 10 tagged slugs across both ADRs must be backed:
  `sidecar-key-overrides-default`, `catalog-data-in-confighash`,
  `empty-init-coherent-render`, `completeness-advisory-nonfailing`,
  `catalog-defaults-generic-denylist`, `skill-ref-dead-fails`,
  `skill-ref-unknown-ignored`, `skills-context-effective-set`, `skills-set-in-confighash`,
  `curated-init-skill-refs-clean`. Run `go run ./cmd/awf invariants`: expect the 10 to
  report Unchecked-free once the ADRs flip (next task); fix any missing tag now.
- [ ] **6.4 Status flips + final commit.** Flip both ADRs' `status: Proposed` →
  `Implemented`. `./x sync` (ACTIVE.md + domain-doc indexes regenerate). `./x gate && ./x
  check && go run ./cmd/awf audit` → commit ADR flips + ACTIVE.md + domain docs + agents-doc
  sidecar + rendered files:
  `docs(adr): flip 0045+0046 Implemented (out-of-box render floor)`.
- [ ] **6.5 Plan freeze happens with the flip** (plans freeze when the linked ADR flips:
  no `Implementation complete` header needed).

---

**Verification summary (every phase):** `./x gate` (100% coverage holds), `./x check` clean,
and for phases 2-5 the empty-init regression surface is the oracle: a fresh non-interactive
init must render coherent artifacts and pass check with zero findings and only `note:` lines.
