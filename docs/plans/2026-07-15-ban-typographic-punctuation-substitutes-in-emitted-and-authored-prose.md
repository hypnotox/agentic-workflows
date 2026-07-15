---
date: 2026-07-15
adrs: [115, 117]
status: Proposed
---
# Plan: Ban typographic punctuation substitutes in emitted and authored prose

## Goal

Land ADR-0115 (a hard gate banning seven typographic punctuation substitutes from the prose awf
ships) and ADR-0117 (an advisory `awf audit` rule warning when authored prose adds them), in that
order. The two are planned together because both rewrite the same line,
`templates/docs/doc-standard.md.tmpl:16`, and ADR-0117 Decision item 8 depends on ADR-0115 landing
first. Because both land in one effort, that line is written **once**, carrying both widenings,
exactly as item 8 directs; phase 6 does it. Planned apart, or landed in the reverse order, an
implementer following ADR-0115 item 9 literally would restore the narrow scope clause and silently
undo the widening.

Non-goals: the 2344 em-dashes in authored ADR bodies and the 4347 under `docs/plans/` stay untouched
(out of scope permanently, per ADR-0115 Consequences and ADR-0117 Decision item 2), and no exemption
mechanism ships.

The seven banned codepoints, named by word and codepoint per the convention this plan establishes:
the em-dash (U+2014), the en-dash (U+2013), the ellipsis (U+2026), and the four curly quotes
(U+2018, U+2019, U+201C, U+201D).

**Authoring convention used throughout this plan.** Tasks name a banned codepoint by word and
codepoint and give the exact **replacement** text, rather than a before/after diff that types the
glyph. Whole-line replacement at a named line is unambiguous, and every task carries a post-check
that fails if a site is left unconverted, so completeness is proved mechanically rather than by
reading a diff.

## Architecture summary

Six phases land ADR-0115, then a seventh lands ADR-0117. Every phase's closing commit passes
`./x gate` on its own.

ADR-0115's gate test scans three surfaces at once (the embedded `templates.FS`, the embedded
`changelog.FS`, and every string literal in production Go under `internal/` and `cmd/`), so all
three must already be clean when it lands. Phases 1 to 3 clean two of them plus the one behavioural
change. Phase 4 cleans the third **and lands the test in the same commit**. No *grep* can
post-check the Go surface (a whole-file grep over non-test Go returns 95 hits where only 79 are in
string literals, the other 16 sitting in comments that ADR-0115 Decision item 4 excludes
deliberately), so the only mechanical check that fits is an AST walk, which is precisely what the
test is. Splitting the phase would therefore land 31 files of prose edits in a commit whose
completeness nothing committed ever proves, and ADR-0115's Invariants section independently
requires the old-test deletion and the new-test addition to be atomic.

Phase 5 retitles three ADRs (ADR-0115 Decision item 5). Phase 6 rewrites the documentation
standard's plain-punctuation rule and flips ADR-0115 to `Implemented`. That one line carries both
widenings at once, the codepoint list (ADR-0115 item 9) and the scope clause (ADR-0117 item 8),
which is what item 8 directs when both ADRs land in one effort, under a single changelog entry. It
lands before the rule that enforces it, satisfying ADR-0117 Decision item 5: awf states the
convention in a standard the adopter renders before it warns about it, and that ordering is the
whole argument. Phase 7 then ships the advisory rule and flips ADR-0117.

## File structure

- **Created:** none.
- **Modified:** `templates/agents/adr-reviewer.md.tmpl`,
  `templates/skills/proposing-adr/SKILL.md.tmpl`,
  `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`, `templates/skills/bugfix/SKILL.md.tmpl`,
  `templates/docs/workflow.md.tmpl`, `templates/partials/review-spine-tail.md`,
  `templates/adr-readme/README.md.tmpl`, `templates/docs/doc-standard.md.tmpl`,
  `changelog/CHANGELOG.md`, `.awf/domains/parts/adr-system/current-state.md`,
  `.awf/domains/parts/tooling/current-state.md`, `internal/adr/adr.go`, `internal/adr/adr_test.go`,
  `internal/project/residue_scan_test.go`, the 31 production Go files phase 4 names, the six test
  files phase 4 names (`cmd/awf/context_test.go`, `cmd/awf/config_test.go`, `cmd/awf/init_test.go`,
  `internal/project/notes_test.go`, `internal/project/sweep_test.go`,
  `internal/initspec/initspec_test.go`),
  `internal/audit/audit.go`, `internal/audit/settings.go`, `internal/audit/settings_test.go`,
  `internal/audit/audit_test.go`, `internal/config/config.go`, `internal/configspec/spec.go`,
  `internal/project/configreference.go`, `internal/project/project.go`,
  `docs/decisions/0007-invariant-backing-tooling.md`,
  `docs/decisions/0018-documentation-authoring-standard.md`,
  `docs/decisions/0022-curated-init-default.md`,
  `docs/decisions/0115-ban-typographic-punctuation-substitutes-in-emitted-prose.md`,
  `docs/decisions/0117-advisory-plain-punctuation-audit-rule-for-authored-prose.md`, plus every
  rendered artifact `./x sync` regenerates (`docs/decisions/ACTIVE.md`, `docs/domains/*.md`,
  `docs/doc-standard.md`, `docs/config-reference.md`, `.claude/**`, `AGENTS.md`, and the matching
  files under `examples/sundial/`).
- **Deleted:** none. (`TestTemplateNoEmDash` and the `emDash` const are removed from
  `internal/project/residue_scan_test.go` in phase 4; the file stays.)

## Phase 1: Clean the shipped templates

