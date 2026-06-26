# Plan: Domain Docs with a Generated Per-Domain ADR Index (ADR-0014)

Design & rationale: [docs/decisions/0014-domain-docs-with-generated-adr-index.md](../decisions/0014-domain-docs-with-generated-adr-index.md). Execution record only — link, don't restate.

## Goal

Add a new awf-managed artifact kind: per-domain docs at `docs/domains/<name>.md` combining a hand-authored `current-position` narrative with a generated `decisions` index of every ADR whose new `domains:` frontmatter includes that domain. End state: `awf check` clean, gate green at 100%, ADR-0014 Implemented, all three tagged invariants backed, and this repo dogfooding five domains.

## Architecture summary

- `internal/adr`: parse `domains` + `superseded_by`; `RenderDomainIndex(dir, domain)` returns a markdown index (grouped by status, `../decisions/<file>` links, superseded entries annotated, placeholder when empty) — reusing `RenderActiveMD`'s grouping.
- `internal/config`: optional `Domains []string`; `Validate` name-sanity.
- `internal/project`: `generateDomainDocs()` renders each declared domain via the domain template + convention parts + injected `.data.domain`/`.data.decisions`, returning **content-only** `RenderedFile`s (like `generateActiveMD`). Sync appends them to `files`; Check excludes `docs/domains/*` from the generic hash loop and regenerate-compares each (stale/missing/orphaned); `orphans()`/`declaredSections()` gain a `domains` kind.
- `templates/domains/domain.md.tmpl` (one overlay section `current-position`; the `## Decisions` index is forced body, un-overridable) + a `domainDoc` catalog entry declaring `[current-position]`.
- Workflow: ADR template + README gain `domains:`; `proposing-adr`/`adr-lifecycle` require it and drop the manual table-maintenance step.
- Dogfood: declare 5 domains, retro-tag 13 ADRs, ship per-domain narrative parts.

## Tech stack

Go 1.26. Packages: `internal/adr`, `internal/config`, `internal/catalog`, `internal/project`. Templates under `templates/`. Gate `./x gate` (100% coverage via `cmd/covercheck`); drift `./x check`.

## File structure

**New:** `templates/domains/domain.md.tmpl`; `internal/adr/domain.go` (+ `domain_test.go`); `internal/project/domains_test.go`; `.claude/awf/domains/parts/<domain>/current-position.md` (×5).
**Modified (Go):** `internal/adr/adr.go`, `internal/config/config.go` (+ `config_test.go`), `internal/catalog/catalog.go`, `internal/project/project.go`.
**Modified (templates/docs):** `templates/embed.go` (add `domains` to the `//go:embed` directive), `templates/catalog.yaml`, `docs/decisions/template.md`, `docs/decisions/README.md`, `templates/skills/proposing-adr/SKILL.md.tmpl`, `templates/skills/adr-lifecycle/SKILL.md.tmpl`.
**Modified (config/ADRs):** `.claude/awf/config.yaml`, all 13 `docs/decisions/000N-*.md` (retro-tag), `docs/decisions/0014-*.md` (status flip).
**Re-rendered:** `.claude/skills/awf-proposing-adr/SKILL.md`, `.claude/skills/awf-adr-lifecycle/SKILL.md`, `docs/domains/*.md`, `.claude/awf/awf.lock`, `docs/decisions/ACTIVE.md`.

---

## Phase 1 — `internal/adr`: parse `domains`/`superseded_by` + `RenderDomainIndex`

### Task 1.1 — Extend the ADR frontmatter parse

- [ ] In `internal/adr/adr.go`, replace the `adrFrontmatter` struct and the `ADR` struct fields, and populate them in `parse`. Current:

