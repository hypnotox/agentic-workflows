package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpExposesOnlyFinalCommands(t *testing.T) {
	code, stdout, stderr := runCLI(t, t.TempDir(), "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help = code %d, stderr %q", code, stderr)
	}
	for _, command := range []string{"init", "render", "check", "resolve", "docs", "new", "effort", "version"} {
		if !strings.Contains(stdout, "  "+command) {
			t.Errorf("help missing %s:\n%s", command, stdout)
		}
	}
	for _, retired := range []string{"  adr ", "  plan ", "  audit ", "  upgrade ", "  uninstall ", "  changelog ", "  edit ", "  reset "} {
		if strings.Contains(stdout, retired) {
			t.Errorf("help contains retired command %q:\n%s", retired, stdout)
		}
	}

	code, stdout, stderr = runCLI(t, t.TempDir(), "new", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("new help = code %d, stderr %q", code, stderr)
	}
	for _, command := range []string{"effort <slug>", "intent <slug>", "spec <slug>", "plan <slug>", "adr <slug>", "topic <id> <pattern>..."} {
		if !strings.Contains(stdout, command) {
			t.Errorf("new help missing %q", command)
		}
	}

	code, stdout, stderr = runCLI(t, t.TempDir(), "effort", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("effort help = code %d, stderr %q", code, stderr)
	}
	for _, command := range []string{"list", "show <slug>", "finish <slug>"} {
		if !strings.Contains(stdout, command) {
			t.Errorf("effort help missing %q", command)
		}
	}
	for _, retired := range []string{"new <slug>", "integrate", "worktree"} {
		if strings.Contains(stdout, retired) {
			t.Errorf("effort help contains retired command %q", retired)
		}
	}
}

func TestDocsAvailableWithoutProjectStateOrMutation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	invalidSource := filepath.Join(root, ".awf", "project.md")
	if err := os.MkdirAll(filepath.Dir(invalidSource), 0o755); err != nil {
		t.Fatal(err)
	}
	before := []byte("not valid frontmatter\n")
	if err := os.WriteFile(invalidSource, before, 0o644); err != nil {
		t.Fatal(err)
	}

	pages := []struct {
		args  []string
		title string
	}{
		{args: []string{"docs"}, title: "# AWF guide\n"},
		{args: []string{"docs", "integration"}, title: "# Integrating AWF\n"},
		{args: []string{"docs", "topics"}, title: "# Working with topics\n"},
		{args: []string{"docs", "effort"}, title: "# Efforts and change documents\n"},
	}
	for _, page := range pages {
		code, stdout, stderr := runCLI(t, root, page.args...)
		if code != 0 || stderr != "" || !strings.HasPrefix(stdout, page.title) {
			t.Errorf("%v = code %d, stdout %q, stderr %q", page.args, code, stdout, stderr)
		}
	}

	after, err := os.ReadFile(invalidSource)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("docs changed source: %q, %v", after, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != ".awf" {
		t.Fatalf("docs changed repository entries: %v, %v", entries, err)
	}
	sourceEntries, err := os.ReadDir(filepath.Dir(invalidSource))
	if err != nil || len(sourceEntries) != 1 || sourceEntries[0].Name() != "project.md" {
		t.Fatalf("docs changed .awf entries: %v, %v", sourceEntries, err)
	}
}

