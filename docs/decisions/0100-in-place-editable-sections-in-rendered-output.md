---
status: Implemented
date: 2026-07-12
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [editable-sections, provenance-markers]
related: [15, 48, 60, 61, 70, 72, 83, 86]
domains: [config, rendering]
---
# ADR-0100: In-Place-Editable Sections in Rendered Output

## Context

Today every awf-rendered file is *wholly* owned. A rendered file is assembled from a template
skeleton plus per-section bodies, each body drawn from the template default or a convention
part input under `.awf/<kind>/parts/...` (ADR-0015), and written out in full; the whole file is
hash-locked (`manifest.Entry.OutputHash`) and `awf check` reports drift on any hand-edit. The
adopter's only override channel is a **part file** (a *render input*): edit the input under
`.awf/`, awf re-injects it, and the *output* stays 100% generated. There is no supported way for
an adopter to edit a rendered *output* directly and have awf preserve that edit across syncs.

That gap blocks a class of artifact awf wants to own but cannot fully generate: files that are
mostly boilerplate awf should own and keep current, but that also carry a region only the adopter
can fill because its content is project- and language-specific. The motivating case is a
**managed command runner** (`./x`): the awf-verb plumbing (`sync`, `check`, ...) is language-agnostic
boilerplate awf should own and keep from rotting, but the project verbs (`gate`, `test`, `build`)
are the adopter's own logic awf cannot render. A follow-on ADR consumes this primitive for the
runner; **this ADR defines only the general mechanism**, consumer-agnostic.

Three couplings shaped the design:

1. **Provenance pointers already segment the output.** ADR-0015 emits a one-line `awf:edit
   <name>` provenance pointer immediately before every non-dropped section body
   (`section-edit-pointer`), and, unlike the structural `awf:section`/`awf:end` markers, which
   are consumed and never leak (`no-section-marker-leak`), those `awf:edit` pointers **survive
   into the rendered output**. So the output is already partitioned into named regions: a
   section's body runs from just after its pointer to just before the next `awf:edit`-family
   pointer, or end-of-file. No new closing-fence grammar is needed to bound an editable region.

2. **A second drift mode already exists.** Generated indexes (`ACTIVE.md`, the config reference,
   the domain docs) are not checked by the frozen-`OutputHash` equality compare; they are checked
   by **regeneration** (re-derived from their sources and compared to disk) and are singled out
   of the hash compare by a **hardcoded set of path checks** in `internal/project/check.go`. A
   file whose adopter-owned region legitimately changes between syncs *must* use this regeneration
   mode, or a legitimate edit would false-positive as a hand-edit. Today membership in that class
   is a hand-maintained path list, not a first-class property, a smell this ADR removes.

3. **A surviving pointer must be a valid comment in the target's language.** Because the `awf:edit`
   pointer survives into the output *as a comment*, it must be a comment in the rendered file's own
   syntax, not only in Markdown. Every section-bearing file awf renders today is Markdown, where an
   HTML `<!-- ... -->` comment is correct; but the first consumer of this primitive is a **shell
   script** (the managed runner), where `<!-- ... -->` is a hard syntax error. awf already adapts one
   comment by target (`injectBanner` emits a `#`-line banner after a `#!` shebang and an HTML banner
   otherwise), so the surviving section pointers must adapt the same way, or an in-place shell file is
   unrunnable.

4. **A rendered script must be executable.** awf's sync writes every rendered file with mode `0644`.
   That suits Markdown and the `bash ...`-invoked scripts awf renders today (the bootstrap, the hook
   payloads), but the managed runner is invoked as `./x` (every command var defaults to `./x ...`),
   which requires the execute bit. A `0644` runner is a permission error on first use. Executability,
   like the comment style, is a shell-script property awf must render, not leave to a post-render
   `chmod` the adopter must remember and re-apply.

