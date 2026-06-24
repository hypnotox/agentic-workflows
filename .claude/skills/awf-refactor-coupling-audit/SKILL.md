---
name: awf-refactor-coupling-audit
description: >
  Use before finalising the scope of a refactor ADR that moves files between
  packages or inverts dependencies. Runs the 6-category coupling audit so
  findings land in the ADR Context section before the Decision is drafted.
  Self-contained; does not gate the workflow chain.
---

# awf-refactor-coupling-audit

A task skill for refactor ADRs. Runs (or dispatches) the 6-category coupling audit that `AGENTS.md` "Refactor playbook" mandates before the ADR scope is finalised. The audit's output is a structured listing that lands in the ADR's Context section so scope reflects the real coupling surface, not the assumed one.

## When to invoke

Before finalising the Context section of a refactor ADR that:

1. Moves a file (or set of files) between packages.
1. Inverts a dependency (introduces an interface in the original package so a subpackage can hold the implementation).
1. Renames or relocates a type with cross-package callers.

Skip when the refactor is contained within one package (no cross-package coupling to audit) or when the refactor is a pure rename without coupling shifts.

This is a **task skill**: it sits off the workflow chain and does not gate it. Its output feeds the ADR's Context section before `awf-proposing-adr` finalises the Decision.

## Procedure

### Audit shape

1. **Pick the audit shape.** For a small-scope refactor (1–3 files), run the audit inline as a sequence of `grep` / Read calls in the main session. For a large-scope refactor (10+ files or coupling surfaces across 5+ packages), dispatch a single `Explore` subagent via the Agent tool to absorb the grep transcript noise per `AGENTS.md` "Use subagents for exploration and file-heavy work".

### 1. Top-level package files

1. Grep the **original** package for concrete type references, function calls, and method invocations against each symbol being moved.

For each hit, decide: does this caller need to update its import path after the move? Can it stay where it is, or does it move with the symbol?

```bash
grep -rn "<MovedSymbol>" <original-package-path>/ --include='*.go' | grep -v _test.go
```

### 2. Sibling test files

1. Grep `*_test.go` separately from production code. Test files often use moved symbols as helpers in **unrelated** tests; the test-coupling profile differs from production coupling and is routinely larger.

Capture **N** (production call sites) and **M** (test call sites) separately — M is typically larger than N and is the bigger implementation surprise if not enumerated up-front.

```bash
grep -rn "<MovedSymbol>" <original-package-path>/ --include='*_test.go'
```

### 3. Subpackages

1. Existing subpackages that import the symbol under its current path will need import updates after the move. Enumerate them; check for would-cycle cases.

```bash
grep -rn "\"agentic-workflows/<original-package-path>\"" <original-package-path>/
```

For each subpackage hit, decide: does the subpackage's import path remain valid after the move, or does it cycle? If a cycle would result, the refactor needs an interface inversion in the original package (see category 6).





### 6. `init()` ordering and cross-package method visibility

1. Methods on a moved type with cross-package callers cannot move without exporting (Go visibility) or inverting (interface in the original package).

```bash
grep -rn "^func (\w\+ \*<MovedType>)" --include='*.go'
```

For each method: is it called from a sibling package? Then it needs to remain reachable — either the type stays exported in its current package, or an interface is introduced in the original package with an implementation in the destination.

`init()` ordering across packages is hard to reason about. Flag any cross-package `init()` chains the move would break — registry seeding and global-state setup are common load-bearing sites.

## Test-coupling planning rule

When moving a source file with **N** tests co-located, also enumerate the **M** tests in *other* files that use the moved symbols as setup helpers. **M is typically larger than N.** A common surprise: production code moves cleanly (N small), test code explodes (M large), and the refactor ships with a sprawling test diff that was not on the radar.

Capture both numbers in the ADR Context section.

## Output

The audit's output goes into the ADR's **Context** section under a "Coupling audit" subsection listing each category before the Decision is drafted:

```
### Coupling audit

- Top-level callers: <list of file:line, or "none">
- Sibling tests: N=<number>, M=<number>
- Subpackage imports: <list or "none">
- Codegen sites: <list or "none">
- Constructor paths: <list or "none">
- Cross-package methods / init(): <list or "none">
```

## Scope shrink rule

If the audit reveals the refactor is larger than the ADR's originally proposed scope, **shrink the scope** with a `docs(adr): defer X` amendment to the Context section before implementation starts. Do not proceed with an underscoped ADR.

## Notes

1. Authoritative source for the audit categories: `AGENTS.md` "Refactor playbook". This skill is a procedural pointer, not a contract restatement — when the playbook prose evolves, follow the prose.
1. Does not commit on your behalf; its output is content the ADR author lands into the ADR.
1. For recurring coupling surprises specific to this project, see the `replaceWith` sections above; fill those in when patterns repeat.
