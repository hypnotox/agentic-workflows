# 2026-07-10 — Adopter config reference: configspec, generated doc, awf config

**Goal:** implement [ADR-0088](../decisions/0088-adopter-config-reference-configspec-authority-generated-doc-and-awf-config-command.md)
(the `internal/configspec` description authority with bidirectional parity; the always-on
generated-index doc `docs/config-reference.md`; the `awf config [<key-or-var>]` command with
a static pre-adoption fallback). Design rationale lives in the ADR — not duplicated here.
The ADR was amended while Proposed (2f296a9): the data-key universe is template-referenced
keys, and the config-reference sidecar rejects `data:` at open.

**Architecture summary:** two feature phases, shaped by the dead-code gate (a production
function must be reachable from `main` in the phase that lands it — `internal/configspec`
cannot land alone, so it lands with its first production consumer, the generated doc).
Phase 1 adds `internal/configspec` (entries, var derivation from `catalog.Vars`, data-key
descriptions), the `config-reference` catalog `DocEntry` (Mandatory + a new `Generated`
flag that keeps it out of `plainSingletons`), the describe-model builder plus
`generateConfigReference` in `internal/project`, sync/check/planned-outputs wiring
(regeneration-checked like domain docs, path-skipped by `checkLockedFiles`), the open-time
`data:` rejection, and the parity/residue/degradation tests. Phase 2 adds `awf config`
(gated inside a tree via the existing per-command `gate(root)` call, static
configspec-only output outside one) and the agent-guide gated-command line. Phase 3 is
docs travel + the status flip. Changelog bullets ride their phases. Every new invariant
slug lands with its backing code: `configspec-key-parity`, `configspec-data-parity`,
`configspec-var-derivation`, `configspec-description-residue`,
`config-reference-regen-drift`, `config-reference-no-bare-vars`,
`config-reference-data-rejected` (P1); `config-command-static-fallback` (P2).

**Tech stack:** Go 1.26, stdlib only (no new dependencies). Packages touched: new
`internal/configspec`; `internal/catalog` (`catalog.go`, `standard.go`),
`internal/project` (new `configreference.go`, plus `singleton.go`, `render.go`,
`project.go`, `check.go`, `validate.go`), `cmd/awf` (new `config.go`, plus `main.go`),
`templates/docs/` (new template), `changelog/`, `.awf/` (agents-doc data, domain
current-state part, glossary part), `docs/decisions` (flip + regen).

**File structure:**

- Created: `internal/configspec/spec.go`, `internal/configspec/spec_test.go`,
  `internal/project/configreference.go`, `internal/project/configreference_test.go`,
  `templates/docs/config-reference.md.tmpl`, `cmd/awf/config.go`, `cmd/awf/config_test.go`,
  `docs/plans/2026-07-10-adopter-config-reference.md` (this plan)
- Modified: `internal/catalog/catalog.go` (DocEntry.Generated), `internal/catalog/standard.go`
  (config-reference entry), `internal/project/singleton.go` (skip Generated),
  `internal/project/render.go` (PlannedOutputs), `internal/project/project.go` (SyncReport),
  `internal/project/check.go` (Check wiring + checkConfigReference),
  `internal/project/validate.go` (config-reference data: rejection),
  `internal/project/unified_doc_model_test.go` (projection pins),
  `cmd/awf/main.go` (commandOrder, usage line, argSpecs, switch), `cmd/awf/help_test.go`
  (global-help pin), `templates/docs/working-with-awf.md.tmpl` (commands + pointer),
  `.awf/agents-doc.yaml` (gated-command line), `.awf/domains/parts/config/current-state.md`,
  `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/glossary/terms.md`,
  `changelog/CHANGELOG.md`, `docs/decisions/0088-*.md` (status flip), plus rendered files
  refreshed by `./x sync`
- Deleted: none

**Phase → ADR Decision map:** P1→D1+D2+D3+D5+D6; P2→D4; P3→doc/flip obligations.
`internal/initspec` is deliberately untouched: var descriptions stay authored in
`catalog.Vars` and configspec *derives* them (ADR-0088 D1), so `awf init --describe` is
behaviorally unchanged by construction — the existing initspec tests are the pin.

