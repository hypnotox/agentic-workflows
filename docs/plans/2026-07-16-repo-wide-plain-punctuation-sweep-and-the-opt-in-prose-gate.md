---
date: 2026-07-16
adrs: [119]
status: Proposed
---
# Plan: Repo-wide plain punctuation sweep and the opt-in prose gate

## Goal

Execute ADR-0119: sweep the 355 remaining banned typographic codepoints from the six live surfaces,
then ship `awf prose-gate`, an opt-in presence-level check over every tracked text file, and enable
it for awf itself. Non-goals: retiring ADR-0115's gate (item 7 keeps it), protecting `./x`'s gate
wiring (named in the ADR's Consequences as a pre-existing gap owned by its own ADR), and re-styling
the bare hyphens commit `8338840` already introduced (ADR-0119 item 13 blesses them).

The design lives in ADR-0119. This plan does not re-argue it.

### Two conventions this plan runs on, and why

**Every diff below writes the banned character as a `<U+NNNN>` token; the file on disk contains the
character itself.** This plan is a tracked file, so once Phase 8 enables the gate, a plan that typed
the glyphs it sweeps would fail the gate it installs. `<U+2014>` means one literal em-dash, and a
line showing two means two.

**An exemption entry names `U+2014`, never the character.** `.awf/config.yaml` is tracked and in
scope, so a codepoint typed into an exemption entry would be a finding against the file that
configures the exemptions. The config surface is therefore codepoint-named by necessity, not taste.

The seven banned codepoints, throughout: U+2014 (em-dash), U+2013 (en-dash), U+2026 (ellipsis),
U+2018, U+2019, U+201C, U+201D (curly quotes). Replacement policy: ADR-0118 item 9 as amended by
ADR-0119 item 13 (a bare hyphen is now permitted).

## Architecture summary

Phases 1 to 5 sweep, cleaning the tree from the outside in and leaving it at zero findings outside
the four exemption entries. Each is a batch task over a measured site set, each closes with `./x
gate` green, and each is independently revertible.

Phase 6 adds `internal/prosegate` (the scanner), the `proseGate` config block, its `configspec`
entries, and the `clispec` command, with the knob defaulting false and awf's own knob still unset:
the command exists and no-ops, so the gate cannot fail on it.

Phase 7 wires the rendered `pre-commit` payload through a new `proseGateCmd` var and widens the
payload's bootstrap-shim guard to a disjunction.

Phase 8 flips awf to its own dogfood: it sets the knob, declares the four exemptions, wires `./x
gate`, corrects the agent guide and the shipped doc-standard, adds the changelog entry, and flips
ADR-0119 and this plan to Implemented.

The sweep must complete before Phase 8, or the gate Phase 8 enables fails on its own tree. Phases 6
and 7 could in principle precede the sweep, but are placed after it so that no commit in this plan
ships a check the tree would fail.

## File structure

- **Created:** `internal/prosegate/prosegate.go`, `internal/prosegate/prosegate_test.go`,
  `cmd/awf/prosegate.go`, `cmd/awf/prosegate_test.go`.
- **Modified:** `docs/research/*.md` (2), `.awf/docs/pitfalls.yaml`, `.awf/docs/glossary.yaml`,
  `.awf/parts/adr-readme/invariants.md`, 11 production `.go` files, 22 `_test.go` files, `x`,
  `.githooks/pre-commit`, `codecov.yml`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`,
  `.goreleaser.yaml`, `.gremlins.yaml`, `README.md`, `internal/config/config.go`,
  `internal/configspec/spec.go`, `internal/project/configreference.go`, `internal/clispec/clispec.go`,
  `cmd/awf/main.go`, `templates/hooks/pre-commit.sh.tmpl`, `templates/docs/doc-standard.md.tmpl`,
  `.awf/agents-doc.yaml`, `.awf/config.yaml`, `changelog/CHANGELOG.md`, plus regenerated
  `docs/pitfalls.md`, `docs/glossary.md`, `docs/decisions/README.md`, `docs/config-reference.md`,
  `AGENTS.md`, `docs/doc-standard.md`, `.awf/hooks/pre-commit.sh`, `.awf/awf.lock`,
  `examples/sundial/**`, `docs/decisions/0119-*.md`, and this plan.
- **Deleted:** none.

## Phase 1: The word-stream harness and docs/research

ADR-0119 item 2 makes a zero-delta word-stream proof the condition of the sweep, not a nicety. This
phase builds that harness and proves it can fail before trusting it, then applies it to the largest
surface. Every later sweep phase reuses it.

- [ ] **Task 1.1: Self-test the word-stream harness.** The harness compares each path's word stream
  (maximal alphanumeric runs) at a git ref against the working tree; a zero delta proves only
  punctuation moved. A harness that cannot fail turns every later PASS into a lie, so it is planted
  with a known-bad edit first.

  Run this to confirm the harness reports FAIL on a word change:

  ```bash
  cp docs/research/deep-analysis-2026-07-15.md /tmp/wp-backup.md
  printf '\nplantedcanaryword\n' >> docs/research/deep-analysis-2026-07-15.md
  python3 - HEAD docs/research <<'PY'
  import re, subprocess, sys
  ref, roots = sys.argv[1], sys.argv[2:]
  paths = subprocess.run(['git', 'ls-files'] + roots, capture_output=True, text=True).stdout.split()
  W = re.compile(r'[0-9A-Za-z]+')
  bad = 0
  for p in paths:
      old = subprocess.run(['git', 'show', f'{ref}:{p}'], capture_output=True, text=True).stdout
      new = open(p, encoding='utf-8').read()
      if W.findall(old) != W.findall(new):
          print('WORD DELTA:', p)
          bad = 1
  print('word-stream: FAIL' if bad else 'word-stream: PASS')
  sys.exit(bad)
  PY
  ```

  Expected output, exactly:

  ```
  WORD DELTA: docs/research/deep-analysis-2026-07-15.md
  word-stream: FAIL
  ```

  Then restore: `cp /tmp/wp-backup.md docs/research/deep-analysis-2026-07-15.md` and confirm the
  same command now prints `word-stream: PASS` with no `WORD DELTA` line. Do not proceed until both
  halves behave as stated.

- [ ] **Task 1.2: Sweep `docs/research/` (batch, 186 sites across 2 files).** ADR-0119 item 3 grants
  the approval ADR-0118 item 1 withheld. The corpus carries zero curly quotes and no external
  quotation, so no verbatim quote can be corrupted; the sites are awf's own headings and callouts.

  **Representative site** (a heading separator, the dominant shape):

  ```diff
  --- a/docs/research/agentic-workflow-landscape-and-awf-standing-2026-07.md
  +++ b/docs/research/agentic-workflow-landscape-and-awf-standing-2026-07.md
  @@ -32,1 +32,1 @@
  -## Part 1 <U+2014> The field consensus
  +## Part 1: The field consensus
  ```

  Apply the identical shape to every affected site: an em-dash whose right side explains or names
  what its left side introduces becomes a colon; two em-dashes bracketing a clause become a pair of
  parentheses; a light aside becomes a comma; two independent clauses become a semicolon. Read whole
  paragraphs before converting, because the corpus is hard-wrapped and a bracketing pair spans lines.

  **Edge site** (an en-dash in a numeric range, and notation that must survive untouched):

  ```diff
  --- a/docs/research/agentic-workflow-landscape-and-awf-standing-2026-07.md
  +++ b/docs/research/agentic-workflow-landscape-and-awf-standing-2026-07.md
  @@ -62,1 +62,1 @@
  -- Chroma Research, *Context Rot* <U+2014> research.trychroma.com/context-rot (independently corroborates 15<U+2013>30% retrieval-accuracy drop from ~8K→128K tokens across 18 frontier models) ⬤
  +- Chroma Research, *Context Rot*: research.trychroma.com/context-rot (independently corroborates a 15-30% retrieval-accuracy drop from ~8K→128K tokens across 18 frontier models) ⬤
  ```

  The en-dash range becomes an ASCII hyphen. The rightwards arrow (U+2192) and the circle glyphs
  (U+2B24, U+25D0) are notation, not punctuation substitutes: they are **not** banned and must not be
  touched. This is the line to read twice.

  **Affected-site set:** `git grep -lP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- docs/research`
  (exactly `docs/research/agentic-workflow-landscape-and-awf-standing-2026-07.md` and
  `docs/research/deep-analysis-2026-07-15.md`).

  **Post-check** (must print `0`):

  ```bash
  git grep -oP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- docs/research | wc -l
  ```

  **Word-stream proof:** re-run the Task 1.1 harness (without the planted word) over
  `docs/research`; it must print `word-stream: PASS` and no `WORD DELTA` line.

- [ ] **Task 1.3: Verify and commit.** Run `./x gate` (green) and `./x check` (`awf check: clean`).
  `git add docs/research/agentic-workflow-landscape-and-awf-standing-2026-07.md
  docs/research/deep-analysis-2026-07-15.md`, then commit:

  ```commit
  docs(tooling): sweep plain punctuation through docs/research
  ```

## Phase 2: The .awf sidecars and their render targets

23 of the 24 sidecar occurrences are swept; the 24th is the `.awf/docs/pitfalls.yaml` entry that
types a U+201C to document what gofmt produces, which ADR-0119 item 6 exempts. The 23 occurrences in
the generated docs are **not** edited: they are fixed by re-rendering, because hand-editing a
rendered file is forbidden.

- [ ] **Task 2.1: Sweep `.awf/docs/pitfalls.yaml` (batch, 19 U+2026 sites; leave the U+201C).**

  **Representative site:**

  ```diff
  --- a/.awf/docs/pitfalls.yaml
  +++ b/.awf/docs/pitfalls.yaml
  @@ -64,1 +64,1 @@
  -        `//go:embed skills agents <U+2026>` form embeds neither `_base`, and `fs.ReadFile(templates.FS, <U+2026>)` fails
  +        `//go:embed skills agents ...` form embeds neither `_base`, and `fs.ReadFile(templates.FS, ...)` fails
  ```

  Apply the identical shape to every affected site: an ellipsis marking elision becomes three ASCII
  periods. All 19 are elision inside code or format examples; none is prose punctuation.

  **Edge site** (the one occurrence that must survive, on line 306):

  ```diff
  (no change)
          a doc comment as the old quoting convention and rewrites it to a curly quote (`<U+201C>`); so a
  ```

  This U+201C is the subject of its own sentence: the entry documents what gofmt emits, and
  punctuating it would make a true statement false. It is exempted in Phase 8, not swept. Touching it
  is a defect.

  **Affected-site set:** `grep -nP '\x{2026}' .awf/docs/pitfalls.yaml` (19 matches across lines 64,
  74, 84, 110, 189, 438, 488, 515, 518, 528, 652, 657, 661, 662, 768, 789).

  **Post-check** (must print `0 19`, meaning zero ellipses remain and the 19 became ASCII):

  ```bash
  echo "$(grep -coP '\x{2026}' .awf/docs/pitfalls.yaml || echo 0) $(grep -cP '\x{201C}' .awf/docs/pitfalls.yaml)"
  ```

  Expected output: `0 1`. Zero U+2026 remain; the single U+201C survives.

- [ ] **Task 2.2: Sweep `.awf/docs/glossary.yaml` (1 site).**

  ```diff
  --- a/.awf/docs/glossary.yaml
  +++ b/.awf/docs/glossary.yaml
  @@ -5,1 +5,1 @@
  -    "checker-cmd idiom": "The shape shared by the repo-only gate/release checkers (`cmd/covercheck`, `cmd/deadcodecheck`, `cmd/releasecheck`, `cmd/pincheck`, `cmd/mutants`): a coverage-ignored `main` that only `os.Exit`s a unit-tested `run(<U+2026>, stdout, stderr) int` seam, so the logic meets the 100% floor while the wrapper stays one line."
  +    "checker-cmd idiom": "The shape shared by the repo-only gate/release checkers (`cmd/covercheck`, `cmd/deadcodecheck`, `cmd/releasecheck`, `cmd/pincheck`, `cmd/mutants`): a coverage-ignored `main` that only `os.Exit`s a unit-tested `run(..., stdout, stderr) int` seam, so the logic meets the 100% floor while the wrapper stays one line."
  ```

- [ ] **Task 2.3: Sweep `.awf/parts/adr-readme/invariants.md` (3 sites).**

  ```diff
  --- a/.awf/parts/adr-readme/invariants.md
  +++ b/.awf/parts/adr-readme/invariants.md
  @@ -4,2 +4,2 @@
  -backed ``- `invariant: <slug>` - <U+2026>`` for a property a test is declared to back, or an
  -``- `unbacked-invariant: <slug>` - <U+2026>. **Verify:** <U+2026>`` for a reasoned contract with no automatic test
  +backed ``- `invariant: <slug>` - ...`` for a property a test is declared to back, or an
  +``- `unbacked-invariant: <slug>` - .... **Verify:** ...`` for a reasoned contract with no automatic test
  ```

  Note the ` - ` separators already present are bare hyphens: ADR-0119 item 13 blesses them, so leave
  them alone.

- [ ] **Task 2.4: Re-render and verify the generated targets.** Run `./x sync`. It must report
  `docs/pitfalls.md`, `docs/glossary.md` and `docs/decisions/README.md` as regenerated. Then confirm
  the render targets carry exactly what their sources now do (expected output `1`, the surviving
  U+201C in the pitfalls render target, and nothing else):

  ```bash
  git grep -oP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- docs/pitfalls.md docs/glossary.md docs/decisions/README.md | wc -l
  ```

  Do not hand-edit any of the three. If a glyph survives in a render target, its source is
  unconverted: fix the source and re-run `./x sync`.

- [ ] **Task 2.5: Verify and commit.** Run `./x gate` (green) and `./x check` (`awf check: clean`).
  Stage the three sources and the three regenerated docs plus `.awf/awf.lock`, then commit:

  ```commit
  docs(rendering): sweep plain punctuation through the .awf sidecars
  ```

## Phase 3: Production Go comments

ADR-0119 item 4 brings comments into scope; its Context refutes ADR-0115 item 4's gofmt reason. 17
occurrences across 11 files, 16 U+2026 and one U+2014.

- [ ] **Task 3.1: Sweep production Go comments (batch, 17 sites across 11 files).**

  **Representative site** (elision in a code shape, 16 of the 17):

  ```diff
  --- a/internal/render/vars.go
  +++ b/internal/render/vars.go
  @@ -13,1 +13,1 @@
  -// (any {{ <U+2026> .skills<U+2026> }} action) - such templates fold the effective skills set
  +// (any {{ ... .skills... }} action) - such templates fold the effective skills set
  ```

  Apply the identical shape to every affected site: an ellipsis becomes three ASCII periods. The
  existing ` - ` separators are bare hyphens from commit `8338840`; ADR-0119 item 13 blesses them,
  so do not restyle them.

  **Edge site** (the only U+2014, and the one ADR-0118 item 10's enumerated scope missed):

  ```diff
  --- a/changelog/embed.go
  +++ b/changelog/embed.go
  @@ -3,1 +3,1 @@
  -// outside its own package directory <U+2014> mirrors templates/embed.go.
  +// outside its own package directory: mirrors templates/embed.go.
  ```

  **Affected-site set** (11 files: `internal/render/vars.go`, `internal/render/render.go`,
  `internal/project/placeholders.go`, `internal/project/configreference.go`,
  `internal/project/render.go`, `internal/project/sweep.go`, `internal/config/scopespec.go`,
  `internal/configspec/spec.go`, `cmd/repoaudit/main.go`, `cmd/awf/new.go`, `changelog/embed.go`):

  ```bash
  git grep -lP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- '*.go' ':!*_test.go'
  ```

  **Post-check** (must print `0`):

  ```bash
  git grep -oP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- '*.go' ':!*_test.go' | wc -l
  ```

  **The gofmt check that matters:** run `gofmt -l ./internal ./cmd ./changelog`. It must print
  nothing. If gofmt rewrites one of your edits into a U+201C, you wrote a double backtick in a
  declaration-attached doc comment: replace it with a single backtick. That is the loop ADR-0119's
  Context says the author wins.

- [ ] **Task 3.2: Verify and commit.** Run `./x gate` (green). Stage the 11 files and commit:

  ```commit
  style(awf): sweep plain punctuation from production Go comments
  ```

## Phase 4: Test fixtures

108 occurrences across 22 files. ADR-0119's Context records the verification: these are fixture
inputs, not assertions about shipped output, and a full sweep on a scratch copy left `go test ./...`
green. No parser reads the glyph as a separator (`declRe` and `slugRe` bound a slug to `[a-z0-9-]+`
and stop at the space before it).

- [ ] **Task 4.1: Sweep `_test.go` fixtures (batch, 108 sites across 22 files).**

  **Representative site** (fixture prose in an ADR body, the dominant shape):

  ```diff
  --- a/internal/invariants/invariants_test.go
  +++ b/internal/invariants/invariants_test.go
  @@ -22,1 +22,1 @@
  -	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: kept` <U+2014> x.\n- `invariant: gone` <U+2014> y.")
  +	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: kept` - x.\n- `invariant: gone` - y.")
  ```

  Apply the identical shape to every affected site. A bare hyphen is the right replacement here
  (ADR-0119 item 13) because these fixtures mimic the rendered invariant-bullet form, which the
  shipped standard itself writes with ` - `.

  **Edge site** (the coupled pair: a fixture and the assertion that captures it).
  `cmd/awf/context_test.go:56` seeds a marker and `:100` asserts on the rest-of-line that
  `touchesRe`'s second capture group carries into `awf context` output. They must move together, or
  the test fails loudly:

  ```diff
  --- a/cmd/awf/context_test.go
  +++ b/cmd/awf/context_test.go
  @@ -56,1 +56,1 @@
  -		"// touches-invariant: unbacked-slug <U+2014> the reasoned production site.\n"+
  +		"// touches-invariant: unbacked-slug - the reasoned production site.\n"+
  @@ -100,1 +100,1 @@
  -	if !strings.Contains(out, "touches: <U+2014> the reasoned production site.") {
  +	if !strings.Contains(out, "touches: - the reasoned production site.") {
  ```

  Sweeping one without the other fails the test. That is the good failure mode; it is named here so
  an implementer does not "fix" the assertion by loosening it.

  **Affected-site set** (22 files):

  ```bash
  git grep -lP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- '*_test.go'
  ```

  **Post-check** (must print `0`):

  ```bash
  git grep -oP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- '*_test.go' | wc -l
  ```

  **Note the deliberate loss.** `internal/project/spine_test.go`'s fixtures currently feed
  adopter-shaped sidecar data containing em-dashes through the renderer, incidentally demonstrating
  that awf passes adopter strings through unrewritten (ADR-0115 item 6). Sweeping them removes the
  only place that behaviour is visible. ADR-0119's Consequences records this as accepted; nothing
  asserts it, so nothing fails. Do not add an exemption to preserve it.

- [ ] **Task 4.2: Verify and commit.** Run `./x gate` (green; every package must pass, and the
  100% coverage floor is unaffected because no production statement changed). Stage the 22 files and
  commit:

  ```commit
  test(awf): sweep plain punctuation from test fixtures
  ```

## Phase 5: Repo infrastructure and the README

15 occurrences across 7 hand-maintained infrastructure files, plus 6 in the README's box diagram.

`x` and `.githooks/pre-commit` are hand-maintained here and carry no GENERATED marker, so they are
edited directly. Do **not** generalise that to `.awf/hooks/*.sh`: those **are** rendered
(`hooks: enabled: true`), carry the GENERATED marker, and already hold zero occurrences.

- [ ] **Task 5.1: Sweep repo infrastructure (batch, 15 sites across 7 files).**

  **Representative site** (an em-dash in a comment, all 15):

  ```diff
  --- a/x
  +++ b/x
  @@ -2,1 +2,1 @@
  -# Command runner for the awf repo <U+2014> the single entry point for repo interactions.
  +# Command runner for the awf repo: the single entry point for repo interactions.
  ```

  Apply the identical shape to every affected site. All 15 are U+2014 in comments; the shape is
  identical at every site, so no edge diff is given.

  **Affected-site set:** `x` (3), `.githooks/pre-commit` (4), `codecov.yml` (3),
  `.github/workflows/ci.yml` (2), `.github/workflows/release.yml` (1), `.goreleaser.yaml` (1),
  `.gremlins.yaml` (1). Enumerate with:

  ```bash
  git grep -lP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- x .githooks codecov.yml .github .goreleaser.yaml .gremlins.yaml
  ```

  **Post-check** (must print `0`):

  ```bash
  git grep -oP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' -- x .githooks codecov.yml .github .goreleaser.yaml .gremlins.yaml | wc -l
  ```

- [ ] **Task 5.2: Sweep `README.md`'s box diagram (6 sites, with re-padding).** All six are U+2026
  inside a two-column ASCII tree. U+2026 occupies one column and `...` occupies three, so every
  replacement widens its cell and the diagram must be re-padded by hand. Line 72's left column
  carries two, widening it by four.

  ```diff
  --- a/README.md
  +++ b/README.md
  @@ -71,3 +71,3 @@
  -├── <kind>/<name>.yaml  sidecars  ├── .claude/skills/<U+2026>     workflow skills
  -├── <kind>/parts/<U+2026>/<U+2026>    overrides ├── .claude/agents/<U+2026>     review agents
  -└── parts/<name>/<U+2026>    singletons  └── docs/<U+2026>               project docs
  +├── <kind>/<name>.yaml  sidecars  ├── .claude/skills/...   workflow skills
  +├── <kind>/parts/.../...  overrides ├── .claude/agents/...   review agents
  +└── parts/<name>/...  singletons  └── docs/...             project docs
  ```

  The diff above shows the intended shape, but the exact padding must be verified by eye, not
  trusted from this plan: after editing, print lines 68 to 73 and confirm that both the right
  column's `├──`/`└──` and the description column (`workflow skills`, `review agents`,
  `project docs`) line up across all rows, including rows 69 and 70 which this diff does not touch.

  ```bash
  sed -n '68,73p' README.md | cat -A | sed 's/\$$//'
  ```

  No test or rendered artifact asserts on this block, so alignment is the only acceptance criterion.

- [ ] **Task 5.3: Verify and commit.** Run `./x gate` (green) and `./x check` (`awf check: clean`).
  Confirm the whole-tree count now stands at exactly the 10 exempt occurrences:

  ```bash
  git grep -oP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' | wc -l
  ```

  Expected output: `10` (7 in ADR-0113, 1 in the 2026-07-13 plan, 1 in `.awf/docs/pitfalls.yaml`, 1
  in `docs/pitfalls.md`). If this prints anything else, a preceding phase is incomplete: find the
  surface before continuing. Stage the 8 files and commit:

  ```commit
  docs(tooling): sweep plain punctuation from repo infrastructure
  ```

## Phase 6: The prose-gate command

The tree is clean as of Phase 5, so the command can land. The knob stays unset in awf's own config
until Phase 8, so this phase's gate cannot fail on the command it adds.

- [ ] **Task 6.1: Add the `proseGate` config block.** In `internal/config/config.go`, add the field
  to `Config` (beside the existing `Bootstrap`/`Hooks`/`Runner` toggles) and the two types. The
  `*ProseGateConfig` nil-means-off shape mirrors `BootstrapConfig` exactly; `Count *int` is optional
  per ADR-0119 item 5.

  ```go
  // ProseGateConfig configures `awf prose-gate` (ADR-0119): a presence-level scan
  // of every tracked text file for the seven banned typographic punctuation
  // substitutes. BootstrapConfig semantics: a nil *ProseGateConfig (key absent)
  // and Enabled false both mean "the command exits zero without scanning". The
  // default is off because the scan blocks a commit, and a tree that has never
  // been swept would fail it on the day it lands.
  type ProseGateConfig struct {
  	Enabled    bool             `yaml:"enabled"`
  	Exemptions []ProseExemption `yaml:"exemptions"`
  }

  // ProseExemption exempts one codepoint in one path. Codepoint is spelled
  // "U+2014", never the character itself: config.yaml is a tracked file the scan
  // reads, so a typed glyph here would be a finding against the file that
  // configures the exemptions. A nil Count permits any number of occurrences; a
  // non-nil Count pins the expected number, so an added occurrence in an exempt
  // file still fails.
  type ProseExemption struct {
  	Path      string `yaml:"path"`
  	Codepoint string `yaml:"codepoint"`
  	Count     *int   `yaml:"count"`
  }
  ```

  Add to the `Config` struct, in the position matching the existing toggles:

  ```go
  	ProseGate *ProseGateConfig `yaml:"proseGate"`
  ```

- [ ] **Task 6.2: Add `internal/prosegate/prosegate.go`.** The scanner. It takes the tracked-path
  set from the caller rather than shelling out itself, so it stays unit-testable without a git
  fixture.

  ```go
  // Package prosegate scans a project's tracked text files for the seven banned
  // typographic punctuation substitutes (ADR-0119). It is the presence-level
  // counterpart to the net-increase audit rule in internal/audit: this package
  // answers "is the tree clean", not "did this commit make it worse".
  package prosegate

  import (
  	"fmt"
  	"os"
  	"sort"
  	"strconv"
  	"strings"
  	"unicode/utf8"
  )

  // Banned maps each banned rune to its display name. Each key is written as an
  // escape, never as the character: this file is itself a tracked file the scan
  // reads, so a typed glyph here would make the scanner fail its own gate.
  // internal/project/residue_scan_test.go's bannedRunes map is the precedent and
  // must stay in agreement with this one. Notation (arrows, mathematical symbols,
  // accented letters) is deliberately absent: this is a closed blocklist of
  // substitutes for ASCII punctuation, never an ASCII-only allowlist.
  var Banned = map[rune]string{
  	'\u2014': "em-dash (U+2014)",
  	'\u2013': "en-dash (U+2013)",
  	'\u2026': "ellipsis (U+2026)",
  	'\u2018': "left single quote (U+2018)",
  	'\u2019': "right single quote (U+2019)",
  	'\u201c': "left double quote (U+201C)",
  	'\u201d': "right double quote (U+201D)",
  }

  // Exemption permits a codepoint in a path, optionally pinning its count.
  type Exemption struct {
  	Path      string
  	Codepoint rune
  	Count     *int
  }

  // Finding is one banned codepoint in one file, with the number found.
  type Finding struct {
  	Path  string
  	Rune  rune
  	Count int
  	// Pinned is set when an exemption pinned a count that did not match.
  	Pinned int
  }

  // ParseCodepoint turns a "U+2014" spelling into its rune. It rejects anything
  // outside the banned set, so a typo cannot silently widen an exemption.
  func ParseCodepoint(s string) (rune, error) {
  	if !strings.HasPrefix(s, "U+") {
  		return 0, fmt.Errorf("codepoint %q: want the form U+2014", s)
  	}
  	n, err := strconv.ParseUint(s[2:], 16, 32)
  	if err != nil {
  		return 0, fmt.Errorf("codepoint %q: %w", s, err)
  	}
  	r := rune(n)
  	if _, ok := Banned[r]; !ok {
  		return 0, fmt.Errorf("codepoint %q is not one of the seven banned substitutes", s)
  	}
  	return r, nil
  }

  // Scan reads each path relative to root and reports every banned rune outside
  // the exemptions. Paths whose contents are not valid UTF-8 are skipped: a
  // default-deny gate must not guess at binary input. An unreadable path is an
  // error, never a silent pass.
  func Scan(root string, paths []string, exemptions []Exemption) ([]Finding, error) {
  	exempt := map[string]*int{}
  	for _, e := range exemptions {
  		exempt[e.Path+"\x00"+string(e.Codepoint)] = e.Count
  	}
  	var out []Finding
  	for _, p := range paths {
  		b, err := os.ReadFile(root + "/" + p)
  		if err != nil {
  			return nil, fmt.Errorf("read %s: %w", p, err)
  		}
  		if !utf8.Valid(b) {
  			continue
  		}
  		counts := map[rune]int{}
  		for _, r := range string(b) {
  			if _, bad := Banned[r]; bad {
  				counts[r]++
  			}
  		}
  		for r, n := range counts {
  			pin, ok := exempt[p+"\x00"+string(r)]
  			switch {
  			case !ok:
  				out = append(out, Finding{Path: p, Rune: r, Count: n})
  			case pin != nil && *pin != n:
  				out = append(out, Finding{Path: p, Rune: r, Count: n, Pinned: *pin})
  			}
  		}
  	}
  	sort.Slice(out, func(i, j int) bool {
  		if out[i].Path != out[j].Path {
  			return out[i].Path < out[j].Path
  		}
  		return out[i].Rune < out[j].Rune
  	})
  	return out, nil
  }

  // Format renders one finding as a diagnostic line.
  func Format(f Finding) string {
  	if f.Pinned > 0 {
  		return fmt.Sprintf("%s: %s appears %d time(s); the exemption pins %d",
  			f.Path, Banned[f.Rune], f.Count, f.Pinned)
  	}
  	return fmt.Sprintf("%s: %s appears %d time(s); use plain punctuation",
  		f.Path, Banned[f.Rune], f.Count)
  }
  ```

- [ ] **Task 6.3: Add `cmd/awf/prosegate.go`.** The handler: resolve the tree, honour the knob,
  refuse without git, print findings, exit non-zero.

  ```go
  package main

  import (
  	"fmt"
  	"io"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"github.com/hypnotox/agentic-workflows/internal/git"
  	"github.com/hypnotox/agentic-workflows/internal/prosegate"
  )

  // runProseGate scans the project's tracked text files for banned typographic
  // punctuation substitutes (ADR-0119). It exits zero without scanning when the
  // knob is off, so a hook or a runner may invoke it unconditionally.
  func runProseGate(root string, stdout, stderr io.Writer) int {
  	cfg, err := config.Load(awfDirOf(root))
  	if err != nil {
  		fmt.Fprintf(stderr, "prose-gate: %v\n", err)
  		return 1
  	}
  	if cfg.ProseGate == nil || !cfg.ProseGate.Enabled {
  		return 0
  	}
  	var exemptions []prosegate.Exemption
  	for _, e := range cfg.ProseGate.Exemptions {
  		r, perr := prosegate.ParseCodepoint(e.Codepoint)
  		if perr != nil {
  			fmt.Fprintf(stderr, "prose-gate: exemption for %s: %v\n", e.Path, perr)
  			return 1
  		}
  		exemptions = append(exemptions, prosegate.Exemption{Path: e.Path, Codepoint: r, Count: e.Count})
  	}
  	paths, err := git.TrackedPaths(root)
  	if err != nil {
  		fmt.Fprintf(stderr, "prose-gate: cannot enumerate tracked files: %v\n", err)
  		return 1
  	}
  	findings, err := prosegate.Scan(root, paths, exemptions)
  	if err != nil {
  		fmt.Fprintf(stderr, "prose-gate: %v\n", err)
  		return 1
  	}
  	for _, f := range findings {
  		fmt.Fprintln(stderr, prosegate.Format(f))
  	}
  	if len(findings) > 0 {
  		fmt.Fprintf(stderr, "prose-gate: %d finding(s)\n", len(findings))
  		return 1
  	}
  	fmt.Fprintln(stdout, "prose-gate: clean")
  	return 0
  }
  ```

  If `awfDirOf` does not exist under that name in `cmd/awf`, use whatever helper the sibling
  handlers (`cmd/awf/check.go`, `cmd/awf/invariants.go`) already use to resolve `<root>/.awf`; do
  not add a second resolver. Wire the handler to the command spec in `cmd/awf/main.go` exactly as
  the sibling ungated commands are wired.

- [ ] **Task 6.4: Add the `clispec` entry.** In `internal/clispec/clispec.go`, add the command with
  `Gating: Ungated` (ADR-0119 item 10). Place it beside `commit-gate`, its species sibling.

  ```go
  	{
  		Name:    "prose-gate",
  		Summary: "Scan tracked text files for typographic punctuation substitutes",
  		HelpBody: "Usage: awf prose-gate\n\n" +
  			"Reports every banned typographic punctuation substitute in the project's\n" +
  			"tracked text files and exits non-zero on any finding. Exits zero without\n" +
  			"scanning unless proseGate.enabled is true, so a hook may invoke it\n" +
  			"unconditionally. Configure exemptions with proseGate.exemptions.\n",
  		MinPos: 0,
  		MaxPos: 0,
  		Gating: Ungated,
  	},
  ```

  **Do not** add `prose-gate` to the `want` slice in `internal/clispec/clispec_test.go`.
  `GatedCommandNames()` filters on `Gating != Ungated`, so an ungated command changes neither the
  derived list nor the AGENTS.md bullet that renders from it. If `TestGatedCommandNames` fails, the
  command has been mis-classified: fix the `Gating` field, not the test.

- [ ] **Task 6.5: Add the `configspec` entries (five).** In `internal/configspec/spec.go`, add one
  entry for the block, one for `enabled`, and one per exemption field, following the
  `invariants.sources` and `audit.allowedScopes` slice-of-struct precedent (`walkPaths` recurses a
  slice-of-struct into `path` plus `path[].<field>`). `TestConfigspecKeyParity` matches
  bidirectionally, so a missing entry and a stray entry both fail.

  Every entry needs a non-empty `Type`, `Default`, `Description` and `Availability`. The description
  must cite no ADR and must name neither the repository nor its owner:
  `TestConfigspecDescriptionResidue` rejects `ADR-[0-9]{4}`, `hypnotox`, and `agentic-workflows`.

  ```go
  	{Kind: "config", Key: "proseGate", Type: "mapping", Default: "absent (the scan is off)", Availability: "always", Description: "Configures the `awf prose-gate` scan: a presence-level check that reports every typographic punctuation substitute (the em-dash U+2014, en-dash U+2013, ellipsis U+2026, and the curly quotes U+2018, U+2019, U+201C, U+201D) in the project's tracked text files. Absent, the command exits zero without scanning."},
  	{Kind: "config", Key: "proseGate.enabled", Type: "bool", Default: "false", Availability: "always", Description: "Whether `awf prose-gate` scans. False, the command exits zero immediately, so a hook may invoke it unconditionally. Default off, because the scan blocks a commit and a tree that has never been swept would fail it."},
  	{Kind: "config", Key: "proseGate.exemptions", Type: "list of {path, codepoint, count} mappings", Default: "empty (nothing is exempt)", Availability: "when `proseGate.enabled` is true", Description: "Places where a banned codepoint is permitted, typically prose that is genuinely about the character it contains. An entry exempts one codepoint in one path."},
  	{Kind: "config", Key: "proseGate.exemptions[].path", Type: "string", Default: "required", Availability: "when `proseGate.enabled` is true", Description: "The repo-relative path the exemption covers. A rendered file and the source it renders from each need their own entry."},
  	{Kind: "config", Key: "proseGate.exemptions[].codepoint", Type: "string", Default: "required", Availability: "when `proseGate.enabled` is true", Description: "The exempted codepoint, spelled `U+2014`, never the character itself: this file is scanned, so a typed character here would be a finding against the file that configures the exemption."},
  	{Kind: "config", Key: "proseGate.exemptions[].count", Type: "int", Default: "unset (any number is permitted)", Availability: "when `proseGate.enabled` is true", Description: "The exact number of occurrences expected. Set, an added occurrence in an exempt file still fails, which suits a frozen record; unset, any number is permitted, which suits a living file."},
  ```

  That is six entries, not five: the block, `enabled`, the list, and three fields. Add the
  live-state case for each key in `internal/project/configreference.go`, following the one-case-per-key
  shape already there (`:109-179`).

- [ ] **Task 6.6: Add `internal/prosegate/prosegate_test.go` and `cmd/awf/prosegate_test.go`.**
  These carry the ADR-0008 proof markers for both of ADR-0119's invariants. The 100% coverage floor
  applies, so every branch above needs a case: clean tree, finding, unexempted codepoint, exempt
  with nil count, exempt with a matching pin, exempt with a mismatched pin, invalid codepoint
  spelling, a codepoint outside the banned set, non-UTF-8 content skipped, unreadable path, knob
  absent, knob false, and the refusal path.

  The package test carries:

  ```go
  // invariant: prose-gate-tracked-file-scan
  func TestScanReportsBannedRunesOutsideExemptions(t *testing.T) {
  ```

  The command test carries the refusal proof. `t.TempDir()` is outside any git repository, which is
  exactly the condition the invariant names:

  ```go
  // invariant: prose-gate-refuses-without-git
  func TestProseGateRefusesOutsideAGitRepo(t *testing.T) {
  ```

  That test must assert the command exits non-zero **and** that its stderr names the failure; a test
  that only checks the exit code would pass against a command that silently reported a clean tree,
  which is the failure mode ADR-0119 item 12 forbids.

- [ ] **Task 6.7: Verify and commit.** Run `./x gate` (green: 100% coverage, no dead code) and `./x
  check` (`awf check: clean`; `docs/config-reference.md` regenerates from the new configspec entries,
  so run `./x sync` first and stage it, along with `examples/sundial`'s regenerated copy and both
  lock files). Commit:

  ```commit
  feat(tooling): add the prose-gate command and its config keys
  ```

## Phase 7: Wire the rendered pre-commit payload

ADR-0119 item 8. The payload gains a `prose-gate` line the way `commit-msg.sh.tmpl` already carries
`awf commit-gate "$1"`, and the bootstrap-shim guard widens to cover the new bare-`awf` call site.

- [ ] **Task 7.1: Add the `proseGateCmd` var and widen the shim guard.** In
  `templates/hooks/pre-commit.sh.tmpl`:

  ```diff
  --- a/templates/hooks/pre-commit.sh.tmpl
  +++ b/templates/hooks/pre-commit.sh.tmpl
  @@ -6,3 +6,3 @@
  -{{ if not .vars.checkCmd }}
  +{{ if or (not .vars.checkCmd) (not .vars.proseGateCmd) }}
   # Run the pinned awf when the bootstrap resolves; fall back to PATH awf.
   awf() { local pinned; if [ -f .awf/bootstrap.sh ] && pinned="$(bash .awf/bootstrap.sh 2>/dev/null)"; then "$pinned" "$@"; else command awf "$@"; fi; }
   {{ end }}
  @@ -10,3 +10,5 @@
   {{- with .vars.checkCmd }}{{ . }}{{ else }}awf check{{ end }}
   {{- with .vars.gateCmd }}
   {{ . }}
   {{- end }}
  +{{ with .vars.proseGateCmd }}{{ . }}{{ else }}awf prose-gate{{ end }}
  ```

  **The guard is a disjunction and this is the point of the task.** The shim is needed when *any*
  call site lacks its var, not when all do. A conjunction would leave an adopter who sets `checkCmd`
  and not `proseGateCmd` rendering a bare unshimmed `awf prose-gate`, losing bootstrap pinning for
  that one line. awf is itself that adopter: it sets `checkCmd: ./x check`.

- [ ] **Task 7.2: Register the var.** Add `proseGateCmd` to the var catalog beside `checkCmd` and
  `gateCmd`, with a descriptor stating that it names the command the rendered pre-commit payload
  calls to run the prose scan, and that unset renders `awf prose-gate`. Follow whatever shape
  `gateCmd`'s descriptor already takes; the catalog is the single source of the var's
  configspec-published description, so do not write a second one.

- [ ] **Task 7.3: Verify publication-safety and both rendered payloads.** Run `./x sync`, then
  confirm the two renderings. With awf's own vars (`checkCmd` set, `proseGateCmd` not yet set, as of
  this phase), `.awf/hooks/pre-commit.sh` must contain the shim **and** a bare `awf prose-gate` line:

  ```bash
  grep -c 'pinned=' .awf/hooks/pre-commit.sh && grep -n 'prose-gate' .awf/hooks/pre-commit.sh
  ```

  Expected: `1`, and a line reading `awf prose-gate`. That combination is the disjunction working;
  if the shim is absent the guard was written as a conjunction. Confirm `examples/sundial`'s payload
  renders coherently too, and that no rendering contains a no-value token:

  ```bash
  git grep -n 'no value\|<no value>' -- .awf/hooks examples/sundial | wc -l
  ```

  Expected: `0`.

- [ ] **Task 7.4: Verify and commit.** Run `./x gate` (green) and `./x check` (`awf check: clean`).
  Stage the template, the catalog, both regenerated payloads, the regenerated config reference, and
  both lock files. Commit:

  ```commit
  feat(rendering): wire prose-gate into the pre-commit payload
  ```

## Phase 8: Enable it for awf, correct the docs, and flip

ADR-0119 items 6, 11, 14 and 15. awf takes its own dogfood, the four exemptions are declared, and
the two documents that currently state the old rule are corrected. This is the final commit: it
carries both status flips.

- [ ] **Task 8.1: Enable the knob and declare the four exemptions.** In `.awf/config.yaml`. Note the
  three judgements span four entries: the pitfalls curly quote exists in the sidecar source **and**
  in its render target, and an entry names one path.

  ```yaml
  proseGate:
    enabled: true
    exemptions:
      - path: docs/decisions/0113-em-dash-free-shipped-templates.md
        codepoint: U+2014
        count: 7
      - path: docs/plans/2026-07-13-invariant-backing-migration-to-enforced-test-scoped-backing.md
        codepoint: U+2014
        count: 1
      - path: .awf/docs/pitfalls.yaml
        codepoint: U+201C
        count: 1
      - path: docs/pitfalls.md
        codepoint: U+201C
        count: 1
  ```

  All four are pinned, because all four are frozen or effectively so. No entry names a path under
  `templates/` or `changelog/`: an exemption there would ship a banned codepoint, and ADR-0115's gate
  is the only thing that would catch it (ADR-0119 item 7).

- [ ] **Task 8.2: Add the `./x prose-gate` arm and wire the gate.** In `x`, add a `prose-gate` case
  running `go run ./cmd/awf prose-gate`, and add the same call to the `gate` arm beside
  `pincheck`, so CI (which runs `./x gate`) scans. Then set `proseGateCmd: ./x prose-gate` in
  `.awf/config.yaml`'s `vars`, so the rendered payload calls the from-source binary rather than a
  PATH `awf`: awf builds from source with `bootstrap.enabled: false`, so a bare PATH call is exactly
  what it must not make.

  awf's pre-commit therefore scans twice, once through the payload's `./x gate` line and once
  through its own `./x prose-gate` line. ADR-0119 item 8 accepts and names this: a rendered payload
  cannot know what a project's runner folds into its gate.

- [ ] **Task 8.3: Correct the agent guide's invariant bullet.** `.awf/agents-doc.yaml:29-30` still
  tells the Go author that "Go comments and tests are out of scope", which items 1, 4 and 11 make
  false. Rewrite the bullet's text to state the repo-wide rule and the opt-in command, and widen its
  `ref` to `ADR-0115, ADR-0119` (the field takes a comma list; line 20 already reads
  `ref: ADR-0001, ADR-0045`, so no schema change is needed). The bullet must not type any of the
  seven codepoints. Re-render with `./x sync` and stage `AGENTS.md`.

- [ ] **Task 8.4: Name the hyphen in the shipped doc-standard.** ADR-0119 item 14.

  ```diff
  --- a/templates/docs/doc-standard.md.tmpl
  +++ b/templates/docs/doc-standard.md.tmpl
  @@ -16,1 +16,1 @@
  -- **Plain punctuation.** Every awf-managed doc, shipped or authored (ADRs, plans, and hand-written docs), uses plain punctuation: a colon, semicolon, comma, or parentheses where an em-dash would go, an ASCII hyphen for a range, three periods for elision, and ASCII quotes for quoting. Seven typographic substitutes are banned, as they read as machine-set: the em-dash (U+2014), en-dash (U+2013), ellipsis (U+2026), and the four curly quotes (U+2018, U+2019, U+201C, U+201D). Notation (arrows, mathematical symbols, accented letters) is unaffected.
  +- **Plain punctuation.** Every awf-managed doc, shipped or authored (ADRs, plans, and hand-written docs), uses plain punctuation: a colon, semicolon, comma, hyphen, or parentheses where an em-dash would go, an ASCII hyphen for a range, three periods for elision, and ASCII quotes for quoting. Seven typographic substitutes are banned, as they read as machine-set: the em-dash (U+2014), en-dash (U+2013), ellipsis (U+2026), and the four curly quotes (U+2018, U+2019, U+201C, U+201D). Notation (arrows, mathematical symbols, accented letters) is unaffected.
  ```

  Scope is untouched: ADR-0117 item 8 already widened this line to "shipped or authored". Only the
  replacement list changes. Re-render and stage `docs/doc-standard.md` and the sundial copy.

- [ ] **Task 8.5: Add the changelog entry.** Under `[Unreleased]` in `changelog/CHANGELOG.md`. It
  must name the new command, the two config keys, the default-off posture, and the pre-commit payload
  re-render adopters will see as drift. **ADR-0115's gate scans `changelog.FS`**, so the entry must
  describe the ban without typing any of the seven codepoints, and must cite no ADR number (the
  changelog is adopter-facing prose).

- [ ] **Task 8.6: Flip both statuses.** Set `status: Implemented` in
  `docs/decisions/0119-repo-wide-plain-punctuation-the-remaining-surfaces-and-an-opt-in-prose-gate.md`
  and in this plan's frontmatter. Run `./x sync` to regenerate `docs/decisions/ACTIVE.md` and the
  four domain indexes. Once ADR-0119 is Implemented, `./x check` enforces its two backed invariant
  slugs, so both proof markers from Task 6.6 must already be in place.

- [ ] **Task 8.7: Verify and commit.** Run `./x gate` (green; it now includes the prose scan) and
  `./x check` (`awf check: clean`, `awf invariants: clean`). Confirm the gate actually enforces:

  ```bash
  go run ./cmd/awf prose-gate && echo "EXIT 0 as expected"
  ```

  Expected: `prose-gate: clean` and `EXIT 0 as expected`. Then prove it fails on a real violation
  rather than passing vacuously, which is the whole point of the phase:

  ```bash
  printf 'x \xe2\x80\x94 y\n' >> README.md
  go run ./cmd/awf prose-gate; echo "exit=$?"
  git checkout README.md
  ```

  Expected: a line naming `README.md` and the em-dash, then `exit=1`. A green `exit=0` here means the
  scan is not reaching tracked files and must be fixed before the commit lands.

  Stage everything and commit:

  ```commit
  feat(tooling): enable prose-gate for awf and flip ADR-0119
  ```

## Verification

The effort is done when all of the following hold:

1. `git grep -oP '[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]' | wc -l` prints `10`,
   and every one is covered by a Phase 8 exemption entry.
2. `go run ./cmd/awf prose-gate` prints `prose-gate: clean` and exits zero.
3. The vacuity check above fails as specified when a codepoint is planted, and recovers on
   `git checkout`.
4. `./x gate` and `./x check` are green, with `awf invariants: clean` proving both ADR-0119 slugs
   are backed.
5. The word-stream proof reports `word-stream: PASS` for every swept path against the commit
   preceding its phase.
6. `gofmt -l ./internal ./cmd ./changelog` prints nothing.
7. `docs/decisions/ACTIVE.md` lists ADR-0119 as Implemented, and this plan reads
   `status: Implemented`.

## Notes

- **The exemption list is a trust boundary.** A future entry naming a path under `templates/` or
  `changelog/` would ship a banned codepoint to every adopter; only ADR-0115's gate would catch it.
  A reviewer seeing such an entry should treat it as a defect, not a judgement call.
- **Out of scope, deliberately:** re-styling the bare hyphens from commit `8338840` (ADR-0119 item 13
  blesses them); the `// touches-invariant: <slug> - <note>` markers legitimately use ` - ` as a
  delimited field and must not be "fixed" to a colon.
- **A known gap, named in ADR-0119's Consequences:** nothing protects `./x`'s gate wiring. Deleting
  the `prose-gate` line from the `gate` arm silently retires this gate, exactly as it would ADR-0012's
  coverage gate or ADR-0063's dead-code gate. That is pre-existing and general, and belongs to its own
  ADR rather than to this plan.
- **If a sweep phase finds a genuine depiction** (prose that is *about* a banned character, where
  punctuating it would make a true statement false), do not sweep it and do not invent an exemption
  silently: ADR-0118 item 3's precedent is that the sweep agent flags it and the ADR is amended to
  authorise it. ADR-0119 is Proposed until Phase 8, so its body is still mutable.
