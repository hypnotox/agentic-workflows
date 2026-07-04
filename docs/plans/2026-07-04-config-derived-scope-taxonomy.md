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
- `internal/render/vars.go` — add `ReferencesScopePlaceholder` + `scopePlaceholderRE` (Task 3.1).
- `.awf/config.yaml` — adopt mapping form with meanings (Task 4).
- `.awf/parts/workflow/commit-discipline.md` — replace hand-written table with `{{=awf:commitScopeTable}}` (Task 4).
- `docs/architecture.md` — render-flow note gains the pre-`Assemble` `{{=awf:…}}` substitution step (Task 3.3).
- `.awf/domains/parts/rendering/current-state.md` — rendering-domain narrative **source**; add the `{{=awf:…}}` sandbox + reflag sentence (Task 3.3). Do **not** hand-edit the rendered `docs/domains/rendering.md`.
- `docs/domains/{config,rendering,tooling}.md` — regenerated (not hand-edited) by `./x sync` on the ADR status flips: 0056 flip regenerates all three (`domains: [config, rendering, tooling]`, Task 1.8); 0057 flip regenerates `config.md`/`rendering.md` (`domains: [rendering, config]`, Task 3.3).
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
  	"fmt"

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
  	// Strictness is NOT inherited: yaml.v3 Node.Decode spins up a fresh
  	// non-strict decoder, so the parent's KnownFields(true) does not apply here.
  	// Enforce the closed key set explicitly (a mapping node's Content is a flat
  	// key,value,key,value… list).
  	for i := 0; i+1 < len(n.Content); i += 2 {
  		if k := n.Content[i].Value; k != "name" && k != "meaning" {
  			return fmt.Errorf("scope mapping has unknown key %q (allowed: name, meaning)", k)
  		}
  	}
  	type raw ScopeSpec // avoid recursion
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
  Note: **`n.Decode(&r)` does *not* inherit the config loader's `KnownFields(true)`** — yaml.v3's `Node.Decode` creates a fresh decoder with strictness off (verified: an unknown key is silently dropped, not errored). The unknown-key check above is therefore explicit and is what backs the "unknown scope key errors" assertion in test 1.6 — do not rely on inherited strictness.

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

- [ ] **1.6 Parse test.** Create `internal/config/scopespec_test.go` with `TestScopeSpecDualForm` (carries `// invariant: scope-config-dual-form`): decode a YAML `audit.allowedScopes` list mixing `- adr` and `- {name: rendering, meaning: the render engine}`; assert the bare element → `{Name:"adr", Meaning:""}` and the mapping element → both fields; assert an unknown mapping key (`- {name: x, foo: y}`) errors (this error comes from `ScopeSpec.UnmarshalYAML`'s explicit key check, **not** from the loader's `KnownFields` — which does not reach into `Node.Decode`); assert a mapping missing `name` errors. Decode through the same strict config-load path (`yaml.NewDecoder` + `KnownFields(true)`) so the test mirrors reality.

- [ ] **1.7 Fix existing tests.** Grep `AllowedScopes` (no trailing colon — a colon-only search misses field *reads*) across `internal/audit`, `internal/project`, `internal/config`. Two concrete sites confirmed by inspection, both in `package audit`:
  - `internal/audit/settings_test.go`: the construction `AllowedScopes: []string{"awf"}` (line ~51) → `[]config.ScopeSpec{{Name: "awf"}}`, **and** the read `s.AllowedScopes[0] != "awf"` (line ~68) → `s.AllowedScopes[0].Name != "awf"`. The `s.AllowedScopes != nil` nil-checks (lines ~24/40) compile unchanged.
  - `internal/audit/audit_test.go`: `AllowedScopes: []string{"awf"}` (line ~32) → `[]config.ScopeSpec{{Name: "awf"}}`; add the `internal/config` import (this file does not import it yet).
  The init-path tests (`cmd/awf/init_test.go`, `internal/initspec/*`) and YAML-string drift/spine fixtures need no change — `SkeletonAudit.AllowedScopes` stays `[]string` and bare-string YAML re-parses cleanly through `ScopeSpec`. A `./x gate` compile failure will surface any site the grep misses.

