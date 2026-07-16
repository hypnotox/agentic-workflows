---
date: 2026-07-16
adrs: [120]
status: Proposed
---
# Plan: Structured supersession tokens, checks, and the corpus retrofit

## Goal

Execute ADR-0120: parse both supersession flavours, land the five corpus checks and two
advisories, subsume invariant retirement under `supersedes-invariant:` tokens, remove
`retires_invariants:` via a generation-10 corpus migration, render supersession into ACTIVE.md and
`awf context`, and retrofit this repo's own corpus (including `examples/sundial`). Non-goals:
tokenizing any corpus other than this repo's and sundial's, repairing adopter corpora beyond what
the migration writes, and any new `.awf/config.yaml` key (the feature is unconditional).

The design lives in ADR-0120. This plan does not re-argue it.

## Architecture summary

Phase order is forced by two constraints. First, the dead-code gate: every parsing surface lands
in the same phase as its first consumer, so Phase 1 pairs the parser with the Decision-format
check, and Phase 2 builds the supersession checks on it. Second, the retirement cutover must stay
green at every commit: Phase 3 makes token retirement work *alongside* `retires_invariants:`
(dual-read), Phase 4 migrates both corpora to tokens, and only then does Phase 5 delete the legacy
field and refuse the key - at no point does a commit sit between "retirements dropped" and
"retirements re-expressed".

Phases 6 and 7 are fan-out: rendering (ACTIVE.md, `awf context`) and the prose surfaces (embedded
templates, catalog strings, config-side parts), each re-syncing both this repo and sundial. Phase
8 is the hand-retrofit ADR-0120 item 11 commits to (tokenize freeform citations, backfill
back-pointers) and the terminal flip of ADR-0120 and this plan to Implemented.

Expected advisory state during the effort: three `note: invariant marker "inv-retirement-*" names
a slug no Implemented ADR declares` lines persist until Phase 5 deletes those legacy proofs;
ADR-0120's own twelve proof markers are dangling notes until the Phase 8 flip declares them.

## File structure

- **Created:** `internal/migrate/retirementtokens.go`, `internal/migrate/retirementtokens_test.go`,
  `internal/project/supersession.go`, `internal/project/supersession_test.go`.
- **Modified:** `internal/adr/adr.go`, `internal/adr/adr_test.go`,
  `internal/invariants/invariants.go`, `internal/invariants/invariants_test.go`,
  `internal/project/check.go`, `internal/project/context.go`, `internal/project/context_test.go`,
  `internal/migrate/migrate.go`, `internal/testsupport/testsupport.go`,
  `internal/catalog/standard.go`, `cmd/awf/check.go` (only if the note channel needs a new call),
  `templates/skills/adr-lifecycle/SKILL.md.tmpl`, `templates/agents-doc/AGENTS.md.tmpl`,
  `templates/adr-readme/README.md.tmpl`, `.awf/agents-doc.yaml`, `.awf/parts/adr-template/frontmatter.md`,
  `.awf/agents/adr-reviewer.yaml`, `.awf/domains/parts/{adr-system,invariants,config,rendering}/current-state.md`,
  `.awf/docs/glossary.yaml`, `.awf/docs/pitfalls.yaml`, `changelog/CHANGELOG.md`,
  `docs/architecture.md`,
  `docs/decisions/0120-*.md` (status flip), this plan (status flip), and the rendered/generated
  fan-out: `docs/decisions/*.md` (88 key strips, 12 appended items, tokenization sweep,
  back-pointer edits), `docs/decisions/ACTIVE.md`, `docs/decisions/README.md`,
  `docs/decisions/template.md`, `AGENTS.md`, `docs/domains/*.md`, `docs/glossary.md`,
  `docs/pitfalls.md`, `.claude/skills/awf-adr-lifecycle/SKILL.md`, `.claude/agents/adr-reviewer.md`,
  `.awf/awf.lock`, and `examples/sundial/**` (lock, rendered skills/docs).
- **Deleted:** none.

## Phase 1: Parse supersession, enumerate Decision items, check the format

Everything later reads what this phase parses. The Decision-format check is the phase's consumer,
so the dead-code gate passes.

