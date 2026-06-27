# Plan: Domain-Doc Currency Audit Rules (ADR-0019)

Implements [ADR-0019](../decisions/0019-domain-doc-staleness-audit-rule.md). Design and rationale
live there — this plan is the execution record only.

## Goal

Add two advisory `Warning` rules to the `awf audit` engine: `domain-doc-staleness` (an ADR reaches
`Implemented` in a configured domain without its current-state narrative being refreshed) and
`undocumented-domain` (an ADR is tagged with a domain that has no domain doc). Dogfood by refreshing
the `tooling` current-state narrative; the final commit flips ADR-0019 `Accepted → Implemented`.

## Architecture summary

Localized extension of the ADR-0017 rule engine — no engine, catalog-schema, or lock change. Two
pure rule functions appended to `evaluate()`, a `domainsOf` frontmatter helper parallel to
`statusOf`, four new `Inputs` fields, two `*bool` `AuditConfig` toggles resolved by `AuditSettings`,
and `Project.Audit()` supplying the new inputs. Both rules are range-level (like
`rulePlanForLargeChange`), advisory, and emit branch-level findings (empty `Commit`).

Phasing respects per-commit compilation and the 100% coverage gate: Phase 1 lands the audit-package
logic + its unit tests (rules dormant in production until wired, fully covered by unit tests); Phase
2 wires config + `Project.Audit` (signature change + sole caller together); Phase 3 dogfoods + flips.

## Tech stack

- Go 1.26. Packages: `internal/audit`, `internal/config`, `internal/project`. Tests:
  `internal/audit/audit_test.go`, `internal/config/config_test.go`.
- The pre-commit hook runs `./x check` then `./x gate` (full `go test ./...` + 100% coverage + vet +
  lint) on every commit; both must pass. Each phase's code and its covering tests land together.

## File structure

**Modified:**
- `internal/audit/audit.go` (imports, `Inputs`, `evaluate`, two rules + helpers)
- `internal/audit/audit_test.go` (rule unit tests)
- `internal/config/config.go` (`AuditConfig`, `AuditSettings`)
- `internal/config/config_test.go` (three `AuditSettings` call sites + toggle assertions)
- `internal/project/project.go` (`Project.Audit` inputs)
- `.awf/domains/parts/tooling/current-state.md` (dogfood refresh, Phase 3)
- `docs/decisions/0019-domain-doc-staleness-audit-rule.md` (status flip, Phase 3)
- `docs/decisions/ACTIVE.md`, `docs/domains/tooling.md`, `.awf/awf.lock` (regenerated, Phase 3)

**Created / Deleted:** none (besides this plan).

> Stage explicitly per phase (`git add <paths>`); never `git add -A`.

---

## Phase 1 — Audit rule engine (`internal/audit`)

### Task 1.1 — Add the `sort` import

In `internal/audit/audit.go`, replace the import block:

```
import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)
```

with:

```
import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)
```

### Task 1.2 — Add the four `Inputs` fields

In `internal/audit/audit.go`, replace:

```
	ADRDir              string // e.g. "docs/decisions"
	ActiveMd            string // e.g. "docs/decisions/ACTIVE.md"
	PlansDir            string // e.g. "docs/plans"
}
```

with:

```
	ADRDir              string // e.g. "docs/decisions"
	ActiveMd            string // e.g. "docs/decisions/ACTIVE.md"
	PlansDir            string // e.g. "docs/plans"
	ConfiguredDomains   []string // config.Domains; staleness limited to these, undocumented-domain fires outside them
	DomainsPartsDir     string   // e.g. ".awf/domains/parts"
	DomainDocStaleness  bool     // run the domain-doc-staleness rule
	UndocumentedDomain  bool     // run the undocumented-domain rule
}
```

### Task 1.3 — Wire the two rules into `evaluate`

In `internal/audit/audit.go`, replace:

```
	out = append(out, ruleDependencyADR(commits, in)...)
	out = append(out, rulePlanForLargeChange(commits, in)...)
	return out
}
```

with:

```
	out = append(out, ruleDependencyADR(commits, in)...)
	out = append(out, rulePlanForLargeChange(commits, in)...)
	out = append(out, ruleDomainDocStaleness(commits, in)...)
	out = append(out, ruleUndocumentedDomain(commits, in)...)
	return out
}
```

### Task 1.4 — Add the two rule functions and helpers

In `internal/audit/audit.go`, immediately before `func finding(`, insert:

