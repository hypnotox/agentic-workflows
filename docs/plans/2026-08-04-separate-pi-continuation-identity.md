---
format: plan-v2
date: 2026-08-04
adrs:
  - separate-pi-continuation-context-from-user-and-routing-identity
status: Proposed
---
# Plan: Separate Pi Continuation Identity

## Goal

Implement the linked ADR in awf so a fresh Pi continuation is durably agent-owned and effort
presentation uses only the external display-suffix event surface while awf metadata and routing
identity remain independent.

Do not change the public `{kickoff}` schema or its bound, add a Pi message role, make presentation or
metadata routing authority, retain the name-override fallback, import a foreign package, prescribe
another repository's implementation, or refactor unrelated handoff, effort-activity, or application
code.

## Architecture summary

The generated awf handoff template remains the owner of queueing, countdown, replacement, and
recovery, but translates accepted kickoff prose once into a visible `agent-handoff` custom message
and uses the replacement-bound custom-message API to trigger the next turn. The generated awf effort
template remains the metadata and effort-snapshot producer, consumes the external display-suffix
capability, and never reconstructs an assigned base name. awf tests only its own emitted and consumed
event shapes and behavior; the external provider's implementation is not part of this plan.

Phase 1 lands the agent-owned handoff and the first two ADR operations. Phase 2 replaces awf's
name-override publication with the already-available display-suffix event surface and applies the
remaining two operations. Both implementation phases run in the managed worktree. The governed tail
performs local terminal review, synchronization, numbering, and integration before the behavior-free
lifecycle closure in the primary checkout.

The awf-owned event boundary is exact:

- awf emits payload-free `remote-pi:capabilities:request` and consumes complete
  `remote-pi:capabilities` snapshots; suffix support is present only when `displaySuffix` is exactly
  `{version:1}`, while unrelated capability members remain permitted;
- awf emits `remote-pi:display-suffix:set` with exactly `{value:string}` for its current canonical
  effort slug or `{value:null}` to clear, and listens for payload-free
  `remote-pi:display-suffix:request`;
- each received capabilities snapshot authoritatively replaces awf's suffix-support state; absent,
  malformed, or unsupported `displaySuffix` withdraws support and makes awf emit null, while no
  response leaves initial support false;
- factory load and every awf `session_start` request capabilities; attach or late support publishes
  the current slug, while detach, switch-detach, ownership loss, shutdown, unsupported snapshots,
  inactive replay, and session start publish null;
- metadata publication remains independent, no display-suffix event carries a namespace or base or
  composite name, no awf output emits name override, and missing external listeners or thrown event
  emissions degrade to metadata-only behavior.

## Phase 1: Persist continuation as agent-owned context

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["agent-owned-handoff"]

### Task 1.1: Specify custom-message ownership before production changes
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:agent-owned-handoff-context", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["tools/pi-extension-test/tests/handoff.test.ts", "tools/pi-extension-test/tests/runtime.test.ts"]

Start from the committed and reviewed plan with `git status --short` producing no output,
`git log -1 --format=%s` naming the plan or its review settlement, `./x check` finishing clean, and
`./x gate` passing. The phase owner works only in the managed awf worktree.

Extend the deterministic handoff harness before changing the template. Capture replacement-context
`sendMessage` calls separately from `sendUserMessage`, capture custom-message fields, and prove that a
successful continuation submits exactly one message:

```ts
{
  customType: "agent-handoff",
  content: `Agent-authored handoff context; this is not user input:\n\n${kickoff}`,
  display: true,
}
```

with `{triggerTurn:true}`. Assert that the accepted kickoff, including leading and trailing
whitespace, is byte-for-byte unchanged after the two envelope newlines; no `[agent-handoff]` text is
duplicated in content; no custom renderer is registered; and automatic submission never calls
`sendUserMessage`. Change editor-fallback and replacement-failure expectations to the same complete
envelope while retaining exact public `{kickoff}`, `details.kickoff`, UTF-16 bound, queue, countdown,
cancellation, lineage, cleanup, notification-disposition, and no-silent-retry assertions.

Extend the pinned in-memory runtime coverage through the real replacement-bound custom-message API.
Prove the persisted child transcript entry has role `custom`, `customType: "agent-handoff"`,
`display: true`, and the exact envelope, while the model-facing request contains the ownership prefix
and unchanged kickoff exactly once. Instantiate Pi's exported `CustomMessageComponent` with that
persisted message, no registered renderer, and the pinned test theme; assert its rendered plain text
contains exactly one `[agent-handoff]` label followed by the envelope. The new assertions must fail
against `sendUserMessage`; run `./x pi-test run` and record that expected red state before production
edits.

