---
date: 2026-07-12
adrs: [100, 101]
status: Proposed
---
# Plan: Managed Command Runner via In-Place-Editable Sections

Implements the **in-place-editable-sections rendering primitive** (ADR-0100) and its first
consumer, the **managed command runner singleton** (ADR-0101). The design lives in those two
ADRs; this plan is the execution record only.

## Goal

Ship a new class of co-owned rendered file whose designated sections the adopter edits directly in
the output (preserved across syncs while awf owns the rest), and use it to render a command-runner
file `x` whose awf-verb dispatch is awf-owned and whose project verbs are adopter-owned. awf-the-
repo opts out; `examples/sundial` adopts it and lands zero advisory notes (ADR-0090).

## Architecture summary

Build the primitive first (render + project + check layers), then the runner consumer:

1. Parse an `inplace` section-marker attribute and emit a fourth `awf:edit-in-place` provenance
   pointer (render layer, mirroring the existing `stub` attribute).
2. Add a first-class `RegenChecked` attribute on `RenderedFile` and migrate the triplicated
   hardcoded generated-index exclusion onto it (behaviour-preserving refactor).
3. Source an in-place section's body by reading it back from the existing output file, bounded by
   awf's next *registered* section pointer or EOF (project layer).
4. Drift-check in-place files by regeneration-with-read-back (compare on-disk to the freshly
   rendered content, not the frozen `OutputHash`).
5. Add the `runner` config toggle, a dedicated render block (not a catalog `DocEntry`), and the
   `x` template with two `awf:edit-in-place` sections.
6. Enable + migrate the `examples/sundial` runner; awf-the-repo stays opted out.
7. Back all twelve invariant slugs (per phase) and flip both ADRs to Implemented in the final commit.

## File structure

- **Created:** `templates/runner/x.tmpl`; new test files as needed (`internal/render/*_test.go`
  additions, `internal/project/runner_test.go`, `internal/project/inplace_test.go`).
- **Modified:** `internal/render/section.go`, `internal/render/render.go`,
  `internal/project/render.go`, `internal/project/check.go`, `internal/project/project.go`,
  `internal/config/config.go`, `examples/sundial/.awf/config.yaml`, docs/domain parts,
  `docs/decisions/0100-*.md`, `docs/decisions/0101-*.md` (status flip).
- **Deleted:** `examples/sundial/x` (hand-written; replaced by the rendered `x`).

---

## Phase 1 — In-place section parsing and the `awf:edit-in-place` pointer

Backs `inv: in-place-pointer-distinct`. Adds the render-layer parse + pointer + splice, tested with
synthetic in-place sections (no runner template yet).

