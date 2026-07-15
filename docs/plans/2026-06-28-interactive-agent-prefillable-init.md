# Plan: Interactive and agent-prefillable `awf init`

Implements **[ADR-0029](../decisions/0029-interactive-agent-prefillable-init.md)** (design authority: do
not duplicate rationale here). Scope per the brainstorm decision: **vars + invariants config**;
the catalog-trim multiselect is **deferred** to a follow-up plan (ADR-0029 stays `Proposed` until
that lands; this plan does **not** flip it to `Implemented`).

## Goal

Give `awf init` value metadata so it can (1) prompt a human on a TTY, (2) self-describe its fillable
values as JSON for an agent (`--describe`), and (3) accept values non-interactively
(`--set k=v` / `--answers file`). The silent non-TTY no-answers path stays byte-identical to today
(seed every var empty). Fillable values this plan covers: the 11 template vars and the invariants
backing config (marker + globs).

## Architecture summary

- **Descriptors live in the catalog.** A new top-level `vars:` block in `templates/catalog.yaml` holds
  an ordered list of `{key, kind, description, default, options, target}` descriptors, parsed into
  `catalog.Catalog.Vars`. `target` routes a value to either a config var (`""`/`var`) or the
  invariants config (`invariants-marker` / `invariants-globs`).
- **`internal/initspec`** is a new package: resolves answers against descriptors (explicit answer →
  interactive prompt → empty), emits the `--describe` JSON, and parses `--answers`/`--set` inputs.
  Pure except for a line-based stdlib prompter (no TUI dependency).
- **`ScaffoldConfig`** changes from `(prefix)` to `(prefix, vars, inv)`: it still seeds every
  referenced var (ADR-0022 `scaffold-seeds-all-vars`), overlays resolved values, and writes the
  invariants config when supplied.
- **CLI** (`cmd/awf/main.go`) wires the flags, TTY detection (stdlib `os.Stdin.Stat()` +
  `ModeCharDevice`, via overridable package vars), and the three resolution modes into `runInit`.

## Tech stack

- Go 1.26; packages touched: `internal/catalog`, new `internal/initspec`, `internal/config`,
  `internal/project`, `cmd/awf`. Stdlib only (`bufio`, `encoding/json`, `gopkg.in/yaml.v3` already a
  dep). No new dependency.
- Gate: `./x gate` (100% coverage) before every commit; `./x check` for drift.

## File structure

Created:
- `internal/initspec/initspec.go`
- `internal/initspec/initspec_test.go`
- `internal/project/descriptor_parity_test.go`

Modified:
- `templates/catalog.yaml` (new `vars:` block)
- `internal/catalog/catalog.go` (`VarDescriptor` type + `Vars` field)
- `internal/config/edit.go` (`Skeleton.Invariants`)
- `internal/project/scaffold.go` (signature + overlay)
- `internal/project/scaffold_test.go`, `cmd/awf/list_add_test.go` (call-site updates)
- `cmd/awf/main.go` (flags, helpers, `runInit`, help text)
- `cmd/awf/run_test.go` (or a new init test file: see Phase 4)
- `.awf/domains/parts/tooling/current-state.md`, `.awf/domains/parts/config/current-state.md`, `README.md` (Phase 5)

---

## Phase 1: Catalog value descriptors + parity gate

### Task 1.1: Add the `VarDescriptor` type and `Vars` field

- [ ] Edit `internal/catalog/catalog.go`. After the `DocSpec` struct (line 33), add:

```go
// VarDescriptor describes one fillable init value: a config var, or (via Target)
// the invariants backing config. Kind ∈ {string, enum, multiselect}; multiselect
// is reserved for the deferred catalog-trim work (ADR-0029). Target ∈ {"" or
// "var", "invariants-marker", "invariants-globs"}; "" means a plain config var.
// Default pre-fills interactive prompts and appears in `awf init --describe`; it
// is never applied on the silent non-interactive path (ADR-0029).
type VarDescriptor struct {
	Key         string   `yaml:"key" json:"key"`
	Kind        string   `yaml:"kind" json:"kind"`
	Description string   `yaml:"description" json:"description"`
	Default     string   `yaml:"default" json:"default"`
	Options     []string `yaml:"options" json:"options"`
	Target      string   `yaml:"target" json:"target"`
}
```

