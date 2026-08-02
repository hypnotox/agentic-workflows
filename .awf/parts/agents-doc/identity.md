## Identity

`awf` is a generic agentic-development-workflow application: it scaffolds, renders, and drift-checks multi-runtime skills, agents, docs, and this agent guide from a committed `.awf/` config tree, and mechanically guards drift, frontmatter, current-state provenance, and invariant backing. The project-owned workflow chain is rendered in each target's native form, and Pi receives generated subagent and handoff extensions. The awf tool is a Go binary (module `github.com/hypnotox/agentic-workflows`, Go 1.26); the standard it renders is language-agnostic. Public, pre-1.0, no external API stability.

Pi supplies five governed TypeScript outputs: standalone context-usage observation, handoff replacement, and the subagent index, routing, and runner. Context usage injects neutral transient session facts and never persists, warns, or acts on pressure.
