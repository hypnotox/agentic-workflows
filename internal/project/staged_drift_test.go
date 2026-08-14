package project

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/templates"
)

func snapshotMembership(tree *snapshot.Tree) map[string]bool {
	indexed := map[string]bool{}
	for _, file := range tree.List() {
		indexed[file.Path] = true
	}
	return indexed
}

// invariant: rendering/sync-and-drift:ordinary-render-freshness (TestStagedDriftClassifiesFreshnessBeforeObservation)
func TestStagedDriftClassifiesFreshnessBeforeObservation(t *testing.T) {
	const path = "ordinary-output"
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		path: {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("locked output"))},
	}}
	rendered := map[string]RenderedFile{
		path: {Path: path, Content: "fresh render", TemplateHash: "template", ConfigHash: "config"},
	}
	want := []manifest.Drift{
		{Path: ".awf/awf.lock", Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f .awf/awf.lock"},
		{Path: path, Kind: "stale", Detail: "rendered output out of date; run awf render"},
	}
	assertStale := func(t *testing.T, reader ProjectTreeReader, indexed map[string]bool) {
		t.Helper()
		got, err := checkStagedRenderedFiles(lock, rendered, reader, indexed, true)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("staged binary-derived drift = %#v, want %#v", got, want)
		}
	}
	t.Run("before hand edit", func(t *testing.T) {
		tree, err := snapshot.NewTree([]snapshot.File{{Path: path, Mode: snapshot.Regular, Bytes: []byte("hand edit")}})
		if err != nil {
			t.Fatal(err)
		}
		assertStale(t, snapshotTreeReader{tree: tree}, snapshotMembership(tree))
	})
	t.Run("before missing", func(t *testing.T) {
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := checkStagedRenderedFiles(lock, rendered, snapshotTreeReader{tree: tree}, snapshotMembership(tree), true)
		if err != nil {
			t.Fatal(err)
		}
		wantMissing := []manifest.Drift{
			{Path: ".awf/awf.lock", Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f .awf/awf.lock"},
			{Path: path, Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f " + path},
		}
		if !reflect.DeepEqual(got, wantMissing) {
			t.Fatalf("staged missing membership = %#v, want %#v", got, wantMissing)
		}
	})
	t.Run("before read failure", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, path), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := checkStagedRenderedFiles(lock, rendered, filesystemProjectReader{root: root}, map[string]bool{path: true}, true); err == nil {
			t.Fatal("staged membership erased reader failure")
		}
	})
}

func TestStagedDriftTreatsSymlinkAsIndexedMetadata(t *testing.T) {
	const path = "generated-output"
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: config.DirName + "/awf.lock", Mode: snapshot.Regular, Bytes: []byte("lock")},
		{Path: path, Mode: snapshot.Symlink, Bytes: []byte("target")},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{path: {}}}
	rendered := map[string]RenderedFile{
		path: {Path: path, Content: "generated", TemplateID: "template", Policy: OutputPolicy{Regenerate: true}},
	}
	got, err := checkStagedRenderedFiles(lock, rendered, snapshotTreeReader{tree: tree}, snapshotMembership(tree), true)
	if err != nil {
		t.Fatal(err)
	}
	want := []manifest.Drift{{Path: path, Kind: "hand-edited", Detail: "staged output differs from the regenerated file; run awf render to restore awf-owned regions"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("staged symlink drift = %#v, want %#v", got, want)
	}
}

