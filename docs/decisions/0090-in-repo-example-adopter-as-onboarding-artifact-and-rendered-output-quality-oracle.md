---
status: Implemented
date: 2026-07-10
tags: [example-adopter]
related: [8, 20, 39, 48, 49, 53, 70, 77, 80, 81, 82, 83, 86, 87, 89]
domains: [tooling, rendering]
---
# ADR-0090: In-repo example adopter as onboarding artifact and rendered-output quality oracle

## Context

awf has no example adopter: cold-start onboarding rests on prose docs plus this
repository's own dogfood tree, which is a poor model (it builds awf from source, carries
repo-development tooling, and its config exists to develop awf, not to adopt it). At the
same time, rendered-output *quality* has no deterministic review surface: the per-artifact
goldens (`internal/project`) and the eval suite (`internal/evals`, ADR-0053) pin template
fragments and chain seams, but nothing shows a template change as an adopter would
receive it: assembled prose in a real `AGENTS.md`, a real skill, a real domain doc. The
user's framing names both goals: an example adopter "which we can also use to
deterministically see that the produced output is high quality."

Grounding discoveries shaping the design (verified against source and empirically in a
scratch adopter):

- awf's project root is exactly `os.Getwd()` (`cmd/awf/main.go`); no upward search, no
  git-root detection. `init`/`sync`/`check`/`invariants` run green with cwd a
  subdirectory of a git repository whose root is elsewhere; none of them touch git. `awf
  audit` `PlainOpen`s `<root>/.git` and hard-errors when the example is the root; it
  cannot run there even by accident.
- A lock whose schema generation is behind the binary refuses every gated command with a
  "run awf upgrade" hint (`cmd/awf/gate.go`, ADR-0039). A committed example therefore
  forces every schema bump through a real `awf upgrade` in-repo: deterministic migration
  rehearsal before any external adopter runs it.
- Nested Go modules are invisible to the enclosing module's `./...` sweeps. Every gate
  step is module-scoped (`x`), `.golangci.yml`/`codecov.yml`/`.gremlins.yaml` carry no
  path config that reaches outside, verified empirically with a nested module containing
  a non-compiling file. Module isolation costs nothing and needs no exclusion lists.
- `./x gate` does **not** invoke `./x check`; they are sibling commands enforced together
  by the pre-commit hook payload and as two CI steps. The enforcement seam for the
  example is `./x check` (and `./x sync`), not the gate.
- Advisory notes (stub sections ADR-0070, part markers ADR-0083, unset vars ADR-0087,
  version-ahead) print to stdout as `note: ` lines with exit 0; success always prints
  `awf check: clean`. A zero-notes assertion is a line filter, not output-emptiness.
- The enclosing repo's `invariants.sources` globs (`**/*.go`, ADR-0077 anchored dialect,
  no negation) match `examples/**/*.go`: a fictional `invariant:` comment whose slug
  collides with a real awf slug would falsely back it.
- The full-surface enabled set is closed and legal: `targets: [claude]` alone is the init
  default; all 16 skills + 3 agents + 7 docs satisfy the ADR-0081 closure
  (`roadmap-graduation` pulls the `roadmap` doc); all seven vars-map string descriptors
  are consumed by that set, so setting them all is unused-var-clean (ADR-0086);
  `commitScopes`, the eighth string-kind descriptor, lands in `audit.allowedScopes`,
  not `vars:`. The planned
  `.awf/` inventory (glossary `data.terms` sidecar (ADR-0089), domain sidecars with
  `paths:` plus parts, agents-doc sidecar `data:`, enabled bootstrap and hooks render
  units) is fully claimed under the ADR-0086 model. Every agents-doc `docMap` entry
  must resolve on disk (ADR-0020 dead-link drift is failing, not advisory).
- The lock is byte-stable across repeated syncs and stamps the `project.Version` const
  even from a source build (ADR-0049), so from-source rendering and a released binary
  produce identical example output.

## Decision