- [ ] In the `Catalog` struct (line 35), add the field after `Docs`:

```go
	Vars        []VarDescriptor       `yaml:"vars"`
```

### Task 1.2: Add the `vars:` descriptor block to the catalog

- [ ] Edit `templates/catalog.yaml`. Append a new top-level block at end of file:

```yaml
vars:
  - key: gateCmd
    kind: string
    description: Command that runs the full pre-commit gate (tests, lint, coverage).
    default: ""
    options: ["./x gate", "make gate", "go test ./..."]
  - key: gateCmdFull
    kind: string
    description: Command for the full/extended gate tier, if the project has one.
    default: ""
    options: ["./x gate full"]
  - key: gateDuration
    kind: string
    description: Approximate gate runtime, quoted in docs (e.g. "~15s").
    default: ""
    options: ["~15s"]
  - key: checkCmd
    kind: string
    description: Command that checks rendered output for drift.
    default: ""
    options: ["./x check", "awf check"]
  - key: testCmd
    kind: string
    description: Command that runs the test suite.
    default: ""
    options: ["./x test", "go test ./...", "npm test"]
  - key: commitScope
    kind: string
    description: Conventional Commits scope for this project.
    default: ""
    options: ["awf"]
  - key: modulePrefix
    kind: string
    description: Module path / import prefix for the project.
    default: ""
    options: ["github.com/you/project"]
  - key: activeMdRegenCmd
    kind: string
    description: Command that regenerates the generated ADR index (ACTIVE.md).
    default: ""
    options: ["./x sync", "awf sync"]
  - key: adrProposeCommitFmt
    kind: string
    description: Commit-subject format for a proposed ADR.
    default: ""
    options: ["docs(adr): propose NNNN <title>"]
  - key: docCurrencyTargets
    kind: string
    description: Docs that must be updated alongside behaviour changes.
    default: ""
    options: ["AGENTS.md, docs/"]
  - key: invariantTestPath
    kind: string
    description: Path or glob where invariant-backing tests live.
    default: ""
    options: ["./internal/..."]
  - key: invariantsMarker
    kind: enum
    target: invariants-marker
    description: Comment marker preceding `invariant: <slug>` backing comments (language-specific).
    default: ""
    options: ["//", "#", "--", ";", "%"]
  - key: invariantsGlobs
    kind: string
    target: invariants-globs
    description: Comma-separated filename globs scanned for invariant backing comments (basename match).
    default: ""
    options: ["*.go", "*.py", "*.ts", "*.rb"]
```

### Task 1.3: Parity test (backs `var-descriptor-parity`)

- [ ] Create `internal/project/descriptor_parity_test.go`:

```go
package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// validKinds and validTargets bound the descriptor schema the embedded catalog
// may use.
var validKinds = []string{"string", "enum", "multiselect"}
var validTargets = []string{"", "var", "invariants-marker", "invariants-globs"}

// TestVarDescriptorParity asserts that every var referenced by any catalog
// template has a matching var-target descriptor, and no var-target descriptor
// names a var absent from every template. Non-var descriptors (the invariants
// marker/globs) are exempt. The referenced set is re-derived from the templates
// here, independently of any production helper.
// invariant: var-descriptor-parity
func TestVarDescriptorParity(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	// Referenced vars across every catalog template family.
	referenced := map[string]bool{}
	var paths []string
	for name := range cat.Skills {
		paths = append(paths, "skills/"+name+"/SKILL.md.tmpl")
	}
	for name := range cat.Agents {
		paths = append(paths, "agents/"+name+".md.tmpl")
	}
	for _, h := range cat.Hooks {
		paths = append(paths, "hooks/"+h+".tmpl")
	}
	for name := range cat.Docs {
		paths = append(paths, "docs/"+name+".md.tmpl")
	}
	for _, p := range paths {
		src, err := templates.FS.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		for _, v := range render.ReferencedVars(string(src)) {
			referenced[v] = true
		}
	}

	// Descriptor set, partitioned by target.
	descByKey := map[string]bool{}
	for _, d := range cat.Vars {
		if !slices.Contains(validKinds, d.Kind) {
			t.Errorf("descriptor %q has invalid kind %q", d.Key, d.Kind)
		}
		if !slices.Contains(validTargets, d.Target) {
			t.Errorf("descriptor %q has invalid target %q", d.Key, d.Target)
		}
		if d.Target == "" || d.Target == "var" {
			descByKey[d.Key] = true
			if !referenced[d.Key] {
				t.Errorf("var descriptor %q names a var referenced by no template", d.Key)
			}
		}
	}

	// Every referenced var has a var descriptor.
	for v := range referenced {
		if !descByKey[v] {
			t.Errorf("referenced var %q has no catalog descriptor", v)
		}
	}
}
```

### Task 1.4: Verify and commit

- [ ] Run `go test ./internal/catalog/ ./internal/project/ 2>&1 | tail`: expect `ok` for both.
- [ ] Run `./x gate`: expect `coverage: 100.0%` and `0 issues.`
- [ ] Run `./x check`: expect `awf check: clean` (catalog.yaml is not drift-tracked; no rendered change).
- [ ] Commit:

```
git add internal/catalog/catalog.go templates/catalog.yaml internal/project/descriptor_parity_test.go
git commit -m "feat(awf): add init value descriptors to the catalog

ADR-0029 Phase 1. A top-level vars: block in templates/catalog.yaml declares the
fillable init values (the 11 template vars plus the invariants marker/globs) as
{key, kind, description, default, options, target} descriptors, parsed into
catalog.Catalog.Vars. TestVarDescriptorParity gates that every template-referenced
var has a descriptor and no descriptor names a dead var (inv: var-descriptor-parity)."
```

---

## Phase 2: `internal/initspec`: resolution, describe, parsing

### Task 2.1: Create the package

- [ ] Create `internal/initspec/initspec.go`:

