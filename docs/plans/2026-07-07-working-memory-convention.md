# Plan: Working-memory convention for chain session continuity

**Date:** 2026-07-07
**ADR:** [ADR-0069](../decisions/0069-working-memory-convention-for-chain-session-continuity.md) — Working-memory convention for chain session continuity

## Goal

Give chain runs a durable resume path across session death/compaction: an always-on rendered
`.awf/memory/.gitignore` (the ephemerality mechanism), an agent-guide working-memory section
(the convention + resume protocol), continuous brainstorm checkpointing, a shared checkpoint
partial across eleven skills, and a retrospective deletion step. Design rationale lives in
ADR-0069; this plan is the execution record.

## Architecture summary

- **Render unit.** `.awf/memory/.gitignore` renders unconditionally from a new
  `templates/memory/gitignore.tmpl` via a `RenderAll` tail block (bootstrap/hooks precedent,
  minus the config gate). Lock/drift/prune/collision/uninstall are automatic for `RenderAll`
  outputs. `injectBanner` gains a `#`-comment branch keyed on the new template id;
  `isManagedMarkdown` excludes it.
- **Convention prose.** New `working-memory` section in the agents-doc template (catalog
  `Sections` addition), protected by a new agents-doc parity test (`agents-doc-section-parity`).
  Brainstorming checkpoints continuously (`procedure` section) and its grounding dispatch-brief
  rule is reworded; a shared partial `templates/partials/memory-checkpoint.md` is spliced via
  `awf:include` inside the terminal/hand-off sections of the nine non-terminal chain nodes;
  `bugfix`/`debugging` each gain a new declared `memory-checkpoint` section; `retrospective`
  gains the deletion step in `procedure`.
- **Coverage lock.** `internal/evals` asserts every checkpoint carrier renders the checkpoint
  token and retrospective renders the deletion step (`memory-checkpoint-chain-coverage`).
- The partial contains no skill names and no template constructs, so no `{{ if index .skills }}`
  guards or unset-degradation cases are needed for it.

## Tech stack

Go 1.26. Packages touched: `internal/project` (render.go, banner.go, check.go + tests),
`internal/catalog` (standard.go), `internal/evals` (chain_test.go), `templates` (embed.go,
memory/, partials/, skills/, agents-doc/, docs/). Dogfood config tree: `.awf/agents-doc.yaml`,
`.awf/domains/parts/rendering/current-state.md`, `.awf/domains/parts/tooling/current-state.md`.

## File structure

Created:
- `templates/memory/gitignore.tmpl`
- `templates/partials/memory-checkpoint.md`
- `internal/project/memory_test.go`

Modified:
- `templates/embed.go`, `internal/project/render.go`, `internal/project/banner.go`,
  `internal/project/check.go`, `internal/project/banner_test.go`,
  `internal/project/coverage_test.go`, `internal/project/docs_sections_test.go`
- `internal/catalog/standard.go` (agents-doc, bugfix, debugging `Sections`)
- `templates/agents-doc/AGENTS.md.tmpl`, `templates/skills/{brainstorming,proposing-adr,
  reviewing-adr,writing-plans,reviewing-plan,reviewing-plan-resync,executing-plans,
  subagent-driven-development,reviewing-impl,bugfix,debugging,retrospective}/SKILL.md.tmpl`
- `templates/docs/workflow.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`
- `internal/evals/chain_test.go`
- `.awf/agents-doc.yaml`, `.awf/domains/parts/rendering/current-state.md`,
  `.awf/domains/parts/tooling/current-state.md`
- `changelog/CHANGELOG.md`, ADR-0069 status flip
- Rendered outputs via `./x sync` (AGENTS.md, `.claude/`+`.cursor/` skills, docs,
  `.awf/memory/.gitignore`, `.awf/awf.lock`)

Deleted: none.

---

## Phase 1 — always-on render unit (banner, scan scoping, dogfood)

