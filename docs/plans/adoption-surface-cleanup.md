---
type: Implementation Plan
title: Tidy the adoption surface and CLI feedback
description: Route for first-time adoption guidance, migration material, description output in resolve, placeholder checking, and this repository's own fixes.
---

# Plan: Tidy the adoption surface and CLI feedback

## Basis

Implements items 9 and 12 of the [workflow streamlining intent](../changes/workflow-streamlining/intent.md), after the [skills rewrite](workflow-skills-rewrite.md). The observation trial did not change these items.

## Changes

### 1. Adoption guidance

- Lead the README with what AWF is for and what an adopter gets, then how to start; remove the stale launcher condition; keep limits and boundaries after that.
- Give the literal first-install command wherever adoption is described.
- Warn before installation that every Markdown file under an existing `docs/` tree becomes part of the knowledge bundle and needs frontmatter for `check` to pass, and state the adopter's options.
- Move transitions from older installations out of the integration guide into one embedded migration page, `awf docs migrate`. Fold in what remains applicable from the root `MIGRATING-v0.50.md`, remove that file, and point to the tagged release for its history. Update references, the page list and its tests.
- Remove disclaimers repeated across the overview, integration, and knowledge guides, keeping each fact in one guide.

### 2. This repository's own guidance

- Remove the dead `.golangci*.yml` selector from the development topic.
- Move release and test procedure out of topics to where it is owned, leaving current knowledge; remove restated agent obligations from the efforts topic.
- Remove duplication between the README, `AGENTS.md`, and topics.

### 3. CLI feedback

- `resolve` prints each referenced topic's description with its source. The exact output form is a parsed contract and is confirmed with the owner before implementation.
- `check` reports a concept whose description is still an unedited starter prompt.
- Errors name a next step consistently, including a missing effort and a failed offline bootstrap.

Update command tests, the topics guide and command help, the knowledge guide, and the changelog; the `check` change is breaking for repositories with unedited starters.

## Verification

`./x gate` and `./x render && ./x check` after each change; independent review of change 3 as a contract change, and of the adoption guides as guidance for a first-time adopter.

## Integration

Changes 1 and 2 are documentation commits; change 3 lands as one commit per behavior. Merging into `main`, pushing, and releasing wait for the owner's confirmation.
