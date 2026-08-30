package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func topicCmdFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitfixture.InitNativeAt(t, root)
	testsupport.WriteAwfConfig(t, root, `prefix: example
integrationBranch: main
domains: [schedule]
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/schedule.yaml"), "paths: [\"internal/**\"]\n")
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{"prior": {}}}
	if err := lock.Save(filepath.Join(root, ".awf/awf.lock")); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-scheduling.md"), testsupport.ADR("Implemented", testsupport.WithTitle("0001: Scheduling origin"), testsupport.WithDomains("schedule"), testsupport.WithBody("## Decision\n\n1. Scheduling.\n")))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0002-revision.md"), testsupport.ADR("Implemented", testsupport.WithTitle("0002: Scheduling revision"), testsupport.WithDomains("schedule"), testsupport.WithBody("## Decision\n\n1. Revise scheduling.\n")))
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/schedule/contracts.yaml"), "title: Scheduling\nsummary: Current scheduling contracts.\npaths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/schedule/contracts/current-state.md"), `Scheduling contracts.

## Claims

### `+"`rule: deterministic-order`"+`
Jobs use deterministic order.
References: schedule/related:direct

### `+"`invariant: stable-output`"+`
Output remains stable.
Backing: test
`)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/schedule/related.yaml"), "title: Related\nsummary: Related scheduling contracts.\napplies: global\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/schedule/related/current-state.md"), `Related contracts.

## Claims

### `+"`rule: direct`"+`
A direct neighbor.
References: schedule/contracts:stable-output
`)
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule.go"), "package schedule\n// state: schedule/contracts:deterministic-order\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule_test.go"), "package schedule\n// invariant: schedule/contracts:stable-output (TestStableOutput)\nfunc TestStableOutput() {}\n")
	return root
}

func TestRunTopicHumanTextAndFlags(t *testing.T) {
	ctx := testContext(t)
	root := topicCmdFixture(t)
	var defaults bytes.Buffer
	if err := runReadTopic(ctx, root, "schedule/contracts", false, false, &defaults); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"identity: topic schedule/contracts", "title: Scheduling", "identity: schedule/contracts:deterministic-order", "backing: test"} {
		if !strings.Contains(defaults.String(), want) {
			t.Errorf("default output missing %q:\n%s", want, defaults.String())
		}
	}
	for _, tc := range []struct {
		name                 string
		references, coverage bool
		want                 string
	}{
		{"references", true, false, "outgoing: schedule/related:direct"},
		{"coverage", false, true, "marker: internal/schedule_test.go:2 | invariant"},
		{"combined", true, true, "selector-rule: both domain and topic selectors must match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := runReadTopic(ctx, root, "schedule/contracts", tc.references, tc.coverage, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("missing %q:\n%s", tc.want, out.String())
			}
		})
	}
	var claim bytes.Buffer
	if err := runReadTopic(ctx, root, "schedule/contracts:stable-output", false, false, &claim); err != nil || !strings.Contains(claim.String(), "identity: claim schedule/contracts:stable-output") || strings.Contains(claim.String(), "deterministic-order") {
		t.Fatalf("claim output: %v\n%s", err, claim.String())
	}
}

func TestPrintTopicPreservesLiteralIdentities(t *testing.T) {
	const identity = "domain/topic  name\tpart"
	result := topic.QueryResult{
		Kind: "topic",
		ID:   identity,
		References: []topic.ClaimReferences{{
			ClaimID:  identity + ":claim",
			Incoming: []string{"source/with  spaces"},
			Outgoing: []string{"target/with\ttab"},
		}},
		Coverage: &topic.QueryCoverage{Applicability: topic.TopicApplicability{
			DomainPaths:     []string{"internal/a  b/**"},
			TopicPaths:      []string{"internal/a\tb/**"},
			ApplicablePaths: []string{"internal/a  b/file.go"}, OwnedPaths: []string{"internal/a  b/file.go"},
			MarkerSites: []topic.MarkerSite{{Path: "internal/a  b/file_test.go", Line: 7, Kind: topic.ProofMarker, ClaimID: identity + ":claim"}},
		}},
	}
	var out bytes.Buffer
	if err := printTopicDetail(&out, result.Detail()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"identity: topic " + identity,
		"identity: " + identity + ":claim",
		"incoming: source/with  spaces",
		"outgoing: target/with\ttab",
		"    internal/a  b/**",
		"    internal/a\tb/**",
		"applicable-paths:",
		"owned-paths:",
		"marker: internal/a  b/file_test.go:7 | invariant | " + identity + ":claim",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("literal topic identity collapsed in %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "matched-paths") {
		t.Fatalf("retired coverage label remains:\n%s", out.String())
	}
}

type failOnWrite struct {
	failAt int
	calls  int
	err    error
}

func (w *failOnWrite) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		return 0, w.err
	}
	return len(p), nil
}

func TestPrintTopicPropagatesEveryHumanWriteFailure(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	sentinel := errors.New("writer failed")
	base := topic.QueryResult{
		Kind: "topic", ID: "schedule/contracts", Title: "Scheduling", Summary: "Summary.",
		Claims:     []topic.QueryClaim{{ID: "schedule/contracts:stable", Type: topic.Invariant, Prose: "Stable.", Backing: topic.Unbacked, Verify: "Inspect."}},
		References: []topic.ClaimReferences{{ClaimID: "schedule/contracts:stable", Incoming: []string{}, Outgoing: []string{"schedule/other:claim"}}},
		Coverage:   &topic.QueryCoverage{Applicability: topic.TopicApplicability{DomainPaths: []string{"internal/**"}, TopicPaths: []string{"internal/schedule*"}, ApplicablePaths: []string{"internal/schedule.go"}, OwnedPaths: []string{"internal/schedule.go"}, MarkerSites: []topic.MarkerSite{}}},
	}
	for _, result := range []topic.QueryResult{base, func() topic.QueryResult {
		global := base
		global.Coverage = &topic.QueryCoverage{Applicability: topic.TopicApplicability{DeclaredGlobal: true, DomainPaths: []string{}, TopicPaths: []string{}, ApplicablePaths: []string{}, OwnedPaths: []string{}, MarkerSites: []topic.MarkerSite{}}}
		return global
	}()} {
		counter := &failOnWrite{failAt: -1, err: sentinel}
		if err := printTopicDetail(counter, result.Detail()); err != nil {
			t.Fatal(err)
		}
		for failAt := 1; failAt <= counter.calls; failAt++ {
			writer := &failOnWrite{failAt: failAt, err: sentinel}
			err := printTopicDetail(writer, result.Detail())
			if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "write presentation") {
				t.Fatalf("write %d/%d error = %v", failAt, counter.calls, err)
			}
		}
	}
}

func TestPrintTopicOptionalHumanFields(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	result := topic.QueryResult{
		Kind: "claim", ID: "schedule/global:stable", Claims: []topic.QueryClaim{{
			ID: "schedule/global:stable", Type: topic.Invariant, Prose: "Stable.", Backing: topic.Unbacked, Verify: "Inspect output.",
		}},
		Coverage: &topic.QueryCoverage{Applicability: topic.TopicApplicability{DeclaredGlobal: true, DomainPaths: []string{}, TopicPaths: []string{}, ApplicablePaths: []string{}, OwnedPaths: []string{}, MarkerSites: []topic.MarkerSite{}}},
	}
	var out bytes.Buffer
	if err := printTopicDetail(&out, result.Detail()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"verify: Inspect output.", "declared: global"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("optional human output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunTopicStaticSyntaxGateAndErrors(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	if err := runReadTopic(ctx, t.TempDir(), "bad", false, false, io.Discard); err == nil || !strings.Contains(err.Error(), "expected <domain>/<topic>") {
		t.Fatalf("syntax error = %v", err)
	}
	var out bytes.Buffer
	if err := runReadTopic(ctx, t.TempDir(), "schedule/contracts", false, false, &out); err != nil {
		t.Fatalf("static = %v", err)
	}
	const staticGolden = "topic: static not inside an awf project\n\nreference:\n  description: Query active current-state topics and claims. Use references for direct claim IDs and coverage for scope and marker sites.\n"
	if out.String() != staticGolden {
		t.Fatalf("static output = %q, want %q", out.String(), staticGolden)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".awf"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runReadTopic(ctx, root, "schedule/contracts", false, false, io.Discard); err == nil {
		t.Fatal("stat fault accepted")
	}
	root = gateFixture(t, "99.0.0", migrate.Current())
	if err := runReadTopic(ctx, root, "schedule/contracts", false, false, io.Discard); err == nil {
		t.Fatal("version gate accepted ahead lock")
	}
	root = t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: \"\"\n")
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{"prior": {}}}
	if err := lock.Save(filepath.Join(root, ".awf/awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runReadTopic(ctx, root, "schedule/contracts", false, false, io.Discard); err == nil {
		t.Fatal("open error hidden")
	}
	root = topicCmdFixture(t)
	if err := runReadTopic(ctx, root, "schedule/missing", false, false, io.Discard); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing topic = %v", err)
	}
}

func TestReadTopicJSONIsRejectedBySpecAndDriver(t *testing.T) {
	read, ok := clispec.Lookup("read")
	if !ok {
		t.Fatal("read spec missing")
	}
	spec, ok := read.Child("topic")
	if !ok {
		t.Fatal("read topic spec missing")
	}
	if _, err := parseArgs(spec, []string{"schedule/contracts", "--json"}); err == nil || err.Error() != `awf topic: unknown flag "--json"` {
		t.Fatalf("read topic parser error = %v", err)
	}
	if strings.Contains(strings.Join(spec.BoolFlags, " "), "--json") || strings.Contains(strings.Join(spec.ValueFlags, " "), "--json") {
		t.Fatal("read topic spec exposes --json")
	}
	var help, stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "read", "topic", "--help"}, &help, &stderr); code != 0 || stderr.Len() != 0 || strings.Contains(help.String(), "--json") {
		t.Fatalf("read topic help exit=%d stdout=%q stderr=%q", code, help.String(), stderr.String())
	}
	if code := run([]string{"awf", "read", "topic", "schedule/contracts", "--json"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `awf read topic: unknown flag "--json"`) || strings.Contains(stderr.String(), `awf topic:`) {
		t.Fatalf("read topic --json exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

// invariant: tooling/authority-queries:authority-query-read-only (TestRunReadTopicDispatchAndReadOnly)
func TestRunReadTopicDispatchAndReadOnly(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"awf", "read", "topic"}, 2, "unexpected arguments"},
		{[]string{"awf", "read", "topic", "bad"}, 2, "invalid topic selector"},
		{[]string{"awf", "read", "topic", "schedule/contracts", "extra"}, 2, "unexpected arguments"},
		{[]string{"awf", "read", "topic", "schedule/contracts", "--unknown"}, 2, "unknown flag"},
	} {
		var out, errOut bytes.Buffer
		if code := run(tc.args, &out, &errOut); code != tc.code || !strings.Contains(errOut.String(), tc.want) {
			t.Errorf("run(%v) = %d, %s", tc.args, code, errOut.String())
		}
	}
	var help, errOut bytes.Buffer
	if code := run([]string{"awf", "read", "topic", "--help"}, &help, &errOut); code != 0 || !strings.Contains(help.String(), "command: awf read topic") {
		t.Fatalf("help = %d %s %s", code, help.String(), errOut.String())
	}
	root := t.TempDir()
	var static bytes.Buffer
	if code := runFrom(root, []string{"awf", "read", "topic", "schedule/contracts", "--coverage"}, &static, &errOut); code != 0 || !strings.Contains(static.String(), "static") {
		t.Fatalf("dispatch = %d %s %s", code, static.String(), errOut.String())
	}

	root = topicCmdFixture(t)
	fixture := gitfixture.At(root)
	gitfixture.NativeAdd(t, fixture, ".")
	beforeTree, beforeIndex := digestFiles(t, root), gitfixture.NativeWriteTree(t, fixture)
	for _, args := range [][]string{{"schedule/contracts"}, {"schedule/contracts:stable-output"}, {"schedule/contracts", "--references", "--coverage"}} {
		var out bytes.Buffer
		if err := runReadTopic(ctx, root, args[0], strings.Contains(strings.Join(args, " "), "--references"), strings.Contains(strings.Join(args, " "), "--coverage"), &out); err != nil {
			t.Fatal(err)
		}
	}
	if after := digestFiles(t, root); after != beforeTree {
		t.Fatalf("topic query mutated tree: %s != %s", after, beforeTree)
	}
	if after := gitfixture.NativeWriteTree(t, fixture); after != beforeIndex {
		t.Fatalf("topic query mutated index: %s != %s", after, beforeIndex)
	}
}

func digestFiles(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = h.Write([]byte(filepath.ToSlash(rel)))
		_, _ = h.Write(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