1. **The repository carries one complete example adopter at `examples/sundial/`.** A new
   top-level `examples/` directory holds a small fictional Go CLI (sunrise/sunset
   schedules), module `example.com/sundial` with its own `go.mod`: module isolation is
   the mechanism that keeps the enclosing test/coverage/vet/lint/deadcode sweeps away
   from it. The example is a whole adopter, all of it committed: fictional compiling
   source with token tests, a minimal `./x` command runner, a hand-written
   `README.md` (what this is, what is generated, how to regenerate), hand-authored
   `.awf/` config tree, hand-written workflow artifacts (three fictional ADRs following
   the rendered template (`status`/`domains` frontmatter feeding ACTIVE.md and the
   domain indexes, one of them Implemented with a backed invariant slug) and one sample
   plan), `.githooks/` stubs mirroring this repo's own wiring, and every rendered file
   (`AGENTS.md`, `CLAUDE.md` bridge, `.claude/skills/sundial-*/`, `.claude/agents/`,
   the docs set including `config-reference.md` and `decisions/{README,template,ACTIVE}.md`,
   domain docs, `.awf/bootstrap.sh` + `.awf/upgrade.sh`, the three hook payloads,
   `.awf/memory/.gitignore`).

2. **The example's config is full-surface.** Prefix `sundial-`; `targets: [claude]`
   (cursor output is near-byte-identical and would double the diff for no review
   signal); every catalog artifact enabled (all skills including opt-in
   `roadmap-graduation`, all agents, all docs including `debugging` and `roadmap`,
   `bootstrap`, `hooks`); all seven vars-map string descriptors set to commands and
   paths that exist in the example tree, `commitScopes` populating
   `audit.allowedScopes`; two domains with
   `paths:` (`internal/almanac/**`, `cmd/**`) and current-state parts; convention parts
   covering each override flavor (plain body replacement, `{{=awf:sectionDefault}}`
   extension, doc parts, agents-doc sidecar `data:` plus parts); a
   `.awf/docs/glossary.yaml` `data.terms` sidecar. Every stub-classified section in the enabled set gets authored
   content: the example models a finished adoption, not a fresh scaffold.

3. **`./x sync` and `./x check` own the determinism.** `./x sync` renders this repo's own
   tree, then builds the from-source binary once and runs `awf sync` with
   `examples/sundial` as cwd (`go run` cannot cross the module boundary; the binary is
   invoked by absolute path). `./x check` likewise runs `awf check` and `awf invariants`
   there, plus `go test ./...` inside the example module (its token tests keep
   `invariantTestPath` honest and its scenery compiling). The example tests ride
   `./x check`, not the gate, deliberately: the gate stays module-scoped and fast,
   example scenery only changes at sync time, and the pre-commit payload and CI
   enforce check beside the gate anyway. Template changes thus surface
   as reviewable rendered diffs in the same commit, and forgetting the re-sync fails
   `./x check`. A gate test
   in the repo module asserts the `./x` example steps exist (the
   `cmd/releasecheck/main_test.go` wiring-test pattern), so the mechanism cannot be
   silently dropped.

4. **The example is held to zero advisory notes.** The example check step fails on any
   `note: ` line in `awf check` stdout. This is a repo-local wrapper policy over awf's
   output: awf's own advisory semantics are untouched and ADR-0070's
   advisory-never-fails invariant stands; the model adopter simply has no smells
   (config smell is an error state). The version-ahead note failing the wrapper is
   deliberate: it forces the re-pin sync after a version bump.

5. **Deliberate exclusions.** `awf audit` does not run in the example (no `.git` at its
   root; it hard-errors). The enclosing gate's Go tooling never touches the example
   module beyond Decision 3's `go test`. Maintenance is the committed-tree model: the
   `.awf/` tree is authored directly like any real adopter's; no init-regeneration
   step exists, and `awf init` scaffolding coverage is an accepted non-goal (init has
   its own tests). The example's hook payloads and `.githooks/` stubs are
   illustrative-only: the example is not a git root, so they can never fire.

6. **Example invariant slugs are namespaced.** The enclosing repo's invariant scan sees
   `examples/**/*.go`, so every fictional slug carries a `sundial-` or `almanac-` style
   prefix distinguishing it from real awf slugs; a collision would falsely back a real
   invariant.