- [ ] **Task 1.1 — Parse the `inplace` section-marker attribute.** In
  `internal/render/section.go`: extend `sectionRE` (line 24) so the optional attribute group
  recognises `inplace` as well as `stub`, mutually exclusive:
  ```
  var sectionRE = regexp.MustCompile(`(?s)<!-- awf:section (\S+)( stub| inplace)? -->\n(.*?)\n?<!-- awf:end -->`)
  ```
  Add `InPlace bool` to `Segment` (after `Stub`, line 15) with a doc comment ("declared by the
  `inplace` marker attribute (ADR-0100); its body is read back from the rendered output"). In
  `ParseSections` set `InPlace: strings.TrimSpace(src[m[4]:m[5]]) == "inplace"` (guard `m[4] >= 0`),
  keeping `Stub: m[4] >= 0 && strings.TrimSpace(src[m[4]:m[5]]) == "stub"`. Update the `sectionRE`
  comment (lines 18–23) to name both legal attributes.
- [ ] **Task 1.2 — Test the parse.** In `internal/render/section_test.go` add cases: `awf:section
  foo inplace` → `InPlace==true, Stub==false`; `awf:section foo stub` → `Stub==true,
  InPlace==false`; `awf:section foo` → both false; an unknown attribute (`awf:section foo bogus`)
  still fails to parse and is caught by `CheckResidualMarkers` (assert `residualMarkerRE` matches).
  Verify: `go test ./internal/render/ -run TestParseSections -count=1` → `ok`.
- [ ] **Task 1.3 — Add the fourth `editPointer` variant + `Assemble` splice.** In
  `internal/render/render.go`: add `InPlace bool` and `InPlaceBody string` to `SectionPlan`
  (struct at lines 15–35; document that `InPlaceBody` is the content read back from the existing
  output, empty on first render). Add the 4th branch to `editPointer` (lines 42–50), first, so it
  takes precedence:
  ```go
  if p.InPlace {
      return fmt.Sprintf("<!-- awf:edit-in-place %s — your edits below are preserved across syncs; awf owns the rest -->\n", name)
  }
  ```
  In `Assemble` (lines 73–93), when `p.InPlace`, emit the pointer then `p.InPlaceBody` if non-empty
  else the template default `s.Text` (verbatim interior — no re-templating):
  ```go
  b.WriteString(editPointer(s.Name, s.Stub, p))
  switch {
  case p.InPlace:
      if p.InPlaceBody != "" { b.WriteString(p.InPlaceBody) } else { b.WriteString(s.Text) }
  case p.HasPart:
      writePartBody(&b, parts, s, p)
  default:
      b.WriteString(s.Text)
  }
  ```
  The `awf:edit-in-place` token must not trip `no-section-marker-leak`/`no-residual-section-marker`
  (`residualMarkerRE` anchors on `awf:(section|end)\b`, so `awf:edit-in-place` is safe — assert in
  the test). **Backing marker placement (ADR-0105, test-scoped):** this repo sets
  `invariants.testGlobs: ['**/*_test.go']`, so a proof `// invariant: <slug>` marker backs a slug
  ONLY when it opens a line inside a `*_test.go` file matching that glob — a production-site marker
  is out of scope and does not back. Put the proof `// invariant: in-place-pointer-distinct` on a
  line inside the Task 1.4 test in `internal/render/render_test.go` (not at the production branch);
  optionally add an advisory `// touches-invariant: in-place-pointer-distinct — <note>; proof in
  render_test.go` at the production site, mirroring the existing `touches-invariant:` markers at
  render.go:41/72/167. Leave `editPointer`'s existing `section-edit-pointer` marker unchanged.
- [ ] **Task 1.4 — Test the pointer + splice.** In `internal/render/render_test.go`: `Assemble`
  with an in-place section and a non-empty `InPlaceBody` emits the `awf:edit-in-place` pointer then
  the body verbatim (including an internal blank line); with empty `InPlaceBody` emits the template
  default; `CheckResidualMarkers` on the assembled output returns nil (no leak). Verify:
  `go test ./internal/render/ -count=1` → `ok`.
- [ ] **Task 1.5 — Verify and commit.** `./x gate`; `git add internal/render/section.go
  internal/render/render.go internal/render/section_test.go internal/render/render_test.go`;
  commit `feat(rendering): parse inplace attribute and edit-in-place pointer` (≤72 chars).

## Phase 2 — `RegenChecked` attribute and generated-index exclusion migration

Backs (partially) `inv: regeneration-checked-attribute`. A behaviour-preserving refactor: move the
triplicated hardcoded generated-index test onto one attribute. No new drift behaviour yet.

- [ ] **Task 2.1 — Add the attribute and a single predicate.** In `internal/project/render.go` add
  `RegenChecked bool` to `RenderedFile` (struct lines 38–61; doc: "excluded from the frozen-
  OutputHash compare; drift is checked by regeneration — ADR-0100"). Set `RegenChecked: true` on the
  generated indexes where their `RenderedFile`s are built: `generateActiveMD`, `generateDomainDocs`,
  `generateConfigReference` (the functions producing `amd`, `dds`, `cref` — called at check.go:489/
  495/501 and in `SyncReport`). Add one helper, e.g. `func regenCheckedPaths(files ...[]RenderedFile)
  map[string]bool`, or thread the flag through the lock as below.
- [ ] **Task 2.2 — Migrate `checkLockedFiles` (check.go:566).** Replace the hardcoded condition
  at line 574 (`path == activeMdRel || path == crefRel || strings.HasPrefix(path, domainsPrefix)`)
  with an attribute-derived membership test built from the regen-checked `RenderedFile` set that
  `Check()` already assembles (`amd`, `dds`, `cref`). The three `activeMdRel/domainsPrefix/crefRel`
  params (derived at check.go:469–471) are removed once nothing else needs them, or kept if the
  regen checkers still take them — minimise the signature churn, but the *exclusion decision* must
  read the attribute, not the literals.
- [ ] **Task 2.3 — Migrate `isGeneratedIndex` (project.go:276–279) and the `TemplateHash == ""`
  heuristic (project.go:257–260).** Route both through the same attribute/predicate so the "is this
  regeneration-checked?" decision has one source of truth. `isGeneratedIndex(rel)` becomes a lookup
  in the regen-checked path set (or an `e.RegenChecked` field if you add one to `manifest.Entry` —
  see Task 2.4). The `case e.TemplateHash == ""` provenance arm stays behaviourally identical.
- [ ] **Task 2.4 — Decide the lock representation (design note, then implement).** The exclusion in
  `checkLockedFiles` iterates `lock.Files` keys; generated indexes are in `lock.Files` but not in
  `RenderAll`'s `rendered` map, so the attribute must be reachable at lock-iteration time. Preferred:
  add `RegenChecked bool` to `manifest.Entry` (manifest.go:14–19), set from `RenderedFile.RegenChecked`
  in `SyncReport` (project.go:183–186), and read it in `checkLockedFiles`/`isGeneratedIndex`. This
  removes the path literals entirely. (If a lock-schema field is judged too heavy, derive the set
  from the freshly generated `RenderedFile`s in `Check`; note the choice in the commit body.)
- [ ] **Task 2.5 — Test the migration is behaviour-preserving + attribute-driven.** Existing
  `internal/project` check tests stay green (no drift-classification change). Add a test asserting
  the generated indexes carry `RegenChecked==true` and a non-generated rendered file carries
  `false`, and that a hand-edited generated index is still *not* flagged `hand-edited` (it is a
  regen `stale`, via the separate checkers). Verify: `go test ./internal/project/ -count=1` → `ok`.
  Put the proof `// invariant: regeneration-checked-attribute` on a line inside this test (a
  `*_test.go` file — the only backing scope under `invariants.testGlobs`), not at the production
  predicate; optionally add an advisory `// touches-invariant: regeneration-checked-attribute` at
  the predicate. (Phase 4 completes this slug's coverage for the in-place, in-`rendered` case.)
- [ ] **Task 2.6 — Verify and commit.** `./x gate`; `git add internal/project/render.go
  internal/project/check.go internal/project/project.go internal/manifest/manifest.go
  internal/project/*_test.go`; commit `refactor(rendering): first-class regeneration-checked
  attribute` (≤72 chars).

## Phase 3 — In-place read-back sourcing (project layer)

Backs `inv: in-place-readback`, `inv: section-source-exclusive`, `inv: in-place-spacing-owned`.

- [ ] **Task 3.1 — Read-back extraction helper.** In `internal/project/render.go` add a helper that,
  given the existing output file bytes and the ordered list of the file's section pointers (the
  registered section names, in template order), returns the current body of a named in-place
  section: the text from just after that section's `awf:edit-in-place <name>` pointer line to the
  line that is awf's **next registered** section pointer (`awf:edit `/`awf:edit-in-place ` for the
  next section name), or EOF if last. Match the boundary by the *expected next pointer string*
  computed from the section registry — never "any pointer-shaped line" — so adopter content
  containing such a line does not truncate (ADR-0100 Decision 2). Trim only leading/trailing blank
  lines (awf-owned framing); the interior is returned verbatim.
- [ ] **Task 3.2 — Wire read-back into `planSections` (render.go:159–191).** `planSections` (and its
  caller `renderTarget`, render.go:474–524) gain access to the output path. For a `Segment` with
  `InPlace==true`: do **not** read a `.awf/parts/` part; instead read the existing output file at
  `filepath.Join(p.Root, outPath)` (absent → `InPlaceBody=""`), extract the section body via Task
  3.1's helper, and set `SectionPlan{InPlace: true, InPlaceBody: body}`. If a convention part file
  *also* exists for an in-place section, return a hard error naming the section
  (`section-source-exclusive`).
- [ ] **Task 3.3 — Tests.** In `internal/project/inplace_test.go`: (a) read-back extracts the exact
  interior between pointers, internal blank lines preserved; (b) absent output → default; (c) a
  boundary test where the in-place body contains a line resembling `<!-- awf:edit next — … -->` for
  a *non-registered* name and is NOT truncated; (d) leading/trailing blank lines are trimmed
  (framing owned); (e) a part file present for an in-place section errors. Verify:
  `go test ./internal/project/ -run 'InPlace' -count=1` → `ok`. Put the proof `// invariant:
  <slug>` markers for the three slugs (`in-place-readback`, `section-source-exclusive`,
  `in-place-spacing-owned`) on lines inside the tests in `internal/project/inplace_test.go` — the
  backing scope under `invariants.testGlobs` — not at the implementing functions; add advisory
  `// touches-invariant: <slug>` markers at the production sites if useful.
- [ ] **Task 3.4 — Verify and commit.** `./x gate`; `git add internal/project/render.go
  internal/project/inplace_test.go`; commit `feat(rendering): read in-place section bodies back
  from rendered output`.

## Phase 4 — Regeneration-with-read-back drift for in-place files

Backs `inv: in-place-tamper-drift` and completes `inv: regeneration-checked-attribute` (the
in-place, in-`rendered` case).

- [ ] **Task 4.1 — Mark in-place files regeneration-checked.** In `renderTarget`/`RenderAll`, a
  `RenderedFile` that contains at least one in-place section sets `RegenChecked=true` (so it skips
  the frozen-`OutputHash` compare). Because such a file *is* in `RenderAll`'s `rendered` map (unlike
  the generated indexes), extend `checkLockedFiles` so a `RegenChecked` file that IS present in
  `rendered` is compared on-disk against the freshly rendered `rf.Content` (which already read the
  in-place body back) — `if manifest.Hash(onDisk) != manifest.Hash([]byte(rf.Content)) → hand-edited`
  — rather than against `e.OutputHash`. A `RegenChecked` file NOT in `rendered` keeps the Phase-2
  behaviour (skip; handled by the separate regen checkers).
