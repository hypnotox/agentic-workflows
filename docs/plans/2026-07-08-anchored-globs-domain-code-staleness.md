# 2026-07-08 — Anchored path globs and the domain-code-staleness audit rule (ADR-0077)

**Goal:** implement [ADR-0077](../decisions/0077-anchored-path-globs-and-the-domain-code-staleness-audit-rule.md):
one anchored full-path glob dialect (`internal/pathglob` on doublestar) across every glob consumer,
the schema-generation-7 migration rewriting legacy no-slash patterns, per-domain sidecar `paths`,
and the advisory `domain-code-staleness` audit rule. Design rationale lives in the ADR — this plan
is execution only.

**Architecture summary:** new leaf package `internal/pathglob` (wraps
`github.com/bmatcuk/doublestar/v4`); `internal/config` swaps `validateBasenameGlob` for
pathglob validation and gains the nested-sequence editor `AnchorNoSlashGlobs`;
`internal/invariants` and `internal/audit` switch to full-relative-path matching; migration
`{To: 7, "anchored-globs"}` + `minVersionBySchema[7]` + `Version` bump to `0.11.0`; `Sidecar`
gains `Paths`; `Project.Audit()` builds `Inputs.DomainPaths`; new rule `ruleDomainCodeStaleness`.
The whole glob-semantics switch (Phase 1) is **one atomic commit**: the matcher swap, the
validator swap, the code-default rewrite, the migration, and this repo's own config rewrite are
mutually dependent — any subset leaves `./x check` red (a `*.go` config under anchored matching
backs no invariants; a `**/*.go` config under basename matching backs none either).

