# 2026-07-07 — Stub sections and the unauthored-content advisory (ADR-0070)

**Goal:** Implement [ADR-0070](../decisions/0070-stub-sections-and-the-unauthored-content-advisory.md):
a `stub` attribute on section markers and a whole-line `<!-- awf:stub -->` part marker feed a
non-failing unauthored-content advisory printed by `awf check` and `awf init`; a comment-anchored
residual-marker guard makes a malformed section marker a hard render error; the shipped templates
are swept so every must-replace authoring prompt carries the attribute.

**Architecture summary:** `internal/render` parses the attribute (`Segment.Stub`), renders a
distinct stub edit pointer, detects the part marker (`HasStubMarker`), computes per-artifact stub
content (`StubSections`), and guards assembled skeletons (`CheckResidualMarkers`). `internal/project`
threads the results through `SectionPlan.PartStub` and two unexported `RenderedFile` fields, keeps
them across `generateDomainDocs`' strip, and exposes one `AdvisoryNotes()` (unset-var notes then
stub notes, one render pass) that replaces the exported `UnsetVarNotes()` in `cmd/awf` check/init.
`awf new` scaffolds starter parts with the marker. Design rationale lives in the ADR — this plan is
execution only.

**Tech stack:** Go 1.26; packages touched: `internal/render`, `internal/project`, `cmd/awf`,
`templates/`; no new dependencies.

**File structure:**
- Modified: `internal/render/section.go`, `internal/render/render.go`,
  `internal/render/section_test.go`, `internal/render/render_test.go`,
  `internal/project/render.go`, `internal/project/check.go`, `internal/project/notes_test.go`,
  `cmd/awf/check.go`, `cmd/awf/init.go`, `cmd/awf/new.go`, `cmd/awf/new_test.go`,
  `cmd/awf/initrender_test.go`, 12 template files under `templates/` (18 section markers),
  `templates/docs/working-with-awf.md.tmpl`, `.awf/agents-doc.yaml`,
  `.awf/docs/parts/glossary/terms.md`, `.awf/domains/parts/rendering/current-state.md`,
  `.awf/domains/parts/tooling/current-state.md`, `changelog/CHANGELOG.md`,
  `docs/decisions/0070-stub-sections-and-the-unauthored-content-advisory.md` (status flip),
  plus re-rendered output (`docs/…`, `.claude/…`, `AGENTS.md`, `.awf/awf.lock`) via `./x sync`.
- Created: none beyond test cases in existing files.
- Deleted: none.

**Conventions for every phase:** run `./x gate` before each commit (the pre-commit hook also runs
it); after any template or `.awf/` change run `./x sync` and stage rendered files with their
source; never `--amend`. A concurrent agent may be active in this repo implementing ADR-0069; it
touches files this plan also edits (`internal/project/render.go`, `internal/project/check.go`,
`.awf/agents-doc.yaml`, `.awf/domains/parts/rendering/current-state.md`, the changelog). Before
every commit run `git status --short` and stage only this plan's files (pathspec-scoped
`git commit … -- <paths>`); before each phase that edits a shared file, re-verify this plan's
quoted anchors and diff context against `HEAD` — the exact-string edits here were verified against
HEAD as of 4350b54 and may need trivial re-anchoring after the ADR-0069 work lands.

---

## Phase 1 — render package: attribute, pointer, marker, guard

