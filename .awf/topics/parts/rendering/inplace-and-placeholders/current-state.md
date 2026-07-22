In-place editable sections and their readback, authoring-comment stripping, convention-part placeholders, and var and data hygiene findings.

## Claims

### `invariant: absent-var-acknowledged`

An absent vars key never produces an unset-var completeness note, while a present key with an empty or null value does.
Origin: ADR-0148
Backing: test

### `invariant: authoring-comment-inplace-inert`

An in-place section body read back from rendered output is never subject to the authoring-comment strip: a directive-shaped line inside an in-place region survives re-render byte for byte.
Origin: ADR-0148
Backing: test

### `invariant: authoring-comment-stripped`

A whole-line awf:comment directive in a template source, an include partial, or a convention part never appears in rendered output, across every render unit.
Origin: ADR-0148
Backing: test

### `invariant: escaped-placeholder-literal`

A backslash placed immediately before an awf placeholder-token opener in a convention part renders the literal token with the backslash consumed, triggering neither placeholder substitution nor the residual-token guard error.
Origin: ADR-0148
Backing: test

### `invariant: in-place-readback`

On both sync and check, an in-place-editable section's body is read back from the existing output file between its awf:edit-in-place pointer and awf's next registered section pointer, matched by that pointer's exact expected string rather than any pointer-shaped adopter line, or to end-of-file when it is the last section; when the output is absent or the pointer is missing, the body falls back to the template default.
Origin: ADR-0148
Backing: test

### `invariant: in-place-spacing-owned`

An in-place region's interior, including its internal blank lines, is spliced back verbatim while only the leading and trailing blank framing is regenerated to a fixed form, so a sync followed by a check is an idempotent fixpoint that reports no drift on unedited whitespace.
Origin: ADR-0148
Backing: test

### `invariant: in-place-tamper-drift`

A file with an in-place-editable section is drift-checked by regenerating every awf-owned section and the file structure from the template, so an edit to an awf-owned region or the structure surfaces as drift and is overwritten, while an edit confined to an in-place section's content lines is preserved and reports clean.
Origin: ADR-0148
Backing: test

### `invariant: part-placeholder-sandboxed`

A `{{=awf:key}}` placeholder in a convention part is resolved by literal substitution against a closed registry of config-derived values, never through the template engine; an unknown or empty key, or any residual `{{=awf` token surviving substitution, is a hard render error that fails both `awf sync` and `awf check`.
Origin: ADR-0148
Backing: test

### `invariant: placeholder-value-token-free`

Building the placeholder registry returns a hard error naming the offending key when any registry value itself contains an awf placeholder token.
Origin: ADR-0148
Backing: test

### `invariant: section-orphan-flagged`

A convention part at docs/parts/<name>/<section>.md whose section is not among the enabled target's catalog-declared sections is reported by check as orphaned drift, while a part at a genuinely declared section is not.
Origin: ADR-0148
Backing: test

### `invariant: section-source-exclusive`

No section is both part-overridable and in-place-editable; a template section declared in-place that also has a matching convention part is a render error rather than a precedence resolution.
Origin: ADR-0148
Backing: test

### `invariant: unused-data-drift`

A sidecar data key with no matching .data reference in its artifact's assembled sources, unioned across all enabled targets, is reported by awf check as failing drift keyed to the sidecar file.
Origin: ADR-0148
Backing: test

### `invariant: unused-var-drift`

A non-empty vars key referenced by no assembled template source and by no gate- or check-command placeholder in any consumed convention part is reported by awf check as failing drift; empty-valued vars keys never are.
Origin: ADR-0148
Backing: test
