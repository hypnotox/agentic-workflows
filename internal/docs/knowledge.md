# Author repository knowledge

Ordinary Markdown recursively under `docs/` forms an [Open Knowledge Format (OKF) v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md) knowledge bundle. This includes manually authored files and document kinds AWF does not create. Documents remain author-owned. Root README and agent instructions, generated skills, AWF's internal guides, and local effort memory are outside the bundle.

Use this guide for the shared metadata and reserved-file contract. Use `awf docs topics` for routing, `awf docs changes` for change definitions and plans, and `awf docs adr` for decision authority. Use `awf docs integration` for installation and manual upgrades.

## Concept metadata

Every non-reserved `.md` file in `docs/` is a UTF-8 concept with leading YAML frontmatter:

```markdown
---
type: Reference
title: Local development
description: Set up the local toolchain and run the repository's checks.
---

# Local development

Author-owned content goes here.
```

`type` and `description` must be non-empty strings. **Description is an AWF requirement beyond baseline OKF**, which requires only type. Write a concise summary that helps readers and ingestion tools assess relevance without reading the whole document. `title` is optional; OKF consumers may derive it from the filename. Prefer a meaningful display title when that improves discovery. AWF starters include both title and description; replace the slug-derived title and description prompt with useful authored values.

The starter types are `Project Topic`, `Change Intent`, `Specification`, `Implementation Plan`, and `Architecture Decision`. Descriptive custom types and unknown extension fields are welcome; this is not a closed taxonomy. Tags, timestamps, provenance, verification, and other optional fields are not required. Non-ADR optional metadata has no additional AWF schema gate or state machine. Missing metadata does not establish review or approval, even though OKF consumers default an absent status to stable.

Concept IDs are paths relative to `docs/`, without `.md`. Links starting with `/` are bundle-root-relative; ordinary relative Markdown links also work and remain convenient for repository browsing. Neither convention changes topic `paths`, which remain **repository-relative**. Type does not determine topic routing. Broken cross-links are not conformance failures.

## Reserved indexes and logs

`index.md` and `log.md` are reserved at every depth. They are not concepts, need neither type nor description, and never become topics or require selectors. AWF recognizes Markdown extensions and these reserved basenames case-insensitively. Concept starters refuse these basenames; a directory called `index` or `log` is fine.

An index groups linked list entries under Markdown headings:

```markdown
# Development

- [Local setup](setup.md) - Toolchain and development commands.
- [Design notes](design/) - Current design guidance.
```

Indexes have no frontmatter except that `docs/index.md` may contain only `okf_version: "0.2"` in a YAML block. There is no need to create an index just to declare the version. Unknown declared versions receive best-effort structural checking, not rejection solely for their version.

A log has no frontmatter and records a flat list of updates under valid `YYYY-MM-DD` date headings, newest first:

```markdown
# Update log

## 2026-06-20

- Added the local setup guide.

## 2026-06-01

- Established the development documentation.
```

Neither reserved file is required. Headings with no entries may represent an empty listing or history. Checks recognize Markdown headings and lists, including Setext headings, ordered lists, and reference-style links; fenced code and quoted examples are not sections or entries. Checks do not require an exhaustive listing, entry descriptions, particular prose, or bold labels in logs.

## Check the bundle

Run `awf check` from the repository root. It reports repository-relative structural findings alongside existing generated-output checks and retains topic-selector validation. Every non-reserved Markdown file under the conventional ADR location, `docs/decisions/`, additionally requires the consistent, explicit state pair defined by `awf docs adr`. This check is location-bound: a custom type does not opt a decision record out, and an Architecture Decision type elsewhere does not opt a file in. Reserved files are exempt.

The checker rejects invalid UTF-8, missing or malformed concept frontmatter, missing/blank/non-string type or description, invalid reserved-file structure, and missing/invalid/inconsistent ADR state pairs. An absent or empty `docs/` is valid. The scan is filesystem-based, includes hidden directories and manually authored Markdown, and does not consult Git or ignore rules. It does not follow symlinks; Markdown file entries must be regular files, and `docs/` itself must be a directory rather than a symlink.

Validation is read-only and offline. It checks structure and declared-state consistency, not truth, completeness, prose quality, freshness, review evidence, agreement, implementation, or attestation. `init`, `render`, and `resolve` retain topic validation but do not acquire bundle-wide conformance gates. None of these commands migrates authored documents or synchronizes metadata.
