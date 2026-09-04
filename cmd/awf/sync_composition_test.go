package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestRunSyncEntryPointsRejectMalformedRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := testContext(t)
	if err := runSync(ctx, root, io.Discard); err == nil || errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("malformed repository error = %v", err)
	}
}

func TestRunSyncPrintingUsesInjectedLoader(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if _, err := os.Stat(config.LockPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock stat error = %v, want not exist", err)
	}
	if err := initializeProject(ctx, root, io.Discard); err != nil {
		t.Fatal(err)
	}
	var loadPaths []string
	loader := project.NewLoader(func(path string) (*config.Config, error) {
		loadPaths = append(loadPaths, path)
		assertProjectLeaseHeld(t, root)
		return config.Load(path)
	}, catalog.Standard, func(_ context.Context, got string) string { return got }, mustOpenGit(t, root))
	writer := &releasedLeaseAssertingWriter{t: t, root: root}
	if err := runSyncPrinting(ctx, loader, root, writer); err != nil {
		t.Fatal(err)
	}
	if !writer.called {
		t.Fatal("result presentation did not run under the project lease")
	}
	lease, err := filesystem.AcquireProjectLease(ctx, root, root)
	if err != nil {
		t.Fatalf("lease not released after outcome: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	want := config.RootDir(root)
	if len(loadPaths) != 1 || loadPaths[0] != want {
		t.Fatalf("config load paths = %v, want [%q]", loadPaths, want)
	}
}

type releasedLeaseAssertingWriter struct {
	t      *testing.T
	root   string
	called bool
}

func (w *releasedLeaseAssertingWriter) Write(payload []byte) (int, error) {
	w.called = true
	lease, err := filesystem.AcquireProjectLease(context.Background(), w.root, w.root)
	if err != nil {
		w.t.Fatalf("project lease remained held during presentation: %v", err)
	}
	if err := lease.Release(); err != nil {
		w.t.Fatal(err)
	}
	return len(payload), nil
}

func TestFinishSyncPrintingReturnsCommandOwnedDiagnosticOnReleaseFailure(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loader.Load(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := composePublisher(state).SyncLeased(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("release failed")
	var out bytes.Buffer
	err = finishSyncPrinting(&out, result, nil, want)
	var diagnostic interface {
		Diagnostic() (presentation.Diagnostic, error)
	}
	if !errors.Is(err, want) || !errors.As(err, &diagnostic) {
		t.Fatalf("release outcome = %v, want command diagnostic preserving release failure", err)
	}
	if out.Len() != 0 {
		t.Fatalf("release stdout = %q, want no false success", out.String())
	}
	doc, diagnosticErr := diagnostic.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	document, err := doc.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	if got := rendered.String(); !strings.Contains(got, "touched path: AGENTS.md") || strings.Contains(strings.ToLower(got), "recovery") {
		t.Fatalf("diagnostic = %q", got)
	}
}

func assertProjectLeaseHeld(t *testing.T, root string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if lease, err := filesystem.AcquireProjectLease(ctx, root, root); !errors.Is(err, context.DeadlineExceeded) {
		if lease != nil {
			_ = lease.Release()
		}
		t.Fatalf("project lease was not held: %v", err)
	}
}

func TestRunSyncKeepsFixedSkillCatalog(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)

	testsupport.WriteAwfConfig(t, root, minimalYAML)
	var out bytes.Buffer
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	const expected = "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != expected {
		t.Errorf("selection-free sync bytes = %q, want %q", out.String(), expected)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "awf-maintenance", "SKILL.md")); err != nil {
		t.Fatalf("fixed catalog skill was pruned on a clean sync: %v", err)
	}
	// A drift-clean re-sync emits the complete empty-success document.
	out.Reset()
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n" {
		t.Errorf("empty sync bytes = %q", got)
	}
}

func TestRunSyncPrintsChangedFiles(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	// A var edit moves the config hash of every artifact referencing it; the
	// re-sync attributes the changed output to the project's own inputs.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: ./x gate", 1))
	var out bytes.Buffer
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"changed AGENTS.md (config)",
		"changed docs/workflow.md (config)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("config change did not update full-catalog output %q:\n%s", want, out.String())
		}
	}
	// A drift-clean re-sync emits the complete empty-success document.
	out.Reset()
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n" {
		t.Errorf("empty sync bytes = %q", got)
	}
	// Enabling an artifact reports its files as added.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: ./x gate", 1)+"")
	out.Reset()
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	const selectionIgnored = "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != selectionIgnored {
		t.Errorf("docs selection-free sync bytes = %q, want %q", out.String(), selectionIgnored)
	}
}
