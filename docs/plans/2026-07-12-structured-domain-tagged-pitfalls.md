---
date: 2026-07-12
adrs: [99]
status: Proposed
---
# Plan: Structured Domain-Tagged Pitfalls Surfaced by awf context

Implements ADR-0099: pitfalls become a sidecar-derived, domain-tagged doc surfaced by
`awf context`. The design lives in ADR-0099; this plan is execution only.

## Goal

Turn `docs/pitfalls.md` from a free-prose part blob into a sidecar-derived doc (the ADR-0089 seam)
whose entries are structured `data.pitfalls` in `.awf/docs/pitfalls.yaml`: a transform renders the
doc, a new `awf check` validates the data, `awf context` surfaces pitfalls by their own `domains:`,
and a schema-9 migration auto-splits an adopter's existing part.

## Architecture summary

The `pitfalls` `DocEntry` gains the ADR-0089 treatment the glossary already has: its stub `entries`
section is retired for plain framing sections plus a body referencing `{{ with .data.pitfalls }}`,
and a `pitfallsTransform` (sibling of `glossaryTransform`) computes the entry list into finished
markdown upstream of render+config-hash. A shared entry parser serves three readers in package
`project`: the transform (render), `checkPitfalls` (validate domain + ADR-link resolution), and
`ContextFor` (surface by domain). `awf context` gains a `Pitfalls []PitfallRef` field on the single
`ContextResult`. A schema-9 migration ports adopters; awf's own tree and `examples/sundial` are
hand-converted in-plan (the migration never runs under `./x sync`).

## Tech stack

Go 1.26. Packages touched: `internal/catalog` (DocEntry data), `internal/project` (new
`pitfalls.go`: transform + parser + validation; `context.go`; `check.go`; `project.go` version),
`internal/configspec` (data-key description), `internal/migrate` (new `pitfalls.go` migration +
registry), `cmd/awf` (`context.go` rendering), `templates/docs/pitfalls.md.tmpl`. Config trees:
`.awf/docs/pitfalls.yaml` (new) + `examples/sundial/.awf/docs/pitfalls.yaml` (new). No new external
dependencies (`gopkg.in/yaml.v3`, `manifest.WriteFileAtomic` already vendored). `./x gate` runs
before every commit.

## File structure

- **Created:** `internal/project/pitfalls.go`, `internal/project/pitfalls_test.go`,
  `internal/migrate/pitfalls.go`, `internal/migrate/pitfalls_test.go`, `.awf/docs/pitfalls.yaml`,
  `examples/sundial/.awf/docs/pitfalls.yaml`.
- **Modified:** `templates/docs/pitfalls.md.tmpl`, `internal/catalog/standard.go`,
  `internal/project/glossary.go`, `internal/project/context.go`, `internal/project/check.go`,
  `internal/project/context_test.go`, `internal/project/check_test.go`, `cmd/awf/context.go`,
  `internal/configspec/spec.go`, `internal/migrate/migrate.go`, `internal/project/project.go`,
  `.awf/agents-doc.yaml`, `.awf/docs/glossary.yaml`,
  `.awf/domains/parts/rendering/current-state.md`,
  `.awf/domains/parts/tooling/current-state.md`, `changelog/CHANGELOG.md`,
  `docs/decisions/0099-structured-domain-tagged-pitfalls-surfaced-by-awf-context.md`, this plan.
  Plus the generated rendered outputs, committed via the in-phase `./x sync` steps:
  `docs/pitfalls.md` (the primary output), `docs/config-reference.md` (regenerated from the Task 1.5
  configspec entry), `AGENTS.md` (guide invariants + version), `docs/decisions/ACTIVE.md`,
  `docs/domains/{rendering,tooling}.md`, `docs/glossary.md`, and both `.awf/awf.lock` files.
- **Deleted:** `.awf/docs/parts/pitfalls/entries.md`,
  `examples/sundial/.awf/docs/parts/pitfalls/entries.md`.

---

## Phase 1 — Sidecar-derived pitfalls model (render + validate + convert)

This phase is deliberately large: the DocEntry sections, the template, the transform, the check,
the configspec entry, and the two hand-conversions are mutually dependent — none passes `./x gate`
+ `./x check` without the others (a template referencing `.data.pitfalls` with no transform, or a
converted tree with no model, both drift). It is one coherent change, not a coupled-phase escape —
its own closing commit passes the gate.

