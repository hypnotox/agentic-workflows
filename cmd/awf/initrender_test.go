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
// invariant: rendering/templates:empty-init-coherent-render
// invariant: tooling/cli:init-unborn-head-supported
func TestEmptyInitChecksOnUnbornHead(t *testing.T) {
	forceNonInteractive(t)
	_, root := gitfixture.InitRepo(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	var initOut, initErr bytes.Buffer
	if code := run([]string{"awf", "init"}, &initOut, &initErr); code != 0 {
		t.Fatalf("init before first commit: exit %d (%s)", code, initErr.String())
	}
	var checkOut, checkErr bytes.Buffer
	if code := run([]string{"awf", "check"}, &checkOut, &checkErr); code != 0 {
		t.Fatalf("check before first commit: exit %d (%s)\n%s", code, checkErr.String(), checkOut.String())
	}
}

// invariant: adr-system/adr-lifecycle:fresh-adoption-v1-cutoff
func TestInitFirstADRChecksClean(t *testing.T) {
	testInitFirstADRChecksClean(t)
}

func TestEmptyInitRendersCoherently(t *testing.T) {
	forceNonInteractive(t)
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
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
				if lines := strings.Split(string(b), "\n"); i+1 < len(lines) {
					next = lines[i+1]
				}
				if !strings.HasPrefix(next, "- ") {
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
	// The fresh tree must also pass check, with notes only (advisory, exit 0) -
	// in particular zero dead-skill-reference findings on the curated default.
	// invariant: rendering/project-output-plan:curated-init-skill-refs-clean
	var checkOut bytes.Buffer
	if err := runCheck(root, false, &checkOut); err != nil {
		t.Fatalf("check on fresh init: %v\n%s", err, checkOut.String())
	}
	if strings.Contains(checkOut.String(), "dead-skill-reference") {
		t.Errorf("curated init render has dead skill references:\n%s", checkOut.String())
	}
	template, err := os.ReadFile(filepath.Join(root, "docs/decisions/template.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(template)
	history := strings.Index(text, "## Status history\n")
	if !strings.Contains(text, "format: current-state-v2") || history < 0 || strings.Index(text, "Implementing; content-sha256") > strings.Index(text, "Applied; state-sequence") {
		t.Fatalf("empty-init V2 ADR template is not lifecycle-safe:\n%s", text)
	}
	if tail := text[history:]; strings.Count(tail, "- YYYY-MM-DD:") != 1 || !strings.Contains(tail, "- YYYY-MM-DD: Proposed") {
		t.Fatalf("fresh Proposed scaffold includes later events:\n%s", tail)
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
// invariant: tooling/cli:completeness-advisory-nonfailing
func TestCheckUnsetVarNotesAreNonFailing(t *testing.T) {
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {testCmd: go test ./..., gateCmd: \"\"}\nskills: [tdd]\nagents: []\n")
	if err := initializeProject(root, io.Discard); err != nil {
		t.Fatalf("sync: %v", err)
	}
	var out bytes.Buffer
	if err := runCheck(root, false, &out); err != nil {
		t.Fatalf("check must stay clean with unset vars, got: %v", err)
	}
	if !strings.Contains(out.String(), "note: skill tdd references unset vars: gateCmd") {
		t.Errorf("missing unset-var note, got:\n%s", out.String())
	}
}

// Stub notes are advisory: they print and never affect the exit code.
// invariant: tooling/cli:stub-advisory-nonfailing
func TestCheckStubNotesAreNonFailing(t *testing.T) {
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {testCmd: go test ./..., gateCmd: make gate, gateCmdFull: make gate full}\nskills: [tdd]\nagents: []\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "parts", "tdd", "notes.md"),
		"<!-- awf:stub -->\nstarter notes\n")
	if err := initializeProject(root, io.Discard); err != nil {
		t.Fatalf("sync: %v", err)
	}
	var out bytes.Buffer
	if err := runCheck(root, false, &out); err != nil {
		t.Fatalf("check must stay clean with unauthored stub content, got: %v", err)
	}
	if !strings.Contains(out.String(), "note: ") ||
		!strings.Contains(out.String(), "has unauthored stub content: stub-marked parts: notes") {
		t.Errorf("missing stub note, got:\n%s", out.String())
	}
}

func TestCheckSurfacesUnsetVarNoteRenderError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {}\nskills: [tdd]\nagents: []\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "tdd.yaml"),
		"data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n")
	if err := runCheck(root, false, io.Discard); err == nil {
		t.Fatal("expected check to surface the render error from the notes pass")
	}
}

func TestCheckFullySetArtifactEmitsNoUnsetVarNote(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCheck(root, false, &out); err != nil {
		t.Fatalf("check: %v", err)
	}
	// The fixture sets every var the tdd skill references; other artifacts
	// (agents-doc) legitimately reference more and may still note.
	if strings.Contains(out.String(), "skill tdd references unset vars") {
		t.Errorf("unexpected unset-var note for the fully-set skill:\n%s", out.String())
	}
}
