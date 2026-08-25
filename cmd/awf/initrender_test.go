package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestEmptyInitRendersCoherently is the out-of-box floor oracle (ADR-0045): a
// non-interactive `awf init` with no answers must render artifacts with no
// empty inline code spans, no tables without body rows, and no dangling
// list-introduction sentences.
// invariant: rendering/templates:empty-init-coherent-render (TestEmptyInitChecksOnUnbornHead)
// invariant: tooling/init-and-enablement:init-unborn-head-supported (TestEmptyInitChecksOnUnbornHead)
func TestEmptyInitChecksOnUnbornHead(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	forceNonInteractive(t)
	root := gitfixture.InitRepo(t).Root()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	var initOut, initErr bytes.Buffer
	if code := run([]string{"awf", "init"}, &initOut, &initErr); code != 0 {
		t.Fatalf("init before first commit: exit %d (%s)", code, initErr.String())
	}
	// The unconditional hook payloads require a gate command before render or
	// check. Init remains usable with the empty scaffolded value.
	setScaffoldGateCmd(t, root)
	var syncOut, syncErr bytes.Buffer
	if code := run([]string{"awf", "render"}, &syncOut, &syncErr); code != 0 {
		t.Fatalf("render before first commit: exit %d (%s)", code, syncErr.String())
	}
	gitfixture.AddAll(t, gitfixture.At(root))
	var checkOut, checkErr bytes.Buffer
	if code := run([]string{"awf", "check"}, &checkOut, &checkErr); code != 0 {
		t.Fatalf("check before first commit: exit %d (%s)\n%s", code, checkErr.String(), checkOut.String())
	}
}

