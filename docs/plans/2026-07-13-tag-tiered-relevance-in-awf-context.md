---
date: 2026-07-13
adrs: [104]
status: Implemented
---
# Plan: Tag-tiered relevance in awf context

Implements [ADR-0104](../decisions/0104-tag-tiered-relevance-in-awf-context.md): SLICE 3 (the
payoff) of the `awf context` relevance rework. Design and rationale live in the ADR; this plan is
the execution record.

## Goal

Replace `awf context <paths>`' coarse domain-membership ADR/pitfall surfacing with the three-tier
relevance model: Tier 1 (ADRs whose invariants are backed under the queried paths), Tier 2 (ADRs and
pitfalls sharing a *finer-than-domain* precise tag, or `related:`-linked), and Tier 3 (the coarse
domain ADR set collapsed to a count). Retire `context-surfaces-pitfalls` (0099) and
`context-surfaces-linked-plans` (0098); back five tiered successors; preserve output-parity /
read-only / static-fallback.

## Architecture summary

Two phases. Phase 1 exposes the one-to-one `slug → declaring Implemented ADR` join (already built
privately inside `invariants.Check`) as a shared function. Phase 2 is a **single coupled commit**:
the `ContextResult` tier restructure, the `ContextFor` tier assembly, the `printContext`/JSON
rendering, the five new `inv:` markers, the removal of the two retired markers, the ADR/plan status
flip, and the doc currency, coupled because removing the two retired markers unbacks 0098/0099
until ADR-0104 is `Implemented` (the retirement-couples-to-flip rule), and the `ContextResult`
struct rewrite couples assembly to rendering.

## Tech stack

Go 1.26. Files: `internal/invariants/invariants.go` (+`_test`), `internal/project/context.go`
(+`_test`), `cmd/awf/context.go` (+`_test`), `.awf/agents-doc.yaml`,
`.awf/domains/parts/{tooling,invariants}/current-state.md`, `changelog/CHANGELOG.md`,
`docs/decisions/0104-*.md`, this plan. Gate: `./x gate` before every commit.

## File structure

- **Created:** none.
- **Modified:** `internal/invariants/invariants.go`, `internal/invariants/invariants_test.go`,
  `internal/project/context.go`, `internal/project/context_test.go`, `cmd/awf/context.go`,
  `cmd/awf/context_test.go`, `.awf/agents-doc.yaml`, `AGENTS.md` (regenerated),
  `.awf/domains/parts/tooling/current-state.md`, `.awf/domains/parts/invariants/current-state.md`,
  `docs/domains/{tooling,invariants}.md` (regenerated), `changelog/CHANGELOG.md`,
  `docs/decisions/0104-*.md` (flip), `docs/decisions/ACTIVE.md` (regenerated), this plan (flip),
  `.awf/awf.lock`.
- **Deleted:** none.

## Phase 1: Expose the slug→declaring-ADR join

