package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/pitfallop"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type leaseProbeWriter struct {
	root    string
	checked bool
	err     error
}

func (w *leaseProbeWriter) Write(p []byte) (int, error) {
	if !w.checked {
		w.checked = true
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()
		lease, err := filesystem.AcquireTrackedLease(ctx, w.root)
		w.err = err
		if lease != nil {
			_ = lease.Release()
		}
	}
	return len(p), nil
}

func TestMutationLeaseRemainsHeldThroughPresentation(t *testing.T) {
	for _, test := range []struct {
		name string
		root func(*testing.T) string
		run  func(string, io.Writer) error
	}{
		{name: "local document project lease", root: scaffoldProject, run: func(root string, out io.Writer) error {
			return newDoc(testContext(t), root, []string{"runbooks/lease", "Lease lifetime"}, nil, out)
		}},
		{name: "topic tracked lease", root: topicCLIProject, run: func(root string, out io.Writer) error {
			return newTopic(testContext(t), root, []string{"rendering", "Lease Lifetime"}, out)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := test.root(t)
			probe := &leaseProbeWriter{root: root}
			if err := test.run(root, probe); err != nil {
				t.Fatal(err)
			}
			if !probe.checked || !errors.Is(probe.err, context.DeadlineExceeded) {
				t.Fatalf("presentation lease probe = checked %t, error %v", probe.checked, probe.err)
			}
		})
	}
}

func TestPitfallLeaseIsReleasedBeforePresentation(t *testing.T) {
	root := scaffoldProject(t)
	probe := &leaseProbeWriter{root: root}
	if err := newPitfall(testContext(t), root, []string{"Release Before Report"}, probe); err != nil {
		t.Fatal(err)
	}
	if !probe.checked || probe.err != nil {
		t.Fatalf("presentation lease probe = checked %t, error %v", probe.checked, probe.err)
	}
}

func TestNewOwnerWrappersRejectUsageAndMalformedRepository(t *testing.T) {
	if err := newDoc(testContext(t), t.TempDir(), nil, nil, io.Discard); err == nil {
		t.Fatal("local document without arguments accepted")
	}
	for _, operation := range []struct {
		name string
		run  func(string) error
	}{
		{name: "local document", run: func(root string) error {
			return newDoc(testContext(t), root, []string{"runbooks/api", "Operate API"}, nil, io.Discard)
		}},
		{name: "topic", run: func(root string) error {
			return newTopic(testContext(t), root, []string{"tooling", "API"}, io.Discard)
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			root := scaffoldProject(t)
			if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
				t.Fatal(err)
			}
			testsupport.WriteFile(t, filepath.Join(root, ".git"), "not a gitdir pointer\n")
			if err := operation.run(root); err == nil {
				t.Fatal("malformed repository accepted")
			}
		})
	}
}

