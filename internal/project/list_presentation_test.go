package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestEnablementPresentationOwnersRenderBareEntries(t *testing.T) {
	plan, err := PlanDocument([]PlanOp{{Node: catalog.Node{Kind: "skill", Name: "reviewing-impl"}, Enable: true}, {Node: catalog.Node{Kind: "agent", Name: "code-reviewer"}, Enable: true, RequiredBy: "reviewing-impl"}})
	if err != nil {
		t.Fatal(err)
	}
	notes, err := EnablementNotesDocument([]string{"agent remains enabled"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		document presentation.Document
		want     string
	}{
		{document: plan, want: "status: enablement plan\n\ncollection:\n  plan operations:\n    + skill reviewing-impl\n    + agent code-reviewer (required by reviewing-impl)\n"},
		{document: notes, want: "status: enablement notes\n\ncollection:\n  notes:\n    agent remains enabled\n"},
	} {
		var out strings.Builder
		if err := presentation.Render(&out, test.document); err != nil {
			t.Fatal(err)
		}
		if out.String() != test.want {
			t.Fatalf("document = %q, want %q", out.String(), test.want)
		}
	}
}

func TestEnablementPresentationOwnersRejectInvalidEntries(t *testing.T) {
	if _, err := PlanDocument([]PlanOp{{Node: catalog.Node{Kind: "skill", Name: "bad\nname"}, Enable: true}}); err == nil {
		t.Fatal("invalid plan entry accepted")
	}
	if _, err := EnablementNotesDocument([]string{"bad\nnote"}); err == nil {
		t.Fatal("invalid enablement note accepted")
	}
	if _, err := listCategory("items", []string{"bad\nitem"}); err == nil {
		t.Fatal("invalid list entry accepted")
	}
}
