---
format: plan-v2
date: 2026-08-04
adrs:
  - separate-pi-continuation-context-from-user-and-routing-identity
status: Proposed
---
# Plan: Separate Pi Continuation Identity

## Goal

Implement the linked ADR so a fresh Pi continuation is durably agent-owned, effort presentation is a display-only suffix over stable Remote Pi identity, capability replay is deterministic across extension replacement, and both repositories prove the same version-1 structural contract.

Do not change the public `{kickoff}` schema or its bound, add a Pi message role, make presentation or metadata routing authority, retain the Remote Pi name-override fallback, import a Remote Pi package, or refactor unrelated handoff, effort-activity, mesh, relay, pairing, or application code.

## Architecture summary

The generated awf handoff template remains the owner of queueing, countdown, replacement, and recovery, but translates accepted kickoff prose once into a visible `agent-handoff` custom message and uses the replacement-bound custom-message API to trigger the next turn. The generated awf effort template remains the metadata and effort-snapshot producer, but consumes only a closed display-suffix capability and never reconstructs Remote Pi's assigned base name. Remote Pi independently owns the normative display-suffix protocol, stable operational identity, presentation composition, and its repository tests. An awf test fixture pins the matching event shapes and the Remote Pi owner commit without creating a runtime or package dependency.

Phase 1 lands the agent-owned handoff and the first two ADR operations. Before Phase 2 dispatch, the parent explicitly authorizes the independently running Remote Pi owner to implement its half, receives its clean reviewed owner commit and exact contract receipt, and checks that receipt against the contract below; the awf phase then pins that commit and lands the consumer plus the remaining two operations. Both implementation phases run in the managed worktree. After terminal implementation review, the governed tail merges and numbers in that worktree, integrates from the intended target checkout, renews review after any divergent merge, and only then runs the behavior-free lifecycle freeze in the primary checkout.

The shared version-1 contract is exact:

- `remote-pi:capabilities:request` has no payload, and `remote-pi:capabilities` returns one complete authoritative singleton-provider snapshot whose `displaySuffix` member is exactly `{version:1}` for support; unrelated members, including metadata, remain permitted;
- `remote-pi:display-suffix:set` accepts exactly `{value:string}` to replace one process-local register or `{value:null}` to clear it unconditionally; there is no namespace, producer identity, producer grammar, value bound, or additional key;
- `remote-pi:display-suffix:request` has no payload, every producer answers synchronously with its current string or null, and ordinary event-listener order makes the last write win;
- Remote Pi composes `<assigned name> - <latest suffix>` only for human-facing presentation and never feeds the suffix or composite into configured name, broker/mesh address, cwd lock, peer self-filtering, relay room identity, or routing;
- an actual capabilities snapshot with absent, malformed, or unsupported-version `displaySuffix` withdraws awf support and makes awf emit null; no response leaves the consumer initially unsupported; malformed set payloads leave the Remote Pi register unchanged, while missing listeners and thrown event handlers remain advisory;
- factory load and every awf `session_start` request capabilities; awf attach or late support writes its effort slug, while detach, switch-detach, ownership loss, shutdown, unsupported snapshots, inactive replay, and session start write null; Remote Pi clears on shutdown and before every session-start replay, then requests synchronous replay before initial presentation.

## Phase 1: Persist continuation as agent-owned context

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["agent-owned-handoff"]

### Task 1.1: Specify custom-message ownership before production changes
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:agent-owned-handoff-context", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["tools/pi-extension-test/tests/handoff.test.ts", "tools/pi-extension-test/tests/runtime.test.ts"]

Start from the committed and reviewed plan with `git status --short` producing no output, `git log -1 --format=%s` naming the plan or its review settlement, `./x check` finishing clean, and `./x gate` passing. The phase owner works only in the managed awf worktree and does not contact or mutate the Remote Pi repository.

Extend the deterministic handoff harness before changing the template. Capture replacement-context `sendMessage` calls separately from `sendUserMessage`, capture custom-message fields, and prove that a successful continuation submits exactly one message:

```ts
{
  customType: "agent-handoff",
  content: `Agent-authored handoff context; this is not user input:\n\n${kickoff}`,
  display: true,
}
```

with `{triggerTurn:true}`. Assert that the accepted kickoff, including leading and trailing whitespace, is byte-for-byte unchanged after the two envelope newlines; no `[agent-handoff]` text is duplicated in content; no custom renderer is registered; and automatic submission never calls `sendUserMessage`. Change editor-fallback and replacement-failure expectations to the same complete envelope while retaining exact public `{kickoff}`, `details.kickoff`, UTF-16 bound, queue, countdown, cancellation, lineage, cleanup, notification-disposition, and no-silent-retry assertions.

