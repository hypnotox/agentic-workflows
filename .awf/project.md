---
format: 2
---

# Project guidance

You are a coding agent responsible for developing and maintaining this project. Own both the immediate task and the project's long-term health.

## Identity

`awf` is a public pre-1.0 Go CLI at `github.com/hypnotox/agentic-workflows`. It projects repository-owned sources into fixed agent guidance, routes repository paths to current topics, optionally inspects routing coverage, keeps local effort memory, offers create-only intent, specification, plan, ADR, and topic starters, and ships embedded adopter guides plus a verified public launcher.

## Invariants

- Keep the product direct and small. Apply KISS and YAGNI; do not add policy engines, compatibility layers, or speculative abstractions.
- Never overwrite an unmarked repository-owned file or automatically delete a retired generated file.
- Keep AWF usable without Git, external skills, services, or network access after its pinned binary is available.
- Update current documentation and topic guidance with the behavior they describe.
- Edit `.awf/project.md` and `docs/topics/**/*.md`, then run `./x render && ./x check`; do not edit generated agent guidance directly.

## Workflow

Use `./x resolve` when repository context is needed: bare for explicit global topics or with paths for globals plus matching topics. Read every returned topic. Once the applicable current context is known, do not query again before every edit. Use `./x docs topics` and `./x docs effort` for the full repository workflows when they become relevant. Keep active effort memory current while work is in progress, compare the result with the agreed outcome and criteria, reconcile active ADRs and topic links, and retain only concrete reusable lessons. Prefer ordinary repository tools and direct code. Use Conventional Commits with one concern per commit.

The generated root `./awf` wrapper intentionally exercises the released bootstrap path. During AWF development, use the dogfooding `./x` commands, which run the checkout source directly.

## Commands

```text
./x test: run the complete Go test suite
./x gate: format-check, test, and build the repository
./x render: render from the checkout source
./x check: check with the checkout source
./x resolve [<path>...]: resolve current topics with the checkout source
./x resolve --coverage <path>...: inspect specific topic routing for explicit paths
./x docs [integration|topics|effort]: read embedded guides from the checkout source
./x new effort <slug>: create local effort memory
./x new intent <slug>: create a tracked change intent
./x new spec <slug>: create a tracked change specification
./x new plan <slug>: create a tracked implementation plan
./x new adr <slug>: create a pending decision record
./x new topic <id> <pattern>...: create a path-routed topic
./x effort list|show|finish ...: inspect or archive local effort memory
./x build: build bin/awf
```

## Documentation

- `internal/docs/*.md`: canonical embedded adopter guides.
- `README.md`: introduction, public entrypoint, and documentation index.
- `MIGRATING-v0.50.md`: one-time manual migration guide for existing adopters.
- `docs/topics/**/*.md`: path-routed current implementation guidance.
- `docs/changes/<slug>/`: author-owned intent and specification defining a change.
- `docs/plans/`: implementation plans derived from an agreed change or other established basis.
- `docs/decisions/`: enduring choices and rationale.
- `CHANGELOG.md`: release history.