### Task 1.2: Translate kickoff once at the replacement boundary
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:agent-owned-handoff-context", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["templates/pi/awf-handoff/index.ts.tmpl", ".pi/extensions/awf-handoff/index.ts", "examples/sundial/.pi/extensions/awf-handoff/index.ts"]

In `templates/pi/awf-handoff/index.ts.tmpl`, define the pure exported helper
`handoffEnvelope(kickoff:string):string`, returning the exact prefix, two line feeds, and the
unmodified accepted kickoff. Keep pending state as the original kickoff so the tool result remains
unchanged; construct the envelope only after the queued command has matched its request and use that
same value for every automatic and recovery path.

Inside `ctx.newSession(...).withSession`, replace `next.sendUserMessage(kickoff)` with
replacement-bound
`next.sendMessage({customType:"agent-handoff",content:envelope,display:true},{triggerTurn:true})`. On
submission failure, keep the child active and place the complete envelope in the replacement editor.
On pre- or post-teardown replacement failure, place the complete envelope in the available recovery
editor. Do not register a renderer, put `[agent-handoff]` into message content, alter countdown or
pending semantics, retry submission, or reuse the old context after replacement. Retain all
minimum-runtime and notification-disposition behavior.

Run `./x render` before TypeScript tests so both generated adopters derive from the template. Require
`./x pi-test run` and `go test ./internal/project` to pass and
`rg -n 'sendUserMessage\(kickoff\)|registerMessageRenderer\([^,]*agent-handoff|\[agent-handoff\]' templates/pi/awf-handoff .pi/extensions/awf-handoff examples/sundial/.pi/extensions/awf-handoff`
to return no output.

### Task 1.3: Apply the handoff claims and publish behavior-facing documentation
Kind: batch
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:agent-owned-handoff-context", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: [".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/domains/parts/rendering/current-state.md", ".awf/parts/working-with-awf/commands.md", ".awf/docs/parts/architecture/overview.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", "templates/docs/working-with-awf.md.tmpl", "README.md", "changelog/CHANGELOG.md", "docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md", "docs/decisions/INDEX.md", "docs/topics/rendering/pi-workflows.md", "docs/domains/rendering.md", "docs/architecture.md", "docs/testing.md", "docs/working-with-awf.md", "examples/sundial/docs/working-with-awf.md", ".awf/awf.lock", "examples/sundial/.awf/awf.lock"]
Representative: "Revise `pi-session-handoff-lifecycle` to retain every replacement mechanic while making the automatic and recovery payload one visible `agent-handoff` custom message whose exact textual envelope triggers the continuation turn."
Edge: "Revise `pi-session-handoff-public-contract` without changing the one-property schema or relying on transport-level non-user attribution: the transcript role is custom and the textual ownership prefix remains required."
Post-check: "Run ./x render && ./x check, then run ./awf context --show pending docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md; require the handoff lifecycle and public-contract operations Applied, the effort-association and using-effort operations Remaining, and none Canceled."

Change the linked ADR from `Proposed` to `Implementing`, append its canonical Implementing event, and
append one Applied event naming exactly these operations in declaration order:

1. `update rendering/pi-workflows:pi-session-handoff-lifecycle`
2. `update rendering/pi-workflows:pi-session-handoff-public-contract`

Preserve each claim's Origin and complete existing Revised-by prefix, append the linked pending ADR
slug once, retain `Backing: test`, and keep its named proof markers on live Go tests. Update the claim
bodies to state the exact custom type, visible default rendering, one ownership prefix, custom
transcript role, transport-independent ownership envelope, `triggerTurn:true`, identical
editor/recovery envelope, and all retained lifecycle/public-input behavior.

Update only authored documentation sources for the same behavior. In both
`.awf/parts/working-with-awf/commands.md` and the shipped
`templates/docs/working-with-awf.md.tmpl`, replace the unchanged-submission claim with the exact
custom agent-handoff envelope and transport-independent ownership boundary so root and Sundial
output agree.
Distinguish automatic agent-authored continuation from user messages, state that the public tool
input remains exact bounded prose, and keep missing-template-value prose coherent. Add one Unreleased
changelog entry. Run `./x render`; include every deterministic root and Sundial extension/doc/index/
lock consequence and never edit generated output directly.

### Phase close

