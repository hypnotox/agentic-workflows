# Testing

## The gate

A single gate command runs the project's checks — tests, vet/lint, and any drift verification — and must be green before every commit. Treat a red gate as a blocker, never a warning: fix the cause or revert, do not commit around it.

## Tiers

The gate has tiers. A fast tier runs on every commit and covers the common path cheaply; a fuller tier runs the slower, broader checks before merging or releasing. Reach for the fuller tier when a change is risky or cross-cutting, and always before integrating.

## Test layout

_Describe where tests live and how they map to the code: the directory convention, how unit, integration, and regression tests are named and separated, and where a new test for a given change belongs._
