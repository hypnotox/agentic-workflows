package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

const ctxCmdYAML = `prefix: example
vars:
  gateCmd: make gate
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
  - core
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`

// acceptedV1 builds a valid Accepted current-state-v1 ADR whose Status history
// records the content digest of its five canonical sections.
func acceptedV1(t *testing.T, num, title, date, stateChanges string) string {
	t.Helper()
	doc := func(status, history string) string {
		return "---\nformat: current-state-v1\nstatus: " + status + "\ndate: " + date + "\n---\n" +
			"# ADR-" + num + ": " + title + "\n\n" +
			"## Context\n\nBackground prose.\n\n" +
			"## Decision\n\n1. The decision.\n\n" +
			"## State changes\n\n" + stateChanges + "\n\n" +
			"## Consequences\n\nConsequence prose.\n\n" +
			"## Alternatives Considered\n\nNone considered.\n\n" +
			"## Status history\n\n" + history + "\n"
	}
	scaffold, err := adr.ParseV1(num+"-x.md", []byte(doc("Proposed", "- "+date+": Proposed")))
	if err != nil {
		t.Fatalf("scaffold parse: %v", err)
	}
	return doc("Accepted", "- "+date+": Proposed\n- "+date+": Accepted; content-sha256: "+adr.ContentDigest(scaffold.Sections))
}

// ctxCmdFixture builds a git-backed adopted tree: a current lock (so the gate
// passes) with a format-v1 cutoff of 2, domain alpha owning internal/foo/** plus
// a global core topic, the scoped topic alpha/one (a rule plus test-backed and
// unbacked invariants), an Accepted v1 ADR with a pending add on alpha/one, and
// a state marker under internal/foo/x.go.
func ctxCmdFixture(t *testing.T) string {
	t.Helper()
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, ctxCmdYAML)
	lock := &manifest.Lock{
		AWFVersion: awfVersion(), SchemaVersion: migrate.Current(),
		Files:             map[string]manifest.Entry{},
		BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "x", TreeDigest: "sha256:x", ADRFormatV1From: 2, LegacyADRGaps: []int{}},
	}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\nOrigin: ADR-0001\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n",
		".awf/topics/metadata/core/g.yaml":             "title: Global\nsummary: Global rules.\napplies: global\n",
		".awf/topics/parts/core/g/current-state.md":    "Intro.\n\n## Claims\n\n### `rule: everywhere`\nApplies everywhere.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"),
			testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
		"docs/decisions/0002-later.md": acceptedV1(t, "0002", "Later", "2026-07-20", "- add `alpha/one:pending-rule`"),
		"internal/foo/x.go":            "package foo\n// state: alpha/one:order\n",
		"internal/foo/y.go":            "package foo\n",
		"internal/foo/y_test.go":       "package foo\n// invariant: alpha/one:tested\n",
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root
}

// TestRunContextHuman shows the grouped Topics section (each applicable topic
// once, with roster, count-plus-drilldown, and direct detail), the per-path
// attribution, and the eligible-unowned remediation hint.
func TestRunContextHuman(t *testing.T) {
	root := ctxCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"internal/foo/x.go", "README.md"}, false, "", false, false, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"live state for this project", "Projection: concise", "## Requests", "README.md [literal]", "internal/foo/x.go [literal]",
		"## Topics", "alpha/one - One", "core/g - Global", "Both domain and topic selectors must match.",
		"(drill down: awf topic alpha/one --coverage)", "Claims (", "Direct claims:", "Details omitted for",
		"## Effective paths", "README.md [eligible-unowned]",
		"No domain owns this path; add a domain glob to a configured domain to own it (see: awf context --uncovered)",
		"internal/foo/x.go [covered]", "Domain: alpha (docs/domains/alpha.md)", "Topic: alpha/one",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q\n%s", want, got)
		}
	}
	// Both queried paths select the global topic, yet its block renders once.
	if strings.Count(got, "core/g - Global") != 1 {
		t.Errorf("global topic block not deduplicated:\n%s", got)
	}
	if strings.Contains(got, "Matched paths: [") {
		t.Errorf("census leaked into context output:\n%s", got)
	}
	if strings.Contains(got, "pending-rule") {
		t.Errorf("foundation render leaked claim projection before Phase 5:\n%s", got)
	}
}

