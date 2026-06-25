# Plan: Language-Agnostic Invariant Backing and a Polyglot Standard

Implements **[ADR-0008](../decisions/0008-language-agnostic-invariant-backing.md)** (Accepted).
Design rationale lives in the ADR — this plan is the execution record.

## Goal

Make invariant backing language-configurable and remove the standard's remaining Go-isms: add an
`invariants` config block (`[]{globs, marker}`, literal marker, basename globs); generalize the
`internal/invariants` scanner to three states (unchecked / disabled / enforced); status-aware,
non-Go-specific CLI messages; de-Go-ify the `refactor-coupling-audit` skill and the invariant
marker prose; reposition the identity as a generic polyglot tool.

## Architecture summary

- `internal/config`: new `InvariantConfig{Disabled, Sources}` + `InvariantSource{Globs, Marker}`; `Config.Invariants *InvariantConfig`; `Validate` rejects path-separator / malformed globs.
- `internal/invariants`: `Check(decisionsDir, root, cfg)` → `Finding{Slug, ADR, Status}` (Unbacked|Unchecked); `scanTags` matches basenames against per-source globs and scans for `<marker> invariant: <slug>` (literal marker, whitespace-tolerant).
- `internal/project.CheckInvariants` passes `p.Cfg.Invariants`; `cmd/awf` printers become status-aware.
- Dogfood: `.claude/awf.yaml` gains `invariants.sources: [{globs:["*.go"], marker:"//"}]`.
- De-Go-ify: `refactor-coupling-audit` + `proposing-adr` skills, `docs/decisions/template.md`, `agentsDoc.data.identity` + the "Backed invariants" bullet, `README.md`.

## Tech stack

- Go 1.26; packages: `internal/config`, `internal/invariants`, `internal/project`, `cmd/awf`. No new deps (`path/filepath` stdlib).
- Gate: `./x gate` per code commit; pre-commit also runs `./x check` (which runs the invariant check).

## File structure

**Modified:** `internal/config/config.go` (+types, field, Validate, `path/filepath` import), `internal/config/config_test.go` (glob-validation test), `internal/invariants/invariants.go` (scanner rewrite + package doc), `internal/invariants/invariants_test.go` (state/marker/glob tests backing the 6 new 0008 slugs, plus the retained 0007 slug tests updated to the 3-arg signature), `internal/project/project.go` (CheckInvariants signature), `cmd/awf/check.go` + `cmd/awf/invariants.go` (status-aware printers), `cmd/awf/invariants_test.go` (add an `invariants` block so the three-state scanner reaches enforced), `.claude/awf.yaml` (invariants block + identity + backed-invariants bullet), `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`, `templates/skills/proposing-adr/SKILL.md.tmpl`, `docs/decisions/template.md`, `README.md`, plus re-synced `.claude/**` + `AGENTS.md`, `docs/decisions/0008-*.md` (Implemented flip).

**ADR-0008 slug → backing test (authoritative):**

| slug | backing test (file) |
|---|---|
| `invariants-three-state` | `TestCheckThreeState` (invariants_test.go) |
| `invariants-multilang-scan` | `TestCheckMultiLangScan` (invariants_test.go) |
| `invariants-marker-literal` | `TestCheckMarkerLiteral` (invariants_test.go) |
| `invariants-marker-whitespace` | `TestCheckMarkerWhitespace` (invariants_test.go) |
| `invariants-glob-basename` | `TestInvariantGlobValidation` (config_test.go) |
| `invariants-zero-slugs-clean` | `TestCheckZeroSlugsClean` (invariants_test.go) |

---

## Phase 1 — Config schema + dogfood block

### Task 1.1 — Add config types, field, validation

- [ ] In `internal/config/config.go`, add `"path/filepath"` to the import block (after `"os"`).
- [ ] Add the field to `Config` (after the `Docs` line):

```go
	Docs       map[string]SkillConfig `yaml:"docs"`
	Invariants *InvariantConfig       `yaml:"invariants"`
	raw        []byte
```

