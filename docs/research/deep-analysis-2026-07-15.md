# awf - Deep Analysis (2026-07-15)

*A whole-project review: code quality, architecture, the guardrail system, the
rendered output, proportionality/adopter experience, and standing versus the
field. Companion to
[agentic-workflow-landscape-and-awf-standing-2026-07.md](agentic-workflow-landscape-and-awf-standing-2026-07.md),
which remains the dedicated landscape report; this document is the broader
project audit and cross-checks that report's conclusions as of this date.*

Produced from a five-lens parallel review (code/architecture, guardrails,
rendered output, adopter/proportionality, field). The author's dispositions on
the recommendations (accepted, deferred, reframed) are recorded inline in the
Actionable section so this artifact stays honest about what was acted on.

---

## Framing

The single fact that reframes the whole assessment: **the repo is ~3 weeks old.**
Every one of its 1,389 commits and 111 ADRs dates from 2026-06-24 onward:
roughly 46 commits and 5 ADRs *per day*, sustained. This is not a human-paced
project that over-documented; it is an AI-agent-driven project whose elaborate
guardrails are load-bearing for its own development method. Read charitably, the
machinery is what lets probabilistic agents produce coherent, non-drifting output
at this velocity: the thesis, dogfooded. Read critically, that same velocity
lets the project accrete faster than anyone can prune, and that shows at the
edges.

---

## The central finding (and its correction)

awf is often described as "**deterministic checks wrap the probabilistic
agent.**" The precise, corrected statement (confirmed by the project author as
the *intended* scope) is that awf's checks wrap **agent output**, not agent
behaviour. That distinction matters and is not a defect:

- awf does not (and deliberately will not) run live-agent golden-transcript
  evals. Wrapping the agent's *execution* is explicitly out of scope on cost
  grounds for a tool that serves the author's own repositories. The workflow
  chain is intended to be *complete* as a set of output guardrails; behavioural
  evaluation is a different, deferred concern.
- The one thing to fix here is therefore **positioning, not engineering**: prose
  that says checks "wrap the agent" over-claims. Tightening it to "guardrails on
  agent **output**" makes the documentation match the real and correct design.

The related, genuinely actionable observation concerns **invariant "backing."** A
backed `` `invariant: <slug>` `` is satisfied by any line matching
`// invariant: <slug>` in any test file: string-prefix matching with zero
awareness of whether the marker sits inside a test function or relates to any
assertion (`internal/invariants/invariants.go:392-411`). The 100% coverage gate
forces the test file to *execute*, but a trivial test satisfies that. So the
invariant system is a rigorous **ledger** (ADR ↔ marker bookkeeping is tightly
policed) but a weak **proof** (nothing binds a marker to a real assertion).

Crucially, the fix must respect awf's **language-agnosticism**: proof markers
live in the *adopter's* test files, which may be any language. A Go-AST binding
would lock the shipped mechanism to Go projects and is therefore rejected. The
agnostic path is in the Actionable section.

---

## What's genuinely good

**Architecture & code.** Clean acyclic layering: a real DAG with
`internal/project` as a deliberate composition root, not accidental coupling.
`internal/render` is exemplary: ~546 LoC of small pure functions, NUL-delimited
brace-free sentinels so verbatim parts can never be parsed as templates, and a
`CommentStyle` abstraction so the pointer-emitter and read-back matcher cannot
diverge. Tests are genuinely behavioural and stdlib-only (zero testify across 92
files), asserting real outcomes and rendered bytes. "Fails-when-stale" exemption
maps (e.g. `nonHandoffRequires`, the `requiresSkills` sweep) force stale
exemptions to break the build rather than rot silently. Zero `TODO`/`FIXME`/
`panic` in production.

