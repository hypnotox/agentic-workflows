---
date: 2026-07-19
adrs: [132]
status: Proposed
---
# Plan: Structured Cross-Runtime Exploration Workflow

## Goal

Implement `/home/hypno/Projects/agentic-workflows/docs/decisions/0132-structured-cross-runtime-exploration-workflow.md`: ship a core cross-runtime exploring skill, migrate adopted configs into its dependency closure, and give Pi a required structured exploration schema and fixed bounded-reporting prompt.

Non-goals: exact non-Pi runtime API syntax, filesystem sandboxing, recursive child orchestration, retained search sessions, and changes to `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/runner.ts.tmpl`, `/home/hypno/Projects/agentic-workflows/.pi/extensions/awf-subagents/runner.ts`, or `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/extensions/awf-subagents/runner.ts`.

## Architecture summary

This plan is one coupled phase and one logical final behavior commit. Tests are written first and may be red in the working tree, but no intermediate state is gated or committed. The catalog, schema-generation-13 migration, consumer templates, Pi schema/prompt, authored docs, generated fanout, invariant markers, ADR lifecycle, and plan lifecycle land together.

The lifecycle coupling is mandatory, not convenient: ADR-0132 retires ADR-0125's `pi-subagent-four-tool-contract`, but that retirement is inactive while ADR-0132 remains Proposed. Removing ADR-0125's marker before the same commit flips ADR-0132 to Implemented would create an owed-but-unbacked invariant and fail the gate. Therefore the implementation replaces the marker, records deviations, flips both lifecycle files, syncs indexes, and runs the only gate before the only commit.

The exploration trigger is conjunctive everywhere: delegate only when the repository location is unknown **and** inline search is expected to pollute parent context. Exact-known-file reads and genuinely trivial lookups remain inline.

## File structure

All paths below are absolute.

- **Created:**
  - `/home/hypno/Projects/agentic-workflows/templates/skills/exploring/SKILL.md.tmpl`
  - generated `awf-exploring` and `sundial-exploring` copies selected by the fanout command in Task 1.7
- **Modified production and tests:**
  - `/home/hypno/Projects/agentic-workflows/internal/catalog/standard.go`
  - `/home/hypno/Projects/agentic-workflows/internal/catalog/graph_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/migrate/migrate.go`
  - `/home/hypno/Projects/agentic-workflows/internal/migrate/migrate_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/migrate/closeenabledset_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/project/project.go`
  - `/home/hypno/Projects/agentic-workflows/internal/project/version_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/project/scaffold_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/project/resolve_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/project/spine_test.go`
  - `/home/hypno/Projects/agentic-workflows/internal/evals/chain_test.go`
  - `/home/hypno/Projects/agentic-workflows/cmd/awf/run_test.go`
  - `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/index.test.ts`
  - `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/skills/debugging/SKILL.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl`
- **Modified authored documentation/config sources:**
  - `/home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/docs/doc-standard.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/README.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/config/current-state.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md`
  - `/home/hypno/Projects/agentic-workflows/examples/sundial/README.md`
  - `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/config.yaml`
  - `/home/hypno/Projects/agentic-workflows/examples/sundial/.awf/config.yaml`
- **Modified generated lifecycle/docs/config/fanout:**
  - `/home/hypno/Projects/agentic-workflows/docs/decisions/0132-structured-cross-runtime-exploration-workflow.md`
  - `/home/hypno/Projects/agentic-workflows/docs/plans/2026-07-19-structured-cross-runtime-exploration-workflow.md`
  - `/home/hypno/Projects/agentic-workflows/docs/decisions/ACTIVE.md`
  - `/home/hypno/Projects/agentic-workflows/docs/domains/config.md`
  - `/home/hypno/Projects/agentic-workflows/docs/domains/rendering.md`
  - `/home/hypno/Projects/agentic-workflows/docs/domains/tooling.md`
  - `/home/hypno/Projects/agentic-workflows/docs/architecture.md`
  - `/home/hypno/Projects/agentic-workflows/docs/releasing.md`
  - `/home/hypno/Projects/agentic-workflows/docs/testing.md`
  - `/home/hypno/Projects/agentic-workflows/docs/doc-standard.md`
  - `/home/hypno/Projects/agentic-workflows/docs/working-with-awf.md`
  - `/home/hypno/Projects/agentic-workflows/docs/config-reference.md`
  - `/home/hypno/Projects/agentic-workflows/AGENTS.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/awf.lock`
  - `/home/hypno/Projects/agentic-workflows/examples/sundial/.awf/awf.lock`
  - every generated skill, Pi extension, guide, and adopter copy returned by Task 1.7's affected-site command
- **Deleted:** none

## Phase 1: Implement and freeze the workflow in one coupled commit

### Task 1.1: Add exact failing Go catalog, scaffold, resolver, migration, and version tests

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/catalog/graph_test.go`, change `wantSkills` in `TestClosureChainUnit` by inserting `"exploring"` in sorted order. Add this exact test:

  ```go
  func TestExploringRequirementsAreOneWay(t *testing.T) {
    for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
        if !slices.Contains(Standard.Skills[consumer].RequiresSkills, "exploring") {
            t.Errorf("%s does not require exploring", consumer)
        }
    }
    for _, forbidden := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
        if slices.Contains(Standard.Skills["exploring"].RequiresSkills, forbidden) {
            t.Errorf("exploring has reciprocal requirement on %s", forbidden)
        }
    }
  }
  ```

  Add `"slices"` to that file's imports.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/scaffold_test.go`, put `// invariant: exploration-skill-closure` immediately above the existing core-set comparison in `TestScaffoldEnablesCoreTargets`, then add `if !slices.Contains(cfg.Skills, "exploring") { t.Error("core scaffold missing exploring") }`. In `TestScaffoldCatalogTrim`, replace both fragile count blocks for `cfg.Skills` and `added` with this exact derived expectation; keep the existing agent, representative-entry, docs, leaf, and gated-doc assertions:

  ```go
    wantNodes := catalog.Closure(cat, []catalog.Node{
        {Kind: "skill", Name: "tdd"},
        {Kind: "skill", Name: "brainstorming"},
    })
    wantSkills, wantAdded := map[string]bool{}, map[string]bool{}
    selected := map[string]bool{"tdd": true, "brainstorming": true}
    for _, node := range wantNodes {
        switch node.Kind {
        case "skill":
            wantSkills[node.Name] = true
            if !selected[node.Name] {
                wantAdded["skill "+node.Name] = true
            }
        case "agent":
            wantAdded["agent "+node.Name] = true
        }
    }
    if got := sliceSet(cfg.Skills); !maps.Equal(got, wantSkills) {
        t.Errorf("closure-completed trim skills = %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantSkills)))
    }
    if got := sliceSet(added); !maps.Equal(got, wantAdded) {
        t.Errorf("closure additions = %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantAdded)))
    }
  ```

