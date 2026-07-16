---
status: Implemented
date: 2026-07-16
supersedes: []
superseded_by: ""
tags: [multi-target, target-seam, template-overlay]
related: [16, 37, 38, 68]
domains: [rendering, tooling]
---
# ADR-0122: Multi-Runtime Targets and Format-Neutral Agents

## Context

awf currently renders adapter artifacts for Claude Code and Cursor. The `Target`
registry introduced by ADR-0037 deliberately makes another runtime a descriptor
addition rather than a render-loop rewrite, but it assumes every agent is a
Markdown file with YAML frontmatter and that every enabled target receives
byte-identical skill and agent content.

Pi, Codex, Gemini, and GitHub Copilot each support the Agent Skills standard,
but their project layouts and reviewer-agent conventions differ. Pi discovers
`SKILL.md` directories and root Markdown skills under `.pi/skills/`, reads
`AGENTS.md`, and has no built-in subagent orchestration. Codex reads skills from
`.agents/skills/`, reads `AGENTS.md`, and runs native subagents, but its custom
agents are TOML profiles under `.codex/agents/`. Gemini reads `.gemini/skills/`,
loads custom Markdown agents from `.gemini/agents/`, and uses a `GEMINI.md`
instruction file. Copilot reads `.github/skills/`, supports custom Markdown
subagents at `.github/agents/<name>.agent.md`, and reads `AGENTS.md`.

Parsing a fully rendered YAML-frontmatter Markdown agent in order to emit Codex
TOML would make Markdown an accidental internal representation and make the
Codex adapter depend on a lossy, post-render translation. The catalog and local
agent seam must instead represent an agent independently of its output dialect.

## Decision

1. Extend the built-in target registry with `pi`, `codex`, `gemini`, and
   `copilot`. Each descriptor owns its skill path, agent path and dialect,
   instruction bridge when required, and review-dispatch style. The target set
   is `{claude, codex, copilot, cursor, gemini, pi}`. Claude keeps its
   `CLAUDE.md` bridge; Gemini receives a `GEMINI.md` bridge importing
   `AGENTS.md`; Cursor, Codex, Pi, and Copilot rely on native `AGENTS.md`
   discovery.

2. Replace Markdown-frontmatter agent rendering as the internal agent model with
   a format-neutral artifact: a literal `name`, separately rendered
   `description` metadata, and a section-rendered Markdown instruction body.
   Description rendering uses the normal template data so existing prefix-aware
   wording survives without making an output dialect canonical. Standard and
   project-local agents share this model. A target encoder emits YAML-frontmatter Markdown for
   Claude, Cursor, Gemini, Copilot, and Pi, and a typed Codex TOML profile with
   `name`, `description`, and `developer_instructions` for Codex. The Codex
   encoder uses a maintained TOML library and typed schema, never handwritten
   escaping or parsing a rendered Markdown artifact.

3. Pi renders reviewer definitions as top-level Markdown skills under
   `.pi/skills/`. Pi-target copies of review-dispatch sections use generic
   runtime-neutral prose: they direct the workflow to use an available reviewer
   or delegation mechanism without claiming native subagents or requiring a
   separate session. The other targets retain the native-subagent wording.

4. A Pi subagent orchestrator is deferred as a roadmap item. This decision ships
   no Pi extension and does not promise autonomous fresh-context review on Pi;
   a future extension may consume the rendered reviewer skills without changing
   their paths or names.

## Invariants

- `invariant: target-dialect-render`: every enabled target renders each skill and
  agent exactly once at that target's declared path and dialect; the emitted
  artifact parses under that runtime's native format. Neutral artifacts still
  render once.
- `invariant: structured-agent-encoding`: target encoders consume structured
  agent metadata and the rendered instruction body; no encoder parses a
  rendered agent artifact to produce another dialect.
- `invariant: pi-generic-review-dispatch`: Pi renders reviewer definitions as
  skills and its review-dispatch prose is generic about available runtime
  mechanisms, with neither a native-subagent assertion nor a separate-session
  requirement.
- A target definition, including its paths, dialect, bridge, and review-dispatch
  style, contributes to each affected artifact's drift inputs. (Textual.)
- The Pi orchestrator remains a future roadmap item rather than an implied
  capability of the initial Pi adapter. (Textual.)

## Consequences

- awf supports six built-in runtimes and makes target-specific output formats an
  explicit part of the adapter seam.
- The agent catalog and local-agent renderer become more structured, but source
  templates no longer duplicate output-format metadata or require an output
  parse to serve Codex.
- Target-specific review wording means selected rendered skills are no longer
  universally byte-identical across all targets. The target test suite must
  assert semantic parity where expected and the Pi fallback where deliberate.
- The change adds a TOML dependency, whose version and behavior become part of
  awf's supply-chain and test surface.
- The repository and Sundial example must dogfood every new adapter and commit
  their rendered output trees, increasing sync and review fan-out.
- The implementation updates the architecture, configuration reference, working
  guide, agent guide, README, and rendering/tooling current-state narratives in
  the same commits as the code. It adds a tracked roadmap entry for the deferred
  Pi orchestrator, and adds any needed ignore negations before committing the
  new output trees.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Render Markdown, parse its YAML frontmatter, then serialize TOML | Couples Codex to a rendered output dialect and invites lossy or malformed translation. |
| Maintain separate Markdown and TOML reviewer templates | Duplicates review policy and lets the two dialects drift. |
| Render Pi only with workflow skills and omit reviewer artifacts | Leaves the workflow chain incomplete and prevents present or future Pi extensions from using the reviewers. |
| Require Pi users to launch a separate reviewer session | Imposes a runtime-specific workflow that may conflict with an adopter-provided or future awf extension. |
| Bundle a Pi subagent extension now | Valuable but independent orchestration work; it would delay useful skills and instruction support. |