**Fixture-fallout rule (applies to every phase):** the new Mandatory doc renders in every
scaffolded fixture, so tests that pin output-file sets, singleton lists, or help text will
move. Update pins only by *adding* the new doc/command (never weaken an assertion).
Known candidates: `internal/project/unified_doc_model_test.go`, `internal/evals`
fixture assertions, `cmd/awf/help_test.go`, and any golden asserting `awf list` output.

---

## Phase 1 — configspec + the generated config-reference doc (ADR-0088 D1, D2, D3, D5, D6)

### 1a. The configspec package

- [ ] Create `internal/configspec/spec.go`. Shape (verbatim signatures; description prose
      is authored here per the rules below):

      ```go
      // Package configspec is the compile-time, adopter-facing description
      // authority for the .awf configuration surface (ADR-0088): every
      // config.yaml key, sidecar field, var, and per-artifact data key an
      // adopter can set. Descriptions are publication prose: they state effect
      // and availability in the adopter's terms — never internal rationale,
      // concrete ADR citations, or repo-identity literals (the residue rules).
      package configspec

      import "github.com/hypnotox/agentic-workflows/internal/catalog"

      // Entry describes one adopter-writable configuration key.
      type Entry struct {
      	Path         string // dotted YAML path: "audit.diffThreshold", "sidecar.sections.<name>.drop"
      	Type         string // value shape as prose: "string", "bool", "string list", "key → value map"
      	Default      string // effective default as prose: `"docs"`, "accept any scope", "none"
      	Description  string // full adopter-voiced description
      	Availability string // when the key has effect: "always", "domain sidecars only", …
      }

      // VarEntry describes one config var. Description text is carried verbatim
      // from the catalog descriptor — the catalog stays the sole var authority;
      // configspec attaches only the availability clause (ADR-0088 D1).
      type VarEntry struct {
      	Key          string
      	Description  string
      	Availability string
      }

      // DataKey describes one adopter-settable sidecar data: key of one artifact.
      type DataKey struct {
      	Kind        string // "skills", "agents", "docs", "agents-doc"
      	Artifact    string // artifact name; "" for the agents-doc singleton
      	Key         string
      	Description string
      }

      func Keys() []Entry      { return keys }
      func VarEntries() []VarEntry { … }
      func DataKeys() []DataKey { return dataKeys }
      ```