The alternative to in-place editing, a two-file split (awf renders a payload, the adopter owns a
thin stub that delegates to it, as ADR-0048 does for git hooks), reuses all existing machinery
but sacrifices single-file ergonomics and yields no reusable capability. In-place editing is the
larger investment, justified by being a general primitive: once awf can own *part* of a file, the
same mechanism later serves lint config, a `Makefile`, or CI config, not only the runner.

## Decision

Introduce **in-place-editable sections**: a section whose adopter-owned content lives directly in
the rendered output file and is preserved across syncs, while awf continues to own every other
section and the file's structure.

1. **A fourth provenance-pointer variant, `awf:edit-in-place <name>`.** An in-place-editable
   section renders this distinct pointer in place of the three existing `awf:edit` variants
   (from-part / stub / default; `internal/render` `editPointer`). Its phrasing states the
   contract (the region below is the adopter's, preserved across syncs) and carries no part
   path (there is no part input). The token is distinct from `awf:edit` so read-back can tell the
   two apart, and it is chosen to pass every existing marker guard unchanged: `no-section-marker-leak`
   and the residual-marker guard (ADR-0070) anchor on `awf:section`/`awf:end`, the `awf:include`
   partial guard rejects only those two literals, and the part-marker advisory (ADR-0083) and stub
   marker are likewise unaffected.

2. **Read-back sourcing, bounded by awf's section registry.** On both sync and check, an
   in-place-editable section's body is sourced by **reading it back from the existing output
   file**, rather than from a template default or a `.awf/parts/` part. The region runs from just
   after the section's `awf:edit-in-place` pointer to the **exact expected pointer of awf's next
   registered section** (from the ordered section list awf is assembling), or end-of-file when the
   in-place section is last. The trailing boundary is thus awf's own next pointer, matched by its
   expected string (**not any pointer-shaped line found in adopter text**), so an adopter body that
   happens to contain a line resembling an `awf:edit`-family pointer does not truncate the region.
   Adopter content is matched (bounded), never parsed. When the output file is absent (first render)
   or the section's pointer is not found, the body falls back to the template default (a starter
   scaffold). The read-back is spliced verbatim, never re-templated.

