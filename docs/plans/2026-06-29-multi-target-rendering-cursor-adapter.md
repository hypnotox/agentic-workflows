# Plan: Multi-Target Rendering and the Cursor Adapter

**ADRs:** [ADR-0037](../decisions/0037-multi-target-rendering-and-cursor-adapter.md) (multi-target rendering + Cursor), [ADR-0038](../decisions/0038-tool-agnostic-skill-agent-prose.md) (tool-agnostic prose). Both `Proposed`, review-settled. This plan links both; they flip to `Implemented` together in the final phase.

## Goal

Generalize ADR-0016's single `Target` rendering seam into `Project.Targets []Target` driven by a `targets:` config enable array, add a Cursor adapter (`.cursor/skills/`, `.cursor/agents/`, no bridge), and neutralize Claude-specific tool vocabulary in the shipped skill prose so the same rendered body reads correctly under both runtimes. awf dogfoods `targets: [claude, cursor]`.

## Architecture summary

Design rationale lives in the two ADRs — do not restate here. Execution shape:

- **Neutral vs adapter render passes.** `RenderAll` renders neutral artifacts (docs, AGENTS.md, adr-readme/template, plans-readme) once, and adapter artifacts (skills, agents, bridge) once per enabled target. Generated `ACTIVE.md` + domain docs (added in `Sync`/`PlannedOutputs`) stay neutral.
- **Same body, two paths.** Claude and Cursor share the SKILL.md/AGENTS.md standards, so the identical rendered body+frontmatter is written to each target's path. The lock is path-keyed (`project.go:138`) and `artifactConfigHash` (renamed from `targetConfigHash`) does not fold the path, so two entries coexist with identical hashes under distinct keys.
- **Pruning + the artifact-sense rename** are prerequisites isolated into their own phases so later diffs stay clean.

## Tech stack

- Go 1.26. Packages touched: `internal/config`, `internal/project`, `cmd/awf`, `templates/` (skill `.tmpl` files + `doc-standard.md.tmpl`), plus `.awf/` dogfood config and repo `.gitignore`.
- Gate: `./x gate` (100% statement coverage — ADR-0012). `./x check` (drift + invariants). Run both before every commit.

## File structure

**Created:**
- `docs/plans/2026-06-29-multi-target-rendering-cursor-adapter.md` (this file)
- `.cursor/skills/awf-*/SKILL.md`, `.cursor/agents/*.md` (dogfood render output, Phase 8)

**Modified:**
- `internal/config/config.go` — `Targets` field, default, validation
- `internal/project/target.go` — `cursorTarget`, registry, `resolveTargets`
- `internal/project/project.go` — `Targets []Target` field, `Open`, Sync prune ancestor-walk
- `internal/project/render.go` — RenderAll neutral/adapter split; artifact-sense rename
- `internal/project/confighash.go` — `targetConfigHash`→`artifactConfigHash`, `consumedParts` param
- `internal/project/check.go` — `localOutPath`→multi-target; orphan-loop vocab
- `internal/project/kind.go` — comment vocab (artifact-sense)
- `cmd/awf/list_add.go`, `cmd/awf/main.go` — bespoke `target` CLI path
- `templates/skills/*/SKILL.md.tmpl` (10 files) — prose neutralization
- `templates/docs/doc-standard.md.tmpl` — tool-agnostic-prose rule
- `.awf/config.yaml` — `targets: [claude, cursor]`
- `.gitignore` — `!.cursor/`
- Test files across `internal/config`, `internal/project`, `cmd/awf`
- ADR-0016 `target-output-paths` backing (removed Phase 8); ADR-0024 `cli-config-kinds` backing (updated Phase 8)

---

## Phase 1 — Artifact-sense rename (mechanical, behaviour-preserving)

ADR-0037 Decision 6. Rename the artifact-name sense of "target" to "artifact" so the adapter `Target` is unambiguous before the slice work. No behaviour change; `git diff` must be rename-only.

