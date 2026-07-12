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
  - beta
  - gamma
invariants:
  sources:
    - globs:
        - '**/*.go'
      marker: '//'
`

// ctxFixture builds an adopted tree with a current lock (so the gate passes),
// three domains (alpha+beta both own cmd/**, gamma owns nothing), a source file
// backing a marker under cmd/, and an ADR tagged alpha declaring an inv slug.
func ctxFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, ctxCmdYAML)
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "alpha.yaml"), "paths:\n  - cmd/**\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "beta.yaml"), "paths:\n  - cmd/**\n  - lib/**\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "gamma.yaml"), "paths: []\n")
	l := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "x.go"), "package x\n// invariant: backed-here\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-a.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
			testsupport.WithTitle("0001: Alpha decision"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- `inv: declared-slug` — a contract.\n## Consequences\nc\n")))
	// A plan linking ADR 0001 (alpha-owned → surfaced for cmd/ queries).
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-linked.md"),
		"---\ndate: 2026-07-12\nadrs: [1]\nstatus: Proposed\n---\n# Plan: Linked\n")
	return root
}

// The human render shows owning domains, path-backed invariants, and related
// ADRs with their declared slugs; unowned paths get their own section.
func TestRunContextHuman(t *testing.T) {
	root := ctxFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"cmd/x.go", "README.md"}, false, "", false, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"live state for this project",
		"alpha — docs/domains/alpha.md",
		"beta — docs/domains/beta.md",
		"backed-here",
		"ADR-0001 (Implemented) Alpha decision — docs/decisions/0001-a.md",
		"invariants: [declared-slug]",
		"## Related plans",
		"2026-07-12-linked.md (Proposed) — docs/plans/2026-07-12-linked.md",
		"## Unowned paths",
		"README.md",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q\n%s", want, got)
		}
	}
}

// The JSON render carries the same assembled set as the human render — one
// assembler feeds both (inv: context-output-parity).
func TestRunContextJSONParity(t *testing.T) {
	root := ctxFixture(t)
	var jsonOut bytes.Buffer
	if err := runContext(root, []string{"cmd/x.go"}, false, "", true, &jsonOut); err != nil {
		t.Fatal(err)
	}
	var res project.ContextResult
	if err := json.Unmarshal(jsonOut.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(res.Domains) != 2 || res.Domains[0].Name != "alpha" || res.Domains[1].Name != "beta" {
		t.Errorf("json domains: %+v", res.Domains)
	}
	if strings.Join(res.Invariants, ",") != "backed-here" {
		t.Errorf("json invariants: %v", res.Invariants)
	}
	if len(res.ADRs) != 1 || strings.Join(res.ADRs[0].Invariants, ",") != "declared-slug" {
		t.Errorf("json adrs: %+v", res.ADRs)
	}
	if len(res.Plans) != 1 || res.Plans[0].Filename != "2026-07-12-linked.md" || res.Plans[0].Status != "Proposed" {
		t.Errorf("json plans: %+v", res.Plans)
	}
	// Same set as the human render.
	var humanOut bytes.Buffer
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, &humanOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha", "beta", "backed-here", "declared-slug", "2026-07-12-linked.md"} {
		if !strings.Contains(humanOut.String(), want) {
			t.Errorf("human render diverges from JSON: missing %q", want)
		}
	}
}

// Outside an adopted tree the command prints the static pre-adoption notice and
// succeeds — never refuses (inv: context-static-fallback). The JSON variant
// emits the paths-only result.
func TestRunContextStaticFallback(t *testing.T) {
	var human bytes.Buffer
	if err := runContext(t.TempDir(), []string{"cmd/x.go"}, false, "", false, &human); err != nil {
		t.Fatalf("static human errored: %v", err)
	}
	if !strings.Contains(human.String(), "not inside an awf project") {
		t.Errorf("static human: %s", human.String())
	}
	var j bytes.Buffer
	if err := runContext(t.TempDir(), []string{"cmd/x.go"}, false, "", true, &j); err != nil {
		t.Fatalf("static json errored: %v", err)
	}
	var res project.ContextResult
	if err := json.Unmarshal(j.Bytes(), &res); err != nil || strings.Join(res.Paths, ",") != "cmd/x.go" {
		t.Errorf("static json: %s (err %v)", j.String(), err)
	}
	if len(res.Plans) != 0 {
		t.Errorf("static fallback must leave Plans empty, got %+v", res.Plans)
	}
}

// A stat fault that is not absence (a file where .awf should be) surfaces as an
// error, never silently treated as pre-adoption.
func TestRunContextStatFault(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".awf"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, io.Discard); err == nil {
		t.Error("a non-absence stat fault must surface")
	}
}

// Inside an adopted tree the command is gated: a binary behind the project
// refuses like every gated command.
func TestRunContextGated(t *testing.T) {
	root := gateFixture(t, "99.0.0", migrate.Current())
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, io.Discard); err == nil {
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
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, io.Discard); err == nil {
		t.Error("expected the open-time validation error")
	}
}

// A fault assembling the context (here a malformed ADR) surfaces as the
// command's error rather than a panic or silence.
func TestRunContextAssembleFault(t *testing.T) {
	root := ctxFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0009-bad.md"), "---\nstatus: \"unterminated\n---\n# ADR-X: T\n")
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, io.Discard); err == nil {
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

// inv: context-read-only — awf context writes nothing: file mtimes and the lock
// bytes are byte-identical before and after runs across the command's branches.
func TestRunContextReadOnly(t *testing.T) {
	root := ctxFixture(t)
	before := snapshotTree(t, root)
	lockBefore := readFile(t, filepath.Join(root, ".awf", "awf.lock"))
	for _, tc := range []struct {
		paths  []string
		asJSON bool
	}{
		{[]string{"cmd/x.go"}, false},
		{[]string{"cmd/x.go"}, true},
		{[]string{"README.md"}, false},
	} {
		if err := runContext(root, tc.paths, false, "", tc.asJSON, io.Discard); err != nil {
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