// TestPrintContextLifecycleGoldens pins the complete human and JSON contract
// for the remaining rows selected from Accepted, first and middle Implementing
// progress. The project-layer golden proves Implemented and partially Abandoned
// rows are excluded; the current claim here stays in the ordinary Topics section.
func TestPrintContextLifecycleGoldens(t *testing.T) {
	inside := true
	res := project.ContextResult{
		Projection: project.ContextConcise,
		Requests:   []project.ContextRequest{{Query: "internal/foo/x.go", Status: project.RequestLiteral, EffectivePaths: []string{"internal/foo/x.go"}}},
		Topics:     []project.InvocationTopicContext{},
		Paths: []project.ContextPath{
			{Path: "internal/foo/x.go", Requests: []string{"internal/foo/x.go"}, Classification: project.PathCovered, Domains: []project.DomainRef{}, Topics: []project.PathTopicRef{}, Artifacts: []project.ArtifactRecord{}},
			{Path: "nested/x", Requests: []string{"nested/x"}, Classification: project.PathNestedAdopter, NestedRoot: "nested/.awf/config.yaml", Domains: []project.DomainRef{}, Topics: []project.PathTopicRef{}, Artifacts: []project.ArtifactRecord{}},
			{Path: "link", Requests: []string{"link"}, Classification: project.PathSymlink, TargetInsideRepository: &inside, Domains: []project.DomainRef{}, Topics: []project.PathTopicRef{}, Artifacts: []project.ArtifactRecord{{Role: project.ArtifactManagedOutput, Identity: "x", Sources: []project.ArtifactLink{}, Outputs: []project.ArtifactLink{}, Navigation: []project.ArtifactLink{}}}},
		},
	}
	var human, encoded bytes.Buffer
	if err := printContext(&human, res, false, "header"); err != nil {
		t.Fatal(err)
	}
	if err := printContext(&encoded, res, true, "ignored"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Projection: concise", "internal/foo/x.go [covered]", "## Requests", "## Effective paths", "Nested root: nested/.awf/config.yaml", "Symlink target inside repository: true", "Artifact: managed-output x"} {
		if !strings.Contains(human.String(), want) {
			t.Errorf("missing %q in %s", want, human.String())
		}
	}
	var round project.ContextResult
	if err := json.Unmarshal(encoded.Bytes(), &round); err != nil {
		t.Fatal(err)
	}
	if len(round.Paths) != 3 || round.Paths[0].Classification != project.PathCovered {
		t.Fatalf("round trip = %#v", round)
	}
}

// TestPrintContextTitlelessTopic covers the human render of a topic with no
// title, which prints the bare id without a dangling separator.
type contextFailWriter struct{}

