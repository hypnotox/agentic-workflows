---
format: current-state-v2
status: Proposed
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
"error" and "warning" independently. Configuration accepts a fifth spelling set: `internal/config/config.go:553`
validates the bare strings "error", "warn", and "off".

The user-visible result is incoherent. An adopter writes `warn` in `.awf/config.yaml` and awf reports
`warning` for audit findings and `warn` for current-state findings.

Three further observations shaped the decision.

First, `internal/currentstate.Severity` carries no production information. All 41 `Finding{...}` literals
across `internal/currentstate/check.go` and `internal/currentstate/transition.go` pass `Error`, and
`internal/project/currentstate.go:409` maps every current-state finding to `audit.Error` unconditionally,
discarding the field. The only consumer of `Warn` is `TestSeverityString` at
`internal/currentstate/check_test.go:480`. ADR-0134 item 11 authorized configured severity for
`currentState.topicCoverage` and `currentState.topicFanout` only, and both are implemented correctly in
`internal/topic`. No decision specifies an advisory current-state finding, and the findings the package
produces are structural violations: a claim with two add operations, a non-contiguous state-sequence, an
Origin naming the wrong ADR.

Second, `off` is not a rank. `internal/topic/coverage.go:135` and `:142` suppress a finding class before
emission, so an emitted finding never carries `off`. One three-member type therefore makes
`Finding{Severity: Off}` representable and meaningless everywhere a finding is constructed. Worse, `off`
is not confined to configuration: `internal/project/context.go:371` passes `Fanout: topic.CoverageOff` as a
hard-coded control-flow switch, so `CoveragePolicy` conflates which checks run with how bad their results
are.

Third, the surface is small. `internal/configspec/spec.go:154` and `:159` show that `currentState.topicCoverage`
and `currentState.topicFanout` are the only two severity-valued settings in the entire config schema.

This decision is a deliberate simplification taken before the second code-design pattern topic is authored,
so that topic describes a design rather than a mess. Working notes for the effort live under `.awf/efforts/`.

## Decision

1. Remove the `currentState.topicCoverage` and `currentState.topicFanout` configuration keys. Topic
   coverage and topic fan-out always evaluate. Coverage reports at error, fan-out reports at warn. Both
   ranks are fixed in code and no longer configurable. `currentState.maxTopicsPerPath` and
   `currentState.maxClaimsPerTopic` remain configured and are unaffected.

2. Remove `off` from the severity vocabulary. A severity value ranks a produced finding and never
   suppresses one.

3. Replace the four severity encodings with one shared rank type carrying exactly two members, spelled
   `error` and `warn`. `internal/audit.Severity`, `internal/topic.CoverageSeverity`, and the private
   `severity` in `cmd/repoaudit/main.go` all resolve to it. The spelling "warning" is emitted nowhere.
   `cmd/repoaudit` adopts the shared type; its comment at `main.go:22` justifying duplication because
   "repoaudit is standalone repo tooling" is already false, since its production code imports
   `internal/git`, and no current-state claim pins its isolation.

4. Replace `topic.CoveragePolicy`'s per-check severity with an explicit selection of which checks to
   evaluate. A caller that does not want fan-out findings does not ask for them, rather than asking and
   suppressing the result with a rank value. The uncovered report reached from `internal/project/context.go`
   selects coverage only, preserving its current output.

5. Current-state findings carry no rank. `internal/currentstate.Severity` and the `Severity` field on
   `internal/currentstate.Finding` are removed rather than wired up: every current-state finding is an
   error, which is what the code already does at `internal/project/currentstate.go:409`. This resolves the
   dead member rather than leaving a production type whose only non-zero value exists for a test.

6. Advance the schema generation and add a migration that removes both keys from a config tree wherever
   they appear, following the established drop-a-key migrations `internal/migrate/dropauditbase.go`,
   `drophooks.go`, `dropreplacewith.go`, and `dropworkflowtelemetry.go`.

## State changes

- add `config/configuration:severity-not-configurable`
- add `tooling/cli:severity-single-spelling`
- add `invariants/topics-and-markers:coverage-evaluation-selects-checks`
- add `invariants/current-state-authority:current-state-findings-unranked`

## Consequences

The output becomes coherent: one spelling of each rank across `awf check`, `awf audit`, `awf context`, and
the repo-local audit tool. Removing `off` removes a representable-but-meaningless state from every finding
construction site, and forcing the `CoveragePolicy` split turns a magic rank value into an explicit
request, which is a cohesion improvement the removal pays for rather than a separate refactor.

Adopters lose the ability to silence topic-coverage findings. That capability is being withdrawn rather
than migrated, and the migration removes the keys unconditionally, so an adopter that had set `off` starts
seeing findings after upgrading. This is accepted deliberately: always-active is the intent, and the keys
were introduced well after the generation any adopter tree currently sits at.

Fan-out stays advisory. Promoting it to error was considered and rejected below; keeping it at warn
preserves ADR-0134's posture and avoids a gate failure the next time a path accumulates enough scoped
topics to exceed `currentState.maxTopicsPerPath`, which `internal/project` is already close to doing.

Audit output changes for anything reading it textually: "warning" becomes "warn". Golden tests covering
audit and repo-audit output change with the same commit. The behaviour behind
`tooling/audit-commands:audit-warn-exit-zero` and `tooling/audit-and-snapshots:repo-audit-error-exit` is
unchanged, so neither claim is updated; each still describes what its command does, and an update carrying
only a renamed severity word would not satisfy
`invariants/current-state-authority:update-requires-substance`.

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
| Keep `off` for coverage only, for adopters onboarding | Retains the policy-versus-rank conflation and would require a claim stating that `off` is never a finding rank, for a case no adopter has. |
| Wire `currentstate.Warn` up instead of removing it | No decision specifies an advisory current-state finding, and the findings the package produces are structural violations that are never advisory. |
| Leave severity as-is and let the outcome-modeling topic describe it | Ships claims that document an incoherence instead of a design, two of which would retire immediately afterwards. |

## Status history

- 2026-07-30: Proposed