```
// invariant: audit-domain-doc-staleness
func ruleDomainDocStaleness(commits []Commit, in Inputs) []Finding {
	if !in.DomainDocStaleness {
		return nil
	}
	refreshed := map[string]bool{} // domains whose source narrative changed in range
	flagged := map[string]bool{}   // configured domains brought to Implemented in range
	for _, c := range commits {
		for _, ch := range c.Changes {
			if d, ok := domainOfPart(ch.Path, in.DomainsPartsDir); ok {
				refreshed[d] = true
			}
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
				continue
			}
			if statusOf(ch.NewText) != "Implemented" {
				continue
			}
			if ch.Action != Added && statusOf(ch.OldText) == "Implemented" {
				continue // already Implemented before this change; not a new transition
			}
			for _, d := range domainsOf(ch.NewText) {
				if contains(in.ConfiguredDomains, d) {
					flagged[d] = true
				}
			}
		}
	}
	var out []Finding
	for _, d := range sortedSet(flagged) {
		if !refreshed[d] {
			out = append(out, Finding{Severity: Warning, Rule: "domain-doc-staleness",
				Detail: fmt.Sprintf("an ADR in domain %q reached Implemented but %s/%s/current-state.md was not refreshed in this range", d, in.DomainsPartsDir, d)})
		}
	}
	return out
}

// invariant: audit-undocumented-domain
func ruleUndocumentedDomain(commits []Commit, in Inputs) []Finding {
	if !in.UndocumentedDomain || len(in.ConfiguredDomains) == 0 {
		return nil
	}
	flagged := map[string]bool{}
	for _, c := range commits {
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
				continue
			}
			for _, d := range domainsOf(ch.NewText) {
				if !contains(in.ConfiguredDomains, d) {
					flagged[d] = true
				}
			}
		}
	}
	var out []Finding
	for _, d := range sortedSet(flagged) {
		out = append(out, Finding{Severity: Warning, Rule: "undocumented-domain",
			Detail: fmt.Sprintf("an ADR is tagged with domain %q, which has no domain doc — add it to config.Domains and author its current-state narrative, or drop the tag", d)})
	}
	return out
}

func domainsOf(text string) []string {
	var meta struct {
		Domains []string `yaml:"domains"`
	}
	if _, found, err := frontmatter.Parse([]byte(text), &meta); err != nil || !found {
		return nil
	}
	return meta.Domains
}

func domainOfPart(path, partsDir string) (string, bool) {
	const suffix = "/current-state.md"
	rest, ok := strings.CutPrefix(path, partsDir+"/")
	if !ok || !strings.HasSuffix(rest, suffix) {
		return "", false
	}
	domain := strings.TrimSuffix(rest, suffix)
	if domain == "" || strings.Contains(domain, "/") {
		return "", false
	}
	return domain, true
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
```

### Task 1.5 — Add unit tests covering both rules to 100%

In `internal/audit/audit_test.go`, append:

```
func adrChange(action Action, status string, domains string) FileChange {
	txt := "---\nstatus: " + status + "\ndomains: [" + domains + "]\n---\nbody\n"
	return FileChange{Path: "docs/decisions/0099-x.md", Action: action, NewText: txt}
}

func TestRuleDomainDocStalenessDisabled(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling"}, DomainsPartsDir: ".awf/domains/parts"}
	if f := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{adrChange(Added, "Implemented", "tooling")}}}, in); f != nil {
		t.Errorf("disabled rule returned %v", f)
	}
}

func TestRuleDomainDocStaleness(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling", "rendering"}, DomainsPartsDir: ".awf/domains/parts", DomainDocStaleness: true}
	partChange := func(p string) FileChange { return FileChange{Path: p, Action: Modified} }

	// Implemented in a configured domain, narrative NOT refreshed -> 1 warning.
	got := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{adrChange(Added, "Implemented", "tooling")}}}, in)
	if len(got) != 1 || got[0].Rule != "domain-doc-staleness" || got[0].Commit != "" {
		t.Fatalf("want 1 branch-level warning, got %v", got)
	}

	// Narrative refreshed in range -> 0. Also exercises domainOfPart valid + invalid-suffix + non-part paths.
	clean := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{
		adrChange(Modified, "Implemented", "tooling"),
		partChange(".awf/domains/parts/tooling/current-state.md"),
		partChange(".awf/domains/parts/tooling/notes.md"),    // under partsDir, wrong file -> not a domain
		partChange(".awf/domains/parts/a/b/current-state.md"), // nested -> rejected
	}}}, in)
	if len(clean) != 0 {
		t.Fatalf("refreshed narrative should be clean, got %v", clean)
	}

	// status only Accepted -> 0; unconfigured domain -> 0 (Rule 2's job); no domains -> 0; already Implemented -> 0.
	for _, ch := range []FileChange{
		adrChange(Added, "Accepted", "tooling"),
		adrChange(Added, "Implemented", "ghost"),
		{Path: "docs/decisions/0099-x.md", Action: Added, NewText: "---\nstatus: Implemented\n---\n"},
		{Path: "docs/decisions/0099-x.md", Action: Modified, OldText: "---\nstatus: Implemented\ndomains: [tooling]\n---\n", NewText: "---\nstatus: Implemented\ndomains: [tooling]\n---\nedited\n"},
		{Path: "docs/decisions/0099-x.md", Action: Deleted},
		{Path: "README.md", Action: Modified}, // not an ADR file
	} {
		if f := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{ch}}}, in); len(f) != 0 {
			t.Errorf("change %+v should be clean, got %v", ch, f)
		}
	}

	// Multi-domain [tooling, rendering], only tooling refreshed -> 1 warning (rendering).
	multi := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{
		adrChange(Added, "Implemented", "tooling, rendering"),
		partChange(".awf/domains/parts/tooling/current-state.md"),
	}}}, in)
	if len(multi) != 1 || multi[0].Detail == "" {
		t.Fatalf("want 1 warning for rendering, got %v", multi)
	}

	// Empty ConfiguredDomains -> inert.
	if f := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{adrChange(Added, "Implemented", "tooling")}}},
		Inputs{ADRDir: "docs/decisions", DomainsPartsDir: ".awf/domains/parts", DomainDocStaleness: true}); len(f) != 0 {
		t.Errorf("no configured domains should be inert, got %v", f)
	}
}

func TestRuleUndocumentedDomain(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling"}, UndocumentedDomain: true}

	// Disabled.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "ghost")}}},
		Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling"}}); f != nil {
		t.Errorf("disabled rule returned %v", f)
	}
	// No configured domains -> inert.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "ghost")}}},
		Inputs{ADRDir: "docs/decisions", UndocumentedDomain: true}); f != nil {
		t.Errorf("no configured domains returned %v", f)
	}
	// ADR tags an unconfigured domain -> 1 warning.
	got := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "ghost")}}}, in)
	if len(got) != 1 || got[0].Rule != "undocumented-domain" {
		t.Fatalf("want 1 warning, got %v", got)
	}
	// Configured domain / no domains / deleted / multi-domain partial.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Modified, "Accepted", "tooling")}}}, in); len(f) != 0 {
		t.Errorf("configured domain should be clean, got %v", f)
	}
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{{Path: "docs/decisions/0099-x.md", Action: Deleted}}}}, in); len(f) != 0 {
		t.Errorf("deleted ADR should be clean, got %v", f)
	}
	// ADR file with no parseable frontmatter -> domainsOf hits its not-found branch -> 0.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{{Path: "docs/decisions/0099-x.md", Action: Added, NewText: "# no frontmatter"}}}}, in); len(f) != 0 {
		t.Errorf("frontmatter-less ADR should be clean, got %v", f)
	}
	multi := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "tooling, ghost")}}}, in)
	if len(multi) != 1 {
		t.Fatalf("want 1 warning for ghost, got %v", multi)
	}
}
```

### Task 1.6 — Verify and commit Phase 1

Run:

```
./x check
./x gate
```

Expected: `./x check` clean (no rendered files changed); `./x gate` passes — full suite green,
**100.0% coverage**, 0 lint issues. If any audit.go line is
reported uncovered, add the covering case to the tests above before committing (do not adjust the
rule logic to chase coverage).

Stage and commit:

```
git add internal/audit/audit.go internal/audit/audit_test.go
git commit -m "feat(awf): add domain-doc currency audit rules (ADR-0019)"
```

---

## Phase 2 — Config toggles and `Project.Audit` wiring

### Task 2.1 — Add the two `AuditConfig` toggles

In `internal/config/config.go`, replace:

```
	DependencyManifests []string `yaml:"dependencyManifests"`
	DiffThreshold       *int     `yaml:"diffThreshold"`
}
```

with:

