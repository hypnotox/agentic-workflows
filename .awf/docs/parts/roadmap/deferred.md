## Pi and shared Agent Skills discovery

Resolve Pi's collision between its `.pi/skills/` output and the shared
`.agents/skills/` workflow skills that Pi also discovers when Codex is enabled.
Keep Pi's top-level reviewer skills available without duplicate workflow skill
names. ADR-0122's Pi and Codex target layouts may need a successor decision.
## Mechanically detecting a nominal invariant proof

`invariant-proof-exercises-its-claim` has now failed to prevent three sessions
of partially-backed proof markers, the last shipping roughly nine at once and
hiding a real defect behind a green gate. It has been strengthened from a
judgement item to an enumerating one, but that is still rung 3: probabilistic,
and applied only when a reviewer runs.

The rung-2 candidate is mutation testing, which this repo already has tooling
for (`cmd/mutants`, and the deterministic gremlins recipe in `docs/testing.md`).
A mutation run scoped to the check and derivation paths would kill a nominal
proof by construction: mutate the clause, and a marker whose test never
exercised it stays green. What is unresolved is the cost - a full run is slow
and advisory-only today - and whether a scoped, gate-wired subset can be made
fast and deterministic enough to block a commit. Worth an ADR if it can, since
a proof marker that cannot fail is worse than no marker at all.
