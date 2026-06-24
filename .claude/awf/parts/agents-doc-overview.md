This repository contains `awf` — the **Agentic Workflows** CLI and standard. `awf` scaffolds, renders, and drift-checks a suite of Claude Code skills, review agents, and git hooks into any Go project from a single `.claude/awf.yaml` config file. It is both the tool that publishes the standard and the first adopter of it.

The full workflow chain (brainstorming → ADR → plan → execution → review) is expressed as project-owned skill files under `.claude/skills/awf-*/` and review agents under `.claude/agents/`. Hooks under `.githooks/` enforce the gate before commits and pushes land.
