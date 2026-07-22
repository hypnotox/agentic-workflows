package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestRunNewScaffoldsADR(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runNew(root, "adr", []string{"My", "New", "Title"}, &out); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	want := filepath.Join(root, "docs", "decisions", "0001-my-new-title.md")
	got := strings.TrimSpace(out.String())
	if got != want {
		t.Errorf("runNew printed %q, want %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Errorf("created file not found: %v", err)
	} else if !strings.Contains(string(data), "format: current-state-v2") {
		t.Errorf("activated scaffold is not V2:\n%s", data)
	}
}

func TestRunNewADRError(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "adr", []string{"!!!"}, os.Stdout); err == nil {
		t.Fatal("expected NewADR error for an all-punctuation title")
	}
}

func TestRunNewUnknownKind(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "widget", []string{"x"}, os.Stdout); err == nil || !strings.Contains(err.Error(), "topic") {
		t.Fatalf("expected error naming every kind, got %v", err)
	}
}

func topicCLIProject(t *testing.T) string {
	t.Helper()
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, minimalYAML+"domains: [rendering]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/rendering.yaml"), "paths: [\"internal/**\"]\n")
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("sync topic fixture: %v", err)
	}
	return root
}

func TestRunNewScaffoldsTopicWithoutSyncMutation(t *testing.T) {
	root := topicCLIProject(t)
	beforeConfig := mustReadCLIFile(t, filepath.Join(root, ".awf/config.yaml"))
	beforeLock := mustReadCLIFile(t, filepath.Join(root, ".awf/awf.lock"))
	var out bytes.Buffer
	if err := runNew(root, "topic", []string{"rendering", "Scheduling", "Contracts"}, &out); err != nil {
		t.Fatal(err)
	}
	wantOut := ".awf/topics/metadata/rendering/scheduling-contracts.yaml\n.awf/topics/parts/rendering/scheduling-contracts/current-state.md\n"
	if out.String() != wantOut {
		t.Errorf("output = %q, want %q", out.String(), wantOut)
	}
	metadata := mustReadCLIFile(t, filepath.Join(root, ".awf/topics/metadata/rendering/scheduling-contracts.yaml"))
	part := mustReadCLIFile(t, filepath.Join(root, ".awf/topics/parts/rendering/scheduling-contracts/current-state.md"))
	if !strings.Contains(metadata, "title: Scheduling Contracts") || !strings.Contains(metadata, "replace/with/project/path/**") {
		t.Errorf("metadata:\n%s", metadata)
	}
	if strings.Contains(part, "### `") || !strings.HasSuffix(part, "## Claims\n") {
		t.Errorf("part invented a claim or lacks final Claims:\n%s", part)
	}
	if got := mustReadCLIFile(t, filepath.Join(root, ".awf/config.yaml")); got != beforeConfig {
		t.Error("topic scaffold mutated config")
	}
	if got := mustReadCLIFile(t, filepath.Join(root, ".awf/awf.lock")); got != beforeLock {
		t.Error("topic scaffold mutated lock")
	}
	if _, err := os.Stat(filepath.Join(root, "docs/topics/rendering/scheduling-contracts.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("topic scaffold unexpectedly synced: %v", err)
	}
}

func TestRunNewTopicUsageAndValidation(t *testing.T) {
	root := topicCLIProject(t)
	for _, args := range [][]string{nil, {"rendering"}} {
		if err := runNew(root, "topic", args, io.Discard); err == nil || !strings.Contains(err.Error(), "usage: awf new topic") {
			t.Fatalf("args %v: %v", args, err)
		}
	}
	for _, tc := range []struct{ domain, want string }{{"Rendering", "lowercase kebab-case"}, {"tooling", "not configured"}} {
		if err := runNew(root, "topic", []string{tc.domain, "Title"}, io.Discard); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("domain %q: %v", tc.domain, err)
		}
	}
}

type partialFailWriteCloser struct {
	file *os.File
	err  error
}

func (w *partialFailWriteCloser) Write(data []byte) (int, error) {
	n, writeErr := w.file.Write(data[:3])
	if writeErr != nil {
		return n, writeErr
	}
	return n, w.err
}

func (w *partialFailWriteCloser) Close() error { return w.file.Close() }

type errorWriteCloser struct {
	write func([]byte) (int, error)
	close func() error
}

func (w errorWriteCloser) Write(data []byte) (int, error) { return w.write(data) }
func (w errorWriteCloser) Close() error                   { return w.close() }

