---
format: current-state-v4
slug: cli-grammar-expresses-creation-and-inventory
status: Proposed
date: 2026-08-06
---
# ADR-cli-grammar-expresses-creation-and-inventory: CLI Grammar Expresses Creation And Inventory


## Context

awf's configuration commands were built around selection. `awf enable` and `awf disable` accept
eight kinds: the three catalog-backed kinds skill, agent and doc, each mapping to its plural array;
the freeform domain kind; the adapter target kind; and the three nameless singleton kinds
bootstrap, hooks and runner, which write a block rather than an array entry (ADR-0024, ADR-0037,
ADR-0040, ADR-0048, ADR-0101; both verbs renamed from `add`/`remove` by ADR-0093). Enabling applies
a requirement closure and pairs a reviewing skill with its agent (ADR-0050, generalized by
ADR-0081), and disabling refuses while dependents remain (ADR-0081). `awf new skill`,
`awf new agent` and `awf new doc` scaffold project-local artifacts (ADR-0068, ADR-0091).

The three companion records retire everything those commands select over. The house-standard record
retires the `skills`, `agents`, `docs` and `targets` arrays and the sidecar `local` field; the
bootstrap record retires the `bootstrap` block; the gates record retires `hooks` and `runner`.
That leaves `awf enable` and `awf disable` with one of their eight kinds, the target commands with
no array, and the three local `new` kinds with no channel to scaffold into. Config parsing is
strict, so these are not merely obsolete: each writes a key or a field that the next load rejects.

The surviving kind exposes a naming error the selection framing concealed. Domains are not selected
from a catalog. There is no pool of domains awf ships that a repository turns on; a repository
invents a domain key, and the operation scaffolds the domain's convention part idempotently, which
is creation. ADR-0093 renamed `add` to `enable` because "`add` reads as 'create,' colliding with
`awf new`". For a domain, create is precisely what happens, so the objection inverts: the collision
ADR-0093 avoided is the right description.

`awf new` already houses the authored artifacts: `adr`, `plan` and `topic`, each scaffolding a file
from a template that the repository then writes into. A domain belongs in that family, and
`awf new topic <domain> "<Title>"` already sits beside it.

What remains once selection is gone is two things: creating artifacts a repository authors, and
reading what exists.

## Decision

1. `decision: cli-creation-and-inventory` The configuration CLI expresses creation and inventory
   only. No command selects which catalog artifacts render, because no such selection exists. A
   command that edits configuration edits a repository fact.

2. `decision: retire-selection-commands` `awf enable` and `awf disable` are retired in full rather
   than narrowed to their one surviving kind. That covers all eight kinds: the three catalog-backed
   arms, the target arm, the three nameless singleton arms for bootstrap, hooks and runner, and the
   domain arm, whose creation and removal item 3 relocates to `awf new` and `awf remove`. It takes
   with them the enablement requirement closure and its provenance plan, the reviewing-skill agent
   pairing, and the dependent-refusal guard. This record retires the command surface; the companion
   records retire the keys those arms wrote.

3. `decision: domain-lifecycle-under-new` A domain is created with `awf new domain <name>` and
   removed with `awf remove domain <name>`, which introduces `awf remove` as a new top-level verb
   carrying exactly one kind. Creation joins `adr`, `plan` and `topic` under `awf new`, matching
   what the operation already does: it validates the name through the config path-safety rule
   before writing, so the command never writes an entry the loader would reject, then writes the
   `domains` entry and scaffolds the domain's convention part without clobbering an existing one.
   Removal deletes the entry and re-renders, so the domain's rendered output is pruned, and reports
   a surviving sidecar or convention part as orphaned rather than deleting authored files.
   `awf list domain` is unchanged.

4. `decision: new-scaffolds-authored-artifacts` `awf new` scaffolds only artifacts a repository
   authors: `adr`, `plan`, `topic` and `domain`. The `skill`, `agent` and `doc` kinds are retired
   with the project-local channel they scaffold into, so no `awf new` kind produces something awf
   would otherwise render.

5. `decision: list-is-inventory` `awf list` reports what exists without reporting whether it is
   selected. The `enabled`, `available` and `local` states are retired because every catalog entry
   is present unconditionally and none is locally owned; a catalog entry is distinguished only by
   whether a sidecar tunes it, and a domain continues to list as configured. The target listing
   survives as a fixed inventory of `claude` and `pi` carrying no state token, and the bare listing
   drops the bootstrap, hooks and runner categories along with the keys behind them.

6. `decision: no-deprecation-window-for-a-retired-key` awf ships no deprecation window for a
   retired configuration key. Because parsing is strict, a command that writes a key the loader no
   longer accepts corrupts the tree it edits, so a deprecation window is precisely the interval in
   which the command is broken. A command and the key it writes retire together, in one release.

## State changes

