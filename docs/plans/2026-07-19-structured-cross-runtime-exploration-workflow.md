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

- [ ] Treat `/home/hypno/Projects/agentic-workflows/internal/project/resolve_test.go` as a repeated-site batch. The affected-site set is the output of:

  ```bash
  rg -n 'len\(plan|ops  int|11-skill|11 skills|planning side' /home/hypno/Projects/agentic-workflows/internal/project/resolve_test.go
  ```

  Representative exact diff: in `TestResolveEnableClosurePlan`, replace `if len(plan) != 14` with an expected-node set derived from `catalog.Closure(catalog.Standard, []catalog.Node{{Kind: "skill", Name: "brainstorming"}})` and compare every planned node to that set, while retaining the seed-first and `RequiredBy` assertions. Edge exact diff: for the partial-closure fixture, derive the wanted plan by filtering that same closure against the fixture's already-enabled node set and compare node sets rather than `len(plan3) != 9`. For `TestResolveDisableCascadeSizes`, replace `ops int` with `want []string` and compare sorted operation node names; obtain each list once from `ResolveDisable` after the catalog change and paste the names, not lengths. The deterministic post-check is that the command above returns no `ops int`, `len(plan`, `11-skill`, `11 skills`, or `planning side` match.

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

- [ ] In `/home/hypno/Projects/agentic-workflows/cmd/awf/run_test.go`, add `TestRunUpgradeAddsExploringAtSchemaThirteen`: use `testsupport.WriteAwfConfig` to write a real test-root config with `skills: [debugging]`, an empty agents list, and a lock stamped at schema 12; call `runUpgrade`; assert output contains `close-enabled-set: enabled skill "exploring" (required by "debugging")` and the applied migration name `exploring-skill-closure`; load the resulting lock and assert schema 13; load the config and assert `exploring` is enabled; finally assert `runCheck(root, io.Discard) == nil`. This is distinct from the direct migration test because it proves registry selection, lock stamping, terminal sync, and project-open/check behavior.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/version_test.go`, retain `TestVersionCoversCurrentSchema` unchanged. Its exact terminal assertion after production changes is `migrate.Current() == 13`, `minVersionBySchema[13] == "0.17.0"`, and `Version == "0.17.0"`; add those three comparisons to the test. There is no implementation-time version decision.

Run the focused red test command:

```bash
go test github.com/hypnotox/agentic-workflows/internal/catalog github.com/hypnotox/agentic-workflows/internal/migrate github.com/hypnotox/agentic-workflows/internal/project github.com/hypnotox/agentic-workflows/cmd/awf
```

Expected: non-zero only because `exploring`, generation 13, and its compatibility entry do not yet exist. Do not commit.

### Task 1.2: Add exact failing render, publication-safety, composed-eval, and order tests

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`, replace `TestPiSubagentFourToolContract` with `TestPiStructuredExplorationContract` and replace `// invariant: pi-subagent-four-tool-contract` with `// invariant: pi-structured-exploration-contract`. Preserve the existing exactly-four registration loop and add exact assertions that the generated exploration registration contains required `task`, `breadth: StringEnum(["targeted", "bounded", "broad"] as const)`, `detail: StringEnum(["paths", "summary", "analysis"] as const)`, and `additionalProperties: false`; assert grounding still has `{task}`, review still has `{kind, task}`, and implementation still has `{task, allowCommits}` by extracting each registration block from one tool name to the next and checking only that block.

- [ ] In the same file add `TestCrossRuntimeExplorationDispatch` with `// invariant: cross-runtime-exploration-dispatch`. Render a fixture enabling `exploring`, `brainstorming`, `debugging`, and `refactor-coupling-audit` for every value returned by `KnownTargets()`. For each target assert its adapter's `example-exploring/SKILL.md` exists. In Pi, assert that skill invokes `subagent_explore` with `task`, `breadth`, and `detail`. In every non-Pi target assert the skill says `target-native fresh-context exploration subagent`, names all three fields, and contains no `subagent_explore`. In every consumer assert the invocation condition contains both `location is unknown` and `inline search would pollute the parent context`, joined by `and`; assert it retains the known-file/trivial exception.