func (contextFailWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestPrintContextFullHumanAndWriteErrors(t *testing.T) {
	claim := project.ClaimDetail{ID: "alpha/one:stable", Type: "invariant", Prose: "Stable.", Backing: "unbacked", Verify: "inspect", Sites: []topic.MarkerSite{{Path: "x.go", Line: 3, Kind: topic.TouchesMarker, ClaimID: "alpha/one:stable"}}, References: project.ClaimReferences{Incoming: []string{"beta/two:in"}, Outgoing: []string{"beta/two:out"}}}
	fullTopic := project.InvocationTopicContext{
		ID: "alpha/one", Title: "One",
		Applicability: project.TopicApplicabilityBrief{DomainPaths: []string{"**"}, TopicPaths: []string{"**"}, MatchedPathCount: 1},
		ClaimIDs:      []string{"alpha/one:other", "alpha/one:stable"}, DirectClaims: []project.ClaimDetail{},
		TopicCommand: "awf topic alpha/one", CoverageCommand: "awf topic alpha/one --coverage",
		Full: &project.FullTopicContext{Claims: []project.ClaimDetail{claim}, Pending: []project.PendingChange{{ADR: "0003", Op: "add", Claim: "alpha/one:new"}}},
	}
	conciseGlobal := project.InvocationTopicContext{
		ID: "core/g", Title: "Global",
		Applicability: project.TopicApplicabilityBrief{DomainPaths: []string{"**"}, DeclaredGlobal: true},
		ClaimIDs:      []string{"core/g:everywhere", "core/g:more"}, DirectClaims: []project.ClaimDetail{claim}, OmittedDetailCount: 1,
		TopicCommand: "awf topic core/g", CoverageCommand: "awf topic core/g --coverage",
	}
	artifact := project.ArtifactRecord{Role: project.ArtifactManagedOutput, Identity: "managed", Sources: []project.ArtifactLink{{Path: "source", Label: "source"}}, Outputs: []project.ArtifactLink{{Path: "output", Label: "output"}}, Navigation: []project.ArtifactLink{{Path: "nav", Label: "nav"}}}
	adrContext := &project.ADRArtifactContext{Number: "0002", Title: "Example", Status: "Implemented", Mutability: "frozen", AuthorityRole: "decision history, never current authority", Operations: []project.ADROperationContext{{Operation: "add", Claim: "alpha/one:stable", Topic: "alpha/one", Progress: "applied", StateSequence: 4, ClaimState: "active-current", Detail: &project.ADROperationDetail{Current: &claim, MarkerSites: []topic.MarkerSite{}}}}}
	res := project.ContextResult{Projection: project.ContextFull, Requests: []project.ContextRequest{}, Topics: []project.InvocationTopicContext{fullTopic, conciseGlobal}, Paths: []project.ContextPath{{Path: "x.go", Requests: []string{"x.go"}, Classification: project.PathCovered, Domains: []project.DomainRef{}, Topics: []project.PathTopicRef{{ID: "alpha/one", DirectClaimIDs: []string{"alpha/one:stable"}}}, Artifacts: []project.ArtifactRecord{artifact}, ADR: adrContext}}}
	var out bytes.Buffer
	if err := printContext(&out, res, false, "header"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Topics", "Full authority", "Pending: ADR-0003", "Matched paths: 1 (drill down: awf topic alpha/one --coverage)",
		"Claims (2): alpha/one:other, alpha/one:stable", "Global topic within owning domain selectors:",
		"Direct claim", "Details omitted for 1 claim(s); drill down: awf topic core/g",
		"Topic: alpha/one", "Direct claims: alpha/one:stable",
		"Source: source", "Output: output", "Navigate: nav", "ADR navigation", "state-sequence 4", "References: incoming",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("full human missing %q:\n%s", want, out.String())
		}
	}
	// A topic with a Full block renders each claim's detail exactly once and no
	// omission line; the omission line belongs to the concise topic only. The
	// third occurrence is the per-path ADR operation's Current detail, which
	// stays outside the topic group by design.
	if strings.Count(out.String(), "alpha/one:stable [invariant] Stable.") != 3 {
		t.Errorf("claim detail count (full topic once + concise direct once + ADR operation once) wrong:\n%s", out.String())
	}
	if strings.Contains(out.String(), "drill down: awf topic alpha/one\n") {
		t.Errorf("omission line rendered alongside full authority:\n%s", out.String())
	}
	if err := printContext(contextFailWriter{}, res, false, "header"); err == nil || !strings.Contains(err.Error(), "write context") {
		t.Fatalf("human write error = %v", err)
	}
	if err := printContext(contextFailWriter{}, res, true, "header"); err == nil || !strings.Contains(err.Error(), "write context JSON") {
		t.Fatalf("JSON write error = %v", err)
	}
}

func TestPrintContextGlobLiteralHint(t *testing.T) {
	res := project.ContextResult{Projection: project.ContextConcise, Requests: []project.ContextRequest{}, Topics: []project.InvocationTopicContext{}, Paths: []project.ContextPath{{Path: "internal/foo/*.go", Requests: []string{"internal/foo/*.go"}, Classification: project.PathNotFound, GlobLiteral: true, Domains: []project.DomainRef{}, Topics: []project.PathTopicRef{}, Artifacts: []project.ArtifactRecord{}}}}
	var out bytes.Buffer
	if err := printContext(&out, res, false, "header"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "globs are not expanded; pass a directory or an exact file") {
		t.Fatalf("glob-literal hint missing: %s", out.String())
	}
}

