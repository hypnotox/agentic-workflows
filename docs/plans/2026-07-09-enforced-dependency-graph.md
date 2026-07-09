# 2026-07-09 — Enforced dependency graph

**Goal:** implement [ADR-0081](../decisions/0081-enforced-dependency-graph-over-catalog-requires-declarations.md)
(typed edge layer + resolver with plan/apply; hard closure validation at open; closure-applying
`awf add`; dependent-refusing `awf remove` with `--with-dependents` and `--dry-run`;
`RequiresDoc` folded in with the ADR-0013 suppression machinery deleted; schema-8
`close-enabled-set` migration; init trim agent derivation + closure). Design rationale lives
in the ADR — not duplicated here.

**Architecture summary:** edges are enumerated in exactly one place
(`catalog.RequiresOf`); `catalog.Closure` is the pure forward walk (first production
consumer: Phase 4's scaffold — the resolver's provenance-carrying walks are
deliberately its own, so `Closure` lands in Phase 4); the resolver
(`internal/project/resolve.go`) turns walks into `PlanOp` plans consumed by `runAdd`/
`runRemove`; validation checks direct edges only (closure by induction). Two hard
sequencing constraints from the ADR: the suppression code + its
`doc-gated-skill-suppressed` marker are deleted only in the flip commit (Phase 6), and
every new exported func lands in the same commit as its production consumer (dead-code
gate). Graph facts (reviewer-verified): two mutually-requiring cores (5-skill planning,
3-skill execution, edges planning→execution), `brainstorming` a pure source,
`retrospective`/`adr-lifecycle` sinks; worst-case cascade = 10 skills + `plan-reviewer`
from the `retrospective` seed.

**Tech stack:** Go 1.26, stdlib only. Packages touched: `internal/catalog` (new
`graph.go`), `internal/project` (new `resolve.go`; `validate.go`, `render.go`,
`scaffold.go`, `project.go`), `internal/migrate` (new migration + `Apply` signature),
`cmd/awf` (`main.go`, `list_add.go`, `init.go`, tests), `templates/` (two prose
edits), `changelog/`, `.awf/` parts.

**File structure:**

- Created: `internal/catalog/graph.go`, `internal/catalog/graph_test.go`,
  `internal/project/resolve.go`, `internal/project/resolve_test.go`,
  `internal/migrate/closeenabledset.go`, `internal/migrate/closeenabledset_test.go`,
  `docs/plans/2026-07-09-enforced-dependency-graph.md` (this plan)
- Modified: `internal/project/validate.go`, `internal/project/render.go`,
  `internal/project/scaffold.go`, `internal/project/project.go`,
  `internal/project/drift_test.go`, `internal/project/skillrefs_test.go`,
  `internal/migrate/migrate.go` (+ the seven existing migration files' `Apply`
  signatures), `cmd/awf/main.go`, `cmd/awf/list_add.go`, `cmd/awf/list_add_test.go`,
  `cmd/awf/init.go`, `cmd/awf/upgrade.go`,
  `templates/agents-doc/AGENTS.md.tmpl` (awf-setup section),
  `templates/docs/working-with-awf.md.tmpl` (commands section),
  `changelog/CHANGELOG.md`, `.awf/agents-doc.yaml`,
  `.awf/docs/parts/glossary/terms.md`,
  `.awf/domains/parts/config/current-state.md`,
  `.awf/domains/parts/rendering/current-state.md`,
  `.awf/domains/parts/tooling/current-state.md`,
  `docs/decisions/0081-*.md` (status flip), `docs/decisions/0013-*.md` /
  `0046-*.md` / `0050-*.md` (`related:` back-pointers), plus rendered files
  refreshed by `./x sync`
- Deleted: none (the suppression code is removed in place)

