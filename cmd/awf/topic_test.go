package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func topicCmdFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	testsupport.WriteAwfConfig(t, root, `prefix: example
skills: []
agents: []
domains: [schedule]
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/schedule.yaml"), "paths: [\"internal/**\"]\n")
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), ADRFormatV1From: 3, ADRFormatV2From: 9999, LegacyADRGaps: []int{}, Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf/awf.lock")); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-scheduling.md"), testsupport.ADR("Implemented", testsupport.WithTitle("0001: Scheduling origin"), testsupport.WithDomains("schedule"), testsupport.WithBody("## Decision\n\n1. Scheduling.\n")))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0002-revision.md"), testsupport.ADR("Implemented", testsupport.WithTitle("0002: Scheduling revision"), testsupport.WithDomains("schedule"), testsupport.WithBody("## Decision\n\n1. Revise scheduling.\n")))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0003-update-active.md"), topicV1ADR(t, "0003", "Update active claim", "- update `schedule/contracts:deterministic-order`", 1))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0004-remove-old.md"), topicV1ADR(t, "0004", "Remove legacy claim", "- remove `schedule/contracts:removed`", 2))
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/schedule/contracts.yaml"), "title: Scheduling\nsummary: Current scheduling contracts.\npaths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/schedule/contracts/current-state.md"), `Scheduling contracts.

## Claims

### `+"`rule: deterministic-order`"+`
Jobs use deterministic order.
Origin: ADR-0001
Revised-by: ADR-0003
References: schedule/related:direct

### `+"`invariant: stable-output`"+`
Output remains stable.
Origin: ADR-0001
Backing: test
`)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/schedule/related.yaml"), "title: Related\nsummary: Related scheduling contracts.\napplies: global\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/schedule/related/current-state.md"), `Related contracts.

## Claims

### `+"`rule: direct`"+`
A direct neighbor.
Origin: ADR-0001
References: schedule/contracts:stable-output
`)
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule.go"), "package schedule\n// state: schedule/contracts:deterministic-order\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule_test.go"), "package schedule\n// invariant: schedule/contracts:stable-output\n")
	return root
}

func topicV1ADR(t *testing.T, number, title, operation string, sequence int) string {
	t.Helper()
	build := func(status, history string) string {
		return "---\nformat: current-state-v1\nstatus: " + status + "\ndate: 2026-07-21\n---\n" +
			"# ADR-" + number + ": " + title + "\n\n" +
			"## Context\n\nContext.\n\n## Decision\n\n1. Change state.\n\n" +
			"## State changes\n\n" + operation + "\n\n## Consequences\n\nConsequence.\n\n" +
			"## Alternatives Considered\n\nNone.\n\n## Status history\n\n" + history + "\n"
	}
	proposed, err := adr.ParseV1(number+"-query.md", []byte(build("Proposed", "- 2026-07-20: Proposed")))
	if err != nil {
		t.Fatal(err)
	}
	digest := adr.ContentDigest(proposed.Sections)
	return build("Implemented", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+digest+"; state-sequence: "+strconv.Itoa(sequence))
}

func TestRunTopicHistoricalOnlyHumanJSON(t *testing.T) {
	root := topicCmdFixture(t)
	claimID := "schedule/contracts:removed"
	if err := runTopic(root, claimID, false, false, false, false, io.Discard); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("default removed-claim query = %v", err)
	}

	var human bytes.Buffer
	if err := runTopic(root, claimID, true, true, true, false, &human); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"claim " + claimID,
		"historical only - no active claim",
		"origin: legacy baseline (not retained in active authority)",
		"Removed-by: ADR-0004 (Implemented) Remove legacy claim",
	} {
		if !strings.Contains(human.String(), want) {
			t.Errorf("historical human output missing %q:\n%s", want, human.String())
		}
	}
	for _, fabricated := range []string{"Incoming:", "Outgoing:", "## Coverage", "[backing:"} {
		if strings.Contains(human.String(), fabricated) {
			t.Errorf("historical human output fabricated %q:\n%s", fabricated, human.String())
		}
	}

	var encoded bytes.Buffer
	if err := runTopic(root, claimID, true, true, true, true, &encoded); err != nil {
		t.Fatal(err)
	}
	var result topic.QueryResult
	if err := json.Unmarshal(encoded.Bytes(), &result); err != nil {
		t.Fatalf("JSON: %v\n%s", err, encoded.String())
	}
	if !result.HistoricalOnly || result.Kind != "claim" || result.ID != claimID || result.Title != "" || result.Summary != "" || result.Claims == nil || len(result.Claims) != 0 || len(result.History) != 1 || !result.History[0].LegacyBaseline || result.History[0].Origin != nil || result.History[0].RemovedBy == nil {
		t.Fatalf("historical JSON projection = %#v", result)
	}
	for _, want := range []string{`"historicalOnly": true`, `"claims": []`, `"legacyBaseline": true`, `"removedBy": {`} {
		if !strings.Contains(encoded.String(), want) {
			t.Errorf("historical JSON missing %q:\n%s", want, encoded.String())
		}
	}
	for _, fabricated := range []string{`"references"`, `"coverage"`, `"summary"`, `"origin"`} {
		if strings.Contains(encoded.String(), fabricated) {
			t.Errorf("historical JSON fabricated %q:\n%s", fabricated, encoded.String())
		}
	}
}

func TestRunTopicHumanJSONAndFlags(t *testing.T) {
	root := topicCmdFixture(t)
	var defaults bytes.Buffer
	if err := runTopic(root, "schedule/contracts", false, false, false, false, &defaults); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"topic schedule/contracts", "Title: Scheduling", "deterministic-order [rule] [backing: none]", "stable-output [invariant] [backing: test]"} {
		if !strings.Contains(defaults.String(), want) {
			t.Errorf("default output missing %q:\n%s", want, defaults.String())
		}
	}
	for _, hidden := range []string{"Origin:", "Revised-by:", "Incoming:", "Outgoing:"} {
		if strings.Contains(defaults.String(), hidden) {
			t.Errorf("default output leaked %q", hidden)
		}
	}
	for _, tc := range []struct {
		name                          string
		history, references, coverage bool
		want                          string
	}{
		{"history", true, false, false, "ADR-0003 (Implemented) Update active claim"},
		{"references", false, true, false, "Outgoing: [schedule/related:direct]"},
		{"coverage", false, false, true, "Marker: internal/schedule_test.go:2 [invariant]"},
		{"combined", true, true, true, "Effective: domain internal/** + topic internal/**"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := runTopic(root, "schedule/contracts", tc.history, tc.references, tc.coverage, false, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("missing %q:\n%s", tc.want, out.String())
			}
		})
	}
	var claim bytes.Buffer
	if err := runTopic(root, "schedule/contracts:stable-output", false, false, false, false, &claim); err != nil || !strings.Contains(claim.String(), "claim schedule/contracts:stable-output") || strings.Contains(claim.String(), "deterministic-order") {
		t.Fatalf("claim output: %v\n%s", err, claim.String())
	}
	var encoded bytes.Buffer
	if err := runTopic(root, "schedule/contracts", true, true, true, true, &encoded); err != nil {
		t.Fatal(err)
	}
	var result topic.QueryResult
	if err := json.Unmarshal(encoded.Bytes(), &result); err != nil {
		t.Fatalf("JSON: %v\n%s", err, encoded.String())
	}
	if result.ID != "schedule/contracts" || len(result.Claims) != 2 || result.Claims[0].Backing != topic.ExplicitNoBacking || len(result.History) != 2 || len(result.References) != 2 || result.Coverage == nil {
		t.Fatalf("JSON projection = %#v", result)
	}
	if !strings.Contains(encoded.String(), `"backing": "none"`) || !strings.Contains(defaults.String(), "[backing: none]") {
		t.Fatalf("rule backing differs across JSON and human projections:\nJSON: %s\nhuman: %s", encoded.String(), defaults.String())
	}
	for _, semantic := range []string{result.Title, result.Claims[0].ID, result.History[0].Origin.Title, result.References[0].Outgoing[0], result.Coverage.MarkerSites[0].Path} {
		if !strings.Contains(runTopicHuman(t, root), semantic) {
			t.Errorf("human/JSON parity missing %q", semantic)
		}
	}
}

func TestPrintTopicHistoryStateSequenceHumanJSON(t *testing.T) {
	result := topic.QueryResult{
		Kind: "topic", ID: "d/t", Claims: []topic.QueryClaim{},
		History: []topic.ClaimHistory{
			{ClaimID: "d/t:x", Origin: &topic.ADRHistory{Number: "0001", Title: "Origin", Status: "Implemented", StateSequence: 3}, RevisedBy: []topic.ADRHistory{{Number: "0002", Title: "Revision", Status: "Implementing", StateSequence: 4}}, RemovedBy: &topic.ADRHistory{Number: "0003", Title: "Removal", Status: "Abandoned", StateSequence: 5}},
			{ClaimID: "d/t:legacy", Origin: &topic.ADRHistory{Number: "0100", Title: "Legacy origin", Status: "Implemented"}, RevisedBy: []topic.ADRHistory{}},
		},
	}
	var human bytes.Buffer
	if err := printTopic(&human, result, false); err != nil {
		t.Fatal(err)
	}
	wantHuman := "topic d/t\n\n## Claims\n\n## History\nd/t:x\n  Origin: ADR-0001 (Implemented) Origin [state-sequence: 3]\n  Revised-by: ADR-0002 (Implementing) Revision [state-sequence: 4]\n  Removed-by: ADR-0003 (Abandoned) Removal [state-sequence: 5]\nd/t:legacy\n  Origin: ADR-0100 (Implemented) Legacy origin\n"
	if human.String() != wantHuman {
		t.Fatalf("human history:\n%s\nwant:\n%s", human.String(), wantHuman)
	}

	var encoded bytes.Buffer
	if err := printTopic(&encoded, result, true); err != nil {
		t.Fatal(err)
	}
	wantJSON := "{\n  \"kind\": \"topic\",\n  \"id\": \"d/t\",\n  \"claims\": [],\n  \"history\": [\n    {\n      \"claimId\": \"d/t:x\",\n      \"origin\": {\n        \"number\": \"0001\",\n        \"title\": \"Origin\",\n        \"status\": \"Implemented\",\n        \"stateSequence\": 3\n      },\n      \"revisedBy\": [\n        {\n          \"number\": \"0002\",\n          \"title\": \"Revision\",\n          \"status\": \"Implementing\",\n          \"stateSequence\": 4\n        }\n      ],\n      \"removedBy\": {\n        \"number\": \"0003\",\n        \"title\": \"Removal\",\n        \"status\": \"Abandoned\",\n        \"stateSequence\": 5\n      }\n    },\n    {\n      \"claimId\": \"d/t:legacy\",\n      \"origin\": {\n        \"number\": \"0100\",\n        \"title\": \"Legacy origin\",\n        \"status\": \"Implemented\"\n      },\n      \"revisedBy\": []\n    }\n  ]\n}\n"
	if encoded.String() != wantJSON {
		t.Fatalf("JSON history:\n%s\nwant:\n%s", encoded.String(), wantJSON)
	}
}

func runTopicHuman(t *testing.T, root string) string {
	t.Helper()
	var out bytes.Buffer
	if err := runTopic(root, "schedule/contracts", true, true, true, false, &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
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
	sentinel := errors.New("writer failed")
	base := topic.QueryResult{
		Kind: "topic", ID: "schedule/contracts", Title: "Scheduling", Summary: "Summary.",
		Claims:     []topic.QueryClaim{{ID: "schedule/contracts:stable", Type: topic.Invariant, Prose: "Stable.", Backing: topic.Unbacked, Verify: "Inspect."}},
		History:    []topic.ClaimHistory{{ClaimID: "schedule/contracts:stable", Origin: &topic.ADRHistory{Number: "0001", Status: "Implemented", Title: "Origin"}, RevisedBy: []topic.ADRHistory{{Number: "0002", Status: "Implemented", Title: "Revision"}}}},
		References: []topic.ClaimReferences{{ClaimID: "schedule/contracts:stable", Incoming: []string{}, Outgoing: []string{"schedule/other:claim"}}},
		Coverage:   &topic.QueryCoverage{DeclaredPaths: []string{"internal/**"}, EffectiveSelectors: []topic.EffectiveSelector{{DomainPath: "internal/**", TopicPath: "internal/schedule*"}}, MarkerSites: []topic.MarkerSite{{Path: "internal/schedule.go", Line: 2, Kind: topic.TouchesMarker, ClaimID: "schedule/contracts:stable", Note: "entry"}}},
	}
	for _, result := range []topic.QueryResult{base, func() topic.QueryResult {
		global := base
		global.Coverage = &topic.QueryCoverage{DeclaredGlobal: true}
		return global
	}()} {
		counter := &failOnWrite{failAt: -1, err: sentinel}
		if err := printTopic(counter, result, false); err != nil {
			t.Fatal(err)
		}
		for failAt := 1; failAt <= counter.calls; failAt++ {
			writer := &failOnWrite{failAt: failAt, err: sentinel}
			err := printTopic(writer, result, false)
			if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "write human topic") {
				t.Fatalf("write %d/%d error = %v", failAt, counter.calls, err)
			}
		}
	}
}

func TestPrintTopicOptionalHumanFields(t *testing.T) {
	result := topic.QueryResult{
		Kind: "claim", ID: "schedule/global:stable", Claims: []topic.QueryClaim{{
			ID: "schedule/global:stable", Type: topic.Invariant, Prose: "Stable.", Backing: topic.Unbacked, Verify: "Inspect output.",
		}},
		Coverage: &topic.QueryCoverage{DeclaredGlobal: true, MarkerSites: []topic.MarkerSite{{
			Path: "internal/schedule.go", Line: 2, Kind: topic.TouchesMarker, ClaimID: "schedule/global:stable", Note: "entry point",
		}}},
	}
	var out bytes.Buffer
	if err := printTopic(&out, result, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Verify: Inspect output.", "Declared: global", " - entry point"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("optional human output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunTopicStaticSyntaxGateAndErrors(t *testing.T) {
	if err := runTopic(t.TempDir(), "bad", false, false, false, false, io.Discard); err == nil || !strings.Contains(err.Error(), "expected <domain>/<topic>") {
		t.Fatalf("syntax error = %v", err)
	}
	for _, asJSON := range []bool{false, true} {
		var out bytes.Buffer
		if err := runTopic(t.TempDir(), "schedule/contracts", false, false, false, asJSON, &out); err != nil || !strings.Contains(out.String(), "static: not inside") {
			t.Fatalf("static = %v, %s", err, out.String())
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".awf"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runTopic(root, "schedule/contracts", false, false, false, false, io.Discard); err == nil {
		t.Fatal("stat fault accepted")
	}
	root = gateFixture(t, "99.0.0", migrate.Current())
	if err := runTopic(root, "schedule/contracts", false, false, false, false, io.Discard); err == nil {
		t.Fatal("version gate accepted ahead lock")
	}
	root = t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: \"\"\n")
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf/awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runTopic(root, "schedule/contracts", false, false, false, false, io.Discard); err == nil {
		t.Fatal("open error hidden")
	}
	root = topicCmdFixture(t)
	if err := runTopic(root, "schedule/missing", true, false, false, false, io.Discard); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing topic = %v", err)
	}
}

func TestRunTopicDispatchAndReadOnly(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"awf", "topic"}, 2, "unexpected arguments"},
		{[]string{"awf", "topic", "bad"}, 2, "invalid topic selector"},
		{[]string{"awf", "topic", "schedule/contracts", "extra"}, 2, "unexpected arguments"},
		{[]string{"awf", "topic", "schedule/contracts", "--unknown"}, 2, "unknown flag"},
	} {
		var out, errOut bytes.Buffer
		if code := run(tc.args, &out, &errOut); code != tc.code || !strings.Contains(errOut.String(), tc.want) {
			t.Errorf("run(%v) = %d, %s", tc.args, code, errOut.String())
		}
	}
	var help, errOut bytes.Buffer
	if code := run([]string{"awf", "topic", "--help"}, &help, &errOut); code != 0 || !strings.Contains(help.String(), "Usage: awf topic") {
		t.Fatalf("help = %d %s %s", code, help.String(), errOut.String())
	}
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var static bytes.Buffer
	if code := run([]string{"awf", "topic", "schedule/contracts", "--coverage"}, &static, &errOut); code != 0 || !strings.Contains(static.String(), "static") {
		t.Fatalf("dispatch = %d %s %s", code, static.String(), errOut.String())
	}

	root = topicCmdFixture(t)
	runGit(t, root, "init")
	runGit(t, root, "add", ".")
	beforeTree, beforeIndex := digestFiles(t, root), runGit(t, root, "write-tree")
	for _, args := range [][]string{{"schedule/contracts"}, {"schedule/contracts:stable-output", "--json"}, {"schedule/contracts", "--history", "--references", "--coverage"}} {
		var out bytes.Buffer
		if err := runTopic(root, args[0], strings.Contains(strings.Join(args, " "), "--history"), strings.Contains(strings.Join(args, " "), "--references"), strings.Contains(strings.Join(args, " "), "--coverage"), strings.Contains(strings.Join(args, " "), "--json"), &out); err != nil {
			t.Fatal(err)
		}
	}
	if after := digestFiles(t, root); after != beforeTree {
		t.Fatalf("topic query mutated tree: %s != %s", after, beforeTree)
	}
	if after := runGit(t, root, "write-tree"); after != beforeIndex {
		t.Fatalf("topic query mutated index: %s != %s", after, beforeIndex)
	}
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
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
