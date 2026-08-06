---
format: current-state-v4
slug: retire-the-bootstrap-installer
status: Proposed
date: 2026-08-06
---
# ADR-retire-the-bootstrap-installer: Retire The Bootstrap Installer


## Context

awf renders a self-pinning installer, `.awf/bootstrap.sh`, and a porcelain that drives it,
`.awf/upgrade.sh` (ADR-0040, relocated by ADR-0047). The installer pins the exact version of the
awf binary that rendered it, verifies a SHA-256 checksum of the download, probes for a binary
already on PATH before fetching, and writes only the resolved path to standard output. The
porcelain obtains a binary solely by invoking the installer and hands off with `exec`. Together
they let a repository acquire the right awf without the person cloning it having awf, or knowing
which version the repository expects.

That capability answers one question: how does a stranger, or a machine acting for one, get a
correct awf for a repository they did not set up. ADR-0040's own alternatives table says so
directly, rejecting an always-on singleton because "an adopter with their own installer should be
able to disable it through the normal CLI grammar", and ADR-0047 relocated the script out of the
repository root because a "default-enabled, self-downloading shell script" landing there was "the
most alarming possible placement for the least-explained file awf emits". Both are arguments about
what a stranger encounters.

The house-standard record withdraws the premise those arguments rest on. awf serves repositories
its owner controls, where awf is built from source or installed on PATH deliberately, not fetched
by a script that a checkout carries for the benefit of someone who has never seen it.

This checkout is the sharpest case. It sets `bootstrap.enabled: false` and has no
`.awf/bootstrap.sh`, because a self-pinning checksum-verified downloader for a released awf, living
inside the repository that produces awf, is circular: the pin would name a release built from the
very tree that carries it, and it would have to be re-pinned on every version bump. Its
`awfInvokeCmd` is `go run ./cmd/awf`.

Retiring the toggle rather than the feature would force exactly that circularity here, since the
house-standard record fixes behaviour preferences in the binary rather than leaving them
configurable. The alternative that avoids both is to retire the capability: the rendered `./awf`
wrapper already resolves `awfInvokeCmd` and falls back to a PATH `awf`, which is the whole
resolution story a repository whose owner installs awf deliberately needs.

## Decision

1. `decision: no-bootstrap-capability` awf renders no installer and no installer porcelain. Nothing
   awf emits downloads a binary, verifies a checksum, or pins a version for retrieval. Acquiring
   awf is outside what awf renders, and a repository obtains it by building from source or by
   installing it on PATH.

2. `decision: retire-bootstrap-outputs` The `bootstrap` config block, both rendered outputs
   (`.awf/bootstrap.sh` and `.awf/upgrade.sh`), and their templates are retired. No rendered output
   path is either file, no bootstrap or upgrade template remains in the embedded template set, and
   the residue-scan exemption list no longer names either template. A tree still carrying
   `bootstrap` is rejected by strict parsing rather than honoured, on the same footing as the
   house-standard record's retired keys, with the schema generation in item 4 as the repair path;
   that rejection carries its own claim rather than resting on the removal of a struct field.

3. `decision: wrapper-resolution-narrows` The rendered `./awf` wrapper resolves the configured
   `awfInvokeCmd` and otherwise a PATH `awf`. The pinned-binary-first arm, which existed only to
   prefer whatever the installer had fetched, is retired with the installer.

4. `decision: bootstrap-key-migration` A schema generation removes the `bootstrap` block from a
   config tree, announcing the removal, leaving every surviving key, value, comment and key order
   byte-intact, and dropping the emptied block. The generation-5 step that writes
   `bootstrap.enabled` stays frozen, continuing to operate on the tree shape of its own generation
   under the house-standard record's freeze rule; a single upgrade run may therefore announce
   adding the key and then removing it. A previously rendered installer is deleted by the ordinary
   lock prune on the next sync, like any other retired rendered output, rather than by the
   migration.

5. `decision: bootstrap-key-forward-ported` The `bootstrap` key is registered for unconditional
   stripping from historical config bytes before the strict decoder sees them, mirroring the
   mechanism the house-standard record establishes for its own keys. This checkout's committed
   configuration carries `bootstrap`, so without the registration every audit or staged check over
   a range predating this record would fail on configuration it is only reading.

## State changes

- add `rendering/singletons-and-payloads:no-bootstrap-outputs`
- add `rendering/templates:no-bootstrap-templates`
- add `rendering/companion-scripts:runner-resolution-invoke-then-path`
- add `rendering/sync-and-drift:residue-exemptions-pinned-one`
- add `config/migrations-and-locks:bootstrap-key-dropped`
- add `config/migrations-and-locks:bootstrap-key-forward-ported`
- add `config/configuration:bootstrap-key-rejected`
- remove `rendering/singletons-and-payloads:bootstrap-config-tree-path`
- remove `rendering/singletons-and-payloads:bootstrap-two-files`
- remove `rendering/companion-scripts:bootstrap-checksum`
- remove `rendering/companion-scripts:bootstrap-env-override`
- remove `rendering/companion-scripts:bootstrap-local-first`
- remove `rendering/companion-scripts:bootstrap-stdout-path-only`
- remove `rendering/companion-scripts:runner-resolution-pinned-first`
- remove `rendering/companion-scripts:upgrade-delegates-fetch`
- remove `rendering/companion-scripts:upgrade-exec-final`
- remove `rendering/sync-and-drift:residue-exemptions-pinned-three`