func TestPrintContextTitlelessTopic(t *testing.T) {
	res := project.ContextResult{Projection: project.ContextConcise, Requests: []project.ContextRequest{}, Topics: []project.InvocationTopicContext{{ID: "alpha/untitled", Applicability: project.TopicApplicabilityBrief{DomainPaths: []string{}, TopicPaths: []string{}}, ClaimIDs: []string{}, DirectClaims: []project.ClaimDetail{}, TopicCommand: "awf topic alpha/untitled", CoverageCommand: "awf topic alpha/untitled --coverage"}}, Paths: []project.ContextPath{{Path: "x", Requests: []string{}, Classification: project.PathCovered, Domains: []project.DomainRef{}, Topics: []project.PathTopicRef{{ID: "alpha/untitled", DirectClaimIDs: []string{}}}, Artifacts: []project.ArtifactRecord{}}}}
	var out bytes.Buffer
	if err := printContext(&out, res, false, "header"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "alpha/untitled -") || !strings.Contains(out.String(), "Claims (0):") {
		t.Fatalf("titleless topic block missing: %s", out.String())
	}
	if strings.Contains(out.String(), "Details omitted") {
		t.Fatalf("zero-claim topic rendered an omission line: %s", out.String())
	}
}

// TestRunContextJSONParity proves the JSON render carries the same assembled set
// as the human render - one assembler feeds both.
// invariant: tooling/context-and-topic:context-output-parity
func TestRunContextFullProjectionAndConciseJSONBoundary(t *testing.T) {
	root := ctxCmdFixture(t)
	var concise, full bytes.Buffer
	if err := runContextProjection(root, []string{"internal/foo/y_test.go"}, false, "", true, false, false, &concise); err != nil {
		t.Fatal(err)
	}
	if err := runContextProjection(root, []string{"internal/foo/y_test.go"}, false, "", true, false, true, &full); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(concise.String(), `"full"`) {
		t.Fatalf("concise JSON leaked full key: %s", concise.String())
	}
	if !strings.Contains(full.String(), `"projection": "full"`) || !strings.Contains(full.String(), `"full"`) || !strings.Contains(full.String(), "alpha/one:stable") {
		t.Fatalf("full JSON is incomplete: %s", full.String())
	}
}

func TestRunContextFullUncoveredConflictPrecedesProjectLoading(t *testing.T) {
	err := runContextProjection(t.TempDir(), nil, false, "", false, true, true, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--full cannot be combined with --uncovered") {
		t.Fatalf("conflict = %v", err)
	}
}

func TestRunContextJSONParity(t *testing.T) {
	root := ctxCmdFixture(t)
	var jsonOut bytes.Buffer
	if err := runContext(root, []string{"internal/foo/y.go"}, false, "", true, false, &jsonOut); err != nil {
		t.Fatal(err)
	}
	var res project.ContextResult
	if err := json.Unmarshal(jsonOut.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(res.Requests) != 1 || len(res.Paths) != 1 || res.Paths[0].Classification != project.PathCovered {
		t.Fatalf("json attribution: %+v", res)
	}
	if len(res.Paths[0].Domains) != 1 || res.Paths[0].Domains[0].Name != "alpha" {
		t.Errorf("json domains: %+v", res.Paths[0].Domains)
	}
	if len(res.Paths[0].Topics) != 2 || res.Paths[0].Topics[0].ID != "alpha/one" || res.Paths[0].Topics[1].ID != "core/g" {
		t.Fatalf("json topics: %+v", res.Paths[0].Topics)
	}
	// Same set as the human render.
	var humanOut bytes.Buffer
	if err := runContext(root, []string{"internal/foo/y.go"}, false, "", false, false, &humanOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha/one", "Domain paths", "Both domain and topic selectors must match", "core/g", "[covered]"} {
		if !strings.Contains(humanOut.String(), want) {
			t.Errorf("human render diverges from JSON: missing %q", want)
		}
	}
}

// TestRunContextStaged reads the index universe under --staged.
func TestRunContextStaged(t *testing.T) {
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	// The disk config satisfies the adoption check and gate; the staged index below
	// carries the topic set the --staged query reads.
	testsupport.WriteAwfConfig(t, root, ctxCmdYAML)
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Stage(t, repo, root, map[string]string{
		".awf/awf.lock":                                string(func() []byte { b, _ := lock.Marshal(); return b }()),
		".awf/config.yaml":                             ctxCmdYAML,
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"),
			testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
		"internal/foo/x.go": "package foo\n",
	})
	// Dirty, corrupt working project files cannot contaminate the valid index.
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "not: [valid")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "awf.lock"), "{not json")
	var out bytes.Buffer
	if err := runContext(root, []string{"internal/foo/x.go"}, true, "", false, false, &out); err != nil {
		t.Fatalf("staged context: %v", err)
	}
	if !strings.Contains(out.String(), "alpha/one") {
		t.Errorf("staged context missing the staged topic:\n%s", out.String())
	}
	out.Reset()
	if err := runContext(root, nil, true, "", false, false, &out); err != nil {
		t.Fatalf("staged selected context: %v", err)
	}
	if !strings.Contains(out.String(), "[git-selected]") {
		t.Fatalf("staged selected status missing: %s", out.String())
	}
	out.Reset()
	if err := runContextProjection(root, []string{"internal/foo/x.go"}, true, "", false, false, true, &out); err != nil || !strings.Contains(out.String(), "Projection: full") {
		t.Fatalf("staged explicit full: %v\n%s", err, out.String())
	}
	out.Reset()
	if err := runContextProjection(root, nil, true, "", false, false, true, &out); err != nil || !strings.Contains(out.String(), "[git-selected]") {
		t.Fatalf("staged selected full: %v\n%s", err, out.String())
	}
}