- [ ] Add the types after `SkillConfig`:

```go
// InvariantConfig configures language-agnostic invariant backing. A nil
// *InvariantConfig (key absent) means "unchecked"; Disabled is the explicit
// opt-out; a non-empty Sources enables enforcement.
type InvariantConfig struct {
	Disabled bool              `yaml:"disabled"`
	Sources  []InvariantSource `yaml:"sources"`
}

// InvariantSource pairs filename globs (matched against a file's basename) with
// the literal comment marker that prefixes a backing `invariant: <slug>` tag.
type InvariantSource struct {
	Globs  []string `yaml:"globs"`
	Marker string   `yaml:"marker"`
}
```

- [ ] In `Validate`, before the final `return nil`, add:

```go
	if c.Invariants != nil {
		for _, src := range c.Invariants.Sources {
			for _, g := range src.Globs {
				if strings.Contains(g, "/") {
					return fmt.Errorf("invariants glob %q must be a filename pattern, not a path", g)
				}
				if _, err := filepath.Match(g, "x"); err != nil {
					return fmt.Errorf("invariants glob %q is malformed: %w", g, err)
				}
			}
		}
	}
```

### Task 1.2 — Config validation test (backs `invariants-glob-basename`)

- [ ] Append to `internal/config/config_test.go`:

```go
// invariant: invariants-glob-basename
func TestInvariantGlobValidation(t *testing.T) {
	ok := &Config{Prefix: "x", DocsDir: "docs", Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"*.go", "*_test.py"}, Marker: "//"}},
	}}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid basename globs rejected: %v", err)
	}
	pathGlob := &Config{Prefix: "x", DocsDir: "docs", Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"**/*.go"}, Marker: "//"}},
	}}
	if err := pathGlob.Validate(); err == nil {
		t.Error("expected path-separator glob to be rejected")
	}
	bad := &Config{Prefix: "x", DocsDir: "docs", Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"[", "*.go"}, Marker: "//"}},
	}}
	if err := bad.Validate(); err == nil {
		t.Error("expected malformed glob to be rejected")
	}
}
```

- [ ] Verify: `go test ./internal/config/` → `ok`.

### Task 1.3 — Dogfood: add the `invariants` block to this repo's config

- [ ] In `.claude/awf.yaml`, insert after the `prefix: awf` line (before `vars:`):

```yaml
invariants:
  sources:
    - globs: ["*.go"]
      marker: "//"
```

(The Phase-1 scanner still has its old signature and ignores this block; it activates in Phase 2.
Adding it now means Phase 2's enforced scan finds the existing `// invariant:` comments and the
dogfood stays green through the transition.)

- [ ] `./x sync` (awf.yaml changed → refresh lock). `./x gate` → `0 issues.`; `./x check` → `awf check: clean` (old scanner still `.go`/`//`).
- [ ] `git add internal/config/config.go internal/config/config_test.go .claude/awf.yaml .claude/awf.lock`
- [ ] `git commit -m "feat(awf): add invariants config block (globs + marker, validated)"`

---

## Phase 2 — Generalize the scanner (+ wire through + tests)

### Task 2.1 — Rewrite `internal/invariants/invariants.go`

- [ ] Replace the entire file with:

```go
// Package invariants checks that each Implemented ADR's `inv: <slug>` invariant
// tag is backed by a `<marker> invariant: <slug>` comment in a configured source
// file. The comment marker and the files scanned are language-configurable via
// the project's invariants config; nothing here assumes Go.
package invariants

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agentic-workflows/internal/adr"
	"agentic-workflows/internal/config"
)

// Status classifies an invariant finding.
type Status string

const (
	Unbacked  Status = "unbacked"  // configured, but no backing tag found
	Unchecked Status = "unchecked" // no invariant sources configured (and not disabled)
)

// Finding is an Implemented-ADR invariant slug that is not satisfied.
type Finding struct {
	Slug   string
	ADR    string // filename of the declaring ADR
	Status Status
}

// Detail is a human, language-neutral remedy line for the finding.
func (f Finding) Detail() string {
	if f.Status == Unchecked {
		return "unchecked — configure invariants.sources or set invariants.disabled: true"
	}
	return "unbacked — add a `<marker> invariant: " + f.Slug + "` comment in a configured source file"
}

var (
	tagRe  = regexp.MustCompile("`inv:\\s*([a-z0-9-]+)`")
	slugRe = regexp.MustCompile(`^\s*invariant:\s*([a-z0-9-]+)`)
)

// Check returns a Finding per unsatisfied Implemented-ADR invariant slug.
// No required slugs → nil. cfg disabled → nil. cfg nil or source-less → every
// required slug is Unchecked. Otherwise unbacked slugs are Unbacked.
func Check(decisionsDir, root string, cfg *config.InvariantConfig) ([]Finding, error) {
	adrs, err := adr.ParseDir(decisionsDir)
	if err != nil {
		return nil, err
	}
	required := map[string]string{} // slug -> declaring ADR filename
	for _, a := range adrs {
		if a.Status != "Implemented" {
			continue
		}
		for _, m := range tagRe.FindAllStringSubmatch(a.Sections["Invariants"], -1) {
			slug := m[1]
			if prev, ok := required[slug]; ok {
				return nil, fmt.Errorf("duplicate inv slug %q (in %s and %s)", slug, prev, a.Filename)
			}
			required[slug] = a.Filename
		}
	}
	if len(required) == 0 {
		return nil, nil
	}
	if cfg != nil && cfg.Disabled {
		return nil, nil
	}

	mk := func(status Status) []Finding {
		out := make([]Finding, 0, len(required))
		for slug, file := range required {
			out = append(out, Finding{Slug: slug, ADR: file, Status: status})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
		return out
	}

	if cfg == nil || len(cfg.Sources) == 0 {
		return mk(Unchecked), nil
	}

	present, err := scanTags(root, cfg.Sources)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for slug, file := range required {
		if !present[slug] {
			findings = append(findings, Finding{Slug: slug, ADR: file, Status: Unbacked})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Slug < findings[j].Slug })
	return findings, nil
}

// scanTags collects slugs backed by a `<marker> invariant: <slug>` comment in a
// file whose basename matches one of a source's globs (skipping
// .git/vendor/node_modules). The marker is matched literally; whitespace between
// the marker and `invariant:` is tolerated.
func scanTags(root string, sources []config.InvariantSource) (map[string]bool, error) {
	present := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		base := filepath.Base(path)
		var markers []string
		for _, src := range sources {
			for _, g := range src.Globs {
				if ok, _ := filepath.Match(g, base); ok {
					markers = append(markers, src.Marker)
					break
				}
			}
		}
		if len(markers) == 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			for _, marker := range markers {
				if idx := strings.Index(line, marker); idx >= 0 {
					if m := slugRe.FindStringSubmatch(line[idx+len(marker):]); m != nil {
						present[m[1]] = true
					}
				}
			}
		}
		return nil
	})
	return present, err
}
```

### Task 2.2 — Thread config through `project` and `cmd`

- [ ] In `internal/project/project.go`, change `CheckInvariants`:

```go
func (p *Project) CheckInvariants() ([]invariants.Finding, error) {
	return invariants.Check(filepath.Join(p.Root, p.Cfg.DocsDir, "decisions"), p.Root, p.Cfg.Invariants)
}
```

- [ ] In `cmd/awf/check.go`, replace the findings-print loop and final message:

old:
```go
	for _, f := range findings {
		fmt.Printf("  %-14s %s — invariant %q has no backing `// invariant: <slug>` test\n", "unbacked-inv", f.ADR, f.Slug)
	}
	if len(drift) == 0 && len(findings) == 0 {
		fmt.Println("awf check: clean")
		return nil
	}
	return fmt.Errorf("awf check: %d drift(s), %d unbacked invariant(s)", len(drift), len(findings))
