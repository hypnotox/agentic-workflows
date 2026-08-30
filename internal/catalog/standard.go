package catalog

// Standard is the compile-time catalog: awf's static description of the standard
// (skills, agents, docs, singletons, the domain-doc spec, and the fillable vars).
// Its default Data bags retain map[string]any, []any, and scalar shapes
// compatible with yaml.v3 output, so each per-file ConfigHash stays
// byte-identical (ADR-0060).
var Standard = &Catalog{
	Skills: map[string]SkillSpec{
		"brainstorming": {Profile: WorkflowProfile{Kind: WorkflowChain, Purpose: "Clarify an outcome and settle an approved design.", Trigger: "Use when work needs a material choice or clarification.", CommonFollowUps: []string{"proposing-adr", "executing-direct"}}, Sections: []string{
			"preamble", "when-to-invoke", "procedure", "example-clarifying-questions",
			"design-sections", "no-spec-rule", "terminal-step", "definitions", "anti-patterns",
		}},
		"grounding": {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Check broad or uncertain repository premises from any workflow.", Trigger: "Use when correctness depends on broad or uncertain repository facts.", CommonFollowUps: []string{"brainstorming", "debugging", "refactor-coupling-audit"}}, RequiresAgent: "grounding-checker", Sections: []string{
			"invocation", "brief-construction-and-dispatch", "finding-classification", "boundaries", "notes",
		}},
		"executing-direct": {Profile: WorkflowProfile{Kind: WorkflowChain, Purpose: "Implement a clear narrow change directly.", Trigger: "Use when outcome, boundary, and verification are clear and no independent design or plan need fires.", CommonFollowUps: []string{"reviewing-impl", "effort-workflow"}}},
		"effort-workflow":  {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Own one awf effort from continuity evaluation through finish.", Trigger: "Use whenever durable continuity materially helps, or to resume or finish an effort."}},
		"using-awf":        {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Maintain awf's generated tree from source through render, drift resolution, and upgrade.", Trigger: "Use when maintaining awf's generated tree: edit `.awf/` sources, render outputs, resolve drift, or upgrade awf."}, Sections: []string{"procedure"}},
		"writing-docs":     {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Author project documentation at its owning surface.", Trigger: "Use when authoring project documentation: select the document that owns the fact and keep it current with the change."}, Sections: []string{"procedure"}},
		"tdd": {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Establish durable evidence before a fix or drive a feature from a failing test.", Trigger: "Use before implementing a bug fix, or a feature whose missing behavior is testable in isolation.", UsuallyFollows: []string{"bugfix", "debugging"}, CommonFollowUps: []string{"executing-direct"}},
			Sections: []string{"surfaces", "notes", "red-flags"},
			Data: map[string]any{
				"testSurfaces": []any{
					map[string]any{"name": "Unit", "kind": "fast isolated test", "location": "beside the code under test"},
					map[string]any{"name": "Integration", "kind": "cross-component test", "location": "the project's integration suite"},
					map[string]any{"name": "End-to-end", "kind": "full-system test", "location": "the project's e2e suite"},
				},
			},
		},
		"debugging": {Profile: WorkflowProfile{Kind: WorkflowTask, Purpose: "Investigate a defect before changing it.", Trigger: "Use when investigating a bug or unexpected behaviour before any fix.", CommonFollowUps: []string{"bugfix"}}, Sections: []string{
			"symptom-list", "debugging-surfaces", "test-isolation", "oracle-invariant",
			"devdb-note", "red-flags", "memory-checkpoint",
		}},
		"exploring": {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Explore repository facts without polluting the main context.", Trigger: "Use for fresh-context repository exploration when inline search would pollute the parent context.", CommonFollowUps: []string{"brainstorming", "debugging", "refactor-coupling-audit"}}, RequiresAgent: "explorer", Sections: []string{
			"when-to-invoke", "breadth", "detail", "dispatch", "results", "boundaries", "notes",
		}},
		"orienting": {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Ground the session in repository truth before starting, resuming, or widening work.", Trigger: "Use when repository truth is needed while taking up a topic: before brainstorming fresh work, when resuming an effort, or when taking over a handoff.", CommonFollowUps: []string{"brainstorming", "debugging", "executing-direct"}}, Sections: []string{
			"when-to-invoke", "guide-ladder", "context-command", "resume-revalidation", "hand-off",
		}},
		"proposing-adr": {FullOnly: true, Profile: WorkflowProfile{Kind: WorkflowChain, Purpose: "Author a decision record for a material design choice.", Trigger: "Use when a durable architectural or workflow decision is needed.", UsuallyFollows: []string{"brainstorming"}, CommonFollowUps: []string{"reviewing-adr", "executing-direct"}},
			Sections: []string{
				"positioning", "when-to-invoke", "conventions", "procedure-number", "procedure-write",
				"state-doc-update", "procedure-state-changes", "procedure-regen",
				"procedure-commit", "autonomous-rule", "terminal-step", "notes",
			},
			Data: map[string]any{
				"adrSections": []any{"Context", "Decision", "State changes", "Consequences", "Alternatives Considered", "Status history"},
				"adrTriggers": []any{
					"Introducing or moving a module/package boundary",
					"Adopting a new external dependency",
					"Changing a persisted format (config, lock file, schema, API contract)",
					"Changing the development workflow's rules",
					"Any decision a future maintainer would need to know the \"why\" for",
				},
			},
		},
		"adr-lifecycle": {FullOnly: true, Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Apply an ADR lifecycle transition correctly.", Trigger: "Use when transitioning an ADR between lifecycle states.", UsuallyFollows: []string{"proposing-adr", "reviewing-adr"}, CommonFollowUps: []string{"executing-direct"}},
			Sections: []string{
				"states", "transitions", "state-changes",
				"procedure-status-edit", "procedure-claim-mutation", "state-doc-update",
				"procedure-regen", "procedure-gate", "commit-templates", "amendment-until-terminal", "notes",
			},
			Data: map[string]any{
				"adrStates": []any{
					map[string]any{"name": "Proposed", "meaning": "ADR is written and under review; content is freely mutable", "mutability": "Freely mutable; body and status may both change"},
					map[string]any{"name": "Accepted", "meaning": "Design is finalised; implementation authorised but not yet started", "mutability": "Status and append-only Status history; the body stays amendable, each amendment appending an Amended event; a schema retrofit may migrate the encoding"},
					map[string]any{"name": "Implementing", "meaning": "A nonempty set of declared operations is applied; Remaining may be empty", "mutability": "Status and append-only Status history; every explicit Applied batch belongs to implementation, and the body stays amendable via Amended events"},
					map[string]any{"name": "Implemented", "meaning": "All declared claim operations are applied", "mutability": "Terminal; status and append-only Status history only; the body is frozen; a schema retrofit may migrate the encoding"},
					map[string]any{"name": "Abandoned", "meaning": "Execution stopped; applied operations remain historical and unapplied operations are canceled", "mutability": "Terminal; status and append-only Status history only; the final entry carries a rationale; the body is frozen"},
				},
			},
		},
		"bugfix": {Profile: WorkflowProfile{Kind: WorkflowTask, Purpose: "Apply a fix with a known root cause.", Trigger: "Use when applying a fix whose root cause is already known.", UsuallyFollows: []string{"debugging"}, CommonFollowUps: []string{"reviewing-impl"}}, Sections: []string{"test-tiers", "pitfalls-check", "oracle-note", "memory-checkpoint"}},
		"reviewing-adr": {FullOnly: true, Profile: WorkflowProfile{Kind: WorkflowChain, Purpose: "Independently review an ADR.", Trigger: "Use when a proposed or amended ADR needs decision-quality review.", UsuallyFollows: []string{"proposing-adr"}, CommonFollowUps: []string{"executing-direct"}}, RequiresAgent: "adr-reviewer", Sections: []string{
			"when-fires", "procedure", "artifact-path-detection", "dispatch-subagent",
			"classify-route-findings", "apply-fixes-commit", "re-review-loop", "status-flip",
			"hand-off-to-plan-review", "notes",
		}},
		"reviewing-impl": {Profile: WorkflowProfile{Kind: WorkflowChain, Purpose: "Independently assure an implementation.", Trigger: "Use when independent review has assurance value for the implementation.", UsuallyFollows: []string{"executing-direct"}, CommonFollowUps: []string{"effort-workflow"}}, RequiresAgent: "code-reviewer", Sections: []string{
			"when-fires", "sha-range-detection", "dispatch-subagent",
			"classify-route-findings", "apply-fixes-commit", "run-audit", "re-review-loop", "hand-off", "notes",
		}},
		"retrospective": {Profile: WorkflowProfile{Kind: WorkflowChain, Purpose: "Capture durable lessons, verify managed topology is absent, and finish the effort last.", Trigger: "Use from effort-workflow after assurance settles or is explicitly skipped and managed topology is removed.", UsuallyFollows: []string{"effort-workflow"}}, Sections: []string{
			"when-fires", "procedure", "recurrence-signal", "promotion-ladder", "control", "notes",
		}},
		"refactor-coupling-audit": {Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Scope dependency and test coupling before a refactor.", Trigger: "Use when scoping a refactor that moves files between packages or inverts dependencies.", CommonFollowUps: []string{"brainstorming", "proposing-adr"}}, Sections: []string{
			"when-to-invoke", "audit-shape-selection", "category-1-top-level-files",
			"category-2-sibling-tests", "category-3-subpackages", "category-4-codegen",
			"category-5-constructors", "category-6-init-visibility", "test-coupling-planning-rule",
			"output-format", "scope-shrink-rule", "notes",
		}},
		"roadmap-graduation": {FullOnly: true, Profile: WorkflowProfile{Kind: WorkflowSupport, Purpose: "Move a settled roadmap item out of the roadmap.", Trigger: "Use when a roadmap entry graduates to an ADR or a PR, or is explicitly dropped.", UsuallyFollows: []string{"reviewing-impl"}}, RequiresDoc: "roadmap", Sections: []string{
			"when-fires", "failure-modes", "identify-entry", "reverify-measurements",
			"graduate-single-commit", "explicit-drop", "same-commit", "doc-currency", "notes",
		}},
	},
	Agents: map[string]AgentSpec{
		"adr-reviewer": {
			FullOnly:    true,
			Name:        "adr-reviewer",
			Description: "Independent, lens-diverse reviewer for ADRs under {{ .layout.adrDir }}/ in {{ .prefix }} projects.\nReturns structured findings per the shared review-discipline spine.",
			Sections:    []string{"universal-lenses", "project-focus"},
			Data: map[string]any{
				"focusItems": []any{
					map[string]any{"name": "consequences-honesty", "description": "trade-offs name real costs and operational implications, not straw men"},
					map[string]any{"name": "claim-topic-cohesion", "description": "each claim this ADR adds belongs in the topic its State changes names: it answers the same question that topic's existing claims answer, rather than landing there because the topic is adjacent or convenient. Flag a destination that gives its topic a second subject, and name the subject the claim belongs to instead. Judge by subject, never by how many claims the topic already holds."},
				},
				"reviewSubject": "ADR",
				"readStep":      "Read the ADR in full. Read every doc, ADR, or current-state topic it references by name.",
				"digestLabel":   "ADR",
				"digestSummary": "- Decision: <one line, the load-bearing item>\n- State changes: <the claim add/update/remove operations>\n- Trade-off: <one notable rejected alternative + why>",
			},
		},
		"code-reviewer": {
			Name:        "code-reviewer",
			Description: "Independent fresh-context reviewer for {{ .prefix }} implementation diffs, covering its universal review lenses from correctness through convention alignment.",
			Sections:    []string{"universal-lenses", "project-focus", "doc-currency"},
			Data: map[string]any{
				"correctnessTraps": []any{
					map[string]any{"description": "error paths: every returned error is checked or explicitly ignored with a stated reason"},
					map[string]any{"description": "boundary conditions at empty, zero, and null/nil inputs"},
				},
				"focusItems": []any{
					map[string]any{"name": "contract-adherence", "description": "the diff satisfies the protected contract, Definition of done, current authority, and reconciled material instructions; unexplained outcome or authorization drift, incoherent landed transactions, and incomplete work are findings, not departures from original choreography"},
					map[string]any{"name": "test-coverage", "description": "behaviour changes carry tests in the same commit; no assertion is weakened to pass"},
					map[string]any{"name": "verification-instrument-can-fail", "description": "for every added or changed mechanical check, require a negative case and a temporary falsification that proves the mutation landed before its passing verdict counts; restore only the temporary mutation, and never use a whole-file reset that can erase unrelated uncommitted work"},
					map[string]any{"name": "check-authority-taxonomy", "description": "classify each material check as an authority, state, or choreography check; preserve authority checks, require state checks to be no stricter than the durable property they prove, and flag choreography-only enforcement with no named authority or state obligation"},
				},
				"docCurrencyItems": []any{
					map[string]any{"check": "the change updates every document that states the old behaviour, in the same commit"},
				},
				"reviewSubject": "diff",
				"readStep":      "Read the diff in full (`git diff baseSha..headSha`). Read every plan, ADR, or state doc referenced by name in the brief.",
				"digestLabel":   "Impl",
				"digestSummary": "- Commits: <one line per commit subject>\n- Headline change: <1-2 sentences>\n- Test additions: <file count or named test files>",
			},
		},
		// The implementer is the one dispatched role that acts rather than
		// reports (ADR-0177). RequiresSkills stays empty on purpose: the
		// contract routes the child into no skill, since the chain belongs to
		// the dispatching parent.
		"implementer": {
			Name:        "implementer",
			Description: "Commit-disabled implementation child for {{ .prefix }} work.\nReturns a structured completed or stopped focused-evidence receipt.",
			Sections:    []string{"identity", "task-scope", "guide-authority", "green-obligation", "escalation", "return-schema"},
			Data: map[string]any{
				"prohibitedShortcuts": []any{
					map[string]any{"description": "adding an abstraction with no current call site, on the argument that a later change will use it"},
					map[string]any{"description": "widening one function's responsibility because the fix is easier to place there than where it belongs"},
				},
			},
		},
		"explorer": {
			Name:        "explorer",
			Description: "Fresh-context exploration subagent for {{ .prefix }} repository questions, handling one information need under a selected breadth and report detail.\nReturns a grounded report only.",
			Sections:    []string{"identity", "single-need", "breadth", "report-detail", "grounding-and-outcomes", "report-discipline"},
		},
		"grounding-checker": {
			Name:        "grounding-checker",
			Description: "Fresh-context grounding-check subagent for {{ .prefix }} designs, testing factual premises, assumptions, altitude, and convention fit against the repository.\nReturns advisory findings only.",
			Sections:    []string{"identity", "verification-scope", "return-schema"},
		},
	},
	DomainDoc: TargetSpec{FullOnly: true, Sections: []string{"current-state"}},
	Docs: map[string]DocEntry{
		// Name-derived docs (Mandatory false).
		"architecture": {Title: "Architecture", Desc: "system shape, packages, key components, dependencies", Sections: []string{"overview", "components", "data-flow", "dependencies"}, TID: "docs/architecture.md.tmpl"},
		"testing":      {Title: "Testing", Desc: "gate tiers, test layout, what each tier covers", Sections: []string{"gate", "tiers", "layout"}, TID: "docs/testing.md.tmpl"},
		"development":  {Title: "Development", Desc: "local setup, the command runner, dependency reference", Sections: []string{"setup", "command-runner", "dependencies"}, TID: "docs/development.md.tmpl"},
		"debugging":    {Title: "Debugging", Desc: "recipes for common failure modes", Sections: []string{"surfaces", "recipes"}, TID: "docs/debugging.md.tmpl"},
		"pitfalls":     {Title: "Pitfalls", Desc: "recurring bugs and tricky areas", Sections: []string{"prepend", "append"}, TID: "docs/pitfalls.md.tmpl"},
		"releasing":    {Title: "Releasing", Desc: "how to cut a release: versioning, artifacts, and the publish process", Sections: []string{"content"}, TID: "docs/releasing.md.tmpl"},
		// The glossary's table is computed from sidecar data.terms, always
		// sorted (ADR-0089); prepend/append are empty-default framing slots.
		// standardTerms is the vocabulary awf ships into every adopter tree
		// (ADR-0207): the transform merges it under data.terms and deletes it,
		// so it is never adopter-settable and carries no configspec descriptor.
		// A project term of the same case-insensitive name overrides one.
		"glossary": {Title: "Glossary", Desc: "project jargon and the awf vocabulary it ships", Sections: []string{"prepend", "append"}, TID: "docs/glossary.md.tmpl", Data: map[string]any{
			"standardTerms": []any{
				map[string]any{"term": "effort", "meaning": "One active slugged unit of continuity, owning a working-memory file when multi-step work, likely continuation, coordination, delegation, or durable observations make continuity materially useful. Work without that need uses none."},
				map[string]any{"term": "managed effort worktree", "meaning": "The checkout an effort creates alongside itself, on its own branch, as the default place its work executes. Integrated and removed explicitly when the effort finishes."},
				map[string]any{"term": "working memory", "meaning": "The file an effort owns for in-flight context: its brief, settled decisions, observations, and handoff log. One writer; finish archives the complete resident, and nothing others must honour lives there alone."},
				map[string]any{"term": "current-state topic", "meaning": "A domain-owned document of prose plus a closing claims section. Its claims, not the decision-record corpus, are what tooling reads for the rules in force now."},
				map[string]any{"term": "claim", "meaning": "One statement of what holds today, declared in a current-state topic and carrying its provenance. An invariant claim is additionally backed, by a test or by stated reasoning."},
				map[string]any{"term": "invariant backing", "meaning": "What makes an invariant claim checkable: either a proof marker on a test, or a written verification procedure where no test can bear it. The two forms are enforced symmetrically."},
				map[string]any{"term": "drift", "meaning": "Divergence between a generated file and what the config would produce now, or between a declaration and reality. The check command is the oracle, and drift fails it."},
				map[string]any{"term": "resident root", "meaning": "A directory inside the config tree holding local machine-owned state rather than rendered output, so the closed-tree sweep leaves it alone instead of reporting it as a stray."},
				map[string]any{"term": "stub", "meaning": "A rendered section still carrying only its placeholder text. Stubs raise a non-failing advisory so unwritten content stays visible instead of passing as authored."},
				map[string]any{"term": "check-in", "meaning": "A deliberate stop for user attention: it names the issue, the options, a recommendation, and the blocked next action, then waits."},
				map[string]any{"term": "mandatory approval check-in", "meaning": "The pre-artifact stop for explicit brainstorming outline approval when a material decision is unresolved. Effort creation is not an approval boundary."},
				map[string]any{"term": "routine checkpoint", "meaning": "The boundary protocol between phases: update working memory, decide whether user attention is required, then either raise a check-in or state a continuity notice and continue."},
				map[string]any{"term": "continuity notice", "meaning": "The routine checkpoint's one-line summary on the clear branch, naming the completed phase and the immediate next action. Informational, never a stop."},
				map[string]any{"term": "retrospective", "meaning": "The terminal step of an effort: capture durable lessons, confirm no managed topology remains, and finish the effort last."},
				map[string]any{"term": "promotion ladder", "meaning": "The path a recurring finding takes from prose guidance toward a deterministic check, so a lesson stops depending on anyone remembering it."},
			},
		}},
		"roadmap": {Title: "Roadmap", Desc: "uncommitted ideas and future phases", Sections: []string{"ideas", "deferred"}, TID: "docs/roadmap.md.tmpl"},
		// Singleton docs (Mandatory true). agents-doc renders to root AGENTS.md
		// (empty Path/TemplateKey, AgentsDoc true); the four DocumentMap docs are cited
		// in AGENTS.md's document map via .layout.*.
		"agents-doc": {Mandatory: true, AgentsDoc: true, TID: "agents-doc/AGENTS.md.tmpl", Sections: []string{
			"awf-setup", "you-and-this-project", "identity", "invariants", "workflow", "working-memory", "commands", "document-map",
		}},
		"adr-readme":               {FullOnly: true, Mandatory: true, Path: "decisions/README.md", TemplateKey: "adrReadme", TID: "adr-readme/README.md.tmpl", Sections: []string{"intro", "when", "naming", "frontmatter", "lifecycle", "state-changes", "index"}},
		"adr-template":             {FullOnly: true, Mandatory: true, Path: "decisions/template.md", TemplateKey: "adrTemplate", TID: "adr-template/template.md.tmpl", Sections: []string{"frontmatter", "body"}},
		"maintainable-code-design": {Mandatory: true, DocumentMap: true, Title: "Maintainable Code Design", Desc: "decision framework for cohesive models, explicit boundaries, dependencies, refactoring, and testable design", Path: "maintainable-code-design.md", TemplateKey: "maintainableCodeDesign", TID: "docs/maintainable-code-design.md.tmpl", Sections: []string{"decision-posture", "contextual-heuristics", "semantic-modeling", "readability", "boundaries-and-dependencies", "pattern-toolbox", "preparatory-refactoring", "failure-modes"}},
		"workflow": {Mandatory: true, DocumentMap: true, Title: "Workflow", Desc: "principles, the brainstorm/ADR/plan chain, commit discipline", Path: "workflow.md", TemplateKey: "workflowRef", TID: "docs/workflow.md.tmpl", Sections: []string{
			"principles", "chain", "working-memory", "commit-discipline", "doc-currency", "composing-the-gate", "local-hooks", "ci",
		}},
		"doc-standard":       {Mandatory: true, DocumentMap: true, Title: "Documentation Standard", Desc: "how-to-write rules for all awf-managed prose", Path: "doc-standard.md", TemplateKey: "docStandard", TID: "docs/doc-standard.md.tmpl", Sections: []string{"principles", "rules", "structure"}},
		"agents-md-standard": {Mandatory: true, DocumentMap: true, Title: "Authoring AGENTS.md", Desc: "layout, content, and rules for the agent guide", Path: "agents-md-standard.md", TemplateKey: "agentsMdStandard", TID: "docs/agents-md-standard.md.tmpl", Sections: []string{"layout", "content", "rules"}},
		"working-with-awf": {Mandatory: true, DocumentMap: true, Title: "Working with awf", Desc: "day-to-day usage: commands, overrides, placeholders, and the sync/check loop", Path: "working-with-awf.md", TemplateKey: "workingWithAwf", TID: "docs/working-with-awf.md.tmpl", Sections: []string{
			"overview", "commands", "config-and-overrides", "model-selection", "placeholders", "sync-and-drift", "upgrading",
		}},
		"pi-runtime-reference": {Mandatory: true, DocumentMap: true, Title: "Pi Runtime Reference", Desc: "Pi-only runtime, subagent, model-routing, and handoff protocol", Path: "pi-runtime-reference.md", TemplateKey: "piRuntimeReference", TID: "docs/pi-runtime-reference.md.tmpl"},
		"config-reference":     {Mandatory: true, Generated: true, DocumentMap: true, Title: "Configuration Reference", Desc: "every .awf config key, var, sidecar field, and data key: descriptions, defaults, availability, and this project's live state", Path: "config-reference.md", TemplateKey: "configReference", TID: "docs/config-reference.md.tmpl", Sections: []string{"intro"}},
	},
	Vars: []VarDescriptor{
		{Key: "gateCmd", Kind: "string", Description: "Command that runs the fast pre-commit gate.", Default: "", Options: []string{"make gate", "go test ./..."}},
		{Key: "gateCmdFull", Kind: "string", Description: "Command for terminal exhaustive verification, if the project has one.", Default: "", Options: []string{"make gate-full"}},
		{Key: "checkCmd", Kind: "string", Description: "Command that checks rendered output for drift. Leave empty to run through the always-rendered `./awf` wrapper.", Default: "", Options: []string{"./awf check", "make check"}},
		{Key: "commitGateCmd", Kind: "string", Description: "Command that validates one commit message (the commit-msg hook payload appends the message-file argument). Leave empty to run through the always-rendered `./awf` wrapper.", Default: "", Options: []string{"./awf check staged commit"}},
		{Key: "testCmd", Kind: "string", Description: "Command that runs the test suite.", Default: "", Options: []string{"go test ./...", "npm test", "make test"}},
		{Key: "commitScopes", Kind: "string", Target: "audit-scopes", Description: "Comma-separated Conventional Commits scopes this project allows. Written to audit.allowedScopes and enforced by awf check staged commit/audit and quoted by the agent guide. Leave empty to accept any scope.", Default: "", Options: []string{"adr,awf,plans"}},
		{Key: "activeMdRegenCmd", Kind: "string", Description: "Command that regenerates the generated ADR decision index (INDEX.md).", Default: "", Options: []string{"./awf render", "awf render"}},
		{Key: "invariantTestPath", Kind: "string", Description: "Path or glob where invariant-backing tests live.", Default: "", Options: []string{"./internal/..."}},
	},
}
