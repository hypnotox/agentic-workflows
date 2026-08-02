package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestCheckStagedDriftErrorPaths(t *testing.T) {
	if _, err := CheckStagedDriftRoot(testContext(t), t.TempDir()); err == nil {
		t.Fatal("staged drift root accepted a non-repository")
	}
	if _, err := (&Project{Root: t.TempDir()}).CheckStagedDrift(testContext(t)); err == nil {
		t.Fatal("staged drift accepted a project with no repository handle")
	}

	fixture := func(t *testing.T, cfg string, extra map[string]string) *Project {
		t.Helper()
		repo := gitfixture.InitRepo(t)
		head := stagedHeadFiles()
		files := map[string]string{
			".awf/config.yaml": cfg,
			".awf/awf.lock":    head[".awf/awf.lock"],
		}
		for path, body := range extra {
			files[path] = body
		}
		gitfixture.Stage(t, repo, files)
		p, err := openRootProject(repo.Root())
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("unknown target", func(t *testing.T) {
		p := fixture(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [unknown]\n", nil)
		if _, err := p.CheckStagedDrift(testContext(t)); err == nil {
			t.Fatal("staged drift accepted an unknown target")
		}
	})
	t.Run("unknown skill", func(t *testing.T) {
		p := fixture(t, "prefix: example\nintegrationBranch: main\nskills: [unknown]\nagents: []\n", nil)
		if _, err := p.CheckStagedDrift(testContext(t)); err == nil {
			t.Fatal("staged drift accepted an unknown skill")
		}
	})
	t.Run("render error", func(t *testing.T) {
		p := fixture(t, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n", map[string]string{
			".awf/skills/tdd.yaml": "data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n",
		})
		if _, err := p.CheckStagedDrift(testContext(t)); err == nil {
			t.Fatal("staged drift hid a render error")
		}
	})
}

// invariant: rendering/sync-and-drift:staged-drift-rendered-output (TestCheckStagedRenderedFilesKindsAndScope)
func TestCheckStagedRenderedFilesKindsAndScope(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: "regen-stale", Mode: snapshot.Regular, Bytes: []byte("old")},
		{Path: "regen-edit", Mode: snapshot.Regular, Bytes: []byte("old")},
		{Path: "regen-clean", Mode: snapshot.Regular, Bytes: []byte("same")},
		{Path: "ordinary-edit", Mode: snapshot.Regular, Bytes: []byte("edited")},
		{Path: "ordinary-clean", Mode: snapshot.Regular, Bytes: []byte("same")},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		".awf/efforts/.gitignore": {},
		"not-produced":            {},
		"regen-stale":             {},
		"regen-edit":              {},
		"regen-clean":             {},
		"ordinary-stale":          {TemplateHash: "old-template", ConfigHash: "config"},
		"ordinary-edit":           {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("same"))},
		"ordinary-clean":          {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("same"))},
		"ordinary-missing":        {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("same"))},
	}}
	rendered := map[string]RenderedFile{
		"regen-stale":      {Path: "regen-stale", Content: "new", Policy: OutputPolicy{Regenerate: true}},
		"regen-edit":       {Path: "regen-edit", Content: "new", TemplateID: "template", Policy: OutputPolicy{Regenerate: true}},
		"regen-clean":      {Path: "regen-clean", Content: "same", Policy: OutputPolicy{Regenerate: true}},
		"ordinary-stale":   {Path: "ordinary-stale", TemplateHash: "new-template", ConfigHash: "config"},
		"ordinary-edit":    {Path: "ordinary-edit", TemplateHash: "template", ConfigHash: "config"},
		"ordinary-clean":   {Path: "ordinary-clean", TemplateHash: "template", ConfigHash: "config"},
		"ordinary-missing": {Path: "ordinary-missing", TemplateHash: "template", ConfigHash: "config"},
	}

	got, err := checkStagedRenderedFiles(lock, rendered, snapshotTreeReader{tree: tree}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []manifest.Drift{
		{Path: "ordinary-edit", Kind: "hand-edited", Detail: "staged output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"},
		{Path: "ordinary-stale", Kind: "stale", Detail: "template or config changed; run awf render"},
		{Path: "regen-edit", Kind: "hand-edited", Detail: "staged output differs from the regenerated file; run awf render to restore awf-owned regions"},
		{Path: "regen-stale", Kind: "stale", Detail: "generated output out of date; run awf render"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("staged rendered drift:\n got %#v\nwant %#v", got, want)
	}

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "fault"), 0o755); err != nil {
		t.Fatal(err)
	}
	faultLock := &manifest.Lock{Files: map[string]manifest.Entry{"fault": {}}}
	faultRendered := map[string]RenderedFile{"fault": {Path: "fault"}}
	if _, err := checkStagedRenderedFiles(faultLock, faultRendered, filesystemProjectReader{root: root}, true); err == nil {
		t.Fatal("staged comparison erased an output read fault")
	}
}
