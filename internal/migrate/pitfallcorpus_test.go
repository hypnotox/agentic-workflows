package migrate

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"gopkg.in/yaml.v3"
)

type faultPitfallCorpusFilesystem struct {
	pitfallCorpusFilesystem
	linkInfo func(string) (fs.FileInfo, error)
	read     func(string) ([]byte, error)
	mkdir    func(string, fs.FileMode) error
	publish  func(string, []byte, fs.FileMode) error
	replace  func(string, []byte, fs.FileMode) error
	remove   func(string) error
}

func (f *faultPitfallCorpusFilesystem) LinkInfo(path string) (fs.FileInfo, error) {
	if f.linkInfo != nil {
		return f.linkInfo(path)
	}
	return f.pitfallCorpusFilesystem.LinkInfo(path)
}

func (f *faultPitfallCorpusFilesystem) Read(path string) ([]byte, error) {
	if f.read != nil {
		return f.read(path)
	}
	return f.pitfallCorpusFilesystem.Read(path)
}

func (f *faultPitfallCorpusFilesystem) MkdirAll(path string, mode fs.FileMode) error {
	if f.mkdir != nil {
		return f.mkdir(path, mode)
	}
	return f.pitfallCorpusFilesystem.MkdirAll(path, mode)
}

func (f *faultPitfallCorpusFilesystem) Publish(path string, data []byte, mode fs.FileMode) error {
	if f.publish != nil {
		return f.publish(path, data, mode)
	}
	return f.pitfallCorpusFilesystem.Publish(path, data, mode)
}

func (f *faultPitfallCorpusFilesystem) Replace(path string, data []byte, mode fs.FileMode) error {
	if f.replace != nil {
		return f.replace(path, data, mode)
	}
	return f.pitfallCorpusFilesystem.Replace(path, data, mode)
}

func (f *faultPitfallCorpusFilesystem) Remove(path string) error {
	if f.remove != nil {
		return f.remove(path)
	}
	return f.pitfallCorpusFilesystem.Remove(path)
}

func openPitfallCorpusTree(t *testing.T, root string) *filesystem.Handle {
	t.Helper()
	tree, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	return tree
}

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

func TestPitfallCorpusMigrationRootConfinementRefusesBeforeOutsideMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		arrange func(*testing.T, string, string) string
	}{
		{
			name: "escaping docs symlink",
			arrange: func(t *testing.T, root, outside string) string {
				if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
					t.Fatal(err)
				}
				sidecar := filepath.Join(outside, "pitfalls.yaml")
				if err := os.WriteFile(sidecar, []byte("data:\n  pitfalls:\n    - title: A\n      body: body\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, ".awf/docs")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				return sidecar
			},
		},
		{
			name: "escaping pitfalls symlink",
			arrange: func(t *testing.T, root, outside string) string {
				sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
				if err := os.Symlink(outside, filepath.Join(root, ".awf/docs/pitfalls")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				return sidecar
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			sidecar := tc.arrange(t, root, outside)
			before, err := os.ReadFile(sidecar)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyPitfallCorpus(root, &Changes{}); err == nil {
				t.Fatal("escaping migration root accepted")
			}
			after, err := os.ReadFile(sidecar)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("legacy authority changed: %q, %v", after, err)
			}
			if _, err := os.Stat(filepath.Join(outside, "pitfalls", "a.md")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("outside destination mutated: %v", err)
			}
		})
	}
}

