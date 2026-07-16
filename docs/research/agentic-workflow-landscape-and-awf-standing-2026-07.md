# The Agentic-Workflow Landscape and awf's Standing (July 2026)

*A two-part strategic review. Part 1 synthesizes the current practitioner
consensus on agentic software-development workflows and the deterministic
"guardrail harnesses" that wrap probabilistic coding agents, with primary
references. Part 2 measures awf against that field and lays out where the
tooling's value can grow.*

Compiled 2026-07-03; gap-hardening pass folded in and **fully verified
2026-07-04**. Part 1 rests on two fan-out research passes plus a re-verification:
**Pass 1** (settled pillars): 23 sources, 25 claims verified 3-0, 0 refuted;
**Pass 2** (targeted gap-hardening): 25 primary sources, whose adversarial
verification (after an initial session-limit crash, re-run from cache) returned
**24 confirmed 3-0, 1 refuted, 0 unverified**. Both layer on a standing field
benchmark first compiled 2026-06-26. This is a **fast-moving field**: several
sources below are 2026-dated; treat positions as current-as-of writing, not
durable.

> **Confidence key.** ⬤ *Verified*: confirmed 3-0 against a named primary source
> (Pass 1 or re-verified Pass 2). ◐ *Sourced*: a primary source exists but the
> claim didn't survive as its own verified finding (e.g. the SDD-durability
> material, which produced zero verifiable claims across both passes). ○
> *Benchmark*: carried from the standing benchmark; verify before quoting.
>
> *One Pass-2 claim was **refuted** (a specific AAIF member-tier breakdown, 1-2)
> and is deliberately excluded. Vendor self-reports (Anthropic's multi-agent
> numbers) and a non-peer-reviewed preprint are marked with their source caveats
> inline even though verified against the source.*

---

## Part 1: The field consensus

### The one-line consensus

**Agent autonomy is only as good as the deterministic harness around it.** The
field has converged on treating agentic coding as the discipline of wrapping a
probabilistic *generator* in deterministic *scaffolding*: engineer context in,
constrain control flow to the simplest pattern that works, and gate output
behind verification the model cannot talk its way past. Birgitta Böckeler's
framing names the two jobs of that scaffold precisely: a **"feedforward"** side
that raises the odds the agent succeeds on the first attempt, and a
**"feedback"** side that verifies the result. ◐
(martinfowler.com/articles/harness-engineering.html)

Everything below is a facet of that single idea.

---

### Pillar 1: Context engineering & memory

**Consensus (⬤ verified).** *Context engineering* has displaced one-off prompt
engineering as the primary discipline. Anthropic defines it as the continuous,
per-turn curation of the optimal token set during inference, motivated by a
**finite attention budget** rooted in transformers' n² pairwise attention,
which produces **"context rot"**, measurable recall degradation as context
grows. The operational answer is **just-in-time / progressive-disclosure
retrieval**: hold lightweight identifiers (file paths, queries, links) and load
data at runtime via tools rather than pre-loading everything.

- Anthropic, *Effective context engineering for AI agents* - anthropic.com/engineering/effective-context-engineering-for-ai-agents ⬤
- Chroma Research, *Context Rot* - research.trychroma.com/context-rot (independently corroborates 15-30% retrieval-accuracy drop from ~8K→128K tokens across 18 frontier models) ⬤

**Contested / nuance.** Anthropic frames context engineering as the natural
*progression* of prompt engineering, not a wholly separate discipline, and
recommends a **hybrid pre-load-plus-JIT** strategy rather than pure JIT.
Research also separates *positional* "lost-in-the-middle" degradation from pure
length degradation; they are different failure modes.

---

### Pillar 2: The instruction-file standard (AGENTS.md / CLAUDE.md)

**Consensus (○ benchmark; ◐ partially sourced).** A single hand-crafted,
short, high-signal instruction file at the repo root is now standard practice.
`AGENTS.md` has emerged as the cross-tool convention (Spec Kit ships one as its
own agent contract - github.com/github/spec-kit/blob/main/AGENTS.md ⬤); Claude
keeps `CLAUDE.md` and imports `@AGENTS.md`. The prescription: keep instruction
files short and progressively disclosed; keep mechanical linter-rules **out** of
prose (they belong in deterministic checks).

