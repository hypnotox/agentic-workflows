package project

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const governanceFootprintContract = "Core and Full select available governance artifacts. They do not select different standards of correctness, autonomy, maintainability, or review quality. Core includes the operational workflow. Full adds ADR, plan, current-state, context, and audit capabilities."

// invariant: rendering/workflow-skill-templates:closed-workflow-profiles (TestGovernanceFootprintsShareQualityBar)
func TestGovernanceFootprintsShareQualityBar(t *testing.T) {
	for _, profile := range []catalog.Profile{catalog.ProfileCore, catalog.ProfileFull} {
		t.Run(string(profile), func(t *testing.T) {
			files := explorationRenderedByPath(t, "prefix: example\nprofile: "+string(profile)+"\nintegrationBranch: main\n")
			for path, wants := range map[string][]string{
				"docs/workflow.md": {governanceFootprintContract, "required verification strength"},
				"AGENTS.md":        {"workflow governs a change's protected contract", "clean-integration operative rule"},
			} {
				for _, want := range wants {
					if !strings.Contains(files[path], want) {
						t.Errorf("%s missing shared governance-footprint contract %q", path, want)
					}
				}
			}
			for _, target := range []string{"pi", "claude"} {
				for path, wants := range map[string][]string{
					"." + target + "/skills/example-executing-direct/SKILL.md": {"Resolve implementation findings autonomously", "current and target owner", "residual debt"},
					"." + target + "/agents/implementer.md":                    {"current and target owner", "residual debt"},
					"." + target + "/agents/code-reviewer.md":                  {"concrete maintainability risk", "current and target owner", "residual debt"},
				} {
					for _, want := range wants {
						if !strings.Contains(files[path], want) {
							t.Errorf("%s missing shared quality doctrine %q", path, want)
						}
					}
				}
			}
		})
	}
}

func coreOperationalResidual(files []RenderedFile, forbidden []string) []string {
	var residual []string
	for _, file := range files {
		content := strings.ReplaceAll(file.Content, governanceFootprintContract, "")
		for _, phrase := range forbidden {
			if strings.Contains(content, phrase) {
				residual = append(residual, file.Path+": "+phrase)
			}
		}
	}
	return residual
}

