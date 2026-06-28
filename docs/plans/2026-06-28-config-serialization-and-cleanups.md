# Plan: config-serialization ownership + four cleanups

**Design:** [ADR-0026](../decisions/0026-config-serialization-ownership.md) (config serialization). The
other four items (init bugfix, typed Layout, coverage dodge, `project.Audit` streamline) are refactors/
a bugfix needing no ADR. Do not duplicate ADR rationale here — link.

## Goal

1. Fix the `awf init` rollback so a collision-abort preserves a pre-existing `.awf/` (data-loss bug).
2. Move `.awf/config.yaml` construction + mutation into `internal/config` behind one `encode` funnel
   (`MarshalSkeleton`, `SetArrayMember` via `yaml.Node` round-trip), replacing `writeArray` + `editArray`.
3. Pay off the `editArray` `coverage-ignore` dodge by deleting `editArray` and unit-testing `SetArrayMember`.
4. Make `project.layout()` return a typed `Layout` struct (+ `templateMap()` adapter) instead of `map[string]any`.
5. Embed `config.AuditSettings` in `audit.Inputs`, collapsing the 15-field copy in `project.Audit`.

## Architecture summary

`internal/config` becomes the sole owner of live config.yaml (de)serialization. Construction and
mutation share one private `encode(v any)` (yaml.v3 encoder, `SetIndent(2)`). Mutation is a node
round-trip that preserves comments/untouched keys and normalizes only the edited sequence to block
style. `internal/migrate` is untouched (ADR-0010 quarantine; ADR-0026 Decision 4). The typed `Layout`
keeps the per-file `ConfigHash` stable by routing the hash and the template namespace through
`templateMap()`, which reproduces today's `layout()` map verbatim. `audit.Inputs` gains an embedded
`config.AuditSettings` (new `audit → config` edge; acyclic — `config` imports no internal package).

## Tech stack

- Go 1.26; `gopkg.in/yaml.v3` (already a direct dep).
- Packages touched: `internal/config` (new `edit.go`), `internal/project` (scaffold/render/project),
  `cmd/awf` (main/list_add), `internal/audit`.

## File structure

- **Created:** `internal/config/edit.go`, `internal/config/edit_test.go`.
- **Modified:** `cmd/awf/main.go`, `cmd/awf/run_test.go`, `internal/project/scaffold.go`,
  `cmd/awf/list_add.go`, `cmd/awf/list_add_test.go`, `internal/project/render.go`,
  `internal/project/project.go`, `internal/project/project_test.go`, `internal/audit/audit.go`,
  `internal/audit/audit_test.go`, `internal/audit/git_test.go`, `docs/decisions/0026-config-serialization-ownership.md`,
  `.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/tooling/current-state.md`,
  `.awf/docs/parts/architecture/components.md` (+ regenerated `docs/**` via `./x sync`).
- **Deleted:** none (functions removed: `writeArray`, `editArray`; test `TestEditArray`).

Each phase ends with `./x gate` (must print `coverage: 100.0%` and `0 issues.`) then a commit. Stage
explicitly (no `git add -A`). Commit-message bodies end with the trailer
`Claude-Session: https://claude.ai/code/session_01EG9dJ2ZRjY6YXv2DkwJy8W`.

---

## Phase 1 — init rollback bugfix (test-first)

- [ ] **1.1 Add the regression test.** In `cmd/awf/run_test.go`, after `TestInitGuardBlocksAndForceOverrides`
  (ends ~line 450), add:
  ```go
  func TestInitRollbackPreservesExistingAwf(t *testing.T) {
  	root := t.TempDir()
  	// Pre-existing authored .awf/ content but no config.yaml -> init scaffolds config,
  	// then a collision (non-managed CLAUDE.md) forces a refusal + rollback.
  	part := filepath.Join(root, ".awf", "skills", "parts", "foo", "extra.md")
  	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(part, []byte("hand-authored\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := runInit(root, false, false, io.Discard, io.Discard); err == nil {
  		t.Fatal("expected init to refuse on collision")
  	}
  	// The scaffolded config.yaml is rolled back...
  	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
  		t.Error("config.yaml should have been removed on rollback")
  	}
  	// ...but the pre-existing authored content survives.
  	if _, err := os.Stat(part); err != nil {
  		t.Errorf("pre-existing .awf content must be preserved, got: %v", err)
  	}
  }
  ```