func TestDocsHelpAndUsage(t *testing.T) {
	for _, args := range [][]string{{"docs", "--help"}, {"help", "docs"}} {
		code, stdout, stderr := runCLI(t, t.TempDir(), args...)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "awf docs [integration|topics|effort]") {
			t.Errorf("%v = code %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	for _, args := range [][]string{{"docs", "unknown"}, {"docs", "topics", "extra"}} {
		code, stdout, stderr := runCLI(t, t.TempDir(), args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "awf:") {
			t.Errorf("%v = code %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestInitRenderCheckRoundTripWithoutGit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())

	code, stdout, stderr := runCLI(t, root, "init")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "rendered: AGENTS.md") {
		t.Fatalf("init = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("init created Git state: %v", err)
	}

	code, stdout, stderr = runCLI(t, root, "check")
	if code != 0 || stdout != "check: ok\n" || stderr != "" {
		t.Fatalf("check = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "render")
	if code != 0 || stdout != "render: up to date\n" || stderr != "" {
		t.Fatalf("repeat render = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRenderReportsAndCheckFailsUnmanagedMarker(t *testing.T) {
	root := t.TempDir()
	if code, _, stderr := runCLI(t, root, "init"); code != 0 {
		t.Fatal(stderr)
	}
	oldPath := filepath.Join(root, "old.md")
	if err := os.WriteFile(oldPath, []byte("<!-- GENERATED by awf: do not edit; change .awf/ and run `awf render` -->\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runCLI(t, root, "render")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "unmanaged AWF-marked file: old.md") {
		t.Fatalf("render = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "check")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "old.md: unmanaged file still carries an AWF ownership marker") {
		t.Fatalf("check = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	if err := os.WriteFile(oldPath, []byte("ordinary file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runCLI(t, root, "check")
	if code != 0 || stdout != "check: ok\n" || stderr != "" {
		t.Fatalf("check after ownership removal = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestResolveCreationAndEffortLifecycle(t *testing.T) {
	root := t.TempDir()
	if code, _, stderr := runCLI(t, root, "init"); code != 0 {
		t.Fatal(stderr)
	}
	code, stdout, stderr := runCLI(t, root, "resolve")
	if code != 0 || stdout != "none\n" || stderr != "" {
		t.Fatalf("resolve without globals = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	topicPath := filepath.Join(root, "docs", "topics", "code", "render.md")
	if err := os.MkdirAll(filepath.Dir(topicPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(topicPath, []byte("---\npaths: [internal/projector/**]\n---\n# Render\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(root, "docs", "topics", "global.md")
	if err := os.WriteFile(globalPath, []byte("---\npaths: ['**']\n---\n# Global\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr = runCLI(t, root, "resolve")
	if code != 0 || stdout != "global\tdocs/topics/global.md\n" || stderr != "" {
		t.Fatalf("resolve globals = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "resolve", "internal/projector/new.go")
	if code != 0 || stdout != "code/render\tdocs/topics/code/render.md\nglobal\tdocs/topics/global.md\n" || stderr != "" {
		t.Fatalf("resolve = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "resolve", "--coverage", `internal\\projector\\new.go`, "README.md", "internal/projector/new.go")
	wantCoverage := "globals:\n  global\tdocs/topics/global.md\npath: \"README.md\"\n  none\npath: \"internal/projector/new.go\"\n  code/render\tdocs/topics/code/render.md\n"
	if code != 0 || stdout != wantCoverage || stderr != "" {
		t.Fatalf("resolve coverage = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "resolve", "--coverage", "line\nbreak")
	wantCoverage = "globals:\n  global\tdocs/topics/global.md\npath: \"line\\nbreak\"\n  none\n"
	if code != 0 || stdout != wantCoverage || stderr != "" {
		t.Fatalf("escaped coverage = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "resolve", "--coverage")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "usage:") {
		t.Fatalf("empty coverage = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if err := os.Remove(globalPath); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runCLI(t, root, "resolve", "README.md")
	if code != 0 || stdout != "none\n" || stderr != "" {
		t.Fatalf("resolve none = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	code, stdout, stderr = runCLI(t, root, "new", "effort", "simple")
	if code != 0 || stdout != "memory: .awf/efforts/simple/memory.md\n" || stderr != "" {
		t.Fatalf("new effort = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	for _, kind := range []string{"plan", "intent", "spec"} {
		code, stdout, stderr = runCLI(t, root, "new", kind, "simple")
		want := kind + ": docs/changes/simple/" + kind + ".md\n"
		if code != 0 || stdout != want || stderr != "" {
			t.Fatalf("new %s = code %d, stdout %q, stderr %q", kind, code, stdout, stderr)
		}
	}
	code, stdout, stderr = runCLI(t, root, "new", "adr", "simple-choice")
	if code != 0 || stdout != "adr: docs/decisions/simple-choice.md\n" || stderr != "" {
		t.Fatalf("new adr = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "new", "topic", "nested/generated", "generated/**")
	if code != 0 || stdout != "topic: docs/topics/nested/generated.md\n" || stderr != "" {
		t.Fatalf("new topic = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "resolve", "generated/future.txt")
	if code != 0 || stdout != "nested/generated\tdocs/topics/nested/generated.md\n" || stderr != "" {
		t.Fatalf("resolve created topic = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	legacyPlan := filepath.Join(root, ".awf", "efforts", "simple", "plan.md")
	if err := os.WriteFile(legacyPlan, []byte("legacy effort-local plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts := map[string][]byte{
		filepath.Join("docs", "changes", "simple", "intent.md"):   []byte("<!-- GENERATED by awf; remove this comment to take ownership -->\nauthored intent\n"),
		filepath.Join("docs", "changes", "simple", "spec.md"):     []byte("edited specification\n"),
		filepath.Join("docs", "changes", "simple", "plan.md"):     []byte("edited plan\n"),
		filepath.Join("docs", "plans", "simple.md"):               []byte("legacy tracked plan\n"),
		filepath.Join("docs", "decisions", "simple-choice.md"):    []byte("edited adr\n"),
		filepath.Join("docs", "topics", "nested", "generated.md"): []byte("---\npaths: ['generated/**']\n---\n# Edited topic\n"),
		filepath.Join("docs", "ordinary.md"):                      []byte("ordinary documentation, not a topic\n"),
	}
	for relative, body := range artifacts {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, relative), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, _, stderr = runCLI(t, root, "render")
	if code != 0 || stderr != "" {
		t.Fatalf("render with artifacts = code %d, stderr %q", code, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "check")
	if code != 0 || stdout != "check: ok\n" || stderr != "" {
		t.Fatalf("check with artifacts = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "effort", "list")
	if code != 0 || stdout != "simple\n" || stderr != "" {
		t.Fatalf("effort list = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "effort", "show", "simple")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "# Effort: simple") {
		t.Fatalf("effort show = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, root, "effort", "finish", "simple")
	if code != 0 || stdout != "archive: .awf/effort-archive/simple\n" || stderr != "" {
		t.Fatalf("effort finish = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	archivedLegacyPlan, err := os.ReadFile(filepath.Join(root, ".awf", "effort-archive", "simple", "plan.md"))
	if err != nil || string(archivedLegacyPlan) != "legacy effort-local plan\n" {
		t.Fatalf("archived legacy plan = %q, %v", archivedLegacyPlan, err)
	}
	for relative, want := range artifacts {
		got, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("artifact %s after render/check/finish = %q, %v", relative, got, err)
		}
	}
}

func TestChangeDocumentCommandsRejectWrongArgumentCounts(t *testing.T) {
	for _, kind := range []string{"intent", "spec", "plan"} {
		for _, args := range [][]string{{"new", kind}, {"new", kind, "slug", "extra"}} {
			root := t.TempDir()
			code, stdout, stderr := runCLI(t, root, args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "usage: awf new "+kind+" <slug>") {
				t.Fatalf("%v = code %d, stdout %q, stderr %q", args, code, stdout, stderr)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid command mutated checkout: %v, %v", entries, err)
			}
		}
	}
}

func TestRetiredCreationRoutesAreNotAliases(t *testing.T) {
	for _, args := range [][]string{{"effort", "new", "old"}, {"plan", "new", "old"}, {"adr", "new", "old"}} {
		code, stdout, stderr := runCLI(t, t.TempDir(), args...)
		if code != 2 || stdout != "" || stderr == "" {
			t.Errorf("retired route %v = code %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestUsageAndOperationalFailuresUseStderr(t *testing.T) {
	code, stdout, stderr := runCLI(t, t.TempDir(), "unknown")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("unknown = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, t.TempDir(), "check")
	if code != 1 || stdout != "" || !strings.Contains(stderr, ".awf/project.md") {
		t.Fatalf("check without source = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func runCLI(t *testing.T, root string, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args := append([]string{"awf"}, arguments...)
	code := run(root, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}