- [ ] **Rename in `internal/config/config.go`:** `func (c *Config) PartPath(kind, target, section string)` → `(kind, artifact, section string)`; update the body's `target` reference. Update the doc comment to say "a section of an artifact".
- [ ] **Rename in `internal/project/confighash.go`:** `targetConfigHash` → `artifactConfigHash` (method name); `consumedParts(kind, target string, ...)` → `(kind, artifact string, ...)`; update bodies + doc comments ("a target" → "an artifact").
- [ ] **Rename in `internal/project/render.go`:** the `target` parameter of `partRel`, `planSections`, and `renderTarget` → `artifact`; update bodies and the call site `p.targetConfigHash(...)` → `p.artifactConfigHash(...)` and `p.consumedParts(kind, artifact, plan)`. Leave the function name `renderTarget` itself unchanged (it renders one managed artifact; renaming it to `renderArtifact` is optional — do NOT, to keep the diff minimal and because the reviewer may prefer the broader rename as a separate cleanup).
- [ ] **Rename in `internal/project/check.go` + `kind.go`:** in `orphans()` update the comment vocabulary "target not in the enable list" → "artifact not in the enable list" (2 occurrences) and the loop discussion; in `kind.go` leave `outPath func(t Target, ...)` (adapter-sense Target) untouched. Do NOT rename `targetState` in `cmd/awf/list_add.go` in this phase — handle it in Phase 6 where the CLI is touched.
- [ ] **Do NOT touch** `catalog.VarDescriptor.Target` (initspec, unrelated sense) or the `target` loop variable in `checkDeadRefs` (`check.go:289`, a markdown-link target — unrelated sense).
- [ ] **Verify:** `go build ./... && ./x gate` → `0 issues.` and `coverage: 100.0%`. `./x check` → `awf check: clean`. Confirm rename-only: `git diff --stat` shows no new logic.
- [ ] **Commit:** `refactor(awf): rename artifact-sense "target" to "artifact" (ADR-0037)`

## Phase 2 — `targets` config enable array

ADR-0037 Decision 1. Backs `inv: targets-default-claude` (default + empty-rejection half; unknown-name half lands in Phase 3).

- [ ] **`internal/config/config.go`:** add field after `Domains` (line 43): `Targets []string \`yaml:"targets"\``. Update the `Config` doc comment to mention the targets enable array.
- [ ] **Default in `Load`** (after the `DocsDir` default, ~line 96):
  ```go
  if len(c.Targets) == 0 {
      c.Targets = []string{"claude"}
  }
  ```
  This makes an absent `targets:` key load as `["claude"]` (byte-identical render for existing configs; no schema bump, mirroring `Domains` — ADR-0014).
- [ ] **Validate in `Validate`** (after the `Domains` loop, ~line 157): reject an explicitly-empty list and per-name sanity (non-empty, no path separators):
  ```go
  if len(c.Targets) == 0 {
      return errors.New("targets must not be empty")
  }
  for _, t := range c.Targets {
      if t == "" || strings.ContainsAny(t, "/\\") || strings.Contains(t, "..") {
          return fmt.Errorf("target %q must be a non-empty name without path separators", t)
      }
  }
  ```
  (Unknown-adapter-name rejection is enforced at `project.Open` resolution in Phase 3, where the registry lives — config stays dependency-free. The `targets-default-claude` invariant is backed across both layers.)
- [ ] **Test `internal/config/config_test.go`:** add cases — (a) config with no `targets:` key → `Load` yields `["claude"]`; (b) `targets: []` (explicit empty after a non-default set via direct struct) → `Validate` errors; (c) `targets: ["a/b"]` → `Validate` errors. Add `// invariant: targets-default-claude` above the default+empty assertions.
- [ ] **Verify:** `./x gate` → 100% / `0 issues.` Existing golden/render tests still pass (default injection is transparent).
- [ ] **Commit:** `feat(awf): add targets config enable array with claude default (ADR-0037)`