- [ ] **1.2 Confirm it fails** for the right reason: `go test ./cmd/awf/ -run TestInitRollbackPreservesExistingAwf`
  → FAIL on "pre-existing .awf content must be preserved" (current `os.RemoveAll` nukes it).
- [ ] **1.3 Fix `runInit`.** In `cmd/awf/main.go`, replace the rollback block (lines 161-163):
  ```go
  			if scaffolded {
  				os.RemoveAll(filepath.Dir(cfgPath)) // writes nothing on abort
  			}
  ```
  with:
  ```go
  			if scaffolded {
  				_ = os.Remove(cfgPath)               // remove the config we scaffolded
  				_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
  			}
  ```
- [ ] **1.4 Gate + commit.** `./x gate`; then commit (`cmd/awf/main.go`, `cmd/awf/run_test.go`):
  `fix(awf): init rollback preserves a pre-existing .awf`.

---

## Phase 2 — config serialization owner (`internal/config/edit.go`)

This phase adds the owner and its tests standalone; wiring of scaffold/list_add follows in Phases 3-4.

- [ ] **2.1 Create `internal/config/edit.go`** verbatim:
  ```go
  package config

  import (
  	"bytes"
  	"fmt"

  	"gopkg.in/yaml.v3"
  )

  // Skeleton is the input to MarshalSkeleton: the fields a freshly-scaffolded
  // .awf/config.yaml carries. Vars is typed map[string]string (not map[string]any)
  // so a nil/null var value is unrepresentable — the scaffold seeds each var with an
  // empty string, which marshals as `x: ""`. A nil interface would marshal as
  // `x: null` and decode back to a nil value that renders as "<no value>", tripping
  // the publication-safe check (ADR-0026 Decision 3).
  type Skeleton struct {
  	Prefix string            `yaml:"prefix"`
  	Vars   map[string]string `yaml:"vars"`
  	Skills []string          `yaml:"skills"`
  	Agents []string          `yaml:"agents"`
  	Hooks  []string          `yaml:"hooks"`
  	Docs   []string          `yaml:"docs"`
  }

  // MarshalSkeleton renders a fresh config.yaml from s in the canonical awf format
  // (two-space block style). It is the construction half of internal/config's
  // ownership of config.yaml serialization (ADR-0026).
  func MarshalSkeleton(s Skeleton) ([]byte, error) {
  	return encode(s)
  }

  // SetArrayMember adds or removes name in the sequence under key in a config.yaml
  // source, via a yaml.Node round-trip that preserves comments and every untouched
  // key (ADR-0026). The edited sequence is normalized to block style, so a flow-style
  // input (`key: [a, b]`) is accepted. Adding a member already present is a no-op;
  // removing a member absent from the key (or a key absent on remove) errors.
  // invariant: config-mutation-roundtrip
  func SetArrayMember(src []byte, key, name string, add bool) ([]byte, error) {
  	var doc yaml.Node
  	if err := yaml.Unmarshal(src, &doc); err != nil {
  		return nil, fmt.Errorf("config: parse: %w", err)
  	}
  	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
  		return nil, fmt.Errorf("config: not a YAML mapping")
  	}
  	root := doc.Content[0]
  	val, vi := mapValue(root, key)
  	switch {
  	case val == nil: // key absent
  		if !add {
  			return nil, fmt.Errorf("config: no %q entry under %q", name, key)
  		}
  		root.Content = append(root.Content, strScalar(key), blockSeq(name))
  	case val.Kind == yaml.SequenceNode:
  		val.Style = 0 // normalize flow -> block
  		idx := seqIndex(val, name)
  		switch {
  		case add:
  			if idx < 0 {
  				val.Content = append(val.Content, strScalar(name))
  			}
  		case idx < 0:
  			return nil, fmt.Errorf("config: no %q entry under %q", name, key)
  		default:
  			// invariant: remove-block-scoped
  			val.Content = append(val.Content[:idx], val.Content[idx+1:]...)
  		}
  	default: // bare `key:` (null value)
  		if !add {
  			return nil, fmt.Errorf("config: no %q entry under %q", name, key)
  		}
  		root.Content[vi] = blockSeq(name)
  	}
  	return encode(&doc)
  }

  // encode is the single funnel for awf-owned config.yaml serialization: a yaml.v3
  // encoder fixed at two-space indentation. Both MarshalSkeleton (construction) and
  // SetArrayMember (mutation) route through it, so the on-disk format has exactly one
  // definition.
  // invariant: config-serialization-owned
  func encode(v any) ([]byte, error) {
  	var buf bytes.Buffer
  	enc := yaml.NewEncoder(&buf)
  	enc.SetIndent(2)
  	if err := enc.Encode(v); err != nil { // coverage-ignore: encode receives a Skeleton or a yaml.Node decoded from valid YAML; only unrepresentable Go types (chan/func) fail, which neither holds
  		return nil, err
  	}
  	_ = enc.Close()
  	return buf.Bytes(), nil
  }

  func mapValue(m *yaml.Node, key string) (*yaml.Node, int) {
  	for i := 0; i+1 < len(m.Content); i += 2 {
  		if m.Content[i].Value == key {
  			return m.Content[i+1], i + 1
  		}
  	}
  	return nil, -1
  }

  func seqIndex(seq *yaml.Node, name string) int {
  	for i, n := range seq.Content {
  		if n.Value == name {
  			return i
  		}
  	}
  	return -1
  }

  func strScalar(v string) *yaml.Node {
  	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
  }

  func blockSeq(name string) *yaml.Node {
  	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{strScalar(name)}}
  }
  ```
  Note: the `remove-block-scoped` marker is re-homed here from `editArray` (ADR-0026 Decision 5);
  `editArray` keeps its own copy until Phase 4 deletes it, so the slug has ≥1 backing marker throughout
  (the check requires at least one). ADR-0026's two slugs are unenforced until the status flips
  (Phase 5), so their markers existing now is harmless.

