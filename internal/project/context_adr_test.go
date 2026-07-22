package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: tooling/context-and-topic:context-adr-operation-projection
func TestADRArtifactProjectionUsesLifecycleApplicationRecords(t *testing.T) {
	add := adr.Operation{Verb: adr.OpAdd, ID: "alpha/one:new"}
	update := adr.Operation{Verb: adr.OpUpdate, ID: "alpha/one:existing"}
	cases := []struct {
		name, status string
		history      []adr.HistoryEvent
		want         []string
		sequences    []int
	}{
		{name: "proposed", status: "Proposed", want: []string{"proposed", "proposed"}, sequences: []int{0, 0}},
		{name: "accepted", status: "Accepted", want: []string{"remaining", "remaining"}, sequences: []int{0, 0}},
		{name: "implementing", status: "Implementing", history: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 7, Operations: []adr.Operation{add}}}, want: []string{"applied", "remaining"}, sequences: []int{7, 0}},
		{name: "implemented", status: "Implemented", history: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 8, Operations: []adr.Operation{add, update}}}, want: []string{"applied", "applied"}, sequences: []int{8, 8}},
		{name: "abandoned", status: "Abandoned", history: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 9, Operations: []adr.Operation{add}}}, want: []string{"applied", "canceled"}, sequences: []int{9, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := adr.ADR{Number: "0002", Title: "ADR-0002: Example", Filename: "0002-example.md", Status: tc.status, Format: adr.CurrentStateV2, Operations: []adr.Operation{add, update}, History: tc.history}
			got := projectADRArtifact("docs/decisions/0002-example.md", "docs/decisions", adr.NewCorpus([]adr.ADR{record}), topic.Corpus{}, ContextConcise)
			if got == nil || got.Number != "0002" || got.Title != "Example" || len(got.Operations) != 2 {
				t.Fatalf("projection = %#v", got)
			}
			for i := range got.Operations {
				if got.Operations[i].Progress != tc.want[i] || got.Operations[i].StateSequence != tc.sequences[i] || got.Operations[i].Detail != nil {
					t.Errorf("operation %d = %#v", i, got.Operations[i])
				}
			}
			if tc.status == "Proposed" && (got.Mutability != "mutable" || got.AuthorityRole != "pending intent, never current authority") {
				t.Errorf("proposed authority = %#v", got)
			}
			if tc.status == "Accepted" {
				full := projectADRArtifact("docs/decisions/0002-example.md", "docs/decisions", adr.NewCorpus([]adr.ADR{record}), topic.Corpus{}, ContextFull)
				if full.Operations[0].Detail == nil || full.Operations[0].Detail.Current != nil || full.Operations[0].Detail.MarkerSites == nil {
					t.Errorf("pending full detail = %#v", full.Operations[0])
				}
			}
		})
	}
}

func TestADRArtifactFullProjectionLinksCurrentClaimHistory(t *testing.T) {
	files := ctxFiles()
	files["docs/decisions/0002-example.md"] = acceptedV1(t, "0002", "Example", "2026-07-20", "- update `alpha/one:stable`")
	p := csRepo(t, ctxConfig, files)
	writeCutoffLock(t, p, 2)
	result, err := p.ContextForFull([]string{"docs/decisions/0002-example.md"})
	if err != nil {
		t.Fatal(err)
	}
	got := result.Paths[0].ADR
	if got == nil || len(got.Operations) != 1 || got.Operations[0].Detail == nil || got.Operations[0].Detail.Current == nil || got.Operations[0].Detail.History == nil {
		t.Fatalf("full ADR projection = %#v", got)
	}
	if got.Operations[0].ClaimState != "active-current" || got.Operations[0].Detail.MarkerSites == nil {
		t.Fatalf("full operation detail = %#v", got.Operations[0])
	}
	if sites := nonNilMarkerSites(nil); sites == nil || len(sites) != 0 {
		t.Fatalf("nil marker sites = %#v", sites)
	}
	inputSites := []topic.MarkerSite{{Path: "x", Line: 1}}
	if sites := nonNilMarkerSites(inputSites); len(sites) != 1 || &sites[0] == &inputSites[0] {
		t.Fatalf("marker-site copy = %#v", sites)
	}
}

