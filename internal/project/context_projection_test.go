package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: tooling/context-and-topic:context-summary-projection
func TestClaimSummaryParserAndProjection(t *testing.T) {
	parse := func(metadata, prose string) (topic.Claim, error) {
		part := "Intro.\n\n## Claims\n\n### `rule: x`\n" + prose + "\n" + metadata + "Origin: ADR-0001\n"
		parsed, err := topic.ParsePart(topic.TopicID{Domain: "alpha", Slug: "one"}, "part", []byte(part))
		if err != nil {
			return topic.Claim{}, err
		}
		return parsed.Claims[0], nil
	}
	declared := strings.Repeat("é", 160)
	claim, err := parse("Summary: "+declared+"\n", "ignored")
	if err != nil || claimSummary(claim) != declared || len([]rune(claimSummary(claim))) != 160 {
		t.Fatalf("declared summary=%q err=%v", claimSummary(claim), err)
	}
	for name, metadata := range map[string]string{
		"out of order": "Origin: ADR-0001\nSummary: late\n",
		"over limit":   "Summary: " + strings.Repeat("é", 161) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			part := "Intro.\n\n## Claims\n\n### `rule: x`\nProse.\n" + metadata
			if name != "out of order" {
				part += "Origin: ADR-0001\n"
			}
			if _, err := topic.ParsePart(topic.TopicID{Domain: "alpha", Slug: "one"}, "part", []byte(part)); err == nil {
				t.Fatal("accepted invalid Summary metadata")
			}
		})
	}
	cases := []struct {
		prose string
		want  string
	}{
		{"First\nline.\n\nSecond.", "First line."},
		{strings.Repeat("a", 160), strings.Repeat("a", 160)},
		{strings.Repeat("a", 161), strings.Repeat("a", 157) + "..."},
		{strings.Repeat("word ", 40), strings.TrimSpace(strings.Repeat("word ", 31)) + "..."},
	}
	for _, tc := range cases {
		claim, err := parse("", tc.prose)
		if err != nil {
			t.Fatal(err)
		}
		if got := claimSummary(claim); got != tc.want {
			t.Errorf("len=%d got=%q want=%q", len([]rune(got)), got, tc.want)
		}
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