func TestRunContextRangeUsesGitSelectedRequests(t *testing.T) {
	root := ctxCmdFixture(t)
	repo, err := gogit.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("adopt", &gogit.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runContext(root, nil, false, "HEAD~1..HEAD", false, false, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[git-selected]") {
		t.Fatalf("range selected status missing: %s", out.String())
	}
	out.Reset()
	if err := runContextProjection(root, nil, false, "HEAD~1..HEAD", false, false, true, &out); err != nil || !strings.Contains(out.String(), "Projection: full") {
		t.Fatalf("full range selection: %v\n%s", err, out.String())
	}
}

func TestRunContextStagedErrors(t *testing.T) {
	// The staged gate fails before project loading when no index lock exists.
	if err := runContext(t.TempDir(), []string{"x"}, true, "", false, false, io.Discard); err == nil {
		t.Fatal("staged context accepted a non-repository")
	}
	// A valid staged lock passes the gate, then malformed staged config fails the
	// staged-root loader without consulting valid working bytes.
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	b, _ := lock.Marshal()
	gitfixture.Stage(t, repo, root, map[string]string{".awf/awf.lock": string(b), ".awf/config.yaml": "not: [valid"})
	if err := runContext(root, []string{"x"}, true, "", false, false, io.Discard); err == nil {
		t.Fatal("staged context accepted malformed index config")
	}
}

// Outside an adopted tree the command prints the static pre-adoption notice and
// succeeds - never refuses. The JSON variant emits the paths-only result.
// invariant: tooling/context-and-topic:context-static-fallback
func TestRunContextStaticFallback(t *testing.T) {
	var human bytes.Buffer
	if err := runContext(t.TempDir(), []string{"cmd/x.go"}, false, "", false, false, &human); err != nil {
		t.Fatalf("static human errored: %v", err)
	}
	if !strings.Contains(human.String(), "not inside an awf project") {
		t.Errorf("static human: %s", human.String())
	}
	for _, full := range []bool{false, true} {
		var j bytes.Buffer
		if err := runContextProjection(t.TempDir(), []string{"cmd/x.go"}, false, "", true, false, full, &j); err != nil {
			t.Fatalf("static json errored: %v", err)
		}
		var res project.ContextResult
		if err := json.Unmarshal(j.Bytes(), &res); err != nil || res.Projection == "" || len(res.Paths) != 0 {
			t.Errorf("static json: %s (err %v)", j.String(), err)
		}
	}
}

// A stat fault that is not absence (a file where .awf should be) surfaces as an
// error, never silently treated as pre-adoption.
func TestRunContextStatFault(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".awf"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, false, io.Discard); err == nil {
		t.Error("a non-absence stat fault must surface")
	}
}

// Inside an adopted tree the command is gated: a binary behind the project
// refuses like every gated command.
func TestRunContextGated(t *testing.T) {
	root := gateFixture(t, "99.0.0", migrate.Current())
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, false, io.Discard); err == nil {
		t.Error("expected the version gate to refuse a behind binary")
	}
}

