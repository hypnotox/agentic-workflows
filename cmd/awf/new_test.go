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
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestRunNewScaffoldsADR(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runNew(ctx, root, "adr", []string{"My", "New", "Title"}, &out); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	want := filepath.Join(root, "docs", "decisions", "0001-my-new-title.md")
	got := strings.TrimSpace(out.String())
	if got != "status: created: "+want {
		t.Errorf("runNew printed %q, want created status for %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Errorf("created file not found: %v", err)
	} else if !strings.Contains(string(data), "format: current-state-v4\nslug: my-new-title\n") {
		t.Errorf("activated scaffold is not V4 with its frozen slug:\n%s", data)
	}
}

func TestRunNewADRError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	if err := newADR(ctx, root, nil, io.Discard); err == nil {
		t.Fatal("expected missing-title usage error")
	}
	if err := runNew(ctx, root, "adr", []string{"!!!"}, os.Stdout); err == nil {
		t.Fatal("expected NewADR error for an all-punctuation title")
	}
}

func TestRunNewUnknownKind(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	if err := runNew(ctx, root, "widget", []string{"x"}, os.Stdout); err == nil || !strings.Contains(err.Error(), "topic") {
		t.Fatalf("expected error naming every kind, got %v", err)
	}
}

func topicCLIProject(t *testing.T) string {
	t.Helper()
	ctx := testContext(t)
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, minimalYAML+"domains: [rendering]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/rendering.yaml"), "paths: [\"internal/**\"]\n")
	if err := runSync(ctx, root, io.Discard); err != nil {
		t.Fatalf("sync topic fixture: %v", err)
	}
	return root
}