Commit: `feat(rendering): render always-on self-ignoring .awf/memory/.gitignore`

- [ ] Create `templates/memory/gitignore.tmpl` with exactly this content (trailing newline):

  ```
  *
  !.gitignore
  ```

- [ ] In `templates/embed.go`, extend the embed directive:

  ```go
  //go:embed all:skills all:agents agents-doc docs domains claude adr-readme adr-template plans-readme bootstrap hooks partials memory
  ```

- [ ] In `internal/project/render.go`, add the TID constant next to `bootstrapTID`:

  ```go
  	memoryTID    = "memory/gitignore.tmpl"
  ```

- [ ] In `internal/project/render.go`, insert after the hooks block (after its closing `}`,
  before `return out, nil` in `RenderAll`):

  ```go
  	// .awf/memory/.gitignore (neutral config-tree singleton; ALWAYS rendered —
  	// ADR-0069, no config gate unlike bootstrap/hooks). Self-ignoring, so the
  	// working-memory convention's ephemerality is mechanical, not remembered.
  	// Deliberately non-configurable: no catalog spec, no sections, no CLI kind.
  	mrf, err := p.renderTarget("memory", "", memoryTID,
  		nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/memory/.gitignore")
  	if err != nil { // coverage-ignore: the memory gitignore template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
  		return nil, err
  	}
  	out = append(out, mrf)
  ```

