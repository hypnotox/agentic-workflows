package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
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
	configPath := filepath.Join(root, ".awf", "config.yaml")
	configBody, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, configPath, strings.Replace(string(configBody), "domains: [schedule]", "domains: [schedule, shared]", 1))
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/shared.yaml"), "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/shared/overlap.yaml"), "title: Overlap\nsummary: Overlapping authority.\npaths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/shared/overlap/current-state.md"), "Overlap.\n\n## Claims\n\n### `rule: direct`\nShared authority.\nOrigin: ADR-0001\n")
	before := digestFiles(t, root)
	var out, stderr bytes.Buffer
	args := []string{"awf", "resolve", "topic", "internal/schedule.go", "future/file.go", "future/file.go", filepath.Join(root, "future", "other.go")}
	if code := runFrom(root, args, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	got := out.String()
	want := "query: topic resolution\n\n" +
		"resolution:\n  path: internal/schedule.go\n  domains:\n    schedule\n    shared\n  topics:\n    schedule/contracts\n    schedule/related\n    shared/overlap\n\n" +
		"resolution:\n  path: future/file.go\n  domains: none\n  topics:\n    schedule/related\n\n" +
		"resolution:\n  path: future/file.go\n  domains: none\n  topics:\n    schedule/related\n\n" +
		"resolution:\n  path: future/other.go\n  domains: none\n  topics:\n    schedule/related\n"
	if got != want {
		t.Fatalf("resolution document:\n%s\nwant:\n%s", got, want)
	}
	if after := digestFiles(t, root); after != before {
		t.Fatalf("resolve mutated tree: %s != %s", after, before)
	}
	for _, path := range []string{"../outside", "", "bad\npath"} {
		out.Reset()
		stderr.Reset()
		if code := runFrom(root, []string{"awf", "resolve", "topic", path}, &out, &stderr); code != 1 || !strings.Contains(stderr.String(), "resolve topic:") || path == "bad\npath" && !strings.Contains(stderr.String(), "malformed") {
			t.Errorf("path %q exit=%d stderr=%q", path, code, stderr.String())
		}
	}
}

