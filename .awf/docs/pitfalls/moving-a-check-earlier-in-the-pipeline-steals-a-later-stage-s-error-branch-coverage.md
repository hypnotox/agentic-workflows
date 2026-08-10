---
title: "Moving a check earlier in the pipeline steals a later stage's error-branch coverage"
domains: ["tooling"]
tags: ["coverage-gate"]
---
A fixture that corrupts state up front (a directory where a sidecar file belongs, an
unreadable file) to pin a *late* stage's error propagation silently changes meaning when a
new earlier stage starts reading the same state: the error now surfaces there, the late
branch goes uncovered, and the 100% gate flags a line nobody edited. ADR-0086's open-time
domain-sidecar validation did this to `TestAuditPropagatesDomainSidecarReadError`;
(2026-07-10) Audit's read-error branch was suddenly unreachable from a pre-corrupted
tree. The repair pattern: corrupt *after* the earlier stage has run (post-`Open`
mutation), so each stage's own error branch keeps a test that reaches it; and when adding
an earlier check, grep the tests for fixtures corrupting the state it newly reads.