- [ ] Treat `/home/hypno/Projects/agentic-workflows/internal/project/resolve_test.go` as a repeated-site batch. Apply this literal import and helper diff:

  ```diff
   import (
  +    "maps"
  +    "slices"
       "testing"

       "github.com/hypnotox/agentic-workflows/internal/catalog"
   )
  +
  +func planNodes(plan []PlanOp) map[catalog.Node]bool {
  +    got := make(map[catalog.Node]bool, len(plan))
  +    for _, op := range plan {
  +        got[op.Node] = true
  +    }
  +    return got
  +}
  +
  +func closureNodes(nodes []catalog.Node) map[catalog.Node]bool {
  +    got := make(map[catalog.Node]bool, len(nodes))
  +    for _, node := range nodes {
  +        got[node] = true
  +    }
  +    return got
  +}
  ```

  Replace the stale opening comments and all of `TestResolveDisableCascadeSizes` with this compilable edge assertion:

  ```diff
  -// openChainProject opens a project with the full 11-skill chain closure and
  +// openChainProject opens a project with the full derived chain closure and
   // its three agents enabled (the drift-test fixture builder).
  @@
  -// Cascade sizes are seed-dependent (ADR-0081 Decision 5): the closure has two
  -// mutually-requiring cores (planning 5, execution 3) with edges only
  -// planning→execution, brainstorming a pure source, and retrospective/
  -// adr-lifecycle sinks. Counts verified against the catalog on 2026-07-09.
  +// Cascade members are seed-dependent (ADR-0081 Decision 5). Pin names, not
  +// corpus sizes, so an unrelated closure addition cannot silently shift scope.
   // invariant: remove-refuses-dependents
   func TestResolveDisableCascadeSizes(t *testing.T) {
       p := openChainProject(t)
       cases := []struct {
           seed string
  -        ops  int
  +        want []string
       }{
  -        {"brainstorming", 1},    // pure source: nothing requires it
  -        {"reviewing-plan", 7},   // planning core + brainstorming + plan-reviewer
  -        {"executing-plans", 10}, // both cores + brainstorming + plan-reviewer
  -        {"retrospective", 11},   // worst case: 10 skills + plan-reviewer
  +        {"brainstorming", []string{"skill brainstorming"}},
  +        {"reviewing-plan", []string{"agent plan-reviewer", "skill brainstorming", "skill proposing-adr", "skill reviewing-adr", "skill reviewing-plan", "skill reviewing-plan-resync", "skill writing-plans"}},
  +        {"executing-plans", []string{"agent plan-reviewer", "skill brainstorming", "skill executing-plans", "skill proposing-adr", "skill reviewing-adr", "skill reviewing-impl", "skill reviewing-plan", "skill reviewing-plan-resync", "skill subagent-driven-development", "skill writing-plans"}},
  +        {"retrospective", []string{"agent plan-reviewer", "skill brainstorming", "skill executing-plans", "skill proposing-adr", "skill retrospective", "skill reviewing-adr", "skill reviewing-impl", "skill reviewing-plan", "skill reviewing-plan-resync", "skill subagent-driven-development", "skill writing-plans"}},
       }
       for _, tc := range cases {
  -        if plan := p.ResolveDisable("skill", tc.seed); len(plan) != tc.ops {
  -            t.Errorf("ResolveDisable(%q) = %d ops, want %d: %v", tc.seed, len(plan), tc.ops, plan)
  +        plan := p.ResolveDisable("skill", tc.seed)
  +        got := make([]string, 0, len(plan))
  +        for _, op := range plan {
  +            got = append(got, op.Node.Kind+" "+op.Node.Name)
  +        }
  +        slices.Sort(got)
  +        if !slices.Equal(got, tc.want) {
  +            t.Errorf("ResolveDisable(%q) = %v, want %v", tc.seed, got, tc.want)
           }
       }
   }
  ```

  Replace the representative full-closure length assertion and the partial-closure edge with these literal diffs; retain the existing seed-first, `RequiredBy`, enabled-leaf, and enabled-dependency assertions around them:

  ```diff
  -// The add plan on an empty config is the seed's full forward closure - the
  -// 11-skill chain unit plus its three agents from the brainstorming seed.
  +// The add plan on an empty config is the seed's full forward closure.
   // invariant: add-applies-closure-plan
   func TestResolveEnableClosurePlan(t *testing.T) {
  @@
       }
       plan := p.ResolveEnable("skill", "brainstorming")
  -    if len(plan) != 14 {
  -        t.Fatalf("ResolveEnable(brainstorming) = %d ops, want 14 (11 skills + 3 agents): %v", len(plan), plan)
  +    closure := catalog.Closure(catalog.Standard, []catalog.Node{{Kind: "skill", Name: "brainstorming"}})
  +    if got, want := planNodes(plan), closureNodes(closure); !maps.Equal(got, want) {
  +        t.Fatalf("ResolveEnable(brainstorming) nodes = %v, want %v", got, want)
       }
  @@
  -    // Enabled-subtree skip mid-walk: with the execution core already enabled,
  -    // brainstorming's closure plans only the planning side (7 skills incl. the
  -    // seed + adr-reviewer + plan-reviewer = 9 ops), never re-adding members.
  +    // Enabled-subtree skip mid-walk never re-adds an enabled member.
       p3, err := Open(scaffold(t, "prefix: example\nskills: [executing-plans, retrospective, reviewing-impl, subagent-driven-development]\nagents: [code-reviewer]\n"))
  @@
       }
       plan3 := p3.ResolveEnable("skill", "brainstorming")
  -    if len(plan3) != 9 {
  -        t.Fatalf("partial-closure add = %d ops, want 9: %v", len(plan3), plan3)
  +    enabled := map[catalog.Node]bool{
  +        {Kind: "skill", Name: "executing-plans"}:             true,
  +        {Kind: "skill", Name: "retrospective"}:               true,
  +        {Kind: "skill", Name: "reviewing-impl"}:              true,
  +        {Kind: "skill", Name: "subagent-driven-development"}: true,
  +        {Kind: "agent", Name: "code-reviewer"}:               true,
  +    }
  +    want3 := map[catalog.Node]bool{}
  +    for _, node := range closure {
  +        if !enabled[node] {
  +            want3[node] = true
  +        }
  +    }
  +    if got := planNodes(plan3); !maps.Equal(got, want3) {
  +        t.Fatalf("partial-closure add nodes = %v, want %v", got, want3)
       }
  ```

  The deterministic post-check is:

  ```bash
  if rg -n 'ops  int|len\(plan|11-skill|11 skills|planning side' /home/hypno/Projects/agentic-workflows/internal/project/resolve_test.go; then exit 1; fi
  ```

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/migrate/migrate_test.go`, append `exploring-skill-closure` to both exact ordered migration-name strings in `TestUpgradeAppliesInOrderIdempotent` and `TestUpgradeStampsTreeLock`, rename `TestCurrentIsTwelve` to `TestCurrentIsThirteen`, and assert `Current() == 13`.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/migrate/closeenabledset_test.go`, add a real-standard fixture, not a synthetic catalog fixture:

  ```go
  func TestCloseEnabledSetAddsExploringFromShippedCatalog(t *testing.T) {
    for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
        t.Run(consumer, func(t *testing.T) {
            root := closeFixture(t, "prefix: ex\nskills: ["+consumer+"]\nagents: []\n", nil)
            var out bytes.Buffer
            if err := applyCloseEnabledSet(root, &out); err != nil {
                t.Fatalf("applyCloseEnabledSet: %v", err)
            }
            want := `close-enabled-set: enabled skill "exploring" (required by "` + consumer + `")`
            if !strings.Contains(out.String(), want) {
                t.Errorf("diagnostic missing %q:\n%s", want, out.String())
            }
            before, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
            if err != nil {
                t.Fatal(err)
            }
            if !strings.Contains(string(before), "- exploring") {
                t.Errorf("upgraded config missing exploring:\n%s", before)
            }
            var second bytes.Buffer
            if err := applyCloseEnabledSet(root, &second); err != nil {
                t.Fatalf("second applyCloseEnabledSet: %v", err)
            }
            after, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
            if err != nil {
                t.Fatal(err)
            }
            if second.Len() != 0 || !bytes.Equal(before, after) {
                t.Errorf("second apply changed output=%q config=\n%s", second.String(), after)
            }
        })
  }
  ```

  This uses `applyCloseEnabledSet` and `catalog.Standard`, so it proves the shipped generation-13 operation. Keep the existing synthetic `closeEnabledSet` test because it proves the distinct dormant-doc interaction, not exploration migration.