**Guardrail mechanics.** The mechanical layer is strong engineering with few
escape hatches. The **hermetic staged-slice pre-commit** (`git checkout-index`
into a throwaway dir, then build + `awf check`) validates the exact staged slice
(catching partial staging a worktree check structurally cannot) and is the
single best guardrail in the system. Drift detection is near-complete
(bidirectional hash compare, regeneration-with-read-back for generated indexes
*and* in-place sections, closed-config-tree sweep, dead-link/dead-skill scans).
Trust-bearing writes are atomic temp-file-plus-rename with a single corrupt-lock
choke point that refuses *before* any write. The `coverage-ignore` self-policing
audit rule is a nice touch.

**Rendered output.** The skill chain is a model of the SKILL.md standard:
trigger-first disambiguating descriptions, right altitude, explicit
machine-enforced handoffs. The "Rationalization | Reality" red-flag tables are
best-in-class anti-rationalization. The three review agents genuinely encourage
critique over agreement, with real adversarial lens diversity
(`alternatives-honesty`, `consequences-honesty` hunt upside-only ADRs). Templates
degrade publication-safe, and awf has written down the right doctrine in
`docs/doc-standard.md`.

**Adopter surface.** The strongest counter to the "it's intimidating" worry:
`examples/sundial/AGENTS.md` is ~99 lines and inherits **none** of awf's 100+
invariants: an adopter gets three generic rules plus their own. The core value
proposition ("treat the agent workflow as a committed, drift-checked build
artifact") is sound and well-scoped. And the project demonstrably self-corrects:
ADR-0107 downgraded a rule to advisory, ADR-0108 trimmed a taxonomy, 16 ADRs
retire invariants: the machinery is not purely additive.

**Field standing.** The two-week window since the landscape report moved *toward*
awf. The ADR-durable / plan-disposable split is now a cited field position.
Single-threaded-writes + isolated read-only subagents + fresh-context verifier is
now explicit industry synthesis. The CLAUDE.md-bridge is confirmed still
necessary. And **`awf audit` (process-conformance from git history) still has no
direct competitor.**

---

## What's missing

1. **Semantic invariant proof**: nothing binds an invariant marker to a real
   assertion (see central finding). To be closed *agnostically* (Actionable T1).
2. **Automated mutation testing.** `cmd/mutants` is built and careful but
   manual-only, absent from CI and never in `./x gate`. The one tool that would
   catch "100% coverage with trivial assertions" is not forced to run. It was
   deferred for brittleness; a usability path is proposed below (Actionable T1).
3. **A content-accuracy drift axis (config vs *reality*).** awf verifies rendered
   == config; it does not verify config content == the actual codebase (stale
   paths, dead commands in the rendered guide). `domain-code-staleness` and
   `context --uncovered` are partially here but advisory. A nascent
   "agent-instruction linter" category in the wild now owns exactly this axis.
4. **Invariant tiering / eviction.** The AGENTS.md invariant list grows one entry
   per Implemented ADR, unbounded and flat: no "core an agent must hold" subset
   versus "gate-enforced mechanism, one hop away," and no demotion path once an
   ADR is Implemented.
5. **In-code architecture orientation & sentinel errors** (minor): package docs
   are one-liners, and there are no `errors.Is` targets; tests string-match
   messages (defensible for a CLI, but couples tests to wording).

> Explicitly **not** missing, by design: a live-agent behavioural eval /
> golden-transcript harness. Deferred on cost; out of scope for a
> personal-repos tool.

---

## What's bad / risks

1. **AGENTS.md violates awf's own documentation standard.** 37.8 KB; the
   Invariants section is ~50 dense multi-clause paragraphs where its own
   `agents-md-standard.md` mandates "one terse imperative line each." This is the
   file loaded *every session*, and most of its byte cost is a mechanism catalog
   the agent rarely needs in working memory: the textbook context-rot failure,
   self-inflicted. The `code-reviewer` prompt shows the same accumulation one
   layer down (dated incident lore) with no eviction policy.
2. **Scope creep at the periphery: `awf context` is effectively a second
   product.** Ten ADRs (0092, 0098, 0099, 0102-0106, 0109, 0110) build a
   relevance/RAG-lite engine with three-tier ranking, a governed tag vocabulary
   with frequency/coverage backstops, an uncovered-coverage floor, and a
   context-ignore list, grafted onto a template renderer. **ADR-0110 is the
   baroque high-water mark:** a full ADR whose goal is to make awf's own
   `--uncovered` report reach zero on awf's own tree, folding internal Go
   packages into "domains" with folds the ADR itself calls "a deliberate stretch,"
   to protect another hand-maintained convention. Guardrails for guardrails.
3. **The `project` god-package.** 4,953 LoC, **81 methods on `*Project`**,
   importing 15 packages. Split by file (good mitigation) but `check.go` alone is
   **955 lines / 28 methods**, the single hardest file for a newcomer.
   `SyncReport` is 182 lines interleaving load/render/validate/backup/write/prune.
4. **194 `coverage-ignore` markers**: the "100% coverage" is really "100% minus
   194 hand-excluded branches." Honestly annotated, low abuse, but several
   (`os.Stat`/`Chmod`/`copyFile` faults) are testable with an injected FS, and
   the count drives visible ceremony (functions contorted so one line-based
   ignore covers them). Coverage without automated mutation testing is a liveness
   metric wearing a verification costume.
5. **The ADR bar has drifted low.** ~5 ADRs/day, ~174k words across 111 ADRs
   (≈ two novels) for a 40k-LoC tool, a parallel corpus every change must keep
   consistent. The dense ADR-cross-referencing comments in the code are the same
   tax one layer down: load-bearing *and* a liability that nothing verifies stays
   accurate.
6. **Bus factor is the dominant structural risk.** One person plus AI agents at
   this volume; coherence depends on the chain being followed faithfully by
   agents indefinitely. No second human has read all 111 ADRs. If the author
   steps away, the realistic outcome is "it freezes": onboarding a human means
   digesting 174k words. (Scoped by the author to personal repos, which bounds
   the blast radius but not the freeze risk for the artifact itself.)
7. **Render is now commoditized (field).** Multi-adapter source→many-format
   rendering is a crowded 2026 category, and the Claude Code plugin marketplace is
   production-mature. This obviates awf's *distribution* story (not its
   generation/governance story); the risk is mindshare, not capability.