- [ ] In `internal/project/banner.go`, change `injectBanner` to take the template id and add
  the `#`-comment branch (full new function body):

  ```go
  // injectBanner inserts the generated-by banner into rendered content: for the
  // memory gitignore, a leading `#` comment (a valid .gitignore comment — ADR-0069);
  // for shebang scripts, a `#`-comment after the shebang line (keeping the script
  // executable); for frontmatter targets, an HTML comment after the closing `---`;
  // otherwise an HTML comment as the first line.
  // invariant: provenance-banner
  func injectBanner(content, tid string) string {
  	if tid == memoryTID {
  		return "# " + bannerText + "\n" + content
  	}
  	if strings.HasPrefix(content, "#!") {
  		// Shell/script target: banner as a # comment after the shebang line.
  		nl := strings.IndexByte(content, '\n')
  		if nl < 0 { // coverage-ignore: a rendered shebang script always has a trailing newline body
  			return content
  		}
  		return content[:nl+1] + "# " + bannerText + "\n" + content[nl+1:]
  	}
  	line := "<!-- " + bannerText + " -->\n"
  	if yamlBlock, body, found := frontmatter.Split([]byte(content)); found {
  		return "---\n" + string(yamlBlock) + "---\n" + line + string(body)
  	}
  	return line + content
  }
  ```

- [ ] In `internal/project/render.go`, update both call sites: in `renderTarget`,
  `content = injectBanner(content)` → `content = injectBanner(content, tid)`; in
  `generateActiveMD`, `content = injectBanner(content)` → `content = injectBanner(content, "")`
  (ACTIVE.md is markdown — the empty id keeps its HTML-comment banner unchanged).

- [ ] In `internal/project/banner_test.go`, update every existing `injectBanner(...)` call to
  pass `""` as the second argument, and add:

  ```go
  // The memory gitignore is neither markdown nor a shebang script: its banner is a
  // leading #-comment keyed on the template id (ADR-0069).
  func TestInjectBannerMemoryGitignore(t *testing.T) {
  	got := injectBanner("*\n!.gitignore\n", memoryTID)
  	want := "# " + bannerText + "\n*\n!.gitignore\n"
  	if got != want {
  		t.Errorf("memory gitignore banner:\ngot  %q\nwant %q", got, want)
  	}
  }
  ```

- [ ] In `internal/project/check.go`, extend `isManagedMarkdown` (and reword its doc
  comment's exclusion phrase — a gitignore is not a shell script — to "except the CLAUDE.md
  bridge and the non-markdown render units (the bootstrap, the git-hook payloads, and the
  memory gitignore — ADR-0048, ADR-0069)"):

  ```go
  func isManagedMarkdown(tid string) bool {
  	return tid != bridgeTID && tid != bootstrapTID && tid != memoryTID &&
  		!strings.HasPrefix(tid, "hooks/")
  }
  ```

- [ ] In `internal/project/coverage_test.go`, extend `TestIsManagedMarkdownExcludesBootstrap`
  by inserting before its closing `}`:

  ```go
  	if isManagedMarkdown(memoryTID) {
  		t.Error("the memory gitignore template must not be scanned for dead references")
  	}
  ```

- [ ] Create `internal/project/memory_test.go`:

  ```go
  package project

  import (
  	"strings"
  	"testing"
  )

  // TestMemoryGitignoreAlwaysOn asserts RenderAll unconditionally emits the
  // self-ignoring .awf/memory/.gitignore with a #-comment banner (ADR-0069) —
  // no config gate, unlike bootstrap/hooks.
  // invariant: memory-gitignore-always-on
  func TestMemoryGitignoreAlwaysOn(t *testing.T) {
  	root := scaffold(t, "prefix: example\n")
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	out, err := p.RenderAll()
  	if err != nil {
  		t.Fatal(err)
  	}
  	var found *RenderedFile
  	for i := range out {
  		if out[i].Path == ".awf/memory/.gitignore" {
  			found = &out[i]
  		}
  	}
  	if found == nil {
  		t.Fatal("expected .awf/memory/.gitignore in every RenderAll output")
  	}
  	want := "# " + bannerText + "\n*\n!.gitignore\n"
  	if found.Content != want {
  		t.Errorf("content = %q, want %q", found.Content, want)
  	}
  	if !strings.HasPrefix(found.Content, "# ") {
  		t.Errorf("banner must be a #-comment, got %q", found.Content)
  	}
  }
  ```

- [ ] In `.awf/agents-doc.yaml`, replace the ADR-0060/0061 invariant entry's `text:` value with
  (one YAML-quoted string; the change appends the boundary sentence):

  ```
  '**Unified compile-time doc model.** The catalog is a compile-time Go value (`catalog.Standard`, no embedded YAML), and every doc — toggleable or always-on singleton — is one `DocEntry` from which every projection (`SingletonKinds`, `plainSingletons`, the `.layout` paths, the toggleable pool) derives; adding a mandatory doc is a single entry. Config-tree render units — the bootstrap, the hook payloads, and the working-memory `.gitignore` (ADR-0069) — are deliberately outside the doc collection: dedicated render blocks, no `DocEntry`.'
  ```

- [ ] In `.awf/domains/parts/rendering/current-state.md`, append to the end of the paragraph
  (same line run, after the ADR-0058 sentence):

  ```
  A third neutral config-tree unit, `.awf/memory/.gitignore` (ADR-0069), renders unconditionally — no config gate, no catalog spec — as a self-ignoring gitignore backing the working-memory convention; `injectBanner` stamps it with a `#`-comment banner via a branch keyed on its template id, and that id is excluded from the managed-markdown scans like the bootstrap and hook payloads.
  ```

- [ ] Run `go test ./...`. Expect failures only in fixtures that enumerate `RenderAll` outputs
  or lock contents exactly (candidates: `internal/project/install_test.go`,
  `internal/project/render_tree_test.go`, `internal/project/drift_test.go`,
  `internal/project/project_test.go`). Fix each by adding `.awf/memory/.gitignore` to the
  expected output/lock set — never by weakening an assertion.
- [ ] Run `./x sync` — expect `.awf/memory/.gitignore` created (verify:
  `head -1 .awf/memory/.gitignore` prints
  `# GENERATED by awf — do not edit; change .awf/ and run \`awf sync\``), AGENTS.md invariant
  wording updated, `docs/domains/rendering.md` updated, lock updated. Run
  `git check-ignore .awf/memory/somefile` → exits 0 (path is ignored).