- [ ] **2.2 Create `internal/config/edit_test.go`** verbatim:
  ```go
  package config

  import (
  	"strings"
  	"testing"
  )

  func TestMarshalSkeleton(t *testing.T) {
  	out, err := MarshalSkeleton(Skeleton{
  		Prefix: "awf",
  		Vars:   map[string]string{"b": "", "a": ""},
  		Skills: []string{"tdd"},
  		Agents: []string{},
  		Hooks:  []string{"pre-commit"},
  		Docs:   []string{"workflow"},
  	})
  	if err != nil {
  		t.Fatal(err)
  	}
  	want := "prefix: awf\n" +
  		"vars:\n  a: \"\"\n  b: \"\"\n" +
  		"skills:\n  - tdd\n" +
  		"agents: []\n" +
  		"hooks:\n  - pre-commit\n" +
  		"docs:\n  - workflow\n"
  	if string(out) != want {
  		t.Errorf("MarshalSkeleton:\n got: %q\nwant: %q", out, want)
  	}
  }

  func TestSetArrayMember(t *testing.T) {
  	cases := []struct {
  		name, src, key, item string
  		add                  bool
  		want                 string
  		wantErr              bool
  	}{
  		{"add appends", "skills:\n  - a\n", "skills", "b", true, "skills:\n  - a\n  - b\n", false},
  		{"add idempotent", "skills:\n  - a\n", "skills", "a", true, "skills:\n  - a\n", false},
  		{"add to empty flow", "agents: []\n", "agents", "x", true, "agents:\n  - x\n", false},
  		{"add to bare key", "docs:\n", "docs", "d", true, "docs:\n  - d\n", false},
  		{"add absent key", "prefix: x\n", "domains", "p", true, "prefix: x\ndomains:\n  - p\n", false},
  		{"add to flow with items", "skills: [a, b]\n", "skills", "c", true, "skills:\n  - a\n  - b\n  - c\n", false},
  		{"remove from items", "skills:\n  - a\n  - b\n", "skills", "a", false, "skills:\n  - b\n", false},
  		{"remove last empties", "docs:\n  - d\n", "docs", "d", false, "docs: []\n", false},
  		{"remove block-scoped", "skills:\n  - debugging\ndocs:\n  - debugging\n", "docs", "debugging", false, "skills:\n  - debugging\ndocs: []\n", false},
  		{"remove not found", "skills:\n  - a\n", "skills", "z", false, "", true},
  		{"remove from empty flow", "skills: []\n", "skills", "a", false, "", true},
  		{"remove bare key", "skills:\n", "skills", "a", false, "", true},
  		{"remove absent key", "prefix: x\n", "skills", "a", false, "", true},
  		{"parse error", "skills: [a, b\n", "skills", "c", true, "", true},
  		{"non-mapping", "- a\n- b\n", "skills", "c", true, "", true},
  		{"empty doc", "", "skills", "c", true, "", true},
  	}
  	for _, tc := range cases {
  		t.Run(tc.name, func(t *testing.T) {
  			got, err := SetArrayMember([]byte(tc.src), tc.key, tc.item, tc.add)
  			if tc.wantErr {
  				if err == nil {
  					t.Fatalf("expected error, got %q", got)
  				}
  				return
  			}
  			if err != nil {
  				t.Fatalf("unexpected error: %v", err)
  			}
  			if string(got) != tc.want {
  				t.Errorf("SetArrayMember:\n got: %q\nwant: %q", got, tc.want)
  			}
  		})
  	}
  }

  func TestSetArrayMemberPreservesComments(t *testing.T) {
  	src := "# top comment\nprefix: x\nskills:\n  - a # inline\n"
  	got, err := SetArrayMember([]byte(src), "skills", "b", true)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !strings.Contains(string(got), "# top comment") {
  		t.Errorf("head comment lost:\n%s", got)
  	}
  	if !strings.Contains(string(got), "- b") {
  		t.Errorf("member not added:\n%s", got)
  	}
  }
  ```
  If yaml.v3 formats any `want` differently (e.g. a quoting nuance), adjust that literal to the actual
  bytes — the contract is two-space block style + comment preservation, not these exact strings; the
  gate's 100% coverage and the scaffold semantic tests are the real backstops.
