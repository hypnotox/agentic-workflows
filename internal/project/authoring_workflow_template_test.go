package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: rendering/workflow-skill-templates:using-awf-transaction-home (TestUsingAwfTemplate)
func TestUsingAwfTemplate(t *testing.T) {
	out := renderSkillGolden(t, "using-awf", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
	})
	want := strings.Join([]string{
		"---",
		"name: example-using-awf",
		"description: \"Load when beginning a generated-tree edit, render, drift repair, or upgrade. Do not load while only investigating or planning possible generated-tree work.\"",
		"---",
		"",
		"<!-- awf:edit procedure: default; create  to override -->",
		"# example-using-awf",
		"",
		"`.awf/` is the source. Never hand-edit rendered outputs, except a declared local document's body after its `awf:edit-in-place` pointer through end-of-file; awf owns every other byte.",
		"",
		"Edit source or that narrow local body, render, check, then stage the source, rendered outputs, and `.awf/awf.lock` together; then run the gate. Removing a local declaration or uninstalling saves a present document as a sibling `.awf-bak` recovery file. A drift finding carries its own repair hint: follow it rather than guessing at the generated output.",
		"",
		"For an upgrade, run the bootstrap script and then perform the residue sweep. `docs/working-with-awf.md` owns detailed commands and generated-tree guidance; `docs/config-reference.md` owns configuration keys and their meanings.",
		"",
	}, "\n")
	if out != want {
		t.Errorf("using-awf must remain the approved thin transaction body:\n%s", out)
	}
}

// invariant: rendering/workflow-skill-templates:writing-docs-delegation (TestWritingDocsTemplate)
func TestWritingDocsTemplate(t *testing.T) {
	out := renderSkillGolden(t, "writing-docs", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
	})
	want := strings.Join([]string{
		"---",
		"name: example-writing-docs",
		"description: \"Load when beginning project-documentation authoring. Do not load merely because documentation may need a later update.\"",
		"---",
		"",
		"<!-- awf:edit procedure: default; create  to override -->",
		"# example-writing-docs",
		"",
		"Select the single document that owns the fact. Read `docs/doc-standard.md` before writing; when another surface owns the detail, reference it rather than restating it. When no standard document owns a repository-specific fact, declare a `localDocs` item with a name, title, and description; reserved roots are `decisions`, `plans`, `domains`, `topics`, and `pitfalls`. Let the document travel in the commit that makes the fact true.",
		"",
		"Author a local document only after its `awf:edit-in-place` pointer through end-of-file; awf owns its heading and shell. Run ordinary render and check after edits. Declaration removal or uninstall preserves a present body in a sibling `.awf-bak` recovery file.",
		"",
		"When authoring reaches a file edit, invoke `example-using-awf` for the generated-tree transaction. `docs/doc-standard.md` owns the documentation rules.",
		"",
	}, "\n")
	if out != want {
		t.Errorf("writing-docs must remain the approved thin delegation body:\n%s", out)
	}
}

func TestOrientingTemplate(t *testing.T) {
	out := renderSkillGolden(t, "orienting", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{},
	})
	if !strings.Contains(out, "name: example-orienting") {
		t.Errorf("expected 'name: example-orienting' in output:\n%s", out)
	}
	for _, want := range []string{"Load when beginning repository orientation", "Do not load for exact-known-file inspection", "Four moments call for orientation", "Ground guide-first:", "CodeGraph", "./awf resolve topic", "`example-exploring`", "A discrepancy resolves in favor of the repository"} {
		if !strings.Contains(out, want) {
			t.Errorf("orienting render missing %q:\n%s", want, out)
		}
	}
}

// invariant: rendering/workflow-skill-templates:orienting-single-home (TestOrientingSkillContract)
func TestOrientingSkillContract(t *testing.T) {
	// The same render proves that orienting is the single home and that all three
	// dependent skills reference it.
	config := func(target string) string {
		return "prefix: example\nprofile: full\nintegrationBranch: main\n"
	}
	for _, target := range KnownTargets() {
		t.Run(target, func(t *testing.T) {
			files := explorationRenderedByPath(t, config(target))
			adapter := map[string]Target{"claude": claudeTarget, "pi": piTarget}[target]
			body := files[adapter.SkillPath("example", "orienting")]
			if body == "" {
				t.Fatalf("missing rendered orienting skill for %s", target)
			}
			// One literal per property the skill contract promises: a heading
			// count alone would survive deleting the moments it counts.
			for _, want := range []string{
				"Four moments call for orientation",
				"**Fresh work:**", "**Effort resume:**", "**Handoff takeover:**", "**Mid-chain re-orientation:**",
				"Ground guide-first:", "CodeGraph for source discovery",
				"./awf resolve topic", "./awf read topic", "./awf read adr",
				"one or more exploration subagents",
				"one information need", "every child is report-only",
				"location is unknown", "and inline search would pollute the parent context",
				"exact-known-file", "genuinely trivial", "`example-exploring`",
				"landed since the checkpoint", "git worktree list", "against the decision index",
				"its decision log including every `Record:` block present", "not yours to re-decide",
				"cited plan and file existence", "A discrepancy resolves in favor of the repository",
				"never creates an effort, never commits", "exact-known-file",
				"single-pass and advisory, never a chain gate",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s orienting skill missing %q", target, want)
				}
			}
			agent := files[adapter.AgentPath("grounding-checker")]
			for _, want := range []string{"Ground guide-first:", "CodeGraph", "./awf resolve topic", "./awf read topic", "./awf read adr"} {
				if !strings.Contains(agent, want) {
					t.Errorf("%s grounding-checker missing %q", target, want)
				}
			}
			// The single home requires both surviving dependent skills to reference it.
			// Brainstorming evaluates continuity at entry, then invokes orientation.
			for _, consumer := range []string{"brainstorming", "proposing-adr"} {
				if ref := files[adapter.SkillPath("example", consumer)]; !strings.Contains(ref, "`example-orienting`") {
					t.Errorf("%s %s does not reference the orienting skill", target, consumer)
				}
			}
			if b := files[adapter.SkillPath("example", "brainstorming")]; !strings.Contains(b, "2. **Orient in the topic.** Invoke `example-orienting`") {
				t.Errorf("%s brainstorming does not invoke orienting after continuity evaluation", target)
			}
		})
	}
}

func TestProposingAdrTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"activeMdRegenCmd": "go test ./internal/adrtools/",
			"gateCmd":          "./x gate",
			"checkCmd":         "./x check",
		},
		"layout": map[string]any{
			"adrDir": "docs/decisions", "adrTemplate": "docs/decisions/template.md",
			"indexMd": "docs/decisions/INDEX.md", "adrReadme": "docs/decisions/README.md",
		},
		"data": map[string]any{
			"adrTriggers": []string{
				"new package boundary or top-level directory",
				"auth or security behaviour change",
				"non-trivial new dependency",
				"workflow rule change",
			},
			"adrSections": []string{
				"Context",
				"Decision",
				"Invariants",
				"Consequences",
				"Alternatives Considered",
			},
		},
	}

	out := renderSkillGolden(t, "proposing-adr", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-proposing-adr") {
		t.Errorf("expected 'name: example-proposing-adr' in output:\n%s", out)
	}

	// Assert the scaffold-first operations as one ordered procedure.
	procedure := "Run `./awf new adr \"<Title>\"` before any ADR-file mutation. Capture the exact path it creates. Read the exact file it creates, then edit that scaffold in place."
	if !strings.Contains(out, procedure) {
		t.Errorf("expected ordered procedure %q in output:\n%s", procedure, out)
	}

	// Assert load-bearing phrases unique to proposing-adr.
	loadBearing := []string{
		"one decision per ADR",
		"Never create or replace an ADR by any other mechanism",
		"Context",
		"Consequences",
		"status: Proposed",
		"example-reviewing-adr",
		"remains meaningful after implementation",
		"post-implementation",
		"counterfactual",
		"consent evidence establishes that it is load-bearing and the ADR explains why it is load-bearing",
		"preserve exactly the frontmatter emitted by `./awf new adr`",
		"Before any ADR-file mutation, identify the explicitly accepted decision set",
		"narrowest durable commitment",
		"outside the ADR until accepted",
		"effort-free",
		"approved design summary",
		"Decision log",
		"`Record:` evidence",
		"plan or direct execution",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Required frontmatter:") && strings.Contains(line, "current-state-v") {
			t.Errorf("proposing guidance chooses a literal current format in %q:\n%s", line, out)
		}
	}
}

func TestAdrLifecycleTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"activeMdRegenCmd": "go test ./internal/adrtools/",
			"gateCmd":          "./x gate",
		},
		"layout": map[string]any{
			"adrDir": "docs/decisions", "indexMd": "docs/decisions/INDEX.md",
			"adrReadme": "docs/decisions/README.md",
		},
		"data": map[string]any{
			"adrStates": []map[string]any{
				{
					"name":       "Proposed",
					"meaning":    "Under discussion; all sections mutable",
					"mutability": "Mutable; amendments encouraged",
				},
				{
					"name":       "Accepted",
					"meaning":    "Design final; implementation in progress",
					"mutability": "Append-only; only `status` editable in place",
				},
				{
					"name":       "Implemented",
					"meaning":    "Implementation complete; decision enacted",
					"mutability": "Append-only; only `status` editable in place",
				},
				{
					"name":       "Abandoned",
					"meaning":    "Will not be implemented; intended operations stay unapplied",
					"mutability": "Terminal; status and append-only Status history only",
				},
			},
		},
	}

	out := renderSkillGolden(t, "adr-lifecycle", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-adr-lifecycle") {
		t.Errorf("expected 'name: example-adr-lifecycle' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to adr-lifecycle
	loadBearing := []string{
		"State changes",
		"status transition",
		"regenerate",
		"Append-only",
		"every explicit Applied batch, including the final batch",
		"direct implicit completion with its matching claim mutations",
		"status-only terminal transaction after explicit application",
		"V4 Decision items begin with a unique inline `decision: <lowercase-kebab-slug>` marker",
		"canonical `#N` remains available only for frozen ADR navigation and is not current-authority or supersession syntax",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}

	observable := []string{"distinct claim IDs", "separately observable authored transaction", "exact prefix", "legal ordered lifecycle"}
	scaffold := renderGolden(t, "adr-template/template.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout(),
	})
	for name, body := range map[string]string{"generic ADR scaffold": scaffold, "generic lifecycle skill": out} {
		for _, phrase := range observable {
			if !strings.Contains(body, phrase) {
				t.Errorf("%s missing observable authored-transaction phrase %q:\n%s", name, phrase, body)
			}
		}
	}
	root := testsupport.RepoRoot(t)
	for _, rel := range []string{".claude/skills/awf-adr-lifecycle/SKILL.md", ".pi/skills/awf-adr-lifecycle/SKILL.md"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range observable {
			if !bytes.Contains(body, []byte(phrase)) {
				t.Errorf("%s missing observable authored-transaction phrase %q", rel, phrase)
			}
		}
	}
}

func TestBrainstormingTemplate(t *testing.T) {
	out := renderSkillGolden(t, "brainstorming", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout()})
	for _, want := range []string{"material choice or clarification", "approved decision set", "narrowest durable commitment", "outside the ADR until accepted"} {
		if !strings.Contains(out, want) {
			t.Fatalf("brainstorming contract missing %q", want)
		}
	}
}
