package effortop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

func TestIntegrationGateCommandUsesOnlyAConfiguredString(t *testing.T) {
	for _, test := range []struct {
		name       string
		configYAML string
		want       string
		wantErr    bool
	}{
		{name: "absent config"},
		{name: "configured", configYAML: "prefix: example\nvars: {gateCmd: make gate}\n", want: "make gate"},
		{name: "blank", configYAML: "prefix: example\nvars: {gateCmd: \"  \"}\n"},
		{name: "non-string", configYAML: "prefix: example\nvars: {gateCmd: [make, gate]}\n"},
		{name: "malformed", configYAML: "unknown: value\n", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.configYAML != "" {
				if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(test.configYAML), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := integrationGateCommand(root)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("integrationGateCommand() = %q, %v; want %q, error=%v", got, err, test.want, test.wantErr)
			}
			if test.wantErr {
				if _, err := Integrate(context.Background(), root, nil, "any-effort"); err == nil {
					t.Fatal("integrate did not propagate malformed gate configuration")
				}
			}
		})
	}
}

func TestPresentationHelpersRejectInvalidSemanticResults(t *testing.T) {
	if _, err := newDocument(effort.Record{}, worktree.Result{}); err == nil {
		t.Fatal("new document accepted an empty worktree result")
	}
	if _, err := newDocument(effort.Record{}, worktree.Result{Condition: "done", NextAction: "continue"}); err == nil {
		t.Fatal("new document accepted an empty effort record")
	}
	if _, err := worktreeDocument(worktree.Result{}, nil); err == nil {
		t.Fatal("worktree document accepted an empty result")
	}
}