- [ ] In `/home/hypno/Projects/agentic-workflows/cmd/awf/run_test.go`, add the `config` import and this literal compilable test. It constructs a real schema-12 lock, calls the existing upgrade handler, and uses only current package APIs:

  ```diff
   import (
  @@
       "testing"

  +    "github.com/hypnotox/agentic-workflows/internal/config"
       "github.com/hypnotox/agentic-workflows/internal/manifest"
       "github.com/hypnotox/agentic-workflows/internal/testsupport"
   )
  @@
  +func TestRunUpgradeAddsExploringAtSchemaThirteen(t *testing.T) {
  +    root := t.TempDir()
  +    testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {}\nskills: [debugging]\nagents: []\n")
  +    lock := &manifest.Lock{AWFVersion: "0.17.0", SchemaVersion: 12, Files: map[string]manifest.Entry{}}
  +    if err := lock.Save(config.LockPath(root)); err != nil {
  +        t.Fatal(err)
  +    }
  +
  +    var out bytes.Buffer
  +    if err := runUpgrade(root, &out); err != nil {
  +        t.Fatalf("runUpgrade: %v", err)
  +    }
  +    for _, want := range []string{
  +        `close-enabled-set: enabled skill "exploring" (required by "debugging")`,
  +        "awf upgrade: applied exploring-skill-closure",
  +    } {
  +        if !strings.Contains(out.String(), want) {
  +            t.Errorf("upgrade output missing %q:\n%s", want, out.String())
  +        }
  +    }
  +    upgradedLock, err := manifest.Load(config.LockPath(root))
  +    if err != nil {
  +        t.Fatal(err)
  +    }
  +    if upgradedLock.SchemaVersion != 13 {
  +        t.Errorf("lock schema = %d, want 13", upgradedLock.SchemaVersion)
  +    }
  +    cfg, err := config.Load(config.RootDir(root))
  +    if err != nil {
  +        t.Fatal(err)
  +    }
  +    found := false
  +    for _, skill := range cfg.Skills {
  +        if skill == "exploring" {
  +            found = true
  +        }
  +    }
  +    if !found {
  +        t.Errorf("upgraded skills missing exploring: %v", cfg.Skills)
  +    }
  +    if err := runCheck(root, io.Discard); err != nil {
  +        t.Errorf("post-upgrade check: %v", err)
  +    }
  +}
  ```

  This is distinct from the direct migration test because it proves registry selection, lock stamping, terminal sync, and project-open/check behavior.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/version_test.go`, retain the existing generic coverage assertions and append these exact terminal assertions inside `TestVersionCoversCurrentSchema`:

  ```diff
       if semver.Compare("v"+Version, "v"+min) < 0 {
           t.Errorf("project.Version %s is below the minimum %s for schema generation %d; bump the const (ADR-0049 Decision 4)", Version, min, migrate.Current())
       }
  +    if migrate.Current() != 13 {
  +        t.Errorf("migrate.Current() = %d, want 13", migrate.Current())
  +    }
  +    if minVersionBySchema[13] != "0.17.0" {
  +        t.Errorf("minVersionBySchema[13] = %q, want %q", minVersionBySchema[13], "0.17.0")
  +    }
  +    if Version != "0.17.0" {
  +        t.Errorf("Version = %q, want %q", Version, "0.17.0")
  +    }
   }
  ```

  There is no implementation-time version decision.

Run the focused red test command:

```bash
go test github.com/hypnotox/agentic-workflows/internal/catalog github.com/hypnotox/agentic-workflows/internal/migrate github.com/hypnotox/agentic-workflows/internal/project github.com/hypnotox/agentic-workflows/cmd/awf
```

Expected: non-zero only because `exploring`, generation 13, and its compatibility entry do not yet exist. Do not commit.

### Task 1.2: Add exact failing render, publication-safety, composed-eval, and order tests

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`, replace the old contract test with this helper and literal test body. The helper slices each registration from its tool name to the next registration, so unchanged role schemas are checked independently:

  ```diff
  -// invariant: pi-subagent-four-tool-contract
  -func TestPiSubagentFourToolContract(t *testing.T) {
  +func registrationBlock(t *testing.T, content, name, nextMarker string) string {
  +    t.Helper()
  +    start := strings.Index(content, `name: "`+name+`"`)
  +    if start < 0 {
  +        t.Fatalf("cannot find registration %s", name)
  +    }
  +    relativeEnd := strings.Index(content[start:], nextMarker)
  +    if relativeEnd <= 0 {
  +        t.Fatalf("cannot isolate registration %s before %s", name, nextMarker)
  +    }
  +    return content[start : start+relativeEnd]
  +}
  +
  +// invariant: pi-structured-exploration-contract
  +func TestPiStructuredExplorationContract(t *testing.T) {
       content := renderPiExtensionFile(t, "index.ts")
       for _, name := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"} {
           if strings.Count(content, `name: "`+name+`"`) != 1 {
  @@
       if got := strings.Count(content, `name: "subagent_`); got != 4 {
           t.Errorf("public subagent registration count = %d, want 4", got)
       }
  -    for _, want := range []string{
  -        `name: "subagent_grounding"`, `rolePrompt("grounding")`,
  -        `name: "subagent_explore"`, `rolePrompt("explore")`,
  -        `name: "subagent_review"`, `StringEnum(["adr", "plan", "code"]`,
  -        `name: "subagent_implement"`, `allowCommits: Type.Boolean()`,
  -        `task: Type.String({ minLength: 1 })`, `{ additionalProperties: false }`,
  -    } {
  -        if !strings.Contains(content, want) {
  -            t.Errorf("extension missing four-tool schema/role contract %q", want)
  +    blocks := map[string]string{
  +        "grounding": registrationBlock(t, content, "subagent_grounding", `name: "subagent_explore"`),
  +        "explore":   registrationBlock(t, content, "subagent_explore", `name: "subagent_review"`),
  +        "review":    registrationBlock(t, content, "subagent_review", `name: "subagent_implement"`),
  +        "implement": registrationBlock(t, content, "subagent_implement", "export default async function"),
  +    }
  +    wants := map[string][]string{
  +        "grounding": {`parameters: Type.Object({ task: Type.String({ minLength: 1 }) }, { additionalProperties: false })`, `rolePrompt("grounding")`},
  +        "explore": {`task: Type.String({ minLength: 1 })`, `breadth: StringEnum(["targeted", "bounded", "broad"] as const)`, `detail: StringEnum(["paths", "summary", "analysis"] as const)`, `{ additionalProperties: false }`, `rolePrompt("explore", { breadth: params.breadth, detail: params.detail })`},
  +        "review": {`kind: StringEnum(["adr", "plan", "code"] as const)`, `task: Type.String({ minLength: 1 })`, `{ additionalProperties: false }`},
  +        "implement": {`task: Type.String({ minLength: 1 })`, `allowCommits: Type.Boolean()`, `{ additionalProperties: false }`, `rolePrompt("implement", { allowCommits: params.allowCommits })`},
  +    }
  +    for role, required := range wants {
  +        for _, want := range required {
  +            if !strings.Contains(blocks[role], want) {
  +                t.Errorf("%s registration missing %q:\n%s", role, want, blocks[role])
  +            }
           }
       }
   }
  ```

- [ ] In the same file add these complete helpers and test bodies. They use only existing `scaffold`, `Open`, `RenderAll`, `KnownTargets`, and golden-render helpers:

  ```go
  func explorationFixtureConfig(target string) string {
      return "prefix: example\nskills: [adr-lifecycle, brainstorming, debugging, executing-plans, exploring, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, writing-plans]\nagents: [adr-reviewer, code-reviewer, plan-reviewer]\ntargets: [" + target + "]\n"
  }

  func renderedByPath(t *testing.T, config string) map[string]string {
      t.Helper()
      p, err := Open(scaffold(t, config))
      if err != nil {
          t.Fatal(err)
      }
      files, err := p.RenderAll()
      if err != nil {
          t.Fatal(err)
      }
      got := map[string]string{}
      for _, file := range files {
          got[file.Path] = file.Content
      }
      return got
  }

  // invariant: cross-runtime-exploration-dispatch
  func TestCrossRuntimeExplorationDispatch(t *testing.T) {
      dirs := map[string]string{
          "claude": ".claude/skills", "codex": ".agents/skills", "copilot": ".github/skills",
          "cursor": ".cursor/skills", "gemini": ".gemini/skills", "pi": ".pi/skills",
      }
      for _, target := range KnownTargets() {
          t.Run(target, func(t *testing.T) {
              files := renderedByPath(t, explorationFixtureConfig(target))
              base := dirs[target] + "/example-"
              exploring := files[base+"exploring/SKILL.md"]
              if exploring == "" {
                  t.Fatalf("missing rendered exploring skill for %s", target)
              }
              if target == "pi" {
                  for _, want := range []string{"subagent_explore", "task", "breadth", "detail"} {
                      if !strings.Contains(exploring, want) {
                          t.Errorf("Pi exploring skill missing %q", want)
                      }
                  }
              } else {
                  for _, want := range []string{"target-native fresh-context exploration subagent", "task", "breadth", "detail"} {
                      if !strings.Contains(exploring, want) {
                          t.Errorf("%s exploring skill missing %q", target, want)
                      }
                  }
                  if strings.Contains(exploring, "subagent_explore") {
                      t.Errorf("%s exploring skill leaks Pi tool name", target)
                  }
              }
              for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
                  body := files[base+consumer+"/SKILL.md"]
                  for _, want := range []string{"location is unknown", "and inline search would pollute the parent context", "exact-known-file", "genuinely trivial"} {
                      if !strings.Contains(body, want) {
                          t.Errorf("%s/%s missing dispatch condition %q", target, consumer, want)
                      }
                  }
              }
          })
      }
  }

  // invariant: bounded-exploration-reporting
  func TestBoundedExplorationReporting(t *testing.T) {
      for _, target := range []string{"gemini", "pi"} {
          files := renderedByPath(t, "prefix: example\nskills: [exploring]\nagents: []\ntargets: ["+target+"]\n")
          dir := map[string]string{"gemini": ".gemini/skills", "pi": ".pi/skills"}[target]
          body := files[dir+"/example-exploring/SKILL.md"]
          for _, want := range []string{
              "targeted < bounded < broad", "paths < summary < analysis", "adaptive maximum",
              "tracked files plus non-ignored untracked", "ignored files", ".git", "nested repositories", "external dependencies",
              "Not found within <breadth> boundary:", "inconclusive", "unverified", "one information need", "new fresh-context call",
          } {
              if !strings.Contains(body, want) {
                  t.Errorf("%s exploring skill missing %q", target, want)
              }
          }
      }
      fallback := renderSkillGolden(t, "exploring", map[string]any{
          "prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{},
      })
      if strings.Contains(fallback, "subagent_explore") || !strings.Contains(fallback, "target-native fresh-context exploration subagent") {
          t.Errorf("empty-capability exploring render has incoherent dispatch:\n%s", fallback)
      }
  }
  ```

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/spine_test.go`, insert this complete golden test beside the other skill goldens:

  ```go
  func TestExploringTemplate(t *testing.T) {
      pi := renderSkillGolden(t, "exploring", map[string]any{
          "prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
          "skills": map[string]bool{}, "targetSubagentTools": true,
      })
      fallback := renderSkillGolden(t, "exploring", map[string]any{
          "prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{},
      })
      for label, body := range map[string]string{"pi": pi, "fallback": fallback} {
          for _, want := range []string{
              "location is unknown and inline search would pollute the parent context",
              "exact-known-file", "genuinely trivial",
          } {
              if !strings.Contains(body, want) {
                  t.Errorf("%s exploring render missing %q:\n%s", label, want, body)
              }
          }
      }
      for _, want := range []string{"subagent_explore", "required task, breadth, and detail"} {
          if !strings.Contains(pi, want) {
              t.Errorf("Pi exploring render missing %q:\n%s", want, pi)
          }
      }
      if !strings.Contains(fallback, "target-native fresh-context exploration subagent") || strings.Contains(fallback, "subagent_explore") {
          t.Errorf("fallback exploring dispatch is not generic:\n%s", fallback)
      }
  }
  ```

  In `TestDebuggingTemplate`, apply this literal data and assertion diff after the existing phrase loop:

  ```diff
  -    "skills": map[string]bool{"tdd": true, "bugfix": true, "brainstorming": true},
  +    "skills": map[string]bool{"tdd": true, "bugfix": true, "brainstorming": true, "exploring": true},
  @@
       for _, phrase := range loadBearing {
  @@
       }
  +    ordered := []string{
  +        "**Form one falsifiable hypothesis.**",
  +        "Invoke `example-exploring`",
  +        "Pick the cheapest oracle",
  +        "**Isolate with a failing test, written first.**",
  +    }
  +    position := -1
  +    for _, phrase := range ordered {
  +        next := strings.Index(out, phrase)
  +        if next <= position {
  +            t.Fatalf("debugging order violation at %q: positions must increase in %v", phrase, ordered)
  +        }
  +        position = next
  +    }
   }
  ```

  This proves hypothesis formation, exploration/evidence validation, oracle handling, then test-first isolation.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/evals/chain_test.go`, add this complete composed seam test. It uses the existing Pi full-catalog fixture and invocation-line helper and deliberately does not duplicate prompt-policy assertions:

  ```go
  func TestExplorationConsumerToPiToolSeam(t *testing.T) {
      cat := loadCatalog(t)
      root := syncFullCatalogForTarget(t, cat, "pi")
      for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
          body := read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-"+consumer, "SKILL.md"))
          if !namesOnInvocationLine(body, evalPrefix+"-exploring") {
              t.Errorf("Pi consumer %q does not invoke %q", consumer, evalPrefix+"-exploring")
          }
      }
      exploring := read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-exploring", "SKILL.md"))
      if !namesOnInvocationLine(exploring, "subagent_explore") {
          t.Error("Pi exploring skill does not invoke subagent_explore")
      }
      extension := read(t, filepath.Join(root, ".pi", "extensions", "awf-subagents", "index.ts"))
      if !strings.Contains(extension, `name: "subagent_explore"`) {
          t.Error("Pi extension does not register subagent_explore")
      }
  }
  ```

Run:

```bash
go test github.com/hypnotox/agentic-workflows/internal/project github.com/hypnotox/agentic-workflows/internal/evals
```

Expected: non-zero for the absent template, dispatch, schema, and consumer wording only. Do not commit.

### Task 1.3: Add exact failing Pi schema and prompt tests

- [ ] In `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/index.test.ts`, replace `registers exactly four governed public tools with closed grounding schema` with `registers exactly four governed public tools with structured exploration schema`. Keep every existing grounding/review/implementation and allowlist assertion. Add this exact exploration schema matrix:

  ```ts
  const exploreSchema = h.tools.get("subagent_explore").parameters;
  for (const breadth of ["targeted", "bounded", "broad"])
    for (const detail of ["paths", "summary", "analysis"])
      assert.equal(Value.Check(exploreSchema, { task: "inspect", breadth, detail }), true);
  for (const invalid of [
    {}, { task: "inspect" }, { task: "", breadth: "targeted", detail: "paths" },
    { task: "inspect", breadth: "targeted" }, { task: "inspect", detail: "paths" },
    { task: "inspect", breadth: "unbounded", detail: "paths" },
    { task: "inspect", breadth: "targeted", detail: "verbose" },
    { task: "inspect", breadth: "targeted", detail: "paths", extra: true },
  ]) assert.equal(Value.Check(exploreSchema, invalid), false);
  assert.deepEqual(exploreSchema.required, ["task", "breadth", "detail"]);
  assert.equal(exploreSchema.additionalProperties, false);
  ```

- [ ] Replace every successful `subagent_explore` execution parameter in that file with all three fields. Add `exploration forwards every breadth and detail into the fixed prompt`: execute all nine enum combinations (which includes the representative edges `broad + paths` and `targeted + analysis`) and assert each resulting request prompt contains `Selected breadth maximum: <breadth>` and `Selected report detail: <detail>`.

- [ ] In the existing isolation test, call exploration with `{ task: "inspect", breadth: "bounded", detail: "summary" }` and assert the prompt contains each exact policy fragment: `adaptive maximum`; definitions for targeted, bounded, and broad; `tracked files plus non-ignored untracked working-tree files`; ignored files, `.git`, nested repositories, and external dependency exclusions; `Not found within <breadth> boundary:`; `inconclusive`; `unverified`; evidence grounding; one information need; `paths`, `summary`, and `analysis`; no edits, commits, or recursive delegation; and sequential refinement only through `a new fresh-context call`. Preserve assertions for root cwd, inherited model/thinking level, `EXPLORE_TOOLS`, partial-details isolation, final-only model content, failures, and absence of any `subagent_` child tool.

Run:

```bash
/home/hypno/Projects/agentic-workflows/x pi-test run
```

Expected: non-zero only because exploration still has the one-field schema and one-line prompt. Do not commit.

### Task 1.4: Implement the catalog, generation-13 migration, exact version mapping, and exploring template

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/catalog/standard.go`, insert this exact catalog entry and add `"exploring"` once to each of brainstorming, debugging, and refactor-coupling-audit's `RequiresSkills` lists:

  ```go
  "exploring": {Core: true, Sections: []string{
    "when-to-invoke", "breadth", "detail", "dispatch", "results", "boundaries", "notes",
  }},
  ```

  Do not set `Chain` and do not add requirements to exploring.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/migrate/migrate.go`, append this exact registry entry:

  ```go
  {To: 13, Name: "exploring-skill-closure", Apply: applyCloseEnabledSet},
  ```

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/project.go`, leave `const Version = "0.17.0"` exactly unchanged and append exactly `13: "0.17.0",` to `minVersionBySchema`. `TestVersionCoversCurrentSchema` and the final gate verify this fixed choice.