- [ ] Run `./x check` → `awf check: clean`. Run `./x gate` → green, coverage 100.0%.
- [ ] Stage and commit: template, embed.go, render.go, banner.go, check.go, the three test
  files, `.awf/agents-doc.yaml`, `.awf/domains/parts/rendering/current-state.md`, and all
  re-rendered outputs (`AGENTS.md`, `docs/domains/rendering.md`, `.awf/memory/.gitignore`,
  `.awf/awf.lock`, plus any fixture files fixed).

## Phase 2 — agent-guide working-memory section + agents-doc parity test

Commit: `feat(rendering): add working-memory guide section and parity test`

- [ ] In `internal/catalog/standard.go`, add `"working-memory"` to the agents-doc `Sections`
  between `"workflow"` and `"commands"`:

  ```go
  		"agents-doc": {Mandatory: true, AgentsDoc: true, TID: "agents-doc/AGENTS.md.tmpl", Sections: []string{
  			"awf-setup", "you-and-this-project", "identity", "invariants", "workflow", "working-memory", "commands", "document-map",
  		}},
  ```

- [ ] In `templates/agents-doc/AGENTS.md.tmpl`, insert a new section between the `workflow`
  section's `<!-- awf:end -->` and `<!-- awf:section commands -->` (blank line before and
  after, matching neighbors):

  ```markdown
  <!-- awf:section working-memory -->
  ## Working memory

  Session context is volatile; the chain's working state must not be. `.awf/memory/` (kept out of version control by a rendered self-ignoring `.gitignore`) holds one working-memory file per in-flight effort: `.awf/memory/<effort-slug>.md`.

  - **On starting work, check `.awf/memory/`.** If an effort file matches the task at hand, resume from its recorded `Phase:`/`Next:` lines instead of restarting. If several files exist, or a file matches no in-flight work you can verify, ask the user which (if any) to resume — never silently resume a stale effort.
  - **While working, prefer just-in-time retrieval.** Hold lightweight identifiers — file paths, ADR numbers, doc names — in the memory file and read the sources on demand rather than preloading them.
  - **File skeleton** (a convention, not a schema — no tool parses it): a header (`# <effort title>`, `Phase:`, `Next:`, `Updated:`), then `## Brief` (the evolving design brief: problem, settled decisions, user constraints verbatim, rejected approaches), `## Handoff log` (one line per completed phase), and `## Scratch` (open questions, references).
  - **Ground rules.** The file is session state, never a design artifact: never commit it (the rendered `.gitignore` makes that mechanical), never cite it in an ADR, plan, or commit message, and delete it when the effort's chain terminates. Files orphaned by an abandoned effort are harmless gitignored residue — delete them when noticed; `awf uninstall` leaves a non-empty `.awf/memory/` in place.
  <!-- awf:end -->
  ```

- [ ] Append to `internal/project/docs_sections_test.go`:

  ```go
  // TestAgentsDocSectionParity asserts the agents-doc template's marker-block set
  // matches its catalog-declared sections, order-exact. The AgentsDoc entry is
  // excluded from both TestDocsSectionParity (Mandatory skip) and
  // TestAdrSingletonSectionParity (plainSingletons excludes it), so without this
  // test a guide section could half-land with a broken override path (ADR-0069).
  // invariant: agents-doc-section-parity
  func TestAgentsDocSectionParity(t *testing.T) {
  	cat := catalog.Standard
  	entry := cat.Docs["agents-doc"]
  	src, err := fs.ReadFile(templates.FS, entry.TID)
  	if err != nil {
  		t.Fatalf("read %s: %v", entry.TID, err)
  	}
  	var markers []string
  	for _, s := range render.ParseSections(string(src)) {
  		if s.IsSection {
  			markers = append(markers, s.Name)
  		}
  	}
  	if strings.Join(markers, ",") != strings.Join(entry.Sections, ",") {
  		t.Errorf("%s markers %v != catalog sections %v", entry.TID, markers, entry.Sections)
  	}
  }
  ```

- [ ] In `.awf/agents-doc.yaml`, add a new invariants entry directly after the ADR-0060/0061
  entry:

  ```yaml
          - ref: ADR-0069
            text: '**Ephemeral working memory.** `awf sync` always renders the self-ignoring `.awf/memory/.gitignore`; working-memory files are session state — never committed, never cited by an ADR, plan, or commit message, deleted when the chain terminates.'
  ```

- [ ] Run `go test ./internal/project/ -run 'SectionParity|AgentsDoc'` → pass. Deliberately
  break parity once (temporarily remove `"working-memory"` from the catalog list, run the test,
  expect failure naming the mismatch, restore) to confirm the guard bites.
- [ ] Run `./x sync` (AGENTS.md gains the section + the new invariant bullet) then `./x check`
  → clean, `./x gate` → green.
- [ ] Stage and commit: standard.go, AGENTS.md.tmpl, docs_sections_test.go,
  `.awf/agents-doc.yaml`, re-rendered `AGENTS.md`, `.awf/awf.lock`.

## Phase 3 — checkpoint partial, skill splices, retrospective deletion, doc templates

Commit: `feat(rendering): checkpoint working memory across the chain skills`

- [ ] Create `templates/partials/memory-checkpoint.md` with exactly (no heading, one
  paragraph, no template constructs, no skill names):

  ```markdown
  **Working-memory checkpoint.** Before handing off, update the effort's working-memory file `.awf/memory/<effort-slug>.md` (create it if missing): set `Phase:` to the phase just completed, `Next:` to the successor step, append one line to `## Handoff log`, and refresh `Updated:`. The file skeleton and ground rules live in the agent guide's working-memory section.
  ```

- [ ] In `templates/skills/brainstorming/SKILL.md.tmpl`:
  - Replace the empty `procedure` section

    ```
    <!-- awf:section procedure -->
    ## Procedure
    <!-- awf:end -->
    ```

    with

    ```
    <!-- awf:section procedure -->
    ## Procedure

    Throughout, checkpoint the evolving design brief to the working-memory file `.awf/memory/<effort-slug>.md` as each decision settles — create it when the first decision lands (see the agent guide's working-memory section). A session death mid-brainstorm must lose minutes, not the negotiation.
    <!-- awf:end -->
    ```

  - In the `grounding-check-output-format` section, replace the sentence
    `Do NOT write the brief to a file.` (end of the first paragraph) with:
    `Synthesise the dispatch brief inline in the subagent prompt — do NOT write it to a file; the only on-disk record of the brainstorm is the evolving design brief in the working-memory file.`
  - In the `terminal-step` section, insert a blank line plus the include line after the last
    bullet (`- **Neither** → …reviewing-impl\`.`), before `<!-- awf:end -->`:

    ```
    <!-- awf:include memory-checkpoint -->
    ```