- [ ] **Task 1.1: Replace the ten ellipses and two en-dashes in `templates/`.** Twelve sites across
  seven files.

  `templates/agents/adr-reviewer.md.tmpl:21` (one ellipsis, U+2026): the fragment
  `("we should prefer ...")` replaces the same fragment written with an ellipsis. Change nothing
  else on the line.

  `templates/skills/proposing-adr/SKILL.md.tmpl:61` (three ellipses, U+2026): the first fragment
  becomes ``a backed ``- `invariant: <slug>`, ...`` for a property``, and the second becomes
  ``an ``- `unbacked-invariant: <slug>`, ... **Verify:** ...`` for a reasoned contract``. Note the
  second drops the sentence period that followed its first ellipsis, so the result reads `...` and
  not four dots. Change nothing else on the line.

  `templates/skills/refactor-coupling-audit/SKILL.md.tmpl:33` (one en-dash, U+2013): the fragment
  becomes `(1-3 files)`. Change nothing else on the line.

  `templates/partials/review-spine-tail.md:24` (one en-dash, U+2013): the fragment becomes
  `(range 50-100 words)`. Change nothing else on the line.

  `templates/docs/workflow.md.tmpl:54` becomes, in full:

  ```markdown
  - an existing hook manager (husky, lefthook, ...): call the payload from its config.
  ```

  `templates/skills/bugfix/SKILL.md.tmpl:15` (one ellipsis, U+2026): the fragment becomes
  `` `brainstorming -> ... -> implementation` ``, keeping the two rightwards arrows (U+2192) exactly
  as they are; they are notation and stay legal (ADR-0115 Decision item 2). Change nothing else on
  the line.

  `templates/skills/bugfix/SKILL.md.tmpl:31` (one ellipsis, U+2026): the fragment becomes
  `` typically `fix(<scope>): ...`; ``. Change nothing else on the line.

  `templates/adr-readme/README.md.tmpl:59-60` (three ellipses, U+2026) become, in full:

  ```markdown
  backed ``- `invariant: <slug>`: ...`` for a property a test is declared to back, or an
  ``- `unbacked-invariant: <slug>`: ... **Verify:** ...`` for a reasoned contract with no automatic test.
  ```

  Post-check, which must print `0`:

  ```
  grep -rlP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' templates/ | wc -l
  ```

