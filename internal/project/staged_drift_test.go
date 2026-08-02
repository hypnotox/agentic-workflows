package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
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

	t.Run("malformed local sidecar", func(t *testing.T) {
		p := fixture(t, "prefix: example\nintegrationBranch: main\nskills: [local]\nagents: []\n", map[string]string{
			".awf/skills/local.yaml": "local: [bad\n",
		})
		if _, err := p.CheckStagedDrift(testContext(t)); err == nil {
			t.Fatal("staged drift erased a local catalog synthesis error")
		}
	})
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

// invariant: rendering/sync-and-drift:staged-drift-rendered-output (TestStagedDriftRenderedOutputInvariant)
func TestStagedDriftRenderedOutputInvariant(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: ".awf/efforts/.gitignore", Mode: snapshot.Regular, Bytes: []byte("resident edit")},
		{Path: ".awf/orphan.yaml", Mode: snapshot.Regular, Bytes: []byte("config hygiene orphan")},
		{Path: "stale.awf-bak", Mode: snapshot.Regular, Bytes: []byte("stale backup")},
		{Path: "dead-reference.md", Mode: snapshot.Regular, Bytes: []byte("[missing](absent.md)")},
		{Path: "invalid-frontmatter.md", Mode: snapshot.Regular, Bytes: []byte("not frontmatter")},
		{Path: "missing-provenance.md", Mode: snapshot.Regular, Bytes: []byte("no generated banner")},
		{Path: "bad-attribution.md", Mode: snapshot.Regular, Bytes: []byte("unattributed")},
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
		"dead-reference.md":       {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("[missing](absent.md)"))},
		"invalid-frontmatter.md":  {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("not frontmatter"))},
		"missing-provenance.md":   {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("no generated banner"))},
		"bad-attribution.md":      {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("unattributed"))},
	}}
	rendered := map[string]RenderedFile{
		".awf/efforts/.gitignore": {Path: ".awf/efforts/.gitignore", TemplateHash: "new-template", ConfigHash: "config"},
		"regen-stale":             {Path: "regen-stale", Content: "new", Policy: OutputPolicy{Regenerate: true}},
		"regen-edit":              {Path: "regen-edit", Content: "new", TemplateID: "template", Policy: OutputPolicy{Regenerate: true}},
		"regen-clean":             {Path: "regen-clean", Content: "same", Policy: OutputPolicy{Regenerate: true}},
		"ordinary-stale":          {Path: "ordinary-stale", TemplateHash: "new-template", ConfigHash: "config"},
		"ordinary-edit":           {Path: "ordinary-edit", TemplateHash: "template", ConfigHash: "config"},
		"ordinary-clean":          {Path: "ordinary-clean", TemplateHash: "template", ConfigHash: "config"},
		"ordinary-missing":        {Path: "ordinary-missing", TemplateHash: "template", ConfigHash: "config"},
		"dead-reference.md":       {Path: "dead-reference.md", TemplateHash: "template", ConfigHash: "config", Policy: OutputPolicy{ScanReferences: true}},
		"invalid-frontmatter.md":  {Path: "invalid-frontmatter.md", TemplateHash: "template", ConfigHash: "config", Policy: OutputPolicy{ValidateFrontmatter: true}},
		"missing-provenance.md":   {Path: "missing-provenance.md", TemplateHash: "template", ConfigHash: "config"},
		"bad-attribution.md":      {Path: "bad-attribution.md", TemplateHash: "template", ConfigHash: "config"},
	}

	got, err := checkStagedRenderedFiles(lock, rendered, snapshotTreeReader{tree: tree}, false)
	if err != nil {
		t.Fatal(err)
	}
	// The exact result also proves the named repo-only fixtures above stay
	// silent: config hygiene, backup, dead-reference, frontmatter, provenance,
	// attribution, and orphaned-lock inputs produce no additional kind.
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

	// Dirty working output and config must not contaminate the staged comparison.
	const baselineConfig = "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n"
	projectRoot := scaffold(t, baselineConfig)
	repo := gitfixture.InitRepoAt(t, projectRoot)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "config", nil)
	p, err := Open(testContext(t), projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "rendered baseline", nil)
	lockOnDisk, err := manifest.Load(filepath.Join(projectRoot, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var output string
	for path := range lockOnDisk.Files {
		if !resident.IsResidentPath(path) {
			output = path
			break
		}
	}
	if output == "" {
		t.Fatal("fixture has no tracked rendered output")
	}
	if err := os.WriteFile(filepath.Join(projectRoot, filepath.FromSlash(output)), []byte("dirty working output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if drift, err := CheckStagedDriftRoot(testContext(t), projectRoot); err != nil || len(drift) != 0 {
		t.Fatalf("working output contaminated staged drift: drift=%v err=%v", drift, err)
	}

	// Conversely, a staged config change must drive rendering even when the
	// working config is restored to the clean baseline.
	stagedConfig := strings.Replace(baselineConfig, "prefix: example", "prefix: staged", 1)
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": stagedConfig})
	testsupport.WriteAwfConfig(t, projectRoot, baselineConfig)
	drift, err := CheckStagedDriftRoot(testContext(t), projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	var stale bool
	for _, finding := range drift {
		stale = stale || finding.Kind == "stale"
	}
	if !stale {
		t.Fatalf("staged config did not drive rendered-output drift: %v", drift)
	}
}