## Phase 3 — Plural targets, Cursor descriptor, and the RenderAll split

ADR-0037 Decisions 2 & 3. The core refactor. Backs `inv: multi-target-render`, `inv: cursor-no-bridge`, and the unknown-name half of `inv: targets-default-claude`.

- [ ] **`internal/project/target.go`:** add the Cursor descriptor + registry + resolver:
  ```go
  var cursorTarget = Target{
      Name:     "cursor",
      SkillDir: ".cursor/skills",
      AgentDir: ".cursor/agents",
      // Cursor reads AGENTS.md natively — no bridge (ADR-0037).
      BridgeFile: "",
  }

  // targetRegistry maps an adapter name to its Target. Adding a runtime is a new
  // entry here, not a render-loop change.
  var targetRegistry = map[string]Target{
      "claude": claudeTarget,
      "cursor": cursorTarget,
  }

  // resolveTargets maps configured adapter names to their Target values in config
  // order, rejecting any unknown name (inv: targets-default-claude).
  func resolveTargets(names []string) ([]Target, error) {
      out := make([]Target, 0, len(names))
      for _, n := range names {
          t, ok := targetRegistry[n]
          if !ok {
              return nil, fmt.Errorf("unknown target %q (known: claude, cursor)", n)
          }
          out = append(out, t)
      }
      return out, nil
  }
  ```
- [ ] **`internal/project/project.go`:** change the struct field `Target Target` → `Targets []Target`. In `Open`, replace `Target: claudeTarget` with a resolution step:
  ```go
  targets, err := resolveTargets(cfg.Targets)
  if err != nil {
      return nil, err
  }
  p := &Project{Root: root, Cfg: cfg, Cat: cat, Targets: targets}
  ```
- [ ] **`internal/project/render.go` — split `RenderAll`.** Restructure so docs + AGENTS.md + adr-singletons render once and skills + agents + bridge render per target. Concretely:
  - Change `renderKindSpec.outPath` to `func(t Target, name string) string` and thread the active target through `renderKind` (add a `target Target` field to `renderKindSpec`, or pass `t` into `renderKind`). The docs spec's `outPath` ignores `t` and returns `p.docOutPath(n)`.
  - Render the **docs** spec once (outside any target loop).
  - Loop `for _, t := range p.Targets` and within it render the **skills** spec (`outPath: func(t Target, n) string { return t.SkillPath(p.Cfg.Prefix, n) }`) and the **agents** spec (`t.AgentPath(n)`).
  - Move the bridge render inside the same target loop, keeping the `if t.BridgeFile != ""` guard, so Claude emits `CLAUDE.md` and Cursor emits nothing. The AGENTS.md (agents-doc) render stays **once**, outside the target loop (the bridge no longer needs to be nested in the agents-doc block — render AGENTS.md once, then loop targets for the bridge).
  - The adr-readme/adr-template/plans-readme singleton loop stays once (neutral).
  - Preserve the existing `// invariant: doc-gated-skill-suppressed` comment on the skills spec.
- [ ] **Replace the `target-output-paths` realization in tests is deferred to Phase 8.** In this phase, the existing `TestClaudeTargetPaths` (target_test.go) stays green because `claudeTarget` still produces the same paths.
- [ ] **Test `internal/project/render_test.go` (or the existing render/spine test file):** with `Cfg.Targets = []string{"claude","cursor"}`, assert `RenderAll` produces, for one sample skill, both `.claude/skills/<prefix>-<name>/SKILL.md` and `.cursor/skills/<prefix>-<name>/SKILL.md` with **byte-identical Content**; one sample agent at both `.claude/agents/<name>.md` and `.cursor/agents/<name>.md`; AGENTS.md and each doc produced **exactly once**; `CLAUDE.md` present, no `.cursor` bridge file. Add `// invariant: multi-target-render` and `// invariant: cursor-no-bridge` above the relevant assertions. Add a `resolveTargets` unknown-name test (`["nope"]` → error) tagged `// invariant: targets-default-claude`.
- [ ] **Update existing single-target test fixtures** that construct a `Project` literal with `Target: claudeTarget` (grep: `coverage_test.go:132`, `kind_test.go:113`) to `Targets: []Target{claudeTarget}`.
- [ ] **Verify:** `./x gate` → 100% / `0 issues.` `./x check` → clean (this repo still renders only `[claude]` until Phase 8, so output is byte-identical).
- [ ] **Commit:** `feat(awf): render adapter artifacts per target with a cursor adapter (ADR-0037)`

