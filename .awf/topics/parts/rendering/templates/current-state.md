The templates tree holds the embedded skill, agent, doc, and adapter template source. The claims below capture the current template-content contracts.

## Claims

### `invariant: catalog-template-sweep`

A catalog-derived loop renders every standard skill and agent template under empty data - iterating the catalog itself rather than a hand-maintained list - and fails on any leak residue or any skill cross-reference in the output that the artifact has not declared.
Origin: ADR-0080
Backing: test

### `invariant: commit-scope-single-storage`

No file under the embedded templates references `.vars.commitScope`, and the catalog `vars:` block carries no commitScope descriptor; every rendered commit-scope mention derives from `audit.allowedScopes` through the commitScopes render-context key.
Origin: ADR-0051
Backing: test

### `invariant: conditional-fallback-case-guard`

Every standard skill or agent template whose post-include-expansion source contains a conditional action - if, with, or range - must have a hand-authored unset-data case in the fallback case list, and the guard names any template missing one.
Origin: ADR-0080
Backing: test

### `invariant: empty-init-coherent-render`

A non-interactive `awf init` with no answers renders artifacts that contain no empty inline code spans, no tables lacking body rows, and no list-introduction sentence followed by nothing, so every artifact degrades to coherent prose.
Origin: ADR-0045
Backing: test

### `invariant: golden-test-completeness`

Every standard catalog skill has a per-artifact Test<Skill>Template function and every catalog agent a Test<Agent>Agent function in the project package's test source, verified by a source scan.
Origin: ADR-0080
Backing: test

### `invariant: local-base-publication-safe`

The base skill and agent templates render leak-free, with no <no value> and no marker or leak residue, under empty data and no content part.
Origin: ADR-0068
Backing: test

### `invariant: local-doc-base-publication-safe`

The base doc template renders leak-free under empty data and no content part, producing generic prose with no unresolved-value token, section marker, or leak residue.
Origin: ADR-0091
Backing: test

### `invariant: template-source-residue`

Every file in the embedded templates tree is free of concrete ADR citations (the token ADR- followed by four digits) and free of the repo-identity literals hypnotox and agentic-workflows, except in an explicit exemption list whose each entry fails when its named file no longer carries the literal.
Origin: ADR-0082
Backing: test

### `invariant: templates-valid-frontmatter`

Every catalog skill and agent template, rendered with representative data, produces leading frontmatter that parses as YAML with a non-empty name and a non-empty description.
Origin: ADR-0006
Backing: test
