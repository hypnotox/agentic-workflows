package topic

import (
	"strings"
	"testing"
)

func TestRenderModelsAndTemplates(t *testing.T) {
	topics := []Topic{{ID: TopicID{"d", "z"}, Metadata: Metadata{Title: "Beta", Summary: "Second.", Paths: []string{"x/**"}}, Part: "<!-- awf:comment -->\nAuthored {{ .raw }}.\n<!-- awf:endcomment -->\n## Claims\n"}, {ID: TopicID{"d", "a"}, Metadata: Metadata{Title: "Alpha", Summary: "First.", Applies: "global"}, Part: "## Claims\n"}, {ID: TopicID{"d", "c"}, Metadata: Metadata{Title: "Same", Summary: "A."}}, {ID: TopicID{"d", "b"}, Metadata: Metadata{Title: "Same", Summary: "A."}}, {ID: TopicID{"d", "d"}, Metadata: Metadata{Title: "Same", Summary: "B."}}}
	idx := BuildIndexModel("d", topics)
	if idx.Topics[0].Title != "Alpha" || idx.Topics[0].Link != "a.md" {
		t.Fatalf("%#v", idx)
	}
	nav := BuildNavigationModel("d", topics)
	if nav.IndexLink != "../topics/d/index.md" || nav.Topics[0].Link != "../topics/d/a.md" {
		t.Fatalf("%#v", nav)
	}
	out, err := RenderIndex(idx)
	if err != nil || !strings.Contains(out, "[Alpha](a.md)") {
		t.Fatalf("%s %v", out, err)
	}
	empty, err := RenderIndex(BuildIndexModel("d", nil))
	if err != nil || !strings.Contains(empty, "No current-state topics") {
		t.Fatalf("%s %v", empty, err)
	}
	model := BuildTopicModel(topics[0], []string{"x/pkg/**"}, MarkerIndex{}, []string{"x/pkg/a.go"})
	out, err = RenderTopic(model)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "awf:comment") || !strings.Contains(out, "Authored {{ .raw }}.") || !strings.Contains(out, "Both domain and topic selectors must match") {
		t.Fatalf("%s", out)
	}
	if strings.HasSuffix(out, "\n\n") || !strings.HasSuffix(out, "\n") {
		t.Fatalf("topic output must end in exactly one newline: %q", out)
	}
	global := BuildTopicModel(topics[1], nil, MarkerIndex{}, nil)
	if !strings.Contains(global.Applicability, "Global") {
		t.Fatal(global.Applicability)
	}
	shell := topics[0]
	model = BuildTopicModel(shell, nil, MarkerIndex{}, nil)
	if !strings.Contains(model.Applicability, "Both domain and topic selectors must match") {
		t.Fatal(model.Applicability)
	}
}

// TestApplicabilitySummarySelectorsOnly proves the rendered applicability
// paragraph carries only selectors, the both-must-match rule (or the global
// variant), and the coverage drilldown: never the matched-path census or marker
// sites, with an empty selector list degrading to coherent prose.
// invariant: invariants/topics-and-markers:rendered-applicability-selectors-only
func TestApplicabilitySummarySelectorsOnly(t *testing.T) {
	scoped := Topic{ID: TopicID{"d", "z"}, Metadata: Metadata{Title: "Beta", Summary: "Second.", Paths: []string{"x/**"}}}
	markers := MarkerIndex{sites: map[string][]MarkerSite{"d/z:claim": {{Path: "x/pkg/a.go", Line: 3, Kind: ProofMarker, ClaimID: "d/z:claim"}}}}
	model := BuildTopicModel(scoped, []string{"x/pkg/**"}, markers, []string{"x/pkg/a.go"})
	if !strings.Contains(model.Applicability, "Owning domain selectors: `x/pkg/**`.") ||
		!strings.Contains(model.Applicability, "Topic selectors: `x/**`.") ||
		!strings.Contains(model.Applicability, "Both domain and topic selectors must match.") ||
		!strings.Contains(model.Applicability, "Run `awf topic d/z --coverage` for current matched paths and marker sites.") {
		t.Fatalf("selectors-only form missing: %s", model.Applicability)
	}
	if strings.Contains(model.Applicability, "Current matched paths") || strings.Contains(model.Applicability, "Marker sites") || strings.Contains(model.Applicability, "x/pkg/a.go") {
		t.Fatalf("census leaked into rendered applicability: %s", model.Applicability)
	}
	global := BuildTopicModel(Topic{ID: TopicID{"d", "g"}, Metadata: Metadata{Title: "G", Summary: "G.", Applies: "global"}}, []string{"x/**"}, MarkerIndex{}, nil)
	if !strings.Contains(global.Applicability, "Global topic within owning domain selectors `x/**`.") || !strings.Contains(global.Applicability, "Run `awf topic d/g --coverage`") {
		t.Fatalf("global variant = %s", global.Applicability)
	}
	empty := BuildTopicModel(Topic{ID: TopicID{"d", "e"}, Metadata: Metadata{Title: "E", Summary: "E."}}, nil, MarkerIndex{}, nil)
	if !strings.Contains(empty.Applicability, "Owning domain selectors: none.") || !strings.Contains(empty.Applicability, "Topic selectors: none.") || strings.Contains(empty.Applicability, "``") {
		t.Fatalf("empty selectors did not degrade to coherent prose: %s", empty.Applicability)
	}
}
func TestRenderTopicTrimsTrailingNewlineFraming(t *testing.T) {
	out, err := RenderTopic(TopicRenderModel{
		Title:         "Title",
		Summary:       "Summary.",
		Applicability: "Scope.",
		Part:          "Body.\r\n\r\n\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "# Title\n\nSummary.\n\n**Applicability:** Scope.\n\nBody.\n"
	if out != want {
		t.Fatalf("RenderTopic bytes = %q, want %q", out, want)
	}
}

func TestRenderErrors(t *testing.T) {
	if _, err := RenderTopic(TopicRenderModel{Part: "<!-- awf:comment no close\n"}); err == nil {
		t.Fatal("malformed comment accepted")
	}
	if _, err := templateSource("topics/missing.tmpl"); err == nil {
		t.Fatal("missing template accepted")
	}
	if _, err := executeRaw("topics/missing.tmpl", nil, "raw"); err == nil {
		t.Fatal("raw missing template accepted")
	}
	if _, err := execute("topics/missing.tmpl", nil); err == nil {
		t.Fatal("missing execute template accepted")
	}
	if _, err := execute("topics/topic.md.tmpl", map[string]any{"Title": func() {}}); err == nil {
		t.Fatal("execute error expected")
	}
}
