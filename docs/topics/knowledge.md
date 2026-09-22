---
type: Project Topic
title: Knowledge bundle validation
description: Structural checking of the docs bundle, metadata ownership, and integration with routing and creation.
paths:
  - 'internal/knowledge/**'
  - 'internal/docs/knowledge.md'
  - 'docs/**'
---

# Knowledge bundle validation

The active [knowledge-bundle decision](../decisions/knowledge-bundle-contract.md) owns the rationale for OKF adoption, required descriptions, optional titles, and separate ADR authority. The [adoption intent](../changes/okf-adoption/intent.md) remains the acceptance reference for maintaining this behavior.

`internal/knowledge` owns read-only recursive Markdown validation beneath `docs/`. `internal/projector.Check` incorporates its repository-relative findings into the existing sorted check report alongside generated-output drift; existing topic-selector validation remains in place. `init`, `render`, and `resolve` do not acquire bundle-wide gates. The scan is independent of Git and network access, accepts absent/empty bundles, and does not follow symlinks.

`internal/docs/knowledge.md` (available as `awf docs knowledge`) is the authoritative shared authoring contract: OKF v0.2 structural conformance plus AWF's required non-empty description. Title remains optional. `internal/docs/adr.md` owns the explicit decision-state mapping for concepts under `docs/decisions/`, and `internal/docs/integration.md` owns manual upgrades. Neither checking nor generation establishes authority, migrates authored metadata, or synchronizes states.

The validator reuses `internal/frontmatter.Parse` with untyped metadata so YAML numbers and booleans cannot silently satisfy string fields. Unknown extensions and optional families remain opaque. Goldmark parses only reserved-file Markdown bodies to recognize actual headings, lists, and links rather than mistaking code examples for structure; it is not an OKF runtime dependency. Links need not resolve. Empty index listings and log histories may contain just their heading.

`knowledge.IsReserved` is shared by validation, topic loading, and create-only starters. Basenames `index.md` and `log.md` are reserved case-insensitively at every depth; directory names are not reserved. Reserved files are not concepts, do not require type/description or ADR metadata, and never route as topics. Topic selection remains based on location and repository-relative `paths`, not type; bundle-root concept IDs and links do not change selector meaning.

`internal/knowledge/knowledge_test.go` covers minimal/custom concepts, typed required values, permissive optional metadata, ADR pairs, reserved structures, bundle boundaries, and non-mutation. Projector and artifact tests cover integration, starter conformance, reserved-name interactions with routing, and explicit legacy-to-current author edits preserving bodies, selectors, and decision states. These checks establish structural behavior, not documentation truth, completeness, freshness, or approval.