- [ ] **2.3 Gate + commit** (`internal/config/edit.go`, `internal/config/edit_test.go`):
  `feat(awf): own config.yaml serialization in internal/config`.

---

## Phase 3 — scaffold via `config.MarshalSkeleton`

- [ ] **3.1 Rewrite `ScaffoldConfig`'s emission.** In `internal/project/scaffold.go`, replace the manual
  YAML block (lines 80-100, from `// Emit YAML manually …` through `return []byte(b.String()), nil`) with:
  ```go
  	vars := make(map[string]string, len(varNames))
  	for _, v := range varNames {
  		vars[v] = ""
  	}
  	return config.MarshalSkeleton(config.Skeleton{
  		Prefix: prefix,
  		Vars:   vars,
  		Skills: skillNames,
  		Agents: agentNames,
  		Hooks:  hookList,
  		Docs:   docNames,
  	})
  ```
- [ ] **3.2 Delete `writeArray`** (lines 103-112) entirely.
- [ ] **3.3 Fix imports** in `scaffold.go`: add `"github.com/hypnotox/agentic-workflows/internal/config"`;
  remove `"strings"` (only `strings.Builder` used it). Keep `fmt`, `io/fs`, `maps`, `slices`, `catalog`,
  `render`, `templates`.
- [ ] **3.4 Verify scaffold tests pass unchanged.** `internal/project/scaffold_test.go` is semantic
  (strict-parse, core-set membership, no placeholder) — no byte-golden — so it should pass as-is. If a
  byte-exact assertion exists and fails, update it to the `MarshalSkeleton` output.
