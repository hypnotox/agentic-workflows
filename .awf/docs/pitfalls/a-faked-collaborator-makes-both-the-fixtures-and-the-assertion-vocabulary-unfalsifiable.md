---
title: "A faked collaborator makes both the fixtures and the assertion vocabulary unfalsifiable"
domains: ["rendering", "tooling"]
related: [244]
---
Where a test fakes or fixtures a collaborator, the proof inherits the fake's model of that
collaborator twice over: once in what the fixtures feed in, and once in the strings the
assertions look for. When either drifts from the real collaborator, the test passes over a
real defect and no amount of additional execution reveals it, because both sides of the
comparison move together.

ADR-0244 hit both halves at one boundary. Every renderer fixture wrote the diff text without
the trailing row terminator that the Go side actually emits on every display row, so the
rendered stray context line that terminator produced in Pi was invisible to the complete
suite, the repository gates, and the plan's own mandated semantic rendering review. In the
same file an assertion checked that a no-op row did not contain `[toolDiff`, a shape only the
test's fake theme produces, while the code under test rendered through Pi's module-global
real theme; the substring could never occur, so the assertion could never fail and the
contract it named was unproven.

So for each faked or fixtured collaborator in the diff, compare against the real one in both
directions: does a fixture carry the exact shape the real producer emits, terminators and all,
and does every assertion's vocabulary come from the collaborator the code actually calls
rather than from the fake the test installed? This is distinct from fixture plurality, which
asks whether the set is heterogeneous enough to exercise a branch; a perfectly plural fixture
set that misreports the producer's shape is still blind. Prefer deriving fixtures from a
recorded real reply, and temporarily perturb any assertion whose subject string the production
path may never produce to prove that the focused test turns red.
