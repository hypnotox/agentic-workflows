---
type: Change Intent
title: Adopt OKF for AWF repository documentation
description: Adopt OKF v0.2 structural conformance with required AWF descriptions and interoperable ADR lifecycle metadata.
status: stable
---

# Intent: Adopt OKF for AWF repository documentation

## Problem and desired outcome

AWF has document conventions, topic-routing frontmatter, and an ADR lifecycle, but no common metadata contract across repository documentation. Adopt Open Knowledge Format (OKF) v0.2 so ordinary documentation can be consumed by compatible tools without an AWF-specific knowledge format.

In adopter repositories, `docs/` is the knowledge bundle. AWF starters produce conforming documents, authoring guidance maintains that convention, and `awf check` checks the bundle's structural conformance. Documents remain author-owned Markdown; this does not make AWF responsible for their truth, completeness, or approval.

## Bundle boundary and metadata

Cover ordinary Markdown recursively under `docs/`, including manually authored documents and document kinds not created by AWF. Nothing outside `docs/` becomes part of the bundle: root README and agent instructions, generated skills, `internal/docs/`, and local effort memory remain outside this contract. An absent or empty `docs/` is not an error.

Use minimal frontmatter. Every concept needs non-empty string `type` and `description` fields. Description is an AWF requirement beyond baseline OKF conformance, useful for ingestion and relevance assessment; diagnostics and guidance must distinguish it from an OKF requirement. `title` remains optional, with filename-derived titles supported by OKF. Include both title and description in all five starters and populate them in AWF's own documentation; encourage meaningful author-maintained values without judging prose quality. Other optional metadata is added only when useful. Do not require tags, provenance, verification records, or timestamps. Accept descriptive custom types and unknown extension fields rather than imposing a closed taxonomy.

Use these canonical types in AWF starters:

| Document | `type` |
|---|---|
| Topic | `Project Topic` |
| Intent | `Change Intent` |
| Specification | `Specification` |
| Plan | `Implementation Plan` |
| ADR | `Architecture Decision` |

Preserve topic `paths` and their repository-relative meaning. OKF concept IDs and bundle-root links are relative to `docs/`; they do not redefine AWF selectors. Keep existing topic routing and coverage behavior, rather than making every concept routable or changing routing based on `type`.

Respect OKF's reserved `index.md` and `log.md` filenames at every depth. These are not concepts and must follow their reserved-file conventions when present. They must not be mistaken for topics, and concept starters must not create them as ordinary documents. Neither file is required. A bundle-root index may declare `okf_version: "0.2"`; do not create an index merely to carry that field.

## ADR lifecycle

Move the existing ADR lifecycle from `status` to `decision_status`, preserving its meaning and authority. ADRs in the existing `docs/decisions/` location explicitly carry both `decision_status` and OKF `status`:

| `decision_status` | `status` | Meaning |
|---|---|---|
| `pending` | `draft` | Proposed; agreement is not established. |
| `accepted` | `draft` | Agreed, but not yet fully in effect. |
| `active` | `stable` | Governs the repository; applicable implementation has been verified. |

New ADRs start as `pending` / `draft`. `decision_status` owns the domain state; `status` is its interoperable representation, not a second approval process. Update them together. Use only `draft` and `stable` for this lifecycle; do not introduce a deprecated/archive state or change existing supersession and retirement rules.

The checker rejects missing, invalid, or inconsistent ADR state pairs. It does not infer agreement, verify implementation, promote decisions, or synchronize fields automatically. Preserve the existing rule that pending or accepted replacements do not displace still-active authority.

This mapping applies to ADRs in `docs/decisions/`. Other document kinds retain optional OKF lifecycle metadata without acquiring an AWF state machine. OKF describes draft as not yet reviewed; AWF deliberately keeps accepted decisions draft until activation. Generic OKF consumers cannot distinguish pending from accepted without reading the extension field.

## Checking and acceptance

Extend the existing `awf check` report rather than introducing a separate gate or required external tool. Validation is read-only, works without Git or network access, and reports actionable repository-relative file diagnostics. Bundle-wide conformance belongs only in `check`; `resolve`, `init`, and `render` retain topic validation without acquiring bundle-wide gates, and exclude reserved files from topic routing.

Success means:

- Every non-reserved Markdown document in `docs/` is checked, not only topics or AWF-created files. Invalid UTF-8, absent or malformed frontmatter, and missing, blank, or non-string `type` or `description` fail the check.
- Valid minimal documents with type and description, custom types, and additional fields pass. Missing optional metadata, absent indexes/logs, and broken cross-links are not conformance failures. Outside ADR state pairs, optional lifecycle, provenance, trust, and computation metadata do not acquire schema validation gates.
- Reserved files are exempt from concept type and description requirements. Check recognizable index heading/listing and newest-first date-grouped log structure, not exact example formatting, listing completeness, descriptions, or prose quality.
- ADR lifecycle consistency and existing topic-selector checks both hold. Valid reserved files inside `docs/topics/` do not become topics or require `paths`.
- All relevant starters produce conforming headers and retain create-only ownership. Files outside the bundle are unaffected by this new check. Non-ADR optional metadata does not acquire additional AWF-specific requirements.

Check structural conformance and declared-state consistency, not content quality, factual freshness, review evidence, or attestation. Do not turn optional OKF capabilities into additional gates.

## Adoption and constraints

Update canonical workflow/adoption guidance, starter templates, AWF's own `docs/`, and affected generated outputs coherently. Reconcile existing statements that AWF ignores ADR metadata or validates only topics. Keep shared rules in one authoritative home and reuse existing frontmatter parsing rather than adding a parallel parser or an OKF runtime dependency.

Provide explicit manual upgrade guidance: add appropriate types and meaningful descriptions, with optional titles; move legacy ADR status into `decision_status`; populate `status` using the mapping; preserve bodies, selectors, and established decision authority. Renaming a field must not imply new approval or activation. Surface conflicting metadata for reconciliation rather than guessing.

`init`, `render`, and `check` must not rewrite adopter-authored documents to migrate them. No automatic cross-repository rollout or migration engine is required.

Verification should cover the acceptance behavior, starter output, reserved-name interactions with topic routing, and an adopter-style legacy-to-current migration. Run the repository's normal tests and render/check workflow, and distinguish checks actually performed from assumptions.

Out of scope: search or ingestion services, MCP integration, generated indexes or graphs, trust/freshness automation, mandatory optional metadata, document-body templates beyond existing starters, and changes to agent-instruction ownership or effort storage.

## Reference and implementation context

Target specification: [Open Knowledge Format v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md), especially §§3–6, 8–9, and 11–12.

Relevant existing owners include `internal/frontmatter/`, `internal/artifactfs/`, topic loading and check integration, and `internal/docs/`. Follow the repository's agent guidance and inspect the current implementation before choosing the implementation route. This document establishes the outcome and boundaries, not an edit sequence.