- [ ] **Task 1.1: Extend `internal/adr` with supersession parsing.** In `internal/adr/adr.go`:

  Add to `adrFrontmatter` and to the `ADR` struct (with matching assignment in `parse`):

  ```go
  	Supersedes []int `yaml:"supersedes"`
  ```

  ```go
  	Supersedes        []int             // `supersedes:` frontmatter: full-supersession claims (ADR-0120)
  	Refs              []SupersessionRef // inline partial-supersession tokens in the Decision section (ADR-0120)
  ```

  (gofmt-align both insertions with the surrounding field blocks; the snippets show content,
  not final column alignment.)

  Add the token types and extraction:

  ```go
  // SupersessionRef is one inline partial-supersession token (ADR-0120):
  // `supersedes: ADR-NNNN#<item>` or `supersedes-invariant: ADR-NNNN#<slug>`
  // as an inline code token inside a Decision section. Exactly one of
  // Item/Slug is set; the key names the kind, never the anchor's shape.
  type SupersessionRef struct {
  	Target string // 4-digit target ADR number, e.g. "0116"
  	Item   int    // Decision item number; 0 for an invariant ref
  	Slug   string // invariant slug; "" for an item ref
  }

  var (
  	// itemRefRe / invRefRe match the two token keys inside inline code. The
  	// superseded kind is named by the key, never inferred from the anchor
  	// (ADR-0120 item 1): items are [1-9][0-9]*, slugs the declRe grammar.
  	itemRefRe = regexp.MustCompile("`supersedes: ADR-([0-9]{4})#([1-9][0-9]*)`")
  	invRefRe  = regexp.MustCompile("`supersedes-invariant: ADR-([0-9]{4})#([a-z0-9-]+)`")
  	// decisionItemRe matches a column-0 numbered Decision item lead. Column-0
  	// anchoring is load-bearing: 0067 and 0115 carry indented numbered
  	// sub-lists that must not enumerate (ADR-0120 item 2).
  	decisionItemRe = regexp.MustCompile(`(?m)^([0-9]+)\. `)
  )

  // parseRefs extracts every supersession token from a Decision section body.
  // Tokens anywhere else in the ADR are inert prose (ADR-0120 item 1), which
  // parse guarantees by passing only Sections["Decision"].
  func parseRefs(decision string) []SupersessionRef {
  	var refs []SupersessionRef
  	for _, m := range itemRefRe.FindAllStringSubmatch(decision, -1) {
  		n, _ := strconv.Atoi(m[2]) // the regex admits only digits
  		refs = append(refs, SupersessionRef{Target: m[1], Item: n})
  	}
  	for _, m := range invRefRe.FindAllStringSubmatch(decision, -1) {
  		refs = append(refs, SupersessionRef{Target: m[1], Slug: m[2]})
  	}
  	return refs
  }

  // DecisionItems returns the numbers of the column-0 numbered items of the
  // Decision section, in order of appearance.
  func (a ADR) DecisionItems() []int {
  	var items []int
  	for _, m := range decisionItemRe.FindAllStringSubmatch(a.Sections["Decision"], -1) {
  		n, _ := strconv.Atoi(m[1])
  		items = append(items, n)
  	}
  	return items
  }
  ```

  In `parse`, set `Supersedes: fm.Supersedes` on the returned ADR and assign
  `a.Refs = parseRefs(a.Sections["Decision"])` after `a.Sections` is built.

- [ ] **Task 1.2: Tests for parsing and enumeration.** In `internal/adr/adr_test.go`, table-driven
  cases: both token kinds extracted with correct Target/Item/Slug; a token outside the Decision
  section ignored; a leading-zero item anchor (`#03`) and an uppercase slug not matched; an
  indented `  1. ` sub-item not enumerated; multi-digit items (`13. `) enumerated;
  `supersedes: [31]` frontmatter round-trips; a fenced code block inside the Decision section
  containing a column-0 `1. ` line and a backticked token example, pinning the accepted
  behavior: the body is read raw, fences included (the corpus is fence-clean at column 0
  today; the pin makes any future surprise a test failure, not silent drift).

- [ ] **Task 1.3: The Decision-format check.** New file `internal/project/supersession.go` opens
  with the format check (the supersession checks join it in Phase 2):

  ```go
  // checkDecisionFormat enforces ADR-0120 item 12: every ADR's Decision section
  // consists of column-0 numbered items, sequential from 1, regardless of
  // status - a Superseded ADR can still be an anchor target.
  // touches-invariant: decision-items-enumerable - the format check itself; proof in supersession_test.go
  func (p *Project) checkDecisionFormat(adrs []adr.ADR, rel string) []manifest.Drift {
  	var drift []manifest.Drift
  	for _, a := range adrs {
  		items := a.DecisionItems()
  		if len(items) == 0 {
  			drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-decision-format", Detail: fmt.Sprintf("ADR-%s: Decision section has no column-0 numbered items", a.Number)})
  			continue
  		}
  		for i, n := range items {
  			if n != i+1 {
  				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-decision-format", Detail: fmt.Sprintf("ADR-%s: Decision item %d found where %d expected (gap, duplicate, or restart)", a.Number, n, i+1)})
  				break
  			}
  		}
  	}
  	return drift
  }
  ```

  Wire it in `internal/project/check.go` beside `checkADRRelatedLinks` (the established
  ParseDir-based corpus-check seam), via a `checkSupersessionAll` aggregator that Phase 2 fills
  in; in this phase the aggregator parses the corpus and calls only `checkDecisionFormat`.

  The proof marker `// invariant: decision-items-enumerable` goes on the corresponding test in
  `internal/project/supersession_test.go` (new file): fixture ADRs exercising no-items, a gap
  (1, 3), a duplicate (1, 1), a restart (1, 2, 1), a multi-digit sequence to 13, and an indented
  sub-list that must not count.