- [ ] Create `/home/hypno/Projects/agentic-workflows/templates/skills/exploring/SKILL.md.tmpl` with the catalog's seven section markers and these exact semantic paragraphs (headings/frontmatter may wrap, but meaning and quoted protocol strings must not change):
  - frontmatter name `{{ .prefix }}-exploring`; description: `Use when a repository information need has both an unknown location and an inline search expected to pollute parent context. Keeps exact-known-file and genuinely trivial lookups inline.`
  - `when-to-invoke`: `Invoke this skill only when both the repository location is unknown and inline search would pollute the parent context. Keep an exact-known-file read or genuinely trivial lookup inline. Send one information need per call.`
  - `breadth`: define `targeted < bounded < broad` exactly as ADR-0132 Decision 4, call the selected value an adaptive maximum, start with the cheapest lookup, and never widen beyond it. Define the broad universe as tracked plus non-ignored untracked working-tree files under repository root, including tracked generated/vendor files and excluding ignored files, `.git`, nested repositories, and external dependencies unless explicitly scoped.
  - `detail`: define `paths < summary < analysis` independently, with paths-only locations/minimal labels, grounded summaries, and evidence-grounded synthesis/uncertainty respectively.
  - `dispatch`: require one self-contained task. Under `{{ if .targetSubagentTools }}`, say `Call subagent_explore exactly once with required task, breadth, and detail.` Otherwise say `Dispatch one target-native fresh-context exploration subagent with task, breadth, detail, boundary, outcome, and report contracts in its brief.`
  - `results`: require material claims to have file/line evidence; require exact prefix `Not found within <breadth> boundary: <what was searched>`; distinguish inconclusive and unverified from absence; consume only relevant final findings; permit refinement only through a new fresh-context call.
  - `boundaries`: report-only, no edits, commits, recursive delegation, widening past maximum, unrelated bundled need, or retained state.
  - `notes`: Pi is deeply integrated; non-Pi targets have semantic parity through native delegation, not identical orchestration.

