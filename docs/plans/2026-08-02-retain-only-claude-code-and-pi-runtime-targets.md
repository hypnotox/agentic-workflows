---
format: plan-v1
date: 2026-08-02
adrs:
  - retain-only-claude-code-and-pi-runtime-targets
status: Proposed
---
# Plan: Retain Only Claude Code and Pi Runtime Targets

## Goal

Implement [ADR-retain-only-claude-code-and-pi-runtime-targets](../decisions/retain-only-claude-code-and-pi-runtime-targets.md): retain Claude Code and Pi as the complete built-in runtime target set, purge the four unused adapters, and neutralize generic bridge rendering without reducing descriptor-owned customization.

This plan does not rewrite historical ADRs, completed plans, research, or changelog entries; remove Pi's OpenAI Codex provider/model identifiers; add compatibility or migration behavior; or redesign the target registry and output planner.

## Architecture summary

`internal/project` retains one generic `Target` descriptor and registry-driven render/output-plan flow. Claude Code and Pi remain concrete descriptors with independent paths, bridge behavior, capabilities, encodings, wording, and additional outputs. Codex-only TOML representation and validation disappear at their current boundary, while structured Markdown agent rendering and Pi's plain TypeScript output encoding remain.

Phase 1 is the complete runtime-contraction transaction: production, tests, proof markers, dependencies, active authored docs, templates, integration metadata, changelog, root generated artifacts, and Sundial configuration/output pruning travel together. It applies the first four ADR operations and leaves the ADR `Implementing`. Phase 2 is the independently truthful neutral bridge-identity refactor and applies the fifth operation. The sixth operation, `multi-target-render`, remains pending until terminal implementation review; that final governed transaction updates the claim and freezes the ADR and plan.

Authoring sources under `.awf/` and `templates/` remain authoritative. Root and Sundial generated artifacts change only through render/prune operations, and unmanaged lookalike files retain the existing ownership protection.

## Phase 1: Contract the runtime set and purge removed adapters

**Execution mode: inline.**

### Task 1.1: Rewrite coverage and contract production in one ordered batch
Kind: batch
Latitude: exact
Paths: ["internal/project/target.go", "internal/project/agent.go", "internal/project/render.go", "internal/project/validate.go", "internal/project/banner.go", "internal/project/banner_test.go", "internal/render/render.go", "internal/render/render_test.go", "internal/project/target_test.go", "internal/project/agent_test.go", "internal/project/output_plan_test.go", "internal/project/coverage_test.go", "internal/project/notes_test.go", "internal/project/project_test.go", "internal/project/local_test.go", "internal/project/render_tree_test.go", "internal/project/subagent_model_selection_test.go", "internal/project/spine_test.go", "internal/contextq/adapter_outputs_test.go", "cmd/awf/list_add_test.go", "internal/config/edit_test.go", "internal/evals/chain_test.go", "go.mod", "go.sum"]
Representative: First rewrite the exact six-target and Claude/Cursor fixtures around `claude`, `pi`, and synthetic descriptors; then remove the four production descriptors and Codex TOML machinery so the rewritten tests pass without collapsing the generic target seam.
Edge: Remove removed-adapter assertions, TOML provenance/comment handling, and awf's direct TOML dependency, but retain any transitive TOML module required independently by development tooling, Pi OpenAI Codex provider/model strings, structured Markdown agents, `PlainAgentDialect`, Pi TypeScript outputs, generic closed-set validation, and descriptor-owned customization.
Post-check: The literal `gofmt -w` and `go mod tidy` commands below succeed; the scoped removed-symbol grep returns no output; and `go test ./internal/project ./internal/render ./internal/contextq ./cmd/awf ./internal/config ./internal/evals` passes.

Start from the settled plan-review commit. Require `git status --short` to produce no output, `./x check` to finish clean, and `go test ./internal/project ./internal/contextq` to pass before edits.

Add or reshape focused tests before deleting production behavior:

