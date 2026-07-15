# Plan: Catalog-trim multiselect at `awf init`

Implements the deferred follow-up slice of **[ADR-0029](../decisions/0029-interactive-agent-prefillable-init.md)**
(design authority: do not duplicate rationale here). The first slice (vars + invariants config)
already shipped; this plan adds the **catalog-trim multiselect** (which skills/docs `awf init`
enables) and **flips ADR-0029 from `Proposed` to `Implemented`**. Predecessor plan:
`docs/plans/2026-06-28-interactive-agent-prefillable-init.md` ("Out of scope" defines this work).

## Goal

Let `awf init` choose which catalog skills/docs to enable, as a **full deselectable** selection over
the curated-core default (ADR-0022). Two `multiselect` descriptors (`skills`, `docs`) whose options
and pre-selected core are computed from the catalog drive three resolution modes already in place:
interactive line prompt (type the complete selection by number; empty = keep core), explicit
`--set skills=tdd,bugfix` / `--answers` (complete verbatim name set), and silent (no selection → core,
byte-identical to today). The silent/default path stays exactly the curated core, preserving
`scaffold-core-only`.

## Design decisions settled in brainstorm (recorded here for the executor; rationale lives in ADR-0029)

- **Full deselectable**, not add-only: a selection *replaces* the enable array verbatim. This is no
  new capability: `awf remove` (ADR-0024) already disables core/chain skills post-init with no
  guardrail; init just front-loads it. The default (no-selection) path is unchanged, so
  `scaffold-core-only` holds. ADR-0029 Decision 4 is amended (Phase 3) to say so explicitly (legal
  while it is still `Proposed`).
- **No guardrail / no link-conversion.** Deselecting a chain skill leaves dangling *prose* references
  in AGENTS.md that ADR-0020's dead-reference check (markdown links only) won't catch: identical to
  the existing `awf remove` exposure, documented in AGENTS.md as user responsibility. Out of scope.
- **`prompt: false` escape-hatch (ADR-0029 Decision 5) is dropped, not implemented.** Every current
  var already has a real descriptor, so the hatch has no consumer; implementing an unused field is
  gold-plating. Phase 3 amends Decision 5 / the parity invariant to require a descriptor for every var
  and record `prompt: false` as a reserved-but-unimplemented affordance.
- **Trim type is `*config.CatalogTrim{Skills, Docs *[]string}`.** A nil pointer (or nil dimension) =
  "no selection → keep core"; a non-nil dimension = verbatim enable set (possibly empty = deselect
  all). Lives in `internal/config` (next to `Skeleton`), so neither `project` nor `cmd/awf` gains an
  import cycle and `cmd/awf` needs no new import (it passes the pointer through by inference).

## Architecture summary

- **Descriptors**: two new `multiselect` entries in `templates/catalog.yaml`'s `vars:` block, with
  `target: catalog-skills` / `catalog-docs`. Their `options` and core-default are **not authored**;
  `initspec.CatalogVars(cat)` fills them from the catalog (all names sorted; `core:true` pre-selected,
  comma-joined into `Default`). `Describe`/`Resolve` operate on the `CatalogVars` output so the option
  list stays derived from the catalog.
- **`internal/initspec`**: `CatalogVars` plus a multiselect path in `Resolve`: explicit answer
  (validated comma-separated names) → interactive prompt (1-based numbers, complete selection) → nil.
  `Resolve` gains a `*config.CatalogTrim` return.
- **`internal/project.ScaffoldConfig`**: gains a trailing `trim *config.CatalogTrim` param; a non-nil
  dimension replaces the core skills/docs verbatim, nil keeps core.
- **`cmd/awf`**: `runInit` computes `descs := initspec.CatalogVars(cat)`, feeds it to `Describe` and
  `Resolve`, and passes the resolved trim to `ScaffoldConfig`. No new flags (`--set`/`--answers`
  already carry `skills=`/`docs=`).

## Tech stack

- Go 1.26; packages touched: `internal/catalog` (data only), `internal/config`, `internal/initspec`,
  `internal/project`, `cmd/awf`. Stdlib only (`strconv`, `slices`, `maps` added to initspec). No new
  dependency.
- Gate: `./x gate` (100% statement coverage) before every commit; `./x check` for drift.