Extend the pinned in-memory runtime coverage in `runtime.test.ts` through the real replacement-bound custom-message API. Prove the persisted child transcript entry has role `custom`, `customType: "agent-handoff"`, `display: true`, and the exact envelope, while the provider request contains exactly one user-role message with the ownership prefix and unchanged kickoff. Instantiate Pi's exported `CustomMessageComponent` with that persisted message, no registered renderer, and the pinned test theme; assert its rendered plain text contains exactly one `[agent-handoff]` label followed by the envelope. The new assertions must fail against `sendUserMessage`; run `./x pi-test run` and record that expected red state before production edits.

### Task 1.2: Translate kickoff once at the replacement boundary
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:agent-owned-handoff-context", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["templates/pi/awf-handoff/index.ts.tmpl", ".pi/extensions/awf-handoff/index.ts", "examples/sundial/.pi/extensions/awf-handoff/index.ts"]

In `templates/pi/awf-handoff/index.ts.tmpl`, define the pure exported helper `handoffEnvelope(kickoff:string):string`, returning the exact prefix, two line feeds, and the unmodified accepted kickoff. Keep pending state as the original kickoff so the tool result remains unchanged; construct the envelope only after the queued command has matched its request and use that same value for every automatic and recovery path.

Inside `ctx.newSession(...).withSession`, replace `next.sendUserMessage(kickoff)` with replacement-bound `next.sendMessage({customType:"agent-handoff",content:envelope,display:true},{triggerTurn:true})`. On submission failure, keep the child active and place the complete envelope in the replacement editor. On pre- or post-teardown replacement failure, place the complete envelope in the available recovery editor. Do not register a renderer, put `[agent-handoff]` into message content, alter countdown or pending semantics, retry submission, or reuse the old context after replacement. Retain all minimum-runtime and notification-disposition behavior.

Run `./x render` before TypeScript tests so both generated adopters derive from the template. Require `./x pi-test run` and `go test ./internal/project` to pass and `rg -n 'sendUserMessage\(kickoff\)|registerMessageRenderer\([^,]*agent-handoff|\[agent-handoff\]' templates/pi/awf-handoff .pi/extensions/awf-handoff examples/sundial/.pi/extensions/awf-handoff` to return no output.

### Task 1.3: Apply the handoff claims and publish behavior-facing documentation
Kind: batch
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:agent-owned-handoff-context", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: [".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/domains/parts/rendering/current-state.md", ".awf/parts/working-with-awf/commands.md", ".awf/docs/parts/architecture/overview.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", "README.md", "changelog/CHANGELOG.md", "docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md", "docs/decisions/INDEX.md", "docs/topics/rendering/pi-workflows.md", "docs/domains/rendering.md", "docs/architecture.md", "docs/testing.md", "docs/working-with-awf.md", ".awf/awf.lock", "examples/sundial/.awf/awf.lock"]
Representative: "Revise `pi-session-handoff-lifecycle` to retain every replacement mechanic while making the automatic and recovery payload one visible `agent-handoff` custom message whose exact textual envelope triggers the continuation turn."
Edge: "Revise `pi-session-handoff-public-contract` without changing the one-property schema or claiming a non-user provider role: the transcript role is custom, Pi's current provider adapter still supplies user-role content, and the textual ownership prefix remains required."
Post-check: "Run ./x render && ./x check, then run ./awf context --show pending docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md; require the handoff lifecycle and public-contract operations Applied, the effort-association and using-effort operations Remaining, and none Canceled."

Change the linked ADR from `Proposed` to `Implementing`, append its canonical Implementing event, and append one Applied event naming exactly these operations in declaration order:

1. `update rendering/pi-workflows:pi-session-handoff-lifecycle`
2. `update rendering/pi-workflows:pi-session-handoff-public-contract`

Preserve each claim's Origin and complete existing Revised-by prefix, append the linked pending ADR slug once, retain `Backing: test`, and keep its named proof markers on live Go tests. Update the claim bodies to state the exact custom type, visible default rendering, one ownership prefix, custom transcript role, current provider user-role limitation, `triggerTurn:true`, identical editor/recovery envelope, and all retained lifecycle/public-input behavior.

