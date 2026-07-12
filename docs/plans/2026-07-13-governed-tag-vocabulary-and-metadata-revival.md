---
date: 2026-07-13
adrs: [103]
status: Proposed
---
# Plan: Governed tag vocabulary and metadata revival

Implements ADR-0103 — the metadata/vocabulary/governance layer of the `awf context` relevance
rework (SLICE 2 of 3). Design and rationale live in
[ADR-0103](../decisions/0103-governed-tag-vocabulary-and-metadata-revival.md); this plan is the
execution record only.

## Goal

Revive the parsed-then-dropped ADR `tags:`/`related:` frontmatter and add a pitfall `tags:` field;
introduce a governed top-level `tags:` config vocabulary (tag → one-line meaning); add two `awf
check` rules (tag-vocabulary membership + ADR `related:` link resolution); then curate a tight
vocabulary and normalize awf's own ADR and pitfall corpus to it. This slice deliberately changes
**nothing** about `awf context` output — the relevance-tiering consumer is the follow-up slice.

## Architecture summary

Six phases, each closing with a gate-passing commit:

1. Lift `tags:`/`related:` into `adr.ADR` (`internal/adr`).
2. Parse a `tags:` field on pitfall entries (`internal/project/pitfalls.go`).
3. Add the top-level `tags:` config key + its `configspec` entry (regenerates `config-reference.md`).
4. Add the two `awf check` governance rules with `inv:` markers (`internal/project/check.go`).
5. Author the curated vocabulary into `.awf/config.yaml` and normalize the whole corpus in one
   commit — the now-active governance check *is* the exhaustive post-check.
6. Doc currency (agent-guide invariants, domain current-state, changelog) + flip ADR-0103 and this
   plan to `Implemented` + regenerate `ACTIVE.md`.

Phases 1–4 keep awf's own `tags:` vocabulary empty, so the membership rule stays inert and the gate
stays green; the ADR `related:` rule (unconditional) lands green because the current corpus already
resolves. Phase 5 turns governance on by populating the vocabulary, in the same commit that makes
the corpus conform.

## Tech stack

Go 1.26. Files touched: `internal/adr/adr.go`, `internal/testsupport/testsupport.go`,
`internal/project/pitfalls.go`, `internal/config/config.go`, `internal/configspec/spec.go`,
`internal/project/check.go`, `.awf/config.yaml`, `.awf/agents-doc.yaml`, `.awf/docs/pitfalls.yaml`,
`.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/adr-system/current-state.md`,
`changelog/`, all `docs/decisions/[0-9]*.md`. Gate: `./x gate` before every commit.

## File structure

- **Created:** none.
- **Modified:** `internal/adr/adr.go`, `internal/adr/adr_test.go`,
  `internal/testsupport/testsupport.go`, `internal/testsupport/testsupport_test.go`,
  `internal/project/pitfalls.go`, `internal/project/pitfalls_test.go`,
  `internal/config/config.go`, `internal/configspec/spec.go`, `internal/project/check.go`,
  `internal/project/check_test.go`, `.awf/config.yaml`, `docs/config-reference.md` (regenerated),
  `.awf/agents-doc.yaml`, `AGENTS.md` (regenerated), `.awf/docs/pitfalls.yaml`,
  `.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/adr-system/current-state.md`,
  `docs/domains/*.md` (regenerated), the changelog `[Unreleased]` section, all
  `docs/decisions/[0-9]*.md` (tag normalization), `docs/decisions/ACTIVE.md` (regenerated),
  `docs/decisions/0103-*.md` (status flip), this plan (status flip), `.awf/awf.lock` (regenerated).
- **Deleted:** none.

## Phase 1 — Lift ADR `tags:`/`related:` into `adr.ADR`