Run:

```bash
gofmt -w /home/hypno/Projects/agentic-workflows/internal/catalog/standard.go /home/hypno/Projects/agentic-workflows/internal/catalog/graph_test.go /home/hypno/Projects/agentic-workflows/internal/migrate/migrate.go /home/hypno/Projects/agentic-workflows/internal/migrate/migrate_test.go /home/hypno/Projects/agentic-workflows/internal/migrate/closeenabledset_test.go /home/hypno/Projects/agentic-workflows/internal/project/project.go /home/hypno/Projects/agentic-workflows/internal/project/version_test.go /home/hypno/Projects/agentic-workflows/internal/project/scaffold_test.go /home/hypno/Projects/agentic-workflows/internal/project/resolve_test.go /home/hypno/Projects/agentic-workflows/internal/project/target_test.go /home/hypno/Projects/agentic-workflows/internal/project/spine_test.go /home/hypno/Projects/agentic-workflows/internal/evals/chain_test.go /home/hypno/Projects/agentic-workflows/cmd/awf/run_test.go
go test github.com/hypnotox/agentic-workflows/internal/catalog github.com/hypnotox/agentic-workflows/internal/migrate github.com/hypnotox/agentic-workflows/internal/project github.com/hypnotox/agentic-workflows/cmd/awf
```

Expected: catalog, scaffold, resolver, migration, and version tests pass; consumer/render tests remain red until Task 1.5.

