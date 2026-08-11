---
format: plan-v2
date: 2026-08-11
adrs: [user-approved-adr-decision-boundaries]
status: Proposed
---
# Plan: User-Approved ADR Decision Boundaries

## Goal

Ship user-consent and minimum-sufficient-scope boundaries across ADR authoring and review without requiring efforts, mechanically interpreting ADR prose, or promoting implementation detail into durable authority.

## Architecture summary

Implement the linked ADR in two independently green inline transactions. First update the ADR guide, scaffold, and authoring workflow so prior consent and minimum-sufficient semantics constrain ADR mutation. Then update the shared review classifier, universal ADR lenses, evidence transport, and settled-summary disclosure. Each transaction updates its owning current-state claim and the pending ADR lifecycle in the same checked pair, updates standard sources before local overlays, renders every generated target, and verifies empty-variable publication safety.

## Phase 1: Constrain ADR authoring to accepted minimum semantics

**Execution mode: inline.**

Completes: ["authoring-boundary"]

### Task 1.1: Pin the authoring boundary in rendering tests
Applying: ["user-approved-adr-decision-boundaries:require-prior-user-acceptance", "user-approved-adr-decision-boundaries:keep-minimum-sufficient-semantics"]
Paths: ["internal/project/spine_test.go"]

Add focused assertions that the standard ADR guide, scaffold, brainstorming handoff, and proposing skill require explicit prior acceptance, decision-set closure, and the narrowest durable semantics; assert that suggestions remain outside the ADR until accepted and implementation detail remains plan or direct-execution content. Preserve the existing post-implementation, counterfactual, scaffold-frontmatter, and empty-variable checks. Run `go test ./internal/project -run 'Test(ProposingAdrTemplate|V3ADRTemplateEmptyDataFallback|BrainstormingTemplate)$'` before production edits and record the expected red evidence.