Update only authored documentation sources for the same behavior. Distinguish automatic agent-authored continuation from user messages, state that the public tool input remains exact bounded prose, explain the current provider-role limitation, and keep missing-template-value prose coherent. Add one Unreleased changelog entry. Run `./x render`; include every deterministic root and Sundial extension/doc/index/lock consequence and never edit generated output directly.

### Phase close

Run `./x pi-test run`, `go test ./internal/project ./internal/evals`, `git diff --check`, `./x render`, and `./x check`; each must exit zero. Stage the complete handoff behavior, tests, claim application, ADR lifecycle event, authored docs, changelog, and generated consequences explicitly. Require `./awf check staged` and `./x gate` to pass, then create one closing commit.

```commit
feat(rendering): make Pi handoff agent-owned
```

## Phase 2: Consume display suffix without changing routing identity

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["stable-routing-display", "cross-repository-contract"]

### Task 2.1: Pin the owner contract and write suffix regressions first
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:presentation-not-routing-identity", "separate-pi-continuation-context-from-user-and-routing-identity:display-suffix-capability", "separate-pi-continuation-context-from-user-and-routing-identity:authoritative-capability-replay", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["tools/pi-extension-test/fixtures/remote-pi-display-suffix-v1.json", "tools/pi-extension-test/tests/using-effort.test.ts"]

Before dispatching this phase, the parent uses the approved Remote Pi peer boundary to send explicit implementation authorization for only the normative provider half stated in Architecture summary. The Remote Pi owner first lands its repository-native executable plan, then closes independently green relay, app, Pi-extension provider, and protocol-documentation commits under that plan. The provider commit owns the exported event shapes, capability snapshot, register/replay behavior, exact tests, and `pi-extension/README.md`; awf pins that full commit, or a later provider-shape correction, rather than the docs-only closeout. The Remote Pi owner changes no awf file, runs each subproject's formatter/analyzer/typecheck, focused and broader tests, and build, independently reviews its change, and returns: its committed plan path, exact changed paths, per-phase commit SHAs, terminal commands and results, the final provider owner commit, and the matching v1 contract fixture. The parent verifies that receipt against all six contract bullets above and stops on any semantic mismatch; naming or formatting corrections determined by those bullets are mechanical, while a material mismatch returns to ADR amendment rather than being hidden in this plan. The receipt must specifically prove empty, very large, and control-bearing strings remain data; malformed set payloads preserve the prior register; presentation or relay-send refusal preserves the accepted register; and every case leaves assigned mesh identity, address, cwd lock, and base-derived room id unchanged.

Start the awf phase only after that clean Remote Pi owner commit exists. In the managed awf worktree require `git status --short` to print nothing, `git log -1 --format=%s` to equal `feat(rendering): make Pi handoff agent-owned` or a later settled review-fix subject, `./x check` to finish clean, and `./x gate` to pass. The Remote Pi receipt is read-only input; the phase owner does not mutate or commit in the Remote Pi checkout.

Create `tools/pi-extension-test/fixtures/remote-pi-display-suffix-v1.json` as data-only structural evidence with these keys in this order: integer `schemaVersion` equal to 1; string `ownerRepository` equal to `github.com/hypnotox/remote_pi`; string `ownerCommit`, validated by the test as the exact lowercase 40-hex commit returned by the Remote Pi owner; object `events` with `capabilitiesRequest`, `capabilities`, `suffixSet`, and `suffixRequest`, each containing exact `name` and `payload` fields, using the JSON string `undefined` for payload-free events; object `semantics` with booleans `authoritativeCapabilities`, `unrelatedCapabilityMembersPermitted`, `singleRegister`, `lastWriterWins`, `synchronousReplay`, and `malformedSetPreservesRegister`, all true; string `composition` equal to `${assignedName} - ${latestSuffix}`; and array `stableIdentity` containing exactly `configuredName`, `meshAddress`, `cwdLock`, `peerSelfFiltering`, `relayRoomId`, and `routing`. The capabilities example is `{displaySuffix:{version:1},metadata:{version:1}}`; set examples are `{value:"demo-effort"}` and `{value:null}`; no fixture key or value names a namespace, producer grammar, bound, Remote Pi helper symbol, release, installation path, or reconnect implementation. Load and assert this fixture in the TypeScript suite; do not import Remote Pi code.

