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
	model := BuildTopicModel(topics[0], []string{"x/pkg/**"}, MarkerIndex{})
	out, err = RenderTopic(model)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "awf:comment") || !strings.Contains(out, "Authored {{ .raw }}.") || !strings.Contains(out, "within domain") {
		t.Fatalf("%s", out)
	}
	if strings.HasSuffix(out, "\n\n") || !strings.HasSuffix(out, "\n") {
		t.Fatalf("topic output must end in exactly one newline: %q", out)
	}
	global := BuildTopicModel(topics[1], nil, MarkerIndex{})
	if !strings.Contains(global.Applicability, "Global") {
		t.Fatal(global.Applicability)
	}
	shell := topics[0]
	model = BuildTopicModel(shell, nil, MarkerIndex{})
	if !strings.Contains(model.Applicability, "No effective") {
		t.Fatal(model.Applicability)
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
