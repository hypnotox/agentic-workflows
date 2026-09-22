---
type: Architecture Decision
title: Use OKF for repository knowledge with a required AWF description
description: Preserve interoperable Markdown knowledge while requiring useful summaries and keeping ADR authority separate from generic lifecycle metadata.
decision_status: active
status: stable
---

# Repository knowledge contract

## Context

AWF's topic selectors and decision lifecycle already give repository documents domain-specific meaning, but their metadata lacked a shared interchange contract. The [agreed adoption intent](../changes/okf-adoption/intent.md) establishes the outcome, acceptance criteria, and scope. The implementation now checks that contract without taking ownership of authored content.

## Decision and rationale

Use OKF v0.2 for ordinary Markdown recursively under `docs/`, keeping its open concept types, extension fields, and permissive optional metadata. This reuses an external knowledge format rather than creating an AWF-specific interchange schema. Repository-root instructions, generated skills, internal guides outside the bundle, and local effort memory retain their existing roles.

Require a non-empty description in addition to OKF's required type. A description supplies a relevance summary that an ingestion consumer otherwise has to extract from prose. Keep title optional because OKF permits deriving a display name from the filename; requiring a second authored title does not offer the same incremental value. Starters include both fields to encourage useful authoring. Identify the description requirement as AWF policy, not baseline OKF conformance, and do not grade its prose.

Keep ADR domain authority in `decision_status` and expose its interoperable representation through `status`. The mapping and maintenance rules live in `awf docs adr`. Separating the fields preserves the distinction between an agreed decision and one already in effect without assigning AWF's states to OKF's status vocabulary. Accepted decisions remain draft until activation, even though they have been reviewed; generic OKF consumers cannot distinguish them from pending proposals. This is a deliberate, conservative interoperability trade-off, not a second approval process.

Keep enforcement read-only and local to `check`, alongside existing topic and generated-output checks. Topic routing continues to use location and repository-relative selectors, not concept type. Reserved indexes and logs are excluded from concepts and routing. Conformance says nothing about factual truth, completeness, review, or implementation evidence. Neither validation nor generation promotes a decision, synchronizes metadata, or migrates authored files.

## Consequences

Existing adopters must add meaningful descriptions and migrate legacy ADR status explicitly, preserving bodies, selectors, and established authority. This is a breaking documentation contract, with manual instructions in `awf docs integration`. Missing optional metadata and unresolved cross-links remain acceptable. There is no required OKF service or runtime tool.

The shared authoring contract lives in `awf docs knowledge`; this record owns the lasting rationale rather than duplicating its validation rules. Tests in `internal/knowledge/`, `internal/artifactfs/`, and `internal/projector/` cover structural acceptance, starter output, reserved-name routing, and manual migration without authored rewrites. The local gate and Linux native-release smoke verified the applicable implementation during adoption. These checks do not attest the contents of adopter documents.