- [ ] Add `TestBoundedExplorationReporting` with `// invariant: bounded-exploration-reporting`. On Pi and one non-Pi edge target, assert the rendered exploring skill contains the ordered breadth and detail values, `adaptive maximum`, `tracked files plus non-ignored untracked`, all four exclusions, `Not found within <breadth> boundary:`, `inconclusive`, `unverified`, `one information need`, and `new fresh-context call`. Render the template once with an empty capability map and assert no `<no value>`, unresolved template action, or incoherent dangling Pi clause.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/project/spine_test.go`, add `TestExploringTemplate` beside the other skill goldens. Render with prefix `example`, first with `.targetSubagentTools = true` and then with it absent. Assert the Pi result names `subagent_explore` and exact `{task, breadth, detail}` dispatch; assert the fallback names generic target-native fresh-context delegation and no Pi token. Both results must contain the conjunctive trigger and exact-known-file/trivial exceptions. `renderGolden` supplies the publication-safety leak assertions.

- [ ] In `TestDebuggingTemplate` in that file, add this positional regression assertion after the existing phrase loop:

  ```go
    ordered := []string{
        "**Form one falsifiable hypothesis.**",
        "Invoke `example-exploring`",
        "Pick the cheapest oracle",
        "**Isolate with a failing test, written first.**",
    }
    position := -1
    for _, phrase := range ordered {
        next := strings.Index(out, phrase)
        if next <= position {
            t.Fatalf("debugging order violation at %q: positions must increase in %v", phrase, ordered)
        }
        position = next
    }
  ```

  Also add `"exploring": true` to that test's `.skills` map. This proves hypothesis formation, exploration/evidence validation, oracle handling, then test-first isolation.

- [ ] In `/home/hypno/Projects/agentic-workflows/internal/evals/chain_test.go`, add `TestExplorationConsumerToPiToolSeam`. Use `syncFullCatalogForTarget(t, cat, "pi")`; for the three catalog consumers assert `namesOnInvocationLine(body, evalPrefix+"-exploring")`; read the rendered Pi exploring skill and generated extension, then assert the skill names `subagent_explore` on an invocation line and the extension registers `name: "subagent_explore"`. Do not copy prompt-policy assertions into this composed seam.

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

  Representative exact diff in brainstorming: prepend step 1 with `When both the needed repository information's location is unknown and inline search would pollute the parent context, invoke {{ .prefix }}-exploring with one information need, breadth, and detail. Keep an exact-known-file read or genuinely trivial lookup inline.` Leave the later `subagent_grounding` step byte-for-byte unchanged.

  Debugging exact diff: in step 3, after its heading and before `Pick the cheapest oracle`, insert `Invoke \`{{ .prefix }}-exploring\` with one falsifiable evidence need, breadth, and detail only if both the evidence location is unknown and inline search would pollute the parent context; keep exact-known-file and genuinely trivial checks inline.` Do not move hypothesis formation, oracle selection/handling, or test isolation.

  Edge exact diff in refactor coupling audit: replace the whole size-threshold dispatch paragraph with `When both the coupling evidence location is unknown and inline search would pollute the parent context, invoke {{ .prefix }}-exploring once per information need with breadth and detail. Keep an exact-known-file or genuinely trivial category check inline. Preserve all six categories and the structured output contract.` Remove every direct `subagent_explore` reference from this consumer.

  Deterministic post-check:

  ```bash
  rg -n 'location is unknown.*and.*inline search would pollute|exact-known-file|genuinely trivial' /home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/debugging/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl
  if rg -n 'subagent_explore' /home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/debugging/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl; then exit 1; fi
  ```

  Expected: the first command finds the conjunctive trigger and exception in all three files; the second exits zero because its inner search finds nothing.

- [ ] In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`, make only these exact exploration changes:
  - add `type ExplorationBreadth = "targeted" | "bounded" | "broad";` and `type ExplorationDetail = "paths" | "summary" | "analysis";`;
  - change `rolePrompt` to `function rolePrompt(role: "grounding" | "explore" | "implement", options: { allowCommits?: boolean; breadth?: ExplorationBreadth; detail?: ExplorationDetail } = {}): string`; replace only its exploration branch with a joined fixed prompt containing the exact policy fragments asserted in Task 1.3 plus `Selected breadth maximum: ${options.breadth}` and `Selected report detail: ${options.detail}`; update the existing implementation branch to read `options.allowCommits` without changing its text;
  - replace exploration parameters with `Type.Object({ task: Type.String({ minLength: 1 }), breadth: StringEnum(["targeted", "bounded", "broad"] as const), detail: StringEnum(["paths", "summary", "analysis"] as const) }, { additionalProperties: false })`;
  - call `rolePrompt("explore", { breadth: params.breadth, detail: params.detail })` only from exploration execution and change the existing implementation call mechanically to `rolePrompt("implement", { allowCommits: params.allowCommits })`.

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

  Deterministic post-check over actual retired phrases and required live surfaces, without fixed corpus counts:

  ```bash
  if rg -n 'eleven core skills|accepts only (a )?`?task`?|subagent_explore \{task\}|one-field exploration schema|fixed one-line prompt' /home/hypno/Projects/agentic-workflows/README.md /home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl /home/hypno/Projects/agentic-workflows/templates/docs/doc-standard.md.tmpl /home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl /home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md /home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md /home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md /home/hypno/Projects/agentic-workflows/.awf/domains/parts/config/current-state.md /home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md /home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md /home/hypno/Projects/agentic-workflows/examples/sundial/README.md /home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md; then exit 1; fi
  rg -n 'twelve core skills|task.*breadth.*detail|generation 13|exploring-skill-closure' /home/hypno/Projects/agentic-workflows/README.md /home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl /home/hypno/Projects/agentic-workflows/.awf/domains/parts/config/current-state.md /home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md /home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md
  ```

  Expected: the first assertion exits zero with no old-phrase output; the second prints a match from every named required surface. Historical ADR and plan files are intentionally outside this live-surface assertion.

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
