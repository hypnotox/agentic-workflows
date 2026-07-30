---
format: current-state-v2
status: Implementing
date: 2026-07-30
---
# ADR-0179: Drop configurable severity and unify the finding rank

## Context

awf encodes severity four separate times, and no current-state claim governs any of it.

`internal/audit/audit.go:23` declares `type Severity int` with `Warning Severity = iota` then `Error`,
rendering "error" and "warning". `internal/currentstate/check.go:13` declares a second `type Severity int`
with the members in the opposite order, `Error Severity = iota` then `Warn`, rendering "error" and "warn".
`internal/topic/coverage.go:56` declares `type CoverageSeverity string` with "error", "warn", and "off".
`cmd/repoaudit/main.go:24` declares a private `severity` whose `label()` at `:31` re-implements audit's
"error" and "warning" independently. Configuration accepts a fifth spelling set:
`internal/config/config.go:553` validates the bare strings "error", "warn", and "off".

The user-visible result is incoherent. An adopter writes `warn` in `.awf/config.yaml` and awf reports
`warning` for audit findings and `warn` for current-state findings.

Three further observations shaped the decision.

First, `internal/currentstate.Severity` carries no production information. All 41 `Finding{...}` literals
across `internal/currentstate/check.go` and `internal/currentstate/transition.go` pass `Error`, and
`internal/project/currentstate.go:409` maps every current-state claim-handshake finding to `audit.Error`
unconditionally, discarding the field. The only consumer of `Warn` is `TestSeverityString` at
`internal/currentstate/check_test.go:480`. ADR-0134 item 11 authorized configured severity for
`currentState.topicCoverage` and `currentState.topicFanout` only, and both are implemented correctly in
`internal/topic`. No decision specifies an advisory claim-handshake finding, and the findings that package
produces are structural violations: a claim with two add operations, a non-contiguous state-sequence, an
Origin naming the wrong ADR.

Second, `off` is not a rank. `internal/topic/coverage.go:135` and `:142` suppress a finding class before
emission, so an emitted finding never carries `off`. One three-member type therefore makes
`Finding{Severity: Off}` representable and meaningless everywhere a finding is constructed. Worse, `off`
is not confined to configuration: `internal/project/context.go:371` passes `Fanout: topic.CoverageOff` as a
hard-coded control-flow switch, so `CoveragePolicy` conflates which checks run with how bad their results
are.

Third, the surface is small. `internal/configspec/spec.go:154` and `:159` show that
`currentState.topicCoverage` and `currentState.topicFanout` are the only two severity-valued settings in the
entire config schema. Both are nonetheless set explicitly in the in-repo adopter tree at
`examples/sundial/.awf/config.yaml`, whose lock is at the current schema generation.

This decision is a deliberate simplification taken before the second code-design pattern topic is authored,
so that topic describes a design rather than a mess. Working notes for the effort live under `.awf/efforts/`.

## Decision

1. Remove the `currentState.topicCoverage` and `currentState.topicFanout` configuration keys. Topic
   coverage and topic fan-out always evaluate. Coverage reports at error, fan-out reports at warn. Both
   ranks are fixed in code and no longer configurable. `currentState.maxTopicsPerPath` and
   `currentState.maxClaimsPerTopic` remain configured and are unaffected.

2. Remove `off` from the severity vocabulary. A severity value ranks a produced finding and never
   suppresses one.

3. Replace the three surviving severity encodings with one shared rank type carrying exactly two members,
   spelled `error` and `warn`. `internal/audit.Severity`, `internal/topic.CoverageSeverity`, and the private
   `severity` in `cmd/repoaudit/main.go` all resolve to it. Item 5 retires the fourth encoding rather than
   unifying it. No finding rank renders as "warning": today only `internal/audit/audit.go:34` and
   `cmd/repoaudit/main.go:35` emit that token as a rank, and other unrelated uses of the word are out of
   scope. `cmd/repoaudit` adopts the shared type despite its file header placing it outside the shipped
   standard: its comment at `main.go:22` justifying the duplication because "repoaudit is standalone repo
   tooling" is already false, since its production code imports `internal/git`, and no current-state claim
   pins its isolation.