7. **The example is the linked onboarding artifact, and the docs travel with the
   change.** The root `README.md` and `docs/working-with-awf.md` point at
   `examples/sundial/` as the worked example, the latter via a convention part at
   `.awf/parts/working-with-awf/<section>.md` (the singleton-doc parts path), never
   a shipped-template edit
   (the link is this repo's identity; rendering it into every adopter's doc would
   violate publication safety and the ADR-0082 residue guard). The root
   `.gitignore`'s unanchored `.claude` negations already cover the example. The
   implementing change also updates, in the same change and via their source parts:
   the `.awf/agents-doc.yaml` invariants list (the three new slugs),
   `docs/testing.md` (the check tier gains the example steps),
   `docs/development.md` (command-runner behavior), and `docs/architecture.md` (the
   new top-level tree). A standalone mirror repository is deferred until onboarding
   demand exists.

## Invariants

- `invariant: example-adopter-checked`: `./x sync` re-renders `examples/sundial` with the
  from-source binary, and `./x check` runs `awf check` and `awf invariants` there;
  example drift or an invariant finding fails `./x check`.
- `invariant: example-zero-notes`: the example check step fails on any `note: ` line in the
  example's `awf check` output.
- `invariant: example-module-isolated`: `examples/sundial` is its own Go module; no enclosing
  `./...` sweep (test, coverage, vet, lint, deadcode) includes its packages.
- Textual: fictional invariant slugs in the example are namespaced away from real awf
  slugs; `awf audit` is not run in the example; the example's hook payloads and
  `.githooks/` stubs are illustrative-only.

## Consequences

Easier:
- Rendered-output quality regressions become reviewable file diffs over a realistic
  adopter, in the same PR that caused them: deterministic, human-judged, no prose
  assertions in Go.
- Every schema migration is rehearsed in-repo: the stale-generation refusal forces
  `awf upgrade` on the example before the change can land green.
- Cold-start onboarding gets a complete, browsable, provably-in-sync worked example,
  including the first committed `bootstrap.sh`/`upgrade.sh` pair (pointing at real
  releases, intended) and a modeled `.githooks/` activation.

Harder / accepted trade-offs:
- Template PRs now carry a second rendered diff plus the example's lock diff. That noise
  is the feature, but it is real diff volume, and contributors must run `./x sync`
  after any template edit or fail check.
- Version bumps re-pin the example's lock and bootstrap; between the bump commit and the
  tag, the committed bootstrap transiently pins a release that does not exist yet (same
  window every release-prep commit already has).
- Authoring the full-surface content (~15 stub sections, three fictional ADRs, a plan,
  fictional source) is a real one-time cost, and the fiction must be kept coherent when
  the catalog grows: a new stub-classified catalog section enabled by default surfaces
  as a zero-notes failure on the next sync, which is the intended prompt to author it.
- The example never exercises `awf init` (accepted non-goal) or `awf audit`.
- One example, one language: the fiction is a Go project. The standard is
  language-agnostic; a non-Go example is future work if demand appears.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Separate example GitHub repository | Best onboarding optics, but drift-checking against unreleased templates needs cross-repo CI and quality diffs leave the PR that caused them; deferred as a mirror, not chosen as the source. |
| In-repo canonical plus release-time mirror from day one | Adds a publish step to the release runbook before any demand exists. |
| Minimal cold-start shell (init default set, empty parts) | Only covers default renders; the quality oracle needs parts, domains, data, and workflow artifacts exercised. |
| Two examples (minimal + kitchen-sink) | Two trees to keep coherent; the full-surface example serves both goals alone. |
| Regenerate from an `init --answers` file | Wipe/collision/backup semantics on every regen, and unlike any real adopter's day-to-day; also weaker as onboarding (adopters maintain a tree, not an answers file). |
| Go golden-tree test rendering into a temp dir | Failure output is a test diff, not a reviewable file diff; duplicates sync plumbing the `./x` wiring gets for free. |
| CI-only render-and-diff job | Catches drift post-push; violates the green-gate-before-commit discipline. |
| Enabling both targets (claude + cursor) | Cursor output is near-byte-identical; doubles diff surface for no review signal. |