- [ ] 1.1 In `internal/render/section.go`, extend the segment model and regex. Replace the
  `Segment` type and `sectionRE` with:

  ```go
  type Segment struct {
  	IsSection bool
  	Name      string
  	Text      string
  	// Stub marks a section whose template default is a must-replace authoring
  	// prompt, declared by the `stub` marker attribute (ADR-0070).
  	Stub bool
  }

  // The body capture is non-greedy; the optional `\n?` before the closing marker
  // absorbs the body's trailing newline so a normal body excludes it, while an
  // empty-body block (markers on consecutive lines) captures "". The optional
  // ` stub` attribute (ADR-0070) is the only legal marker attribute; any other
  // token makes the marker unparseable, which CheckResidualMarkers turns into a
  // hard render error instead of a silent leak.
  var sectionRE = regexp.MustCompile(`(?s)<!-- awf:section (\S+)( stub)? -->\n(.*?)\n?<!-- awf:end -->`)
  ```

  In `ParseSections`, the submatch indices shift (name m[2:3], attribute m[4:5], body m[6:7]) —
  update the loop's inline index comment (`// m[0]:m[1] whole match; m[2]:m[3] name; m[4]:m[5]
  body`) to the new layout alongside the append:

  ```go
  		segs = append(segs, Segment{
  			IsSection: true,
  			Name:      src[m[2]:m[3]],
  			Stub:      m[4] >= 0,
  			Text:      src[m[6]:m[7]],
  		})
  ```

- [ ] 1.2 Append to `internal/render/section.go` (add `fmt` and `strings` to its imports):

  ```go
  // stubMarkerLine is the whole-line marker a convention part carries to declare
  // itself unauthored starter content (ADR-0070). Whole-line matching means prose
  // that quotes the marker inline never counts.
  const stubMarkerLine = "<!-- awf:stub -->"

  // HasStubMarker reports whether a part body contains a line that is exactly the
  // awf:stub marker (modulo surrounding whitespace). Detection never mutates the
  // body — parts render byte-for-byte verbatim, marker included (ADR-0034, ADR-0070).
  // invariant: stub-part-verbatim
  func HasStubMarker(body string) bool {
  	for _, line := range strings.Split(body, "\n") {
  		if strings.TrimSpace(line) == stubMarkerLine {
  			return true
  		}
  	}
  	return false
  }

  // residualMarkerRE matches a marker-shaped comment opener that survived section
  // assembly: `<!--` + optional whitespace + awf:section/awf:end. Comment-anchored,
  // never a bare-identifier scan — a section default may legally quote the bare
  // token in prose (ADR-0070 Decision 5).
  var residualMarkerRE = regexp.MustCompile(`<!--\s*awf:(section|end)\b`)

  // CheckResidualMarkers hard-errors when an assembled skeleton still contains a
  // marker-shaped awf:section/awf:end token — a malformed marker (unknown
  // attribute, missing name) that ParseSections could not consume and that would
  // otherwise leak verbatim into rendered output. It runs pre-Execute: part bodies
  // are NUL sentinels and data is uninterpolated, so parts and data that quote the
  // full comment form stay out of scope.
  // invariant: no-residual-section-marker
  func CheckResidualMarkers(assembled string) error {
  	if m := residualMarkerRE.FindString(assembled); m != "" {
  		return fmt.Errorf("assembled template still contains a section marker (%q) — malformed awf:section/awf:end marker", m)
  	}
  	return nil
  }
  ```

- [ ] 1.3 In `internal/render/render.go`, add the `PartStub` field, the stub pointer variant, and
  the stub reporter. `SectionPlan` gains one field after `PartBody`:

  ```go
  	// PartStub marks a part body carrying the whole-line awf:stub marker —
  	// declared-unauthored starter content (ADR-0070). Set by the project layer,
  	// which reads part bodies; consumed by StubSections.
  	PartStub bool
  ```

  Replace `editPointer` (the pointer for a stub section rendering its default changes; part-backed
  and plain defaults are unchanged):

  ```go
  // editPointer is the awf:edit provenance comment emitted before a section body.
  // A stub-attributed section rendering its template default gets a distinct
  // pointer so the rendered file itself distinguishes a must-replace default from
  // a valid one (ADR-0070).
  // invariant: section-edit-pointer
  func editPointer(name string, stub bool, p SectionPlan) string {
  	if p.HasPart {
  		return fmt.Sprintf("<!-- awf:edit %s — from %s -->\n", name, p.EditPath)
  	}
  	if stub {
  		return fmt.Sprintf("<!-- awf:edit %s — stub; replace by creating %s -->\n", name, p.EditPath)
  	}
  	return fmt.Sprintf("<!-- awf:edit %s — default; create %s to override -->\n", name, p.EditPath)
  }
  ```

  In `Assemble`, the call site becomes `b.WriteString(editPointer(s.Name, s.Stub, p))`. Append:

  ```go
  // StubSections reports a parsed template's unauthored stub content under a plan
  // (ADR-0070): defaults = stub-attributed sections rendering their template
  // default; parts = sections whose convention part carries the awf:stub marker.
  // Dropped sections report nothing.
  func StubSections(segs []Segment, plan map[string]SectionPlan) (defaults, parts []string) {
  	for _, s := range segs {
  		if !s.IsSection {
  			continue
  		}
  		p := plan[s.Name]
  		switch {
  		case p.Drop:
  		case p.HasPart && p.PartStub:
  			parts = append(parts, s.Name)
  		case !p.HasPart && s.Stub:
  			defaults = append(defaults, s.Name)
  		}
  	}
  	return defaults, parts
  }
  ```

- [ ] 1.4 Unit tests. In `internal/render/section_test.go` add:
  - `TestParseSectionsStubAttribute` — `<!-- awf:section a stub -->\nbody\n<!-- awf:end -->`
    parses with `Stub: true`, `Name: "a"`, `Text: "body"`; the plain form parses `Stub: false`.
  - `TestParseSectionsUnknownAttributeDoesNotParse` — `<!-- awf:section a bogus -->…<!-- awf:end -->`
    yields no section segment (whole text stays literal), and
    `CheckResidualMarkers` returns a non-nil error for it whose message contains
    `malformed awf:section/awf:end marker`.
  - `TestHasStubMarker` — table: exact line → true; line with leading/trailing spaces → true;
    marker quoted inline (`` see `<!-- awf:stub -->` for details ``) → false; absent → false.
  - `TestCheckResidualMarkersBareTokenLegal` — a skeleton containing `` `awf:section` `` (backtick
    prose, no `<!--`) passes; a skeleton containing `<!-- awf:end -->` or `<!--  awf:section x -->`
    fails.

  In `internal/render/render_test.go` add/extend:
  - `TestEditPointerStub` (or extend the existing pointer assertions at render_test.go:32-33,66,80)
    — a stub segment with no part renders the pointer fragment
    `— stub; replace by creating`; with a part it renders `— from`; a non-stub default still
    renders `— default; create`.
  - `TestStubSections` — table over the four cases: drop (nothing), part+PartStub (in `parts`),
    no-part+Stub (in `defaults`), no-part+non-stub (nothing); assert returned slices exactly.
  - `TestAssembleStubPartRendersVerbatim` — a plan with `HasPart: true, PartStub: true` whose
    `PartBody` contains the marker line: `Execute` output contains `<!-- awf:stub -->` verbatim
    (backs `inv: stub-part-verbatim` behaviourally).

- [ ] 1.5 Run `go test ./internal/render/`. Expected: `ok`. Run `./x gate`; expected: green,
  `coverage: 100.0%`.

- [ ] 1.6 Commit:
  `feat(rendering): parse stub section markers and guard residual markers`
  (body: ADR-0070 Decision items 1-3 and 5 at the render layer; the guard closes the
  malformed-marker leak in production).

## Phase 2 — project threading, AdvisoryNotes, CLI printing

- [ ] 2.1 In `internal/project/render.go`:
  - `RenderedFile` gains two unexported fields after `assembled`:

    ```go
    	// stubDefaults / stubParts feed the ADR-0070 unauthored-content advisory:
    	// stub-attributed sections rendered at default, and convention parts
    	// carrying the awf:stub marker. Consumed path-keyed by stubNotes.
    	stubDefaults []string
    	stubParts    []string
    ```

  - In `planSections`, after `sp.PartBody = body` add `sp.PartStub = render.HasStubMarker(body)`.
  - In `renderTarget`, split the parse from the assemble, add the guard and the stub report.
    Replace the single line
    `assembled, parts := render.Assemble(render.ParseSections(expanded), plan)` with:

    ```go
    	segs := render.ParseSections(expanded)
    	assembled, parts := render.Assemble(segs, plan)
    	if err := render.CheckResidualMarkers(assembled); err != nil { // coverage-ignore: awf-owned embedded templates are marker-well-formed, so the guard cannot fire through RenderAll; its error branch is unit-tested in internal/render
    		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
    	}
    	stubDefaults, stubParts := render.StubSections(segs, plan)
    ```

    and extend the returned literal with `stubDefaults: stubDefaults, stubParts: stubParts,`.
  - In `generateDomainDocs`, keep the stub fields across the strip — the append becomes:

    ```go
    		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content,
    			stubDefaults: rf.stubDefaults, stubParts: rf.stubParts})
    ```

- [ ] 2.2 In `internal/project/check.go`, replace the exported `UnsetVarNotes` with the combined
  advisory (one render pass, ADR-0070 Decision 4). Delete the `UnsetVarNotes` method and add:

  ```go
  // AdvisoryNotes returns the non-failing render advisories in print order — the
  // ADR-0045 unset-var notes, then the ADR-0070 stub notes — computed from one
  // RenderAll pass plus the domain-doc generation, which renders outside it.
  func (p *Project) AdvisoryNotes() ([]string, error) {
  	files, err := p.RenderAll()
  	if err != nil {
  		return nil, err
  	}
  	dds, err := p.generateDomainDocs()
  	if err != nil {
  		return nil, err
  	}
  	return append(p.unsetVarNotes(files), stubNotes(append(files, dds...))...), nil
  }

  // unsetVarNotes reports, per rendered artifact, the vars its assembled template
  // references that are unset (missing or empty) in config — the non-failing
  // render-completeness advisory (ADR-0045 item 4). One line per artifact with at
  // least one hit, sorted; adapter duplicates are collapsed by template id.
  func (p *Project) unsetVarNotes(files []RenderedFile) []string {
  	seen := map[string]bool{}
  	var notes []string
  	for _, f := range files {
  		if seen[f.TemplateID] {
  			continue
  		}
  		seen[f.TemplateID] = true
  		var unset []string
  		for _, r := range render.ReferencedVars(f.assembled) {
  			if v := p.Cfg.Vars[r]; v == nil || v == "" {
  				unset = append(unset, r)
  			}
  		}
  		if len(unset) == 0 {
  			continue
  		}
  		notes = append(notes, fmt.Sprintf("%s references unset vars: %s",
  			artifactLabel(f.TemplateID), strings.Join(unset, ", ")))
  	}
  	sort.Strings(notes)
  	return notes
  }

  // stubNotes reports, per rendered artifact, its unauthored stub content —
  // stub-attributed sections still at default and awf:stub-marked parts. One line
  // per output path: artifacts sharing a template id (local artifacts, the domain
  // docs) each report independently, and a multi-target project prints one line
  // per target path by design (ADR-0070).
  // invariant: stub-notes-path-keyed
  func stubNotes(files []RenderedFile) []string {
  	var notes []string
  	for _, f := range files {
  		if len(f.stubDefaults) == 0 && len(f.stubParts) == 0 {
  			continue
  		}
  		var clauses []string
  		if len(f.stubDefaults) > 0 {
  			clauses = append(clauses, "sections at stub default: "+strings.Join(f.stubDefaults, ", "))
  		}
  		if len(f.stubParts) > 0 {
  			clauses = append(clauses, "stub-marked parts: "+strings.Join(f.stubParts, ", "))
  		}
  		notes = append(notes, fmt.Sprintf("%s has unauthored stub content — %s",
  			f.Path, strings.Join(clauses, "; ")))
  	}
  	sort.Strings(notes)
  	return notes
  }
  ```

  The `UnsetVarNotes` doc reference in `internal/project/render.go:40` (the `assembled` field
  comment) is updated to say `unsetVarNotes`.

- [ ] 2.3 CLI printing. In `cmd/awf/check.go:24` replace `p.UnsetVarNotes()` with
  `p.AdvisoryNotes()` and put the non-failing invariant marker on the print loop:

  ```go
  	notes, err := p.AdvisoryNotes()
  	if err != nil {
  		return err
  	}
  	// Advisories are printed before drift and never feed the failure count —
  	// unauthored stub content cannot fail a gated command (ADR-0070).
  	// invariant: stub-advisory-nonfailing
  	for _, n := range notes {
  		fmt.Fprintf(stdout, "note: %s\n", n)
  	}
  ```

  In `cmd/awf/init.go:100` replace `np.UnsetVarNotes()` with `np.AdvisoryNotes()` (the comment
  above it becomes "the same advisory notes awf check prints (ADR-0045, ADR-0070)"). Update the
  `coverage-ignore` reason on its error branch from "RenderAll succeeded moments ago inside
  runSync" to note that runSync just rendered this same tree *and* generated its domain docs —
  both `AdvisoryNotes` inputs succeeded moments ago.

- [ ] 2.4 Update the four tests in `internal/project/notes_test.go` from `p.UnsetVarNotes()` to
  `p.AdvisoryNotes()` (same expectations — the fixtures have no stub content, so the note sets are
  unchanged; `TestUnsetVarNotesSurfacesRenderError` keeps working because `AdvisoryNotes` surfaces
  the same `RenderAll` error). Then add, in the same file — all three run green in this phase,
  none needs the Phase 4 attribute sweep:
  - `TestAdvisoryNotesSurfacesDomainDocError` — a fixture with `domains: [config]` plus a
    malformed ADR file (`docs/decisions/0001-bad.md` with unparseable frontmatter):
    `AdvisoryNotes` returns a non-nil error. `RenderAll` never parses ADRs, so this is the only
    test that can reach the `generateDomainDocs` error branch inside `AdvisoryNotes` — unlike
    `Check`, where that branch is coverage-ignored because `generateActiveMD` fails first,
    `AdvisoryNotes` has no earlier ADR parse, so without this test the 100%-coverage gate fails
    at task 2.6.
  - `TestStubNotesPathKeyedAcrossTargets` — a fixture with `targets: [claude, cursor]` and a
    `<!-- awf:stub -->`-marked convention part for one `tdd` skill section: two note lines, one
    per adapter skill output path (backs `inv: stub-notes-path-keyed`). The stub-part path runs
    through `HasStubMarker`/`PartStub` and needs no template attribute, so it is Phase-2-viable.
  - `TestStubNotesDefaultsClauseUnit` — a direct in-package unit test of `stubNotes` over
    hand-built `RenderedFile` values (`notes_test.go` is `package project`, so the unexported
    fields are settable): one file with `stubDefaults` only, one with both `stubDefaults` and
    `stubParts` (asserting the `; `-joined single-line format), one with neither (no note). This
    covers the defaults clause, which no fixture can reach until the Phase 4 sweep.

  Two further tests need the swept template attributes and land in task 4.2, not here:
  `TestStubNotesReportsDefaultsAndParts` and `TestStubNotesDomainDocs` (specified in 4.2).

- [ ] 2.5 Add `TestCheckStubNotesAreNonFailing` in `cmd/awf/initrender_test.go` (beside
  `TestCheckUnsetVarNotesAreNonFailing`, line 116): scaffold a project, write a
  `<!-- awf:stub -->`-marked convention part for an enabled artifact's declared section, run
  `runSync` then `runCheck` into a buffer: the output contains a `note: ` line containing
  `has unauthored stub content` and `runCheck` returns nil (backs `inv: stub-advisory-nonfailing`
  behaviourally; no Phase 4 dependency — the stub-part path needs no template attribute).

- [ ] 2.6 Run `./x gate`. Expected: green, 100% coverage — 2.4's updated tests keep the unset-var
  paths covered; `TestStubNotesPathKeyedAcrossTargets` and `TestCheckStubNotesAreNonFailing`
  cover the parts clause and the CLI print loop; `TestStubNotesDefaultsClauseUnit` covers the
  defaults clause; `TestAdvisoryNotesSurfacesDomainDocError` covers `AdvisoryNotes`'
  domain-doc error branch.

- [ ] 2.7 Commit:
  `feat(rendering): surface unauthored-stub advisory from check and init`
  (body: ADR-0070 Decision 4 — path-keyed non-failing notes, one render pass with the unset-var
  advisory; UnsetVarNotes folds into AdvisoryNotes).

## Phase 3 — `awf new` starter parts carry the marker

- [ ] 3.1 In `cmd/awf/new.go`, replace the `localPartStub` const:

  ```go
  // localPartStub is the starter body for a new local artifact's content part —
  // plain prose only (no live {{=awf:…}} placeholder, which would hard-error if its
  // value were unset this render). The leading awf:stub marker line declares the
  // part unauthored (ADR-0070): awf check reports it until the author deletes the
  // line, and the part still renders verbatim, marker included.
  const localPartStub = "<!-- awf:stub -->\n" +
  	"Replace this with the artifact's body, then delete the awf:stub marker line above — " +
  	"awf check flags this part as unauthored while the marker remains. This file is a " +
  	"convention part: edit it to author the content, and see docs/working-with-awf.md for " +
  	"the placeholder syntax.\n"
  ```

- [ ] 3.2 Extend `TestRunNewScaffoldsSkill` in `cmd/awf/new_test.go` (line 62; it already stats
  the content part at `.awf/skills/parts/deploy-check/content.md` and the rendered
  `.claude/skills/example-deploy-check/SKILL.md`): assert the written content part starts with
  `<!-- awf:stub -->\n`, and that the rendered skill file contains the marker verbatim
  (stub-part-verbatim, behaviourally). Its trailing `runCheck` assertion stays nil — the new stub
  note is non-failing.

- [ ] 3.3 Run `./x gate`. Expected: green. Commit:
  `feat(awf): mark scaffolded starter parts with the awf:stub marker`
  (body: ADR-0070 Decision 2 — awf new artifacts self-flag until authored).

## Phase 4 — template sweep + this repo's re-render

- [ ] 4.1 Add the `stub` attribute to exactly these 18 section markers (change
  `<!-- awf:section <name> -->` to `<!-- awf:section <name> stub -->`; nothing else in the file):

  | Template | Sections |
  |---|---|
  | `templates/agents/_base.md.tmpl` | `content` |
  | `templates/skills/_base/SKILL.md.tmpl` | `content` |
  | `templates/agents-doc/AGENTS.md.tmpl` | `identity` |
  | `templates/domains/domain.md.tmpl` | `current-state` |
  | `templates/docs/architecture.md.tmpl` | `overview`, `components`, `data-flow`, `dependencies` |
  | `templates/docs/debugging.md.tmpl` | `surfaces`, `recipes` |
  | `templates/docs/development.md.tmpl` | `setup`, `command-runner`, `dependencies` |
  | `templates/docs/glossary.md.tmpl` | `terms` |
  | `templates/docs/pitfalls.md.tmpl` | `entries` |
  | `templates/docs/roadmap.md.tmpl` | `ideas`, `deferred` |
  | `templates/docs/testing.md.tmpl` | `layout` |

  Everything else stays plain (the classification's borderline calls — `testing` `gate`/`tiers`,
  `agents-doc` `you-and-this-project`/`invariants`, the ADR template, empty override slots, and
  data-driven reviewer slots — are deliberately valid; see ADR-0070 Decision 6).

- [ ] 4.2 Land the two Phase 2-deferred tests in `internal/project/notes_test.go` (they need
  4.1's attributes):
  - `TestStubNotesReportsDefaultsAndParts` — scaffold a fixture project (testsupport) enabling the
    `development` doc with no parts: `AdvisoryNotes` contains exactly one line
    `docs/development.md has unauthored stub content — sections at stub default: setup, command-runner, dependencies`
    (order follows template section order: setup at line 3, command-runner at line 9, dependencies
    at line 15 of `templates/docs/development.md.tmpl`; `sort.Strings` in `stubNotes` orders the
    note lines, not the section names within a line). Then write a `<!-- awf:stub -->`-marked part
    for one section (`.awf/docs/parts/development/setup.md` containing the marker line plus one
    prose line): the line now reads
    `… sections at stub default: command-runner, dependencies; stub-marked parts: setup`.
  - `TestStubNotesDomainDocs` — a fixture with `domains: [config]` and no current-state part:
    `AdvisoryNotes` contains `docs/domains/config.md has unauthored stub content — sections at stub default: current-state`.

- [ ] 4.3 Run `./x gate` and fix pointer-fragment assertions that now see `— stub;` instead of
  `— default;` on swept sections. `internal/render/render_test.go:32-33,66,80` and
  `internal/project/project_test.go:95` assert against unswept fixtures (the in-package test
  template; skill `tdd`) and should stay green; the realistic candidates are fixtures that render
  a swept template's default — golden/pointer assertions in `internal/project/docs_sections_test.go`,
  `internal/project/spine_test.go`, agents-doc `identity` renders in project/cmd tests, and the
  `internal/evals` full-catalog suite. Fix exactly what fails. Do not relax any assertion — update
  the expected fragment to the new pointer text.

- [ ] 4.4 Run `./x sync && ./x check`. Expected: `awf sync: done`, then `awf check: clean` with
  **no** `unauthored stub content` notes for this repo — every swept surface here is part-backed
  (architecture ×4, development ×3, testing layout, glossary, pitfalls, agents-doc identity, all
  five domain current-states) and no local artifacts or debugging/roadmap docs are enabled. If a
  note appears, a part is genuinely missing — author it, don't suppress. Rendered-file content in
  this repo should be unchanged except `.awf/awf.lock` template hashes; `git status --short`
  confirms.

- [ ] 4.5 Commit (template sources, tests, lock, any re-rendered files):
  `feat(rendering): classify must-replace template defaults as stub`
  (body: ADR-0070 Decision 6 — the 18 authoring-prompt defaults across the docs family, the domain
  current-state, the agent-guide identity, and the two local-artifact base templates).

## Phase 5 — docs, guide, changelog

- [ ] 5.1 `templates/docs/working-with-awf.md.tmpl` — in the `config-and-overrides` section, after
  the paragraph describing convention parts, insert (bare-token quoting for `awf:section` is
  deliberate — the full comment form in a section default would trip the residual guard):

  ```markdown
  Some section defaults are must-replace authoring prompts rather than shippable prose. Those are
  declared with a `stub` attribute on their `awf:section` marker; the rendered pointer reads
  `— stub; replace by creating <path>`, and `awf check` prints a non-failing note per artifact
  until the part exists. A convention part can itself carry a whole-line `<!-- awf:stub -->`
  marker to declare unauthored starter content — `awf new skill|agent` scaffolds its content part
  that way; delete the marker line once the part is real.
  ```

- [ ] 5.2 `.awf/agents-doc.yaml` — two edits in the invariants data list:
  - Add after the last entry, keeping ADR-numeric order (the concurrent ADR-0069 work may have
    appended its own entry after the ADR-0060/0061 one by the time this executes):

    ```yaml
            - ref: ADR-0070
              text: '**Stub advisory, residual-marker guard.** Unauthored stub content — a stub-attributed section at its template default, or a part carrying the whole-line `awf:stub` marker — is a non-failing `awf check`/`awf init` note keyed by output path, never a failure; and a marker-shaped `awf:section`/`awf:end` token surviving section assembly is a hard render error.'
    ```

  - Currency fix in the same file (recorded pitfall: hard-coded counts): in the ADR-0054 entry,
    change `the nine-node chain graph is connected` to `the ten-node chain graph is connected`.

- [ ] 5.3 `.awf/docs/parts/glossary/terms.md` — add a sorted row (after `retrospective`):

  ```markdown
  | stub | Must-replace starter content: a section default declared with the `stub` marker attribute, or a convention part carrying the whole-line `<!-- awf:stub -->` marker. Rendered publication-safe but reported by the `awf check`/`awf init` unauthored-content advisory until authored (ADR-0070). |
  ```

- [ ] 5.4 Domain narratives (ADR-0070 `domains: [rendering, tooling]` — both refresh before the
  Implemented flip):
  - `.awf/domains/parts/rendering/current-state.md`: after the sentence describing local-artifact
    synthesis, insert:

    ```
    ADR-0070 layers an unauthored-content signal onto the section model: a `stub` marker attribute flags must-replace defaults (distinct `— stub;` edit pointer), a whole-line `<!-- awf:stub -->` marker declares a convention part unauthored without breaking ADR-0034's verbatim contract, `StubSections`/`stubNotes` surface both as a non-failing, path-keyed advisory from `awf check`/`awf init` (one render pass with the unset-var notes, domain docs included), and a comment-anchored residual-marker guard makes any marker-shaped `awf:section`/`awf:end` token surviving assembly a hard render error instead of a silent leak.
    ```

  - `.awf/domains/parts/tooling/current-state.md`: in the `awf new` paragraph, after the
    project-local sentence, insert:

    ```
    The scaffolded starter part opens with the whole-line `<!-- awf:stub -->` marker (ADR-0070), so `awf check` reports the artifact as unauthored until the author deletes the line.
    ```

- [ ] 5.5 `changelog/CHANGELOG.md` — under `## [Unreleased]` add (merge the bullets into an
  existing `### Features` block if the ADR-0069 work already created one):

  ```markdown
  ### Features
  - Must-replace template defaults are now declared with a `stub` attribute on their section
    marker, and `awf new`'s starter parts open with a whole-line `<!-- awf:stub -->` marker.
    `awf check` and `awf init` print a non-failing note per artifact with unauthored stub
    content; a stub section's rendered pointer reads `— stub; replace by creating <path>`
    (ADR-0070). Upgrading re-renders every artifact whose template was swept — expect one large
    `awf sync` commit.
  - A malformed `awf:section`/`awf:end` marker is now a hard render error instead of leaking
    verbatim into rendered output (ADR-0070).
  ```