- [ ] **Task 1.1: Extract `DeclaringADRs` from `Check`.** In `internal/invariants/invariants.go`,
  add an exported function holding the exact `required`-map logic currently inline in `Check`
  (Implemented-filter, duplicate-slug refusal, ADR-0031 retirement application), and call it from
  `Check`. It takes already-parsed ADRs (both callers `adr.ParseDir` already), so no double read.

  ```
  // DeclaringADRs returns the slug → declaring-ADR-filename map for adrs: every
  // inv: slug declared (in the Invariants section) by an Implemented ADR, with
  // ADR-0031 retirements applied. It refuses two Implemented ADRs declaring the
  // same slug (duplicate) and a retirement of a slug no Implemented ADR declares
  // (dangling). Check and ContextFor (ADR-0104 Tier 1) share it.
  func DeclaringADRs(adrs []adr.ADR) (map[string]string, error) {
  	required := map[string]string{}
  	for _, a := range adrs {
  		if a.Status != "Implemented" {
  			continue
  		}
  		for _, m := range declRe.FindAllStringSubmatch(a.Sections["Invariants"], -1) {
  			slug := m[1]
  			if prev, ok := required[slug]; ok {
  				return nil, fmt.Errorf("duplicate inv slug %q (in %s and %s)", slug, prev, a.Filename)
  			}
  			required[slug] = a.Filename
  		}
  	}
  	for _, a := range adrs {
  		if a.Status != "Implemented" {
  			continue
  		}
  		for _, slug := range a.RetiresInvariants {
  			if _, ok := required[slug]; !ok {
  				return nil, fmt.Errorf("dangling retirement: ADR %s retires %q, which no Implemented ADR declares as an inv slug", a.Filename, slug)
  			}
  			delete(required, slug)
  		}
  	}
  	return required, nil
  }
  ```

  Then in `Check`, replace the inline `required := map[string]string{}` block (the two loops through
  the duplicate-refusal and retirement application, currently ~L70-96) with:

  ```
  	required, err := DeclaringADRs(adrs)
  	if err != nil {
  		return nil, err
  	}
  ```

  `DeclaringADRs` is a plain extraction: no new `inv:` slug. Its one-to-one / duplicate-refusal /
  retirement behaviour is the pre-existing `Check` contract, unit-tested directly in Task 1.2; the
  context-facing contract that matters is `context-tier1-governs` (ADR-0104), backed in Phase 2.

- [ ] **Task 1.2: Test `DeclaringADRs` directly.** In `internal/invariants/invariants_test.go`, add
  a test asserting: a slug maps to its single Implemented declarer; a non-Implemented ADR's
  declaration is ignored; two Implemented ADRs declaring one slug returns the duplicate error; a
  retirement drops the slug; a dangling retirement errors. Mirror the existing `Check` tests'
  fixture style (`testsupport.ADR` with `WithTitle`/`WithBody` carrying an `## Invariants` section
  and `WithRetiresInvariants`). The existing `Check` tests continue to cover the same branches
  through `Check`; keep them.

- [ ] **Task 1.3: Verify and commit.** `./x gate` (pure refactor + new export; `Check` behaviour
  unchanged; `./x check` clean). Then
  `git add internal/invariants/invariants.go internal/invariants/invariants_test.go` and commit:
  `refactor(invariants): expose DeclaringADRs slug-to-ADR join for reuse`.

## Phase 2: Tier the context output, retire, and flip (single coupled commit)

**Why one commit:** removing the `context-surfaces-pitfalls` and `context-surfaces-linked-plans`
markers (Task 2.4) unbacks 0099/0098 while ADR-0104 is still `Proposed`, so `./x check` fails until
the `retires_invariants` flip (Task 2.7) lands: the retirement-couples-to-flip rule. And the
`ContextResult` struct rewrite (Task 2.1) couples the assembly (Task 2.2) to the rendering
(Task 2.3): a partial phase does not compile. All Phase-2 tasks therefore share one closing commit;
`./x gate` runs once at the end.

- [ ] **Task 2.1: Restructure `ContextResult`.** In `internal/project/context.go`, replace the
  `ContextResult` struct and adjust `ADRRef`/`PitfallRef`:

  ```
  type ContextResult struct {
  	Paths      []string     `json:"paths"`
  	Domains    []DomainRef  `json:"domains"`
  	Invariants []string     `json:"invariants"`
  	Governing  []ADRRef     `json:"governing"`  // Tier 1: invariants backed under the query
  	Related    []ADRRef     `json:"related"`    // Tier 2: precise-tag or related: linked
  	Pitfalls   []PitfallRef `json:"pitfalls"`   // Tier 2: precise-tag match
  	Plans      []PlanRef    `json:"plans"`      // linked to a Tier-1/Tier-2 ADR
  	Background int          `json:"background"` // Tier 3: collapsed domain-ADR count
  	Unowned    []string     `json:"unowned"`
  }
  ```

  Drop `ADRRef.Invariants` (the per-ADR echo the flat `## Invariants` already carries: ADR-0104
  Decision item 6 compaction); `ADRRef` becomes `{Number, Title, Status, Path}`. Change `PitfallRef`
  from `Domains []string` to `Tags []string` (the surfacing reason is now the shared tag), keeping
  `{Title, Tags, Path}`. Update the doc comments to describe the tiers.

  **Dead-code follow-through:** `context.go:122` was the *only* production caller of
  `invariants.DeclaredSlugs`; dropping `ADRRef.Invariants` makes it unreachable from any `main`, so
  the dead-code gate (ADR-0063) fails unless it goes. Delete `DeclaredSlugs` from
  `internal/invariants/invariants.go` and `TestDeclaredSlugs` from
  `internal/invariants/invariants_test.go` in this same Phase-2 commit (confirmed no other caller;
  the tier assembly uses `DeclaringADRs`, not `DeclaredSlugs`).