Rewrite the Remote Pi portion of `using-effort.test.ts` before production changes. Require metadata replacement and its existing namespace payload to remain independent. Require factory-time and every `session_start` capability request; a valid snapshot with exact `displaySuffix:{version:1}` publishes the current slug through `remote-pi:display-suffix:set`; unrelated capability members remain accepted; late support publishes immediately; a complete snapshot with the field absent, malformed, or wrong-version withdraws support and emits `{value:null}`; repeated clears are idempotently emitted; payload-free `remote-pi:display-suffix:request` synchronously replays the current slug or null; detach, ownership loss, restart, and shutdown emit unconditional null clears; thrown or missing event integration never changes tool, context, heartbeat, or metadata outcomes. Assert no display-suffix event name, payload, fixture field, or structural type contains `name-override`, `namespace`, producer grammar, or a base/composite name, while metadata continues to use its existing namespace. Assert no suffix publication changes the current effort snapshot. Run `./x pi-test run`; the new assertions must fail against the name-override implementation.

### Task 2.2: Replace name override with authoritative display-suffix replay
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:presentation-not-routing-identity", "separate-pi-continuation-context-from-user-and-routing-identity:display-suffix-capability", "separate-pi-continuation-context-from-user-and-routing-identity:authoritative-capability-replay", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/index.ts", "tools/pi-extension-test/tests/using-effort.test.ts"]

In `templates/pi/awf-effort/index.ts.tmpl`, replace the `nameOverride` capability shape, `nameOK`, `namePublished`, name set/request listeners, and all `remote-pi:name-override:*` emissions with named awf-owned structural types matching the pinned fixture. Treat every received capabilities event as a complete snapshot: support is true only when its `displaySuffix` member is exactly `{version:1}`, while unrelated members are ignored; any other actual snapshot sets support false and emits `{value:null}`. No response leaves initial support false.

Keep metadata publication generated from the current immutable effort snapshot and independent of suffix support. When support is true and an effort is current, publish only `{value:<canonical slug>}`. A late valid snapshot republishes current state. The payload-free display-suffix replay listener synchronously publishes the current slug when supported and current, otherwise null. Clear suffix unconditionally and idempotently on explicit detach, successful switch detachment, owner loss, `session_start`, and `session_shutdown`, even if local publication flags believe nothing was set. At `session_start`, clear local association/publication as today, emit null metadata and `{value:null}`, then request capabilities again. Retain the factory-time request. Catch every optional event emission failure, never emit a name override, never inspect an assigned name, never send a namespace, never compose presentation locally, and never make suffix support a condition for metadata, association, context, heartbeat, or detach.

Run `./x render` before tests. Require `./x pi-test run` to pass at 100 percent configured TypeScript coverage. Require `if rg -n 'name-override|nameOverride|NameOverride|_displayName|assigned.*name' templates/pi/awf-effort .pi/extensions/awf-effort tools/pi-extension-test/tests/using-effort.test.ts; then exit 1; fi` to exit zero with no match. Separately assert from parsed fixture/types and captured `remote-pi:display-suffix:*` payloads that the display-suffix family has no namespace field; do not grep the complete effort source for `namespace`, because retained metadata uses it.

