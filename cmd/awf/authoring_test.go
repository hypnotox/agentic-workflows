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
// invariant: tooling/cli:semantic-artifact-authoring (TestPartAuthoringCLI)
func TestPartAuthoringCLI(t *testing.T) {
	root := scaffoldProject(t)
	part := catalog.Standard.Skills["brainstorming"].Sections[0]
	partPath := filepath.ToSlash(filepath.Join(".awf/skills/parts/brainstorming", part+".md"))
	outputPath := ".claude/skills/example-brainstorming/SKILL.md"
	lockBefore := readAuthoringFile(t, root, ".awf/awf.lock")

	var stdout, stderr bytes.Buffer
	if code := runAuthoringAt(root, strings.NewReader("stdin body\n"), []string{"awf", "edit", "skill", "brainstorming", part, "--stdin"}, &stdout, &stderr); code != 0 {
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
	if code := runAuthoringAt(root, strings.NewReader("unused"), []string{"awf", "edit", "skill", "brainstorming", part, "--content", ""}, &stdout, &stderr); code != 0 {
		t.Fatalf("empty edit exit=%d stderr=%q", code, stderr.String())
	}
	if got := readAuthoringFile(t, root, partPath); len(got) != 0 {
		t.Fatalf("explicit empty override = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runAuthoringAt(root, strings.NewReader("unused"), []string{"awf", "reset", "skill", "brainstorming", part}, &stdout, &stderr); code != 0 {
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
	part := catalog.Standard.Skills["brainstorming"].Sections[0]
	outputPath := ".claude/skills/example-brainstorming/SKILL.md"
	beforeOutput := readAuthoringFile(t, root, outputPath)
	beforeLock := readAuthoringFile(t, root, ".awf/awf.lock")
	partPath := filepath.Join(root, ".awf", "skills", "parts", "brainstorming", part+".md")

	cases := [][]string{
		{"awf", "edit", "skill", "brainstorming", part},
		{"awf", "edit", "skill", "brainstorming", part, "--content", "x", "--stdin"},
		{"awf", "edit", "skill", "brainstorming"},
		{"awf", "reset", "skill", "brainstorming", part, "--content", "x"},
		{"awf", "edit", "skill", "absent", part, "--content", "x"},
		{"awf", "edit", "skill", "brainstorming", part, "--content", "{{=awf:notDeclared}}"},
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

// invariant: tooling/cli:semantic-artifact-authoring (TestSidecarAuthoringCLIUsesTypedIdempotentModes)
func TestSidecarAuthoringCLIUsesTypedIdempotentModes(t *testing.T) {
	root := scaffoldProject(t)
	sourcePath := ".awf/skills/brainstorming.yaml"
	run := func(args ...string) (string, string, int) {
		var stdout, stderr bytes.Buffer
		code := runAuthoringAt(root, strings.NewReader("unused"), append([]string{"awf"}, args...), &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	stdout, stderr, code := run("edit", "sidecar", "skill", "brainstorming", "data.members", "--add-json", `{"name":"api","enabled":true}`)
	if code != 0 {
		t.Fatalf("structured add exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	first := readAuthoringFile(t, root, sourcePath)
	if !strings.Contains(string(first), "name: api") || !strings.Contains(string(first), "enabled: true") {
		t.Fatalf("structured sidecar = %q", first)
	}
	stdout, stderr, code = run("edit", "sidecar", "skill", "brainstorming", "data.members", "--add-json", `{"enabled":true,"name":"api"}`)
	if code != 0 || !bytes.Equal(first, readAuthoringFile(t, root, sourcePath)) || !strings.Contains(stdout, "source effect: none") {
		t.Fatalf("idempotent add exit=%d stdout=%q stderr=%q source=%q", code, stdout, stderr, readAuthoringFile(t, root, sourcePath))
	}
	stdout, stderr, code = run("edit", "sidecar", "skill", "brainstorming", "data.members", "--remove-json", `{"name":"absent"}`)
	if code != 0 || !bytes.Equal(first, readAuthoringFile(t, root, sourcePath)) {
		t.Fatalf("absent remove exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = run("reset", "sidecar", "skill", "brainstorming", "data.members")
	if code != 0 {
		t.Fatalf("final reset exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(sourcePath))); !os.IsNotExist(err) {
		t.Fatalf("final reset retained sidecar: %v", err)
	}
}

// invariant: tooling/cli:semantic-artifact-authoring (TestSidecarAuthoringCLISupportsKnownBooleanAndDomainCapabilities)
func TestSidecarAuthoringCLISupportsKnownBooleanAndDomainCapabilities(t *testing.T) {
	root := syncedGitProject(t, minimalYAML+"domains: [tooling]\n")
	run := func(args ...string) {
		var stdout, stderr bytes.Buffer
		if code := runAuthoringAt(root, strings.NewReader("unused"), append([]string{"awf"}, args...), &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	section := catalog.Standard.Skills["brainstorming"].Sections[0]
	run("edit", "sidecar", "skill", "brainstorming", "sections."+section+".drop", "--json-value", "true")
	skillSidecar := string(readAuthoringFile(t, root, ".awf/skills/brainstorming.yaml"))
	if !strings.Contains(skillSidecar, "drop: true") {
		t.Fatalf("typed boolean controls = %q", skillSidecar)
	}
	run("edit", "sidecar", "domain", "tooling", "paths", "--add", "internal/**")
	domainSidecar := string(readAuthoringFile(t, root, ".awf/domains/tooling.yaml"))
	if !strings.Contains(domainSidecar, "paths:") || !strings.Contains(domainSidecar, "- internal/**") {
		t.Fatalf("domain paths = %q", domainSidecar)
	}
}

// invariant: tooling/cli:semantic-artifact-authoring (TestSidecarAuthoringCLIRejectsModesCapabilitiesAndInvalidCandidateWithoutMutation)
func TestSidecarAuthoringCLIRejectsModesCapabilitiesAndInvalidCandidateWithoutMutation(t *testing.T) {
	root := scaffoldProject(t)
	outputPath := ".claude/skills/example-brainstorming/SKILL.md"
	beforeOutput := readAuthoringFile(t, root, outputPath)
	beforeLock := readAuthoringFile(t, root, ".awf/awf.lock")
	section := catalog.Standard.Skills["brainstorming"].Sections[0]
	cases := [][]string{
		{"awf", "edit", "sidecar", "skill", "brainstorming", "data.key"},
		{"awf", "edit", "sidecar", "skill", "brainstorming", "data.key", "--value", "x", "--json-value", `"x"`},
		{"awf", "edit", "sidecar", "skill", "brainstorming", "data.key", "--json-value", "1 2"},
		{"awf", "edit", "sidecar", "skill", "brainstorming", "data", "--value", "x"},
		{"awf", "edit", "sidecar", "skill", "brainstorming", "sections.absent.drop", "--json-value", "true"},
		{"awf", "edit", "sidecar", "domain", "absent", "paths", "--add", "internal/**"},
		{"awf", "edit", "sidecar", "skill", "brainstorming", "dataDefaults.invalid", "--value", "not-a-boolean"},
		{"awf", "edit", "sidecar", "skill", "brainstorming", "sections." + section + ".drop", "--value", "true"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := runAuthoringAt(root, strings.NewReader("unused"), args, &stdout, &stderr); code == 0 {
			t.Errorf("%v unexpectedly succeeded: %q", args, stdout.String())
		}
		if _, err := os.Stat(filepath.Join(root, ".awf/skills/brainstorming.yaml")); !os.IsNotExist(err) {
			t.Fatalf("%v changed source: %v", args, err)
		}
		if !bytes.Equal(beforeOutput, readAuthoringFile(t, root, outputPath)) || !bytes.Equal(beforeLock, readAuthoringFile(t, root, ".awf/awf.lock")) {
			t.Fatalf("%v changed output or lock", args)
		}
	}
}

// invariant: tooling/cli:semantic-artifact-authoring (TestPartAuthoringCLIRendersPartialReportOnce)
func TestPartAuthoringCLIRendersPartialReportOnce(t *testing.T) {
	root := scaffoldProject(t)
	part := catalog.Standard.Skills["brainstorming"].Sections[0]
	output := filepath.Join(root, ".claude", "skills", "example-brainstorming", "SKILL.md")
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runAuthoringAt(root, strings.NewReader("unused"), []string{"awf", "edit", "skill", "brainstorming", part, "--content", "committed body"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("partial authoring exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: artifact part partially committed") || !strings.Contains(stdout.String(), "source effect: created") {
		t.Fatalf("partial report = %q", stdout.String())
	}
	// handlerReport recognizes the producedReportError returned after the
	// successful report render, so the driver preserves its failing exit without
	// emitting a second diagnostic.
	if stderr.Len() != 0 {
		t.Fatalf("produced partial report was also diagnosed: %q", stderr.String())
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