- [ ] **Task 4.2 — Tests (synthetic in-place file).** In `internal/project`: construct a rendered
  file with an in-place section, sync it, then (a) edit the in-place interior → `check` reports no
  drift (read-back matches); (b) edit an awf-owned region → `check` reports `hand-edited`/drift and
  the next sync overwrites it; (c) idempotence: a second `sync` with no edit is a no-op and `check`
  is clean (fixpoint). Verify: `go test ./internal/project/ -count=1` → `ok`. Put the proof
  `// invariant: in-place-tamper-drift` on a line inside this test (a `*_test.go` file — the backing
  scope under `invariants.testGlobs`), not at the production compare site.
- [ ] **Task 4.3 — Verify and commit.** `./x gate`; `git add internal/project/check.go
  internal/project/render.go internal/project/*_test.go`; commit `feat(rendering): regeneration-
  with-read-back drift for in-place files`.

## Phase 5 — The runner singleton

Backs `inv: runner-singleton-toggle`, `inv: runner-awf-verbs-owned`, `inv: runner-project-verbs-in-place`,
`inv: runner-render-publication-safe`, `inv: singleton-kinds-complete`.

- [ ] **Task 5.1 — `RunnerConfig` toggle.** In `internal/config/config.go` add, mirroring
  `HooksConfig` (type at line 107; `BootstrapConfig` is at 96): `type RunnerConfig struct { Enabled
  bool `yaml:"enabled"` }` with the nil-or-false-means-disabled doc comment, and the pointer field
  `Runner *RunnerConfig `yaml:"runner"`` on `Config` (after `Hooks`, which is at line 57). No
  defaulting in `Load`. Additive and default-off — **no migration, no schema bump** (confirmed:
  `migrate.Current()` is bumped only by a new registry entry; `KnownFields(true)` accepts an absent
  `runner:` as nil). **Config-reference parity is required, not optional (ADR-0088):** the
  reflection-parity check makes an unregistered config key a hard gate failure, so add a
  `runner.enabled` descriptor to `internal/configspec/spec.go` (mirroring the `invariants.testGlobs`
  entry at spec.go:147 — Path/Type/Default) and regenerate `docs/config-reference.md` via `./x sync`;
  stage the regenerated reference. This is a mandatory sub-step of this task and Task 5.5's commit.
