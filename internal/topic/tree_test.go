package topic

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// treeFrom builds a snapshot Tree from an in-memory path->content map, so an
// error case can shape an exact universe without touching the filesystem.
func treeFrom(t *testing.T, files map[string]string) *snapshot.Tree {
	t.Helper()
	var fl []snapshot.File
	for p, c := range files {
		fl = append(fl, snapshot.File{Path: p, Mode: snapshot.Regular, Bytes: []byte(c)})
	}
	tree, err := snapshot.NewTree(fl)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func parseCfg(t *testing.T, body string) *config.Config {
	t.Helper()
	cfg, err := config.Parse("/nonexistent", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// oneImplementedADR is the provenance corpus every fixture claim cites.
func oneImplementedADR() adr.Corpus {
	return mustCorpus([]adr.ADR{{Number: "0001", Status: "Implemented"}})
}

// TestLoadCorpusFromTreeValidWithoutCurrentState covers the snapshot loader's
// nil-currentState marker path, a configured domain whose sidecar is absent
// (owning no paths), and the happy assembly path.
func TestLoadCorpusFromTreeSkipsSymlinkInputs(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: ".awf/topics/metadata/alpha/link.yaml", Mode: snapshot.Symlink, Bytes: []byte("bad")},
		{Path: ".awf/topics/parts/alpha/link/current-state.md", Mode: snapshot.Symlink, Bytes: []byte("bad")},
		{Path: ".awf/domains/alpha.yaml", Mode: snapshot.Symlink, Bytes: []byte("bad")},
		{Path: "marker.go", Mode: snapshot.Symlink, Bytes: []byte("// state: alpha/link:x")},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(".awf", []byte("prefix: x\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: ['**']\n      marker: //\n"))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := LoadCorpusFromTree(tree, cfg, adr.Corpus{})
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.All()) != 0 || len(corpus.DomainPaths["alpha"]) != 0 || len(corpus.Markers.All()) != 0 {
		t.Fatalf("corpus=%#v", corpus)
	}
}

func TestLoadCorpusFromTreeValidWithoutCurrentState(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
	})
	c, err := LoadCorpusFromTree(tree, parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n"), oneImplementedADR())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.All()) != 1 || c.DomainPaths["alpha"] != nil || len(c.Markers.All()) != 0 {
		t.Fatalf("corpus: %#v paths=%#v markers=%#v", c.All(), c.DomainPaths, c.Markers.All())
	}
}

func TestLoadCorpusFromTreeSkipsResidentMarkerSources(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		".awf/effort-archive/old_test.go": "// invariant: unknown/topic:claim (TestOld)\nfunc TestOld() {}\n",
	})
	cfg := parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"**/*_test.go\"]\n      marker: //\n  testGlobs: [\"**/*_test.go\"]\n")
	if _, err := LoadCorpusFromTree(tree, cfg, oneImplementedADR()); err != nil {
		t.Fatalf("resident marker source entered immutable-tree scan: %v", err)
	}
}

func TestLoadCorpusFromTreeSkipsNestedAdoptedProjectMarkers(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
		"examples/nested/.awf/config.yaml":             "prefix: nested\nintegrationBranch: main\n",
		"examples/nested/internal/x_test.go":           "// invariant: nested/model:unknown\n",
	})
	cfg := parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"**/*_test.go\"]\n      marker: //\n  testGlobs: [\"**/*_test.go\"]\n")
	c, err := LoadCorpusFromTree(tree, cfg, oneImplementedADR())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Markers.All()) != 0 {
		t.Fatalf("nested adopted-project markers leaked into parent corpus: %#v", c.Markers.All())
	}
}

