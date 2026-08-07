package contextq

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// invariant: tooling/context-and-topic:context-adr-operation-projection (TestContextADRProjection)
func TestContextADRProjection(t *testing.T) {
	files := ctxFiles()
	raw := `---
format: current-state-v1
status: Proposed
date: 2026-07-27
---
# ADR-0002: Example

## Context

Context.

## Decision

1. Decision.

## State changes

- add ` + "`alpha/one:new-rule`" + `

## Consequences

Consequence.

## Alternatives Considered

None.

## Status history

- 2026-07-27: Proposed
`
	state, err := ctxRepo(t, ctxConfig, files).ContextState(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	// An explicit non-ADR request remains unrelated and has no plan relationship.
	if got := queryFor(t, ctxRepo(t, ctxConfig, files)).ContextForOptions([]string{"not-a-decision"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{FacetReferences}}); got.Requests[0].Exact.Context.ADR != nil {
		t.Fatalf("unrelated request ADR=%#v", got.Requests[0].Exact.Context.ADR)
	}
	record, err := adr.ParseV1("0002-example.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	corpus := mustCorpus(append(state.Loaded.ADRs, record))
	plain := projectADRArtifact("docs/decisions/0002-example.md", state.Layout.ADRDir, corpus, state.Loaded.Topics, nil, nil)
	if plain == nil || plain.Status != "Proposed" || len(plain.Operations) != 0 {
		t.Fatalf("plain=%#v", plain)
	}
	linked := projectADRArtifact("docs/decisions/0002-example.md", state.Layout.ADRDir, corpus, state.Loaded.Topics, []string{"docs/plans/2026-08-01-a.md", "docs/plans/2026-08-02-b.md"}, []ContextFacet{FacetReferences})
	if linked == nil || len(linked.LinkedPlans) != 2 || linked.LinkedPlans[0] != "docs/plans/2026-08-01-a.md" {
		t.Fatalf("linked plans=%#v", linked)
	}
	if noReferences := projectADRArtifact("docs/decisions/0002-example.md", state.Layout.ADRDir, corpus, state.Loaded.Topics, []string{"docs/plans/2026-08-01-a.md"}, nil); noReferences == nil || len(noReferences.LinkedPlans) != 0 {
		t.Fatalf("non-reference plans=%#v", noReferences)
	}
	full := projectADRArtifact("docs/decisions/0002-example.md", state.Layout.ADRDir, corpus, state.Loaded.Topics, nil, []ContextFacet{FacetPending, FacetEvidence})
	if len(full.Operations) != 1 || full.Operations[0].Progress != "proposed" || full.Operations[0].ClaimState != "not-yet-current" {
		t.Fatalf("full=%#v", full)
	}
	if projectADRArtifact("docs/decisions/not-an-adr.md", state.Layout.ADRDir, corpus, state.Loaded.Topics, nil, nil) != nil {
		t.Fatal("lookalike attributed")
	}
	if projectADRArtifact("docs/decisions/0002-wrong.md", state.Layout.ADRDir, corpus, state.Loaded.Topics, nil, nil) != nil {
		t.Fatal("wrong filename attributed")
	}
	if projectADRArtifact("elsewhere/0002-example.md", state.Layout.ADRDir, corpus, state.Loaded.Topics, nil, nil) != nil {
		t.Fatal("outside directory attributed")
	}
	// A pending record is looked up and presented under its slug: a
	// number-keyed lookup resolves nothing for it, and a number-valued
	// identity would present it as the empty string.
	pendingRecord := adr.ADR{Slug: "still-pending", Title: "ADR-still-pending: Pending", Filename: "still-pending.md", Status: "Proposed", Format: adr.CurrentStateV3}
	slugged := projectADRArtifact("docs/decisions/still-pending.md", state.Layout.ADRDir, mustCorpus([]adr.ADR{pendingRecord}), state.Loaded.Topics, nil, nil)
	if slugged == nil || slugged.Number != "still-pending" || slugged.Title != "Pending" {
		t.Fatalf("pending artifact=%#v", slugged)
	}
	update := adr.Operation{Verb: adr.OpUpdate, ID: "alpha/one:order", Slug: "order"}
	add := adr.Operation{Verb: adr.OpAdd, ID: "alpha/one:new-rule", Slug: "new-rule"}
	broken := adr.ADR{Number: "0005", Title: "ADR-0005: Broken", Filename: "0005-broken.md", Status: "Implementing", Format: adr.CurrentStateV2, Operations: []adr.Operation{add}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Operations: []adr.Operation{update}}}}
	brokenImpact := projectADRArtifact("docs/decisions/0005-broken.md", state.Layout.ADRDir, mustCorpus([]adr.ADR{broken}), state.Loaded.Topics, nil, []ContextFacet{FacetPending})
	if brokenImpact == nil || len(brokenImpact.Operations) != 0 {
		t.Fatal(brokenImpact)
	}
	implementing := adr.ADR{Number: "0003", Title: "ADR-0003: Implementing", Filename: "0003-implementing.md", Status: "Implementing", Format: adr.CurrentStateV2, Operations: []adr.Operation{update, add}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Operations: []adr.Operation{update}}}}
	impl := projectADRArtifact("docs/decisions/0003-implementing.md", state.Layout.ADRDir, mustCorpus([]adr.ADR{implementing}), state.Loaded.Topics, nil, []ContextFacet{FacetPending, FacetEvidence})
	if impl == nil || len(impl.Operations) != 2 || impl.Operations[0].Progress != "applied" || impl.Operations[1].Progress != "remaining" || impl.Operations[0].Detail == nil {
		t.Fatalf("implementing=%#v", impl)
	}
	remove := adr.Operation{Verb: adr.OpRemove, ID: "alpha/one:tested", Slug: "tested"}
	abandoned := adr.ADR{Number: "0004", Title: "ADR-0004: Abandoned", Filename: "0004-abandoned.md", Status: "Abandoned", Format: adr.CurrentStateV2, Operations: []adr.Operation{remove, add}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Operations: []adr.Operation{remove}}}}
	add2 := adr.Operation{Verb: adr.OpAdd, ID: "alpha/one:another", Slug: "another"}
	changes := pendingChanges(mustCorpus([]adr.ADR{{Number: "0001", Status: "Implemented"}, {Number: "0003", Title: "ADR-0003: Later", Status: "Accepted", Format: adr.CurrentStateV2, Operations: []adr.Operation{add}}, {Number: "0002", Title: "ADR-0002: Pending", Status: "Accepted", Format: adr.CurrentStateV2, Operations: []adr.Operation{add, add2}}}), map[string]bool{"alpha/one": true})
	if len(changes) != 3 || changes[0].ADR != "0002" || changes[0].Claim != "alpha/one:new-rule" || changes[1].Claim != "alpha/one:another" || changes[2].ADR != "0003" {
		t.Fatalf("pending declaration order=%#v", changes)
	}
	if got := pendingChanges(mustCorpus([]adr.ADR{{Number: "0002", Status: "Accepted", Format: adr.CurrentStateV2, Operations: []adr.Operation{add}}}), map[string]bool{"other/topic": true}); len(got) != 0 {
		t.Fatal(got)
	}
	// A pending record answers to its slug, so it is projected under that
	// identity and sorted after every numbered record rather than dropped by a
	// number lookup that resolves nothing.
	pending := adr.ADR{Slug: "keep-the-corpus", Title: "ADR-keep-the-corpus: Pending", Filename: "keep-the-corpus.md", Status: "Accepted", Format: adr.CurrentStateV3, Operations: []adr.Operation{add}}
	numbered := adr.ADR{Number: "0002", Title: "ADR-0002: Numbered", Filename: "0002-numbered.md", Status: "Accepted", Format: adr.CurrentStateV2, Operations: []adr.Operation{add2}}
	mixed := pendingChanges(mustCorpus([]adr.ADR{pending, numbered}), map[string]bool{"alpha/one": true})
	if len(mixed) != 2 || mixed[0].ADR != "0002" || mixed[1].ADR != "keep-the-corpus" || mixed[1].Title != "Pending" || mixed[1].Claim != "alpha/one:new-rule" {
		t.Fatalf("pending identity projection=%#v", mixed)
	}
	malformed := adr.ADR{Number: "0006", Status: "Accepted", Format: adr.CurrentStateV2, Operations: []adr.Operation{add}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Operations: []adr.Operation{update}}}}
	if got := pendingChanges(mustCorpus([]adr.ADR{malformed}), map[string]bool{"alpha/one": true}); len(got) != 0 {
		t.Fatal(got)
	}
	gone := projectADRArtifact("docs/decisions/0004-abandoned.md", state.Layout.ADRDir, mustCorpus([]adr.ADR{abandoned}), state.Loaded.Topics, nil, []ContextFacet{FacetPending, FacetEvidence})
	if gone == nil || gone.Operations[1].Progress != "canceled" {
		t.Fatalf("abandoned=%#v", gone)
	}
	// The projected mutability is the lifecycle's amendability, not "is it
	// Proposed": ADR-0188 keeps a V2 body amendable through Accepted and
	// Implementing and freezes it only at a terminal status, while a V1 body
	// still freezes the moment it leaves Proposed.
	if plain.Mutability != "mutable" {
		t.Errorf("v1 Proposed mutability = %q, want mutable", plain.Mutability)
	}
	if impl.Mutability != "mutable" {
		t.Errorf("v2 Implementing mutability = %q, want mutable", impl.Mutability)
	}
	if gone.Mutability != "frozen" {
		t.Errorf("v2 Abandoned mutability = %q, want frozen", gone.Mutability)
	}
	for _, tc := range []struct {
		name, status, want string
		format             adr.Format
	}{
		{"v2 Accepted", "Accepted", "mutable", adr.CurrentStateV2},
		{"v2 Implemented", "Implemented", "frozen", adr.CurrentStateV2},
		{"v1 Accepted", "Accepted", "frozen", adr.CurrentStateV1},
		{"v1 Implemented", "Implemented", "frozen", adr.CurrentStateV1},
	} {
		rec := adr.ADR{Number: "0007", Title: "ADR-0007: Case", Filename: "0007-case.md", Status: tc.status, Format: tc.format}
		got := projectADRArtifact("docs/decisions/0007-case.md", state.Layout.ADRDir, mustCorpus([]adr.ADR{rec}), state.Loaded.Topics, nil, nil)
		if got == nil || got.Mutability != tc.want {
			t.Errorf("%s mutability = %#v, want %q", tc.name, got, tc.want)
		}
	}
}

