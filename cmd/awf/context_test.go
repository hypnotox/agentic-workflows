package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/clispec"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const ctxCmdYAML = `prefix: example
integrationBranch: main
vars:
  gateCmd: make gate
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
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
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
		"internal/foo/y_test.go":       "package foo\n// invariant: alpha/one:tested (TestTested)\nfunc TestTested() {}\n",
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root
}

func TestRunContextAdoptedNonGitProject(t *testing.T) {
	root := ctxCmdFixture(t)
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runContext(testContext(t), root, []string{"internal/foo/x.go"}, false, "", false, false, nil, &out); err != nil {
		t.Fatalf("context in adopted non-Git project: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "alpha/one | One") || !strings.Contains(got, "classification: covered") {
		t.Fatalf("non-Git context omitted live authority:\n%s", got)
	}
}

func TestRunContextHumanAndFacets(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := ctxCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(ctx, root, []string{"internal/foo"}, false, "", false, false, []string{"relationships", "invariants", "evidence", "all-rules", "evidence"}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"selection: explicit", "request-1:", "directory: 3 included", "authority:", "alpha/one | One", "test", "alpha/one | alpha/one:tested | invariant | 1 | internal/foo/y_test.go:2"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestContextHeadingsPassSemanticValuesOnly(t *testing.T) {
	ctx := testContext(t)
	root := ctxCmdFixture(t)
	cases := []struct {
		name string
		run  func(*bytes.Buffer) error
		want string
	}{
		{"normal", func(out *bytes.Buffer) error {
			return runContext(ctx, root, []string{"internal/foo/x.go"}, false, "", false, false, nil, out)
		}, "context: live state for this project\n"},
		{"uncovered", func(out *bytes.Buffer) error { return runContext(ctx, root, nil, false, "", true, false, nil, out) }, "context: coverage gaps for this project\n"},
	}
	gitfixture.AddAll(t, gitfixture.At(root))
	cases = append(cases, struct {
		name string
		run  func(*bytes.Buffer) error
		want string
	}{"staged", func(out *bytes.Buffer) error {
		return runContext(ctx, root, []string{"internal/foo/x.go"}, true, "", false, false, nil, out)
	}, "context: staged state for this project\n"})
	for _, tc := range cases {
		var out bytes.Buffer
		if err := tc.run(&out); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !strings.HasPrefix(out.String(), tc.want) {
			t.Errorf("%s heading:\n%s", tc.name, out.String())
		}
	}
}

func TestRunContextRendersMarkerRelationships(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := ctxCmdFixture(t)
	body := "package foo\n// state: alpha/one:order\n// touches-state: alpha/one:stable - exercised here\n// touches-state: alpha/one:stable - exercised here\n// invariant: alpha/one:tested (TestTested)\n// invariant: alpha/one:tested (TestTested)\nfunc TestTested() {}\n"
	if err := os.WriteFile(filepath.Join(root, "internal", "foo", "x_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runContext(ctx, root, []string{"internal/foo/x_test.go"}, false, "", false, false, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "state: alpha/one:order") || !strings.Contains(out.String(), "touches: alpha/one:stable") || !strings.Contains(out.String(), "proofs: alpha/one:tested") || strings.Contains(out.String(), "Direct rules:") || strings.Contains(out.String(), "Invariants:") {
		t.Fatalf("marker relationship projection missing:\n%s", out.String())
	}
}

func TestRenderContextRequestSourceAttribution(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := ctxCmdFixture(t)
	body := "package foo\n// state: alpha/one:order\n// touches-state: alpha/one:stable - exercised here\n// invariant: alpha/one:tested (TestTested)\nfunc TestTested() {}\n"
	if err := os.WriteFile(filepath.Join(root, "internal", "foo", "x_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runContext(ctx, root, []string{"internal/foo", "internal/foo/x_test.go"}, false, "", false, false, []string{"relationships"}, &out); err != nil {
		t.Fatal(err)
	}
	const sourceBlock = "claim-sources:\n" +
		"    alpha/one | alpha/one:order | 1 | State\n" +
		"    alpha/one | alpha/one:order | 2 | State\n" +
		"    alpha/one | alpha/one:stable | 1 | Touches\n" +
		"    alpha/one | alpha/one:stable | 2 | Touches\n" +
		"    alpha/one | alpha/one:tested | 1 | Proofs\n" +
		"    alpha/one | alpha/one:tested | 2 | Proofs\n"
	if got := out.String(); strings.Count(got, sourceBlock) != 1 {
		t.Fatalf("source attribution rows:\n%s", got)
	}
}

// invariant: tooling/context-and-topic:context-terminal-output-cap (TestRunContextModesShareDeliveryIncludingOversize)
func TestRunContextModesShareDeliveryIncludingOversize(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	var sizes []int
	deliver := func(rendered []byte, root string, stdout io.Writer) error {
		sizes = append(sizes, len(rendered))
		return nil
	}
	root := ctxCmdFixture(t)
	if err := runContextWithDelivery(ctx, root, []string{"internal/foo/x.go"}, false, "", false, false, nil, io.Discard, deliver); err != nil {
		t.Fatal(err)
	}
	if err := runContextWithDelivery(ctx, root, []string{"internal"}, false, "", true, false, nil, io.Discard, deliver); err != nil {
		t.Fatal(err)
	}
	if err := runContextWithDelivery(ctx, t.TempDir(), []string{"x"}, false, "", false, false, nil, io.Discard, deliver); err != nil {
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
	if err := runContextWithDelivery(ctx, root, []string{"internal/foo/x.go"}, false, "", false, false, []string{"all-rules"}, io.Discard, deliver); err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 4 || sizes[0] == 0 || sizes[1] == 0 || sizes[2] == 0 || sizes[3] <= 8192 {
		t.Fatalf("delivery sizes = %v", sizes)
	}
}

// invariant: tooling/context-and-topic:context-static-fallback (TestRunContextStaticAndUsage)
func TestRunContextStaticAndUsage(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	var out bytes.Buffer
	if err := runContext(ctx, root, []string{"x"}, false, "", false, false, nil, &out); err != nil {
		t.Fatal(err)
	}
	const want = "context: static: not inside an awf project; live classification and authority require an adopted project\nselection: explicit\n\nrequests:\n  status: none\n\nauthority:\n  topics: none\n"
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
		{nil, false, "", false, false, nil, "usage:"}, {[]string{"x"}, false, "", false, false, []string{"bad"}, "unknown context facet"}, {nil, false, "a..b", true, false, nil, "--range"}, {nil, false, "", true, true, nil, "cannot be combined"}, {nil, false, "", true, false, []string{"relationships"}, "cannot be combined"},
	}
	for _, tc := range cases {
		if err := runContext(ctx, root, tc.paths, tc.staged, tc.rng, tc.uncovered, tc.full, tc.shows, io.Discard); err == nil || !strings.Contains(err.Error(), tc.part) {
			t.Errorf("err=%v want %q", err, tc.part)
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

func TestContextHumanOnlyFlagGrammar(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	spec, _ := clispec.Lookup("context")
	for _, args := range [][]string{{"--json"}, {"--uncovered", "--json"}} {
		if _, err := parseArgs(spec, args); err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestContextDispatchHandler(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	var out bytes.Buffer
	if result := handlers["context"](&cmdCtx{ctx: testContext(t), root: t.TempDir(), inv: invocation{bools: map[string]bool{}, values: map[string]string{}, multi: map[string][]string{}, positionals: []string{"x"}}, stdout: &out}); result.err != nil {
		t.Fatal(result.err)
	}
}

func TestRunContextSelectionAndProjectErrors(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	if err := runContext(ctx, t.TempDir(), nil, true, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("changed-path selection accepted a non-repository")
	}
	root := ctxCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(ctx, root, []string{"internal/foo/x.go"}, false, "", false, true, nil, &out); err != nil {
		t.Fatal(err)
	}
	if err := runContext(ctx, root, []string{"internal"}, false, "", true, false, nil, &out); err != nil {
		t.Fatal(err)
	}
	if err := runContext(ctx, root, nil, false, "bad-range", false, false, nil, io.Discard); err == nil {
		t.Fatal("bad range accepted")
	}
	repo := gitfixture.InitRepo(t)
	rangeRoot := repo.Root()
	gitfixture.Commit(t, repo, "one", map[string]string{"a": "1"})
	gitfixture.Commit(t, repo, "two", map[string]string{"a": "2"})
	if err := runContext(ctx, rangeRoot, nil, false, "HEAD~1..HEAD", false, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	clean := gitfixture.InitRepo(t).Root()
	if err := runContext(ctx, clean, nil, true, "", false, false, nil, io.Discard); err == nil || !strings.Contains(err.Error(), "no changed paths") {
		t.Fatalf("empty staged err=%v", err)
	}
	if err := runContext(ctx, root, []string{"x"}, true, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("invalid staged transition accepted")
	}
	stagedRoot := ctxCmdFixture(t)
	gitfixture.AddAll(t, gitfixture.At(stagedRoot))
	_ = runContext(ctx, stagedRoot, []string{"internal/foo/x.go"}, true, "", false, false, nil, io.Discard)
	_ = runContext(ctx, stagedRoot, nil, true, "", true, false, nil, io.Discard)
	if err := os.WriteFile(filepath.Join(root, ".awf", "awf.lock"), []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(ctx, root, []string{"x"}, false, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("bad lock accepted")
	}
	broken := ctxCmdFixture(t)
	if err := os.WriteFile(filepath.Join(broken, ".awf", "topics", "parts", "alpha", "one", "current-state.md"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(ctx, broken, []string{"x"}, false, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("broken context accepted")
	}
	if err := runContext(ctx, broken, nil, false, "", true, false, nil, io.Discard); err == nil {
		t.Fatal("broken uncovered accepted")
	}
	static := t.TempDir()
	if err := runContext(ctx, static, []string{"x"}, false, "", false, false, nil, &failOutput{}); err == nil {
		t.Fatal("stdout failure accepted")
	}
}

func TestRunContextPropagatesMalformedRepository(t *testing.T) {
	root := ctxCmdFixture(t)
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := testContext(t)
	for _, uncovered := range []bool{false, true} {
		err := runContext(ctx, root, []string{"internal/foo/x.go"}, false, "", uncovered, false, nil, io.Discard)
		if err == nil || errors.Is(err, awfgit.ErrNotARepository) {
			t.Fatalf("uncovered=%v malformed repository error = %v", uncovered, err)
		}
	}
}

type failOutput struct{}

func (*failOutput) Write([]byte) (int, error) { return 0, errors.New("stdout") }

func TestRunUncoveredStaticAndFilesystemErrors(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	var out bytes.Buffer
	if err := runContext(ctx, root, nil, false, "", true, false, nil, &out); err != nil {
		t.Fatal(err)
	}
	const staticUncoveredGolden = "context: static: not inside an awf project; live coverage appears inside one\n\ncoverage:\n  result: all scanned paths are owned and covered by a scoped topic\n"
	if out.String() != staticUncoveredGolden {
		t.Fatalf("static uncovered:\n%s", out.String())
	}
	bad := filepath.Join(t.TempDir(), "root")
	if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runContext(ctx, bad, []string{"x"}, false, "", false, false, nil, io.Discard); err == nil {
		t.Fatal("accepted file root")
	}
	if err := runContext(ctx, bad, nil, false, "", true, false, nil, io.Discard); err == nil {
		t.Fatal("uncovered accepted file root")
	}
}