// invariant: config/configuration:template-source-root (TestCheckStagedDriftUsesIndexedTemplateSourceMappings)
func TestCheckStagedDriftUsesIndexedTemplateSourceMappings(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	if err := fs.WalkDir(templates.FS, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := templates.FS.ReadFile(name)
		if err != nil {
			return err
		}
		out := filepath.Join(root, "templates", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, body, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	repo := gitfixture.InitRepoAt(t, root)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "source mappings", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "rendered", nil)
	// The immutable index enables mappings while the working tree is missing a
	// required root source. A staged operation must not consult that deletion.
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: example\nprofile: full\nintegrationBranch: main\nrender:\n  templateSourceRoot: templates\n"})
	if err := os.Remove(filepath.Join(root, "templates", "docs", "architecture.md.tmpl")); err != nil {
		t.Fatal(err)
	}
	drift, err := CheckStagedDriftRoot(testContext(t), root)
	if err != nil {
		t.Fatalf("staged mapping consulted missing working source: %v", err)
	}
	if len(drift) == 0 {
		t.Fatal("staged root activation did not participate in drift")
	}
}

func TestCheckStagedDriftLocalDocsUsesIndexUniverse(t *testing.T) {
	const configYAML = "prefix: example\nprofile: full\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n"
	root := scaffold(t, configYAML)
	repo := gitfixture.InitRepoAt(t, root)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "config", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "local document", nil)
	// Stage the coherent config, output, and lock universe, then remove its
	// working-tree local facts. The staged checker must not consult the latter.
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml":          mustReadFile(t, filepath.Join(root, ".awf/config.yaml")),
		"docs/runbooks/incident.md": mustReadFile(t, filepath.Join(root, "docs/runbooks/incident.md")),
		".awf/awf.lock":             mustReadFile(t, filepath.Join(root, ".awf/awf.lock")),
	})
	before, err := CheckStagedDriftRoot(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "docs/runbooks/incident.md")); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	after, err := CheckStagedDriftRoot(testContext(t), root)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("staged local-doc universe contaminated: before=%v after=%v err=%v", before, after, err)
	}
}

// invariant: rendering/sync-and-drift:staged-drift-rendered-output (TestCheckStagedDriftOperationalErrors)
func TestCheckStagedDriftOperationalErrors(t *testing.T) {
	t.Run("cancelled index read", func(t *testing.T) {
		root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
		repo := gitfixture.InitRepoAt(t, root)
		gitfixture.AddAll(t, repo)
		ctx, cancel := context.WithCancel(testContext(t))
		cancel()
		if _, err := CheckStagedDriftRoot(ctx, root); err == nil {
			t.Fatal("cancelled staged index read succeeded")
		}
	})
	t.Run("invalid lock", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		testsupport.WriteAwfConfig(t, repo.Root(), "prefix: example\nprofile: full\nintegrationBranch: main\n")
		gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: example\nprofile: full\nintegrationBranch: main\n", ".awf/awf.lock": "{bad"})
		if _, err := CheckStagedDriftRoot(testContext(t), repo.Root()); err == nil {
			t.Fatal("invalid staged lock succeeded")
		}
	})
	t.Run("missing config", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		if _, err := CheckStagedDriftRoot(testContext(t), repo.Root()); err == nil || !strings.Contains(err.Error(), "no staged .awf/config.yaml") {
			t.Fatalf("missing staged config error = %v", err)
		}
	})
	t.Run("invalid config", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: ["})
		if _, err := CheckStagedDriftRoot(testContext(t), repo.Root()); err == nil {
			t.Fatal("invalid staged config succeeded")
		}
	})
	t.Run("invalid pitfalls", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Stage(t, repo, map[string]string{
			".awf/config.yaml":          "prefix: example\nprofile: full\nintegrationBranch: main\n",
			".awf/docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n",
		})
		if _, err := CheckStagedDriftRoot(testContext(t), repo.Root()); err == nil {
			t.Fatal("invalid staged pitfalls succeeded")
		}
	})
}

