---
format: current-state-v2
status: Implemented
date: 2026-07-29
---
# ADR-0178: Explicit dependency composition architecture

## Context

awf has good local injection precedents but no repository-wide composition rule. Some packages accept
narrow dependencies at construction time: `internal/worktree` has a semantic Git runner,
`internal/effort` accepts clocks, UUID allocation, Git queries, and removal through `Options`, and
`internal/config` can parse an injected `TreeReader`. Other seams are package globals replaced by
tests, such as context-spill creation and removal, command working-directory discovery, and command
handler factories. Still other policy code calls `os` or `os/exec` directly. The result is not one
uniformly bad mechanism; it is an inconsistent dependency direction that makes the next seam depend
on whichever package happens to own the call today.

`internal/project.Open` is a representative boundary. It loads the config tree from disk, validates
and derives an effective catalog, resolves targets, and asks native Git for the primary control root.
It therefore combines policy assembly with filesystem access, Git/process access, and the
non-filesystem catalog input. `cmd/awf/runSync` and every existing post-mutation render converge on
that open path, making it a useful first vertical slice: one composition change reaches top-level
render, init rendering, upgrade rendering, and enable/disable rendering without inventing an unused
framework.

The design must preserve several constraints. Policy packages continue to own their models and
invariants. External mechanisms must not leak their representations inward. A seam must improve a
current consumer, not merely anticipate reuse. Interfaces are not a goal by themselves; a function
or immutable value is the smaller dependency when the consumer needs one operation. Tests must use
real production boundaries without making production APIs test-shaped, and the 100 percent coverage
gate does not justify injecting an otherwise unreachable compile-time-catalog failure.

The current documentation has no durable home for these cross-package rules. The rendering,
tooling, config, invariants, and ADR-system domains describe product areas, while dependency
composition is repository-wide code-design authority. A path-scoped domain would either claim code
it does not own or require overlapping topic selectors across most of the tree. The current-state
model already supports the required shape: a pathless owning domain and one explicitly global topic.

Two adjacent governance surfaces also need deliberate handling. Agent sidecar lists replace catalog
defaults wholesale, so adding a dependency-composition review hint without comparing and backfilling
the defaults can silently drop existing review obligations. And the domain-aligned commit taxonomy
has no `code-design` scope. Structural dependency work should use Conventional Commits' existing
`refactor` type rather than confusing a type with a new `refactor` scope.

This decision establishes the architecture and a working foundation only. The paused
filesystem/global-seam refactor is a downstream consumer and remains outside this effort; a
repository-wide conversion would violate the requirement that every introduced capability have one
concrete first consumer.

## Decision

1. Add a pathless `code-design` domain as the owner of repository-wide code-structure guidance. Add
   `code-design/dependency-composition` with `applies: global`; the topic remains stored under its
   owner but applies throughout the repository. The domain declares no `paths`, so it creates neither
   path-topic fanout nor a domain coverage obligation. The topic's identified claims are the durable
   authority for the rules below; this ADR remains historical rationale.

2. Compose volatile dependencies at the outermost layer that has enough knowledge to choose their
   production mechanisms. Policy packages receive already-selected semantic dependencies and do not
   discover process-wide implementations through mutable package globals. Production construction is
   explicit and centralized at the executable or application boundary; tests construct the same
   consumer with controlled dependencies. This is a dependency-direction rule, not a requirement for
   a universal container, service locator, or repository-wide `Dependencies` bag.

3. Let each consumer own the narrowest contract that expresses what it needs. Name dependencies by
   semantic operation rather than by mechanism: for example, resolve the resident root rather than
   expose arbitrary Git commands. Keep mechanism adapters outside the policy package they serve and
   translate mechanism-specific values and errors at that boundary. Do not move policy into an
   adapter merely because the adapter performs the call.

4. Prefer direct function and immutable-value injection for one-operation dependencies. Introduce an
   interface only when the consumer needs a cohesive multi-operation behavioural contract whose
   relationship has domain meaning. Constructors reject missing required dependencies rather than
   silently selecting production globals. Tests use per-instance fakes or functions, remain safe to
   run in parallel, and do not swap shared package variables.

