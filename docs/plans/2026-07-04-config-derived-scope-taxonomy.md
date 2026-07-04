# Plan: Config-derived commit-scope taxonomy (ADR-0056 + ADR-0057)

**ADRs:** [ADR-0056](../decisions/0056-structured-commit-scope-config-with-meanings.md) (structured scope config with meanings), [ADR-0057](../decisions/0057-sandboxed-placeholder-substitution-in-convention-parts.md) (sandboxed `{{=awf:…}}` part placeholders). Design rationale lives in the ADRs; this plan is execution only.

## Goal

Eliminate the last hand-written commit-scope drift surface. `docs/workflow.md`'s taxonomy table is a raw convention part (`.awf/parts/workflow/commit-discipline.md`) that hand-writes all eight scope tokens with no reflag. End state: `audit.allowedScopes` is the single source for each scope's **name and meaning**, and the workflow.md taxonomy renders from it via `{{=awf:commitScopeTable}}`.

## Architecture summary

- **ADR-0056** widens each `audit.allowedScopes` element to a `{name, meaning}` mapping *or* a bare string, via a `config.ScopeSpec` type with a custom `UnmarshalYAML`. The commit-gate still matches on `Name`; `Meaning` is metadata.
- **ADR-0057** adds a project-layer literal-substitution pass over raw part bodies for a closed `{{=awf:identifier}}` registry of config-derived strings, with unknown/empty-key and residual-token hard errors, and a placeholder-aware confighash reflag.

## Tech stack

- Go 1.26. Packages touched: `internal/config`, `internal/audit`, `internal/project`, `internal/render` (read-only reference), `cmd/awf` (indirect via audit). YAML: `gopkg.in/yaml.v3`.
- Gate: `./x gate` before every commit (100% statement coverage, ADR-0012). New non-test Go must be fully covered.

## File structure

**Modified:**
- `internal/config/config.go` — `AuditConfig.AllowedScopes []string` → `[]ScopeSpec`; add `ScopeSpec` + `UnmarshalYAML`.
- `internal/audit/settings.go` — `Settings.AllowedScopes` carries name+meaning; `Resolve` maps through.
- `internal/audit/audit.go` — scope match reads `.Name`.
- `internal/project/render.go` — `commitScopesDisplay` reads `.Name`; new registry + substitution wired into `planSections`.
- `internal/project/confighash.go` — fold scope data when a consumed part references `{{=awf:commitScope*}}`.
- `.awf/config.yaml` — adopt mapping form with meanings (Task 4).
- `.awf/parts/workflow/commit-discipline.md` — replace hand-written table with `{{=awf:commitScopeTable}}` (Task 4).
- `docs/architecture.md`, `docs/domains/rendering.md` — render-flow note (Task 3).
- `docs/decisions/0056-*.md`, `docs/decisions/0057-*.md`, `docs/decisions/ACTIVE.md` — status flips.

**Created:**
- `internal/config/scopespec.go` (+ `_test.go`) — the `ScopeSpec` type and its parse test.
- `internal/project/placeholders.go` (+ `_test.go`) — the registry and substitution pass.

**Deleted:** none.

---

## Phase 1 — Structured scope config (ADR-0056)

Atomic: `AllowedScopes` changes type, so `config` + `audit` + `project` must compile together in one commit.

- [ ] **1.1 Add the `ScopeSpec` type.** Create `internal/config/scopespec.go`:
  ```go
  package config

  import (
  	"errors"

  	"gopkg.in/yaml.v3"
  )

  // ScopeSpec is one allowed commit scope: a name and an optional human meaning.
  // In config a scope is written either as a bare string (name only) or a
  // {name, meaning} mapping (ADR-0056).
  type ScopeSpec struct {
  	Name    string `yaml:"name"`
  	Meaning string `yaml:"meaning"`
  }

  // UnmarshalYAML accepts either a scalar node (the bare-string form → empty
  // meaning) or a strict mapping node. invariant: scope-config-dual-form
  func (s *ScopeSpec) UnmarshalYAML(n *yaml.Node) error {
  	if n.Kind == yaml.ScalarNode {
  		s.Name, s.Meaning = n.Value, ""
  		return nil
  	}
  	if n.Kind != yaml.MappingNode {
  		return errors.New("scope entry must be a string or a {name, meaning} mapping")
  	}
  	type raw ScopeSpec // avoid recursion; inherit strict decode from the decoder
  	var r raw
  	if err := n.Decode(&r); err != nil {
  		return err
  	}
  	if r.Name == "" {
  		return errors.New("scope mapping requires a non-empty name")
  	}
  	*s = ScopeSpec(r)
  	return nil
  }
  ```
  Note: the parent decoder is `KnownFields(true)` (config load), so `n.Decode(&r)` inherits strict field checking — an unknown key in a scope mapping errors. Confirm this in the test (1.5).

