package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

func TestPiWorkflowRouterExactOutputSet(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [exploring, tdd]\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]RenderedFile{}
	for _, file := range files {
		if strings.HasPrefix(file.Path, ".pi/skills/") || strings.HasPrefix(file.Path, ".pi/awf-workflows/") {
			got[file.Path] = file
		}
	}
	want := []string{
		".pi/awf-workflows/exploring.md",
		".pi/awf-workflows/tdd.md",
		".pi/skills/awf-workflow/SKILL.md",
	}
	if len(got) != len(want) {
		t.Fatalf("Pi workflow output set = %v, want %v", mapKeys(got), want)
	}
	for _, path := range want {
		file, ok := got[path]
		if !ok {
			t.Errorf("missing Pi workflow output %s", path)
			continue
		}
		if strings.Count(file.Content, "<!-- "+bannerText+" -->") != 1 {
			t.Errorf("%s lacks exact managed provenance", path)
		}
	}
	router := got[".pi/skills/awf-workflow/SKILL.md"].Content
	for _, wantText := range []string{"name: awf-workflow", "`exploring`", "`tdd`", "Call `awf_workflow` alone", "Never pass a path"} {
		if !strings.Contains(router, wantText) {
			t.Errorf("router missing %q:\n%s", wantText, router)
		}
	}
	for path := range got {
		if strings.Contains(path, "example-exploring") || strings.Contains(path, "example-tdd") {
			t.Errorf("old discoverable governed skill survived: %s", path)
		}
	}
	dashboard := renderedByPath(t, files, ".pi/extensions/awf-dashboard/index.ts")
	for _, wantText := range []string{`"exploring":{"kind":"support","phaseEffect":"current","activity":"exploration","requiresPhases":[]}`, `"tdd":{"kind":"support","phaseEffect":"current","activity":"tdd","requiresPhases":["implementation"]}`, `StringEnum(workflowSkillNames as any)`, `name: "awf_workflow"`, `workflowToolPreflight`, `workflowBodyPath(deps.extensionFile, skill)`, `settleWorkflowIdentity(skill, ctx)`, `workflow request canceled after durable acknowledgment`, `governed workflow body is missing after lifecycle acknowledgment`, `pi.on("agent_end"`} {
		if !strings.Contains(dashboard, wantText) {
			t.Errorf("dashboard workflow loader missing rendered contract %q", wantText)
		}
	}
	if strings.Contains(dashboard, `"brainstorming":{"kind":"chain"`) {
		t.Error("dashboard workflow loader advertised a disabled mapping")
	}
}

// invariant: rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router
func TestPiWorkflowRouterBehavioralProof(t *testing.T) {
	provePiContractBehavior(t,
		"workflow router enforces mapped lifecycle effects",
		"workflow router settles provisional identity before loading",
		"workflow router rejects competing frontiers",
		"workflow router acknowledges durably before returning fixed bodies",
		"workflow router completes retrospective mechanically",
	)
}

func TestPiWorkflowRouterErrorPropagation(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [exploring]\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.workflowRouterData([]string{"missing"}); err == nil {
		t.Fatal("router data accepted a stale workflow")
	}
	if _, err := p.workflowLoaderData([]string{"missing"}); err == nil {
		t.Fatal("loader data accepted a stale workflow")
	}
	sidecar := filepath.Join(root, ".awf", "skills", "exploring.yaml")
	if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sidecar, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.routedWorkflowNames(); err == nil {
		t.Fatal("routed workflow names hid a sidecar read failure")
	}
	if _, err := p.artifactConfigHash("{{ .workflowSkills }}", config.Sidecar{}, nil); err == nil {
		t.Fatal("workflow config hash hid a routed sidecar read failure")
	}

	brokenRoot := scaffoldFiles(t, "prefix: example\nskills: [exploring]\nagents: []\ntargets: [pi]\n", map[string]string{
		"skills/parts/exploring/notes.md": "valid notes\n",
	})
	broken, err := Open(brokenRoot)
	if err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(brokenRoot, ".awf", "skills", "parts", "exploring", "notes.md")
	if err := os.WriteFile(part, []byte("<!-- awf:comment malformed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := broken.RenderAll(); err == nil {
		t.Fatal("hidden workflow render hid a malformed part")
	}
}

func TestPiWorkflowRouterEmptyDataUsesMissingKeyZero(t *testing.T) {
	out := renderGolden(t, "pi/awf-workflow/SKILL.md.tmpl", map[string]any{})
	if strings.Contains(out, "<no value>") || !strings.Contains(out, "No governed awf workflows are enabled") {
		t.Fatalf("empty-data router is not coherent:\n%s", out)
	}
}

func TestPiWorkflowOutputsAreLockOwnedDriftCheckedAndReferenceScanned(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [exploring]\nagents: []\ntargets: [pi]\n", map[string]string{
		"skills/parts/exploring/notes.md": "See `example-tdd` before continuing.\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".pi/skills/awf-workflow/SKILL.md", ".pi/awf-workflows/exploring.md"} {
		entry, ok := lock.Files[path]
		if !ok || entry.TemplateHash == "" || entry.ConfigHash == "" || entry.OutputHash == "" {
			t.Errorf("lock does not own complete hashes for %s: %#v", path, entry)
		}
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	foundReference := false
	for _, item := range drift {
		if item.Path == ".pi/awf-workflows/exploring.md" && item.Kind == "dead-skill-reference" && item.Detail == "example-tdd" {
			foundReference = true
		}
	}
	if !foundReference {
		t.Fatalf("hidden workflow dead-skill reference was not validated: %#v", drift)
	}
}

func TestPiWorkflowSyncPrunesOldDiscoverableOutputsAndTargetDisableCleansAll(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [exploring]\nagents: []\ntargets: [claude]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(root, ".claude", "skills", "example-exploring", "SKILL.md")
	if _, err := os.Stat(old); err != nil {
		t.Fatal(err)
	}
	// Rename the prior managed target directory to the legacy Pi path and its lock
	// key by first syncing a Pi project with an unmapped local reservation would not
	// exercise pruning. Instead, the prior Claude output proves the same stale-lock
	// ownership path while the explicit legacy Pi file below is made lock-owned by
	// moving its lock entry through the serialized manifest.
	legacy := filepath.Join(root, ".pi", "skills", "example-exploring", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, body, 0o644); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, ".awf", "awf.lock")
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.Replace(string(raw), `".claude/skills/example-exploring/SKILL.md"`, `".pi/skills/example-exploring/SKILL.md"`, 1)
	if rewritten == string(raw) {
		t.Fatal("fixture lock did not contain the old discoverable output")
	}
	if err := os.WriteFile(lockPath, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(root, ".awf", "config.yaml")
	if err := os.WriteFile(cfg, []byte("prefix: example\nskills: [exploring]\nagents: []\ntargets: [pi]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy discoverable output survived sync pruning: %v", err)
	}
	for _, rel := range []string{".pi/skills/awf-workflow/SKILL.md", ".pi/awf-workflows/exploring.md"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("new Pi output %s missing: %v", rel, err)
		}
	}
	if err := os.WriteFile(cfg, []byte("prefix: example\nskills: [exploring]\nagents: []\ntargets: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".pi/skills/awf-workflow/SKILL.md", ".pi/awf-workflows/exploring.md"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Errorf("disabled Pi output survived cleanup %s: %v", rel, err)
		}
	}
}