- [ ] **Task 2.2: Rewrite the tier assembly in `ContextFor`.** Replace the ADR/plan/pitfall
  surfacing block (current `internal/project/context.go` ~L110-183, from `adrs, err := adr.ParseDir`
  through the pitfalls loop) with the tiered assembly. Keep the preceding domain/`owners`/`matched`
  and `res.Invariants` computation unchanged. Add two package-level helpers near the bottom of the
  file (`p.layout()` returns the exported `Layout`, `internal/project/layout.go`):

  ```
  // adrRefOf projects an ADR to its context reference (Title prefix stripped).
  func adrRefOf(a adr.ADR, lay Layout) ADRRef {
  	return ADRRef{
  		Number: a.Number,
  		Title:  strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "),
  		Status: a.Status,
  		Path:   lay.DocsDir + "/decisions/" + a.Filename,
  	}
  }

  // sharesTag reports whether any of tags is in set.
  func sharesTag(tags []string, set map[string]bool) bool {
  	for _, t := range tags {
  		if set[t] {
  			return true
  		}
  	}
  	return false
  }
  ```

  The assembly (replacing the old block):

  ```
  	adrs, err := adr.ParseDir(p.decisionsDir())
  	if err != nil {
  		return ContextResult{}, err
  	}

  	// Tier 1 ("governs this code"): ADRs declaring an invariant slug present as a
  	// marker under a queried path (one-to-one slug -> declaring Implemented ADR).
  	// invariant: context-tier1-governs
  	declaring, err := invariants.DeclaringADRs(adrs)
  	if err != nil {
  		return ContextResult{}, err
  	}
  	byFile := map[string]adr.ADR{}
  	for _, a := range adrs {
  		byFile[a.Filename] = a
  	}
  	tier1 := map[string]bool{}
  	var t1 []adr.ADR
  	for _, slug := range res.Invariants {
  		fn, ok := declaring[slug]
  		if !ok {
  			continue
  		}
  		a := byFile[fn]
  		if tier1[a.Number] {
  			continue
  		}
  		tier1[a.Number] = true
  		t1 = append(t1, a)
  		res.Governing = append(res.Governing, adrRefOf(a, lay))
  	}
  	sort.Slice(res.Governing, func(i, j int) bool { return res.Governing[i].Number < res.Governing[j].Number })

  	// Precise tag set: union of Tier-1 tags minus any tag naming a configured
  	// domain (a domain-mirror tag is Tier-3 relatedness, not Tier-2 precision).
  	domainName := map[string]bool{}
  	for _, d := range p.Cfg.Domains {
  		domainName[d] = true
  	}
  	precise := map[string]bool{}
  	relatedNum := map[int]bool{}
  	for _, a := range t1 {
  		for _, tag := range a.Tags {
  			if !domainName[tag] {
  				precise[tag] = true
  			}
  		}
  		for _, n := range a.Related {
  			relatedNum[n] = true
  		}
  	}

  	// Tier 2 ("topically related"): non-Tier-1, non-Superseded ADRs sharing a
  	// precise tag or named in a Tier-1 ADR's related:.
  	// invariant: context-tier2-topical
  	inTier2 := map[string]bool{}
  	for _, a := range adrs {
  		if tier1[a.Number] || strings.HasPrefix(a.Status, "Superseded") {
  			continue
  		}
  		n, _ := strconv.Atoi(a.Number)
  		if sharesTag(a.Tags, precise) || relatedNum[n] {
  			inTier2[a.Number] = true
  			res.Related = append(res.Related, adrRefOf(a, lay))
  		}
  	}
  	sort.Slice(res.Related, func(i, j int) bool { return res.Related[i].Number < res.Related[j].Number })

  	// Tier 2 pitfalls: share a precise tag (only when the pitfalls doc is enabled).
  	// invariant: context-surfaces-tiered-pitfalls
  	if slices.Contains(p.Cfg.Docs, "pitfalls") {
  		sc, err := p.Cfg.Sidecar("docs", "pitfalls")
  		if err != nil {
  			return ContextResult{}, err
  		}
  		entries, err := pitfallEntries(sc.Data["pitfalls"])
  		if err != nil {
  			return ContextResult{}, err
  		}
  		for _, e := range entries {
  			if sharesTag(e.Tags, precise) {
  				res.Pitfalls = append(res.Pitfalls, PitfallRef{Title: e.Title, Tags: e.Tags, Path: lay.DocsDir + "/pitfalls.md"})
  			}
  		}
  		sort.Slice(res.Pitfalls, func(i, j int) bool { return res.Pitfalls[i].Title < res.Pitfalls[j].Title })
  	}

  	// Tier 3 ("domain background"): domain-membership ADRs in neither Tier 1 nor
  	// Tier 2, reported only as a collapsed count.
  	// invariant: context-tier3-collapsed
  	for _, a := range adrs {
  		if tier1[a.Number] || inTier2[a.Number] {
  			continue
  		}
  		for _, dm := range a.Domains {
  			if owners[dm] {
  				res.Background++
  				break
  			}
  		}
  	}

  	// Plans linked to a Tier-1 or Tier-2 ADR.
  	// invariant: context-surfaces-tiered-plans
  	surfaced := map[int]bool{}
  	for _, a := range append(append([]ADRRef{}, res.Governing...), res.Related...) {
  		if n, err := strconv.Atoi(a.Number); err == nil { // coverage-ignore: a.Number is always a 4-digit numeral from FilenameRe
  			surfaced[n] = true
  		}
  	}
  	plans, err := plan.ParseDir(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"))
  	if err != nil {
  		return ContextResult{}, err
  	}
  	for _, pl := range plans {
  		if !pl.HasFrontmatter {
  			continue
  		}
  		for _, n := range pl.ADRs {
  			if surfaced[n] {
  				res.Plans = append(res.Plans, PlanRef{Filename: pl.Filename, Path: lay.PlansDir + "/" + pl.Filename, Status: pl.Status, ADRs: pl.ADRs})
  				break
  			}
  		}
  	}
  	sort.Slice(res.Plans, func(i, j int) bool { return res.Plans[i].Filename < res.Plans[j].Filename })
  	return res, nil
  ```

  Confirm the imports (`sort`, `strconv`, `slices`, `filepath`, `strings`, `invariants`, `plan`,
  `adr`) are all already present in `context.go` (they were used by the replaced code). Remove any
  that the rewrite no longer uses to keep the build clean.

