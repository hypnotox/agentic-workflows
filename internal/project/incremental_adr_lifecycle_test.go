package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestIncrementalADRLifecyclePublicPairs exercises incremental lifecycle history
// through the project staged checker and range audit rather than private
// currentstate helpers.
func TestIncrementalADRLifecyclePublicPairs(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = lockJSON(t, &manifest.Lock{AWFVersion: "0.20.0", SchemaVersion: 46, Files: map[string]manifest.Entry{}})
	v1Ops := "- add `alpha/one:v1`"
	files["docs/decisions/0002-v1-direct.md"] = strings.Replace(publicV2ADR(t, "0002", "V1 direct", "Proposed", v1Ops, ""), adr.V2FormatMarker, adr.V1FormatMarker, 1)
	gitfixture.Stage(t, repo, files)
	base := gitfixture.Commit(t, repo, "chore(config): adopt authority", nil)
	p := openStaged(t, dir)

	checkAndCommit := func(subject string, changes map[string]string) {
		t.Helper()
		gitfixture.Stage(t, repo, changes)
		report, err := checkStagedProject(p, testContext(t))
		if err != nil {
			t.Fatalf("%s staged check: %v", subject, err)
		}
		if got := currentStateFindings(report); len(got) != 0 {
			t.Fatalf("%s findings = %v", subject, got)
		}
		gitfixture.Commit(t, repo, subject, nil)
	}

	// Perform a real V1 Proposed -> Implemented operation transaction through
	// CheckStaged before exercising any V2 shape.
	v1Done := strings.Replace(publicV2ADR(t, "0002", "V1 direct", "Implemented", v1Ops,
		"- 2026-07-21: Implemented; content-sha256: %s"), adr.V2FormatMarker, adr.V1FormatMarker, 1)
	checkAndCommit("feat(invariants): apply v1 direct state", map[string]string{
		"docs/decisions/0002-v1-direct.md":             v1Done,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1"),
	})

	ops3 := "- add `alpha/one:a`\n- add `alpha/one:b`\n- add `alpha/one:c`"
	first3 := publicV2ADR(t, "0003", "Incremental", "Implementing", ops3,
		"- 2026-07-21: Implementing; content-sha256: %s\n- 2026-07-21: Applied; operations: add `alpha/one:c`, add `alpha/one:b`")
	checkAndCommit("feat(invariants): apply out-of-order first batch", map[string]string{
		"docs/decisions/0003-incremental.md":           first3,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "b", "c"),
	})
	corpus, err := adr.LoadCorpus(filepath.Join(dir, "docs", "decisions"))
	if err != nil {
		t.Fatal(err)
	}
	if index := adr.RenderIndexMD(corpus); !strings.Contains(index, "ADR-0003") || !strings.Contains(index, "Implementing") || strings.Index(index, "ADR-0003") > strings.Index(index, "## History") {
		t.Fatalf("Implementing ADR not in flight:\n%s", index)
	}

	// A direct batch from another ADR interleaves with the in-flight ADR.
	ops4 := "- add `alpha/one:x`"
	direct4 := publicV2ADR(t, "0004", "Interleave", "Implemented", ops4,
		"- 2026-07-21: Implemented; content-sha256: %s")
	checkAndCommit("feat(invariants): interleave direct batch", map[string]string{
		"docs/decisions/0004-interleave.md":            direct4,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "b", "c", "x"),
	})

	final3 := publicV2ADR(t, "0003", "Incremental", "Implementing", ops3,
		"- 2026-07-21: Implementing; content-sha256: %s\n- 2026-07-21: Applied; operations: add `alpha/one:c`, add `alpha/one:b`\n- 2026-07-22: Applied; operations: add `alpha/one:a`")
	checkAndCommit("feat(invariants): apply earlier-declared final batch", map[string]string{
		"docs/decisions/0003-incremental.md":           final3,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x"),
	})

	implemented3 := publicV2ADR(t, "0003", "Incremental", "Implemented", ops3,
		"- 2026-07-21: Implementing; content-sha256: %s\n- 2026-07-21: Applied; operations: add `alpha/one:c`, add `alpha/one:b`\n- 2026-07-22: Applied; operations: add `alpha/one:a`\n- 2026-07-24: Implemented; content-sha256: %s")
	checkAndCommit("docs(adr): record settled terminal review", map[string]string{
		"docs/decisions/0003-incremental.md": implemented3,
	})

	ops5 := "- add `alpha/one:y`\n- add `alpha/one:z`"
	partial5 := publicV2ADR(t, "0005", "Partial", "Implementing", ops5,
		"- 2026-07-24: Implementing; content-sha256: %s\n- 2026-07-24: Applied; operations: add `alpha/one:y`")
	checkAndCommit("feat(invariants): apply partial batch", map[string]string{
		"docs/decisions/0005-partial.md":               partial5,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x", "y"),
	})
	abandoned5 := publicV2ADR(t, "0005", "Partial", "Abandoned", ops5,
		"- 2026-07-24: Implementing; content-sha256: %s\n- 2026-07-24: Applied; operations: add `alpha/one:y`\n- 2026-07-25: Abandoned; content-sha256: %s; rationale: stop before z")
	checkAndCommit("docs(adr): abandon partial execution", map[string]string{"docs/decisions/0005-partial.md": abandoned5})

	// Reverse deletion plus its inverse mutation is still forbidden.
	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0005-partial.md":               publicV2ADR(t, "0005", "Partial", "Proposed", ops5, ""),
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x"),
	})
	reversed, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(currentStateFindings(reversed), "\n"), "history") {
		t.Fatalf("reverse pair findings = %v", currentStateFindings(reversed))
	}
	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0005-partial.md":               abandoned5,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x", "y"),
	})

	// Commit a bad pair without checking, then a repairing endpoint. Static truth
	// at the endpoint cannot rescue the first-parent range violation.
	ops6 := "- add `alpha/one:q`"
	direct6 := publicV2ADR(t, "0006", "Bad intermediate", "Implemented", ops6,
		"- 2026-07-26: Implemented; content-sha256: %s")
	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0006-bad-intermediate.md": direct6})
	gitfixture.Commit(t, repo, "feat(invariants): record bad intermediate", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "q", "x", "y")})
	head := gitfixture.Commit(t, repo, "fix(invariants): repair endpoint", nil)
	findings, _, err := auditProject(p, testContext(t), base, head)
	if err != nil {
		t.Fatal(err)
	}
	var transitionFindings int
	for _, finding := range findings {
		if finding.Rule == "current-state-transition" {
			transitionFindings++
		}
	}
	if transitionFindings < 2 {
		t.Fatalf("range transition findings = %d, all = %#v", transitionFindings, findings)
	}
}

