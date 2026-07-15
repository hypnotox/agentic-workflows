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
docs:
  - pitfalls
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
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "x.go"), "package x\n"+
		"// invariant: gov-slug\n"+
		"// touches-invariant: unbk-slug — the reasoned production site.\n"+
		"// invariant: orphan-slug\n") // present but declared by no ADR → no class label
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-a.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("precise"),
			testsupport.WithTitle("0001: Alpha decision"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- `invariant: gov-slug` — a contract.\n"+
				"- `unbacked-invariant: unbk-slug` — a reasoned contract. **Verify:** inspect by hand.\n## Consequences\nc\n")))
	// 0002 shares the precise tag → Tier 2 (Related). 0003 is domain-owned only →
	// Tier 3 background count. Both exercise their render blocks.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0002-b.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("precise"),
			testsupport.WithTitle("0002: Related decision"), testsupport.WithDomains("alpha"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0003-c.md"),
		testsupport.ADR("Proposed", testsupport.WithDate("2026-06-25"), testsupport.WithTags("other"),
			testsupport.WithTitle("0003: Background decision"), testsupport.WithDomains("beta"),
			testsupport.WithBody("## Invariants\n- textual only.\n## Consequences\nc\n")))
	// A plan linking the Tier-1 ADR 0001 → surfaced for cmd/ queries.
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-07-12-linked.md"),
		"---\ndate: 2026-07-12\nadrs: [1]\nstatus: Proposed\n---\n# Plan: Linked\n")
	// A pitfall sharing the precise tag → surfaced for cmd/ queries.
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "docs", "pitfalls.yaml"),
		"data:\n  pitfalls:\n    - title: Worktree hazard\n      tags: [precise]\n      body: use a worktree\n")
	return root
}

// The human render shows owning domains, path-backed invariants, the Tier-1
// governing ADR, the linked plan, and the tag-matched pitfall; unowned paths get
// their own section.
func TestRunContextHuman(t *testing.T) {
	root := ctxFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"cmd/x.go", "README.md"}, false, "", false, false, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"live state for this project",
		"alpha — docs/domains/alpha.md",
		"beta — docs/domains/beta.md",
		"gov-slug [backed]",
		"unbk-slug [unbacked]",
		"\n  orphan-slug\n", // present but undeclared → rendered without a class label
		"Verify: inspect by hand.",
		"touches: — the reasoned production site.",
		"## Governing ADRs (invariants backed here)",
		"ADR-0001 (Implemented) Alpha decision — docs/decisions/0001-a.md",
		"## Related ADRs (shared tag)",
		"ADR-0002 (Accepted) Related decision — docs/decisions/0002-b.md",
		"## Domain background: 1 more ADR(s)",
		"## Related plans",
		"2026-07-12-linked.md (Proposed) — docs/plans/2026-07-12-linked.md",
		"## Related pitfalls (shared tag)",
		"Worktree hazard [precise] — docs/pitfalls.md",
		"## Unowned paths",
		"README.md",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q\n%s", want, got)
		}
	}
}

// The JSON render carries the same assembled set as the human render - one
// assembler feeds both (inv: context-output-parity).
func TestRunContextJSONParity(t *testing.T) {
	root := ctxFixture(t)
	var jsonOut bytes.Buffer
	if err := runContext(root, []string{"cmd/x.go"}, false, "", true, false, &jsonOut); err != nil {
		t.Fatal(err)
	}
	var res project.ContextResult
	if err := json.Unmarshal(jsonOut.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(res.Domains) != 2 || res.Domains[0].Name != "alpha" || res.Domains[1].Name != "beta" {
		t.Errorf("json domains: %+v", res.Domains)
	}
	// Three path-present invariants, slug-sorted: the backed gov-slug (proof
	// marker), an orphan-slug present but declared by no ADR (no class), and the
	// unbacked unbk-slug (touches marker) carrying its Verify note + site note.
	if len(res.Invariants) != 3 || res.Invariants[0].Slug != "gov-slug" ||
		res.Invariants[1].Slug != "orphan-slug" || res.Invariants[2].Slug != "unbk-slug" {
		t.Fatalf("json invariants: %+v", res.Invariants)
	}
	if res.Invariants[0].Class != "backed" {
		t.Errorf("gov-slug class: got %q want backed", res.Invariants[0].Class)
	}
	if res.Invariants[1].Class != "" {
		t.Errorf("orphan-slug must carry no class, got %q", res.Invariants[1].Class)
	}
	unbk := res.Invariants[2]
	if unbk.Class != "unbacked" || unbk.Verify != "inspect by hand." {
		t.Errorf("unbk-slug label: %+v", unbk)
	}
	if len(unbk.Touches) != 1 || !strings.Contains(unbk.Touches[0], "reasoned production site") {
		t.Errorf("unbk-slug touches: %+v", unbk.Touches)
	}
	if len(res.Governing) != 1 || res.Governing[0].Number != "0001" {
		t.Errorf("json governing: %+v", res.Governing)
	}
	if len(res.Plans) != 1 || res.Plans[0].Filename != "2026-07-12-linked.md" || res.Plans[0].Status != "Proposed" {
		t.Errorf("json plans: %+v", res.Plans)
	}
	if len(res.Pitfalls) != 1 || res.Pitfalls[0].Title != "Worktree hazard" || res.Pitfalls[0].Path != "docs/pitfalls.md" {
		t.Errorf("json pitfalls: %+v", res.Pitfalls)
	}
	// Same set as the human render.
	var humanOut bytes.Buffer
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, false, &humanOut); err != nil {
		t.Fatal(err)
	}
	// invariant: context-output-parity
	for _, want := range []string{"alpha", "beta", "gov-slug", "2026-07-12-linked.md", "Worktree hazard"} {
		if !strings.Contains(humanOut.String(), want) {
			t.Errorf("human render diverges from JSON: missing %q", want)
		}
	}
}

