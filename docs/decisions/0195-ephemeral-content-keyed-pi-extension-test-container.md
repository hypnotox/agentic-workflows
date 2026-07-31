---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0195: Ephemeral content-keyed Pi extension test container

## Context

ADR-0123 established the Pi extension gate lane: `./x gate` runs the extension test suite
inside Docker through `tools/pi-extension-test/container.sh`, so a contributor needs no host
Node or npm. `TestPiRealRuntimeSmoke` in `internal/project/target_test.go` also shells out to
`./x pi-test run`, so plain `go test ./...` drives the same script. The script is
hand-maintained infrastructure outside the render and lock set, alongside `./x`,
`.github/workflows/`, and `.goreleaser.yaml`.

The lane keys every Docker object on the repository path. `repo_hash` is the first 12
characters of `sha256(git rev-parse --show-toplevel)`, and it names a long-lived container, a
`node_modules` volume, and a label. A managed effort worktree has its own toplevel path, so
every worktree gets its own set. Three defects follow, all measured on 2026-07-31 in a
checkout with eight live managed worktrees.

Objects accumulate without bound. Cleanup filters on the label of the root the caller is
standing in, so it can only ever reach the current path's container. A worktree that is
created, gated, and removed orphans its container, volume, and image permanently, because
nothing can match them again. The census found 49 containers with 31 of them running, 50
volumes, and 51 images. Inspecting each container's recorded `/source` bind mount showed 10
live source paths against 39 dead ones. The dead paths are dominated by short-lived review and
verification checkouts under the system temporary directory, so orphan production is
structural rather than accidental. The oldest containers had been running for two days and
some had been exited for eleven.

The dependency fingerprint is not a dependency fingerprint. `hash_files` pipes
`sha256sum "$tool_dir/Dockerfile" ...` into `sha256sum`, and `sha256sum` prints each file's
path beside its digest, so the resulting `dep_hash` varies with the absolute path of the
checkout. The primary checkout fingerprints to `5d3629f6f19e` and a managed worktree of the
same commit to `341fafc8a239`. No two paths ever share an image or a dependency volume, and
each one runs its own full `npm ci` build. That is why 51 images exist: by image size they
collapse to only four genuine dependency states, at roughly 570 MB, 550 MB, 548 MB, and 707 MB.

The per-run source copy races concurrent git activity. The prepare step runs
`cp -a /source/. /workspace/repo/` where `/source` is a read-only bind mount of the whole
repository root. When a concurrent git process creates and removes `.git/index.lock` during
the copy, `cp` sees the directory entry and then fails to stat it, failing the gate with
`cp: can't stat '/source/./.git/index.lock'`. This was observed once and passed on an
unchanged re-run. A contended checkout makes concurrent git activity normal.

The copy is also far larger than the suite needs. It moves 376 MB: 43 MB of `.git` plus an
untracked 206 MB host `node_modules` under `tools/pi-extension-test/` plus the rest of the
tree. What the suite compiles and runs is about 470 KB, because
`tools/pi-extension-test/tsconfig.json` includes exactly `tests/**/*.ts` and
`../../.pi/extensions/**/*.ts`, and the test command reads only those trees plus the fixtures.
Copying the host `node_modules` additionally breaks the isolation the lane exists to provide:
it lands nearer the test files than the repository-root symlink, so Node resolves the host
tree first and the suite does not necessarily run against the dependencies the image pinned.
This also explains a failure class the pitfalls entry currently attributes to the git race:
a module-resolution failure out of a nested `node_modules` is what a copied host tree
produces, and it has a different cause and a different fix from a partially copied checkout.

The constraint that shaped the choice is concurrency. Any cleanup that widens beyond the
current path can delete an object belonging to a gate running concurrently in another
worktree, which is a live risk in this repository rather than a theoretical one. A design that
has to be guarded against that risk is worse than a design in which the risk cannot arise.

A prototype of the decision below ran the full suite end to end: 63 of 63 tests passing at 100
percent lines, branches, functions, and statements, from a cold ephemeral container with no
volume, in 5.9 seconds against 9.2 seconds for the warm persistent container it replaces. The
ephemeral design is faster because the 376 MB copy costs more than the roughly 800 ms of
container startup it adds.

## Decision

1. The gate lane runs one `docker run --rm` per invocation. No container outlives the run that
   created it, and the script no longer creates, starts, stops, inspects, or names a container.
2. The dependency fingerprint hashes file contents rather than `sha256sum` output, so it is
   identical for every checkout of the same content. Its inputs are the Dockerfile,
   `package.json`, and `package-lock.json`.
3. The image is the only durable Docker object the lane creates. It is tagged solely by that
   content fingerprint and is therefore shared by every checkout, worktree, and clone on the
   machine.
4. Repository-path keying is removed entirely: the `repo_hash` value, the
   `dev.awf.pi-test.repo` label, and every object name derived from them.
