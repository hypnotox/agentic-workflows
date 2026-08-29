---
title: "A proof marker does not prove every clause in its invariant claim"
domains: ["invariants"]
---
The backing checker proves that a named test-scoped marker exists, not that the test
exercises every clause of the claim. Read the claim as a conjunction, identify each status,
direction, artifact, and failure branch it names, and confirm each clause has a refuting case.
For rendered prose claims, delete each named clause in turn and watch the suite fail.