- [ ] **Task 1.1 — Add the entry model, parser, structural validation, and transform.** Create
  `internal/project/pitfalls.go`:

  ```go
  package project

  import (
  	"fmt"
  	"maps"
  	"strings"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  )

  // pitfallsSidecarPath names the authoring surface in every pitfalls content error.
  const pitfallsSidecarPath = config.DirName + "/docs/pitfalls.yaml"

  // pitfallEntry is one authored pitfall: a heading title, the optional owning
  // domains that drive awf-context surfacing, optional related ADR numbers, and the
  // markdown body. Shared by the render transform, checkPitfalls, and ContextFor
  // (ADR-0099).
  type pitfallEntry struct {
  	Title   string
  	Domains []string
  	Related []int
  	Body    string
  }

  // pitfallEntries validates data.pitfalls into the ordered entry list. An absent or
  // null key yields nil, nil (the template's else branch renders the placeholder).
  // Structural violations — a non-list value, a non-mapping element, an
  // empty/newline-bearing title, an empty body, a wrong-typed field — are hard errors
  // naming the sidecar. Domain and ADR-link *resolution* is checkPitfalls' job (it
  // needs the project's domains and ADRs); this validates shape only.
  // invariant: pitfall-data-validated
  func pitfallEntries(raw any) ([]pitfallEntry, error) {
  	if raw == nil {
  		return nil, nil
  	}
  	list, ok := raw.([]any)
  	if !ok {
  		return nil, pitfallErr("must be a list of pitfall entries")
  	}
  	out := make([]pitfallEntry, 0, len(list))
  	for i, el := range list {
  		m, err := pitfallStringMap(i, el)
  		if err != nil {
  			return nil, err
  		}
  		e, err := pitfallEntryFrom(i, m)
  		if err != nil {
  			return nil, err
  		}
  		out = append(out, e)
  	}
  	return out, nil
  }

  // pitfallStringMap normalizes the two shapes yaml.v3 hands a mapping element.
  func pitfallStringMap(i int, el any) (map[string]any, error) {
  	switch m := el.(type) {
  	case map[string]any:
  		return m, nil
  	case map[any]any:
  		out := make(map[string]any, len(m))
  		for k, v := range m {
  			ks, isStr := k.(string)
  			if !isStr {
  				return nil, pitfallErr(fmt.Sprintf("entry %d: key %v is not a string", i, k))
  			}
  			out[ks] = v
  		}
  		return out, nil
  	default:
  		return nil, pitfallErr(fmt.Sprintf("entry %d must be a mapping", i))
  	}
  }

  // pitfallEntryFrom validates one mapping into a pitfallEntry.
  func pitfallEntryFrom(i int, m map[string]any) (pitfallEntry, error) {
  	title, err := pitfallString(i, m, "title")
  	if err != nil {
  		return pitfallEntry{}, err
  	}
  	if strings.TrimSpace(title) == "" {
  		return pitfallEntry{}, pitfallErr(fmt.Sprintf("entry %d: title is empty", i))
  	}
  	if strings.Contains(title, "\n") {
  		return pitfallEntry{}, pitfallErr(fmt.Sprintf("entry %d: title %q contains a newline — titles are single-line headings", i, title))
  	}
  	body, err := pitfallString(i, m, "body")
  	if err != nil {
  		return pitfallEntry{}, err
  	}
  	if strings.TrimSpace(body) == "" {
  		return pitfallEntry{}, pitfallErr(fmt.Sprintf("entry %d (%q): body is empty", i, title))
  	}
  	domains, err := pitfallStrings(i, title, m, "domains")
  	if err != nil {
  		return pitfallEntry{}, err
  	}
  	related, err := pitfallInts(i, title, m, "related")
  	if err != nil {
  		return pitfallEntry{}, err
  	}
  	return pitfallEntry{Title: strings.TrimSpace(title), Domains: domains, Related: related, Body: strings.TrimRight(body, "\n")}, nil
  }

  // pitfallString reads a required string field.
  func pitfallString(i int, m map[string]any, key string) (string, error) {
  	v, ok := m[key]
  	if !ok {
  		return "", pitfallErr(fmt.Sprintf("entry %d: missing %q", i, key))
  	}
  	s, isStr := v.(string)
  	if !isStr {
  		return "", pitfallErr(fmt.Sprintf("entry %d: %q must be a string", i, key))
  	}
  	return s, nil
  }

  // pitfallStrings reads an optional list-of-strings field (nil when absent).
  func pitfallStrings(i int, title string, m map[string]any, key string) ([]string, error) {
  	v, ok := m[key]
  	if !ok || v == nil {
  		return nil, nil
  	}
  	list, isList := v.([]any)
  	if !isList {
  		return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q must be a list", i, title, key))
  	}
  	out := make([]string, 0, len(list))
  	for _, el := range list {
  		s, isStr := el.(string)
  		if !isStr || strings.TrimSpace(s) == "" {
  			return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q entries must be non-empty strings", i, title, key))
  		}
  		out = append(out, strings.TrimSpace(s))
  	}
  	return out, nil
  }

  // pitfallInts reads an optional list-of-ints field (nil when absent).
  func pitfallInts(i int, title string, m map[string]any, key string) ([]int, error) {
  	v, ok := m[key]
  	if !ok || v == nil {
  		return nil, nil
  	}
  	list, isList := v.([]any)
  	if !isList {
  		return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q must be a list", i, title, key))
  	}
  	out := make([]int, 0, len(list))
  	for _, el := range list {
  		n, isInt := el.(int)
  		if !isInt {
  			return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q entries must be ADR numbers", i, title, key))
  		}
  		out = append(out, n)
  	}
  	return out, nil
  }

  // pitfallsTransform replaces data.pitfalls — the authored entry list — with the
  // finished markdown, mirroring glossaryTransform (ADR-0089). An absent key is left
  // untouched; a null/empty list yields "" so the template's else branch renders the
  // placeholder.
  func pitfallsTransform(sc config.Sidecar) (config.Sidecar, error) {
  	raw, ok := sc.Data["pitfalls"]
  	if !ok {
  		return sc, nil
  	}
  	entries, err := pitfallEntries(raw)
  	if err != nil {
  		return sc, err
  	}
  	out := sc
  	out.Data = maps.Clone(sc.Data)
  	out.Data["pitfalls"] = pitfallsMarkdown(entries)
  	return out, nil
  }

  // pitfallsMarkdown renders entries in authored order (a YAML sequence is already
  // deterministic — no sort needed, unlike the glossary map). Each entry is a
  // `## <title>` section, an optional italic Domains line, an optional italic Related
  // line of plain ADR-NNNN references (the transform cannot resolve numbers to
  // filenames, so these are text not links), and the body.
  func pitfallsMarkdown(entries []pitfallEntry) string {
  	if len(entries) == 0 {
  		return ""
  	}
  	var b strings.Builder
  	for i, e := range entries {
  		if i > 0 {
  			b.WriteString("\n")
  		}
  		fmt.Fprintf(&b, "## %s\n\n", e.Title)
  		if len(e.Domains) > 0 {
  			fmt.Fprintf(&b, "_Domains: %s_\n\n", strings.Join(e.Domains, ", "))
  		}
  		if len(e.Related) > 0 {
  			refs := make([]string, len(e.Related))
  			for j, n := range e.Related {
  				refs[j] = fmt.Sprintf("ADR-%04d", n)
  			}
  			fmt.Fprintf(&b, "_Related: %s_\n\n", strings.Join(refs, ", "))
  		}
  		b.WriteString(e.Body)
  		b.WriteString("\n")
  	}
  	return b.String()
  }

  // pitfallErr prefixes every content violation with the authoring surface.
  func pitfallErr(msg string) error {
  	return fmt.Errorf("%s data.pitfalls: %s", pitfallsSidecarPath, msg)
  }
  ```

- [ ] **Task 1.2 — Dispatch the transform.** In `internal/project/glossary.go`, extend
  `docDataTransform` to route `pitfalls` and update its comment's last sentence to "The glossary and
  pitfalls docs compute today.":

  ```
  	func docDataTransform(name string, sc config.Sidecar) (config.Sidecar, error) {
  -		if name != "glossary" {
  -			return sc, nil
  +		switch name {
  +		case "glossary":
  +			return glossaryTransform(sc)
  +		case "pitfalls":
  +			return pitfallsTransform(sc)
  +		default:
  +			return sc, nil
  		}
  -		return glossaryTransform(sc)
  	}
  ```

- [ ] **Task 1.3 — Retire the stub template.** Replace `templates/docs/pitfalls.md.tmpl` entirely:

  ```
  # Pitfalls

  <!-- awf:section prepend -->
  <!-- awf:end -->

  {{ with .data.pitfalls }}{{ . }}{{ else }}_No pitfalls recorded yet — add entries under `data.pitfalls` in `.awf/docs/pitfalls.yaml`._
  {{ end }}
  <!-- awf:section append -->
  <!-- awf:end -->
  ```

- [ ] **Task 1.4 — Update the DocEntry sections.** In `internal/catalog/standard.go`:

  ```
  -		"pitfalls":     {Title: "Pitfalls", Desc: "recurring bugs and tricky areas", Sections: []string{"entries"}, TID: "docs/pitfalls.md.tmpl"},
  +		"pitfalls":     {Title: "Pitfalls", Desc: "recurring bugs and tricky areas", Sections: []string{"prepend", "append"}, TID: "docs/pitfalls.md.tmpl"},
  ```

- [ ] **Task 1.5 — Describe the new data key.** In `internal/configspec/spec.go`, add to the
  `dataKeys` table (near the glossary `terms` entry):

  ```go
  	{Kind: "docs", Artifact: "pitfalls", Key: "pitfalls", Description: "The pitfalls as an ordered list of `{title, domains, related, body}` entries; the doc renders each as a `## title` section (an empty/newline title or empty body fails the render), `domains` (optional) drive `awf context` surfacing and must resolve to configured domains, and `related` (optional) ADR numbers must resolve to real ADRs. Unset, the doc renders a pointer telling you where to add entries."},
  ```

- [ ] **Task 1.6 — Add the check.** In `internal/project/check.go`, add `checkPitfalls` (model on
  `checkPlans`) and call it from `Check()` next to the `checkPlans()` call:

  ```go
  // checkPitfalls validates the pitfalls sidecar when the doc is enabled: each entry's
  // domains: must resolve to a configured domain, and each related: number to an
  // existing ADR. Structural validation (title/body) is the transform's job; this
  // resolves the links the transform cannot see. A disabled pitfalls doc, or a sidecar
  // with no data.pitfalls, yields no drift.
  // invariant: pitfall-domains-resolved
  // invariant: pitfall-adr-link-resolved
  func (p *Project) checkPitfalls() ([]manifest.Drift, error) {
  	if !slices.Contains(p.Cfg.Docs, "pitfalls") {
  		return nil, nil
  	}
  	sc, err := p.Cfg.Sidecar("docs", "pitfalls")
  	if err != nil {
  		return nil, err
  	}
  	entries, err := pitfallEntries(sc.Data["pitfalls"])
  	if err != nil {
  		return nil, err
  	}
  	domains := map[string]bool{}
  	for _, d := range p.Cfg.Domains {
  		domains[d] = true
  	}
  	adrs, err := adr.ParseDir(p.decisionsDir())
  	if err != nil {
  		return nil, err
  	}
  	known := map[string]bool{}
  	for _, a := range adrs {
  		known[a.Number] = true
  	}
  	var drift []manifest.Drift
  	for _, e := range entries {
  		for _, d := range e.Domains {
  			if !domains[d] {
  				drift = append(drift, manifest.Drift{Path: pitfallsSidecarPath, Kind: "pitfall-domain", Detail: fmt.Sprintf("%q: unknown domain %q", e.Title, d)})
  			}
  		}
  		for _, n := range e.Related {
  			if !known[fmt.Sprintf("%04d", n)] {
  				drift = append(drift, manifest.Drift{Path: pitfallsSidecarPath, Kind: "pitfall-adr-link", Detail: fmt.Sprintf("%q: ADR-%04d", e.Title, n)})
  			}
  		}
  	}
  	return drift, nil
  }
  ```

  Wire it in `Check()`:

  ```
  	drift = append(drift, planDrift...)
  +	pitfallDrift, err := p.checkPitfalls()
  +	if err != nil { // coverage-ignore: every error checkPitfalls returns is pre-empted by an earlier Check() step (RenderAll's transform reads data.pitfalls; checkPlans parses the decisions dir), so this wiring branch is unreachable
  +		return nil, err
  +	}
  +	drift = append(drift, pitfallDrift...)
  ```

  (`adr`, `slices`, `fmt`, `manifest` are already imported — `checkPlans`/`unusedDataDrift` use them.)
  The `// coverage-ignore:` is required: `checkPitfalls`'s own error returns are covered by the
  Task 1.9 unit tests that call it directly, but this `Check()` wiring branch cannot be reached
  because `RenderAll` (structural + sidecar-read errors) and `checkPlans` (ADR-parse error) run
  first — mirroring the `unusedDataDrift` wiring ignore. Without it, Task 1.10's gate fails the
  100% floor (ADR-0012).

- [ ] **Task 1.7 — Convert awf's own pitfalls (batch).** Create `.awf/docs/pitfalls.yaml` by
  converting every top-level `## ` section of `.awf/docs/parts/pitfalls/entries.md` into a
  `data.pitfalls` list entry, then delete the part. Representative transformation — the first entry
  becomes:

  ```yaml
  data:
    pitfalls:
      - title: "`awf audit` and `extensions.worktreeConfig`"
        domains: [tooling]
        body: |
          `git.PlainOpen` (go-git) refuses to open a repo whose `.git/config` has
          `extensions.worktreeConfig = true` ...
          (the section's body verbatim, dedented into the block scalar)
  ```

  Affected-site set: every top-level `## ` heading in `.awf/docs/parts/pitfalls/entries.md`
  (`grep -c '^## ' .awf/docs/parts/pitfalls/entries.md`). Mechanical part: heading text → `title`
  (strip surrounding backticks only where they wrap the whole heading; keep inline backticks),
  section body → `body` block scalar verbatim. Judgment part: tag each entry's `domains:` from the
  five configured domains (`rendering`, `config`, `invariants`, `tooling`, `adr-system`) by reading
  the entry — a genuinely cross-cutting/process pitfall (e.g. parallel worktrees) gets no `domains:`;
  set `related:` only where the body already cites a specific ADR as the pitfall's origin. Then
  `rm .awf/docs/parts/pitfalls/entries.md && rmdir .awf/docs/parts/pitfalls`.

  Post-check: `go run ./cmd/awf sync && go run ./cmd/awf check` clean;
  `git diff --stat docs/pitfalls.md` shows the rendered doc changed only in framing (section bodies
  byte-preserved modulo the added `_Domains:_`/`_Related:_` lines); no drift/`note:` line mentions
  `pitfalls`.

- [ ] **Task 1.8 — Convert the example adopter's pitfalls.** Unlike awf's own file, sundial's
  `examples/sundial/.awf/docs/parts/pitfalls/entries.md` is a single `## Entries` heading followed by
  a two-bullet list (`time.Now()` in tests; longitude sign confusion) — *not* one `## ` per pitfall.
  Split the two bullets into two titled `data.pitfalls` entries in
  `examples/sundial/.awf/docs/pitfalls.yaml`, e.g.:

  ```yaml
  data:
    pitfalls:
      - title: "`time.Now()` in tests"
        domains: [almanac]
        body: |
          The sun table depends on the date; a test that formats "today" goes red twice
          a year at the solstices. Fix the date with `time.Date`.
      - title: "Longitude sign confusion"
        domains: [almanac, cli]
        body: |
          East is positive; a flipped sign shifts solar noon by minutes per degree and
          looks like a model bug (it isn't — check the input first).
  ```

  Domains are drawn from sundial's configured `[almanac, cli]` (in `examples/sundial/.awf/config.yaml`).
  Delete the example's `parts/pitfalls/entries.md` and its now-empty dir. Post-check: the `./x check`
  example step is clean and note-free (ADR-0090 `example-zero-notes`).

- [ ] **Task 1.9 — Add tests.** Create `internal/project/pitfalls_test.go` covering `pitfallEntries`
  (valid list; nil/absent → nil; non-list; non-mapping element; empty title; newline title; empty
  body; missing field; wrong field types; domains/related happy + wrong-type paths), `pitfallsMarkdown`
  (empty → ""; single entry; domains line; related line; multi-entry spacing), and `pitfallsTransform`
  (absent key untouched; null/empty → ""; clone-not-mutate, mirroring
  `TestGlossaryTransformClonesData`). Add `checkPitfalls` cases to `internal/project/check_test.go`:
  doc disabled → nil; unknown domain → `pitfall-domain` drift; dangling `related` → `pitfall-adr-link`
  drift; clean sidecar → no drift; structural error propagates. Reach the 100% floor on the new code.

- [ ] **Task 1.10 — Gate and commit.** `./x gate` and `./x check` clean. Commit:

  ```
  feat(rendering): model pitfalls as a sidecar-derived doc

  Retire the pitfalls stub part for a data.pitfalls entry list rendered by a
  transform (ADR-0089 seam, ADR-0099): structured {title, domains, related, body}
  entries, validated at render (structure) and awf check (domain + ADR-link
  resolution). Convert awf's own and examples/sundial's pitfalls to the sidecar and
  drop the part blobs.
  ```

## Phase 2 — Surface pitfalls in awf context

- [ ] **Task 2.1 — Add the ref type and field.** In `internal/project/context.go`, add `PitfallRef`
  near `PlanRef` and a `Pitfalls` field on `ContextResult`:

  Adding `[]PitfallRef` (wider than the current widest type `[]DomainRef`) reflows every struct
  tag under gofmt, so write the whole struct with the shifted alignment (not a partial hunk):

  ```go
  type ContextResult struct {
  	Paths      []string     `json:"paths"`
  	Domains    []DomainRef   `json:"domains"`
  	Invariants []string      `json:"invariants"`
  	ADRs       []ADRRef      `json:"adrs"`
  	Plans      []PlanRef     `json:"plans"`
  	Pitfalls   []PitfallRef  `json:"pitfalls"`
  	Unowned    []string      `json:"unowned"`
  }
  ```

  (Run `./x fmt` after editing — the exact column widths are whatever gofmt produces; the point is
  the committed diff must be gofmt-clean.)

  ```go
  // PitfallRef is a pitfall surfaced because one of its domains: owns a queried path.
  // Path is the docsDir-rooted pitfalls doc; Domains are the entry's own tags.
  type PitfallRef struct {
  	Title   string   `json:"title"`
  	Domains []string `json:"domains"`
  	Path    string   `json:"path"`
  }
  ```

- [ ] **Task 2.2 — Add the reader.** In `ContextFor`, after the plans block and before the final
  `return`, surface pitfalls when the doc is enabled:

  ```go
  	// Surface pitfalls whose own domains: owns a queried path (like ADRs, not
  	// transitively like plans). Only when the toggleable pitfalls doc is enabled.
  	// invariant: context-surfaces-pitfalls
  	if slices.Contains(p.Cfg.Docs, "pitfalls") {
  		sc, err := p.Cfg.Sidecar("docs", "pitfalls")
  		if err != nil {
  			return ContextResult{}, err
  		}
  		entries, err := pitfallEntries(sc.Data["pitfalls"])
  		if err != nil {
  			return ContextResult{}, err
  		}
  		for _, e := range entries {
  			for _, d := range e.Domains {
  				if owners[d] {
  					res.Pitfalls = append(res.Pitfalls, PitfallRef{
  						Title: e.Title, Domains: e.Domains,
  						Path: lay.DocsDir + "/pitfalls.md",
  					})
  					break
  				}
  			}
  		}
  		sort.Slice(res.Pitfalls, func(i, j int) bool { return res.Pitfalls[i].Title < res.Pitfalls[j].Title })
  	}
  ```

  Add `"slices"` to the `context.go` import block.

- [ ] **Task 2.3 — Render the human output.** In `cmd/awf/context.go` `printContext`, add a section
  after the plans section (JSON encodes `res` whole — no change, preserving `context-output-parity`):

  ```
  		if len(res.Pitfalls) > 0 {
  			fmt.Fprintln(stdout, "\n## Related pitfalls")
  			for _, pf := range res.Pitfalls {
  				fmt.Fprintf(stdout, "  %s %v — %s\n", pf.Title, pf.Domains, pf.Path)
  			}
  		}
  ```

- [ ] **Task 2.4 — Add tests.** Extend `internal/project/context_test.go`: a pitfall whose domain
  owns a queried path surfaces; a domainless pitfall never surfaces; a pitfall whose domain does not
  own the path does not surface; disabled pitfalls doc → empty `Pitfalls`; a structural-error sidecar
  propagates; **and a Sidecar-read error propagates** (e.g. `pitfalls.yaml` present as a directory) —
  `ContextFor` has two distinct error branches (the `Sidecar` read and `pitfallEntries`), both
  coverable here because `ContextFor` runs standalone before `RenderAll`. Assert `--json` includes the
  `pitfalls` array (output-parity) via the existing JSON test pattern.

- [ ] **Task 2.5 — Gate and commit.** `./x gate` + `./x check` clean. Commit:

  ```
  feat(tooling): surface pitfalls in awf context

  ContextResult gains Pitfalls []PitfallRef; ContextFor reads the pitfalls sidecar
  (when the doc is enabled) and surfaces each entry whose own domains: owns a queried
  path, on the single read-only ContextResult — human + --json in parity (ADR-0099).
  ```

## Phase 3 — Adopter migration + schema bump

- [ ] **Task 3.1 — Write the migration.** Create `internal/migrate/pitfalls.go` with
  `applyPitfallsData`: read `<awf>/docs/parts/pitfalls/entries.md`; if present, split it on top-level
  `## ` headings *outside fenced code blocks* into `data.pitfalls` entries (`title` from the heading,
  `body` from the section, `domains`/`related` omitted), write `<awf>/docs/pitfalls.yaml` via
  `manifest.WriteFileAtomic`, delete the part file and its now-empty dir, print one
  `pitfalls-data: split entry "<title>"` provenance line per created entry, and close with
  `pitfalls-data: review .awf/docs/pitfalls.yaml and tag each entry's domains: (untagged entries do
  not surface in awf context)`. Idempotent: an absent part file is a no-op (no output, no write).
  Atomic: the sidecar write only fires when a part was split. Track fenced state by toggling on lines
  whose trimmed prefix is a triple backtick. Model the file-op structure and provenance on
  `applySingletonStandardDocs`.

- [ ] **Task 3.2 — Register it and bump the schema.** In `internal/migrate/migrate.go`:

  ```
  		{To: 8, Name: "close-enabled-set", Apply: applyCloseEnabledSet},
  +		{To: 9, Name: "pitfalls-data", Apply: applyPitfallsData},
  ```

  In `internal/project/project.go` (ADR-0049 — a schema bump requires both the version and the floor):

  ```
  -	const Version = "0.16.0"
  +	const Version = "0.17.0"
  ```
  ```
  	var minVersionBySchema = map[int]string{
  		6: "0.6.0",
  		7: "0.11.0",
  		8: "0.12.0",
  +		9: "0.17.0",
  	}
  ```

- [ ] **Task 3.3 — Add migration tests.** Create `internal/migrate/pitfalls_test.go`: a tree with a
  two-`##`-entry part splits into a two-entry `pitfalls.yaml`, the part file and dir are gone, and the
  provenance + review lines print; a `##` inside a fenced block does not split; a tree with no part is
  a clean no-op (no output, no file); a re-run after a prior split is a no-op. Reach the 100% floor.

- [ ] **Task 3.4 — Restamp both trees to schema 9.** Registering `To: 9` puts every schema-8 tree
  into the "gate" state, so `sync`/`check` refuse until upgraded. Run `go run ./cmd/awf upgrade`
  (awf's own tree — the migration no-ops since Phase 1 removed the part; it restamps the lock to 9)
  and `go build -o /tmp/awf ./cmd/awf && (cd examples/sundial && /tmp/awf upgrade)`. Both
  `.awf/awf.lock` files now read `schemaVersion: 9`. Then `./x sync` to refresh hashes.

- [ ] **Task 3.5 — Gate and commit.** `./x gate` + `./x check` clean. Commit:

  ```
  feat(config): schema-9 auto-split migration for adopter pitfalls

  applyPitfallsData splits an adopter's entries.md part into data.pitfalls (fenced ##
  lines skipped), atomically, printing per-entry provenance and a review instruction,
  then deletes the part — idempotent. Bumps schema to 9 with the matching version
  floor (ADR-0049, ADR-0099); restamps awf's own and sundial's locks.
  ```

## Phase 4 — Doc currency + status freeze

- [ ] **Task 4.1 — Guide invariants.** In `.awf/agents-doc.yaml` `data.invariants`, add four entries
  (ref + text) for `pitfall-data-validated`, `pitfall-domains-resolved`, `pitfall-adr-link-resolved`,
  and `context-surfaces-pitfalls`, each citing ADR-0099, in the style of the existing entries.

- [ ] **Task 4.2 — Domain current-state.** Update `.awf/domains/parts/rendering/current-state.md`
  (the pitfalls doc is now sidecar-derived like the glossary — a second transform occupant) and
  `.awf/domains/parts/tooling/current-state.md` (`awf context` surfaces pitfalls; `awf check` gains
  pitfall validation; schema is 9). Prefer count-free phrasing (per the pitfall about hard-coded
  counts in domain narratives).

- [ ] **Task 4.3 — Changelog.** Add a `## [Unreleased]` entry to `changelog/CHANGELOG.md` under
  **Breaking changes** (the pitfalls doc model changed; adopters upgrade) and **Features**
  (`awf context` surfaces pitfalls). State the migration: `awf upgrade` auto-splits `entries.md` into
  `data.pitfalls` with empty domains and prints a review instruction; adopters then tag domains, and
  should review any pitfall whose body had `##` inside fenced code.

- [ ] **Task 4.4 — Glossary.** ADR-0099's Downstream list names the glossary. Add a term to
  `.awf/docs/glossary.yaml` `data.terms` for the new load-bearing concept — e.g. `"pitfall entry":`
  defining the structured `{title, domains, related, body}` unit of `data.pitfalls`, that it renders
  a `## title` section and surfaces in `awf context` by its `domains:` (cite ADR-0099) — in the style
  of the existing entries. (The `sidecar-derived doc` term already exists from ADR-0089; this adds the
  pitfall-specific unit, not a duplicate.)

- [ ] **Task 4.5 — Flip statuses (the freeze).** In
  `docs/decisions/0099-structured-domain-tagged-pitfalls-surfaced-by-awf-context.md` set `status:
  Proposed` → `status: Implemented`. In this plan's frontmatter set `status: Proposed` →
  `status: Implemented`. Run `./x sync` to regenerate `docs/decisions/ACTIVE.md`. `./x check` clean —
  this is where the four `inv:` markers are enforced (ADR-0099 now Implemented); confirm each is
  backed.

- [ ] **Task 4.6 — Gate and commit.** `./x gate`, `./x check`, and `./x audit-local` clean. Commit:

  ```
  docs(rendering): implement ADR-0099 — flip status, currency

  Guide invariants (4 new slugs), rendering/tooling current-state, and the changelog
  [Unreleased] entry with the migration recipe. Flip ADR-0099 and this plan to
  Implemented; regenerate ACTIVE.md.
  ```

## Verification

- `./x gate` and `./x check` clean at every phase commit; the `./x check` example step is note-free
  (ADR-0090 `example-zero-notes`).
- `go run ./cmd/awf context internal/git/git.go` (a tooling-owned path) lists the relevant pitfall
  under "## Related pitfalls"; `--json` includes the same entry in a `pitfalls` array.
- `docs/pitfalls.md` renders every converted entry with its body intact; a domainless entry renders
  but never surfaces in `awf context`.
- A throwaway tree with a `##`-delimited `entries.md` under `.awf/docs/parts/pitfalls/`, run through
  `awf upgrade`, yields a `data.pitfalls` sidecar, no part file, per-entry provenance, and the review
  instruction.
- After the final commit, ADR-0099 and this plan both read `status: Implemented`; the four `inv:`
  slugs are backed (`./x check` clean).

## Notes

- **`related:` renders as plain `ADR-NNNN` text, not hyperlinks (RESOLVED, user 2026-07-12).** Plain
  text is the established convention across exactly these surfaces, not a compromise: `awf context`
  cites ADRs as plain `ADR-NNNN` + path (terminal text, no link), the glossary cites them plain
  in-cell, and the pitfall bodies themselves already carry 57 plain `ADR-NNNN` citations and zero
  markdown links. So `_Related: ADR-0067, ADR-0092_` matches how every pitfall body and `awf context`
  already reference ADRs. The transform also cannot resolve numbers to filenames (it receives only
  the sidecar). **Resync action:** soften ADR-0099 item 2 + its Alternatives row from "linked ADR
  references" to "ADR references (link-validated)".
- Structural validation lives in the transform (`pitfall-data-validated`); domain/ADR-link resolution
  lives in `checkPitfalls` (`pitfall-domains-resolved`, `pitfall-adr-link-resolved`) — the split
  mirrors glossary-structure-in-transform vs plan-links-in-checkPlans, because the transform cannot
  see the project's domain set or ADRs.
- Deferred (ADR-0099 Consequences): `awf new pitfall` (N/A for a singleton doc) and a `promotion:`
  field (must not become an auto-promotion signal without revisiting ADR-0067).
