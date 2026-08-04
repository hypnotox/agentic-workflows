package project

import (
	"bytes"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestSyncMutationPreservesRepeatedSpacesInPaths(t *testing.T) {
	mutation, err := SyncMutation(
		[]Backup{{Path: "old  path", Bak: "old  path.awf-bak", Index: true}},
		[]Change{{Path: "existing  output", Cause: "config"}, {Path: "new  output", Cause: "added"}},
		[]string{"obsolete  output"},
	)
	if err != nil {
		t.Fatal(err)
	}
	document, err := mutation.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const want = "status: completed\n\nmutation:\n  changes:\n    backups:\n      old  path to old  path.awf-bak\n    outputs:\n      changed existing  output (config)\n      added new  output\n    pruned:\n      obsolete  output\n  notes:\n    awf now generates old  path; retire any external generator for it\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != want {
		t.Fatalf("sync presentation = %q, want %q", out.String(), want)
	}
}