```go
// ADR is a parsed ADR record.
type ADR struct {
	Number   string            // e.g. "0001"
	Title    string            // e.g. "ADR-0001: Template Overlay Rendering Engine"
	Status   string            // e.g. "Accepted"
	Filename string            // e.g. "0001-template-overlay-rendering-engine.md"
	Path     string            // path as globbed
	Sections map[string]string // `## ` heading -> section body
}
```

Add two fields:

```go
// ADR is a parsed ADR record.
type ADR struct {
	Number       string            // e.g. "0001"
	Title        string            // e.g. "ADR-0001: Template Overlay Rendering Engine"
	Status       string            // e.g. "Accepted"
	Filename     string            // e.g. "0001-template-overlay-rendering-engine.md"
	Path         string            // path as globbed
	Domains      []string          // `domains:` frontmatter (ADR-0014)
	SupersededBy string            // `superseded_by:` frontmatter (e.g. "0008", or "")
	Sections     map[string]string // `## ` heading -> section body
}
```

- [ ] Replace `adrFrontmatter` and the `parse` assembly. Current:

```go
// adrFrontmatter holds the YAML fields we care about.
type adrFrontmatter struct {
	Status string `yaml:"status"`
}
```

with:

```go
// adrFrontmatter holds the YAML fields we care about.
type adrFrontmatter struct {
	Status       string   `yaml:"status"`
	Domains      []string `yaml:"domains"`
	SupersededBy string   `yaml:"superseded_by"`
}
```

- [ ] In `parse`, change the `ADR{...}` literal from `a := ADR{Status: fm.Status, Sections: sections(string(body))}` to:

```go
	a := ADR{Status: fm.Status, Domains: fm.Domains, SupersededBy: fm.SupersededBy, Sections: sections(string(body))}
```

### Task 1.2 — Add `RenderDomainIndex`

- [ ] Create `internal/adr/domain.go`:

```go
package adr

import (
	"fmt"
	"sort"
	"strings"
)

