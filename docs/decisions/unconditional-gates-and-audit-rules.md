---
format: current-state-v4
slug: unconditional-gates-and-audit-rules
status: Implementing
date: 2026-08-06
---
# ADR-unconditional-gates-and-audit-rules: Unconditional Gates And Audit Rules


## Context

Four singletons, nine audit settings and the topic fan-out budget let a repository choose how much
of awf's checking applies to it. The hooks payloads render only when `hooks.enabled` is set
(ADR-0048); the `./awf` wrapper only when `runner.enabled` is set (ADR-0101, ADR-0156); the prose
gate scans only when `proseGate.enabled` is set (ADR-0119); the memory-citation gate only when
`memoryCite.enabled` is set (ADR-0158). `awf audit` carries five per-rule booleans (ADR-0019,
ADR-0025, ADR-0077, ADR-0117), plus a commit-type list, a subject-length limit, a diff threshold
and a dependency manifest glob set (ADR-0017). `currentState.maxTopicsPerPath` tunes the
path-scoped topic fan-out budget.

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

The measurement is suggestive rather than decisive, and it has to be read carefully. This checkout
sets `hooks.enabled`, `runner.enabled`, `proseGate.enabled` and `memoryCite.enabled` all true, and
carries none of the nine audit settings except `allowedScopes`, nor a non-default fan-out budget:
the five booleans, the type list, the subject limit, the diff threshold, the manifest globs and the
budget all run at their built-in values. But the served set is plural and heterogeneous, and a
census of one member of it is weak evidence. The companion record that proposed retiring the
bootstrap installer was withdrawn for exactly this reason: it read one repository's value as the
whole picture and missed that the key varies across the others.

What decides this record is therefore durability, not the census. Both scanners ship default-off,
so an unswept tree differs from this one today; that difference exists only until the tree is
swept, after which every served repository agrees. The owner has confirmed that none durably
declines a gate. A difference that resolves as adoption completes is transitional and does not
admit a key under the house-standard record's test. Bootstrap is the contrasting case and it is
kept for that contrast: a repository that builds awf from source will never converge with one that
installs a pinned binary.

ADR-0117 recorded the counter-pressure that now decides the matter: "a default-off check is a check
nobody runs". Under the withdrawn premise that was a cost worth paying for adopter autonomy. Under
a house standard it is simply a check nobody runs.

Severity is not uniform across this group, and that shapes what retiring each knob costs. Of the
five advisory rules, four emit warnings; uncommitted-changes emits an error, and the audit exits
non-zero on any error. Of the four tuning values, the diff threshold emits a warning, but the
subject-length limit emits an error and is evaluated by the same shared function that backs
`awf check staged commit`, so it blocks a commit rather than warning about one.

The commit-scope taxonomy is the one member of this group that survives. `audit.allowedScopes` is a
repository's own vocabulary of what its commits are about, which differs between repositories by
construction, and ADR-0051 already established it as the single home for that vocabulary.

## Decision

1. `decision: gates-always-run` The prose gate and the memory-citation gate always scan. Neither
   carries an enablement knob and neither exits early unscanned; that is the whole of the claim
   this item mints. The aggregate repository check's disabled-child disclosure has nothing left to
   disclose, and it retires by removing the CLI claim that states it, so the aggregate-presentation
   subject stays in its own topic. Their exemption lists survive
   untouched: an exemption names a place where a rule is knowingly not applied, which is a fact
   about the repository, not a preference about whether the rule exists.

2. `decision: payload-and-wrapper-always-render` The hook payloads and the `./awf` wrapper always
   render. The five payloads under `.awf/hooks/` and the repo-root wrapper are unconditional
   outputs. awf still never activates a hook: it writes no `.git/` path and runs no `git config`,
   so the payloads remain inert until wired by the repository, which is a separate matter from
   whether they exist.

