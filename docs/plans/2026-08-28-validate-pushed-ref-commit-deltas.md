---
format: plan-v2
date: 2026-08-28
adrs: [validate-only-pushed-ref-commit-deltas]
status: Proposed
---
# Plan: Validate pushed ref commit deltas

## Goal

Make the generated pre-push hook validate only commits introduced by the updates being pushed, so already-accepted remote history cannot block conforming descendants, while preserving fail-closed evidence handling and the existing full-gate mutation evidence. Changing explicit commit-policy preview, reference-transaction selection, the grandfather baseline, configuration schema, or server-side policy is out of scope.

## Architecture summary

The pre-push payload buffers and validates the complete protocol input before policy or gate execution. Existing destination refs contribute `remote_oid..peeled_local_tip`; new commit-bearing branches and tags contribute the range from a freshly queried destination `integrationBranch` tip to the recursively peeled local commit. Deletions contribute no policy commits. The common commit-policy verifier expands the complete range union and retains one evaluation per commit, while malformed object IDs, missing objects, unpeelable tags, and unavailable or unresolvable required remote integration evidence refuse before the gate. Non-commit targets contribute no commits with an explicit note.

The configured full gate remains after successful policy validation and retains its separate per-update `--range` evidence contract for mutation selection. The publisher projects `integrationBranch` into template data and folds it into the config hash only for artifacts that consume it, so a branch change regenerates the hook without reflagging unrelated outputs. Template interpolation retains the `missingkey=zero` renderer without introducing another Git or policy owner, and unset variables must still produce a coherent executable payload with no unresolved or no-value tokens.

## Phase 1: Select and enforce pushed commit deltas

**Execution mode: inline.**

Completes: ["pushed-ref-delta-policy"]

### Task 1.1: Establish failing pushed-delta hook regressions
Applying: ["validate-only-pushed-ref-commit-deltas:pushed-ref-delta-enforcement", "validate-only-pushed-ref-commit-deltas:pushed-ref-delta-evidence"]
Paths: ["internal/project/hooks_test.go"]

Extend the rendered-payload and native-Git harnesses before changing the template. Observe focused failures proving that an existing ref selects only commits in its advertised-old-tip delta; a new branch and a new annotated tag use a freshly queried destination integration-branch tip; a force update cannot omit newly reachable commits; deletions select no policy commits; and multiple updates form a deduplicated union. Include recursive tag peeling, non-commit targets, malformed records and object IDs, missing local objects, unavailable or unresolvable integration-tip evidence, and policy refusal before any gate execution. Preserve separate assertions for the unchanged gate `--range` arguments, explicit-preview behavior, reference-transaction behavior, and worktree-local policy resolution.

### Task 1.2: Render and apply the delta-selecting payload
Kind: batch
Applying: ["validate-only-pushed-ref-commit-deltas:pushed-ref-delta-enforcement", "validate-only-pushed-ref-commit-deltas:pushed-ref-delta-evidence", "validate-only-pushed-ref-commit-deltas:pushed-ref-delta-template-safety"]
Paths: ["templates/hooks/pre-push.sh.tmpl", ".awf/hooks/pre-push.sh", "internal/project/hooks_test.go", "internal/project/publication_safe_template_test.go", "internal/publisher/render.go", "internal/publisher/render_test.go", "internal/publisher/confighash.go", "internal/publisher/confighash_test.go", "internal/render/vars.go", "internal/render/vars_test.go", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", "docs/topics/rendering/singletons-and-payloads.md", "changelog/CHANGELOG.md", "docs/decisions/0315-validate-only-pushed-ref-commit-deltas.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: ["an existing branch whose remote ancestry contains a nonconforming commit but whose pushed descendant range conforms", "a new annotated tag peeled to a commit divergent from the freshly resolved integration tip"]
Edge: ["all hook input is validated before the first policy or gate command", "remote-tracking refs never substitute for destination evidence", "deletions do not trigger integration-tip lookup", "the common verifier receives range union evidence rather than complete local targets", "the full gate retains its independent mutation ranges", "only templates that consume integrationBranch change config hash when it changes", "unset template variables render without unresolved or no-value tokens"]
Post-check: "Run the focused project hook, publisher render/config-hash, render reference-detection, and publication-safety suites against configured and unset-variable renders, including real existing-ref, new-branch, new-tag, force-update, deletion, multi-update, missing-integration-tip, malformed-object, recursive-peel, non-commit, policy-before-gate, and worktree-policy cases. Require policy logs to contain exactly the introduced commit union, require the gate logs to preserve their separate exact range evidence, and prove changing `integrationBranch` reflags and regenerates the pre-push consumer without changing unrelated artifact hashes. Render from `.awf/`, inspect the generated pre-push payload and rendering topic for coherent meaning, and require `./x check` plus affected-package feedback to finish cleanly."

Replace complete-target policy selection in the authored pre-push template with the ADR-defined delta evidence while retaining one Git-facing hook owner and the existing common verifier. Extend publisher render data with the configured integration branch and make config-hash consumption detection specific to templates that read it, with direct projection, consumer, non-consumer, and template-comment tests. After the red regressions pass, transition ADR-0315 to Implementing and apply its sole `rendering/singletons-and-payloads:commit-policy-hook-payloads` update in this transaction; update the source claim, generated topic, Unreleased changelog, rendered payload, index, and lock together. The claim must distinguish local reference-transaction ranges from pushed remote deltas and retain inert wiring, invoking-worktree resolution, malformed-input refusal, and policy-before-gate behavior.

### Phase close

```commit
fix(rendering): validate pushed ref deltas (applies 0315 batch)
```

## Definition of done

- `dod: pushed-ref-delta-policy` A push validates each commit newly introduced by its advertised ref updates exactly once, does not revalidate history already reachable from the authoritative remote base, fails closed when required exact evidence is unavailable, and runs the unchanged full gate only after policy success.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record implementation findings and any material route deviations here.

- Plan review reasoned finding: the initial draft did not include the publisher render-data and consumer-scoped config-hash owners required to make `integrationBranch` available without stale generated hooks. Resolved by adding those owners, their reference detector, focused projection/hash tests, and the requirement that unrelated outputs remain byte-stable.