- [ ] **Task 1.4: Run the format check over both corpora.**

  ```bash
  ./x check
  ```

  Expected: `awf check: clean` with exactly four dangling-marker notes: the three legacy
  `inv-retirement-*` plus `decision-items-enumerable`, whose proof landed this phase while
  ADR-0120 is still Proposed. (The grounding sweep verified all 120 of this repo's ADRs and
  all 3 sundial ADRs already comply with the format.) Each later phase adds one note per new
  ADR-0120 proof slug: expect 9 after Phase 2, 12 after Phase 4, 10 after Phase 5 (the three
  legacy notes go, `retires-invariants-key-refused` arrives), 12 after Phase 6, and 0 after
  the Task 8.4 flip.

- [ ] **Task 1.5: Commit.**

  ```commit
  feat(adr-system): parse supersession refs, check Decision format
  ```

## Phase 2: The supersession corpus checks and advisories

- [ ] **Task 2.1: Export the declaration scan.** In `internal/invariants/invariants.go`, extract
  the declaration matching of `DeclaringADRs` into an exported helper so the ref-validity check
  and Phase 4's migration resolve slugs against the same grammar (ADR-0120 item 2's "same
  declaration grammar as invariant backing"):

  ```go
  // DeclaredSlugs returns the invariant slugs a's Invariants section declares
  // (backed and unbacked alike), in declaration order. Status-independent: the
  // ref-validity check and the retirement migration resolve slug anchors against
  // any ADR's declarations, not just Implemented ones (ADR-0120 item 2).
  func DeclaredSlugs(a adr.ADR) []string {
  	var slugs []string
  	for _, bullet := range invariantBullets(a.Sections["Invariants"]) {
  		if m := declRe.FindStringSubmatch(bullet); m != nil {
  			slugs = append(slugs, m[2])
  		}
  	}
  	return slugs
  }
  ```

  `DeclaringADRs` keeps its own loop (it needs the full bullet for class/Verify extraction); the
  helper is the shared *grammar*, not shared plumbing. No behavior change; existing tests green.

- [ ] **Task 2.2: The four error checks and two advisories.** In
  `internal/project/supersession.go`, fill the aggregator:

  ```go
  // checkSupersessionAll runs the drift half of the ADR-0120 corpus checks:
  // Decision format, full-supersession three-way symmetry, token ref validity,
  // partial back-pointers, and flavour exclusivity. The advisory-note half is
  // supersessionNotes' (AdvisoryNotes' source); both consume computeSupersession.
  func (p *Project) checkSupersessionAll() ([]manifest.Drift, error) {
  	adrs, err := adr.ParseDir(p.decisionsDir())
  	if err != nil { // reachable via a direct call over a malformed ADR; pre-empted inside full Check()
  		return nil, err
  	}
  	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
  	drift := p.checkDecisionFormat(adrs, rel)
  	d2, _ := computeSupersession(adrs, rel)
  	return append(drift, d2...), nil
  }
  ```

  `computeSupersession(adrs, rel) ([]manifest.Drift, []string)` implements, over one pass with
  `byNum map[string]adr.ADR` and an
  `add(a, kind, detail)` helper appending `manifest.Drift{Path: rel + "/" + a.Filename, ...}`:

  - **Full symmetry (`Kind: "adr-supersession"`):** for each `a.Supersedes` entry `n`
    (formatted `%04d`): the target exists; the target's `Status` is exactly
    `"Superseded by ADR-" + a.Number`; the target's `SupersededBy` equals `a.Number`. Reverse
    direction: every ADR whose `Status` has the `Superseded` prefix or whose `SupersededBy` is
    non-empty must carry both in the matching suffixed/scalar form and have exactly one claimant
    whose `Supersedes` names it; a second claimant is a drift on the higher-numbered one.
  - **Ref validity (`Kind: "adr-token-ref"`):** for each `r` in `a.Refs`: the target exists;
    the target is not `Proposed`; an item ref satisfies
    `r.Item <= len(target.DecisionItems())` (the format check guarantees 1..N); an invariant
    ref's slug is in `invariants.DeclaredSlugs(target)`.
  - **Back-pointer (`Kind: "adr-token-backpointer"`):** a ref whose target's status is
    `Accepted` or `Implemented` requires `slices.Contains(target.Related, <a.Number as int>)`.
  - **Flavour exclusivity (`Kind: "adr-token-exclusive"`):** a ref whose `Target` also appears
    in `a.Supersedes` is a drift.
  - **Advisories (returned as `[]string`):** (1) a ref whose target's status has the
    `Superseded` prefix: `"ADR-%s token targets ADR-%s, which was fully superseded"`; (2) an
    anchor (target + item or slug) claimed by refs in two or more **live** ADRs - `Accepted`
    or `Implemented`, ADR-0120 item 4's definition; a Proposed claimant is not yet in force:
    `"anchor ADR-%s#%s claimed by ADR-%s and ADR-%s"`. Notes never enter the drift slice.

  Wiring, one shape, no either/or: split the compute from the plumbing. A
  `computeSupersession(adrs, rel)` returns `(drift, notes)`; `checkSupersessionAll` (called
  from `Check()` after `checkADRRelatedLinks`) consumes only the drift half; a new
  `supersessionNotes()` source re-parses the corpus and returns the notes half, appended in
  `AdvisoryNotes()` (internal/project/check.go:28) exactly as its existing note sources are -
  mirroring how tag-health notes reach `cmd/awf/check.go:33`. The double ParseDir matches the
  corpus checks' existing per-check parse pattern.