func TestLoadCorpusFromTreeErrors(t *testing.T) {
	invariantPart := "Intro.\n\n## Claims\n\n### `invariant: stable`\nStable.\nOrigin: ADR-0001\nBacking: test\n"
	currentStateCfg := "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n"
	for _, tc := range []struct {
		name    string
		cfg     string
		files   map[string]string
		wantErr string
	}{
		{
			name: "malformed metadata",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/topics/metadata/alpha/one.yaml":          "title: [unterminated\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
			},
			wantErr: "parse topic metadata",
		},
		{
			name: "bad part path",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
				".awf/topics/parts/alpha/current-state.md":     rulePart("r", "0001", ""),
			},
			wantErr: "invalid topic part path",
		},
		{
			name: "domain sidecar decode",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/domains/alpha.yaml":                      "bogusField: 1\n",
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
			},
			wantErr: "parse domain sidecar alpha",
		},
		{
			name: "empty domain path",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/domains/alpha.yaml":                      "paths: ['']\n",
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
			},
			wantErr: "domain sidecar alpha paths",
		},
		{
			name: "duplicate domain path",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/domains/alpha.yaml":                      "paths: [internal/**, internal/**]\n",
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
			},
			wantErr: "domain sidecar alpha paths",
		},
		{
			name: "malformed domain path",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/domains/alpha.yaml":                      "paths: ['[']\n",
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
			},
			wantErr: "domain sidecar alpha paths",
		},
		{
			name: "assemble failure",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/topics/metadata/alpha/two.yaml": "title: Two\nsummary: T.\npaths: [\"internal/**\"]\n",
			},
			wantErr: "topic alpha/two has metadata but no current-state part",
		},
		{
			name: "marker scan failure",
			cfg:  currentStateCfg,
			files: map[string]string{
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
				"internal/x_test.go":                           "package x\n// invariant: alpha/one:ghost (TestGhost)\nfunc TestGhost() {}\n",
			},
			wantErr: "unknown claim ID",
		},
		{
			name: "backing finalize failure",
			cfg:  "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": invariantPart,
			},
			wantErr: "test-backed invariant alpha/one:stable has no proof marker",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadCorpusFromTree(treeFrom(t, tc.files), parseCfg(t, tc.cfg), oneImplementedADR())
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

// treeFromDir builds a selected-tree snapshot from fixture files without needing
// a Git repository.
func treeFromDir(t *testing.T, root string) *snapshot.Tree {
	t.Helper()
	var files []snapshot.File
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		mode := snapshot.Regular
		if info.Mode().Perm()&0o111 != 0 {
			mode = snapshot.Executable
		}
		files = append(files, snapshot.File{Path: filepath.ToSlash(rel), Mode: mode, Bytes: b})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := snapshot.NewTree(files)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestLoadCorpusFromTreeLoadsFilesystemFixture(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: test\nintegrationBranch: main\ndomains: [alpha, beta]\ncurrentState:\n  sources:\n    - globs: [\"internal/**\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/alpha.yaml"), "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/beta.yaml"), "paths: [\"pkg/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-x.md"), testsupport.ADR("Implemented", testsupport.WithTitle("0001: X"), testsupport.WithBody("## Decision\n\n1. X.\n")))
	writeTopic(t, root, "alpha", "one", "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		"Intro.\n\n## Claims\n\n### `invariant: stable`\nStable.\nOrigin: ADR-0001\nBacking: test\n")
	writeTopic(t, root, "beta", "two", "title: Two\nsummary: T.\napplies: global\n", rulePart("g", "0001", ""))
	// A proof marker under testGlobs backs the test-backed invariant, so the
	// backing contract passes and the marker index is non-empty in both loaders.
	testsupport.WriteFile(t, filepath.Join(root, "internal/pkg/x_test.go"), "package pkg\n// invariant: alpha/one:stable (TestStable)\nfunc TestStable() {}\n")

	cfg, err := config.Load(filepath.Join(root, ".awf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.LoadCorpus(filepath.Join(root, "docs/decisions"))
	if err != nil {
		t.Fatal(err)
	}

	treeCorpus, err := LoadCorpusFromTree(treeFromDir(t, root), cfg, adrs)
	if err != nil {
		t.Fatal(err)
	}
	if sites := treeCorpus.Markers.All(); len(sites) != 1 || sites[0].Path != "internal/pkg/x_test.go" || sites[0].ClaimID != "alpha/one:stable" {
		t.Fatalf("tree marker sites: %#v", sites)
	}
}

// invariant: tooling/audit-and-snapshots:audit-history-policy-projection (TestLoadAuthorityCorpusFromTreeOmitsMarkersAndDomainPaths)
// invariant: config/validation:domain-path-globs-valid (TestLoadAuthorityCorpusFromTreeOmitsMarkersAndDomainPaths)
func TestLoadAuthorityCorpusFromTreeOmitsMarkersAndDomainPaths(t *testing.T) {
	files := map[string]string{
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
		".awf/domains/alpha.yaml":                      "unknown: [\n",
		"internal/invalid_test.go":                     "package invalid\n// invariant: alpha/one:missing (TestMissing)\nfunc TestMissing() {}\n",
	}
	cfg := parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**/*_test.go\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n")
	tree := treeFrom(t, files)

	got, err := LoadAuthorityCorpusFromFiles(tree.List(), cfg, oneImplementedADR())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.All()) != 1 || got.All()[0].ID.String() != "alpha/one" || len(got.All()[0].Claims) != 1 {
		t.Fatalf("authority topics = %#v", got.All())
	}
	if got.DomainPaths != nil || len(got.Markers.All()) != 0 {
		t.Fatalf("reduced corpus retained omitted projections: paths=%#v markers=%#v", got.DomainPaths, got.Markers.All())
	}
	if _, err := LoadCorpusFromTree(tree, cfg, oneImplementedADR()); err == nil ||
		!strings.Contains(err.Error(), "parse domain sidecar alpha") {
		t.Fatalf("full corpus accepted malformed domain sidecar: %v", err)
	}
	bothMalformed := treeFrom(t, map[string]string{
		".awf/topics/metadata/alpha/one.yaml":          "title: [\n",
		".awf/topics/parts/alpha/one/current-state.md": files[".awf/topics/parts/alpha/one/current-state.md"],
		".awf/domains/alpha.yaml":                      files[".awf/domains/alpha.yaml"],
	})
	if _, err := LoadCorpusFromTree(bothMalformed, cfg, oneImplementedADR()); err == nil ||
		strings.Contains(err.Error(), "domain sidecar") {
		t.Fatalf("full corpus changed metadata-before-sidecar error precedence: %v", err)
	}
	markerOnly := treeFrom(t, map[string]string{
		".awf/topics/metadata/alpha/one.yaml":          files[".awf/topics/metadata/alpha/one.yaml"],
		".awf/topics/parts/alpha/one/current-state.md": files[".awf/topics/parts/alpha/one/current-state.md"],
		"internal/invalid_test.go":                     files["internal/invalid_test.go"],
	})
	if _, err := LoadCorpusFromTree(markerOnly, cfg, oneImplementedADR()); err == nil ||
		!strings.Contains(err.Error(), "unknown claim ID") {
		t.Fatalf("full corpus accepted malformed proof marker: %v", err)
	}
}