```go
// Package initspec resolves awf init answers against the catalog's value
// descriptors and emits the descriptor schema (ADR-0029). It bridges the
// catalog's VarDescriptor set to a resolved (vars, invariants-config) pair via
// explicit answers, an optional line-based prompter, or the silent default.
package initspec

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// Describe marshals the descriptor set as JSON ({"descriptors": [...]}) for
// `awf init --describe`. An empty Target is normalized to "var".
func Describe(descs []catalog.VarDescriptor) ([]byte, error) {
	out := make([]catalog.VarDescriptor, len(descs))
	for i, d := range descs {
		if d.Target == "" {
			d.Target = "var"
		}
		out[i] = d
	}
	return json.MarshalIndent(map[string]any{"descriptors": out}, "", "  ")
}

// ParseAnswersFile parses a flat key→value answer map from JSON or YAML bytes.
func ParseAnswersFile(b []byte) (map[string]string, error) {
	m := map[string]string{}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("initspec: parse answers: %w", err)
	}
	return m, nil
}

// MergeSetFlags overlays "key=value" strings onto base (later wins).
func MergeSetFlags(base map[string]string, sets []string) error {
	for _, s := range sets {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return fmt.Errorf("initspec: --set %q is not key=value", s)
		}
		base[k] = v
	}
	return nil
}

// Resolve maps descriptors + answers to a vars map and an optional invariants
// config. For each descriptor the value is: the explicit answer if present;
// otherwise an interactive prompt (when interactive); otherwise empty. The
// invariants-marker/globs targets are collected into a *config.InvariantConfig:
// both non-empty → enabled config; exactly one → error; neither → nil.
func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool) (map[string]string, *config.InvariantConfig, error) {
	vars := map[string]string{}
	var marker, globs string
	r := bufio.NewReader(in)
	for _, d := range descs {
		val, ok := answers[d.Key]
		if !ok {
			if interactive {
				p, err := prompt(r, out, d)
				if err != nil {
					return nil, nil, err
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
	inv, err := invConfig(marker, globs)
	if err != nil {
		return nil, nil, err
	}
	return vars, inv, nil
}

func invConfig(marker, globs string) (*config.InvariantConfig, error) {
	if marker == "" && globs == "" {
		return nil, nil
	}
	if marker == "" || globs == "" {
		return nil, fmt.Errorf("initspec: invariantsMarker and invariantsGlobs must be set together")
	}
	var gs []string
	for _, g := range strings.Split(globs, ",") {
		if g = strings.TrimSpace(g); g != "" {
			gs = append(gs, g)
		}
	}
	return &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: gs, Marker: marker}}}, nil
}

// prompt reads one line for descriptor d, returning d.Default on empty input.
// For an enum, a numeric reply selects the option at that 1-based index.
func prompt(r *bufio.Reader, out io.Writer, d catalog.VarDescriptor) (string, error) {
	fmt.Fprintf(out, "%s: %s\n", d.Key, d.Description)
	if d.Kind == "enum" {
		for i, o := range d.Options {
			fmt.Fprintf(out, "  %d) %s\n", i+1, o)
		}
	} else if len(d.Options) > 0 {
		fmt.Fprintf(out, "  e.g. %s\n", strings.Join(d.Options, ", "))
	}
	fmt.Fprintf(out, "  [%s]: ", d.Default)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("initspec: read input: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return d.Default, nil
	}
	if d.Kind == "enum" {
		var n int
		if _, e := fmt.Sscanf(line, "%d", &n); e == nil && n >= 1 && n <= len(d.Options) {
			return d.Options[n-1], nil
		}
	}
	return line, nil
}
```

### Task 2.2: Tests

- [ ] Create `internal/initspec/initspec_test.go` covering every branch:

```go
package initspec

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func descs() []catalog.VarDescriptor {
	return []catalog.VarDescriptor{
		{Key: "gateCmd", Kind: "string", Default: "./x gate", Options: []string{"./x gate", "make"}},
		{Key: "invariantsMarker", Kind: "enum", Target: "invariants-marker", Options: []string{"//", "#"}},
		{Key: "invariantsGlobs", Kind: "string", Target: "invariants-globs"},
	}
}

func TestResolveSilentSeedsEmpty(t *testing.T) {
	vars, inv, err := Resolve(descs(), nil, strings.NewReader(""), &strings.Builder{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "" {
		t.Errorf("silent gateCmd = %q, want empty", vars["gateCmd"])
	}
	if inv != nil {
		t.Errorf("silent inv = %v, want nil", inv)
	}
}

func TestResolveExplicitAnswersWin(t *testing.T) {
	a := map[string]string{"gateCmd": "make test", "invariantsMarker": "//", "invariantsGlobs": "*.go,*.s"}
	vars, inv, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "make test" {
		t.Errorf("gateCmd = %q", vars["gateCmd"])
	}
	if inv == nil || len(inv.Sources) != 1 || inv.Sources[0].Marker != "//" ||
		len(inv.Sources[0].Globs) != 2 || inv.Sources[0].Globs[0] != "*.go" {
		t.Errorf("inv = %+v", inv)
	}
}

func TestResolveInteractiveDefaultAndEnumIndexAndLiteral(t *testing.T) {
	// gateCmd: empty line → default; marker: "2" → second option; globs: literal.
	in := strings.NewReader("\n2\n*.go\n")
	vars, inv, err := Resolve(descs(), nil, in, &strings.Builder{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "./x gate" {
		t.Errorf("gateCmd = %q, want default", vars["gateCmd"])
	}
	if inv == nil || inv.Sources[0].Marker != "#" {
		t.Errorf("marker = %+v, want #", inv)
	}
}

func TestResolveInteractiveStringLiteralAndEnumNonNumeric(t *testing.T) {
	// gateCmd: literal; marker: non-numeric literal; globs: literal.
	in := strings.NewReader("custom\n//\n*.go\n")
	vars, inv, err := Resolve(descs(), nil, in, &strings.Builder{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "custom" {
		t.Errorf("gateCmd = %q", vars["gateCmd"])
	}
	if inv.Sources[0].Marker != "//" {
		t.Errorf("marker = %q", inv.Sources[0].Marker)
	}
}

func TestResolveInvariantsHalfSetErrors(t *testing.T) {
	a := map[string]string{"invariantsMarker": "//"}
	if _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false); err == nil {
		t.Fatal("expected error for marker without globs")
	}
}

// errReader fails on first read so the non-EOF ReadString branch in prompt (and
// its propagation through Resolve) is exercised; strings.NewReader never errors.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestResolvePromptReadError(t *testing.T) {
	if _, _, err := Resolve(descs(), nil, errReader{}, &strings.Builder{}, true); err == nil {
		t.Fatal("expected read error from prompt")
	}
}

func TestDescribeNormalizesTargetAndIsValidJSON(t *testing.T) {
	b, err := Describe(descs())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"descriptors"`) || !strings.Contains(s, `"target": "var"`) {
		t.Errorf("describe JSON missing fields:\n%s", s)
	}
}

