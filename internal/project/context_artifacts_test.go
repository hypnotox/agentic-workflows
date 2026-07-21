package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// invariant: tooling/cli:context-known-artifact-navigation
// invariant: rendering/project-output-plan:managed-output-attribution
func TestArtifactRecordsFollowDeclarations(t *testing.T) {
	decls := []OutputDeclaration{{Path: "docs/out.md", TemplateID: "docs/out.md.tmpl", Declarers: []string{"out"}, Inputs: []OutputInput{{Path: ".awf/docs/parts/out/content.md", Role: ArtifactConventionPart}}}}
	generated := artifactRecords("docs/out.md", decls, "docs/decisions")
	if len(generated) != 1 || generated[0].Role != ArtifactManagedOutput || len(generated[0].Sources) != 1 {
		t.Fatalf("generated=%#v", generated)
	}
	source := artifactRecords(".awf/docs/parts/out/content.md", decls, "docs/decisions")
	if len(source) != 1 || source[0].Role != ArtifactConventionPart || len(source[0].Outputs) != 1 || source[0].Outputs[0].Path != "docs/out.md" {
		t.Fatalf("source=%#v", source)
	}
	unmanaged := artifactRecords("docs/lookalike.md", decls, "docs/decisions")
	if unmanaged == nil || len(unmanaged) != 0 {
		t.Fatalf("unmanaged=%#v", unmanaged)
	}
	tree, _ := snapshot.NewTree([]snapshot.File{{Path: "docs/out.md", Mode: snapshot.Regular, Bytes: []byte("current")}})
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"docs/out.md": {OutputHash: manifest.Hash([]byte("old"))}}}
	applyArtifactSnapshots(generated, "docs/out.md", tree, lock)
	if generated[0].Snapshot == nil || !generated[0].Snapshot.InManifest || !generated[0].Snapshot.Drifted {
		t.Fatalf("snapshot=%#v", generated[0].Snapshot)
	}
	if _, err := json.Marshal(source); err != nil {
		t.Fatal(err)
	}
}

func TestProjectTreeReaders(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "a.txt", Mode: snapshot.Regular, Bytes: []byte("a")}, {Path: "link", Mode: snapshot.Symlink, Bytes: []byte("a.txt")}})
	if err != nil {
		t.Fatal(err)
	}
	r := snapshotTreeReader{tree: tree}
	b, ok := r.ReadFile("a.txt")
	if !ok || string(b) != "a" {
		t.Fatal("snapshot read")
	}
	b[0] = 'X'
	again, _ := r.ReadFile("a.txt")
	if string(again) != "a" {
		t.Fatal("snapshot alias")
	}
	if _, ok := r.ReadFile("link"); ok {
		t.Fatal("scanned symlink")
	}
	if got := r.Paths(""); !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Fatalf("paths=%v", got)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := filesystemProjectReader{root: root}
	if b, ok := fr.ReadFile("a.txt"); !ok || string(b) != "a" {
		t.Fatal("filesystem read")
	}
	if _, ok := fr.ReadFile("missing"); ok {
		t.Fatal("missing read")
	}
	if got := fr.Paths(""); !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Fatalf("filesystem paths=%v", got)
	}
	if got := fr.Paths("missing"); len(got) != 0 {
		t.Fatalf("missing paths=%v", got)
	}
}

