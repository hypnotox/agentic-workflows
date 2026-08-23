package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
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
	if err := syncProject(p); err != nil {
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
	if err := syncProject(p); err != nil {
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
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	corpus, pitfalls, topics, effective, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	op, err := outputPlanWithPitfalls(renderInputsForTest(p), corpus, pitfalls, topics, effective)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{config.DirName + "/awf.lock"}
	for _, output := range planWriteFiles(op) {
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
	if err := syncProject(p); err != nil {
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

// TestCheckStagedDriftProjectsFullFromAWorkingCoreTree proves the index
// universe is selected from the immutable complete catalog, not the Core
// working ProjectState that opened the check.
// invariant: rendering/project-output-plan:profile-projected-render (TestCheckStagedDriftProjectsFullFromAWorkingCoreTree)
func TestCheckStagedDriftPreservesInjectedCompleteCatalog(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: core\nintegrationBranch: main\n")
	repo := gitfixture.InitRepoAt(t, root)
	repoHandle, _, err := awfgit.OpenContaining(root)
	if err != nil {
		t.Fatal(err)
	}
	injected := *catalog.Standard
	injected.Skills = maps.Clone(catalog.Standard.Skills)
	tdd := injected.Skills["tdd"]
	tdd.FullOnly = true
	injected.Skills["tdd"] = tdd
	loader := NewLoader(config.Load, &injected, func(_ context.Context, root string) string { return root }, repoHandle)
	p, err := loader.Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".pi", "skills", "example-tdd", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("injected Core catalog unexpectedly rendered tdd: %v", err)
	}
	gitfixture.AddAll(t, repo)
	drift, err := checkStagedDriftProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Fatalf("staged reprojection discarded injected complete catalog: %#v", drift)
	}
}

func TestCheckStagedDriftProjectsFullFromAWorkingCoreTree(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: core\nintegrationBranch: main\n")
	repo := gitfixture.InitRepoAt(t, root)
	core, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(core); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "core baseline", nil)

	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: test-gate\n")
	full, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(full); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, repo)

	// CheckStagedDriftRoot opens this restored Core working tree before reading
	// the Full index. A working-tree-derived catalog would omit Full outputs.
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\nvars:\n  gateCmd: test-gate\n")
	drift, err := CheckStagedDriftRoot(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Fatalf("Core working tree contaminated coherent Full index: %#v", drift)
	}
}
func boundaryADR(format, title string) string {
	return "---\nformat: " + format + "\nstatus: Proposed\ndate: 2026-07-21\n---\n" +
		"# ADR-0002: " + title + "\n\n## Context\n\nContext.\n\n## Decision\n\n1. Decide.\n\n" +
		"## State changes\n\nNone.\n\n## Consequences\n\nConsequence.\n\n" +
		"## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-21: Proposed\n"
}

func TestCheckStagedCleanWithCoverage(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{"internal/bar.go": "package internalx\n"})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	// A different working lock must not contaminate the staged universe.
	testsupport.WriteFile(t, lockFile(p.Root()), "{not json")

	report, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if len(report.Static) != 0 {
		t.Fatalf("static findings = %#v; want none for an unchanged topic set", report.Static)
	}
	findings := currentStateFindings(report)
	if len(findings) != 1 || !strings.Contains(findings[0], "internal/bar.go") {
		t.Fatalf("findings = %#v; want exactly the uncovered internal/bar.go", findings)
	}
}

