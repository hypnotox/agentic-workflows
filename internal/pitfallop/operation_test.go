package pitfallop

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
)

const fixtureConfig = "prefix: example\nintegrationBranch: main\nvars: {}\n"

func fixture(t *testing.T) (string, *project.Loader) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(fixtureConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
	return root, loader
}

func transaction(t *testing.T, root string, loader *project.Loader) (*projectmutation.Transaction, *filesystem.Lease) {
	t.Helper()
	lease, err := filesystem.AcquireTrackedLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := projectmutation.UseTracked(context.Background(), root, loader, lease)
	if err != nil {
		t.Fatal(err)
	}
	return tx, lease
}

func runCreate(t *testing.T, root string, loader *project.Loader, title string) (Outcome, error) {
	t.Helper()
	tx, lease := transaction(t, root, loader)
	outcome, operationErr := Create(context.Background(), title, tx)
	return outcome, Finish(outcome, operationErr, lease.Release())
}

// invariant: tooling/cli:pitfall-scaffold (TestCreatePublishesOneAuthoredSourceWithoutRender)
func TestCreatePublishesOneAuthoredSourceWithoutRender(t *testing.T) {
	root, loader := fixture(t)
	generated := filepath.Join(root, "docs", "pitfalls.md")
	lock := filepath.Join(root, ".awf", "awf.lock")
	if err := os.MkdirAll(filepath.Dir(generated), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(generated, []byte("generated sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, []byte("lock sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := runCreate(t, root, loader, "Unicode + punctuation: 日本語")
	if err != nil {
		t.Fatal(err)
	}
	const relative = ".awf/docs/pitfalls/unicode-punctuation.md"
	if outcome.SourcePath != relative || outcome.ResiduePath != "" {
		t.Fatalf("outcome = %#v", outcome)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	const want = "---\ntitle: 'Unicode + punctuation: 日本語'\n---\nDescribe the durable hazard, its consequence, and the safer practice.\n"
	if string(raw) != want {
		t.Fatalf("source = %q, want %q", raw, want)
	}
	for path, want := range map[string]string{generated: "generated sentinel\n", lock: "lock sentinel\n"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("unselected output %s = %q, %v", path, got, err)
		}
	}
	var rendered bytes.Buffer
	document, err := outcome.Document()
	if err != nil {
		t.Fatal(err)
	}
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	if rendered.String() != "status: pitfall created\nauthored path: "+relative+"\n" {
		t.Fatalf("document = %q", rendered.String())
	}
}

func TestCreateRefusesMalformedDuplicateAndSelectsFirstSuffixGap(t *testing.T) {
	root, loader := fixture(t)
	dir := filepath.Join(root, ".awf", "docs", "pitfalls")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte("malformed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCreate(t, root, loader, "New"); err == nil || !strings.Contains(err.Error(), "missing frontmatter") {
		t.Fatalf("malformed corpus error = %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "bad.md")); err != nil {
		t.Fatal(err)
	}
	for name, title := range map[string]string{"hazard.md": "Other", "hazard-2.md": "Another", "hazard-4.md": "Fourth"} {
		source := "---\ntitle: " + title + "\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, title := range []string{"", "index", "日本語"} {
		if _, err := runCreate(t, root, loader, title); err == nil {
			t.Fatalf("title %q accepted", title)
		}
	}
	outcome, err := runCreate(t, root, loader, "Hazard")
	if err != nil || outcome.SourcePath != ".awf/docs/pitfalls/hazard-3.md" {
		t.Fatalf("suffix outcome = %#v, %v", outcome, err)
	}
	if _, err := runCreate(t, root, loader, " hazard "); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("duplicate error = %v", err)
	}
}

type faultFilesystem struct {
	scaffoldFilesystem
	readErr, walkErr, mkdirErr error
	publish                    func(string, []byte, fs.FileMode) error
}

func (f *faultFilesystem) Read(path string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.scaffoldFilesystem.Read(path)
}
func (f *faultFilesystem) Walk(path string, visit func(string, fs.FileInfo) (bool, error)) error {
	if f.walkErr != nil {
		return f.walkErr
	}
	return f.scaffoldFilesystem.Walk(path, visit)
}
func (f *faultFilesystem) MkdirAll(path string, mode fs.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	return f.scaffoldFilesystem.MkdirAll(path, mode)
}
func (f *faultFilesystem) Publish(path string, source []byte, mode fs.FileMode) error {
	if f.publish != nil {
		return f.publish(path, source, mode)
	}
	return f.scaffoldFilesystem.Publish(path, source, mode)
}

func openTree(t *testing.T, root string) *filesystem.Handle {
	t.Helper()
	tree, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tree.Close() })
	return tree
}

func TestCreatePropagatesCorpusAndDirectoryFailures(t *testing.T) {
	t.Run("source root is a file", func(t *testing.T) {
		root, loader := fixture(t)
		path := filepath.Join(root, ".awf", "docs", "pitfalls")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := runCreate(t, root, loader, "New"); err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("source-root error = %v", err)
		}
	})
	t.Run("nested source", func(t *testing.T) {
		root, loader := fixture(t)
		nested := filepath.Join(root, ".awf", "docs", "pitfalls", "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nested, "bad.md"), []byte("---\ntitle: Bad\n---\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := runCreate(t, root, loader, "New"); err == nil || !strings.Contains(err.Error(), "direct child") {
			t.Fatalf("nested-source error = %v", err)
		}
	})
	t.Run("injected read walk and mkdir causes", func(t *testing.T) {
		root, _ := fixture(t)
		tree := openTree(t, root)
		if err := tree.MkdirAll(".awf/docs/pitfalls", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := tree.Publish(".awf/docs/pitfalls/a.md", []byte("---\ntitle: A\n---\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		readFault := errors.New("read sentinel")
		if _, err := createConfined("New", &faultFilesystem{scaffoldFilesystem: tree, readErr: readFault}); !errors.Is(err, readFault) {
			t.Fatalf("read cause = %v", err)
		} else if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhaseAuthority {
			t.Fatalf("read phase = %q, %t", phase, ok)
		}
		walkFault := errors.New("walk sentinel")
		if _, err := createConfined("New", &faultFilesystem{scaffoldFilesystem: tree, walkErr: walkFault}); !errors.Is(err, walkFault) {
			t.Fatalf("walk cause = %v", err)
		} else if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhaseAuthority {
			t.Fatalf("walk phase = %q, %t", phase, ok)
		}
		if err := tree.Remove(".awf/docs/pitfalls/a.md"); err != nil {
			t.Fatal(err)
		}
		if err := tree.Remove(".awf/docs/pitfalls"); err != nil {
			t.Fatal(err)
		}
		mkdirFault := errors.New("mkdir sentinel")
		if _, err := createConfined("New", &faultFilesystem{scaffoldFilesystem: tree, mkdirErr: mkdirFault}); !errors.Is(err, mkdirFault) {
			t.Fatalf("mkdir cause = %v", err)
		} else if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhasePublication {
			t.Fatalf("mkdir phase = %q, %t", phase, ok)
		}
	})
}

func TestCreatePreservesExclusiveRaceAndRecomputesOnRetry(t *testing.T) {
	root, _ := fixture(t)
	tree := openTree(t, root)
	raced := false
	faults := &faultFilesystem{scaffoldFilesystem: tree}
	faults.publish = func(path string, source []byte, mode fs.FileMode) error {
		if !raced {
			raced = true
			if err := tree.Publish(path, []byte("---\ntitle: Competing writer\n---\nbody\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return tree.Publish(path, source, mode)
	}
	if _, err := createConfined("Race", faults); !errors.Is(err, os.ErrExist) {
		t.Fatalf("race error = %v", err)
	} else if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhasePublication {
		t.Fatalf("race phase = %q, %t", phase, ok)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/race-2.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("race silently advanced suffix: %v", err)
	}
	outcome, err := createConfined("Race", faults)
	if err != nil || outcome.SourcePath != ".awf/docs/pitfalls/race-2.md" {
		t.Fatalf("retry outcome = %#v, %v", outcome, err)
	}
}

func TestCreatePrecommitFailureLeavesDestinationAbsent(t *testing.T) {
	root, _ := fixture(t)
	tree := openTree(t, root)
	fault := errors.New("publication preparation failed")
	faults := &faultFilesystem{scaffoldFilesystem: tree, publish: func(string, []byte, fs.FileMode) error { return fault }}
	outcome, err := createConfined("Unpublished", faults)
	if !errors.Is(err, fault) || committed(outcome) {
		t.Fatalf("precommit outcome = %#v, %v", outcome, err)
	}
	if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhasePublication {
		t.Fatalf("publication phase = %q, %t", phase, ok)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/unpublished.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed publication left destination: %v", err)
	}
}

func TestCommittedCleanupResidueIsTypedAndRetryNeverAdvances(t *testing.T) {
	root, _ := fixture(t)
	tree := openTree(t, root)
	cleanupFault := errors.New("persistent cleanup failure")
	const sourcePath = ".awf/docs/pitfalls/committed.md"
	const residuePath = ".awf/docs/pitfalls/.filepublication-injected.tmp"
	faults := &faultFilesystem{scaffoldFilesystem: tree}
	faults.publish = func(path string, source []byte, mode fs.FileMode) error {
		if err := tree.Publish(path, source, mode); err != nil {
			return err
		}
		if err := tree.Publish(residuePath, []byte("temporary"), 0o600); err != nil {
			return err
		}
		return &filepublication.CommittedCleanupError{DestinationPath: path, ResiduePath: residuePath, Cause: cleanupFault}
	}
	outcome, err := createConfined("Committed", faults)
	var partial *PartialError
	var cleanup *filepublication.CommittedCleanupError
	if !errors.As(err, &partial) || !errors.As(err, &cleanup) || !errors.Is(err, cleanupFault) {
		t.Fatalf("cleanup error = %#v, %v", partial, err)
	}
	if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhaseCleanup {
		t.Fatalf("cleanup phase = %q, %t", phase, ok)
	}
	if outcome.SourcePath != sourcePath || outcome.ResiduePath != residuePath {
		t.Fatalf("cleanup outcome = %#v", outcome)
	}
	var rendered bytes.Buffer
	document, documentErr := partial.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{sourcePath, residuePath, "as committed", "do not rerun awf new pitfall"} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("partial document missing %q:\n%s", want, rendered.String())
		}
	}
	if _, retryErr := createConfined("Committed", tree); retryErr == nil || !strings.Contains(retryErr.Error(), residuePath) {
		t.Fatalf("retry ignored residue: %v", retryErr)
	}
	if err := tree.Remove(residuePath); err != nil {
		t.Fatal(err)
	}
	if _, retryErr := createConfined("Committed", tree); retryErr == nil || !strings.Contains(retryErr.Error(), "duplicates") {
		t.Fatalf("retry ignored committed title: %v", retryErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/committed-2.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("retry allocated suffix: %v", statErr)
	}
}

func TestFinishRetainsCleanupResidueAndReleaseRecovery(t *testing.T) {
	root, _ := fixture(t)
	tree := openTree(t, root)
	cleanupFault := errors.New("cleanup sentinel")
	releaseFault := errors.New("release sentinel")
	const sourcePath = ".awf/docs/pitfalls/combined.md"
	const residuePath = ".awf/docs/pitfalls/.filepublication-combined.tmp"
	faults := &faultFilesystem{scaffoldFilesystem: tree}
	faults.publish = func(path string, source []byte, mode fs.FileMode) error {
		if err := tree.Publish(path, source, mode); err != nil {
			return err
		}
		if err := tree.Publish(residuePath, []byte("temporary"), 0o600); err != nil {
			return err
		}
		return &filepublication.CommittedCleanupError{DestinationPath: path, ResiduePath: residuePath, Cause: cleanupFault}
	}
	outcome, operationErr := createConfined("Combined", faults)
	err := Finish(outcome, operationErr, releaseFault)
	var partial *PartialError
	if !errors.As(err, &partial) || !errors.Is(err, cleanupFault) || !errors.Is(err, releaseFault) {
		t.Fatalf("combined error = %#v, %v", partial, err)
	}
	if !containsPhase(err, projectmutation.PhaseCleanup) || !containsPhase(err, projectmutation.PhaseRelease) {
		t.Fatalf("combined phases missing: %v", err)
	}
	wantRecovery := []string{
		"inspect and treat the authored source " + sourcePath + " as committed",
		"remove publication cleanup residue " + residuePath + " before further project mutation",
		"repair the lease-release fault before further project mutation",
		"do not rerun awf new pitfall with the same title; the committed duplicate will be refused",
	}
	if !slices.Equal(partial.Recovery, wantRecovery) {
		t.Fatalf("recovery = %#v, want %#v", partial.Recovery, wantRecovery)
	}
	if _, retryErr := createConfined("Combined", tree); retryErr == nil || !strings.Contains(retryErr.Error(), residuePath) {
		t.Fatalf("retry ignored residue: %v", retryErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/combined-2.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("combined retry allocated suffix: %v", statErr)
	}
}

func TestCreateRejectsRootSwapAfterOpeningConfinedHandle(t *testing.T) {
	root, loader := fixture(t)
	tx, lease := transaction(t, root, loader)
	moved := root + "-moved"
	var swapErr error
	outcome, err := create(context.Background(), "Swapped", tx, func() {
		if swapErr = os.Rename(root, moved); swapErr != nil {
			return
		}
		if swapErr = os.MkdirAll(filepath.Join(root, ".awf"), 0o755); swapErr != nil {
			return
		}
		swapErr = os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(fixtureConfig), 0o644)
	})
	err = Finish(outcome, err, lease.Release())
	if swapErr != nil {
		t.Fatal(swapErr)
	}
	if !errors.Is(err, filesystem.ErrIdentityChanged) {
		t.Fatalf("root swap error = %v", err)
	}
	for _, candidate := range []string{root, moved} {
		if _, statErr := os.Stat(filepath.Join(candidate, ".awf/docs/pitfalls/swapped.md")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("root swap mutated %s: %v", candidate, statErr)
		}
	}
}

func TestCreateConfinesPitfallSourceRoot(t *testing.T) {
	for _, tc := range []struct {
		name, link string
		prepare    func(*testing.T, string)
	}{
		{name: "ancestor docs symlink", link: ".awf/docs", prepare: func(t *testing.T, root string) {}},
		{name: "leaf pitfalls symlink", link: ".awf/docs/pitfalls", prepare: func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".awf", "docs"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, loader := fixture(t)
			outside := t.TempDir()
			tc.prepare(t, root)
			link := filepath.Join(root, filepath.FromSlash(tc.link))
			if err := os.Symlink(outside, link); err != nil {
				t.Skipf("symlink unsupported: %v", err)
			}
			beforeSelected := treeState(t, root)
			beforeOutside := treeState(t, outside)
			if _, err := runCreate(t, root, loader, "Escaping"); err == nil {
				t.Fatal("escaping source root accepted")
			}
			if after := treeState(t, root); !mapsEqual(beforeSelected, after) {
				t.Fatalf("selected tree mutated:\nbefore=%v\nafter=%v", beforeSelected, after)
			}
			if after := treeState(t, outside); !mapsEqual(beforeOutside, after) {
				t.Fatalf("external tree mutated:\nbefore=%v\nafter=%v", beforeOutside, after)
			}
		})
	}
}

func treeState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			state[rel] = "symlink:" + target
			return nil
		}
		if entry.IsDir() {
			state[rel] = "directory"
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		state[rel] = "file:" + string(raw)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return state
}

func mapsEqual(left, right map[string]string) bool {
	return len(left) == len(right) && func() bool {
		for key, value := range left {
			if right[key] != value {
				return false
			}
		}
		return true
	}()
}

func TestCreateRequiresTrackedScope(t *testing.T) {
	if _, err := Create(context.Background(), "Title", nil); err == nil {
		t.Fatal("nil transaction accepted")
	}
	root, loader := fixture(t)
	tx, err := projectmutation.AcquireProject(context.Background(), root, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Release() //nolint:errcheck // test cleanup
	if _, err := Create(context.Background(), "Title", tx); err == nil {
		t.Fatal("project-scoped transaction accepted")
	}
}

func TestFinishPromotesCloseAndReleaseFaultsAfterCommit(t *testing.T) {
	outcome := Outcome{SourcePath: ".awf/docs/pitfalls/committed.md"}
	closeFault := errors.New("close sentinel")
	err := finishClose(outcome, nil, closeFault)
	var partial *PartialError
	if !errors.As(err, &partial) || !errors.Is(err, closeFault) {
		t.Fatalf("close partial = %#v, %v", partial, err)
	}
	if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhaseCleanup {
		t.Fatalf("close phase = %q, %t", phase, ok)
	}
	releaseFault := errors.New("release sentinel")
	err = Finish(outcome, nil, &projectmutation.Failure{Phase: projectmutation.PhaseRelease, Cause: releaseFault})
	partial = nil
	if !errors.As(err, &partial) || !errors.Is(err, releaseFault) {
		t.Fatalf("release partial = %#v, %v", partial, err)
	}
	if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhaseRelease {
		t.Fatalf("release phase = %q, %t", phase, ok)
	}
	var rendered bytes.Buffer
	document, documentErr := partial.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{outcome.SourcePath, "lease-release fault", "before further project mutation", "do not rerun awf new pitfall"} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("release document missing %q:\n%s", want, rendered.String())
		}
	}
}

func TestDocumentsRejectLineBreakingPaths(t *testing.T) {
	if _, err := (Outcome{SourcePath: "bad\npath"}).Document(); err == nil {
		t.Fatal("complete document accepted line-breaking path")
	}
	partial := newPartial(Outcome{SourcePath: ".awf/docs/pitfalls/example.md", ResiduePath: "bad\nresidue"}, errors.New("fault"))
	if _, err := partial.Document(); err == nil {
		t.Fatal("partial document accepted line-breaking residue")
	}
}