## File structure

Created: none.

Modified:
- `internal/config/edit.go` (new `CatalogTrim` type)
- `internal/project/scaffold.go` (signature + trim overlay + `inv: catalog-trim-applied`)
- `internal/project/scaffold_test.go` (7 call-site updates + a stale-comment fix + trim test)
- `internal/project/descriptor_parity_test.go` (`validTargets`)
- `cmd/awf/list_add_test.go` (1 call-site update)
- `cmd/awf/main.go` (Phase 1: scaffold call; Phase 2: `CatalogVars` wiring + trim pass-through)
- `templates/catalog.yaml` (two multiselect descriptors)
- `internal/initspec/initspec.go` (`CatalogVars`, multiselect resolution, `Resolve` signature)
- `internal/initspec/initspec_test.go` (existing call updates + multiselect tests)
- `cmd/awf/init_test.go` (trim wiring test + describe assertion)
- `docs/decisions/0029-interactive-agent-prefillable-init.md` (amend Decisions 4/5, flip status, add invariant)
- `.awf/domains/parts/tooling/current-state.md`, `.awf/domains/parts/config/current-state.md`, `README.md` (Phase 3)
- `docs/domains/tooling.md`, `docs/domains/config.md`, `docs/decisions/ACTIVE.md`, `.awf/awf.lock` (regenerated, Phase 3)

---

## Phase 1: `ScaffoldConfig` accepts an optional catalog trim

Pure plumbing: the signature changes and the overlay lands, but every call still passes `nil`, so the
output is byte-identical and `scaffold-core-only` is intact. Lands first so Phase 2 has a stable seam.

### Task 1.1: Add the `CatalogTrim` type

- [ ] Edit `internal/config/edit.go`. After the `Skeleton` struct (ends line 25), add:

```go
// CatalogTrim optionally overrides which catalog skills/docs a scaffolded config
// enables (ADR-0029 catalog trim). A nil *CatalogTrim (or a nil dimension within
// it) means "no selection: keep the curated-core default"; a non-nil dimension is
// the verbatim, fully-deselectable enable set (an empty slice deselects all).
type CatalogTrim struct {
	Skills *[]string
	Docs   *[]string
}
```

### Task 1.2: Change `ScaffoldConfig` signature + overlay

- [ ] Edit `internal/project/scaffold.go`. Change the signature (line 22):

```go
func ScaffoldConfig(prefix string, vars map[string]string, inv *config.InvariantConfig, trim *config.CatalogTrim) ([]byte, error) {
```

- [ ] Replace the two `slices.Sort` lines that currently follow the core-skill/doc loops (lines 72-73:
  `slices.Sort(skillNames)` / `slices.Sort(docNames)`) with the trim overlay followed by the sorts:

```go
	// A non-nil trim dimension (ADR-0029 full-deselectable catalog trim) replaces the
	// curated-core default verbatim; nil keeps exactly the core (scaffold-core-only).
	// invariant: catalog-trim-applied
	if trim != nil && trim.Skills != nil {
		skillNames = slices.Clone(*trim.Skills)
	}
	if trim != nil && trim.Docs != nil {
		docNames = slices.Clone(*trim.Docs)
	}
	slices.Sort(skillNames)
	slices.Sort(docNames)
```

  (The `// invariant: scaffold-core-only` marker on the core-building loop above stays put; the
  default `trim == nil` path still yields exactly the core.)

### Task 1.3: Update non-test call site

- [ ] Edit `cmd/awf/main.go`. In `runInit`, change the `ScaffoldConfig` call (line 271) to pass `nil`
  (full trim wiring lands in Phase 2):

```go
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv, nil)
```

### Task 1.4: Update test call sites + add the trim test

- [ ] Edit `internal/project/scaffold_test.go`. Append `, nil` to the trim arg of every
  `ScaffoldConfig(...)` call (lines 23, 53, 98, 127, 163, 205, 238: 7 calls), e.g. line 23 becomes
  `b, err := ScaffoldConfig("example", nil, nil, nil)`. Also update the stale doc comment at line 20
  (`// TestScaffoldParsesCleanly verifies that ScaffoldConfig("example", nil, nil) produces YAML`) to
  the new 4-arg form `ScaffoldConfig("example", nil, nil, nil)`.