- [ ] **3.5 Gate + commit** (`internal/project/scaffold.go`): `refactor(awf): scaffold via config.MarshalSkeleton`.

---

## Phase 4 — mutate config via `config.SetArrayMember` (delete `editArray`)

- [ ] **4.1 Rewrite `rewriteConfig`.** In `cmd/awf/list_add.go`, replace the body of `rewriteConfig`
  (lines 122-136) with:
  ```go
  func rewriteConfig(root, key, name string, add bool) error {
  	cfgPath := filepath.Join(root, ".awf", "config.yaml")
  	b, err := os.ReadFile(cfgPath)
  	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
  		return err
  	}
  	updated, err := config.SetArrayMember(b, key, name, add)
  	if err != nil { // coverage-ignore: callers guard add-present / remove-absent before this, and config.Load already rejected a malformed config, so SetArrayMember cannot error here
  		return err
  	}
  	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault that root bypasses
  		return err
  	}
  	return nil
  }
  ```
- [ ] **4.2 Delete `editArray`** (lines 153-197) entirely, including its `// coverage-ignore` tail (the
  dodge is paid off by `TestSetArrayMember`) and its `// invariant: remove-block-scoped` marker (re-homed
  to `SetArrayMember` in Phase 2).
- [ ] **4.3 Fix imports** in `list_add.go`: remove `"strings"` (only `editArray` used it; confirm with
  `grep -n 'strings\.' cmd/awf/list_add.go` → no matches after deletion). `config`, `slices`, `maps` stay.
- [ ] **4.4 Delete `TestEditArray`** from `cmd/awf/list_add_test.go` (lines 45-82) — replaced by
  `TestSetArrayMember` in `internal/config`.
- [ ] **4.5 Rewrite the flow-style test.** Replace `TestRunAddRemoveRefuseFlowStyle` (lines 121-132) with a
  test asserting flow-style now *works* end-to-end:
  ```go
  // TestRunAddRemoveFlowStyle confirms a hand-edited flow-style array is now edited
  // (not refused): SetArrayMember normalizes it to block style. minimalYAML uses
  // flow-style `skills: [tdd]`.
  func TestRunAddRemoveFlowStyle(t *testing.T) {
  	root := scaffoldProject(t)
  	if err := runAdd(root, "skill", "bugfix", io.Discard); err != nil {
  		t.Fatalf("add to flow-style array: %v", err)
  	}
  	cfg := readConfig(t, root)
  	if !strings.Contains(cfg, "- bugfix") || !strings.Contains(cfg, "- tdd") {
  		t.Errorf("expected block-style skills with both members:\n%s", cfg)
  	}
  	if err := runRemove(root, "skill", "tdd", io.Discard); err != nil {
  		t.Fatalf("remove from (now block) array: %v", err)
  	}
  	if strings.Contains(readConfig(t, root), "- tdd") {
  		t.Error("tdd not removed")
  	}
  }
  ```
- [ ] **4.6 Gate + commit** (`cmd/awf/list_add.go`, `cmd/awf/list_add_test.go`):
  `refactor(awf): mutate config via config.SetArrayMember`.

After this phase, all of ADR-0026's code (construction, mutation, marker re-home, migrate carve-out)
is in place — Phase 5 closes the ADR.

---

## Phase 5 — implement ADR-0026 (status flip + doc currency)

- [ ] **5.1 Flip status.** In `docs/decisions/0026-config-serialization-ownership.md`, change
  `status: Accepted` → `status: Implemented`.
- [ ] **5.2 Refresh the `config` domain narrative.** In `.awf/domains/parts/config/current-state.md`, add a
  sentence that `internal/config` now owns config.yaml construction (`MarshalSkeleton`) and mutation
  (`SetArrayMember`, a comment-preserving `yaml.Node` round-trip) behind one `encode` funnel, per ADR-0026.
- [ ] **5.3 Refresh the `tooling` domain narrative.** In `.awf/domains/parts/tooling/current-state.md`,
  update the `awf add`/`remove` sentence: array edits now go through `config.SetArrayMember` (node
  round-trip, comment-safe, flow-style accepted) rather than a 2-space string editor.
