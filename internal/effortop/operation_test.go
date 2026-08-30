package effortop

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// TestRunCoordinatesResidentAndTopologyOwners proves that the operation owns
// the default-new transaction while effort retains resident publication and
// worktree retains managed topology creation.
func TestRunCoordinatesResidentAndTopologyOwners(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	fixture := gitfixture.InitNativeAt(t, root)
	if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fixture, "tracked")
	gitfixture.NativeCommit(t, fixture, "base")
	for _, path := range []string{"efforts", "worktrees", "effort-archive"} {
		if err := os.MkdirAll(filepath.Join(root, ".awf", path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "effort-archive", ".gitignore"), []byte("*\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	roots, err := awfgit.ResolveControlRoots(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := awfgit.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:     func() time.Time { return time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC) },
		UUID:      func() (string, error) { return "018f47a0-7b3d-4c52-8f1a-123456789abc", nil },
		Worktrees: repo.WorktreeList, BranchExists: repo.BranchExists, ValidateRef: repo.ValidateRefName,
		ExpectedArchiveMarker: func() ([]byte, error) { return []byte("*\n"), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := worktree.Open(roots, func(path string) (worktree.Runner, error) { return awfgit.Open(path) }, func(name awfgit.ResidentName) (worktree.ResidentHandle, error) {
		root, rootErr := roots.ResidentRoot(name)
		if rootErr != nil {
			return nil, rootErr
		}
		return filesystem.Open(root)
	}, service)
	if err != nil {
		t.Fatal(err)
	}
	document, err := New(ctx, service, manager, effort.NewInput{Slug: "operation", Title: "Operation owned new"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "managed worktree added for operation") {
		t.Fatalf("document=%q", rendered.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "efforts", "operation", "state.json")); err != nil {
		t.Fatalf("resident: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "worktrees", "operation")); err != nil {
		t.Fatalf("topology: %v", err)
	}
}

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