3. `decision: audit-advisories-always-run` The five per-rule audit booleans are retired and all five
   rules always evaluate. Four of them (domain-doc staleness, domain-code staleness, undocumented
   domain, plain punctuation) emit warnings and cannot fail a run. Uncommitted-changes emits an
   error and therefore fails the audit, so making it unconditional is a real behaviour commitment
   rather than a free one; it is accepted because the rule defaults on today, so no repository that
   has not deliberately silenced it changes behaviour. A rule that is inert for structural reasons
   stays inert for those reasons: a domain rule with no configured domains still finds nothing.

4. `decision: audit-thresholds-fixed` Four values are fixed in the binary at the values this record
   is written against, named here rather than by reference so the record stays falsifiable as
   defaults evolve: the subject-length limit is 72; the plan diff threshold is 400; the accepted
   commit-type set is the eleven-member Conventional Commits set (build, chore, ci, docs, feat, fix,
   perf, refactor, revert, style, test); and the dependency-manifest glob set is the nineteen-member
   language-agnostic default set, too long to enumerate here and therefore pinned by claim rather
   than by prose. `audit.allowedScopes` remains configured, because a commit-scope vocabulary is a
   repository fact rather than a tuning preference.

5. `decision: fan-out-budget-fixed` The path-scoped topic fan-out budget is fixed in the binary at
   8. It is a tuning number rather than a repository fact, it has never been varied, and its finding
   is a warning, so fixing it neither blocks nor removes a signal. The value is pinned by a claim of
   its own, because today it is pinned only through the configspec table, which the key's removal
   takes with it.

6. `decision: conditional-units-narrow-to-bootstrap` The config-tree render-unit descriptor
   survives as the single declaration home for every config-tree output, and its enablement facet
   narrows to one live subject. The hook payloads and the runner wrapper stay members with
   unconditional enablement rather than moving to a second table, so path, template identity, render
   kind and fixed sections continue to be declared in one place and the runner keeps supplying the
   only non-nil section set. Bootstrap remains the descriptor's one conditional member, because
   `bootstrap.enabled` is a live repository fact rather than a behaviour preference. The claim
   governing the descriptor is rewritten rather than retired, and no config-tree output acquires a
   bespoke declaration path.

7. `decision: toggle-key-migration` A schema generation removes `hooks`, `runner`, the
   `proseGate.enabled` and `memoryCite.enabled` keys, the five advisory booleans, the commit-type
   list, the subject-length limit, the diff threshold, the dependency-manifest globs and
   `currentState.maxTopicsPerPath` from a config tree, announcing each removal it performs. A block
   emptied by the removal is dropped, while `audit` retains `allowedScopes`, `proseGate` and
   `memoryCite` retain their exemptions, and `currentState` retains its sources and test globs.
   Migration steps predating this generation stay frozen, continuing to operate on the tree shape
   of their own generation, including seeding a fan-out budget and anchoring manifest globs as
   those existed then; this generation removes the keys afterward.

8. `decision: toggle-keys-forward-ported` Every key this record retires is registered for
   unconditional stripping from historical config bytes before the strict decoder sees them,
   mirroring the mechanism the house-standard record establishes for its own keys. It carries its
   own claim rather than extending that record's, because a pending record's claim does not yet
   exist to update. This is required rather than incidental: this repository's committed
   configuration carries `hooks.enabled`, `runner.enabled`, `proseGate.enabled`,
   `memoryCite.enabled` and `currentState.maxTopicsPerPath`, so without the registration every
   `awf audit` and staged check over a range predating this record would fail on configuration it
   is only reading.

## State changes

