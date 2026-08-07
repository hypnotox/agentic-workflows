package initspec

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestOutcomeOwnsCompleteInitializationPresentation(t *testing.T) {
	syncNote, _ := presentation.Prose("sync note")
	syncAction, _ := presentation.Prose("continue")
	document, err := (Outcome{
		ConfigPath:     ".awf/config.yaml",
		ExistingConfig: true,
		IgnoredAnswers: true,
		Sync:           presentation.Mutation{Notes: []presentation.Value{syncNote}, NextActions: []presentation.Value{syncAction}},
		Advisories:     []string{"advisory"},
		NextActions:    []string{"commit changes"},
	}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const want = "status: initialization completed\n\nmutation:\n  identity:\n    config: .awf/config.yaml\n    config action: kept and re-rendered\n  notes:\n    sync note\n    --set/--answers values were ignored; edit .awf/config.yaml instead\n    advisory\n  next actions:\n    step 1: continue\n    step 2: commit changes\n"
	if out.String() != want {
		t.Fatalf("outcome = %q, want %q", out.String(), want)
	}
}

func TestOutcomeRejectsInvalidOwnedValues(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome Outcome
	}{
		{name: "config path", outcome: Outcome{ConfigPath: "bad\npath"}},
		{name: "advisory", outcome: Outcome{ConfigPath: "config", Advisories: []string{" \n\t"}}},
		{name: "next action", outcome: Outcome{ConfigPath: "config", NextActions: []string{" \n\t"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.outcome.Document(); err == nil {
				t.Fatal("invalid outcome accepted")
			}
		})
	}
}