3. **awf owns the framing; the adopter owns the region interior.** Only the **leading and trailing
   whitespace** of an in-place region is awf-owned framing (the blank-line separation between a
   section body and its neighbouring pointers), re-applied canonically on every render. Every line
   **inside** the region, including internal blank lines (a runner block's own spacing), is
   adopter-owned and spliced back verbatim; awf never reflows or canonicalizes the interior. Because
   the interior is echoed byte-for-byte and only the outer framing is regenerated to a fixed form,
   the round-trip is an idempotent fixpoint: after a sync the file is canonical, and a re-check reads
   the same interior back and matches, so benign whitespace never churns as drift.

4. **A section is part-overridable OR in-place-editable, never both.** The two override channels
   are mutually exclusive per section: a section sourced from a `.awf/parts/` input (the existing
   `awf:edit` mechanism) cannot also be in-place-editable, and vice versa. There is no precedence
   chain. An in-place-editable section adds no `.awf/` part file, so it introduces no new claimed
   path under the closed config tree (ADR-0086).

5. **Drift by regeneration-with-read-back.** A file carrying any in-place-editable section is
   drift-checked by regeneration, not by frozen-`OutputHash` equality: awf re-derives every
   awf-owned section and the file structure **from the template** (never from disk) and reads
   in-place sections back from the existing output, then compares the assembly to disk. An edit to
   an awf-owned section or to the file structure therefore surfaces as drift and is overwritten on
   the next sync; only in-place-section content is preserved. awf-owned regions and layout cannot
   be persistently tampered.

6. **A first-class `regeneration-checked` attribute, replacing the hardcoded path list.**
   Membership in the regeneration-checked class (no frozen-`OutputHash` compare) becomes an
   explicit attribute on the rendered-file model rather than a hand-maintained set of path checks
   in `check.go`. The existing regeneration-checked outputs (`ACTIVE.md`, the config reference,
   the domain docs) are migrated onto the attribute in the same change, and files with an
   in-place-editable section carry it. No new file joins the class by editing a hardcoded path.

7. **Provenance pointers render in the target file's comment syntax.** All `awf:edit`-family
   pointers (the three `awf:edit` variants and the `awf:edit-in-place` variant) are emitted in a
   **comment style** chosen per target: a `#`-line comment (`# awf:edit-in-place <name>: ...`) for a
   `#!`-shebang target such as a shell script, and the HTML comment (`<!-- ... -->`) for every other
   target. The style is derived from the (expanded) template source by the same shebang sniff
   `injectBanner` uses, so **the pointer emitter and the read-back matcher derive it identically and
   always agree on the exact expected pointer string**. Only the *comment delimiters* change; the
   `awf:edit`/`awf:edit-in-place <name>: ...` token and phrasing are constant, so pointer distinctness
   (Decision 1) and read-back bounding (Decision 2) hold in either style. The structural
   `awf:section`/`awf:end` markers are consumed during assembly and never reach output, so their
   guards (`no-section-marker-leak`, the residual-marker guard, the `awf:include` guard) are
   comment-style-independent and unaffected. Markdown output is unchanged (no `#!`, so HTML style).

8. **A rendered `#!`-shebang file is written executable.** On sync, a rendered file whose content
   begins with a `#!` shebang is written with mode `0755` (executable); every other rendered file
   stays `0644`. The test is the one `#!`-prefix predicate Decision 7 and `injectBanner` already use
   (there it reads the expanded template source, here the rendered content, and the two agree because
   the shebang is the first line of both), so "is this a script?" has one definition across the
   banner, the pointer comment style, and the file mode. The
   mode is enforced on every sync (a pre-existing file's mode is corrected, not just set at
   creation), so the runner is runnable as `./x` immediately after the first render, and the
   bootstrap and hook payloads (already `#!` scripts) become executable too (harmless: they are
   still invoked via `bash ...`). Drift is unaffected: `awf check` compares rendered *content*, not
   file mode.

## Invariants

- `invariant: in-place-pointer-distinct`: an in-place-editable section renders an `awf:edit-in-place
  <name>` provenance pointer in the target's comment syntax (a `#` line comment for a `#!`-shebang
  target, an HTML comment otherwise), textually distinct from the `awf:edit` pointer; neither the
  pointer nor any in-place read-back causes `awf:section`/`awf:end` marker residue to appear in output
  (`no-section-marker-leak` and the `awf:include` partial guard stay green with in-place sections
  present).
- `invariant: in-place-readback`: on sync and check, an in-place-editable section's body is the text
  read back from the existing output file between its `awf:edit-in-place` pointer and awf's next
  *registered* section pointer (matched by that pointer's expected string in the target's comment
  syntax, never any pointer-shaped line in adopter text), or end-of-file when the section is last;
  when the output is absent or the section's pointer is not found, the body is the template default.
- `invariant: in-place-tamper-drift`: regeneration re-derives every awf-owned section and the file
  structure from the template, so an edit to an awf-owned region or to the file structure is
  reported as drift, while an edit confined to an in-place-editable section's content lines is not.
- `invariant: section-source-exclusive`: no section is simultaneously part-overridable and
  in-place-editable; a declaration asserting both is a render/build error.
- `invariant: in-place-spacing-owned`: an in-place region's interior (all lines between its framing,
  including internal blank lines) is spliced back verbatim while only the leading/trailing framing
  is regenerated to a fixed form, so the sync→check round-trip is an idempotent fixpoint (a second
  sync with no adopter edit is a no-op and reports no drift).
- `invariant: regeneration-checked-attribute`: the files excluded from the frozen-`OutputHash` compare
  are exactly those a first-class attribute on the rendered-file model marks regeneration-checked;
  `ACTIVE.md`, the config reference, and the domain docs carry that attribute, and every file with
  an in-place-editable section carries it. (That this attribute *replaces* the former hardcoded
  path list is enforced by the dead-code gate on the removed literals, not by this bullet.)