4. The shared rank type lands in a new leaf package that owns no other concern. `internal/audit` and
   `internal/topic` import each other in neither direction today, so housing the rank in either one would
   make an unrelated sibling depend on it purely to borrow a type, which
   `docs/maintainable-code-design.md` names as dependency inversion by accident. Because a new package
   under no `.awf/domains/*.yaml` selector is an unowned path, and because item 1 fixes coverage at error,
   the same implementation must give the new package domain ownership and scoped topic coverage or
   `awf check` fails at error severity on the commit that introduces it. That obligation is discharged by
   extending the `tooling` domain and the `tooling/audit-and-snapshots` topic selectors to match the new
   package, which needs no further claim: the topic is already claim-bearing, and an empty topic shell would
   not satisfy coverage, since `internal/topic/coverage.go:180` skips any topic carrying no claims. The
   topic governs the audit finding surfaces the rank is emitted from, so the fit is topical rather than
   incidental.

5. Claim-handshake findings carry no rank. `internal/currentstate.Severity` and the `Severity` field on
   `internal/currentstate.Finding` are removed rather than wired up: every finding that package produces is
   an error, which is what `internal/project/currentstate.go:409` and `:41` already do: the audit path
   forces `audit.Error` and the check path reads only `f.Message`. This is scoped to the
   provenance and transition findings `internal/currentstate` produces. The ranked coverage and fan-out
   findings that `internal/project.CurrentStateReport` also carries, and the `audit.Warning` finding
   constructed at `internal/project/currentstate.go:404`, keep their ranks and are out of scope.

6. Advance the schema generation and add a migration that removes both keys from a config tree wherever
   they appear, following the established drop-a-key migrations `internal/migrate/dropauditbase.go`,
   `drophooks.go`, `dropreplacewith.go`, and `dropworkflowtelemetry.go`. The migration scope includes both
   in-repo trees: the root `.awf/config.yaml` at lines 53 and 54, and
   `examples/sundial/.awf/config.yaml`, each set both keys and must land at the new generation with them
   removed, with both trees and locks re-rendered in the same commit. awf is its own first adopter, so
   omitting the root tree would leave the project failing its own strict `currentState` validation.

7. Update the affected documentation in the same implementation, since this change falsifies or stales
   authored and generated prose: the `topic coverage` entry in `.awf/docs/glossary.yaml`, which currently
   states that the two keys and `maxTopicsPerPath` "tune the severities"; the roadmap idea in
   `.awf/docs/parts/roadmap/ideas.md` proposing to promote the topic-claim-budget advisory to a
   configurable severity with an adopter-facing config key, which this decision forecloses; a Breaking
   changes entry in `changelog/CHANGELOG.md` under `[Unreleased]`, since removing config-schema keys is
   adopter-facing; and the regenerated `docs/config-reference.md` and `docs/glossary.md` in both the root and
   `examples/sundial` trees. The same obligation covers every production doc comment and authored description
   this falsifies, including `internal/project/currentstate.go:26-28`, `:51-52` and `:445-446`,
   `internal/topic/coverage.go:53-55`, `:89-90` and `:112-121`, `internal/project/context.go:361-363`,
   `cmd/repoaudit/main.go:22-23`, and the `currentState.maxTopicsPerPath` description in
   `internal/configspec/spec.go`, which is the authored source of the regenerated config reference above. The
   list is illustrative rather than exhaustive: the obligation is that no authored prose survives describing a
   severity as configured or suppressible. Every status transition commits the regenerated
   `docs/decisions/INDEX.md` and lock from `./x render`.