- [ ] **Task 1.1 — Add `Tags`/`Related` to the frontmatter and record structs.** In
  `internal/adr/adr.go`, extend `adrFrontmatter` (currently lifting only
  status/domains/superseded_by/retires_invariants) and the exported `ADR` struct, and populate them
  in `parse`:

  ```
  // in the ADR struct, after Domains:
  	Tags              []string          // `tags:` frontmatter (keyword labels)
  	Related           []int             // `related:` frontmatter (ADR numbers)

  // in adrFrontmatter:
  	Tags              []string `yaml:"tags"`
  	Related           []int    `yaml:"related"`

  // in parse(), extend the ADR literal:
  	a := ADR{Status: fm.Status, Domains: fm.Domains, Tags: fm.Tags, Related: fm.Related, SupersededBy: fm.SupersededBy, RetiresInvariants: fm.RetiresInvariants, Sections: sections(string(body))}
  ```

- [ ] **Task 1.2 — Add a `WithRelated` fixture option.** In `internal/testsupport/testsupport.go`,
  add a `related []int` field to `adrOpts`, a `WithRelated` option, and emit it in `ADR()` between
  the `tags:` and `domains:` lines (matching frontmatter order):

  ```
  // option (near WithTags):
  // WithRelated sets the frontmatter related array (ADR numbers).
  func WithRelated(nums ...int) ADROption { return func(o *adrOpts) { o.related = nums } }

  // in ADR(), after the tags block, before the domains block:
  	if o.related != nil {
  		parts := make([]string, len(o.related))
  		for i, n := range o.related {
  			parts[i] = strconv.Itoa(n)
  		}
  		b.WriteString("related: [" + strings.Join(parts, ", ") + "]\n")
  	}
  ```

  Add `"strconv"` to the imports if not present. Update the `adrOpts` struct definition to include
  `related []int`. In `internal/testsupport/testsupport_test.go`, extend the existing full-fixture
  assertion (the `want :=` string around line 102) to include a `related:` line by adding
  `testsupport.WithRelated(1, 5)` to that test's option list and inserting `related: [1, 5]\n` into
  the expected string at the correct position (after the `tags:` line).

- [ ] **Task 1.3 — Assert the lift in `adr_test.go`.** Add a test to `internal/adr/adr_test.go`:

  ```
  // TestParseDirExtractsTagsAndRelated confirms the revived tags:/related:
  // frontmatter is lifted into adr.ADR (previously parsed past and dropped).
  func TestParseDirExtractsTagsAndRelated(t *testing.T) {
  	dir := t.TempDir()
  	content := testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
  		testsupport.WithTags("context", "config"), testsupport.WithRelated(1, 92),
  		testsupport.WithTitle("0007: Tagged"), testsupport.WithBody("## Context\nx\n"))
  	if err := os.WriteFile(filepath.Join(dir, "0007-tagged.md"), []byte(content), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	adrs, err := adr.ParseDir(dir)
  	if err != nil {
  		t.Fatalf("ParseDir: %v", err)
  	}
  	if len(adrs) != 1 {
  		t.Fatalf("expected 1 ADR, got %d", len(adrs))
  	}
  	got := adrs[0]
  	if len(got.Tags) != 2 || got.Tags[0] != "context" || got.Tags[1] != "config" {
  		t.Errorf("tags: got %#v", got.Tags)
  	}
  	if len(got.Related) != 2 || got.Related[0] != 1 || got.Related[1] != 92 {
  		t.Errorf("related: got %#v", got.Related)
  	}
  }
  ```

- [ ] **Task 1.4 — Verify and commit.** `./x gate`. Then:
  `git add internal/adr/adr.go internal/adr/adr_test.go internal/testsupport/testsupport.go internal/testsupport/testsupport_test.go`
  and commit: `feat(adr-system): lift tags and related frontmatter into adr.ADR`.

## Phase 2 — Parse a `tags:` field on pitfall entries

