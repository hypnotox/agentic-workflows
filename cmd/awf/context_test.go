package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/clispec"
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

func TestRunContextHumanAndFacets(t *testing.T) {
	root := ctxCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"internal/foo"}, false, "", false, false, []string{"evidence", "all-rules", "evidence"}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Selection: explicit", "[1] internal/foo", "Directory: 3 included", "## Authority", "Directly related:", "Additional topic rules:", "Backing: test", "Evidence invariant:"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// invariant: tooling/context-and-topic:context-terminal-output-cap
func TestRunContextModesShareDeliveryIncludingOversize(t *testing.T) {
	oldDeliver := deliverContext
	var sizes []int
	deliverContext = func(rendered []byte, root string, stdout io.Writer) error {
		sizes = append(sizes, len(rendered))
		return nil
	}
	t.Cleanup(func() { deliverContext = oldDeliver })
	root := ctxCmdFixture(t)
	if err := runContext(root, []string{"internal/foo/x.go"}, false, "", false, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"internal"}, false, "", true, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runContext(t.TempDir(), []string{"x"}, false, "", false, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	var part strings.Builder
	part.WriteString("Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\nOrigin: ADR-0001\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n")
	for i := range 180 {
		fmt.Fprintf(&part, "\n### `rule: rule-%03d`\nRule %03d carries enough projection prose to exercise capped delivery routing.\nOrigin: ADR-0001\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "topics", "parts", "alpha", "one", "current-state.md"), []byte(part.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"internal/foo/x.go"}, false, "", false, false, []string{"all-rules"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 4 || sizes[0] == 0 || sizes[1] == 0 || sizes[2] == 0 || sizes[3] <= 8192 {
		t.Fatalf("delivery sizes = %v", sizes)
	}
}

// invariant: tooling/context-and-topic:context-static-fallback
func TestRunContextStaticAndUsage(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := runContext(root, []string{"x"}, false, "", false, false, nil, &out); err != nil {
		t.Fatal(err)
	}
	const want = "context (static: not inside an awf project; live classification and authority require an adopted project)\nSelection: explicit\n\n## Requests\n  none\n\n## Authority\n  none\n"
	if out.String() != want {
		t.Fatalf("static:\n%s", out.String())
	}
	cases := []struct {
		paths           []string
		staged          bool
		rng             string
		uncovered, full bool
		shows           []string
		part            string
	}{
		{nil, false, "", false, false, nil, "usage:"}, {[]string{"x"}, false, "", false, false, []string{"bad"}, "unknown context facet"}, {nil, false, "a..b", true, false, nil, "--range"}, {nil, false, "", true, true, nil, "cannot be combined"},
	}
	for _, tc := range cases {
		if err := runContext(root, tc.paths, tc.staged, tc.rng, tc.uncovered, tc.full, tc.shows, io.Discard); err == nil || !strings.Contains(err.Error(), tc.part) {
			t.Errorf("err=%v want %q", err, tc.part)
		}
	}
}

func TestRenderContextGrammar(t *testing.T) {
	inside := false
	impact := project.ContextPathImpact{Classification: project.PathSymlink, TargetInsideRepository: &inside, Provenance: []project.ContextProvenance{{Role: "template", Identity: "skills/example/SKILL.md.tmpl", Sources: []project.ArtifactLink{{Path: "templates/x", Label: "template source"}}, Outputs: []project.ArtifactLink{}, Navigation: []project.ArtifactLink{}}}, Domains: []project.DomainRef{{Name: "tooling"}}, Topics: []project.ContextPathTopic{{ID: "tooling/example"}}, DirectRuleIDs: []string{}, InvariantIDs: []string{}, ProofIDs: []string{}, Warnings: []project.ContextWarning{project.WarningGlobLiteral}}
	res := project.ContextResult{
		Selection: project.SelectionRange, Range: "a..b",
		Requests: []project.ContextRequestReport{{Index: 1, Argument: "x", Exact: &project.ContextExactEntry{Path: "x", Context: impact}}},
		Topics:   []project.TopicImpact{{ID: "tooling/example", Title: "Example", Summary: "Summary.", Direct: []project.ContextClaimImpact{{ID: "tooling/example:r", Type: "rule", Summary: "Rule.", Incoming: []string{"a"}, Outgoing: []string{"b"}}}}},
	}
	var out bytes.Buffer
	renderContext(&out, res, "header", []project.ContextFacet{project.FacetArtifacts})
	for _, want := range []string{"Selection: range a..b", "File: x", "Symlink target inside repository: false", "Source: templates/x", "Warning: globs", "Incoming: a", "Outgoing: b"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q:\n%s", want, out.String())
		}
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

func TestRenderAllContextBranches(t *testing.T) {
	var uncovered bytes.Buffer
	renderUncovered(&uncovered, project.UncoveredResult{ScanRoots: []string{"internal"}, Uncovered: []project.UncoveredTopic{{Path: "internal/x", Domain: "d"}}, Unowned: []project.UnownedEntry{{Path: "file", UnownedCount: 1}, {Path: "dir/", UnownedCount: 1, ExcludedCount: 2}, {Path: ".", UnownedCount: 2}}}, "header")
	for _, want := range []string{"scan roots", "## Uncovered", "## Unowned", "1 unowned file", "2 files excluded"} {
		if !strings.Contains(uncovered.String(), want) {
			t.Errorf("uncovered missing %q: %s", want, uncovered.String())
		}
	}
	current := project.ContextClaimImpact{ID: "d/t:i", Type: "invariant", Summary: "Invariant.", Backing: "unbacked", Verify: "inspect", Evidence: []project.ContextEvidence{{Kind: "state", Count: 4}, {Kind: "invariant", Count: 1, Sites: []topic.MarkerSite{{Path: "x_test.go", Line: 3}}}}}
	impact := project.ContextPathImpact{Classification: project.PathNestedAdopter, NestedRoot: "child/.awf/config.yaml", Provenance: []project.ContextProvenance{{Role: "template", Identity: "x", Sources: []project.ArtifactLink{}, Outputs: []project.ArtifactLink{{Path: "out", Label: "managed output"}}, Navigation: []project.ArtifactLink{{Path: "nav", Label: "managed output"}}}}, Domains: []project.DomainRef{}, Topics: []project.ContextPathTopic{}, DirectRuleIDs: []string{}, InvariantIDs: []string{}, ProofIDs: []string{}, Warnings: []project.ContextWarning{project.WarningEligibleUnowned}, ADR: &project.ADRArtifactContext{Number: "2", Title: "Decision", Status: "Implementing", Mutability: "frozen", AuthorityRole: "pending intent or decision history; not current authority", Operations: []project.ADROperationContext{{Operation: "update", Claim: "d/t:i", Progress: "applied", ClaimState: "active-current", StateSequence: 7, Detail: &project.ADROperationDetail{Current: &current, Evidence: current.Evidence}}, {Operation: "remove", Claim: "d/t:old", Progress: "applied", ClaimState: "historically-removed", Detail: &project.ADROperationDetail{History: &topic.ClaimHistory{RemovedBy: &topic.ADRHistory{Number: "0002", StateSequence: 7}}}}}}}
	res := project.ContextResult{Selection: project.SelectionStaged, Requests: []project.ContextRequestReport{{Index: 1, Argument: "empty", Directory: &project.ContextDirectory{Included: 0, Excluded: []project.ContextClassificationCount{{Classification: project.PathGeneratedOutput, Count: 2}}, Groups: []project.ContextGroup{{Count: 2, Members: []string{"a", "b"}, Context: impact}}}}}, Topics: []project.TopicImpact{{ID: "d/t", Title: "T", Summary: "S", Selectors: &project.ContextSelectorImpact{DomainPaths: []string{}, TopicPaths: []string{}, DeclaredGlobal: false}, Invariants: []project.ContextClaimImpact{current}, Pending: project.ContextPendingImpact{OperationCount: 4, ADRs: []string{"0001", "0002", "0003"}, AdditionalADRCount: 1}}}}
	var out bytes.Buffer
	renderContext(&out, res, "header", []project.ContextFacet{project.FacetArtifacts})
	for _, want := range []string{"Selection: staged", "Excluded: generated-output=2", "Members: a, b", "Nested root:", "Output: out", "Navigate: nav", "ADR: ADR-2", "Current claim:", "Removal history:", "Verify: inspect", "Evidence state: 4 sites", "Pending: 4 operations", "+1 ADRs", "Selectors: domain=[]; topic=[]"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q:\n%s", want, out.String())
		}
	}
	res.Topics[0].Pending.Operations = []project.PendingChange{{ADR: "0002", Op: "add", Claim: "d/t:r", Progress: "remaining"}}
	res.Topics[0].Selectors.DeclaredGlobal = true
	out.Reset()
	renderContext(&out, res, "header", nil)
	if !strings.Contains(out.String(), "Pending operation:") || !strings.Contains(out.String(), "topic=global") {
		t.Fatal(out.String())
	}
}

func TestContextHumanOnlyFlagGrammar(t *testing.T) {
	spec, _ := clispec.Lookup("context")
	for _, args := range [][]string{{"--json"}, {"--uncovered", "--json"}} {
		if _, err := parseArgs(spec, args); err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestContextDispatchHandler(t *testing.T) {
	var out bytes.Buffer
	if err := handlers["context"](&cmdCtx{root: t.TempDir(), inv: invocation{bools: map[string]bool{}, values: map[string]string{}, multi: map[string][]string{}, positionals: []string{"x"}}, stdout: &out}); err != nil {
		t.Fatal(err)
	}
}

func TestRunContextSelectionAndProjectErrors(t *testing.T) {
	root := ctxCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"internal/foo/x.go"}, false, "", false, true, nil, &out); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"internal"}, false, "", true, false, nil, &out); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, nil, false, "bad-range", false, false, nil, io.Discard); err == nil {
		t.Fatal("bad range accepted")
	}
	repo, rangeRoot := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, rangeRoot, "one", map[string]string{"a": "1"})
	gitfixture.Commit(t, repo, rangeRoot, "two", map[string]string{"a": "2"})
	if err := runContext(rangeRoot, nil, false, "HEAD~1..HEAD", false, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	_, clean := gitfixture.InitRepo(t)
	if err := runContext(clean, nil, true, "", false, false, nil, io.Discard); err == nil || !strings.Contains(err.Error(), "no changed paths") {
		t.Fatalf("empty staged err=%v", err)
	}
	if err := runContext(root, []string{"x"}, true, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("invalid staged transition accepted")
	}
	stagedRoot := ctxCmdFixture(t)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = stagedRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, output)
	}
	_ = runContext(stagedRoot, []string{"internal/foo/x.go"}, true, "", false, false, nil, io.Discard)
	_ = runContext(stagedRoot, nil, true, "", true, false, nil, io.Discard)
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(ctxCmdYAML+"targets: [unknown]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"x"}, false, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("invalid target accepted")
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "awf.lock"), []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"x"}, false, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("bad lock accepted")
	}
	broken := ctxCmdFixture(t)
	if err := os.WriteFile(filepath.Join(broken, ".awf", "topics", "parts", "alpha", "one", "current-state.md"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(broken, []string{"x"}, false, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("broken context accepted")
	}
	if err := runContext(broken, nil, false, "", true, false, nil, io.Discard); err == nil {
		t.Fatal("broken uncovered accepted")
	}
	static := t.TempDir()
	if err := runContext(static, []string{"x"}, false, "", false, false, nil, &failOutput{}); err == nil {
		t.Fatal("stdout failure accepted")
	}
}

type failOutput struct{}

func (*failOutput) Write([]byte) (int, error) { return 0, errors.New("stdout") }

func TestRunUncoveredStaticAndFilesystemErrors(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := runContext(root, nil, false, "", true, false, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "all scanned paths") {
		t.Fatal(out.String())
	}
	bad := filepath.Join(t.TempDir(), "root")
	if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runContext(bad, []string{"x"}, false, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("accepted file root")
	}
	if err := runContext(bad, nil, false, "", true, false, nil, io.Discard); err == nil {
		t.Fatal("uncovered accepted file root")
	}
}