// RenderDomainIndex renders the per-domain ADR index for the decisions directory
// dir: every ADR whose domains frontmatter includes domain, grouped by status in
// the same order as ACTIVE.md, with links relative to docs/domains/ (one dir over)
// and each superseded entry annotated with its successor. Returns a placeholder
// line when no ADR matches, so the rendered section is never empty.
func RenderDomainIndex(dir, domain string) (string, error) {
	adrs, err := ParseDir(dir)
	if err != nil {
		return "", err
	}
	groups := make(map[string][]ADR)
	for _, a := range adrs {
		for _, d := range a.Domains {
			if d == domain {
				groups[a.Status] = append(groups[a.Status], a)
				break
			}
		}
	}
	if len(groups) == 0 {
		return "_No decisions recorded for this domain yet._\n", nil
	}
	for k := range groups {
		sort.Slice(groups[k], func(i, j int) bool { return groups[k][i].Number < groups[k][j].Number })
	}
	var ordered []string
	seen := map[string]bool{}
	for _, s := range statusOrder {
		if len(groups[s]) > 0 {
			ordered = append(ordered, s)
			seen[s] = true
		}
	}
	var extra []string
	for k := range groups {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	ordered = append(ordered, extra...)

	var sb strings.Builder
	for i, status := range ordered {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "### %s\n\n", status)
		for _, a := range groups[status] {
			fmt.Fprintf(&sb, "- [%s](../decisions/%s)", a.Title, a.Filename)
			if a.SupersededBy != "" {
				fmt.Fprintf(&sb, " → superseded by ADR-%s", a.SupersededBy)
			}
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}
```

### Task 1.3 — Tests + back `domain-index-matches-domains`

- [ ] Create `internal/adr/domain_test.go` with a table test that writes a temp decisions dir holding ≥3 ADR files with varied `domains:`/`status:`/`superseded_by:` frontmatter and asserts `RenderDomainIndex`: (a) includes exactly the ADRs whose `domains` contains the queried domain, grouped under the right `### <status>` heading; (b) renders `→ superseded by ADR-NNNN` for a superseded entry; (c) returns the `_No decisions…_` placeholder for a domain with no matches; (d) uses `../decisions/<file>` links. Tag the membership assertion `// invariant: domain-index-matches-domains`.
- [ ] Cover the remaining branches of `RenderDomainIndex` so `./x gate` stays at 100% (mirror `adr_test.go`'s `TestRenderActiveMDSortsWithinStatusAndOrdersExtra` and `TestRenderActiveMDParseError`): (e) **within-status sort** — two ADRs sharing one domain *and* one status, asserting they render in `Number` order (exercises the `sort.Slice` less-func body); (f) **extra-status branch** — an ADR with a status outside `statusOrder` (e.g. `Draft`) that matches the domain, asserting it renders after the known statuses (exercises the `!seen[k]` append + `sort.Strings(extra)`); (g) **parse-error propagation** — a matching domain plus a malformed-frontmatter ADR, asserting `RenderDomainIndex` returns a non-nil error and `""` (exercises the `ParseDir` error return).
- [ ] Also assert `parse` now populates `Domains`/`SupersededBy` (extend an existing `adr_test.go` case or add one).

### Task 1.4 — Verify & commit

- [ ] `./x gate` → `coverage: 100.0%`, `0 issues`. `./x check` → clean (no rendered change).
- [ ] Commit:

```
git add internal/adr/
git commit -m "feat(awf): parse ADR domains/superseded_by and render per-domain index

Adds adr.RenderDomainIndex (grouped by status, ../decisions links, superseded
annotation, empty placeholder) and parses the domains/superseded_by frontmatter.
Backs inv domain-index-matches-domains. ADR-0014."
```

---

## Phase 2 — `internal/config`: `Domains` field + name-sanity validation

### Task 2.1 — Add the field

- [ ] In `internal/config/config.go`, add `Domains` to the `Config` struct after `Docs`:

```go
	Docs       []string         `yaml:"docs"`
	Domains    []string         `yaml:"domains"`
	Invariants *InvariantConfig `yaml:"invariants"`
```

### Task 2.2 — Validate domain names

- [ ] In `config.go` `Validate`, insert a domains loop before the `if c.Invariants != nil` block:

```go
	for _, d := range c.Domains {
		if d == "" {
			return errors.New("domain name must not be empty")
		}
		if strings.ContainsAny(d, "/\\") || strings.Contains(d, "..") {
			return fmt.Errorf("domain %q must not contain path separators or \"..\"", d)
		}
	}
```

### Task 2.3 — Test + back `domain-name-validated`

- [ ] In `internal/config/config_test.go`, add `TestValidateRejectsBadDomainName`: a `Config` with `Domains: []string{"../evil"}` (and a valid `Prefix`) → `Validate` returns an error mentioning the domain; a clean `Domains: []string{"rendering"}` → no error. Cover the empty-name and backslash cases too. Tag `// invariant: domain-name-validated`.

### Task 2.4 — Verify & commit

- [ ] `./x gate` green (100%); `./x check` clean.
- [ ] Commit:

```
git add internal/config/
git commit -m "feat(awf): add config domains array with name-sanity validation

Optional Domains []string (backward-safe, no schema bump); Validate rejects
path separators / \"..\" in a domain name. Backs inv domain-name-validated. ADR-0014."
```

---

## Phase 3 — Domain-doc artifact: template, catalog entry, render + drift wiring

### Task 3.1 — The domain-doc template

- [ ] In `templates/embed.go`, add `domains` to the `//go:embed` directive so the new template is embedded in `templates.FS`. Current:

```go
//go:embed catalog.yaml skills hooks agents agents-doc docs
```

with:

```go
//go:embed catalog.yaml skills hooks agents agents-doc docs domains
```

(Without this, `renderTarget`'s `fs.ReadFile(templates.FS, "domains/domain.md.tmpl")` fails at runtime and `TestDomainDocSectionParity` cannot read the template.)

- [ ] Create `templates/domains/domain.md.tmpl`:

```
# {{ .data.domain }}

<!-- awf:section current-position -->
## Current position

_Describe where this domain stands today: the shape it has settled into, the load-bearing constraints, and anything a newcomer must know before changing it. Refreshed by hand when the position materially shifts._
<!-- awf:end -->

## Decisions

{{ .data.decisions }}
```

The `current-position` narrative is the only overlay section (drop/replaceWith/convention-part). The `## Decisions` block is **forced body** — plain template, outside any `awf:section` marker — so it is always injected and is structurally un-overridable (no marker → no overlay → a convention part cannot replace it). It renders last, beneath the narrative.

### Task 3.2 — Catalog `domainDoc` section taxonomy

- [ ] In `internal/catalog/catalog.go`, add a field to the `Catalog` struct after `AgentsDoc`:

```go
	AgentsDoc SkillSpec            `yaml:"agentsDoc"`
	DomainDoc SkillSpec            `yaml:"domainDoc"`
	Docs      map[string]DocSpec   `yaml:"docs"`
```

- [ ] In `templates/catalog.yaml`, add a top-level `domainDoc` block (sibling of `agents`/`docs`, anywhere among the top-level keys):

```yaml
domainDoc:
  sections:
    - current-position
```

Only `current-position` is a declared (overlay) section. `decisions` is forced body, not a section, so it is **not** listed — which makes `declaredSections("domains")` return `[current-position]`, and `orphans()` flags any other domain part (e.g. a stray `decisions.md`) as orphaned instead of letting it silently shadow the generated index.

### Task 3.3 — `generateDomainDocs` + Sync/Check/orphans wiring

- [ ] In `internal/project/project.go`, add `generateDomainDocs` after `generateActiveMD` (~line 498):

```go
// generateDomainDocs renders one content-only doc per declared domain
// (<docsDir>/domains/<name>.md): the domain template + its convention parts, with
// the per-domain ADR index injected as .data.decisions. Like ACTIVE.md, the result
// carries no TemplateID/Hash — drift is checked by regeneration, since the index
// depends on external ADR frontmatter state.
func (p *Project) generateDomainDocs() ([]RenderedFile, error) {
	decisionsDir := filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")
	var out []RenderedFile
	for _, name := range sortedStrings(p.Cfg.Domains) {
		index, err := adr.RenderDomainIndex(decisionsDir, name)
		if err != nil {
			return nil, err
		}
		data := p.data(config.Sidecar{})
		data["data"] = map[string]any{"domain": name, "decisions": index}
		rf, err := p.renderTarget("domains", name, "domains/domain.md.tmpl",
			p.Cat.DomainDoc.Sections, config.Sidecar{}, data,
			strings.TrimRight(p.Cfg.DocsDir, "/")+"/domains/"+name+".md")
		if err != nil {
			return nil, err
		}
		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content})
	}
	return out, nil
}
```

- [ ] In `Sync` (~line 553), after the `generateActiveMD` append block, add:

```go
	dds, err := p.generateDomainDocs()
	if err != nil {
		return err
	}
	files = append(files, dds...)
```

- [ ] In `Check`, generalize the ACTIVE.md exclusion in the generic loop. Replace:

```go
	activeMdRel := strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md"
	var drift []manifest.Drift
	for _, path := range sortedKeys(lock.Files) {
		if path == activeMdRel {
			continue // generated artifact — checked separately below
		}
```

with:

```go
	activeMdRel := strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md"
	domainsPrefix := strings.TrimRight(p.Cfg.DocsDir, "/") + "/domains/"
	var drift []manifest.Drift
	for _, path := range sortedKeys(lock.Files) {
		if path == activeMdRel || strings.HasPrefix(path, domainsPrefix) {
			continue // generated artifacts — checked separately below
		}
```

- [ ] In `Check`, after the ACTIVE.md regenerate-compare block (before `return drift, nil`), add the domain-doc regenerate-compare + orphan:

```go
	dds, err := p.generateDomainDocs()
	if err != nil {
		return nil, err
	}
	produced := map[string]bool{}
	for _, dd := range dds {
		produced[dd.Path] = true
		onDisk, err := os.ReadFile(filepath.Join(p.Root, dd.Path))
		if err != nil {
			drift = append(drift, manifest.Drift{Path: dd.Path, Kind: "missing", Detail: "domain doc absent; run awf sync"})
		} else if manifest.Hash(onDisk) != manifest.Hash([]byte(dd.Content)) {
			drift = append(drift, manifest.Drift{Path: dd.Path, Kind: "stale", Detail: "domain doc out of date; run awf sync"})
		}
	}
	for path := range lock.Files {
		if strings.HasPrefix(path, domainsPrefix) && !produced[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "domain removed; run awf sync"})
		}
	}
```

- [ ] In `orphans()` (~line 607), add `domains` to the enabled map and the kind loop:

```go
	enabled := map[string]map[string]bool{
		"skills":  sliceSet(p.Cfg.Skills),
		"agents":  sliceSet(p.Cfg.Agents),
		"docs":    sliceSet(p.Cfg.Docs),
		"domains": sliceSet(p.Cfg.Domains),
	}
	var drift []manifest.Drift
	for _, kind := range []string{"skills", "agents", "docs", "domains"} {
```

- [ ] In `declaredSections` (~line 673), add a `domains` case:

```go
	case "docs":
		return p.Cat.Docs[name].Sections
	case "domains":
		return p.Cat.DomainDoc.Sections
```

(adjust to the switch's exact existing shape).

### Task 3.4 — Tests + back `domain-doc-regenerated`

- [ ] Create `internal/project/domains_test.go`:
  - `TestDomainDocRendersIndexAndNarrative`: scaffold a project with `domains: [rendering]`, an `agents-doc.yaml` `local: true`, a `docs/decisions/` holding two ADRs tagged `domains: [rendering]`, and a `current-position.md` part; `Sync`; assert `docs/domains/rendering.md` exists, contains the part's narrative and both ADR links under `## Decisions`, and no `<no value>`.
  - `TestDomainDocStaleOnAdrRetag` (tag `// invariant: domain-doc-regenerated`): after the sync above, append `domains: [rendering]` to a third ADR on disk (without re-sync); `Check` reports `docs/domains/rendering.md` as `stale`.
  - `TestDomainDocOrphanedWhenDomainRemoved`: sync with `[rendering]`, then `Open` a config without it; `Check` reports the orphaned domain doc.
  - `TestDomainDocSectionParity`: assert the `awf:section` marker names in `templates/domains/domain.md.tmpl` equal `catalog.DomainDoc.Sections` — i.e. exactly `[current-position]`; the `## Decisions` block is forced body and carries no marker (mirror `TestDocsSectionParity`).
  - `TestDomainPartOrphan`: a part at `domains/parts/rendering/decisions.md` (named for the forced-body index, which is deliberately not a declared section) → orphan drift — proving the generated index cannot be silently shadowed; likewise `domains/parts/rendering/bogus.md` (undeclared section) → orphan; and a part dir for a non-declared domain → orphan.
  - `TestDomainDocMissingWhenDeleted`: after the sync in `TestDomainDocRendersIndexAndNarrative`, delete `docs/domains/rendering.md` on disk; `Check` reports it `missing` (exercises the `os.ReadFile` error arm of the Check domain block — not hit by the stale/orphan tests).
  - `TestDomainDocRenderError` (or fold into an existing case): scaffold `domains: [rendering]` with a malformed-frontmatter ADR under `docs/decisions/`, call `Sync`; assert it returns an error (exercises `generateDomainDocs`'s `RenderDomainIndex` error return and its propagation through the `Sync` call site).
- [ ] Ensure both arms of every new branch are covered (empty-domains vs populated; stale vs missing vs in-sync; orphan path). For the `renderTarget` error return inside `generateDomainDocs` — unreachable in practice (`.data.domain`/`.data.decisions` are always set so no `<no value>`, and the template is embedded) — add a `// coverage-ignore:` comment with that rationale rather than a contrived test, mirroring the hook-render error in `RenderAll`.

### Task 3.5 — Verify & commit

- [ ] `./x gate` green (100%); `./x sync` (this repo declares no domains yet → no domain docs; catalog gains `domainDoc` but it's inert); `./x check` clean.
- [ ] Commit:

```
git add internal/ templates/ .claude/awf/awf.lock
git commit -m "feat(awf): render per-domain docs with regenerate-and-compare drift

New domains artifact kind: domain.md.tmpl + domainDoc catalog taxonomy +
generateDomainDocs (injects the ADR index as .data.decisions), wired into Sync,
Check (excluded from the hash loop, regenerate-compared like ACTIVE.md, orphan on
removal), and orphans()/declaredSections(). Backs inv domain-doc-regenerated. ADR-0014."
```

---

## Phase 4 — Workflow surfaces

### Task 4.1 — ADR template + README

- [ ] In `docs/decisions/template.md` frontmatter, add `domains: []` after `tags: []`.
- [ ] In `docs/decisions/README.md` `## Frontmatter` YAML block, add a `domains: [rendering]   # coarse domain keys driving docs/domains/<d>.md indexes` line, and one sentence under the block noting `domains:` feeds the per-domain doc indexes.

### Task 4.2 — Skill templates

- [ ] In `templates/skills/proposing-adr/SKILL.md.tmpl`, in the required-frontmatter list, add `domains` (≥1 coarse domain key) alongside the existing fields.
- [ ] In `templates/skills/adr-lifecycle/SKILL.md.tmpl`, change the domain-doc step (currently "Add to the Load-bearing ADRs table; refresh the Current-position prose") to drop the table-maintenance half and keep the prose refresh, e.g.: "Refresh the Current-position prose if the domain's position materially shifts. The `## Decisions` index is generated from each ADR's `domains:` — set that field; do not hand-maintain a decisions table."

### Task 4.3 — Re-sync, verify, commit

- [ ] `./x sync` (re-renders the two skills); `./x check` clean; `./x gate` green.
- [ ] Commit:

```
git add docs/decisions/template.md docs/decisions/README.md templates/skills/ .claude/ 
git commit -m "feat(awf): require ADR domains: and drop the manual decisions table

ADR template + README document the new domains: field; proposing-adr requires it;
adr-lifecycle drops table-maintenance (now auto-generated) and keeps the
current-position prose refresh. ADR-0014."
```

---

## Phase 5 — Dogfood: declare domains, retro-tag ADRs, narratives

### Task 5.1 — Declare domains

- [ ] In `.claude/awf/config.yaml`, add a `domains:` block (alphabetical, sibling of `docs:`):

```yaml
domains:
    - adr-system
    - config
    - invariants
    - rendering
    - tooling
```

### Task 5.2 — Retro-tag the 13 existing ADRs

- [ ] Add a `domains:` line to each ADR's frontmatter (after `tags:`), per this map (0014 already carries its own):

| ADR | `domains:` |
|---|---|
| 0001 | `[rendering]` |
| 0002 | `[tooling]` |
| 0003 | `[tooling]` |
| 0004 | `[adr-system]` |
| 0005 | `[config, adr-system]` |
| 0006 | `[rendering, adr-system]` |
| 0007 | `[invariants]` |
| 0008 | `[invariants]` |
| 0009 | `[config]` |
| 0010 | `[config]` |
| 0011 | `[rendering]` |
| 0012 | `[tooling]` |
| 0013 | `[rendering]` |

### Task 5.3 — Per-domain narrative parts

- [ ] Create `.claude/awf/domains/parts/<domain>/current-position.md` for each of the five domains, each a `## Current position` heading + 2-4 sentences of real current-state prose (e.g. `rendering`: the marker-section overlay engine, `missingkey=zero`, `.layout` vs `.vars`, publication-safety). Keep them honest and brief.

### Task 5.4 — Sync, verify, commit

- [ ] `./x sync` → renders `docs/domains/{adr-system,config,invariants,rendering,tooling}.md`, each with its narrative + populated `## Decisions` index.
- [ ] Confirm no `<no value>` in `docs/domains/*.md`; each index lists the retro-tagged ADRs.
- [ ] `./x check` clean; `./x gate` green.
- [ ] Commit:

```
git add .claude/ docs/domains/ docs/decisions/
git commit -m "feat(awf): dogfood five domain docs and retro-tag all ADRs

Declares adr-system/config/invariants/rendering/tooling; tags every ADR with
domains:; ships per-domain current-position narratives. Generated indexes
populate from the retro-tagged frontmatter. ADR-0014."
```

---

## Phase 6 — Flip ADR-0014 to Implemented

### Task 6.1 — Status flip + verify backings

- [ ] In `docs/decisions/0014-domain-docs-with-generated-adr-index.md`, set `status: Accepted` → `status: Implemented`.
- [ ] `./x sync` (moves 0014 to Implemented in ACTIVE.md, and the `adr-system`/`rendering` domain indexes update to show 0014 Implemented).
- [ ] `./x check` — now enforces 0014's three tagged slugs (`domain-index-matches-domains`, `domain-doc-regenerated`, `domain-name-validated`). Expect clean; if any is reported unbacked, add the missing `// invariant: <slug>` comment to its test.
- [ ] `./x gate` green.
- [ ] Commit:

```
git add docs/decisions/0014-domain-docs-with-generated-adr-index.md docs/decisions/ACTIVE.md docs/domains/ .claude/awf/awf.lock
git commit -m "docs(adr): mark 0014 Implemented"
```

### Task 6.2 — Terminal step

- [ ] Invoke `awf-reviewing-impl` against the Phase 1–6 commit range.

---

## Verification checklist (end state)

- [ ] `docs/domains/{adr-system,config,invariants,rendering,tooling}.md` exist; each `## Decisions` lists its ADRs (status-grouped, `../decisions/` links).
- [ ] Retagging any ADR's `domains:` without `./x sync` makes `./x check` report the affected domain doc `stale`.
- [ ] `./x check` clean; `./x gate` green at 100%; `./x invariants` clean.
- [ ] ADR-0014 `Implemented` in `ACTIVE.md`; its `domains:` index appears under `adr-system` and `rendering`.
