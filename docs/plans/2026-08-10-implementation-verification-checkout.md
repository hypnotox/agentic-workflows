---
format: plan-v2
date: 2026-08-10
adrs: [260]
status: Proposed
---
# Plan: Implementation Verification Checkout

## Goal

Make Pi implementation commit policy verify the caller-selected root or managed-worktree checkout without changing either Pi process CWD, and retire the false-monitor pitfall with complete recovery guidance. Binding ordinary mutations to an effort checkout and changing the cross-runtime implementer contract are non-goals.

## Architecture summary

One inline phase keeps the test-first runtime change, parent-facing workflow guidance, four current-state claim updates, and generated-tree transaction in one independently green commit. The extension resolves only the optional selected checkout, compares its canonical Git checkout-root and common-directory identity with the project root, and feeds that identity to the existing snapshots while leaving `RunRequest.cwd` unchanged. Existing Git topology parsing stays in `internal/git`; no TypeScript worktree-list parser or child-report parser is added.

## Phase 1: Scope implementation commit verification

**Execution mode: inline.**

Completes: ["checkout-policy-correct", "workflow-and-authority-current"]

### Task 1.1: Pin checkout-scoped policy before production changes
Latitude: exact
Applying: ["verification-checkout-for-implementation-commit-policy:verification-identity", "verification-checkout-for-implementation-commit-policy:registered-worktree-boundary", "verification-checkout-for-implementation-commit-policy:checkout-scoped-policy", "verification-checkout-for-implementation-commit-policy:explicit-caller-selection"]
Paths: ["tools/pi-extension-test/tests/index.test.ts", "internal/project/target_test.go", "internal/project/phase_transaction_ownership_test.go"]

Extend the TypeScript harness to record every Git command and `cwd` independently from each runner request. Add failing cases for omitted root verification; a valid linked worktree whose HEAD advances while root HEAD stays fixed; a selected-worktree forbidden commit; one-leading-`@` and symlink canonicalization; and pre-dispatch refusal of empty-after-normalization, missing, non-Git, subdirectory, stale, and foreign-repository identities. Assert that successful and failed results expose the resolved verification checkout, that the unchanged-HEAD diagnostic names it and gives the explicit retry repair, and that every runner request still uses the project root. Extend generated-template and workflow-surface backing tests to pin the public field, the no-CWD-change boundary, and managed-worktree caller guidance without treating the generic implementer agent as a consumer.

### Task 1.2: Resolve verification identity and apply existing policy there
Latitude: exact
Applying: ["verification-checkout-for-implementation-commit-policy:verification-identity", "verification-checkout-for-implementation-commit-policy:registered-worktree-boundary", "verification-checkout-for-implementation-commit-policy:checkout-scoped-policy"]
Paths: ["templates/pi/awf-subagents/index.ts.tmpl", ".pi/extensions/awf-subagents/index.ts", "tools/pi-extension-test/tests/index.test.ts", "internal/project/target_test.go"]

Add optional `verificationCheckout` only to `subagent_implement`, including call options and structured details. Normalize one leading `@`, reject an empty result, resolve relative values from the project root, canonicalize the selected path, and use Git checkout-root plus absolute common-directory identity probes to require an exact same-repository checkout root before dispatch. Keep the default root path and existing non-Git unverifiable behavior when the field is omitted. Feed the resolved verification identity only to both snapshots and both permission directions; keep role loading, queueing, runner construction, and child `cwd` rooted at the project root. Do not invoke `git worktree list`, parse topology, infer effort association, or trust child output.

