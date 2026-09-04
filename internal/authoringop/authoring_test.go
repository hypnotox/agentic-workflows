package authoringop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
func bytesAt(t *testing.T, root, rel string) []byte {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func transactionFixture(t *testing.T, local bool) (string, *project.Loader) {
	t.Helper()
	root := t.TempDir()
	extra := ""
	if local {
		extra = "localDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n"
	}
	writeFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate}\n"+extra)
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
	state, err := loader.Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.New(state, project.Version).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatal(err)
	}
	return root, loader
}

// invariant: tooling/filesystem-access:focused-project-mutations-visible (TestRunValidatesBeforeMutationAndSynchronizesFreshAuthority)
func TestRunValidatesBeforeMutationAndSynchronizesFreshAuthority(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["awf-maintenance"].Sections[0]
	request := Request{Mode: Edit, Kind: "skill", Name: "awf-maintenance", Part: section, Content: []byte("authored body\n")}
	outcome, err := Run(context.Background(), root, request, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Source != SourceCreated || len(outcome.Publisher.Changes()) == 0 {
		t.Fatalf("outcome = %#v", outcome)
	}
	if got := string(bytesAt(t, root, outcome.SourcePath)); got != "authored body\n" {
		t.Fatalf("source = %q", got)
	}
	if !strings.Contains(string(bytesAt(t, root, ".claude/skills/awf-maintenance/SKILL.md")), "authored body") {
		t.Fatal("fresh source not rendered")
	}
	before := bytesAt(t, root, outcome.SourcePath)
	_, err = Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "awf-maintenance", Part: section, Content: []byte("{{=awf:notDeclared}}")}, loader, nil)
	if err == nil || !bytes.Equal(before, bytesAt(t, root, outcome.SourcePath)) {
		t.Fatalf("invalid candidate mutated source: %v", err)
	}
}

func TestPublisherFailureLeavesAuthoredSourceVisible(t *testing.T) {
	root, loader := transactionFixture(t, false)
	blocked := filepath.Join(root, ".claude/skills/awf-maintenance/SKILL.md")
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	section := catalog.Standard.Skills["awf-maintenance"].Sections[0]
	outcome, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "awf-maintenance", Part: section, Content: []byte("visible source")}, loader, nil)
	if err == nil || outcome.Source != SourceCreated {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
	if got := string(bytesAt(t, root, outcome.SourcePath)); got != "visible source" {
		t.Fatalf("source=%q", got)
	}
}

func TestRunAcquiresLeaseBeforeAuthorityReadAndJoinsReleaseError(t *testing.T) {
	root, initializer := transactionFixture(t, false)
	acquired := false
	observing := false
	reads := 0
	loader := project.NewLoaderWithoutRepository(func(path string) (*config.Config, error) {
		if observing && !acquired {
			t.Fatal("authority read before lease")
		}
		if observing {
			reads++
		}
		return config.Load(path)
	}, catalog.Standard, func(context.Context, string) string { return root })
	fault := errors.New("release sentinel")
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		acquired = true
		lease, err := initializer.AcquireProjectLease(ctx, root)
		return lease, func() error { return errors.Join(lease.Release(), fault) }, err
	}
	observing = true
	section := catalog.Standard.Skills["awf-maintenance"].Sections[0]
	outcome, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "awf-maintenance", Part: section, Content: []byte("body")}, loader, acquire)
	if outcome.Source != SourceCreated || !errors.Is(err, fault) || reads != 2 {
		t.Fatalf("outcome=%#v reads=%d error=%v", outcome, reads, err)
	}
}
