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

Implementation uses two incremental ADR application transactions followed by one active-documentation and adopter transaction. The first transaction contracts the registry, removes TOML machinery, rewrites generic tests around Claude/Pi and synthetic descriptors, and applies the first four State changes. The second transaction replaces the accidental Claude bridge identity and applies the fifth operation. The sixth operation, `multi-target-render`, remains pending until terminal implementation review; that final governed transaction updates the claim and freezes the ADR and plan.

Authoring sources under `.awf/` and `templates/` remain authoritative. Root and Sundial generated artifacts are changed only through render/prune operations, and unmanaged lookalike files retain the existing ownership protection.

## Phase 1: Contract the runtime registry and remove TOML machinery

**Execution mode: inline.**

### Task 1.1: Rewrite target and encoding coverage around the surviving abstraction
Kind: batch
Latitude: exact
Paths: ["internal/project/target_test.go", "internal/project/agent_test.go", "internal/project/output_plan_test.go", "internal/project/coverage_test.go", "internal/project/notes_test.go", "internal/project/project_test.go", "internal/project/local_test.go", "internal/project/render_tree_test.go", "internal/project/subagent_model_selection_test.go", "internal/project/spine_test.go", "internal/contextq/adapter_outputs_test.go", "cmd/awf/list_add_test.go", "internal/config/edit_test.go", "internal/evals/chain_test.go"]
Representative: Replace the exact six-target roster and Claude/Cursor multi-target fixtures with exact `claude`, `pi` expectations while retaining iteration through `KnownTargets()` and asserting descriptor-derived skill, agent, bridge, capability, encoding, and extra-output behavior.
Edge: Remove Codex TOML, Cursor bridge, Gemini bridge, and Copilot path assertions, but retain Pi OpenAI Codex provider/model strings and add explicit unknown-target failures for each removed configuration name.
Post-check: `git grep -nE 'codexTarget|copilotTarget|cursorTarget|geminiTarget|TOMLAgentDialect|TestCodexTargetRendersTOMLAgents' -- 'internal/**/*_test.go' 'cmd/**/*_test.go'` returns no output, while `go test ./internal/project ./internal/contextq ./cmd/awf ./internal/config ./internal/evals` passes.

Start from a clean worktree at the settled plan-review commit. `git status --short` must produce no output, `./x check` must finish clean, and `go test ./internal/project ./internal/contextq` must pass before edits. This phase applies the first four ADR operations in declaration order and leaves the ADR `Implementing` with two Remaining operations.

Add or reshape focused tests before production deletion:

- `TestKnownTargets` proves the registry order and set are exactly `claude`, `pi`; place `// invariant: rendering/catalog-and-targets:built-in-runtime-targets (TestKnownTargets)` immediately above it.
- Retain `target-dialect-render` proof coverage on a test that renders structured agents for both survivors and parses their Markdown frontmatter. Do not weaken this to path existence alone.
- Retain `structured-agent-encoding` proof coverage on a Markdown test that separately supplies name, rendered description, and rendered instruction body, proving the encoder does not parse another rendered artifact.
- Rewrite `multi-target-render` proof coverage around Claude and Pi and assert neutral outputs such as `AGENTS.md` occur once. The claim text remains unchanged until the terminal transaction.
- Replace removed-adapter incidental variation with a synthetic descriptor test covering custom skill directory, agent directory, agent suffix, encoding, capabilities, and extra outputs. Bridge customization is completed in Phase 2.
- Rewrite pruning coverage so removing one surviving target deletes only lock-owned output files and empty ancestors, without using a removed target as live configuration.
- Delete the `cursor-no-bridge` proof marker with its claim-specific assertions. Preserve every unrelated proof marker carried by the touched tests and keep each marker immediately above a test whose name appears verbatim in that file.