- add `tooling/quality-gates:gates-always-run`
- add `tooling/audit-and-snapshots:audit-advisories-always-run`
- add `tooling/audit-and-snapshots:audit-thresholds-fixed`
- add `config/migrations-and-locks:toggle-keys-dropped`
- add `invariants/topics-and-markers:fan-out-budget-fixed`
- add `rendering/companion-scripts:runner-wrapper-rendered`
- remove `rendering/companion-scripts:runner-singleton-toggle`
- update `rendering/project-output-plan:conditional-unit-single-source`
- remove `tooling/cli:check-disabled-child-disclosure`
- remove `tooling/init-and-enablement:init-hooks-default-on`
- update `rendering/singletons-and-payloads:hook-payloads-rendered`
- update `rendering/companion-scripts:runner-pure-forwarder`
- update `rendering/companion-scripts:hook-payloads-fallback-safe`
- update `config/validation:hooks-commands-resolvable`
- add `config/migrations-and-locks:toggle-keys-forward-ported`
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

Fourteen config keys disappear, four of them the singleton enablement toggles, and fifteen claims
are rewritten: eleven lose an enablement clause, four lose a reference to a value the config no
longer supplies, and one narrows its member set. Reading what a repository checks becomes reading
awf rather than reading awf intersected with a repository's answers.

Three audit claims need no operation despite the configuration behind them retiring. `audit-domain-doc-staleness`
and `audit-undocumented-domain` never mention a knob in their prose, so they stay true verbatim
once the knob is gone. `audit-dependency-warn` likewise survives untouched, because it states the
rule's behaviour without naming the glob set that feeds it, and item 4 fixes that set rather than
removing it.

Two migration claims are correspondingly left alone. `severity-keys-dropped` (which seeds a fan-out
budget at its own generation) and `glob-migration-anchored` (which anchors manifest globs at its
own generation) both stay true verbatim under item 7's freeze, because each describes what its
generation did to the tree shape of its time.

The subject-length retirement is the sharpest cost in this record and is not advisory. The limit
emits an error, and the same shared function backs `awf check staged commit`, so a repository that
disagrees with 72 is blocked at commit rather than warned at audit; the plan-fence check fails for
the same reason. The diff threshold, by contrast, only warns. Fixing the limit is defensible
because 72 is the widely used convention and this repository runs it unmodified, but it is a real
loss of latitude rather than a free simplification.

Making uncommitted-changes unconditional makes an error-severity rule unconditional. Because it
defaults on, no repository that has not deliberately silenced it is affected, and this one has not.

`hooks-commands-resolvable` simplifies asymmetrically. Its first arm, requiring `vars.gateCmd`
when hooks render, becomes unconditional and keeps binding, which is not even a new obligation
since hooks default on today. Its second arm, which required `checkCmd` and `commitGateCmd` when
hooks rendered without the runner, becomes unreachable, since the runner always renders and the
payloads can always use its `./awf` form. Both arms validate at sync and check rather than at init,
so a freshly scaffolded tree with unanswered vars is unaffected.

Every value this record fixes gains a claim pinning it. Once a value leaves the config tree it also
leaves the configspec table, so nothing would otherwise hold it and a later silent change to 72, to
the type set, or to the fan-out budget would break no test. Given the subject limit's severity,
that gap is not cosmetic. The audit values are pinned together in the audit topic; the fan-out
budget is pinned in the topics-and-markers topic that owns it rather than folded in beside them.

Retiring the runner toggle would otherwise drop a fact worth keeping. The claim being removed
carries both the toggle behaviour and the assertion that a render emits exactly one wrapper at the
repo-root path. The toggle half retires; the render-existence half is re-minted, mirroring how the
hook payloads claim states that exactly five payloads render.

Landing this in a repository with an unswept tree fails its next commit. That is the cost ADR-0119
and ADR-0158 designed the default-off knobs to avoid, and it is now paid once per repository as a
setup step instead of avoided forever. This repository has already paid it. The sweep itself is a
rollout instruction and belongs to the implementing plan rather than to this record.