- [ ] In the same file, after `TestScaffoldEnablesCoreTargets` (ends line 93), add a test that backs
  `catalog-trim-applied`, covering both `Skills set / Docs nil` and `Skills nil / Docs set` so each of
  the two `trim != nil && trim.X != nil` branches is hit in both outcomes:

```go
// TestScaffoldCatalogTrim asserts a non-nil trim dimension replaces the curated
// core verbatim while a nil dimension keeps the core (full-deselectable trim).
// invariant: catalog-trim-applied
func TestScaffoldCatalogTrim(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	coreDocs := map[string]bool{}
	for name, spec := range cat.Docs {
		if spec.Core {
			coreDocs[name] = true
		}
	}

	// Skills selected verbatim (incl. deselecting core); Docs nil -> keep core.
	pickSkills := []string{"tdd", "brainstorming"}
	b, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Skills: &pickSkills})
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if got := sliceSet(cfg.Skills); !maps.Equal(got, map[string]bool{"tdd": true, "brainstorming": true}) {
		t.Errorf("trim skills = %v, want [brainstorming tdd]", slices.Sorted(maps.Keys(got)))
	}
	if got := sliceSet(cfg.Docs); !maps.Equal(got, coreDocs) {
		t.Errorf("nil docs trim should keep core docs, got %v", slices.Sorted(maps.Keys(got)))
	}

	// Docs deselected to empty; Skills nil -> keep core skills.
	emptyDocs := []string{}
	coreSkills := map[string]bool{}
	for name, spec := range cat.Skills {
		if spec.Core {
			coreSkills[name] = true
		}
	}
	b2, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Docs: &emptyDocs})
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg2, err := config.Load(writeScaffold(t, b2))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfg2.Docs) != 0 {
		t.Errorf("empty docs trim should enable no docs, got %v", cfg2.Docs)
	}
	if got := sliceSet(cfg2.Skills); !maps.Equal(got, coreSkills) {
		t.Errorf("nil skills trim should keep core skills, got %v", slices.Sorted(maps.Keys(got)))
	}
}
```

- [ ] Edit `cmd/awf/list_add_test.go` line 23: `project.ScaffoldConfig("example", nil, nil)` →
  `project.ScaffoldConfig("example", nil, nil, nil)`.

### Task 1.5: Verify and commit

- [ ] Run `go test ./internal/project/ ./internal/config/ ./cmd/awf/ 2>&1 | tail`: expect `ok` for all.
- [ ] Run `./x gate`: expect `coverage: 100.0%` and `0 issues.`
- [ ] Run `./x check`: expect `awf check: clean` (no rendered change; the new `// invariant:` marker is
  not yet declared by an Implemented ADR, so it is an unchecked extra, harmless).
- [ ] Commit:

```
git add internal/config/edit.go internal/project/scaffold.go internal/project/scaffold_test.go cmd/awf/list_add_test.go cmd/awf/main.go
git commit -m "refactor(awf): ScaffoldConfig accepts an optional catalog trim

ADR-0029 catalog-trim Phase 1. ScaffoldConfig(prefix, vars, inv, trim) overlays a
non-nil *config.CatalogTrim dimension over the curated-core skills/docs enable
arrays verbatim; nil keeps exactly the core (scaffold-core-only intact). Every
current caller passes nil, so the output stays byte-identical. Backs the new
inv: catalog-trim-applied via TestScaffoldCatalogTrim."
```

---

## Phase 2: Catalog descriptors + multiselect resolution + CLI wiring

### Task 2.1: Add the two multiselect descriptors

- [ ] Edit `templates/catalog.yaml`. Append after the `invariantsGlobs` descriptor (end of the `vars:`
  block). `options`/`default` are intentionally omitted; `initspec.CatalogVars` computes them from the
  catalog:

```yaml
  - key: skills
    kind: multiselect
    target: catalog-skills
    description: Workflow skills to enable (core pre-selected; deselect to trim or add opt-in skills). Options/default computed from the catalog.
  - key: docs
    kind: multiselect
    target: catalog-docs
    description: Docs to enable (core pre-selected; deselect to trim or add opt-in docs). Options/default computed from the catalog.
```

### Task 2.2: Extend the parity allow-list