// TestCoreRenderedWorkflowExcludesFullAuthority expands every selected artifact
// for both targets. It deliberately scans operational references instead of
// ordinary prose such as an adopter discussing its own historical documents.
// invariant: rendering/catalog-and-targets:profile-dependency-closure (TestCoreRenderedWorkflowExcludesFullAuthority)
// invariant: rendering/workflow-skill-templates:closed-workflow-profiles (TestCoreRenderedWorkflowExcludesFullAuthority)
func TestCoreRenderedWorkflowExcludesFullAuthority(t *testing.T) {
	// These are bound operational spellings, not authority words in ordinary
	// adopter prose. Each entry names an executable command, artifact identity,
	// reviewer schema, or workflow instruction that Core must not expose.
	forbiddenCoreOperationalReferences := []string{
		"`./awf context", "`./awf adr", "`./awf plan", "`./awf audit",
		"example-proposing-adr", "example-writing-plans", "example-reviewing-plan",
		"example-executing-plans", "example-subagent-driven-development", "example-adr-lifecycle",
		"adr-reviewer", "plan-reviewer",
		"Before ADR or plan authoring", "effort-free ADR evidence",
		"durable ADR, plan", "completes deferred artifact transitions",
		"agent guide, ADRs", "ADR, plan, or code reviewer",
		"kind adr, plan, or code", `StringEnum(["adr", "plan", "code"]`,
		"ADR", "current-state", "State changes", "governance workflow",
		"plan authoring", "plan review", "plan is warranted", "creating an ADR or plan",
		"stale-ADR", "plan-adherence", "plan adherence", "plan's stated file paths", "Read every plan, ADR, or state doc",
		"`awf new adr`", "`awf audit`", "decisions or plans directory", "Current-state topic ownership",
		"decisions, plans, domains, topics", "domain sidecars", "`domains` (optional)", "optional `domains`, `tags`",
	}
	for _, tc := range []struct {
		name    string
		profile catalog.Profile
	}{
		{name: "core", profile: catalog.ProfileCore},
		{name: "full", profile: catalog.ProfileFull},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configYAML := "prefix: example\nprofile: " + string(tc.profile) + "\nintegrationBranch: main\n"
			if tc.profile == catalog.ProfileCore {
				// Fresh init enables the bootstrap pair. Sync, rather than RenderAll,
				// also includes generated outputs such as config-reference.md.
				configYAML += "bootstrap:\n  enabled: true\n"
			}
			root := scaffold(t, configYAML)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			var files []RenderedFile
			if tc.profile == catalog.ProfileCore {
				if err := p.Sync(); err != nil {
					t.Fatal(err)
				}
				lock, err := manifest.Load(lockFile(root))
				if err != nil {
					t.Fatal(err)
				}
				for path := range lock.Files {
					body, err := os.ReadFile(filepath.Join(root, path))
					if err != nil {
						t.Fatalf("read complete Core output %s: %v", path, err)
					}
					files = append(files, RenderedFile{Path: path, Content: string(body)})
				}
			} else {
				files, err = p.RenderAll()
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(files) == 0 {
				t.Fatal("selected governance footprint rendered no artifacts")
			}

			var residual []string
			if tc.profile == catalog.ProfileCore {
				residual = coreOperationalResidual(files, forbiddenCoreOperationalReferences)
			}
			// The non-empty rendered population is the checked success sentinel:
			// only then does an empty residual set prove semantic absence.
			if tc.profile == catalog.ProfileCore && len(residual) != 0 {
				t.Fatalf("Core retained Full-only operational references: %v", residual)
			}

			if tc.profile == catalog.ProfileFull {
				foundContext, foundADR := false, false
				for _, file := range files {
					foundContext = foundContext || strings.Contains(file.Content, "./awf context")
					foundADR = foundADR || strings.Contains(file.Content, "-proposing-adr")
				}
				if !foundContext || !foundADR {
					t.Fatalf("Full output lost governance semantics: context=%v ADR=%v", foundContext, foundADR)
				}
			}
		})
	}
}

func TestCoreOperationalReferenceScannerRejectsEveryArtifactClass(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: core\nintegrationBranch: main\nbootstrap:\n  enabled: true\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	var files []RenderedFile
	for path := range lock.Files {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, RenderedFile{Path: path, Content: string(body)})
	}
	mutations := map[string]string{
		"docs/workflow.md":                               "ADR",
		".pi/agents/code-reviewer.md":                    "current-state",
		".claude/skills/example-reviewing-impl/SKILL.md": "State changes",
		".awf/hooks/commit-msg.sh":                       "governance workflow",
		".pi/extensions/awf-subagents/index.ts":          "kind adr, plan, or code",
		".awf/upgrade.sh":                                "current-state cutover",
		"docs/config-reference.md":                       "`./awf audit",
	}
	for path, mutation := range mutations {
		found := false
		for _, file := range files {
			if file.Path != path {
				continue
			}
			found = true
			file.Content += "\n" + mutation + "\n"
			if got := coreOperationalResidual([]RenderedFile{file}, []string{mutation}); len(got) != 1 {
				t.Errorf("scanner accepted injected Full reference %q in rendered %s: %v", mutation, path, got)
			}
		}
		if !found {
			t.Errorf("rendered mutation target %s missing", path)
		}
	}
}