## Phase 4 — Prune empty ancestor directories

ADR-0037 Decision 4. Backs `inv: target-prune-ancestors`.

- [ ] **`internal/project/project.go` — Sync prune loop** (currently ~line 144-153, `for path := range old.Files { if !want[path] {...} }`). Replace the per-file `os.Remove(file)` + single-level `os.Remove(filepath.Dir(file))` with: remove the file, then collect every ancestor dir up to `p.Root` into a set; after the prune loop, remove the collected dirs deepest-first (only succeeds when empty) — the exact idiom from `Uninstall` (install.go:102-111):
  ```go
  if old != nil {
      dirs := map[string]bool{}
      for path := range old.Files {
          if want[path] {
              continue
          }
          abs := filepath.Join(p.Root, path)
          _ = os.Remove(abs)
          for d := filepath.Dir(abs); d != p.Root; d = filepath.Dir(d) {
              dirs[d] = true
          }
      }
      dirList := slices.Collect(maps.Keys(dirs))
      slices.SortFunc(dirList, func(a, b string) int { return len(b) - len(a) })
      for _, d := range dirList {
          _ = os.Remove(d)
      }
  }
  ```
  Add the needed imports (`maps`, `slices`) if absent in project.go.
- [ ] **Test `internal/project/*_test.go`:** Sync with `Targets: [claude, cursor]` into a temp repo, then re-Open with `Targets: [claude]` and Sync again; assert `.cursor/`, `.cursor/skills/`, `.cursor/agents/` are all gone (not just the leaf SKILL.md's immediate parent). Add `// invariant: target-prune-ancestors`.
- [ ] **Verify:** `./x gate` → 100% / `0 issues.`
- [ ] **Commit:** `fix(awf): prune empty ancestor dirs when a target is removed (ADR-0037)`

## Phase 5 — Local skills/agents under multiple targets

ADR-0037 Consequences (local-skills resolution). awf ships no local skills, but the path must be defined.

- [ ] **`internal/project/check.go`:** change `localOutPath(kind, name) string` → `localOutPaths(kind, name) []string` returning one conventional path per enabled target (using `descriptorByPlural(kind).outPath(t, p.Cfg.Prefix, name)` for each `t := range p.Targets`); neutral kinds return nil. Update `checkLocalFrontmatter` to validate the local file at **each** target's path (a declared local skill must exist, with valid frontmatter, at every enabled target's path; absence at any target is a drift/fail entry). Update the doc comments to state the multi-target rule.
- [ ] **Test:** a fixture project with `Targets: [claude, cursor]` and a `local: true` skill sidecar; assert `checkLocalFrontmatter` reports the file absent at **both** target paths when neither exists, and passes when both exist with valid frontmatter. (No new invariant slug — this is a textual consequence; cover for the 100% gate.)
- [ ] **Verify:** `./x gate` → 100% / `0 issues.`
- [ ] **Commit:** `feat(awf): validate local skills at every target path (ADR-0037)`

## Phase 6 — `awf add/remove/list target` CLI

ADR-0037 Decision 5. Bespoke path, not a `kindDescriptor`. Backs `inv: target-cli`.