- [ ] Edit `internal/project/descriptor_parity_test.go` line 15:

```go
var validTargets = []string{"", "var", "invariants-marker", "invariants-globs", "catalog-skills", "catalog-docs"}
```

  (The two new descriptors carry non-var targets, so they are exempt from the var↔descriptor parity
  check exactly like the invariants marker/globs; only the target allow-list needs them.)

### Task 2.3: `CatalogVars` + multiselect resolution in `initspec`

- [ ] Edit `internal/initspec/initspec.go`. Refresh the package doc comment (lines 1-4): the phrase
  "a resolved (vars, invariants-config) pair" is now a triple; change it to
  "a resolved (vars, invariants-config, catalog-trim) triple".

- [ ] Update the import block to add `maps`, `slices`, `strconv`:

```go
import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)
```

- [ ] Add `CatalogVars` and its helper (place after `Describe`, before `ParseAnswersFile`):

```go
// CatalogVars returns the catalog's value descriptors with the catalog-trim
// multiselect descriptors' Options and Default computed from the catalog itself:
// Options lists every skill (or doc) name sorted, and Default comma-joins the
// curated-core names (the pre-selected set). Other descriptors pass through
// unchanged. Describe and Resolve operate on the returned slice so the trim option
// list stays derived from the catalog (ADR-0029).
func CatalogVars(cat *catalog.Catalog) []catalog.VarDescriptor {
	skills := map[string]bool{}
	for name, spec := range cat.Skills {
		skills[name] = spec.Core
	}
	docs := map[string]bool{}
	for name, spec := range cat.Docs {
		docs[name] = spec.Core
	}
	out := make([]catalog.VarDescriptor, len(cat.Vars))
	for i, d := range cat.Vars {
		switch d.Target {
		case "catalog-skills":
			d.Options, d.Default = namesAndCore(skills)
		case "catalog-docs":
			d.Options, d.Default = namesAndCore(docs)
		}
		out[i] = d
	}
	return out
}

// namesAndCore returns every name (sorted) and the comma-joined subset whose value
// is true (the core, pre-selected set).
func namesAndCore(core map[string]bool) ([]string, string) {
	all := slices.Sorted(maps.Keys(core))
	var coreNames []string
	for _, n := range all {
		if core[n] {
			coreNames = append(coreNames, n)
		}
	}
	return all, strings.Join(coreNames, ",")
}
```

- [ ] Change the `Resolve` signature and body. Replace the whole `Resolve` function (lines 54-103) with:

```go
// Resolve maps descriptors + answers to a vars map, an optional invariants config,
// and an optional catalog trim. For a string/enum descriptor the value is: the
// explicit answer if present; otherwise an interactive prompt (when interactive);
// otherwise empty. A multiselect descriptor resolves to a verbatim selection (see
// resolveMultiselect) routed to the catalog-skills/catalog-docs trim dimension. The
// invariants-marker/globs targets are collected into a *config.InvariantConfig:
// both non-empty -> enabled config; exactly one -> error; neither -> nil.
func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool) (map[string]string, *config.InvariantConfig, *config.CatalogTrim, error) {
	vars := map[string]string{}
	var marker, globs string
	var skillsSel, docsSel *[]string
	r := bufio.NewReader(in)
	for _, d := range descs {
		if d.Kind == "multiselect" {
			sel, err := resolveMultiselect(r, out, d, answers, interactive)
			if err != nil {
				return nil, nil, nil, err
			}
			switch d.Target {
			case "catalog-skills":
				skillsSel = sel
			case "catalog-docs":
				docsSel = sel
			}
			continue
		}
		val, ok := answers[d.Key]
		if !ok {
			if interactive {
				p, err := prompt(r, out, d)
				if err != nil {
					return nil, nil, nil, err
				}
				val = p
			} else {
				val = ""
			}
		}
		switch d.Target {
		case "invariants-marker":
			marker = val
		case "invariants-globs":
			globs = val
		default:
			vars[d.Key] = val
		}
	}

	var gs []string
	for _, g := range strings.Split(globs, ",") {
		if g = strings.TrimSpace(g); g != "" {
			gs = append(gs, g)
		}
	}
	var inv *config.InvariantConfig
	switch {
	case marker == "" && len(gs) == 0:
		// inv stays nil: no invariants config supplied (decide on parsed globs, so
		// a whitespace-only globs value counts as unset).
	case marker == "" || len(gs) == 0:
		return nil, nil, nil, errors.New("initspec: invariantsMarker and invariantsGlobs must be set together")
	default:
		inv = &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: gs, Marker: marker}}}
	}

	var trim *config.CatalogTrim
	if skillsSel != nil || docsSel != nil {
		trim = &config.CatalogTrim{Skills: skillsSel, Docs: docsSel}
	}
	return vars, inv, trim, nil
}

// resolveMultiselect resolves one multiselect descriptor to an optional verbatim
// selection: the explicit answer (comma-separated names, each validated against the
// descriptor's options) if present; an interactive prompt (1-based option numbers
// for the complete desired set) when interactive; nil otherwise. A nil result means
// "no selection: keep the scaffold's curated-core default".
func resolveMultiselect(r *bufio.Reader, out io.Writer, d catalog.VarDescriptor, answers map[string]string, interactive bool) (*[]string, error) {
	if raw, ok := answers[d.Key]; ok {
		sel := splitNames(raw)
		for _, n := range sel {
			if !slices.Contains(d.Options, n) {
				return nil, fmt.Errorf("initspec: %s: unknown option %q", d.Key, n)
			}
		}
		return &sel, nil
	}
	if !interactive {
		return nil, nil
	}
	return promptMultiselect(r, out, d)
}

// promptMultiselect renders the numbered option list (core marked [x]) and reads a
// complete selection as comma-separated 1-based numbers. Empty input keeps the core
// default (nil); an out-of-range or non-numeric token errors.
func promptMultiselect(r *bufio.Reader, out io.Writer, d catalog.VarDescriptor) (*[]string, error) {
	core := map[string]bool{}
	for _, n := range splitNames(d.Default) {
		core[n] = true
	}
	fmt.Fprintf(out, "%s: %s\n", d.Key, d.Description)
	for i, o := range d.Options {
		mark := " "
		if core[o] {
			mark = "x"
		}
		fmt.Fprintf(out, "  %d) [%s] %s\n", i+1, mark, o)
	}
	fmt.Fprint(out, "  enter full selection (comma-sep numbers), empty=keep: ")
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("initspec: read input: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	var sel []string
	for _, tok := range strings.Split(line, ",") {
		if tok = strings.TrimSpace(tok); tok == "" {
			continue
		}
		n, e := strconv.Atoi(tok)
		if e != nil || n < 1 || n > len(d.Options) {
			return nil, fmt.Errorf("initspec: %s: invalid option %q", d.Key, tok)
		}
		sel = append(sel, d.Options[n-1])
	}
	return &sel, nil
}

// splitNames trims and drops empties from a comma-separated string.
func splitNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}
```

### Task 2.4: Wire `CatalogVars` + trim through the CLI

- [ ] Edit `cmd/awf/main.go`, `runInit`. Replace the describe + resolve + scaffold lines so all three
  run over `CatalogVars(cat)` and the trim reaches `ScaffoldConfig`. Specifically:

  - Replace the describe block (lines 239-246) with:

```go
	descs := initspec.CatalogVars(cat)
	if describe {
		out, err := initspec.Describe(descs)
		if err != nil { // coverage-ignore: descriptors marshal to JSON; cannot fail
			return err
		}
		fmt.Fprintln(stdout, string(out))
		return nil
	}
```

  - Change the `Resolve` call (line 260) to take `descs` and the new return:

```go
	vars, inv, trim, err := initspec.Resolve(descs, answers, stdin, stdout, isInteractive())
	if err != nil {
		return err
	}
```

  - Change the `ScaffoldConfig` call (the Phase-1 `nil`, line 271) to pass `trim`:

```go
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv, trim)
```

### Task 2.5: initspec tests