8. Replace `topic.CoveragePolicy`'s per-check severity with an explicit selection of which checks to
   evaluate. A caller that does not want fan-out findings does not ask for them, rather than asking and
   suppressing the result with a rank value. The uncovered report reached from
   `internal/project/context.go` selects coverage only, preserving its current output.

## State changes

- add `config/configuration:severity-not-configurable`
- add `config/migrations-and-locks:severity-keys-dropped`
- add `tooling/audit-commands:severity-single-spelling`
- add `invariants/topics-and-markers:coverage-evaluation-selects-checks`
- add `invariants/current-state-authority:currentstate-handshake-findings-unranked`

## Consequences

The output becomes coherent: one spelling of each rank across `awf check`, `awf audit`, `awf context`, and
the repo-local audit tool. Removing `off` removes a representable-but-meaningless state from every finding
construction site, and forcing the `CoveragePolicy` split turns a magic rank value into an explicit
request, which is a cohesion improvement the removal pays for rather than a separate refactor.

ADR-0134 item 11's authorization of configured severity was never converted into a current-state claim, so
there is no claim to update or remove and a new `config/configuration` claim is the only available
mechanism. An ADR is history rather than active authority, so narrowing what an earlier one authorized is
expressed by the claim this decision adds, not by an operation against ADR-0134.

No existing claim requires an update, and that conclusion rests on a complete survey rather than a sample.
Ten claims across three topics name a rank by the token this decision abolishes. Seven are in
`tooling/audit-and-snapshots`: `audit-dependency-warn`, `audit-domain-code-staleness`,
`audit-domain-doc-staleness`, `audit-plain-punctuation`, `audit-plan-threshold-warn`,
`audit-undocumented-domain`, and `repo-audit-error-exit`. Two are in `tooling/audit-commands`:
`audit-reports-evaluated-scope` and `audit-warn-exit-zero`. One is in `tooling/changelog-and-release`,
distinguishing a Warning with a zero exit code from an Error. Each names a rank CLASS whose
behaviour is unchanged, not a rendered token, so each stays true after the token changes. An update
carrying only a renamed severity word would in any case fail
`invariants/current-state-authority:update-requires-substance`.
`invariants/current-state-authority:uncovered-lists-unowned-unignored` governs which paths are listed as
unowned rather than the coverage evaluation path, so the `CoveragePolicy` split leaves it untouched.

Adopters lose the ability to silence topic-coverage findings. That capability is being withdrawn rather
than migrated, and the migration removes the keys unconditionally, so a tree that had set `off` starts
seeing findings after upgrading. This is accepted deliberately: always-active is the intent. The practical
exposure is nil: per the maintainer's confirmed knowledge of external adopter state no external tree sits
near the current generation, and both in-repo trees set `error` and `warn`, matching the schema defaults, so
the migration is behaviour-neutral everywhere. Their only cost is the key removal and lock re-render item 6
covers. No tree in the repository sets `off`, so the withdrawn suppression has zero live instances.

The test corpus pays a migration cost beyond golden output, and the sites differ from one another.
`cmd/awf/check_test.go:115` hard-codes `topicFanout: off` inside the shared `coverageYAML` helper while
parameterizing `topicCoverage`, so that helper's severity parameter becomes dead and its four callers may
gain fan-out lines they do not assert. `cmd/awf/check_test.go:164` and `:244` set `topicCoverage: off` to
suppress coverage entirely, so each must supply real scoped topic coverage or assert the findings it now
produces. `internal/config/edit_test.go:123` is different again: it anchors a comment-preservation case on
`topicCoverage` purely as a YAML child and must be re-anchored on a surviving `currentState` key.
`TestCurrentStateSeverityValidation` at `internal/config/config_test.go:477` retires with the keys, while
the `topicCoverage` and `topicFanout` sub-cases inside `TestCurrentStateStrictValidation` (`:507`),
`TestCurrentStateRejectsNonStringScalars` (`:591`), and `TestCurrentStateRejectsWrongValueTypes` (`:620`)
drop without retiring those tests, whose `sources`, `testGlobs`, `maxTopicsPerPath`, and
`maxClaimsPerTopic` cases all survive. Audit and repo-audit golden output changes from "warning" to "warn"
in the same commit.

