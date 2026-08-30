---
title: "Make custom staged-slice hooks explicit about branch and cleanup"
domains: ["tooling", "adr-system"]
---
A custom staged-slice hook must name its temporary branch, resolve the invoking checkout's
hook path, and delete temporary state before an `exec` replacement. Otherwise branch-aware
checks can inspect the wrong branch or cleanup can be skipped.

Related decisions: [ADR-0202](../decisions/0202-slug-identified-pending-adrs-numbered-at-integration.md)