// invariant: tooling/context-and-topic:adr-linked-plan-references (TestContextADRLinkedPlansUseResolvedSnapshotReferences)
func TestContextADRLinkedPlansUseResolvedSnapshotReferences(t *testing.T) {
	files := ctxFiles()
	files["docs/decisions/0007-numbered.md"] = testsupport.ADR("Implemented", testsupport.WithDate("2026-08-07"), testsupport.WithTitle("0007: Numbered"), testsupport.WithBody("## Context\n\nContext.\n\n## Decision\n\n1. Decision.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-07: Implemented\n"))
	files["docs/decisions/still-pending.md"] = linkedPlanADRFixture("still-pending")
	files["docs/plans/2026-08-01-a.md"] = linkedPlanFixture("[7]", "Implemented")
	files["docs/plans/2026-08-02-z.md"] = linkedPlanFixture("[7, still-pending]", "Proposed")
	p := ctxRepo(t, ctxConfig, files)
	lock := &manifest.Lock{AWFVersion: "0.0.0", SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := lock.Save(lockFile(p.Root)); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, gitfixture.At(p.Root))

	wantNumbered := []string{"docs/plans/2026-08-01-a.md", "docs/plans/2026-08-02-z.md"}
	working := queryFor(t, p).ContextForOptions([]string{"docs/decisions/0007-numbered.md", "docs/decisions/still-pending.md", "internal/foo/x.go"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{FacetReferences}})
	if got := working.Requests[0].Exact.Context.ADR.LinkedPlans; !slices.Equal(got, wantNumbered) {
		t.Fatalf("numbered linked plans = %#v, want %#v", got, wantNumbered)
	}
	if got, want := working.Requests[1].Exact.Context.ADR.LinkedPlans, []string{"docs/plans/2026-08-02-z.md"}; !slices.Equal(got, want) {
		t.Fatalf("pending linked plans = %#v, want %#v", got, want)
	}
	if working.Requests[2].Exact.Context.ADR != nil {
		t.Fatalf("unrelated request gained ADR context: %#v", working.Requests[2].Exact.Context.ADR)
	}
	if rendered := RenderContextText(working, "header", []ContextFacet{FacetReferences}); !strings.Contains(rendered, "linked-plans: docs/plans/2026-08-01-a.md, docs/plans/2026-08-02-z.md") {
		t.Fatalf("deterministic linked plans missing from output:\n%s", rendered)
	}

	// The working tree drops the pending link while the index keeps the staged
	// parsed-plan association, proving both constructors use their own snapshot.
	testsupport.WriteFile(t, filepath.Join(p.Root, "docs/plans/2026-08-02-z.md"), linkedPlanFixture("[7]", "Proposed"))
	working = queryFor(t, p).ContextForOptions([]string{"docs/decisions/still-pending.md"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{FacetReferences}})
	staged := stagedQueryFor(t, p.Root).ContextForOptions([]string{"docs/decisions/still-pending.md"}, ContextOptions{Selection: SelectionExplicit, Facets: []ContextFacet{FacetReferences}})
	if got := working.Requests[0].Exact.Context.ADR.LinkedPlans; len(got) != 0 {
		t.Fatalf("working linked plans = %#v, want none", got)
	}
	if got, want := staged.Requests[0].Exact.Context.ADR.LinkedPlans, []string{"docs/plans/2026-08-02-z.md"}; !slices.Equal(got, want) {
		t.Fatalf("staged linked plans = %#v, want %#v", got, want)
	}
}

func linkedPlanADRFixture(slug string) string {
	return "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-07\nslug: " + slug + "\n---\n# ADR-" + slug + ": Linked plan fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: fixture` Fixture.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-07: Proposed\n"
}

func linkedPlanFixture(adrs, status string) string {
	return "---\nformat: plan-v2\ndate: 2026-08-07\nadrs: " + adrs + "\nstatus: " + status + "\n---\n# Plan: Linked plan fixture\n\n## Goal\n\nExercise linked plans.\n\n## Architecture summary\n\nUse parsed links.\n\n## Phase 1: Expose\n\n**Execution mode: inline.**\n\nCompletes: [\"complete\"]\n\n### Task 1.1: Render\n\nQuery the ADR.\n\n### Phase close\n\n```commit\ntest(tooling): query linked plans\n```\n\n## Definition of done\n\n- `dod: complete` Linked plans render.\n"
}

// mustCorpus builds a corpus from fixture records that carry no duplicate
// identity, so the construction error the seam returns cannot occur here.
func mustCorpus(records []adr.ADR) adr.Corpus {
	c, err := adr.NewCorpus(records)
	if err != nil { // coverage-ignore: fixture records are duplicate-free by construction
		panic(err)
	}
	return c
}
