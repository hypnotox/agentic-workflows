package migrate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"gopkg.in/yaml.v3"
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

func TestPitfallCorpusMigrationPreflightRetryAndSections(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "sections:\n  prepend:\n    drop: true\ndata:\n  pitfalls:\n    - title: ' First! '\n      domains: [' rendering ', config]\n      tags: [' proof ', workflow]\n      related: [2, 1]\n      body: |\n        first body\n        second line\n    - title: First\n      domains: null\n      tags: null\n      related: null\n      body: second body\n")
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
	parsed, err := pitfall.Parse(pitfall.SourceFile{Path: pitfall.SourceDir + "/first.md", Bytes: b, Regular: true})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Title != "First!" || strings.Join(parsed.Domains, ",") != "rendering,config" || strings.Join(parsed.Tags, ",") != "proof,workflow" || len(parsed.Related) != 2 || parsed.Related[0] != 2 || parsed.Related[1] != 1 || parsed.Body != "first body\nsecond line\n" {
		t.Fatalf("canonical legacy preservation = %#v", parsed)
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

func TestPitfallCorpusMigrationRejectsNonRegularDestinations(t *testing.T) {
	for _, kind := range []string{"symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n    - title: B\n      body: other\n")
			before, err := os.ReadFile(sidecar)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(root, ".awf/docs/pitfalls/a.md")
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				canonical, err := pitfall.Serialize(pitfall.Entry{Title: "A", Body: "body"})
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "identical.md")
				if err := os.WriteFile(target, canonical, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, destination); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "directory":
				if err := os.Mkdir(destination, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := applyPitfallCorpus(root, &Changes{}); err == nil || !strings.Contains(err.Error(), "not a direct regular file") {
				t.Fatalf("non-regular destination error = %v", err)
			}
			after, err := os.ReadFile(sidecar)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("legacy authority changed: %q, %v", after, err)
			}
			if _, err := os.Lstat(destination); err != nil {
				t.Fatalf("destination changed: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, ".awf/docs/pitfalls/b.md")); !os.IsNotExist(err) {
				t.Fatalf("later destination mutated: %v", err)
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
	if err := op.writeSidecar(filepath.Join(root, "sidecar"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "sidecar")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("sidecar mode = %v, %v", info, err)
	}
	if err := op.writeSidecar(root, []byte("x"), 0o644); err == nil {
		t.Fatal("wrote directory")
	}
	if err := op.removeSidecar(filepath.Join(root, "sidecar")); err != nil {
		t.Fatal(err)
	}
	if err := op.removeSidecar(filepath.Join(root, "missing")); err == nil {
		t.Fatal("removed missing")
	}
	if empty, err := preflightPitfallSidecarRemainder([]byte("[")); err == nil || empty {
		t.Fatal("malformed remainder accepted")
	}
	if empty, err := preflightPitfallSidecarRemainder([]byte("sections: {}\n")); err != nil || !empty {
		t.Fatalf("empty map remainder = %v, %v", empty, err)
	}
	for _, tc := range []struct {
		name      string
		raw       string
		wantEmpty bool
		wantErr   bool
	}{
		{"multiple-top-level", "sections: {}\nother: x\n", false, true},
		{"sections-wrong-kind", "sections: []\n", false, true},
		{"null-sections", "sections: null\n", true, false},
		{"unsupported-section", "sections:\n  other: {}\n", false, true},
		{"null-override", "sections:\n  prepend: null\n", true, false},
		{"empty-override", "sections:\n  append: {}\n", true, false},
		{"ineffective-override", "sections:\n  prepend:\n    drop: false\n", true, false},
		{"effective-override", "sections:\n  append:\n    drop: true\n", false, false},
		{"override-wrong-kind", "sections:\n  prepend: true\n", false, true},
		{"unsupported-override-field", "sections:\n  prepend:\n    custom: true\n", false, true},
		{"invalid-drop", "sections:\n  prepend:\n    drop: nope\n", false, true},
		{"duplicate-section", "sections:\n  prepend: {}\n  prepend: {}\n", false, true},
		{"duplicate-drop", "sections:\n  prepend:\n    drop: true\n    drop: false\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			empty, err := preflightPitfallSidecarRemainder([]byte(tc.raw))
			if (err != nil) != tc.wantErr || (!tc.wantErr && empty != tc.wantEmpty) {
				t.Fatalf("preflight remainder = %v, %v; wantEmpty=%t wantErr=%t", empty, err, tc.wantEmpty, tc.wantErr)
			}
		})
	}
	var scalar yaml.Node
	if err := yaml.Unmarshal([]byte("value\n"), &scalar); err != nil {
		t.Fatal(err)
	}
	if mappingPathPresent(&scalar, "data") {
		t.Fatal("scalar document reported a mapping path")
	}
}

func TestPitfallCorpusMigrationRefusalBranches(t *testing.T) {
	for _, tc := range []struct{ name, content, want string }{
		{"malformed", "data: [\n", "parse"},
		{"registry-wrong-type", "data:\n  pitfalls: value\n", "parse"},
		{"entry-wrong-type", "data:\n  pitfalls: [value]\n", "must be a mapping"},
		{"duplicate-entry-key", "data:\n  pitfalls:\n    - title: A\n      title: B\n      body: x\n", "duplicate legacy"},
		{"numeric-title", "data:\n  pitfalls:\n    - title: 1\n      body: x\n", "must be a string"},
		{"empty-title", "data:\n  pitfalls:\n    - title: '  '\n      body: x\n", "ASCII slug"},
		{"empty-body", "data:\n  pitfalls:\n    - title: A\n      body: ' '\n", "body is empty"},
		{"domains-wrong-kind", "data:\n  pitfalls:\n    - title: A\n      domains: value\n      body: x\n", "must be a list"},
		{"numeric-domain", "data:\n  pitfalls:\n    - title: A\n      domains: [1]\n      body: x\n", "non-empty strings"},
		{"bool-tag", "data:\n  pitfalls:\n    - title: A\n      tags: [true]\n      body: x\n", "non-empty strings"},
		{"related-wrong-kind", "data:\n  pitfalls:\n    - title: A\n      related: 1\n      body: x\n", "must be a list"},
		{"string-related", "data:\n  pitfalls:\n    - title: A\n      related: ['1']\n      body: x\n", "ADR numbers"},
		{"overflow-related", "data:\n  pitfalls:\n    - title: A\n      related: [999999999999999999999999999999999]\n      body: x\n", "ADR numbers"},
		{"invalid-tagged-integer", "data:\n  pitfalls:\n    - title: A\n      related: [!!int nope]\n      body: x\n", "ADR numbers"},
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

func TestPitfallCorpusMigrationDropsSemanticallyEmptyRemainders(t *testing.T) {
	for _, tc := range []struct {
		name, sections string
		retained       bool
	}{
		{"null-sections", "sections: null\n", false},
		{"empty-sections", "sections: {}\n", false},
		{"null-override", "sections:\n  prepend: null\n", false},
		{"empty-override", "sections:\n  append: {}\n", false},
		{"effective-override", "sections:\n  prepend:\n    drop: true\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			sidecar := writeLegacyPitfalls(t, root, tc.sections+"data:\n  pitfalls: null\n")
			if err := applyPitfallCorpus(root, &Changes{}); err != nil {
				t.Fatal(err)
			}
			remaining, err := os.ReadFile(sidecar)
			if tc.retained {
				if err != nil || !strings.Contains(string(remaining), "drop: true") || strings.Contains(string(remaining), "pitfalls") {
					t.Fatalf("effective remainder = %q, %v", remaining, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("semantically empty remainder survived: %q, %v", remaining, err)
			}
		})
	}
}

func TestPitfallCorpusMigrationRetiresPresentEmptyAndNullRegistries(t *testing.T) {
	for _, registry := range []string{"[]", "null"} {
		t.Run(registry, func(t *testing.T) {
			root := t.TempDir()
			sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls: "+registry+"\n")
			if err := applyPitfallCorpus(root, &Changes{}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
				t.Fatalf("present retired registry survived: %v", err)
			}
		})
	}
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "sections:\n  append:\n    drop: true\ndata:\n  pitfalls: null\n")
	if err := applyPitfallCorpus(root, &Changes{}); err != nil {
		t.Fatal(err)
	}
	remaining, err := os.ReadFile(sidecar)
	if err != nil || !strings.Contains(string(remaining), "sections:") || strings.Contains(string(remaining), "pitfalls") {
		t.Fatalf("sections-only remainder = %q, %v", remaining, err)
	}
}

func TestPitfallCorpusMigrationRejectsUnknownRemainderBeforeMutation(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  extra: retained\n  pitfalls:\n    - title: A\n      body: body\n")
	before, _ := os.ReadFile(sidecar)
	creates := 0
	op := productionPitfallCorpusOperation()
	op.create = func(string, []byte) error { creates++; return nil }
	err := applyPitfallCorpusWith(root, &Changes{}, op)
	if err == nil || !strings.Contains(err.Error(), "only sections configuration may remain") {
		t.Fatalf("unknown remainder error = %v", err)
	}
	after, _ := os.ReadFile(sidecar)
	if creates != 0 || !bytes.Equal(before, after) {
		t.Fatalf("preflight ordering changed state: creates=%d before=%q after=%q", creates, before, after)
	}
}

func TestPitfallCorpusMigrationAtomicSidecarFailurePreservesBytes(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "sections:\n  prepend:\n    drop: true\ndata:\n  pitfalls:\n    - title: A\n      body: body\n")
	if err := os.Chmod(sidecar, 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(sidecar)
	op := productionPitfallCorpusOperation()
	op.writeSidecar = func(string, []byte, os.FileMode) error { return errors.New("injected atomic replacement failure") }
	if err := applyPitfallCorpusWith(root, &Changes{}, op); err == nil || !strings.Contains(err.Error(), "injected atomic") {
		t.Fatalf("replacement error = %v", err)
	}
	after, _ := os.ReadFile(sidecar)
	if !bytes.Equal(before, after) {
		t.Fatalf("failed replacement changed authority: before=%q after=%q", before, after)
	}
	if err := applyPitfallCorpus(root, &Changes{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sidecar)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("retry did not preserve mode: %v, %v", info, err)
	}
}

func TestPitfallCorpusMigrationChainsHistoricalGeneration9(t *testing.T) {
	root := t.TempDir()
	part := filepath.Join(root, ".awf/docs/parts/pitfalls/entries.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("## Historical pitfall\n\nhistorical body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyPitfallsData(root, &Changes{}); err != nil {
		t.Fatal(err)
	}
	if err := applyPitfallCorpus(root, &Changes{}); err != nil {
		t.Fatal(err)
	}
	leaf, err := os.ReadFile(filepath.Join(root, ".awf/docs/pitfalls/historical-pitfall.md"))
	if err != nil || !strings.Contains(string(leaf), "historical body") {
		t.Fatalf("generation-9 to generation-43 leaf = %q, %v", leaf, err)
	}
	positions := map[int]int{}
	for i, migration := range registry {
		if migration.To == 9 || migration.To == pitfallCorpusGeneration {
			positions[migration.To] = i
		}
	}
	if positions[9] >= positions[pitfallCorpusGeneration] || registry[positions[9]].Name != "pitfalls-data" || registry[positions[pitfallCorpusGeneration]].Name != "pitfall-corpus" {
		t.Fatalf("migration chain positions = %v", positions)
	}
}

func TestPitfallCorpusMigrationPublicationFailureLeavesRetryableState(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
	destination := filepath.Join(root, ".awf/docs/pitfalls/a.md")
	op := productionPitfallCorpusOperation()
	op.create = func(string, []byte) error { return errors.New("injected atomic exclusive publication failure") }
	if err := applyPitfallCorpusWith(root, &Changes{}, op); err == nil || !strings.Contains(err.Error(), "exclusive publication") {
		t.Fatalf("publication error = %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("failed publication left destination: %v", err)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("failed publication retired authority: %v", err)
	}
	if err := applyPitfallCorpus(root, &Changes{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("ordinary retry did not publish: %v", err)
	}
}

// This named stack exercises the complete generation-43 preflight, publication,
// retirement, retry, sections, and generation-registration contract.
// invariant: config/migrations-and-locks:pitfall-corpus-migration (TestPitfallCorpusMigrationContract)
func TestPitfallCorpusMigrationContract(t *testing.T) {
	t.Run("fields-sections-identical-retry", TestPitfallCorpusMigrationPreflightRetryAndSections)
	t.Run("relative-conflict-duplicate-preflight", TestPitfallCorpusMigrationPreflightsLinksConflictsAndDuplicates)
	t.Run("non-regular-destination-preflight", TestPitfallCorpusMigrationRejectsNonRegularDestinations)
	t.Run("create-before-retire", TestPitfallCorpusMigrationInterruptionKeepsAuthorityAndRetries)
	t.Run("empty-null-retirement", TestPitfallCorpusMigrationRetiresPresentEmptyAndNullRegistries)
	t.Run("semantic-remainder-retirement", TestPitfallCorpusMigrationDropsSemanticallyEmptyRemainders)
	t.Run("unknown-remainder-ordering", TestPitfallCorpusMigrationRejectsUnknownRemainderBeforeMutation)
	t.Run("atomic-sidecar", TestPitfallCorpusMigrationAtomicSidecarFailurePreservesBytes)
	t.Run("atomic-exclusive-publication", TestPitfallCorpusMigrationPublicationFailureLeavesRetryableState)
	t.Run("generation-chain", TestPitfallCorpusMigrationChainsHistoricalGeneration9)
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