```
	DependencyManifests []string `yaml:"dependencyManifests"`
	DiffThreshold       *int     `yaml:"diffThreshold"`
	DomainDocStaleness  *bool    `yaml:"domainDocStaleness"`
	UndocumentedDomain  *bool    `yaml:"undocumentedDomain"`
}
```

### Task 2.2 — Resolve the toggles in `AuditSettings`

In `internal/config/config.go`, replace the signature line:

```
func (c *Config) AuditSettings() (baseBranch string, allowedTypes, allowedScopes, dependencyManifests []string, subjectMax, diffThreshold int) {
```

with:

```
func (c *Config) AuditSettings() (baseBranch string, allowedTypes, allowedScopes, dependencyManifests []string, subjectMax, diffThreshold int, domainDocStaleness, undocumentedDomain bool) {
```

Then replace:

```
	subjectMax, diffThreshold = 72, 400
	if a == nil {
		return
	}
```

with:

```
	subjectMax, diffThreshold = 72, 400
	domainDocStaleness, undocumentedDomain = true, true
	if a == nil {
		return
	}
```

Then replace:

```
	if a.DiffThreshold != nil {
		diffThreshold = *a.DiffThreshold
	}
	return
}
```

with:

```
	if a.DiffThreshold != nil {
		diffThreshold = *a.DiffThreshold
	}
	if a.DomainDocStaleness != nil {
		domainDocStaleness = *a.DomainDocStaleness
	}
	if a.UndocumentedDomain != nil {
		undocumentedDomain = *a.UndocumentedDomain
	}
	return
}
```

### Task 2.3 — Update the three `config_test` call sites

In `internal/config/config_test.go`:

Replace the `TestAuditSettingsDefaultsWhenNil` call + add a toggle assertion:

```
	base, types, scopes, manifests, max, thr := c.AuditSettings()
	if base != "main" {
```

with:

```
	base, types, scopes, manifests, max, thr, domStale, undoc := c.AuditSettings()
	if !domStale || !undoc {
		t.Errorf("toggles default to off: domStale=%v undoc=%v", domStale, undoc)
	}
	if base != "main" {
```

Replace, in `TestAuditSettingsZeroAuditFallsBackToDefaults`:

```
	base, types, scopes, manifests, max, thr := c.AuditSettings()
	if base != "main" || !containsStr(types, "feat") || scopes != nil ||
```

with:

```
	base, types, scopes, manifests, max, thr, domStale, undoc := c.AuditSettings()
	if !domStale || !undoc {
		t.Errorf("empty AuditConfig should keep toggles on: %v %v", domStale, undoc)
	}
	if base != "main" || !containsStr(types, "feat") || scopes != nil ||
```

In `TestAuditSettingsExplicitOverrides`, add the two toggles to the literal — replace:

```
		DiffThreshold:       intPtr(0),
	}}
	base, types, scopes, manifests, max, thr := c.AuditSettings()
```

with:

```
		DiffThreshold:       intPtr(0),
		DomainDocStaleness:  boolPtr(false),
		UndocumentedDomain:  boolPtr(false),
	}}
	base, types, scopes, manifests, max, thr, domStale, undoc := c.AuditSettings()
	if domStale || undoc {
		t.Errorf("explicit false toggles not honored: domStale=%v undoc=%v", domStale, undoc)
	}
```

If a `boolPtr` helper does not already exist in `internal/config/config_test.go`, add it near
`intPtr`:

```
func boolPtr(b bool) *bool { return &b }
```

(Verify with `grep -n "func boolPtr" internal/config/config_test.go` before adding; add only if absent.)

### Task 2.4 — Supply the new inputs from `Project.Audit`

In `internal/project/project.go`, replace:

```
	base, types, scopes, manifests, subjectMax, threshold := p.Cfg.AuditSettings()
```

with:

```
	base, types, scopes, manifests, subjectMax, threshold, domStale, undoc := p.Cfg.AuditSettings()
```

Then replace:

```
		ADRDir:              lay["adrDir"].(string),
		ActiveMd:            lay["activeMd"].(string),
		PlansDir:            lay["plansDir"].(string),
	})
```

with:

```
		ADRDir:              lay["adrDir"].(string),
		ActiveMd:            lay["activeMd"].(string),
		PlansDir:            lay["plansDir"].(string),
		ConfiguredDomains:   p.Cfg.Domains,
		DomainsPartsDir:     ".awf/domains/parts",
		DomainDocStaleness:  domStale,
		UndocumentedDomain:  undoc,
	})
```