- [ ] **`cmd/awf/list_add.go`:** add a bespoke branch for the `target` kind token in `runAdd`/`runRemove`/`runList`, BEFORE the `project.PluralKind` lookup (so `target` does not fall through to `unknownKind`). For `add`/`remove`: validate `name` against the known-adapter set (expose a `project.KnownTargets() []string` or `project.IsKnownTarget(name) bool` helper from `target.go` so cmd doesn't duplicate the registry), guard already-enabled / not-enabled, then `rewriteConfig(root, "targets", name, add/false)` and `runSync`. For `list target`: print the `targets:` array with `enabled`/`available` state against `KnownTargets()`. Update `unknownKind`'s message to include `target`.
- [ ] **Rename `targetState`** (artifact-sense) → `artifactState` in this file (deferred from Phase 1), updating its call site, to avoid a third "target" reading colliding with the new adapter CLI token.
- [ ] **`cmd/awf/main.go`:** ensure the `add`/`remove`/`list` dispatch reaches the new branch (the kind token `target` is parsed the same way as `skill`/`doc`; no separate subcommand needed).
- [ ] **Test `cmd/awf/list_add_test.go` (or equivalent):** `awf add target cursor` on a `[claude]` repo updates `config.yaml` to `[claude, cursor]` and re-syncs (a `.cursor/` tree appears); `awf add target nope` errors; `awf remove target cursor` reverts and prunes; `awf list target` shows both with state. Add `// invariant: target-cli`.
- [ ] **Verify:** `./x gate` → 100% / `0 issues.` `./x check` → clean.
- [ ] **Commit:** `feat(awf): add awf add/remove/list target CLI (ADR-0037)`

## Phase 7 — Tool-agnostic skill prose (ADR-0038)

ADR-0038 Decisions 1-3. Backs `inv: skill-prose-tool-agnostic`.

- [ ] **Neutralize the 10 skill templates.** In each, rewrite Claude-tool vocabulary to action-language, preserving procedure. Exact sites (verified):
  - `templates/skills/brainstorming/SKILL.md.tmpl:25` — `Prefer multiple choice (\`AskUserQuestion\` tool when available)` → `Prefer multiple-choice questions where your runtime supports them`.
  - `templates/skills/brainstorming/SKILL.md.tmpl:39` — `dispatch ONE subagent via the \`Agent\` tool (\`subagent_type: Explore\` ...)` → `dispatch ONE fresh-context exploration subagent (read-only; one that can run a command only when the grounding-check needs it)`.
  - `templates/skills/reviewing-plan/SKILL.md.tmpl:32`, `reviewing-adr/SKILL.md.tmpl:29`, `reviewing-plan-resync/SKILL.md.tmpl:23` — `Invoke the agent tool with subagent type \`<name>\`` → `Dispatch the \`<name>\` subagent`.
  - `templates/skills/reviewing-impl/SKILL.md.tmpl:33` — `Invoke \`Agent({subagent_type: "code-reviewer", ...})\`` → `Dispatch the \`code-reviewer\` subagent`.
  - `templates/skills/refactor-coupling-audit/SKILL.md.tmpl:33` — `dispatch a single \`Explore\` subagent via the Agent tool` → `dispatch a single fresh-context exploration subagent`.
  - `templates/skills/executing-plans/SKILL.md.tmpl:58`, `proposing-adr/SKILL.md.tmpl:77`, `writing-plans/SKILL.md.tmpl:67`, `subagent-driven-development/SKILL.md.tmpl:73` — `via the \`Skill\` tool` → `via the project's skill-invocation mechanism` (or reword to "invoke the \`<name>\` skill").
  - `templates/skills/subagent-driven-development/SKILL.md.tmpl:40,48` — neutralize the `\`Agent\` prompt` / `via the \`Agent\` tool` references to "the subagent's prompt" / "dispatch a subagent".
  Preserve the neutral word "subagent"/"subagent prompt" (the denylist is word-anchored so it won't false-positive). Keep project identifiers (`awf-reviewing-adr`, `./x gate`) verbatim.
- [ ] **`templates/docs/doc-standard.md.tmpl`:** add a rule to the existing `rules` section (no new section — preserves `docs_sections_test` parity): rendered skill/agent prose names the action an agent performs, not a runtime's tool names.
- [ ] **Golden guard test `internal/project/*_test.go`:** render every catalog skill + agent (reuse the `frontmatter_test.go` per-template render harness) and assert each body contains none of the denylist tokens, matched **case-insensitively and word-anchored**: `subagent_type`, "subagent type", "Agent tool", "the agent tool", "`Agent` prompt", "Skill tool", "AskUserQuestion". Add `// invariant: skill-prose-tool-agnostic`.
- [ ] **`./x sync`** to re-render this repo's `.claude/skills/*` with the neutralized prose; stage the regenerated skill files.
- [ ] **Verify:** `./x gate` → 100% / `0 issues.` `./x check` → clean (re-rendered skills match).
- [ ] **Commit:** `feat(awf): neutralize runtime tool vocabulary in skill prose (ADR-0038)`

## Phase 8 — Dogfood + flip both ADRs to Implemented

Backs the retirement/replacement and activates enforcement.

- [ ] **`.gitignore`:** add `!.cursor/` (negate any global ignore of `.cursor`, mirroring the existing `!.claude/` / `!CLAUDE.md` lines) so the dogfooded Cursor render commits and is drift-checked.
- [ ] **`.awf/config.yaml`:** add `targets: [claude, cursor]` (or insert `cursor` into an existing `targets:` if Phase 6 wrote one).
- [ ] **Retire the old invariant backing:** remove the `// invariant: target-output-paths` comment + its now-superseded assertion from `internal/project/target_test.go` (ADR-0016 stays Implemented; ADR-0037's `retires_invariants: [target-output-paths]` authorizes this once 0037 is Implemented). Ensure `multi-target-render`'s backing test (Phase 3) covers both claude and cursor paths.
- [ ] **Update `cli-config-kinds` backing:** in `internal/project/kind_test.go` (or wherever `cli-config-kinds` is backed), extend the assertion for the added `target` CLI token so the invariant text ("CLI-addressable kinds") still matches — per ADR-0037's Invariants note that `cli-config-kinds` is extended, not contradicted.
- [ ] **Flip both ADRs:** set `status: Implemented` in ADR-0037 and ADR-0038 frontmatter.
- [ ] **`./x sync`** — re-renders, generates the `.cursor/` tree (skills + agents; no bridge), regenerates `ACTIVE.md` (both ADRs now Implemented) and the rendering/config/tooling domain indexes.
- [ ] **Doc-currency (same commit):** update `docs/architecture.md` (plural-`Targets` seam, neutral/adapter split, Cursor adapter no-bridge) and refresh the `rendering`, `config`, `tooling` domain current-state narratives (`.awf/domains/parts/<d>/current-state.md`); add the `awf add/remove/list target` grammar to the agent guide "Working with awf" section (`.awf/parts/agents-doc/...`) and the README command table. Re-run `./x sync` after part edits.
- [ ] **Verify:** `./x gate` → 100% / `0 issues.` `./x check` → `awf check: clean` (the new `.cursor/` tree is tracked and matches; all tagged invariants for both ADRs now enforced and backed). `awf audit` advisory clean.
- [ ] **Commit:** `feat(awf)!: render to claude and cursor; implement ADR-0037 and ADR-0038` (stage the new `.cursor/` tree explicitly; one concern — the multi-target go-live).

---

## Notes for the executor

- One commit per phase; `./x gate` + `./x check` green before each commit; Conventional Commits, `awf` scope.
- Tagged-invariant enforcement activates only when the ADRs flip (Phase 8) — the `// invariant:` comments + backing tests land in their implementing phases (2-7) and are inert until then, except `target-output-paths`/`cli-config-kinds` which stay backed (ADR-0016/0024 are already Implemented) until Phase 8 retires/updates them.
- The render-loop split (Phase 3) is the one place exact diffs are hard to pin without the live file open; follow the structure above and let `./x gate` + the byte-identical-output check on this repo (`./x check` clean through Phase 7) confirm correctness.
