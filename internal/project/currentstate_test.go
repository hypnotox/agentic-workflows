package project

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func currentStateFindings(r CurrentStateReport) []string {
	var out []string
	for _, coverage := range r.Coverage {
		if coverage.Severity == severity.Error {
			out = append(out, coverage.Message())
		}
	}
	for _, drift := range r.PlanDrift {
		out = append(out, fmt.Sprintf("%s %s: %s", drift.Kind, drift.Path, drift.Detail))
	}
	return out
}

func currentStateWarningNotes(report CurrentStateReport) []string {
	var out []string
	for _, finding := range report.Coverage {
		if finding.Severity == severity.Warn {
			out = append(out, finding.Message())
		}
	}
	return append(out, report.PlanNotes...)
}

func TestCurrentStateReportRouting(t *testing.T) {
	r := CurrentStateReport{PlanDrift: []manifest.Drift{{Path: "docs/plans/v2.md", Kind: "plan-reference", Detail: "missing ADR"}}, Coverage: []topic.CoverageFinding{{Path: "internal/a.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error, CandidateTopics: []string{"alpha/global"}}, {Path: "internal/b.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 3}}}
	findings := currentStateFindings(r)
	if len(findings) != 2 || !strings.Contains(findings[0], "internal/a.go") || findings[1] != "plan-reference docs/plans/v2.md: missing ADR" {
		t.Fatalf("findings = %#v", findings)
	}
	if notes := currentStateWarningNotes(r); len(notes) != 1 || !strings.Contains(notes[0], "internal/b.go") {
		t.Fatalf("notes = %#v", notes)
	}
}

// csNoPolicyYAML declares no currentState block. It is the base rather than a
// subtraction from csYAML on purpose: under ADR-0192, the two shapes produce
// identical coverage and fan-out findings. A derivation that silently failed
// to strip the block would leave the no-policy tests green while exercising
// the wrong shape. Deriving in this direction has no replacement pattern to
// fall out of sync.
const csNoPolicyYAML = `prefix: example
profile: full
integrationBranch: main
domains:
  - alpha
`

// csYAML is csNoPolicyYAML plus the currentState block.
const csYAML = csNoPolicyYAML + "currentState:\n"

// csRuleTopic is a one-claim current-state part citing an Implemented Origin ADR.
const csRuleTopic = "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\n"

// csRepo builds a git-backed project: a fresh repo, the given config, and the
// given working files (untracked but nonignored, so the working Tree includes
// them). It writes an Implemented ADR-0001 the topic can cite unless the caller
// supplies its own decisions file.
func csRepo(t *testing.T, cfg string, files map[string]string) *ProjectState {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	// A base commit so the working Tree can resolve HEAD; the fixture files below
	// stay untracked-nonignored and are still part of the working universe.
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, cfg)
	if _, ok := files[".awf/awf.lock"]; !ok {
		testsupport.WriteFile(t, filepath.Join(dir, ".awf/awf.lock"), `{"awfVersion":"0.39.2","schemaVersion":46,"files":{"prior":{}}}`)
	}
	if _, ok := files["docs/decisions/0001-first.md"]; !ok {
		files["docs/decisions/0001-first.md"] = testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"),
			testsupport.WithBody("## Context\nx\n## Consequences\nc\n"))
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(dir, rel), body)
	}
	p, err := Open(testContext(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCheckCurrentState runs the working-tree check end to end: a covered path
// yields nothing, an owned-but-unscoped path yields one uncovered finding, and a
// generated (lock-listed) paths remain independently excluded. A
// permanent lock validation supplies authority.
func TestCheckCurrentState(t *testing.T) {
	p := csRepo(t, csYAML, map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": csRuleTopic,
		"internal/foo/x.go":                            "package foo\n",
		"internal/bar.go":                              "package internalx\n",
		"internal/skip.go":                             "package internalx\n",
		"internal/gen.go":                              "package internalx\n",
	})
	lock := &manifest.Lock{
		AWFVersion: "0.39.2", SchemaVersion: 46,
		Files: map[string]manifest.Entry{"internal/gen.go": {}},
	}
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, lockFile(p.Root()), string(b))

	report, err := checkCurrentStateProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckCurrentState: %v", err)
	}
	findings := currentStateFindings(report)
	if len(findings) != 2 || !strings.Contains(strings.Join(findings, "\n"), "internal/bar.go") || !strings.Contains(strings.Join(findings, "\n"), "internal/skip.go") {
		t.Fatalf("findings = %#v; want bar and skip uncovered findings", findings)
	}
	for _, excluded := range []string{"internal/foo/x.go", "internal/gen.go"} {
		for _, f := range findings {
			if strings.Contains(f, excluded) {
				t.Errorf("%s should not be reported (covered or generated)", excluded)
			}
		}
	}
	notes := currentStateWarningNotes(report)
	if len(notes) != 0 {
		t.Errorf("notes = %#v; want none", notes)
	}
}