- [ ] **Task 2.3: Tests.** In `internal/project/supersession_test.go`, fixture corpora via
  `internal/testsupport` ADR builders, covering: symmetric full pair passes; each one-sided
  form fails; two full claimants fail; dangling item ref, out-of-range item, unknown slug, and
  Proposed target each fail; live target without back-pointer fails, with back-pointer passes;
  token plus frontmatter into one target fails; token into a Superseded target yields a note and
  no drift; two live claimants of one anchor yield a note. Extend the builders with
  `WithSupersedes([]int)`; use the existing `WithBody` (`internal/testsupport/testsupport.go:115`)
  for Decision-section fixtures. Proof markers on these tests:
  `// invariant: supersession-full-symmetry`, `// invariant: supersession-token-ref-validity`,
  `// invariant: supersession-backpointer`, `// invariant: supersession-flavour-exclusive`,
  `// invariant: supersession-conflict-advisory`. Then confirm this repo's corpus is green:
  `./x check` prints `awf check: clean` (nine dangling-marker notes: three legacy plus six
  ADR-0120 proofs landed so far).

- [ ] **Task 2.4: Commit.**

  ```commit
  feat(adr-system): add the supersession corpus checks
  ```

## Phase 3: Token-based retirement (dual-read)

- [ ] **Task 3.1: Apply `supersedes-invariant:` retirements in `DeclaringADRs`.** In
  `internal/invariants/invariants.go`, after the existing `RetiresInvariants` retirement loop,
  add the token path (dual-read until Phase 5 removes the legacy loop):

  ```go
  	// Token retirements (ADR-0120 item 6): a `supersedes-invariant:` token
  	// carried by an Implemented ADR drops its slug from owed backing. Dangling
  	// detection scans every ADR's declarations - a slug declared only by a
  	// non-Implemented ADR is not owed, so retiring it is a no-op, not an error.
  	// touches-invariant: token-retirement-implemented-only - the status test below; proof in invariants_test.go
  	// touches-invariant: token-retirement-dangling-errors - the declaredAnywhere refusal; proof in invariants_test.go
  	declaredAnywhere := map[string]bool{}
  	for _, a := range adrs {
  		for _, slug := range DeclaredSlugs(a) {
  			declaredAnywhere[slug] = true
  		}
  	}
  	for _, a := range adrs {
  		if a.Status != "Implemented" {
  			continue
  		}
  		for _, r := range a.Refs {
  			if r.Slug == "" {
  				continue
  			}
  			if !declaredAnywhere[r.Slug] {
  				return nil, fmt.Errorf("dangling retirement: ADR %s supersedes invariant %q, which no ADR declares", a.Filename, r.Slug)
  			}
  			delete(required, r.Slug)
  		}
  	}
  ```

- [ ] **Task 3.2: Tests.** In `internal/invariants/invariants_test.go`: an Implemented carrier's
  token drops the slug; a Proposed, an Accepted, and a Superseded carrier each leave it owed
  (the Superseded case is the lapse semantics of ADR-0120 item 6); a token for an undeclared
  slug errors with the "dangling retirement" message; a slug declared only by a Superseded ADR
  retires as a no-op without error. Proof markers:
  `// invariant: token-retirement-implemented-only`,
  `// invariant: token-retirement-dangling-errors`.

- [ ] **Task 3.3: Commit.**

  ```commit
  feat(invariants): retire slugs via supersedes-invariant tokens
  ```

## Phase 4: The generation-10 corpus migration, run on both corpora

