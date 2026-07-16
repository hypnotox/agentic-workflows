---
date: 2026-07-16
adrs: [121]
status: Implemented
---
# Plan: Whole-line authoring comments

## Goal

Implement ADR-0121: the whole-line `<!-- awf:comment ... -->` authoring directive stripped at
the two source-ingestion seams, the optional `invariants.sources[].close` token, the `parts-raw`
succession, and the dogfood wiring with a retro-tagging sweep over awf's own parts and templates.
Non-goals: no scaffold seeding of the tagging recipe, no fence-aware invariant scanner, no
stripping of plain (non-directive) HTML comments.

## Architecture summary

Four phases, each gate-green on its own. Phase 1 lands the scanner-side `close` token
(config field, configspec entry, both scan loops) - independent of the strip. Phase 2 lands the
render-side strip: a fence-aware line filter in `internal/render`, wired at the template seam
(post-`ExpandIncludes`, pre-`ParseSections`) and the part seam (`planSections`, pre-substitution),
with the confighash placeholder detectors and the three part scanners moved to stripped input,
plus the adopter-facing documentation. Phase 3 wires the dogfood `invariants.sources` entry and
retro-tags awf's own parts and templates with `touches-invariant` markers. Phase 4 renames the
`parts-raw` markers to the successor slug, updates the three domain narratives and the changelog,
and flips ADR-0121 and this plan to Implemented.

Expected transient state: the proof markers for five of ADR-0121's six new slugs land with their
tests in phases 1-2 (the sixth, `parts-raw-except-authoring-comments`, arrives only with the
phase 4 rename), while the slugs are declared by a still-`Proposed` ADR, so `./x check` prints
one non-failing `invariant marker "<slug>" names a slug no Implemented ADR declares` note per
marker-carrying slug from its landing phase until the phase 4 flip clears them. Notes never fail
the gate.

## File structure

- **Created:**
  - `internal/render/comment.go`
  - `internal/render/comment_test.go`
- **Modified:**
  - `internal/config/config.go` (InvariantSource gains `Close`)
  - `internal/configspec/spec.go` (new `invariants.sources[].close` entry; sources type string)
  - `internal/invariants/invariants.go` (markerSpec plumbing, `trimClose`, both scan loops)
  - `internal/invariants/invariants_test.go` (close-token tests)
  - `internal/project/render.go` (both strip call sites)
  - `internal/project/confighash.go` (placeholder detectors read stripped bytes)
  - `internal/project/render_test.go` / `internal/project/inplace_test.go` /
    `internal/project/drift_test.go` (e2e strip, in-place regression, var-in-comment tests;
    exact test files per task)
  - `templates/docs/working-with-awf.md.tmpl` (directive documentation + recipe)
  - `.awf/docs/parts/architecture/data-flow.md` (render-flow note)
  - `.awf/docs/glossary.yaml` (authoring-comment term)
  - `.awf/config.yaml` (dogfood sources entry)
  - `.awf/domains/parts/rendering/current-state.md`, `.awf/domains/parts/invariants/current-state.md`
    (narratives + retro tags), `.awf/domains/parts/config/current-state.md` (narrative only)
  - `.awf/parts/adr-readme/invariants.md`, `templates/adr-readme/README.md.tmpl` (retro tags)
  - `changelog/CHANGELOG.md` (Unreleased feature entry)
  - `docs/decisions/0121-whole-line-authoring-comments-in-templates-and-parts.md` (status flip)
  - Rendered files re-synced by `./x sync` in each phase (`docs/*.md`, `docs/decisions/README.md`,
    `docs/config-reference.md`, `.awf/awf.lock`, `examples/sundial/**` where the embedded
    template edits propagate)
- **Deleted:** none

## Phase 1: the `invariants.sources[].close` token

