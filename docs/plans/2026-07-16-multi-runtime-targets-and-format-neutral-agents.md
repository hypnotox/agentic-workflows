---
date: 2026-07-16
adrs: [122]
status: Proposed
---
# Plan: Multi-Runtime Targets and Format-Neutral Agents

## Goal

Implement ADR-0122: add Pi, Codex, Gemini, and Copilot targets while making agents
format-neutral and emitting validated Codex TOML profiles. Do not ship a Pi
subagent extension; record it as a roadmap item.

## Architecture summary

Split agent metadata from its instruction body, then encode that one rendered agent
artifact per target. Target descriptors define paths, agent dialect, optional bridge,
and review-dispatch style. Markdown targets retain YAML-frontmatter agent files;
Codex uses a typed TOML encoder. Pi receives generic review-dispatch prose.

## File structure

- **Created:** `internal/project/agent_test.go`, `templates/gemini/GEMINI.md.tmpl`,
  `docs/roadmap.md` and its `.awf` source when enabled.
- **Modified:** `go.mod`, `go.sum`, `internal/catalog/{catalog.go,standard.go}`,
  `internal/project/{target.go,target_test.go,render.go,check.go,local.go,local_test.go,coverage_test.go}`,
  `templates/agents/*.md.tmpl`, review skill templates, target/config tests,
  adapter documentation, both adopter configs, generated output trees and locks.
- **Deleted:** none.

## Phase 1: Canonical agent artifact

- [ ] **Task 1.1: Introduce the format-neutral catalog and renderer model.** In
  `internal/catalog/catalog.go`, replace agent use of `TargetSpec` with an
  `AgentSpec` that retains sections, requirements, base/local support and data,
  while declaring a literal name and separately rendered description metadata.
  Render the description with normal template data so prefix-aware standard prose
  and sidecar-provided local descriptions remain supported. In
  `internal/catalog/standard.go`, move each standard agent's frontmatter metadata
  into its `AgentSpec`; update `internal/project/local.go` so synthesized local
  agents supply their name from the artifact key and description from sidecar data.
  Remove YAML frontmatter from `templates/agents/_base.md.tmpl` and the three
  standard agent templates, leaving only their section-rendered instruction
  bodies.
- [ ] **Task 1.2: Add a typed agent encoder.** Add `internal/project/agent.go`
  with an internal `{Name, Description, Body}` agent value and Markdown/TOML
  encoders. The Markdown encoder emits the existing YAML frontmatter shape. Add a
  pinned TOML dependency in `go.mod`/`go.sum`; encode a typed Codex profile with
  exactly `name`, `description`, and `developer_instructions`, and strictly decode
  it before returning output. Extend `render.CommentStyle` and banner injection
  so a TOML profile receives valid `#` provenance comments rather than HTML
  comments. Do not parse a rendered Markdown file in either encoder.
- [ ] **Task 1.3: Route Markdown and Codex rendering through the new model.**
  Change `internal/project/target.go` to give the target descriptor an agent
  dialect and complete agent-path fields, then register `codex` with
  `.agents/skills` and `.codex/agents/*.toml`; retain the existing Claude and
  Cursor paths and Markdown dialect. In `internal/project/render.go`, keep agent
  body assembly on the normal section/part pipeline but select the target encoder
  before banner injection and manifest hashing. Keep skills and docs unchanged.
  Change `check.go`/sync validation so Markdown agents use frontmatter validation
  and Codex profiles use strict TOML validation; retain local artifact validation
  on every enabled target path. This task deliberately makes the Phase 1 TOML
  encoder production-reachable, as required by the dead-code gate.
- [ ] **Task 1.4: Prove format neutrality.** Add `internal/project/agent_test.go`
  cases that construct a standard and local agent, assert Markdown metadata/body
  output, assert strict TOML round-trip and multiline escaping, and assert that no
  Markdown parser is called by the TOML path. Update catalog, local, coverage, and
  render tests to preserve byte-identical Claude/Cursor Markdown output.
- [ ] **Task 1.5: Verify and commit.** Run `./x gate` with expected `coverage:
  100.0%` and `0 issues.`
```commit
refactor(rendering): model agents independently of output dialect
```

## Phase 2: Target capabilities and adapter outputs