- [ ] Splice the same two lines (blank line + `<!-- awf:include memory-checkpoint -->`) after
  the step text and before `<!-- awf:end -->` in each of these eight sections:
  - `templates/skills/proposing-adr/SKILL.md.tmpl` — section `terminal-step`
  - `templates/skills/reviewing-adr/SKILL.md.tmpl` — section `hand-off-to-resync`
  - `templates/skills/writing-plans/SKILL.md.tmpl` — section `terminal-step`
  - `templates/skills/reviewing-plan/SKILL.md.tmpl` — section `hand-off`
  - `templates/skills/reviewing-plan-resync/SKILL.md.tmpl` — section `hand-off-to-impl`
  - `templates/skills/executing-plans/SKILL.md.tmpl` — section `terminal-step`
  - `templates/skills/subagent-driven-development/SKILL.md.tmpl` — section `terminal-step`
  - `templates/skills/reviewing-impl/SKILL.md.tmpl` — section `hand-off`

- [ ] In `internal/catalog/standard.go`, append `"memory-checkpoint"` to two skill `Sections`:

  ```go
  		"bugfix": {Sections: []string{"test-tiers", "pitfalls-check", "oracle-note", "memory-checkpoint"}},
  ```

  and for `debugging`:

  ```go
  		"debugging": {Sections: []string{
  			"symptom-list", "debugging-surfaces", "test-isolation", "oracle-invariant",
  			"devdb-note", "red-flags", "memory-checkpoint",
  		}},
  ```