Run `./x pi-test run`, `go test ./internal/project ./internal/evals`, `git diff --check`,
`./x render`, and `./x check`; each must exit zero. Stage the complete handoff behavior, tests, claim
application, ADR lifecycle event, authored docs, changelog, and generated consequences explicitly.
Require `./awf check staged` and `./x gate` to pass, then create one closing commit.

```commit
feat(rendering): make Pi handoff agent-owned
```

## Phase 2: Consume display suffix without changing routing identity

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["stable-routing-display"]

### Task 2.1: Specify awf suffix publication before production changes
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:presentation-not-routing-identity", "separate-pi-continuation-context-from-user-and-routing-identity:display-suffix-capability", "separate-pi-continuation-context-from-user-and-routing-identity:authoritative-capability-replay", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["tools/pi-extension-test/tests/using-effort.test.ts"]

Start in the clean managed awf worktree with
`git merge-base --is-ancestor 9ec6b00e HEAD` exiting zero and `git log -1 --format=%s` equal to
`docs(plans): confine Pi continuation plan to awf` or a later exact review-fix subject recorded in
this plan's Notes. Require `./x check` and `./x gate` to pass.

Rewrite the optional-presentation portion of `using-effort.test.ts` before production changes. Keep
metadata replacement and its existing namespace payload independent. Require factory-time and every
`session_start` capability request; verify the capability and replay listeners are installed before
a synchronous response to the factory-time request. A valid complete snapshot with exact-key
`displaySuffix:{version:1}` publishes the current slug through `remote-pi:display-suffix:set`;
unrelated top-level capability members remain accepted, while
`displaySuffix:{version:1,extra:true}`, absent, malformed, or wrong-version members withdraw support
and emit `{value:null}`. Require repeated clears to remain explicit and idempotent; payload-free
`remote-pi:display-suffix:request` synchronously replays the current slug or null; detach, ownership
loss, restart, and shutdown emit unconditional null clears; and a successful switch emits the old
slug, null detachment, then the new slug. Missing external listeners or thrown event emissions never
change tool, context, heartbeat, association, or metadata outcomes. Assert that
`remote-pi:display-suffix:*` event names, payloads, and awf-owned structural types contain no
name-override, namespace, base name, or composite name, while metadata continues to use its existing
namespace. Assert no suffix publication changes the immutable effort snapshot. Run
`./x pi-test run`; the new assertions must fail against the name-override implementation.

### Task 2.2: Replace name override with authoritative display-suffix replay
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:presentation-not-routing-identity", "separate-pi-continuation-context-from-user-and-routing-identity:display-suffix-capability", "separate-pi-continuation-context-from-user-and-routing-identity:authoritative-capability-replay", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/index.ts", "tools/pi-extension-test/tests/using-effort.test.ts", ".awf/awf.lock"]

In `templates/pi/awf-effort/index.ts.tmpl`, replace the `nameOverride` capability shape, `nameOK`,
`namePublished`, name set/request listeners, and every `remote-pi:name-override:*` emission with
named awf-owned structural types for the exact event boundary in Architecture summary. Treat every
received capabilities event as a complete snapshot: support is true only when its `displaySuffix`
member is exactly `{version:1}`, while unrelated members are ignored; any other actual snapshot sets
support false and emits `{value:null}`. No response leaves initial support false.

Keep metadata publication generated from the current immutable effort snapshot and independent of
suffix support. When support is true and an effort is current, publish only
`{value:<canonical slug>}`. A late valid snapshot republishes current state. The payload-free replay
listener synchronously publishes the current slug when supported and current, otherwise null. Clear
suffix unconditionally and idempotently on explicit detach, successful switch detachment, owner
loss, `session_start`, and `session_shutdown`, even if local publication flags believe nothing was
set. At `session_start`, clear local association/publication as today, emit null metadata and
`{value:null}`, then request capabilities again. Retain the factory-time request. Catch every
optional event emission failure, never emit a name override, never inspect an assigned name, never
send a suffix namespace, never compose presentation locally, and never make suffix support a
condition for metadata, association, context, heartbeat, or detach.

Run `./x render` before tests. Require `./x pi-test run` to pass at 100 percent configured TypeScript
coverage. Require
`if rg -n 'name-override|nameOverride|NameOverride|_displayName|assigned.*name' templates/pi/awf-effort .pi/extensions/awf-effort tools/pi-extension-test/tests/using-effort.test.ts; then exit 1; fi`
to exit zero with no match. Separately assert from awf-owned structural types and captured
`remote-pi:display-suffix:*` payloads that the display-suffix family has no namespace field; do not
grep the complete effort source for `namespace`, because retained metadata uses it.