- [ ] **Task 1.1: config field.** In `internal/config/config.go`, extend `InvariantSource`:

  ```go
  // InvariantSource pairs anchored path globs (ADR-0077; matched against a file's
  // slash-separated repo-relative path) with
  // the literal comment marker that prefixes a backing `invariant: <slug>` tag.
  // Close is the optional literal close token for block-comment markers (`-->`,
  // `*/`): when non-empty, one trailing token (plus surrounding whitespace) is
  // stripped from a matched marker line before tag parsing (ADR-0121). Additive
  // and optional - empty means no stripping - so no schema-generation bump.
  type InvariantSource struct {
  	Globs  []string `yaml:"globs"`
  	Marker string   `yaml:"marker"`
  	Close  string   `yaml:"close"`
  }
  ```

  No new validation: an empty `close` is indistinguishable from an absent one by design
  (ADR-0121 Decision 5) and means "no close token".

- [ ] **Task 1.2: configspec entry.** In `internal/configspec/spec.go`, change the
  `invariants.sources` entry's `Type` from `"list of {globs, marker} mappings"` to
  `"list of {globs, marker, close} mappings"`, and insert after the
  `invariants.sources[].marker` entry:

  ```go
  	{
  		Path: "invariants.sources[].close", Type: "string", Default: "none: no close token stripped",
  		Description:  "Optional literal close token for block-comment markers (`-->`, `*/`). When set, one trailing close token (plus surrounding whitespace) is stripped from a matched marker line before the `invariant: <slug>` / `touches-invariant: <slug>` tag is parsed, so a touches note stays clean and a note-less touches marker still counts as bare. Empty or absent means no stripping.",
  		Availability: "Within each `invariants.sources` entry.",
  	},
  ```