- [ ] **Task 2.1 — Add `Tags` to `pitfallEntry`.** In `internal/project/pitfalls.go`, extend the
  `pitfallEntry` struct and parse the field in `pitfallEntryFrom` via the existing `pitfallStrings`
  helper (same shape discipline as `domains`). This is validate-and-consume metadata; the render
  transform (`pitfallsMarkdown`) is **unchanged** — tags are not rendered prose in this slice.

  ```
  // in pitfallEntry, after Related:
  	Tags    []string

  // in pitfallEntryFrom, after the domains block, before the related block:
  	tags, err := pitfallStrings(i, title, m, "tags")
  	if err != nil {
  		return pitfallEntry{}, err
  	}
  // and add Tags: tags to the returned pitfallEntry literal.
  ```

  Update the `pitfallEntry` doc comment to mention the optional `tags:`.

- [ ] **Task 2.2 — Assert the pitfall tags parse.** In `internal/project/pitfalls_test.go`, add a
  case asserting a `tags:` list parses into `pitfallEntry.Tags` and that a non-list `tags:` value is
  a hard error (mirror the existing `domains`/`related` shape tests in that file — locate the
  existing `pitfallEntries` happy-path and error-path tests and add parallel `tags` coverage).

- [ ] **Task 2.3 — Verify and commit.** `./x gate`. Then
  `git add internal/project/pitfalls.go internal/project/pitfalls_test.go` and commit:
  `feat(config): parse a tags field on pitfall entries`.

## Phase 3 — Add the governed `tags:` config vocabulary key

- [ ] **Task 3.1 — Add the `Tags` field to `config.Config`.** In `internal/config/config.go`, add to
  the `Config` struct (after `Domains`, before `Targets` to keep it near the other freeform keys):

  ```
  	Tags       map[string]string `yaml:"tags"`
  ```

  Update the struct's doc comment block only if it enumerates keys (it does not — no change needed
  beyond the field).

- [ ] **Task 3.2 — Add the `configspec` entry.** In `internal/configspec/spec.go`, add an `Entry` to
  the `keys` slice immediately after the `targets` entry (the reflection parity test,
  `configspec-key-parity`, requires exactly one entry for the new leaf; a `map[string]string` is a
  freeform-namespace leaf like `vars`):

  ```
  	{
  		Path: "tags", Type: "key → value map", Default: "none",
  		Description:  "A governed vocabulary of cross-cutting keyword tags, each mapping a tag name to a one-line meaning. ADR `tags:` and pitfall `tags:` are validated against it: with a non-empty vocabulary, a used tag that is not a declared member is failing drift, as is a member with an empty meaning. An empty or absent vocabulary disables the check (tags are then free-form). Declaring a member no artifact uses is allowed.",
  		Availability: "Always; the membership check is inert until the vocabulary is non-empty.",
  	},
  ```

  Keep the description publication-safe: no ADR citations, no repo-identity literals (the residue
  rules are test-enforced).

- [ ] **Task 3.3 — Regenerate the config reference.** Run `./x sync` — it regenerates
  `docs/config-reference.md` from the `configspec` table (the `config-reference-regen-drift`
  invariant). Confirm the new `tags` row appears.

- [ ] **Task 3.4 — Verify and commit.** `./x gate` (the `configspec-key-parity` test now passes with
  the new field+entry; `./x check` shows no drift). Then
  `git add internal/config/config.go internal/configspec/spec.go docs/config-reference.md .awf/awf.lock`
  and commit: `feat(config): add the governed tags vocabulary key`.

## Phase 4 — Add the two `awf check` governance rules