Two topic narratives need different treatment, and only one is stale outright.
`rendering/singletons-and-payloads` opens on "always-on and toggleable singleton outputs", which
stays accurate because bootstrap survives as the toggleable one; the hook payloads simply move to
the always-on side. `rendering/companion-scripts` opens on "when hooks are enabled", which becomes
false and needs rewriting outright. Narratives carry no claim id, so no operation covers either and
nothing flags the drift; the implementing plan handles both.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the two scanner knobs, retire only the audit booleans | The scanner knobs carry the strongest form of the withdrawn argument (a blocking check on an unswept tree), so they are the clearest case, not the exception. |
| Keep the knobs but default them true | Turns an opt-in into an opt-out while keeping the whole conditional surface, the disabled-child note and the migration seeding; the reasoning that a default-off check is a check nobody runs applies equally to a knob nobody sets. |
| Keep `audit.subjectMaxLength` configurable | It is the one retirement here that blocks rather than warns, so the case for keeping it is real; rejected because 72 is the widely used convention, this repository runs it unmodified, and a repository that wants a different limit is disagreeing with the standard rather than describing itself. |
| Keep `audit.diffThreshold` configurable | Advisory only, and a repository that wants a different threshold is tuning a nudge; 400 has never been varied here. |
| Keep `audit.allowedTypes` as a repository fact alongside `allowedScopes` | Scopes are a repository's own vocabulary; the Conventional Commits type set is the specification's, and this repository runs it unmodified. |
| Keep `audit.dependencyManifests` configurable for non-Go repositories | The default glob set is already language-agnostic and broad; a repository whose manifests it misses is better served by widening the built-in set for everyone. |
| Keep `currentState.maxTopicsPerPath` as a repository fact | A fan-out budget is a tuning number, not a description of the repository; it has never been varied and its finding is a warning. |
| Retire the conditional-render descriptor rather than narrowing it | It would be correct only if bootstrap retired too; with `bootstrap.enabled` surviving as a repository fact the descriptor keeps a real member, and removing it would force the bootstrap pair into a bespoke conditional path outside the single source. |
| Have the migration refuse when the tree would fail the newly unconditional scanners | The migration edits configuration and cannot run a repository-wide content scan safely; the gate reports the violations precisely, which is the right tool. |
| Retire the exemption lists along with the enablement knobs | An exemption records a place where prose is genuinely about the character it contains; it is a repository fact, and removing it would make a true statement unwritable. |

## Status history

- 2026-08-06: Proposed
- 2026-08-07: Implementing; content-sha256: 373cc8954c992435147434addf0bb09f318012b530caad7fc55fd13b8c66fcd0
- 2026-08-07: Applied; operations: add `tooling/quality-gates:gates-always-run`, add `tooling/audit-and-snapshots:audit-advisories-always-run`, add `tooling/audit-and-snapshots:audit-thresholds-fixed`, add `config/migrations-and-locks:toggle-keys-dropped`, add `invariants/topics-and-markers:fan-out-budget-fixed`, add `rendering/companion-scripts:runner-wrapper-rendered`, remove `rendering/companion-scripts:runner-singleton-toggle`, update `rendering/project-output-plan:conditional-unit-single-source`, remove `tooling/cli:check-disabled-child-disclosure`, remove `tooling/init-and-enablement:init-hooks-default-on`, update `rendering/singletons-and-payloads:hook-payloads-rendered`, update `rendering/companion-scripts:runner-pure-forwarder`, update `rendering/companion-scripts:hook-payloads-fallback-safe`, update `config/validation:hooks-commands-resolvable`, add `config/migrations-and-locks:toggle-keys-forward-ported`, update `tooling/quality-gates:memory-citation-gate`, update `tooling/quality-gates:prose-gate-refuses-without-git`, update `tooling/audit-and-snapshots:audit-conventional-commits`, update `tooling/audit-and-snapshots:audit-plain-punctuation`, update `tooling/audit-and-snapshots:audit-uncommitted-changes`, update `tooling/audit-and-snapshots:audit-domain-code-staleness`, update `tooling/audit-and-snapshots:audit-plan-threshold-warn`, update `tooling/cli:repo-check-capability-plan`, update `adr-system/plan-artifacts:plan-commit-subject-shape-checked`, update `adr-system/plan-artifacts:plan-commit-subject-length-checked`