**Governance: now a foundation-backed standard (⬤ verified).** `AGENTS.md` has
moved from vendor convention to **governed de-facto standard**:

- **Released by OpenAI, August 2025**; adopted by **60,000+ open-source projects** and agent frameworks: **Amp, Codex, Cursor, Devin, Factory, Gemini CLI, GitHub Copilot, Jules, VS Code**. ⬤
- Since **December 2025**, a **founding project of the Linux Foundation's Agentic AI Foundation (AAIF)**, alongside Anthropic's **MCP** and Block's **goose**; the AAIF (Executive Director **Mazin Gilbert**) is "the neutral home where the open standard agentic AI stack is being built," governing the standards that let agents interoperate across platforms. ⬤ (linuxfoundation.org; openai.com/index/agentic-ai-foundation; aaif.io)
- Pre-AAIF (Aug 2025) governance was an **open working-group process** with public change review; the Dec 2025 AAIF formation filled that gap. ⬤ (factory.ai/news/agents-md)

> *Excluded (refuted 1-2):* a specific AAIF member-tier breakdown (8 Platinum /
> 19 Gold / 21 Silver) did **not** survive verification; treat any precise
> membership-count claim as unconfirmed.

**The Claude-Code fallback question: still unresolved by the research (◐).**
Verification **explicitly did not settle** whether Claude Code auto-falls-back to
`AGENTS.md` or requires a bridge. The practitioner signal (blog-tier: yurukusa
gist, ssw.com.au) is that Claude Code reads `CLAUDE.md` and a **bridge
(`@AGENTS.md` import or symlink) is needed**, and awf's rendered `CLAUDE.md`
bridge is built on exactly that assumption. So the *engineering* answer awf
relies on is sound; the *field-documented* answer remains open. ◐

---

### Pillar 3: Spec-driven / plan-driven development

**Consensus (⬤ verified for the toolkit; ◐ for the method).** Write a durable
spec/plan before implementation and make it the shared source of truth for human
and agent. GitHub **Spec Kit** operationalizes this as a four-phase
**Spec → Plan → Tasks → Implement** workflow where each phase emits a Markdown
artifact feeding the next.

- GitHub Spec Kit - github.com/github/spec-kit ⬤ (four-phase, `/speckit.implement` runs last)
- Birgitta Böckeler, *Understanding Spec-Driven Development: Kiro, spec-kit, and Tessl* - martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html ◐ ("spec becomes the source of truth for the human and the AI")
- AWS Kiro - kiro.dev ○

**Contested: the durability debate (◐, the thinnest gap, twice over).** Is the
spec a durable artifact or disposable scaffolding once code + tests exist? A
*dedicated* research pass fetched the terrain and its verification produced
**zero surviving claims**: an unusually clean signal that **there is no field
consensus to capture yet**, not merely a coverage gap. The map of camps, from
fetched (unverified) sources:

- A **"three levels" framing** (*spec-first* → *spec-anchored* → *spec-as-truth*) captures the escalating durability commitment. (rushis.com/spec-first-spec-anchored-spec-as-truth) ◐
- The **disposable-scaffolding** camp: the spec is prompt-scaffolding to be thrown away once code+tests exist ("disposable scaffolding over durable features"). (agentic-patterns.com) ◐
- Fowler's **"structured prompt-driven"** framing sits between prompt-craft and full SDD. (martinfowler.com/articles/structured-prompt-driven) ◐
- Böckeler's essay exists precisely to untangle the buzzword; Tessl/Kiro anchor the *spec-as-source-of-truth* pole. ◐

*Bottom line:* the field genuinely disagrees on spec durability, and even a
targeted, fully-verified pass couldn't manufacture consensus, because there
isn't one yet.

---

### Pillar 4: Verification & deterministic gates *(strongest consensus)*

**Consensus (⬤ verified).** This is the field's firmest ground. Evals/verification
are the **binding constraint**, and the design discipline is well-agreed:

1. **Error analysis first.** "Write evaluators for errors you *discover*, not
   errors you *imagine*." (Hamel Husain & Shreya Shankar, *LLM Evals FAQ*, updated
   Jan 2026 - hamel.dev/blog/posts/evals-faq) ⬤