- `TestKnownTargets` proves the registry order and set are exactly `claude`, `pi`; place `// invariant: rendering/catalog-and-targets:built-in-runtime-targets (TestKnownTargets)` immediately above it.
- Retain `target-dialect-render` proof coverage on a test that renders structured agents for both survivors and parses their Markdown frontmatter. Do not weaken this to path existence alone.
- Retain `structured-agent-encoding` proof coverage on a Markdown test that separately supplies name, rendered description, and rendered instruction body, proving the encoder does not parse another rendered artifact.
- Rewrite `multi-target-render` proof coverage around Claude and Pi and assert neutral outputs such as `AGENTS.md` occur once. The claim text remains unchanged until the terminal transaction.
- Replace removed-adapter incidental variation with a synthetic descriptor test covering custom skill directory, agent directory, agent suffix, encoding, capabilities, and extra outputs. Bridge customization is completed in Phase 2.
- Rewrite pruning coverage so removing one surviving target deletes only lock-owned output files and empty ancestors, without using a removed target as live configuration.
- Add explicit project-open and CLI failures for `codex`, `copilot`, `cursor`, and `gemini`; each uses the existing unknown-target identity and lists only the surviving known targets.
- Delete the `cursor-no-bridge` proof marker with its claim-specific assertions. Preserve every unrelated proof marker carried by touched tests and keep each marker immediately above a test whose name occurs verbatim in that file.

Then delete `codexTarget`, `copilotTarget`, `cursorTarget`, and `geminiTarget` plus their registry entries. Keep `KnownTargets()` registry-derived and deterministic. Remove only `TOMLAgentDialect`, `codexAgentProfile`, TOML encode/validate dispatch, TOML provenance/comment handling, and awf's direct `github.com/BurntSushi/toml` requirement. `go mod tidy` may retain it indirectly when independently required by development tooling. Do not add a migration, alias, warning, silent configuration rewrite, dead-code exemption, or coverage exemption.

Run exactly:

```bash
gofmt -w internal/project/target.go internal/project/agent.go internal/project/render.go internal/project/validate.go internal/project/banner.go internal/project/banner_test.go internal/render/render.go internal/render/render_test.go internal/project/target_test.go internal/project/agent_test.go internal/project/output_plan_test.go internal/project/coverage_test.go internal/project/notes_test.go internal/project/project_test.go internal/project/local_test.go internal/project/render_tree_test.go internal/project/subagent_model_selection_test.go internal/project/spine_test.go internal/contextq/adapter_outputs_test.go cmd/awf/list_add_test.go internal/config/edit_test.go internal/evals/chain_test.go
go mod tidy
git grep -nE 'codexTarget|copilotTarget|cursorTarget|geminiTarget|TOMLAgentDialect|encodeTOMLAgent|validateTOMLAgent|TOMLComment|TestCodexTargetRendersTOMLAgents' -- internal cmd
```

The final grep returns no output. `go mod why -m github.com/BurntSushi/toml` may report only a development-tooling chain and must not report `internal/project`.

### Task 1.2: Purge adapter-only templates, active prose, and integration metadata
Kind: batch
Latitude: exact
Paths: ["templates/embed.go", "templates/gemini/GEMINI.md.tmpl", "templates/docs/working-with-awf.md.tmpl", ".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/development/dependencies.md", ".awf/docs/parts/roadmap/deferred.md", ".awf/docs/pitfalls.yaml", ".gitignore", "README.md", "changelog/CHANGELOG.md"]
Representative: Delete the Gemini template embed/tree, remove Codex TOML dependency prose, replace six-runtime rosters and Cursor toggle examples with Claude/Pi or runtime-neutral examples, and append an Unreleased breaking-change entry for immediate removal of four target names.
Edge: Preserve shared `.github` infrastructure rules, immutable historical changelog entries, generic target enable/disable guidance, historical records elsewhere, and every Pi `openai-codex` provider/model identifier.
Post-check: `git diff --check` passes; the scoped authoring-source grep below returns no removed-adapter support assertion; and the new Unreleased changelog entry names the four removed configuration values and the lack of compatibility or migration.

Remove `.gitignore` comments and negations that exist only to commit `.agents`, `.codex`, `.cursor`, `.gemini`, or `GEMINI.md`. Preserve `.claude`, `.pi`, shared `.github` infrastructure, and unrelated repository exceptions. Update authoring sources rather than generated `docs/**` counterparts. Append one `[Unreleased]` Breaking changes bullet; do not rewrite any historical changelog entry.

Run:

```bash
git grep -nE 'BurntSushi|templates/gemini|GEMINI\.md|\.cursor/|\.codex/agents|\.agents/skills|\.gemini/skills|\.github/agents|six (built-in )?runtime|six target' -- templates/embed.go templates/docs/working-with-awf.md.tmpl .awf/docs .awf/topics .gitignore README.md
```

At this point the only allowed hit is the still-pending old `multi-target-render` example under `.awf/topics/parts/rendering/project-output-plan/current-state.md`; every other result is a defect. The changelog is deliberately excluded because its new breaking-change entry and old history truthfully name removed targets.

### Task 1.3: Contract Sundial authoring before any render
Latitude: exact
Paths: ["examples/sundial/.awf/config.yaml"]

Replace Sundial's ordered target list with exactly:

```yaml
targets:
  - claude
  - pi
```

Do not edit its generated outputs or lock by hand. Require a YAML-aware config read through the new binary to succeed and `git diff -- examples/sundial/.awf/config.yaml` to contain no unrelated configuration change. This task must complete before Task 1.5 invokes root rendering, because nested-adopter rendering rejects removed target names immediately.

### Task 1.4: Apply the first four current-state operations atomically
Latitude: exact
Paths: ["docs/decisions/retain-only-claude-code-and-pi-runtime-targets.md", ".awf/topics/parts/rendering/catalog-and-targets/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md"]

Transition the ADR directly from `Proposed` to `Implementing` in this transaction. Set `status: Implementing`, append the canonical stamped Implementing event, then append one Applied event naming exactly these declaration-ordered operations:

```text
add rendering/catalog-and-targets:built-in-runtime-targets, update rendering/catalog-and-targets:structured-agent-encoding, update rendering/catalog-and-targets:target-dialect-render, remove rendering/project-output-plan:cursor-no-bridge
```

Land these exact claim endpoints, using the ADR's pending identity until numbering mechanically changes it:

```markdown
### `invariant: built-in-runtime-targets`

The built-in runtime target registry contains exactly `claude` and `pi` in deterministic `KnownTargets` order. Configured names outside that set fail through unknown-target validation, and descriptor-driven rendering and enablement remain generic rather than branching on the two names.
Origin: ADR-retain-only-claude-code-and-pi-runtime-targets
Backing: test
```

```markdown
### `invariant: structured-agent-encoding`

Agent rendering consumes structured metadata - a literal name, a separately rendered description, and a rendered instruction body - before a target encoder emits its artifact. The Markdown encoder never parses another rendered agent artifact, and arbitrary target-owned outputs retain their separately declared encoding.
Origin: ADR-0122
Revised-by: ADR-retain-only-claude-code-and-pi-runtime-targets
Backing: test
```

```markdown
### `invariant: target-dialect-render`

Each enabled target renders every skill and agent exactly once at that descriptor's declared path and encoding, and the emitted artifact parses under the runtime's native format. The built-in Claude Code and Pi targets emit Markdown agents while retaining independent descriptor-owned paths, suffixes, capabilities, bridges, wording, and additional outputs.
Origin: ADR-0122
Revised-by: ADR-retain-only-claude-code-and-pi-runtime-targets
Backing: test
```

Remove the complete `cursor-no-bridge` claim block. Preserve surrounding topic prose and unrelated claims byte-for-byte except deterministic render framing.

### Task 1.5: Render root and Sundial and prune every managed removed-target output
Kind: batch
Latitude: exact
Paths: ["pathspec:docs/**", "AGENTS.md", "CLAUDE.md", "pathspec:.claude/**", "pathspec:.pi/**", ".awf/awf.lock", "pathspec:examples/sundial/.agents/**", "pathspec:examples/sundial/.codex/**", "pathspec:examples/sundial/.cursor/**", "pathspec:examples/sundial/.gemini/**", "pathspec:examples/sundial/.github/agents/**", "pathspec:examples/sundial/.github/skills/**", "examples/sundial/GEMINI.md", "pathspec:examples/sundial/.claude/**", "pathspec:examples/sundial/.pi/**", "examples/sundial/AGENTS.md", "examples/sundial/CLAUDE.md", "pathspec:examples/sundial/docs/**", "examples/sundial/.awf/awf.lock"]
Representative: Run `./x render` once all Phase 1 authoring changes are present; allow lock-owned removed-target trees to prune and regenerate every Claude/Pi, documentation, index, domain/topic, and lock output selected by the render.
Edge: Do not delete unrelated `.github` infrastructure or unmanaged lookalike files; do not hand-edit generated files; and do not modify historical ADRs, dated plans, research, or historical changelog entries.
Post-check: Root and Sundial checks pass; no removed-target generated path or lock entry remains; generated docs reflect all Phase 1 authored changes except the explicitly Remaining `multi-target-render` claim.

Run `./x render && ./x check`, then inspect every generated modification and deletion. Require:

```bash
git ls-files 'examples/sundial/.agents/**' 'examples/sundial/.codex/**' 'examples/sundial/.cursor/**' 'examples/sundial/.gemini/**' 'examples/sundial/.github/agents/**' 'examples/sundial/.github/skills/**' 'examples/sundial/GEMINI.md'
git grep -nE '(^|[ /])(codex|copilot|cursor|gemini)([ /]|$)' -- examples/sundial/.awf/config.yaml examples/sundial/.awf/awf.lock
```

