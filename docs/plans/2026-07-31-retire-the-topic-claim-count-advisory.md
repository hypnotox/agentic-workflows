---
date: 2026-07-31
adrs: [194]
status: Proposed
---
# Plan: Retire the topic claim-count advisory

## Goal

Execute ADR-0194: remove `currentState.maxClaimsPerTopic` and the `awf check` topic
claim-count note from every surface, ship a schema-28 migration that removes the key from an
adopter tree, and land the replacement guidance (a topic-cohesion rule in the shipped
doc-standard template and a cohesion focus lens on `adr-reviewer`). Non-goals: this plan does
not touch `currentState.maxTopicsPerPath` and does not split
`rendering/workflow-skill-templates`.

## Architecture summary

Three phases, matching ADR-0194's V2 operation batching.

Phase 1 is purely additive: the doc-standard rule and the `adr-reviewer` lens. It closes green
on its own because nothing yet depends on it and no claim changes.

Phase 2 is the retirement, and it is one indivisible transaction. It cannot be sliced:
`.awf/config.yaml` is strict-parsed, so the moment the `MaxClaimsPerTopic` field leaves
`internal/config` the key must already be gone from both config trees; `awf check` enforces
claim/proof-marker symmetry in both directions, so the two claim removals and their four
markers must move together; and `migrate.Current()` reads the registry tail, so registering
generation 28 obliges both locks to advance in the same commit. Docs travel with the change by
invariant. Phase 2 applies two of the three declared operations and moves ADR-0194 to
`Implementing`.

Phase 3 is the deferred final batch, and it runs after terminal review. It adds
`config/migrations-and-locks:claim-budget-key-dropped`, places that claim's proof marker onto
the migration test Phase 2 leaves unmarked, flips ADR-0194 to `Implemented`, and freezes this
plan. The split is forced, not stylistic: `internal/adr/application.go` rejects an
`Implementing` status whose operations are all applied (it requires at least one still
pending), while the executing-plans contract requires the `Implemented` flip to follow
terminal review. Applying all three operations in Phase 2 is therefore illegal in both
readings, and the new claim's proof marker would orphan `awf check` if it landed before its
claim.

## File structure

- **Created:**
  - `internal/migrate/dropmaxclaimspertopic.go`
  - `internal/migrate/dropmaxclaimspertopic_test.go`
- **Modified:**
  - `templates/docs/doc-standard.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`,
    `templates/docs/agents-md-standard.md.tmpl`
  - `internal/catalog/standard.go`, `.awf/agents/adr-reviewer.yaml`
  - `internal/config/config.go`, `internal/config/edit.go`
  - `internal/project/scaffold.go`, `internal/project/currentstate.go`,
    `internal/project/configreference.go`, `internal/project/project.go`
  - `internal/topic/coverage.go`
  - `internal/configspec/spec.go`
  - `internal/migrate/migrate.go`, `internal/migrate/dropworkflowtelemetry_test.go`,
    `internal/migrate/dropseveritysettings_test.go`
  - `internal/project/version_test.go`, `internal/config/config_test.go`,
    `internal/config/edit_test.go`, `internal/configspec/spec_test.go`,
    `internal/topic/coverage_test.go`, `internal/project/currentstate_test.go`,
    `internal/project/configreference_test.go`, `internal/project/drift_test.go`,
    `cmd/awf/config_test.go`, `cmd/awf/check_test.go`
  - `.awf/config.yaml`, `examples/sundial/.awf/config.yaml`
  - `.awf/topics/parts/tooling/cli/current-state.md`,
    `.awf/topics/parts/config/configuration/current-state.md`,
    `.awf/topics/parts/config/migrations-and-locks/current-state.md`
  - `.awf/docs/parts/testing/gate.md`, `.awf/docs/parts/roadmap/ideas.md`
  - `changelog/CHANGELOG.md`
  - `docs/decisions/0194-retire-the-topic-claim-count-advisory-for-authoring-guidance-and-a-review-lens.md`
  - every file `./x render` regenerates from the above, including `docs/doc-standard.md`,
    `docs/working-with-awf.md`, `docs/agents-md-standard.md`, `docs/config-reference.md`,
    `docs/testing.md`, `docs/roadmap.md`, `docs/topics/tooling/cli.md`,
    `docs/topics/config/configuration.md`, `docs/topics/config/migrations-and-locks.md`,
    `docs/decisions/INDEX.md`, `.claude/agents/adr-reviewer.md`, `.pi/agents/adr-reviewer.md`,
    `.awf/awf.lock`, and the corresponding `examples/sundial/` renders and lock
- **Deleted:** none. `internal/migrate/maxclaimspertopic.go` and its test are retained by
  ADR-0194 item 6; historical migrations are never deleted.