5. Every new composition capability must name one concrete first consumer in the same green
   transaction. Do not add adapters, constructors, interface methods, option fields, or composition
   helpers for anticipated reuse. A later consumer may reuse or reshape an existing boundary when it
   arrives; textual similarity alone does not justify a shared abstraction.

6. Establish the first vertical slice with a composed `project.Loader`. The Loader receives the
   config-tree loader, the standard catalog value, and a semantic invoking-root-to-resident-root
   resolver needed by project opening. After load it invokes `Config.Validate`, preserving the
   current distinction between decoding and semantic validation. It continues to own target
   resolution, effective-catalog derivation, standard-catalog defense-in-depth validation, and
   effective catalog conformance because those are project policy, not adapter work. Its production
   control-root adapter closes over `context.Background()` and preserves the current best-effort
   fallback to the invoking root. Cancellation is absent from the Loader contract until a
   signal-aware or embedded caller actually owns it.

7. Make `runSync` the Loader's single first explicitly composed consumer. Compose the production
   Loader once on that wiring path and route `runSync`, initialized sync, and the shared printing
   helper through it, so top-level render and every existing post-mutation render use the same
   assembly. Refactor the present `project.Open` body into one Loader-owned opening implementation;
   keep `project.Open` as a documented transitional compatibility wrapper that constructs production
   dependencies and delegates to that implementation for existing command and test callers. It is
   the only grandfathered in-policy production constructor, gains no new caller, and must disappear
   as concrete consumers later receive outer composition; new code uses an explicitly composed
   Loader. This keeps one copy of loading and validation policy without expanding this foundation
   into an all-command conversion.

   No second command is converted merely to demonstrate reuse. Focused tests inject Loader
   dependencies and exercise reachable config-load, config-validation, resident-root,
   effective-catalog, and conformance outcomes. Keep the compile-time standard-catalog validator as
   production defense in depth; do not make it an injectable dependency solely to test its
   unreachable failure.

8. Make the authority visible without copying its normative prose into prompts. Rely on the global
   topic for repository-wide current-state context, and add concise workflow and reviewer hints that
   direct affected agents to `code-design/dependency-composition`. For every list-valued agent data
   override touched by the implementation, compare it with the catalog default and explicitly
   preserve every default unless its removal is deliberate and documented. In particular, restore
   the ADR reviewer's omitted `decision-clarity` and `consequences-honesty` focus items and the generic
   documentation-currency checks that its wholesale overrides currently drop. Where a convention
   part should append a hint to an existing skill or agent section, use the `sectionDefault`
   substitution channel rather than replacing the generic contract. Preserve missingkey-zero
   publication safety, and render affected prompts with empty-string variables to verify that no
   unresolved-value or no-value token appears.

9. Add `code-design` to `audit.allowedScopes` with the meaning "dependency composition and
   cross-package code structure" and render every derived scope surface in the same implementation
   transaction. Dependency-composition structural commits use the existing `refactor` Conventional
   Commit type with the `code-design` scope. Do not add `refactor` as a scope: type and ownership are
   separate dimensions, and non-refactor changes in this domain retain their appropriate existing
   type.

10. Update documentation with the foundation. Correct `docs/architecture.md`'s stale statement that
    awf and its tests need no host Git binary: native Git is required by repository, effort, and
    worktree operations even though `go-git` remains the audit implementation. Describe the
    composition root and representative Loader boundary in architecture and development guidance,
    and update generated domain/topic, scope, workflow, agent, config-reference, and glossary
    surfaces driven by their authored `.awf/` inputs. Every Accepted, Implementing, and Implemented
    status transition runs `./x render` and commits the regenerated `docs/decisions/INDEX.md` and lock
    output with the transition.

11. Defer wholesale seam conversion. Record the paused filesystem/global-seam work as a downstream
    implementation consumer of this authority, not a phase of this decision. Each future conversion
    must identify its consumer, preserve the owning package's policy, remove the replaced global seam
    in the same transaction, and satisfy the same current-state rules. The implementation plan for
    this ADR covers only the domain/topic authority, awareness and scope governance, documentation,
    Loader composition, and its direct tests.

## State changes