// invariant: tooling/authority-queries:unowned-path-census (TestResolveTopicUncoveredCommand)
// invariant: invariants/current-state-authority:uncovered-lists-unowned (TestResolveTopicUncoveredCommand)
func TestResolveTopicUncoveredCommand(t *testing.T) {
	root := topicCmdFixture(t)
	configPath := filepath.Join(root, ".awf", "config.yaml")
	configBody, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, configPath, string(configBody)+"contextIgnore: [\"ignored/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, "unowned", "one.txt"), "one\n")
	testsupport.WriteFile(t, filepath.Join(root, "unowned", "two.txt"), "two\n")
	testsupport.WriteFile(t, filepath.Join(root, "ignored", "still-reported.txt"), "ignored only by context coverage\n")
	testsupport.WriteFile(t, filepath.Join(root, "prior"), "generated\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "efforts", "resident", "memory.md"), "resident\n")
	testsupport.WriteFile(t, filepath.Join(root, "nested", ".awf", "config.yaml"), "prefix: nested\nintegrationBranch: main\n")
	testsupport.WriteFile(t, filepath.Join(root, "nested", "excluded.txt"), "nested\n")
	var out, stderr bytes.Buffer
	if code := runFrom(root, []string{"awf", "resolve", "topic", "--uncovered"}, &out, &stderr); code != 0 {
		t.Fatalf("uncovered exit=%d stderr=%q", code, stderr.String())
	}
	if got := out.String(); !strings.Contains(got, "unowned/") || !strings.Contains(got, "ignored/") || strings.Contains(got, "nested/") || strings.Contains(got, "prior") || strings.Contains(got, ".awf/efforts") {
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

func TestReadADRCommandCanonicalIdentityLinksAndProgressPartitions(t *testing.T) {
	root := topicCmdFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0005-progress.md"), authorityV3ADR(t, "0005", "progress", "Implementing", "- add `schedule/contracts:phase-one-applied`\n- add `schedule/contracts:pending`\n- update `schedule/contracts:deterministic-order`"))
	topicPath := filepath.Join(root, ".awf/topics/parts/schedule/contracts/current-state.md")
	topicBody, err := os.ReadFile(topicPath)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, topicPath, string(topicBody)+"\n### `rule: phase-one-applied`\nApplied fixture claim.\nOrigin: ADR-0005\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0006-canceled.md"), authorityV3ADR(t, "0006", "canceled", "Abandoned", "- add `schedule/contracts:canceled-add`\n- update `schedule/contracts:deterministic-order`"))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0007-alias-record.md"), authorityV3ADR(t, "0007", "alias-record", "Proposed", "None."))
	for name, identity := range map[string]string{"2026-08-29-number-link.md": "0007", "2026-08-29-slug-link.md": "alias-record"} {
		body := strings.Replace(readCommandV2Plan, "adrs: [fixture, context, third]", "adrs: [\""+identity+"\"]", 1)
		testsupport.WriteFile(t, filepath.Join(root, "docs/plans", name), body)
	}

	read := func(identity string) string {
		t.Helper()
		var out, stderr bytes.Buffer
		if code := runFrom(root, []string{"awf", "read", "adr", identity}, &out, &stderr); code != 0 {
			t.Fatalf("read adr %s exit=%d stderr=%q", identity, code, stderr.String())
		}
		return out.String()
	}
	implementing := read("0005")
	for _, want := range []string{"status: Implementing", "applied:\n    add schedule/contracts:phase-one-applied", "remaining:\n    add schedule/contracts:pending\n    update schedule/contracts:deterministic-order", "canceled: none"} {
		if !strings.Contains(implementing, want) {
			t.Errorf("Implementing projection missing %q:\n%s", want, implementing)
		}
	}
	abandoned := read("0006")
	for _, want := range []string{"status: Abandoned", "applied: none", "remaining: none", "canceled:\n    add schedule/contracts:canceled-add\n    update schedule/contracts:deterministic-order"} {
		if !strings.Contains(abandoned, want) {
			t.Errorf("Abandoned projection missing %q:\n%s", want, abandoned)
		}
	}
	numbered, slugged := read("0007"), read("alias-record")
	if numbered != slugged {
		t.Fatalf("identity aliases produced different projections:\nnumbered:\n%s\nslugged:\n%s", numbered, slugged)
	}
	for _, want := range []string{"identity: ADR-0007", "2026-08-29-number-link.md", "2026-08-29-slug-link.md"} {
		if !strings.Contains(numbered, want) {
			t.Errorf("canonical alias projection missing %q:\n%s", want, numbered)
		}
	}
}

func TestReadADRCommandReportsPlanDiagnostic(t *testing.T) {
	root := topicCmdFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-29-broken.md"), "---\nformat: plan-v2\nadrs: [\n---\n")
	var out, stderr bytes.Buffer
	if code := runFrom(root, []string{"awf", "read", "adr", "0003"}, &out, &stderr); code != 1 {
		t.Fatalf("malformed plan exit=%d output=%q stderr=%q", code, out.String(), stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "docs/plans/2026-08-29-broken.md") || strings.Contains(got, "malformed plan") {
		t.Fatalf("plan diagnostic = %q", got)
	}
}

func authorityV3ADR(t *testing.T, number, slug, status, changes string) string {
	t.Helper()
	build := func(recordStatus, history string) string {
		return "---\nformat: current-state-v3\nslug: " + slug + "\nstatus: " + recordStatus + "\ndate: 2026-08-29\n---\n# ADR-" + number + ": Authority Fixture\n\n## Context\n\nFixture context.\n\n## Decision\n\n1. Fixture decision.\n\n## State changes\n\n" + changes + "\n\n## Consequences\n\nFixture consequence.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n" + history + "\n"
	}
	proposedHistory := "- 2026-08-29: Proposed"
	proposed := build("Proposed", proposedHistory)
	record, err := adr.ParseV3(number+"-"+slug+".md", []byte(proposed))
	if err != nil {
		t.Fatalf("parse Proposed authority fixture: %v", err)
	}
	digest := adr.ContentDigest(record.Sections)
	switch status {
	case "Proposed":
		return proposed
	case "Implementing":
		return build(status, proposedHistory+"\n- 2026-08-29: Implementing; content-sha256: "+digest+"\n- 2026-08-29: Applied; operations: add `schedule/contracts:phase-one-applied`")
	case "Abandoned":
		return build(status, proposedHistory+"\n- 2026-08-29: Abandoned; content-sha256: "+digest+"; rationale: fixture stopped")
	default:
		t.Fatalf("unsupported authority fixture status %q", status)
		return ""
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