- [ ] **1.2 Retype `AuditConfig.AllowedScopes`.** In `internal/config/config.go` change line ~97 `AllowedScopes []string` → `AllowedScopes []ScopeSpec`. Leave the `yaml:"allowedScopes"` tag.

- [ ] **1.3 Widen audit settings + Resolve.** In `internal/audit/settings.go`: change `Settings.AllowedScopes []string` → `[]config.ScopeSpec`, and in `Resolve` keep `s.AllowedScopes = a.AllowedScopes` (types now match). Add a helper on `Settings`:
  ```go
  // ScopeNames returns just the allowed scope names, for gate matching.
  func (s Settings) ScopeNames() []string {
  	names := make([]string, len(s.AllowedScopes))
  	for i, sc := range s.AllowedScopes {
  		names[i] = sc.Name
  	}
  	return names
  }
  ```

- [ ] **1.4 Gate matches on name.** In `internal/audit/audit.go` line ~143 change the scope check to use names:
  ```go
  if scope := m[3]; scope != "" && len(s.AllowedScopes) > 0 && !containsFold(s.ScopeNames(), scope) {
  ```

- [ ] **1.5 Project display reads `.Name`.** In `internal/project/render.go` `commitScopesDisplay` (line ~62), iterate `.Name`:
  ```go
  scopes := audit.Resolve(p.Cfg.Audit).AllowedScopes
  if len(scopes) == 0 {
  	return ""
  }
  quoted := make([]string, len(scopes))
  for i, s := range scopes {
  	quoted[i] = "`" + s.Name + "`"
  }
  return strings.Join(quoted, ", ")
  ```
  Leave `SkeletonAudit.AllowedScopes []string` in `internal/config/edit.go` unchanged — init keeps emitting bare strings (they re-parse cleanly through `ScopeSpec`).

- [ ] **1.6 Parse test.** Create `internal/config/scopespec_test.go` with `TestScopeSpecDualForm` (carries `// invariant: scope-config-dual-form`): decode a YAML `audit.allowedScopes` list mixing `- adr` and `- {name: rendering, meaning: the render engine}`; assert the bare element → `{Name:"adr", Meaning:""}` and the mapping element → both fields; assert an unknown mapping key (`- {name: x, foo: y}`) errors; assert a mapping missing `name` errors. Use the strict `KnownFields` decoder path the real config load uses.

- [ ] **1.7 Fix existing tests.** Update any test constructing `AllowedScopes`/`Settings.AllowedScopes` as `[]string` (search: `AllowedScopes:` in `internal/audit/*_test.go`, `internal/project/*_test.go`, `internal/config/*_test.go`) to `[]config.ScopeSpec{{Name: "..."}}`. Notably `internal/audit/settings_test.go` (the `[awf]` case) and any drift/render fixtures.

- [ ] **1.8 Flip ADR-0056 and verify.**
  - Set `status: Implemented` in `docs/decisions/0056-structured-commit-scope-config-with-meanings.md`.
  - `./x sync` (regenerates ACTIVE.md), `./x check` (clean), `./x gate` (100%, `scope-config-dual-form` now backed).
  - **Commit** (stage explicitly): `feat(config): structured commit-scope config with meanings`. Include the two config files, the two audit files, render.go, the new test, fixed tests, the ADR, ACTIVE.md, awf.lock.
  - Expected `./x gate` tail: `coverage: 100.0%` and `0 issues.`