- [ ] **Task 2.3: Render the tiers in `printContext`.** In `cmd/awf/context.go`, replace the
  `## Related ADRs` / `## Related plans` / `## Related pitfalls` blocks (keep the JSON branch,
  `## Domains`, `## Invariants`, `## Unowned` unchanged: the JSON branch already encodes the new
  struct, preserving `context-output-parity`):

  ```
  	if len(res.Governing) > 0 {
  		fmt.Fprintln(stdout, "\n## Governing ADRs (invariants backed here)")
  		for _, a := range res.Governing {
  			fmt.Fprintf(stdout, "  ADR-%s (%s) %s: %s\n", a.Number, a.Status, a.Title, a.Path)
  		}
  	}
  	if len(res.Related) > 0 {
  		fmt.Fprintln(stdout, "\n## Related ADRs (shared tag)")
  		for _, a := range res.Related {
  			fmt.Fprintf(stdout, "  ADR-%s (%s) %s: %s\n", a.Number, a.Status, a.Title, a.Path)
  		}
  	}
  	if len(res.Plans) > 0 {
  		fmt.Fprintln(stdout, "\n## Related plans")
  		for _, pl := range res.Plans {
  			fmt.Fprintf(stdout, "  %s (%s): %s\n", pl.Filename, pl.Status, pl.Path)
  		}
  	}
  	if len(res.Pitfalls) > 0 {
  		fmt.Fprintln(stdout, "\n## Related pitfalls (shared tag)")
  		for _, pf := range res.Pitfalls {
  			fmt.Fprintf(stdout, "  %s %v: %s\n", pf.Title, pf.Tags, pf.Path)
  		}
  	}
  	if res.Background > 0 {
  		fmt.Fprintf(stdout, "\n## Domain background: %d more ADR(s) (see the domain docs above)\n", res.Background)
  	}
  ```

  Keep the `// invariant: context-output-parity` marker on `printContext`.