Both commands return no output. `(cd examples/sundial && go test ./...)` passes. Retain and stage all Claude/Pi artifacts, `AGENTS.md`, and `CLAUDE.md` selected by render.

### Task 1.6: Verify active residue and the first lifecycle batch

Run the following scoped residue check:

```bash
git grep -nE 'codexTarget|copilotTarget|cursorTarget|geminiTarget|TOMLAgentDialect|encodeTOMLAgent|validateTOMLAgent|TOMLComment|templates/gemini|GEMINI\.md|\.cursor/|\.codex/agents|\.agents/skills|\.gemini/skills|\.github/agents' -- ':!docs/decisions/**' ':!docs/plans/**' ':!docs/research/**' ':!changelog/**' ':!.awf/topics/parts/rendering/project-output-plan/current-state.md' ':!docs/topics/rendering/project-output-plan.md' ':!docs/domains/rendering.md' ':!templates/pi/**' ':!.pi/**' ':!examples/sundial/.pi/**'
```

It returns no output. Inspect the three excluded current-state projections and require any removed-target path reference to belong only to the still-Remaining `multi-target-render` claim, never the removed `cursor-no-bridge` claim. Separately require `git grep -n 'openai-codex/' -- templates/pi .pi examples/sundial/.pi` to return retained routing matches.

Run `git diff --check`, `go test ./...`, `./x check`, and `./awf check staged` after explicit staging. The staged check must recognize exactly the four-operation first batch and leave `bridge-render-identity` and `multi-target-render` Remaining.

### Phase close

Stage the complete production, test, dependency, claim, active-doc, template, changelog, integration-metadata, root-render, and Sundial transaction explicitly. Require `./awf check staged` and `./x gate` to pass, then create one commit:

```commit
refactor(rendering): contract runtime targets
```

## Phase 2: Neutralize bridge rendering identity

**Execution mode: inline.**

### Task 2.1: Specify descriptor-owned neutral bridge behavior
Latitude: exact
Paths: ["internal/project/output_declarations_test.go", "internal/project/output_plan_test.go", "internal/project/target_test.go", "internal/project/render_tree_test.go"]

Start with Phase 1 committed, `git status --short` empty, `./x check` clean, and `go test ./internal/project` passing. This phase applies only the fifth declared ADR operation and leaves `multi-target-render` Remaining.

Add `TestBridgeRenderIdentity` with `// invariant: rendering/project-output-plan:bridge-render-identity (TestBridgeRenderIdentity)` immediately above it. Use Claude plus a synthetic descriptor with a distinct bridge path/template to prove:

- bridge recipe construction and rendering use the neutral internal kind `target-bridge`, never the target name `claude`;
- the descriptor remains the sole source of `BridgeFile` and `BridgeTemplate`;
- input observation does not derive or probe a fictitious `target-bridge` sidecar and does not inherit a Claude-specific sidecar/template association;
- custom bridge path/template, agent suffix, and the other descriptor customization fields remain effective;
- empty template variables retain missing-key-zero behavior and produce no `<no value>` token; and
- Pi's empty bridge declaration emits no bridge while its target-owned outputs remain unchanged.

The test must inspect the bridge plan node's declared recipe/template and observed inputs, not only compare rendered bytes.

### Task 2.2: Replace the hard-coded Claude bridge identity at both boundaries
Latitude: exact
Paths: ["internal/project/render.go", "internal/project/output_plan.go", "internal/project/singleton.go"]

Introduce one package-local neutral identity constant only if both bridge render construction and input observation consume it; otherwise use the same neutral literal at the single shared ownership point. Replace the generic bridge call's `claude` kind and update the observation exclusion so it does not search for a neutral-kind sidecar. Do not alter Claude's descriptor-owned `CLAUDE.md` path/template or Pi's empty bridge declaration, and do not add target-name branching.

Run exactly:

```bash
gofmt -w internal/project/output_declarations_test.go internal/project/output_plan_test.go internal/project/target_test.go internal/project/render_tree_test.go internal/project/render.go internal/project/output_plan.go internal/project/singleton.go
go test ./internal/project -run 'Bridge|TargetDescriptorCustomization|AllTargetPaths' -count=1
git grep -n 'renderTarget("claude"' -- internal/project
```

The tests pass and the final grep returns no output.

### Task 2.3: Apply the bridge identity claim
Latitude: exact
Paths: ["docs/decisions/retain-only-claude-code-and-pi-runtime-targets.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", "docs/topics/rendering/project-output-plan.md", "docs/domains/rendering.md", ".awf/awf.lock"]