type readerForLoadCorpusTest struct {
	paths   []string
	files   map[string][]byte
	readErr error
}

func (r readerForLoadCorpusTest) Paths(string) ([]string, error) { return r.paths, nil }
func (r readerForLoadCorpusTest) ReadFile(name string) ([]byte, bool, error) {
	if r.readErr != nil {
		return nil, false, r.readErr
	}
	data, ok := r.files[name]
	return data, ok, nil
}

type streamingMarkerReader struct {
	paths                   []string
	materialized, passes    int
	linesPerPass, lineBytes int
}

func (r *streamingMarkerReader) Paths(string) ([]string, error) { return r.paths, nil }
func (r *streamingMarkerReader) ReadFile(string) ([]byte, bool, error) {
	r.materialized++
	return nil, false, os.ErrPermission
}
func (r *streamingMarkerReader) ReadLines(_ string, maxLineBytes int, visit func(string) error) (bool, error) {
	r.passes++
	line := strings.Repeat("x", r.lineBytes)
	if len(line) > maxLineBytes {
		return true, os.ErrInvalid
	}
	for range r.linesPerPass {
		if err := visit(line); err != nil {
			return true, err
		}
	}
	return true, nil
}

func TestLoadCorpusFromReaderPropagatesReadFailure(t *testing.T) {
	_, err := LoadCorpusFromReader(readerForLoadCorpusTest{paths: []string{".awf/topics/metadata/alpha/one.yaml"}, readErr: os.ErrPermission}, parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n"), oneImplementedADR())
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("LoadCorpusFromReader error = %v", err)
	}
}

func TestLoadCorpusFromReaderStreamsLargeLogicalMarkerSources(t *testing.T) {
	reader := &streamingMarkerReader{
		paths: []string{"first.go", "second.go", "third.go"},
		// The logical corpus is 192 MiB, but the reader retains one shared line.
		linesPerPass: 64 << 10,
		lineBytes:    1 << 10,
	}
	cfg := parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: ['**/*.go']\n      marker: //\n")
	if _, err := LoadCorpusFromReader(reader, cfg, oneImplementedADR()); err != nil {
		t.Fatal(err)
	}
	if reader.materialized != 0 || reader.passes != len(reader.paths) {
		t.Fatalf("materialized reads = %d, line passes = %d, want 0 and %d", reader.materialized, reader.passes, len(reader.paths))
	}
}

func TestLoadCorpusFromReaderReadsOnlySemanticInputs(t *testing.T) {
	cfg := parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: ['**/*.go']\n      marker: //\n")
	for _, path := range []string{"large.out", ".awf/effort-archive/large.go"} {
		t.Run(path, func(t *testing.T) {
			reader := readerForLoadCorpusTest{paths: []string{path}, readErr: os.ErrPermission}
			if _, err := LoadCorpusFromReader(reader, cfg, oneImplementedADR()); err != nil {
				t.Fatalf("LoadCorpusFromReader read non-input %q: %v", path, err)
			}
		})
	}
}

func TestLoadCorpusFromReaderRejectsDuplicateSelectedPaths(t *testing.T) {
	const path = ".awf/topics/metadata/alpha/one.yaml"
	reader := readerForLoadCorpusTest{paths: []string{path, path}, files: map[string][]byte{path: []byte("x")}}
	if _, err := LoadCorpusFromReader(reader, parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n"), oneImplementedADR()); err == nil {
		t.Fatal("duplicate selected paths were accepted")
	}
}