- [ ] **Task 2.4: Confirm the two retired markers are gone.** The old
  `// invariant: context-surfaces-linked-plans` and `// invariant: context-surfaces-pitfalls`
  comments were on the code Task 2.2 replaced, so they are already removed. Confirm:
  `grep -rn 'context-surfaces-linked-plans\|context-surfaces-pitfalls' internal/ cmd/` returns
  nothing (the slugs survive only in ADR prose).

- [ ] **Task 2.5: Rewrite the context tests.** In `internal/project/context_test.go` and
  `cmd/awf/context_test.go`, replace assertions keyed on the old flat `ADRs`/domain-based `Pitfalls`
  with tier assertions. Cover, at 100%: a query whose backed invariant puts an ADR in Tier 1
  (`Governing`); a precise-tag ADR and pitfall in Tier 2; a domain-mirror-only Tier-1 tag yielding no
  tag-based Tier 2; a `related:`-linked Tier-2 ADR under an empty precise set; a Superseded ADR
  excluded from Tier 2; a domain-membership ADR counted in `Background`; a plan linked to a
  Governing/Related ADR surfaced; the pitfalls-disabled path; **a present marker whose slug no
  Implemented ADR declares** (the `if !ok { continue }` Tier-1 skip, e.g. an `// invariant:` marker
  for a Proposed ADR's slug under the queried path); **one Implemented ADR declaring two distinct
  present slugs** (the `if tier1[a.Number] { continue }` Tier-1 dedup skip); and JSON/human parity
  (mirror the existing parity test). Use `testsupport.ADR` with `WithTags`/`WithRelated`/`WithDomains` and an
  `## Invariants` body section carrying an `` - `inv: <slug>` `` bullet plus a matching source-marker
  fixture (a `.go` file under a domain glob with `// invariant: <slug>`) so the Tier-1 join resolves.
  Keep the read-only / output-parity / static-fallback tests green. The `DeclaringADRs` error path in
  `ContextFor` (a duplicate slug across two Implemented ADRs) is reachable via a fixture; test it
  rather than coverage-ignore.

- [ ] **Task 2.6: Doc currency.** In `.awf/agents-doc.yaml`, remove the two invariant bullets for
  `context-surfaces-linked-plans` (ADR-0098) and `context-surfaces-pitfalls` (ADR-0099), and add
  five for `ref: ADR-0104` (`context-tier1-governs`, `context-tier2-topical`,
  `context-tier3-collapsed`, `context-surfaces-tiered-plans`, `context-surfaces-tiered-pitfalls`),
  each phrased to the ADR's Invariants statements. Add a sentence to
  `.awf/domains/parts/tooling/current-state.md` (the tiered `awf
  context` output) and `.awf/domains/parts/invariants/current-state.md` (the shared `DeclaringADRs`
  join + the tiered surfacing invariants replacing the domain-based ones). Add a changelog
  `[Unreleased]` **Breaking changes** entry (the `awf context` human and JSON output shape changes:
  `governing`/`related`/`background` fields replace the flat `adrs`, pitfalls surface by tag).

- [ ] **Task 2.7: Flip and regenerate.** Set ADR-0104 `status:` to `Implemented` and this plan's
  `status:` to `Implemented`. Run `./x sync` to regenerate `AGENTS.md`, the `tooling`/`invariants`
  domain indices, and `ACTIVE.md`. ADR-0104's `retires_invariants` now takes effect: 0098/0099's two
  slugs are retired (their removed markers no longer fail the check), the five new markers are
  enforced-backed.

- [ ] **Task 2.8: Verify and commit.** `./x gate` and `./x check` (green: new slugs backed, retired
  slugs no longer required, no dangling retirement, output-parity preserved). Then stage explicitly:
  `internal/invariants/invariants.go internal/invariants/invariants_test.go
  internal/project/context.go internal/project/context_test.go cmd/awf/context.go
  cmd/awf/context_test.go .awf/agents-doc.yaml AGENTS.md
  .awf/domains/parts/tooling/current-state.md .awf/domains/parts/invariants/current-state.md
  docs/domains/tooling.md docs/domains/invariants.md changelog/CHANGELOG.md
  docs/decisions/0104-tag-tiered-relevance-in-awf-context.md docs/decisions/ACTIVE.md
  docs/plans/2026-07-13-tag-tiered-relevance-in-awf-context.md .awf/awf.lock`, plus any
  `examples/sundial` outputs `git status --short` reports (verify per the render-fan-out pitfall;
  none expected, since no template/configspec/catalog changed). Commit:
  `feat(tooling): tag-tiered relevance in awf context; implement 0104`.

## Verification

- `./x gate` green at both phase boundaries; `./x check` clean after Phase 2.
- `grep -rn 'context-surfaces-linked-plans\|context-surfaces-pitfalls' internal/ cmd/` returns
  nothing; the slugs survive only in ADR prose.
- Dogfood: `go run ./cmd/awf context internal/project/context.go` shows a small `## Governing ADRs`,
  a bounded `## Related ADRs`, and a `## Domain background: N more ADR(s)` line rather than the old
  ~90-ADR dump. Record before/after in Notes.
- Human and `--json` renderings report the same tiers (parity test green).
- ADR-0104 and this plan are `Implemented`; `ACTIVE.md` lists 0104 under Implemented.

## Notes

- **Out of scope (recommended follow-ups from ADR-0104):** the domain-coverage expansion for unowned
  packages (a separate load-bearing decision: new domain vs fold-in), and any further Tier-2
  narrowing (weight by shared-tag count, ≥2 shared tags). This plan changes only the surfacing model.
- **Two user-decision ADR findings** were resolved with reasoned defaults and are flagged for review:
  the precise-tag-set domain-mirror exclusion, and the domain-coverage demotion to a follow-up.
- **Dogfood at flip** (`awf context internal/project/context.go`): Tier 1 (`Governing`) is now
  precise (ADR-0102 + ADR-0104, the two ADRs whose `context-*`/`uncovered-*` invariants are backed
  in that file) versus the pre-slice ~90-ADR undifferentiated dump. Tier 3 background is 0 (the file
  is domain-unowned). **Tier 2 (`Related`) is still broad (~28 ADRs)** because the Tier-1 ADRs carry
  the high-frequency non-domain-mirror tag `testing`, which the domain-mirror exclusion does not
  remove. This is the residual risk ADR-0104's Consequences flagged: the precise Tier-1 win lands,
  but further Tier-2 narrowing (weight by shared-tag count, or ≥2 shared tags) is a worthwhile
  follow-up. **Flagged for review.**
- No `coverage-ignore` was added in Task 2.5; the one pre-existing `a.Number` ignore was preserved.
- No `examples/sundial` regeneration (no template/configspec/catalog change), so the render-fan-out
  pitfall did not apply this slice.
