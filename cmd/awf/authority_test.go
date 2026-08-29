package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: tooling/authority-queries:authority-read-projections (TestReadTopicCommandMatchesLegacyProjection)
// invariant: tooling/authority-queries:authority-query-read-only (TestReadTopicCommandMatchesLegacyProjection)
func TestReadTopicCommandMatchesLegacyProjection(t *testing.T) {
	root := topicCmdFixture(t)
	var legacy, read, stderr bytes.Buffer
	if code := runFrom(root, []string{"awf", "topic", "schedule/contracts", "--history", "--references", "--coverage"}, &legacy, &stderr); code != 0 {
		t.Fatalf("legacy exit=%d stderr=%q", code, stderr.String())
	}
	if code := runFrom(root, []string{"awf", "read", "topic", "schedule/contracts", "--history", "--references", "--coverage"}, &read, &stderr); code != 0 {
		t.Fatalf("read exit=%d stderr=%q", code, stderr.String())
	}
	if read.String() != legacy.String() {
		t.Fatalf("read topic changed projection:\n%s\nwant:\n%s", read.String(), legacy.String())
	}
}

// invariant: tooling/authority-queries:path-topic-resolution (TestResolveTopicCommandLexicalAttribution)
// invariant: tooling/authority-queries:authority-query-read-only (TestResolveTopicCommandLexicalAttribution)
func TestResolveTopicCommandLexicalAttribution(t *testing.T) {
	root := topicCmdFixture(t)
	before := digestFiles(t, root)
	var out, stderr bytes.Buffer
	args := []string{"awf", "resolve", "topic", "internal/schedule.go", "future/file.go", "future/file.go", filepath.Join(root, "future", "other.go")}
	if code := runFrom(root, args, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	got := out.String()
	for _, want := range []string{
		"path: internal/schedule.go", "domains:\n    schedule", "topics:\n    schedule/contracts", "schedule/related",
		"path: future/file.go", "path: future/other.go", "domains: none", "topics:\n    schedule/related",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("resolution missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "path: future/file.go") != 2 {
		t.Errorf("duplicate input was not preserved:\n%s", got)
	}
	if after := digestFiles(t, root); after != before {
		t.Fatalf("resolve mutated tree: %s != %s", after, before)
	}
	for _, path := range []string{"../outside", ""} {
		out.Reset()
		stderr.Reset()
		if code := runFrom(root, []string{"awf", "resolve", "topic", path}, &out, &stderr); code != 1 || !strings.Contains(stderr.String(), "resolve topic:") {
			t.Errorf("path %q exit=%d stderr=%q", path, code, stderr.String())
		}
	}
}

// invariant: tooling/authority-queries:unowned-path-census (TestResolveTopicUncoveredCommand)
func TestResolveTopicUncoveredCommand(t *testing.T) {
	root := topicCmdFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "unowned", "one.txt"), "one\n")
	testsupport.WriteFile(t, filepath.Join(root, "unowned", "two.txt"), "two\n")
	testsupport.WriteFile(t, filepath.Join(root, "nested", ".awf", "config.yaml"), "prefix: nested\nintegrationBranch: main\n")
	testsupport.WriteFile(t, filepath.Join(root, "nested", "excluded.txt"), "nested\n")
	var out, stderr bytes.Buffer
	if code := runFrom(root, []string{"awf", "resolve", "topic", "--uncovered"}, &out, &stderr); code != 0 {
		t.Fatalf("uncovered exit=%d stderr=%q", code, stderr.String())
	}
	if got := out.String(); !strings.Contains(got, "unowned/") || strings.Contains(got, "nested/") {
		t.Fatalf("uncovered output = %s", got)
	}
	for _, args := range [][]string{{"awf", "resolve", "topic"}, {"awf", "resolve", "topic", "--uncovered", "unowned"}} {
		out.Reset()
		stderr.Reset()
		if code := runFrom(root, args, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage: awf resolve topic") {
			t.Errorf("%v: exit=%d stderr=%q", args, code, stderr.String())
		}
	}
}

// invariant: tooling/authority-queries:authority-read-projections (TestReadADRCommandProgressAndAbsence)
func TestReadADRCommandProgressAndAbsence(t *testing.T) {
	root := topicCmdFixture(t)
	linkedPlan := strings.Replace(readCommandV2Plan, "adrs: [fixture, context, third]", `adrs: ["0003"]`, 1)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-29-linked.md"), linkedPlan)
	var out, stderr bytes.Buffer
	if code := runFrom(root, []string{"awf", "read", "adr", "0003"}, &out, &stderr); code != 0 {
		t.Fatalf("read adr exit=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"identity: ADR-0003", "status: Implemented", "applied:", "remaining: none", "canceled: none", "linked-plans:\n    2026-08-29-linked.md"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("ADR output missing %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	stderr.Reset()
	if code := runFrom(root, []string{"awf", "read", "adr", "missing"}, &out, &stderr); code != 1 || !strings.Contains(stderr.String(), "ADR-missing not found") {
		t.Fatalf("missing ADR exit=%d stderr=%q", code, stderr.String())
	}
}

// invariant: tooling/authority-queries:authority-query-full-profile-only (TestAuthorityCommandsAreFullOnly)
func TestAuthorityCommandsAreFullOnly(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\n")
	for _, args := range [][]string{{"awf", "read", "topic", "x/y"}, {"awf", "read", "adr", "0001"}, {"awf", "resolve", "topic", "README.md"}} {
		var out, stderr bytes.Buffer
		if code := runFrom(root, args, &out, &stderr); code != 1 || !strings.Contains(stderr.String(), "selected core governance footprint") {
			t.Errorf("%v: exit=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunResolveTopicUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "resolve", "topic"}, &out, &errb); code != 2 {
		t.Fatalf("exit = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "awf resolve topic <path>...") {
		t.Fatalf("stderr = %q", errb.String())
	}
}