- [ ] **Task 4.1 — Add `checkTagVocabulary` and `checkADRRelatedLinks`.** In
  `internal/project/check.go`, add two methods (place them after `checkPitfalls`). Import `sort` if
  not already imported (it is used elsewhere in the package; confirm the file's import block). Use a
  repo-relative decisions path mirroring `checkPlans`' `rel` construction, and
  `config.DirName + "/config.yaml"` for the vocabulary path.

  ```
  // checkTagVocabulary validates tag governance when the config tags: vocabulary
  // is non-empty: every tag used by an ADR (frontmatter tags:) or a pitfall
  // (tags:) must be a declared vocabulary member, and every member must declare a
  // non-empty meaning. An empty or absent vocabulary is inert (tags are then
  // free-form). A declared member no artifact uses is intentionally permitted,
  // mirroring an unused configured domain under pitfall-domains-resolved.
  // invariant: tag-vocabulary-governed
  func (p *Project) checkTagVocabulary() ([]manifest.Drift, error) {
  	if len(p.Cfg.Tags) == 0 {
  		return nil, nil
  	}
  	cfgPath := config.DirName + "/config.yaml"
  	var drift []manifest.Drift
  	for _, tag := range slices.Sorted(maps.Keys(p.Cfg.Tags)) {
  		if strings.TrimSpace(p.Cfg.Tags[tag]) == "" {
  			drift = append(drift, manifest.Drift{Path: cfgPath, Kind: "tag-vocabulary", Detail: fmt.Sprintf("tag %q has an empty meaning", tag)})
  		}
  	}
  	adrs, err := adr.ParseDir(p.decisionsDir())
  	if err != nil { // coverage-ignore: an earlier Check() step (checkPlans) already ParseDir'd the decisions dir, so a parse error is pre-empted
  		return nil, err
  	}
  	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
  	for _, a := range adrs {
  		for _, tag := range a.Tags {
  			if _, ok := p.Cfg.Tags[tag]; !ok {
  				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-tag", Detail: fmt.Sprintf("ADR-%s: unknown tag %q", a.Number, tag)})
  			}
  		}
  	}
  	pf, err := p.pitfallTagEntries()
  	if err != nil { // coverage-ignore: the pitfalls sidecar was validated at Open, so this re-read cannot fail
  		return nil, err
  	}
  	for _, e := range pf {
  		for _, tag := range e.Tags {
  			if _, ok := p.Cfg.Tags[tag]; !ok {
  				drift = append(drift, manifest.Drift{Path: pitfallsSidecarPath, Kind: "pitfall-tag", Detail: fmt.Sprintf("%q: unknown tag %q", e.Title, tag)})
  			}
  		}
  	}
  	return drift, nil
  }

  // pitfallTagEntries returns the pitfall entries when the pitfalls doc is
  // enabled, else nil — factored so checkTagVocabulary reads tags without
  // duplicating checkPitfalls' sidecar plumbing.
  func (p *Project) pitfallTagEntries() ([]pitfallEntry, error) {
  	if !slices.Contains(p.Cfg.Docs, "pitfalls") {
  		return nil, nil
  	}
  	sc, err := p.Cfg.Sidecar("docs", "pitfalls")
  	if err != nil { // coverage-ignore: the pitfalls sidecar was validated at Open
  		return nil, err
  	}
  	return pitfallEntries(sc.Data["pitfalls"])
  }

  // checkADRRelatedLinks fails an ADR whose related: names an ADR number with no
  // matching file under the decisions dir — structurally identical to the
  // pitfall/plan link checks. Unconditional (independent of the tag vocabulary).
  // invariant: adr-related-link-resolved
  func (p *Project) checkADRRelatedLinks() ([]manifest.Drift, error) {
  	adrs, err := adr.ParseDir(p.decisionsDir())
  	if err != nil { // coverage-ignore: checkPlans already ParseDir'd the decisions dir
  		return nil, err
  	}
  	known := map[string]bool{}
  	for _, a := range adrs {
  		known[a.Number] = true
  	}
  	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
  	var drift []manifest.Drift
  	for _, a := range adrs {
  		for _, n := range a.Related {
  			if !known[fmt.Sprintf("%04d", n)] {
  				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-related-link", Detail: fmt.Sprintf("ADR-%s: ADR-%04d", a.Number, n)})
  			}
  		}
  	}
  	return drift, nil
  }
  ```

  Confirm `maps` and `slices` are imported in `check.go` (add if missing).

- [ ] **Task 4.2 — Wire both into `Check()`.** In `internal/project/check.go`, after the
  `checkPitfalls` block (currently ending at `drift = append(drift, pitfallDrift...)`), add:

  ```
  	tagDrift, err := p.checkTagVocabulary()
  	if err != nil { // coverage-ignore: checkTagVocabulary's reads are pre-empted by earlier Check() steps
  		return nil, err
  	}
  	drift = append(drift, tagDrift...)
  	relDrift, err := p.checkADRRelatedLinks()
  	if err != nil { // coverage-ignore: adr.ParseDir here is pre-empted by checkPlans
  		return nil, err
  	}
  	drift = append(drift, relDrift...)
  ```

- [ ] **Task 4.3 — Test both rules.** Add to `internal/project/check_test.go`:

  ```
  // A non-member tag on an ADR or a pitfall yields tag drift; an empty-meaning
  // member yields tag-vocabulary drift; a fully-conforming corpus yields none.
  // invariant: tag-vocabulary-governed
  func TestCheckTagVocabulary(t *testing.T) {
  	cfg := "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: [rendering]\n" +
  		"tags:\n  rendering: the render engine\n  empty: \"\"\n"
  	root := scaffoldFiles(t, cfg, map[string]string{
  		"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: P\n      tags: [rendering, ghost]\n      body: ok\n",
  	})
  	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
  		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
  			testsupport.WithTags("rendering", "bogus"), testsupport.WithTitle("0001: A"),
  			testsupport.WithBody("## Context\nx\n")))
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	drift, err := p.checkTagVocabulary()
  	if err != nil {
  		t.Fatalf("checkTagVocabulary: %v", err)
  	}
  	got := map[string]string{}
  	for _, d := range drift {
  		got[d.Kind] = d.Detail
  	}
  	if len(drift) != 3 || !strings.Contains(got["adr-tag"], "bogus") ||
  		!strings.Contains(got["pitfall-tag"], "ghost") || !strings.Contains(got["tag-vocabulary"], "empty") {
  		t.Fatalf("want adr-tag(bogus)+pitfall-tag(ghost)+tag-vocabulary(empty), got %#v", drift)
  	}
  }

  // An empty/absent vocabulary makes the membership rule inert.
  func TestCheckTagVocabularyInert(t *testing.T) {
  	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
  	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
  		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
  			testsupport.WithTags("anything"), testsupport.WithTitle("0001: A"),
  			testsupport.WithBody("## Context\nx\n")))
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	drift, err := p.checkTagVocabulary()
  	if err != nil || drift != nil {
  		t.Fatalf("empty vocabulary must be inert, got %#v / %v", drift, err)
  	}
  }

  // A dangling ADR related: number yields adr-related-link drift; a resolving one
  // yields none. Unconditional (no vocabulary configured here).
  // invariant: adr-related-link-resolved
  func TestCheckADRRelatedLinks(t *testing.T) {
  	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
  	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
  		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
  			testsupport.WithRelated(1, 42), testsupport.WithTitle("0001: A"),
  			testsupport.WithBody("## Context\nx\n")))
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	drift, err := p.checkADRRelatedLinks()
  	if err != nil {
  		t.Fatalf("checkADRRelatedLinks: %v", err)
  	}
  	if len(drift) != 1 || drift[0].Kind != "adr-related-link" || !strings.Contains(drift[0].Detail, "0042") {
  		t.Fatalf("want one adr-related-link(0042) drift, got %#v", drift)
  	}
  }
  ```

  If, after `./x gate`, the two `err != nil` wiring branches in `Check()` or the ParseDir/sidecar
  branches in the new methods are reported uncovered (they are marked `coverage-ignore` above because
  earlier `Check()` steps pre-empt them), keep the `coverage-ignore` comments; if coverage flags any
  as genuinely reachable, add a direct error-injection test mirroring
  `TestCheckPitfallsADRParseError` rather than an unjustified ignore.

- [ ] **Task 4.4 — Verify and commit.** `./x gate` (awf's own `tags:` vocabulary is still absent, so
  `checkTagVocabulary` is inert on this tree; every ADR `related:` already resolves, so
  `checkADRRelatedLinks` is clean). Then
  `git add internal/project/check.go internal/project/check_test.go` and commit:
  `feat(tooling): govern ADR/pitfall tags and resolve ADR related links in awf check`.

## Phase 5 — Adopt the vocabulary and normalize the corpus

This phase turns governance on. The vocabulary and the corpus normalization land in **one commit**
because they are inseparable: adding a non-empty `tags:` vocabulary activates `checkTagVocabulary`,
so the corpus must already conform in the same gate. The active check is the exhaustive post-check.

- [ ] **Task 5.1 — Author the curated vocabulary into `.awf/config.yaml`.** Insert this top-level
  block (alphabetically, immediately before the `targets:` key):

  ```
  tags:
    adoption: Adopter-facing behaviour, portability, and the example adopter
    adr-system: The ADR machinery — parsing, ACTIVE.md generation, and lifecycle
    advisory: Non-failing advisory notes and warnings that never change exit status
    agents: The independent review-agent artifacts
    audit: The workflow-conformance audit over a branch's commits
    bootstrap: The bootstrap and upgrade installer porcelain
    catalog: The compile-time catalog of skills, agents, and docs
    changelog: The changelog and release-notes pipeline
    config: The .awf config tree, its schema, and parsing
    context: The awf context read-only query command
    dispatch: Subagent dispatch and the review-agent seam
    docs: Managed documentation and the documentation standard
    domains: Domain keys, their generated docs, and file territory
    drift: Drift detection between committed config and rendered output
    feedback-loop: The retrospective loop promoting findings toward checks
    governance: Checks that a used label or link resolves to a declared authority
    hooks: The git-hook payloads and their wiring
    invariants: Invariant declaration, backing markers, and enforcement
    memory: Ephemeral working-memory session state
    migration: Schema-generation migrations of the config tree
    parts: Convention-part overrides and section assembly
    pitfalls: The pitfalls knowledge surface
    plans: The plan artifact and its conventions
    publication-safety: Publication-safe degradation of unset template values
    refactor: Structural refactors and coupling changes
    release: Releasing — versioning, GoReleaser, and distribution
    rendering: The template render engine, overlay, and template sources
    scaffold: awf new / init scaffolding of new artifacts
    security: Checksums, atomic writes, and supply-chain integrity
    sidecar-derived-doc: Docs computed from sidecar data via a transform
    skills: The workflow skill artifacts
    staleness: Code and doc staleness detection
    testing: Test suites, gate tiers, coverage, and mutation triage
    tooling: The awf CLI, command dispatch, and the ./x runner
    workflow: The brainstorm→ADR→plan→implement→review chain
  ```

- [ ] **Task 5.2 — Normalize every ADR's `tags:` (batch).** Apply the synonym map below to every
  `docs/decisions/[0-9]*.md` `tags:` line: replace each tag with its canonical member, then
  de-duplicate within the entry preserving first-occurrence order. A tag already equal to a canonical
  member is unchanged. Every resulting tag must be one of the 35 vocabulary members.

  **Synonym map** (non-canonical → canonical; every tag in the current corpus is covered):

  | → rendering | → config | → tooling | → testing | → adoption | → docs | → release | → parts | → workflow | → adr-system | → catalog | → context | → security | → publication-safety | → scaffold | → dispatch | → audit | → bootstrap | → governance |
  |---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
  | render, templates, sections, layout, embed, stub, placeholders | schema, vars, yaml, serialization, lock, manifest, globs, frontmatter | cli, gate, check, ci, hygiene, dead-code, error-handling, sync, uninstall, dependency-graph | e2e, coverage, mutation-testing, verification, evals | adopter-extensibility, portability, onboarding, examples, dogfood, repo-local, adapter, cursor, extension, standard, quality | documentation, agent-guide, discoverability | versioning, goreleaser, distribution | overrides, partials, convention-parts | chain, convention, conventions, commit | adr, lifecycle | singleton | query | atomicity, supply-chain | safety, safety-net | init, setup | review, reviewing-impl | conformance | upgrade | validation |

  Canonical members that also appear verbatim in the corpus (unchanged): `tooling`, `rendering`,
  `config`, `docs`, `workflow`, `adoption`, `catalog`, `audit`, `testing`, `release`, `context`,
  `advisory`, `refactor`, `invariants`, `changelog`, `adr-system`, `skills`, `publication-safety`,
  `plans`, `parts`, `hooks`, `bootstrap`, `staleness`, `migration`, `memory`, `feedback-loop`,
  `domains`, `dispatch`, `agents`, `security`, `scaffold`, `sidecar-derived-doc`, `drift`,
  `pitfalls`, `governance`.

  - **Representative** — `docs/decisions/0092-read-only-context-query-command.md`:
    `tags: [cli, workflow, domains, invariants, query]` → `tags: [tooling, workflow, domains, invariants, context]`.
  - **Edge (collapse+dedupe)** — any ADR whose mapped tags repeat, e.g. an entry
    `tags: [render, templates, rendering, cli, gate]` → map to
    `[rendering, rendering, rendering, tooling, tooling]` → dedupe →
    `tags: [rendering, tooling]`. Apply the same collapse wherever mapping produces duplicates.
  - **Affected set:** every file matched by `ls docs/decisions/[0-9]*.md` (ADR-0103's own
    `tags: [context, config, adr-system, invariants, tooling]` are all canonical → unchanged).
  - **Post-check:** after Task 5.1's vocabulary is in place, this command must print nothing (every
    used ADR tag is a vocabulary member):

    ```
    comm -23 \
      <(grep -h '^tags:' docs/decisions/[0-9]*.md | sed 's/^tags: \[//; s/\]$//; s/, /\n/g' | sed 's/^ *//;s/ *$//' | grep -v '^$' | sort -u) \
      <(sed -n 's/^  \([a-z-]*\):.*/\1/p' .awf/config.yaml | sort -u)
    ```