### Task 1.2: Update authoring sources and apply the routing claim
Kind: batch
Applying: ["user-approved-adr-decision-boundaries:require-prior-user-acceptance", "user-approved-adr-decision-boundaries:keep-minimum-sufficient-semantics"]
Paths: ["templates/adr-readme/README.md.tmpl", "templates/adr-template/template.md.tmpl", ".awf/parts/adr-template/body.md", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/skills/proposing-adr/SKILL.md.tmpl", ".awf/topics/parts/rendering/templates/current-state.md", "docs/decisions/user-approved-adr-decision-boundaries.md", "internal/project/spine_test.go", ".awf/awf.lock", "docs/decisions/INDEX.md", "docs/decisions/README.md", "docs/decisions/template.md", "docs/topics/rendering/templates.md", ".pi/skills/awf-brainstorming/SKILL.md", ".claude/skills/awf-brainstorming/SKILL.md", ".pi/skills/awf-proposing-adr/SKILL.md", ".claude/skills/awf-proposing-adr/SKILL.md"]
Representative: `templates/adr-readme/README.md.tmpl` and `templates/skills/proposing-adr/SKILL.md.tmpl` show the durable guide and mutation precondition.
Edge: `.awf/parts/adr-template/body.md` preserves awf's local scaffold override, and the rendered proposing skill keeps the effort-free approved-summary path without requiring memory.
Post-check: Run `go test ./internal/project -run 'Test(ProposingAdrTemplate|V3ADRTemplateEmptyDataFallback|BrainstormingTemplate)$'`, inspect the generated ADR guide, scaffold, brainstorming skill, and proposing skill for coherent consent and altitude semantics, then require `./x render && ./x check` to leave no drift.

Update the standard sources and awf's scaffold override so an ADR commitment requires explicit prior user acceptance and uses the narrowest durable semantics. Make proposing-adr refuse pre-consent mutation: effort-backed work takes accepted decisions from the Decision log and `Record:` evidence, while effort-free work takes the explicitly approved conversational summary; additional suggestions return to approval before insertion. Keep brainstorming responsible for presenting and approving the complete design, without making an ADR or effort mandatory. Update `rendering/templates:decision-artifact-routing`, its `Verify:` procedure, and provenance. Transition the pending ADR to Implementing and apply only `update rendering/templates:decision-artifact-routing` in this transaction. Render and semantically inspect all generated targets; do not add a prose classifier or alter terminal ADRs.

### Phase close

Close the authoring, claim, lifecycle, generated-output, and focused-test changes as one green transaction.

```commit
feat(rendering): enforce accepted ADR semantics
```

## Phase 2: Review ADR consent evidence and scope independently

**Execution mode: inline.**

Completes: ["review-boundary", "publication-contract"]

### Task 2.1: Pin adherence, scope, evidence, and remediation behavior
Applying: ["user-approved-adr-decision-boundaries:ground-adherence-in-consent-evidence", "user-approved-adr-decision-boundaries:review-adherence-and-scope-separately", "user-approved-adr-decision-boundaries:remove-surplus-and-disclose-refinements"]
Paths: ["internal/project/spine_test.go", "internal/catalog/batch_test.go"]

Replace assertions that leave consensus adherence idle for effort-free ADR work. Add focused cases proving that ADR review receives verbatim effort-memory user entries and `Record:` blocks when an effort exists, receives the explicitly approved design summary when it does not, and never treats repository facts as consent. Pin distinct universal decision-adherence and ADR-scope lenses, authority-preserving reasoned removal of surplus commitments, user-decision routing for changes to accepted semantics, and final summary disclosure of removals and semantics-preserving refinements. Keep the shared classifier single-homed and all reviewers report-only. Run `go test ./internal/catalog ./internal/project -run 'Test(AdrReviewerAgent|PlanReviewerAgent|CodeReviewerAgent|ReviewingAdrTemplate|MemoryLogConsumerCoverage|AuthorityGuidedReviewRemediation|PlanReviewerChangeSpecificExecutabilitySanctionsBatch)$'` before production edits and record the expected red evidence.

### Task 2.2: Update universal review sources and apply workflow claims
Kind: batch
Applying: ["user-approved-adr-decision-boundaries:ground-adherence-in-consent-evidence", "user-approved-adr-decision-boundaries:review-adherence-and-scope-separately", "user-approved-adr-decision-boundaries:remove-surplus-and-disclose-refinements"]
Paths: ["templates/agents/adr-reviewer.md.tmpl", "templates/partials/review-spine-head.md", "templates/skills/reviewing-adr/SKILL.md.tmpl", "internal/catalog/standard.go", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/user-approved-adr-decision-boundaries.md", "changelog/CHANGELOG.md", "internal/project/spine_test.go", "internal/catalog/batch_test.go", ".awf/awf.lock", "docs/decisions/INDEX.md", "docs/topics/rendering/workflow-skill-templates.md", ".pi/agents/adr-reviewer.md", ".claude/agents/adr-reviewer.md", ".pi/agents/plan-reviewer.md", ".claude/agents/plan-reviewer.md", ".pi/agents/code-reviewer.md", ".claude/agents/code-reviewer.md", ".pi/skills/awf-reviewing-adr/SKILL.md", ".claude/skills/awf-reviewing-adr/SKILL.md"]
Representative: `templates/agents/adr-reviewer.md.tmpl` owns the distinct lenses and `templates/skills/reviewing-adr/SKILL.md.tmpl` owns evidence transport and disclosure.
Edge: `templates/partials/review-spine-head.md` also renders into plan and code reviewers, so both targets of all three reviewers must retain coherent classification; empty effort-free evidence must never fabricate consent.
Post-check: Run `go test ./internal/catalog ./internal/project -run 'Test(AdrReviewerAgent|PlanReviewerAgent|CodeReviewerAgent|ReviewingAdrTemplate|MemoryLogConsumerCoverage|AuthorityGuidedReviewRemediation|PlanReviewerChangeSpecificExecutabilitySanctionsBatch)$'`; inspect generated ADR, plan, and code reviewer outputs for both enabled targets plus both reviewing-adr skills for distinct lenses, classification, evidence transport, contradictory fragments, consent-boundary preservation, and mandatory disclosure; record the inspected boundaries and result in phase completion evidence; require empty-variable renders to contain no `<no value>` token; then require `./x render && ./x check` to leave no drift.

Make decision-adherence and ADR-scope distinct universal lenses while retaining useful durable-commitment checks. Refine the shared classifier so removing an unaccepted surplus commitment is authority-preserving and reasoned, while contradicting or changing accepted semantics remains a user decision. Branch reviewing-adr evidence transport explicitly between effort-memory user provenance and the effort-free approved summary, and require its settled approval summary to inventory removed surplus commitments and semantics-preserving refinements, including none. Update `rendering/workflow-skill-templates:authority-guided-review-remediation` and `rendering/workflow-skill-templates:memory-log-consumer-coverage`, their proof coverage, provenance, and the Unreleased changelog. Apply those two remaining claim operations while leaving the ADR Implementing for deferred terminal closure. Render and semantically inspect every target.

### Phase close

Close the review semantics, evidence transport, claims, generated outputs, changelog, and focused-test changes as one green transaction.

```commit
feat(rendering): review ADR consent boundaries
```

## Definition of done

- `dod: authoring-boundary` Generated ADR guides, scaffolds, brainstorming guidance, and proposing workflows require prior user acceptance and minimum-sufficient durable semantics while routing implementation detail away from the ADR.
- `dod: review-boundary` Generated ADR review distinguishes adherence from scope, consumes the correct consent evidence in both effort modes, removes unauthorized surplus commitments without retroactive consent, and discloses reasoned refinements and removals.
- `dod: publication-contract` All three declared claim updates are applied in checked lifecycle pairs, every affected target renders coherent empty-variable prose, focused tests pass, `./x check` is clean, and the full gate retains 100% statement coverage.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.
