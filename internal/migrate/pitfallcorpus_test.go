package migrate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func writeLegacyPitfalls(t *testing.T, root, body string) string {
	t.Helper()
	path := filepath.Join(config.RootDir(root), "docs", "pitfalls.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// invariant: config/migrations-and-locks:pitfall-corpus-migration (TestPitfallCorpusMigrationPreflightRetryAndSections)
func TestPitfallCorpusMigrationPreflightRetryAndSections(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "sections:\n  prepend:\n    drop: true\ndata:\n  pitfalls:\n    - title: ' First! '\n      domains: [rendering]\n      tags: [proof]\n      related: [1]\n      body: |\n        first body\n    - title: First\n      body: second body\n")
	var changes Changes
	if err := applyPitfallCorpus(root, &changes); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, ".awf/docs/pitfalls/first.md")
	second := filepath.Join(root, ".awf/docs/pitfalls/first-2.md")
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"title: First!", "domains: [rendering]", "tags: [proof]", "related: [1]", "first body"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("leaf missing %q:\n%s", want, b)
		}
	}
	remaining, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(remaining), "pitfalls") || !strings.Contains(string(remaining), "sections:") {
		t.Fatalf("remainder:\n%s", remaining)
	}
	var retry Changes
	if err := applyPitfallCorpus(root, &retry); err != nil || len(retry.Items()) != 0 {
		t.Fatalf("retry = %v, %#v", err, retry.Items())
	}
}

func TestPitfallCorpusMigrationPreflightsLinksConflictsAndDuplicates(t *testing.T) {
	for _, tc := range []struct{ name, entries, prepare, want string }{
		{"relative-link", "    - title: A\n      body: '[x](relative.md)'\n", "", "relative link"},
		{"duplicate-title", "    - title: A\n      body: one\n    - title: ' a '\n      body: two\n", "", "duplicates"},
		{"conflict", "    - title: A\n      body: one\n", "conflict", "conflicts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n"+tc.entries)
			if tc.prepare != "" {
				p := filepath.Join(root, ".awf/docs/pitfalls/a.md")
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(tc.prepare), 0644); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(sidecar)
			if err := applyPitfallCorpus(root, &Changes{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v", err)
			}
			after, _ := os.ReadFile(sidecar)
			if string(after) != string(before) {
				t.Fatal("preflight changed old authority")
			}
		})
	}
}

func TestProductionPitfallCorpusOperation(t *testing.T) {
	op := productionPitfallCorpusOperation()
	root := t.TempDir()
	leaf := filepath.Join(root, "nested", "leaf.md")
	if err := op.create(leaf, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := op.create(leaf, []byte("x")); err == nil {
		t.Fatal("exclusive create replaced file")
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := op.create(filepath.Join(blocked, "leaf"), []byte("x")); err == nil {
		t.Fatal("create under file succeeded")
	}
	if err := op.writeSidecar(filepath.Join(root, "sidecar"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := op.writeSidecar(root, []byte("x")); err == nil {
		t.Fatal("wrote directory")
	}
	if err := op.removeSidecar(filepath.Join(root, "sidecar")); err != nil {
		t.Fatal(err)
	}
	if err := op.removeSidecar(filepath.Join(root, "missing")); err == nil {
		t.Fatal("removed missing")
	}
	if empty, err := pitfallSidecarRemainderEmpty([]byte("[")); err == nil || empty {
		t.Fatal("malformed remainder accepted")
	}
	if empty, err := pitfallSidecarRemainderEmpty([]byte("sections: {}\n")); err != nil || empty {
		t.Fatalf("nonempty remainder = %v, %v", empty, err)
	}
}

func TestPitfallCorpusMigrationRefusalBranches(t *testing.T) {
	for _, tc := range []struct{ name, content, want string }{
		{"malformed", "data: [\n", "parse"},
		{"empty-title", "data:\n  pitfalls:\n    - title: '  '\n      body: x\n", "ASCII slug"},
		{"empty-body", "data:\n  pitfalls:\n    - title: A\n      body: ' '\n", "body is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLegacyPitfalls(t, root, tc.content)
			if err := applyPitfallCorpus(root, &Changes{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	root := t.TempDir()
	side := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: x\n")
	if err := os.Remove(side); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(side, 0755); err != nil {
		t.Fatal(err)
	}
	if err := applyPitfallCorpus(root, &Changes{}); err == nil {
		t.Fatal("directory sidecar accepted")
	}
}

func TestPitfallCorpusMigrationInterruptionKeepsAuthorityAndRetries(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: One\n      body: one\n    - title: Two\n      body: two\n")
	prod := productionPitfallCorpusOperation()
	creates := 0
	op := prod
	op.create = func(path string, data []byte) error {
		creates++
		if creates == 2 {
			return errors.New("injected create failure")
		}
		return prod.create(path, data)
	}
	if err := applyPitfallCorpusWith(root, &Changes{}, op); err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatal("old authority retired before all leaves existed")
	}
	if err := applyPitfallCorpus(root, &Changes{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/two.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("empty sidecar survived: %v", err)
	}
}