- [ ] **Task 1.3: scanner plumbing.** In `internal/invariants/invariants.go`:

  1. Add, next to the `touchMark` type:

     ```go
     // markerSpec is one scan marker for a file: the literal line-prefix comment
     // marker and the optional literal close token stripped once from the line
     // end before tag parsing (ADR-0121).
     type markerSpec struct {
     	open  string
     	close string
     }

     // trimClose strips one trailing close token, plus surrounding whitespace,
     // from rest when the source declares one; an empty close leaves rest as-is,
     // and a line not carrying the token is parsed unchanged (best-effort).
     func trimClose(rest, close string) string {
     	if close == "" {
     		return rest
     	}
     	t := strings.TrimRight(rest, " \t")
     	if strings.HasSuffix(t, close) {
     		return strings.TrimRight(strings.TrimSuffix(t, close), " \t")
     	}
     	return rest
     }
     ```

  2. In `scanTags`, change the per-file marker collection from `var markers []string` /
     `markers = append(markers, src.Marker)` to `var markers []markerSpec` /
     `markers = append(markers, markerSpec{open: src.Marker, close: src.Close})`, and in the
     line loop replace

     ```go
     			for _, marker := range markers {
     				if !strings.HasPrefix(trimmed, marker) {
     					continue
     				}
     				rest := trimmed[len(marker):]
     ```

     with

     ```go
     			for _, marker := range markers {
     				if !strings.HasPrefix(trimmed, marker.open) {
     					continue
     				}
     				rest := trimClose(trimmed[len(marker.open):], marker.close)
     ```

  3. Apply the identical two changes to the duplicated loop in `MarkersUnder` and to
     `markersFor` (return `[]markerSpec`; dedupe key `m.open + "\x00" + m.close`; the
     testGlobs union branch appends every source's `markerSpec`). Both scan loops must carry
     the close strip - they are duplicated code (ADR-0121 Decision 5 covers both paths).

- [ ] **Task 1.4: close-token tests.** In `internal/invariants/invariants_test.go`, add a test
  `TestCloseTokenStripping` covering, over a temp tree with a source
  `{globs: ["*.md"], marker: "<!-- awf:comment", close: "-->"}` (plus the existing `//` source
  shape for the mixed case) and a decisions-dir fixture whose Implemented ADR declares
  `close-a`, `close-b`, and `close-c` (the bare-touches branch is reachable only for a
  declared slug - an undeclared one routes to the dangling note instead):

  - a proof line `<!-- awf:comment invariant: close-a -->` backs `close-a` (close stripped
    before `slugRe`);
  - a touches line `<!-- awf:comment touches-invariant: close-b - a note -->` records slug
    `close-b` with note exactly `- a note` (no trailing `-->`);
  - a note-less `<!-- awf:comment touches-invariant: close-c -->` yields an empty note and
    fires the bare-touches advisory;
  - a marker line whose close token is absent (`<!-- awf:comment invariant: close-a` with no
    trailing `-->`) parses unchanged - the `trimClose` best-effort `return rest` branch, owed
    for the coverage gate;
  - a close-less `//`-marker line in the same tree parses unchanged (mixed sources);
  - a `MarkersUnder` query over the same tree returns the close-stripped notes (both paths
    proven).

  Carry the proof marker on the test:

  ```go
  // invariant: invariant-marker-close-token
  ```

- [ ] **Task 1.5: verify and commit.** Run `./x sync` (regenerates `docs/config-reference.md`
  with the new key), then `./x gate` (green; expect one non-failing dangling-marker note for
  `invariant-marker-close-token` until the phase 4 flip). Stage
  `internal/config/config.go internal/configspec/spec.go internal/invariants/invariants.go
  internal/invariants/invariants_test.go docs/config-reference.md .awf/awf.lock` plus any other
  files `./x sync` reports changed, and commit:

  ```commit
  feat(invariants): add the invariants.sources close token
  ```

## Phase 2: the `awf:comment` strip

- [ ] **Task 2.1: strip function.** Create `internal/render/comment.go`:

  ```go
  package render

  import (
  	"fmt"
  	"strings"
  )

  // commentOpen is the exact authoring-comment literal (ADR-0121). The strip and
  // the documented invariants.sources marker must stay byte-identical, so a
  // comment that strips here is exactly a comment the scanner can read; a
  // whitespace variant is not the directive and passes through visibly.
  const commentOpen = "<!-- awf:comment"

  // StripAuthoringComments removes whole-line awf:comment authoring directives
  // from src: a line whose trimmed form opens with the exact commentOpen literal
  // at a token boundary (followed by a space, a tab, "-->", or the end of the
  // line) and ends with "-->" is removed together with its trailing newline.
  // Fenced code blocks are preserved verbatim, so a part or template can
  // demonstrate the syntax. A whole line outside a fence that opens at the
  // boundary but does not end with "-->" - a missing close, the bare opener, or
  // text trailing the close - is a hard error; the input is returned unchanged
  // alongside it. Mid-line occurrences and prefix-sharing tokens (awf:commentary)
  // never fire.
  func StripAuthoringComments(src string) (string, error) {
  	var kept []string
  	inFence := false
  	fence := ""
  	for i, line := range strings.Split(src, "\n") {
  		trimmed := strings.TrimSpace(line)
  		if inFence {
  			if strings.HasPrefix(trimmed, fence) {
  				inFence = false
  			}
  			kept = append(kept, line)
  			continue
  		}
  		switch {
  		case strings.HasPrefix(trimmed, "```"):
  			inFence, fence = true, "```"
  			kept = append(kept, line)
  		case strings.HasPrefix(trimmed, "~~~"):
  			inFence, fence = true, "~~~"
  			kept = append(kept, line)
  		case opensAuthoringComment(trimmed):
  			if !strings.HasSuffix(trimmed, "-->") {
  				return src, fmt.Errorf("line %d: malformed awf:comment (the whole line must end with \"-->\"): %s", i+1, trimmed)
  			}
  			// A directive line: dropped, its newline consumed by the join.
  		default:
  			kept = append(kept, line)
  		}
  	}
  	return strings.Join(kept, "\n"), nil
  }

  // opensAuthoringComment reports whether a trimmed line opens with the exact
  // directive literal at a token boundary: followed by whitespace, "-->", or
  // the end of the line. "<!-- awf:commentary" fails the boundary.
  func opensAuthoringComment(trimmed string) bool {
  	rest, ok := strings.CutPrefix(trimmed, commentOpen)
  	if !ok {
  		return false
  	}
  	return rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t") || strings.HasPrefix(rest, "-->")
  }
  ```

- [ ] **Task 2.2: strip unit tests.** Create `internal/render/comment_test.go` with table-driven
  cases proving, at minimum:

  - whole-line directive stripped, including its newline (mid-file: no blank residue;
    comment-only input strips to `""`; a directive as the last line without a trailing
    newline also strips to `""`);
  - indented directive line stripped (trimmed-form matching);
  - mid-line occurrence preserved verbatim, and `<!-- awf:commentary -->` (closed) preserved;
  - unclosed prefix-sharing token (`<!-- awf:commentary no close`) preserved, never an error;
  - fenced directive line (backtick and tilde fences) preserved; a malformed opener inside a
    fence preserved, not an error;
  - malformed openers outside a fence error naming the line: bare `<!-- awf:comment`,
    `<!-- awf:comment no close`, and `<!-- awf:comment x --> extra`, each returning the input
    unchanged;
  - `<!-- awf:comment-->` (immediate close) stripped.

  Carry proof markers on the covering test functions:

  ```go
  // invariant: authoring-comment-whole-line-only
  // invariant: authoring-comment-malformed-fails
  ```

- [ ] **Task 2.3: template seam.** In `internal/project/render.go` `renderTarget`, between the
  `ExpandIncludes` call and `ParseSections`, insert:

  ```go
  	stripped, err := render.StripAuthoringComments(expanded)
  	if err != nil { // coverage-ignore: awf-owned embedded templates never author a malformed awf:comment opener, so the strip cannot fail through RenderAll; its error branch is unit-tested in internal/render
  		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
  	}
  	segs := render.ParseSections(stripped)
  	style := render.CommentStyleForSource(stripped)
  ```

  (replacing the existing `segs := render.ParseSections(expanded)` /
  `style := render.CommentStyleForSource(expanded)` lines). `TemplateHash` stays
  `manifest.Hash([]byte(expanded))` - the unstripped post-include source (ADR-0121 Decision 2);
  add to its existing comment block the line:

  ```go
  		// touches-invariant: authoring-comment-stripped - TemplateHash covers the pre-strip source, so a comment-only template edit reflags stale and self-settles
  ```

- [ ] **Task 2.4: part seam.** In `internal/project/render.go` `planSections`, replace the part
  read block

  ```go
  		b, err := os.ReadFile(p.Cfg.PartPath(kind, artifact, s))
  		if err == nil {
  			body, serr := p.substitutePlaceholders(p.partRel(kind, artifact, s), string(b), reg)
  			if serr != nil {
  				return nil, serr
  			}
  			sp.HasPart = true
  			sp.PartBody = body
  			sp.PartStub = render.HasStubMarker(body)
  			// Scanned on the raw on-disk bytes, never the substituted body
  			// (ADR-0083 Decision 4), with fenced examples excluded.
  			sp.PartMarker = render.HasMarkerLine(refs.WithoutFences(string(b)))
  			sp.PartVarRefs = render.PlaceholderVarRefs(string(b))
  		} else if !errors.Is(err, os.ErrNotExist) {
  ```

  with

  ```go
  		b, err := os.ReadFile(p.Cfg.PartPath(kind, artifact, s))
  		if err == nil {
  			// Stripped before substitution (ADR-0121 Decision 2): a substituted
  			// value can never create or mask a whole-line directive, and an
  			// unknown placeholder demonstrated inside a comment must not error.
  			raw, serr := render.StripAuthoringComments(string(b))
  			if serr != nil {
  				return nil, fmt.Errorf("part %s: %w", p.partRel(kind, artifact, s), serr)
  			}
  			body, serr := p.substitutePlaceholders(p.partRel(kind, artifact, s), raw, reg)
  			if serr != nil {
  				return nil, serr
  			}
  			sp.HasPart = true
  			sp.PartBody = body
  			sp.PartStub = render.HasStubMarker(body)
  			// Scanned on the stripped pre-substitution bytes (ADR-0083 Decision 4's
  			// raw-bytes contract preserved in effect - the strip cannot add or
  			// remove a marker-shaped line; ADR-0121), fenced examples excluded.
  			sp.PartMarker = render.HasMarkerLine(refs.WithoutFences(raw))
  			sp.PartVarRefs = render.PlaceholderVarRefs(raw)
  		} else if !errors.Is(err, os.ErrNotExist) {
  ```

- [ ] **Task 2.5: confighash detectors.** In `internal/project/confighash.go`
  `artifactConfigHash`, after the `os.ReadFile(pp)` of each consumed part, strip before the two
  placeholder detectors while hashing the raw bytes unchanged:

  ```go
  		stripped, serr := render.StripAuthoringComments(string(b))
  		if serr != nil { // coverage-ignore: planSections stripped this same consumed part earlier in the render pass and errored there, so a malformed opener cannot reach this re-read
  			return "", serr
  		}
  		if render.ReferencesScopePlaceholder(stripped) {
  ```

  (and `render.ReferencesInvariantMarkerPlaceholder(stripped)` likewise; the
  `parts[...] = manifest.Hash(b)` line keeps hashing the raw on-disk bytes).

- [ ] **Task 2.6: project-level tests.** Add e2e tests (in `internal/project/render_test.go`
  unless a sibling file is the better home; name the file in the commit):

  - In `internal/project/render_test.go`: a scaffolded project whose convention part carries a
    whole-line directive, a mid-line occurrence, and a fenced directive renders output
    containing the mid-line and fenced forms and not the whole-line form (the template-source
    seam is covered by the render-layer unit tests of task 2.2). Proof marker:
    `// invariant: authoring-comment-stripped`.
  - In `internal/project/render_test.go`: a part whose only content is directive lines renders
    its section empty with the pointer present (ADR-0034 Decision 4 semantics preserved).
  - In `internal/project/render_test.go`: a part with a malformed opener fails sync naming the
    part path.
  - In `internal/project/render_test.go`: a part whose authoring comment contains an unknown
    placeholder (`{{=awf:nonexistent}}`) renders without error - the strip-before-substitution
    motivation of ADR-0121 Decision 2.
  - In `internal/project/inplace_test.go`: an in-place section (ADR-0100 fixture) whose on-disk
    output region contains a directive-shaped line survives re-render byte-for-byte. Proof
    marker: `// invariant: authoring-comment-inplace-inert`.
  - In `internal/project/drift_test.go` (beside the existing part-scopes-in-confighash test): a
    part referencing a `{{=awf:commitScope*}}` placeholder only inside an authoring comment
    does not fold scopes into its ConfigHash, and a var-reading placeholder (e.g.
    `{{=awf:gateCmd}}`) appearing only inside an authoring comment does not join `PartVarRefs`
    (note: `PlaceholderVarRefs` matches `{{=awf:key}}` registry tokens, never raw
    `{{ .vars.x }}` text, so the placeholder form is the meaningful case).

- [ ] **Task 2.7: adopter documentation.** Three doc surfaces (no ADR citations in the template
  edit - the ADR-0082 residue scan bans them):

  1. `templates/docs/working-with-awf.md.tmpl`, in the `config-and-overrides` section, append
     after the paragraph ending "delete the marker line once the part is real.":

     ~~~markdown

     **Authoring comments.** A whole line that is exactly an `awf:comment` HTML comment is
     stripped at render and never reaches output - from template defaults and convention parts
     alike - so parts can carry internal notes and `touches-invariant: <slug>` tags that must
     not ship:

     ```
     <!-- awf:comment an internal note that never renders -->
     ```

     A tag rides the same form: the comment text `touches-invariant: <slug> - <note>` makes
     the line a scannable invariant tag (kept out of the fenced example above deliberately -
     a fenced demo carrying real tag grammar would be recorded by the fence-unaware scanner).
     The rule is whole-line and exact-literal: the line must open with `<!-- awf:comment` and
     end with `-->`. A mid-line occurrence and a whitespace variant render verbatim; a
     whole-line opener that does not end with `-->` is a hard render error naming the part or
     template. Fenced code blocks are preserved, so examples like the one above are safe. A
     part whose only content is authoring comments strips to an empty body and renders its
     section empty (the pointer stays) rather than falling back to the default. To scan such
     tags, point an `invariants.sources` entry at your parts with
     `marker: '<!-- awf:comment'` and `close: '-->'`; in a fenced demo, break the tag token so
     the fence-unaware scanner does not record it. Inside an include partial, comment text must
     avoid the `awf:include`/`awf:section`/`awf:end` substrings, and comment text is ordinary
     prose to every other check that reads the source (a punctuation gate included, where one
     is on).
     ~~~

  2. `.awf/docs/parts/architecture/data-flow.md`, append as a new final paragraph:

     ```markdown

     Both seams strip **whole-line authoring comments** first (ADR-0121): a line that is exactly
     an `<!-- awf:comment ... -->` HTML comment is removed from template source (after include
     expansion) and from part bodies (before placeholder substitution), so parts and templates
     carry `touches-invariant:` tags and internal notes that never render. Fenced examples are
     preserved; a malformed whole-line opener is a hard render error; in-place regions
     (ADR-0100), read back from output, are never stripped. The lock hashes see the unstripped
     bytes, so a comment-only edit reflags stale and self-settles.
     ```

  3. `.awf/docs/glossary.yaml`, add to `data.terms` (alphabetical position):

     ```yaml
     "authoring comment": "A whole line that is exactly an `<!-- awf:comment ... -->` HTML comment in a template default or convention part (ADR-0121): stripped at source ingestion with its newline, so it never reaches rendered output. Exact-literal and whole-line-only (mid-line and whitespace-variant forms render verbatim; a malformed whole-line opener is a hard render error; fenced demos are preserved). The carrier for `touches-invariant:` tags in parts and templates, scanned via an `invariants.sources` entry with `marker: '<!-- awf:comment'` and `close: '-->'`."
     ```

- [ ] **Task 2.8: verify and commit.** `./x sync` (re-renders `docs/working-with-awf.md`,
  `docs/architecture.md`, `docs/glossary.md`, and the sundial example's copies of the edited
  embedded template), then `./x gate` (green; dangling-marker notes now cover the four new
  render slugs plus phase 1's, all clearing at the flip). Stage the source files from tasks
  2.1-2.7 plus every rendered file `./x sync` reported, and commit:

  ```commit
  feat(rendering): strip whole-line awf:comment authoring comments
  ```

## Phase 3: dogfood wiring and retro-tagging

- [ ] **Task 3.1: config entry.** In `.awf/config.yaml`, extend `invariants.sources`:

  ```yaml
  invariants:
    disabled: false
    sources:
      - globs:
          - '**/*.go'
        marker: //
      - globs:
          - '.awf/**/parts/**/*.md'
          - 'templates/**'
        marker: '<!-- awf:comment'
        close: '-->'
    testGlobs:
      - '**/*_test.go'
  ```

  Parts-scoped deliberately: `.awf/memory/` must never carry markers that count
  (ADR-0121 Decision 6).

- [ ] **Task 3.2: retro-tag parts and templates (batch).** Add `touches-invariant` authoring
  comments where existing content narrates a declared invariant. One rationale, mechanical
  per-site variation; every tag names a slug declared by an Implemented ADR (never one of
  ADR-0121's own not-yet-owed slugs) and carries a non-empty note.

  Representative site - prepend to `.awf/domains/parts/rendering/current-state.md` as its first
  lines:

  ```
  <!-- awf:comment touches-invariant: section-default-splice - the sectionDefault splice narrated below -->
  <!-- awf:comment touches-invariant: part-scopes-in-confighash - the placeholder confighash folding narrated below -->
  ```

  Edge site (an embedded template: slug-only, no ADR citation, generic prose) - prepend to the
  invariant-guidance section body of `templates/adr-readme/README.md.tmpl` (first line inside
  the section whose default renders the marker table):

  ```
  <!-- awf:comment touches-invariant: invariant-markers-derived - this guidance renders the sources-derived marker mapping -->
  ```

  Affected-site set (exhaustive):

  | Site | Tag(s) |
  |---|---|
  | `.awf/domains/parts/rendering/current-state.md` | `section-default-splice`, `part-scopes-in-confighash` (representative above) |
  | `.awf/domains/parts/invariants/current-state.md` | `proof-marker-test-scoped`, `bare-touches-note` |
  | `.awf/parts/adr-readme/invariants.md` | `invariant-markers-derived`, `invariant-markers-in-confighash` |
  | `templates/adr-readme/README.md.tmpl` | `invariant-markers-derived` (edge above) |
  | `templates/docs/working-with-awf.md.tmpl` (top of the `sync-and-drift` section body) | `in-place-tamper-drift` |

  Each non-representative tag line follows the exact shape
  `<!-- awf:comment touches-invariant: <slug> - <short note naming what below narrates it> -->`.

  Post-check (all four must hold):

  1. `go run ./cmd/awf context .awf/domains/parts/rendering/current-state.md` lists
     `section-default-splice` and `part-scopes-in-confighash` under Invariants with their notes;
  2. `./x check 2>&1 | grep -c 'names a slug no Implemented ADR declares'` prints exactly `5`
     (the five ADR-0121 slugs carrying markers so far - the successor slug's markers arrive
     with the phase 4 rename; no retro tag added a dangling slug);
  3. `./x check 2>&1 | grep -c 'carries no note'` prints `0`;
  4. `git grep -h 'awf:comment touches-invariant' -- .awf templates | wc -l` prints `8`, and
     `git grep -l 'awf:comment touches-invariant' -- .awf templates` lists exactly the five
     files in the site table.

- [ ] **Task 3.3: verify and commit.** `./x sync` (the sources edit re-renders the
  invariant-marker tables in `docs/decisions/README.md` and every artifact folding
  `invariantMarkers` into its ConfigHash; the tagged templates re-render their artifacts with
  the tags stripped, so output is unchanged), `./x gate` green, stage `.awf/config.yaml`,
  the five tagged files, and every rendered file sync reported, and commit:

  ```commit
  chore(config): dogfood authoring-comment invariant tagging
  ```

## Phase 4: succession bookkeeping and the flip

- [ ] **Task 4.1: rename the `parts-raw` markers.** Rename to the successor slug
  (ADR-0121 Decision 4):

  - `internal/render/render_test.go` (~line 319): `// invariant: parts-raw` becomes
    `// invariant: parts-raw-except-authoring-comments`; extend the covering test with a case
    asserting a part body containing literal `{{ }}` text renders the braces byte-for-byte
    (the strip runs upstream of the render layer, so this test's contract is unchanged; the
    strip-plus-verbatim composition is proven by the task 2.6 project-level test).
  - `internal/render/render.go` (~line 253, the `Execute` doc comment): the
    `touches-invariant: parts-raw - ...` marker line becomes
    `touches-invariant: parts-raw-except-authoring-comments - part bodies restored verbatim post-strip, never templated; proof in render_test.go`.

  Post-check: `git grep -nE 'invariant: parts-raw( |$)' -- '*.go'` prints nothing (a `\b`
  word-boundary grep would still match the successor slug at its `w`/`-` boundary, so the
  space-or-end alternation is the one that goes quiet).

- [ ] **Task 4.2: successor tag and prose in the rendering narrative.** Append to the
  authoring-comment tag block at the top of `.awf/domains/parts/rendering/current-state.md`
  (from task 3.2):

  ```
  <!-- awf:comment touches-invariant: parts-raw-except-authoring-comments - the verbatim-parts contract and its carve-outs narrated below -->
  ```

  And update that part's prose: in the sentence run beginning "Convention parts are raw input
  (ADR-0034):", immediately after the text ending "and is never variable-interpolated.",
  insert:

  ```markdown
  ADR-0121 adds the one removal to the substitutions: whole-line `awf:comment` authoring
  comments are stripped from part bodies (and template sources) at ingestion, retiring
  `parts-raw` for `parts-raw-except-authoring-comments`. Comment text in embedded templates
  stays subject to the ADR-0082 residue scan (invariant slugs only, no ADR citations or
  repo-identity literals) and the plain-punctuation gates.
  ```

- [ ] **Task 4.3: invariants and config domain narratives.** Append to
  `.awf/domains/parts/invariants/current-state.md` as a new final paragraph:

  ```markdown
  ADR-0121 widens the scan surface to prose: an `invariants.sources` entry may declare a
  `close:` token (`-->`, `*/`) stripped once from a matched marker line's end before tag
  parsing, in both the backing scan and the `MarkersUnder` context query, so block-comment
  markers yield clean touches notes and a note-less touches marker still fires the
  bare-touches advisory. awf's own config points a `<!-- awf:comment` marker at
  `.awf/**/parts/**/*.md` and `templates/**`, so parts and templates carry
  `touches-invariant` tags that `awf context` surfaces and the render strip keeps out of
  output.
  ```

  Append to `.awf/domains/parts/config/current-state.md` as a new final paragraph:

  ```markdown
  `invariants.sources[].close` (ADR-0121) is the schema's newest optional key: a literal close
  token for block-comment markers, additive with no schema-generation bump (the `testGlobs`
  precedent) - empty or absent means no stripping.
  ```

- [ ] **Task 4.4: changelog.** In `changelog/CHANGELOG.md` under `## [Unreleased]` /
  `### Features`, add (creating the category heading if absent, ordered per the file's
  category order):

  ```markdown
  - Whole-line `<!-- awf:comment ... -->` authoring comments in templates and convention parts
    (ADR-0121): stripped at render with their newline, so parts and templates can carry
    internal notes and `touches-invariant:` tags that never reach rendered output. Whole-line
    and exact-literal only (mid-line and whitespace-variant forms still render; a malformed
    whole-line opener is a hard render error naming the source; fenced demos are preserved).
    `invariants.sources` entries gain an optional `close:` token (`-->`, `*/`) stripped from
    marker lines before tag parsing, so block-comment-family markers - the new tagging recipe
    included - yield clean touches notes.
  ```

- [ ] **Task 4.5: flip and close.** Edit
  `docs/decisions/0121-whole-line-authoring-comments-in-templates-and-parts.md` frontmatter
  `status: Proposed` to `status: Implemented`, and this plan's frontmatter to
  `status: Implemented`; append any implementation findings (deviations, discoveries) to this
  plan's Notes section in the same flip commit (the plan freezes at Implemented). Run `./x sync` (regenerates `docs/decisions/ACTIVE.md` and
  `docs/domains/{rendering,invariants,config}.md`; the retirement activates and the six new
  slugs become owed). Run `./x gate`: green, and
  `./x check 2>&1 | grep -c 'names a slug no Implemented ADR declares'` now prints `0`.
  Stage everything from tasks 4.1-4.5 plus the regenerated files and commit:

  ```commit
  docs(adr): implement 0121 whole-line authoring comments
  ```

## Verification

- `./x gate` green at every phase boundary; after phase 4, `./x check` and `./x invariants`
  emit no dangling-marker or bare-touches notes.
- `go run ./cmd/awf context .awf/domains/parts/rendering/current-state.md templates/adr-readme`
  surfaces the retro tags with notes.
- No whole-line directive survives into rendered output:
  `grep -rn '^<!-- awf:comment' docs/ .claude/ AGENTS.md` prints exactly one hit - the fenced
  demo line in `docs/working-with-awf.md` (fences are preserved by design) - and nothing else.
- A manual smoke: add `<!-- awf:comment smoke -->` as a whole line to any consumed part, run
  `./x sync`, confirm the rendered artifact is byte-identical, then remove the line and
  re-sync (the lock settles both times).

## Notes

- Five of the six new proof markers dangle (non-failing notes) from their landing phase until
  the phase 4 flip; the successor slug's markers land in the flip commit itself. Expected and
  listed per phase.
- The `supersedes-invariant: ADR-0034#parts-raw` retirement and the successor declaration
  activate together at the flip, so no window opens where `parts-raw` is owed but its markers
  are renamed away: renames land in the same commit as the flip.
- After phase 4, ADR-0120's same-anchor advisory notes `ADR-0034#1` claimed by ADR-0057 and
  ADR-0121 - accepted, per ADR-0121's Consequences.

Implementation findings (recorded at the flip):

- `internal/project` had no pre-existing `render_test.go`; the file was created rather than
  extended. The task 2.6 var-consumption case landed in `unused_test.go` beside
  `TestPartPlaceholderConsumesVar` (its fixture home) instead of `drift_test.go`; the
  scope-folding case landed in `drift_test.go` as planned.
- `trimClose`'s second parameter is named `closeTok`, not `close`: the linter's `predeclared`
  rule rejects shadowing the builtin.
- Phase 1 needed a follow-up commit (`d349de7a`): the configspec entry also regenerates the
  sundial example's config reference and lock, which the closing commit had not staged.
- All pinned post-checks passed at their stated values (5 dangling notes through phases 2-3,
  8 tag lines across 5 files, context surfacing both representative tags).
