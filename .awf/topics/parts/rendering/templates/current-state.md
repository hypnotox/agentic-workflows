The templates tree holds embedded AWF skill, document, script, and project-artifact template source. The claims below capture the current template-content contracts.

## Claims

### `invariant: catalog-template-sweep`

A catalog-derived loop renders every standard AWF skill template under empty data - iterating the catalog itself rather than a hand-maintained list - and fails on any leak residue or undeclared skill cross-reference in the output.
Backing: test

### `invariant: commit-scope-single-storage`

No file under the embedded templates references `.vars.commitScope`, and the catalog `vars:` block carries no commitScope descriptor; every rendered commit-scope mention derives from `audit.allowedScopes` through the commitScopes render-context key.
Backing: test

### `invariant: conditional-fallback-case-guard`

Every standard AWF skill template whose post-include-expansion source contains a conditional action - if, with, or range - must have a hand-authored unset-data case in the fallback case list, and the guard names any template missing one.
Backing: test

### `invariant: singleton-conditional-key-live`

Every conditional in a live singleton template is supplied by that artifact's real render context and
is exercised with both outcomes. The population derives from catalog and singleton declarations;
recognition-only templates are excluded, and missingkey=zero retains generic empty fallbacks.
Backing: test

### `invariant: empty-init-coherent-render`

A non-interactive `awf init` with no answers renders artifacts that contain no empty inline code spans, no tables lacking body rows, and no list-introduction sentence followed by nothing, so every artifact degrades to coherent prose.
Backing: test

### `invariant: template-source-residue`

Every file in the embedded templates tree is free of concrete ADR citations (the token ADR- followed by four digits) and free of the repo-identity literals hypnotox and agentic-workflows, except in an explicit exemption list whose each entry fails when its named file no longer carries the literal.
Backing: test

### `invariant: retired-config-guidance-absent`

Live template and current-state source never presents the retired sidecar field or former artifact-selection channels as supported configuration. The focused residue test permits only named unrelated historical migration and Pi preference-file references, so it cannot become a blanket scan that erases truthful vocabulary.
Backing: test

### `invariant: decision-artifact-routing`

Decision records preserve only load-bearing choices that should outlive implementation and were accepted before authoring; current-state claims own active rules and invariants; direct execution and effort-local plans own implementation detail; and effort memory owns unsettled or transient context. New records are plain accepted date-slug Markdown with Context, Decision, and Consequences. Historical numbered records remain append-only bytes rather than live authority.
Backing: unbacked
Verify: Compare each new decision with its consent evidence, confirm it records a load-bearing choice rather than implementation choreography, and verify its accepted date-slug filename and Context, Decision, and Consequences sections. Confirm current rules remain in current-state topics and existing numbered decision files were not rewritten.

### `invariant: source-embed-parity`

The repository source population walks `os.DirFS(".")`, includes every file below root template directories while excluding only root Go source and test files, and compares its exact regular-file set bidirectionally with the regular-file set walked from the embedded filesystem root. Failures diagnose sorted paths as missing from embed or unexpected in embed; this proves file-set parity only, not semantic validation.
Backing: test

### `invariant: templates-valid-frontmatter`

Every catalog AWF skill template, rendered with representative data, produces leading frontmatter that parses as YAML with a non-empty name and a non-empty description.
Backing: test