func TestNonPiWorkflowRenderingParity(t *testing.T) {
	const governed = "skills: [adr-lifecycle, brainstorming, bugfix, debugging, executing-direct, executing-plans, exploring, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, tdd, writing-plans]\nagents: [adr-reviewer, code-reviewer, plan-reviewer]\n"
	for _, target := range []string{"claude", "codex", "copilot", "cursor", "gemini"} {
		root := scaffold(t, "prefix: example\n"+governed+"targets: ["+target+"]\n")
		p, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		files, err := p.RenderAll()
		if err != nil {
			t.Fatal(err)
		}
		want := p.Targets[0].SkillPath("example", "executing-direct")
		body := renderedByPath(t, files, want)
		if body == "" {
			t.Errorf("%s lost ordinary workflow skill %s", target, want)
		}
		if !strings.Contains(body, "Invoke `example-reviewing-impl` as the terminal step.") || strings.Contains(body, "awf_workflow") {
			t.Errorf("%s lost target-native successor guidance:\n%s", target, body)
		}
		for _, file := range files {
			if file.Path == ".pi/skills/awf-workflow/SKILL.md" || strings.HasPrefix(file.Path, ".pi/awf-workflows/") {
				t.Errorf("%s rendered Pi workflow output %s", target, file.Path)
			}
		}
	}
}

func TestPiHiddenWorkflowBodiesUseRouterSuccessors(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [adr-lifecycle, brainstorming, bugfix, debugging, executing-direct, executing-plans, exploring, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, tdd, writing-plans]\nagents: [adr-reviewer, code-reviewer, plan-reviewer]\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Path, ".pi/awf-workflows/") && (strings.Contains(strings.ToLower(file.Content), "invoke `example-") || strings.Contains(strings.ToLower(file.Content), "invoke `awf-")) {
			t.Errorf("%s retained a bypassing governed skill invocation:\n%s", file.Path, file.Content)
		}
	}
	for path, successor := range map[string]string{
		".pi/awf-workflows/executing-direct.md": `skill: "reviewing-impl"`,
		".pi/awf-workflows/reviewing-impl.md":   `skill: "retrospective"`,
	} {
		body := renderedByPath(t, files, path)
		if !strings.Contains(body, "Call `awf_workflow` alone") && !strings.Contains(body, "call `awf_workflow` alone") {
			t.Errorf("%s lacks router successor mechanism:\n%s", path, body)
		}
		if !strings.Contains(body, successor) {
			t.Errorf("%s lacks successor %s:\n%s", path, successor, body)
		}
	}
}

func TestPiWorkflowRouterPolicyIsMarkdownManaged(t *testing.T) {
	if got := declaredPolicy("skills", false); got != (OutputPolicy{ValidateFrontmatter: true, ScanReferences: true, ScanSkillReferences: true}) {
		t.Fatalf("skill policy = %#v", got)
	}
	if piTarget.agentCommentStyle() != render.HTMLComment {
		t.Fatal("Pi workflow provenance is not Markdown")
	}
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