// TestCheckStagedNoPolicy proves the staged path evaluates coverage and fan-out
// for a tree that declares no currentState block, the staged half of the
// contract TestCheckCurrentStateNoPolicy pins for the working tree (ADR-0192).
// stagedHeadFiles already scopes one claim-bearing topic to internal/foo/**, so
// eight more claimless topics take that path over the nil-receiver budget of 8.
// invariant: rendering/sync-and-drift:coverage-evaluation-unconditional (TestCheckStagedNoPolicy)
func TestCheckStagedNoPolicy(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	head := stagedHeadFiles()
	head[".awf/config.yaml"] = csNoPolicyYAML
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("fan%d", i)
		head[".awf/topics/metadata/alpha/"+name+".yaml"] = fmt.Sprintf("title: Fan %d\nsummary: Fan-out fixture topic %d.\npaths:\n  - internal/foo/**\n", i, i)
		head[".awf/topics/parts/alpha/"+name+"/current-state.md"] = "Intro.\n\n## Claims\n"
	}
	gitfixture.Stage(t, repo, head)
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{
		"internal/bar.go":   "package internalx\n",
		"internal/foo/x.go": "package foo\n",
	})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if report.Coverage == nil {
		t.Fatal("staged coverage = nil; want evaluation without a currentState policy")
	}
	want := []topic.CoverageFinding{
		{Path: "internal/bar.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error},
		{Path: "internal/foo/x.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 9},
	}
	if !reflect.DeepEqual(report.Coverage, want) {
		t.Fatalf("staged coverage:\n got %#v\nwant %#v", report.Coverage, want)
	}
}

// TestCheckStagedRejectsBridgePromotionWithArbitraryV2Boundary uses snapshots
// whose ADR-0002 bytes are valid only under each side's own lock: V1 under the
// bridge HEAD and V2 under the staged permanent lock. Phase 3 must reject that
// arbitrary V2 activation rather than treating it as the sealed V1 promotion.
// TestCheckStagedTransitionFinding stages a claim removal with no removing ADR:
// the HEAD-to-index diff surfaces the unmatched mutation.
func TestCheckStagedTransitionFinding(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	// Re-stage the topic part with its claim removed.
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if len(report.Static) == 0 || !strings.Contains(report.Static[0].Message, "was removed with no ADR remove operation") {
		t.Fatalf("static = %#v; want the unmatched-removal finding", report.Static)
	}
}

// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestCheckStagedMarksOlderIntroductionsProvisionalWithoutSuppressingFindings)
func TestCheckStagedMarksOlderIntroductionsProvisionalWithoutSuppressingFindings(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	head := stagedHeadFiles()
	head["docs/decisions/0003-existing.md"] = boundaryADR(adr.V1FormatMarker, "Existing")
	head["docs/decisions/0004-aggregate.md"] = publicV2ADR(t, "0004", "Aggregate", "Proposed", "- add `alpha/one:x`\n- add `alpha/one:y`\n- add `alpha/one:z`", "")
	gitfixture.Stage(t, repo, head)
	gitfixture.Commit(t, repo, "head", nil)

	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0002-stale.md": boundaryADR(adr.V2FormatMarker, "Stale")})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	clean, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("clean CheckStaged: %v", err)
	}
	want := []currentstate.Introduction{{Identity: "0002", Format: adr.CurrentStateV2}}
	if !reflect.DeepEqual(clean.Provisional, want) || len(currentStateFindings(clean)) != 0 {
		t.Fatalf("clean provisional report = %#v, findings = %#v; want %#v and no findings", clean.Provisional, currentStateFindings(clean), want)
	}

	aggregate := publicV2ADR(t, "0004", "Aggregate", "Implementing", "- add `alpha/one:x`\n- add `alpha/one:y`\n- add `alpha/one:z`",
		"- 2026-07-22: Implementing; content-sha256: %s\n- 2026-07-22: Applied; operations: add `alpha/one:x`\n- 2026-07-22: Applied; operations: add `alpha/one:y`")
	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0003-existing.md":              boundaryADR(adr.V2FormatMarker, "Existing"),
		"docs/decisions/0004-aggregate.md":             aggregate,
		".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n",
		"internal/bar.go":                              "package internalx\n",
	})
	report, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged with unrelated violations: %v", err)
	}
	if !reflect.DeepEqual(report.Provisional, want) {
		t.Fatalf("provisional = %#v, want %#v", report.Provisional, want)
	}
	findings := strings.Join(currentStateFindings(report), "\n")
	for _, wantFinding := range []string{
		"was removed with no ADR remove operation",
		"internal/bar.go",
		"changed governed format across this transition",
		"adds claim alpha/one:x, which has no active claim",
	} {
		if !strings.Contains(findings, wantFinding) {
			t.Fatalf("unrelated blocking finding %q was suppressed:\n%s", wantFinding, findings)
		}
	}
	if notes := strings.Join(report.Information(), "\n"); !strings.Contains(notes, "provisional older-format ADR-0002") {
		t.Fatalf("provisional note missing:\n%s", notes)
	}
}

// TestCheckStagedNestedAdopter validates HEAD/index snapshots through a project
// rooted inside a containing monorepo.
func TestCheckStagedNestedAdopter(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := map[string]string{}
	for path, body := range stagedHeadFiles() {
		files["fixtures/nested-adopter/"+path] = body
	}
	lockBytes, err := attestedLock().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	files["fixtures/nested-adopter/.awf/awf.lock"] = string(lockBytes)
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "nested head", nil)
	gitfixture.Stage(t, repo, map[string]string{
		"fixtures/nested-adopter/.awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n",
	})
	p := openStaged(t, filepath.Join(dir, "fixtures", "nested-adopter"))
	report, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("nested CheckStaged: %v", err)
	}
	if !strings.Contains(strings.Join(currentStateFindings(report), "\n"), "was removed with no ADR remove operation") {
		t.Fatalf("nested findings = %#v; want staged transition finding", currentStateFindings(report))
	}
}

// TestCheckStagedUnmergedIndex rejects a conflicted index at the staged-check
// boundary rather than attempting to construct a partial after universe.
func TestCheckStagedUnmergedIndex(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.StageUnmerged(t, repo, "conflict.md")
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); !errors.Is(err, awfgit.ErrIndexUnmerged) {
		t.Fatalf("CheckStaged unmerged index: got %v, want ErrIndexUnmerged", err)
	}
}