- [ ] Edit `internal/initspec/initspec_test.go`. Update every existing `Resolve(...)` call to capture
  the new fourth return; change `vars, inv, err := Resolve(...)` to `vars, inv, _, err := Resolve(...)`
  in `TestResolveSilentSeedsEmpty`, `TestResolveExplicitAnswersWin`,
  `TestResolveInteractiveDefaultAndEnumIndex`,
  `TestResolveInteractiveLiteralAndEnumNonNumeric`, and the half-set/read-error tests
  (`TestResolvePromptReadError`, `TestResolveInvariantsHalfSetErrors`,
  `TestResolveInvariantsWhitespaceGlobsIsHalfSet`) update
  their `if _, _, err := Resolve(...)` to `if _, _, _, err := Resolve(...)`.

- [ ] Add multiselect tests (append to the file). A shared helper builds descriptors whose options are
  pre-filled the way `CatalogVars` would fill them:

```go
func trimDescs() []catalog.VarDescriptor {
	return []catalog.VarDescriptor{
		{Key: "skills", Kind: "multiselect", Target: "catalog-skills",
			Options: []string{"brainstorming", "bugfix", "tdd"}, Default: "brainstorming"},
		{Key: "docs", Kind: "multiselect", Target: "catalog-docs",
			Options: []string{"testing", "workflow"}, Default: "workflow"},
	}
}

func TestCatalogVarsComputesTrimOptions(t *testing.T) {
	cat := &catalog.Catalog{
		Skills: map[string]catalog.SkillSpec{"brainstorming": {Core: true}, "tdd": {}},
		Docs:   map[string]catalog.DocSpec{"workflow": {Core: true}, "testing": {}},
		Vars: []catalog.VarDescriptor{
			{Key: "gateCmd", Kind: "string"},
			{Key: "skills", Kind: "multiselect", Target: "catalog-skills"},
			{Key: "docs", Kind: "multiselect", Target: "catalog-docs"},
		},
	}
	got := CatalogVars(cat)
	if !slices.Equal(got[1].Options, []string{"brainstorming", "tdd"}) || got[1].Default != "brainstorming" {
		t.Errorf("skills descriptor = %+v", got[1])
	}
	if !slices.Equal(got[2].Options, []string{"testing", "workflow"}) || got[2].Default != "workflow" {
		t.Errorf("docs descriptor = %+v", got[2])
	}
	if got[0].Options != nil { // non-trim descriptor untouched
		t.Errorf("gateCmd descriptor mutated: %+v", got[0])
	}
}

func TestResolveMultiselectSilentKeepsCore(t *testing.T) {
	_, _, trim, err := Resolve(trimDescs(), nil, strings.NewReader(""), &strings.Builder{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if trim != nil {
		t.Errorf("silent trim = %+v, want nil", trim)
	}
}

func TestResolveMultiselectExplicit(t *testing.T) {
	a := map[string]string{"skills": "tdd,brainstorming"}
	_, _, trim, err := Resolve(trimDescs(), a, strings.NewReader(""), &strings.Builder{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if trim == nil || trim.Skills == nil || !slices.Equal(*trim.Skills, []string{"tdd", "brainstorming"}) {
		t.Errorf("trim.Skills = %+v", trim)
	}
	if trim.Docs != nil {
		t.Errorf("docs not answered, want nil dimension, got %+v", trim.Docs)
	}
}

func TestResolveMultiselectExplicitUnknownName(t *testing.T) {
	a := map[string]string{"skills": "nope"}
	if _, _, _, err := Resolve(trimDescs(), a, strings.NewReader(""), &strings.Builder{}, false); err == nil {
		t.Fatal("expected error for unknown option name")
	}
}

func TestResolveMultiselectInteractive(t *testing.T) {
	// skills: "1,3," -> brainstorming,tdd (the trailing empty token exercises the
	// promptMultiselect `tok == ""` continue, required by the 100% coverage gate);
	// docs: empty -> keep core (nil dimension).
	in := strings.NewReader("1,3,\n\n")
	_, _, trim, err := Resolve(trimDescs(), nil, in, &strings.Builder{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if trim == nil || trim.Skills == nil || !slices.Equal(*trim.Skills, []string{"brainstorming", "tdd"}) {
		t.Errorf("trim.Skills = %+v", trim)
	}
	if trim.Docs != nil {
		t.Errorf("empty docs prompt should keep core (nil), got %+v", trim.Docs)
	}
}

func TestResolveMultiselectInteractiveInvalidToken(t *testing.T) {
	for _, line := range []string{"9\n", "x\n"} { // out-of-range, non-numeric
		if _, _, _, err := Resolve(trimDescs(), nil, strings.NewReader(line), &strings.Builder{}, true); err == nil {
			t.Errorf("expected error for input %q", line)
		}
	}
}

func TestResolveMultiselectPromptReadError(t *testing.T) {
	if _, _, _, err := Resolve(trimDescs(), nil, errReader{}, &strings.Builder{}, true); err == nil {
		t.Fatal("expected read error from multiselect prompt")
	}
}
```

  Note for the executor: `TestResolveMultiselectInteractive` feeds `"1,3,\n\n"`. The trailing comma
  yields an empty final token, which deterministically exercises the `tok == ""` `continue` in
  `promptMultiselect`: the only test that does, and required by the hard 100% statement-coverage gate.
  Do not drop the trailing comma; if `./x gate` ever reports that `continue` uncovered, the input was
  altered.