// invariant: tooling/cli:cli-creation-and-inventory (TestRunNewDocScaffoldsLocalDocument)
func TestRunNewDocScaffoldsLocalDocument(t *testing.T) {
	t.Run("derived title", func(t *testing.T) {
		root := scaffoldProject(t)
		var out bytes.Buffer
		if err := newDoc(testContext(t), root, []string{"runbooks/api-v2", "How to operate API v2"}, nil, &out); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); !strings.Contains(got, "status: local document created") || !strings.Contains(got, "local-document declaration replacement: true") || !strings.Contains(got, "docs/runbooks/api-v2.md") {
			t.Fatalf("output does not retain complete outcome: %q", got)
		}
		assertLocalDocs(t, root, config.LocalDocs{{Name: "runbooks/api-v2", Title: "Api V2", Description: "How to operate API v2"}})
		if _, err := os.Stat(filepath.Join(root, "docs/runbooks/api-v2.md")); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("explicit title through driver", func(t *testing.T) {
		root := scaffoldProject(t)
		var out, errOut bytes.Buffer
		if code := runFrom(root, []string{"awf", "new", "doc", "runbooks/api-v2", "How to operate API v2", "--title", "API v2"}, &out, &errOut); code != 0 {
			t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
		}
		assertLocalDocs(t, root, config.LocalDocs{{Name: "runbooks/api-v2", Title: "API v2", Description: "How to operate API v2"}})
	})
}

func TestRunNewDocRefusesBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name, docName, description string
		title                      *string
		prepare                    func(*testing.T, string)
	}{
		{name: "empty name", docName: "", description: "Description"},
		{name: "empty description", docName: "runbooks/api", description: ""},
		{name: "reserved name", docName: "plans/api", description: "Description"},
		{name: "managed collision", docName: "architecture", description: "Description"},
		{name: "configured duplicate without destination", docName: "runbooks/api", description: "Again", prepare: func(t *testing.T, root string) {
			if err := newDoc(testContext(t), root, []string{"runbooks/api", "First"}, nil, io.Discard); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(root, "docs/runbooks/api.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "existing unmanaged destination", docName: "runbooks/api", description: "Description", prepare: func(t *testing.T, root string) {
			testsupport.WriteFile(t, filepath.Join(root, "docs/runbooks/api.md"), "foreign")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldProject(t)
			if tc.prepare != nil {
				tc.prepare(t, root)
			}
			beforeConfig := mustReadCLIFile(t, config.ConfigPath(root))
			beforeLock := mustReadCLIFile(t, config.LockPath(root))
			if err := newDoc(testContext(t), root, []string{tc.docName, tc.description}, tc.title, io.Discard); err == nil {
				t.Fatal("invalid local document accepted")
			}
			if got := mustReadCLIFile(t, config.ConfigPath(root)); got != beforeConfig {
				t.Fatal("refusal mutated config")
			}
			if got := mustReadCLIFile(t, config.LockPath(root)); got != beforeLock {
				t.Fatal("refusal mutated lock")
			}
		})
	}

	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
	}{
		{name: "empty explicit title", args: []string{"awf", "new", "doc", "runbooks/api", "Description", "--title", ""}, wantCode: 1},
		{name: "repeated title flag", args: []string{"awf", "new", "doc", "runbooks/api", "Description", "--title", "API", "--title", "Again"}, wantCode: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldProject(t)
			beforeConfig := mustReadCLIFile(t, config.ConfigPath(root))
			beforeLock := mustReadCLIFile(t, config.LockPath(root))
			var out, errOut bytes.Buffer
			if code := runFrom(root, tc.args, &out, &errOut); code != tc.wantCode {
				t.Fatalf("exit = %d, stderr = %q", code, errOut.String())
			}
			if got := mustReadCLIFile(t, config.ConfigPath(root)); got != beforeConfig {
				t.Fatal("flag refusal mutated config")
			}
			if got := mustReadCLIFile(t, config.LockPath(root)); got != beforeLock {
				t.Fatal("flag refusal mutated lock")
			}
		})
	}
}

func assertLocalDocs(t *testing.T, root string, want config.LocalDocs) {
	t.Helper()
	cfg, err := config.Load(filepath.Join(root, ".awf"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.LocalDocs, want) {
		t.Fatalf("localDocs = %#v, want %#v", cfg.LocalDocs, want)
	}
}

// invariant: tooling/cli:pitfall-scaffold (TestRunNewPitfallScaffoldsOneAuthoredSourceWithoutRender)
func TestRunNewPitfallScaffoldsOneAuthoredSourceWithoutRender(t *testing.T) {
	root := scaffoldProject(t)
	generated := filepath.Join(root, "docs", "pitfalls.md")
	beforeGenerated := mustReadCLIFile(t, generated)
	beforeLock := mustReadCLIFile(t, filepath.Join(root, ".awf", "awf.lock"))
	beforeCensus := newPitfallFileCensus(t, root)
	var out bytes.Buffer
	if err := runNew(testContext(t), root, "pitfall", []string{"CLI Hazard!"}, &out); err != nil {
		t.Fatal(err)
	}
	const relative = ".awf/docs/pitfalls/cli-hazard.md"
	if out.String() != "status: pitfall created\nauthored path: "+relative+"\n" {
		t.Fatalf("output = %q", out.String())
	}
	const want = "---\ntitle: CLI Hazard!\n---\nDescribe the durable hazard, its consequence, and the safer practice.\n"
	if got := mustReadCLIFile(t, filepath.Join(root, filepath.FromSlash(relative))); got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
	if got := mustReadCLIFile(t, generated); got != beforeGenerated {
		t.Fatal("pitfall scaffold mutated generated index")
	}
	if got := mustReadCLIFile(t, filepath.Join(root, ".awf", "awf.lock")); got != beforeLock {
		t.Fatal("pitfall scaffold mutated lock")
	}
	afterCensus := newPitfallFileCensus(t, root)
	delete(afterCensus, relative)
	if !maps.Equal(beforeCensus, afterCensus) {
		var changed []string
		for path, before := range beforeCensus {
			if after, ok := afterCensus[path]; !ok || after != before {
				changed = append(changed, path)
			}
		}
		for path := range afterCensus {
			if _, ok := beforeCensus[path]; !ok {
				changed = append(changed, path)
			}
		}
		slices.Sort(changed)
		t.Fatalf("pitfall scaffold changed files beyond its selected source: %v", changed)
	}
	for _, args := range [][]string{nil, {"split", "title"}} {
		if err := runNew(testContext(t), root, "pitfall", args, io.Discard); err == nil || !strings.Contains(err.Error(), "usage: awf new pitfall") {
			t.Fatalf("args %v: %v", args, err)
		}
	}
	if err := runNew(testContext(t), root, "pitfall", []string{""}, io.Discard); err == nil || !strings.Contains(err.Error(), "title is empty") {
		t.Fatalf("empty title error = %v", err)
	}

	broken := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, broken, minimalYAML+"unknown: true\n")
	if err := runNew(testContext(t), broken, "pitfall", []string{"Title"}, io.Discard); err == nil {
		t.Fatal("pitfall scaffold accepted a project Session loading error")
	}
}