5. The named `node_modules` volume is removed. The run resolves dependencies from the tree
   already baked into the image, reached by a symlink from the working copy, so the lane
   creates no volume at all.
6. `tools/pi-extension-test/docker-entrypoint.sh` is deleted along with the Dockerfile
   `ENTRYPOINT` that invoked it. Its only responsibilities were seeding the removed volume and
   idling the removed long-lived container.
7. The prepare step copies only the trees the suite compiles and runs, and never copies `.git`.
   It continues to strip the ts-nocheck directive after the source copy and before running the
   TypeScript compiler, preserving the ordering that
   `rendering/pi-workflows:pi-extension-editor-quiet-strip` describes.
8. No gate run deletes a shared Docker object. Only the explicit `reset` command removes
   anything, which is what makes the design safe against a gate running concurrently in another
   worktree without needing a lock, a lease, or a liveness predicate.
9. The `stop` subcommand is removed from the script and from `./x`, because an ephemeral run
   leaves nothing to stop. `reset` prunes the lane's images, and additionally removes
   containers and volumes left behind by the superseded path-keyed design.
10. The `contract` subcommand is removed. `./x` routes only `run`, `stop`, and `reset`, and
    nothing in the repository invokes `contract`, so it is unreachable today. This removal is
    incidental to rewriting the script rather than motivated by the defects above, and is
    recorded as a numbered commitment so that a deliberate capability removal is not mistaken
    for an accidental one.
11. The Dockerfile no longer installs git. No dependency resolves through a git protocol URL
    and nothing in the suite shells out to git.

## State changes

- update `tooling/quality-gates:pi-extension-container-gate`

## Consequences

The orphan class this decision targets cannot recur. Containers and volumes stop existing as
durable objects, so there is nothing to leak when a worktree is removed, and no reaper, liveness
predicate, or path-existence sweep has to be written or proven safe. The concurrency hazard is
removed by construction rather than mitigated.

Gate runs get faster rather than slower, by roughly a third on the measured suite, and the
lane stops rebuilding one 570 MB image per checkout path. A contributor with several worktrees
builds once.

The lane also becomes genuinely hermetic for the first time, because the host `node_modules`
that previously shadowed the image's pinned tree is no longer copied into the working copy.

Accepted trade-offs. Each run pays container startup, roughly 800 ms, that the warm container
avoided; the copy this replaces cost more, but a future change that shrinks the copy further
would not recover this component. Two concurrent cold gates can both build the same tag; Docker
tolerates this, both succeed, and the layer cache makes the second cheap, so no lock is
introduced. `reset` during a concurrent gate untags an image the running container still holds
layers for, so the in-flight run completes and the next one rebuilds.

Implementing this decision falsifies documentation that must travel in the same commit. The
command-runner reference at `.awf/docs/parts/development/command-runner.md` describes the
persistent container, the `stop` subcommand, and the repo-keyed container, volume, and image,
and must be rewritten for the ephemeral content-keyed lane and re-rendered. The pitfalls entry
covering the copy race records three failure classes that this decision removes: the partial
checkout produced by copying `.git` under concurrent git activity, the nested `node_modules`
module-resolution failure that the copied host tree actually caused, and the contention between
a concurrent gate and `TestPiRealRuntimeSmoke` over one shared persistent container. That entry
is retired or rewritten to whatever remains true, such as concurrent image builds.

Operationally, this decision creates a one-time cleanup of the objects the superseded design
left behind, and the `reset` command is extended to perform it. Removing the unreachable
`contract` subcommand is a deliberate capability removal recorded here rather than a silent one.

This decision changes the claim that ADR-0123 established, which is what brings it here rather
than into a routine refactor. Separately, the backing test for that claim was deleted by an
unrelated commit while its proof marker was left in place, so the claim has been declared
`Backing: test` while being proven by nothing; implementing this decision restores real backing.
The general problem, that a proof marker can outlive the test it was proving without the drift
check noticing, is out of scope here and is tracked on its own.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the persistent container and add a sweep that reaps containers whose recorded source path no longer exists | Cleans up after a problem this decision prevents, keeps more than 30 idle containers running, and requires proving a liveness predicate safe against concurrent gates rather than making the hazard impossible |
| Label objects by the primary control root instead of the worktree path | One cleanup would reach every object belonging to the repository, but it widens deletion across concurrently gating worktrees, which is the hazard to avoid |
| Fix only the fingerprint and the copy, and reap the existing orphans once by hand | Smallest change and no new machinery, but containers still orphan permanently on every worktree removal, so the original complaint returns |
| Keep copying the whole root and exclude `.git` with a tar-based copy | BusyBox `tar` does support exclusion, but it leaves the 206 MB host `node_modules` copy and the isolation defect in place; copying only what the suite needs addresses all three at once |

## Status history

- 2026-07-31: Proposed
