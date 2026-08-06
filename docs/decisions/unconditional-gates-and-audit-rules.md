---
format: current-state-v4
slug: unconditional-gates-and-audit-rules
status: Proposed
date: 2026-08-06
---
# ADR-unconditional-gates-and-audit-rules: Unconditional Gates And Audit Rules


## Context

Four singletons and nine audit settings let a repository choose how much of awf's checking applies
to it. The hooks payloads render only when `hooks.enabled` is set (ADR-0048); the `./awf` wrapper
only when `runner.enabled` is set (ADR-0101, ADR-0156); the prose gate scans only when
`proseGate.enabled` is set (ADR-0119); the memory-citation gate only when `memoryCite.enabled` is
set (ADR-0158). `awf audit` carries five per-rule booleans (ADR-0019, ADR-0025, ADR-0077,
ADR-0117), plus a commit-type list, a subject-length limit, a diff threshold and a dependency
manifest glob set (ADR-0017).

Two distinct arguments produced that surface. The audit booleans came from ADR-0019's decision to
"make each rule independently disable-able ... so an adopter can silence either nudge", carried
forward by each later rule citing the shape. The two scanner knobs came from a sharper argument
about defaults: ADR-0119 defaulted the prose gate to false because it blocks, reasoning that
"presence-level enforcement on an unswept tree costs them their build, so they must ask for it",
and ADR-0158 mirrored that for memory citations because otherwise the policy would be "half-on for
every adopter the moment they upgrade". Both arguments are about a repository awf arrives at
without warning.

The house-standard record withdraws that premise. There is no repository awf arrives at unbidden;
there is a set of repositories its owner sets up, where a one-time sweep before enabling a blocking
scanner is a setup step rather than an imposition.

The measurement is decisive. This checkout sets `hooks.enabled`, `runner.enabled`,
`proseGate.enabled` and `memoryCite.enabled` all true, and carries none of the nine audit settings
except `allowedScopes`: the five booleans, the type list, the subject limit, the diff threshold and
the manifest globs all run at their built-in defaults. The entire audit tuning surface expresses
nothing here, and the four singleton toggles express nothing except agreement.

ADR-0117 recorded the counter-pressure that now decides the matter: "a default-off check is a check
nobody runs". Under the withdrawn premise that was a cost worth paying for adopter autonomy. Under
a house standard it is simply a check nobody runs.

The commit-scope taxonomy is the one member of this group that survives. `audit.allowedScopes` is a
repository's own vocabulary of what its commits are about, which differs between repositories by
construction, and ADR-0051 already established it as the single home for that vocabulary.

## Decision

1. `decision: gates-always-run` The prose gate and the memory-citation gate always scan. Neither
   carries an enablement knob, neither exits early unscanned, and the aggregate repository check
   reports no disabled-child note because no child can be disabled. Their exemption lists survive
   untouched: an exemption names a place where a rule is knowingly not applied, which is a fact
   about the repository, not a preference about whether the rule exists.

2. `decision: payload-and-wrapper-always-render` The hook payloads and the `./awf` wrapper always
   render. The five payloads under `.awf/hooks/` and the repo-root wrapper are unconditional
   outputs. awf still never activates a hook: it writes no `.git/` path and runs no `git config`,
   so the payloads remain inert until wired by the repository, which is a separate matter from
   whether they exist.

3. `decision: audit-advisories-always-run` The five advisory audit rules (domain-doc staleness,
   domain-code staleness, undocumented domain, plain punctuation, uncommitted changes) always
   evaluate. A rule that is inert for structural reasons stays inert for those reasons: a domain
   rule with no configured domains still finds nothing. The audit remains advisory and never gates,
   which is unchanged and is what makes always-running cheap.

4. `decision: audit-thresholds-fixed` The accepted commit-type set, the subject-length limit, the
   plan diff threshold and the dependency-manifest glob set are fixed in the binary at the values
   that are the current defaults. `audit.allowedScopes` remains configured, because a commit-scope
   vocabulary is a repository fact rather than a tuning preference.

5. `decision: toggle-key-migration` A schema generation removes `hooks`, `runner`, the
   `proseGate.enabled` and `memoryCite.enabled` keys, the five advisory booleans, the commit-type
   list, the subject-length limit, the diff threshold, the dependency-manifest globs and
   `currentState.maxTopicsPerPath` from a config tree, announcing each removal it performs. A block
   emptied by the removal is dropped, while `audit` retains `allowedScopes`, `proseGate` and
   `memoryCite` retain their exemptions, and `currentState` retains its sources and test globs.