func TestRunNewPitfallPresentationFailureRetainsCommittedOutcome(t *testing.T) {
	root := scaffoldProject(t)
	err := newPitfall(testContext(t), root, []string{"Writer Hazard"}, errorWriter{})
	var partial *pitfallop.PartialError
	if !errors.As(err, &partial) || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("presentation failure = %#v, %v", partial, err)
	}
	const relative = ".awf/docs/pitfalls/writer-hazard.md"
	if partial.Outcome.SourcePath != relative {
		t.Fatalf("committed outcome = %#v", partial.Outcome)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); statErr != nil {
		t.Fatalf("committed source missing: %v", statErr)
	}
}

func TestRunNewPitfallReleaseFaultReportsCommittedSourceOnce(t *testing.T) {
	root := scaffoldProject(t)
	generated := filepath.Join(root, "docs", "pitfalls.md")
	lock := filepath.Join(root, ".awf", "awf.lock")
	beforeGenerated := mustReadCLIFile(t, generated)
	beforeLock := mustReadCLIFile(t, lock)
	releaseFault := errors.New("release sentinel")
	var stdout bytes.Buffer
	err := newPitfallWithReleaseFault(testContext(t), root, []string{"Release Hazard"}, &stdout, func() error {
		return releaseFault
	})
	if !errors.Is(err, releaseFault) {
		t.Fatalf("release error = %v", err)
	}
	if phase, ok := projectmutation.FailurePhase(err); !ok || phase != projectmutation.PhaseRelease {
		t.Fatalf("raw command release phase = %q, %t", phase, ok)
	}
	var stderr bytes.Buffer
	if code := completeHandlerResult(&stdout, &stderr, handlerReport(err)); code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("generic stderr = %q", stderr.String())
	}
	const relative = ".awf/docs/pitfalls/release-hazard.md"
	if strings.Count(stdout.String(), "status: pitfall scaffold partially committed") != 1 {
		t.Fatalf("partial report count or status = %q", stdout.String())
	}
	for _, want := range []string{relative, "as committed", "repair the lease-release fault before further project mutation", "do not rerun awf new pitfall with the same title"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("partial report missing %q:\n%s", want, stdout.String())
		}
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); statErr != nil {
		t.Fatalf("committed source missing: %v", statErr)
	}
	if got := mustReadCLIFile(t, generated); got != beforeGenerated {
		t.Fatal("release-fault scaffold mutated generated index")
	}
	if got := mustReadCLIFile(t, lock); got != beforeLock {
		t.Fatal("release-fault scaffold mutated lock")
	}
	if retryErr := newPitfall(testContext(t), root, []string{"Release Hazard"}, io.Discard); retryErr == nil || !strings.Contains(retryErr.Error(), "duplicates") {
		t.Fatalf("same-title recovery retry = %v", retryErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "docs", "pitfalls", "release-hazard-2.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("same-title retry allocated suffix: %v", statErr)
	}
}