- [ ] **Task 5.3 — Tag awf's pitfall corpus (batch).** Add a `tags:` list to every entry in
  `.awf/docs/pitfalls.yaml`, drawn from the vocabulary and reflecting the entry's topic (seed from
  the entry's existing `domains:`, mapped through the synonym map, plus topical members from its
  title/body). Every entry gets at least one tag.

  - **Representative** — an entry with `domains: [rendering]` about template section assembly gains
    `tags: [rendering, parts]`.
  - **Edge** — a cross-cutting process entry with no `domains:` gains topical tags only, e.g.
    `tags: [workflow, feedback-loop]`.
  - **Affected set:** every entry under `data.pitfalls` in `.awf/docs/pitfalls.yaml`.
  - **Post-check:** every entry has a non-empty `tags:` (this command prints nothing if the entry
    count equals the tags-line count):

    ```
    test "$(grep -c '^    - title:' .awf/docs/pitfalls.yaml)" = "$(grep -c '^      tags:' .awf/docs/pitfalls.yaml)" || echo MISMATCH
    ```

    Tags do not render into `docs/pitfalls.md` (the transform is unchanged), so `./x sync` leaves the
    rendered doc byte-identical.

- [ ] **Task 5.4 — Regenerate and verify.** Run `./x sync` (regenerates `docs/config-reference.md`
  to reflect the now-populated vocabulary in its live-state section, and re-hashes `.awf/awf.lock`).
  Then `./x check` — the now-active `checkTagVocabulary` validates that **every** ADR and pitfall tag
  is a vocabulary member and every member has a meaning; a clean result is the exhaustive proof the
  corpus conforms. Run both Task 5.2 and 5.3 post-checks; both must be silent.

