package publisher

import (
	"bytes"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestSyncMutationRejectsLineBreaksInLiteralPaths(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(string) error
	}{
		{
			name: "backup path",
			run: func(invalid string) error {
				_, err := syncMutation([]Backup{{Path: "old" + invalid + "path", Bak: "old.awf-bak"}}, nil, nil)
				return err
			},
		},
		{
			name: "backup destination",
			run: func(invalid string) error {
				_, err := syncMutation([]Backup{{Path: "old-path", Bak: "old" + invalid + ".awf-bak"}}, nil, nil)
				return err
			},
		},
		{
			name: "change path",
			run: func(invalid string) error {
				_, err := syncMutation(nil, []Change{{Path: "output" + invalid + ".md", Cause: "added"}}, nil)
				return err
			},
		},
		{
			name: "pruned path",
			run: func(invalid string) error {
				_, err := syncMutation(nil, nil, []string{"old" + invalid + ".md"})
				return err
			},
		},
		{
			name: "indexed ownership note",
			run: func(invalid string) error {
				_, err := syncMutation([]Backup{{Path: "INDEX" + invalid + ".md", Bak: "INDEX.awf-bak", Index: true}}, nil, nil)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, invalid := range []string{"\n", "\r"} {
				if err := test.run(invalid); err == nil || err.Error() != "presentation value contains a line break" {
					t.Fatalf("syncMutation(%q) error = %v", invalid, err)
				}
			}
		})
	}
}

func TestSyncMutationPreservesRepeatedSpacesInPaths(t *testing.T) {
	mutation, err := syncMutation(
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