- add `code-design/dependency-composition:outer-composition`
- add `code-design/dependency-composition:consumer-owned-contracts`
- add `code-design/dependency-composition:mechanism-adapters`
- add `code-design/dependency-composition:direct-injection-first`
- add `code-design/dependency-composition:concrete-first-consumer`
- add `code-design/dependency-composition:sync-project-loader-wiring`
- add `code-design/dependency-composition:dependency-composition-commit-classification`

## Consequences

awf gains one explicit dependency direction that can guide later refactors without forcing every
package into one framework. The first slice is useful immediately: render and post-mutation render
share controlled project-opening dependencies, and their error paths can be tested without process
state or package-global swaps. The global topic gives implementation and review one current authority
while the pathless domain avoids claiming ownership of the whole source tree.

The design adds visible wiring and constructor parameters. Small direct calls may become a function
field plus a production adapter, and consumers must name their needs precisely. That cost is accepted
where volatility or testing pressure is real; item 5 prevents paying it speculatively. Consumer-owned
contracts can duplicate similar-looking function types until shared policy, not syntax, proves they
are one concept.

A global topic is high-authority guidance, so vague claims would create repository-wide noise. The
identified rules keep it bounded to dependency selection, direction, and first-consumer discipline.
Implementation review must verify each new claim independently; the Loader wiring claim is backed by
behavioural tests, while the design rules remain reasoned contracts with concrete verification
instructions when authored.

Adding a domain and commit scope expands governance surfaces. Rendered configuration reference,
workflow, agent guide, domain navigation, lock data, and reviewer guidance must move together. The
wholesale-list audit is deliberate implementation work, not incidental cleanup, because an appended
review hint can otherwise erase generic defaults silently.

The architecture remains transitional. Existing package-global seams and direct mechanism calls are
not suddenly nonconforming implementation debt to sweep in this effort; they become bounded future
conversion candidates. The paused seam refactor can resume against settled authority after this ADR
and its foundation are implemented and reviewed.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Introduce a universal dependency container or service locator | It hides dependency direction, weakens compile-time visibility, and creates capabilities with no first consumer. |
| Define one repository-wide filesystem and process interface | Consumers need different semantic operations; a universal mechanism-shaped interface would leak representation and grow speculatively. |
| Standardize on interfaces for every seam | Single-operation dependencies are clearer as functions or values, while interfaces add naming, fake, and method-set cost without new meaning. |
| Keep package-global function swaps as the standard test seam | They make dependencies implicit, prevent parallel-safe tests, and let production construction vary through mutable process state. |
| Put composition inside each policy package through defaulting options | Hidden defaults preserve convenience at the cost of making production selection implicit and split across packages. |
| Convert every existing seam in this effort | The scope would be repository-wide, obscure whether the architecture works through a representative slice, and violate the concrete-first-consumer constraint. |
| Give `code-design` repository-wide path ownership | It would overlap every product domain, create coverage and fanout pressure, and confuse code structure authority with file ownership. |
| Store the topic under an existing product domain | Dependency composition is not owned by rendering, tooling, config, invariants, or ADR-system; choosing one would misdirect authority. |
| Add `refactor` as the commit scope | `refactor` already expresses change type. The scope should identify the code-design owner, not duplicate the type. |

## Status history

- 2026-07-29: Proposed
- 2026-07-30: Accepted; content-sha256: 49f987e32c2c08dacb656500fe87c393933cf071f34e2a2e2ed66f407a465814
- 2026-07-30: Implementing; content-sha256: 49f987e32c2c08dacb656500fe87c393933cf071f34e2a2e2ed66f407a465814
- 2026-07-30: Applied; operations: add `code-design/dependency-composition:outer-composition`, add `code-design/dependency-composition:consumer-owned-contracts`, add `code-design/dependency-composition:mechanism-adapters`, add `code-design/dependency-composition:direct-injection-first`, add `code-design/dependency-composition:concrete-first-consumer`, add `code-design/dependency-composition:dependency-composition-commit-classification`
- 2026-07-30: Applied; operations: add `code-design/dependency-composition:sync-project-loader-wiring`
- 2026-07-30: Implemented; content-sha256: 49f987e32c2c08dacb656500fe87c393933cf071f34e2a2e2ed66f407a465814