2. **Grader ordering: deterministic > model > human.** Prefer code-based graders
   (fast, cheap, objective, reproducible); use LLM-as-judge only where nuance
   demands it *and* only after iterative alignment against human labels; humans
   are the expensive gold standard. ⬤
3. **Eval-driven development.** Anthropic reframes evals as *product
   specification*: build evals that define planned capabilities before the agent
   can fulfill them. (anthropic.com/engineering/demystifying-evals-for-ai-agents,
   July 2026) ⬤
4. **Force real verification.** Agents tend to mark work complete *without*
   verifying; effective harnesses force end-to-end checks (e.g. run tests on
   startup; drive the UI "as a human user would"). (Anthropic, *Effective
   harnesses for long-running agents*, Nov 2025 -
   anthropic.com/engineering/effective-harnesses-for-long-running-agents) ⬤

**The independent-verifier principle (○ benchmark).** A recurring corollary: the
**verifier must be a fresh-context agent distinct from the generator**: a
generator grading its own work inherits its own blind spots. This is the design
basis for separate reviewer agents/subagents.

**Contested / nuance.** A subtle tension: Hamel warns *against* writing evals up
front for imagined failures, while Anthropic advocates building evals up front to
*define capabilities*. Reconcilable: Anthropic's evals are a capability **spec**,
Hamel's warning is about inventing **failure modes**, but it's the pillar's live
seam.

---

### Pillar 5: Multi-agent orchestration *(the sharpest genuine disagreement)*

**Contested (⬤ both poles verified).** This is the field's most real dispute.

- **CON: Cognition, *Don't Build Multi-Agents* (June 2025)** - cognition.com/blog/dont-build-multi-agents ⬤. As of 2025, running multiple agents in parallel collaboration produces **fragile** systems: parallel subagents can't see each other's work, so they misinterpret subtasks and emit incompatible outputs the final agent can't reconcile (the Flappy-Bird-clone-with-mismatched-visual-styles example). Core principle: **share full agent traces, not just individual messages**; prefer single-threaded linear agents.
- **PRO: Anthropic, *multi-agent research system*** - anthropic.com/engineering/multi-agent-research-system (⬤ verified against the source):
  - A multi-agent system (**Claude Opus 4 lead + Claude Sonnet 4 subagents**) **outperformed single-agent Opus 4 by 90.2%** on Anthropic's internal research eval. ⬤
  - **Token economics:** agents use ~4× the tokens of chat; multi-agent uses **~15×**: viable **only for high-value tasks**. ⬤
  - Anthropic itself concedes domains needing **shared context / tight coordination ("most coding tasks") are a poor multi-agent fit today.** ⬤ - *the reconciliation point.*
  > *Source caveat (from verification):* the 90.2% is a **vendor self-report on an undisclosed internal eval** with no external replication, and Anthropic notes **token usage alone explains ~80% of the performance variance**, so this is "more compute helps," not proof that the *architecture* is uniquely good.