func TestParseAnswersFile(t *testing.T) {
	m, err := ParseAnswersFile([]byte("gateCmd: ./x gate\n"))
	if err != nil || m["gateCmd"] != "./x gate" {
		t.Fatalf("m=%v err=%v", m, err)
	}
	if _, err := ParseAnswersFile([]byte("- not a map\n")); err == nil {
		t.Fatal("expected error for non-map answers")
	}
}

func TestMergeSetFlags(t *testing.T) {
	base := map[string]string{}
	if err := MergeSetFlags(base, []string{"a=1", "b=2"}); err != nil {
		t.Fatal(err)
	}
	if base["a"] != "1" || base["b"] != "2" {
		t.Errorf("base=%v", base)
	}
	if err := MergeSetFlags(base, []string{"bad"}); err == nil {
		t.Fatal("expected error for missing =")
	}
}
```

### Task 2.3: Verify and commit

- [ ] Run `go test ./internal/initspec/ 2>&1 | tail`: expect `ok`.
- [ ] Run `./x gate`: expect `coverage: 100.0%`, `0 issues.` (`TestResolvePromptReadError` covers the
  non-EOF `ReadString` branch in `prompt`; `strings.NewReader` cannot trigger it, so that test is required.)
- [ ] Commit:

```
git add internal/initspec/
git commit -m "feat(awf): resolve init answers and emit the descriptor schema

ADR-0029 Phase 2. internal/initspec resolves descriptors + answers to a (vars,
invariants-config) pair (explicit answer > interactive prompt > empty), emits the
--describe JSON, and parses --answers/--set inputs. Line-based stdlib prompter;
no new dependency."
```

---

## Phase 3: `ScaffoldConfig` takes resolved vars + invariants

### Task 3.1: Add `Skeleton.Invariants`

- [ ] Edit `internal/config/edit.go`. In the `Skeleton` struct (line 17), add after `Docs`:

```go
	Invariants *InvariantConfig `yaml:"invariants,omitempty"`