```
new:
```go
	for _, f := range findings {
		fmt.Printf("  %-14s %s — invariant %q %s\n", "invariant", f.ADR, f.Slug, f.Detail())
	}
	if len(drift) == 0 && len(findings) == 0 {
		fmt.Println("awf check: clean")
		return nil
	}
	return fmt.Errorf("awf check: %d drift(s), %d invariant issue(s)", len(drift), len(findings))
```

- [ ] In `cmd/awf/invariants.go`, replace the findings loop and message:

old:
```go
	for _, f := range findings {
		fmt.Printf("  %s — invariant %q has no backing `// invariant: <slug>` test\n", f.ADR, f.Slug)
	}
	return fmt.Errorf("awf invariants: %d unbacked invariant(s)", len(findings))
```
new:
```go
	for _, f := range findings {
		fmt.Printf("  %s — invariant %q %s\n", f.ADR, f.Slug, f.Detail())
	}
	return fmt.Errorf("awf invariants: %d invariant issue(s)", len(findings))
```

### Task 2.3 — Tests backing the ADR-0008 slugs

- [ ] Replace `internal/invariants/invariants_test.go` with:

```go
package invariants_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentic-workflows/internal/config"
	"agentic-workflows/internal/invariants"
)

func writeADR(t *testing.T, dir, name, status, invBody string) {
	t.Helper()
	content := "---\nstatus: " + status + "\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-X: T\n## Invariants\n" + invBody + "\n## Consequences\nc\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func goSrc(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The three tests below preserve the ADR-0007 invariant slugs (their only
// backing comments live in this file, which this task rewrites). They are
// retained, updated to the new three-arg Check signature, so the dogfood
// `*.go`/`//` scan keeps finding `invariants-implemented-only`,
// `invariants-unbacked-detected`, and `invariants-duplicate-slug`.

// invariant: invariants-implemented-only
func TestCheckImplementedOnly(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-impl` — x.")
	writeADR(t, dir, "0002-b.md", "Proposed", "- `inv: fixture-prop` — x.")
	writeADR(t, dir, "0003-c.md", "Accepted", "- `inv: fixture-acc` — x.")
	writeADR(t, dir, "0004-d.md", "Superseded by ADR-0001", "- `inv: fixture-sup` — x.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-impl" {
		t.Errorf("expected only fixture-impl required, got %#v", f)
	}
}

// invariant: invariants-unbacked-detected
func TestCheckUnbackedAndBacked(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-backed` — x.\n- `inv: fixture-missing` — y.")
	goSrc(t, root, "x.go", "package x\n// invariant: fixture-backed\nfunc T() {}\n")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-missing" {
		t.Errorf("expected only fixture-missing unbacked, got %#v", f)
	}
}

// invariant: invariants-duplicate-slug
func TestCheckDuplicateSlug(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-dup` — x.")
	writeADR(t, dir, "0002-b.md", "Implemented", "- `inv: fixture-dup` — y.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	if _, err := invariants.Check(dir, root, cfg); err == nil {
		t.Error("expected error for duplicate slug")
	}
}

// invariant: invariants-three-state
func TestCheckThreeState(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-one` — x.")
	src := []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}

	// nil config -> unchecked
	f, err := invariants.Check(dir, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Status != invariants.Unchecked {
		t.Fatalf("nil cfg: want 1 unchecked, got %#v", f)
	}
	// disabled -> clean
	f, _ = invariants.Check(dir, root, &config.InvariantConfig{Disabled: true, Sources: src})
	if len(f) != 0 {
		t.Errorf("disabled: want clean, got %#v", f)
	}
	// sources, unbacked -> unbacked
	f, _ = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
	if len(f) != 1 || f[0].Status != invariants.Unbacked {
		t.Fatalf("sources unbacked: want 1 unbacked, got %#v", f)
	}
	// sources, backed -> clean
	goSrc(t, root, "x.go", "package x\n// invariant: fixture-one\n")
	f, _ = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
	if len(f) != 0 {
		t.Errorf("sources backed: want clean, got %#v", f)
	}
}

