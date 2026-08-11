---
title: "Check the Open boundary before ignoring an assembly error"
domains: ["tooling"]
tags: ["coverage-gate", "cli-dispatch"]
related: [12, 92, 102]
---
Before adding `coverage-ignore` to an assembly error, verify whether `Open` already validates
every input that assembly reads. Retain the branch only when a later-only error source remains
reachable.
