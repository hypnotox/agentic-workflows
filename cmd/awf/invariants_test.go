package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invTopicYAML configures a domain owning internal/**, a marker source, and a
// test-backing glob so an invariant claim can carry its proof marker.
const invTopicYAML = `prefix: example
skills: []
agents: []
domains:
  - alpha
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`

// invClaimPart carries a test-backed and an unbacked invariant claim plus a rule.
const invClaimPart = "Intro.\n\n## Claims\n\n" +
	"### `rule: r`\nA rule.\nOrigin: ADR-0001\n\n" +
	"### `invariant: backed`\nBacked.\nOrigin: ADR-0001\nBacking: test\n\n" +
	"### `invariant: reasoned`\nReasoned.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: inspect.\n"

// invFirstADR is a legacy Implemented ADR the topic claims can cite as Origin.
func invFirstADR() string {
	return testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"),
		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n"))
}

// TestRunInvariantsReportsClaims proves runInvariants reports the topic corpus's
// invariant claims: backing mode, an unbacked claim's Verify guidance, and a
// test-backed claim's proof-marker site. Rule claims never appear.
func TestRunInvariantsReportsClaims(t *testing.T) {
	dir := gitProjectFiles(t, invTopicYAML, map[string]string{
		"docs/decisions/0001-first.md":                 invFirstADR(),
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/one/current-state.md": invClaimPart,
		"internal/foo.go":                              "package foo\n",
		"internal/foo_test.go":                         "package foo\n// invariant: alpha/one:backed\n",
	})
	var buf bytes.Buffer
	if err := runInvariants(dir, &buf); err != nil {
		t.Fatalf("runInvariants: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"alpha/one:backed [test]", "proof: internal/foo_test.go:", "alpha/one:reasoned [unbacked]", "Verify: inspect."} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "alpha/one:r [") {
		t.Errorf("rule claim must not appear in the invariants report:\n%s", out)
	}
}

// TestRunInvariantsEmpty proves a project with no invariant claims reports none.
func TestRunInvariantsEmpty(t *testing.T) {
	dir := gitProjectFiles(t, "prefix: example\nskills: []\nagents: []\n", nil)
	var buf bytes.Buffer
	if err := runInvariants(dir, &buf); err != nil {
		t.Fatalf("runInvariants: %v", err)
	}
	if !strings.Contains(buf.String(), "no invariant claims") {
		t.Errorf("got %q", buf.String())
	}
}

// TestRunInvariantsLoadError proves a backing-contract violation surfaces as an
// error (a load error), not a reported entry: a test-backed invariant with no
// proof marker.
func TestRunInvariantsLoadError(t *testing.T) {
	dir := gitProjectFiles(t, invTopicYAML, map[string]string{
		"docs/decisions/0001-first.md":                 invFirstADR(),
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `invariant: backed`\nBacked.\nOrigin: ADR-0001\nBacking: test\n",
		"internal/foo.go":                              "package foo\n",
	})
	if err := runInvariants(dir, io.Discard); err == nil {
		t.Fatal("expected a load error for the test-backed invariant with no proof marker")
	}
}