- [ ] **5.4 Update architecture.** In `.awf/docs/parts/architecture/components.md`, extend the
  `internal/config` entry: it now owns config.yaml *write* (construction + mutation) as well as the
  load/schema, with `internal/migrate` excepted (ADR-0010 quarantine).
- [ ] **5.5 Regenerate + check.** `./x sync` (regenerates `docs/decisions/ACTIVE.md`, `docs/domains/config.md`,
  `docs/domains/tooling.md`, `docs/architecture.md`). Then `./x check` — now that ADR-0026 is `Implemented`,
  it enforces `config-serialization-owned` + `config-mutation-roundtrip`; both markers exist in `edit.go`,
  so it must print `awf check: clean`.
- [ ] **5.6 Gate + commit** (the ADR, the three `.awf/**` parts, and all `docs/**` regenerated by sync):
  `docs(awf): implement ADR-0026 config serialization ownership`.

---

## Phase 6 — typed `project.Layout`

- [ ] **6.1 Replace `layout()` and add `templateMap()`.** In `internal/project/render.go`, replace the
  `layout()` method (lines 42-73) with a typed `Layout` struct, a `layout()` returning it, and a
  `templateMap()` that reproduces today's map exactly:
  ```go
  // Layout is the fixed, awf-given docs layout derived from cfg.DocsDir, in typed
  // form for Go consumers. templateMap projects it into the .layout namespace for
  // templates (which read a map, not unexported struct fields) and into the
  // per-file ConfigHash — reproducing the historical map verbatim, so the hash is
  // unchanged (no drift).
  type Layout struct {
  	DocsDir     string
  	ADRDir      string
  	ActiveMd    string
  	AdrReadme   string
  	AdrTemplate string
  	PlansDir    string
  	PlansReadme string
  	Docs        map[string]string
  	WorkflowRef string
  	DomainsDir  string
  }

  func (p *Project) layout() Layout {
  	d := strings.TrimRight(p.Cfg.DocsDir, "/")
  	dec := d + "/decisions"
  	docs := map[string]string{}
  	for _, name := range p.Cfg.Docs {
  		docs[name] = p.docOutPath(name)
  	}
  	workflowRef := "AGENTS.md"
  	if wp, ok := docs["workflow"]; ok {
  		workflowRef = wp
  	}
  	return Layout{
  		DocsDir: d, ADRDir: dec, ActiveMd: dec + "/ACTIVE.md",
  		AdrReadme: dec + "/README.md", AdrTemplate: dec + "/template.md",
  		PlansDir: d + "/plans", PlansReadme: d + "/plans/README.md",
  		Docs: docs, WorkflowRef: workflowRef, DomainsDir: d + "/domains",
  	}
  }

  func (l Layout) templateMap() map[string]any {
  	docs := map[string]any{}
  	for k, v := range l.Docs {
  		docs[k] = v
  	}
  	return map[string]any{
  		"docsDir": l.DocsDir, "adrDir": l.ADRDir, "activeMd": l.ActiveMd,
  		"adrReadme": l.AdrReadme, "adrTemplate": l.AdrTemplate,
  		"plansDir": l.PlansDir, "plansReadme": l.PlansReadme,
  		"docs": docs, "workflowRef": l.WorkflowRef, "domainsDir": l.DomainsDir,
  	}
  }
  ```
- [ ] **6.2 Route templates + hash through `templateMap()`.**
  - In `data()` (render.go:38): `"layout": p.layout().templateMap()`.
  - In `targetConfigHash()` (render.go:364): `proj := map[string]any{"prefix": p.Cfg.Prefix, "layout": p.layout().templateMap()}`.
- [ ] **6.3 Use struct fields in the singleton loop.** In `RenderAll` (render.go:264-272), change
  `lay := p.layout()` consumers from `lay["adrReadme"].(string)` etc. to `lay.AdrReadme`,
  `lay.AdrTemplate`, `lay.PlansReadme`.