### Task 2.5 — Verify and commit Phase 2

Run:

```
./x gate
./x check
```

Expected: gate passes at 100% coverage, 0 lint; `./x check` clean (no rendered files changed).

Stage and commit:

```
git add internal/config/config.go internal/config/config_test.go internal/project/project.go
git commit -m "feat(awf): wire domain-doc currency rules into config and audit"
```

---

## Phase 3 — Dogfood the rule and mark ADR-0019 Implemented

### Task 3.1 — Refresh the `tooling` current-state narrative

In `.awf/domains/parts/tooling/current-state.md`, replace:

```
awf is positioned as a tool-agnostic renderer (ADR-0016): adapter output paths (skills, agents, the `CLAUDE.md` bridge) come from a named `Target` rather than `.claude/` literals, with `claudeTarget` the sole built-in. `awf init` pre-flights every path it would write and aborts on a collision with a pre-existing, non-managed file unless `--force` is passed.
```

with:

```
awf is positioned as a tool-agnostic renderer (ADR-0016): adapter output paths (skills, agents, the `CLAUDE.md` bridge) come from a named `Target` rather than `.claude/` literals, with `claudeTarget` the sole built-in. `awf init` pre-flights every path it would write and aborts on a collision with a pre-existing, non-managed file unless `--force` is passed.

`awf audit` (ADR-0017) reports advisory workflow-conformance findings over a branch's git history, wired into no gate. Its rules cover Conventional-Commits, ADR-status/ACTIVE.md co-change, dependency-without-ADR, large-change-without-plan, and — for domain-doc currency (ADR-0019) — `domain-doc-staleness` (an ADR reaching Implemented in a configured domain without its current-state narrative refreshed) and `undocumented-domain` (an ADR tagged with a domain that has no domain doc). Each rule is independently disable-able via `audit` config.
```

### Task 3.2 — Flip ADR-0019 to Implemented

In `docs/decisions/0019-domain-doc-staleness-audit-rule.md`, change:

```
status: Accepted
```

to:

```
status: Implemented
```

Both tagged slugs (`audit-domain-doc-staleness`, `audit-undocumented-domain`) are backed by the
`// invariant:` comments added in Phase 1, so `awf check`/`awf invariants` stay clean once Implemented.

### Task 3.3 — Verify and commit Phase 3

Run:

```
./x sync
./x check
./x gate
./x invariants
```

Expected: `./x sync` regenerates `docs/decisions/ACTIVE.md` (0019 → Implemented) and
`docs/domains/tooling.md` (the narrative now mentions the audit rules); `./x check` clean; `./x
gate` passes; `./x invariants` reports the two new slugs backed.

Confirm:

```
grep -n "Implemented" docs/decisions/ACTIVE.md | grep 0019
grep -c "domain-doc-staleness\|undocumented-domain" docs/domains/tooling.md
```

Expected: ADR-0019 under Implemented; the tooling domain doc mentions both rule names.

Stage and commit:

```
git add .awf/domains/parts/tooling/current-state.md docs/decisions/0019-domain-doc-staleness-audit-rule.md docs/decisions/ACTIVE.md docs/domains/tooling.md .awf/awf.lock
git commit -m "feat(awf): mark ADR-0019 Implemented; dogfood the staleness rule"
```

---

## Verification (whole plan)

```
./x gate          # 100% coverage, 0 lint
./x check         # no drift
./x invariants    # audit-domain-doc-staleness + audit-undocumented-domain backed
./x audit         # clean (no-op on main: empty range)
```

## Terminal step

The ADR flip lands in Phase 3 (final commit), so no separate lifecycle commit is needed. Invoke
`awf-reviewing-impl` against the Phase 1–3 commit range.

## Notes

- ADR-driven plan: the `Accepted → Implemented` flip is the final-commit action (Task 3.2); no
  `# Implementation complete` freeze header is used.
- `AuditSettings` extends its return tuple from six to eight values to resolve the two toggles in one
  place (consistent with the existing resolver style); the sole caller (`Project.Audit`) and the
  three `config_test` call sites update in the same phase so each commit compiles.
- The new rules are dormant in production after Phase 1 (their `Inputs` toggles default to the zero
  value `false` until Phase 2 wires `Project.Audit`); Phase 1's unit tests still exercise them to
  100%.
- Scope: both rules are advisory and a no-op on awf's own `main` (empty range), per ADR-0017.