// TestProfileTransitionPreservesHistoryAndRestoresGovernance exercises the
// working-tree Full -> Core -> Full path, including each native target.
// invariant: rendering/project-output-plan:profile-projected-render (TestProfileTransitionPreservesHistoryAndRestoresGovernance)
func TestProfileTransitionPreservesHistoryAndRestoresGovernance(t *testing.T) {
	const coreConfig = "prefix: example\nprofile: core\nintegrationBranch: main\nvars:\n  gateCmd: test-gate\n"
	cleanCoreRoot := scaffold(t, coreConfig)
	cleanCore, err := Open(testContext(t), cleanCoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanCore.Sync(); err != nil {
		t.Fatal(err)
	}
	cleanCoreLock, err := manifest.Load(lockFile(cleanCoreRoot))
	if err != nil {
		t.Fatal(err)
	}

	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{
		"parts/adr-template/body.md": "Full-only ADR template override.\n",
	})
	historical := map[string]string{
		"docs/decisions/0001-history.md":   "---\nstatus: Implemented\n---\n# ADR-0001: Authored history\n",
		"docs/plans/2026-01-01-history.md": "---\nformat: plan-v2\ndate: 2026-01-01\nstatus: Implemented\n---\n# Plan: Authored history\n",
	}
	for path, content := range historical {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	full, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := full.Sync(); err != nil {
		t.Fatal(err)
	}
	fullLock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	fullMembership := maps.Clone(fullLock.Files)
	for _, path := range []string{
		".claude/skills/example-proposing-adr/SKILL.md",
		".pi/skills/example-proposing-adr/SKILL.md",
		"docs/decisions/INDEX.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("Full output %s missing: %v", path, err)
		}
	}
	// Retained history need not remain parseable while Core is selected.
	historical["docs/decisions/0001-history.md"] = "---\nformat: [broken\n"
	historical["docs/plans/2026-01-01-history.md"] = "---\nformat: [broken\n"
	for path, content := range historical {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Core must receive no Full-only source. The deleted part is representative
	// of the governance-footprint-owned .awf sources that must not become dormant layers.
	testsupport.WriteAwfConfig(t, root, coreConfig)
	if err := os.Remove(filepath.Join(root, ".awf", "parts", "adr-template", "body.md")); err != nil {
		t.Fatal(err)
	}
	core, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Sync(); err != nil {
		t.Fatal(err)
	}
	coreLock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(coreLock.Files) != len(cleanCoreLock.Files) {
		t.Fatalf("transitioned Core lock differs from clean Core: transitioned=%d clean=%d", len(coreLock.Files), len(cleanCoreLock.Files))
	}
	for path := range cleanCoreLock.Files {
		if _, ok := coreLock.Files[path]; !ok {
			t.Errorf("transitioned Core lock omitted clean Core member %s", path)
		}
	}
	for path := range coreLock.Files {
		if _, ok := cleanCoreLock.Files[path]; !ok {
			t.Errorf("transitioned Core lock retained non-Core member %s", path)
		}
	}
	for path, want := range historical {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != want {
			t.Fatalf("Core transition altered authored history %s: got %q err=%v", path, got, err)
		}
	}
	for _, path := range []string{
		".claude/skills/example-proposing-adr/SKILL.md",
		".pi/skills/example-proposing-adr/SKILL.md",
		"docs/decisions/INDEX.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("Core transition retained managed Full output %s: %v", path, err)
		}
	}
	for _, dir := range []string{
		".claude/skills/example-proposing-adr",
		".pi/skills/example-proposing-adr",
	} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Fatalf("Core transition retained empty Full-only ancestor %s: %v", dir, err)
		}
	}
	if _, err := core.CheckReport(testContext(t)); err != nil {
		t.Fatalf("Core consulted malformed retained history: %v", err)
	}

	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: test-gate\n")
	full, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := full.Sync(); err == nil {
		t.Fatal("Full accepted malformed retained history")
	}
	for path := range historical {
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := full.Sync(); err != nil {
		t.Fatalf("Full did not restore governance after history correction: %v", err)
	}
	restoredLock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(restoredLock.Files) != len(fullMembership) {
		t.Fatalf("Full lock membership size was not restored: restored=%d initial=%d", len(restoredLock.Files), len(fullMembership))
	}
	for path := range fullMembership {
		if _, ok := restoredLock.Files[path]; !ok {
			t.Errorf("Full lock membership did not restore %s", path)
		}
	}
	for _, path := range []string{
		".claude/skills/example-proposing-adr/SKILL.md",
		".pi/skills/example-proposing-adr/SKILL.md",
		"docs/decisions/INDEX.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("Full restoration omitted %s: %v", path, err)
		}
	}
}