- [ ] `keys` covers every adopter-writable leaf of `config.Config` and `Sidecar`
      (`internal/config/config.go`), with sidecar fields under the `sidecar.` prefix.
      The exact path set (the parity test in 1b re-derives it by reflection — this list
      is the hand-authored side):

      ```
      prefix                          docsDir                       vars
      skills                          agents                        docs
      domains                         targets
      invariants.disabled             invariants.sources[].globs    invariants.sources[].marker
      audit.baseBranch                audit.allowedTypes
      audit.allowedScopes[].name      audit.allowedScopes[].meaning
      audit.subjectMaxLength          audit.dependencyManifests     audit.diffThreshold
      audit.domainDocStaleness        audit.domainCodeStaleness
      audit.undocumentedDomain        audit.uncommittedChanges
      bootstrap.enabled               hooks.enabled
      sidecar.data                    sidecar.sections              sidecar.sections.<name>.drop
      sidecar.local                   sidecar.paths
      ```

      Description-authoring rules (enforced by 1b's tests where machine-checkable):
      - Semantics come from the Go doc comments in `internal/config/config.go` and
        `internal/audit` (`audit.Resolve`), rewritten in adopter voice: what the key
        does, what values mean (nil-vs-empty semantics spelled out where they differ —
        `audit.allowedScopes`, `audit.allowedTypes`), and the repair/edit implications.
      - No concrete `ADR-` citation, no `hypnotox`/`agentic-workflows` literals.
      - `Availability` states the consumption condition: `paths` → "domain sidecars
        only — rejected at open on any other kind"; `sidecar.data` → "keys must be
        referenced by the artifact's template; unreferenced keys are failing drift";
        `vars` → "each var is consumed only while an enabled artifact's template
        references it"; plain always-effective keys → "always".
      - `Default` for `audit.subjectMaxLength` and `audit.diffThreshold` must embed the
        numeric defaults from `audit.Resolve(nil)` (pinned by test in 1b); `docsDir` →
        `"docs"`; `targets` → `"claude"`.

- [ ] `VarEntries()` derives from `catalog.Standard.Vars` at call time: one entry per
      descriptor whose `Target` is `""` (the plain config vars — multiselects and the
      audit-scopes descriptor are init routing, not config vars; `commitScopes` is
      described under `audit.allowedScopes[].name` instead). `Description` is the
      descriptor's `Description` string **verbatim**; `Availability` comes from a
      package-level `varAvailability map[string]string` with one hand-authored clause
      per key. The `gateCmd`/`checkCmd` clauses must state the extra part-placeholder
      channel ("also consumed by `{{=awf:gateCmd}}` / `{{=awf:checkCmd}}` placeholders
      in convention parts"); no other var claims placeholder consumption
      (`render.PlaceholderVarRefs` recognizes exactly those two).

      ```go
      // invariant: configspec-var-derivation
      func VarEntries() []VarEntry {
      	var out []VarEntry
      	for _, d := range catalog.Standard.Vars {
      		if d.Target != "" {
      			continue
      		}
      		out = append(out, VarEntry{Key: d.Key, Description: d.Description, Availability: varAvailability[d.Key]})
      	}
      	return out
      }
      ```

- [ ] `dataKeys` covers, per artifact, every `.data.K` key its include-expanded template
      references (ADR-0088 D1 as amended): the three reviewer agents (`focusItems`,
      `docCurrencyItems`, `reviewSubject`, `readStep`, `digestLabel`, `digestSummary`;
      code-reviewer additionally `correctnessTraps`), the data-bearing skills
      (`tdd.testSurfaces`, `proposing-adr.{adrSections,adrTriggers}`,
      `adr-lifecycle.adrStates`), the agents-doc keys, the doc-template keys, and the
      local base templates' keys (kind `skills`/`agents`, artifact `_base`). Do NOT trust
      this prose list — derive the authoritative set with the 1b parity test first, run
      it, and author one description per key it demands. Injected domain keys
      (`domain`, `decisions`) are exempt and get no entry.

- [ ] Create `internal/configspec/spec_test.go` — see 1b.

### 1b. Parity, residue, and defaults tests (all in `internal/configspec/spec_test.go`)

- [ ] **Key parity** (`// invariant: configspec-key-parity`): a reflection walk over
      `config.Config` and `config.Sidecar` (test imports `internal/config` and
      `internal/audit`; production configspec imports neither) computes the expected
      path set, then asserts set-equality against `Keys()` paths — both directions,
      and every entry's `Description`/`Type`/`Availability` non-empty. Walk rules:
      - Only exported fields with a yaml tag; skip `Config.root`/`Config.raw`
        (unexported) automatically via `PkgPath != ""`.
      - Scalar, `*int`/`*bool`, `[]string`, and `map[string]any` fields are leaves
        (`vars`, `sidecar.data` are namespace leaves — their keys are freeform).
      - Pointer-to-struct and struct fields recurse with `<tag>.` prefix
        (`invariants.`, `audit.`, `bootstrap.`, `hooks.`).
      - Slice-of-struct recurses with `<tag>[].` prefix (`invariants.sources[].`,
        `audit.allowedScopes[].` — `ScopeSpec{name, meaning}`).
      - Map-of-struct recurses with `<tag>.<name>.` prefix (`sidecar.sections.<name>.drop`);
        the map itself also stays an entry (`sidecar.sections`).
      - `Sidecar` fields walk under the `sidecar.` prefix.
- [ ] **Data-key parity** (`// invariant: configspec-data-parity`): derive the expected
      `(kind, artifact, key)` set from the catalog + templates FS — for every
      `catalog.Standard` skill/agent (plus `_base` for both kinds), every doc entry
      (toggleable and Mandatory, `agents-doc` included, the Generated config-reference
      itself EXCLUDED — its keys are injected, not adopter-settable), read the template
      via `fs.ReadFile(templates.FS, tid)`, run `render.ExpandIncludes`, and collect
      `render.ReferencedDataKeys` union the artifact's catalog-declared `Data` keys.
      The domain-doc template is skipped entirely (injected `domain`/`decisions`;
      domain sidecars are paths-only). Assert set-equality with `DataKeys()` both
      directions, and every description non-empty.
- [ ] **Var derivation**: assert `VarEntries()` keys equal exactly the `Target == ""`
      descriptors of `catalog.Standard.Vars`, each Description byte-identical to the
      descriptor's, each Availability non-empty; assert `varAvailability` carries no
      key outside that set (a stale clause fails).
- [ ] **Description residue** (`// invariant: configspec-description-residue`): over
      every string in `Keys()`, `VarEntries()`, `DataKeys()` (all fields), assert no
      match for `regexp.MustCompile(`ADR-[0-9]{4}`)` and no occurrence of `hypnotox` or
      `agentic-workflows`.
- [ ] **Defaults pinned to audit**: `Entry("audit.subjectMaxLength").Default` contains
      `fmt.Sprint(audit.Resolve(nil).SubjectMaxLength)`; same for `DiffThreshold`.
- [ ] Run `go test ./internal/configspec/` — the data-key parity failure output is the
      authoritative key list for 1a's `dataKeys`; iterate until green.

### 1c. Catalog entry + render/sync/check wiring

- [ ] In `internal/catalog/catalog.go`, add to `DocEntry` (after `AgentsDoc bool`):

      ```go
      	// Generated marks a Mandatory doc rendered outside RenderAll from computed
      	// project state (the config reference): excluded from plainSingletons and
      	// hash checking, regeneration-checked like ACTIVE.md and the domain docs.
      	Generated bool
      ```

- [ ] In `internal/catalog/standard.go`, add to `Docs` (after `working-with-awf`):

      ```go
      		"config-reference": {Mandatory: true, Generated: true, DocumentMap: true, Title: "Configuration Reference", Desc: "every .awf config key, var, sidecar field, and data key — descriptions, defaults, availability, and this project's live state", Path: "config-reference.md", TemplateKey: "configReference", TID: "docs/config-reference.md.tmpl", Sections: []string{"intro"}},
      ```

- [ ] In `internal/project/singleton.go` `buildPlainSingletons`, extend the skip:

      ```go
      		if e.AgentsDoc || e.Generated {
      			continue
      		}
      ```

- [ ] Create `templates/docs/config-reference.md.tmpl`. One overridable `intro` section;
      every generated block is UNMARKED literal template body (not part-overridable) and
      renders each collection through a `{{ range … }}{{ else }}` with a coherent
      "none configured" line (ADR-0088 D3). Exact template:

      ```
      <!-- awf:section intro -->
      # Configuration Reference

      Every key an adopter can set in `.awf/config.yaml`, artifact sidecars, and
      `vars:` — with its meaning, default, availability, and this project's current
      state. This file is generated per-project by `{{ "{{=awf:checkCmd|awf sync}}" }}`-style
      tooling: edit `.awf/` and re-render; never edit this file. For how to apply
      overrides, see the working-with-awf guide; for ad-hoc queries, run `awf config
      [<key-or-var>]`.
      <!-- awf:end -->

      ## config.yaml keys

      | Key | Type | Default | Current | Description | Availability |
      |---|---|---|---|---|---|
      {{ range .data.configKeys }}| `{{ .path }}` | {{ .type }} | {{ .default }} | {{ .current }} | {{ .description }} | {{ .availability }} |
      {{ else }}| — | | | | No config keys are described; re-render with a current awf. | |
      {{ end }}

      ## Vars

      Each var is consumed only while an enabled artifact references it. State reads:
      **set** (a value), **empty** (present with no value — an open to-do), **absent**
      (deliberately declined; the generic prose renders).

      {{ range .data.varEntries }}- `{{ .key }}` — {{ .description }} {{ .availability }}
        State: {{ .state }}. {{ .consumers }}
      {{ else }}No vars are described for this project.
      {{ end }}

      ## Sidecar fields

      {{ range .data.sidecarFields }}- `{{ .path }}` ({{ .type }}) — {{ .description }} {{ .availability }}
      {{ else }}No sidecar fields are described; re-render with a current awf.
      {{ end }}

      ## Per-artifact data keys

      {{ range .data.dataKeys }}- `{{ .artifact }}` · `data.{{ .key }}`{{ .state }} — {{ .description }}
      {{ else }}No enabled artifact exposes overridable data keys.
      {{ end }}
      ```

      Notes: the intro's placeholder-looking text is escaped prose (the `{{ "…" }}`
      literal idiom the working-with-awf template already uses), so the template
      references no vars and no bare `.vars`/`.data` — only the four dedicated
      `.data.*` collections (`inv: config-reference-no-bare-vars`). Adjust wording
      freely; keep the table/range/else structure and the three state words.

- [ ] Create `internal/project/configreference.go`:
      - `func potentialVarConsumers() (map[string][]string, error)` — for every catalog
        skill/agent/doc + the plain singletons + agents-doc + hook payloads, read the
        RAW template from `templates.FS` (the `NeededVars` precedent, same tid formulas
        via `descriptorByPlural(...).tid` / `plainSingletons` / `hookNames`), collect
        `render.ReferencedVars`, and invert to var → sorted artifact labels
        (`artifactLabel(tid)` for display). Add the include-expansion guard test in
        1d so the raw read stays sound.
      - `func (p *Project) configReferenceData(files []RenderedFile) (map[string]any, error)` —
        builds the four dedicated collections:
        - `configKeys`: one row per `configspec.Keys()` entry with `Path` not starting
          `sidecar.`; `current` renders the configured value from `p.Cfg` (a small
          switch on the path prefix: scalars quoted, lists as `n entries`, nil pointer
          blocks as `(default)`, `vars` as `n keys`).
        - `varEntries`: per `configspec.VarEntries()`: `state` from the three-way map
          presence check on `p.Cfg.Vars` (`set` / `empty — open to-do` / `absent —
          declined`); `consumers` from the enabled files' union
          (`render.ReferencedVars(f.assembled)` + `f.partVarRefs`, labeled via
          `artifactLabel`/`localLabel` exactly as `unusedVarNotes` does) rendered as
          "Consumed by: a, b." or, when empty, "Dormant: no enabled artifact references
          it; enabling X, Y would." from `potentialVarConsumers()` (or "no catalog
          artifact references it." when even that is empty).
        - `sidecarFields`: the `sidecar.`-prefixed `configspec.Keys()` entries.
        - `dataKeys`: per `configspec.DataKeys()` on an ENABLED artifact (or `_base`
          keys when any local artifact of that kind exists; agents-doc always): `state`
          is `" (overridden)"` when the artifact's sidecar sets the key, `" (default)"`
          when the catalog declares a default, `""` otherwise.
      - `func (p *Project) generateConfigReference(files []RenderedFile) (*RenderedFile, error)` —
        reads the `config-reference` sidecar; on `Local` returns `(nil, nil)`; else
        builds `data := p.data(sc)`, overwrites `data["data"]` with the four
        collections map, calls `p.renderTarget("config-reference", "",
        "docs/config-reference.md.tmpl", p.Cat.Docs["config-reference"].Sections, sc,
        data, crefRel(p))`, and returns the domain-doc-style stripped copy (Path,
        Content, assembled, stub/marker/partVarRefs — NO TemplateID/hashes) so
        `checkLockedFiles` treats it as generated.
      - `func (p *Project) crefRel() string` —
        `strings.TrimRight(p.Cfg.DocsDir, "/") + "/" + p.Cat.Docs["config-reference"].Path`.

- [ ] Wire generation into the three producers, mirroring the domain docs exactly:
      - `internal/project/project.go` `SyncReport`: after the `files = append(files, dds...)`
        line: generate (passing the pre-ACTIVE `files` slice captured BEFORE amd/dds were
        appended — bind `rfs := files` right after the frontmatter validation loop), and
        `if cref != nil { files = append(files, *cref) }`.
      - `internal/project/render.go` `PlannedOutputs`: append `cref.Path` when non-nil.
      - `internal/project/check.go` `Check`: compute `crefRel` beside `activeMdRel`;
        pass it to `checkLockedFiles` as a third skip (add a parameter, skip
        `path == crefRel` exactly like `activeMdRel`); after the domain-doc block add:

        ```go
        	cref, err := p.generateConfigReference(files)
        	if err != nil { // coverage-ignore: renderTarget over the embedded reference template cannot fail after RenderAll succeeded
        		return nil, err
        	}
        	drift = append(drift, p.checkConfigReference(lock, crefRel, cref)...)
        ```

        with:

        ```go
        // checkConfigReference regeneration-checks the generated config reference
        // (ADR-0088): like ACTIVE.md and the domain docs, its content depends on
        // config state the lock hashes cannot see.
        // invariant: config-reference-regen-drift
        func (p *Project) checkConfigReference(lock *manifest.Lock, crefRel string, cref *RenderedFile) []manifest.Drift {
        	if cref == nil {
        		if _, ok := lock.Files[crefRel]; ok {
        			return []manifest.Drift{{Path: crefRel, Kind: "orphaned", Detail: "config reference is local:; run awf sync"}}
        		}
        		return nil
        	}
        	return p.regenDrift(crefRel, cref.Content,
        		"config reference absent; run awf sync", "config reference out of date; run awf sync")
        }
        ```

      - Feed the consumption checks: change `p.unusedVarDrift(slices.Concat(files, dds))`
        to include `crefFiles` (a 0/1-element slice), and pass the same concat to
        `p.unusedDataDrift` — the reference's intro part placeholders must count as var
        consumption, and its sidecar (data-rejected at open) stays inert there.
      - `AdvisoryNotes` (`check.go` top): include the generated reference in the
        stub/marker scan the same way `dds` is included (its intro part can be
        stub-marked or carry marker residue like any part).
- [ ] In `internal/project/validate.go`, in the singleton-sidecar validation loop
      (`plainSingletons` — which no longer visits config-reference), add an explicit
      block after it:

      ```go
      	// The config reference's data namespace is injected at generation
      	// (ADR-0088): authored data: would be silently overwritten while its key
      	// names look consumed, so it is rejected like the domain paths-only rule.
      	// invariant: config-reference-data-rejected
      	cr, err := p.Cfg.Sidecar("config-reference", "")
      	if err != nil {
      		return err
      	}
      	if len(cr.Data) > 0 {
      		return errors.New("config-reference: the reference tables are generated — data: has no effect; remove it from .awf/config-reference.yaml (sections:/local: remain available)")
      	}
      	if len(cr.Paths) > 0 {
      		return errors.New("config-reference: paths: is read only from domain sidecars; remove it from .awf/config-reference.yaml")
      	}
      ```

      (Place it so it runs for every gated command at open, matching the ADR-0086
      Decision 5 siblings; exact anchor: immediately after the existing
      `plainSingletons` sidecar loop in `validateAgainstCatalog`.)

### 1d. Phase-1 tests

- [ ] `internal/project/configreference_test.go`:
      - **Golden render** (scaffolded project via the package's existing helpers, `Sync`,
        read `docs/config-reference.md`): contains the banner, the `# Configuration
        Reference` heading, a known key row (`audit.diffThreshold`), a var line with
        `State:` and either `Consumed by:` or `Dormant:`, and the document-map linkage
        (AGENTS.md contains `config-reference.md` — DocumentMap: true).
      - **Empty-state degradation**: a minimal fixture (no domains, all vars empty, no
        data overrides) renders the `{{ else }}` lines — assert "No vars" NOT present
        (vars exist as empty = open to-do) but assert the data-keys else-line renders
        when no enabled artifact has keys; assert no `<no value>`, no empty `| |`-only
        table rows.
      - **Three-way var state**: fixture with one var set, one empty, one deleted →
        the three state strings render respectively; the absent one shows `absent`.
      - **Dormant hint**: disable an artifact that alone references a var (leave the
        var set) → its entry shows `Dormant:` naming the artifact; NOTE this fixture
        will also trip `unused-var-drift` in Check — assert via the render output, not
        a clean Check.
      - **Regen drift** (`config-reference-regen-drift` backing): hand-edit the
        rendered file → `Check` reports `stale` at the doc's path; delete it →
        `missing`; set `local: true` in `.awf/config-reference.yaml` after a sync →
        `orphaned`.
      - **No-bare-vars** (`// invariant: config-reference-no-bare-vars`): assert
        `!render.ReferencesBareVars(cref.assembled)` and
        `!render.ReferencesBareData(cref.assembled)` on the generated file, plus a
        guard: no file under `templates/partials/` matches `\.vars\.` (the
        include-expansion guard ADR-0088 D5 requires).
      - **Data rejection** (`config-reference-data-rejected` backing): sidecar with
        `data:\n  k: v\n` → `Open` fails naming `.awf/config-reference.yaml`; with
        `paths:` → the paths message; with only `sections:\n  intro:\n    drop: true\n`
        → opens and renders without the intro.
      - **Intro override**: part at `.awf/parts/config-reference/intro.md` replaces the
        intro body; the generated tables still render (unmarked body is not
        overridable).
- [ ] Update `internal/project/unified_doc_model_test.go`: the pinned projections gain
      `config-reference` in `SingletonKinds()`; `plainSingletons` must NOT contain it;
      `templateMap()` gains `configReference` → `docs/config-reference.md`.
- [ ] Run `go test ./...`; repair remaining pinned-set fallout per the fixture-fallout
      rule (evals fixture, golden lists). awf's own tree: run `go run ./cmd/awf sync`
      and stage the newly rendered `docs/config-reference.md`, the refreshed
      `AGENTS.md`/`.awf/awf.lock`, and any doc the document-map line changed.
- [ ] Add to `changelog/CHANGELOG.md` under `## [Unreleased]` / `### Added`:

      ```markdown
      - `docs/config-reference.md`: a generated, always-on configuration reference —
        every config key, var, sidecar field, and per-artifact data key with full
        descriptions, defaults, availability, and the project's live state (which
        vars are set/empty/absent, what consumes them, what enabling would activate).
        Regeneration-checked like the domain docs; the intro section is overridable,
        the generated tables are not, and `data:` on its sidecar refuses at open.
      ```

- [ ] Run `./x gate` — green. Commit:
      `feat(rendering): add configspec authority and generated config reference (ADR-0088)`.

## Phase 2 — the `awf config` command (ADR-0088 D4)

- [ ] Create `cmd/awf/config.go`:
      - `runConfig(cwd, key string, stdout io.Writer) error`. Project detection: if
        `os.Stat(config.ConfigPath(cwd))` reports not-exist → STATIC mode
        (`// invariant: config-command-static-fallback`): print the catalog-wide
        reference (descriptions, defaults, availability, catalog-wide potential
        consumers via `project.PotentialVarConsumers()` — export the Phase-1 helper)
        with a first line `config reference (static — not inside an awf project; live
        state appears inside one)`; exit nil (code 0).
      - LIVE mode: `gate(root)` runs in main.go's switch (see wiring below) — inside
        `runConfig` call `project.Open(cwd)` then build the same collections the doc
        uses (export a `func (p *Project) ConfigReferenceModel() (…)` wrapper around
        `configReferenceData` + `RenderAll`) and print them as text: one `## config.yaml
        keys` block (path, current, description, availability per key), `## Vars`
        (state + consumers/dormant), `## Sidecar fields`, `## Data keys`.
      - Single-key mode (`key != ""`): case-sensitive exact match against, in order:
        config-key paths, var keys, sidecar field paths, then data-key names (a data
        key prints every artifact carrying it). No match →
        `fmt.Errorf("unknown key or var %q; run `+"`awf config`"+` for the full reference", key)`
        (exit 1 — a valid CLI shape, not a usageErr).
- [ ] Wire `cmd/awf/main.go`:
      - `commandOrder`: insert `"config"` after `"list"`.
      - The bare-usage stderr line in `run()`: add `config` after `list` in the
        `<init|sync|…>` enumeration.
      - `argSpecs["config"]`:

        ```go
        	"config": {
        		maxPos: 1, summary: "Describe config keys and vars (live state inside a project)",
        		help: `Usage: awf config [<key-or-var>]

        Print the configuration reference: every config key, var, sidecar field, and
        data key with descriptions, defaults, and availability. Inside an awf project
        the output adds live state (current values; which enabled artifacts consume
        each var; dormant hints). Outside one, a static catalog-wide reference prints.
        With an argument, print just that entry (a config key path like
        audit.diffThreshold, a var name like gateCmd, a sidecar field like
        sidecar.local, or a data key name).
        `,
        	},
        ```

      - Switch case (gate only when inside a project — mirror how other gated commands
        run `gate` via their run funcs; here the command must NOT refuse outside a
        tree):

        ```go
        	case "config":
        		key := ""
        		if len(args) >= 3 {
        			key = args[2]
        		}
        		cmdErr = runConfig(cwd, key, stdout)
        ```

        and inside `runConfig`'s live branch, first line: `if err := gate(root); err != nil { return err }`
        (static branch never gates — there is no project to be behind).
- [ ] `cmd/awf/config_test.go` (follow `run_test.go` patterns): static mode in an empty
      dir (exit 0, contains `static`, contains `gateCmd`); live mode in a scaffolded+
      synced fixture (contains `Consumed by:`/`Dormant:`/current values); single-key hit
      for a config key, a var, a sidecar field, and a data key; unknown key → exit 1
      with the message; behind-schema fixture → gate error (live only); `--help` prints
      the help text.
- [ ] Update `cmd/awf/help_test.go` for the new global-help line and usage enumeration.
- [ ] `.awf/agents-doc.yaml`: in the Binary-version-gate invariant text, extend the
      gated-command enumeration to `(sync, check, invariants, audit, list, config, add,
      remove, new)` and append: `config degrades to a static catalog reference outside
      an adopted tree instead of refusing.` Run `go run ./cmd/awf sync`; stage AGENTS.md
      + lock + the refreshed config reference (its content does not change here, but the
      sync may re-stamp).
- [ ] Add to `changelog/CHANGELOG.md` under `### Added`:

      ```markdown
      - `awf config [<key-or-var>]`: print the configuration reference from the CLI —
        full reference or a single entry, with live state inside a project (current
        values, consumers, dormant hints) and a static catalog-wide fallback outside
        one for pre-adoption discovery.
      ```

- [ ] Run `./x gate` — green. Commit: `feat(awf): add awf config command (ADR-0088)`.

## Phase 3 — docs travel + status flip (ADR-0088 doc obligations)

- [ ] `templates/docs/working-with-awf.md.tmpl`:
      - `commands` section: add `awf config [<key-or-var>]` with a one-line description
        (mirror the argSpec summary) to the command list.
      - `config-and-overrides` section: add one sentence pointing at the generated
        reference: every key/var/data key is described in the configuration reference
        doc (link `config-reference.md` relative) and via `awf config`.
- [ ] `.awf/docs/parts/architecture/components.md`: add `internal/configspec` (the
      description authority + parity) and the generated config reference + `awf config`
      command to the component narrative, in this repo's own voice.
- [ ] `.awf/domains/parts/config/current-state.md`: append the ADR-0088 state — the
      configspec authority, the generated reference doc, the `awf config` command, the
      sidecar `data:` rejection.
- [ ] `.awf/docs/parts/glossary/terms.md`: add terms **configspec** (the compile-time
      adopter-facing description authority) and **config reference** (the generated
      always-on doc), if the glossary's shape fits single-line term entries.
- [ ] Run `go run ./cmd/awf sync` (regenerates working-with-awf, architecture, the
      domain doc, glossary, ACTIVE.md); verify with `go run ./cmd/awf check` — clean.
- [ ] Flip `docs/decisions/0088-*.md` frontmatter `status: Proposed` → `status:
      Implemented`; run `./x sync` to regenerate ACTIVE.md; confirm
      `go run ./cmd/awf invariants` reports no unbacked slug (all eight ADR-0088 slugs
      must have their backing comments from P1/P2).
- [ ] Add to `changelog/CHANGELOG.md` under `### Changed` (docs-travel bullet only if
      the working-with-awf/AGENTS.md wording changes warrant one; otherwise skip).
- [ ] Run `./x gate` — green. Commit:
      `docs(adr): flip 0088 to Implemented — config reference shipped`
      (body: names the two features, the amendment, and the docs-travel set).

---

**Deferred (recorded in ADR-0088 Consequences, not this plan):** man page and JSON
Schema projections; folding `awf init --describe` into `awf config`.