### Task 1.5: Replace consumer orchestration and implement Pi's exact contract

- [ ] Apply this repeated consumer-template batch. The exhaustive affected-site set is exactly:
  - `/home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/skills/debugging/SKILL.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl`

  Apply these literal before/after diffs. In brainstorming, replace only step 1; the later `subagent_grounding` branch stays byte-for-byte unchanged:

  ```diff
  -1. **Explore project context.** Read `AGENTS.md`, relevant docs (architecture, workflow, testing), recent commits in the affected area (`git log --oneline -20 <path>`). Check domain docs under `{{ .layout.domainsDir }}`. Once you have identified the candidate files the work touches, run `awf context <paths>` to resolve their owning domains, backed invariants, and related ADRs; read the current-state docs and ADRs it surfaces.
  +1. **Explore project context.** When both the needed repository information's location is unknown and inline search would pollute the parent context, invoke `{{ .prefix }}-exploring` with one information need, breadth, and detail. Keep an exact-known-file read or genuinely trivial lookup inline. Read `AGENTS.md`, relevant docs (architecture, workflow, testing), recent commits in the affected area (`git log --oneline -20 <path>`). Check domain docs under `{{ .layout.domainsDir }}`. Once you have identified the candidate files the work touches, run `awf context <paths>` to resolve their owning domains, backed invariants, and related ADRs; read the current-state docs and ADRs it surfaces.
  ```

  In debugging, replace only step 3, preserving hypothesis formation before it and test isolation after it:

  ```diff
  -3. **Enumerate observable surfaces and validate the hypothesis.** Pick the cheapest oracle that can confirm or refute it. Examples of surfaces to consider: application logs, database state, HTTP endpoints, generated artifacts, pipeline stage outputs, and the test assertion output itself. Inspect the surface that the hypothesis predicts will be wrong. Update the hypothesis if the evidence refutes it and loop.
  +3. **Enumerate observable surfaces and validate the hypothesis.** Invoke `{{ .prefix }}-exploring` with one falsifiable evidence need, breadth, and detail only if both the evidence location is unknown and inline search would pollute the parent context; keep exact-known-file and genuinely trivial checks inline. Pick the cheapest oracle that can confirm or refute it. Examples of surfaces to consider: application logs, database state, HTTP endpoints, generated artifacts, pipeline stage outputs, and the test assertion output itself. Inspect the surface that the hypothesis predicts will be wrong. Update the hypothesis if the evidence refutes it and loop.
  ```

  In the refactor coupling audit, replace the complete current size-threshold paragraph:

  ```diff
  -**Pick the audit shape.** For a small-scope refactor (1-3 files), run the audit inline as a sequence of `grep` and file reads in the main session. {{ if .targetSubagentTools }}For a large-scope refactor (10+ files or coupling surfaces across 5+ packages), call `subagent_explore` exactly once with all six audit categories and the required structured output in `task`.{{ else }}For a large-scope refactor (10+ files or coupling surfaces across 5+ packages), dispatch a single fresh-context exploration subagent to absorb the grep transcript noise.{{ end }}
  +**Pick the audit shape.** When both the coupling evidence location is unknown and inline search would pollute the parent context, invoke `{{ .prefix }}-exploring` once per information need with breadth and detail. Keep an exact-known-file or genuinely trivial category check inline. Preserve all six categories and the structured output contract.
  ```

  This removes every direct `subagent_explore` reference from all three consumers.

  Deterministic post-check:

  ```bash
  rg -n 'location is unknown.*and.*inline search would pollute|exact-known-file|genuinely trivial' /home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/debugging/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl
  if rg -n 'subagent_explore' /home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/debugging/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl; then exit 1; fi
  ```

  Expected: the first command finds the conjunctive trigger and exception in all three files; the second exits zero because its inner search finds nothing.