- [ ] In `templates/skills/bugfix/SKILL.md.tmpl`, insert a new marked section after the
  `oracle-note` section's `<!-- awf:end -->` and before `## Notes` (blank lines around):

  ```
  <!-- awf:section memory-checkpoint -->
  <!-- awf:include memory-checkpoint -->
  <!-- awf:end -->
  ```

- [ ] In `templates/skills/debugging/SKILL.md.tmpl`, append the same three lines as a new
  section after the `red-flags` section's `<!-- awf:end -->` at end of file.

- [ ] In `templates/skills/retrospective/SKILL.md.tmpl`, append a step 5 to the `procedure`
  section (after step 4, before `<!-- awf:end -->`):

  ```
  5. **Delete the effort's working-memory file** (`.awf/memory/<effort-slug>.md`), if one exists — the chain is complete and the ADR/plan/commits are the durable record. Working memory never outlives its effort.
  ```

- [ ] In `templates/docs/workflow.md.tmpl`, `chain` section, append to the end of the prose
  paragraph (after `…toward a deterministic check.`):
  ` Throughout, each chain skill checkpoints its position — and brainstorming its evolving design brief — to a working-memory file under \`.awf/memory/\` (see the agent guide's working-memory section), so a session death or context compaction resumes instead of restarting; the retrospective deletes the file.`

- [ ] In `templates/docs/working-with-awf.md.tmpl`, `overview` section, append a paragraph
  before `<!-- awf:end -->`:

  ```
  awf also always renders `.awf/memory/.gitignore` — a self-ignoring gitignore that keeps the working-memory directory (per-effort session-state files, described in the agent guide's working-memory section) out of version control. The `.gitignore` itself is rendered and drift-checked; the directory's contents never are.
  ```

- [ ] Run `./x sync` then `./x check` → clean (all skill renders across `.claude/` and
  `.cursor/` update, plus `docs/workflow.md`, `docs/working-with-awf.md`). Spot-check:
  `grep -c "Working-memory checkpoint." .claude/skills/awf-*/SKILL.md` prints `1` for exactly
  the eleven carriers (the nine chain nodes plus bugfix and debugging) and `0` for the other
  four (adr-lifecycle, refactor-coupling-audit, retrospective, tdd), and
  `grep "Delete the effort's working-memory file" .claude/skills/awf-retrospective/SKILL.md`
  matches.
- [ ] Run `./x gate` → green. If `skill-section-parity` or golden tests fail, the failure names
  the exact template/section drift — fix the template, not the test (the only legitimate test
  updates are golden fixtures that pin full rendered bodies).
- [ ] Stage and commit: the partial, twelve skill templates, standard.go, the two doc
  templates, and all re-rendered outputs + lock.

## Phase 4 — eval-suite coverage lock

Commit: `test(rendering): lock chain working-memory checkpoint coverage`