func TestWriteAndCloseTopicFileErrors(t *testing.T) {
	closeErr := errors.New("close failed")
	writer := errorWriteCloser{
		write: func(data []byte) (int, error) { return len(data), nil },
		close: func() error { return closeErr },
	}
	if err := writeAndCloseTopicFile("topic.yaml", writer, []byte("content")); !errors.Is(err, closeErr) || !strings.Contains(err.Error(), "close topic scaffold path") {
		t.Fatalf("close error = %v", err)
	}

	writer = errorWriteCloser{
		write: func([]byte) (int, error) { return 0, nil },
		close: func() error { return nil },
	}
	if err := writeAndCloseTopicFile("topic.yaml", writer, []byte("content")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write error = %v", err)
	}
}

func TestCreateTopicParentsErrors(t *testing.T) {
	t.Run("file ancestor", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := createTopicParents(file); err == nil || !strings.Contains(err.Error(), "is not a directory") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("stat failure", func(t *testing.T) {
		statErr := errors.New("stat failed")
		testsupport.SwapVar(t, &topicStat, func(string) (os.FileInfo, error) { return nil, statErr })
		if _, err := createTopicParents(filepath.Join(t.TempDir(), "child")); !errors.Is(err, statErr) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestRollbackTopicScaffoldDirectoryInspection(t *testing.T) {
	t.Run("missing directory is already clean", func(t *testing.T) {
		primary := errors.New("primary")
		err := rollbackTopicScaffold(primary, nil, []string{filepath.Join(t.TempDir(), "missing")})
		if !errors.Is(err, primary) || strings.Contains(err.Error(), "inspect created") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("inspection failure is joined", func(t *testing.T) {
		primary := errors.New("primary")
		inspectErr := errors.New("inspect failed")
		testsupport.SwapVar(t, &topicReadDir, func(string) ([]os.DirEntry, error) { return nil, inspectErr })
		err := rollbackTopicScaffold(primary, nil, []string{t.TempDir()})
		if !errors.Is(err, primary) || !errors.Is(err, inspectErr) || !strings.Contains(err.Error(), "inspect created topic scaffold directory") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestRunNewTopicRollsBackSecondMkdirFailure(t *testing.T) {
	root := topicCLIProject(t)
	before := topicTreeShape(t, root)
	mkdirErr := errors.New("second parent failed")
	testsupport.SwapVar(t, &topicMkdirAll, func(path string, mode os.FileMode) error {
		if strings.Contains(filepath.ToSlash(path), "/topics/parts/") {
			return mkdirErr
		}
		return os.MkdirAll(path, mode)
	})
	err := runNew(root, "topic", []string{"rendering", "Mkdir Rollback"}, io.Discard)
	if !errors.Is(err, mkdirErr) || !strings.Contains(err.Error(), "/topics/parts/rendering/mkdir-rollback/current-state.md") {
		t.Fatalf("error = %v", err)
	}
	if after := topicTreeShape(t, root); !slices.Equal(after, before) {
		t.Fatalf("rollback changed tree shape:\nbefore %v\nafter  %v", before, after)
	}
}

func TestRunNewTopicRollsBackPartialSecondWriteInReverseOrder(t *testing.T) {
	root := topicCLIProject(t)
	writeErr := errors.New("partial second write failed")
	opens := 0
	testsupport.SwapVar(t, &topicOpenFile, func(path string, flag int, mode os.FileMode) (topicWriteCloser, error) {
		opens++
		file, err := os.OpenFile(path, flag, mode)
		if err != nil || opens == 1 {
			return file, err
		}
		return &partialFailWriteCloser{file: file, err: writeErr}, nil
	})
	var removed []string
	testsupport.SwapVar(t, &topicRemove, func(path string) error {
		removed = append(removed, filepath.ToSlash(path))
		return os.Remove(path)
	})
	err := runNew(root, "topic", []string{"rendering", "Partial"}, io.Discard)
	if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "/topics/parts/rendering/partial/current-state.md") {
		t.Fatalf("error = %v", err)
	}
	wantFiles := []string{
		filepath.ToSlash(filepath.Join(root, ".awf/topics/parts/rendering/partial/current-state.md")),
		filepath.ToSlash(filepath.Join(root, ".awf/topics/metadata/rendering/partial.yaml")),
	}
	if len(removed) < len(wantFiles) || !slices.Equal(removed[:len(wantFiles)], wantFiles) {
		t.Fatalf("file rollback order = %v, want prefix %v", removed, wantFiles)
	}
	lastDepth := int(^uint(0) >> 1)
	for _, path := range removed[len(wantFiles):] {
		depth := strings.Count(path, "/")
		if depth > lastDepth {
			t.Fatalf("directory rollback was not deepest-first: %v", removed)
		}
		lastDepth = depth
	}
	for _, path := range []string{
		filepath.Join(root, ".awf/topics/metadata/rendering/partial.yaml"),
		filepath.Join(root, ".awf/topics/parts/rendering/partial/current-state.md"),
	} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("created file survived rollback at %s: %v", path, statErr)
		}
	}
}

func TestRunNewTopicPreservesPreExistingParentOnRollback(t *testing.T) {
	root := topicCLIProject(t)
	preExisting := filepath.Join(root, ".awf/topics/parts/rendering/preserved")
	if err := os.MkdirAll(preExisting, 0o755); err != nil {
		t.Fatal(err)
	}
	before := topicTreeShape(t, root)
	writeErr := errors.New("second write failed")
	opens := 0
	testsupport.SwapVar(t, &topicOpenFile, func(path string, flag int, mode os.FileMode) (topicWriteCloser, error) {
		opens++
		file, err := os.OpenFile(path, flag, mode)
		if err != nil || opens == 1 {
			return file, err
		}
		return &partialFailWriteCloser{file: file, err: writeErr}, nil
	})
	if err := runNew(root, "topic", []string{"rendering", "Preserved"}, io.Discard); !errors.Is(err, writeErr) {
		t.Fatalf("error = %v", err)
	}
	if after := topicTreeShape(t, root); !slices.Equal(after, before) {
		t.Fatalf("rollback did not preserve tree shape:\nbefore %v\nafter  %v", before, after)
	}
}

func TestRunNewTopicJoinsCleanupFailure(t *testing.T) {
	root := topicCLIProject(t)
	writeErr := errors.New("second write failed")
	cleanupErr := errors.New("cleanup failed")
	opens := 0
	testsupport.SwapVar(t, &topicOpenFile, func(path string, flag int, mode os.FileMode) (topicWriteCloser, error) {
		opens++
		file, err := os.OpenFile(path, flag, mode)
		if err != nil || opens == 1 {
			return file, err
		}
		return &partialFailWriteCloser{file: file, err: writeErr}, nil
	})
	removes := 0
	testsupport.SwapVar(t, &topicRemove, func(path string) error {
		removes++
		if removes == 1 {
			return cleanupErr
		}
		return os.Remove(path)
	})
	err := runNew(root, "topic", []string{"rendering", "Cleanup"}, io.Discard)
	if !errors.Is(err, writeErr) || !errors.Is(err, cleanupErr) || !strings.Contains(err.Error(), "remove created topic scaffold path") {
		t.Fatalf("joined error = %v", err)
	}
}

func TestRunNewTopicJoinsDirectoryCleanupFailure(t *testing.T) {
	root := topicCLIProject(t)
	writeErr := errors.New("second write failed")
	cleanupErr := errors.New("directory cleanup failed")
	opens := 0
	testsupport.SwapVar(t, &topicOpenFile, func(path string, flag int, mode os.FileMode) (topicWriteCloser, error) {
		opens++
		file, err := os.OpenFile(path, flag, mode)
		if err != nil || opens == 1 {
			return file, err
		}
		return &partialFailWriteCloser{file: file, err: writeErr}, nil
	})
	testsupport.SwapVar(t, &topicRemove, func(path string) error {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return cleanupErr
		}
		return os.Remove(path)
	})
	err := runNew(root, "topic", []string{"rendering", "Directory Cleanup"}, io.Discard)
	if !errors.Is(err, writeErr) || !errors.Is(err, cleanupErr) || !strings.Contains(err.Error(), "remove created topic scaffold directory") {
		t.Fatalf("joined error = %v", err)
	}
}

func topicTreeShape(t *testing.T, root string) []string {
	t.Helper()
	var shape []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			rel += "/"
		}
		shape = append(shape, rel)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return shape
}

func TestRunNewTopicLateCollisionPreservesExistingBytes(t *testing.T) {
	root := topicCLIProject(t)
	const existing = "existing authored bytes\n"
	opens := 0
	testsupport.SwapVar(t, &topicOpenFile, func(path string, flag int, mode os.FileMode) (topicWriteCloser, error) {
		opens++
		if opens == 2 {
			if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return os.OpenFile(path, flag, mode)
	})
	err := runNew(root, "topic", []string{"rendering", "Late Collision"}, io.Discard)
	part := filepath.Join(root, ".awf/topics/parts/rendering/late-collision/current-state.md")
	if !errors.Is(err, os.ErrExist) || !strings.Contains(err.Error(), filepath.ToSlash(part)) {
		t.Fatalf("collision error = %v", err)
	}
	if got := mustReadCLIFile(t, part); got != existing {
		t.Fatalf("existing bytes = %q, want %q", got, existing)
	}
	metadata := filepath.Join(root, ".awf/topics/metadata/rendering/late-collision.yaml")
	if _, statErr := os.Stat(metadata); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("first file survived rollback: %v", statErr)
	}
}

func TestRunNewTopicFirstWriteAndOpenErrors(t *testing.T) {
	if err := runNew(t.TempDir(), "topic", []string{"rendering", "Failure"}, io.Discard); err == nil || !strings.Contains(err.Error(), "awf init") {
		t.Fatalf("unadopted project error = %v", err)
	}

	root := topicCLIProject(t)
	testsupport.SwapVar(t, &topicOpenFile, func(string, int, os.FileMode) (topicWriteCloser, error) {
		return nil, errors.New("open failed")
	})
	if err := runNew(root, "topic", []string{"rendering", "Failure"}, io.Discard); err == nil || !strings.Contains(err.Error(), "open failed") {
		t.Fatalf("open error = %v", err)
	}

	root = topicCLIProject(t)
	testsupport.WriteAwfConfig(t, root, minimalYAML+"domains: [rendering]\ndocs: [ghost-doc]\n")
	if err := runNew(root, "topic", []string{"rendering", "Failure"}, io.Discard); err == nil {
		t.Fatal("expected project.Open error")
	}
}

func mustReadCLIFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunNewScaffoldsPlan(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runNew(root, "plan", []string{"Some", "Plan", "Title"}, &out); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	got := strings.TrimSpace(out.String())
	// Date-prefixed under docs/plans (no sequential number); the date is today's,
	// so match on shape rather than couple the test to the wall clock.
	if dir := filepath.Dir(got); dir != filepath.Join(root, "docs", "plans") {
		t.Errorf("plan written to %q, want under docs/plans", got)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-some-plan-title\.md$`).MatchString(filepath.Base(got)) {
		t.Errorf("plan filename %q not YYYY-MM-DD-some-plan-title.md", filepath.Base(got))
	}
	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("created file not found: %v", err)
	}
	if !strings.Contains(string(body), "# Plan: Some Plan Title") || !strings.Contains(string(body), "status: Proposed") {
		t.Errorf("plan not scaffolded from template:\n%s", body)
	}
}

func TestRunNewPlanMissingTitle(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "plan", nil, os.Stdout); err == nil {
		t.Fatal("expected usage error for a missing plan title")
	}
}

func TestRunNewPlanRefusesExisting(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "plan", []string{"Same", "Plan"}, io.Discard); err != nil {
		t.Fatalf("first runNew: %v", err)
	}
	if err := runNew(root, "plan", []string{"Same", "Plan"}, io.Discard); err == nil {
		t.Fatal("expected overwrite refusal for a same-day same-title plan")
	}
}

func TestRunNewPlanOpenError(t *testing.T) {
	root := scaffoldProject(t)
	// Passes the schema/version gate but fails project.Open (a ghost enabled doc),
	// covering newPlan's Open-error return.
	testsupport.WriteAwfConfig(t, root, minimalYAML+"docs: [ghost-doc]\n")
	if err := runNew(root, "plan", []string{"Some", "Plan"}, io.Discard); err == nil {
		t.Fatal("expected project.Open error")
	}
}

func TestRunNewScaffoldsDoc(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"guides/ci", "How", "CI", "runs"}, io.Discard); err != nil {
		t.Fatalf("new doc: %v", err)
	}
	sc := filepath.Join(root, ".awf", "docs", "guides", "ci.yaml")
	if b, err := os.ReadFile(sc); err != nil {
		t.Fatalf("sidecar not written: %v", err)
	} else if !strings.Contains(string(b), "title: Ci") || !strings.Contains(string(b), "description: How CI runs") {
		t.Errorf("sidecar content wrong:\n%s", b)
	}
	part := filepath.Join(root, ".awf", "docs", "parts", "guides", "ci", "content.md")
	if b, err := os.ReadFile(part); err != nil {
		t.Fatalf("part not written: %v", err)
	} else if !strings.Contains(string(b), "awf:stub") {
		t.Errorf("part missing stub marker:\n%s", b)
	}
	out := filepath.Join(root, "docs", "guides", "ci.md")
	if _, err := os.Stat(out); err != nil {
		t.Errorf("rendered doc missing: %v", err)
	}
}

func TestRunNewDocMissingDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"ci"}, io.Discard); err == nil {
		t.Fatal("expected usage error for missing description")
	}
}

func TestRunNewDocEmptyDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"ci", "   "}, io.Discard); err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestRunNewDocInvalidName(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"Bad", "desc"}, io.Discard); err == nil {
		t.Fatal("expected error for invalid doc name")
	}
}

func TestRunNewDocRefusesExisting(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"ci", "desc"}, io.Discard); err != nil {
		t.Fatalf("first new doc: %v", err)
	}
	// Disable ci in config but leave its sidecar+part on disk, so the second run's
	// catalog-collision check misses and the authored-files guard fires (mirrors
	// TestRunNewRefusesExistingLocalArtifactFiles).
	cfgPath := config.ConfigPath(root)
	src, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := config.SetArrayMember(src, "docs", "ci", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runNew(root, "doc", []string{"ci", "desc"}, io.Discard); err == nil {
		t.Fatal("expected refusal for existing doc files")
	}
}

func TestRunNewDocCollidesWithCatalog(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"architecture", "desc"}, io.Discard); err == nil {
		t.Fatal("expected collision error for a catalog doc name")
	}
}

func TestRunNewDispatch(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "adr", "Some", "Title"}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
}

func TestRunNewMissingArgs(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "adr"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for missing title, got %d", code)
	}
}

func TestRunNewTopicDispatchAndHelp(t *testing.T) {
	root := topicCLIProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "topic", "rendering", "Dispatch", "Topic"}, &out, &errb); code != 0 {
		t.Fatalf("dispatch exit = %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "dispatch-topic.yaml") {
		t.Errorf("dispatch output = %q", out.String())
	}
	out.Reset()
	if code := run([]string{"awf", "new", "topic", "--help"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "awf new topic <domain> <title>") {
		t.Fatalf("help exit = %d, output = %q, error = %q", code, out.String(), errb.String())
	}
}

func TestRunNewTopicMissingArgsDispatch(t *testing.T) {
	root := topicCLIProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "topic", "rendering"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2, got %d (%s)", code, errb.String())
	}
}

// An unrecognized `new` subcommand is not a clispec child, so resolve leaves it
// in the positionals; the new handler reunites it as the kind and runNew reports
// the unknown-kind usage error (exit 2).
func TestRunNewUnknownKindDispatch(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "widget", "x"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown kind, got %d (%s)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "unknown kind") {
		t.Errorf("missing unknown-kind message: %q", errb.String())
	}
}

func TestRunNewScaffoldsSkill(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"deploy-check", "Verify the deploy is green."}, io.Discard); err != nil {
		t.Fatalf("runNew skill: %v", err)
	}
	sc, err := os.ReadFile(filepath.Join(root, ".awf", "skills", "deploy-check.yaml"))
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	if !strings.Contains(string(sc), "Verify the deploy is green.") {
		t.Errorf("sidecar missing description:\n%s", sc)
	}
	part, err := os.ReadFile(filepath.Join(root, ".awf", "skills", "parts", "deploy-check", "content.md"))
	if err != nil {
		t.Errorf("content part not written: %v", err)
	}
	if !strings.HasPrefix(string(part), "<!-- awf:stub -->\n") {
		t.Errorf("starter part must open with the awf:stub marker (ADR-0070):\n%s", part)
	}
	cfg, _ := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if !strings.Contains(string(cfg), "deploy-check") {
		t.Errorf("skill not enabled in config:\n%s", cfg)
	}
	rendered, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "example-deploy-check", "SKILL.md"))
	if err != nil {
		t.Errorf("rendered skill missing: %v", err)
	}
	if !strings.Contains(string(rendered), "<!-- awf:stub -->") {
		t.Errorf("stub-marked part must render verbatim, marker included:\n%s", rendered)
	}
	if err := runCheck(root, false, io.Discard); err != nil {
		t.Errorf("post-scaffold check not clean: %v", err)
	}
}

// awf new must refuse when the name already has files under .awf/, even when
// the name is not in the enable array (an enabled+declared local is caught by
// the catalog-pool guard; a disabled one left its sidecar and authored part on
// disk, and a re-run must not silently reset them to the stub).
func TestRunNewRefusesExistingLocalArtifactFiles(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"deploy-check", "Verify the deploy is green."}, io.Discard); err != nil {
		t.Fatalf("runNew skill: %v", err)
	}
	partPath := filepath.Join(root, ".awf", "skills", "parts", "deploy-check", "content.md")
	const authored = "Authored body - must survive a re-run.\n"
	if err := os.WriteFile(partPath, []byte(authored), 0o644); err != nil {
		t.Fatal(err)
	}
	// Disable the skill but keep its authored files, as `awf disable skill` would.
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(strings.ReplaceAll(string(cfg), "  - deploy-check\n", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runNew(root, "skill", []string{"deploy-check", "Other description."}, io.Discard); err == nil {
		t.Fatal("expected error re-running awf new over existing local artifact files")
	}
	part, err := os.ReadFile(partPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(part) != authored {
		t.Errorf("authored content part was overwritten:\n%s", part)
	}
	sc, err := os.ReadFile(filepath.Join(root, ".awf", "skills", "deploy-check.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sc), "Verify the deploy is green.") {
		t.Errorf("authored sidecar was overwritten:\n%s", sc)
	}
}

func TestRunNewScaffoldsAgent(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "agent", []string{"deploy-bot", "Runs the deploy checks."}, io.Discard); err != nil {
		t.Fatalf("runNew agent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "agents", "deploy-bot.yaml")); err != nil {
		t.Errorf("agent sidecar not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "deploy-bot.md")); err != nil {
		t.Errorf("rendered agent missing: %v", err)
	}
}

func TestRunNewSkillMissingDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"lonely"}, io.Discard); err == nil {
		t.Fatal("expected usage error when description is missing")
	}
}

func TestRunNewSkillEmptyDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"x", "   "}, io.Discard); err == nil {
		t.Fatal("expected error for a whitespace-only description")
	}
}

func TestRunNewSkillReservedName(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"_base", "desc"}, io.Discard); err == nil {
		t.Fatal("expected reserved-name rejection")
	}
}

func TestRunNewSkillCollision(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"tdd", "desc"}, io.Discard); err == nil {
		t.Fatal("expected collision with the catalog skill tdd")
	}
}

func TestRunNewSkillOpenError(t *testing.T) {
	root := scaffoldProject(t)
	// Passes the schema/version gate (lock intact) but fails project.Open - an
	// enabled doc that is not in the catalog.
	testsupport.WriteAwfConfig(t, root, minimalYAML+"docs: [ghost-doc]\n")
	if err := runNew(root, "skill", []string{"newone", "a description"}, io.Discard); err == nil {
		t.Fatal("expected project.Open error")
	}
}

func TestRunNewDocOpenError(t *testing.T) {
	root := scaffoldProject(t)
	// Passes the schema/version gate but fails project.Open (a ghost enabled doc),
	// covering newLocalDoc's Open-error return.
	testsupport.WriteAwfConfig(t, root, minimalYAML+"docs: [ghost-doc]\n")
	if err := runNew(root, "doc", []string{"newdoc", "a description"}, io.Discard); err == nil {
		t.Fatal("expected project.Open error")
	}
}

// seedScaffoldVars: an absent referenced var is seeded empty, a present one is
// untouched, and a malformed source surfaces the editor's error.
// invariant: tooling/init-and-enablement:new-seeds-scaffold-vars
func TestSeedScaffoldVars(t *testing.T) {
	src := []byte("prefix: x\nvars:\n  kept: value\n")
	got, err := seedScaffoldVars(src, []string{"kept", "added"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kept: value", "added: \"\""} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if _, err := seedScaffoldVars([]byte(":\n:"), []string{"x"}); err == nil {
		t.Fatal("expected error on malformed config source")
	}
}

// The shipped base templates reference no vars, so awf new seeds nothing today
// - this pins the no-op so a future var-bearing base template consciously
// changes it (ADR-0087 Decision 4).
func TestRunNewSeedsNoVarsToday(t *testing.T) {
	for _, kind := range []string{"skill", "agent"} {
		refs, err := project.ScaffoldVarRefs(kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != 0 {
			t.Errorf("base %s template gained var refs %v - confirm awf new seeding and update this pin", kind, refs)
		}
	}
}