**Phase → ADR Decision map:** P1→D1(edges)+D3; P2→D2+D4+D5+D6;
P3→D8; P4→D1(closure)+D9; P5→fixture obligations the decisions imply;
P6→D7 + flip obligations. (Prose obligations ride P2 per the ADR's same-commit rule.)

---

## Phase 1 — edge enumerator + hard closure validation

- [ ] Create `internal/catalog/graph.go`:

      ```go
      package catalog

      // Node is one artifact in the Requires* dependency graph (ADR-0081).
      // Docs are pure sinks: DocEntry declares no requirements.
      type Node struct {
      	Kind string // "skill", "agent", or "doc"
      	Name string
      }

      // RequiresOf enumerates n's direct requirement edges declared in cat — the
      // single source of edge truth (ADR-0081 Decision 1). An unknown name yields
      // a zero-value spec and therefore no edges: project-local artifacts
      // (ADR-0068) are leaves.
      func RequiresOf(cat *Catalog, n Node) []Node {
      	var out []Node
      	switch n.Kind {
      	case "skill":
      		spec := cat.Skills[n.Name]
      		for _, s := range spec.RequiresSkills {
      			out = append(out, Node{Kind: "skill", Name: s})
      		}
      		if spec.RequiresAgent != "" {
      			out = append(out, Node{Kind: "agent", Name: spec.RequiresAgent})
      		}
      		if spec.RequiresDoc != "" {
      			out = append(out, Node{Kind: "doc", Name: spec.RequiresDoc})
      		}
      	case "agent":
      		for _, s := range cat.Agents[n.Name].RequiresSkills {
      			out = append(out, Node{Kind: "skill", Name: s})
      		}
      	}
      	return out
      }
      ```

      (`Closure` deliberately does NOT land here — its first production consumer is
      Phase 4's scaffold; landing it earlier fails the dead-code gate, since the
      resolver's provenance-carrying walks do not call it.)

- [ ] In `internal/project/validate.go`, replace the ADR-0050 pairing block inside
      `checkKindAgainstCatalog` — the four lines

      ```go
      		if d.Plural == "skills" {
      			if req := p.Cat.Skills[name].RequiresAgent; req != "" && !slices.Contains(p.Cfg.Agents, req) {
      				return fmt.Errorf("skill %q requires agent %q; enable the agent or disable the skill", name, req)
      			}
      		}
      ```

      and its preceding ADR-0050 comment (keep the marker line
      `// invariant: reviewing-skill-agent-pairing` — it re-anchors here) with:

      ```go
      		// Closure validation (ADR-0081): every enabled, non-local artifact's
      		// direct catalog requirements are enabled — transitive closure follows
      		// by induction. Generalizes the ADR-0050 RequiresAgent pairing (that
      		// edge is now one case of the same loop); a silently-thinner chain is
      		// the failure mode the workflow exists to prevent.
      		// invariant: reviewing-skill-agent-pairing
      		// invariant: enabled-set-closed
      		if d.Plural == "skills" || d.Plural == "agents" {
      			if err := p.checkNodeRequirements(catalog.Node{Kind: d.Singular, Name: name}); err != nil {
      				return err
      			}
      		}
      ```

      and add below `SkillsRequiringAgent`:

      ```go
      // checkNodeRequirements fails when any of n's direct catalog requirements is
      // not enabled, with a repair hint naming the exact edit and awf upgrade as
      // the pre-migration recovery path (ADR-0081 Decision 3).
      func (p *Project) checkNodeRequirements(n catalog.Node) error {
      	for _, r := range catalog.RequiresOf(p.Cat, n) {
      		if !p.nodeEnabled(r) {
      			return fmt.Errorf("%s %q requires %s %q; add it to %s: in .awf/config.yaml (or run `awf upgrade` after a binary upgrade), or remove the %s",
      				n.Kind, n.Name, r.Kind, r.Name, r.Kind+"s", n.Kind)
      		}
      	}
      	return nil
      }

      // nodeEnabled reports whether n appears in its kind's config enable array.
      func (p *Project) nodeEnabled(n catalog.Node) bool {
      	switch n.Kind {
      	case "skill":
      		return slices.Contains(p.Cfg.Skills, n.Name)
      	case "agent":
      		return slices.Contains(p.Cfg.Agents, n.Name)
      	case "doc":
      		return slices.Contains(p.Cfg.Docs, n.Name)
      	}
      	return false
      }
      ```

      Add `"github.com/hypnotox/agentic-workflows/internal/catalog"` to the imports.
      Note `checkKindAgainstCatalog` already `continue`s on `sc.Local` before this
      point — local artifacts stay exempt.

- [ ] Repair the fixtures the new validation refuses (audit: `grep -rn "skills:"
      --include="*_test.go" internal/ cmd/ | grep -v "map\["` and fix every fixture
      enabling a non-leaf skill without its closure). Known today:
      - `internal/project/skillrefs_test.go:100` — `TestEffectiveSkillsMembership`,
        the behavioral anchor for `skills-context-effective-set` (suppression
        membership; `brainstorming` there carries a `local: true` sidecar). Its
        suppression coverage must SURVIVE until Phase 6, so do not rework it onto
        dead references: add `docs: [roadmap]` to the fixture so `Open` passes the
        new validation, then drive `roadmap-graduation`'s suppression via the same
        post-`Open` mutation pattern the test already uses (drop `roadmap` from
        `p.Cfg.Docs` after `Open`) and keep the membership assertions unchanged.
      - Expected-refusal pairing tests update rather than rework:
        `TestOpenRejectsPairedSkillWithoutAgent` (`internal/project/project_test.go`,
        fixture `skills: [reviewing-impl]`) pins the old ADR-0050 error string —
        update its want to the `checkNodeRequirements` message (note
        `reviewing-impl`'s first failing edge is now a `RequiresSkills` edge, not
        the agent) or merge it into `TestOpenRefusesUnclosedEnabledSet`;
        previously-clean pairing fixtures (`TestOpenAllowsPairedSkillWithAgent`)
        gain their skill closure.
      - Any other fixture surfaced by the audit: leaves stay as-is; a chain skill
        gains its closure; a dead-ref scenario that validation forecloses moves to
        a part-introduced reference (ADR-0046's check remains the oracle for part-
        and local-sourced references).
- [ ] Add to `internal/catalog/graph_test.go` a unit test for `RequiresOf` over
      `Standard` (skill with all three edge kinds — none exists today, so assert
      `reviewing-plan` yields exactly `[{skill reviewing-plan-resync} {skill writing-plans} {agent plan-reviewer}]`,
      `roadmap-graduation` yields `[{doc roadmap}]`, `plan-reviewer` yields
      `[{skill reviewing-plan-resync}]`, `adr-lifecycle` yields `nil`, and an
      unknown local name yields `nil`).
- [ ] Add to `internal/project/validate_test.go` (or the file holding the ADR-0050
      pairing tests): `TestOpenRefusesUnclosedEnabledSet` — fixtures
      `skills: [brainstorming]` (missing skill requirement),
      `skills: [roadmap-graduation]` (missing doc requirement), and
      `agents: [plan-reviewer]` + `skills: []` (agent's skill requirement) each
      fail `Open` with the requiring artifact, the missing node, and the string
      `awf upgrade` in the error; a `local: true` sidecar on the same name opens
      clean.
- [ ] Run `./x gate` — green. Commit:
      `feat(config): validate the enabled set closed at open (ADR-0081)` — body
      names Decision 3, the 0050 generalization, and the fixture-repair rule.

## Phase 2 — resolver, graph-aware add/remove CLI

- [ ] Create `internal/project/resolve.go`:

      ```go
      package project

      import (
      	"slices"

      	"github.com/hypnotox/agentic-workflows/internal/catalog"
      )

      // PlanOp is one enable-array change in a resolver plan (ADR-0081 Decision 2).
      // RequiredBy carries provenance: the artifact demanding the op ("" for the
      // node the user named).
      type PlanOp struct {
      	Node       catalog.Node
      	Add        bool
      	RequiredBy string
      }

      // ResolveAdd plans enabling (kind, name): the node plus its missing forward
      // closure. An already-enabled dependency is skipped along with its subtree —
      // the open-time validation invariant guarantees enabled implies closed.
      // invariant: add-applies-closure-plan
      func (p *Project) ResolveAdd(kind, name string) []PlanOp {
      	type item struct {
      		n  catalog.Node
      		by string
      	}
      	seed := catalog.Node{Kind: kind, Name: name}
      	seen := map[catalog.Node]bool{seed: true}
      	queue := []item{{seed, ""}}
      	var plan []PlanOp
      	for len(queue) > 0 {
      		it := queue[0]
      		queue = queue[1:]
      		if it.n != seed && p.nodeEnabled(it.n) {
      			continue
      		}
      		plan = append(plan, PlanOp{Node: it.n, Add: true, RequiredBy: it.by})
      		for _, r := range catalog.RequiresOf(p.Cat, it.n) {
      			if !seen[r] {
      				seen[r] = true
      				queue = append(queue, item{r, it.n.Name})
      			}
      		}
      	}
      	return plan
      }

      // ResolveRemove plans disabling (kind, name): the node plus every enabled,
      // non-local artifact that transitively requires it (reverse closure, fixed
      // point over direct edges). Local-sidecar artifacts have no catalog edges
      // demanded of them, mirroring the validator's skip.
      // invariant: remove-refuses-dependents
      func (p *Project) ResolveRemove(kind, name string) []PlanOp {
      	target := catalog.Node{Kind: kind, Name: name}
      	removed := map[catalog.Node]bool{target: true}
      	plan := []PlanOp{{Node: target, Add: false}}
      	for changed := true; changed; {
      		changed = false
      		for _, n := range p.enabledGraphNodes() {
      			if removed[n] {
      				continue
      			}
      			for _, r := range catalog.RequiresOf(p.Cat, n) {
      				if removed[r] {
      					removed[n] = true
      					plan = append(plan, PlanOp{Node: n, Add: false, RequiredBy: r.Name})
      					changed = true
      					break
      				}
      			}
      		}
      	}
      	return plan
      }

      // enabledGraphNodes returns the enabled skills and agents that carry catalog
      // edges — non-local only, mirroring validate's skip. Docs are sinks and
      // never depend on anything.
      func (p *Project) enabledGraphNodes() []catalog.Node {
      	var out []catalog.Node
      	for _, name := range p.Cfg.Skills {
      		if sc, err := p.Cfg.Sidecar("skills", name); err == nil && !sc.Local {
      			out = append(out, catalog.Node{Kind: "skill", Name: name})
      		}
      	}
      	for _, name := range p.Cfg.Agents {
      		if sc, err := p.Cfg.Sidecar("agents", name); err == nil && !sc.Local {
      			out = append(out, catalog.Node{Kind: "agent", Name: name})
      		}
      	}
      	return out
      }
      ```

      Delete `SkillsRequiringAgent` from `validate.go` (its only caller was
      `runRemove`'s agent guard, replaced below; move its
      `// invariant: remove-agent-pairing-guard` marker onto the new refusal site
      in `list_add.go`). Keep `slices` imports tidy.

- [ ] In `cmd/awf/main.go`:
      - Add a positional extractor beside `checkArgs`:

        ```go
        // positionals returns rest's non-flag tokens, skipping each valueFlag's
        // consumed value — the flag-tolerant arity source for add/remove.
        func positionals(rest []string, boolFlags, valueFlags []string) []string {
        	var out []string
        	for i := 0; i < len(rest); i++ {
        		a := rest[i]
        		switch {
        		case slices.Contains(valueFlags, a):
        			i++
        		case slices.Contains(boolFlags, a):
        		case strings.HasPrefix(a, "-"):
        		default:
        			out = append(out, a)
        		}
        	}
        	return out
        }
        ```

      - `argSpecs["add"]` gains `boolFlags: []string{"--dry-run"}`;
        `argSpecs["remove"]` gains `boolFlags: []string{"--with-dependents", "--dry-run"}`.
      - Rework both switch cases on `pos := positionals(args[2:], spec.boolFlags, spec.valueFlags)`
        (fetch `spec := argSpecs[args[1]]` once): the `len(args) == 4` /
        `len(args) == 3` conditions become `len(pos) == 2` / `len(pos) == 1`, the
        singleton check reads `pos[0]`, and the calls become
        `runAdd(cwd, pos[0], pos[1], hasFlag(args, "--dry-run"), stdout)` and
        `runRemove(cwd, pos[0], pos[1], hasFlag(args, "--with-dependents"), hasFlag(args, "--dry-run"), stdout)`
        (singleton forms pass `false` flags). One uniform rule for every
        non-graph path — `domain`, `target`, `bootstrap`, `hooks`: a graph-only
        flag (`--dry-run`, `--with-dependents`) on them returns a usage error
        naming the graph kinds (`skill`, `agent`, `doc`), so no flag is ever
        silently ignored.
      - Help text: `add`'s usage becomes `awf add <kind> <name> [--dry-run]` with a
        Flags block (`--dry-run    print the closure plan without changing the config`);
        `remove`'s becomes `awf remove <kind> <name> [--with-dependents] [--dry-run]`
        with both flags described (`--with-dependents    also remove every enabled
        artifact that transitively requires <name>`).

- [ ] In `cmd/awf/list_add.go`, rewire `runAdd(root, kind, name string, dryRun bool, stdout io.Writer)`:
      - After the existing catalog-pool and already-enabled guards, for
        catalog-backed kinds (`skill`, `agent`, `doc`) compute
        `plan := p.ResolveAdd(kind, name)` and print each op:
        `fmt.Fprintf(stdout, "plan: + %s %s%s\n", op.Node.Kind, op.Node.Name, requiredBySuffix(op))`
        where `requiredBySuffix` renders `" (required by <name>)"` when
        `op.RequiredBy != ""`. On `dryRun`, return after printing (no write, no
        sync). Otherwise build `edits` from the plan (`enableEdit{key:
        pluralOf(op.Node.Kind), name: op.Node.Name}` — add a tiny `pluralOf` via
        `project.PluralKind`) and pass them all to the one `rewriteConfig` call.
        This replaces the ADR-0050 `pairedAgent` block (move its
        `// invariant: add-skill-pairs-agent` marker onto the plan-building block)
        and the ADR-0013 RequiresDoc advisory note (delete it — the doc is now a
        plan op). `domain` keeps its existing bespoke path (not a graph kind; the
        uniform graph-only-flag usage error above covers it).
      - Rewire `runRemove(root, kind, name string, withDependents, dryRun bool, stdout io.Writer)`:
        replace the agent-pairing guard with, for catalog-backed kinds,
        `plan := p.ResolveRemove(kind, name)`; print every op
        (`"plan: - %s %s%s"`). If `len(plan) > 1 && !withDependents`, return
        `fmt.Errorf("removing %s %q also removes the %d artifacts above; re-run with --with-dependents to apply", kind, name, len(plan)-1)`
        — carry `// invariant: remove-agent-pairing-guard` here. On `dryRun`,
        return after printing. Otherwise apply all ops via `rewriteConfig`, loop
        the `hasSidecarOrParts` orphan note over every removed node, and after a
        cascade print for each still-enabled agent with no requiring skill:
        `note: agent %q is no longer required by any enabled skill; it stays enabled (remove it separately if unwanted)`.
- [ ] Add a glossary entry for **plan op** (one line: a single enable-array
      change in a resolver plan, carrying add/remove direction and required-by
      provenance) to `.awf/docs/parts/glossary/terms.md`, alphabetically placed.
- [ ] Update the prose this behavior obsoletes — same commit per the ADR's
      Consequences ("Prose this ADR obsoletes updates in the same commits that
      change the behavior"):
      - `templates/agents-doc/AGENTS.md.tmpl` (awf-setup section): rewrite "The
        workflow-chain skills reference one another by name, so disable them as
        a unit rather than piecemeal, or a handoff will point at a skill that
        isn't enabled." to "The workflow-chain skills reference one another by
        name; `awf add` enables a skill's full requirement closure and
        `awf remove` refuses while enabled skills still require the target
        (`--with-dependents` removes the unit together)."
      - `templates/docs/working-with-awf.md.tmpl` (commands section): extend the
        add/remove descriptions with the closure semantics and the
        `--with-dependents` / `--dry-run` forms (one sentence each, matching the
        help text).
      - Run `./x sync` — this repo's rendered AGENTS.md/CLAUDE.md/
        working-with-awf/glossary refresh lands in this commit (adopter-facing
        template change, covered by the Phase-3 Breaking changelog entry).
- [ ] Tests:
      - `internal/project/resolve_test.go`: seed-dependent cascade sizes pinned to
        the ADR's verified numbers over a full-chain fixture — remove
        `brainstorming` → plan length 1; remove `reviewing-plan` (planning core) →
        7 ops (6 skills + plan-reviewer); remove `executing-plans` (execution
        core) → 10 ops (9 skills + plan-reviewer); remove `retrospective` → 11 ops
        (10 skills + plan-reviewer). Add-plan test on an empty-skills fixture:
        `ResolveAdd("skill", "brainstorming")` enables 11 skills + 3 agents worth
        of ops. Local-sidecar skill is never pulled into a remove plan.
      - `cmd/awf/list_add_test.go`: rework the ADR-0050 pairing tests to the plan
        output (`plan: + agent plan-reviewer (required by reviewing-plan)`);
        e2e: `awf add skill brainstorming` on an init'd-empty project writes one
        config rewrite containing the full closure; `awf remove skill
        executing-plans` refuses with the 10-op plan and exit 1;
        `--with-dependents` applies it; `--dry-run` leaves config bytes identical
        (read before/after); `awf remove doc roadmap` refuses while
        roadmap-graduation is enabled.
- [ ] Run `./x gate` — green. Commit:
      `feat(awf): graph-aware add/remove with plan output (ADR-0081)` — body names
      Decisions 2/4/5/6 and the re-anchored 0050 invariants.

## Phase 3 — schema-8 close-enabled-set migration

- [ ] Thread an output writer through migrations: `Migration.Apply` becomes
      `func(root string, out io.Writer) error`; update the seven existing `apply*`
      funcs to the new signature (each ignores `out`), `Upgrade(root)` becomes
      `Upgrade(root string, out io.Writer) ([]string, error)` (return values
      unchanged), and every call site follows — `runUpgrade` passes its `stdout`,
      and each `Upgrade(root)` call in `internal/migrate/migrate_test.go` gains
      an `io.Discard` (or a buffer where output is asserted). Reconcile
      `internal/migrate/migrate.go`'s package doc comment: it gains its first
      `internal/catalog` import but stays off the render/sync/check load path.
- [ ] Extend the pinned applied-migration lists (the docs/pitfalls.md
      registry-drift pitfall): `TestUpgradeAppliesInOrderIdempotent`
      (`internal/migrate/migrate_test.go:168`) gains `close-enabled-set` in both
      its want-list and its `t.Errorf` message, and `TestUpgradeStampsTreeLock`'s
      expectations follow; audit the file for any other exact-list assertion.
- [ ] Create `internal/migrate/closeenabledset.go` — registry entry
      `{To: 8, Name: "close-enabled-set", Apply: applyCloseEnabledSet}`:
      two ordered steps per ADR-0081 Decision 8, computed from `config.Load(root)`
      + `catalog.Standard` (mirroring the validator's `local:` sidecar skip via
      `cfg.Sidecar`), applied through `editConfig` with `config.SetArrayMember`:
      1. drop every enabled non-`local` doc-gated skill whose doc is disabled
         (`fmt.Fprintf(out, "close-enabled-set: dropped dormant skill %q (its %q doc is disabled)\n", ...)`);
      2. additive fixed point over `catalog.RequiresOf` for every remaining
         enabled skill/agent, adding missing skills, agents, and docs
         (`"close-enabled-set: enabled %s %q (required by %q)"`).
      A missing config file is a no-op (the `editConfig` skeleton).
      `// invariant: close-enabled-set-migration` on `applyCloseEnabledSet`.
- [ ] In `internal/project/project.go`: `Version` → `"0.12.0"`;
      `minVersionBySchema` gains `8: "0.12.0"`.
- [ ] `changelog/CHANGELOG.md` under `## [Unreleased]`, new `### Breaking changes`
      group (above Others):

      ```markdown
      ### Breaking changes
      - The catalog `requires*` declarations are now an enforced dependency graph
        (schema 8 — run `awf upgrade`). A config enabling an artifact without its
        required skills/agents/docs is refused by every command; the migration
        closes your enabled set (adding missing requirements, printing each) and
        drops dormant doc-gated skills (enabled while their doc was disabled —
        they rendered nothing before, so your output is unchanged). `awf add`
        now enables the full requirement closure in one edit, printing a plan;
        `awf remove` refuses while enabled artifacts still require the target —
        `--with-dependents` removes them together, `--dry-run` previews either
        plan. The render-time suppression of doc-gated skills is gone: enabled
        now always means rendered.
      ```

- [ ] `internal/migrate/closeenabledset_test.go`: dormant skill dropped (and its
      output line printed); dormant-but-demanded skill re-added with its doc
      (synthetic catalog or the roadmap-graduation + a hand-built requiring
      config); missing chain siblings added to a fixed point; `local:`-sidecar
      dormant skill kept; re-run is a byte-identical no-op (idempotence); e2e in
      `cmd/awf`: a schema-7 config that Phase 1 refuses at open passes
      `awf upgrade` then opens clean.
- [ ] Add a glossary entry for **dormant doc-gated skill** (one line: a skill
      enabled while its required doc is disabled — pre-schema-8 it silently
      rendered nothing; the close-enabled-set migration drops it) to
      `.awf/docs/parts/glossary/terms.md`, alphabetically placed; `./x sync`
      refreshes `docs/glossary.md` in this commit.
- [ ] Run `./x gate` — green. Commit:
      `feat(config): close the enabled set in a schema-8 migration (ADR-0081)`.

## Phase 4 — init: derive agents from the trim, close the selection

- [ ] In `internal/project/scaffold.go`, replace the unconditional
      `agentNames := slices.Sorted(maps.Keys(cat.Agents))` with: when
      `trim != nil && trim.Skills != nil`, run `catalog.Closure` over the trimmed
      skills (as skill Nodes), partition the result — the closure's skills become
      `skillNames` (closure-completing the trim, ADR-0081 Decision 9), its agents
      become `agentNames`, its docs merge into `docNames`; when `trim` is nil
      (curated default), keep all agents exactly as today (a default, not a
      derived set). `catalog.Closure` and its tests land HERE (this is its first
      production consumer — dead-code gate): append the func to
      `internal/catalog/graph.go`:

      ```go
      // Closure returns the forward closure of seeds under RequiresOf, seeds
      // included, breadth-first with edges in declaration order (deterministic).
      func Closure(cat *Catalog, seeds []Node) []Node {
      	seen := map[Node]bool{}
      	var out []Node
      	queue := append([]Node(nil), seeds...)
      	for len(queue) > 0 {
      		n := queue[0]
      		queue = queue[1:]
      		if seen[n] {
      			continue
      		}
      		seen[n] = true
      		out = append(out, n)
      		queue = append(queue, RequiresOf(cat, n)...)
      	}
      	return out
      }
      ```

      with `internal/catalog/graph_test.go` gaining `TestClosureIsCycleSafe` (a
      synthetic two-node mutually-requiring catalog: terminates, returns both,
      seeds first) and `TestClosureChainUnit` (closure of `Standard`'s Chain
      seeds = exactly 11 skills + 3 agents, asserting the sorted lists verbatim:
      skills adr-lifecycle, brainstorming, executing-plans, proposing-adr,
      retrospective, reviewing-adr, reviewing-impl, reviewing-plan,
      reviewing-plan-resync, subagent-driven-development, writing-plans; agents
      adr-reviewer, code-reviewer, plan-reviewer); and `chainClosureConfig`
      (`internal/project/drift_test.go`) switches to building its lists from
      `catalog.Closure` (drop the inline recursive walk; keep the sorted-YAML
      assembly).
      For the addition notes: `ScaffoldConfig` additionally returns the
      closure-added names (`(content []byte, added []string, err error)` — names
      beyond the user's trim selection, kind-prefixed like `skill
      reviewing-plan-resync`), and `runInit` (`cmd/awf/init.go:79`) prints one
      `note: also enabled <kind> <name> (required by your selection)` line per
      entry. The signature change also updates the collision-probe call at
      `cmd/awf/init.go:150` (`ScaffoldConfig(filepath.Base(root), nil, nil, nil)`)
      and the five direct calls in `internal/project/scaffold_test.go`
      (:23, :68, :110, :133, :152), each discarding `added` (a nil trim yields
      no additions).
      Update the `invariant: catalog-trim-applied` comment prose to say the trim
      is closure-completed, and add `// invariant: init-set-closed` on the closure
      block.
- [ ] Tests (in the existing scaffold/init test files): the curated default
      satisfies closure (walk `catalog.RequiresOf` over the scaffolded arrays —
      this locks `init-set-closed` for the default); an `--answers` trim
      deselecting `reviewing-plan-resync` comes back closure-completed WITH
      `plan-reviewer` retained; a trim keeping only leaves (`tdd`) scaffolds
      exactly `tdd` + zero agents; a trim selecting `roadmap-graduation` gains the
      `roadmap` doc.
- [ ] Run `./x gate` — green. Commit:
      `feat(awf): close the init selection and derive its agents (ADR-0081)`.

## Phase 5 — full-suite fixture sweep

- [ ] Run `go test ./...` and `./x gate full`; repair any remaining fixture the
      closure validation or the new CLI signatures broke that Phases 1–4 did not
      already touch — the arity/usage assertions live in `cmd/awf/run_test.go`
      and `cmd/awf/list_add_test.go`, and the `awf init` e2e tests cover the new
      `ScaffoldConfig` returns. The repair rule from Phase 1 applies: leaves stay
      minimal, chain fixtures derive via `chainClosureConfig`/`catalog.Closure`,
      dead-ref scenarios use part-introduced references.
- [ ] Run `./x gate` — green. Commit:
      `test(awf): sweep fixtures for closure validation and flags (ADR-0081)`
      (skip the commit if Phases 1–4 left nothing — fold any single-file fix into
      the nearest phase instead).

## Phase 6 — suppression removal, docs, flip

- [ ] In `internal/project/render.go`: delete `skillDocGateOpen` and simplify
      `effectiveSkills` — the loop keeps the sidecar read and sets
      `eff[name] = true` unconditionally (locals included); rewrite its doc
      comment: "the enabled set — closure validation (ADR-0081) makes enabled
      mean rendered; local-declared names are hand-maintained but present" and
      keep the `// invariant: skills-context-effective-set` marker (amended
      semantics per ADR-0081 Decision 7). Delete the suppression gate at the
      render site (the `skillDocGateOpen` caller at render.go:285) together with
      its `// invariant: doc-gated-skill-suppressed` marker — the retirement
      lands with the flip in this same commit.
- [ ] Rewrite `TestEffectiveSkillsMembership`
      (`internal/project/skillrefs_test.go`) as the behavioral anchor for the
      amended invariant: effective = enabled — the doc-gated skill stays a member
      with its doc enabled or (post-`Open` mutation) disabled, and the
      `local: true` skill stays a member; drop the suppression-membership
      assertions Phase 1 preserved.
- [ ] `.awf/agents-doc.yaml` `data.invariants` gains five one-per-slug bullets
      (8-space indent before `- ref:`):

      ```yaml
              - ref: ADR-0081
                text: '**Enabled set closed.** Every enabled, non-`local` artifact''s direct catalog requirements (`requiresSkills`, `requiresAgent`, `requiresDoc`) are enabled; a violation fails every gated command at project open with a repair hint.'
              - ref: ADR-0081
                text: '**Add applies the closure plan.** `awf add` enables the requested artifact''s full missing forward closure in a single config rewrite, printing one provenance line per plan op; `--dry-run` prints without applying.'
              - ref: ADR-0081
                text: '**Remove refuses dependents.** Without `--with-dependents`, `awf remove` refuses while enabled transitive dependents exist, printing the dependent plan; with it, the full reverse closure lands in a single rewrite.'
              - ref: ADR-0081
                text: '**Close-enabled-set migration.** The schema-8 migration closes the enabled set additively for skill, agent, and doc requirements, drops dormant non-`local` doc-gated skills, and is idempotent and atomic.'
              - ref: ADR-0081
                text: '**Init set closed.** `awf init`''s scaffolded enabled set — curated default or closure-completed trim (agents derived from the trimmed skills) — satisfies the closure invariant.'
      ```

- [ ] Update the three domain current-state parts (append one sentence each,
      staged from `git status` after sync — every `domains:` entry implies its
      rendered doc, per the pitfalls note):
      - `.awf/domains/parts/config/current-state.md`: the enabled set is
        closure-validated at open (ADR-0081); schema 8's migration closes it.
      - `.awf/domains/parts/rendering/current-state.md`: enabled now means
        rendered — the ADR-0013 doc-gate suppression is gone (ADR-0081).
      - `.awf/domains/parts/tooling/current-state.md`: `awf add`/`remove` are
        graph-aware with plan output, `--with-dependents`, `--dry-run` (ADR-0081).
- [ ] Add `81` to the `related:` frontmatter arrays of
      `docs/decisions/0013-*.md`, `docs/decisions/0046-*.md`, and
      `docs/decisions/0050-*.md` (partial-amendment forward pointers).
- [ ] Flip `docs/decisions/0081-*.md` frontmatter `status: Proposed` →
      `status: Implemented`.
- [ ] Run `./x sync`, `./x check` (all five new slugs backed; the retired slug
      accepted via `retires_invariants`), `./x gate` — green. Stage from
      `git status`. Commit: `docs(adr): implement 0081 enforced dependency graph`
      — body summarizes the guards, the retirement, and the flip obligations.

## Execution notes

- Phase order is load-bearing: P1 before P2 (validation is the induction premise
  the resolver's skip-enabled-subtree optimization relies on); P7 last (marker
  retirement + flip must coincide); every exported func lands with its production
  consumer in the same commit (dead-code gate).
- Never hand-edit rendered files — change templates or `.awf/` parts and `./x sync`.
- `./x audit-local` runs at impl review; the Phase-3 Breaking changelog entry
  covers the adopter-facing `internal/catalog`/`internal/config`/`templates/`
  touches; Phase 2's template edits are also covered by it.
- If any pinned cascade count or closure list mismatches at execution time, stop:
  the catalog changed since 2026-07-09 — re-derive the numbers from
  `catalog.Standard` and amend the still-`Proposed` ADR if its examples drifted;
  never weaken an assertion to pass.