Keep the ADR `Implementing` and append one middle Applied event naming exactly `add rendering/project-output-plan:bridge-render-identity`. Add this exact claim:

```markdown
### `invariant: bridge-render-identity`

Every target-declared bridge renders through the neutral `target-bridge` identity while its descriptor remains the sole owner of bridge path and template. Input observation does not derive a target-specific sidecar or template from that neutral identity, so a future bridge target cannot inherit Claude-specific inputs accidentally.
Origin: ADR-retain-only-claude-code-and-pi-runtime-targets
Backing: test
```

Run `./x render`; stage the ADR event, claim, proof, production change, tests, generated topic/domain output, and lock together. `./awf check staged` must recognize this one middle batch and report only `multi-target-render` Remaining.

### Task 2.4: Verify the bridge transaction

Run `git diff --check`, `go test ./...`, `./x render`, and `./x check`; all finish clean. Inspect generated output to confirm Claude still emits `CLAUDE.md`, Pi emits no bridge, and no target output changed merely because the internal bridge identity became neutral.

### Phase close

Stage the complete bridge application transaction explicitly, require `./awf check staged` and `./x gate` to pass, and create one commit:

```commit
refactor(rendering): neutralize bridge render identity
```

## Definition of done

- `KnownTargets()` returns exactly `claude`, `pi`; all four removed names fail through normal unknown-target validation without migration, alias, warning, or rewrite behavior.
- Claude and Pi retain descriptor-owned customization of paths, suffixes, encodings, bridges, capabilities, wording, and additional outputs, with synthetic coverage preventing the surviving implementation from collapsing those seams.
- Codex TOML production code, validation, provenance style, tests, and direct module dependency are absent; any remaining indirect TOML module is justified only by independent development tooling, while structured Markdown agents and Pi plain TypeScript outputs remain covered.
- Generic bridge rendering and input observation use the neutral `target-bridge` identity, while Claude still emits `CLAUDE.md` and Pi emits no bridge.
- Root and Sundial rendering/checks are clean; Sundial enables only Claude and Pi and contains no managed output or lock entry for a removed adapter.
- Active source, documentation, roadmap, ignore metadata, and generated outputs contain no removed-adapter support residue except the explicitly pending old `multi-target-render` example, while immutable historical records and Pi OpenAI Codex model identifiers remain unchanged.
- `[Unreleased]` documents the four removed target values as an immediate breaking change without rewriting historical changelog entries.
- Every invariant proof marker names a present test that proves its current claim; `cursor-no-bridge` and its markers are absent.
- `go test ./...`, `./x render`, `./x check`, `./awf check staged`, and `./x gate` pass, including 100% statement coverage and dead-code checks.
- After terminal implementation review settles, the final transaction applies `update rendering/project-output-plan:multi-target-render`, freezes the ADR and this plan as `Implemented`, and leaves the repository clean.

## Notes

- The terminal implementation-review flow owns the sixth operation and lifecycle freeze. After Phase 2 review has zero unresolved findings, update `multi-target-render` to the exact endpoint below, append the final Applied event followed by the stamped Implemented event, change ADR status to `Implemented`, change this plan's status to `Implemented`, run `./x render`, and commit the complete claim/proof/status/generated transaction only after `./awf check staged` and `./x gate` pass:

```markdown
### `invariant: multi-target-render`

With multiple targets enabled, each adapter artifact renders once per target at that descriptor's declared paths - including Claude Code and Pi skills and agents - while neutral artifacts such as `AGENTS.md` render exactly once regardless of target count. Descriptor-specific wording, bridges, capabilities, encodings, and additional outputs remain independently customizable.
Origin: ADR-0037
Revised-by: ADR-retain-only-claude-code-and-pi-runtime-targets
Backing: test
```

- Until that terminal transaction, the old `multi-target-render` claim remains active and visibly pending correction. Do not edit its prose early or append the final Applied/Implemented events before review establishes completion.
- Historical mentions of Codex, Copilot, Cursor, and Gemini are evidence of past support, not residue. Residue commands intentionally exclude immutable record paths and distinguish the removed Codex runtime adapter from retained OpenAI Codex model routing.
- Implementation discovery: `go mod tidy` retains `github.com/BurntSushi/toml` only as an indirect dependency of the pinned golangci-lint toolchain (`go mod why -m github.com/BurntSushi/toml` reaches revive), not through awf runtime code. The purge removes awf's direct dependency and all runtime use; deleting an independently required transitive tool dependency is outside the adapter boundary.
- Record implementation deviations, reviewer findings, and any newly discovered active residue here while the plan remains Proposed. A load-bearing scope change requires returning to the ADR rather than silently widening this plan.