## Phase 1: Land the replacement guidance

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction. Checkbox tasks are ordered steps, not transaction boundaries. Everything here is
additive: no claim changes, no config-schema change, so the phase closes green before any
retirement begins.

- [ ] **Task 1.1: Add the topic-cohesion rule to the shipped doc-standard template.** In
  `templates/docs/doc-standard.md.tmpl`, inside the `<!-- awf:section rules -->` block, append
  this bullet as the last entry of the list, immediately after the `**Adjust `.layout.*`
  values for link relativity.**` bullet and before `<!-- awf:end -->`:

  ```
  - **One subject per topic.** A current-state topic collects the claims a reader looks up together because they govern one mechanism. Judge a candidate claim by whether it answers the same question the topic's existing claims answer, not by how many claims the topic already holds; splitting one broad claim into two precise ones is hygiene, not growth. Split a topic when it has acquired a second subject, and name the new topic for that subject.
  ```

  It must contain no digit and no project-specific identifier: the rule ships to every adopter
  and ADR-0194 item 8 requires it to be publication-safe and generic. Do not add a
  project-local `.awf/docs/parts/doc-standard/rules.md` override; that would replace the whole
  rules section and fork every shipped rule.

- [ ] **Task 1.2: Add the cohesion lens to the catalog default for `adr-reviewer`.** In
  `internal/catalog/standard.go`, in the `adr-reviewer` entry's `"focusItems"` slice (which
  currently holds `decision-clarity` and `consequences-honesty`), append a third element:

  ```go
  map[string]any{"name": "claim-topic-cohesion", "description": "each claim this ADR adds belongs in the topic its State changes names: it answers the same question that topic's existing claims answer, rather than landing there because the topic is adjacent or convenient. Flag a destination that gives its topic a second subject, and name the subject the claim belongs to instead. Judge by subject, never by how many claims the topic already holds."},
  ```

  No test asserts the length of this slice (verified: `internal/catalog/batch_test.go` reads
  only `plan-reviewer`'s `focusItems`), so this is a pure addition.

- [ ] **Task 1.3: Add the same lens to this project's `adr-reviewer` sidecar.** In
  `.awf/agents/adr-reviewer.yaml`, append to the existing `focusItems:` list (after the
  `claim-agrees-with-its-own-decision` entry):

  ```yaml
        - name: claim-topic-cohesion
          description: 'each claim this ADR adds belongs in the topic its State changes names: it answers the same question that topic''s existing claims answer, rather than landing there because the topic is adjacent or convenient. Flag a destination that gives its topic a second subject, and name the subject the claim belongs to instead. Judge by subject, never by how many claims the topic already holds.'
  ```

  This duplication is mandatory, not accidental: `internal/project/datamerge.go` merges
  sidecar data per key, so this file's `focusItems` replaces the catalog list wholesale.
  Editing only one of Task 1.2 and Task 1.3 either ships the lens to adopters while silently
  skipping this repository, or the reverse. The same hazard is recorded in `docs/pitfalls.md`
  for this agent's `docCurrencyItems`.

- [ ] **Task 1.4: Regenerate and confirm no drift.** Run `./x render`, then `./x check`.
  Expected terminal state: `awf check: clean`, with the pre-existing
  `rendering/workflow-skill-templates` claim-count note still present (Phase 2 removes it).
  The render must have updated `docs/doc-standard.md`, `examples/sundial/docs/doc-standard.md`,
  `.claude/agents/adr-reviewer.md`, `.pi/agents/adr-reviewer.md`, and both locks. Confirm the
  new rule reached the example adopter: `grep -c 'One subject per topic'
  examples/sundial/docs/doc-standard.md` returns a non-zero count.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction
  explicitly by path (no `git add -A`); run `go run ./cmd/awf check --staged` then `./x gate`.
  Both must pass before committing.

```commit
docs(rendering): add the topic-cohesion rule and review lens

Ships the guidance that replaces the topic claim-count advisory
ADR-0194 retires: a generic authoring rule in the doc-standard
template, and a cohesion lens on adr-reviewer.

The lens is added in both internal/catalog/standard.go and
.awf/agents/adr-reviewer.yaml because sidecar merging replaces
focusItems per key rather than appending to it.
```

## Phase 2: Retire the key, the note, and their two claims

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction, and it is indivisible for the three reasons named in the Architecture summary.
Checkbox tasks are ordered steps, not transaction boundaries; the working tree will not build
or check cleanly between them, and that is expected. Only the phase-close is verified.

This phase applies the first two declared operations only. It does **not** author
`config/migrations-and-locks:claim-budget-key-dropped` and does **not** place that claim's
proof marker; both belong to Phase 3, and landing either here would orphan a marker against a
claim that does not yet exist.

- [ ] **Task 2.1: Remove the config surface.** In `internal/config/config.go`:
  - delete the `MaxClaimsPerTopic *int \`yaml:"maxClaimsPerTopic"\`` field from
    `CurrentStateConfig`;
  - delete the `case "maxClaimsPerTopic":` arm from `(*CurrentStateConfig).UnmarshalYAML`,
    leaving the `default:` arm to reject the key with the existing
    `field %s not found in type config.CurrentStateConfig` error (this is ADR-0194 item 5's
    hard failure: no new error path is added);
  - delete the whole `EffectiveMaxClaimsPerTopic` method and its doc comment;
  - in the positive-integer validation loop, delete the
    `{"maxClaimsPerTopic", c.CurrentState.MaxClaimsPerTopic},` entry, leaving
    `maxTopicsPerPath` as the sole element.

  Leave `EffectiveMaxTopicsPerPath`, `MaxTopicsPerPath`, and the `decodeIntegerScalar` helper
  untouched; all three keep production callers.

- [ ] **Task 2.2: Delete the skeleton current-state type and its seed.** In
  `internal/config/edit.go`, delete the `CurrentState *SkeletonCurrentState
  \`yaml:"currentState,omitempty"\`` field from the skeleton struct and delete the
  `SkeletonCurrentState` type with its doc comment. In `internal/project/scaffold.go`, delete
  the `CurrentState: &config.SkeletonCurrentState{MaxClaimsPerTopic: 20},` line from the
  skeleton literal. Deleting the type rather than emptying it is required: an empty struct
  with no constructor would trip the ADR-0063 dead-code gate. A scaffolded tree now writes no
  `currentState` block at all, which ADR-0192 made behaviourally inert.

- [ ] **Task 2.3: Remove the advisory evaluator and its report field.** In
  `internal/topic/coverage.go`, delete the whole `ClaimBudgetNotes` function and its doc
  comment. In `internal/project/currentstate.go`:
  - delete the `Advisories: topic.ClaimBudgetNotes(ws.Loaded.Topics,
    ws.Cfg.CurrentState.EffectiveMaxClaimsPerTopic()),` line from the `CurrentStateReport`
    literal in `CheckCurrentState`;
  - delete the `Advisories []string` field from the `CurrentStateReport` struct;
  - rewrite `Notes()` so it no longer clones `r.Advisories`. Its body becomes:

    ```go
    func (r CurrentStateReport) Notes() []string {
        out := []string{}
        for _, c := range r.Coverage {
            if c.Severity == severity.Warn {
                out = append(out, coverageLine(c))
            }
        }
        return out
    }
    ```

    `out` is initialized to an empty slice rather than a nil `var` so `Notes()` keeps its
    existing non-nil-on-empty contract, which the removed nil guard used to supply. Remove the
    now-unused `slices` import from the file only if no other reference to it remains; check
    with `grep -n 'slices\.' internal/project/currentstate.go` after the edit.

- [ ] **Task 2.4: Remove the reference and configspec entries.** In
  `internal/project/configreference.go`, delete the `currentState.maxClaimsPerTopic` case,
  which is the `withDefault(strconv.Itoa(p.Cfg.CurrentState.EffectiveMaxClaimsPerTopic()),
  ...)` arm. In `internal/configspec/spec.go`, delete the whole entry whose
  `Path: "currentState.maxClaimsPerTopic"`, including its `Type`, `Default`, `Description`,
  and `Availability` fields.

- [ ] **Task 2.5: Add the generation-28 migration and its forward-port branch.** Create
  `internal/migrate/dropmaxclaimspertopic.go` with exactly this content:

  ```go
  package migrate

  import (
  	"bytes"
  	"fmt"
  	"io"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  )

  // applyDropMaxClaimsPerTopic ports schema 27 -> 28: currentState.maxClaimsPerTopic
  // is removed (ADR-0194), so awf check emits no topic claim-count note. config.yaml
  // is strict-parsed, so a surviving key would hard-fail on the new binary rather
  // than warn. The removal is announced because deleting a value an adopter
  // deliberately set must be readable from command output rather than recovered by
  // git archaeology. The edit routes through RemoveMappingKey because the key is
  // nested under currentState, which RemoveKey cannot reach.
  //
  // Unlike applyDropSeveritySettings, this migration seeds nothing when the removal
  // empties the block. ADR-0192 made topic coverage and fan-out evaluate
  // independently of currentState block presence, so a dropped block changes no
  // behaviour, and seeding would write back a key the adopter never set.
  func applyDropMaxClaimsPerTopic(root string, w io.Writer) error {
  	return editConfig(root, func(src []byte) ([]byte, error) {
  		out, err := config.RemoveMappingKey(src, "currentState", "maxClaimsPerTopic")
  		if err != nil {
  			return nil, err
  		}
  		if !bytes.Equal(out, src) {
  			fmt.Fprint(w, "drop-max-claims-per-topic: removed currentState.maxClaimsPerTopic\n")
  		}
  		return out, nil
  	})
  }
  ```

  In `internal/migrate/migrate.go`, append to `registry`, after the
  `{To: 27, Name: "adr-number-provenance", ...}` entry:

  ```go
  	{To: 28, Name: "drop-max-claims-per-topic", Apply: treeOnly(applyDropMaxClaimsPerTopic)},
  ```

  `Current()` returns `registry[len(registry)-1].To`, so this alone advances the generation to
  28; do not edit `Current()`.

  Then add the matching byte-level branch to `ConfigForCurrentSchema` in the same file, after
  the existing `if migration.To == 25 { ... }` block:

  ```go
  		if migration.To == 28 {
  			var err error
  			out, err = config.RemoveMappingKey(out, "currentState", "maxClaimsPerTopic")
  			if err != nil {
  				return nil, fmt.Errorf("migration %q (to %d): %w", migration.Name, migration.To, err)
  			}
  		}
  ```

  This branch is not optional and its omission is a silent trap. `awf check --staged` reads
  the before-side config from HEAD, whose lock still records schema 27, and forward-ports it
  through `ConfigForCurrentSchema` so the current strict parser can read it. Without this
  branch the retired key survives forward-porting, `config.ParseTree` rejects it, and the
  staged check fails on the very commit that removes the key. The generation-25 severity
  removal carries the same branch for the same reason.

- [ ] **Task 2.6: Test the migration to the 100% statement gate.** Create
  `internal/migrate/dropmaxclaimspertopic_test.go`, following the table shape of the sibling
  `internal/migrate/dropseveritysettings_test.go`. Do **not** add a proof marker: the claim it
  would name does not exist until Phase 3, and an orphaned marker fails `awf check`. Cover
  exactly these cases, each asserting both the resulting config bytes and the announcement
  text written to the writer:
  - key present alongside a sibling: `currentState:\n  maxTopicsPerPath: 8\n  maxClaimsPerTopic: 20\n`
    becomes `currentState:\n  maxTopicsPerPath: 8\n`, and the writer receives
    `drop-max-claims-per-topic: removed currentState.maxClaimsPerTopic\n`;
  - key is the block's only child: `currentState:\n  maxClaimsPerTopic: 20\n` leaves no
    `currentState` block at all (assert the output contains no `currentState`), and the
    announcement is still written. This case is the deliberate no-seed departure from
    generation 25 and must be asserted, not merely exercised;
  - key absent: a config with a `currentState` block that has no `maxClaimsPerTopic` is
    returned byte-identical and the writer receives nothing;
  - no `currentState` block at all: returned byte-identical, writer receives nothing;
  - malformed YAML: `applyDropMaxClaimsPerTopic` returns a non-nil error and writes no
    announcement.

  Cover the forward-port branch separately, because `ConfigForCurrentSchema` is a distinct
  function from the migration: assert that
  `ConfigForCurrentSchema([]byte("prefix: x\ncurrentState:\n  maxTopicsPerPath: 8\n  maxClaimsPerTopic: 20\n"), 27)`
  returns bytes with no `maxClaimsPerTopic`, and that a malformed input at `from` 27 returns
  an error whose message contains `drop-max-claims-per-topic`.

  Also assert registration: `Current() == 28` and the last registry entry's `Name` is
  `"drop-max-claims-per-topic"`. Run `go test ./internal/migrate/...`; expected terminal state
  `ok`.

- [ ] **Task 2.7: Map the generation and move the three generation pins.** In
  `internal/project/project.go`, add `28: "0.30.0",` to `minVersionBySchema` after the
  `27: "0.30.0",` entry. Do **not** change `const Version`: ADR-0194 item 4 records that
  0.30.0 is unreleased and generations 26 and 27 already share it, so the locks' `awfVersion`
  stays `0.30.0` and only `schemaVersion` moves. Then update the assertions that pin the
  current generation:
  - `internal/project/version_test.go`: the `minVersionBySchema[27] != Version` assertion
    remains true and stays as is; change the unmapped-schema probe from
    `ValidateSchemaMinimumVersion(28, Version)` to `ValidateSchemaMinimumVersion(29, Version)`,
    keeping the `no minimum` substring assertion and updating the failure message's generation
    number to match;
  - `internal/migrate/dropworkflowtelemetry_test.go`: change `if Current() != 27` to
    `if Current() != 28`.

  Run `go test ./internal/project/... ./internal/migrate/...`; expected terminal state `ok`.

- [ ] **Task 2.8: Remove the key from both config trees and upgrade both locks.** Delete the
  `  maxClaimsPerTopic: 20` line from `.awf/config.yaml` and from
  `examples/sundial/.awf/config.yaml`; both files retain `maxTopicsPerPath: 8`, so neither
  `currentState` block collapses. Then advance both locks by running the migration through the
  real upgrade path rather than hand-editing lock fields:

  ```
  go run ./cmd/awf upgrade
  (cd examples/sundial && go run ../../cmd/awf upgrade)
  ```

  Each must report the generation-28 migration and leave its `.awf/awf.lock` at
  `schemaVersion: 28` with `awfVersion: 0.30.0` unchanged. Because this task already removed
  the key by hand, the migration finds nothing to remove and prints no removal announcement;
  that is expected and is not a failure.

- [ ] **Task 2.9: Settle every test that reads the retired surface.** This is a batch task
  over an exhaustive site set; every site is parent-owned and there are no helpers. Each entry
  states its disposition, because "remove every reference" would hide a design choice where a
  test proves something worth keeping.

  Exact representative (marker deleted, test kept) in `internal/config/edit_test.go`: delete
  the line `// invariant: config/configuration:topic-claim-budget-configured` immediately above
  `func TestSetMappingInteger`, and rewrite that test's fixture data so it no longer uses the
  retired key as its sample nested key. In the case whose current expectation is
  `"# top\nprefix: x\ncurrentState:\n  maxClaimsPerTopic: 20\n"`, and in every sibling case in
  that table, substitute `maxTopicsPerPath` for `maxClaimsPerTopic` and keep the integer
  values unchanged. The test itself must survive: ADR-0194 item 6 keeps the historical
  migration that uses `SetMappingInteger`.

  Exact edge (test deleted outright) in `cmd/awf/check_test.go`: delete
  `func TestRunCheckClaimBudgetNote` together with its
  `// invariant: tooling/cli:topic-claim-budget-advisory` marker line, and delete
  `func TestRunCheckStagedSuppressesClaimBudgetNote`, which carries no marker but asserts the
  staged suppression of a note that no longer exists.

  Exhaustive affected-site set with dispositions:
  - `cmd/awf/check_test.go`: per the edge above.
  - `internal/config/edit_test.go`: per the representative above.
  - `internal/config/config_test.go`: four tests break, not one. Delete the
    `// invariant: config/configuration:topic-claim-budget-configured` marker above
    `TestCurrentStateDefaultsAndPresence` and drop that test's assertions reading
    `MaxClaimsPerTopic` or `EffectiveMaxClaimsPerTopic`, keeping its `maxTopicsPerPath` twins.
    In `TestCurrentStateStrictValidation`, remove `maxClaimsPerTopic: 20` from the `valid`
    fixture and delete the `zero claim maximum` and `negative claim maximum` cases. In
    `TestCurrentStateMaximumIntegerOverflow`, reduce the looped field list to
    `maxTopicsPerPath` alone. In `TestCurrentStateRejectsWrongValueTypes`, delete the five
    `maxClaimsPerTopic` fixtures, and rewrite the duplicate-key case
    `"prefix: x\ncurrentState:\n  maxClaimsPerTopic: 20\n  maxClaimsPerTopic: 21\n"` to use
    `maxTopicsPerPath` twice so the `already set` assertion survives. Deleting rather than
    re-pointing the other cases is coverage-safe because the surviving `maxTopicsPerPath`
    twins already exercise every `decodeIntegerScalar` branch; confirm with the gate, not by
    inspection.
  - `internal/configspec/spec_test.go`: delete the marker only; the marked test also asserts
    the surviving `currentState` configspec key set, so keep the test and remove
    `currentState.maxClaimsPerTopic` from its expected key list.
  - `internal/migrate/dropseveritysettings_test.go`: this file must change even though it
    tests a retained historical migration. Delete the
    `cfg.CurrentState.MaxClaimsPerTopic` assertion, which will not compile once Task 2.1
    lands; remove `maxClaimsPerTopic: 20` from the fixture that feeds `config.Parse`, which
    would otherwise fail strict parsing; and correct the
    `"currentState block must survive: three siblings remain"` message to name two siblings.
    Leave the fixtures at lines that only feed `applyDropSeveritySettings` byte-level helpers
    unchanged, since those never reach the strict parser.
  - `internal/topic/coverage_test.go`: delete every test and helper that calls the removed
    `ClaimBudgetNotes`.
  - `internal/project/currentstate_test.go`: delete
    `TestCheckCurrentStateClaimBudgetAdvisory` outright; it is wholly about the retired
    behaviour and has no surviving subject.
  - `internal/project/drift_test.go`: `TestClaimBudgetDriftIsLimitedToConsumingGuidance`
    proves that changing a `currentState` integer key drifts only the guidance that consumes
    it. That regression is about the drift mechanism, not about this key, so re-point it at
    `maxTopicsPerPath` and rename it to `TestTopicMaximumDriftIsLimitedToConsumingGuidance`
    rather than deleting it. Deleting it would lose the drift-scoping coverage with no
    replacement.
  - `internal/project/configreference_test.go` and `cmd/awf/config_test.go`: remove the
    assertions naming the retired key and the removed accessor, keeping their
    `maxTopicsPerPath` counterparts.

  Deterministic post-check, in order:
  - `grep -rn 'MaxClaimsPerTopic\|maxClaimsPerTopic\|ClaimBudgetNotes' --include='*.go' .`
    returns output only from `internal/migrate/maxclaimspertopic.go`,
    `internal/migrate/maxclaimspertopic_test.go`, `internal/migrate/dropmaxclaimspertopic.go`,
    `internal/migrate/dropmaxclaimspertopic_test.go`, and `internal/migrate/migrate.go`. Every
    other Go hit must be gone.
  - `grep -rn 'invariant: .*topic-claim-budget' --include='*.go' .` returns no output. The
    grep is scoped to the marker form deliberately: the plain string `topic-claim-budget`
    survives in `internal/migrate/maxclaimspertopic_test.go`, which asserts the retained
    historical migration's announcement text, and must not be treated as a leftover.

- [ ] **Task 2.10: Author the two claim removals.** In
  `.awf/topics/parts/tooling/cli/current-state.md`, delete the whole
  `### \`invariant: topic-claim-budget-advisory\`` block including its prose, `Origin:`, and
  `Backing:` lines. In `.awf/topics/parts/config/configuration/current-state.md`, delete the
  whole `### \`invariant: topic-claim-budget-configured\`` block the same way. Do not touch
  `.awf/topics/parts/config/migrations-and-locks/current-state.md` in this phase; the third
  operation is Phase 3's.

- [ ] **Task 2.11: Move ADR-0194 to Implementing and record the first batch.** In
  `docs/decisions/0194-retire-the-topic-claim-count-advisory-for-authoring-guidance-and-a-review-lens.md`:
  - change the frontmatter `status: Proposed` to `status: Implementing`. This is required, not
    cosmetic: `internal/adr/application.go` rejects applied operations under a `Proposed`
    status, and rejects an `Implementing` status whose operations are all applied. Applying
    two of three leaves one pending, which is exactly the state `Implementing` requires.
  - append to `## Status history`, after the existing `- 2026-07-31: Proposed` line, exactly
    these two events in this order:

    ```
    - <today>: Implementing; content-sha256: <digest>
    - <today>: Applied; operations: remove `tooling/cli:topic-claim-budget-advisory`, remove `config/configuration:topic-claim-budget-configured`
    ```

  The two operations must appear in the ADR's `State changes` declaration order, which is the
  order written above, and each id sits in an inline code span. The `Implementing` event
  carries a `content-sha256`; the `Applied` event carries `operations:` and **no** digest.

  `<digest>` is `adr.ContentDigest` over the five canonical sections (Context, Decision, State
  changes, Consequences, Alternatives Considered) in that fixed order, excluding frontmatter
  and Status history. It is not a `sha256sum` of the file. Obtain it after Task 2.14's render,
  when `awf check` is otherwise clean: write any 64-character lowercase hex placeholder, run
  `go run ./cmd/awf check`, and read the real value from the resulting
  `latest stamped content-sha256 "<placeholder>" does not match the computed digest "<real>"`
  error. Substitute it and re-run; that error must disappear. Running this before the render
  risks the message being masked by unrelated drift failures.

- [ ] **Task 2.12: Update the authored doc sources.** Edit authored sources only; never a
  generated file.
  - `templates/docs/working-with-awf.md.tmpl`: delete the sentences describing the advisory.
    The paragraph currently ends `...which vars are set, what consumes them, and what enabling
    would activate. \`currentState.maxClaimsPerTopic\` is a positive topic-size advisory ... while
    an omitted value still reads as 20.` Truncate it so it ends at `...what enabling would
    activate.` and delete the rest of the paragraph.
  - `templates/docs/agents-md-standard.md.tmpl`: in the `rules` section paragraph, delete the
    single sentence `A configured \`currentState.maxClaimsPerTopic\` note is advisory and never
    truncates either projection. ` leaving the surrounding sentences and their spacing intact.
  - `.awf/docs/parts/testing/gate.md`: delete the leading clause of the paragraph that begins
    `Current-state checks also emit a non-failing working-tree note for each topic strictly
    above \`currentState.maxClaimsPerTopic\`; equality is quiet, staged checks suppress the
    advisory, and the example adopter is required to produce no notes.` Retain the
    example-adopter requirement, which ADR-0194 leaves in force and
    `internal/project/example_wiring_test.go` still asserts: replace the deleted clause with
    `The example adopter is required to produce no notes.` and leave the rest of the paragraph
    from `Repository regression tests keep bare...` unchanged.
  - `.awf/docs/parts/roadmap/ideas.md`: delete the whole bullet beginning `- Promote the
    topic-claim-budget advisory from a non-failing note to a fixed blocking rank`, through its
    final `...so the promotion is to a rank fixed in code or not at all.` Delete it rather than
    rewriting it: ADR-0194 item 12 records that retiring the check makes the idea incoherent,
    not merely stale.

- [ ] **Task 2.13: Add the changelog entry.** In `changelog/CHANGELOG.md`, add a new bullet to
  the existing `### Breaking changes` list under `## [Unreleased]`, matching the surrounding
  entries' voice:

  ```
  - Remove the `currentState.maxClaimsPerTopic` config key and the non-failing topic
    claim-count note `awf check` emitted from it. Schema generation 28 removes the key from an
    existing tree; because `config.yaml` is strict-parsed, a tree that still carries the key
    fails to load on this binary until `awf upgrade` runs. Topic cohesion is now an authoring
    and review concern: see the `One subject per topic` rule in the documentation standard.
  ```

- [ ] **Task 2.14: Regenerate and verify the end state.** Run `./x render`, then `./x check`.
  Expected terminal state: `awf check: clean` with **no** claim-count note in the output; the
  `rendering/workflow-skill-templates` note that has appeared on every prior run must now be
  absent. Confirm the retirement reached every surface:
  - `grep -rn 'maxClaimsPerTopic' docs/ examples/sundial/docs/ AGENTS.md` returns hits only
    inside `docs/decisions/` and `docs/plans/`, which are frozen or in-flight records;
  - `go run ./cmd/awf check invariants` reports no unbacked or orphaned claim.

  Task 2.11's digest is resolved here, once this render leaves the tree otherwise clean.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction
  explicitly by path (no `git add -A`); run `go run ./cmd/awf check --staged` then `./x gate`.
  Both must pass. The gate must report 100% statement coverage and no dead code; if the new
  migration or its forward-port branch leaves an uncovered statement, extend Task 2.6's table
  rather than adding a `coverage-ignore`.

```commit
refactor(config): retire the topic claim-count advisory

Removes currentState.maxClaimsPerTopic and the awf check note it drove,
across config, configspec, the generated reference, the scaffold
skeleton, and the topic evaluator, and ships schema generation 28 to
remove the key from an adopter tree. Strict parsing rejects a survivor,
so an unmigrated tree fails until awf upgrade runs.

One transaction because it cannot be sliced: the key must leave both
config trees in the same commit the field leaves internal/config, claim
and proof-marker symmetry is enforced in both directions, and
migrate.Current() obliges both locks to advance together.

Applies the first ADR-0194 batch, retiring
tooling/cli:topic-claim-budget-advisory and
config/configuration:topic-claim-budget-configured, and moves the
decision to Implementing with one operation still pending.
```

## Phase 3: Add the backing claim and flip

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction. It runs **after terminal review has settled**, not immediately after Phase 2:
the executing-plans contract places the `Implemented` flip after terminal review, and the
lifecycle cannot represent an all-applied `Implementing` state, so the final operation and the
flip travel together here.

- [ ] **Task 3.1: Author the migration claim.** In
  `.awf/topics/parts/config/migrations-and-locks/current-state.md`, add:

  ```
  ### `invariant: claim-budget-key-dropped`

  Schema generation 28 removes currentState.maxClaimsPerTopic from a config tree, announcing the removal it performs, and leaves every other configured key and its value intact. It seeds no replacement child, so a currentState block whose only remaining member was the retired key is dropped with it.
  Origin: ADR-0194
  Backing: test
  ```

  That file orders its claims alphabetically by slug, so place this block between
  `### \`invariant: awf-relocation-migration\`` and
  `### \`invariant: close-enabled-set-migration\``. Do not add a `Revised-by:` line: the claim
  is new.

- [ ] **Task 3.2: Place the proof marker.** In `internal/migrate/dropmaxclaimspertopic_test.go`,
  add the line `// invariant: config/migrations-and-locks:claim-budget-key-dropped`
  immediately above the test that asserts the removal behaviour and the no-seed block drop.
  Marker and claim must land in the same commit; either alone fails `awf check`.

- [ ] **Task 3.3: Apply the final batch and flip ADR-0194.** In
  `docs/decisions/0194-retire-the-topic-claim-count-advisory-for-authoring-guidance-and-a-review-lens.md`,
  change the frontmatter `status: Implementing` to `status: Implemented`, and append to
  `## Status history`, in this order:

  ```
  - <today>: Applied; operations: add `config/migrations-and-locks:claim-budget-key-dropped`
  - <today>: Implemented; content-sha256: <digest>
  ```

  `<digest>` is the same value Phase 2 stamped, because no canonical section changes between
  the two phases; confirm rather than assume by the `awf check` mismatch procedure in Task
  2.11. With all three operations applied, `Implemented` has no remaining operations, which is
  what `internal/adr/application.go` requires of a terminal status.

- [ ] **Task 3.4: Freeze this plan.** Change this file's frontmatter `status: Proposed` to
  `status: Implemented`, and record in the Notes section any finding surfaced during
  implementation: a wrong diff, an unsliceable phase, a bad estimate.

- [ ] **Task 3.5: Regenerate and verify.** Run `./x render`, then `./x check`; expected
  terminal state `awf check: clean`. Confirm the claim landed with its proof:
  `go run ./cmd/awf topic config/migrations-and-locks` lists `claim-budget-key-dropped` with
  its proof site, and `go run ./cmd/awf check invariants` reports it backed.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction
  explicitly by path (no `git add -A`); run `go run ./cmd/awf check --staged` then `./x gate`.
  Both must pass.

```commit
feat(invariants): back the generation-28 key removal (0194 batch)

Applies the final ADR-0194 operation: adds
config/migrations-and-locks:claim-budget-key-dropped with its proof
marker on the generation-28 migration test, pinning the deliberate
absence of a block-preservation seed.

Flips ADR-0194 to Implemented and freezes the plan. The operation is
deferred to this batch because the lifecycle rejects an all-applied
Implementing state while the Implemented flip must follow terminal
review.
```

## Verification

Beyond the per-phase gates, the effort is done when all of the following hold:

- `./x gate` passes from a clean tree, including 100% statement coverage and the dead-code
  gate, which together prove no orphaned helper survived the removal.
- `./x check` emits no note of any kind for this repository, and `./x check` on the example
  adopter emits none either (the runner already enforces the latter and fails the check with
  `the example adopter has advisory notes` otherwise).
- A tree still carrying the retired key fails loudly rather than silently. Verify by hand:
  temporarily re-add `maxClaimsPerTopic: 20` to `.awf/config.yaml`, run
  `go run ./cmd/awf check`, confirm the error names
  `field maxClaimsPerTopic not found in type config.CurrentStateConfig`, then remove the line
  again and confirm the check returns to clean.
- `go run ./cmd/awf upgrade` on a tree pinned at schema 27 advances it to 28 and prints the
  removal announcement.
- After Phase 3, `go run ./cmd/awf check` validates ADR-0194's terminal status: all three
  declared operations applied, none remaining.

## Notes

- **The ADR number will change.** ADR-0194 collides with an unmerged
  `0194-slug-identified-pending-adrs-numbered-at-integration.md` on branch
  `awf/eliminate-manual-adr-renumbering-when-parallel-efforts-merge`. 0194 was nonetheless the
  only legal number here: `awf check` enforces ADR contiguity from 1, and the lock's
  `legacyAdrGaps` is immutable authority that rejects any gap at or above the format cutoff, so
  a deliberate hole could not be recorded. Whichever effort integrates second renumbers. If
  that is this one, renumber by rewriting the unpublished branch and rebasing onto main, never
  inside a merge, and do it **before** Phase 2 lands its `Applied` event: the staged check's
  history-prefix rule refuses a renumber once an Applied batch exists. Renumbering after Phase
  2 means rewriting that commit too, including this plan's `adrs:` frontmatter and every
  in-file `ADR-0194` reference.
- **Out of scope, worth fixing separately:** `docs/topics/tooling/git-access.md.awf-bak` is a
  stray backup file committed to main by the git-seam merge. It is tracked, `awf check` does
  not flag it, and it belongs to no phase here.
- `internal/project/example_wiring_test.go` and its claim
  `tooling/quality-gates:example-zero-notes` are deliberately untouched. That assertion greps
  `^note: ` over the example adopter's check output and is note-class-agnostic, so it stays
  meaningful with one note class fewer.