```

(`omitempty` keeps the silent output byte-identical: a nil pointer emits no `invariants:` key.)

### Task 3.2: Change `ScaffoldConfig`

- [ ] Edit `internal/project/scaffold.go`. Change the signature (line 22) and var-seeding so resolved
  values overlay the all-referenced base, and the invariants config is passed through:

```go
func ScaffoldConfig(prefix string, vars map[string]string, inv *config.InvariantConfig) ([]byte, error) {
```

- [ ] Replace the final `vars` construction block (lines 80-83, the `vars := make(...)` loop seeding `""`)
  with an overlay of the resolved values onto the all-referenced base:

```go
	seeded := make(map[string]string, len(varNames))
	for _, v := range varNames {
		seeded[v] = vars[v] // resolved value, or "" for an absent/unresolved var
	}
```

- [ ] In the `config.MarshalSkeleton(config.Skeleton{...})` call (line 84), pass the new map and the
  invariants config:

```go
	return config.MarshalSkeleton(config.Skeleton{
		Prefix:     prefix,
		Vars:       seeded,
		Skills:     skillNames,
		Agents:     agentNames,
		Hooks:      hookList,
		Docs:       docNames,
		Invariants: inv,
	})
```

### Task 3.3: Update call sites

- [ ] `cmd/awf/main.go` line 207: change to (full flag wiring lands in Phase 4):

```go
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), nil, nil)
```

- [ ] `internal/project/scaffold_test.go`: every `ScaffoldConfig("...")` call → add `, nil, nil`
  (lines 23, 53, 98, 127, 163, 205, 238: 7 call sites).
- [ ] `cmd/awf/list_add_test.go` line 23: `project.ScaffoldConfig("example")` → `project.ScaffoldConfig("example", nil, nil)`.

### Task 3.4: Verify and commit

- [ ] Run `go test ./... 2>&1 | tail`: expect all `ok` (the nil,nil path is byte-identical, so existing
  golden/clean-sync assertions still hold).
- [ ] Run `./x gate`: expect `coverage: 100.0%`, `0 issues.`
- [ ] Run `./x check`: expect `awf check: clean`.
- [ ] Commit:

```
git add internal/config/edit.go internal/project/scaffold.go internal/project/scaffold_test.go cmd/awf/list_add_test.go cmd/awf/main.go
git commit -m "refactor(awf): ScaffoldConfig takes resolved vars and invariants

ADR-0029 Phase 3. ScaffoldConfig(prefix, vars, inv) overlays resolved var values
onto the all-referenced base (scaffold-seeds-all-vars preserved) and writes the
invariants config when supplied. Skeleton.Invariants is omitempty so the (nil,
nil) call stays byte-identical to today's seed-empty output."
```

---

## Phase 4: CLI: `--describe`, `--set`, `--answers`, interactive

### Task 4.1: Flag spec + extraction helpers + TTY seam

- [ ] Edit `cmd/awf/main.go`. Update the init `argSpec` (line 138):

```go
	"init":       {boolFlags: []string{"--force", "--force-hooks", "--describe"}, valueFlags: []string{"--set", "--answers"}, maxPos: 0},
```

- [ ] Add overridable seams near `var getwd = os.Getwd` (line 19):

```go
var stdin io.Reader = os.Stdin

// isInteractive reports whether stdin is a terminal (so init should prompt).
var isInteractive = func() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
```

- [ ] Add a repeatable-flag extractor near `baseFlag` (line 190):

```go
// multiFlag returns every value following an occurrence of flag in args[2:].
func multiFlag(args []string, flag string) []string {
	var out []string
	rest := args[2:]
	for i, a := range rest {
		if a == flag && i+1 < len(rest) {
			out = append(out, rest[i+1])
		}
	}
	return out
}

// valueFlag returns the value after the first occurrence of flag, or "".
func valueFlag(args []string, flag string) string {
	rest := args[2:]
	for i, a := range rest {
		if a == flag && i+1 < len(rest) {
			return rest[i+1]
		}
	}
	return ""
}
```

### Task 4.2: Wire `runInit`

- [ ] Change the init dispatch (line 70) to extract the new flags:

```go
	case "init":
		cmdErr = runInit(cwd, hasFlag(args, "--force"), hasFlag(args, "--force-hooks"),
			hasFlag(args, "--describe"), multiFlag(args, "--set"), valueFlag(args, "--answers"),
			stdout, stderr)
