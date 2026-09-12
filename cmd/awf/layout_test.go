package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceFormatCutoverRequiresManualMigration(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, ".awf", "project.md")
	legacyTopics := filepath.Join(root, ".awf", "topics")
	legacyTopic := filepath.Join(legacyTopics, "code", "go.md")
	if err := os.MkdirAll(filepath.Dir(legacyTopic), 0o755); err != nil {
		t.Fatal(err)
	}
	project := []byte("---\nformat: 1\n---\n# Existing project\n")
	topic := []byte("---\npaths: ['src/**/*.go']\n---\n# Existing topic\n")
	for path, body := range map[string][]byte{projectPath: project, legacyTopic: topic} {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, args := range [][]string{{"resolve"}, {"resolve", "src/new.go"}, {"resolve", "--coverage", "src/new.go"}, {"render"}, {"check"}} {
		code, stdout, stderr := runCLI(t, root, args...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "unsupported AWF source format 1") || !strings.Contains(stderr, "accepts format 2") {
			t.Errorf("unmigrated %v = code %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	for path, want := range map[string][]byte{projectPath: project, legacyTopic: topic} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("unmigrated source changed: %s = %q, %v", path, got, err)
		}
	}
	for _, path := range []string{"AGENTS.md", "docs"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("unmigrated commands created %s: %v", path, err)
		}
	}

	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(legacyTopics, filepath.Join(root, "docs", "topics")); err != nil {
		t.Fatal(err)
	}
	project = bytes.Replace(project, []byte("format: 1"), []byte("format: 2"), 1)
	if err := os.WriteFile(projectPath, project, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, root, "resolve", "src/new.go")
	if code != 0 || stdout != "code/go\tdocs/topics/code/go.md\n" || stderr != "" {
		t.Fatalf("migrated resolve = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	for _, command := range []string{"render", "check"} {
		if code, _, stderr := runCLI(t, root, command); code != 0 || stderr != "" {
			t.Fatalf("migrated %s = code %d, stderr %q", command, code, stderr)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "topics", "code", "go.md"))
	if err != nil || !bytes.Equal(got, topic) {
		t.Fatalf("migrated topic changed: %q, %v", got, err)
	}
}

func TestInitUsesSourceFormatTwo(t *testing.T) {
	root := t.TempDir()
	if code, _, stderr := runCLI(t, root, "init"); code != 0 || stderr != "" {
		t.Fatalf("init = code %d, stderr %q", code, stderr)
	}
	project, err := os.ReadFile(filepath.Join(root, ".awf", "project.md"))
	if err != nil || !bytes.HasPrefix(project, []byte("---\nformat: 2\n---\n")) {
		t.Fatalf("initialized source = %q, %v", project, err)
	}
}