6. `decision: sweep-before-landing` Because both scanners block, a repository must be swept clean
   before this record lands in it, rather than discovering the violations at its next commit. For
   this repository both gates are already enabled and the tree is clean, so the obligation falls on
   any repository adopting the change later, as a setup step.

## State changes

- add `tooling/quality-gates:gates-always-run`
- add `tooling/audit-and-snapshots:audit-advisories-always-run`
- add `config/migrations-and-locks:toggle-keys-dropped`
- remove `rendering/companion-scripts:runner-singleton-toggle`
- remove `tooling/cli:check-disabled-child-disclosure`
- remove `tooling/init-and-enablement:init-hooks-default-on`
- update `rendering/singletons-and-payloads:hook-payloads-rendered`
- update `rendering/companion-scripts:runner-pure-forwarder`
- update `rendering/companion-scripts:hook-payloads-fallback-safe`
- update `rendering/project-output-plan:conditional-unit-single-source`
- update `config/validation:hooks-commands-resolvable`
- update `config/validation:glob-migration-anchored`
- update `config/migrations-and-locks:severity-keys-dropped`
- update `tooling/quality-gates:memory-citation-gate`
- update `tooling/quality-gates:prose-gate-refuses-without-git`
- update `tooling/audit-and-snapshots:audit-conventional-commits`
- update `tooling/audit-and-snapshots:audit-plain-punctuation`
- update `tooling/audit-and-snapshots:audit-uncommitted-changes`
- update `tooling/audit-and-snapshots:audit-domain-code-staleness`
- update `tooling/audit-and-snapshots:audit-plan-threshold-warn`
- update `tooling/cli:repo-check-capability-plan`
- update `adr-system/plan-artifacts:plan-commit-subject-shape-checked`
- update `adr-system/plan-artifacts:plan-commit-subject-length-checked`

## Consequences

Nine config keys and four singleton toggles disappear, and thirteen claims lose a conditional
clause. Reading what a repository checks becomes reading awf rather than reading awf intersected
with a repository's answers.

Two audit claims need no operation despite their toggles retiring: `audit-domain-doc-staleness` and
`audit-undocumented-domain` never mention a knob in their prose, so they stay true verbatim once
the knob is gone. `audit-dependency-warn` likewise survives untouched, because it states the rule's
behaviour without naming the glob set that feeds it, and that set is fixed rather than removed.

The conditional-render descriptor loses its last members. With bootstrap retired by the companion
record and hooks and the runner unconditional here, no config-tree render unit derives an
enablement any more. Whether the descriptor still earns its place or collapses into the ordinary
output plan is an implementation question this record leaves open rather than prejudging, because
the answer depends on what remains after both records land.

`hooks-commands-resolvable` simplifies asymmetrically. Its first arm, requiring `vars.gateCmd`
when hooks render, becomes unconditional and keeps binding. Its second arm, which required
`checkCmd` and `commitGateCmd` when hooks rendered without the runner, becomes unreachable, since
the runner always renders and the payloads can always use its `./awf` form. Both arms validate at
sync and check rather than at init, so a freshly scaffolded tree with unanswered vars is unaffected.

Landing this in a repository with an unswept tree fails its next commit. That is the cost ADR-0119
and ADR-0158 designed the default-off knobs to avoid, and it is now paid once per repository as a
setup step instead of avoided forever. This repository has already paid it.

Retiring the audit thresholds means a repository cannot loosen a commit-subject limit or a diff
threshold when it disagrees. Both are advisory; the audit warns and never gates, so disagreement
costs a warning rather than a blocked commit.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the two scanner knobs, retire only the audit booleans | The scanner knobs carry the strongest form of the withdrawn argument (a blocking check on an unswept tree), so they are the clearest case, not the exception. |
| Keep the knobs but default them true | Turns an opt-in into an opt-out while keeping the whole conditional surface, the disabled-child note and the migration seeding; the reasoning that a default-off check is a check nobody runs applies equally to a knob nobody sets. |
| Keep `audit.allowedTypes` as a repository fact alongside `allowedScopes` | Scopes are a repository's own vocabulary; the Conventional Commits type set is the specification's, and this repository runs it unmodified. |
| Keep `audit.dependencyManifests` configurable for non-Go repositories | The default glob set is already language-agnostic and broad; a repository whose manifests it misses is better served by widening the built-in set for everyone. |
| Have the migration refuse when the tree would fail the newly unconditional scanners | The migration edits configuration and cannot run a repository-wide content scan safely; the gate reports the violations precisely, which is the right tool. |
| Retire the exemption lists along with the enablement knobs | An exemption records a place where prose is genuinely about the character it contains; it is a repository fact, and removing it would make a true statement unwritable. |

## Status history

- 2026-08-06: Proposed