func publicV2ADR(t *testing.T, number, title, status, operations, tail string) string {
	t.Helper()
	build := func(history string) string {
		return "---\nformat: current-state-v2\nstatus: " + status + "\ndate: 2026-07-21\n---\n# ADR-" + number + ": " + title + "\n\n" +
			"## Context\n\nContext.\n\n## Decision\n\n1. Apply state.\n\n## State changes\n\n" + operations + "\n\n" +
			"## Consequences\n\nConsequence.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-21: Proposed" + history + "\n"
	}
	proposed := strings.Replace(build(""), "status: "+status, "status: Proposed", 1)
	record, err := adr.ParseV2(number+"-fixture.md", []byte(proposed))
	if err != nil {
		t.Fatal(err)
	}
	digest := adr.ContentDigest(record.Sections)
	if tail == "" {
		return proposed
	}
	formatted := strings.ReplaceAll(tail, "%s", digest)
	return build("\n" + formatted)
}

func publicTopicClaims(slugs ...string) string {
	var b strings.Builder
	b.WriteString("Intro.\n\n## Claims\n")
	slugs = append([]string{"r"}, slugs...)
	for _, slug := range slugs {
		owner := "0003"
		switch slug {
		case "r":
			owner = "0001"
		case "v1":
			owner = "0002"
		case "x":
			owner = "0004"
		case "y":
			owner = "0005"
		case "q":
			owner = "0006"
		}
		prose := "Rule " + slug + "."
		if slug == "r" {
			prose = "Rule prose."
		}
		fmt.Fprintf(&b, "\n### `rule: %s`\n\n%s\nOrigin: ADR-%s\n", slug, prose, owner)
	}
	return b.String()
}