// Outside an adopted tree the command prints the static pre-adoption notice and
// succeeds - never refuses (inv: context-static-fallback). The JSON variant
// emits the paths-only result.
func TestRunContextStaticFallback(t *testing.T) {
	var human bytes.Buffer
	if err := runContext(t.TempDir(), []string{"cmd/x.go"}, false, "", false, false, &human); err != nil {
		t.Fatalf("static human errored: %v", err)
	}
	// invariant: context-static-fallback
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

// A fault assembling the context (here a malformed ADR) surfaces as the
// command's error rather than a panic or silence.
func TestRunContextAssembleFault(t *testing.T) {
	root := ctxFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0009-bad.md"), "---\nstatus: \"unterminated\n---\n# ADR-X: T\n")
	if err := runContext(root, []string{"cmd/x.go"}, false, "", false, false, io.Discard); err == nil {
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

// inv: context-read-only - awf context writes nothing: file mtimes and the lock
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
		if err := runContext(root, tc.paths, false, "", tc.asJSON, false, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	// invariant: context-read-only
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
  - render
invariants:
  sources:
    - globs:
        - '**/*.go'
      marker: '//'
`

// uncoveredCmdFixture builds an adopted tree that is ALSO a git repo (so
// TrackedPaths resolves): a domain owning internal/render/**, a current lock, and
// two committed files - one covered (internal/render/r.go), one not
// (internal/plan/p.go).
func uncoveredCmdFixture(t *testing.T) string {
	t.Helper()
	repo, root := gitfixture.InitRepo(t)
	testsupport.WriteAwfConfig(t, root, uncoveredCmdYAML)
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "render.yaml"), "paths:\n  - internal/render/**\n")
	l := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"internal/render", "internal/plan"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitfixture.Commit(t, repo, root, "base", map[string]string{
		"internal/render/r.go": "package r\n",
		"internal/plan/p.go":   "package p\n",
	})
	return root
}

// --uncovered lists tracked paths owned by no domain, collapsing a fully-uncovered
// subtree; a scan root renders the `scan roots:` line.
func TestRunContextUncoveredHuman(t *testing.T) {
	root := uncoveredCmdFixture(t)
	var out bytes.Buffer
	if err := runContext(root, []string{"internal/plan"}, false, "", false, true, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"## Uncovered", "internal/plan/", "scan roots:"} {
		if !strings.Contains(got, want) {
			t.Errorf("uncovered human missing %q\n%s", want, got)
		}
	}
}

// The --uncovered JSON render carries the same uncovered set as the human render -
// one assembled UncoveredResult feeds both (inv: uncovered-output-parity).
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
	if strings.Join(res.Entries, ",") != "internal/plan/" {
		t.Errorf("json entries: got %v want [internal/plan/]", res.Entries)
	}
	var human bytes.Buffer
	if err := runContext(root, nil, false, "", false, true, &human); err != nil {
		t.Fatal(err)
	}
	// invariant: uncovered-output-parity
	if !strings.Contains(human.String(), "internal/plan/") {
		t.Errorf("human render diverges from JSON: %s", human.String())
	}
}

// --uncovered rejects the --staged/--range selectors (a different intent).
func TestRunContextUncoveredRejectsSelectors(t *testing.T) {
	if err := runContext(t.TempDir(), nil, true, "", false, true, io.Discard); err == nil || !strings.Contains(err.Error(), "--staged/--range") {
		t.Errorf("--uncovered with --staged must be a usage error, got: %v", err)
	}
}

// Outside an adopted tree --uncovered prints the static notice and succeeds (inv:
// context-static-fallback); the empty result exercises printUncovered's no-entries
// branch.
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

// An adopted tree that is not a git repo makes TrackedPaths fault, surfacing as the
// command's error.
func TestRunContextUncoveredTrackedPathsFault(t *testing.T) {
	root := ctxFixture(t) // adopted (config+lock+domains) but no git repo
	if err := runContext(root, nil, false, "", false, true, io.Discard); err == nil {
		t.Error("expected a TrackedPaths open-repo error in a non-git adopted tree")
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