### Task 1.2: Remove four descriptors and all Codex TOML production machinery
Kind: batch
Latitude: exact
Paths: ["internal/project/target.go", "internal/project/agent.go", "internal/project/render.go", "internal/project/validate.go", "internal/project/banner.go", "internal/project/banner_test.go", "internal/render/render.go", "internal/render/render_test.go", "go.mod", "go.sum"]
Representative: Delete `codexTarget`, `copilotTarget`, `cursorTarget`, and `geminiTarget` plus their registry entries; retain `Target`, target validation, ordered registry resolution, Claude and Pi descriptors, and every per-target customization field.
Edge: Remove only `TOMLAgentDialect`, `codexAgentProfile`, TOML encode/validate dispatch, TOML provenance/comment handling, and `github.com/BurntSushi/toml`; preserve `MarkdownAgentDialect`, `PlainAgentDialect`, structured agent metadata, Pi TypeScript outputs, and generic closed-set validation.
Post-check: `git grep -nE 'codexTarget|copilotTarget|cursorTarget|geminiTarget|TOMLAgentDialect|encodeTOMLAgent|validateTOMLAgent|TOMLComment|BurntSushi' -- internal go.mod go.sum` returns no output; `go mod tidy`, `gofmt -w` over changed Go files, and `go test ./internal/project ./internal/render ./internal/contextq ./cmd/awf` pass.

Keep `KnownTargets()` registry-derived and deterministic. Existing configs naming a removed target must reach the existing unknown-target error, whose available-target list contains only Claude and Pi. Do not add a migration, alias, warning, or silent rewrite. If TOML removal exposes a branch or helper with no remaining production caller, delete it rather than adding a dead-code or coverage exemption.

### Task 1.3: Apply the first four current-state operations atomically
Latitude: exact
Paths: ["docs/decisions/retain-only-claude-code-and-pi-runtime-targets.md", ".awf/topics/parts/rendering/catalog-and-targets/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", "docs/decisions/INDEX.md", "docs/topics/rendering/catalog-and-targets.md", "docs/topics/rendering/project-output-plan.md", "docs/domains/rendering.md", ".awf/awf.lock"]

Transition the ADR directly from `Proposed` to `Implementing` in this transaction. Set `status: Implementing`, append the canonical stamped Implementing event, then append one Applied event naming exactly these declaration-ordered operations:

```text
add rendering/catalog-and-targets:built-in-runtime-targets, update rendering/catalog-and-targets:structured-agent-encoding, update rendering/catalog-and-targets:target-dialect-render, remove rendering/project-output-plan:cursor-no-bridge
```

Land these exact claim endpoints, using the ADR's current pending identity in provenance until numbering mechanically changes it:

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

Remove the complete `cursor-no-bridge` claim block. Preserve surrounding topic prose and all unrelated claims byte-for-byte except deterministic render framing. Run `./x render` and stage every claim-derived topic/domain/index/lock change with the ADR, production behavior, dependency removal, tests, and proof markers. `./awf check staged` must recognize exactly the four-operation first batch and leave `bridge-render-identity` and `multi-target-render` Remaining.

### Task 1.4: Verify the contraction transaction

Run `git diff --check`, `go test ./...`, `./x render`, and `./x check`; all finish clean. Run `git grep -nE 'codexTarget|copilotTarget|cursorTarget|geminiTarget|TOMLAgentDialect|encodeTOMLAgent|validateTOMLAgent|TOMLComment|BurntSushi' -- ':!docs/decisions/**' ':!docs/plans/**' ':!docs/research/**' ':!changelog/**'`; it returns no active implementation hit. Do not treat `openai-codex/...` strings under retained Pi routing as residue.

### Phase close

Stage the complete first application transaction explicitly, require `./awf check staged` and `./x gate` to pass, and create one commit:

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

Run `gofmt -w` on changed Go files and `go test ./internal/project -run 'Bridge|TargetDescriptorCustomization|AllTargetPaths' -count=1`; all pass. `git grep -n 'renderTarget("claude"' -- internal/project` returns no output.

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

### Phase close

Stage the complete bridge application transaction explicitly, require `./awf check staged` and `./x gate` to pass, and create one commit:

```commit
refactor(rendering): neutralize bridge render identity
```

## Phase 3: Purge active adapter residue and regenerate Sundial

**Execution mode: inline.**