- `invariant: shebang-rendered-executable`: a rendered file whose content begins with a `#!` shebang
  is written with an executable mode (`0755`) and every other rendered file with `0644`; the mode
  follows the one `#!`-prefix predicate (shared with the banner and pointer comment style) and is
  enforced on every sync, not only at file creation.

## Consequences

Easier:
- awf can own boilerplate-heavy files that also need an adopter-specific region, keeping the
  awf-owned part from rotting while the adopter edits their region in place, in one file: no
  two-file split, no delegation stub.
- The general primitive is reusable beyond its first consumer (lint config, `Makefile`, CI config).
- Regeneration-checked membership becomes explicit and self-documenting; adding a regeneration-
  checked file no longer means editing a hardcoded path list (ADR-0060/0061 single-source ethos).

Harder / accepted trade-offs:
- A new drift path (regeneration-with-read-back), a new pointer variant, and a per-target pointer
  comment style enlarge the render/check surface. This is genuinely more machinery than a two-file
  split, accepted for the reusable primitive it buys. The comment style is a two-value sniff (`#!`
  shebang → `#`-line comment, else HTML), shared by the pointer emitter and the read-back matcher so
  they cannot diverge. The sniff assumes a `#!`-shebang target uses a `#`-line comment, true for
  shell, Python, Ruby, but **not** e.g. `#!/usr/bin/env node` (JS comments are `//`): such a target
  is silently *misclassified* as `#`-style, not cleanly rejected, and would render a broken file.
  This mirrors `injectBanner`'s existing shebang assumption, so it is a shared, consistent limitation
  rather than a new one; supporting a non-`#` shebang target (or any comment syntax that is neither
  `#` nor HTML) means refining the sniff, not merely adding a branch.
- Sync now sets file mode from content (executable for `#!` scripts), a small new side effect on the
  write path. The bootstrap and hook payloads flip from `0644` to `0755`, a one-time mode change in
  every adopter, harmless since they are still `bash ...`-invoked, and the same `#!` sniff already
  governs their banner.
- The adopter may extend a file **only inside** its in-place-editable section(s); content added
  elsewhere is discarded on the next sync (and drift-flagged before it). Templates consuming this
  primitive must place in-place sections at the real extension points. This is also a coherence
  feature: the file's structure stays awf-owned.
- **First adoption of a pre-existing hand-written file is lossy.** A file already on disk with no
  `awf:edit-in-place` pointer is treated as foreign: on first sync it is backed up (`*.awf-bak`)
  and overwritten with the scaffold, so its adopter content lands only in the backup and must be
  hand-ported into the new in-place section. Mitigation: the backup is retained and the port is a
  one-time cost per file; the consuming ADR/plan owns any specific migration.
- If an adopter deletes an in-place section's `awf:edit-in-place` pointer, read-back loses its
  anchor and the section reverts to the default on the next sync (git-recoverable, and drift-flagged
  beforehand). A future refinement may add an `awf check` note for a missing/duplicated in-place
  pointer; this ADR does not require it.

Ruled out:
- A dedicated closing-fence marker in the output (the pointer sequence already bounds regions).
- A precedence chain between part-override and in-place sources (mutual exclusivity is simpler).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Two-file split (awf payload + adopter stub, as ADR-0048 does for hooks) | Reuses all existing machinery but loses single-file ergonomics and yields no reusable primitive; the whole point is a general in-place capability. |
| New OPEN+CLOSE fence markers to bound the editable region | Unnecessary: the durable `awf:edit`-family pointer sequence already partitions the output; a closing marker would risk tripping `no-section-marker-leak`/`awf:include` guards. |
| Keep read-back content in a `.awf/parts/` input instead of the output | That is the existing part mechanism; it keeps the output fully generated and does not let the adopter edit the output in place: the exact gap this ADR closes. |
| Extend the hardcoded regeneration-checked path list with each new file | Perpetuates a by-hand path list; a first-class attribute matches awf's compile-time single-source model (ADR-0060/0061). |
| Precedence chain (part input overrides in-place, or vice versa) | Ambiguous and harder to reason about than declaring each section as exactly one source. |