// An invalid config fails project open like every gated command.
func TestRunContextOpenError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: \"\"\n")
	l := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, false, io.Discard); err == nil {
		t.Error("expected the open-time validation error")
	}
}

// A fault assembling the context (here a malformed ADR in the working tree)
// surfaces as the command's error rather than a panic or silence.
func TestRunContextAssembleFault(t *testing.T) {
	root := ctxCmdFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0009-bad.md"), "---\nstatus: \"unterminated\n---\n# ADR-X: T\n")
	if err := runContext(root, []string{"internal/foo/x.go"}, false, "", false, false, io.Discard); err == nil {
		t.Error("expected the assemble fault to surface")
	}
}

// No paths and no git selector is a usage error (exit 2); --help prints help.
func TestRunContextDispatch(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"awf", "context"}, &out, &errBuf); code != 2 {
		t.Errorf("no-args exit: got %d want 2 (%s)", code, errBuf.String())
	}
	out.Reset()
	if code := run([]string{"awf", "context", "--help"}, &out, &errBuf); code != 0 {
		t.Errorf("--help exit: got %d want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: awf context") {
		t.Errorf("--help body: %s", out.String())
	}
	// With a path, run dispatches to runContext; the test's cwd (cmd/awf) is not
	// an adopted tree, so it prints the static fallback and succeeds.
	out.Reset()
	if code := run([]string{"awf", "context", "somepath"}, &out, &errBuf); code != 0 {
		t.Errorf("dispatch exit: got %d want 0 (%s)", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "not inside an awf project") {
		t.Errorf("dispatch body: %s", out.String())
	}
	out.Reset()
	errBuf.Reset()
	if code := run([]string{"awf", "context", "--full", "somepath"}, &out, &errBuf); code != 0 || !strings.Contains(out.String(), "Projection: full") {
		t.Errorf("full dispatch: code %d, stdout %s, stderr %s", code, out.String(), errBuf.String())
	}
	out.Reset()
	errBuf.Reset()
	if code := run([]string{"awf", "context", "--full", "--uncovered"}, &out, &errBuf); code != 2 || !strings.Contains(errBuf.String(), "--full cannot be combined") {
		t.Errorf("full uncovered conflict: code %d, stderr %s", code, errBuf.String())
	}
}

// awf context writes nothing: file mtimes and the lock bytes are byte-identical
// before and after runs across the command's branches.
// invariant: tooling/context-and-topic:context-read-only
func TestRunContextReadOnly(t *testing.T) {
	root := ctxCmdFixture(t)
	before := snapshotTree(t, root)
	lockBefore := readFile(t, filepath.Join(root, ".awf", "awf.lock"))
	for _, tc := range []struct {
		paths  []string
		asJSON bool
	}{
		{[]string{"internal/foo/x.go"}, false},
		{[]string{"internal/foo/x.go"}, true},
		{[]string{"README.md"}, false},
	} {
		if err := runContext(root, tc.paths, false, "", tc.asJSON, false, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	if after := snapshotTree(t, root); after != before {
		t.Errorf("awf context mutated the tree:\nbefore %s\nafter  %s", before, after)
	}
	if lockAfter := readFile(t, filepath.Join(root, ".awf", "awf.lock")); lockAfter != lockBefore {
		t.Error("awf context mutated the lock")
	}
}

// The --staged/--range selectors resolve paths from git when none are given.
func TestRunContextGitSelectors(t *testing.T) {
	// A git fault (here a non-repo cwd) surfaces as exit 1.
	t.Run("git error", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var out, errBuf bytes.Buffer
		if code := run([]string{"awf", "context", "--range", "a..b"}, &out, &errBuf); code != 1 {
			t.Errorf("git error exit: got %d want 1 (%s)", code, errBuf.String())
		}
	})
	// A selector resolving to no paths is a usage error (exit 2).
	t.Run("empty selector", func(t *testing.T) {
		repo, dir := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
		t.Chdir(dir)
		var out, errBuf bytes.Buffer
		if code := run([]string{"awf", "context", "--staged"}, &out, &errBuf); code != 2 {
			t.Errorf("empty-selector exit: got %d want 2 (%s)", code, errBuf.String())
		}
	})
	// A selector resolving to paths dispatches to runContext; the fixture cwd is
	// a git repo but not an awf tree, so the static fallback prints (exit 0).
	t.Run("range dispatches", func(t *testing.T) {
		repo, dir := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, dir, "one", map[string]string{"a.txt": "a"})
		gitfixture.Commit(t, repo, dir, "two", map[string]string{"b.txt": "b"})
		t.Chdir(dir)
		var out, errBuf bytes.Buffer
		if code := run([]string{"awf", "context", "--range", "HEAD~1..HEAD"}, &out, &errBuf); code != 0 {
			t.Errorf("range-dispatch exit: got %d want 0 (%s)", code, errBuf.String())
		}
		if !strings.Contains(out.String(), "not inside an awf project") {
			t.Errorf("expected static fallback: %s", out.String())
		}
	})
}

const uncoveredCmdYAML = `prefix: example
vars:
  gateCmd: make gate
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
contextIgnore:
  - .awf/**
  - docs/**
currentState:
  topicCoverage: error
  topicFanout: off
`

// uncoveredCmdFixture builds a git-backed adopted tree where alpha owns
// internal/** while the topic covers only internal/foo/**, so internal/bar.go is
// owned-but-uncovered and a top-level stray is unowned.
func uncoveredCmdFixture(t *testing.T) string {
	t.Helper()
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, uncoveredCmdYAML)
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"),
			testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
		"internal/foo/x.go": "package foo\n",
		"internal/bar.go":   "package internalx\n",
		"stray.txt":         "stray\n",
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root
}

