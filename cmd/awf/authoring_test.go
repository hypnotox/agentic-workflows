package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func runAuthoringAt(root string, stdin *strings.Reader, args []string, stdout, stderr *bytes.Buffer) int {
	return newRunner(func() (string, error) { return root, nil }, stdin, func() bool { return false }).run(args, stdout, stderr)
}

func readAuthoringFile(t *testing.T, root, relative string) []byte {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestPartAuthoringCLI pins the edit and reset families' real operation result,
// including runner-owned stdin, explicit empty content, and inherited reset.
func TestPartAuthoringCLI(t *testing.T) {
	root := scaffoldProject(t)
	part := catalog.Standard.Skills["tdd"].Sections[0]
	partPath := filepath.ToSlash(filepath.Join(".awf/skills/parts/tdd", part+".md"))
	outputPath := ".claude/skills/example-tdd/SKILL.md"
	lockBefore := readAuthoringFile(t, root, ".awf/awf.lock")

	var stdout, stderr bytes.Buffer
	if code := runAuthoringAt(root, strings.NewReader("stdin body\n"), []string{"awf", "edit", "skill", "tdd", part, "--stdin"}, &stdout, &stderr); code != 0 {
		t.Fatalf("stdin edit exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := string(readAuthoringFile(t, root, partPath)); got != "stdin body\n" {
		t.Fatalf("stdin source = %q", got)
	}
	if got := string(readAuthoringFile(t, root, outputPath)); !strings.Contains(got, "stdin body") {
		t.Fatalf("rendered output omitted authored body: %q", got)
	}
	if bytes.Equal(lockBefore, readAuthoringFile(t, root, ".awf/awf.lock")) || !strings.Contains(stdout.String(), "status: artifact part authored") {
		t.Fatalf("successful edit did not report and synchronize: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runAuthoringAt(root, strings.NewReader("unused"), []string{"awf", "edit", "skill", "tdd", part, "--content", ""}, &stdout, &stderr); code != 0 {
		t.Fatalf("empty edit exit=%d stderr=%q", code, stderr.String())
	}
	if got := readAuthoringFile(t, root, partPath); len(got) != 0 {
		t.Fatalf("explicit empty override = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runAuthoringAt(root, strings.NewReader("unused"), []string{"awf", "reset", "skill", "tdd", part}, &stdout, &stderr); code != 0 {
		t.Fatalf("reset exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(partPath))); !os.IsNotExist(err) {
		t.Fatalf("reset retained override: %v", err)
	}
	if got := string(readAuthoringFile(t, root, outputPath)); strings.Contains(got, "stdin body") {
		t.Fatalf("reset retained authored output: %q", got)
	}
}

func TestPartAuthoringCLIRejectsInvalidModesAndTargetsWithoutMutation(t *testing.T) {
	root := scaffoldProject(t)
	part := catalog.Standard.Skills["tdd"].Sections[0]
	outputPath := ".claude/skills/example-tdd/SKILL.md"
	beforeOutput := readAuthoringFile(t, root, outputPath)
	beforeLock := readAuthoringFile(t, root, ".awf/awf.lock")
	partPath := filepath.Join(root, ".awf", "skills", "parts", "tdd", part+".md")

	cases := [][]string{
		{"awf", "edit", "skill", "tdd", part},
		{"awf", "edit", "skill", "tdd", part, "--content", "x", "--stdin"},
		{"awf", "edit", "skill", "tdd"},
		{"awf", "reset", "skill", "tdd", part, "--content", "x"},
		{"awf", "edit", "skill", "absent", part, "--content", "x"},
		{"awf", "edit", "skill", "tdd", part, "--content", "{{=awf:notDeclared}}"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := runAuthoringAt(root, strings.NewReader("stdin"), args, &stdout, &stderr); code == 0 {
			t.Errorf("%v unexpectedly succeeded: %q", args, stdout.String())
		}
		if _, err := os.Stat(partPath); !os.IsNotExist(err) {
			t.Fatalf("%v changed source: %v", args, err)
		}
		if !bytes.Equal(beforeOutput, readAuthoringFile(t, root, outputPath)) || !bytes.Equal(beforeLock, readAuthoringFile(t, root, ".awf/awf.lock")) {
			t.Fatalf("%v changed output or lock", args)
		}
	}
}

func TestLocalDocumentBodyCLIEditAndReset(t *testing.T) {
	root := syncedGitProject(t, minimalYAML+"localDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	path := "docs/runbooks/incident.md"
	var stdout, stderr bytes.Buffer
	if code := runAuthoringAt(root, strings.NewReader("unused"), []string{"awf", "edit", "doc", "runbooks/incident", "body", "--content", "operator body"}, &stdout, &stderr); code != 0 {
		t.Fatalf("local edit exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	edited := string(readAuthoringFile(t, root, path))
	if !strings.Contains(edited, "operator body") || !strings.Contains(edited, "# Incident") {
		t.Fatalf("local edit changed shell or omitted body: %q", edited)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runAuthoringAt(root, strings.NewReader("unused"), []string{"awf", "reset", "doc", "runbooks/incident", "body"}, &stdout, &stderr); code != 0 {
		t.Fatalf("local reset exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	reset := string(readAuthoringFile(t, root, path))
	if strings.Contains(reset, "operator body") || !strings.Contains(reset, "# Incident") || !strings.Contains(reset, "awf:edit-in-place body") {
		t.Fatalf("local reset changed shell or retained body: %q", reset)
	}
}
