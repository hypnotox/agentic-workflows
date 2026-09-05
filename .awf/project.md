---
format: 1
---

# Project guidance

You are a coding agent responsible for developing and maintaining this project. Own both the immediate task and the project's long-term health.

## Identity

`awf` is a public pre-1.0 Go CLI at `github.com/hypnotox/agentic-workflows`. It projects a small `.awf` source tree into fixed agent guidance, routes repository paths to current topics, and keeps optional local effort memory.

## Invariants

- Keep the product direct and small. Apply KISS and YAGNI; do not add policy engines, compatibility layers, or speculative abstractions.
- Never overwrite an unmarked repository-owned file or automatically delete a retired generated file.
- Keep AWF usable without Git, external skills, services, or network access after its pinned binary is available.
- Update current documentation and topic guidance with the behavior they describe.
- Edit `.awf/project.md` and `.awf/topics/**/*.md`, then run `./x render && ./x check`; do not edit generated agent guidance directly.

## Workflow

Use `./x resolve <path>...` before substantial changes and read every returned topic. Keep implementation decisions in active effort memory while work is in progress; fold still-needed guidance into its owning topic before finishing. Prefer ordinary repository tools and direct code. Use Conventional Commits with one concern per commit.

The generated root `./awf` wrapper intentionally exercises the released bootstrap path. During AWF development, use the dogfooding `./x` commands, which run the checkout source directly.

## Commands

```text
./x test: run the complete Go test suite
./x gate: format-check, test, and build the repository
./x render: render from the checkout source
./x check: check with the checkout source
./x resolve <path>...: resolve current topics with the checkout source
./x effort <command>...: manage local effort memory with the checkout source
./x build: build bin/awf
```

## Documentation

- `README.md`: product contract, installation, and daily use.
- `MIGRATING-v0.50.md`: one-time manual migration guide for existing adopters.
- `.awf/topics/**/*.md`: path-routed implementation guidance.
- `CHANGELOG.md`: release history.