### Task 3.1: Remove adapter-only templates, integration metadata, and active prose
Kind: batch
Latitude: exact
Paths: ["templates/embed.go", "templates/gemini/GEMINI.md.tmpl", "templates/docs/working-with-awf.md.tmpl", ".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/development/dependencies.md", ".awf/docs/parts/roadmap/deferred.md", ".awf/docs/pitfalls.yaml", ".gitignore", "README.md"]
Representative: Delete the Gemini template embed/tree, remove Codex TOML dependency prose, replace six-runtime rosters and Cursor toggle examples with Claude/Pi or runtime-neutral examples, and remove obsolete roadmap/pitfall material owned solely by removed adapters.
Edge: Preserve shared `.github` infrastructure rules, immutable historical records, generic target enable/disable documentation, and every Pi `openai-codex` provider/model identifier.
Post-check: The active-source residue command in Task 3.3 returns no removed-adapter support claim, while historical paths and retained Pi routing remain unchanged.

Start with Phase 2 committed, `git status --short` empty, `./x check` clean, and `./x gate` passing. This phase updates live documentation and adopter topology but deliberately does not mutate or apply the final `multi-target-render` claim; that operation remains for the terminal review transaction.

Remove `.gitignore` comments and negations that exist only to commit `.agents`, `.codex`, `.cursor`, `.gemini`, or `GEMINI.md`. Preserve `.claude`, `.pi`, shared `.github` infrastructure, and unrelated repository exceptions. Update authoring sources rather than generated `docs/**` counterparts.

### Task 3.2: Reduce Sundial to Claude and Pi and prune every managed removed-target output
Kind: batch
Latitude: exact
Paths: ["examples/sundial/.awf/config.yaml", "pathspec:examples/sundial/.agents/**", "pathspec:examples/sundial/.codex/**", "pathspec:examples/sundial/.cursor/**", "pathspec:examples/sundial/.gemini/**", "pathspec:examples/sundial/.github/agents/**", "pathspec:examples/sundial/.github/skills/**", "examples/sundial/GEMINI.md", "pathspec:examples/sundial/.claude/**", "pathspec:examples/sundial/.pi/**", "examples/sundial/AGENTS.md", "examples/sundial/CLAUDE.md", "pathspec:examples/sundial/docs/**", "examples/sundial/.awf/awf.lock"]
Representative: Change Sundial's ordered `targets` list to `claude`, `pi`, then render with the new binary so lock-owned `.agents`, `.codex`, `.cursor`, `.gemini`, `.github/agents`, `.github/skills`, and `GEMINI.md` outputs are deleted and surviving outputs are regenerated.
Edge: Do not delete unrelated `.github` repository infrastructure or any unmanaged lookalike file; verify deletion is justified by the pre-render lock and existing pruning semantics.
Post-check: `git ls-files 'examples/sundial/.agents/**' 'examples/sundial/.codex/**' 'examples/sundial/.cursor/**' 'examples/sundial/.gemini/**' 'examples/sundial/.github/agents/**' 'examples/sundial/.github/skills/**' 'examples/sundial/GEMINI.md'` returns no output, `git grep -nE '(^|[ /])(codex|copilot|cursor|gemini)([ /]|$)' -- examples/sundial/.awf/config.yaml examples/sundial/.awf/awf.lock` returns no output, and `(cd examples/sundial && go test ./...)` passes.

Use the repository's normal root render command so the nested adopter is synchronized by the supported workflow; do not hand-edit its lock or generated files. Inspect every staged deletion and retain all Claude/Pi artifacts, `AGENTS.md`, and `CLAUDE.md`.

### Task 3.3: Regenerate active documentation and prove residue boundaries
Kind: batch
Latitude: exact
Paths: ["pathspec:docs/**", "AGENTS.md", "CLAUDE.md", "pathspec:.claude/**", "pathspec:.pi/**", ".awf/awf.lock", "pathspec:examples/sundial/docs/**", "examples/sundial/AGENTS.md", "examples/sundial/CLAUDE.md"]
Representative: Run `./x render` from the repository root and stage every deterministic generated change caused by Phase 3 authoring sources and Sundial configuration.
Edge: Do not edit or delete `docs/decisions/**` other than generated `INDEX.md`, dated `docs/plans/**`, `docs/research/**`, or `changelog/**`; do not treat `openai-codex` strings in retained Pi outputs as adapter residue.
Post-check: `./x check`, `git diff --check`, the scoped active-source grep below, and `./x gate` all finish clean.

