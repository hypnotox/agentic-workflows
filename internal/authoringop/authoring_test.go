package authoringop

import (
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

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func transactionFixture(t *testing.T, local bool) (string, *project.Loader) {
	t.Helper()
	root := t.TempDir()
	localConfig := ""
	if local {
		localConfig = "localDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n"
	}
	writeFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate}\n"+localConfig)
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
	state, cfg, err := loader.OpenForOperation(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(root), project.Version).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatal(err)
	}
	return root, loader
}

func bytesAt(t *testing.T, root, rel string) []byte {
	t.Helper()
	bytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

// invariant: rendering/sync-and-drift:authoring-sync-transaction (TestPartAuthoringValidatesBeforeSourceAndSynchronizesCommittedAuthority)
func TestPartAuthoringValidatesBeforeSourceAndSynchronizesCommittedAuthority(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	request := Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("authored body\n")}
	lockBefore := bytesAt(t, root, ".awf/awf.lock")
	outcome, err := Run(context.Background(), root, request, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Source != SourceCreated || !outcome.Publisher.HasCommittedEffects() {
		t.Fatalf("outcome = %#v", outcome)
	}
	if got := string(bytesAt(t, root, outcome.SourcePath)); got != "authored body\n" {
		t.Fatalf("source = %q", got)
	}
	if string(bytesAt(t, root, ".awf/awf.lock")) == string(lockBefore) {
		t.Fatal("successful authoring did not update lock")
	}

	// Explicit empty content remains an authored override rather than reset.
	outcome, err = Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte{}}, loader, nil)
	if err != nil || outcome.Source != SourceReplaced {
		t.Fatalf("empty edit outcome=%#v err=%v", outcome, err)
	}
	if got := bytesAt(t, root, outcome.SourcePath); len(got) != 0 {
		t.Fatalf("empty override = %q", got)
	}

	outcome, err = Run(context.Background(), root, Request{Mode: Reset, Kind: "skill", Name: "tdd", Part: section}, loader, nil)
	if err != nil || outcome.Source != SourceRemoved {
		t.Fatalf("reset outcome=%#v err=%v", outcome, err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(outcome.SourcePath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reset retained source: %v", err)
	}
}

func TestInvalidCandidateLeavesSourceOutputAndLockUnchanged(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	output := ".claude/skills/example-tdd/SKILL.md"
	beforeOutput := bytesAt(t, root, output)
	beforeLock := bytesAt(t, root, ".awf/awf.lock")
	_, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("{{=awf:notDeclared}}\n")}, loader, nil)
	if err == nil || !strings.Contains(err.Error(), "candidate") {
		t.Fatalf("invalid candidate error = %v", err)
	}
	part := filepath.Join(root, ".awf/skills/parts/tdd", section+".md")
	if _, statErr := os.Stat(part); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid candidate changed source: %v", statErr)
	}
	if string(bytesAt(t, root, output)) != string(beforeOutput) || string(bytesAt(t, root, ".awf/awf.lock")) != string(beforeLock) {
		t.Fatal("invalid candidate changed output or lock")
	}
}

// invariant: rendering/inplace-and-placeholders:local-doc-body-inline (TestLocalDocumentBodyEditAndResetPreserveShell)
func TestLocalDocumentBodyEditAndResetPreserveShell(t *testing.T) {
	root, loader := transactionFixture(t, true)
	path := "docs/runbooks/incident.md"
	original := bytesAt(t, root, path)
	outcome, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "doc", Name: "runbooks/incident", Part: "body", Content: []byte("operator body\n\nexact")}, loader, nil)
	if err != nil || outcome.Source != SourceLocalBody {
		t.Fatalf("local edit outcome=%#v err=%v", outcome, err)
	}
	edited := bytesAt(t, root, path)
	if !strings.Contains(string(edited), "operator body\n\nexact") || !strings.Contains(string(edited), "# Incident") || !strings.Contains(string(edited), "awf:edit-in-place body") {
		t.Fatalf("edited local document = %q", edited)
	}
	_, err = Run(context.Background(), root, Request{Mode: Reset, Kind: "doc", Name: "runbooks/incident", Part: "body"}, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	reset := bytesAt(t, root, path)
	if strings.Contains(string(reset), "operator body") || !strings.Contains(string(reset), "# Incident") || !strings.Contains(string(reset), "awf:edit-in-place body") {
		t.Fatalf("reset local document = %q", reset)
	}
	if !strings.Contains(string(original), "# Incident") {
		t.Fatalf("fixture shell = %q", original)
	}
}

func TestLeaseReleaseFaultRetainsEffectsAndCause(t *testing.T) {
	root, loader := transactionFixture(t, false)
	section := catalog.Standard.Skills["tdd"].Sections[0]
	releaseFault := errors.New("release sentinel")
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		lease, err := loader.AcquireProjectLease(ctx, root)
		if err != nil {
			return nil, nil, err
		}
		return lease, func() error { return errors.Join(lease.Release(), releaseFault) }, nil
	}
	outcome, err := Run(context.Background(), root, Request{Mode: Edit, Kind: "skill", Name: "tdd", Part: section, Content: []byte("body")}, loader, acquire)
	var partial *PartialError
	if !errors.As(err, &partial) || !errors.Is(err, releaseFault) {
		t.Fatalf("release error = %v", err)
	}
	if outcome.Source == SourceNone || !outcome.Publisher.HasCommittedEffects() || partial.Outcome.Source != outcome.Source {
		t.Fatalf("release partial lost effects: outcome=%#v partial=%#v", outcome, partial)
	}
	if _, documentErr := partial.Document(); documentErr != nil {
		t.Fatalf("partial document error=%v", documentErr)
	}
}
