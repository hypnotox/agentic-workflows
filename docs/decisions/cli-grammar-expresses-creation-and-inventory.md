---
format: current-state-v4
slug: cli-grammar-expresses-creation-and-inventory
status: Proposed
date: 2026-08-06
---
# ADR-cli-grammar-expresses-creation-and-inventory: CLI Grammar Expresses Creation And Inventory


## Context

awf's configuration commands were built around selection. `awf enable` and `awf disable` operate on
four kinds, skill, agent, doc and domain, each mapping to its plural array (ADR-0024, renamed from
`add`/`remove` by ADR-0093). `awf add`, `awf remove` and `awf list target` mutate and read the
targets array (ADR-0037). Enabling applies a requirement closure and pairs a reviewing skill with
its agent (ADR-0148), and disabling refuses while dependents remain (ADR-0081). `awf new skill`,
`awf new agent` and `awf new doc` scaffold project-local artifacts (ADR-0068, ADR-0091).

The house-standard record retires everything those commands select over. With the `skills`,
`agents`, `docs` and `targets` arrays and the sidecar `local` field gone, `awf enable` and
`awf disable` have three of their four kinds removed, the target commands have no array, and the
three local `new` kinds have no channel to scaffold into. Config parsing is strict, so these are
not merely obsolete: each writes a key or a field that the next load rejects.

The surviving fourth kind exposes a naming error the selection framing concealed. Domains are not
selected from a catalog. There is no pool of domains awf ships that a repository turns on; a
repository invents a domain key, and the operation scaffolds the domain's convention part
idempotently, which is creation. ADR-0093 renamed `add` to `enable` because "`add` reads as
'create,' colliding with `awf new`". For a domain, create is precisely what happens, so the
objection inverts: the collision ADR-0093 avoided is the right description.

`awf new` already houses the authored artifacts: `adr`, `plan` and `topic`, each scaffolding a file
from a template that the repository then writes into. A domain belongs in that family, and
`awf new topic <domain> "<Title>"` already sits beside it.

What remains once selection is gone is two things: creating artifacts a repository authors, and
reading what exists. `awf list` survives as pure inventory; it can no longer report a name as
enabled or available, because every catalog entry renders, though it still distinguishes an entry
whose sidecar tunes it.

## Decision

1. `decision: cli-creation-and-inventory` The configuration CLI expresses creation and inventory
   only. No command selects which catalog artifacts render, because no such selection exists. A
   command that edits configuration edits a repository fact.

2. `decision: retire-selection-commands` `awf enable` and `awf disable` are retired in full rather
   than narrowed to their one surviving kind, along with the `add`, `remove` and `list` target
   commands, the enablement requirement closure and its provenance plan, the reviewing-skill agent
   pairing, and the dependent-refusal guard. Each exists to maintain an array that no longer
   exists.

3. `decision: domain-lifecycle-under-new` A domain is created with `awf new domain <name>` and
   removed with `awf remove domain <name>`. Creation joins `adr`, `plan` and `topic` under
   `awf new`, matching what the operation already does: it writes the `domains` entry and scaffolds
   the domain's convention part without clobbering an existing one. Removal deletes the entry and
   is the sole surviving member of `awf remove`, retaining the synchronized cleanup that the
   disable path performed. `awf list domain` is unchanged.

4. `decision: new-scaffolds-authored-artifacts` `awf new` scaffolds only artifacts a repository
   authors: `adr`, `plan`, `topic` and `domain`. The `skill`, `agent` and `doc` kinds are retired
   with the project-local channel they scaffold into, so no `awf new` kind produces something awf
   would otherwise render.

5. `decision: list-is-inventory` `awf list` reports what exists without reporting whether it is
   selected. A catalog entry is present unconditionally; the distinction the listing still draws is
   whether a sidecar tunes an entry, which reflects authored configuration rather than membership.

6. `decision: cli-lands-with-selection-retirement` This record and the house-standard record land
   in the same release. Under strict parsing, a retired selection key and a command that writes it
   cannot coexist across a release boundary in either order without leaving a command that corrupts
   the tree it edits.

## State changes

- add `tooling/cli:cli-creation-and-inventory`
- add `tooling/cli:domain-lifecycle-commands`
- remove `tooling/cli:cli-config-kinds`
- remove `tooling/cli:target-cli`
- remove `tooling/init-and-enablement:add-applies-closure-plan`
- remove `tooling/init-and-enablement:add-skill-pairs-agent`
- remove `tooling/init-and-enablement:remove-agent-pairing-guard`
- remove `tooling/init-and-enablement:remove-refuses-dependents`
- update `tooling/init-and-enablement:new-seeds-scaffold-vars`

## Consequences

Six claims retire and the `tooling/init-and-enablement` topic loses its enablement half, leaving
init and `new`. Its title and description name `add` and `remove`, so both need rewriting; whether
the remainder still warrants its own topic is a shape question this record does not settle.

Two `config/configuration` claims survive precisely because the domain commands survive.
`config-mutation-roundtrip` (the yaml.Node round-trip that preserves comments and formatting) and
`remove-block-scoped` (removing a member affects only its own key's sequence) had the enable and
disable commands as their callers; `awf new domain` and `awf remove domain` keep both mechanisms
reachable and both claims backed. Retiring the domain commands too would have orphaned them, which
is a further reason the domain lifecycle belongs in the CLI rather than in hand-editing.

`new-seeds-scaffold-vars` narrows rather than retires. It seeds an empty vars entry for every
variable the scaffolded template source references; with the three local kinds gone it binds over
`adr`, `plan`, `topic` and `domain`, and implementation must confirm it still has a reachable case
rather than becoming vacuous.

A repository that scripted against `awf enable` or the target commands breaks outright. There is no
deprecation window, because a deprecation window is exactly the interval in which the command
writes a key the loader rejects. Failing loudly at the command is better than writing a tree that
fails at the next load.

`awf remove` exists to hold one kind. That is thin, and a noun-first `awf domain` group was
considered; the deciding factor is that creation genuinely belongs with the other authored
artifacts under `awf new`, and splitting creation from removal across two command shapes for
symmetry's sake would be worse than one thin verb.

Reading the CLI no longer requires knowing the enabled set. Every skill, agent and doc named in the
help or the listing is present, so the listing stops being a report about configuration and becomes
a report about awf.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Narrow `awf enable`/`awf disable` to the surviving domain kind | Keeps a selection verb for the one kind that is not selected; the naming error is the reason this record exists. |
| `awf domain new`/`remove`/`list` as a noun-first group | Defensible, since domain becomes the only configurable collection, but it invents a command shape awf has nowhere else and separates domain creation from the authored artifacts it resembles. |
| `awf add domain`/`awf remove domain`, reverting ADR-0093's verbs | ADR-0093's objection was that `add` reads as create and collides with `awf new`; for domains that reading is correct, so the honest response is to use `awf new`, not to revive a verb it rejected. |
| Keep `awf new skill|agent|doc` scaffolding into templates rather than the local channel | A repository does not author templates; a scaffolded template would be a fork of the catalog entry with no mechanism keeping it current. |
| Leave domains to hand-editing and retire all configuration commands | Orphans the config-mutation and block-scoped-removal claims and their backing, and makes the one genuinely creative configuration act the only one with no command. |
| Deprecate the selection commands for one release before removing them | Strict parsing means a deprecated command writes a key the next load rejects, so the deprecation window is precisely the broken interval. |

## Status history

- 2026-08-06: Proposed