```

- [ ] Add imports to `cmd/awf/main.go`: `"github.com/hypnotox/agentic-workflows/internal/catalog"`,
  `"github.com/hypnotox/agentic-workflows/internal/initspec"`, `"github.com/hypnotox/agentic-workflows/templates"`.

- [ ] Change the `runInit` signature (line 200) and prepend the describe + resolve logic; replace the
  `project.ScaffoldConfig(filepath.Base(root), nil, nil)` call (from Phase 3) with the resolved values:

```go
func runInit(root string, force, forceHooks, describe bool, sets []string, answersFile string, stdout, stderr io.Writer) error {
	cat, err := catalog.Load(templates.FS)
	if err != nil { // coverage-ignore: catalog.Load over the embedded FS cannot fail at runtime
		return err
	}
	if describe {
		out, err := initspec.Describe(cat.Vars)
		if err != nil { // coverage-ignore: descriptors marshal to JSON; cannot fail
			return err
		}
		fmt.Fprintln(stdout, string(out))
		return nil
	}
	answers := map[string]string{}
	if answersFile != "" {
		b, err := os.ReadFile(answersFile)
		if err != nil {
			return fmt.Errorf("awf init: read --answers: %w", err)
		}
		if answers, err = initspec.ParseAnswersFile(b); err != nil {
			return err
		}
	}
	if err := initspec.MergeSetFlags(answers, sets); err != nil {
		return err
	}
	vars, inv, err := initspec.Resolve(cat.Vars, answers, stdin, stdout, isInteractive())
	if err != nil {
		return err
	}

	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	scaffolded := false
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv)
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
			return err
		}
		scaffolded = true
		fmt.Fprintf(stdout, "scaffolded %s\n", cfgPath)
	}
	// ... existing body from `p, err := project.Open(root)` (line 217) onward unchanged ...
```

  Keep the remainder of `runInit` (from `p, err := project.Open(root)` through the end) exactly as it
  is today.

### Task 4.3: Help text

- [ ] In `helpText` (line 28), extend the init block:

```
  init         Scaffold .awf/, render the workflow-core set, and activate git hooks
                 --force        overwrite colliding files, backing each up to <path>.awf-bak
                 --force-hooks  take over an existing core.hooksPath (husky/lefthook)
                 --describe     print the fillable value descriptors as JSON and exit
                 --set k=v      set a value non-interactively (repeatable)
                 --answers FILE read values from a JSON/YAML answers file
```

### Task 4.4: Tests (back `describe-read-only`, `explicit-answers-win`, `init-noninteractive-default`)

- [ ] Add to `cmd/awf/run_test.go` (follow the existing test style: `run([]string{...}, &bytes.Buffer{}, ...)`
  with a `t.Chdir(tmp)` or the package's existing cwd seam; set `isInteractive = func() bool { return false }`
  and restore it with `t.Cleanup`). Cover:

  1. **describe-read-only**: `run([]string{"awf","init","--describe"}, out, errOut)` in a temp dir:
     assert exit 0, `out` parses as JSON containing `"descriptors"`, and **no `.awf/` dir was created**.
     Tag the test `// invariant: describe-read-only`.
  2. **explicit-answers-win**: `run([]string{"awf","init","--set","gateCmd=make gate"}, ...)`:
     assert the written `.awf/config.yaml` contains `gateCmd: make gate`. Tag `// invariant: explicit-answers-win`.
  3. **init-noninteractive-default**: with `isInteractive` forced false and no answer flags,
     `run([]string{"awf","init"}, ...)`: assert every var in the written config is empty (`gateCmd: ""`)
     and there is no `invariants:` key. Tag `// invariant: init-noninteractive-default`.
  4. **--answers file**: write a temp JSON `{"testCmd":"go test ./..."}`, run `init --answers <file>`,
     assert config has `testCmd: go test ./...`.
  5. **error paths**: `init --set bad` (no `=`) → exit 2-or-1 with the `--set` error; `init --answers /nope`
     → read error; **`init --answers <file>` where the file exists but holds a non-map** (e.g. write
     `- a\n`) → the `ParseAnswersFile` error branch in `runInit` (`os.ReadFile` succeeds, so this is a
     distinct statement from the read-error case above and is required for 100% coverage);
     `init --set invariantsMarker=//` (globs missing) → the half-set error.
  6. **real TTY seam**: one test that calls the production `isInteractive()` *without* overriding it
     (e.g. `func TestIsInteractive(t *testing.T) { _ = isInteractive() }`). `isInteractive` is a closure
     with a body, so unless one test executes the real function its two statements are uncovered and the
     100% gate fails; under `go test` stdin is not a char device, so it returns false and both statements run.

  Each test that writes config should run in its own temp dir and may call `run` once; assert on the
  file bytes via `os.ReadFile`. Stub `isInteractive` to false (with `t.Cleanup` restore) in every test
  *except* `TestIsInteractive`.