- [ ] **Task 4.1: The migration.** New file `internal/migrate/retirementtokens.go` implementing
  `applyRetirementTokens(root string, out io.Writer) error`, registered in `migrate.go` as
  `{To: 10, Name: "retirement-tokens", Apply: applyRetirementTokens}` (making `Current()` = 10;
  the ADR-0039 gate then forces every stale project through `awf upgrade`).

  Behavior, per ADR-0120 item 8 and the `upgrade-migrates-retirements` invariant:

  1. Load config as `applyCloseEnabledSet` does, resolve `<DocsDir>/decisions`; a root with no
     decisions dir is a no-op (adopters without the docs module).
  2. Glob `NNNN-*.md`. For each file, locate the column-0 `retires_invariants:` line inside the
     frontmatter block (the corpus form is a single line; a multi-line YAML list under that key
     fails the migration naming the file, keeping meaning-preservation checkable).
  3. Strip the line. For a non-empty list, resolve each slug to its declaring ADR via
     `invariants.DeclaredSlugs` over the *pre-edit* parsed corpus; no declarer → error
     `retirement-tokens: %s retires %q, declared by no ADR`; two or more declarers → error
     naming the slug and every declarer (the loud-failure posture; this corpus's 14 retired
     slugs each resolve uniquely); then append to the file's Decision
     section (immediately before the next `## ` heading), with `N` = last item number + 1:

     ```
     N. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,
        ADR-0120).** This ADR retires `supersedes-invariant: ADR-XXXX#<slug>`[, ...].
     ```

  4. For every token written, parse the target ADR's `related:` line and insert the carrier's
     number when absent (bare int, preserving existing order, appended last).
  5. Print one `retirement-tokens: <op>` line per stripped key, appended item, and back-pointer
     insertion. Idempotent: a corpus with no `retires_invariants:` keys prints nothing.

  Edit raw bytes with string surgery, never re-serialize frontmatter (re-serialization would
  reorder keys and violate meaning-preservation); model loud-failure and no-op paths on
  `applyCloseEnabledSet`.

- [ ] **Task 4.2: Migration tests.** `internal/migrate/retirementtokens_test.go`, fixture trees
  via `internal/testsupport` (the package convention; no golden files): empty key (stripped, no
  item appended); non-empty key (stripped, item appended with the token, target `related:` gains
  the carrier, output lines match exactly); target already back-pointed (no duplicate);
  unresolvable slug (error naming file and slug); multi-line key (error); re-run after migration
  (no output, byte-identical corpus); no decisions dir (no-op). Proof marker:
  `// invariant: upgrade-migrates-retirements`.