func TestRunNewPitfallCombinedCleanupAndReleaseFaultReportsAllRecovery(t *testing.T) {
	root := scaffoldProject(t)
	generated := filepath.Join(root, "docs", "pitfalls.md")
	lock := filepath.Join(root, ".awf", "awf.lock")
	beforeGenerated := mustReadCLIFile(t, generated)
	beforeLock := mustReadCLIFile(t, lock)
	cleanupFault := errors.New("cleanup sentinel")
	releaseFault := errors.New("release sentinel")
	const sourcePath = ".awf/docs/pitfalls/combined-hazard.md"
	const residuePath = ".awf/docs/pitfalls/.filepublication-combined.tmp"
	create := func(ctx context.Context, title string, tx *projectmutation.Transaction) (pitfallop.Outcome, error) {
		outcome, err := pitfallop.Create(ctx, title, tx)
		if err != nil {
			return outcome, err
		}
		if outcome.SourcePath != sourcePath {
			t.Fatalf("source path = %q, want %q", outcome.SourcePath, sourcePath)
		}
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(residuePath)), "temporary")
		outcome.ResiduePath = residuePath
		cleanup := &filepublication.CommittedCleanupError{DestinationPath: sourcePath, ResiduePath: residuePath, Cause: cleanupFault}
		return outcome, pitfallop.Finish(outcome, &projectmutation.Failure{Phase: projectmutation.PhaseCleanup, Cause: cleanup}, nil)
	}
	var stdout bytes.Buffer
	err := newPitfallWithFaults(testContext(t), root, []string{"Combined Hazard"}, &stdout, func() error { return releaseFault }, create)
	if !errors.Is(err, cleanupFault) || !errors.Is(err, releaseFault) {
		t.Fatalf("combined causes = %v", err)
	}
	var partial *pitfallop.PartialError
	if !errors.As(err, &partial) {
		t.Fatalf("combined partial = %v", err)
	}
	var stderr bytes.Buffer
	if code := completeHandlerResult(&stdout, &stderr, handlerReport(err)); code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("generic stderr = %q", stderr.String())
	}
	report := stdout.String()
	if strings.Count(report, "status: pitfall scaffold partially committed") != 1 {
		t.Fatalf("partial report count = %q", report)
	}
	if strings.Count(report, sourcePath) != 2 || strings.Count(report, residuePath) != 2 {
		t.Fatalf("combined report does not retain exact identity and recovery paths:\n%s", report)
	}
	ordered := []string{
		"inspect and treat the authored source " + sourcePath + " as committed",
		"remove publication cleanup residue " + residuePath + " before further project mutation",
		"repair the lease-release fault before further project mutation",
		"do not rerun awf new pitfall with the same title; the committed duplicate will be refused",
	}
	position := -1
	for _, want := range ordered {
		if strings.Count(report, want) != 1 {
			t.Fatalf("combined report occurrence %q:\n%s", want, report)
		}
		next := strings.Index(report, want)
		if next <= position {
			t.Fatalf("combined report order at %q:\n%s", want, report)
		}
		position = next
	}
	if got := mustReadCLIFile(t, generated); got != beforeGenerated {
		t.Fatal("combined-fault scaffold mutated generated index")
	}
	if got := mustReadCLIFile(t, lock); got != beforeLock {
		t.Fatal("combined-fault scaffold mutated lock")
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(sourcePath))); statErr != nil {
		t.Fatalf("committed source missing: %v", statErr)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(residuePath))); err != nil {
		t.Fatal(err)
	}
	if retryErr := newPitfall(testContext(t), root, []string{"Combined Hazard"}, io.Discard); retryErr == nil || !strings.Contains(retryErr.Error(), "duplicates") {
		t.Fatalf("same-title recovery retry = %v", retryErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "docs", "pitfalls", "combined-hazard-2.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("combined retry allocated suffix: %v", statErr)
	}
}

func newPitfallFileCensus(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(raw)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
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

func mustReadCLIFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// invariant: tooling/cli:pitfall-scaffold (TestRunNewDispatch)
// invariant: tooling/cli:cli-creation-and-inventory (TestRunNewDispatch)
func TestRunNewRejectsRetiredPlanKind(t *testing.T) {
	err := runNew(testContext(t), t.TempDir(), "plan", []string{"retired"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("retired plan kind error = %v", err)
	}
}

func TestRunNewDispatch(t *testing.T) {
	root := scaffoldProject(t)
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "new", "pitfall", "One Complete Title"}, &out, &errb); code != 0 {
		t.Fatalf("pitfall dispatch: exit %d (%s)", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := runFrom(root, []string{"awf", "new", "pitfall", "split", "title"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "unexpected arguments") {
		t.Fatalf("split pitfall title: exit %d (%s)", code, errb.String())
	}
}

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
	output := filepath.Join(root, "docs", "domains", "delivery.md")
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("rendered domain output missing: %v", err)
	}
	var removal bytes.Buffer
	if err := runRemoveDomain(ctx, root, "delivery", &removal); err != nil {
		t.Fatalf("remove domain: %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("rendered domain output survived removal: %v", err)
	}
	if got, err := os.ReadFile(part); err != nil || !bytes.Equal(got, before) {
		t.Fatalf("authored part changed by removal: %q, %v", got, err)
	}
	if !strings.Contains(removal.String(), "orphaned") {
		t.Fatalf("removal did not report surviving authored part: %q", removal.String())
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