// invariant: invariants-multilang-scan
func TestCheckMultiLangScan(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-py` — x.")
	if err := os.WriteFile(filepath.Join(root, "t.py"), []byte("# invariant: fixture-py\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.py"}, Marker: "#"}}}
	f, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Errorf("python-backed slug should be clean, got %#v", f)
	}
}

// invariant: invariants-marker-literal
func TestCheckMarkerLiteral(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-lit` — x.")
	// marker contains regex metacharacters; must be matched literally.
	if err := os.WriteFile(filepath.Join(root, "t.txt"), []byte("[x] invariant: fixture-lit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.txt"}, Marker: "[x]"}}}
	f, _ := invariants.Check(dir, root, cfg)
	if len(f) != 0 {
		t.Errorf("literal marker should match, got %#v", f)
	}
}

// invariant: invariants-marker-whitespace
func TestCheckMarkerWhitespace(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-a` — x.\n- `inv: fixture-b` — y.")
	// one with a space after the marker, one without.
	goSrc(t, root, "x.go", "package x\n// invariant: fixture-a\n//invariant: fixture-b\n")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, _ := invariants.Check(dir, root, cfg)
	if len(f) != 0 {
		t.Errorf("whitespace-tolerant marker should match both, got %#v", f)
	}
}

// invariant: invariants-zero-slugs-clean
func TestCheckZeroSlugsClean(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- a textual invariant with no slug.")
	for _, cfg := range []*config.InvariantConfig{nil, {}, {Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}} {
		f, err := invariants.Check(dir, root, cfg)
		if err != nil || len(f) != 0 {
			t.Errorf("zero slugs must be clean (cfg=%#v): got %#v err=%v", cfg, f, err)
		}
	}
}
```

### Task 2.4 — Update the cmd-level invariants test for three-state

The existing `cmd/awf/invariants_test.go` (`TestRunCheckFailsOnUnbackedInvariant`,
backing `invariants-in-check`) builds an `awf.yaml` with **no** `invariants` block, then
expects `runCheck` to go clean once the slug is backed by a `.go` file. Under the new
three-state scanner a `nil` `Invariants` reports `unchecked` (never clean), so the test
would fail. Add an `invariants` block to its yaml so the path reaches the enforced state.

- [ ] In `cmd/awf/invariants_test.go`, change the `yaml` literal:

old:
```go
	yaml := "prefix: example\nskills: {}\nagents: {}\nhooks: []\n"
```
new:
```go
	yaml := "prefix: example\ninvariants:\n  sources:\n    - globs: [\"*.go\"]\n      marker: \"//\"\nskills: {}\nagents: {}\nhooks: []\n"
```

(The slug `invariants-in-check` stays backed by the existing `// invariant: invariants-in-check`
comment above the test func — keep it.)

### Task 2.5 — Verify + commit

- [ ] `go test ./internal/... ./cmd/...` → `ok` (config, invariants, project, cmd all pass).
- [ ] `./x sync`; `./x gate` → `0 issues.`; `./x check` → `awf check: clean` (0005-0007 slugs now enforced under the dogfood `*.go`/`//` config and backed by existing comments); `./x invariants` → `awf invariants: clean`.
- [ ] `git add internal/invariants/ internal/project/project.go cmd/awf/check.go cmd/awf/invariants.go cmd/awf/invariants_test.go .claude/awf.lock`
- [ ] `git commit -m "feat(awf): language-configurable invariant scanner (globs + markers, three states)"`

---

## Phase 3 — De-Go-ify prose + identity

### Task 3.1 — `refactor-coupling-audit` skill: language-neutral procedure

In `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`, replace each Go-specific block:

- [ ] Category 1 command (the `grep … --include='*.go' | grep -v _test.go` block):
```bash
# Search the original package's production source for <MovedSymbol>
# (scope the file filter to your language's source extension; exclude test files).
grep -rn "<MovedSymbol>" <original-package-path>/
```
- [ ] Category 2 command (`grep … --include='*_test.go'`):
```bash
# Search the package's test files (your language's test-file convention).
grep -rn "<MovedSymbol>" <original-package-path>/
```
- [ ] Category 3 command (the import-path grep) — keep `modulePrefix`, neutralize:
```bash
# For languages with import paths, find importers of the original package.
grep -rn "{{ .vars.modulePrefix }}/<original-package-path>" <original-package-path>/
```
- [ ] Category 4 (the `go:generate` block at lines ~77-81): replace the prose + command with:
```
Check code-generation that references the moved symbols — Go `go:generate` directives,
build scripts, schema/codegen configs, or derived-table generators in your toolchain:
```
```bash
# Go example; adapt to your project's codegen.
grep -rn "go:generate" | grep -i "<symbol-or-package>"
```
- [ ] Category 5 command (`init.go/constructor.go/new*.go`):
```bash
# Your language's constructor / initialisation entry points.
grep -rn "<MovedSymbol>" <constructor-or-init-files>
```
- [ ] Category 6 prose+command (lines ~103-106): replace the receiver-method prose first line and command:
  - prose: `Functions or methods defined on the moved type with cross-package callers cannot move without preserving reachability — e.g. Go export/visibility, or introducing an interface in the original package with the implementation in the destination.`
  - command:
```bash
# Find functions/methods defined on the moved type (Go method-receiver example shown; adapt).
grep -rn "<MovedType>" <original-package-path>/
```

### Task 3.2 — `proposing-adr` skill: neutralize the marker prose

- [ ] In `templates/skills/proposing-adr/SKILL.md.tmpl`, replace the `invariants-rule` step body:

old:
```
1. **Tag enforceable Invariants and back them with a test.** Give each machine-checkable Invariants bullet an explicit slug, ``- `inv: <slug>` — …``, and add a `// invariant: <slug>` comment to a test that exercises it, shipping in the same commit. `{{ .prefix }}-check` fails once the ADR is `Implemented` if a tagged slug has no backing test. Bullets without a slug remain textual contracts (not machine-checked). Run `{{ .vars.gateCmd }}` and `{{ .vars.checkCmd }}` to confirm.
```
new:
```
1. **Tag enforceable Invariants and back them with a test.** Give each machine-checkable Invariants bullet an explicit slug, ``- `inv: <slug>` — …``, and back it with a comment tag — your project's comment marker followed by `invariant: <slug>` (e.g. `// invariant: <slug>` in Go/Rust/TS, `# invariant: <slug>` in Python/Ruby) — in a source file matching a glob in your `.claude/awf.yaml` `invariants.sources`, shipping in the same commit. `{{ .prefix }}-check` fails once the ADR is `Implemented` if a tagged slug is unbacked, or if `invariants` is unconfigured (set `invariants.sources` or `invariants.disabled: true`). Bullets without a slug remain textual contracts. Run `{{ .vars.gateCmd }}` and `{{ .vars.checkCmd }}` to confirm.
```

### Task 3.3 — ADR template: neutralize the marker prose

- [ ] In `docs/decisions/template.md`, replace the Invariants section body:

old:
```
should trigger a new ADR if violated. Tag each machine-enforceable bullet with a slug and back
it with a `// invariant: <slug>` test; `awf check` enforces tagged slugs once the ADR is
`Implemented`. Untagged bullets are textual contracts.
```
new:
```
should trigger a new ADR if violated. Tag each machine-enforceable bullet with a slug and back it
with a comment tag (`<your marker> invariant: <slug>`, e.g. `// invariant: <slug>` or
`# invariant: <slug>`) in a source matching your `.claude/awf.yaml` `invariants.sources`; `awf
check` enforces tagged slugs once the ADR is `Implemented`. Untagged bullets are textual contracts.
```

### Task 3.4 — Identity + README positioning

- [ ] In `.claude/awf.yaml`, replace the `agentsDoc.data.identity` value (the "into any Go project" paragraph) with:

```
      `awf` is a generic agentic-development-workflow application: it scaffolds, renders, and drift-checks a suite of Claude Code skills, review agents, git hooks, docs, and this agent guide into any project from a single `.claude/awf.yaml` — supplying a default way to set things up and the tooling to enforce parts of it (drift, frontmatter, invariant backing). The full workflow chain is project-owned skill files under `.claude/skills/awf-*/` and review agents under `.claude/agents/`; hooks under `.githooks/` enforce the gate. The awf tool is a Go binary (module `agentic-workflows`, Go 1.26); the standard it renders is language-agnostic. Private, pre-1.0, no external API stability.
```

- [ ] In `.claude/awf.yaml`, replace the "Backed invariants" invariant bullet's `text:` line (the `ref:` line below it changes from `ADR-0007` to `ADR-0008`, since this bullet now states the ADR-0008-revised behaviour):

```yaml
      - text: "**Backed invariants.** Each machine-enforceable ADR Invariants bullet carries an `inv: <slug>` tag backed by a `<marker> invariant: <slug>` comment in a source matching `invariants.sources`; `awf check` (and `awf invariants`) fail when an Implemented ADR has an unbacked — or unconfigured — tagged slug."
        ref: ADR-0008
```

- [ ] In `README.md`, replace the description paragraph (lines 3-4):

old:
```
`awf` renders standardised `.claude` skills, review agents, and git hooks into a project
from shared templates plus a per-project `.claude/awf.yaml`.
```
new:
```
`awf` is a generic agentic-development-workflow tool: it renders a standardised suite of Claude
Code skills, review agents, git hooks, and docs into any project from shared templates plus a
per-project `.claude/awf.yaml`, and supplies the tooling to drift-check and enforce parts of the
standard. The awf tool is a Go binary; the standard it renders is language-agnostic.
```

### Task 3.5 — Re-sync + commit

- [ ] `./x sync` — re-renders `.claude/skills/awf-refactor-coupling-audit/SKILL.md`, `.claude/skills/awf-proposing-adr/SKILL.md`, and `AGENTS.md`.
- [ ] Verify no Go-isms remain in the rendered standard: `grep -rn "include='\*.go'\|go:generate\|// invariant: <slug>" .claude/skills/ AGENTS.md` → only acceptable hits (none of the removed forms). `./x gate` → `0 issues.`; `./x check` → clean; `./x invariants` → clean.
- [ ] `git add templates/ docs/decisions/template.md .claude/ AGENTS.md README.md`
- [ ] `git commit -m "docs(awf): de-Go-ify skills, marker prose, and identity for polyglot use"`

---

## Phase 4 — Finalize

### Task 4.1 — Flip ADR-0008 to Implemented

- [ ] In `docs/decisions/0008-language-agnostic-invariant-backing.md`, change `status: Accepted` → `status: Implemented`. (Its six `inv:` slugs are backed by the Phase 1-2 tests and enforced under the dogfood `*.go`/`//` config.)
- [ ] `./x sync` — regenerates `ACTIVE.md`.
- [ ] `./x gate` → `0 issues.`; `./x check` → clean; `./x invariants` → `awf invariants: clean`.
- [ ] `git add docs/decisions/0008-language-agnostic-invariant-backing.md docs/decisions/ACTIVE.md .claude/awf.lock`
- [ ] `git commit -m "docs(adr): mark 0008 Implemented"`

### Task 4.2 — Terminal handoff

- [ ] Invoke `awf-reviewing-impl` against the implementation commit range (Phases 1-4).

---

## Notes

- **Ordering:** Phase 1 adds the config type AND the dogfood `invariants` block while the old scanner still ignores config (so `awf check` stays green). Phase 2 swaps to the config-driven scanner; because the block already declares `*.go`/`//`, the existing 0005-0007 `// invariant:` comments are found and the dogfood stays clean. Phase 4 flips 0008 only after its slugs are backed (Phase 1-2 tests).
- **Fixture isolation:** invariants tests use `fixture-`-prefixed slugs in temp dirs; no collision with real ADR slugs.
- Module-path rename + publish remain separate threads (now unblocked to position awf as polyglot).