### Task 2.3: Apply effort claims and align awf documentation
Kind: batch
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:presentation-not-routing-identity", "separate-pi-continuation-context-from-user-and-routing-identity:display-suffix-capability", "separate-pi-continuation-context-from-user-and-routing-identity:authoritative-capability-replay", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["templates/skills/using-effort/SKILL.md.tmpl", ".awf/parts/workflow/chain.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/domains/parts/rendering/current-state.md", ".awf/docs/parts/architecture/overview.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", "README.md", "changelog/CHANGELOG.md", "internal/project/target_test.go", "internal/project/output_plan_test.go", "docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md", ".pi/skills/awf-using-effort/SKILL.md", "docs/architecture.md", "docs/testing.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/topics/rendering/pi-workflows.md", "docs/domains/rendering.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Revise `pi-effort-session-association` so complete metadata remains independent while the only optional presentation publication is capability-gated display suffix with authoritative snapshots, late replay, unconditional clears, and no routing/name fallback."
Edge: "Revise `using-effort-skill` to call the temporary text a display-only suffix, preserve exact attach/detach and fixed-path guidance, and state that display suffix is never a routing input."
Post-check: "Run ./x render && ./x check, then run ./awf context --show pending docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md; require all four named operations Applied, none Remaining or Canceled, and the ADR still Implementing."

Append one Applied event to the Implementing ADR naming exactly these operations in declaration order:

1. `update rendering/pi-workflows:pi-effort-session-association`
2. `update rendering/pi-workflows:using-effort-skill`

Preserve each claim's Origin and complete Revised-by prefix, append the linked pending ADR slug once,
retain `Backing: test`, and keep or relocate proof markers only with their named live tests. Extend
the Go source-contract tests to assert awf's exact display-suffix event family, factory plus
session-start negotiation, authoritative withdrawal, explicit-null replay/clear, metadata
independence, and absence of name-override strings. Do not add claims or tests about a foreign
provider's implementation.

Update authored skill, guide, architecture, testing, domain, README, and changelog prose in the same
transaction. Describe only awf as an ordinary optional-presentation producer: it publishes its slug
or null through the external event surface, never reads or composes a base name, never treats
presentation as routing authority, and degrades to metadata-only behavior without restoring name
override. Do not document another repository's implementation mechanics, paths, tests, commands,
commits, review, or release policy. Run `./x render`; include deterministic root consequences,
require the Sundial adopter to remain unchanged for the unselected effort output, and never edit
generated files directly.

### Phase close

Run `./x pi-test run`, `go test ./internal/project ./internal/evals`, `git diff --check`,
`./x render`, and `./x check`; each must exit zero. Stage the complete suffix consumer, tests, claim
application, docs, changelog, and generated consequences explicitly. Require
`./awf check staged` and `./x gate` to pass, then create one closing commit.

```commit
feat(rendering): separate Pi effort display identity
```

## Governed post-phase workflow tail

This tail is not an implementation phase or transaction. Immediately after Phase 2 and its
phase-review fixes are clean and green, invoke the native `awf-reviewing-impl` workflow over the
exact range `46edbd9c..HEAD`, including Phase 1 implementation and settlement plus Phase 2. Resolve findings
in new checked and gated commits until terminal review reports zero unresolved findings.

Then synchronize the managed worktree with `main`. Run `git status --short` and require no output.
Run `git merge --no-commit --no-ff main`. On conflict, run `git status --short`, report the exact
paths, and stop for resolution or abort; do not number or integrate. If Git reports
`Already up to date.`, require the worktree to remain clean. Otherwise require `MERGE_HEAD` to exist
and the merge index to be fully staged, run `./awf check staged && ./x gate`, and commit with
`git commit -m "Merge main into Pi continuation identity"`. Because this merge composes histories
after terminal review, invoke `awf-reviewing-impl` again over the combined awf history and settle
every finding before continuing.

Number only from that reviewed synchronized worktree. Run
`./awf adr number separate-pi-continuation-context-from-user-and-routing-identity` and retain its
single printed `<slug> -> NNNN` mapping. Resolve the new ADR path with
`adr_path="$(find docs/decisions -maxdepth 1 -type f -name '[0-9][0-9][0-9][0-9]-separate-pi-continuation-context-from-user-and-routing-identity.md' -print)"`;
require one nonempty line. Run `./x render` and require `git diff --name-only` to contain exactly the
numbered ADR, `docs/decisions/INDEX.md`,
`.awf/topics/parts/rendering/pi-workflows/current-state.md`,
`docs/topics/rendering/pi-workflows.md`, `docs/domains/rendering.md`, and `.awf/awf.lock`; stop and
investigate any extra or missing path. Stage those paths explicitly, run
`./awf check staged && ./x gate`, and commit with subject
`docs(adr): number Pi continuation identity` and the retained mapping as the body.