- [ ] In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`, apply these literal diffs. First add the two types and replace `rolePrompt` in full; the grounding strings and implementation string remain unchanged except for reading `options.allowCommits`:

  ```diff
   type ReviewKind = keyof typeof REVIEWER_PATHS;
  +type ExplorationBreadth = "targeted" | "bounded" | "broad";
  +type ExplorationDetail = "paths" | "summary" | "analysis";
   interface GitSnapshot { available: boolean; head?: string; status?: string; }
  @@
  -function rolePrompt(role: "grounding" | "explore" | "implement", allowCommits?: boolean): string {
  +function rolePrompt(role: "grounding" | "explore" | "implement", options: { allowCommits?: boolean; breadth?: ExplorationBreadth; detail?: ExplorationDetail } = {}): string {
       if (role === "grounding") {
         return [
           "You are a fresh-context grounding-check subagent. Read and run evidence-producing commands, but do not edit files or commit.",
           "Verify the supplied design's factual premises against source and architecture. Surface unstated assumptions and edge cases. Assess whether the effort needs an ADR, a plan, or narrower scope. Check Accepted or Implemented ADR and invariant fit.",
           "Return findings only as {kind: open-question | possible-issue, topic, detail, grounding, confidence: verified | interpreted | unverified}.",
           "verified means mechanically confirmed against source; interpreted means the reading requires judgment; unverified means the claim could not be confirmed.",
         ].join("\n");
       }
       if (role === "explore") {
  -      return "You are a fresh-context exploration subagent. Read and run evidence-producing commands, but do not edit files or commit. Return concise findings with file:line grounding.";
  +      return [
  +        "You are a fresh-context exploration subagent. Read files and run evidence-producing commands only. This is report-only: do not edit files or commit.",
  +        "Handle exactly one information need. Do not bundle unrelated questions and do not recursively delegate.",
  +        `Selected breadth maximum: ${options.breadth}`,
  +        "Breadth is ordered targeted < bounded < broad. targeted locates one declaration, implementation, file, or exact fact; bounded investigates within a named symbol, package, component, or subsystem; broad searches across the project search universe, including relevant source, tests, documentation, decisions, and workflow artifacts.",
  +        "Treat the selected breadth as an adaptive maximum: start with the cheapest targeted lookup, widen only when evidence requires it, and never widen beyond the selected maximum. If the boundary is exhausted, report that explicitly.",
  +        "For broad searches, the project search universe is tracked files plus non-ignored untracked working-tree files under the current repository root. Include tracked generated and vendored files. Exclude ignored files, .git, nested repositories, and external dependencies unless the task explicitly brings one of those surfaces into scope.",
  +        `Selected report detail: ${options.detail}`,
  +        "Report detail is ordered paths < summary < analysis and is independent of breadth. paths returns only relevant file:line or file:start-end locations with minimal labels and no search narrative; summary returns grounded locations plus concise explanations of what each contains and why it matters; analysis directly answers the task with an evidence-grounded synthesis of relationships, call flow, usage patterns, assumptions, and uncertainty.",
  +        "Ground every material claim with file:line evidence.",
  +        "Distinguish not-found, inconclusive, and unverified outcomes. A not-found result begins exactly: Not found within <breadth> boundary: <what was searched>. A not-found result may suggest one concise next refinement. An inconclusive or unverified result is not an absence claim.",
  +        "Return only the relevant final report, never the search narrative or intermediate activity.",
  +        "Any sequential refinement is parent-driven through a new fresh-context call; retain no search session or state.",
  +      ].join("\n");
       }
  -    return `You are a fresh-context implementation subagent. Follow AGENTS.md and the task exactly. You may edit files. Commits are ${allowCommits ? "allowed when the task requests them" : "forbidden; do not change HEAD"}. Report changed files, verification, and blockers.`;
  +    return `You are a fresh-context implementation subagent. Follow AGENTS.md and the task exactly. You may edit files. Commits are ${options.allowCommits ? "allowed when the task requests them" : "forbidden; do not change HEAD"}. Report changed files, verification, and blockers.`;
   }
  ```

  Then replace the complete exploration schema and execute call exactly:

  ```diff
  -    parameters: Type.Object({ task: Type.String({ minLength: 1 }) }, { additionalProperties: false }),
  +    parameters: Type.Object({
  +      task: Type.String({ minLength: 1 }),
  +      breadth: StringEnum(["targeted", "bounded", "broad"] as const),
  +      detail: StringEnum(["paths", "summary", "analysis"] as const),
  +    }, { additionalProperties: false }),
       async execute(_id, params, signal, onUpdate, ctx) {
  -      return toolResult("explore", params.task, await run("explore", params.task, EXPLORE_TOOLS, rolePrompt("explore"), signal, onUpdate, ctx));
  +      return toolResult("explore", params.task, await run("explore", params.task, EXPLORE_TOOLS, rolePrompt("explore", { breadth: params.breadth, detail: params.detail }), signal, onUpdate, ctx));
       },
  ```

  Finally change only the implementation prompt call shape:

  ```diff
  -        const result = await run("implement", params.task, IMPLEMENT_TOOLS, rolePrompt("implement", params.allowCommits), signal, onUpdate, ctx);
  +        const result = await run("implement", params.task, IMPLEMENT_TOOLS, rolePrompt("implement", { allowCommits: params.allowCommits }), signal, onUpdate, ctx);
  ```

  Do not modify `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/runner.ts.tmpl`, `RunRequest`, process/cwd/model/thinking inheritance, allowlists, event retention, renderers, cancellation, output bounds, review loading, or implementation serialization.

Run:

```bash
/home/hypno/Projects/agentic-workflows/x upgrade
/home/hypno/Projects/agentic-workflows/x sync
/home/hypno/Projects/agentic-workflows/x pi-test run
go test github.com/hypnotox/agentic-workflows/internal/project github.com/hypnotox/agentic-workflows/internal/evals
```

Expected: the main adopted tree reports `exploring-skill-closure`, reaches generation 13, and adds exploring; Pi tests pass at 100% statement/branch/function/line coverage; Go render/eval tests pass. Sync may still produce generated documentation changes pending Task 1.6; do not gate or commit.

### Task 1.6: Update authored guidance with exact terminal assertions

- [ ] Apply this authored-doc batch. The exhaustive affected-site set is the thirteen authored paths below; the representative and edge wording are exact, and the other sites use the identical conjunctive trigger/schema vocabulary where applicable:
  - `/home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl`: add exploring to task skills and add `Use exploration only when both the repository location is unknown and inline search would pollute parent context; keep exact-known-file and genuinely trivial lookups inline.`
  - `/home/hypno/Projects/agentic-workflows/templates/docs/doc-standard.md.tmpl`: add `Keep action-first tool-agnostic prose by default. An awf-owned capability-selected integration may name its native tool, as Pi rendering names subagent_explore; this exception does not permit arbitrary runtime tool names.`
  - `/home/hypno/Projects/agentic-workflows/README.md`: say the standard has twelve core skills; document required `{task, breadth, detail}` and the three values of each enum.
  - `/home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl`: replace every `{task}`-only exploration call with required `{task, breadth, detail}` and state adaptive maximum, independent detail, and new-call sequential refinement.
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md`: add the core skill, generation-13 reuse of close-enabled-set, cross-runtime dispatch, and dynamic Pi prompt while retaining the runner boundary.
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md`: require a real-Pi smoke with one successful `targeted + paths` call and one `bounded` not-found followed by a corrected or `broad` fresh call; every call names task, breadth, and detail.
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md`: assign schema/prompt behavior to TypeScript and catalog/migration/cross-target behavior to Go; say tests prove instruction contracts, not arbitrary model compliance.
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/config/current-state.md`: record generation 13, `exploring-skill-closure`, and automatic closure of all three consumer configurations.
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`: record core exploring, six-target semantic rendering, and Pi's structured branch.
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md`: replace the live eleven-core statement with twelve and record generation 13 and the exact Pi contract lane.
  - `/home/hypno/Projects/agentic-workflows/examples/sundial/README.md`: add exploring to the full-surface example and describe cross-runtime semantic dispatch.
  - `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`: under Unreleased, announce the core skill, required Pi breadth/detail (including hand-authored-call breakage), and automatic schema-13 closure.
  - `/home/hypno/Projects/agentic-workflows/templates/skills/exploring/SKILL.md.tmpl`: retain the exact trigger and schema prose from Task 1.4 as the source of truth.

  Deterministic post-check over the actual retired variants and every required live surface, without a fixed corpus count or an aggregate positive match:

  ```bash
  live=(
    /home/hypno/Projects/agentic-workflows/README.md
    /home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl
    /home/hypno/Projects/agentic-workflows/templates/docs/doc-standard.md.tmpl
    /home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl
    /home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md
    /home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md
    /home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md
    /home/hypno/Projects/agentic-workflows/.awf/domains/parts/config/current-state.md
    /home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md
    /home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md
    /home/hypno/Projects/agentic-workflows/examples/sundial/README.md
    /home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md
  )
  if rg -n 'eleven core skills|eleven skills|eleven `core`-flagged workflow skills|accepts only (a )?`?task`?|subagent_explore \{task\}|one-field exploration schema|fixed one-line prompt' "${live[@]}"; then exit 1; fi

  while IFS='|' read -r path pattern; do
    if ! rg -q "$pattern" "$path"; then
      printf 'missing terminal vocabulary /%s/ in %s\n' "$pattern" "$path" >&2
      exit 1
    fi
  done <<'EOF'
  /home/hypno/Projects/agentic-workflows/README.md|twelve core skills
  /home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl|repository location is unknown.*inline search would pollute parent context
  /home/hypno/Projects/agentic-workflows/templates/docs/doc-standard.md.tmpl|capability-selected integration.*subagent_explore
  /home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl|task.*breadth.*detail
  /home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md|generation 13
  /home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md|targeted.*paths
  /home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md|instruction contracts
  /home/hypno/Projects/agentic-workflows/.awf/domains/parts/config/current-state.md|exploring-skill-closure
  /home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md|six-target semantic rendering
  /home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md|twelve
  /home/hypno/Projects/agentic-workflows/examples/sundial/README.md|cross-runtime semantic dispatch
  /home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md|schema.?13
  EOF
  ```

  Expected: the retired-variant assertion exits zero with no output, and the loop independently proves each named surface contains its designated terminal-state vocabulary. Historical ADR and plan files are intentionally outside this live-surface assertion.

### Task 1.7: Complete adopter upgrade, regenerate exhaustive fanout, flip lifecycle, gate, and make the only commit

- [ ] Do not hand-edit `/home/hypno/Projects/agentic-workflows/.awf/config.yaml` or `/home/hypno/Projects/agentic-workflows/examples/sundial/.awf/config.yaml`. Task 1.5 already rehearsed the shipped migration against the main adopted tree. Rehearse it against Sundial with the same source-built binary:

  ```bash
  bindir="$(mktemp -d)"
  go build -o "$bindir/awf" github.com/hypnotox/agentic-workflows/cmd/awf
  (cd /home/hypno/Projects/agentic-workflows/examples/sundial && "$bindir/awf" upgrade)
  rm -rf "$bindir"
  ```

  Expected: Sundial reports `exploring-skill-closure`, reaches generation 13, and adds exploring to `/home/hypno/Projects/agentic-workflows/examples/sundial/.awf/config.yaml`. Together with Task 1.5, this proves both adopted trees migrate.

- [ ] Before sync, record implementation findings in `/home/hypno/Projects/agentic-workflows/docs/plans/2026-07-19-structured-cross-runtime-exploration-workflow.md`: replace `Implementation deviations: pending.` with `Implementation deviations: none.` if execution matched this plan, otherwise list every changed path/test seam and why. Then change `status: Proposed` to `status: Implemented` in both that plan and `/home/hypno/Projects/agentic-workflows/docs/decisions/0132-structured-cross-runtime-exploration-workflow.md`. Do not change ADR-0038 or ADR-0125 status.

- [ ] Confirm `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go` no longer contains `invariant: pi-subagent-four-tool-contract` and contains each ADR-0132 marker exactly on its named executed test: `pi-structured-exploration-contract`, `cross-runtime-exploration-dispatch`, and `bounded-exploration-reporting`; confirm `exploration-skill-closure` is on the scaffold/catalog proof. This marker replacement must be in the same commit as the Implemented flip because retirement is inactive while ADR-0132 is Proposed.

- [ ] Sync all generated outputs and indexes:

  ```bash
  /home/hypno/Projects/agentic-workflows/x sync
  /home/hypno/Projects/agentic-workflows/x check
  ```

  Expected: `/home/hypno/Projects/agentic-workflows/docs/decisions/ACTIVE.md`, `/home/hypno/Projects/agentic-workflows/docs/domains/config.md`, `/home/hypno/Projects/agentic-workflows/docs/domains/rendering.md`, and `/home/hypno/Projects/agentic-workflows/docs/domains/tooling.md` show ADR-0132 as Implemented/current; both locks are generation 13; check is clean.

- [ ] Use this command's output as the exhaustive generated affected-site set:

  ```bash
  git -C /home/hypno/Projects/agentic-workflows diff --name-only -- /home/hypno/Projects/agentic-workflows/.claude /home/hypno/Projects/agentic-workflows/.pi /home/hypno/Projects/agentic-workflows/AGENTS.md /home/hypno/Projects/agentic-workflows/docs /home/hypno/Projects/agentic-workflows/examples/sundial
  ```

  Representative site: `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-exploring/SKILL.md` must contain the exact Pi dispatch sentence and all protocol terms. Edge site: `/home/hypno/Projects/agentic-workflows/examples/sundial/.gemini/skills/sundial-exploring/SKILL.md` has the identical semantic shape but generic native dispatch and no Pi token. Every other generated exploring copy has one of those two shapes according to target capability.

  Deterministic fanout and no-runner-change checks:

  ```bash
  find /home/hypno/Projects/agentic-workflows/.claude /home/hypno/Projects/agentic-workflows/.pi /home/hypno/Projects/agentic-workflows/examples/sundial/.claude /home/hypno/Projects/agentic-workflows/examples/sundial/.agents /home/hypno/Projects/agentic-workflows/examples/sundial/.github /home/hypno/Projects/agentic-workflows/examples/sundial/.cursor /home/hypno/Projects/agentic-workflows/examples/sundial/.gemini /home/hypno/Projects/agentic-workflows/examples/sundial/.pi -path '*-exploring/SKILL.md' -print | sort
  if rg -n 'subagent_explore' /home/hypno/Projects/agentic-workflows/.claude/skills /home/hypno/Projects/agentic-workflows/examples/sundial/.agents/skills /home/hypno/Projects/agentic-workflows/examples/sundial/.github/skills /home/hypno/Projects/agentic-workflows/examples/sundial/.cursor/skills /home/hypno/Projects/agentic-workflows/examples/sundial/.gemini/skills; then exit 1; fi
  test -z "$(git -C /home/hypno/Projects/agentic-workflows diff --name-only -- /home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/runner.ts.tmpl /home/hypno/Projects/agentic-workflows/.pi/extensions/awf-subagents/runner.ts /home/hypno/Projects/agentic-workflows/examples/sundial/.pi/extensions/awf-subagents/runner.ts)"
  ```

  Expected: the first command lists all enabled main/adopter copies, every non-Pi search is empty, and no runner source/generated copy changed.

- [ ] Run the only gate only after implementation, marker replacement, lifecycle flips, and sync are all present:

  ```bash
  /home/hypno/Projects/agentic-workflows/x gate
  git -C /home/hypno/Projects/agentic-workflows diff --check
  ```

  Expected: the gate passes with 100% Go statement coverage and 100% Pi-extension statement/branch/function/line coverage; `TestVersionCoversCurrentSchema` proves generation 13 maps to `0.17.0` while project `Version` remains `0.17.0`; no whitespace errors exist.

- [ ] Stage only files from `git -C /home/hypno/Projects/agentic-workflows status --short` that belong to ADR-0132, including all authored sources, generated copies, both configs/locks, ADR/plan lifecycle files, `/home/hypno/Projects/agentic-workflows/docs/decisions/ACTIVE.md`, and all three domain docs. Verify staged paths against the Task 1.7 affected-site output and commit once:

  ```commit
  feat(rendering): add structured exploration workflow
  ```

## Verification

From the clean tree after the one final commit:

```bash
/home/hypno/Projects/agentic-workflows/x check
/home/hypno/Projects/agentic-workflows/x gate
git -C /home/hypno/Projects/agentic-workflows status --short
rg -n 'status: Implemented' /home/hypno/Projects/agentic-workflows/docs/decisions/0132-structured-cross-runtime-exploration-workflow.md /home/hypno/Projects/agentic-workflows/docs/plans/2026-07-19-structured-cross-runtime-exploration-workflow.md
rg -n 'name: "subagent_explore"|targeted|bounded|broad|paths|summary|analysis' /home/hypno/Projects/agentic-workflows/.pi/extensions/awf-subagents/index.ts
if rg -n 'invariant: pi-subagent-four-tool-contract' /home/hypno/Projects/agentic-workflows/internal/project/target_test.go; then exit 1; fi
```

Expected: check and gate pass; status output is empty; both lifecycle files are Implemented; generated Pi contains the structured registration/enums; the retired marker is absent.

Manual release smoke follows `/home/hypno/Projects/agentic-workflows/docs/releasing.md` and remains non-gated: on real Pi 0.80.9 or newer, run one successful `targeted + paths` lookup with task/breadth/detail and one `bounded` not-found followed by a corrected or `broad` fresh call. Confirm intermediate activity appears only in tool details and only the final report enters model-visible content.

## Notes

- The phase is intentionally one commit because the dependency closure, generation-13 repair, consumer calls, Pi schema, invariant retirement, and lifecycle state must agree at every committed gate.
- Keep `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/runner.ts.tmpl` and every generated runner copy unchanged. A need to change one is an ADR resync trigger, not an incidental deviation.
- Generic non-Pi action wording is settled by ADR-0132. Exact target-native API syntax is out of scope.
- Do not use hard-coded corpus counts; derive sets or assert terminal absence/presence.
- Implementation deviations: pending.