// --uncovered lists domain-owned paths with no scoped topic and, separately, the
// eligible unowned paths; a scan root renders the `scan roots:` line.
func TestRunContextUncoveredHuman(t *testing.T) {
	root := uncoveredCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"internal"}, false, "", false, true, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"## Uncovered", "internal/bar.go (alpha)", "scan roots:"} {
		if !strings.Contains(got, want) {
			t.Errorf("uncovered human missing %q\n%s", want, got)
		}
	}
	// Constructed rendering: directory entries (trailing slash or the root ".")
	// carry counts with singular/plural forms and omit the excluded clause at
	// zero; plain file entries render bare.
	res := project.UncoveredResult{Unowned: []project.UnownedEntry{
		{Path: ".pi/", UnownedCount: 1, ExcludedCount: 22},
		{Path: "README.md", UnownedCount: 1, ExcludedCount: 0},
		{Path: "gen/", UnownedCount: 2, ExcludedCount: 1},
		{Path: "sub/", UnownedCount: 2, ExcludedCount: 0},
		{Path: ".", UnownedCount: 3, ExcludedCount: 2},
	}}
	var constructed bytes.Buffer
	if err := printUncovered(&constructed, res, false, "header"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"  .pi/ (1 unowned file; 22 files excluded from coverage beneath)\n",
		"  README.md\n",
		"  gen/ (2 unowned files; 1 file excluded from coverage beneath)\n",
		"  sub/ (2 unowned files)\n",
		"  . (3 unowned files; 2 files excluded from coverage beneath)\n",
	} {
		if !strings.Contains(constructed.String(), want) {
			t.Errorf("constructed uncovered render missing %q\n%s", want, constructed.String())
		}
	}
}

// The --uncovered JSON render carries the same set as the human render.
// invariant: tooling/context-and-topic:uncovered-output-parity
func TestRunContextUncoveredJSONParity(t *testing.T) {
	root := uncoveredCmdFixture(t)
	var j bytes.Buffer
	if err := runContext(root, nil, false, "", true, true, &j); err != nil {
		t.Fatal(err)
	}
	var res project.UncoveredResult
	if err := json.Unmarshal(j.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(res.Uncovered) != 1 || res.Uncovered[0].Path != "internal/bar.go" {
		t.Errorf("json uncovered: %+v", res.Uncovered)
	}
	want := []project.UnownedEntry{{Path: "README.md", UnownedCount: 1}, {Path: "stray.txt", UnownedCount: 1}}
	if !reflect.DeepEqual(res.Unowned, want) {
		t.Errorf("json unowned: %#v want %#v", res.Unowned, want)
	}
	var human bytes.Buffer
	if err := runContext(root, nil, false, "", false, true, &human); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"internal/bar.go", "stray.txt"} {
		if !strings.Contains(human.String(), want) {
			t.Errorf("human render diverges from JSON: missing %q", want)
		}
	}
}