In the clean intended target checkout, run `git status --short` and require no output, then run
`./awf effort integrate pi-handoff-identity`. Accept only an already-integrated or fast-forward
result with a clean index, or a divergent result explicitly reported as staged without a commit. For
a divergent result, require `MERGE_HEAD` and a fully staged index, run
`./awf check staged && ./x gate`, and commit with `git commit -m "Merge awf/pi-handoff-identity"`;
then invoke `awf-reviewing-impl` again over the combined target history and settle every finding.
Stop on conflicts, red verification, or a user-decision finding. The ADR remains Implementing and
this plan remains Proposed throughout this tail. Proceed to Phase 3 only in the integrated primary
checkout after awf terminal review is settled.

## Phase 3: Freeze reviewed continuation identity

**Execution mode: inline.**

Completes: ["reviewed-lifecycle", "repository-green"]

### Task 3.1: Record terminal status after awf review and integration
Latitude: exact
Paths: ["glob:docs/decisions/[0-9][0-9][0-9][0-9]-separate-pi-continuation-context-from-user-and-routing-identity.md", "docs/plans/2026-08-04-separate-pi-continuation-identity.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Post-check: "Resolve the ADR path with find and require exactly one matching numbered file; after render require git diff --name-only to be exactly that ADR, this plan, docs/decisions/INDEX.md, and .awf/awf.lock."

Run only in the integrated primary checkout. Start with `git status --short` producing no output, the
numbered ADR at `status: Implementing`, all four State changes Applied exactly once, this plan at
`status: Proposed`, terminal awf implementation review settled, `./x check` clean, and `./x gate`
passing. This phase contains no production behavior, runtime test expectation, user-facing behavior
documentation, or external-provider change.

Resolve `adr_path` with the exact `find` command from the governed tail and require one nonempty
line. Append that numbered ADR's canonical Implemented history event using the latest content stamp;
do not append another Applied or Reapplied event. Under this plan's Notes, record actual awf
implementation deviations, review fixes, and validation performed, or state that there were none
beyond the planned awf transactions. Change this plan's `status: Proposed` to `status: Implemented`
without rewriting frozen execution instructions.

Run `./x render`. Confirm `git diff --name-only` contains exactly the ADR, this plan,
`docs/decisions/INDEX.md`, and `.awf/awf.lock`, with no unreviewed behavior or claim mutation.

### Phase close

Stage only the lifecycle transaction. Require `./awf check staged`, `./x check`, and `./x gate` to
pass, then run `./awf context --show pending "$adr_path"`; require every operation Applied with no
Remaining or Canceled operation. Commit:

```commit
docs(rendering): finalize Pi continuation identity
```

After this commit, run `./awf effort worktree remove pi-handoff-identity` without force. Require the
managed path, registration, and `awf/pi-handoff-identity` branch all to be absent before invoking the
native retrospective workflow. Retrospective records any durable lesson, verifies removed topology
again, and finishes the effort last.

## Definition of done

- `dod: agent-owned-handoff` A replacement child persists exactly one visible `agent-handoff` custom
  transcript message with one exact ownership prefix and unchanged accepted kickoff, triggers the
  next turn through replacement-bound `sendMessage`, and uses the identical envelope for editor
  recovery without changing public input or retained replacement mechanics.
- `dod: stable-routing-display` Attaching, switching, detaching, restarting, losing ownership, and
  late capability delivery publish independent metadata and optional display suffix correctly; no
  awf output emits name override, reconstructs a base name, or uses presentation as routing
  authority.
- `dod: reviewed-lifecycle` All four ADR operations apply in their awf behavior transactions,
  terminal awf review settles, and integration completes before the ADR and plan freeze as
  Implemented in the primary checkout.
- `dod: repository-green` Every awf phase closes with explicit staging, clean drift, staged
  authority, and the full gate green; the final primary checkout is clean and managed effort
  topology is absent before retrospective.

## Notes

- Frozen historical ADRs and implemented plans that describe the former user message or name
  override remain unchanged; stale-contract searches deliberately target current templates,
  generated outputs, tests, and active documentation rather than rewriting history.
- The external display-suffix event surface is an input to awf. This plan neither specifies nor
  verifies another repository's implementation.
- Phase 1 landed as `d9afaf1d` with review settlement `331d80c5`; its independent verify review
  returned zero unresolved findings.