- [ ] 5.6 Run `./x sync && ./x check` (expected: clean; `docs/working-with-awf.md`, `AGENTS.md`,
  glossary, domain docs re-render), then `./x gate`. Commit everything from 5.1-5.5 plus rendered
  output (including `.awf/awf.lock`):
  `docs(rendering): document stub sections and the stub advisory`
  (72-char subject limit rules out spelling "unauthored-content advisory" here; the body can).

## Phase 6 — ADR flip

- [ ] 6.1 Edit `docs/decisions/0070-stub-sections-and-the-unauthored-content-advisory.md`
  frontmatter: `status: Proposed` → `status: Implemented`.

- [ ] 6.2 Run `./x sync` (regenerates `ACTIVE.md` + domain indexes), then `./x invariants` —
  expected output: no findings (all four slugs backed: `stub-part-verbatim` and
  `no-residual-section-marker` in `internal/render/section.go`, `stub-notes-path-keyed` in
  `internal/project/check.go`, `stub-advisory-nonfailing` in `cmd/awf/check.go`). Then `./x gate`.

- [ ] 6.3 Commit `docs/decisions/0070-…md`, `docs/decisions/ACTIVE.md`,
  `docs/domains/rendering.md`, `docs/domains/tooling.md`, `.awf/awf.lock`:
  `docs(adr): mark 0070 implemented`.

No plan-freeze line is added: per `docs/plans/README.md` an ADR-driven plan freezes automatically
once its ADR flips to Implemented.