### Task 4.5: Verify and commit

- [ ] Run `go test ./cmd/awf/ 2>&1 | tail`: expect `ok`.
- [ ] Run `./x gate`: expect `coverage: 100.0%`, `0 issues.`
- [ ] Manual smoke (optional): build a binary and run `awf init --describe` in a temp dir; confirm JSON
  prints and no `.awf/` appears.
- [ ] Commit:

```
git add cmd/awf/main.go cmd/awf/run_test.go
git commit -m "feat(awf): interactive, --describe, and --set/--answers init

ADR-0029 Phase 4. awf init now prompts on a TTY, prints its value descriptors as
JSON with --describe (writing nothing), and accepts --set k=v (repeatable) and
--answers FILE. TTY detection via stdlib os.Stdin.Stat. Backs describe-read-only,
explicit-answers-win, and init-noninteractive-default."
```

---

## Phase 5: Docs currency (ADR-0029 stays `Proposed`)

> This plan implements the vars+invariants slice only; catalog-trim is deferred, so ADR-0029 is **not**
> flipped to `Implemented` here. Update the domain narratives and the command surface for what shipped.

### Task 5.1: Domain narratives

- [ ] Edit `.awf/domains/parts/tooling/current-state.md`: add a sentence that `awf init` now supports
  interactive prompts, `--describe` (JSON schema for agents), and `--set`/`--answers` non-interactive
  supply, while the silent non-TTY path still seeds empty.
- [ ] Edit `.awf/domains/parts/config/current-state.md`: add that `templates/catalog.yaml` carries a
  `vars:` descriptor block ({key, kind, description, default, options, target}) driving init, and that
  `ScaffoldConfig` now takes resolved vars + an optional invariants config.

### Task 5.2: README command surface

- [ ] Edit `README.md`: in the Commands table `awf init` row, append a note that init also accepts
  `--describe` (print fillable values as JSON), `--set k=v`, and `--answers FILE`. Keep it one line.

### Task 5.3: Re-render, verify, commit

- [ ] Run `./x sync`: regenerates `docs/domains/tooling.md` + `docs/domains/config.md` from the edited parts.
- [ ] Run `./x check`: expect `awf check: clean`.
- [ ] Run `./x gate`: expect `coverage: 100.0%`, `0 issues.`
- [ ] Commit:

```
git add .awf/domains/parts/tooling/current-state.md .awf/domains/parts/config/current-state.md docs/domains/tooling.md docs/domains/config.md README.md .awf/awf.lock
git commit -m "docs(awf): document interactive/agent-prefillable init

ADR-0029 (vars+invariants slice). Refresh the tooling and config domain
narratives and the README init command row for --describe/--set/--answers.
ADR-0029 stays Proposed; the catalog-trim multiselect is a deferred follow-up."
```

---

## Out of scope (deferred follow-up plan)

- **Catalog-trim multiselect** at init (choosing which skills/docs to enable beyond the curated core),
  including the multiselect prompt widget and the `scaffold-core-only` interaction. When that lands,
  flip ADR-0029 to `Implemented`.
- Detection-based prefill (explicitly rejected in ADR-0029).
- Free-text prose authoring (identity/you-and-this-project): ADR-0029 Decision 6 keeps it manual.