// TestCheckCurrentStateNoPolicy proves coverage and fan-out both evaluate for a
// tree that declares no currentState block: evaluation does not depend on the
// block's presence (ADR-0192). internal/foo/** carries nine topics, one
// claim-bearing so the path is covered and eight claimless, which together
// exceed the nil-receiver default budget of 8 and yield the fan-out finding;
// internal/bar.go is owned by the domain but scoped by no claim-bearing topic,
// so it yields the coverage finding.
//
// The DeepEqual below pins severity.Error and severity.Warn exactly.
// Those assertions also back severity-not-configurable's fixed-rank clause,
// so its proof marker remains attached here.
// Together they form the complete rank oracle.
// invariant: rendering/sync-and-drift:coverage-evaluation-unconditional (TestCheckCurrentStateNoPolicy)
// invariant: config/configuration:severity-not-configurable (TestCheckCurrentStateNoPolicy)
func TestCheckCurrentStateNoPolicy(t *testing.T) {
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\ndomains: [alpha]\n"
	files := map[string]string{
		".awf/domains/alpha.yaml": "paths:\n  - internal/**\n",
		"internal/bar.go":         "package internalx\n",
		"internal/foo/x.go":       "package foo\n",
	}
	for i := 1; i <= 9; i++ {
		name := fmt.Sprintf("fan%d", i)
		files[".awf/topics/metadata/alpha/"+name+".yaml"] = fmt.Sprintf("title: Fan %d\nsummary: Fan-out fixture topic %d.\npaths:\n  - internal/foo/**\n", i, i)
		part := "Intro.\n\n## Claims\n"
		if i == 1 {
			part = csRuleTopic
		}
		files[".awf/topics/parts/alpha/"+name+"/current-state.md"] = part
	}
	p := csRepo(t, cfg, files)
	report, err := checkCurrentStateProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckCurrentState: %v", err)
	}
	if report.Coverage == nil {
		t.Fatal("coverage = nil; want evaluation without a currentState policy")
	}
	want := []topic.CoverageFinding{
		{Path: "internal/bar.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error},
		{Path: "internal/foo/x.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 9},
	}
	if !reflect.DeepEqual(report.Coverage, want) {
		t.Fatalf("coverage:\n got %#v\nwant %#v", report.Coverage, want)
	}
}

// TestCheckCurrentStateOutsideRepo covers the filesystem working-tree fallback
// for a scaffolded project that is not a git repository.
func TestCheckStagedRootOutsideRepo(t *testing.T) {
	if _, err := currentstatecoord.CheckStagedRoot(testContext(t), t.TempDir()); err == nil {
		t.Fatal("CheckStagedRoot accepted a non-repository")
	}
}

func TestCheckCurrentStateOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", nil)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/awf.lock"), `{"awfVersion":"0.39.2","schemaVersion":46,"files":{"prior":{}}}`)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkCurrentStateProject(p, testContext(t)); err != nil {
		t.Fatalf("filesystem current-state fallback: %v", err)
	}
}

func TestCheckCurrentStateNoInvariantClaims(t *testing.T) {
	p := csRepo(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{})
	report, err := checkCurrentStateProject(p, testContext(t))
	if err != nil {
		t.Fatalf("current-state check with no invariant claims: %v", err)
	}
	if findings := currentStateFindings(report); len(findings) != 0 {
		t.Fatalf("current-state findings with no invariant claims = %v, want none", findings)
	}
}

// TestCheckCurrentStateCorruptLock covers the lock-read failure: a malformed
// awf.lock is not gated before this project method.
func TestCheckCurrentStateCorruptLock(t *testing.T) {
	p := csRepo(t, csYAML, map[string]string{".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	testsupport.WriteFile(t, lockFile(p.Root()), "{not json")
	if _, err := checkCurrentStateProject(p, testContext(t)); err == nil {
		t.Fatal("expected a lock parse error")
	}
}

// TestCheckCurrentStateLoadError propagates a corpus load failure: a decisions
// file that is not parseable.
func TestCheckCurrentStateLoadError(t *testing.T) {
	p := csRepo(t, csYAML, map[string]string{
		".awf/domains/alpha.yaml":      "paths:\n  - internal/**\n",
		"docs/decisions/0001-first.md": "---\nstatus: [unterminated\n---\n# X\n",
	})
	if _, err := checkCurrentStateProject(p, testContext(t)); err == nil {
		t.Fatal("expected a corpus load error from the malformed ADR")
	}
}