- [ ] **6.4 Use struct fields in `Audit`.** In `internal/project/project.go` (lines 153-155), change
  `lay["adrDir"].(string)` / `lay["activeMd"].(string)` / `lay["plansDir"].(string)` to
  `lay.ADRDir` / `lay.ActiveMd` / `lay.PlansDir`.
- [ ] **6.5 Rewrite `TestLayoutDerivesFromDocsDir`** (`internal/project/project_test.go:511-547`) for the
  struct, keeping every invariant marker (`layout-derivation`, `domains-dir-given`, `workflow-ref-fallback`,
  `layout-docs-enabled-only`) and adding a hash-stability assertion that `templateMap()` still carries the
  ten historical keys:
  ```go
  // invariant: layout-derivation
  func TestLayoutDerivesFromDocsDir(t *testing.T) {
  	p := &Project{Cfg: &config.Config{DocsDir: "documentation", Docs: []string{"architecture", "workflow"}}}
  	l := p.layout()
  	if l.DocsDir != "documentation" || l.ADRDir != "documentation/decisions" ||
  		l.ActiveMd != "documentation/decisions/ACTIVE.md" || l.AdrReadme != "documentation/decisions/README.md" ||
  		l.AdrTemplate != "documentation/decisions/template.md" || l.PlansDir != "documentation/plans" ||
  		l.PlansReadme != "documentation/plans/README.md" {
  		t.Errorf("layout = %+v", l)
  	}
  	// invariant: domains-dir-given
  	if l.DomainsDir != "documentation/domains" {
  		t.Errorf("domainsDir = %q", l.DomainsDir)
  	}
  	// invariant: workflow-ref-fallback (enabled arm)
  	if l.WorkflowRef != "documentation/workflow.md" {
  		t.Errorf("workflowRef = %q", l.WorkflowRef)
  	}
  	// invariant: layout-docs-enabled-only
  	wantDocs := map[string]string{
  		"architecture": "documentation/architecture.md",
  		"workflow":     "documentation/workflow.md",
  	}
  	if !reflect.DeepEqual(l.Docs, wantDocs) {
  		t.Errorf("Docs = %v, want %v", l.Docs, wantDocs)
  	}
  	// templateMap reproduces the historical .layout map (ConfigHash stability).
  	tm := l.templateMap()
  	for _, k := range []string{"docsDir", "adrDir", "activeMd", "adrReadme", "adrTemplate",
  		"plansDir", "plansReadme", "docs", "workflowRef", "domainsDir"} {
  		if _, ok := tm[k]; !ok {
  			t.Errorf("templateMap missing key %q", k)
  		}
  	}
  	// invariant: workflow-ref-fallback (fallback arm) — without the workflow doc enabled,
  	// workflowRef resolves to the always-present AGENTS.md.
  	noWf := &Project{Cfg: &config.Config{DocsDir: "documentation", Docs: []string{"architecture"}}}
  	if got := noWf.layout().WorkflowRef; got != "AGENTS.md" {
  		t.Errorf("workflowRef fallback = %v, want AGENTS.md", got)
  	}
  	if got := p.docOutPath("architecture"); got != "documentation/architecture.md" {
  		t.Errorf("docOutPath = %q", got)
  	}
  }
  ```
  Confirm `reflect` is already imported in `project_test.go` (it is — used at the old line 535).
- [ ] **6.6 Gate + commit** (`internal/project/render.go`, `internal/project/project.go`,
  `internal/project/project_test.go`): `refactor(awf): typed project Layout`. The gate's drift/golden
  tests must stay green — `templateMap()` keeps every `ConfigHash` byte-identical, so `.awf/awf.lock`
  does not change.

---

## Phase 7 — embed `config.AuditSettings` in `audit.Inputs`

