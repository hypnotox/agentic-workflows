package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
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
// a global core topic, the scoped topic alpha/one (a rule and an unbacked
// invariant), an Accepted v1 ADR with a pending add on alpha/one, and a state
// marker under internal/foo/x.go.
func ctxCmdFixture(t *testing.T) string {
	t.Helper()
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, ctxCmdYAML)
	lock := &manifest.Lock{
		AWFVersion: awfVersion(), SchemaVersion: migrate.Current(),
		Files:             map[string]manifest.Entry{},
		BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "x", TreeDigest: "sha256:x", ADRFormatV1From: 2},
	}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\nOrigin: ADR-0001\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n",
		".awf/topics/metadata/core/g.yaml":             "title: Global\nsummary: Global rules.\napplies: global\n",
		".awf/topics/parts/core/g/current-state.md":    "Intro.\n\n## Claims\n\n### `rule: everywhere`\nApplies everywhere.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"),
			testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
		"docs/decisions/0002-later.md": acceptedV1(t, "0002", "Later", "2026-07-20", "- add `alpha/one:pending-rule`"),
		"internal/foo/x.go":            "package foo\n// state: alpha/one:order\n",
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root
}

// TestRunContextHuman shows owning domains, the applicable scoped and global
// topics with their current claims (narrowed by the state marker), the Accepted
// pending change, and the unowned path in its own section.
func TestRunContextHuman(t *testing.T) {
	root := ctxCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"internal/foo/x.go", "README.md"}, false, "", false, false, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"live state for this project",
		"alpha: docs/domains/alpha.md",
		"alpha/one - One",
		"[rule] alpha/one:order: Order is deterministic.",
		"core/g (global) - Global",
		"[rule] core/g:everywhere:",
		"## Pending accepted changes",
		"ADR-0002 (Later) add alpha/one:pending-rule",
		"## Unowned paths",
		"README.md",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q\n%s", want, got)
		}
	}
	// The state marker narrows alpha/one to the order claim; stable must not show.
	if strings.Contains(got, "alpha/one:stable") {
		t.Errorf("state marker should have narrowed away the stable claim:\n%s", got)
	}
}

// TestPrintContextTitlelessTopic covers the human render of a topic with no
// title, which prints the bare id without a dangling separator.
func TestPrintContextTitlelessTopic(t *testing.T) {
	res := project.ContextResult{Topics: []project.TopicContext{{ID: "alpha/untitled"}}}
	var out bytes.Buffer
	if err := printContext(&out, res, false, "header"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "  alpha/untitled\n") {
		t.Errorf("title-less topic not rendered bare:\n%s", got)
	}
	if strings.Contains(got, "alpha/untitled -") {
		t.Errorf("title-less topic rendered a dangling separator:\n%s", got)
	}
}

// TestRunContextJSONParity proves the JSON render carries the same assembled set
// as the human render - one assembler feeds both.
// invariant: tooling/cli:context-output-parity
func TestRunContextJSONParity(t *testing.T) {
	root := ctxCmdFixture(t)
	var jsonOut bytes.Buffer
	if err := runContext(root, []string{"internal/foo/x.go"}, false, "", true, false, &jsonOut); err != nil {
		t.Fatal(err)
	}
	var res project.ContextResult
	if err := json.Unmarshal(jsonOut.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(res.Domains) != 1 || res.Domains[0].Name != "alpha" {
		t.Errorf("json domains: %+v", res.Domains)
	}
	if len(res.Topics) != 2 || res.Topics[0].ID != "alpha/one" || res.Topics[1].ID != "core/g" {
		t.Fatalf("json topics: %+v", res.Topics)
	}
	if len(res.Pending) != 1 || res.Pending[0].ADR != "0002" || res.Pending[0].Claim != "alpha/one:pending-rule" {
		t.Errorf("json pending: %+v", res.Pending)
	}
	// Same set as the human render.
	var humanOut bytes.Buffer
	if err := runContext(root, []string{"internal/foo/x.go"}, false, "", false, false, &humanOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha/one", "core/g", "pending-rule"} {
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
// invariant: tooling/cli:context-static-fallback
func TestRunContextStaticFallback(t *testing.T) {
	var human bytes.Buffer
	if err := runContext(t.TempDir(), []string{"cmd/x.go"}, false, "", false, false, &human); err != nil {
		t.Fatalf("static human errored: %v", err)
	}
	if !strings.Contains(human.String(), "not inside an awf project") {
		t.Errorf("static human: %s", human.String())
	}
	var j bytes.Buffer
	if err := runContext(t.TempDir(), []string{"cmd/x.go"}, false, "", true, false, &j); err != nil {
		t.Fatalf("static json errored: %v", err)
	}
	var res project.ContextResult
	if err := json.Unmarshal(j.Bytes(), &res); err != nil || strings.Join(res.Paths, ",") != "cmd/x.go" {
		t.Errorf("static json: %s (err %v)", j.String(), err)
	}
	if len(res.Topics) != 0 {
		t.Errorf("static fallback must leave Topics empty, got %+v", res.Topics)
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
}

// awf context writes nothing: file mtimes and the lock bytes are byte-identical
// before and after runs across the command's branches.
// invariant: tooling/cli:context-read-only
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
}

// The --uncovered JSON render carries the same set as the human render.
// invariant: tooling/cli:uncovered-output-parity
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
	if strings.Join(res.Unowned, ",") != "README.md,stray.txt" {
		t.Errorf("json unowned: %v want [README.md stray.txt]", res.Unowned)
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
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		b.WriteString(rel + "@" + info.ModTime().String() + ";")
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
