package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: tooling/context-and-topic:context-summary-projection
func TestClaimSummaryProjection(t *testing.T) {
	parse := func(summary, prose string) topic.Claim {
		t.Helper()
		part := "Intro.\n\n## Claims\n\n### `rule: x`\n" + prose + "\n" + summary + "Origin: ADR-0001\n"
		parsed, err := topic.ParsePart(topic.TopicID{Domain: "alpha", Slug: "one"}, "part", []byte(part))
		if err != nil {
			t.Fatal(err)
		}
		return parsed.Claims[0]
	}
	declared := strings.Repeat("é", 160)
	if got := claimSummary(parse("Summary: "+declared+"\n", "ignored fallback")); got != declared {
		t.Fatalf("declared summary=%q", got)
	}
	cases := []struct {
		name  string
		prose string
		want  string
	}{
		{"fold first paragraph", "First\nline.\n\nSecond paragraph.", "First line."},
		{"ASCII 160 unchanged", strings.Repeat("a", 160), strings.Repeat("a", 160)},
		{"ASCII 161 hard cut", strings.Repeat("a", 161), strings.Repeat("a", 157) + "..."},
		{"Unicode 161 hard cut by code point", strings.Repeat("é", 161), strings.Repeat("é", 157) + "..."},
		{"truncate at word boundary", strings.Repeat("word ", 40), strings.TrimSpace(strings.Repeat("word ", 31)) + "..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := claimSummary(parse("", tc.prose))
			if got != tc.want || len([]rune(got)) > 160 {
				t.Errorf("runes=%d got=%q want=%q", len([]rune(got)), got, tc.want)
			}
		})
	}
}

// invariant: tooling/context-and-topic:context-concise-projection
// invariant: tooling/context-and-topic:context-full-authority-packet
func TestProjectionHelpers(t *testing.T) {
	changes := []PendingChange{{ADR: "0004"}, {ADR: "0001"}, {ADR: "0002"}, {ADR: "0003"}}
	bounded := contextPending(changes, false)
	if bounded.OperationCount != 4 || bounded.AdditionalADRCount != 1 || len(bounded.Operations) != 0 {
		t.Fatal(bounded)
	}
	expanded := contextPending(changes, true)
	if len(expanded.Operations) != 4 {
		t.Fatal(expanded)
	}
	var corpus topic.Corpus
	if got := claimStateForOperation("remove", "d/t:x", "applied", corpus, nil); got != "historically-removed" {
		t.Fatal(got)
	}
	history := &topic.ClaimHistory{RemovedBy: &topic.ADRHistory{Number: "1"}}
	if got := claimStateForOperation("add", "d/t:x", "applied", corpus, history); got != "historically-removed" {
		t.Fatal(got)
	}
	if got := claimStateForOperation("add", "d/t:x", "remaining", corpus, nil); got != "not-yet-current" {
		t.Fatal(got)
	}
}

func TestContextFacetProjectionAndClosestCategory(t *testing.T) {
	files := ctxFiles()
	files["internal/foo/a_test.go"] = "package foo\n// invariant: alpha/one:tested\n// invariant: alpha/one:tested\n"
	files["internal/foo/b_test.go"] = "package foo\n// invariant: alpha/one:tested\n"
	files["internal/foo/c_test.go"] = "package foo\n// invariant: alpha/one:tested\n"
	files[".awf/topics/parts/alpha/one/current-state.md"] = "Intro.\n\n## Claims\n\n### `rule: order`\nOrder prose.\nSummary: Order summary.\nOrigin: ADR-0001\nReferences: core/g:everywhere\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n"
	p := csRepo(t, ctxConfig, files)
	ws, err := p.workingCurrentState()
	if err != nil {
		t.Fatal(err)
	}
	if got := claimStateForOperation("add", "alpha/one:order", "applied", ws.Loaded.Topics, nil); got != "active-current" {
		t.Fatal(got)
	}
	facets, _ := ParseContextFacets([]string{"all-rules", "evidence", "selectors", "references", "pending"}, false)
	res, err := p.ContextForOptions([]string{"internal/foo/x.go", "internal/foo/y_test.go"}, ContextOptions{Selection: SelectionExplicit, Facets: facets})
	if err != nil {
		t.Fatal(err)
	}
	var alpha TopicImpact
	for _, impact := range res.Topics {
		if impact.ID == "alpha/one" {
			alpha = impact
		}
	}
	if len(alpha.Direct) != 2 || len(alpha.Invariants) != 1 || len(alpha.Additional) != 0 || alpha.Selectors == nil {
		t.Fatalf("alpha=%#v", alpha)
	}
	if alpha.Direct[0].Summary != "Order summary." || len(alpha.Direct[1].Evidence) == 0 {
		t.Fatalf("direct=%#v", alpha.Direct)
	}
	if len(alpha.Direct[0].Outgoing) != 1 || alpha.Direct[0].Outgoing[0] != "core/g:everywhere" {
		t.Fatalf("refs=%#v", alpha.Direct[0])
	}
}
