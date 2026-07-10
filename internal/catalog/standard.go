package catalog

// Standard is the compile-time catalog: awf's static description of the standard
// (skills, agents, docs, singletons, the domain-doc spec, and the fillable vars).
// It replaces the former embedded catalog.yaml runtime parse (ADR-0060). Default
// Data bags are kept as map[string]any / []any / scalars — the shapes yaml.v3
// produced — so the per-file ConfigHash stays byte-identical.
var Standard = &Catalog{
	Skills: map[string]SkillSpec{
		"brainstorming": {Core: true, Chain: true, RequiresSkills: []string{"proposing-adr", "reviewing-adr", "reviewing-impl", "writing-plans"}, Sections: []string{
			"preamble", "when-to-invoke", "procedure", "example-clarifying-questions",
			"design-sections", "no-spec-rule", "grounding-check-output-format",
			"grounding-check-dispatch-template", "terminal-step", "definitions", "anti-patterns",
		}},
		"writing-plans": {Core: true, Chain: true, RequiresSkills: []string{"adr-lifecycle", "proposing-adr", "reviewing-plan", "reviewing-plan-resync"}, Sections: []string{
			"positioning", "when-to-invoke", "conventions-path", "conventions-header",
			"conventions-tasks", "conventions-no-placeholders", "gate-tier-note",
			"conventions-test-first", "procedure-confirm-scope", "plan-template-ref",
			"procedure-write-plan", "doc-currency-check", "self-review", "plan-commit-step",
			"terminal-step", "plan-lifecycle", "plan-resync", "notes",
		}},
		"executing-plans": {Core: true, Chain: true, RequiresSkills: []string{"reviewing-impl", "subagent-driven-development"}, Sections: []string{
			"positioning", "when-to-invoke", "procedure-resolve-plan", "procedure-raise-concerns",
			"procedure-per-task", "tdd-opt-in", "gate-tier-detail", "procedure-adr-final-commit",
			"procedure-non-adr-final-commit", "terminal-step", "project-invariants", "notes-gate",
			"notes-auto-commit", "notes-one-concern", "notes-docs-travel", "red-flags",
		}},
		"subagent-driven-development": {Core: true, Chain: true, RequiresSkills: []string{"executing-plans", "reviewing-impl"}, Sections: []string{
			"positioning", "per-task-review-note", "when-to-invoke", "procedure-resolve-plan",
			"procedure-raise-concerns", "procedure-extract-context", "dispatch-conventions",
			"procedure-status-handling", "per-task-review", "final-task-adr-flip", "terminal-step",
			"notes", "red-flags",
		}},
		"tdd": {
			Sections: []string{"surfaces", "notes", "red-flags"},
			Data: map[string]any{
				"testSurfaces": []any{
					map[string]any{"name": "Unit", "kind": "fast isolated test", "location": "beside the code under test"},
					map[string]any{"name": "Integration", "kind": "cross-component test", "location": "the project's integration suite"},
					map[string]any{"name": "End-to-end", "kind": "full-system test", "location": "the project's e2e suite"},
				},
			},
		},
		"debugging": {Sections: []string{
			"symptom-list", "debugging-surfaces", "test-isolation", "oracle-invariant",
			"devdb-note", "red-flags", "memory-checkpoint",
		}},
		"proposing-adr": {
			Core: true, Chain: true,
			RequiresSkills: []string{"adr-lifecycle", "reviewing-adr"},
			Sections: []string{
				"positioning", "when-to-invoke", "conventions", "procedure-number", "procedure-write",
				"state-doc-update", "procedure-predecessor-flip", "invariants-rule", "procedure-regen",
				"procedure-commit", "autonomous-rule", "terminal-step", "notes",
			},
			Data: map[string]any{
				"adrSections": []any{"Context", "Decision", "Invariants", "Consequences", "Alternatives Considered"},
				"adrTriggers": []any{
					"Introducing or moving a module/package boundary",
					"Adopting a new external dependency",
					"Changing a persisted format (config, lock file, schema, API contract)",
					"Changing the development workflow's rules",
					"Any decision a future maintainer would need to know the \"why\" for",
				},
			},
		},
		"adr-lifecycle": {
			Core: true,
			Sections: []string{
				"states", "transitions", "supersedence-full", "supersedence-partial",
				"procedure-status-edit", "procedure-predecessor-flip", "state-doc-update",
				"procedure-regen", "procedure-gate", "commit-templates", "amendment-while-proposed", "notes",
			},
			Data: map[string]any{
				"adrStates": []any{
					map[string]any{"name": "Proposed", "meaning": "ADR is written and under review; content is freely mutable", "mutability": "Freely mutable — body and status may both change"},
					map[string]any{"name": "Accepted", "meaning": "Design is finalised; implementation authorised but not yet complete", "mutability": "Status field only — body is frozen"},
					map[string]any{"name": "Implemented", "meaning": "Design and implementation have both landed in the repository", "mutability": "Status field only — body is frozen"},
					map[string]any{"name": "Superseded", "meaning": "Replaced by a later ADR; kept for historical record", "mutability": "Status field only — body is frozen"},
				},
			},
		},
		"bugfix": {Sections: []string{"test-tiers", "pitfalls-check", "oracle-note", "memory-checkpoint"}},
		"reviewing-plan": {Core: true, Chain: true, RequiresAgent: "plan-reviewer", RequiresSkills: []string{"reviewing-plan-resync", "writing-plans"}, Sections: []string{
			"when-fires", "procedure", "artifact-path-detection", "dispatch-subagent",
			"classify-route-findings", "apply-fixes-commit", "re-review-loop", "hand-off", "notes",
		}},
		"reviewing-plan-resync": {Core: true, Chain: true, RequiresAgent: "plan-reviewer", RequiresSkills: []string{"executing-plans", "reviewing-adr", "reviewing-plan", "subagent-driven-development"}, Sections: []string{
			"when-fires", "dispatch-subagent-narrowed", "classify-route-findings",
			"apply-fixes-commit", "re-review-loop", "hand-off-to-impl", "notes",
		}},
		"reviewing-adr": {Core: true, Chain: true, RequiresAgent: "adr-reviewer", RequiresSkills: []string{"adr-lifecycle", "executing-plans", "proposing-adr", "reviewing-plan-resync", "subagent-driven-development", "writing-plans"}, Sections: []string{
			"when-fires", "procedure", "artifact-path-detection", "dispatch-subagent",
			"classify-route-findings", "apply-fixes-commit", "re-review-loop", "status-flip",
			"hand-off-to-resync", "notes",
		}},
		"reviewing-impl": {Core: true, Chain: true, RequiresAgent: "code-reviewer", RequiresSkills: []string{"executing-plans", "retrospective", "subagent-driven-development"}, Sections: []string{
			"when-fires", "sha-range-detection", "docs-only-check", "dispatch-subagent",
			"classify-route-findings", "apply-fixes-commit", "run-audit", "re-review-loop", "hand-off", "notes",
		}},
		"retrospective": {Core: true, Chain: true, Sections: []string{
			"when-fires", "procedure", "recurrence-signal", "promotion-ladder", "control", "notes",
		}},
		"refactor-coupling-audit": {Sections: []string{
			"when-to-invoke", "audit-shape-selection", "category-1-top-level-files",
			"category-2-sibling-tests", "category-3-subpackages", "category-4-codegen",
			"category-5-constructors", "category-6-init-visibility", "test-coupling-planning-rule",
			"output-format", "scope-shrink-rule", "notes",
		}},
		"roadmap-graduation": {RequiresDoc: "roadmap", Sections: []string{
			"when-fires", "failure-modes", "identify-entry", "reverify-measurements",
			"graduate-single-commit", "explicit-drop", "same-commit", "doc-currency", "notes",
		}},
	},
	Agents: map[string]TargetSpec{
		"adr-reviewer": {
			Sections: []string{"universal-lenses", "project-focus", "doc-currency"},
			Data: map[string]any{
				"focusItems": []any{
					map[string]any{"name": "decision-clarity", "description": "each Decision item is a discrete, implementable commitment a reader could act on without further consultation"},
					map[string]any{"name": "consequences-honesty", "description": "trade-offs name real costs and operational implications, not straw men"},
				},
				"docCurrencyItems": []any{
					map[string]any{"check": "every document that states the behaviour this ADR changes is updated in the same commit"},
					map[string]any{"check": "the decision index is regenerated when the ADR's status changes"},
				},
				"reviewSubject": "ADR",
				"readStep":      "Read the ADR in full. Read every doc, ADR, or state doc it references by name.",
				"digestLabel":   "ADR",
				"digestSummary": "- Decision: <one line, the load-bearing item>\n- Invariants: <1–2 headlines>\n- Trade-off: <one notable rejected alternative + why>",
			},
		},
		"plan-reviewer": {
			Sections:       []string{"universal-lenses", "project-focus", "doc-currency", "resync-note"},
			RequiresSkills: []string{"reviewing-plan-resync"},
			Data: map[string]any{
				"focusItems": []any{
					map[string]any{"name": "step-exactness", "description": "every task names exact file paths, exact content or diffs, and exact commands with expected output"},
					map[string]any{"name": "dependency-order", "description": "tasks are ordered so each builds only on already-completed work"},
				},
				"docCurrencyItems": []any{
					map[string]any{"check": "the plan schedules updates for every document its changes invalidate, in the same commits"},
				},
				"reviewSubject": "plan",
				"readStep":      "Read the artifact in full. Read every doc, ADR, or state doc it references by name.",
				"digestLabel":   "Plan",
				"digestSummary": "- Goal: <one line from the plan header>\n- Shape: <phase count, commit count, files created/modified>\n- Headline tasks: <1–2 sentences naming the load-bearing tasks>",
			},
		},
		"code-reviewer": {
			Sections: []string{"universal-lenses", "project-focus", "doc-currency"},
			Data: map[string]any{
				"correctnessTraps": []any{
					map[string]any{"description": "error paths: every returned error is checked or explicitly ignored with a stated reason"},
					map[string]any{"description": "boundary conditions at empty, zero, and null/nil inputs"},
				},
				"focusItems": []any{
					map[string]any{"name": "plan-adherence", "description": "the diff matches the plan's stated file paths and content; unexplained drift is a finding"},
					map[string]any{"name": "test-coverage", "description": "behaviour changes carry tests in the same commit; no assertion is weakened to pass"},
				},
				"docCurrencyItems": []any{
					map[string]any{"check": "the change updates every document that states the old behaviour — same commit"},
				},
				"reviewSubject": "diff",
				"readStep":      "Read the diff in full (`git diff baseSha..headSha`). Read every plan, ADR, or state doc referenced by name in the brief.",
				"digestLabel":   "Impl",
				"digestSummary": "- Commits: <one line per commit subject>\n- Headline change: <1–2 sentences>\n- Test additions: <file count or named test files>",
			},
		},
	},
	DomainDoc: TargetSpec{Sections: []string{"current-state"}},
	Docs: map[string]DocEntry{
		// Toggleable docs (Mandatory false) — rendered only when enabled in config.
		"architecture": {Title: "Architecture", Desc: "system shape, packages, key components, dependencies", Sections: []string{"overview", "components", "data-flow", "dependencies"}, TID: "docs/architecture.md.tmpl"},
		"testing":      {Title: "Testing", Desc: "gate tiers, test layout, what each tier covers", Sections: []string{"gate", "tiers", "layout"}, TID: "docs/testing.md.tmpl"},
		"development":  {Title: "Development", Desc: "local setup, the command runner, dependency reference", Sections: []string{"setup", "command-runner", "dependencies"}, TID: "docs/development.md.tmpl"},
		"debugging":    {Title: "Debugging", Desc: "recipes for common failure modes", Sections: []string{"surfaces", "recipes"}, TID: "docs/debugging.md.tmpl"},
		"pitfalls":     {Title: "Pitfalls", Desc: "recurring bugs and tricky areas", Sections: []string{"entries"}, TID: "docs/pitfalls.md.tmpl"},
		// The glossary's table is computed from sidecar data.terms, always
		// sorted (ADR-0089); prepend/append are empty-default framing slots.
		"glossary": {Title: "Glossary", Desc: "project jargon and term ownership", Sections: []string{"prepend", "append"}, TID: "docs/glossary.md.tmpl"},
		"roadmap":  {Title: "Roadmap", Desc: "uncommitted ideas and future phases", Sections: []string{"ideas", "deferred"}, TID: "docs/roadmap.md.tmpl"},
		// Always-on singletons (Mandatory true). agents-doc renders to root AGENTS.md
		// (empty Path/TemplateKey, AgentsDoc true); the four DocumentMap docs are cited
		// in AGENTS.md's document map via .layout.*.
		"agents-doc": {Mandatory: true, AgentsDoc: true, TID: "agents-doc/AGENTS.md.tmpl", Sections: []string{
			"awf-setup", "you-and-this-project", "identity", "invariants", "workflow", "working-memory", "commands", "document-map",
		}},
		"adr-readme":   {Mandatory: true, Path: "decisions/README.md", TemplateKey: "adrReadme", TID: "adr-readme/README.md.tmpl", Sections: []string{"intro", "when", "naming", "frontmatter", "invariants", "active-md"}},
		"adr-template": {Mandatory: true, Path: "decisions/template.md", TemplateKey: "adrTemplate", TID: "adr-template/template.md.tmpl", Sections: []string{"frontmatter", "body"}},
		"plans-readme": {Mandatory: true, Path: "plans/README.md", TemplateKey: "plansReadme", TID: "plans-readme/README.md.tmpl", Sections: []string{"intro", "naming", "structure"}},
		"workflow": {Mandatory: true, DocumentMap: true, Title: "Workflow", Desc: "principles, the brainstorm/ADR/plan chain, commit discipline", Path: "workflow.md", TemplateKey: "workflowRef", TID: "docs/workflow.md.tmpl", Sections: []string{
			"principles", "chain", "commit-discipline", "doc-currency", "composing-the-gate", "local-hooks", "ci",
		}},
		"doc-standard":       {Mandatory: true, DocumentMap: true, Title: "Documentation Standard", Desc: "how-to-write rules for all awf-managed prose", Path: "doc-standard.md", TemplateKey: "docStandard", TID: "docs/doc-standard.md.tmpl", Sections: []string{"principles", "rules", "structure"}},
		"agents-md-standard": {Mandatory: true, DocumentMap: true, Title: "Authoring AGENTS.md", Desc: "layout, content, and rules for the agent guide", Path: "agents-md-standard.md", TemplateKey: "agentsMdStandard", TID: "docs/agents-md-standard.md.tmpl", Sections: []string{"layout", "content", "rules"}},
		"working-with-awf": {Mandatory: true, DocumentMap: true, Title: "Working with awf", Desc: "day-to-day usage: commands, overrides, placeholders, and the sync/check loop", Path: "working-with-awf.md", TemplateKey: "workingWithAwf", TID: "docs/working-with-awf.md.tmpl", Sections: []string{
			"overview", "commands", "config-and-overrides", "placeholders", "sync-and-drift", "upgrading",
		}},
		"config-reference": {Mandatory: true, Generated: true, DocumentMap: true, Title: "Configuration Reference", Desc: "every .awf config key, var, sidecar field, and data key — descriptions, defaults, availability, and this project's live state", Path: "config-reference.md", TemplateKey: "configReference", TID: "docs/config-reference.md.tmpl", Sections: []string{"intro"}},
	},
	Vars: []VarDescriptor{
		{Key: "gateCmd", Kind: "string", Description: "Command that runs the full pre-commit gate (tests, lint, coverage).", Default: "", Options: []string{"./x gate", "make gate", "go test ./..."}},
		{Key: "gateCmdFull", Kind: "string", Description: "Command for the full/extended gate tier, if the project has one.", Default: "", Options: []string{"./x gate full"}},
		{Key: "checkCmd", Kind: "string", Description: "Command that checks rendered output for drift. Leave empty to have the rendered hook payloads run the pinned awf via the bootstrap shim.", Default: "", Options: []string{"./x check"}},
		{Key: "commitGateCmd", Kind: "string", Description: "Command that validates one commit message (the commit-msg hook payload appends the message-file argument). Leave empty to have the payload run the pinned awf via the bootstrap shim.", Default: "", Options: []string{"./x commit-gate"}},
		{Key: "testCmd", Kind: "string", Description: "Command that runs the test suite.", Default: "", Options: []string{"./x test", "go test ./...", "npm test"}},
		{Key: "commitScopes", Kind: "string", Target: "audit-scopes", Description: "Comma-separated Conventional Commits scopes this project allows. Written to audit.allowedScopes — enforced by awf commit-gate/audit and quoted by the reviewing skills. Leave empty to accept any scope.", Default: "", Options: []string{"adr,awf,plans"}},
		{Key: "activeMdRegenCmd", Kind: "string", Description: "Command that regenerates the generated ADR index (ACTIVE.md).", Default: "", Options: []string{"./x sync", "awf sync"}},
		{Key: "invariantTestPath", Kind: "string", Description: "Path or glob where invariant-backing tests live.", Default: "", Options: []string{"./internal/..."}},
		{Key: "skills", Kind: "multiselect", Target: "catalog-skills", Description: "Workflow skills to enable (core pre-selected; deselect to trim or add opt-in skills). Options/default computed from the catalog."},
		{Key: "docs", Kind: "multiselect", Target: "catalog-docs", Description: "Docs to enable (core pre-selected; deselect to trim or add opt-in docs). Options/default computed from the catalog."},
	},
}
