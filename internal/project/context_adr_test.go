package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

// invariant: tooling/context-and-topic:context-adr-operation-projection
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
	p := csRepo(t, ctxConfig, files)
	ws, err := p.workingCurrentState(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	record, err := adr.ParseV1("0002-example.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	corpus := mustCorpus(append(ws.Loaded.ADRs, record))
	plain := projectADRArtifact("docs/decisions/0002-example.md", p.layout().ADRDir, corpus, ws.Loaded.Topics, nil)
	if plain == nil || plain.Status != "Proposed" || len(plain.Operations) != 0 {
		t.Fatalf("plain=%#v", plain)
	}
	full := projectADRArtifact("docs/decisions/0002-example.md", p.layout().ADRDir, corpus, ws.Loaded.Topics, []ContextFacet{FacetPending, FacetEvidence})
	if len(full.Operations) != 1 || full.Operations[0].Progress != "proposed" || full.Operations[0].ClaimState != "not-yet-current" {
		t.Fatalf("full=%#v", full)
	}
	if projectADRArtifact("docs/decisions/not-an-adr.md", p.layout().ADRDir, corpus, ws.Loaded.Topics, nil) != nil {
		t.Fatal("lookalike attributed")
	}
	if projectADRArtifact("docs/decisions/0002-wrong.md", p.layout().ADRDir, corpus, ws.Loaded.Topics, nil) != nil {
		t.Fatal("wrong filename attributed")
	}
	if projectADRArtifact("elsewhere/0002-example.md", p.layout().ADRDir, corpus, ws.Loaded.Topics, nil) != nil {
		t.Fatal("outside directory attributed")
	}
	update := adr.Operation{Verb: adr.OpUpdate, ID: "alpha/one:order", Slug: "order"}
	add := adr.Operation{Verb: adr.OpAdd, ID: "alpha/one:new-rule", Slug: "new-rule"}
	broken := adr.ADR{Number: "0005", Title: "ADR-0005: Broken", Filename: "0005-broken.md", Status: "Implementing", Format: adr.CurrentStateV2, Operations: []adr.Operation{add}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Operations: []adr.Operation{update}}}}
	brokenImpact := projectADRArtifact("docs/decisions/0005-broken.md", p.layout().ADRDir, mustCorpus([]adr.ADR{broken}), ws.Loaded.Topics, []ContextFacet{FacetPending})
	if brokenImpact == nil || len(brokenImpact.Operations) != 0 {
		t.Fatal(brokenImpact)
	}
	implementing := adr.ADR{Number: "0003", Title: "ADR-0003: Implementing", Filename: "0003-implementing.md", Status: "Implementing", Format: adr.CurrentStateV2, Operations: []adr.Operation{update, add}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Operations: []adr.Operation{update}}}}
	impl := projectADRArtifact("docs/decisions/0003-implementing.md", p.layout().ADRDir, mustCorpus([]adr.ADR{implementing}), ws.Loaded.Topics, []ContextFacet{FacetPending, FacetEvidence})
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
	gone := projectADRArtifact("docs/decisions/0004-abandoned.md", p.layout().ADRDir, mustCorpus([]adr.ADR{abandoned}), ws.Loaded.Topics, []ContextFacet{FacetPending, FacetEvidence})
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
		got := projectADRArtifact("docs/decisions/0007-case.md", p.layout().ADRDir, mustCorpus([]adr.ADR{rec}), ws.Loaded.Topics, nil)
		if got == nil || got.Mutability != tc.want {
			t.Errorf("%s mutability = %#v, want %q", tc.name, got, tc.want)
		}
	}
}