### Task 2.3: Apply effort claims and align all presentation documentation
Kind: batch
Latitude: exact
Applying: ["separate-pi-continuation-context-from-user-and-routing-identity:presentation-not-routing-identity", "separate-pi-continuation-context-from-user-and-routing-identity:display-suffix-capability", "separate-pi-continuation-context-from-user-and-routing-identity:authoritative-capability-replay", "separate-pi-continuation-context-from-user-and-routing-identity:independently-owned-delivery"]
Paths: ["templates/skills/using-effort/SKILL.md.tmpl", ".awf/parts/workflow/chain.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/domains/parts/rendering/current-state.md", ".awf/docs/parts/architecture/overview.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", "README.md", "changelog/CHANGELOG.md", "internal/project/target_test.go", "internal/project/output_plan_test.go", "docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md", ".pi/skills/awf-using-effort/SKILL.md", "docs/architecture.md", "docs/testing.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/topics/rendering/pi-workflows.md", "docs/domains/rendering.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "Revise `pi-effort-session-association` so complete metadata remains independent while the only optional presentation publication is capability-gated display suffix with authoritative snapshots, late replay, unconditional clears, and no routing/name fallback."
Edge: "Revise `using-effort-skill` to call the temporary text a display-only suffix, preserve exact attach/detach and fixed-path guidance, and state that broker-assigned peer addresses remain stable and are the only routing target."
Post-check: "Run ./x render && ./x check, then run ./awf context --show pending docs/decisions/separate-pi-continuation-context-from-user-and-routing-identity.md; require all four named operations Applied, none Remaining or Canceled, and the ADR still Implementing."

Append one Applied event to the Implementing ADR naming exactly these operations in declaration order:

1. `update rendering/pi-workflows:pi-effort-session-association`
2. `update rendering/pi-workflows:using-effort-skill`

Preserve each claim's Origin and complete Revised-by prefix, append the linked pending ADR slug once, retain `Backing: test`, and keep or relocate proof markers only with their named live tests. Extend the Go source-contract tests to assert the fixture owner-commit format, exact display-suffix event family, factory plus session-start negotiation, authoritative withdrawal, explicit-null replay/clear, metadata independence, and absence of name-override strings. Do not add a Remote Pi-owned invariant to awf.

Update authored skill, guide, architecture, testing, domain, README, and changelog prose in the same transaction. State that Remote Pi owns one generic namespace-free last-writer-wins register and `<assigned base> - <latest suffix>` projection, while the assigned base, mesh address, cwd lock, relay room, and routing remain unchanged. State that awf is an ordinary producer, any producer write or null can replace it, and old or missing Remote Pi support degrades to metadata-only without restoring the retired name override. Name the independent owner-commit fixture as provenance, not a dependency or release floor. Run `./x render`; include deterministic root consequences, require the Sundial adopter to remain unchanged for the unselected effort output, and never edit generated files directly.

### Phase close

Run `./x pi-test run`, `go test ./internal/project ./internal/evals`, `git diff --check`, `./x render`, and `./x check`; each must exit zero. Stage the complete suffix consumer, fixture, tests, claim application, docs, changelog, and generated consequences explicitly. Require `./awf check staged` and `./x gate` to pass, then create one closing commit.

```commit
feat(rendering): separate Pi effort display identity
```

## Governed post-phase workflow tail

This tail is not an implementation phase or transaction. Immediately after Phase 2 and its phase-review fixes are clean and green, invoke the native `awf-reviewing-impl` workflow over every awf implementation and review-fix commit after the committed plan-review baseline. Resolve findings in new checked and gated commits until terminal review reports zero unresolved findings. The review verifies the pinned Remote Pi receipt and fixture structurally but does not claim authority to review or mutate the Remote Pi owner commit; require the Remote Pi peer's own terminal review/test receipt to remain clean.

Then synchronize the managed worktree with `main`. Run `git status --short` and require no output. Run `git merge --no-commit --no-ff main`. On conflict, run `git status --short`, report the exact paths, and stop for resolution or abort; do not number or integrate. If Git reports `Already up to date.`, require the worktree to remain clean. Otherwise require `MERGE_HEAD` to exist and the merge index to be fully staged, run `./awf check staged && ./x gate`, and commit with `git commit -m "Merge main into Pi continuation identity"`. Because this merge composes histories after terminal review, invoke `awf-reviewing-impl` again over the combined awf history and settle every finding before continuing.

Number only from that reviewed synchronized worktree. Run `./awf adr number separate-pi-continuation-context-from-user-and-routing-identity` and retain its single printed `<slug> -> NNNN` mapping. Resolve the new ADR path with `adr_path="$(find docs/decisions -maxdepth 1 -type f -name '[0-9][0-9][0-9][0-9]-separate-pi-continuation-context-from-user-and-routing-identity.md' -print)"`; require one nonempty line. Run `./x render` and require `git diff --name-only` to contain only the numbered ADR, `docs/decisions/INDEX.md`, `.awf/topics/parts/rendering/pi-workflows/current-state.md`, `docs/topics/rendering/pi-workflows.md`, `docs/domains/rendering.md`, `.awf/awf.lock`, and any other deterministic provenance substitution reported by render; investigate any behavioral diff. Stage each reported path explicitly, run `./awf check staged && ./x gate`, and commit with subject `docs(adr): number Pi continuation identity` and the retained mapping as the body.

In the clean intended target checkout, run `git status --short` and require no output, then run `./awf effort integrate pi-handoff-identity`. Accept only an already-integrated or fast-forward result with a clean index, or a divergent result explicitly reported as staged without a commit. For a divergent result, require `MERGE_HEAD` and a fully staged index, run `./awf check staged && ./x gate`, and commit with `git commit -m "Merge awf/pi-handoff-identity"`; then invoke `awf-reviewing-impl` again over the combined target history and settle every finding. Stop on conflicts, red verification, a Remote Pi contract mismatch, or a user-decision finding. The ADR remains Implementing and this plan remains Proposed throughout this tail. Proceed to Phase 3 only in the integrated primary checkout after both repositories' terminal reviews are settled.

## Phase 3: Freeze reviewed continuation identity

**Execution mode: inline.**

Completes: ["reviewed-lifecycle", "repository-green"]

### Task 3.1: Record terminal status only after both ownership halves settle
Latitude: exact
Paths: ["glob:docs/decisions/[0-9][0-9][0-9][0-9]-separate-pi-continuation-context-from-user-and-routing-identity.md", "docs/plans/2026-08-04-separate-pi-continuation-identity.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Post-check: "Resolve the ADR path with find and require exactly one matching numbered file; after render require git diff --name-only to be exactly that ADR, this plan, docs/decisions/INDEX.md, .awf/awf.lock, and any mechanically required lifecycle output named by render."

Run only in the integrated primary checkout. Start with `git status --short` producing no output, the numbered ADR at `status: Implementing`, all four State changes Applied exactly once, this plan at `status: Proposed`, the Remote Pi owner commit pinned by the fixture, terminal awf implementation review settled, Remote Pi's owner review/test receipt clean, `./x check` clean, and `./x gate` passing. This phase contains no production behavior, runtime test expectation, user-facing behavior documentation, or Remote Pi change.

Resolve `adr_path` with the exact `find` command from the governed tail and require one nonempty line. Append that numbered ADR's canonical Implemented history event using the same content stamp as its Implementing event; do not append another Applied or Reapplied event. Under this plan's Notes, record actual implementation deviations, review fixes, final Remote Pi owner commit, and validation performed, or state that there were none beyond the planned independently owned transactions. Change this plan's `status: Proposed` to `status: Implemented` without rewriting frozen execution instructions.

Run `./x render`. Confirm the diff contains only the ADR, plan, decision index, lock, and any mechanically required lifecycle render consequence, with no unreviewed behavior or claim mutation.

### Phase close

Stage only the lifecycle transaction. Require `./awf check staged`, `./x check`, and `./x gate` to pass, then run `./awf context --show pending "$adr_path"`; require every operation Applied with no Remaining or Canceled operation. Commit:

```commit
docs(rendering): finalize Pi continuation identity
```

After this commit, run `./awf effort worktree remove pi-handoff-identity` without force. Require the managed path, registration, and `awf/pi-handoff-identity` branch all to be absent before invoking the native retrospective workflow. Retrospective records any durable lesson, verifies removed topology again, and finishes the effort last.

## Definition of done

- `dod: agent-owned-handoff` A replacement child persists exactly one visible `agent-handoff` custom transcript message with one exact ownership prefix and unchanged accepted kickoff, triggers the next turn through replacement-bound `sendMessage`, and uses the identical envelope for editor recovery without changing public input or retained replacement mechanics.
- `dod: stable-routing-display` Attaching, switching, detaching, restarting, losing ownership, and late capability delivery publish independent metadata and optional display suffix correctly; no awf output emits name override, reconstructs a base name, or changes mesh address, cwd lock, relay room, or routing authority.
- `dod: cross-repository-contract` Remote Pi owns and proves stable identity plus live `<assigned base> - <suffix>` presentation; its tests cover empty, large, control-bearing, malformed, and presentation/transport-refusal cases without register or identity corruption; awf pins its reviewed provider commit and exact version-1 structural fixture without package/runtime coupling, and missing or malformed integration degrades to metadata-only behavior.
- `dod: reviewed-lifecycle` All four ADR operations apply in their behavior transactions, both repositories' implementation reviews settle, integration and any divergent-merge re-review complete, and only then do the ADR and plan freeze as Implemented in the primary checkout.
- `dod: repository-green` Every awf phase closes with explicit staging, clean drift, staged authority, and the full gate green; the final primary checkout is clean and managed effort topology is absent before retrospective.

## Notes

- Frozen historical ADRs and implemented plans that describe the former user message or name override remain unchanged; stale-contract searches deliberately target current templates, generated outputs, tests, and active documentation rather than rewriting history.
- The Remote Pi owner commit is discovered only after its independently governed implementation lands. Recording the returned exact hash in the fixture is the planned executable provenance step, not a package pin or an unresolved placeholder.
- Phase 2 dispatch must not grant the Remote Pi peer awf ownership, and the awf phase owner must not edit Remote Pi. The peer may refine internal helper placement and test partitioning, but the six exact namespace-free contract bullets and repository boundary are not implementation latitude.