## Consequences

Ten claims retire and `rendering/companion-scripts` loses half its content, leaving the runner
wrapper and the hook payloads. Both that topic and `rendering/singletons-and-payloads` name
bootstrap in their summary and intro, so both narratives need rewriting; whether the remaining
companion-scripts claims justify a topic of their own or belong with the payloads is a shape
question this record does not settle.

Two claim ids name what this record retires, so both go through a remove plus an add rather than an
update: `residue-exemptions-pinned-three` names a count that becomes one, and
`runner-resolution-pinned-first` names a resolution arm that ceases to exist. Minting the negative
output claims is deliberate rather than redundant, because an absence that nothing asserts is
invisible to the closed-tree and residue checks; after this record no surviving claim mentions
either file by name.

A previously rendered installer is deleted on the next sync. That is the ordinary prune path for
any output that leaves the plan, and it is the right outcome: a retained file would fail
`awf check` as orphaned drift, since an unclaimed path under `.awf/` is a failing finding rather
than a tolerated leftover. The file is git-tracked in the repositories served, so the deletion is
recoverable. What this record rules out is the migration deleting it, which would put a shell-script
removal in a config editor.

Shipped prose carrying installer claims becomes false across three different authoring channels,
and the distinction matters because each is a different transaction shape. Two surfaces are
template-sourced: the working-with-awf upgrade procedure and the workflow doc's CI acquisition
recipe. Three are project-content-sourced, in exactly the channel the house-standard record
preserves: the glossary's upgrade-porcelain vocabulary entry and the pitfalls entry, both sidecar
data, and the releasing doc's tamper posture, a convention part, whose integrity story is half the
bootstrap's checksum verification (ADR-0079) and which reduces to the published checksums file
alone. One is descriptor-sourced: the `awfInvokeCmd` catalog var description still says the empty
value resolves the bootstrap-pinned binary, and the configspec derivation requires it to match
verbatim, which also carries the config reference's own rows for `bootstrap.enabled` and for the
runner's bootstrap-resolved fallback. The file-by-file edits belong to the plan.

The `enable`, `disable` and `list` arms for the nameless `bootstrap` singleton kind read the config
field this record removes, so they must change in the same compilation unit. The CLI-grammar record
owns that grammar and retires those arms; the two records must land in the same release.

Nothing changes for this repository operationally, because it already declines bootstrap and
invokes awf from source. The change is felt only by a repository that had the installer rendered:
its `.awf/upgrade.sh` stops being the way to upgrade, and `awf upgrade` (the command, which is
unrelated and survives untouched) remains the migration entry point.

A repository whose only path to a correct awf was the pinned installer now needs its owner to
install one. That is the accepted cost, and it is small precisely because the set of served
repositories is the set its owner sets up. It would be a serious cost under the published-standard
premise, which is why ADR-0040 made the opposite call at the time.

The version pin disappears as an artifact. A repository no longer carries a machine-readable record
of which awf version rendered it in the installer; the lock's schema generation remains the
authoritative compatibility signal, and the binary-version gate continues to refuse a stale binary
for gated commands, so nothing that enforced correctness rested on the pin.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Hardwire `bootstrap` always-on, as the house-standard record does for other preferences | Renders a self-pinning downloader for a released awf inside the repository that produces awf, re-pinned on every version bump. |
| Keep `bootstrap.enabled` as the one surviving behaviour toggle | It is a preference about awf's own behaviour, exactly what the house-standard record's admission test excludes; keeping it re-opens the category for one case. |
| Keep the installer but drop the `upgrade.sh` porcelain | Leaves the whole download-and-verify surface for the sake of a script whose only caller is the porcelain being removed. |
| Retire the rendered artifacts but keep the capability elsewhere, as a documented install snippet or an `awf bootstrap` subcommand | Under the house-standard premise the acquisition audience is the owner, for whom building from source and a PATH binary already answer it; a second documented mechanism buys nothing. |
| Retire the outputs but keep the templates embedded for later | An embedded template no output plan references is residue the closed-tree checks exist to catch. |
| Exempt a previously rendered installer from prune, or back it up like the co-owned runner | The runner exemption exists because an adopter hand-authored content inside it; nobody edits a generated installer, and an exempted file would then fail the orphaned-drift check instead. |

## Status history

- 2026-08-06: Proposed