- [ ] **Task 5.5 — Commit.** `./x gate`. Then stage the vocabulary, the normalized corpus, and the
  regenerated files explicitly:
  `git add .awf/config.yaml docs/decisions/[0-9]*.md .awf/docs/pitfalls.yaml docs/config-reference.md .awf/awf.lock`
  and commit: `feat(config): adopt the governed tag vocabulary and normalize the corpus`. The commit
  body notes the ADR-tag normalization and pitfall tagging are the direct, gate-verified consequence
  of populating the vocabulary.

## Phase 6 — Doc currency and status flip

- [ ] **Task 6.1 — Add the two invariant bullets to the agent guide.** In `.awf/agents-doc.yaml`,
  add two bullets to the invariants list (locate the existing invariants `data` list; append after
  the ADR-0102 `uncovered-*` bullets), phrased to match the ADR's invariant statements:

  ```
  - **Governed tag vocabulary.** With a non-empty `tags:` vocabulary, `awf check` fails on any tag used by an ADR or a pitfall that is not a declared vocabulary member, and on any member with an empty meaning; an empty or absent vocabulary is inert. A declared member no artifact uses is allowed. (`inv: tag-vocabulary-governed`, ADR-0103)
  - **ADR related links resolve.** `awf check` fails an ADR whose `related:` names an ADR number with no matching file under `docs/decisions/`, independent of the tag vocabulary. (`inv: adr-related-link-resolved`, ADR-0103)
  ```

  Match the exact bullet style of the surrounding entries (the ADR-0102 bullets are the nearest
  model). Run `./x sync` to regenerate `AGENTS.md`.