**ADR-0008 retirement mechanics:** ADR-0077 retires `invariants-glob-basename`
(`retires_invariants` frontmatter). Retirement takes effect only when ADR-0077 reaches
`Implemented`, so the backing comment at `internal/config/config_test.go:429` must survive until
the final status-flip commit: Phase 1 rewrites that test's assertions to the anchored semantics but
keeps the `// invariant: invariants-glob-basename` comment line; Phase 3's flip commit deletes it.
(ADR-0077 Decision 2's "backing test removed in the same change" is realised as this
comment-at-flip deletion: the invariants checker keys on the backing comment, not the test body,
and the basename assertions cannot survive Phase 1's atomic matcher swap.)

**Tech stack:** Go 1.26; new dep `github.com/bmatcuk/doublestar/v4`; packages touched:
`internal/pathglob` (new), `internal/config`, `internal/invariants`, `internal/audit`,
`internal/migrate`, `internal/project`; config/docs: `.awf/config.yaml`, `.awf/domains/*.yaml`
(new sidecars), `.awf/domains/parts/{tooling,config,invariants}/current-state.md`,
`.awf/docs/parts/{architecture/components,development/dependencies,glossary/terms}.md`,
`.awf/agents-doc.yaml`, `templates/docs/working-with-awf.md.tmpl`, `changelog/CHANGELOG.md`.

**File structure:**
- Created: `internal/pathglob/pathglob.go`, `internal/pathglob/pathglob_test.go`,
  `internal/migrate/anchoredglobs.go`, `internal/migrate/anchoredglobs_test.go`,
  `.awf/domains/{adr-system,config,invariants,rendering,tooling}.yaml`
- Modified: `go.mod`/`go.sum`, `internal/config/{config.go,config_test.go,edit.go,edit_test.go}`,
  `internal/invariants/{invariants.go,invariants_test.go}`,
  `internal/audit/{audit.go,audit_test.go,settings.go,settings_test.go}`,
  `internal/migrate/migrate.go`, `internal/project/{project.go,project_test.go or audit-input test
  file}`, `.awf/config.yaml`, `.awf/awf.lock`, the doc parts listed above, rendered outputs via
  `./x sync`, `docs/decisions/0077-*.md` (status flip, final commit)
- Deleted: none

---

## Phase 1 — one anchored glob dialect everywhere + schema-7 migration (single commit)

- [ ] **1.1 Add the dependency.**

```
go get github.com/bmatcuk/doublestar/v4@latest
```

  Expect `go.mod` to gain `require github.com/bmatcuk/doublestar/v4 v4.x.x` (zero new transitive
  requirements).

- [ ] **1.2 New package tests.** Create `internal/pathglob/pathglob_test.go`:

```go
package pathglob

import "testing"

// The table is the spec for ADR-0077's anchored dialect: no basename mode,
// `**/` is the only any-depth form, slashed patterns anchor at the repo root.
func TestMatchAnchored(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*.go", "a.go", true},
		{"*.go", "cmd/a.go", false}, // anchored: no basename fallback
		{"**/*.go", "a.go", true},   // `**/` matches zero directories too
		{"**/*.go", "cmd/x/a.go", true},
		{"cmd/**", "cmd/awf/main.go", true},
		{"cmd/**", "internal/audit/audit.go", false},
		{"internal/audit/*.go", "internal/audit/audit.go", true},
		{"internal/audit/*.go", "internal/audit/sub/x.go", false},
		{"go.mod", "go.mod", true},
		{"go.mod", "sub/go.mod", false},
		{"**/go.mod", "sub/go.mod", true},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestMatchMalformedPatternMatchesNothing(t *testing.T) {
	if Match("[", "a.go") {
		t.Error("malformed pattern must match nothing")
	}
}

func TestValidate(t *testing.T) {
	if err := Validate("**/*.go"); err != nil {
		t.Errorf("valid pattern rejected: %v", err)
	}
	if err := Validate("["); err == nil {
		t.Error("expected malformed pattern to be rejected")
	}
}
```

  Run `go test ./internal/pathglob/` — expect `FAIL` (package does not exist yet / undefined).

- [ ] **1.3 New package.** Create `internal/pathglob/pathglob.go`:

```go
// Package pathglob is awf's single glob dialect (ADR-0077): anchored full-path
// doublestar matching against slash-separated repo-relative paths. There is
// deliberately no basename mode — `*.go` matches only top-level .go files;
// any-depth is written `**/*.go`. Leaf package: imports nothing from awf.
package pathglob

import (
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
)

// Validate rejects a malformed doublestar pattern.
func Validate(pattern string) error {
	if !doublestar.ValidatePattern(pattern) {
		return fmt.Errorf("glob %q is malformed", pattern)
	}
	return nil
}

// Match reports whether the slash-separated repo-relative path matches the
// anchored pattern. A malformed pattern matches nothing — Validate at config
// load / audit-input building keeps that branch cold in practice.
// invariant: pathglob-anchored
func Match(pattern, relPath string) bool {
	ok, err := doublestar.Match(pattern, relPath)
	return err == nil && ok
}
```

  Run `go test ./internal/pathglob/` — expect `ok`.

- [ ] **1.4 Config validator swap.** In `internal/config/config.go`:
  - Add `"github.com/hypnotox/agentic-workflows/internal/pathglob"` to the imports.
  - Replace the whole `validateBasenameGlob` function (currently rejecting `/`-containing
    patterns) with:

```go
// validatePathGlob rejects a malformed anchored path-glob pattern (ADR-0077).
// Patterns are matched against slash-separated repo-relative paths; `**/` is
// the any-depth form.
func validatePathGlob(g string) error {
	return pathglob.Validate(g)
}
```

  - Rename both call sites (`validateBasenameGlob` → `validatePathGlob`) in `Validate`.
  - Update the `InvariantSource` doc comment to:
    `// InvariantSource pairs anchored path globs (ADR-0077; matched against a file's slash-separated repo-relative path) with`
    (second line unchanged).
  - In the `Validate` error message at `config.go:231` (the only one carrying a glob example):
    `list at least one filename glob (e.g. "*.go")` → `list at least one path glob (e.g. "**/*.go")`.

- [ ] **1.5 Config validator tests.** In `internal/config/config_test.go`, rewrite
  `TestInvariantGlobValidation` (keep the `// invariant: invariants-glob-basename` comment line
  directly above it for now — deleted in task 3.6): the `ok` case's globs become
  `[]string{"**/*.go", "**/*_test.py"}`; the `pathGlob` case (which asserted `**/*.go` is
  *rejected*) now asserts it is **accepted**:

```go
	pathGlob := &Config{Prefix: "x", DocsDir: "docs", Targets: []string{"claude"}, Invariants: &InvariantConfig{
		Sources: []InvariantSource{{Globs: []string{"**/*.go", "cmd/**"}, Marker: "//"}},
	}}
	if err := pathGlob.Validate(); err != nil {
		t.Errorf("path globs must be accepted under ADR-0077: %v", err)
	}
```

  The `bad` (malformed `[`), `emptyMarker`, and `emptyGlobs` cases stay as they are.

  Also rewrite `TestAuditDependencyManifestValidation` (`config_test.go:463-476`): its bad case
  currently asserts `src/go.mod` is rejected as a path-separator glob — under `validatePathGlob`
  it is accepted, so flip that case to assert acceptance and keep a malformed-pattern rejection
  case using `[`.

- [ ] **1.6 Invariants matcher swap.** In `internal/invariants/invariants.go`:
  - Import `"github.com/hypnotox/agentic-workflows/internal/pathglob"`.
  - In `scanTags`, replace the basename matching (`base := filepath.Base(path)` and the
    `filepath.Match(g, base)` loop) with repo-relative anchored matching:

```go
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil { // coverage-ignore: WalkDir yields paths under root, so Rel cannot fail
			return rerr
		}
		relSlash := filepath.ToSlash(rel)
		var markers []string
		for _, src := range sources {
			for _, g := range src.Globs {
				if pathglob.Match(g, relSlash) {
					markers = append(markers, src.Marker)
					break
				}
			}
		}
```

  - Update the `scanTags` doc comment: "in a file whose slash-separated repo-relative path
    matches one of a source's anchored globs (ADR-0077; skipping .git/vendor/node_modules)".
  - In `internal/invariants/invariants_test.go`, update every fixture glob `*.go` → `**/*.go`
    (and any other no-slash fixture globs likewise), and add one anchored-scope case: a source
    with `Globs: []string{"sub/**"}` backs a slug tagged in `sub/x.go` but not one tagged only in
    a top-level `y.go`.

- [ ] **1.7 Audit matcher swap + code defaults.** In `internal/audit/audit.go`:
  - Import `"github.com/hypnotox/agentic-workflows/internal/pathglob"`; drop the now-unused
    `"path/filepath"` import **only if** nothing else uses it (`isADRFile` uses `filepath.Dir` —
    it stays).
  - Replace `matchesAny` and its call site:

```go
// matchesAny reports whether the repo-relative path matches any anchored glob.
func matchesAny(globs []string, path string) bool {
	for _, g := range globs {
		if pathglob.Match(g, path) {
			return true
		}
	}
	return false
}
```

    Call site in `ruleDependencyADR`:
    `matchesAny(in.DependencyManifests, filepath.Base(ch.Path))` →
    `matchesAny(in.DependencyManifests, ch.Path)`.
  - In `internal/audit/settings.go`, rewrite `defaultDependencyManifests` — every entry gains the
    `**/` prefix so nested manifests keep matching (ADR-0077 Decision 2):

```go
func defaultDependencyManifests() []string {
	return []string{
		"**/go.mod", "**/package.json", "**/pyproject.toml", "**/setup.py", "**/requirements*.txt",
		"**/Cargo.toml", "**/Gemfile", "**/*.gemspec", "**/composer.json", "**/pom.xml", "**/build.gradle",
		"**/build.gradle.kts", "**/*.csproj", "**/Directory.Packages.props", "**/mix.exs",
		"**/Package.swift", "**/pubspec.yaml", "**/*.cabal", "**/package.yaml",
	}
}
```

  - Update the tests exactly: in `internal/audit/settings_test.go`, the two default-set
    assertions `slices.Contains(s.DependencyManifests, "go.mod")` (lines 27-28 region and 41)
    become `slices.Contains(s.DependencyManifests, "**/go.mod")`. In
    `internal/audit/audit_test.go`, every fixture feeding `DependencyManifests` a bare basename
    pattern switches to the `**/`-prefixed form (preserving each test's intent); add one new case
    asserting a nested `sub/go.mod` change triggers `dependency-adr` under
    `defaultDependencyManifests()`.

- [ ] **1.8 Nested-sequence editor tests.** Append to `internal/config/edit_test.go`:

```go
func TestAnchorNoSlashGlobs(t *testing.T) {
	src := []byte(`prefix: x
invariants:
  disabled: false
  sources:
    - globs:
        - '*.go'
        - cmd/**
      marker: //
audit:
  dependencyManifests:
    - go.mod
    - '**/package.json'
`)
	out, err := AnchorNoSlashGlobs(src)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"**/*.go", "cmd/**", "**/go.mod", "**/package.json"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "**/cmd/**") || strings.Contains(s, "**/**/package.json") {
		t.Errorf("slashed pattern was rewritten:\n%s", s)
	}
	// Idempotent: a second pass changes nothing.
	again, err := AnchorNoSlashGlobs(out)
	if err != nil || string(again) != s {
		t.Errorf("not idempotent (err %v):\n%s", err, again)
	}
}

func TestAnchorNoSlashGlobsAbsentKeysNoop(t *testing.T) {
	src := []byte("prefix: x\nskills:\n  - tdd\n")
	out, err := AnchorNoSlashGlobs(src)
	if err != nil || string(out) != string(src) {
		// encode() may normalize; assert semantic no-op instead of byte equality
		if err != nil || strings.Contains(string(out), "**/") {
			t.Errorf("expected no-op, got (err %v):\n%s", err, out)
		}
	}
}
```

  (Add `"strings"` to the test file's imports if absent.) Run
  `go test ./internal/config/ -run TestAnchorNoSlashGlobs` — expect `FAIL: undefined: AnchorNoSlashGlobs`.

- [ ] **1.9 Nested-sequence editor.** Append to `internal/config/edit.go` (add `"strings"` to
  imports):

```go
// AnchorNoSlashGlobs rewrites every no-slash glob scalar under
// invariants.sources[].globs and audit.dependencyManifests to `**/<pattern>`,
// preserving comments and untouched keys (ADR-0026). Slashed patterns are left
// alone, so the rewrite is idempotent; absent keys are a no-op. It is the
// nested-sequence editor the schema-7 anchored-globs migration (ADR-0077)
// consumes — the sequence analog of SetMappingScalar.
func AnchorNoSlashGlobs(src []byte) ([]byte, error) {
	doc, root, err := parseMapping(src)
	if err != nil {
		return nil, err
	}
	if inv, _ := mapValue(root, "invariants"); inv != nil && inv.Kind == yaml.MappingNode {
		if srcs, _ := mapValue(inv, "sources"); srcs != nil && srcs.Kind == yaml.SequenceNode {
			for _, s := range srcs.Content {
				if s.Kind != yaml.MappingNode {
					continue
				}
				if globs, _ := mapValue(s, "globs"); globs != nil && globs.Kind == yaml.SequenceNode {
					anchorSeq(globs)
				}
			}
		}
	}
	if aud, _ := mapValue(root, "audit"); aud != nil && aud.Kind == yaml.MappingNode {
		if dm, _ := mapValue(aud, "dependencyManifests"); dm != nil && dm.Kind == yaml.SequenceNode {
			anchorSeq(dm)
		}
	}
	return encode(doc)
}

// anchorSeq rewrites each non-empty no-slash scalar member of seq to `**/<value>`.
// invariant: glob-migration-anchored
func anchorSeq(seq *yaml.Node) {
	for _, n := range seq.Content {
		if n.Kind == yaml.ScalarNode && n.Value != "" && !strings.Contains(n.Value, "/") {
			n.Value = "**/" + n.Value
		}
	}
}
```

  Run `go test ./internal/config/` — expect `ok`.

- [ ] **1.10 Migration tests.** Create `internal/migrate/anchoredglobs_test.go` mirroring the
  existing migration tests' fixture style (testsupport project scaffolding): write a `.awf/`
  tree whose `config.yaml` contains the Phase-1.8 fixture body, call `applyAnchoredGlobs(root)`,
  assert the rewritten file contains `**/*.go` and `**/go.mod` and still contains `cmd/**`
  un-doubled; assert a root without `.awf/config.yaml` is a nil-error no-op. Run
  `go test ./internal/migrate/ -run AnchoredGlobs` — expect `FAIL: undefined: applyAnchoredGlobs`.

- [ ] **1.11 Migration.** Create `internal/migrate/anchoredglobs.go`:

```go
package migrate

import "github.com/hypnotox/agentic-workflows/internal/config"

// applyAnchoredGlobs ports a tree to the anchored path-glob dialect (ADR-0077):
// every no-slash pattern in invariants.sources[].globs and
// audit.dependencyManifests becomes `**/<pattern>`, preserving behaviour for
// every pattern valid under the old validator (doublestar brace alternation is
// the accepted edge, ADR-0077). Serialization stays owned by internal/config
// (ADR-0026); the write is atomic via editConfig (ADR-0076).
func applyAnchoredGlobs(root string) error {
	return editConfig(root, config.AnchorNoSlashGlobs)
}
```

  In `internal/migrate/migrate.go`, append to the registry:

```go
	{To: 7, Name: "anchored-globs", Apply: applyAnchoredGlobs},
```

  Update the registry-pinning tests in `internal/migrate/migrate_test.go`:
  - `TestCurrentIsSix` (line 609): assert `Current() == 7` and rename to `TestCurrentIsSeven`.
  - `TestUpgradeAppliesInOrderIdempotent` (line 168) and `TestUpgradeStampsTreeLock` (line 890):
    append `,anchored-globs` to their exact expected applied-migration list strings.

  Run `go test ./internal/migrate/` — expect `ok`.

- [ ] **1.12 Version authority + changelog.** In `internal/project/project.go`:
  `Version = "0.10.0"` → `Version = "0.11.0"`; add `7: "0.11.0",` to `minVersionBySchema`. (The
  gate test `internal/project/version_test.go` enforces exactly this pairing.) In the same task —
  the gate test `cmd/awf/changelog_test.go` (`TestChangelogLatestMatchesVersion`) pins the newest
  changelog entry to `project.Version`, so add the `0.11.0` section to `changelog/CHANGELOG.md`
  now: **Breaking changes:** one anchored path-glob dialect (schema 7 — run `awf upgrade`;
  `*.go` now means top-level only, migrated configs are rewritten to `**/*.go`); **Features:**
  domain sidecar `paths` + `domain-code-staleness` audit rule; path globs now legal in
  `invariants.sources[].globs` and `audit.dependencyManifests`.

- [ ] **1.13 Migrate this repo + re-render.**

```
go run ./cmd/awf upgrade
./x sync
```

  Expect upgrade to report applying `anchored-globs`; `.awf/config.yaml`'s `globs:` becomes
  `- '**/*.go'`; `.awf/awf.lock` restamps `schemaVersion: 7` and `awfVersion: 0.11.0`; sync
  re-renders every artifact whose prose embeds the globs (the `invariantMarkerTable` placeholder
  now shows `**/*.go`).

- [ ] **1.14 Gate and commit (one atomic commit).**

```
./x gate && ./x check
git add go.mod go.sum internal/pathglob internal/config internal/invariants internal/audit internal/migrate internal/project changelog/CHANGELOG.md .awf/config.yaml .awf/awf.lock <every file ./x sync re-rendered — take the list from git status>
git commit -m "feat(config): anchor path globs on doublestar, migrate schema to 7"
```

  Body: one anchored full-path glob dialect across config validation, invariant scanning, and
  audit manifest matching (ADR-0077 Decisions 1–3); schema-7 migration rewrites legacy no-slash
  patterns; `invariants-glob-basename` backing comment retained until the ADR-0077 flip.

## Phase 2 — domain sidecar `paths` + the `domain-code-staleness` rule (single commit)

- [ ] **2.1 Sidecar field.** In `internal/config/config.go`, extend `Sidecar`:

```go
type Sidecar struct {
	Data     map[string]any             `yaml:"data"`
	Sections map[string]SectionOverride `yaml:"sections"`
	Local    bool                       `yaml:"local"`
	// Paths declares a domain's file territory as anchored path globs
	// (ADR-0077); read only from domain sidecars, inert on other kinds.
	Paths []string `yaml:"paths"`
}
```

  In `internal/config/config_test.go`, extend the existing sidecar-parsing test with a fixture
  containing `paths:\n  - cmd/**` and assert `sc.Paths == []string{"cmd/**"}`.

- [ ] **2.2 Audit config + settings.** In `internal/config/config.go`, add to `AuditConfig` after
  `DomainDocStaleness`:

```go
	DomainCodeStaleness *bool       `yaml:"domainCodeStaleness"`
```

  In `internal/audit/audit.go`, extend the `Inputs` doc comment's promoted-knob enumeration:
  `…DomainDocStaleness, UndocumentedDomain, UncommittedChanges` →
  `…DomainDocStaleness, DomainCodeStaleness, UndocumentedDomain, UncommittedChanges`.

  In `internal/audit/settings.go`: add `DomainCodeStaleness bool` to `Settings` (after
  `DomainDocStaleness`), default it `true` in `Resolve`, and add the override arm:

```go
	if a.DomainCodeStaleness != nil {
		s.DomainCodeStaleness = *a.DomainCodeStaleness
	}
```

  Extend the existing `Resolve` default/override tests in `settings_test.go` with the new field
  (default true; explicit false wins).

- [ ] **2.3 Rule tests.** Append to `internal/audit/audit_test.go`, mirroring the
  `ruleDomainDocStaleness` test style (build `[]Commit` fixtures + `Inputs` directly). Inputs:
  `DomainsPartsDir: ".awf/domains/parts"`, `GeneratedPaths: map[string]bool{"docs/domains/tooling.md": true}`,
  `DomainPaths: map[string][]string{"tooling": {"cmd/**", "internal/audit/**"}}`,
  `Settings: Settings{DomainCodeStaleness: true}`. Cases (each asserting findings count, rule name
  `domain-code-staleness`, severity `Warning`, branch-level `Commit == ""`):
  1. a commit changing `cmd/awf/main.go`, no part change → exactly one Warning naming `tooling`;
  2. same change plus a commit changing `.awf/domains/parts/tooling/current-state.md` → no finding;
  3. a commit changing only `docs/domains/tooling.md` (generated) → no finding;
  4. a commit changing `internal/render/render.go` (matches no domain) → no finding;
  5. `DomainCodeStaleness: false` with case-1 fixtures → no finding;
  6. `DomainPaths: nil` with case-1 fixtures → no finding;
  7. two matching domains churned, neither part refreshed → two findings, sorted by domain name.

  Run `go test ./internal/audit/ -run DomainCodeStaleness` — expect `FAIL` (undefined rule).

- [ ] **2.4 Rule.** In `internal/audit/audit.go`: add to `Inputs` (after `DomainsPartsDir`):

```go
	DomainPaths       map[string][]string // domain -> anchored path globs (ADR-0077); empty = rule inert
```

  Add to `evaluate` after the `ruleUndocumentedDomain` line:

```go
	out = append(out, ruleDomainCodeStaleness(commits, in)...)
```

  Add the rule after `ruleUndocumentedDomain`:

```go
// invariant: audit-domain-code-staleness
func ruleDomainCodeStaleness(commits []Commit, in Inputs) []Finding {
	if !in.DomainCodeStaleness || len(in.DomainPaths) == 0 {
		return nil
	}
	refreshed := map[string]bool{} // domains whose source narrative changed in range
	churned := map[string]bool{}   // domains whose declared territory changed in range
	for _, c := range commits {
		for _, ch := range c.Changes {
			if d, ok := domainOfPart(ch.Path, in.DomainsPartsDir); ok {
				refreshed[d] = true
			}
			if in.GeneratedPaths[ch.Path] {
				continue
			}
			for d, globs := range in.DomainPaths {
				if !churned[d] && matchesAny(globs, ch.Path) {
					churned[d] = true
				}
			}
		}
	}
	var out []Finding
	for _, d := range slices.Sorted(maps.Keys(churned)) {
		if !refreshed[d] {
			out = append(out, Finding{Severity: Warning, Rule: "domain-code-staleness",
				Detail: fmt.Sprintf("files in domain %q changed but %s/%s/current-state.md was not refreshed in this range — if anything meaningful changed, document it", d, in.DomainsPartsDir, d)})
		}
	}
	return out
}
```

  Run `go test ./internal/audit/` — expect `ok`.

- [ ] **2.5 Input building.** In `internal/project/project.go` (`Audit` method), import
  `"github.com/hypnotox/agentic-workflows/internal/pathglob"` and `"fmt"` (if absent) and insert
  before the `return audit.Run(...)`:

```go
	domainPaths := map[string][]string{}
	for _, d := range p.Cfg.Domains {
		sc, err := p.Cfg.Sidecar("domains", d)
		if err != nil {
			return nil, err
		}
		for _, g := range sc.Paths {
			if err := pathglob.Validate(g); err != nil {
				return nil, fmt.Errorf("domain %q paths: %w", d, err)
			}
		}
		if len(sc.Paths) > 0 {
			domainPaths[d] = sc.Paths
		}
	}
```

  and add `DomainPaths: domainPaths,` to the `audit.Inputs` literal. Tests (no existing
  `Project.Audit` input suite exists — the closest is the corrupt-lock refusal in
  `internal/project/drift_test.go:407`; `Audit` runs `Collect`, so the happy path needs a git
  fixture via `internal/testsupport/gitfixture`):
  - happy path: a gitfixture repo whose `.awf/domains/tooling.yaml` declares `paths: [cmd/**]`;
    `Audit("")` succeeds (empty range) — assert no error, proving sidecar reading and validation
    ran clean; then commit a `cmd/` change on a branch and assert the `domain-code-staleness`
    Warning appears in the findings, proving `DomainPaths` reached the rule;
  - malformed pattern: sidecar `paths: ['[']` → `Audit` returns an error containing
    `domain "tooling" paths` (fails before `Collect`, so no git repo needed beyond the fixture);
  - sidecar read error: a **directory** squatting at `.awf/domains/tooling.yaml` (mirroring
    `TestSidecarReadErrorWhenPathIsDir`) → `Audit` returns the read error, covering the
    `return nil, err` branch for the 100% gate;
  - a domain with no sidecar (or no `paths`) is absent from the map (assert via the happy-path
    fixture's second domain).

- [ ] **2.6 Gate and commit.**

```
./x gate && ./x check
git add internal/config internal/audit internal/project
git commit -m "feat(tooling): add domain-code-staleness audit rule (ADR-0077)"
```

  Body: per-domain sidecar `paths` territory + range-scoped advisory Warning when a domain's
  files churn without its current-state narrative (ADR-0077 Decisions 4–5).

## Phase 3 — dogfood, docs, changelog, status flip

- [ ] **3.1 Dogfood domain sidecars.** Create five sidecars (exact contents):

  `.awf/domains/adr-system.yaml`:
```yaml
paths:
  - internal/adr/**
```
  `.awf/domains/config.yaml`:
```yaml
paths:
  - internal/config/**
  - internal/migrate/**
  - internal/manifest/**
```
  `.awf/domains/invariants.yaml`:
```yaml
paths:
  - internal/invariants/**
```
  `.awf/domains/rendering.yaml`:
```yaml
paths:
  - internal/render/**
  - internal/catalog/**
  - templates/**
```
  `.awf/domains/tooling.yaml`:
```yaml
paths:
  - cmd/**
  - internal/audit/**
  - internal/coverage/**
  - internal/changelog/**
  - internal/evals/**
  - x
```

  Verify: `go run ./cmd/awf audit` exits 0 (advisory) and — when run on a branch with commits
  ahead of the audit base — **fires `domain-code-staleness` warnings** for the territories
  Phases 1-2 churned (tooling/config/invariants), since their narratives land only in 3.3. The
  warnings firing here is the dogfood signal that the rule works; 3.4 asserts they clear after
  3.3. (Run directly on `main` the range is empty and the check is vacuous.)

- [ ] **3.2 Usage-guide template.** In `templates/docs/working-with-awf.md.tmpl`, add a
  subsection at the end of the `## Config and overrides` section:

````markdown
### Path globs and domain territories

Every glob in awf — `invariants.sources[].globs`, `audit.dependencyManifests`, and domain
`paths` — uses one anchored dialect: a pattern matches a file's slash-separated repo-relative
path in full. `*.go` matches only top-level `.go` files; write `**/*.go` for any depth
(this deliberately differs from gitignore, where a slash-free pattern floats). `cmd/**` covers
a subtree; `internal/audit/*.go` scopes one directory.

A domain sidecar `.awf/domains/<name>.yaml` may declare the domain's file territory:

```yaml
paths:
  - cmd/**
  - internal/audit/*.go
```

When files matching a domain's `paths` change on a branch without a co-change to
`.awf/domains/parts/<name>/current-state.md`, `awf audit` raises an advisory
`domain-code-staleness` warning — if anything meaningful changed, document it. Domains without
`paths` opt out; the rule is disable-able via `audit.domainCodeStaleness: false`.
````

  Run `./x sync` and confirm `docs/working-with-awf.md` renders the subsection.

- [ ] **3.3 Domain narratives + doc parts.** Update (rewrite the affected sentences, keeping each
  file's existing voice):
  - `.awf/domains/parts/tooling/current-state.md` — in the audit-rules sentence, add
    `domain-code-staleness` (ADR-0077: churn in a domain's sidecar-declared `paths` territory
    without a current-state refresh warns) alongside the ADR-0019 pair.
  - `.awf/domains/parts/invariants/current-state.md` — "filename globs + a literal marker" →
    anchored path globs (ADR-0077) matched against repo-relative paths.
  - `.awf/domains/parts/config/current-state.md` — describe the sidecar `paths` field and the
    schema-7 anchored-globs migration; **correct the two stale claims** (ADR-0077 Consequences):
    drop "including the `invariants` marker/globs" from the `ScaffoldConfig` sentence (it takes
    no invariants config), and revise "Additive optional fields (like `domains`) are
    backward-safe and need no version bump" to note the sidecar `paths` extension shipped under
    the schema-7 bump because strict sidecar parsing rejects unknown keys.
  - `.awf/docs/parts/architecture/components.md` — add a bullet for `internal/pathglob` (the
    single anchored glob dialect, ADR-0077) in the package list.
  - `.awf/docs/parts/development/dependencies.md` — add a row/entry:
    `github.com/bmatcuk/doublestar/v4` — anchored path-glob matching (`internal/pathglob`,
    ADR-0077).
  - `.awf/docs/parts/glossary/terms.md` — add alphabetically-sorted entries: **anchored glob**
    (a pattern matched against the full slash-separated repo-relative path; awf's only glob
    dialect, ADR-0077 — `**/` is the any-depth form) and **domain territory / `paths`** (the
    anchored globs a domain sidecar declares; the `domain-code-staleness` audit rule's input).
  - `.awf/agents-doc.yaml` — add three invariants entries (matching the existing entries' text
    style): the anchored-dialect contract (`*.go` top-level only, no production basename
    matcher; ADR-0077), the `domain-code-staleness` rule contract (advisory Warning, keyed on
    the source part, inert without `paths`), and the schema-7 migration contract (no-slash
    patterns rewritten `**/<pattern>`, idempotent, atomic).
  (The `changelog/CHANGELOG.md` `0.11.0` section already landed in task 1.12, gate-pinned to the
  version bump.)

  Run `./x sync && ./x check` — clean; commit:

```
git add .awf docs AGENTS.md CLAUDE.md
git commit -m "docs(awf): document anchored globs, domain paths, dogfood sidecars"
```

  (Take the exact rendered-file list from `git status`; stage everything sync touched.)

- [ ] **3.4 Retrospective-check the audit dogfood.** Run `go run ./cmd/awf audit` once more —
  expect exit 0 and no `domain-code-staleness` finding (3.3 co-changed the narratives). If a
  warning fires for a domain whose narrative genuinely didn't shift, that is the rule working;
  refresh or accept the advisory note before proceeding.

- [ ] **3.5 Mutation triage (advisory).** `./x mutants` over the branch diff; triage survivors in
  `internal/pathglob`, the rule, and `AnchorNoSlashGlobs`. Trust the run only when it reports
  `Timed out: 0`. No gate; note outcomes in the retrospective.

- [ ] **3.6 Status flip (final commit).** In
  `docs/decisions/0077-anchored-path-globs-and-the-domain-code-staleness-audit-rule.md`:
  `status: Proposed` → `status: Implemented`. Delete the
  `// invariant: invariants-glob-basename` comment line above `TestInvariantGlobValidation` in
  `internal/config/config_test.go` (retirement takes effect with this flip; ADR-0031). Run
  `./x sync` (regenerates `ACTIVE.md` + the three domain indexes). Then:

```
./x gate && ./x check
git add docs/decisions internal/config/config_test.go docs/domains .awf/awf.lock
git commit -m "docs(adr): mark 0077 implemented, retire invariants-glob-basename"
```

- [ ] **3.7 Hand off to `awf-reviewing-impl`** (terminal review), then `awf-retrospective`;
  delete `.awf/memory/domain-code-staleness-audit.md` when the chain terminates.