### Task 1.3: Publish workflow guidance, authority, and recovery
Kind: batch
Latitude: exact
Applying: ["verification-checkout-for-implementation-commit-policy:explicit-caller-selection", "verification-checkout-for-implementation-commit-policy:checkout-scoped-policy"]
Paths: ["templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", ".pi/skills/awf-subagent-driven-development/SKILL.md", ".claude/skills/awf-subagent-driven-development/SKILL.md", ".pi/skills/awf-executing-plans/SKILL.md", ".claude/skills/awf-executing-plans/SKILL.md", "templates/docs/working-with-awf.md.tmpl", "docs/working-with-awf.md", "README.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/topics/rendering/pi-runtime.md", "docs/topics/rendering/pi-workflows.md", "docs/topics/rendering/workflow-skill-templates.md", ".awf/docs/pitfalls.yaml", "docs/pitfalls.md", "changelog/CHANGELOG.md", "docs/decisions/0260-verification-checkout-for-implementation-commit-policy.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: The Pi phase-owner dispatch names `verificationCheckout` for a supplied managed-worktree path while the Claude branch remains runtime-neutral.
Edge: A project override may keep `docs/working-with-awf.md` byte-identical even though the generic template and lock relationship change; inspect both authored and produced boundaries.
Post-check: Run `./x render && ./x check`; require the four updated claims to render with `Revised-by: ADR-0260`, their existing proof markers to exercise every added clause, the managed-worktree monitor pitfall to be absent from both source and rendered catalog, the adjacent mutation-routing roadmap entry to remain present, and the Unreleased changelog plus README and generic working guide to describe verification identity without claiming mutation binding or CWD changes. Read the rendered Pi skills, current-state claims, README, working-guide template output expectations, changelog, and pitfalls boundary together; reject contradictory checkout-routing language, Pi-only wording in Claude branches, unresolved/no-value tokens, or a second topology parser.

Update Pi branches of both phase-owner and commit-disabled-helper guidance so callers set `verificationCheckout` when operations intentionally target a supplied managed-worktree path and omit it for root work; keep actual mutation paths explicit in each task. Document the optional field and root default in the generic working guide and README. Update all four declared claims with preserved provenance plus `Revised-by: ADR-0260`, transition ADR-0260 to Implementing, and apply the four operations atomically with their claim mutations. Remove the resolved pitfall because the fixed monitor and its unchanged-HEAD diagnostic now provide complete recovery; preserve the separate mutation-routing roadmap item. Add the adopter-facing Unreleased entry, render every generated output, and perform the focused semantic meaning review required for generated prose.

### Phase close

The runtime, its falsifiable checkout matrix, caller guidance, current-state authority, pitfall retirement, and adopter documentation form one coherent behavior change.

```commit
fix(rendering): verify selected checkout (applies 0260 batch)
```

## Definition of done

- `dod: checkout-policy-correct` A managed-worktree commit satisfies commit-capable verification while root HEAD remains unchanged, forbidden commits are detected in the same selected checkout, invalid explicit identities refuse before dispatch, and parent and child Pi CWDs remain the project root.
- `dod: workflow-and-authority-current` Managed-worktree callers select the verification identity while retaining explicit mutation paths; all four ADR operations are Applied with backed current-state claims, adopter documentation and changelog are current, the resolved pitfall is removed, the separate mutation-routing gap remains, and render, check, and gate are green.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- 2026-08-10 Phase 1 semantic review: inspected the rendered Pi phase-owner skills, byte-stable Claude counterparts, rendered current-state claims, README, generic working-guide template and project override boundary, changelog, and source/rendered pitfalls catalog together. The outputs consistently describe verification identity without mutation binding or CWD changes, contain no Pi-only Claude wording or unresolved/no-value token, preserve the separate mutation-routing roadmap item, and remove the resolved monitor pitfall. The project-specific `docs/working-with-awf.md` override remained byte-identical while the generic template and lock relationship changed, as anticipated.
- 2026-08-10 Phase 1 review: mechanical settlement preserves filesystem error causes and distinguishes raw-empty and exactly-one-leading-`@` cases, adds explicit omitted-root non-Git behavior, and connects the named Go proof to the TypeScript behavior census. A reasoned prose correction separates rooted CWD and role loading from unchanged task-explicit mutation routing. One authority conflict remains: checkout-root plus common-directory Git queries accept a copied linked-worktree `.git` pointer at an unregistered path, while backlink validation would parse worktree topology contrary to the approved boundary; implementation cannot settle until that choice is approved.
