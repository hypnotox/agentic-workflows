package catalog

// Standard is the compile-time catalog: awf's static description of the standard
// (skills, docs, singletons, the domain-doc spec, and the fillable vars).
// Its default Data bags retain map[string]any, []any, and scalar shapes
// compatible with yaml.v3 output, so each per-file ConfigHash stays
// byte-identical (ADR-0060).
var Standard = &Catalog{
	Skills: map[string]SkillSpec{
		"awf-effort": {Sections: []string{
			"continuity-and-resident", "execution-and-checkpoints", "integration-and-recovery", "close",
		}},
		"awf-topics":      {Sections: []string{"claims"}},
		"awf-decisions":   {Sections: []string{"format"}},
		"awf-maintenance": {Sections: []string{"generated-documents", "upgrades"}},
	},
	DomainDoc: TargetSpec{Sections: []string{"current-state"}},
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
				map[string]any{"term": "agentic skill", "meaning": "An operator-installed generic `agentic-*` capability. AWF references these external skills but does not install, probe, configure, or rename them."},
				map[string]any{"term": "AWF skill", "meaning": "One of the four fixed repository-local skills: `awf-effort`, `awf-topics`, `awf-decisions`, or `awf-maintenance`. Project prefixes do not rename them."},
				map[string]any{"term": "effort", "meaning": "One active slugged unit of continuity, owning a working-memory file when multi-step work, likely continuation, coordination, delegation, or durable observations make continuity materially useful. Work without that need uses none."},
				map[string]any{"term": "managed effort worktree", "meaning": "The checkout an effort creates alongside itself, on its own branch, as the default place its work executes. Integrated and removed explicitly when the effort finishes."},
				map[string]any{"term": "working memory", "meaning": "The file an effort owns for in-flight context: its brief, settled decisions, observations, and handoff log. One writer; finish archives the complete resident, and nothing others must honour lives there alone."},
				map[string]any{"term": "current-state topic", "meaning": "A domain-owned document of prose plus a closing claims section. Its claims, not the decision-record corpus, are what tooling reads for the rules in force now."},
				map[string]any{"term": "claim", "meaning": "One statement of what holds today, declared in a current-state topic and carrying its provenance. An invariant claim is additionally backed, by a test or by stated reasoning."},
				map[string]any{"term": "invariant backing", "meaning": "What makes an invariant claim checkable: either a proof marker on a test, or a written verification procedure where no test can bear it. The two forms are enforced symmetrically."},
				map[string]any{"term": "drift", "meaning": "Divergence between a generated file and what the config would produce now, or between a declaration and reality. The check command is the oracle, and drift fails it."},
				map[string]any{"term": "resident root", "meaning": "A directory inside the config tree holding local machine-owned state rather than rendered output, so the closed-tree sweep leaves it alone instead of reporting it as a stray."},
				map[string]any{"term": "stub", "meaning": "A rendered section still carrying only its placeholder text. Stubs raise a non-failing advisory so unwritten content stays visible instead of passing as authored."},
			},
		}},
		"roadmap": {Title: "Roadmap", Desc: "uncommitted ideas and future phases", Sections: []string{"ideas", "deferred"}, TID: "docs/roadmap.md.tmpl"},
		// Singleton docs (Mandatory true). agents-doc renders to root AGENTS.md
		// (empty Path/TemplateKey, AgentsDoc true); the four DocumentMap docs are cited
		// in AGENTS.md's document map via .layout.*.
		"agents-doc": {Mandatory: true, AgentsDoc: true, TID: "agents-doc/AGENTS.md.tmpl", Sections: []string{
			"awf-setup", "you-and-this-project", "identity", "invariants", "workflow", "working-memory", "commands", "document-map",
		}},
		"workflow": {Mandatory: true, DocumentMap: true, Title: "Workflow", Desc: "principles, conditional capabilities, continuity, review, and commit discipline", Path: "workflow.md", TemplateKey: "workflowRef", TID: "docs/workflow.md.tmpl", Sections: []string{
			"principles", "chain", "working-memory", "commit-discipline", "doc-currency", "composing-the-gate", "local-hooks", "ci",
		}},
		"doc-standard":       {Mandatory: true, DocumentMap: true, Title: "Documentation Standard", Desc: "how-to-write rules for all awf-managed prose", Path: "doc-standard.md", TemplateKey: "docStandard", TID: "docs/doc-standard.md.tmpl", Sections: []string{"principles", "rules", "structure"}},
		"agents-md-standard": {Mandatory: true, DocumentMap: true, Title: "Authoring AGENTS.md", Desc: "layout, content, and rules for the agent guide", Path: "agents-md-standard.md", TemplateKey: "agentsMdStandard", TID: "docs/agents-md-standard.md.tmpl", Sections: []string{"layout", "content", "rules"}},
		"working-with-awf": {Mandatory: true, DocumentMap: true, Title: "Working with awf", Desc: "day-to-day usage: commands, overrides, placeholders, and the sync/check loop", Path: "working-with-awf.md", TemplateKey: "workingWithAwf", TID: "docs/working-with-awf.md.tmpl", Sections: []string{
			"overview", "commands", "config-and-overrides", "advanced-workflow", "placeholders", "sync-and-drift", "upgrading",
		}},
		"pi-runtime-reference": {Mandatory: true, DocumentMap: true, Title: "Pi Runtime Reference", Desc: "Pi prerequisites, external role support, and AWF effort handoff guidance", Path: "pi-runtime-reference.md", TemplateKey: "piRuntimeReference", TID: "docs/pi-runtime-reference.md.tmpl"},
		"config-reference":     {Mandatory: true, Generated: true, DocumentMap: true, Title: "Configuration Reference", Desc: "every .awf config key, var, sidecar field, and data key: descriptions, defaults, availability, and this project's live state", Path: "config-reference.md", TemplateKey: "configReference", TID: "docs/config-reference.md.tmpl", Sections: []string{"intro"}},
	},
	Vars: []VarDescriptor{
		{Key: "gateCmd", Kind: "string", Description: "Command that runs the fast pre-commit gate.", Default: "", Options: []string{"make gate", "go test ./..."}},
		{Key: "checkCmd", Kind: "string", Description: "Command that checks rendered output for drift. Leave empty to run through the always-rendered `./awf` wrapper.", Default: "", Options: []string{"./awf check", "make check"}},
		{Key: "testCmd", Kind: "string", Description: "Command that runs the test suite.", Default: "", Options: []string{"go test ./...", "npm test", "make test"}},
	},
}