- [ ] **7.1 Embed in `Inputs`.** In `internal/audit/audit.go`, add the import
  `"github.com/hypnotox/agentic-workflows/internal/config"` and replace the nine settings fields of
  `Inputs` (lines 72-77 + the four staleness/domain bools at 84-86) with the embedded struct, keeping the
  six project-derived fields:
  ```go
  // Inputs are the resolved audit settings plus the project-derived layout the rules need.
  type Inputs struct {
  	config.AuditSettings          // BaseBranch, AllowedTypes, AllowedScopes, SubjectMaxLength,
  	                              // DependencyManifests, DiffThreshold, DomainDocStaleness,
  	                              // UndocumentedDomain, UncommittedChanges
  	GeneratedPaths    map[string]bool
  	ADRDir            string // e.g. "docs/decisions"
  	ActiveMd          string // e.g. "docs/decisions/ACTIVE.md"
  	PlansDir          string // e.g. "docs/plans"
  	ConfiguredDomains []string
  	DomainsPartsDir   string // e.g. ".awf/domains/parts"
  }
  ```
  Rule bodies read promoted fields unchanged (`in.AllowedTypes`, `in.SubjectMaxLength`, `in.DiffThreshold`,
  `in.DomainDocStaleness`, `in.UndocumentedDomain`, `in.UncommittedChanges`, `in.BaseBranch`) — no edits to
  the rule functions.
- [ ] **7.2 Collapse the copy in `project.Audit`.** In `internal/project/project.go` (lines 145-161),
  replace the field-by-field `audit.Inputs{…}` with:
  ```go
  	return audit.Run(p.Root, audit.Inputs{
  		AuditSettings:     s,
  		GeneratedPaths:    generated,
  		ADRDir:            lay.ADRDir,
  		ActiveMd:          lay.ActiveMd,
  		PlansDir:          lay.PlansDir,
  		ConfiguredDomains: p.Cfg.Domains,
  		DomainsPartsDir:   ".awf/domains/parts",
  	})
  ```
  (`s := p.Cfg.ResolveAudit()` and the `baseOverride` line stay; `lay := p.layout()` is now the typed form.)
- [ ] **7.3 Nest the test `Inputs` literals.** Add `import "…/internal/config"` to both
  `internal/audit/audit_test.go` and `internal/audit/git_test.go`, then wrap any settings field in
  `AuditSettings:`. Mechanical rule: fields `BaseBranch / AllowedTypes / AllowedScopes / SubjectMaxLength /
  DependencyManifests / DiffThreshold / DomainDocStaleness / UndocumentedDomain / UncommittedChanges` move
  inside `config.AuditSettings{…}`; `ADRDir / ActiveMd / PlansDir / GeneratedPaths / ConfiguredDomains /
  DomainsPartsDir` stay flat. Example (audit_test.go:27):
  ```go
  in := Inputs{AuditSettings: config.AuditSettings{AllowedTypes: []string{"feat", "fix"}, AllowedScopes: []string{"awf"}, SubjectMaxLength: 20}}
  ```
  Sites to update — **audit_test.go:** 27, 96, 129, 153, 206, 251, 257, 266 (the mixed ones keep their
  flat fields and gain an `AuditSettings:` field). **git_test.go:** 151, 163, 238, 250, 256, 268 (all set
  only `BaseBranch`/`UncommittedChanges`, so each becomes `Inputs{AuditSettings: config.AuditSettings{…}}`).
  Literals that set *only* flat fields (audit_test.go:53 `Inputs{}`, :65, :108, :140, :199, :261) are
  unchanged. Resolve each against the file as it stands; `./x gate` (compile + 100% coverage) is the backstop.
- [ ] **7.4 Gate + commit** (`internal/audit/audit.go`, `internal/audit/audit_test.go`,
  `internal/audit/git_test.go`, `internal/project/project.go`):
  `refactor(awf): embed AuditSettings in audit.Inputs`.

---

## Final verification

- [ ] `./x gate` → `coverage: 100.0%` and `0 issues.`
- [ ] `./x check` → `awf check: clean` (ADR-0026 Implemented, both slugs backed; no drift).
- [ ] `awf audit` over the branch → no `error`-severity findings (clean working tree, Conventional Commits).
- [ ] `git grep -n 'writeArray\|editArray'` → no matches (both removed).