Fan-out stays advisory. Promoting it to error was considered and rejected below; keeping it at warn
preserves ADR-0134's posture and avoids a gate failure the next time a path accumulates enough scoped
topics to exceed `currentState.maxTopicsPerPath`, which `internal/project` is already close to doing.

The new leaf package in item 4 is a small structural cost paid to avoid an accidental dependency, and item
1 makes that cost concrete: fixing coverage at error means the introducing commit must also give the
package domain ownership and a scoped topic. The `coverage-evaluation-selects-checks` claim lands in
`invariants/topics-and-markers` because coverage evaluation is `internal/topic` code and that topic's
selectors are the only ones matching it; its metadata summary needs widening in the same commit to cover
coverage evaluation policy alongside parsing and resolution. `severity-single-spelling` lands in
`tooling/audit-commands` rather than `tooling/cli`: three tooling topics share one identical selector set and
a fourth is a strict superset of it, so selector uniqueness cannot choose between them, and the rank tokens
are emitted by the advisory audit
surfaces that topic already governs, alongside the two claims this one is nearest,
`audit-warn-exit-zero` and `audit-reports-evaluated-scope`.

The schema generation advances, so the binary-version gate refuses an older binary against a migrated tree
until the release ships, per ADR-0039.

This unblocks the second code-design pattern topic, which covers how awf represents and surfaces outcomes.
Two claims that topic would otherwise need, one requiring severity spellings to match configuration and one
stating that `off` is never a finding rank, become unnecessary once this decision lands, reducing that topic
from five claims to two.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the knobs and add a second type separating configured strictness from finding rank | The split exists only to house `off`; removing `off` collapses the two types into one honest rank, and it preserves a config surface with no demonstrated use. |
| Keep one three-member type including `off` | Makes a meaningless state representable at every finding construction site, and leaves `off` doing control flow at `internal/project/context.go:371`. |
| Promote fan-out to error along with coverage | Reverses ADR-0134's stated posture of reporting excessive fan-out "without pretending every overlap is an error", and would fail the gate as soon as one more scoped topic matches a path already near `maxTopicsPerPath`. |
| House the shared rank in `internal/audit` | Makes `internal/topic`, which imports it in no direction today, depend on a policy package purely to borrow a type. |
| House the shared rank in `internal/topic` | The same accidental coupling in the other direction: an `internal/audit` finding has nothing to do with topics. |
| Keep two rank types and claim only that both spell their values identically | Identical spelling maintained by convention is exactly the drift this decision removes, and a vocabulary-only fix was weighed and set aside in favour of one type. |
| Keep `off` for coverage only, for adopters onboarding | Retains the policy-versus-rank conflation and would require a claim stating that `off` is never a finding rank, for a case no adopter has. |
| Wire `currentstate.Warn` up instead of removing it | No decision specifies an advisory claim-handshake finding, and the findings that package produces are structural violations that are never advisory. |
| Leave severity as-is and let the outcome-modeling topic describe it | Ships claims that document an incoherence instead of a design, two of which would retire immediately afterwards. |

## Status history

- 2026-07-30: Proposed
- 2026-07-30: Implementing; content-sha256: a0c621987223c8d720be545d5224fc1d70fb4f0df5614879a3379a40406fb760
- 2026-07-30: Applied; state-sequence: 90; operations: add `config/configuration:severity-not-configurable`, add `config/migrations-and-locks:severity-keys-dropped`
- 2026-07-30: Applied; state-sequence: 91; operations: add `tooling/audit-commands:severity-single-spelling`, add `invariants/topics-and-markers:coverage-evaluation-selects-checks`