- [ ] Append to `internal/evals/chain_test.go`:

  ```go
  // memoryCheckpointSkills are the templates that must carry the working-memory
  // checkpoint (ADR-0069): the nine non-terminal chain nodes plus the bugfix and
  // debugging task skills. The terminal retrospective instead carries the
  // deletion step.
  var memoryCheckpointSkills = []string{
  	"brainstorming", "proposing-adr", "reviewing-adr", "writing-plans",
  	"reviewing-plan", "reviewing-plan-resync", "executing-plans",
  	"subagent-driven-development", "reviewing-impl", "bugfix", "debugging",
  }

  // TestMemoryCheckpointCoverage asserts every non-terminal chain node and the
  // multi-step task skills instruct the working-memory checkpoint in the rendered
  // full-catalog output, and the chain terminal instructs the deletion (ADR-0069).
  // invariant: memory-checkpoint-chain-coverage
  func TestMemoryCheckpointCoverage(t *testing.T) {
  	cat := loadCatalog(t)
  	root := syncFullCatalog(t, cat)
  	const token = "**Working-memory checkpoint.**"
  	for _, name := range memoryCheckpointSkills {
  		if body := read(t, skillPath(root, name)); !strings.Contains(body, token) {
  			t.Errorf("skill %q missing the working-memory checkpoint", name)
  		}
  	}
  	if body := read(t, skillPath(root, "retrospective")); !strings.Contains(body, "Delete the effort's working-memory file") {
  		t.Errorf("retrospective missing the working-memory deletion step")
  	}
  }
  ```

- [ ] Run `go test ./internal/evals/` → pass. Deliberately break once (temporarily delete the
  include line from `templates/skills/bugfix/SKILL.md.tmpl`, expect the test to fail naming
  `bugfix`, restore) to confirm the lock bites.
- [ ] In `.awf/domains/parts/tooling/current-state.md`, append to the end of the final
  (`internal/evals`) paragraph, after `…blank-path provenance pointer.` (same line run —
  ADR-0069 declares the tooling domain, so its current-state narrative must refresh before
  the Phase 5 Implemented flip or the `domain-doc-staleness` audit rule fires):

  ```
  ADR-0069 adds a working-memory coverage lock (`memory-checkpoint-chain-coverage`): the nine non-terminal chain-node skills plus `bugfix` and `debugging` must render the checkpoint instruction in the full-catalog output, and `retrospective` the deletion step.
  ```

- [ ] Run `./x sync` (`docs/domains/tooling.md` picks up the narrative) then `./x check` →
  clean.
- [ ] Run `./x gate` → green. Stage and commit: `internal/evals/chain_test.go`,
  `.awf/domains/parts/tooling/current-state.md`, `docs/domains/tooling.md`, and
  `.awf/awf.lock` if changed (plus the restored-template no-op if git shows nothing else).

## Phase 5 — changelog, ADR flip

Commit: `docs(adr): mark 0069 implemented`

- [ ] In `changelog/CHANGELOG.md`, under `## [Unreleased]`, add:

  ```markdown
  ### Features
  - Working-memory convention for chain session continuity (ADR-0069): `awf sync` now always
    renders a self-ignoring `.awf/memory/.gitignore`; the agent guide gains a working-memory
    section (per-effort `.awf/memory/<effort-slug>.md` files, resume protocol, JIT-retrieval
    guidance); brainstorming checkpoints its design brief continuously; the chain skills plus
    bugfix/debugging checkpoint phase/handoff state; the retrospective deletes the file.
  ```

- [ ] In `docs/decisions/0069-working-memory-convention-for-chain-session-continuity.md`,
  flip frontmatter `status: Proposed` → `status: Implemented`.
- [ ] Run `./x sync` (ACTIVE.md + domain-doc indexes pick up the flip), `./x check` → clean.
- [ ] Run `go run ./cmd/awf invariants` → no unbacked slugs (the three ADR-0069 slugs are
  backed by markers landed in Phases 1, 2, and 4).
- [ ] Run `./x gate` → green.
- [ ] Stage and commit: the ADR, `changelog/CHANGELOG.md`, `docs/decisions/ACTIVE.md`,
  regenerated domain docs, `.awf/awf.lock` if changed.