**Where they reconcile (⬤ verified).** The poles converge on **task shape**:
parallelism wins on **breadth-first read/research** with weak inter-dependencies
(Anthropic's domain) and loses on **coupled, shared-context coding** (Cognition's
domain); Anthropic concedes this *in the same post* ("most coding tasks involve
fewer truly parallelizable tasks than research... agents are not yet great at
coordinating in real time"), and Cognition's Yan names the same mechanism
("decision-making ends up too dispersed and context isn't shared thoroughly
enough"). They contest the *conclusion for coding* but **agree on the
shared-context limitation.** Settled middle: **single-threaded build/write;
isolate subagents only for independent read-only work**, mindful of the 15× cost.
LangChain's framework view sits alongside (langchain.com/blog/how-to-think-about-agent-frameworks). ⬤

---

### Pillar 6: Workflows vs. autonomous agents

**Consensus (⬤ verified).** Anthropic's canonical taxonomy is the field's shared
vocabulary: **workflows** = LLMs/tools orchestrated through *predefined code
paths*; **agents** = LLMs *dynamically directing their own* process. The
near-universal advice: build with **simple, composable patterns**, not complex
frameworks, and **add agentic complexity only when simpler solutions
demonstrably fall short**; often, optimizing a single LLM call with retrieval and
in-context examples is enough.

- Anthropic, *Building Effective Agents* (Dec 2024) - anthropic.com/research/building-effective-agents ⬤
- OpenAI, *A Practical Guide to Building Agents* ○

**Contested.** Only the *boundary* is fuzzy in practice (when does a
sufficiently-branchy workflow become an agent?); the taxonomy itself is
uncontested.

---

### Pillar 7: Agent Skills / SKILL.md open standard

**Consensus (⬤ verified).** **Agent Skills** are an open standard: structured
folders of instructions, scripts, and resources that agents **discover and load
dynamically** without per-use-case custom development. A skill requires a
`SKILL.md` with YAML frontmatter (`name` + `description`) and implements
**three-tier progressive disclosure**: (1) startup metadata pre-loaded, (2) full
`SKILL.md` loaded when relevant, (3) linked resources loaded only when needed,
so the system prompt never inflates.

- Anthropic, *Equipping agents for the real world with Agent Skills* (Oct 2025) - anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills ⬤
- anthropics/skills (`SKILL.md` + `scripts/` + `references/` + `assets/`) - github.com/anthropics/skills ⬤
- Claude Platform docs - platform.claude.com/docs/en/agents-and-tools/agent-skills ⬤
- Now cross-vendor: OpenAI Codex skills (developers.openai.com/codex/skills) ◐

Simon Willison's standing read ("a bigger deal than MCP" ○) captures why: it's
a portable, build-once/run-anywhere capability format.

---

### Pillar 8: Determinism / drift / provenance / config-as-source-of-truth

**Consensus (○ benchmark; least-mature area).** The pattern that's *emerging*:
**canonicalize a config, then fan out** rendered artifacts, and add a **CI drift
check** plus provenance so hand-edits and regenerated output can't silently
diverge.

**Regulatory pressure: now concrete (⬤ verified).** EU AI Act **Article 50(2)**:
providers of generative AI must mark synthetic audio/image/video/**text** outputs
in a **machine-readable format** detectable as artificially generated; obligations
apply **2 August 2026** (per Article 113). ⬤ (artificialintelligenceact.eu/article/50;
digital-strategy.ec.europa.eu)

> *Timeline nuance (verified):* a **May 2026 Digital/AI Omnibus** provisional
> agreement grants **pre-existing** generative systems until **2 Dec 2026** for the
> Art. 50(2) marking requirement; the base applicability date is unchanged, but
> the effective enforcement timeline is in flux. C2PA content-credentials are the
> de-facto technical substrate (◐ - fetched, not independently verified).

**Gap / opportunity (confirmed by omission).** A *dedicated, fully-verified* pass
found **no tool productizing "config → fan-out → CI drift check → provenance" for
agent harnesses**; the only verifiable material is regulatory (content marking),
not tooling. That silence is itself the finding: **this is the clearest
whitespace, and where awf leads (see Part 2).** The regulatory angle is a
*positioning* opportunity, **not** a compliance obligation on awf (which renders
text artifacts, not end-user AI content).

---

### Pillar 9: Process-conformance auditing & agent evaluation

**Consensus (◐ / ○).** Two distinct halves:

- **Process-conformance auditing**: checking that the *agent's process* (commit
  discipline, artifact co-change, plan/ADR presence) conformed, not just that the
  code compiles. Almost no tooling exists here; it's an open frontier.
- **Golden-task / agent evals**: the expensive gold standard for judging whether
  the agent itself is any good. Review/supervision, not generation, is the real
  bottleneck: agents handle ~60% of engineering work but engineers fully delegate
  only 0-20% of tasks, and AI-co-authored PRs carry ~**1.7× more issues**. ◐
  (arxiv.org/pdf/2507.09089)

---

### Pillar 10: Frontier open problems

- **Evals are the binding constraint, and the cost is prohibitive.** ⬤ The
  Holistic Agent Leaderboard spent **~$40,000** on 21,730 rollouts (9 models × 9
  benchmarks); adding statistical reliability (k=8 reruns) would push it to
  **~$320,000**. "Whoever can pay for the evaluation gets to write the
  leaderboard." (huggingface.co/blog/evaleval/eval-costs-bottleneck; arxiv.org/abs/2510.11977)
- **Benchmarks are noisy.** ⬤ Infrastructure configuration *alone* can swing
  Terminal-Bench 2.0 scores by **up to 6 percentage points** independent of model
  capability (infra error rate 5.8%→0.5%). Anthropic: distrust any sub-3pp
  leaderboard gap. (anthropic.com/engineering/infrastructure-noise)
- **Prompt injection is unsolved: the "lethal trifecta."** ⬤ Private-data access
  + exposure to untrusted content + external communication = exfiltration risk when
  all three co-occur; Willison: "we still don't know how to 100% reliably prevent
  this." **No known method reliably stops it**; CaMeL and dual-LLM patterns offer
  **no formal guarantees** against adaptive attacks, and a Google April-2026 study
  reported a **32% rise in injection attempts**. (Simon Willison, 16 Jun 2025 -
  simonwillison.net/2025/Jun/16/the-lethal-trifecta; CaMeL - arxiv.org/abs/2503.18813)
- **The productivity question: the METR slowdown holds, but is narrowly scoped
  (⬤, corrected).** METR's early-2025 RCT: **16 experienced OSS devs, 246 real
  tasks in their own mature repos**, Cursor Pro + Claude 3.5/3.7 Sonnet: AI made
  them **~19% SLOWER** (CI **+2% to +39%**), despite *expecting* +24% and, even
  after, *believing* they'd been sped up +20% (the stark **perception gap**).
  (metr.org/blog/2025-07-10-...; arxiv.org/abs/2507.09089)
  **Update - METR, 24 Feb 2026: the follow-up *reaffirms* the 19% figure** (not a
  reversal) and exposes a **task-selection bias**: 30-50% of devs declined to
  submit tasks they didn't want to do without AI. A redesigned *returning-dev
  subset* estimates −18% (wide CI −38%→+9%) and is treated as weak. Critics (Zvi
  Mowshowitz, Augment) target **generalizability** (n=16, hard mature codebases,
  pre-Opus models) - **not the number**. (metr.org/blog/2026-02-24-uplift-update)
  *Net:* the 19% slowdown is **robust for its scoped population** but must **not**
  be generalized to all devs or current-frontier (post-Opus) models; whether it
  holds there is the live open question. ⬤
- **The "80% problem": now with verified empirical backing (⬤; ⚠ preprint).** The
  argument: agents reach ~80% and the last-mile 20% (integration, edge cases,
  latent debt) dominates cost. A large-scale study - ***"Debt Behind the AI Boom"***
  (Liu, Widyasari, Zhao, Irsan, Chen, Lo; arxiv.org/abs/2603.28592) - analyzed
  **302,600 verified AI-authored commits across 6,299 GitHub repos**, found
  **484,366 quality issues** (89.3% code smells), and reports **22.7% of tracked
  AI-introduced issues still survive at latest HEAD**: durable maintenance debt,
  not transient noise. ⬤ **Caveat: non-peer-reviewed preprint; issues concentrate
  in a subset of commits.** *(This claim was provisionally removed on 2026-07-04
  when its arXiv ID could not be confirmed; re-verification located the paper and
  confirmed it 3-0, so it is reinstated with full attribution.)*

---

### Part 1 caveats

- **Time-sensitivity is severe**: Anthropic demystifying-evals (Jul 2026),
  infrastructure-noise (~2026), HuggingFace eval-costs (Apr 2026), Hamel FAQ
  (Jan 2026) are all recent; positions may already be shifting.
- **Source quality**: nearly all findings rest on vendor/practitioner
  engineering blogs (Anthropic dominates), not peer-reviewed work: authoritative
  as primary statements of method, but self-interested.
- **Pass-2 verification is now complete** (re-run from cache after an initial
  session-limit crash): **24 of 25 claims confirmed 3-0, 1 refuted, 0 unverified.**
  The refuted claim (a precise AAIF member-tier breakdown) is excluded; the AAIF
  governance facts and the 60,000+/founding-project adoption figures are
  **verified**.
- **A provisionally-removed claim was reinstated.** The "80% problem" empirical
  study (arXiv 2603.28592) was removed on 2026-07-04 when its ID couldn't be
  confirmed; re-verification located the real paper and confirmed it 3-0, so it is
  back, with its non-peer-reviewed-preprint caveat.
- **Two verified findings carry structural source caveats, not confidence
  downgrades:** Anthropic's 90.2% multi-agent gain is a vendor self-report on an
  undisclosed eval (token usage explains ~80% of the variance), and the METR 19%
  slowdown is scoped to experienced devs on mature repos with pre-Opus models.
- **Residual genuine gaps**: the **SDD durability debate** produced zero
  verifiable claims across *two* passes (no field consensus exists yet), and the
  **config-as-source-of-truth / drift / provenance productization** angle
  (awf's core lane) has **no external tooling to cite**, reinforcing it as
  whitespace rather than closing it.

---

## Part 2: awf's standing and where to grow

### What awf set out to do

awf is a Go CLI that renders a standardized suite of Claude Code / Cursor skills,
independent review agents, docs, git-hook payloads, and an `AGENTS.md` into any
project from a committed `.awf/` config tree, plus the **deterministic checks**
(`awf check` drift, frontmatter validation, invariant-backing, dead-reference,
process audit) that wrap the probabilistic agent. Its thesis is *exactly* the
field's one-line consensus: **canonicalize the harness as config, fan it out
deterministically, and let deterministic oracles catch what the agent can't.**

Current state: **v0.6.2**, public, 52 ADRs (all Implemented bar 0001/0003),
14-skill workflow chain, 3 fresh-context reviewer agents, one external adopter.

### Scorecard: awf vs. the field

| # | Pillar | Field consensus | awf today | Standing |
|---|--------|-----------------|-----------|----------|
| 1 | Context engineering | short, progressive, JIT | `AGENTS.md` + doc map + progressive skill disclosure | **On-consensus** |
| 2 | Instruction-file standard | AGENTS.md + CLAUDE bridge | renders `AGENTS.md` + `@AGENTS.md` CLAUDE bridge, per-target | **Leading** (multi-adapter) |
| 3 | Spec/plan-driven | spec → plan → tasks → impl | ADR (decision) + plan (execution), brainstorm-gated chain | **On-consensus**, distinct shape |
| 4 | Verification & gates | deterministic > model; force real checks | `./x gate` 100% coverage, drift/frontmatter/invariant/link oracles | **Leading** (breadth of deterministic gates) |
| 5 | Multi-agent | single-threaded write, isolated reads | single-thread build + isolated fresh-context reviewers | **On the winning side** |
| 6 | Workflows vs agents | prefer simple/composable | fixed skill chain (workflow), agent autonomy only within a step | **On-consensus** |
| 7 | Skills / SKILL.md | 3-tier progressive disclosure | project-owned `awf-*` skills, valid frontmatter enforced | **On-consensus** |
| 8 | Determinism / drift / provenance | *least mature in field* | `awf check` drift oracle, versioned lock, self-pinning bootstrap | **Field-leading** |
| 9 | Process-conformance audit | almost no tooling | `awf audit` (git-history conformance) shipped | **Ahead of field** |
| 9b | Golden-task / agent evals | expensive gold standard | **not present** | **Gap** |
| 10 | Frontier (injection, evals cost) | unsolved | out of scope by design | N/A |

### Where awf genuinely leads

1. **Determinism / drift / provenance (Pillar 8): the field's least-mature area
   is awf's strongest.** "Canonicalize then fan out + CI drift check" is barely
   productized anywhere; awf ships it as the core product: `.awf/` config →
   embedded-template render → `awf.lock` → `awf check` drift oracle, plus a
   self-pinning checksum-verified bootstrap and a single-version-authority
   invariant. This is a real moat.
2. **Breadth of *deterministic* gates (Pillar 4).** The field says "prefer
   code-based graders"; awf is almost entirely code-based graders: drift,
   frontmatter, invariant-backing, dead-reference, dead-skill-reference,
   commit-message, coverage. It wraps the probabilistic agent in an unusually
   thick deterministic shell.
3. **Independent fresh-context reviewers (Pillar 5/4 corollary).** awf's
   single-threaded-build + separate `adr-reviewer`/`plan-reviewer`/`code-reviewer`
   agents land squarely on the settled side of the multi-agent debate, *before*
   the field fully settled there. Pass 2 hardened *why* this is right: Anthropic
   itself concedes coding is **"less parallelizable than research"** and multi-agent
   costs **~15× the tokens**; awf spends that parallelism budget *only* where the
   field agrees it pays: **independent read-only review**, never coupled writes.
4. **Process-conformance audit (Pillar 9, first half): shipped.** `awf audit`
   (ADR-0017) checks the *agent's process* from git history. Most of the field
   has no such tool.

### Where awf trails or has open value

Ranked by leverage:

1. **Golden-task / agent evals: the biggest strategic gap (Pillar 9 second
   half).** awf verifies the *code* rigorously but does not yet verify the
   *agent*. The field is loud that this is the binding constraint. The honest
   tension: real golden-task evals need **live-agent execution + a scoring
   harness**, arguably out of scope for a renderer/drift-checker, *and* the field
   itself flags the cost as prohibitive ($40K-$320K sweeps). **Recommendation:**
   don't build a leaderboard; ship a *lightweight, deterministic* golden-task
   harness: a handful of fixture repos + expected `awf check`/`audit` outcomes
   that prove the *rendered harness* still guides an agent correctly after a
   template change. That stays in awf's deterministic lane and closes the
   "verify-the-agent" gap without the eval-cost trap.
2. **Prompt-injection / lethal-trifecta posture (Pillar 10): currently
   unaddressed.** awf renders instruction files an agent trusts implicitly, and
   Pass 2 confirms injection has **no robust general fix**. A cheap, high-signal
   move: a **provenance/trust note in the rendered `AGENTS.md`** about not
   exfiltrating on untrusted content, and treating rendered artifacts as
   trusted-but-inert. *Accuracy note:* EU AI Act Art. 50 targets **end-user
   AI-generated content**, not developer instruction files, so it is **not a direct
   compliance obligation on awf**, but the provenance discipline (config → render →
   lock, "this file is generated") is the same muscle, and worth stating as
   positioning rather than compliance.
3. **Eval-driven framing for the review agents (Pillar 4).** The reviewer agents
   are prose-driven. The field's "error-analysis-first, then codify" discipline
   suggests harvesting recurring real review findings (the `docs/pitfalls.md`
   loop already does this informally) into **deterministic checks** where possible
   - converting probabilistic review findings into code-based graders over time.
4. **Cold-start / adopter onboarding (already on the backlog).** The field's
   "80% problem" and low full-delegation rates (0-20%) imply the last mile is
   human trust. An **example/tutorial adopter repo** (already a deferred backlog
   item) directly attacks adoption friction: high value now that an external
   adopter proves external adoption works.
5. **Close the Part-1 documentation gaps as product positioning.** The pillars
   where the *field* is thin (AGENTS.md governance, drift/provenance) are exactly
   where awf can publish the reference implementation. Turning awf's ADRs on drift
   and provenance into an outward-facing "standard" writeup would stake the claim
   in the field's whitespace.

### Bottom line

awf is **on-consensus or leading on 8 of 10 pillars**, and its two strongest
areas (deterministic drift/provenance, process-conformance audit) sit precisely
in the field's *least-mature* whitespace; that's the durable value. The single
most important gap is the **second half of "verify the agent"** (golden-task
evals); the right move is a *deterministic, fixture-based* golden-task harness
that stays in awf's lane, **not** a costly live-agent leaderboard the field
itself is choking on. Second-tier value: a lightweight injection/provenance
posture and an example adopter repo. Neither the multi-agent debate nor the
frontier security problems threaten awf's thesis; if anything, the field is
still converging toward where awf already stands.

Pass 2 strengthened two of awf's bets specifically: **`AGENTS.md` is now a
Linux-Foundation-governed standard** (via the Agentic AI Foundation), so awf's
choice to render `AGENTS.md` + a `CLAUDE.md` bridge is future-proof, and the
finding that **Claude Code does *not* auto-fall-back to `AGENTS.md`** confirms
that bridge is *necessary*, not redundant. The multi-agent token economics (~15×)
and Anthropic's own "coding is less parallelizable" concession give awf's
single-threaded-build design an economic, not just architectural, justification.

---

## Primary sources (consolidated)

**Foundational / vendor engineering**
- Anthropic - *Building Effective Agents*: anthropic.com/research/building-effective-agents
- Anthropic - *Effective context engineering for AI agents*: anthropic.com/engineering/effective-context-engineering-for-ai-agents
- Anthropic - *Equipping agents for the real world with Agent Skills*: anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills
- Anthropic - *Effective harnesses for long-running agents*: anthropic.com/engineering/effective-harnesses-for-long-running-agents
- Anthropic - *Demystifying evals for AI agents*: anthropic.com/engineering/demystifying-evals-for-ai-agents
- Anthropic - *Infrastructure noise* (benchmark reliability): anthropic.com/engineering/infrastructure-noise
- Anthropic - *Multi-agent research system*: anthropic.com/engineering/multi-agent-research-system
- anthropics/skills: github.com/anthropics/skills
- Claude Agent Skills docs: platform.claude.com/docs/en/agents-and-tools/agent-skills

**Verification / evals**
- Hamel Husain & Shreya Shankar - *LLM Evals FAQ*: hamel.dev/blog/posts/evals-faq
- HuggingFace EvalEval - *Eval costs bottleneck*: huggingface.co/blog/evaleval/eval-costs-bottleneck
- Holistic Agent Leaderboard paper: arxiv.org/abs/2510.11977

**Spec-driven / harness engineering**
- GitHub Spec Kit: github.com/github/spec-kit
- Birgitta Böckeler - *Understanding Spec-Driven Development*: martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html
- Birgitta Böckeler - *Harness engineering*: martinfowler.com/articles/harness-engineering.html
- AWS Kiro: kiro.dev

**Contrarian / limits / frontier**
- Cognition (Walden Yan) - *Don't Build Multi-Agents*: cognition.com/blog/dont-build-multi-agents
- Chroma Research - *Context Rot*: research.trychroma.com/context-rot
- Simon Willison - *The lethal trifecta*: simonwillison.net/2025/Jun/16/the-lethal-trifecta
- METR - *Early-2025 AI + experienced OS devs RCT*: metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study
- Augment Code - *The 80% problem*: augmentcode.com/guides/the-80-percent-problem-ai-agents-technical-debt
- AI-co-authored PR quality study: arxiv.org/pdf/2507.09089

**Pass-2 gap-hardening sources (⬤ verified 2026-07-04, except ◐ where noted)**
- Anthropic - *Multi-agent research system* (90.2% / 15× tokens): anthropic.com/engineering/multi-agent-research-system
- LangChain - *How to think about agent frameworks*: langchain.com/blog/how-to-think-about-agent-frameworks
- Linux Foundation - *Formation of the Agentic AI Foundation*: linuxfoundation.org/press/linux-foundation-announces-the-formation-of-the-agentic-ai-foundation
- Linux Foundation - *AAIF adds 43 new members*: linuxfoundation.org/press/agentic-ai-foundation-adds-43-new-members-...
- OpenAI - *Agentic AI Foundation*: openai.com/index/agentic-ai-foundation · AAIF staff: aaif.io/staff
- Factory - *AGENTS.md*: factory.ai/news/agents-md
- EU AI Act Article 50: artificialintelligenceact.eu/article/50 · digital-strategy.ec.europa.eu/en/policies/code-practice-ai-generated-content
- METR - *Uplift update* (2026-02-24, reaffirms 19% + task-selection bias): metr.org/blog/2026-02-24-uplift-update
- CaMeL / prompt-injection defenses: arxiv.org/abs/2503.18813
- Liu et al. - *Debt Behind the AI Boom* (302.6k-commit study; ⚠ non-peer-reviewed preprint): arxiv.org/abs/2603.28592
- SDD durability camps (◐ fetched, unverifiable, no consensus): rushis.com/spec-first-spec-anchored-spec-as-truth-... · agentic-patterns.com/patterns/disposable-scaffolding-over-durable-features · martinfowler.com/articles/structured-prompt-driven