### Task 2.6: CLI trim wiring test + describe assertion

- [ ] Edit `cmd/awf/init_test.go`. In `TestInitDescribeReadOnly`, after the `len(parsed.Descriptors)`
  check (line 49), assert the computed trim options surface (proves `CatalogVars` feeds `Describe`):

```go
	var hasTrimOptions bool
	for _, d := range parsed.Descriptors {
		if d["target"] == "catalog-skills" {
			if opts, ok := d["options"].([]any); ok && len(opts) > 0 {
				hasTrimOptions = true
			}
		}
	}
	if !hasTrimOptions {
		t.Errorf("describe missing computed catalog-skills options:\n%s", out.String())
	}
```

- [ ] Add a trim wiring test (proves `--set skills=`/`docs=` reach the scaffolded config):

```go
// TestInitCatalogTrim asserts --set skills=/docs= drive the scaffolded enable
// arrays verbatim (full-deselectable catalog trim, ADR-0029).
func TestInitCatalogTrim(t *testing.T) {
	root := t.TempDir()
	swapGetwd(t, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	code := run([]string{"awf", "init", "--set", "skills=tdd,brainstorming", "--set", "docs=testing"}, &out, &errb)
	if code != 0 {
		t.Fatalf("init --set trim: exit %d (%s)", code, errb.String())
	}
	cfg := readInitConfig(t, root)
	for _, want := range []string{"skills:", "- brainstorming", "- tdd", "docs:", "- testing"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q:\n%s", want, cfg)
		}
	}
	// A core skill not in the selection must be absent (full-deselectable).
	if strings.Contains(cfg, "- reviewing-impl") {
		t.Errorf("trim should have deselected reviewing-impl:\n%s", cfg)
	}
}
```

### Task 2.7: Verify and commit

- [ ] Run `go test ./... 2>&1 | tail`: expect all `ok`.
- [ ] Run `./x gate`: expect `coverage: 100.0%` and `0 issues.`
- [ ] Run `./x check`: expect `awf check: clean` (catalog.yaml is not drift-tracked; no rendered
  change yet: domain narratives land in Phase 3).
- [ ] Manual smoke (optional): `go run ./cmd/awf init --describe` in a temp dir and confirm the
  `skills`/`docs` descriptors show populated `options`.
- [ ] Commit:

```
git add templates/catalog.yaml internal/initspec/initspec.go internal/initspec/initspec_test.go internal/project/descriptor_parity_test.go cmd/awf/main.go cmd/awf/init_test.go
git commit -m "feat(awf): catalog-trim multiselect at awf init

ADR-0029 catalog-trim Phase 2. Two multiselect descriptors (skills, docs) whose
options/core-default are computed from the catalog by initspec.CatalogVars drive a
full-deselectable trim: interactive (type the complete selection by number),
explicit --set skills=/docs= (verbatim validated names), or silent (nil -> core).
Resolve returns a *config.CatalogTrim wired through runInit into ScaffoldConfig."
```

---

## Phase 3: Flip ADR-0029 to Implemented + docs currency

> This is the slice that completes ADR-0029, so the status flips here and the plan freezes. All edits
> are docs/ADR; the gate passes quickly with no code change.

### Task 3.1: Amend and flip ADR-0029