---

## Phase 2 — Placeholder substitution mechanism (ADR-0057 core)

- [ ] **2.1 Registry + substitution.** Create `internal/project/placeholders.go`:
  - `func (p *Project) placeholderRegistry() map[string]string` — build the available (non-empty only) key→value map from the resolved config/render context:
    - `commitScopeList` → `p.commitScopesDisplay()` (comma names).
    - `commitScopeTable` → a markdown table built from `audit.Resolve(p.Cfg.Audit).AllowedScopes` (columns `scope` | `use it for`; one row per `ScopeSpec` using `Name` and `Meaning`; empty string if no scopes).
    - `commitScopeSentence` → e.g. `` "The allowed commit scopes are " + list + "." `` or empty when no scopes.
    - `prefix` → `p.Cfg.Prefix`.
    - Gate commands from vars (`gateCmd`, `checkCmd`) where non-empty.
    - Only insert a key when its value is non-empty.
  - `var awfPlaceholderRE = regexp.MustCompile(`\{\{=awf:([A-Za-z][A-Za-z0-9]*)\}\}`)`.
  - `func (p *Project) substitutePlaceholders(partName, body string, reg map[string]string) (string, error)`:
    - Replace each strict match: if `reg[key]` present → value; else → hard error `fmt.Errorf("%s: unknown or empty placeholder {{=awf:%s}}; available: %s", partName, key, availableKeys(reg))` where `availableKeys` returns the sorted registry keys.
    - After replacement, if `strings.Contains(out, "{{=awf")` → hard error `fmt.Errorf("%s: malformed awf placeholder (residual {{=awf); available: %s", partName, availableKeys(reg))`.
    - Fast path: if body has no `{{=awf` substring, return it unchanged (no regex work).

- [ ] **2.2 Wire into `planSections`.** In `internal/project/render.go` (`planSections`, ~line 122) after `b, err := os.ReadFile(...)` and setting `sp.PartBody = string(b)`, run substitution:
  ```go
  body, serr := p.substitutePlaceholders(p.partRel(kind, artifact, s), string(b), p.placeholderRegistry())
  if serr != nil {
  	return nil, serr
  }
  sp.HasPart = true
  sp.PartBody = body
  ```
  Compute the registry once per `planSections` call (hoist `reg := p.placeholderRegistry()` above the loop) to avoid rebuilding per section.

- [ ] **2.3 Tests.** Create `internal/project/placeholders_test.go` (carries `// invariant: part-placeholder-sandboxed`) covering every branch for 100% coverage:
  - no-placeholder body → unchanged (fast path).
  - known non-empty key (`commitScopeList`, `commitScopeTable`) → substituted content present.
  - unknown key (`{{=awf:nope}}`) → error naming key + available list.
  - empty-value key: build a Project with accept-any scopes so `commitScopeTable` is absent from the registry; `{{=awf:commitScopeTable}}` → error.
  - near-miss residuals `{{=awf:}}`, `{{= awf:x}}`, `{{=awf:commit-scope}}` → residual-guard error.
  - `placeholderRegistry` with populated scopes (keys present) and empty scopes (scope keys absent, `prefix` still present).
  - Drive at least one case through a real `planSections`/`Sync` over a scaffold with a part using a placeholder, asserting the rendered output contains the table (integration coverage of the wiring).

---

## Phase 3 — Placeholder-aware confighash reflag (ADR-0057 reflag)

- [ ] **3.1 Fold scope data for placeholder-using parts.** In `internal/project/confighash.go` `artifactConfigHash` (~line 47), after the existing `render.ReferencesScopes(assembled)` block, also scan consumed part bodies. Since the loop at ~line 56 already reads each part body `b`, add: if any part body matches the placeholder scope reference, fold scope data. Concretely, add a helper in `internal/render/vars.go`:
  ```go
  var scopePlaceholderRE = regexp.MustCompile(`\{\{=awf:commitScope[A-Za-z0-9]*\}\}`)

  // ReferencesScopePlaceholder reports whether a raw convention-part body uses a
  // {{=awf:commitScope*}} sandbox placeholder (ADR-0057), so the artifact folds
  // resolved scope data into its config hash.
  func ReferencesScopePlaceholder(body string) bool { return scopePlaceholderRE.MatchString(body) }
  ```
  In `artifactConfigHash`, track a bool while reading parts; if set, `proj["commitScopes"] = audit.Resolve(p.Cfg.Audit).AllowedScopes` (guard against double-set with the template-source branch — set once).