func TestPitfallCorpusMigrationRejectsNonRegularRoots(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
	before, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf/docs/pitfalls"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = applyPitfallCorpus(root, &Changes{})
	if err == nil || !strings.Contains(err.Error(), "destination root .awf/docs/pitfalls is not a direct directory") {
		t.Fatalf("non-regular root error = %v", err)
	}
	after, readErr := os.ReadFile(sidecar)
	if readErr != nil || !bytes.Equal(before, after) {
		t.Fatalf("non-regular root changed authority: %q, %v", after, readErr)
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
	root := t.TempDir()
	op := openPitfallCorpusTree(t, root)
	if err := op.MkdirAll("nested", 0o755); err != nil {
		t.Fatal(err)
	}
	const leaf = "nested/leaf.md"
	if err := op.Publish(leaf, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := op.Publish(leaf, []byte("x"), 0o644); err == nil {
		t.Fatal("exclusive create replaced file")
	}
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := op.Publish("blocked/leaf", []byte("x"), 0o644); err == nil {
		t.Fatal("create under file succeeded")
	}
	if err := op.Publish("sidecar", []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := op.Replace("sidecar", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "sidecar")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("sidecar mode = %v, %v", info, err)
	}
	if err := op.Replace(".", []byte("x"), 0o644); err == nil {
		t.Fatal("wrote directory")
	}
	if err := op.Remove("sidecar"); err != nil {
		t.Fatal(err)
	}
	if err := op.Remove("missing"); err == nil {
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
	tree := openPitfallCorpusTree(t, root)
	op := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
	op.publish = func(string, []byte, os.FileMode) error { creates++; return nil }
	err := applyPitfallCorpusWith(&Changes{}, op)
	if err == nil || !strings.Contains(err.Error(), "only sections configuration may remain") {
		t.Fatalf("unknown remainder error = %v", err)
	}
	after, _ := os.ReadFile(sidecar)
	if creates != 0 || !bytes.Equal(before, after) {
		t.Fatalf("preflight ordering changed state: creates=%d before=%q after=%q", creates, before, after)
	}
}

func TestPitfallCorpusMigrationFilesystemErrorsPreserveIdentityAndPathContext(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		inject     func(*faultPitfallCorpusFilesystem, *filesystem.Handle, error)
	}{
		{"sidecar stat", "inspect " + pitfallSidecarPath, func(op *faultPitfallCorpusFilesystem, _ *filesystem.Handle, injected error) {
			op.linkInfo = func(path string) (fs.FileInfo, error) {
				if path == pitfallSidecarPath {
					return nil, injected
				}
				return op.pitfallCorpusFilesystem.LinkInfo(path)
			}
		}},
		{"sidecar read", "read " + pitfallSidecarPath, func(op *faultPitfallCorpusFilesystem, _ *filesystem.Handle, injected error) {
			op.read = func(path string) ([]byte, error) {
				if path == pitfallSidecarPath {
					return nil, injected
				}
				return op.pitfallCorpusFilesystem.Read(path)
			}
		}},
		{"docs root stat", "inspect source root " + pitfallDocsRoot, func(op *faultPitfallCorpusFilesystem, _ *filesystem.Handle, injected error) {
			op.linkInfo = func(path string) (fs.FileInfo, error) {
				if path == pitfallDocsRoot {
					return nil, injected
				}
				return op.pitfallCorpusFilesystem.LinkInfo(path)
			}
		}},
		{"destination root stat", "inspect destination root " + pitfall.SourceDir, func(op *faultPitfallCorpusFilesystem, _ *filesystem.Handle, injected error) {
			op.linkInfo = func(path string) (fs.FileInfo, error) {
				if path == pitfall.SourceDir {
					return nil, injected
				}
				return op.pitfallCorpusFilesystem.LinkInfo(path)
			}
		}},
		{"destination stat", "inspect destination .awf/docs/pitfalls/a.md", func(op *faultPitfallCorpusFilesystem, _ *filesystem.Handle, injected error) {
			op.linkInfo = func(path string) (fs.FileInfo, error) {
				if path == ".awf/docs/pitfalls/a.md" {
					return nil, injected
				}
				return op.pitfallCorpusFilesystem.LinkInfo(path)
			}
		}},
		{"mkdir", "create destination root " + pitfall.SourceDir, func(op *faultPitfallCorpusFilesystem, _ *filesystem.Handle, injected error) {
			op.mkdir = func(string, fs.FileMode) error { return injected }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
			before, _ := os.ReadFile(sidecar)
			tree := openPitfallCorpusTree(t, root)
			op := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
			injected := errors.New("injected " + tc.name)
			tc.inject(op, tree, injected)
			err := applyPitfallCorpusWith(&Changes{}, op)
			if !errors.Is(err, injected) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want identity and %q", err, tc.want)
			}
			after, _ := os.ReadFile(sidecar)
			if !bytes.Equal(before, after) {
				t.Fatal("filesystem preflight error retired authority")
			}
		})
	}
}

func TestPitfallCorpusMigrationAdditionalFilesystemRefusals(t *testing.T) {
	if err := applyPitfallCorpus(filepath.Join(t.TempDir(), "missing"), &Changes{}); err == nil || !strings.Contains(err.Error(), "open repository root") {
		t.Fatalf("missing repository root error = %v", err)
	}

	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
	before, _ := os.ReadFile(sidecar)
	tree := openPitfallCorpusTree(t, root)
	sidecarInfo, err := tree.LinkInfo(pitfallSidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	nonDirectory := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
	nonDirectory.linkInfo = func(path string) (fs.FileInfo, error) {
		if path == pitfallDocsRoot {
			return sidecarInfo, nil
		}
		return tree.LinkInfo(path)
	}
	if err := applyPitfallCorpusWith(&Changes{}, nonDirectory); err == nil || !strings.Contains(err.Error(), "source root "+pitfallDocsRoot+" is not a direct directory") {
		t.Fatalf("non-directory docs root error = %v", err)
	}

	if err := tree.MkdirAll(pitfall.SourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical, err := pitfall.Serialize(pitfall.Entry{Title: "A", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Publish(pitfall.SourceDir+"/a.md", canonical, 0o644); err != nil {
		t.Fatal(err)
	}
	readErr := errors.New("injected destination read failure")
	readFault := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
	readFault.read = func(path string) ([]byte, error) {
		if path == pitfall.SourceDir+"/a.md" {
			return nil, readErr
		}
		return tree.Read(path)
	}
	if err := applyPitfallCorpusWith(&Changes{}, readFault); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "read destination "+pitfall.SourceDir+"/a.md") {
		t.Fatalf("destination read error = %v", err)
	}
	after, _ := os.ReadFile(sidecar)
	if !bytes.Equal(before, after) {
		t.Fatal("additional filesystem refusals retired authority")
	}
}

func TestPitfallCorpusMigrationAtomicSidecarFailurePreservesBytes(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "sections:\n  prepend:\n    drop: true\ndata:\n  pitfalls:\n    - title: A\n      body: body\n")
	if err := os.Chmod(sidecar, 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(sidecar)
	tree := openPitfallCorpusTree(t, root)
	op := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
	op.replace = func(string, []byte, os.FileMode) error { return errors.New("injected atomic replacement failure") }
	if err := applyPitfallCorpusWith(&Changes{}, op); err == nil || !strings.Contains(err.Error(), "injected atomic") {
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

func TestPitfallCorpusMigrationRemovalFailureKeepsRetryableAuthority(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
	tree := openPitfallCorpusTree(t, root)
	removeErr := errors.New("injected sidecar removal failure")
	op := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree, remove: func(string) error { return removeErr }}
	var changes Changes
	err := applyPitfallCorpusWith(&changes, op)
	if !errors.Is(err, removeErr) {
		t.Fatalf("removal error = %v", err)
	}
	if !strings.Contains(err.Error(), "retire sidecar "+pitfallSidecarPath) {
		t.Fatalf("removal diagnostic = %v", err)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("removal failure retired authority: %v", err)
	}
	leaf := filepath.Join(root, ".awf/docs/pitfalls/a.md")
	if _, err := os.Stat(leaf); err != nil {
		t.Fatalf("removal failure lost created retry leaf: %v", err)
	}
	if len(changes.Items()) != 1 || !strings.Contains(changes.Items()[0].Text, "created .awf/docs/pitfalls/a.md") {
		t.Fatalf("changes = %v", changes.Items())
	}
	if err := applyPitfallCorpus(root, &Changes{}); err != nil {
		t.Fatalf("ordinary retry: %v", err)
	}
	if _, err := os.Stat(sidecar); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("retry did not retire authority: %v", err)
	}
}

func TestPitfallCorpusMigrationCommittedCleanupKeepsAuthorityUntilSafeRetry(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
	tree := openPitfallCorpusTree(t, root)
	cleanupErr := errors.New("persistent publication cleanup failure")
	const residue = ".awf/docs/pitfalls/.filepublication-injected.tmp"
	op := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
	op.publish = func(path string, data []byte, mode os.FileMode) error {
		if err := tree.Publish(path, data, mode); err != nil {
			return err
		}
		if err := tree.Publish(residue, []byte("temporary"), 0o600); err != nil {
			return err
		}
		return &filepublication.CommittedCleanupError{DestinationPath: path, ResiduePath: residue, Cause: cleanupErr}
	}
	var changes Changes
	err := applyPitfallCorpusWith(&changes, op)
	var committed *filepublication.CommittedCleanupError
	if !errors.As(err, &committed) || !errors.Is(err, cleanupErr) {
		t.Fatalf("committed cleanup error = %v", err)
	}
	if !strings.Contains(err.Error(), "remove the residue and retry awf upgrade before retiring legacy authority") || !strings.Contains(err.Error(), residue) {
		t.Fatalf("committed cleanup diagnostic = %v", err)
	}
	leaf := filepath.Join(root, ".awf/docs/pitfalls/a.md")
	raw, readErr := os.ReadFile(leaf)
	info, statErr := os.Stat(leaf)
	if readErr != nil || statErr != nil || !strings.Contains(string(raw), "title: A") || info.Mode().Perm() != 0o644 {
		t.Fatalf("committed leaf = %q, %v, %v", raw, info, errors.Join(readErr, statErr))
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("ambiguous cleanup retired authority: %v", err)
	}
	if len(changes.Items()) != 1 || !strings.Contains(changes.Items()[0].Text, "created .awf/docs/pitfalls/a.md") {
		t.Fatalf("committed changes = %v", changes.Items())
	}
	if err := tree.Remove(residue); err != nil {
		t.Fatal(err)
	}
	if err := applyPitfallCorpus(root, &Changes{}); err != nil {
		t.Fatalf("ordinary retry did not recognize committed identical leaf: %v", err)
	}
	if _, err := os.Stat(sidecar); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("retry did not retire authority: %v", err)
	}
}

func TestPitfallCorpusMigrationPublicationFailureLeavesRetryableState(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: A\n      body: body\n")
	destination := filepath.Join(root, ".awf/docs/pitfalls/a.md")
	tree := openPitfallCorpusTree(t, root)
	op := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
	op.publish = func(string, []byte, os.FileMode) error {
		return errors.New("injected atomic exclusive publication failure")
	}
	if err := applyPitfallCorpusWith(&Changes{}, op); err == nil || !strings.Contains(err.Error(), "exclusive publication") {
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
	t.Run("root-confinement-preflight", TestPitfallCorpusMigrationRootConfinementRefusesBeforeOutsideMutation)
	t.Run("non-regular-root-preflight", TestPitfallCorpusMigrationRejectsNonRegularRoots)
	t.Run("non-regular-destination-preflight", TestPitfallCorpusMigrationRejectsNonRegularDestinations)
	t.Run("create-before-retire", TestPitfallCorpusMigrationInterruptionKeepsAuthorityAndRetries)
	t.Run("empty-null-retirement", TestPitfallCorpusMigrationRetiresPresentEmptyAndNullRegistries)
	t.Run("semantic-remainder-retirement", TestPitfallCorpusMigrationDropsSemanticallyEmptyRemainders)
	t.Run("unknown-remainder-ordering", TestPitfallCorpusMigrationRejectsUnknownRemainderBeforeMutation)
	t.Run("filesystem-error-context", TestPitfallCorpusMigrationFilesystemErrorsPreserveIdentityAndPathContext)
	t.Run("additional-filesystem-refusals", TestPitfallCorpusMigrationAdditionalFilesystemRefusals)
	t.Run("atomic-sidecar", TestPitfallCorpusMigrationAtomicSidecarFailurePreservesBytes)
	t.Run("atomic-exclusive-publication", TestPitfallCorpusMigrationPublicationFailureLeavesRetryableState)
	t.Run("sidecar-removal-retry", TestPitfallCorpusMigrationRemovalFailureKeepsRetryableAuthority)
	t.Run("committed-cleanup-retry", TestPitfallCorpusMigrationCommittedCleanupKeepsAuthorityUntilSafeRetry)
	t.Run("generation-chain", TestPitfallCorpusMigrationChainsHistoricalGeneration9)
}

func TestPitfallCorpusMigrationInterruptionKeepsAuthorityAndRetries(t *testing.T) {
	root := t.TempDir()
	sidecar := writeLegacyPitfalls(t, root, "data:\n  pitfalls:\n    - title: One\n      body: one\n    - title: Two\n      body: two\n")
	tree := openPitfallCorpusTree(t, root)
	creates := 0
	op := &faultPitfallCorpusFilesystem{pitfallCorpusFilesystem: tree}
	op.publish = func(path string, data []byte, mode os.FileMode) error {
		creates++
		if creates == 2 {
			return errors.New("injected create failure")
		}
		return tree.Publish(path, data, mode)
	}
	if err := applyPitfallCorpusWith(&Changes{}, op); err == nil || !strings.Contains(err.Error(), "injected") {
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