> Confidence note on the field scan: competitor *capabilities* were verified via
> direct fetches, but specific tool names, star counts, and a couple of cited
> arXiv IDs are point-in-time / third-party and should be treated as directional.
> The *direction* (render commoditized, verifiers ascendant, process-audit still
> uncontested) is solid.

---

## Actionable: prioritized, with author dispositions

### Tier 1 - harden the moat / close false confidence

1. **Strengthen invariant backing *agnostically* (accepted, reframed).** The
   shipped marker-scan must stay a textual ledger; parsing test bodies would
   lock awf to a single language, which is rejected. Instead:
   - **Stop over-claiming.** Correct "proof/backing" prose toward "ledger," and
     make the **review agent** explicitly own the semantic check ("does this test
     actually assert the invariant it backs?").
   - **Cheap agnostic floor-raising** in the scanner: reject a marker at
     column-0 / file-top so it must sit *inside* a block; optionally a
     per-project configurable `testFuncPattern` regex an adopter can set to
     require the marker be near a recognised test declaration. No language
     knowledge baked in.
   - **Harden awf-the-tool's *own* invariant tests** with Go tooling *privately
     in awf's gate* (see mutation testing below). This is awf's own test quality,
     never shipped to adopters, so agnosticism is preserved.
2. **Make mutation testing usable (accepted as next-up; open design).** Deferred
   originally for brittleness; the path to trustworthy signal:
   - **Scope tight, not `./...`**: run gremlins only on guardrail-critical
     packages (`invariants`, `coverage`, `manifest`, `render`, the `project`
     check paths). Brittleness scales with surface.
   - **Baseline/allowlist of known survivors**: a checked-in golden file of
     accepted (equivalent-mutant) survivors; CI fails only on a *new* survivor.
     This is the "fails-when-stale exemption map" trick awf already uses
     everywhere, applied to mutation output.
   - **Treat any timeout as "run invalid, retry," never "fail."** `cmd/mutants`
     already exits non-zero on timeouts; pair it in a scheduled job with
     `--workers 1` + tuned `--timeout-coefficient` so only a `Timed out: 0` run
     scores.
   - **Automate as nightly advisory** (not in `./x gate`), surfacing the
     diff-vs-baseline, signal without blocking.
   - **Point it at the code paths backed invariants guard.** A backed invariant
     whose guarded mutant *survives* is a test that doesn't bite: mutation
     testing doing, for awf's own Go, exactly what the shipped textual scanner
     structurally cannot. This is the concrete bridge between items 1 and 2.
3. **Behavioural agent eval: deferred by design.** Recorded here for
   completeness only; out of scope on cost grounds. No action.

### Tier 2 - cut context-rot & scope creep

4. **Tier the AGENTS.md invariants.** Keep only invariants an agent must
   *actively hold* to behave well; relocate gate-enforced mechanism invariants to
   a generated `docs/invariants.md` one hop from the map. Add a lint failing any
   invariant bullet over N lines. Give `code-reviewer` focusItems an
   eviction/age-out policy.
5. **Correct the "wraps the agent" positioning to "wraps agent output"** across
   the guide/README/skills. Matches the deliberate scope; removes the
   over-claim that invites the "but you don't test the agent" critique.
6. **Freeze `awf context`; stop dogfooding the coverage floor on awf's own tree.**
   Declare the relevance engine feature-complete (bugfix-only); no 11th context
   ADR. Let `--uncovered` be a tool *adopters* run on their code; awf's repo does
   not need its packages force-fit into five domains to quiet its own report.
7. **Write one "what awf is *not*" ADR** before 1.0 (not a code-intelligence
   platform, not a RAG tool, not project management) so the periphery has a stated
   ceiling that costs an ADR to breach. **Raise the ADR warrant bar** in
   `awf-proposing-adr` ("what will the project *forget* without this? - 'nothing'
   is a valid answer"); track the simplification/reversal ratio as a health
   metric.

### Tier 3 - code health & adoptability

8. **Split `check.go` (955 LoC)** into a `check/` subpackage; decompose
   `SyncReport`; add a `doc.go` orientation for `project`. Audit the 194
   `coverage-ignore` sites for the testable subset (the `os.Stat`/`Chmod`/
   `copyFile` cluster).
9. **Add the content-accuracy drift axis** (dead-path / dead-command in the
   rendered guide): extend `domain-code-staleness` from advisory toward a real
   check. This is the one axis the ecosystem genuinely passed awf on.
10. **Reposition + ride distribution.** Lead messaging with the governance/gate
    layer (drift-oracle + backed-invariants + append-only-ADR provenance +
    process audit: the uncontested intersection), not multi-adapter rendering
    (commoditized). Publish awf's rendered artifacts *as* a Claude Code plugin
    rather than competing with the marketplace. Put a "what you actually inherit"
    side-by-side (sundial's ~99-line guide vs awf's 38 KB) above the fold in the
    README; the self-presentation currently overstates adoption cost.
11. **Bus-factor insurance:** a "maintainer's digest", a curated reading path
    through the ~10 ADRs that actually define the architecture, so a future
    reader does not face all 111.

---

## Bottom line

The core is proportionate, the engineering is unusually disciplined, and the
adopter surface is cleaner than the intimidating self-presentation suggests. Two
themes separate awf-as-it-is from awf-as-it-claims. First, the safety story is
scoped to agent *output*, not agent *behaviour*, which is the right, deliberate
boundary, so the fix is to say so plainly and to close the one genuine
false-confidence surface (invariant backing) *agnostically*, with scoped
mutation testing as the private semantic backstop for awf's own code. Second, the
periphery is over-scoped by a machinery that makes adding features too cheap;
discipline that (freeze `awf context`, raise the ADR bar, write the scope-ceiling
ADR) and awf stays adoptable. The mechanical guardrails, the render engine, and
the skill/review-agent output are genuinely strong and need no rescue.