- [ ] **Task 4.3: Migrate this repo and sundial.**

  ```bash
  go build -o /tmp/awf ./cmd/awf
  go run ./cmd/awf upgrade && (cd examples/sundial && /tmp/awf upgrade)
  ./x sync && ./x check && ./x invariants
  ```

  (The repo root is not a main package and sundial is its own Go module, so the sundial run
  uses a prebuilt binary, following `./x`'s build pattern.)

  Expected: `retirement-tokens:` lines for 88 stripped keys, 12 appended items, and the
  back-pointer insertions the corpus lacks (grounding measured 13 of 14 edges absent; the run
  prints the exact list); sundial prints no `retirement-tokens:` lines (its ADRs never carried
  the key) but its lock stamps to generation 10; then `awf check: clean` (twelve
  dangling-marker notes: three legacy plus nine ADR-0120 proofs landed so far) and
  `awf invariants: clean` - the 12 retiring ADRs' retirements now flow through Phase 3's token
  path.

- [ ] **Task 4.4: Commit** (code, both corpora, both locks: one concern, the cutover). Include
  the `docs/architecture.md` currency edit in this commit: extend its `internal/migrate` bullet
  (lines ~71-72) with the corpus-writing generation-10 migration and the new
  `internal/migrate` → `internal/adr` import, per ADR-0120 item 8's precedent framing.

  ```commit
  feat(config): migrate retires_invariants to supersession tokens
  ```

## Phase 5: Remove `retires_invariants:` from the schema

- [ ] **Task 5.1: Drop the field and the legacy path.** In `internal/adr/adr.go`: delete
  `RetiresInvariants` from `ADR` and `adrFrontmatter`. In `internal/invariants/invariants.go`:
  delete the legacy retirement loop (the Phase 3 token loop remains) and its
  `dangling retirement: ADR %s retires %q, which no Implemented ADR declares` error string. In
  `internal/testsupport/testsupport.go`: delete `WithRetiresInvariants` and its serialization
  arm. Delete the legacy-path tests in `internal/invariants/invariants_test.go` (the ones
  carrying the old `inv-retirement-*` proof markers around lines 703-731) - their behavioral
  coverage moved to Task 3.2, and deleting them clears the three dangling-marker advisory notes.

- [ ] **Task 5.2: Refuse the raw key.** In `internal/project/supersession.go`, extend
  `checkSupersessionAll` with a raw-frontmatter scan (the parsed struct no longer sees the key;
  non-strict YAML would silently ignore it - ADR-0120 item 7): for each `NNNN-*.md`, a line of
  the frontmatter block matching `^retires_invariants:` emits `Kind: "adr-retired-key"`,
  `Detail: "ADR-%s: retires_invariants: is no longer read; run awf upgrade"`. Test fixtures:
  the key present empty and non-empty both drift; a clean file does not. Proof marker:
  `// invariant: retires-invariants-key-refused`.

- [ ] **Task 5.3: Drop the key from the scaffolding template.** Edit
  `.awf/parts/adr-template/frontmatter.md`: delete the `retires_invariants: []` line. Run
  `./x sync`; `docs/decisions/template.md` regenerates without it, so `awf new adr` from this
  commit on cannot scaffold the refused key.

- [ ] **Task 5.4: Gate and commit.** `./x gate`; the three legacy dangling-marker notes are gone
  as of this commit, leaving ten (nine prior ADR-0120 proofs plus
  `retires-invariants-key-refused`).

  ```commit
  feat(invariants): remove retires_invariants from the ADR schema
  ```

## Phase 6: Rendering - ACTIVE.md and `awf context`

- [ ] **Task 6.1: The shared supersession index.** In `internal/adr/adr.go`:

  ```go
  // Override is one superseded anchor on a live ADR: the anchor (item number or
  // slug) and the successor that superseded it (ADR-0120 item 10).
  type Override struct {
  	Item      int    // 0 for a slug override
  	Slug      string // "" for an item override
  	Successor string // 4-digit successor number
  }

  // SupersessionIndex derives the render view: full chains (predecessor,
  // successor) sorted by predecessor, and per-target overrides from every
  // non-Superseded ADR's refs, item refs before slug refs, in (anchor,
  // successor) order.
  func SupersessionIndex(adrs []ADR) (chains [][2]string, overrides map[string][]Override)
  ```

  Chains come from each ADR's `Supersedes` (formatted `%04d`); overrides from each ADR whose
  status lacks the `Superseded` prefix, one entry per ref. Rendering deliberately includes
  Proposed carriers where the conflict advisory (Task 2.2) does not: the annotation is
  discoverability, not enforcement, the window is transient (authoritative at the Task 8.4
  flip), and Phase 6's own verification needs the 0116 annotations observable while 0120 is
  still Proposed.

- [ ] **Task 6.2: ACTIVE.md.** In `RenderActiveMD`, after the status groups, when
  `SupersessionIndex` returns anything, append (omitting either subsection when empty:
  publication-safe degradation, and sundial's ACTIVE.md stays byte-identical):

  ```
  ## Supersedence

  ### Chains

  - ADR-0031 superseded by ADR-0120

  ### Superseded anchors on live ADRs

  - ADR-0116: item 2 superseded by ADR-0120; item 5 superseded by ADR-0120
  ```

  A slug override renders as `` slug `<slug>` superseded by ADR-NNNN ``. Proof marker
  `// invariant: active-md-supersedence-rendering` on a `RenderActiveMD` test in
  `internal/adr/adr_test.go` asserting a full chain, an item annotation, a slug annotation, and
  both subsections absent for a supersession-free corpus.

- [ ] **Task 6.3: `awf context` annotations.** In `internal/project/context.go`, at the
  per-ADR entry rendering site (around line 263, where Tier 1 and Tier 2 ADR lines are built),
  append, when `overrides[a.Number]` is non-empty:
  `" [item 2, item 5 superseded by ADR-0120]"` (items then slugs; each anchor qualified by its
  successor once; successors grouped where equal, as in the example). Tier 3 collapsed lines
  are counts, not entries, and are NOT annotated; the tier-semantics tests backing
  `context-tier1-marker-union`, `context-tier2-precise-tag`, and `context-tier3-collapsed`
  must stay green unmodified. Compute the index once per context run from the already-parsed
  corpus. Proof marker
  `// invariant: context-annotates-superseded-anchors` on an `internal/project/context_test.go`
  case: a fixture where a surfaced ADR has an overridden item shows the annotation; an ADR
  without overrides renders unchanged.

- [ ] **Task 6.4: Re-render, gate, commit.** `./x sync && ./x gate`; ACTIVE.md now carries the
  0031 chain and the 0116 annotations; stage regenerated files and locks. Assert sundial is
  untouched: `git status --short examples/sundial` prints nothing.

  ```commit
  feat(rendering): render supersession in ACTIVE.md and context
  ```

## Phase 7: Prose surfaces

One commit: every task below is one concern (ADR-0120 item 11's prose sweep) and none is
independently shippable.

- [ ] **Task 7.1: The lifecycle skill template.** In `templates/skills/adr-lifecycle/SKILL.md.tmpl`:

  - **`supersedence-full` section:** append two bullets: an ADR has at most one full successor
    (`superseded_by:` is scalar; `awf check` enforces the three-way symmetry), and full
    supersession excludes tokens from the same successor into the same target. The existing
    ACTIVE.md "Supersedence chains" bullet stands - Phase 6 made it true.
  - **`supersedence-partial` section:** replace the "Successor's prose explicitly cites the
    overridden items" bullet with the token rule: the successor's Decision section carries
    `` `supersedes: ADR-NNNN#<item>` `` or `` `supersedes-invariant: ADR-NNNN#<slug>` `` at the
    citation site, one token per overridden anchor; `awf check` validates the ref, requires the
    predecessor back-pointer, and refuses a token into a `Proposed` target. Append the
    authoring-time rule (ADR-0120 item 5): a new token targets the ADR that currently owns the
    anchor; a token whose target was later fully superseded degrades to an advisory note.
    Append the retirement rule (item 6): a `supersedes-invariant:` token on an Implemented ADR
    retires the slug; fully superseding a token-carrier lapses its retirements, and the new
    successor re-carries any it still intends.
  - **Notes and states table:** where the body is described as frozen, adopt the item-9
    carve-out wording: "the body's meaning is frozen; a schema retrofit may migrate its
    machine-readable encoding (ADR-0120)".

- [ ] **Task 7.2: The two-site AGENTS.md bullet, plus the append-only bullet.** Replace the
  retirement clause in **both** `templates/agents-doc/AGENTS.md.tmpl` (embedded default) and
  `.awf/agents-doc.yaml` line 28 (this repo's override - the ADR-0116 Decision 4 two-site
  hazard): `Retirement by an Implemented successor ADR drops a slug (ADR-0031).` becomes
  ``A `supersedes-invariant:` token on an Implemented successor ADR retires a slug (ADR-0120).``

  Also qualify the "Append-only ADRs" invariants bullet at
  `templates/agents-doc/AGENTS.md.tmpl` line 38 with the item-9 carve-out: append "Decision
  meaning is frozen once an ADR leaves Proposed; a meaning-preserving schema retrofit may
  migrate its encoding (ADR-0120)." Single site: the `.awf/agents-doc.yaml` override does not
  carry this bullet.

- [ ] **Task 7.3: The decisions README template.** In `templates/adr-readme/README.md.tmpl`: in
  the frontmatter block, document `supersedes:` as three-way-checked; add a
  `## Supersession and the Decision format` section spelling the two token grammars, the anchor
  rules (column-0 items sequential from 1 - the format `awf check` enforces), the back-pointer
  requirement, and the Proposed-target refusal.

- [ ] **Task 7.4: Catalog strings and reviewer lens.** In `internal/catalog/standard.go`: the
  three `adrStates` mutability strings gain the carve-out suffix ("; a schema retrofit may
  migrate the encoding, ADR-0120"); extend the adr-reviewer `docCurrencyItems` with a
  supersession-token currency item - first checking whether `.awf/agents/adr-reviewer.yaml`
  overrides `docCurrencyItems` wholesale, and mirroring the edit there if so (the two-site
  hazard again).

- [ ] **Task 7.5: Config-side prose (batch).** One rationale (retirement moved to tokens;
  supersession now checked), mechanical variation per file. Affected sites:
  `.awf/domains/parts/{adr-system,invariants,config,rendering}/current-state.md` (rewrite the
  supersedence/retirement narrative), `.awf/docs/glossary.yaml` (update "back-pointer"; add
  "supersession token"), `.awf/docs/pitfalls.yaml` (rewrite its two staled entries: the
  planning guidance around a `retires_invariants:` entry at ~line 450 and the prose-citation
  partial-supersedence convention at ~lines 251-277, both now token-mechanism; record the
  two-site AGENTS-bullet hazard if absent). Representative: the `adr-system` current-state
  paragraph replacing "cited in prose" with the token rule. Edge: `.awf/config.yaml`'s
  `invariant-retirement` tag meaning stays ("Invariant retirement via successor ADR" remains
  true under tokens). Post-check:

  ```bash
  ./x sync && ./x check && ! grep -rn "retires_invariants" templates/ internal/catalog/ .awf/parts/ .awf/agents-doc.yaml .awf/docs/
  ```

- [ ] **Task 7.6: Sync fan-out, gate, commit.** `./x sync` regenerates AGENTS.md, the lifecycle
  skill, the reviewer agent, README, template, domain docs, glossary, pitfalls, and sundial's
  copies; stage all.

  ```commit
  docs(adr-system): move supersession prose to the token rules
  ```

## Phase 8: The corpus retrofit and the flip

- [ ] **Task 8.1: Tokenize the freeform citations (batch).** Under the ADR-0120 item 9 carve-out
  (inserting a token adjacent to an existing prose citation). Affected-site set: every match of

  ```bash
  grep -rnE "[Ss]upersedes .*(Decision item|item [0-9]|Invariant)" docs/decisions/0*.md
  ```

  reviewed by hand against three exclusions: citations in a successor that *fully* supersedes
  the cited ADR (tokenizing trips flavour exclusivity - e.g. 0115's citations of 0113);
  citations that describe rather than override ("generalizes": ADR-0116 Decision 3's owed-when
  scoping); and ADR-0120 itself (already tokenized). Known sites from the grounding sweep: 0105
  (0008 item 4), 0119 (five citations into 0115 and 0118), 0081 (0046 item 4), 0108 (0097/0098
  items), 0020 (0005/0006 items). Representative diff (0105):

  ```diff
  -   local, and cannot drift from a separate list. It **supersedes ADR-0008 Decision item 4** via
  +   local, and cannot drift from a separate list. It **supersedes ADR-0008 Decision item 4**
  +   (`supersedes: ADR-0008#4`) via
  ```

  Tokenization inserts text and changes nothing else - no formatting, emphasis, or wording
  edits; the item-9 carve-out authorizes insertion adjacent to an existing citation, nothing
  broader.

  Edge: a citation of a *Superseded* target is still tokenized (it degrades to the expected
  advisory note, preserving history) unless the full-supersession exclusion applies. For every
  token written into a live target, add the back-pointer to the target's `related:` in the same
  commit.

  Post-check: `./x check` with zero `adr-token-*` drifts; the note lines are exactly the
  same-anchor and superseded-target advisories the sweep predicts (enumerate them in the commit
  body).

- [ ] **Task 8.2: Backfill the ADR-0116 Decision 6 edges.** Add the successor's number to the
  predecessor's `related:` for the ten deferred edges (8<-105, 104<-106, 7<-8, 3<-30, 23<-32,
  1<-45, 30<-49, 39<-76, 16<-76, 11<-89), skipping any Phase 4 or Task 8.1 already wrote.
  Metadata-only edits, legal on live ADRs per ADR-0116 Decision 2.

- [ ] **Task 8.3: Changelog.** Add the `changelog/CHANGELOG.md` entry under Unreleased: the two
  token grammars, the new checks (Decision format, symmetry, refs, back-pointers, exclusivity,
  the raw-key refusal), the generation-10 migration ("run `awf upgrade`; the migration rewrites
  `docs/decisions/`"), the `retires_invariants:` removal, and the rendering additions.

- [ ] **Task 8.4: Flip and close.** Flip ADR-0120 to `status: Implemented` and this plan to
  `status: Implemented`. The flip makes 0120's twelve backed slugs owed - confirm every proof
  marker landed: `decision-items-enumerable` (Task 1.3), `supersession-full-symmetry`,
  `supersession-token-ref-validity`, `supersession-backpointer`,
  `supersession-flavour-exclusive`, `supersession-conflict-advisory` (Task 2.3),
  `token-retirement-implemented-only`, `token-retirement-dangling-errors` (Task 3.2),
  `upgrade-migrates-retirements` (Task 4.2), `retires-invariants-key-refused` (Task 5.2),
  `active-md-supersedence-rendering` (Task 6.2), `context-annotates-superseded-anchors`
  (Task 6.3). The flip also makes 0120's two item tokens live; 0116's back-pointer already
  exists. `./x sync && ./x gate`; stage ACTIVE.md (0120 changes groups; the Supersedence
  section already lists the 0031 chain).

  ```commit
  docs(adr): retrofit corpus tokens; implement 0120
  ```

## Verification

- `./x gate full` green at HEAD.
- `go run ./cmd/awf upgrade` prints nothing (idempotence at generation 10); same in
  `examples/sundial`.
- `./x check` prints `awf check: clean` with only the advisory notes Task 8.1 enumerated - zero
  dangling-marker notes.
- `go run ./cmd/awf context internal/adr/adr.go` (any query surfacing ADR-0116) shows its
  `[item 2, item 5 superseded by ADR-0120]` annotation.
- `grep -c "^retires_invariants:" docs/decisions/0*.md | grep -v ":0"` prints nothing: only the
  column-0 key is gone; prose mentions inside frozen bodies and the migration-appended
  bookkeeping items survive, which is correct.

## Notes

- The migration edits raw bytes, never re-serializes frontmatter: key order and formatting of
  untouched lines survive byte-identical, so meaning-preservation is checkable by diff.
- Two-site hazards while editing prose: the AGENTS.md invariants bullet
  (template + `.awf/agents-doc.yaml`) and the adr-reviewer `docCurrencyItems`
  (catalog + `.awf/agents/adr-reviewer.yaml`), per ADR-0116 Decision 4.
- Expected transient state: the three `inv-retirement-*` dangling-marker notes live from Phase 1
  through Phase 5; ADR-0120's twelve proof markers are dangling notes until the Task 8.4 flip.
- Out of scope, for the backlog: an `awf audit` rule flagging body edits to non-Proposed ADRs
  (none exists today; the item-9 carve-out makes reviewer discipline the only guard), and
  tokenizing adopter corpora beyond sundial.