- [ ] **Task 6.2 — Update the domain current-state parts.** Add a sentence to each of
  `.awf/domains/parts/config/current-state.md` (the `internal/config` `Tags` key + the vocabulary
  governance surface) and `.awf/domains/parts/adr-system/current-state.md` (the revived
  `tags:`/`related:` lift in `internal/adr` and the ADR `related:` link check), reflecting the new
  state. Run `./x sync` to regenerate the `docs/domains/*.md` indices.

- [ ] **Task 6.3 — Add the changelog entry.** In the changelog `[Unreleased]` section (under
  `### Added`), add an entry naming the governed tag vocabulary, the revived ADR `tags:`/`related:`
  and pitfall `tags:` metadata, and the two new `awf check` rules. No adopter-migration recipe is
  needed (the config key is additive and absent-safe; no schema migration). Follow the changelog's
  existing entry format.

- [ ] **Task 6.4 — Flip statuses and regenerate `ACTIVE.md`.** Set
  `docs/decisions/0103-governed-tag-vocabulary-and-metadata-revival.md` frontmatter `status:` to
  `Implemented`, and this plan's frontmatter `status:` to `Implemented`. Run `./x sync` to regenerate
  `docs/decisions/ACTIVE.md`.

- [ ] **Task 6.5 — Verify and commit.** `./x gate` and `./x check` (with the ADR now `Implemented`,
  the `inv: tag-vocabulary-governed` and `inv: adr-related-link-resolved` slugs are enforced-backed
  by their markers from Phase 4). Then stage the doc-currency files, the flipped ADR and plan, and
  the regenerated `AGENTS.md`/`docs/domains/*.md`/`ACTIVE.md`/`.awf/awf.lock`, and commit:
  `docs(adr): implement 0103 — govern tags, flip status`.

## Verification

- `./x gate` and `./x check` pass at every phase boundary and after Phase 6.
- After Phase 5, `awf check` is clean with the vocabulary active — proof every ADR and pitfall tag is
  a governed member and every member has a meaning.
- The two Phase-5 post-check commands are silent (no unmapped ADR tag; every pitfall entry tagged).
- `docs/pitfalls.md` is byte-identical before and after (tags are not rendered this slice).
- `awf context` output is unchanged from before this slice (no consumer added) — spot-check with a
  representative query.
- ADR-0103 and this plan are both `status: Implemented`; `ACTIVE.md` lists 0103 under Implemented.

## Notes

- **Deliberately out of scope (the follow-up tiering slice):** spending tags in `awf context`
  (path→invariant→declaring-ADR precise tag set, tiered surfacing, the slug→declaring-ADR tag union
  with Superseded/retired filtering), reviving `related:` as a Tier-2 signal, retiring
  `context-surfaces-pitfalls` and reconciling `context-surfaces-linked-plans`, output compaction, and
  the domain-coverage config for unowned packages. This slice changes nothing about `awf context`
  output.
- The vocabulary is authored quality-first, not as a mechanical union of every label in use; the
  synonym map in Task 5.2 records the merges applied to the existing corpus.