- [ ] **1.8 Flip ADR-0056 and verify.**
  - Set `status: Implemented` in `docs/decisions/0056-structured-commit-scope-config-with-meanings.md`.
  - `./x sync` (regenerates ACTIVE.md), `./x check` (clean), `./x gate` (100%, `scope-config-dual-form` now backed).
  - **Commit** (stage explicitly): `feat(config): support structured commit-scope entries with meanings`. Include the two config files, the two audit files, render.go, the new test, fixed tests, the ADR, ACTIVE.md, **the regenerated domain index docs `docs/domains/config.md`, `docs/domains/rendering.md`, `docs/domains/tooling.md`** (0056 carries `domains: [config, rendering, tooling]`, so the Proposed→Implemented flip changes each domain's ADR-status index — ADR-0033 co-change), awf.lock.
  - Note: retyping `commitScopes` to `[]ScopeSpec` changes the *marshalled* shape folded into the confighash of every `.commitScopes`-referencing artifact (guide + reviewing skills), so `./x sync` will churn many `awf.lock` hash lines even though the rendered *content* is unchanged (`commitScopesDisplay` still emits names only). This churn is expected; stage the whole `awf.lock`.
  - Expected `./x gate` tail: `coverage: 100.0%` and `0 issues.`

---

## Phase 2 — Placeholder substitution mechanism (ADR-0057 core)

> **Phase 2 is not committed on its own.** It has no commit/flip step: its files (placeholders.go, the render.go wiring, its tests) land together with Phase 3 in the single `feat(rendering)` commit at 3.3 (the mechanism and its confighash reflag are one concern). Run `./x gate` at the end of Phase 3, not here. Do not create a partial commit after 2.3.

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
    - After replacement, run the **residual leak guard** with a *loose* regex, not a literal substring. Define a package var whose pattern is the Go raw-string literal `\{\{=\s*awf` (i.e. `awfResidualRE := regexp.MustCompile` of that pattern); if `awfResidualRE.MatchString(out)` → hard error `fmt.Errorf("%s: malformed awf placeholder (residual %s); available: %s", partName, awfResidualRE.FindString(out), availableKeys(reg))`. **A literal `strings.Contains(out, "{{=awf")` is WRONG** — it misses the space near-miss `{{= awf:x}}` (which ADR-0057 item 5 and test 2.3 require catching): `{{= awf` does not contain the substring `{{=awf`. The `\s*` in the pattern bridges the space.
    - Fast path: if body has no `{{=` substring, return it unchanged (no regex work). The fast-path sentinel is `{{=` (three chars), **not** `{{=awf` — otherwise `{{= awf:x}}` takes the fast path and never reaches the residual guard, rendering verbatim. A bare `{{=` that is not an awf placeholder (e.g. Mustache set-delimiter prose) falls through: the strict regex won't substitute it and the loose residual pattern (`awf` required after `{{=\s*`) won't match it, so it passes unchanged.

- [ ] **2.2 Wire into `planSections`.** In `internal/project/render.go` (`planSections`, ~line 122). The substitution must run **inside the existing `if err == nil` branch** (the branch that sets `HasPart`/`PartBody`) — a missing part (`os.ErrNotExist`) must stay `HasPart:false`, so do **not** move `sp.HasPart = true` outside the read-success branch. Replace the body of that branch:
  ```go
  b, err := os.ReadFile(p.Cfg.PartPath(kind, artifact, s))
  if err == nil {
  	body, serr := p.substitutePlaceholders(p.partRel(kind, artifact, s), string(b), reg)
  	if serr != nil {
  		return nil, serr
  	}
  	sp.HasPart = true
  	sp.PartBody = body
  } else if !errors.Is(err, os.ErrNotExist) {
  	return nil, fmt.Errorf("read part %s/%s/%s: %w", kind, artifact, s, err)
  }
  ```
  Hoist `reg := p.placeholderRegistry()` above the `for` loop (compute once per `planSections` call, not per section); the snippet uses that `reg`.

- [ ] **2.3 Tests.** Create `internal/project/placeholders_test.go` (carries `// invariant: part-placeholder-sandboxed`) covering every branch for 100% coverage:
  - no-placeholder body → unchanged (fast path).
  - known non-empty key (`commitScopeList`, `commitScopeTable`) → substituted content present.
  - unknown key (`{{=awf:nope}}`) → error naming key + available list.
  - empty-value key: build a Project with accept-any scopes so `commitScopeTable` is absent from the registry; `{{=awf:commitScopeTable}}` → error.
  - near-miss residuals `{{=awf:}}`, `{{= awf:x}}`, `{{=awf:commit-scope}}` → residual-guard error.
  - `placeholderRegistry` with populated scopes (keys present) and empty scopes (scope keys absent, `prefix` still present).
  - **Every conditional insert must fire once for the 100% statement gate.** Each `if value != "" { reg[key] = value }` line is an executable statement that is only covered when its value is non-empty. Build at least one Project fixture whose registry populates **every** key — `commitScopeList`, `commitScopeTable`, `commitScopeSentence` (populated scopes), `prefix` (non-empty prefix), and `gateCmd`/`checkCmd` (set `vars.gateCmd`/`vars.checkCmd` non-empty) — otherwise the never-inserted keys leave dead statements and `./x gate` fails below 100%.
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
  - **Commit**: `feat(rendering): add sandboxed {{=awf:}} placeholder substitution in parts`. Include placeholders.go(+test), render.go wiring, confighash.go, vars.go, drift test, the ADR, `docs/architecture.md`, the domain source `.awf/domains/parts/rendering/current-state.md`, **the regenerated domain index docs `docs/domains/rendering.md` and `docs/domains/config.md`** (0057 carries `domains: [rendering, config]` — the flip changes both indexes, ADR-0033), ACTIVE.md, awf.lock. (This single commit also carries all of Phase 2's work — see the Phase 2 note.)

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

- [ ] **4.2 Derive the taxonomy table and de-tokenise the prose.** In `.awf/parts/workflow/commit-discipline.md`:
  - Replace the hand-written markdown table (the `| scope | use it for |` block and its 8 rows) with a single line `{{=awf:commitScopeTable}}`.
  - Keep the intro sentence and the "stored once in `audit.allowedScopes`" line.
  - **Reword the domains-convention paragraph so it enumerates no scope tokens** (ADR-0055's list is now in the derived table above it). The paragraph currently hand-writes six: the domain-mirror list (`` `adr-system` ``, `` `config` ``, `` `invariants` ``, `` `rendering` ``, `` `tooling` ``) and the `` `adr` ``/`` `adr-system` `` wrong-but-valid example. Rewrite to: "The code scopes mirror the domain vocabulary in `.awf/config.yaml` — see [the domain docs](domains) for what each area covers. The correspondence is hand-maintained, not machine-enforced (ADR-0055): adding a domain does not add a scope. The gate only checks set membership; it cannot catch a wrong-but-valid pick (a docs scope where a code scope was meant), so pick the scope that names the area you actually changed."
  - Verify: `grep -nE '\x60(adr|adr-system|awf|config|invariants|plans|rendering|tooling)\x60' .awf/parts/workflow/commit-discipline.md` returns nothing (zero hand-written scope tokens remain).

- [ ] **4.3 Sync, verify, commit.**
  - `./x sync` — `docs/workflow.md` renders the table from config; the placeholder-reflag folds scope data into the workflow.md hash.
  - `grep -n "adr-system" docs/workflow.md` shows the rendered table row (proves derivation).
  - `./x check` (clean), `./x gate` (100%).
  - Sanity: temporarily add a 9th scope to `.awf/config.yaml`, `./x check` must report `docs/workflow.md` (or its artifact) drift/stale until re-synced; revert.
  - **Commit**: `feat(config): derive workflow.md commit-scope taxonomy from config`. Include `.awf/config.yaml`, the part, `docs/workflow.md`, awf.lock.

---

## Done when

- `audit.allowedScopes` carries name+meaning; the commit-gate is unchanged.
- `docs/workflow.md`'s taxonomy renders from config via `{{=awf:commitScopeTable}}` — no hand-written scope tokens anywhere; a scopes edit reflags it stale.
- Both ADRs Implemented; `./x gate full` and `./x check` green at 100% coverage.