- [ ] **Task 2.1: Complete target capabilities and registry.** In
  `internal/project/target.go`, extend the Phase 1 path/dialect descriptor with
  bridge and review-dispatch style fields. Register `pi` (`.pi/skills`, root
  Markdown reviewer skills, generic review), `gemini` (`.gemini/skills`,
  `.gemini/agents/*.md`, `GEMINI.md` bridge, native review), and `copilot`
  (`.github/skills`, `.github/agents/*.agent.md`, native review); preserve the
  Phase 1 Codex and existing Claude/Cursor mappings. Make unknown-target errors
  and `KnownTargets` derive their sorted names from the registry.
- [ ] **Task 2.2: Propagate the completed descriptor into render and drift.** In
  `internal/project/render.go`, pass the target's review style to skill rendering
  and fold the complete descriptor into affected config hashes. Classify Codex
  profiles as non-Markdown so dead-reference and managed-Markdown scans do not
  parse TOML as Markdown. Ensure bridge rendering chooses `CLAUDE.md` or
  `GEMINI.md` without rendering bridges for native-AGENTS targets. Add
  `templates/gemini/GEMINI.md.tmpl` containing the supported `@AGENTS.md` import
  and embed it in `templates/embed.go`.
- [ ] **Task 2.3: Make Pi review wording generic.** In every standard reviewing
  skill template that directs `adr-reviewer`, `plan-reviewer`, or `code-reviewer`
  dispatch, branch only on the target review style. Native targets retain current
  subagent wording. The Pi branch says to use the available reviewer or delegation
  mechanism, without requiring a separate session or asserting native subagents.
  Add target-render tests that assert the Pi text and reject both prohibited
  claims.
- [ ] **Task 2.4: Expand target tests and CLI coverage.** Update
  `internal/project/target_test.go` to parse all six target outputs, assert each
  exact path, assert neutral artifacts render once, assert Claude/Gemini bridges,
  and assert no bridge for Cursor, Codex, Pi, or Copilot. Update local-path,
  pruning, notes, and `cmd/awf/list_add_test.go` coverage for each target and the
  Copilot suffix. Preserve the target CLI's registry-derived list behavior.
- [ ] **Task 2.5: Verify and commit.** Run `./x gate` with expected `coverage:
  100.0%` and `0 issues.`
```commit
feat(rendering): add Pi Codex Gemini and Copilot targets
```

## Phase 3: Documentation, roadmap, and dogfood

- [ ] **Task 3.1: Update authored documentation.** Update `README.md`,
  `.awf/docs/parts/architecture/overview.md`,
  `.awf/domains/parts/{rendering,tooling}/current-state.md`,
  `.awf/parts/agents-doc/{identity,awf-setup}.md`, and
  `internal/configspec/spec.go` to list the six targets, their paths, dialects,
  and Pi limitation. Enable the managed roadmap doc in `.awf/config.yaml` and add
  its convention part with a Pi subagent orchestrator entry explicitly deferred
  from ADR-0122.
- [ ] **Task 3.2: Dogfood all targets.** Add `pi`, `codex`, `gemini`, and
  `copilot` to root and `examples/sundial/.awf/config.yaml`; add `.gitignore`
  negations for each new hidden output root. Run `./x sync` to generate every
  target tree, bridges, config reference, roadmap, domain docs, and both locks.
  Inspect that Codex profiles are TOML, Copilot agents use `.agent.md`, Pi
  reviewer files are top-level skills, and Gemini has `GEMINI.md`.
- [ ] **Task 3.3: Flip decision and plan lifecycle.** Set ADR-0122 and this plan
  to `Implemented`, add proof markers for `target-dialect-render`,
  `structured-agent-encoding`, and `pi-generic-review-dispatch` on their tests,
  then run `./x sync` to regenerate `ACTIVE.md` and domain indexes.
- [ ] **Task 3.4: Verify and commit.** Run `./x gate` and `./x check`; expected
  output includes `coverage: 100.0%`, `0 issues.`, and `awf check: clean`.
```commit
feat(awf): ship multi-runtime targets and agent encoders
```

## Verification

- `go test ./internal/project ./cmd/awf` parses every rendered Markdown and TOML
  agent target and verifies target path, bridge, hash, local-artifact, and prune
  behavior.
- `./x sync && ./x check && ./x gate` leaves both the repository and Sundial
  clean with all six adapter trees committed.

## Notes

A Pi orchestrator extension is deliberately out of scope. The roadmap entry is
its handoff point; it may use the stable Pi reviewer-skill paths introduced here.

Phase 1 was amended before implementation because a standalone TOML encoder would
violate the repository's dead-code gate. It therefore includes the minimal Codex
path/dialect registration and encoder selection needed to make that production
path reachable; Phase 2 completes the remaining adapter capabilities.
