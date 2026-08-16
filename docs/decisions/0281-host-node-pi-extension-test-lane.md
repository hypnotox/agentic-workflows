---
format: current-state-v4
slug: host-node-pi-extension-test-lane
status: Implementing
date: 2026-08-16
---
# ADR-0281: Host Node Pi extension test lane


## Context

The Pi-extension suite runs TypeScript checks, Node tests, fake-runtime fixtures, and coverage. It
does not test Docker behavior. ADR-0123 nevertheless placed the lane in Docker to preserve a
Go-first contributor environment with no host Node or npm requirement. ADR-0198 later made that
container ephemeral and content-keyed to eliminate persistent-container leaks, checkout-path image
multiplication, and unsafe full-repository copies.

That local-environment constraint no longer serves this repository. The repository now accepts
pinned host Node and npm as prerequisites for this lane, and NVM can give local execution the same
explicit Node version as CI. Docker now contributes an image build, cleanup surface, and
Docker-specific proof machinery without providing a test semantic that the suite needs.

Removing Docker must not weaken the useful guarantees it happened to provide. The dependency tree
must still derive from the lockfile rather than ambient packages, generated sources must still be
mutated only in an ephemeral copy, strict compilation and coverage must remain unchanged, and
concurrent gates in one checkout must not race over a shared dependency tree. The host lane also has
to remain consistent across macOS development and Linux CI.

## Decision

1. `decision: host-node-lane` Run the Pi-extension suite directly on the host with no Docker
   prerequisite. Host Node and npm are repository development prerequisites for this lane.
2. `decision: exact-lts-pin` Pin Node v24.19.0, the selected latest-LTS release, exactly in the
   repository. Local NVM selection and CI setup consume that single pin, so neither a moving LTS
   alias nor an independently maintained CI version can change the runtime implicitly. Local
   execution never downloads the runtime silently: when NVM lacks the pinned version, it fails with
   an actionable `nvm install v24.19.0` instruction.
3. `decision: lockfile-owned-dependencies` Resolve all Pi-test tools and packages from the harness's
   lockfile-installed dependency tree through `npm ci --ignore-scripts`. A reusable installation is
   valid only for its dependency inputs, exact Node and npm versions, operating system, and
   architecture; no global npm package participates and dependency preparation runs no package
   lifecycle scripts.
4. `decision: serialized-checkout-lane` Serialize the complete Pi lane per checkout. A concurrent
   invocation waits, and stale ownership can be recovered, so dependency replacement cannot race a
   test resolving from the same tree.
5. `decision: ephemeral-source-copy` Compile and test a narrow temporary copy of the required
   generated Pi and harness inputs. Strip the editor-only `// @ts-nocheck` directive portably in that
   copy after copying and before strict compilation; never mutate the checkout for test preparation.
6. `decision: retained-assurance` Retain strict TypeScript compilation, 100 percent statement,
   branch, function, and line coverage, staged-path lane selection, and deterministic local and CI
   execution.
7. `decision: retire-docker-cleanup` Retire the Pi lane's reset and Docker-object cleanup capability.
   A host lane creates no lane-owned Docker state for those operations to manage.

## State changes

- update `tooling/quality-gates:pi-extension-container-gate`
- update `rendering/pi-workflows:pi-extension-editor-quiet-strip`

## Consequences

Contributors running the full gate need the pinned Node release and npm, but no longer need Docker.
The Pi lane loses container filesystem and process isolation, which is acceptable because its
fixtures use temporary state, fake Pi processes, and no live model or credentials. A narrow temporary
workspace continues to protect generated sources from preparation mutations and excludes unrelated
operator-local Pi state.

The lockfile, exact runtime pin, platform-aware installation identity, and local tool resolution
preserve reproducibility without an image. CI uses its npm download cache but performs a clean
lockfile installation when no valid dependency tree exists. Local warm runs reuse the per-checkout
tree.

Holding the checkout lock for the complete lane reduces same-checkout concurrency. This is deliberate:
waiting is simpler and safer than creating isolated dependency trees or allowing `npm ci` to replace
packages beneath another run. Different worktrees retain independent locks and installations.

The Dockerfile, image fingerprinting, reset command, legacy-object reaper, and their dedicated tests
and documentation disappear. Historical ADRs remain accurate records of the constraints and defects
they addressed; this decision changes current authority forward rather than rewriting them.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the ephemeral Docker lane | Preserves a no-Node host, but retains infrastructure that no test semantic requires. |
| Run directly against ambient Node and packages | Smallest script, but makes runtime and dependency drift invisible. |
| Pin the moving NVM LTS alias | Automatically follows releases, but makes an unchanged checkout nondeterministic. |
| Install dependencies cleanly on every invocation | Avoids reusable-tree validation, but repeats a costly installation even when every dependency input is unchanged. |
| Give every concurrent run an isolated dependency tree | Preserves concurrency at the cost of repeated installs and broader cache ownership. |
| Fail immediately when the checkout is busy | Simpler than waiting, but makes ordinary overlapping gate activity unnecessarily brittle. |

## Status history

- 2026-08-16: Proposed
- 2026-08-16: Accepted; content-sha256: bc80ea8b4bb4189070a07586065748ae5beb62d7a6fa1334a376c7243698a22d
- 2026-08-16: Implementing; content-sha256: bc80ea8b4bb4189070a07586065748ae5beb62d7a6fa1334a376c7243698a22d
- 2026-08-16: Applied; operations: update `tooling/quality-gates:pi-extension-container-gate`
- 2026-08-16: Applied; operations: update `rendering/pi-workflows:pi-extension-editor-quiet-strip`