func TestBuildOutputDeclarationsFamiliesAndReservations(t *testing.T) {
	read := memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x"), ".awf/topics/metadata/d/readme.txt": []byte("x"), ".awf/skills/local.yaml": []byte("local: true\n"), ".awf/skills/parts/local/content.md": []byte("part"), ".awf/agents/agent.yaml": []byte("local: true\n"), ".awf/agents/parts/agent/content.md": []byte("part")}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndocsDir: docs\nskills: [local]\nagents: [agent]\ndocs: [enabled]\ndomains: [d]\nrunner: {enabled: true}\nbootstrap: {enabled: true}\nhooks: {enabled: true}\n"), configReaderAdapter{read})
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{"local": {Base: true, Sections: []string{"content"}}}, Agents: map[string]catalog.AgentSpec{"agent": {Base: true, Sections: []string{"content"}}}, Docs: map[string]catalog.DocEntry{"agents-doc": {Mandatory: true, AgentsDoc: true, TID: "agents-doc/AGENTS.md.tmpl"}, "architecture": {Mandatory: true, Path: "architecture.md", TID: "docs/architecture.md.tmpl"}, "disabled": {Path: "disabled.md", TID: "docs/disabled.md.tmpl"}, "enabled": {Path: "enabled.md", TID: "docs/enabled.md.tmpl"}}}
	target := Target{Name: "one", SkillDir: ".one/skills", Outputs: []TargetOutput{{Path: "shared", TemplateID: "target.tmpl"}}}
	other := target
	other.Name = "two"
	decls, err := BuildOutputDeclarations(cfg, cat, []Target{target, other}, read)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]OutputDeclaration{}
	for _, d := range decls {
		byPath[d.Path] = d
	}
	for _, p := range []string{".one/skills/p-local/SKILL.md", "shared", "AGENTS.md", "docs/domains/d.md", "docs/topics/d/t.md", "docs/decisions/INDEX.md", "x", ".awf/bootstrap.sh", ".awf/upgrade.sh", ".awf/hooks/pre-commit.sh", ".awf/memory/.gitignore"} {
		if _, ok := byPath[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
	for _, tc := range []struct {
		read memoryProjectReader
		body string
	}{{memoryProjectReader{".awf/agents-doc.yaml": []byte("local: [bad")}, "prefix: p\n"}, {memoryProjectReader{".awf/docs/enabled.yaml": []byte("local: [bad")}, "prefix: p\ndocs: [enabled]\n"}} {
		badCfg, _ := config.ParseTree(".awf", []byte(tc.body), configReaderAdapter{tc.read})
		if _, err := BuildOutputDeclarations(badCfg, cat, []Target{target}, tc.read); err == nil {
			t.Fatal("malformed document declaration accepted")
		}
	}
	badRead := memoryProjectReader{".awf/agents/bad.yaml": []byte("local: [bad")}
	badCfg, _ := config.ParseTree(".awf", []byte("prefix: p\nagents: [bad]\n"), configReaderAdapter{badRead})
	if _, err := BuildOutputDeclarations(badCfg, cat, []Target{target}, badRead); err == nil {
		t.Fatal("malformed agent declaration accepted")
	}
	if !byPath[".one/skills/p-local/SKILL.md"].Reservation || !reflect.DeepEqual(byPath["shared"].Declarers, []string{"one", "two"}) {
		t.Fatalf("declarations=%#v", decls)
	}
}

func TestDeclarationPlanParityDiagnostics(t *testing.T) {
	if err := validateDeclarationPlanParity([]OutputNode{{Path: "a"}}, nil); err == nil {
		t.Fatal("length mismatch accepted")
	}
	if err := validateDeclarationPlanParity(nil, []OutputDeclaration{{Path: "a"}}); err == nil {
		t.Fatal("reverse length mismatch accepted")
	}
	if err := validateDeclarationPlanParity([]OutputNode{{Path: "a"}}, []OutputDeclaration{{Path: "b"}}); err == nil {
		t.Fatal("path mismatch accepted")
	}
	if err := validateDeclarationPlanParity([]OutputNode{{Path: "a", Reservation: true}}, []OutputDeclaration{{Path: "a"}}); err == nil {
		t.Fatal("reservation mismatch accepted")
	}
	if got := difference([]string{"a", "b"}, []string{"b"}); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("difference=%v", got)
	}
}

type memoryProjectReader map[string][]byte

func (r memoryProjectReader) ReadFile(p string) ([]byte, bool) {
	b, ok := r[p]
	return append([]byte(nil), b...), ok
}
func (r memoryProjectReader) Paths(prefix string) []string {
	out := []string{}
	for p := range r {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

type configReaderAdapter struct{ memoryProjectReader }

func (r configReaderAdapter) ReadFile(p string) ([]byte, bool) {
	return r.memoryProjectReader.ReadFile(".awf/" + p)
}
func (r configReaderAdapter) Paths(prefix string) []string { return nil }