func TestRunContextUncoveredStagedHumanAndJSON(t *testing.T) {
	root := uncoveredCmdFixture(t)
	repo, err := gogit.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	b, _ := lock.Marshal()
	gitfixture.Stage(t, repo, root, map[string]string{
		".awf/awf.lock": string(b), ".awf/config.yaml": uncoveredCmdYAML,
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md":                 testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25")),
		"internal/foo/x.go":                            "package foo\n", "internal/bar.go": "package internalx\n", "stray.txt": "stray\n",
	})
	_ = os.Remove(filepath.Join(root, ".awf", "config.yaml"))
	_ = os.Remove(filepath.Join(root, ".awf", "awf.lock"))
	var human, j bytes.Buffer
	if err := runContext(root, nil, true, "", false, true, &human); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, nil, true, "", true, true, &j); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(human.String(), "internal/bar.go") {
		t.Fatalf("staged human = %s", human.String())
	}
	var res project.UncoveredResult
	if err := json.Unmarshal(j.Bytes(), &res); err != nil || len(res.Uncovered) != 1 {
		t.Fatalf("staged json = %s (%v)", j.String(), err)
	}
}

func TestRunContextUncoveredStagedErrors(t *testing.T) {
	if err := runContext(t.TempDir(), nil, true, "", false, true, io.Discard); err == nil {
		t.Fatal("staged uncovered accepted a non-repository")
	}
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	lock := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	b, _ := lock.Marshal()
	gitfixture.Stage(t, repo, root, map[string]string{".awf/awf.lock": string(b), ".awf/config.yaml": "not: [valid"})
	if err := runContext(root, nil, true, "", false, true, io.Discard); err == nil {
		t.Fatal("staged uncovered accepted malformed index config")
	}
}

func TestRunContextUncoveredRejectsRange(t *testing.T) {
	if err := runContext(t.TempDir(), nil, false, "a..b", false, true, io.Discard); err == nil || !strings.Contains(err.Error(), "--range") {
		t.Errorf("--uncovered with --range must be a usage error, got: %v", err)
	}
}

// Outside an adopted tree --uncovered prints the static notice and succeeds; the
// empty result exercises printUncovered's no-entries branch.
func TestRunContextUncoveredStaticFallback(t *testing.T) {
	var out bytes.Buffer
	if err := runContext(t.TempDir(), nil, false, "", false, true, &out); err != nil {
		t.Fatalf("static uncovered errored: %v", err)
	}
	if !strings.Contains(out.String(), "not inside an awf project") {
		t.Errorf("static uncovered: %s", out.String())
	}
}

// A non-absence stat fault surfaces in --uncovered mode too.
func TestRunContextUncoveredStatFault(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".awf"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, nil, false, "", false, true, io.Discard); err == nil {
		t.Error("a non-absence stat fault must surface")
	}
}

// --uncovered is gated: a binary behind the project refuses.
func TestRunContextUncoveredGated(t *testing.T) {
	root := gateFixture(t, "99.0.0", migrate.Current())
	if err := runContext(root, nil, false, "", false, true, io.Discard); err == nil {
		t.Error("expected the version gate to refuse a behind binary")
	}
}

// An invalid config fails project open in --uncovered mode.
func TestRunContextUncoveredOpenError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: \"\"\n")
	l := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, nil, false, "", false, true, io.Discard); err == nil {
		t.Error("expected the open-time validation error")
	}
}

// An adopted tree that is not a git repo makes the working-Tree read fault,
// surfacing as the command's error.
func TestRunContextUncoveredWorkingTreeFault(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, uncoveredCmdYAML)
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "alpha.yaml"), "paths:\n  - internal/**\n")
	l := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, nil, false, "", false, true, io.Discard); err == nil {
		t.Error("expected a working-tree open error in a non-git adopted tree")
	}
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b.WriteString(rel + "@" + info.Mode().String() + ":")
		if !d.IsDir() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			b.Write(content)
		}
		b.WriteByte(';')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
