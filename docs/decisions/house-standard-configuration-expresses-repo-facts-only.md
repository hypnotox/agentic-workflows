---
format: current-state-v4
slug: house-standard-configuration-expresses-repo-facts-only
status: Proposed
date: 2026-08-06
---
# ADR-house-standard-configuration-expresses-repo-facts-only: House Standard: Configuration Expresses Repo Facts Only


## Context

awf's configuration tree carries roughly thirty-two documented keys. They do not form one
category. Twelve express a fact that genuinely differs between repositories: the skill-name prefix,
the integration branch, the command vars, the domain keys, the tag vocabulary, the commit-scope
taxonomy, the context-ignore globs, the current-state marker sources, the proof-eligible test
globs, the commit policy, and the two gate exemption lists. Nearly all hold a non-default value in
this checkout, because a repository that did not set them would be describing itself wrongly; the
exceptions are the declined `gateCmdFull` var and the empty `memoryCite.exemptions` list, and a
declined or empty value is itself a statement about the repository.

The rest express a preference about awf's own behaviour: which catalog skills, agents and docs
render, which adapter runtimes are targeted, where documentation lives, and whether an artifact is
hand-maintained instead of rendered. Measured against this checkout, that surface expresses almost
nothing. Of twenty-one catalog skills, twenty are enabled; of six agents, six; of eight toggleable
docs, seven. The enable arrays distinguish two artifacts in total.

The machinery bought with those two bits is substantial and interlocking: requirement closure over
a cyclic skill graph (ADR-0081), the dead-reference gate (ADR-0020, ADR-0046), `requiresDoc` render
suppression (ADR-0013), the toggleable-versus-mandatory doc partition (ADR-0061), the curated
`core` init default (ADR-0022), the `enable`/`disable` commands (ADR-0024, ADR-0093), and the
project-local artifact channel (ADR-0068, ADR-0091). Each was individually well-reasoned. Each
rested on the same premise.