- [ ] **3.2 Drift test.** Add `TestScopesEditReflagsPlaceholderPart` to `internal/project/drift_test.go` (carries `// invariant: part-scopes-in-confighash`), parallel to `TestScopesEditReflagsReferencingArtifacts`:
  - Positive: scaffold an artifact whose part body contains `{{=awf:commitScopeTable}}`; hash; edit `audit.allowedScopes`; assert the artifact's confighash changes (reflagged).
  - Negative: a part without the placeholder does not reflag on a scopes edit.

- [ ] **3.3 Flip ADR-0057, docs, verify.**
  - Set `status: Implemented` in `docs/decisions/0057-*.md`.
  - Update `docs/architecture.md` render-flow note (~lines 88-91) to mention the part-placeholder substitution pass; update the `rendering` domain narrative source (`.awf/domains/parts/rendering/current-state.md`) with a sentence on the `{{=awf:…}}` sandbox and its reflag.
  - `./x sync`, `./x check` (clean), `./x gate` (100%, both ADR-0057 invariants backed).
  - **Commit**: `feat(rendering): sandboxed {{=awf:}} placeholder substitution in parts`. Include placeholders.go(+test), render.go wiring, confighash.go, vars.go, drift test, the ADR, architecture/domain docs, ACTIVE.md, awf.lock.

---

## Phase 4 — awf adopts it (the payoff)

- [ ] **4.1 Structured scopes in awf's config.** In `.awf/config.yaml` replace the bare `audit.allowedScopes` list with the mapping form, meanings from ADR-0055's taxonomy:
  ```yaml
  audit:
    allowedScopes:
      - name: adr
        meaning: ADR markdown documents
      - name: adr-system
        meaning: the ADR machinery code (ACTIVE.md generation, lifecycle)
      - name: awf
        meaning: genuinely cross-cutting / repo-meta work — the umbrella of last resort
      - name: config
        meaning: the .awf config tree, schema, migrations
      - name: invariants
        meaning: invariant backing and checks
      - name: plans
        meaning: plan markdown documents
      - name: rendering
        meaning: the render engine and templates
      - name: tooling
        meaning: CLI, audit/gate, coverage, CI, ./x, changelog, evals
  ```

- [ ] **4.2 Derive the taxonomy table.** In `.awf/parts/workflow/commit-discipline.md` replace the hand-written markdown table (the `| scope | use it for |` block and its 8 rows) with a single line `{{=awf:commitScopeTable}}`. Keep the intro sentence, the "stored once in `audit.allowedScopes`" line, and the domains-convention note (it contains no scope tokens). Verify zero hand-written scope tokens remain in the file.

- [ ] **4.3 Sync, verify, commit.**
  - `./x sync` — `docs/workflow.md` renders the table from config; the placeholder-reflag folds scope data into the workflow.md hash.
  - `grep -n "adr-system" docs/workflow.md` shows the rendered table row (proves derivation).
  - `./x check` (clean), `./x gate` (100%).
  - Sanity: temporarily add a 9th scope to `.awf/config.yaml`, `./x check` must report `docs/workflow.md` (or its artifact) drift/stale until re-synced; revert.
  - **Commit**: `feat(awf): derive workflow.md commit-scope taxonomy from config`. Include `.awf/config.yaml`, the part, `docs/workflow.md`, awf.lock.

---

## Done when

- `audit.allowedScopes` carries name+meaning; the commit-gate is unchanged.
- `docs/workflow.md`'s taxonomy renders from config via `{{=awf:commitScopeTable}}` — no hand-written scope tokens anywhere; a scopes edit reflags it stale.
- Both ADRs Implemented; `./x gate full` and `./x check` green at 100% coverage.
