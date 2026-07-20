package topic

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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
	return adr.NewCorpus([]adr.ADR{{Number: "0001", Status: "Implemented"}})
}

// TestLoadCorpusFromTreeValidWithoutCurrentState covers the snapshot loader's
// nil-currentState marker path, a configured domain whose sidecar is absent
// (owning no paths), and the happy assembly path.
func TestLoadCorpusFromTreeValidWithoutCurrentState(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
	})
	c, err := LoadCorpusFromTree(tree, parseCfg(t, "prefix: test\ndomains: [alpha]\n"), oneImplementedADR())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.All()) != 1 || c.DomainPaths["alpha"] != nil || len(c.Markers.All()) != 0 {
		t.Fatalf("corpus: %#v paths=%#v markers=%#v", c.All(), c.DomainPaths, c.Markers.All())
	}
}

func TestLoadCorpusFromTreeSkipsNestedAdoptedProjectMarkers(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
		"examples/nested/.awf/config.yaml":             "prefix: nested\n",
		"examples/nested/internal/x_test.go":           "// invariant: nested/model:unknown\n",
	})
	cfg := parseCfg(t, "prefix: test\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"**/*_test.go\"]\n      marker: //\n  testGlobs: [\"**/*_test.go\"]\n")
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
	currentStateCfg := "prefix: test\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n"
	for _, tc := range []struct {
		name    string
		cfg     string
		files   map[string]string
		wantErr string
	}{
		{
			name: "malformed metadata",
			cfg:  "prefix: test\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/topics/metadata/alpha/one.yaml":          "title: [unterminated\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
			},
			wantErr: "parse topic metadata",
		},
		{
			name: "bad part path",
			cfg:  "prefix: test\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
				".awf/topics/parts/alpha/current-state.md":     rulePart("r", "0001", ""),
			},
			wantErr: "invalid topic part path",
		},
		{
			name: "domain sidecar decode",
			cfg:  "prefix: test\ndomains: [alpha]\n",
			files: map[string]string{
				".awf/domains/alpha.yaml":                      "bogusField: 1\n",
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": rulePart("r", "0001", ""),
			},
			wantErr: "parse domain sidecar alpha",
		},
		{
			name: "assemble failure",
			cfg:  "prefix: test\ndomains: [alpha]\n",
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
				"internal/x_test.go":                           "package x\n// invariant: alpha/one:ghost\n",
			},
			wantErr: "unknown claim ID",
		},
		{
			name: "backing finalize failure",
			cfg:  "prefix: test\ndomains: [alpha]\n",
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

// treeFromDir builds a snapshot Tree from every regular file under root,
// mirroring the working universe without needing a Git repository, so a
// filesystem load and a snapshot load can be compared over identical bytes.
func treeFromDir(t *testing.T, root string) *snapshot.Tree {
	t.Helper()
	var files []snapshot.File
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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

func TestLoadCorpusFromTreeMatchesFilesystem(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: test\ndomains: [alpha, beta]\ncurrentState:\n  sources:\n    - globs: [\"internal/**\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/alpha.yaml"), "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/beta.yaml"), "paths: [\"pkg/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-x.md"), testsupport.ADR("Implemented", testsupport.WithTitle("0001: X"), testsupport.WithBody("## Decision\n\n1. X.\n")))
	writeTopic(t, root, "alpha", "one", "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
		"Intro.\n\n## Claims\n\n### `invariant: stable`\nStable.\nOrigin: ADR-0001\nBacking: test\n")
	writeTopic(t, root, "beta", "two", "title: Two\nsummary: T.\napplies: global\n", rulePart("g", "0001", ""))
	// A proof marker under testGlobs backs the test-backed invariant, so the
	// backing contract passes and the marker index is non-empty in both loaders.
	testsupport.WriteFile(t, filepath.Join(root, "internal/pkg/x_test.go"), "package pkg\n// invariant: alpha/one:stable\n")

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

	fsCorpus, err := LoadCorpus(root, cfg, adrs)
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
	assertSameCorpus(t, fsCorpus, treeCorpus)
}

// assertSameCorpus checks that two corpora carry identical semantic content -
// topics, metadata, claims, ownership globs, and marker sites - ignoring only
// the source paths, which legitimately differ between an absolute filesystem
// path and a repo-relative snapshot path.
func assertSameCorpus(t *testing.T, want, got Corpus) {
	t.Helper()
	byID := func(ts []Topic) map[string]Topic {
		m := map[string]Topic{}
		for _, tp := range ts {
			m[tp.ID.String()] = tp
		}
		return m
	}
	wm, gm := byID(want.All()), byID(got.All())
	if len(wm) != len(gm) {
		t.Fatalf("topic count %d != %d", len(wm), len(gm))
	}
	for id, wt := range wm {
		gt, ok := gm[id]
		if !ok {
			t.Fatalf("snapshot corpus missing topic %s", id)
		}
		if !reflect.DeepEqual(wt.Metadata, gt.Metadata) {
			t.Fatalf("%s metadata: %#v != %#v", id, wt.Metadata, gt.Metadata)
		}
		if !reflect.DeepEqual(wt.Claims, gt.Claims) {
			t.Fatalf("%s claims: %#v != %#v", id, wt.Claims, gt.Claims)
		}
	}
	if !reflect.DeepEqual(want.DomainPaths, got.DomainPaths) {
		t.Fatalf("domainPaths: %#v != %#v", want.DomainPaths, got.DomainPaths)
	}
	if !reflect.DeepEqual(want.Markers.All(), got.Markers.All()) {
		t.Fatalf("markers: %#v != %#v", want.Markers.All(), got.Markers.All())
	}
}