That premise was that awf is a published standard whose adopters are strangers, so it must not
impose this repository's process on projects that branch differently, structure documentation
differently, or ship their own installer. The premise is stated most plainly in ADR-0073 ("awf
audit is part of the shipped awf standard: every adopter runs it ... defaulting a changelog rule
into awf audit would impose this repo's release process") and recurs in ADR-0017, ADR-0022,
ADR-0040, ADR-0048, ADR-0103 and ADR-0156.

The project owner has withdrawn that premise. awf is a house standard, used only in repositories
the owner controls; anyone who disagrees is expected to fork. With the premise gone, the
optionality it justified is unbacked: it protects hypothetical adopters who are not served, at the
cost of machinery every real change must reason about.

ADR-0084 already reached this conclusion for one key family, ruling that a catalog var descriptor
exists only for a functional value and that a knob whose only effect is prose wording never gets
one, on the finding that "the knob has never expressed anything". ADR-0183 reached it for finding
severity, on the finding that a configurable rank "preserves a config surface with no demonstrated
use". Neither generalized. This record generalizes the rule to the whole tree and applies it to
artifact selection; three companion records apply it to the bootstrap installer, to the gate and
audit toggles, and to the CLI grammar.

The shaping surface is deliberately untouched. Convention parts (272 of them here), `sidecar.data`,
`sidecar.dataDefaults` and `sidecar.sections.<name>.drop` carry per-repository content, and content
differs between repositories by construction. ADR-0084 routes prose customization there precisely
so it does not become config keys, and ADR-0236 added `dataDefaults` so that rejecting a standard
default stays visible in configuration rather than happening silently. Collapsing that surface
would push the same pressure back into the keys this record retires.

## Decision

1. `decision: repo-facts-only` A configuration key exists only to express a fact that differs
   between the repositories awf serves. A key that expresses a preference about awf's own behaviour
   is not a key, because awf has one behaviour. The admission test for a proposed key is empirical
   rather than counterfactual: whether the served repositories actually hold different values. A
   key nobody varies is fixed in the binary, however plausibly a hypothetical repository might have
   varied it. This subsumes ADR-0084's var-descriptor policy and extends it to the whole config
   tree, to sidecar fields, and to init answers. Reintroducing a behaviour preference as a key
   requires a successor record.

2. `decision: full-catalog-render` Every catalog skill, agent and doc renders for every target on
   every sync. The render set is the whole catalog. No config-derived selection, no requirement
   closure, no `requiresDoc` gate and no per-artifact suppression stands between the catalog and
   the output plan, so adding a catalog entry changes every served repository's output. The
   catalog's mandatory flag did double duty, partitioning the toggleable doc pool and defining the
   singleton set; only the first role is retired. The surviving distinction is structural: a
   singleton is a catalog document whose output path is fixed by awf rather than derived from a
   configurable name, and that is the predicate the singleton claims key off once the pool is gone.

3. `decision: retire-selection-keys` The `skills`, `agents`, `docs`, `targets` and `docsDir` keys
   and the sidecar `local` field are retired. A tree carrying any of them is rejected by strict
   parsing rather than honoured, on the same footing as the severity keys ADR-0183 retired. The
   existing strict-decoder rule that rejects a `data`, `sections` or `local` key at the root of
   `config.yaml` survives this retirement and is carried forward rather than lapsing with the claim
   that currently states it.

4. `decision: fixed-targets-and-docs-root` The rendered target set is exactly `claude` and `pi`, and
   the documentation root is exactly `docs/`, both fixed in the binary. Descriptor-driven rendering
   stays generic rather than branching on the two target names, and the decisions directory, index,
   plans directory, domains directory and every doc output continue to derive structurally from the
   single documentation root rather than being independently configurable.

5. `decision: retire-local-artifacts` The project-local artifact channel is retired. Every skill,
   agent and doc awf knows about is rendered from its template; none is hand-maintained at the
   conventional output path. A repository needing different content authors a convention part,
   which is the surviving customization path. The `rendering/local-artifacts` topic holds nothing
   but claims about this channel, so the topic itself retires with its metadata and convention
   part, and the paths it selected remain covered by the surviving rendering topics.

6. `decision: selection-key-migration` A schema generation strips the retired keys from a config
   tree and the `local` field from every sidecar, announcing each removal, leaving every surviving
   key, value, comment and key order byte-intact, and dropping a block the removal empties. The
   sidecar step preflights every match before any write and refuses with an actionable repair where
   a `local: true` sidecar means an artifact the repository owns by hand would now be overwritten.
   Migration steps predating this generation stay frozen: they continue to operate on the tree
   shape of their own generation, including the enable arrays and the documentation root as those
   existed then, and this generation removes the keys afterward.

7. `decision: historical-config-forward-ported` A retired key still parses on the path that reads a
   historical commit without a lock, where no migration chain runs to strip it. A commit carrying a
   lock is already forward-ported through the schema chain and needs nothing further. Without this,
   a staged check or an audit over a range predating this record fails on configuration it is only
   reading, so retiring a key from the live schema is not retiring it from history.

## State changes

- add `config/configuration:config-expresses-repo-facts-only`
- add `config/configuration:no-artifact-selection-surface`
- add `rendering/project-output-plan:full-catalog-render`
- add `rendering/doc-outputs:docs-root-fixed`
- add `rendering/doc-outputs:layout-docs-full-catalog`
- add `config/migrations-and-locks:selection-keys-dropped`
- add `config/migrations-and-locks:sidecar-local-field-dropped`
- add `config/migrations-and-locks:retired-keys-forward-ported`
- remove `config/configuration:enable-arrays`
- remove `config/configuration:docsdir-default`
- remove `config/configuration:targets-default-claude`
- remove `config/validation:duplicate-target-rejected`
- remove `config/validation:local-doc-name-path-validated`
- remove `config/validation:local-name-validated`
- remove `rendering/catalog-and-targets:enabled-set-closed`
- remove `rendering/catalog-and-targets:mandatory-doc-pool-exclusion`
- remove `rendering/project-output-plan:catalog-trim-applied`
- remove `rendering/project-output-plan:scaffold-core-only`
- remove `rendering/project-output-plan:reviewing-skill-agent-pairing`
- remove `rendering/project-output-plan:skills-context-effective-set`
- remove `rendering/project-output-plan:curated-init-skill-refs-clean`
- remove `rendering/local-artifacts:local-catalog-clone`
- remove `rendering/local-artifacts:local-doc-catalog-clone`
- remove `rendering/local-artifacts:local-doc-map-fields`
- remove `rendering/local-artifacts:local-doc-no-shadow`
- remove `rendering/local-artifacts:local-doc-renders-from-base`
- remove `rendering/local-artifacts:local-doc-requires-declaration`
- remove `rendering/local-artifacts:local-frontmatter`
- remove `rendering/local-artifacts:local-no-shadow`
- remove `rendering/local-artifacts:local-renders-from-base`
- remove `rendering/local-artifacts:local-requires-declaration`
- remove `rendering/doc-outputs:skill-ref-dead-fails`
- remove `rendering/doc-outputs:layout-docs-enabled-only`
- remove `rendering/sync-and-drift:skills-set-in-confighash`
- remove `tooling/init-and-enablement:init-set-closed`
- update `config/configuration:config-serialization-owned`
- update `config/configuration:sidecar-data-defaults-control`
- update `config/configspec-and-reference:config-reference-data-rejected`
- update `config/configspec-and-reference:config-reference-regen-drift`
- update `config/migrations-and-locks:upgrade-migrates-retirements`
- update `config/migrations-and-locks:upgrade-migrates-supersession-keys`
- update `rendering/catalog-and-targets:built-in-runtime-targets`
- update `rendering/catalog-and-targets:target-dialect-render`
- update `rendering/catalog-and-targets:unified-doc-model`
- update `rendering/catalog-and-targets:requires-skills-exact`
- update `rendering/catalog-and-targets:var-descriptor-set-pinned`
- update `rendering/project-output-plan:output-plan-complete`
- update `rendering/project-output-plan:multi-target-render`
- update `rendering/project-output-plan:kind-dispatch-single-table`
- update `rendering/project-output-plan:inert-sidecar-field-rejected`
- update `rendering/project-output-plan:scaffold-seeds-all-vars`
- update `rendering/doc-outputs:domains-dir-given`
- update `rendering/doc-outputs:layout-derivation`
- update `rendering/doc-outputs:skill-ref-unknown-ignored`
- update `rendering/doc-outputs:stub-notes-path-keyed`
- update `rendering/doc-outputs:working-with-awf-mandatory`
- update `rendering/singletons-and-payloads:adr-system-singletons-rendered`
- update `rendering/singletons-and-payloads:plain-singleton-via-renderkind`
- update `rendering/singletons-and-payloads:singleton-kinds-complete`
- update `rendering/adapter-outputs:generated-adapter-runtime-ownership`
- update `rendering/sync-and-drift:managed-output-attribution`
- update `rendering/sync-and-drift:drift-source-set`
- update `rendering/sync-and-drift:agent-guide-size-advisory`
- update `rendering/sync-and-drift:target-prune-ancestors`
- update `rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`
- update `rendering/guide-and-doc-templates:docs-section-parity`
- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- update `rendering/workflow-skill-templates:workflow-transitions-advisory`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-workflows:pi-native-workflow-skills`
- update `rendering/pi-workflows:using-effort-skill`
- update `rendering/pi-workflows:pi-effort-memory-tools`
- update `rendering/pi-workflows:pi-effort-session-association`
- update `tooling/init-and-enablement:init-prompts-enabled-vars`
- update `tooling/evaluations:evals-full-catalog-coverage`

## Consequences

The config tree loses five keys and one sidecar field, and the render path loses the entire notion
of a selected subset. Reasoning about what a repository renders becomes reading the catalog rather
than intersecting the catalog with three arrays, a requirement closure, a doc gate and a local
flag. The closed-config-tree consumption check (ADR-0086) does the demolition work: every retired
key becomes authored-but-unconsumed and fails loudly rather than lingering.

The `config-expresses-repo-facts-only` claim is a governing rule with no mechanically checkable
content, so it lands as a `rule:` carrying a `Verify:` note rather than a test-backed invariant.
Every other added claim here is testable and takes ordinary test backing.

Two artifacts that were off in this checkout become live: the `roadmap-graduation` skill, which was
the catalog's only `requiresDoc`-gated entry, and the `debugging` doc. Neither is referenced by any
current-state claim, and the golden-task eval fixture already renders both, so this is a content
question rather than a correctness risk. Each lacks a convention-parts directory and therefore
raises a non-failing stub advisory until one is authored.

Every served repository now renders both the `claude` and `pi` adapter layouts and roots its
documentation at `docs/`. A repository that wanted otherwise has no configuration answer; it forks.
That is the accepted cost of the withdrawn premise, and it is what makes the rest of this record
coherent.

Retiring the local channel means a repository can no longer take ownership of a rendered artifact
wholesale. The convention part remains, and it is finer-grained: a section body rather than a whole
file. A repository that genuinely needs a different artifact shape must change the template, which
is the correct place for a change that a house standard should make once.

This record and the CLI-grammar record must land in the same release. Config parsing is strict, so
from the moment the selection keys are retired, `awf enable`, `awf disable` and the target commands
write a key the next load rejects, and `awf new skill|agent|doc` scaffolds a sidecar field that no
longer exists. Landing this record alone would leave those commands actively breaking the tree they
edit rather than merely obsolete.

Residual wording is deliberately left behind. Several claims this record does not operate on still
phrase themselves around "an enabled target" or a selectable skill; each stays substantively true
because every target and every skill is now always present, so they are left to a later sweep
rather than inflating this record's operation set.

Migration carries real risk in exactly one place: a repository with a `local: true` sidecar has a
hand-maintained file that would silently become awf-rendered. The migration refuses rather than
overwrites, which converts a data-loss failure into a repair instruction. This checkout uses the
flag nowhere, so the path is exercised by test rather than by this repository.

The four `tooling/init-and-enablement` claims describing `enable` and `disable` behaviour
(`add-applies-closure-plan`, `add-skill-pairs-agent`, `remove-refuses-dependents`,
`remove-agent-pairing-guard`) are retired by the CLI-grammar record, which is where those commands
are removed, rather than here.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the enable arrays, retire only the machinery around them | The arrays are the machinery's reason to exist; keeping them keeps the closure, the gate, the partition and the commands. |
| Keep the arrays but stop shipping a curated default, so init enables everything | Leaves a selection surface nobody uses and the full cost of supporting it, while changing only the starting point. |
| Keep the `docs` array alone, as the one plausibly varying selector | It is the closest call, since seven of eight are on and doc needs do differ by repository shape, but one surviving array keeps the whole toggleable pool, the mandatory partition and the enable commands, so it buys none of the simplification. |
| Retire selection but keep `docsDir` and `targets` as repo facts | Neither differs across the repositories served; under item 1's empirical test, a key nobody varies is fixed in the binary. |
| Keep `local: true` as the escape hatch for artifacts a repository owns | It is the wholesale-ownership channel that convention parts replace more precisely, and it is used nowhere; ADR-0021, ADR-0059, ADR-0068 and ADR-0091 all justified it by adopter autonomy, the withdrawn premise. |
| Also collapse convention parts and sidecar shaping into fixed templates | The shaping surface is the exercised one and carries per-repository content, not behaviour preference; collapsing it would push that content back into new config keys. |
| Split the doctrine into its own record with no claim operations | A rule that changes nothing on its own is weaker than one applied in the same record; the three companion records cite this one rather than a rule stated in isolation. |
| One record for the whole config-surface collapse | Roughly a hundred and eight claim operations in one State changes section is not reviewably a single transaction. |

## Status history

- 2026-08-06: Proposed