// TestCheckStagedNoHead covers the unborn-HEAD before side: a repository with no
// commit yet stages a complete covered universe.
func TestCheckStagedNoHead(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files["internal/foo/x.go"] = "package foo\n"
	gitfixture.Stage(t, repo, files)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if len(currentStateFindings(report)) != 0 {
		t.Fatalf("findings = %#v; want none (covered universe, bootstrap add)", currentStateFindings(report))
	}
}

// TestCheckStagedNoStagedConfig covers the missing index config: the working tree
// carries a config so Open succeeds, but it is never staged.
func TestCheckStagedNoStagedConfig(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	gitfixture.Stage(t, repo, map[string]string{"internal/x.go": "package x\n"})
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err == nil || !strings.Contains(err.Error(), "no staged") {
		t.Fatalf("CheckStaged err = %v; want a no-staged-config error", err)
	}
}

// TestCheckStagedRequiresStagedLock proves an adopted staged universe cannot
// silently fall back to cutoff zero when its lock is deleted. The same staged
// slice also deletes a governed current-state-v1 ADR, which cutoff zero would
// misroute as legacy and fail to diagnose.
func TestCheckStagedRequiresStagedLock(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files["docs/decisions/0002-v1.md"] = "---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-07-20\n---\n" +
		"# ADR-0002: V1\n\n## Context\n\nContext.\n\n## Decision\n\n1. Decide.\n\n" +
		"## State changes\n\nNone.\n\n## Consequences\n\nConsequence.\n\n" +
		"## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-20: Proposed\n"
	lockBytes, err := attestedLock().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	files[".awf/awf.lock"] = string(lockBytes)
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.StageRemoval(t, repo, ".awf/awf.lock", "docs/decisions/0002-v1.md")
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err == nil || !strings.Contains(err.Error(), "no staged .awf/awf.lock") {
		t.Fatalf("CheckStaged err = %v; want required staged-lock diagnostic", err)
	}
}

// TestCheckStagedLockError covers the lock-read failure in the staged check.
func TestCheckStagedLockError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": "{not json"})
	if _, err := checkStagedProject(p, testContext(t)); err == nil {
		t.Fatal("expected a lock parse error")
	}
}

func TestCheckStagedRejectsCorruptHeadLock(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = "{not json"
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	if _, err := checkStagedProject(p, testContext(t)); err == nil || !strings.Contains(err.Error(), "snapshot lock") {
		t.Fatalf("corrupt HEAD lock error = %v", err)
	}
}

// TestCheckStagedHeadLoadError covers a load failure on the HEAD (before) side: a
// committed ADR whose frontmatter does not parse.
func TestCheckStagedHeadLoadError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files["docs/decisions/0001-first.md"] = "---\nstatus: [unterminated\n---\n# X\n"
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err == nil {
		t.Fatal("expected a HEAD-side corpus load error")
	}
}

func TestCheckStagedIndexConfigValidationError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: \"\"\n"})
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/config.yaml"), csYAML)
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err == nil || !strings.Contains(err.Error(), "prefix") {
		t.Fatalf("validation error = %v", err)
	}
}

// TestCheckStagedIndexLoadError covers a load failure on the index (after) side:
// HEAD is clean, but a malformed ADR is staged.
func TestCheckStagedIndexLoadError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0002-bad.md": "---\nstatus: [unterminated\n---\n# X\n"})
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err == nil {
		t.Fatal("expected an index-side corpus load error")
	}
}

// TestCheckStagedOutsideRepo covers the before-side HEAD probe failing: a
// scaffolded project that is not a git repository.
func TestCheckStagedOutsideRepo(t *testing.T) {
	t.Parallel()
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkStagedProject(p, testContext(t)); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}

// TestCheckStagedHeadConfigParseError covers loadTreeCurrentState's config parse
// failure: the committed HEAD config is malformed while the working tree carries
// a valid one, so Open succeeds but the HEAD universe cannot load.
func TestCheckStagedHeadConfigParseError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: example\nprofile: full\nintegrationBranch: main\n"})
	gitfixture.Commit(t, repo, "head", nil)
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err == nil {
		t.Fatal("expected a HEAD-side config parse error")
	}
}

// TestCheckStagedIgnoresWorkingTree proves the staged check reads the index and
// HEAD, never the working tree: a garbage working-tree topic part that would fail
// to parse leaves the staged result clean.
func TestCheckStagedIgnoresWorkingTree(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	// Corrupt the topic part on disk only; the index and HEAD keep the valid one.
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/topics/parts/alpha/one/current-state.md"), "garbage, no Claims section\n")

	report, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged must ignore the dirty working tree, got: %v", err)
	}
	if len(report.Static) != 0 || len(currentStateFindings(report)) != 0 {
		t.Fatalf("expected a clean staged result despite the dirty working tree, got static=%#v findings=%#v", report.Static, currentStateFindings(report))
	}
}