- add `tooling/cli:cli-creation-and-inventory`
- add `tooling/cli:domain-lifecycle-commands`
- remove `tooling/cli:cli-config-kinds`
- remove `tooling/cli:target-cli`
- remove `tooling/init-and-enablement:add-applies-closure-plan`
- remove `tooling/init-and-enablement:add-skill-pairs-agent`
- remove `tooling/init-and-enablement:remove-agent-pairing-guard`
- remove `tooling/init-and-enablement:remove-refuses-dependents`
- remove `tooling/init-and-enablement:new-seeds-scaffold-vars`

## Consequences

Seven claims retire and the `tooling/init-and-enablement` topic loses its enablement half and its
only `new` claim, leaving it an init-only topic. Its title and description name `add` and `remove`,
so both need rewriting, and whether the remainder still warrants its own topic is a shape question
this record does not settle.

`new-seeds-scaffold-vars` retires rather than narrowing. Its only caller is the local-artifact
scaffold, and only on the non-doc branch, so `adr`, `plan`, `topic` and `domain` never reach it;
the domain scaffold writes a fixed string constant with no template source at all. The claim is
already inert today, because both local base templates are varless. Retiring it takes
`seedScaffoldVars` and `project.ScaffoldVarRefs` with it, which the dead-code gate requires, while
`config.SeedVarKey` survives on its migration callers. The same gate forces a larger cascade from
item 2 that is easy to miss: the enablement resolver's exported surface and the plan-document
presentation it feeds have no consumer outside the retired enable and disable arms, so they retire
with those arms rather than lingering as unreachable production code.

The two `config/configuration` mutation claims are not at risk either way, and backing and
reachability are separate questions here. `config-mutation-roundtrip` and `remove-block-scoped` are
both proven directly in the config package's own edit tests, which exercise the editor rather than
any caller, so no command retirement can unback them. What callers determine is reachability for
the dead-code gate, and five live callers survive in the frozen historical migrations the
house-standard record preserves, with the domain commands keeping the editor reachable from the
live CLI surface as well.

A repository can no longer author an artifact awf does not ship. Convention parts reshape a catalog
artifact; they cannot introduce one, and the local channel and its scaffolding commands are both
gone. Landing the artifact in the catalog, or forking, are the remaining options. Under a house
standard that is coherent, but it is a real loss rather than pure simplification.

A repository that scripted against `awf enable` or the target commands breaks outright. There is no
deprecation window, for the reason item 6 gives. Failing loudly at the command is better than
writing a tree that fails at the next load.

`awf remove` is introduced to hold one kind, which is thin. A noun-first `awf domain` group was
considered; the deciding factor is that creation genuinely belongs with the other authored
artifacts under `awf new`, and splitting creation from removal across two command shapes for
symmetry's sake would be worse than one thin verb.

The added `tooling/cli:cli-creation-and-inventory` claim states the CLI-grammar fact, that the
command set is creation plus inventory and no command selects render membership. The
config-surface form of the same rule stays with the house-standard record's
`no-artifact-selection-surface` claim, so the two do not restate one rule in two topics.
`domain-lifecycle-commands` carries both the grammar and the scaffold-and-removal behaviour,
because the topic that would otherwise own the behaviour half is reduced to init alone.

The listing's state vocabulary is deliberately left unclaimed. No current-state claim covers it
today, and item 5's commitment is proven by the listing tests rather than by a claim.

Item 6's rule likewise stays record-only. A rule that awf ships no deprecation window for a retired
key constrains how a future retirement is scheduled, not what any code path does, so there is
nothing a test could pin; it binds as recorded reasoning that a later retirement cites, in the same
way item 5's vocabulary is left to its tests.

Two implementation constraints follow that the plan must respect. The domain arms of `new` and
`remove` must dispatch through the exported kind accessors rather than comparing a literal kind
name, so the single-dispatch-table claim stays true. And this record must land in the same release
as the house-standard record, per item 6.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Narrow `awf enable`/`awf disable` to the surviving domain kind | Keeps a selection verb for the one kind that is not selected; the naming error is the reason this record exists. |
| `awf domain new`/`remove`/`list` as a noun-first group | Defensible, since domain becomes the only configurable collection, but it invents a command shape awf has nowhere else and separates domain creation from the authored artifacts it resembles. |
| `awf add domain`/`awf remove domain`, reverting ADR-0093's verbs | ADR-0093's objection was that `add` reads as create and collides with `awf new`; for domains that reading is correct, so the honest response is to use `awf new`, not to revive a verb it rejected. |
| Let `awf new topic` create an unconfigured domain implicitly, so domain creation needs no command | A mistyped domain name would silently create a domain, and it hides a configuration write inside a topic-scaffolding command. |
| Keep `awf new` scaffolding a skill, agent or doc into templates rather than the local channel | A repository does not author templates; a scaffolded template would be a fork of the catalog entry with no mechanism keeping it current. |
| Leave domains to hand-editing and retire all configuration commands | Makes the one genuinely creative configuration act the only one with no command, and drops the pre-write refusal, so a mistyped name surfaces as a failing load rather than an actionable command error. |
| Deprecate the selection commands for one release before removing them | Strict parsing means a deprecated command writes a key the next load rejects, so the deprecation window is precisely the broken interval. |

## Status history

- 2026-08-06: Proposed