- [ ] **Task 1.2: Re-render, verify, and commit.** Run `./x sync` (this rewrites the rendered
  skills, agents, docs, `AGENTS.md`, and the `examples/sundial` copies), then `./x check` (expect
  `check: clean`, with the advisory note that the `template-em-dash-free` marker names a slug no
  Implemented ADR declares; that note is expected until phase 6, per ADR-0115's Invariants section),
  then `./x gate` (expect it to pass). Stage the seven template files and every file the sync
  changed, then commit:

  ```commit
  docs(rendering): use plain punctuation in shipped templates
  ```

## Phase 2: Clean the embedded changelog

- [ ] **Task 2.1: Replace the 103 em-dashes and 5 ellipses in `changelog/CHANGELOG.md`.** A batch
  task. `awf changelog` prints this file verbatim to adopters (`cmd/awf/changelog.go:38`), so it is
  emitted prose. Rewriting entries for already-released versions is deliberate and accepted
  (ADR-0115 Consequences): the published GitHub release notes for those tags will diverge. No word
  changes; only punctuation.

  **Transformation.** Replace each em-dash (U+2014) with the punctuation the sentence wants: a colon
  when what follows explains what precedes (the common case), a semicolon when both sides are
  independent clauses, a comma for a light aside, and parentheses for a parenthetical. A pair of
  em-dashes bracketing a clause becomes a pair of parentheses. Never substitute a bare hyphen.
  Replace each ellipsis (U+2026) with three periods.

  **Representative** (`changelog/CHANGELOG.md:14`, an em-dash introducing an explanation): the line
  becomes

  ```markdown
    unified from `inv: <slug>` to `invariant: <slug>`: the same token the source
  ```

  **Edge** (`changelog/CHANGELOG.md:428-429`, a bracketing pair spanning two lines, and `:68`, an
  ellipsis inside code formatting): the em-dash pair opens at the end of line 428 and closes
  mid-line-429, bracketing the `<!-- awf:section ... -->` / `<!-- awf:end -->` examples, and becomes
  a pair of parentheses around those examples, leaving the `which is inert inside a part` clause
  outside them. At line 68 the fragment becomes `` (harmless; still `bash ...`-invoked) ``.

  **Affected-site set**, exhaustively:

  ```
  grep -nP '[\x{2014}\x{2026}]' changelog/CHANGELOG.md
  ```

  **Post-check**, which must print `0`:

  ```
  grep -cP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' changelog/CHANGELOG.md
  ```

- [ ] **Task 2.2: Verify and commit.** Run `./x gate` (expect it to pass; `internal/changelog`'s
  parser tests read this file, so a structurally malformed edit fails here). Stage
  `changelog/CHANGELOG.md` and commit:

  ```commit
  docs(tooling): use plain punctuation in the embedded changelog
  ```

## Phase 3: Parenthesize the ACTIVE.md status suffix

- [ ] **Task 3.1: Change the generated separator to parentheses.** ADR-0115 Decision item 8. In
  `internal/adr/adr.go:131`, inside `RenderActiveMD`, the `fmt.Fprintf` line becomes:

  ```go
  			fmt.Fprintf(&sb, "- [%s](%s) (%s)\n", a.Title, a.Filename, a.Status)
  ```

  A row then renders as `- [ADR-0001: Title](0001-file.md) (Accepted)`, and a superseded row as
  `- [ADR-0003: Title](0003-file.md) (Superseded by ADR-0032)`. A colon is unavailable because ADR
  titles already contain one; a hyphen reads mushily against the list item's own leading hyphen.

- [ ] **Task 3.2: Update the assertion that pins the old separator.** `internal/adr/adr_test.go:130`
  asserts the em-dash form and fails the moment task 3.1 lands. The loop's slice literal becomes:

  ```go
  	for _, entry := range []string{"(Superseded by ADR-0003)", "(Superseded by ADR-0004)"} {
  ```

  This file is a `_test.go` and so is out of the ban's scope (ADR-0115 Decision item 4); it changes
  here only because the behaviour it asserts changed. `internal/adr/domain_test.go` needs no change:
  the domain index renders a rightwards arrow (U+2192) for supersession, which is notation and stays.

- [ ] **Task 3.3: Re-render, verify, and commit.** Run `./x sync` (regenerates
  `docs/decisions/ACTIVE.md`, 117 rows, and `examples/sundial/docs/decisions/ACTIVE.md`, 3 rows;
  both must be staged or `./x check` fails on drift in the example tree), then `./x gate` (expect it
  to pass). Confirm every row converted: `grep -c ') (' docs/decisions/ACTIVE.md` must print `117`,
  matching `grep -c '^- \[ADR-' docs/decisions/ACTIVE.md`. Stage
  `internal/adr/adr.go`, `internal/adr/adr_test.go`, `docs/decisions/ACTIVE.md`, and
  `examples/sundial/docs/decisions/ACTIVE.md`, then commit:

  ```commit
  refactor(adr-system): parenthesize the ACTIVE.md status suffix
  ```

## Phase 4: Clean production Go literals and land the gate

This phase's tasks share one closing commit and are not sliced, for two reasons that are worth
stating precisely. First, the test is the cleanup's only **permanent** completeness proof: an
implementer can rebuild an equivalent AST scan ad hoc (task 4.1 gives the recipe), but a 4a/4b
split would land 31 files of prose edits in a commit that no committed check ever validates, and
the throwaway scan would leave no trace in the tree. Second, ADR-0115's Invariants section requires
the old-test deletion and the new-test addition to be atomic, which pins the swap to a single
commit regardless. Both halves would in fact be gate-green if split, so this is a deliberate choice
about what the history proves, not a mechanical necessity.

- [ ] **Task 4.1: Replace the 70 remaining em-dashes, 3 en-dashes, and 5 ellipses in production Go
  string literals.** A batch task. Scope is string literals only, in non-test files under
  `internal/` and `cmd/`. **Go comments are out of scope and must not be touched**: gofmt's
  doc-comment normalization rewrites a double-backtick pair into U+201C, so cleaning comments would
  pit this gate against gofmt in a loop neither wins (ADR-0115 Decision item 4, recorded in
  `.awf/docs/pitfalls.yaml`). 16 banned codepoints legitimately remain in comments after this task;
  that is correct, not residue.

  **Transformation.** As in task 2.1: an em-dash becomes a colon, semicolon, comma, or parentheses
  as the sentence wants; an en-dash in a numeric range becomes an ASCII hyphen; an ellipsis becomes
  three periods.

  **Representative** (`cmd/awf/context.go:156`, an em-dash separating a title from a path):

  ```go
  			fmt.Fprintf(stdout, "  ADR-%s (%s) %s: %s\n", a.Number, a.Status, a.Title, a.Path)
  ```

  **Edge 1** (`internal/catalog/standard.go:131`, an en-dash inside a numeric range in an embedded
  markdown fragment): the fragment becomes `<1-2 headlines>`. The same shape applies at
  `internal/catalog/standard.go:148` and `:168`.

  **Edge 2** (`internal/configspec/spec.go:103`, an ellipsis marking elision in a prose
  description): the fragment becomes `(architecture, testing, development, ...)`. The same shape
  applies at `spec.go:143`, `:168`, `:182`, and `:223`. Descriptions here must cite no ADR number
  (`TestConfigspecDescriptionResidue`), so change punctuation only.

  **Affected-site set.** The 31 files below, from the AST scan that produced ADR-0115's
  authoritative counts. `internal/adr/adr.go` also carries one, but phase 3 already cleaned it, so
  it is absent here.

  ```
  cmd/awf/audit.go          cmd/pincheck/main.go               internal/project/check.go
  cmd/awf/check.go          cmd/releasecheck/main.go           internal/project/glossary.go
  cmd/awf/config.go         cmd/repoaudit/main.go              internal/project/install.go
  cmd/awf/context.go        internal/audit/audit.go            internal/project/pitfalls.go
  cmd/awf/dispatch.go       internal/catalog/standard.go       internal/project/render.go
  cmd/awf/init.go           internal/clispec/clispec.go        internal/project/sweep.go
  cmd/awf/list_add.go       internal/configspec/spec.go        internal/project/validate.go
  cmd/awf/main.go           internal/initspec/initspec.go      internal/render/render.go
  cmd/awf/new.go            internal/invariants/invariants.go  internal/render/section.go
  cmd/covercheck/main.go    internal/manifest/manifest.go
  cmd/mutants/main.go       internal/migrate/pitfalls.go
  ```

  Do not re-derive this set with a whole-file grep: it sweeps in the out-of-scope comments and
  reports 95 hits across more files instead of 79 across these. If the set must be rebuilt, walk
  `internal/` and `cmd/`, skip `_test.go`, parse each file with `go/parser`, and inspect every
  `*ast.BasicLit` of kind `STRING`, which is exactly what task 4.2's test then does permanently.

  **Two of these files feed rendered output, so this task creates drift that task 4.3 must sync.**
  `internal/catalog/standard.go` carries the reviewer `digestSummary` data that renders through
  `templates/partials/review-spine-tail.md` into `.claude/agents/adr-reviewer.md`,
  `.claude/agents/plan-reviewer.md`, `.claude/agents/code-reviewer.md`, their `.cursor/agents/`
  copies, and the `examples/sundial/.claude/agents/` copies. `internal/configspec/spec.go` carries
  the descriptions that render into `docs/config-reference.md` and
  `examples/sundial/docs/config-reference.md`.

  **Post-check:** task 4.2's test, run as
  `go test ./internal/project/ -run TestEmittedProseNoTypographicSubstitutes`, which must pass.

- [ ] **Task 4.1b: Update the test assertions that mirror the cleaned literals.** Six test files
  assert these production strings verbatim and fail the moment task 4.1 lands. Task 4.2's test
  cannot catch them (it skips `_test.go` by design), so `./x gate` in task 4.3 is their only
  arbiter; they are enumerated here rather than left to be discovered.

  **The rule.** An assertion that mirrors a production string changed in task 4.1 changes with it,
  to match the new punctuation exactly. A test-owned fixture, input, or failure message keeps its
  glyph untouched: `_test.go` is out of the ban's scope (ADR-0115 Decision item 4), and these files
  change here only because the production behaviour they assert changed.

  - `cmd/awf/context_test.go:94,95` (mirror `cmd/awf/context.go:137`), `:102,104` (mirror
    `context.go:156,162`), `:107` (mirrors `context.go:168`), `:109` (mirrors `context.go:174`).
    **Leave `:56,61,62` and `:100` alone**: those are fixture text and the output derived from it,
    not a production separator.
  - `internal/project/notes_test.go:176,177,202,216,233` (mirror `internal/project/check.go:192`)
    and `:260` (mirrors the marker-shaped-line note in `check.go`). **Leave `:370` alone**: it is a
    `t.Errorf` failure message.
  - `internal/project/sweep_test.go:22,23` (mirror `internal/project/sweep.go:152` and the
    stale-backup detail). **Leave `:35` alone**: it is fixture content.
  - `cmd/awf/config_test.go:27` (mirrors `cmd/awf/config.go:28`).
  - `internal/initspec/initspec_test.go:276,279` (mirror `internal/initspec/initspec.go:248,293`).
  - `cmd/awf/init_test.go:198` (mirrors the same `initspec.go` format string, via the `gateCmd` key).

  **Post-check:** `./x gate` in task 4.3. To confirm none was missed, no test may assert a string
  that production no longer emits; the gate fails loudly and by name if one does.

- [ ] **Task 4.2: Swap the gate test.** In `internal/project/residue_scan_test.go`, delete the
  `emDash` const (lines 81 to 84), `TestTemplateNoEmDash`, and its
  `// invariant: template-em-dash-free` marker, and replace them with the block below. Add `go/ast`,
  `go/parser`, `go/token`, `path/filepath`, and `strconv` to the import list, plus
  `changelogfs "github.com/hypnotox/agentic-workflows/changelog"` (the alias every other importer of
  that package uses; `changelog` is a leaf package, so there is no import cycle). `io/fs`,
  `strings`, and `templates` are already imported and all stay in use.

  ```go
  // bannedRunes are the seven typographic punctuation substitutes banned from
  // emitted prose (ADR-0115). Each key is written as an escape so this file states
  // the rule without typing the glyphs it bans. Notation (arrows, mathematical
  // symbols, accented letters) is deliberately absent: this is a closed blocklist
  // of substitutes for ASCII punctuation, never an ASCII-only allowlist.
  var bannedRunes = map[rune]string{
  	'\u2014': "em-dash (U+2014)",
  	'\u2013': "en-dash (U+2013)",
  	'\u2026': "ellipsis (U+2026)",
  	'\u2018': "left single quote (U+2018)",
  	'\u2019': "right single quote (U+2019)",
  	'\u201c': "left double quote (U+201C)",
  	'\u201d': "right double quote (U+201D)",
  }

  // scanEmbedded reports every banned rune in every file of an embedded FS and
  // returns the number of files inspected, at most one report per rune per file.
  func scanEmbedded(t *testing.T, label string, fsys fs.FS) int {
  	t.Helper()
  	seen := 0
  	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
  		if err != nil {
  			return err
  		}
  		if d.IsDir() {
  			return nil
  		}
  		seen++
  		b, err := fs.ReadFile(fsys, path)
  		if err != nil {
  			return err
  		}
  		flagged := map[rune]bool{}
  		for _, r := range string(b) {
  			if name, bad := bannedRunes[r]; bad && !flagged[r] {
  				flagged[r] = true
  				t.Errorf("%s: %s contains the %s; emitted prose uses plain punctuation (ADR-0115)", label, path, name)
  			}
  		}
  		return nil
  	})
  	if err != nil {
  		t.Fatal(err)
  	}
  	return seen
  }

  // scanGoLiterals reports every banned rune in a string literal of every non-test
  // Go file under dir and returns the number of files inspected. Comments are
  // deliberately not inspected: gofmt rewrites a double-backtick pair into U+201C,
  // so scanning them would pit this gate against gofmt (ADR-0115 Decision item 4).
  func scanGoLiterals(t *testing.T, dir string) int {
  	t.Helper()
  	seen := 0
  	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
  		if err != nil {
  			return err
  		}
  		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
  			return nil
  		}
  		seen++
  		fset := token.NewFileSet()
  		file, perr := parser.ParseFile(fset, path, nil, 0)
  		if perr != nil {
  			t.Fatalf("parse %s: %v", path, perr)
  		}
  		ast.Inspect(file, func(n ast.Node) bool {
  			lit, ok := n.(*ast.BasicLit)
  			if !ok || lit.Kind != token.STRING {
  				return true
  			}
  			val, uerr := strconv.Unquote(lit.Value)
  			if uerr != nil {
  				val = lit.Value
  			}
  			flagged := map[rune]bool{}
  			for _, r := range val {
  				if name, bad := bannedRunes[r]; bad && !flagged[r] {
  					flagged[r] = true
  					t.Errorf("%s:%d: string literal contains the %s; emitted prose uses plain punctuation (ADR-0115)",
  						path, fset.Position(lit.Pos()).Line, name)
  				}
  			}
  			return true
  		})
  		return nil
  	})
  	if err != nil {
  		t.Fatal(err)
  	}
  	return seen
  }

  // TestEmittedProseNoTypographicSubstitutes scans the three surfaces awf ships:
  // the embedded template FS, the embedded changelog FS, and every string literal
  // in production Go under internal/ and cmd/. Each surface carries a seen-count
  // guard, so a mis-anchored walk fails rather than passing vacuously. Adopter
  // content, and this repository's authored ADR and plan bodies, are out of scope:
  // the ban covers what awf ships, in awf's own voice (ADR-0115).
  // invariant: emitted-prose-no-typographic-substitutes
  func TestEmittedProseNoTypographicSubstitutes(t *testing.T) {
  	if n := scanEmbedded(t, "templates", templates.FS); n < 40 {
  		t.Fatalf("inspected only %d embedded template file(s); expected the whole tree - did the FS move?", n)
  	}
  	if n := scanEmbedded(t, "changelog", changelogfs.FS); n < 1 {
  		t.Fatalf("inspected only %d embedded changelog file(s); expected CHANGELOG.md - did the embed move?", n)
  	}
  	goFiles := 0
  	for _, dir := range []string{"../../internal", "../../cmd"} {
  		goFiles += scanGoLiterals(t, dir)
  	}
  	if goFiles < 60 {
  		t.Fatalf("inspected only %d production Go file(s) under internal/ and cmd/; expected the whole tree - did the anchor move?", goFiles)
  	}
  }
  ```

  The rune keys are written as escapes so the test states the rule without typing the glyphs it
  bans. The guards are floors, not counts: the tree currently holds 53 embedded template files, 1
  embedded changelog file, and 86 non-test Go files under `internal/` and `cmd/`. The `../../`
  anchor is relative to the package directory, which is where `go test` runs; the precedent for both
  the walk-and-parse shape and the vacuous-pass guard is `internal/testsupport/deps_test.go`. The
  test adds no production function, so neither the dead-code gate nor the coverage gate is affected.

- [ ] **Task 4.3: Re-render, verify, and commit.** Run `./x sync` first: task 4.1 changed two
  render *sources*, so skipping it leaves `./x check` reporting drift rather than `check: clean`.
  The sync regenerates `.claude/agents/adr-reviewer.md`, `.claude/agents/plan-reviewer.md`,
  `.claude/agents/code-reviewer.md`, their `.cursor/agents/` copies, `docs/config-reference.md`,
  and the `examples/sundial/` copies of all of them.

  Then run `go test ./internal/project/ -run TestEmittedProseNoTypographicSubstitutes` (expect
  `ok`), then `./x gate` (expect it to pass; it fails here if task 4.1b missed an assertion, in
  which case fix it and re-run), then `./x check` (expect `check: clean` with one advisory note,
  now naming `emitted-prose-no-typographic-substitutes` as a slug no Implemented ADR declares). The
  note is expected until phase 6: its subject simply moves from the deleted marker to the new one,
  and the ban stays enforced throughout because the test runs in the gate regardless of its ledger
  status. Stage the 31 files from task 4.1, the six test files from task 4.1b,
  `internal/project/residue_scan_test.go`, and every file the sync regenerated, then commit:

  ```commit
  refactor(awf): gate plain punctuation in emitted prose
  ```

## Phase 5: Normalize the three em-dashed ADR titles

- [ ] **Task 5.1: Retitle ADR-0007, ADR-0018, and ADR-0022.** ADR-0115 Decision item 5. This is an
  elective, maintainer-approved carve-out from the append-only invariant, bounded by five
  conditions: heading line only, the seven codepoints only, every word preserved, explicit approval,
  and never a licence to edit an ADR's argument. **Change the `# ` heading line and nothing else in
  these files.** The headings at `docs/decisions/0007-invariant-backing-tooling.md:10`,
  `docs/decisions/0018-documentation-authoring-standard.md:10`, and
  `docs/decisions/0022-curated-init-default.md:10` become, in full:

  ```markdown
  # ADR-0007: Invariant-Backing Tooling: `inv:` Tags and the `awf invariants` Checker
  ```

  ```markdown
  # ADR-0018: Documentation Authoring Standard: `doc-standard.md` and `agents-md-standard.md`
  ```

  ```markdown
  # ADR-0022: Curated Init Default: Workflow-Core Targets
  ```

  Post-check, which must print `0`:

  ```
  grep -lP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' docs/decisions/0007-*.md docs/decisions/0018-*.md docs/decisions/0022-*.md | wc -l
  ```

  It passes only because these three ADR bodies happen to carry no other banned codepoint. It is a
  check on these three files, never a licence to clean any other ADR body.

- [ ] **Task 5.2: Re-render, verify, and commit.** Run `./x sync` (the titles are harvested into
  `docs/decisions/ACTIVE.md` and into `docs/domains/config.md`, `docs/domains/invariants.md`,
  `docs/domains/rendering.md`, and `docs/domains/tooling.md`), then `./x gate` (expect it to pass).
  Stage the three ADR files and the five regenerated files, then commit:

  ```commit
  docs(adr): normalize punctuation in three ADR titles
  ```

## Phase 6: State the widened rule and flip ADR-0115

- [ ] **Task 6.1: Rewrite the documentation standard's plain-punctuation rule, once, carrying
  both widenings.** This single edit discharges **ADR-0115 Decision item 9** (the codepoint list
  widens from one to seven) and **ADR-0117 Decision item 8** (the scope clause widens from shipped
  prose to all awf-managed prose). Item 8 directs exactly this when both ADRs land in one effort:
  "the line is written once, carrying both, under a single changelog entry".
  `templates/docs/doc-standard.md.tmpl:16` becomes, in full:

  ```markdown
  - **Plain punctuation.** Every awf-managed doc, shipped or authored (ADRs, plans, and hand-written docs), uses plain punctuation: a colon, semicolon, comma, or parentheses where an em-dash would go, an ASCII hyphen for a range, three periods for elision, and ASCII quotes for quoting. Seven typographic substitutes are banned, as they read as machine-set: the em-dash (U+2014), en-dash (U+2013), ellipsis (U+2026), and the four curly quotes (U+2018, U+2019, U+201C, U+201D). Notation (arrows, mathematical symbols, accented letters) is unaffected.
  ```

  Writing it here, rather than after the audit rule, is what ADR-0117 Decision item 5 requires: awf
  states the convention in a standard the adopter renders **before** the rule warns about it, and
  phase 7's rule then enforces what this line already says. Two constraints bind the text, both from
  guards over `templates.FS`: it must type none of the glyphs it bans (task 4.2's test scans this
  template), and it must cite no ADR number (`TestTemplateSourceResidue`), which is why the line
  attributes nothing.

- [ ] **Task 6.2: Add the changelog entry.** This is an adopter-facing release: the ACTIVE.md
  separator change makes every adopter's committed `ACTIVE.md` drift against the new binary, so
  their `awf check` fails until they re-sync. Add to the `### Breaking changes` list under
  `## [Unreleased]` in `changelog/CHANGELOG.md`, in plain punctuation (task 4.2's test now scans
  this file):

  This is the **single changelog entry** ADR-0117 Decision item 8 calls for: it covers both the ban
  and the documentation standard's rewritten rule, because task 6.1 wrote that line once carrying
  both widenings.

  ```markdown
  - Seven typographic punctuation substitutes are banned from the prose awf ships (ADR-0115): the
    em-dash (U+2014), en-dash (U+2013), ellipsis (U+2026), and the four curly quotes (U+2018,
    U+2019, U+201C, U+201D). The generated `docs/decisions/ACTIVE.md` now renders a row's status in
    parentheses (`- [ADR-0001: Title](0001-file.md) (Accepted)`) instead of after an em-dash, so
    **every adopter's committed `ACTIVE.md` drifts until they run `awf sync`**, and `awf check`
    reports it until they do. The shipped templates, awf's own output strings, and this changelog
    are cleaned to match. The rendered documentation standard's plain-punctuation rule is rewritten
    to name all seven codepoints and now covers authored prose (ADRs, plans, and hand-written docs)
    as well as shipped prose (ADR-0117), so `docs/doc-standard.md` re-renders too. Nothing rewrites
    prose you have already written. Notation (arrows, mathematical symbols, accented letters) is
    unaffected.
  ```

- [ ] **Task 6.3: Flip ADR-0115 to Implemented.** In
  `docs/decisions/0115-ban-typographic-punctuation-substitutes-in-emitted-prose.md`, the frontmatter
  `status:` line becomes `status: Implemented`. Leave `retires_invariants: []` exactly as it is:
  populating it would fail `awf check` with a dangling retirement, because flipping ADR-0113 to
  `Superseded by ADR-0115` already dropped `template-em-dash-free` from the required set. ADR-0115's
  Invariants section explains this at length; do not "repair" it.

- [ ] **Task 6.4: Record the append-only carve-out in the adr-system narrative.** Docs travel with
  the change, and `.awf/domains/parts/adr-system/current-state.md:3` opens "Decisions live as
  append-only ADRs under `docs/decisions/`", which ADR-0115 Decision item 5 now narrows. Recording
  it here is the point: this narrative is what `awf context` feeds a future agent, and without it
  the next agent reads ADR-0028 and refuses an identical normalization, or reads the retitles as
  licence to edit ADR rationale. Append the following to that paragraph, after the sentence ending
  "drift-checked by a regenerate-and-compare path.":

  ```markdown
  Append-only protects rationale, not orthography (ADR-0115): an Implemented ADR's arguments, decision items, alternatives, and consequences are never rewritten, but a meaning-preserving punctuation fix to its `# ` heading line is normalization rather than revision, and is permitted with explicit maintainer approval; the carve-out reaches the heading only, never the body.
  ```

  This narrative is a convention part, not a rendered artifact, so it is authored directly. The
  other three domains ADR-0115 tags need no refresh, and the reason is worth stating rather than
  leaving as an omission: `rendering`'s narrative describes the render engine, `invariants`' the
  backing machinery, and `tooling`'s the commands, and none of the three asserts anything about
  prose punctuation, the ban's scope, or the ACTIVE.md row format.

- [ ] **Task 6.5: Re-render, verify, and commit.** Run `./x sync` (regenerates
  `docs/doc-standard.md`, `examples/sundial/docs/doc-standard.md`, `docs/decisions/ACTIVE.md` for
  the status change, the domain docs, and the sundial ACTIVE.md), then `./x check` (expect
  `check: clean`, and **the advisory note about an unbacked slug is now gone**, because ADR-0115 is
  Implemented and `TestEmittedProseNoTypographicSubstitutes` backs its slug), then `./x invariants`
  (expect `emitted-prose-no-typographic-substitutes` reported as backed, and `template-em-dash-free`
  reported nowhere), then `./x gate` (expect it to pass). Stage
  `templates/docs/doc-standard.md.tmpl`, `changelog/CHANGELOG.md`, the ADR,
  `.awf/domains/parts/adr-system/current-state.md`, and every regenerated file, then commit:

  The subject cites both ADRs because this commit discharges an item from each: it flips ADR-0115
  and carries ADR-0117 item 8's scope widening in the same line.

  ```commit
  feat(rendering): state the plain-punctuation rule (ADR-0115, ADR-0117)
  ```

## Phase 7: Ship the advisory audit rule and flip ADR-0117

- [ ] **Task 7.1: Add the config key.** Five coordinated touchpoints; missing one trips the
  closed-config-tree checks (ADR-0086) or leaves the reference stale. The descriptor list is not
  alphabetical but a declaration order mirrored across the files, and the generated reference's row
  order derives from it, so the new key goes in at the same relative position in each: immediately
  after `audit.undocumentedDomain` and before `audit.uncommittedChanges`.

  In `internal/config/config.go`, in `AuditConfig`, after the `UndocumentedDomain` field:

  ```go
  	PlainPunctuation    *bool       `yaml:"plainPunctuation"`
  ```

  In `internal/audit/settings.go`, in `Settings`, after `UndocumentedDomain`:

  ```go
  	PlainPunctuation    bool
  ```

  in `Resolve`'s default literal, after `UndocumentedDomain: true,`:

  ```go
  		PlainPunctuation:    true,
  ```

  and in `Resolve`, after the `UndocumentedDomain` override branch:

  ```go
  	if a.PlainPunctuation != nil {
  		s.PlainPunctuation = *a.PlainPunctuation
  	}
  ```

  In `internal/configspec/spec.go`, between the `audit.undocumentedDomain` and
  `audit.uncommittedChanges` entries:

  ```go
  	{
  		Path: "audit.plainPunctuation", Type: "bool", Default: "true",
  		Description:  "Advisory rule: warn when a commit raises the count of typographic punctuation substitutes (the em-dash U+2014, en-dash U+2013, ellipsis U+2026, and the curly quotes U+2018, U+2019, U+201C, U+201D) in an authored markdown file under `docsDir`. Existing occurrences never warn; only a net increase does. Generated files are skipped.",
  		Availability: "Read by `awf audit`.",
  	},
  ```

  This description must cite no ADR number (`TestConfigspecDescriptionResidue`).

  In `internal/project/configreference.go`, in `currentValue`, between the `audit.undocumentedDomain`
  and `audit.uncommittedChanges` cases:

  ```go
  	case "audit.plainPunctuation":
  		return withDefault(strconv.FormatBool(res.PlainPunctuation), a == nil || a.PlainPunctuation == nil)
  ```

  `TestConfigspecKeyParity` fails the gate if the `AuditConfig` field and the descriptor disagree, so
  this task is verified by `./x gate` in task 7.7 rather than by a command of its own.

- [ ] **Task 7.2: Cover the new toggle in the settings tests.** The 100% coverage gate (ADR-0012)
  fails if the override branch added in task 7.1 is uncovered, and the existing assertions enumerate
  the toggles. In `internal/audit/settings_test.go`, three assertions each span a condition line and
  its `t.Errorf` line, and both lines change together:

  - `TestResolveDefaultsWhenNil`, lines 15/16: add `|| !s.PlainPunctuation` to the condition and
    `plain=%v` plus `s.PlainPunctuation` to the `t.Errorf`, following `UncommittedChanges`' shape.
    This is what proves the new default is `true`.
  - `TestResolveZeroAuditFallsBackToDefaults`, lines 37/38: add `|| !s.PlainPunctuation` and the
    matching `t.Errorf` argument. Follow **`UndocumentedDomain`'s** shape here, not
    `UncommittedChanges`': this assertion deliberately omits `UncommittedChanges`, so there is no
    `UncommittedChanges` referent on these lines to copy.
  - `TestResolveExplicitOverrides`, lines 60/61: add `|| s.PlainPunctuation` (unnegated, as its
    siblings are) and the matching `t.Errorf` argument. This is the assertion that covers the new
    override branch.

  In the same test's `AuditConfig` literal, insert `PlainPunctuation:    boolPtr(false),` between
  `UndocumentedDomain:` (line 57) and `UncommittedChanges:` (line 58), matching the field
  declaration order established in task 7.1.

- [ ] **Task 7.3: Thread `DocsDir` into the audit inputs.** ADR-0117 Decision item 3 names this the
  rule's one piece of new plumbing. In `internal/audit/audit.go`, in the `Inputs` struct, after the
  `ADRDir` field:

  ```go
  	DocsDir           string   // e.g. "docs"; the authored-prose root (ADRDir and PlansDir sit under it)
  ```

  and in that struct's doc comment, add `PlainPunctuation` to the parenthesised knob list so it stays
  an accurate enumeration. In `internal/project/project.go`, in the `audit.Inputs` literal inside
  `Audit`, alongside the other layout fields:

  ```go
  		DocsDir:           lay.DocsDir,
  ```

  `Layout.DocsDir` already exists and is already trailing-slash-trimmed, so no new derivation is
  needed. `Inputs` embeds `Settings`, so the knob is readable as `in.PlainPunctuation` with no
  further promotion.

- [ ] **Task 7.4: Add the rule.** In `internal/audit/audit.go`, register it in `evaluate` after the
  `ruleDomainCodeStaleness` line:

  ```go
  	out = append(out, rulePlainPunctuation(commits, in)...)
  ```

  and add the rule alongside the other rule functions:

  ```go
  // bannedProseRunes are the typographic punctuation substitutes the documentation
  // standard bans. Each is written as an escape so this file states the rule
  // without typing the glyphs it bans.
  var bannedProseRunes = map[rune]string{
  	'\u2014': "em-dash (U+2014)",
  	'\u2013': "en-dash (U+2013)",
  	'\u2026': "ellipsis (U+2026)",
  	'\u2018': "left single quote (U+2018)",
  	'\u2019': "right single quote (U+2019)",
  	'\u201c': "left double quote (U+201C)",
  	'\u201d': "right double quote (U+201D)",
  }

  // countBanned tallies each banned rune in s.
  func countBanned(s string) map[rune]int {
  	out := map[rune]int{}
  	for _, r := range s {
  		if _, bad := bannedProseRunes[r]; bad {
  			out[r]++
  		}
  	}
  	return out
  }

  // touches-invariant: audit-plain-punctuation - plain-punctuation audit rule; proof in audit_test.go
  func rulePlainPunctuation(commits []Commit, in Inputs) []Finding {
  	if !in.PlainPunctuation || in.DocsDir == "" {
  		return nil
  	}
  	var out []Finding
  	for _, c := range commits {
  		for _, ch := range c.Changes {
  			if ch.Action == Deleted || !strings.HasSuffix(ch.Path, ".md") ||
  				!underDir(ch.Path, in.DocsDir) || in.GeneratedPaths[ch.Path] {
  				continue
  			}
  			before, after := countBanned(ch.OldText), countBanned(ch.NewText)
  			var risen []string
  			for r, name := range bannedProseRunes {
  				if after[r] > before[r] {
  					risen = append(risen, fmt.Sprintf("%s (%d to %d)", name, before[r], after[r]))
  				}
  			}
  			if len(risen) == 0 {
  				continue
  			}
  			slices.Sort(risen)
  			out = append(out, finding(Warning, "plain-punctuation", c,
  				fmt.Sprintf("%s adds typographic punctuation: %s; authored prose uses plain punctuation (a colon, semicolon, comma, or parentheses; an ASCII hyphen for a range; three periods for elision)",
  					ch.Path, strings.Join(risen, ", "))))
  		}
  	}
  	return out
  }
  ```

  Every helper used here already exists in the package: `underDir` (the `docsDir` prefix check
  ADR-0117 Decision item 3 asks for), `finding` (which stamps Commit and Subject, as this rule
  reports per commit rather than branch-level), and the `Warning` severity constant. `fmt`,
  `strings`, and `slices` are already imported. Sorting `risen` keeps the message deterministic,
  since map iteration order is not.

- [ ] **Task 7.5: Back the invariant with a test.** In `internal/audit/audit_test.go`, add the test
  below, following the file's established shape: cases exercised against the rule function directly,
  with the `// invariant:` proof marker on the asserting statement.

  ```go
  func TestRulePlainPunctuation(t *testing.T) {
  	in := Inputs{DocsDir: "docs", Settings: Settings{PlainPunctuation: true},
  		GeneratedPaths: map[string]bool{"docs/decisions/ACTIVE.md": true}}
  	change := func(path, oldText, newText string) []Commit {
  		return []Commit{{Hash: "abc1234", Subject: "docs(adr): x",
  			Changes: []FileChange{{Path: path, Action: Modified, OldText: oldText, NewText: newText}}}}
  	}
  	dash, dots := "\u2014", "\u2026"

  	// A rising count warns, naming the file, the codepoint, and the commit.
  	got := rulePlainPunctuation(change("docs/decisions/0001-x.md", "plain", "an "+dash+" dash"), in)
  	// invariant: audit-plain-punctuation
  	if len(got) != 1 || got[0].Rule != "plain-punctuation" || got[0].Severity != Warning ||
  		got[0].Commit != "abc1234" || !strings.Contains(got[0].Detail, "em-dash (U+2014)") {
  		t.Fatalf("want 1 warning naming the em-dash, got %v", got)
  	}
  	// An unchanged count is silent: grandfathering is emergent, not configured.
  	if f := rulePlainPunctuation(change("docs/plans/p.md", "a"+dash+"b", "c"+dash+"d"), in); len(f) != 0 {
  		t.Errorf("net-zero swap should be silent, got %v", f)
  	}
  	// A falling count is silent.
  	if f := rulePlainPunctuation(change("docs/plans/p.md", "a"+dash+"b"+dash+"c", "a"+dash+"b"), in); len(f) != 0 {
  		t.Errorf("a removal should be silent, got %v", f)
  	}
  	// A new file has empty OldText, so every occurrence in it is new.
  	added := []Commit{{Hash: "d", Changes: []FileChange{{Path: "docs/x.md", Action: Added, NewText: "a " + dots + " b"}}}}
  	if f := rulePlainPunctuation(added, in); len(f) != 1 || !strings.Contains(f[0].Detail, "ellipsis (U+2026)") {
  		t.Fatalf("want 1 warning naming the ellipsis on an added file, got %v", f)
  	}
  	// A generated path is skipped: its glyphs are its source's fault.
  	if f := rulePlainPunctuation(change("docs/decisions/ACTIVE.md", "", "a"+dash+"b"), in); len(f) != 0 {
  		t.Errorf("generated path should be skipped, got %v", f)
  	}
  	// Outside docsDir is skipped.
  	if f := rulePlainPunctuation(change("README.md", "", "a"+dash+"b"), in); len(f) != 0 {
  		t.Errorf("path outside docsDir should be skipped, got %v", f)
  	}
  	// A non-markdown path under docsDir is skipped: FileChange loads text only for .md.
  	if f := rulePlainPunctuation(change("docs/x.txt", "", "a"+dash+"b"), in); len(f) != 0 {
  		t.Errorf("non-markdown path should be skipped, got %v", f)
  	}
  	// A deleted file is skipped.
  	deleted := []Commit{{Hash: "e", Changes: []FileChange{{Path: "docs/x.md", Action: Deleted, OldText: "a" + dash + "b"}}}}
  	if f := rulePlainPunctuation(deleted, in); len(f) != 0 {
  		t.Errorf("deleted file should be skipped, got %v", f)
  	}
  	// Disabled.
  	if f := rulePlainPunctuation(change("docs/x.md", "", "a"+dash+"b"), Inputs{DocsDir: "docs"}); f != nil {
  		t.Errorf("disabled rule returned %v", f)
  	}
  	// Unset DocsDir is inert.
  	if f := rulePlainPunctuation(change("docs/x.md", "", "a"+dash+"b"),
  		Inputs{Settings: Settings{PlainPunctuation: true}}); f != nil {
  		t.Errorf("unset DocsDir should be inert, got %v", f)
  	}
  }
  ```

- [ ] **Task 7.6: Add the rule to the tooling narrative.** Docs travel with the change.
  `.awf/domains/parts/tooling/current-state.md:7` enumerates every `awf audit` rule ("Its rules
  cover Conventional-Commits, ... `domain-doc-staleness`, `undocumented-domain`, and
  `domain-code-staleness`. One rule, `uncommitted-changes` ... is range-independent"), so shipping
  a rule without listing it there leaves the enumeration wrong. In the sentence naming the
  domain-doc-currency rules, after the `domain-code-staleness` clause and before the
  `uncommitted-changes` sentence, add:

  ```markdown
  A further advisory rule, `plain-punctuation` (ADR-0117), warns when a commit raises the count of a typographic punctuation substitute (the em-dash, en-dash, ellipsis, or a curly quote) in a non-generated markdown file under `docsDir`: the trigger is a net increase per commit per file, comparing the change's old and new text, so existing prose is grandfathered without an allowlist and a doc that legitimately depicts a glyph warns rather than blocks.
  ```

  This narrative is a convention part, not a rendered artifact, so it is authored directly.

- [ ] **Task 7.7: Flip ADR-0117, re-render, verify, and commit.** In
  `docs/decisions/0117-advisory-plain-punctuation-audit-rule-for-authored-prose.md`, the frontmatter
  `status:` line becomes `status: Implemented`. Add to the `### Features` list under
  `## [Unreleased]` in `changelog/CHANGELOG.md`:

  ```markdown
  - `awf audit` gains an advisory `plain-punctuation` rule (ADR-0117), on by default and switched
    off with `audit.plainPunctuation: false`. It warns, and never errors, when a commit **raises**
    the count of a typographic punctuation substitute in an authored markdown file under `docsDir`.
    Prose already written never warns: only a net increase does, so there is no allowlist, no cutoff
    date, and nothing to migrate. Generated files are skipped.
  ```

  Run `./x sync` (regenerates `docs/config-reference.md`,
  `examples/sundial/docs/config-reference.md`, `docs/decisions/ACTIVE.md` for the status change, and
  the domain docs), then `./x gate` (expect it to pass, including the 100% coverage gate over the
  new rule and its branches), then `./x check` (expect `check: clean`), then `./x invariants`
  (expect `audit-plain-punctuation` reported as backed). Confirm the rule is live with `./x audit`:
  it exits zero and warns on the plan and ADR files this effort itself added, which is the rule
  working as designed (ADR-0117 Decision item 6: advisory severity is the depiction escape hatch).
  Two further advisory warnings are expected here and are not defects: `domain-doc-staleness` and
  `domain-code-staleness` fire for the domains this effort churns, and the narratives that were
  factually affected are refreshed by tasks 6.4 and 7.6. Stage every changed and regenerated file,
  including `.awf/domains/parts/tooling/current-state.md`, then commit:

  ```commit
  feat(tooling): add the plain-punctuation audit rule (ADR-0117)
  ```

## Verification

Whole-effort acceptance checks, run from a clean tree at the end:

- `./x gate` passes.
- `./x check` prints `check: clean` with no advisory note about an unbacked or dangling invariant.
- `./x invariants` reports both `emitted-prose-no-typographic-substitutes` and
  `audit-plain-punctuation` as backed, and reports `template-em-dash-free` nowhere.
- `./x audit` exits zero.
- The gate test passes for the right reason, not vacuously. Temporarily reintroduce one banned
  codepoint into a production Go string literal and confirm
  `go test ./internal/project/ -run TestEmittedProseNoTypographicSubstitutes` fails, naming the file
  and line; then temporarily point the Go walk at a nonexistent directory and confirm the
  seen-count guard fails rather than reporting success. Revert both.
- The three shipped surfaces are clean. The first two must print `0`, the third `117`:

  ```
  grep -rlP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' templates/ | wc -l
  grep -cP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' changelog/CHANGELOG.md
  grep -c ') (' docs/decisions/ACTIVE.md
  ```

- The grandfathered backlog is untouched: the em-dash counts under `docs/decisions/` (2344 in ADR
  bodies) and `docs/plans/` (4347) are unchanged, save for this plan's own contribution and the
  three normalized headings.

## Notes

- **The doc-standard line is written once, in phase 6, carrying both widenings.** ADR-0117 Decision
  item 8 directs this outright for the case at hand: "if both land in one effort, the line is
  written once, carrying both, under a single changelog entry." An earlier draft of this plan split
  it across two phases, reading item 8's clause as permissive; it is not, and review corrected it.
  The one objection to the single edit, that ADR-0115's flip commit then ships a scope claim
  ADR-0115 does not own and nothing yet enforces, does not survive contact with the ADRs: item 9
  requires only that the line name all seven codepoints and says nothing about the scope clause, and
  ADR-0117 item 5 requires only that the standard widen *before* the rule enforces it, which phase 6
  satisfies by preceding phase 7. Recorded because the two-edit reading is the tempting one and a
  future reader deserves to know it was considered and rejected on the ADR's own text.
- **Phase 4 is the plan's one coupled commit**, and its header states why rather than assuming it:
  the test is the cleanup's only permanent completeness proof, and ADR-0115 requires the test swap
  to be atomic anyway. Both halves would pass the gate if split, so the reason is what the history
  proves, not what the gate forces.
- **Deferred deliberately** (ADR-0115 Consequences records both): the sidecar blind spot (23
  ellipses reach `docs/pitfalls.md`, `docs/glossary.md`, and `docs/decisions/README.md` from
  per-project sources the gate does not read, which ADR-0115 Decision item 7 depends on), and the
  seven tracked files outside any scanned surface (`x`, `.githooks/pre-commit`, both
  `.github/workflows/` files, `.goreleaser.yaml`, `.gremlins.yaml`, `codecov.yml`). Two of those
  matter: `x` and `.githooks/` are co-owned units whose rendered counterparts must satisfy the ban,
  so awf's own from-source runner violates the rule its own template must pass. awf disables both
  units for itself, so no rendered artifact is affected.
- **`awf new adr` collides across worktrees.** It computes the next number from the current tree
  only, so a sibling worktree can claim the same number; this effort hit exactly that once already.
  Before scaffolding an ADR in a parallel-worktree session, check every branch first.