func TestRunNewScaffoldsTopicWithoutSyncMutation(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := topicCLIProject(t)
	beforeConfig := mustReadCLIFile(t, filepath.Join(root, ".awf/config.yaml"))
	beforeLock := mustReadCLIFile(t, filepath.Join(root, ".awf/awf.lock"))
	var out bytes.Buffer
	if err := runNew(ctx, root, "topic", []string{"rendering", "Scheduling", "Contracts"}, &out); err != nil {
		t.Fatal(err)
	}
	wantOut := "topic:\n  created files:\n    .awf/topics/metadata/rendering/scheduling-contracts.yaml\n    .awf/topics/parts/rendering/scheduling-contracts/current-state.md\n"
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
	ctx := testContext(t)
	_ = ctx
	root := topicCLIProject(t)
	for _, args := range [][]string{nil, {"rendering"}} {
		if err := runNew(ctx, root, "topic", args, io.Discard); err == nil || !strings.Contains(err.Error(), "usage: awf new topic") {
			t.Fatalf("args %v: %v", args, err)
		}
	}
	for _, tc := range []struct{ domain, want string }{{"Rendering", "lowercase kebab-case"}, {"tooling", "not configured"}} {
		if err := runNew(ctx, root, "topic", []string{tc.domain, "Title"}, io.Discard); err == nil || !strings.Contains(err.Error(), tc.want) {
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
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
	root := topicCLIProject(t)
	before := topicTreeShape(t, root)
	mkdirErr := errors.New("second parent failed")
	testsupport.SwapVar(t, &topicMkdirAll, func(path string, mode os.FileMode) error {
		if strings.Contains(filepath.ToSlash(path), "/topics/parts/") {
			return mkdirErr
		}
		return os.MkdirAll(path, mode)
	})
	err := runNew(ctx, root, "topic", []string{"rendering", "Mkdir Rollback"}, io.Discard)
	if !errors.Is(err, mkdirErr) || !strings.Contains(err.Error(), "/topics/parts/rendering/mkdir-rollback/current-state.md") {
		t.Fatalf("error = %v", err)
	}
	if after := topicTreeShape(t, root); !slices.Equal(after, before) {
		t.Fatalf("rollback changed tree shape:\nbefore %v\nafter  %v", before, after)
	}
}

func TestRunNewTopicRollsBackPartialSecondWriteInReverseOrder(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	err := runNew(ctx, root, "topic", []string{"rendering", "Partial"}, io.Discard)
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
	ctx := testContext(t)
	_ = ctx
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
	if err := runNew(ctx, root, "topic", []string{"rendering", "Preserved"}, io.Discard); !errors.Is(err, writeErr) {
		t.Fatalf("error = %v", err)
	}
	if after := topicTreeShape(t, root); !slices.Equal(after, before) {
		t.Fatalf("rollback did not preserve tree shape:\nbefore %v\nafter  %v", before, after)
	}
}

func TestRunNewTopicJoinsCleanupFailure(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	err := runNew(ctx, root, "topic", []string{"rendering", "Cleanup"}, io.Discard)
	if !errors.Is(err, writeErr) || !errors.Is(err, cleanupErr) || !strings.Contains(err.Error(), "remove created topic scaffold path") {
		t.Fatalf("joined error = %v", err)
	}
}

func TestRunNewTopicJoinsDirectoryCleanupFailure(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	err := runNew(ctx, root, "topic", []string{"rendering", "Directory Cleanup"}, io.Discard)
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
	ctx := testContext(t)
	_ = ctx
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
	err := runNew(ctx, root, "topic", []string{"rendering", "Late Collision"}, io.Discard)
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
	ctx := testContext(t)
	_ = ctx
	if err := runNew(ctx, t.TempDir(), "topic", []string{"rendering", "Failure"}, io.Discard); err == nil || !strings.Contains(err.Error(), "awf init") {
		t.Fatalf("unadopted project error = %v", err)
	}

	root := topicCLIProject(t)
	testsupport.SwapVar(t, &topicOpenFile, func(string, int, os.FileMode) (topicWriteCloser, error) {
		return nil, errors.New("open failed")
	})
	if err := runNew(ctx, root, "topic", []string{"rendering", "Failure"}, io.Discard); err == nil || !strings.Contains(err.Error(), "open failed") {
		t.Fatalf("open error = %v", err)
	}

	root = topicCLIProject(t)
	testsupport.WriteAwfConfig(t, root, minimalYAML+"domains: [rendering]\n")
	if err := runNew(ctx, root, "topic", []string{"rendering", "Failure"}, io.Discard); err == nil {
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
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runNew(ctx, root, "plan", []string{"Some", "Plan", "Title"}, &out); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	got := strings.TrimPrefix(strings.TrimSpace(out.String()), "status: created: ")
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
	if !strings.Contains(string(body), "format: plan-v2\n") || !strings.Contains(string(body), "# Plan: Some Plan Title") || !strings.Contains(string(body), "status: Proposed") {
		t.Errorf("plan not scaffolded from plan-v2 template:\n%s", body)
	}
	plans, err := plan.ParseDir(filepath.Dir(got))
	if err != nil {
		t.Fatalf("scaffolded plan does not parse cleanly: %v", err)
	}
	if len(plans) != 1 || plans[0].Filename != filepath.Base(got) || plans[0].Format != "plan-v2" {
		t.Fatalf("parsed scaffold = %#v, want created plan-v2", plans)
	}
}

func TestRunNewPlanMissingTitle(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	if err := runNew(ctx, root, "plan", nil, os.Stdout); err == nil {
		t.Fatal("expected usage error for a missing plan title")
	}
}

func TestRunNewPlanRefusesExisting(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	if err := runNew(ctx, root, "plan", []string{"Same", "Plan"}, io.Discard); err != nil {
		t.Fatalf("first runNew: %v", err)
	}
	if err := runNew(ctx, root, "plan", []string{"Same", "Plan"}, io.Discard); err == nil {
		t.Fatal("expected overwrite refusal for a same-day same-title plan")
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

// invariant: tooling/cli:cli-creation-and-inventory (TestRunNewDomainLifecycle)
// invariant: tooling/cli:domain-lifecycle-commands (TestRunNewDomainLifecycle)
func TestRunNewDomainLifecycle(t *testing.T) {
	root := scaffoldProject(t)
	ctx := testContext(t)
	if err := runNew(ctx, root, "domain", []string{"delivery"}, io.Discard); err != nil {
		t.Fatalf("new domain: %v", err)
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cfg.Domains, "delivery") {
		t.Fatalf("domains = %v", cfg.Domains)
	}
	part := filepath.Join(root, ".awf", "domains", "parts", "delivery", "current-state.md")
	before, err := os.ReadFile(part)
	if err != nil {
		t.Fatal(err)
	}
	if err := runNew(ctx, root, "domain", []string{"delivery"}, io.Discard); err == nil {
		t.Fatal("duplicate domain accepted")
	}
	after, err := os.ReadFile(part)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("existing part changed: %v", err)
	}
}

func TestRunNewDomainRejectsInvalidNameBeforeWriting(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(testContext(t), root, "domain", []string{"../bad"}, io.Discard); err == nil {
		t.Fatal("invalid domain accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "domains", "parts")); !os.IsNotExist(err) {
		t.Fatalf("invalid domain wrote files: %v", err)
	}
}

func TestRunNewRetiredKind(t *testing.T) {
	if err := runNew(testContext(t), t.TempDir(), "skill", []string{"x"}, io.Discard); err == nil || !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("retired kind error = %v", err)
	}
}