- [ ] Edit `docs/decisions/0029-interactive-agent-prefillable-init.md`:
  - Change the frontmatter `status: Proposed` → `status: Implemented`.
  - In **Decision 4**, after the sentence ending "beyond that default-case guarantee." append:
    "A trim selection is **full-deselectable**: it replaces the enable array verbatim and may drop
    curated-core targets, mirroring `awf remove` (ADR-0024), which already disables core/chain skills
    post-init with no guardrail. The default (no-selection) path is unchanged, so `scaffold-core-only`
    holds."
  - In **Decision 5**, replace the parenthetical "(or an explicit `prompt: false` descriptor that
    means 'seed empty, never prompt')" with "(`prompt: false` is reserved as a future seed-empty
    affordance; every current var has a descriptor, so it is unimplemented)". Apply the same edit to the
    `inv: var-descriptor-parity` bullet in the **Invariants** section so the parity rule reads as a
    descriptor-required-for-every-var gate with `prompt: false` noted reserved.
  - In the **Invariants** section, add a bullet:
    "- `inv: catalog-trim-applied`: a non-nil catalog-trim dimension passed to `ScaffoldConfig`
    replaces the curated-core skills/docs enable array verbatim (the full-deselectable trim); a nil
    dimension keeps exactly the core."

### Task 3.2: Domain narratives + README

- [ ] Edit `.awf/domains/parts/tooling/current-state.md`. In the sentence that currently ends
  "...the silent non-TTY path still seeds every var empty.", append:
  " Init also offers a full-deselectable catalog trim (ADR-0029): the catalog `vars:` block declares
  two `multiselect` descriptors (`skills`, `docs`) whose options and pre-selected core are computed
  from the catalog, so an operator can deselect core or add opt-in targets interactively or via
  `--set skills=`/`--set docs=`; the selection replaces the enable array verbatim, mirroring
  `awf remove`."

- [ ] Edit `.awf/domains/parts/config/current-state.md`. In the sentence ending "...rather than only
  blank vars.", append:
  " The `vars:` block also carries two `multiselect` catalog-trim descriptors (`skills`/`docs`,
  `target: catalog-skills`/`catalog-docs`) whose options derive from the catalog; `ScaffoldConfig`
  takes an optional `*config.CatalogTrim` that replaces the core skills/docs enable arrays verbatim
  when supplied (nil keeps the curated core)."

- [ ] Edit `README.md` line 80 (the `awf init` table row). Append to the cell, before the closing `|`:
  " `--set skills=`/`--set docs=` trim which catalog skills/docs are enabled (core pre-selected)."

### Task 3.3: Re-render, verify, commit

- [ ] Run `./x sync`: regenerates `docs/decisions/ACTIVE.md` (status flip), `docs/domains/tooling.md`,
  and `docs/domains/config.md` (edited parts).
- [ ] Run `./x check`: expect `awf check: clean` (including the invariants pass: `catalog-trim-applied`
  is now declared by Implemented ADR-0029 and backed by the `// invariant:` marker in scaffold.go).
- [ ] Run `./x gate`: expect `coverage: 100.0%` and `0 issues.`
- [ ] Commit:

```
git add docs/decisions/0029-interactive-agent-prefillable-init.md docs/decisions/ACTIVE.md .awf/domains/parts/tooling/current-state.md .awf/domains/parts/config/current-state.md docs/domains/tooling.md docs/domains/config.md README.md .awf/awf.lock
git commit -m "docs(awf): flip ADR-0029 to Implemented for catalog trim

The catalog-trim multiselect (full-deselectable skills/docs at init) lands,
completing ADR-0029. Amend Decision 4 (full-deselectable, awf remove precedent),
Decision 5 (prompt:false reserved/unimplemented), add inv: catalog-trim-applied,
and refresh the tooling/config domain narratives + README init row."
```

---

## Out of scope (unchanged from ADR-0029)

- **`prompt: false` seed-empty escape-hatch**: reserved in ADR-0029 Decision 5, no current consumer,
  deferred until a var actually needs seed-empty-no-prompt.
- **Chain-coupling guardrails / converting skill cross-references to markdown links** so a deselected
  chain skill is caught by ADR-0020: a separate concern; the exposure already exists via `awf remove`.
- **Detection-based prefill** and **free-text prose authoring** (ADR-0029 Decisions / rejections).
```