// invariant: rendering/sync-and-drift:generated-artifacts-tracked (TestCheckStagedDriftTracksWholeOutputPlan)
func TestCheckStagedDriftTracksWholeOutputPlan(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	repo := gitfixture.InitRepoAt(t, root)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "staged config", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	corpus, pitfalls, topics, effective, err := p.deriveOperationStateWithPitfalls()
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.outputPlanWithPitfalls(testContext(t), corpus, pitfalls, topics, effective)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{config.DirName + "/awf.lock"}
	for _, output := range op.writeFiles() {
		want = append(want, output.Path)
	}
	slices.Sort(want)

	drift, err := CheckStagedDriftRoot(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(drift))
	for _, finding := range drift {
		if finding.Kind != "untracked" {
			t.Fatalf("whole-plan staged tracking produced non-membership drift: %#v", finding)
		}
		got = append(got, finding.Path)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("staged tracking paths differ from every OutputPlan write plus lock:\n got %q\nwant %q", got, want)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// invariant: rendering/sync-and-drift:generated-artifacts-tracked (TestStagedDriftRenderedOutputInvariant)
// invariant: rendering/sync-and-drift:staged-drift-rendered-output (TestStagedDriftRenderedOutputInvariant)
// invariant: rendering/sync-and-drift:ordinary-render-freshness (TestStagedDriftRenderedOutputInvariant)
func TestStagedDriftRenderedOutputInvariant(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte("lock")},
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
		{Path: "ordinary-binary", Mode: snapshot.Regular, Bytes: []byte("edited")},
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
		"ordinary-binary":         {TemplateHash: "template", ConfigHash: "config", OutputHash: manifest.Hash([]byte("same"))},
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
		"ordinary-binary":         {Path: "ordinary-binary", Content: "fresh", TemplateHash: "template", ConfigHash: "config"},
		"ordinary-edit":           {Path: "ordinary-edit", Content: "same", TemplateHash: "template", ConfigHash: "config"},
		"ordinary-clean":          {Path: "ordinary-clean", Content: "same", TemplateHash: "template", ConfigHash: "config"},
		"ordinary-missing":        {Path: "ordinary-missing", Content: "same", TemplateHash: "template", ConfigHash: "config"},
		"dead-reference.md":       {Path: "dead-reference.md", Content: "[missing](absent.md)", TemplateHash: "template", ConfigHash: "config", Policy: OutputPolicy{ScanReferences: true}},
		"invalid-frontmatter.md":  {Path: "invalid-frontmatter.md", Content: "not frontmatter", TemplateHash: "template", ConfigHash: "config", Policy: OutputPolicy{ValidateFrontmatter: true}},
		"missing-provenance.md":   {Path: "missing-provenance.md", Content: "no generated banner", TemplateHash: "template", ConfigHash: "config"},
		"bad-attribution.md":      {Path: "bad-attribution.md", Content: "unattributed", TemplateHash: "template", ConfigHash: "config"},
	}

	got, err := checkStagedRenderedFiles(lock, rendered, snapshotTreeReader{tree: tree}, snapshotMembership(tree), false)
	if err != nil {
		t.Fatal(err)
	}
	// The exact result also proves the named repo-only fixtures above stay
	// silent: config hygiene, backup, dead-reference, frontmatter, provenance,
	// attribution, and orphaned-lock inputs produce no additional kind.
	want := []manifest.Drift{
		{Path: "ordinary-binary", Kind: "stale", Detail: "rendered output out of date; run awf render"},
		{Path: "ordinary-edit", Kind: "hand-edited", Detail: "staged output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"},
		{Path: "ordinary-missing", Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f ordinary-missing"},
		{Path: "ordinary-stale", Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f ordinary-stale"},
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
	faultLock := &manifest.Lock{Files: map[string]manifest.Entry{"fault": {OutputHash: manifest.Hash(nil)}}}
	faultReader := filesystemProjectReader{root: root}
	faultRendered := map[string]RenderedFile{"fault": {Path: "fault"}}
	if _, err := checkStagedRenderedFiles(faultLock, faultRendered, faultReader, map[string]bool{"fault": true}, true); err == nil {
		t.Fatal("staged comparison erased an ordinary output read fault")
	}
	faultRendered["fault"] = RenderedFile{Path: "fault", Policy: OutputPolicy{Regenerate: true}}
	if _, err := checkStagedRenderedFiles(faultLock, faultRendered, faultReader, map[string]bool{"fault": true}, true); err == nil {
		t.Fatal("staged comparison erased a regenerated output read fault")
	}

	// Dirty working output and config must not contaminate the staged comparison.
	const baselineConfig = "prefix: example\nprofile: full\nintegrationBranch: main\n"
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