// setScaffoldGateCmd fills the scaffold's empty gateCmd var in-place: the
// actionable follow-up the ADR-0156 command-wiring error demands of a
// no-answer init before its first ordinary sync or check.
func setScaffoldGateCmd(t *testing.T, root string) {
	t.Helper()
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	replaced := strings.Replace(string(b), `gateCmd: ""`, "gateCmd: make gate", 1)
	if replaced == string(b) {
		t.Fatalf("scaffold config carries no empty gateCmd seed:\n%s", b)
	}
	if err := os.WriteFile(cfgPath, []byte(replaced), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitFirstADRChecksCleanRender(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	testInitFirstADRChecksClean(t)
}

func TestEmptyInitRendersCoherently(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	forceNonInteractive(t)
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("init: exit %d (%s)", code, errb.String())
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				continue // fence delimiter
			}
			if unpairedDoubleBacktickRun(line) {
				t.Errorf("%s:%d: unpaired double-backtick run (empty-var residue): %q", rel, i+1, line)
			}
			if isTableSeparator(line) {
				next := ""
				if lines := strings.Split(string(b), "\n"); i+1 < len(lines) {
					next = lines[i+1]
				}
				if !strings.HasPrefix(next, "|") {
					t.Errorf("%s:%d: table separator with no body rows", rel, i+1)
				}
			}
			if strings.HasSuffix(line, "include:") {
				next := ""
				for _, candidate := range strings.Split(string(b), "\n")[i+1:] {
					if strings.TrimSpace(candidate) != "" {
						next = candidate
						break
					}
				}
				if !strings.HasPrefix(strings.TrimSpace(next), "- ") {
					t.Errorf("%s:%d: list introduction with no items: %q", rel, i+1, line)
				}
			}
			if strings.Contains(line, "sections:** in that order") {
				t.Errorf("%s:%d: dangling list-introduction sentence: %q", rel, i+1, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// With the gateCmd wiring supplied (the one follow-up the ADR-0156
	// command-wiring error demands), the tree must pass check with notes only
	// (advisory, exit 0) - in particular zero dead-skill-reference findings on
	// the curated default.
	setScaffoldGateCmd(t, root)
	if err := runSync(ctx, root, io.Discard); err != nil {
		t.Fatalf("sync after wiring gateCmd: %v", err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "rendered", nil)
	var checkOut bytes.Buffer
	if err := runCheck(ctx, root, &checkOut); err != nil {
		t.Fatalf("check on fresh init: %v\n%s", err, checkOut.String())
	}
	if strings.Contains(checkOut.String(), "dead-skill-reference") {
		t.Errorf("curated init render has dead skill references:\n%s", checkOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs/decisions/template.md")); !os.IsNotExist(err) {
		t.Fatalf("default Core init emitted Full-only ADR template: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil || !strings.Contains(string(cfg), "profile: core") {
		t.Fatalf("default init profile = %q, %v", cfg, err)
	}
}

// unpairedDoubleBacktickRun reports whether the line holds an odd number of
// >=2-backtick runs - an unpaired double run is the residue of an empty-var
// span, while a legitimate double-backtick-delimited span (and an inline
// triple-backtick run) contributes a pair.
func unpairedDoubleBacktickRun(line string) bool {
	runs := 0
	length := 0
	for _, r := range line + " " {
		if r == '`' {
			length++
			continue
		}
		if length >= 2 {
			runs++
		}
		length = 0
	}
	return runs%2 == 1
}

// isTableSeparator matches a markdown table separator row: starts with '|',
// contains a '-', and holds only '|', '-', ':', and spaces.
func isTableSeparator(line string) bool {
	if !strings.HasPrefix(line, "|") || !strings.Contains(line, "-") {
		return false
	}
	return strings.IndexFunc(line, func(r rune) bool {
		return r != '|' && r != '-' && r != ':' && r != ' '
	}) == -1
}

// Unset-var notes are advisory: they print and never affect the exit code.
// invariant: tooling/cli:completeness-advisory-nonfailing (TestCheckUnsetVarNotesAreNonFailing)
func TestCheckUnsetVarNotesAreNonFailing(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {testCmd: \"\", gateCmd: make gate}\n")
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("sync: %v", err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "rendered", nil)
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("check must stay clean with unset vars, got: %v", err)
	}
	if !strings.Contains(out.String(), "advisory | skill tdd references unset vars: testCmd") {
		t.Errorf("missing unset-var note, got:\n%s", out.String())
	}
}

// Stub notes are advisory: they print and never affect the exit code.
// invariant: tooling/cli:stub-advisory-nonfailing (TestCheckStubNotesAreNonFailing)
func TestCheckStubNotesAreNonFailing(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate, gateCmdFull: make gate full}\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "parts", "tdd", "notes.md"),
		"<!-- awf:stub -->\nstarter notes\n")
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("sync: %v", err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "rendered", nil)
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("check must stay clean with unauthored stub content, got: %v", err)
	}
	if !strings.Contains(out.String(), "advisory |") ||
		!strings.Contains(out.String(), "has unauthored stub content: stub-marked parts: notes") {
		t.Errorf("missing stub note, got:\n%s", out.String())
	}
}

// Glossary terseness notes are advisory: they print and never affect the exit
// code, exactly like the unset-var and stub families.
// invariant: tooling/cli:terseness-advisory-nonfailing (TestCheckGlossaryTersenessNotesAreNonFailing)
func TestCheckGlossaryTersenessNotesAreNonFailing(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "docs", "glossary.yaml"),
		"data:\n  terms:\n    - term: bloated\n      meaning: \""+strings.Repeat("x", 400)+"\"\n")
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("sync: %v", err)
	}
	gitfixture.AddAll(t, repo)
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("check must stay clean with an over-length glossary meaning, got: %v", err)
	}
	if !strings.Contains(out.String(), "advisory |") ||
		!strings.Contains(out.String(), `term "bloated" meaning is 400 characters`) {
		t.Errorf("missing glossary terseness note, got:\n%s", out.String())
	}
}

func TestCheckSurfacesUnsetVarNoteRenderError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "tdd.yaml"),
		"data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n")
	if err := runCheck(ctx, root, io.Discard); err == nil {
		t.Fatal("expected check to surface the render error from the notes pass")
	}
}

func TestCheckFullySetArtifactEmitsNoUnsetVarNote(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	repo := gitfixture.At(root)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "rendered", nil)
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("check: %v", err)
	}
	// The fixture sets every var the tdd skill references; other artifacts
	// (agents-doc) legitimately reference more and may still note.
	if strings.Contains(out.String(), "skill tdd references unset vars") {
		t.Errorf("unexpected unset-var note for the fully-set skill:\n%s", out.String())
	}
}

func TestRunInitSyncError(t *testing.T) {
	ctx := testContext(t)
	// Config exists (skip scaffold); a squatting output dir makes the inner
	// runSync fail, covering runInit's runSync error return.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(ctx, root, false, false, nil, "", io.Discard); err == nil {
		t.Error("expected runInit to surface the sync error")
	}
}
