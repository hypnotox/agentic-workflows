---
title: "Re-prove every touched coverage ignore"
domains: ["tooling"]
tags: ["coverage-gate"]
---
A `coverage-ignore` is a reachability claim, not proof that its branch is impossible. The
gate requires a reason and enforces 100% after exclusions, while the local audit only warns
when production ignores are added or touched. During review, apply
`coverage-ignore-reachability` to new, retained, and refactor-inherited ignores: refute each
one against the state or call order it declares impossible, and remove it when that argument
no longer holds.