func TestADRArtifactProjectionInvalidProgressStaysBounded(t *testing.T) {
	op := adr.Operation{Verb: adr.OpAdd, ID: "alpha/one:new"}
	record := adr.ADR{Number: "0002", Title: "ADR-0002: Example", Filename: "0002-example.md", Status: "Accepted", Format: adr.CurrentStateV2, Operations: []adr.Operation{op}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 1, Operations: []adr.Operation{op}}}}
	got := projectADRArtifact("docs/decisions/0002-example.md", "docs/decisions", adr.NewCorpus([]adr.ADR{record}), topic.Corpus{}, ContextConcise)
	if got == nil || len(got.Operations) != 0 {
		t.Fatalf("invalid progress projection = %#v", got)
	}
}

func TestADRArtifactProjectionDirectV2RemoveAndRelocatedDocsDir(t *testing.T) {
	claim := "alpha/one:gone"
	add := adr.ADR{Number: "0001", Title: "ADR-0001: Add", Filename: "0001-add.md", Status: "Implemented", Format: adr.CurrentStateV2, Operations: []adr.Operation{{Verb: adr.OpAdd, ID: claim}}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 1, Operations: []adr.Operation{{Verb: adr.OpAdd, ID: claim}}}}}
	remove := adr.ADR{Number: "0002", Title: "ADR-0002: Remove", Filename: "0002-remove.md", Status: "Implemented", Format: adr.CurrentStateV2, Operations: []adr.Operation{{Verb: adr.OpRemove, ID: claim}}, History: []adr.HistoryEvent{{Kind: adr.HistoryApplied, Sequence: 2, Operations: []adr.Operation{{Verb: adr.OpRemove, ID: claim}}}}}
	corpus := adr.NewCorpus([]adr.ADR{add, remove})
	cfg, err := config.ParseTree(".awf", []byte("prefix: x\ndomains: [alpha]\n"), configReaderAdapter{memoryProjectReader{}})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: ".awf/domains/alpha.yaml", Mode: snapshot.Regular, Bytes: []byte("paths: [\"internal/**\"]\n")},
		{Path: ".awf/topics/metadata/alpha/one.yaml", Mode: snapshot.Regular, Bytes: []byte("title: One\nsummary: One.\npaths: [\"internal/**\"]\n")},
		{Path: ".awf/topics/parts/alpha/one/current-state.md", Mode: snapshot.Regular, Bytes: []byte("Intro.\n\n## Claims\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	topics, err := topic.LoadCorpusFromTree(tree, cfg, corpus)
	if err != nil {
		t.Fatal(err)
	}
	got := projectADRArtifact("handbook/decisions/0002-remove.md", "handbook/decisions", corpus, topics, ContextFull)
	if got == nil || got.Status != "Implemented" || len(got.Operations) != 1 || got.Operations[0].Progress != "applied" || got.Operations[0].StateSequence != 2 || got.Operations[0].ClaimState != "historically-removed" {
		t.Fatalf("direct V2 relocated remove projection = %#v", got)
	}
	if got.Operations[0].Detail == nil || got.Operations[0].Detail.Current != nil || got.Operations[0].Detail.History == nil || got.Operations[0].Detail.History.RemovedBy == nil || got.Operations[0].Detail.History.RemovedBy.StateSequence != 2 || got.Operations[0].Detail.MarkerSites == nil {
		t.Fatalf("removed detail = %#v", got.Operations[0].Detail)
	}
	if outside := projectADRArtifact("docs/decisions/0002-remove.md", "handbook/decisions", corpus, topic.Corpus{}, ContextFull); outside != nil {
		t.Fatalf("default docsDir lookalike projected in relocated layout: %#v", outside)
	}
}

func TestADRArtifactProjectionRequiresExplicitParsedIdentity(t *testing.T) {
	record := adr.ADR{Number: "0002", Title: "ADR-0002: Example", Filename: "0002-example.md", Status: "Accepted", Format: adr.CurrentStateV2, Operations: []adr.Operation{}}
	corpus := adr.NewCorpus([]adr.ADR{record})
	for _, path := range []string{"docs/decisions/README.md", "elsewhere/0002-example.md", "docs/decisions/0002-lookalike.md"} {
		if got := projectADRArtifact(path, "docs/decisions", corpus, topic.Corpus{}, ContextConcise); got != nil {
			t.Errorf("%s projected as %#v", path, got)
		}
	}
}
