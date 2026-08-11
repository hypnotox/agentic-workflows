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

### `invariant: singleton-conditional-key-live`

Every conditional in a live singleton template is supplied by that artifact's real render context and
is exercised with both outcomes. The population derives from catalog and singleton declarations;
recognition-only templates are excluded, and missingkey=zero retains generic empty fallbacks.
Origin: ADR-0235
Backing: test

### `invariant: empty-init-coherent-render`

A non-interactive `awf init` with no answers renders artifacts that contain no empty inline code spans, no tables lacking body rows, and no list-introduction sentence followed by nothing, so every artifact degrades to coherent prose.
Origin: ADR-0045
Backing: test

### `invariant: golden-test-completeness`

Every standard catalog skill has a per-artifact Test<Skill>Template function and every catalog agent a Test<Agent>Agent function in the project package's test source, verified by a source scan.
Origin: ADR-0080
Backing: test

### `invariant: template-source-residue`

Every file in the embedded templates tree is free of concrete ADR citations (the token ADR- followed by four digits) and free of the repo-identity literals hypnotox and agentic-workflows, except in an explicit exemption list whose each entry fails when its named file no longer carries the literal.
Origin: ADR-0082
Backing: test

### `invariant: decision-artifact-routing`

ADR Decision items own only commitments explicitly accepted by the user before authoring, stated as the narrowest durable semantics that preserve the approved decision and remain meaningful after implementation; current-state claims own active rules and invariants; plans and direct execution own implementation detail; and effort memory owns unsettled or transient working context. Relatedness, usefulness, repository facts, and architectural reasoning do not authorize another commitment, and a suggestion stays outside the ADR until accepted. ADR review applies the post-implementation and counterfactual tests semantically, treats a misplaced implementation directive as a reasoned finding, and accepts a mechanism only when the record explains why that mechanism itself is load-bearing. Authoring guidance preserves scaffold-emitted ADR frontmatter, objective rendering checks enforce publication contracts without inferring prose meaning, and terminal ADR bodies remain unchanged.
Origin: ADR-0224
Revised-by: ADR-user-approved-adr-decision-boundaries
Backing: unbacked
Verify: For each new or amended ADR and its linked plan, compare every Decision item with its user-consent evidence; confirm it was accepted before insertion, states the narrowest semantics that preserve the accepted decision, and does not promote implementation detail. Apply the post-implementation and counterfactual tests; confirm any retained mechanism was accepted as load-bearing, scaffold-emitted frontmatter was preserved, and no terminal ADR body was retrofitted.

### `invariant: source-embed-parity`

The repository source population walks `os.DirFS(".")`, includes every file below root template directories while excluding only root Go source and test files, and compares its exact regular-file set bidirectionally with the regular-file set walked from the embedded filesystem root. Failures diagnose sorted paths as missing from embed or unexpected in embed; this proves file-set parity only, not semantic validation.
Origin: ADR-0242
Backing: test

### `invariant: templates-valid-frontmatter`

Every catalog skill and agent template, rendered with representative data, produces leading frontmatter that parses as YAML with a non-empty name and a non-empty description.
Origin: ADR-0006
Backing: test