- [ ] **Task 5.2 — The runner template.** Create `templates/runner/x.tmpl` (embedded FS). Structure:
  a `# GENERATED by awf` banner, `set -euo pipefail`, an `awf:section runner-setup inplace` block
  (in-place; seeded default = a genuine *adopter*-setup placeholder — e.g. a one-line comment such
  as `# Add project-specific setup or helper functions here.` — **not** the pinned-binary resolver:
  the resolver is awf-owned, so putting it in an adopter-editable section would let an adopter break
  `runner-awf-verbs-owned`), the `case "$cmd" in` opener, then the **awf-owned** arms in the
  awf-owned skeleton (outside any in-place section) for `sync | check | invariants | audit | context
  | commit-gate | new)` each delegating **directly to the pinned binary** exactly per ADR-0101
  Decision 2 — `"$(bash .awf/bootstrap.sh)" "$cmd" "$@"` — with **no `command awf` PATH fallback**
  (ADR-0101 Decision 4: the runner targets the pinned-via-bootstrap path only; do **not** reuse the
  hooks templates' fallback-bearing `awf()` helper, whose fallback is a hooks-only affordance), an
  `awf:section runner-project-verbs inplace` block (in-place; seeded default: a `gate)`/`test)`
  starter), the `*)` usage arm, and `esac`. Every var reference must degrade (publication-safe,
  ADR-0045). The two `inplace` sections are the only adopter-editable regions. Keep the awf-verb arm
  list exactly `sync check invariants audit context commit-gate new` (matches `cmd/awf/dispatch.go`
  adopter-facing verbs).
- [ ] **Task 5.3 — The render block.** In `internal/project/render.go` `RenderAll`, add a runner
  block after hooks (after line 404, before the memory block), mirroring the hooks block but passing
  the runner's section list (so `planSections` sees the in-place sections) and rendering to root path
  `x`:
  ```go
  if p.Cfg.Runner != nil && p.Cfg.Runner.Enabled {
      rrf, err := p.renderTarget("runner", "", runnerTID,
          []string{"runner-setup", "runner-project-verbs"}, config.Sidecar{}, p.data(config.Sidecar{}), "x")
      if err != nil { return nil, err }
      out = append(out, rrf)
  }
  ```
  Add the `runnerTID` const alongside `bootstrapTID`/`hookNames` (render.go:23–36). The runner is
  **not** a catalog `DocEntry` — do not touch `catalog.Standard.Docs`, so `SingletonKinds()` and
  `unified_doc_model_test.go` stay green. Note: the runner passes sections through `renderTarget`
  directly (not `renderKind`), like the section-less singletons but *with* a section list.
- [ ] **Task 5.4 — Tests + golden.** In `internal/project/runner_test.go`: enabled → exactly one
  `x` at root, containing the awf-owned arms (assert each verb delegates directly via
  `"$(bash .awf/bootstrap.sh)" "$cmd" "$@"` — pinned-only, no `command awf` fallback, and the arms
  sit outside the adopter-editable in-place sections) and the two
  `awf:edit-in-place` pointers; disabled/absent → no `x`. Publication-safe: render under empty data,
  no unresolved token, no marker residue (reuse the catalog-derived sweep pattern or a direct
  assertion). Assert `catalog.SingletonKinds()` is unchanged (the existing
  `TestUnifiedDocModelProjections` already guards this — add an explicit runner-not-in-SingletonKinds
  assertion for `singleton-kinds-complete`). Put the proof `// invariant: <slug>` markers for all
  five slugs (`runner-singleton-toggle`, `runner-awf-verbs-owned`, `runner-project-verbs-in-place`,
  `runner-render-publication-safe`, `singleton-kinds-complete`) on lines inside the tests in
  `internal/project/runner_test.go` (and `internal/config/*_test.go` for the toggle) — the backing
  scope under `invariants.testGlobs` — not at production sites. Verify:
  `go test ./internal/project/ ./internal/config/ -count=1` → `ok`.
- [ ] **Task 5.5 — Verify and commit.** `./x gate`; `git add internal/config/config.go
  internal/configspec/spec.go docs/config-reference.md templates/runner/x.tmpl
  internal/project/render.go internal/project/runner_test.go`; commit `feat(config): managed
  command-runner singleton`.

## Phase 6 — Example adopter adopts the runner; awf opts out

Backs `inv: runner-example-adopted`. awf-the-repo needs no config change (absent `runner:` key =
disabled); its hand-written `./x` is unchanged and stays outside the render set (ADR-0002 §5).

- [ ] **Task 6.1 — Enable the runner for sundial.** In `examples/sundial/.awf/config.yaml` add
  (after the `hooks:` block, lines 47–48):
  ```yaml
  runner:
    enabled: true
  ```
- [ ] **Task 6.2 — Render and migrate sundial's runner.** Delete the hand-written
  `examples/sundial/x`. Run `./x sync` (re-renders sundial from source) so a rendered `x` appears.
  Edit the `runner-project-verbs` in-place section of the rendered `examples/sundial/x` to sundial's
  actual verbs — port the old bodies: `gate)` → `go test ./...` then `go vet ./...` (with the `full`
  comment), and `test)` → `go test ./... "$@"`. Run `./x sync` again (reads the in-place body back →
  preserved). The `context` verb is now present for free via the awf-owned dispatch — the drift
  ADR-0092 introduced is fixed. The `runner-setup` in-place section stays at its seeded adopter-setup
  placeholder (sundial needs no extra setup — the awf-owned arms already resolve the pinned binary
  directly via `.awf/bootstrap.sh`).
- [ ] **Task 6.3 — Verify zero-notes and green.** Run `./x check` — sundial must be drift-free,
  invariant-clean, and emit **no `note:` lines** (ADR-0090; the `./x check` step greps for `^note: `
  and fails on any). Run `(cd examples/sundial && go test ./...)` → `ok`. Confirm the rendered
  `examples/sundial/x` contains a `context` arm and passes `bash -n examples/sundial/x` (syntax).
  Back `runner-example-adopted` with a proof marker: add an assertion in
  `internal/project/example_wiring_test.go` (where `example-adopter-checked`/`example-zero-notes`
  are backed) that the sundial example enables the runner and renders `examples/sundial/x`, and put
  `// invariant: runner-example-adopted` on a line inside that test (the `**/*_test.go` backing scope).
- [ ] **Task 6.4 — Commit.** `./x gate`; `git add examples/sundial/.awf/config.yaml examples/sundial/x`
  and `git rm` the old path if git tracks the delete as a modify+rename; commit `feat(config): adopt
  the managed runner in the sundial example`.

## Phase 7 — Docs, invariant audit, and ADR status flip

- [ ] **Task 7.1 — Docs travel with the change.** Update the `config` and `rendering` domain
  current-state parts (`.awf/domains/parts/config/current-state.md`,
  `.awf/domains/parts/rendering/current-state.md`) to mention the in-place-editable-section primitive
  and the runner singleton; update `docs/architecture.md` / `docs/working-with-awf.md` sections (via
  their `.awf/docs/parts/...` parts) where the singleton set or override channels are enumerated.
  Specifically, ADR-0100 makes in-place editing a **second adopter override channel**, so correct
  any adopter-facing "the only override channel is a part file" framing (in `docs/working-with-awf.md`
  and, if it enumerates override mechanisms, the AGENTS.md "Working with awf" bullet) to document
  in-place editing of designated output sections alongside convention parts.
  Also update the agent-guide part backing the "Unified compile-time doc model" invariant bullet
  (which enumerates the config-tree render units deliberately outside the doc collection — currently
  "the bootstrap, the hook payloads, and the working-memory `.gitignore`") to name the runner as a
  fourth such unit; the runner is not a `DocEntry`, so this enumeration would otherwise go stale. Run
  `./x sync` and stage the regenerated docs (incl. AGENTS.md). (Do not hand-edit generated outputs.)
- [ ] **Task 7.2 — Confirm every invariant is backed.** Run the real backing oracle — a plain
  `grep` is NOT faithful under ADR-0105 (it matches out-of-scope production markers, and
  `invariant: <slug>` is a substring of an advisory `touches-invariant: <slug>`). Use `./x invariants`
  (equivalently the invariant portion of `./x check`), which enforces the test-scoped proof rule
  (`invariants.testGlobs: ['**/*_test.go']`). It must report all twelve slugs backed:
  in-place-pointer-distinct, in-place-readback, in-place-tamper-drift, section-source-exclusive,
  in-place-spacing-owned, regeneration-checked-attribute, runner-singleton-toggle,
  runner-awf-verbs-owned, runner-project-verbs-in-place, runner-render-publication-safe,
  runner-example-adopted, singleton-kinds-complete. Fix any "declared backed but no proof marker in
  backing scope" finding by adding the proof `// invariant: <slug>` to the owning phase's `*_test.go`.
- [ ] **Task 7.3 — Flip both ADRs to Implemented and freeze the plan.** In
  `docs/decisions/0100-*.md` and `docs/decisions/0101-*.md` set `status: Implemented`; set this
  plan's frontmatter `status: Implemented`. Add the partial-supersession back-pointer: append `101`
  to ADR-0002's `related:` (currently `[]`) — the established convention when an ADR partially
  supersedes another (cf. ADR-0097/0098 each gaining `108`); ADR-0101 already lists `2`, so this
  makes the link bidirectional and `adr-related-link-resolved` stays green. Run `./x sync` to
  regenerate `docs/decisions/ACTIVE.md` and the domain ADR indexes. `./x check` must now enforce all
  twelve backed invariants and pass.
- [ ] **Task 7.4 — Final verify and commit.** `./x gate` and `./x check` (both must pass, including
  the sundial example check and its `go test`). `git add` the two flipped ADRs (0100, 0101), the
  ADR-0002 back-pointer edit, the plan, `ACTIVE.md`, the regenerated domain docs (incl. AGENTS.md),
  any doc parts, and `.awf/awf.lock`; commit `feat(rendering): in-place-
  editable sections and managed runner` (≤72 chars; cite ADR-0100/ADR-0101 in the body).

## Verification

- `./x gate` green at every phase commit; `./x check` clean (main repo + sundial example, zero notes).
- All twelve invariant slugs backed (Task 7.2: `./x invariants` reports all twelve backed); `./x check` enforces them post-flip.
- `examples/sundial/x` is a rendered file containing a `context` arm and sundial's own `gate`/`test`
  in its in-place section; editing the in-place section and re-syncing preserves it; editing an
  awf-owned arm and re-syncing overwrites it (spot-check the tamper/fixpoint behaviour by hand).
- `catalog.SingletonKinds()` unchanged; `TestUnifiedDocModelProjections` green.
- Both ADRs `Implemented`; this plan `Implemented`.

## Notes

- **Precision boundary.** New files (`templates/runner/x.tmpl`), the config toggle, and the sundial
  edits are given exactly. The intricate render/check internals (Phases 2–4: `planSections`
  read-back, the `RegenChecked` predicate migration, and the regeneration-with-read-back compare) are
  specified by anchor (file:line) + representative snippet; exact lines are TDD-driven per the
  project's test-first discipline. Record any diff that turns out wrong here during execution.
- **Lock representation fork (Task 2.4).** Adding `RegenChecked` to `manifest.Entry` is the cleaner
  removal of the path literals but touches the lock schema (no schema-*generation* bump — the lock's
  JSON shape is not the config schema; confirm `manifest` tests). If it proves heavier than deriving
  the set in `Check`, take the derive-in-`Check` route and note it in the Phase-2 commit body.
- **Out of scope (deferred, per ADR-0101):** a hooks↔runner cross-check (a project enabling hooks
  without the runner), a filename parameter for the runner, and any mechanical cross-check of the
  runner template's awf-verb list against `cmd/awf/dispatch.go`. Each is a candidate follow-up.
- **awf-the-repo stays opted out.** No change to awf's own `.awf/config.yaml` or its hand-written
  `./x`; the dogfood is via sundial (ADR-0090).