Run this scoped residue check and inspect every result rather than performing a lexical repository-wide purge:

```bash
git grep -nE 'codexTarget|copilotTarget|cursorTarget|geminiTarget|TOMLAgentDialect|encodeTOMLAgent|validateTOMLAgent|TOMLComment|BurntSushi|templates/gemini|GEMINI\.md|\.cursor/|\.codex/agents|\.agents/skills|\.gemini/skills|\.github/agents' -- ':!docs/decisions/**' ':!docs/plans/**' ':!docs/research/**' ':!changelog/**' ':!templates/pi/**' ':!.pi/**' ':!examples/sundial/.pi/**'
```

The command returns no removed-adapter implementation, template, active-documentation, config, generated-output, or lock residue. Separately require `git grep -n 'openai-codex/' -- templates/pi .pi examples/sundial/.pi` to return retained routing matches, proving the exception was preserved intentionally. Confirm root `.awf/config.yaml` still lists only Claude and Pi.

### Phase close

Stage the complete active-documentation and adopter purge explicitly, require `./awf check staged` and `./x gate` to pass, and create one commit:

```commit
docs(rendering): purge removed runtime adapter surfaces
```

## Definition of done

- `KnownTargets()` returns exactly `claude`, `pi`; all four removed names fail through normal unknown-target validation without migration, alias, warning, or rewrite behavior.
- Claude and Pi retain descriptor-owned customization of paths, suffixes, encodings, bridges, capabilities, wording, and additional outputs, with synthetic coverage preventing the surviving implementation from collapsing those seams.
- Codex TOML production code, validation, provenance style, tests, and module dependency are absent; structured Markdown agents and Pi plain TypeScript outputs remain covered.
- Generic bridge rendering and input observation use the neutral `target-bridge` identity, while Claude still emits `CLAUDE.md` and Pi emits no bridge.
- Root and Sundial rendering/checks are clean; Sundial enables only Claude and Pi and contains no managed output or lock entry for a removed adapter.
- Active source, documentation, roadmap, ignore metadata, and generated outputs contain no removed-adapter support residue, while immutable historical records and Pi OpenAI Codex model identifiers remain unchanged.
- Every invariant proof marker names a present test that proves its current claim; `cursor-no-bridge` and its markers are absent.
- `go test ./...`, `./x render`, `./x check`, `./awf check staged`, and `./x gate` pass, including 100% statement coverage and dead-code checks.
- After terminal implementation review settles, the final transaction applies `update rendering/project-output-plan:multi-target-render`, freezes the ADR and this plan as `Implemented`, and leaves the repository clean.

## Notes

- The terminal implementation-review flow owns the sixth operation and lifecycle freeze. After Phase 3 review has zero unresolved findings, update `multi-target-render` to the exact endpoint below, append the final Applied event followed by the stamped Implemented event, change ADR status to `Implemented`, change this plan's status to `Implemented`, run `./x render`, and commit the complete claim/proof/status/generated transaction only after `./awf check staged` and `./x gate` pass:

```markdown
### `invariant: multi-target-render`

With multiple targets enabled, each adapter artifact renders once per target at that descriptor's declared paths - including Claude Code and Pi skills and agents - while neutral artifacts such as `AGENTS.md` render exactly once regardless of target count. Descriptor-specific wording, bridges, capabilities, encodings, and additional outputs remain independently customizable.
Origin: ADR-0037
Revised-by: ADR-retain-only-claude-code-and-pi-runtime-targets
Backing: test
```

- Until that terminal transaction, the old `multi-target-render` claim remains active and visibly pending correction. Do not edit its prose early or append the final Applied/Implemented events before review establishes completion.
- Historical mentions of Codex, Copilot, Cursor, and Gemini are evidence of past support, not residue. The residue commands intentionally exclude immutable record paths and distinguish the removed Codex runtime adapter from retained OpenAI Codex model routing.
- Record implementation deviations, reviewer findings, and any newly discovered active residue here while the plan remains Proposed. A load-bearing scope change requires returning to the ADR rather than silently widening this plan.
