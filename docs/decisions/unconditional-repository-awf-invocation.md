---
format: current-state-v4
slug: unconditional-repository-awf-invocation
status: Proposed
date: 2026-08-12
---
# ADR-unconditional-repository-awf-invocation: Unconditional Repository Awf Invocation


## Context

ADR-0253 made the repository-root `./awf` wrapper an unconditional rendered output, but the
render model and instruction corpus still preserve the earlier optional-runner distinction.
`internal/project` publishes a `runnerEnabled` value fixed to true, several templates retain
unreachable bare-`awf` alternatives, and other executable skill and agent instructions bypass the
value and spell bare `awf` directly. In this repository, where no `awf` binary is on PATH, an agent
following those instructions receives a command-not-found failure despite the wrapper beside it.

ADR-0156 also introduced `vars.awfInvokeCmd` so this repository could make its wrapper run awf from
source. The wrapper already has a `runner-body` convention-part boundary, and the project is the
only intended source-execution adaptation. The user confirmed that no adopter uses the var, so
preserving a public configuration key or migration path for it would retain a distinction with no
served repository fact behind it. Retiring the key removes its catalog/config-reference descriptor
and render input, removes this repository's configured value, and regenerates the wrapper's config
hash and lock-manifest entry through ordinary render; no migration or backward-compatibility path
remains.

The semantic boundary is between invocation and resolution. Rendered repository instructions invoke
`./awf`; the default wrapper resolves the bootstrap-pinned binary and then PATH, while a repository
with local execution semantics replaces the wrapper body through the existing convention part.
Conceptual references to the awf product or its CLI grammar need not acquire a path prefix.

## Decision

1. `decision: repository-wrapper-invocation` Every adopter- or agent-facing executable awf CLI
   instruction and hook fallback rendered into a repository uses its unconditional repo-root
   `./awf` wrapper. The render model carries no runner enablement signal and no such instruction
   assumes that `awf` is on PATH. Commands internal to bootstrap or wrapper binary resolution stay
   outside this invocation-guidance rule.

2. `decision: wrapper-resolution-part-owned` The standard wrapper body resolves the
   bootstrap-pinned binary first and PATH `awf` second. Repository-specific execution semantics are
   expressed by overriding the existing runner-body convention part, not by configuration data
   interpreted inside the standard wrapper template.

3. `decision: invocation-var-retired` The `awfInvokeCmd` var is retired without a compatibility
   migration. No adopter uses it; this repository expresses its source-execution adaptation through
   the runner-body part.

## State changes

- update `rendering/companion-scripts:runner-resolution-pinned-first`
- update `rendering/catalog-and-targets:var-descriptor-set-pinned`
- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- add `rendering/workflow-skill-templates:repository-awf-invocation`
- add `rendering/guide-and-doc-templates:guide-awf-invocation`

## Consequences

Agents receive one executable spelling that works in every adopted repository and always traverses
the repository's pinned or adapted resolution boundary. The dead `runnerEnabled` datum and its
conditional branches disappear, and the public var set loses a key whose only live value described
this repository's development setup.

This repository must carry a small runner-body convention part containing its from-source `exec`.
That makes the adaptation authored project policy rather than generic configuration. Bare `awf`
remains appropriate in product names, usage syntax, diagnostics, internal binary resolution, and
prose that does not direct a repository-local execution; distinguishing those cases requires a
focused instruction-corpus sweep rather than an indiscriminate textual replacement.

The two new invocation claims are test-backed invariants. Their proofs scan the applicable rendered
agent, guide, skill, and hook instruction surfaces, and the existing empty-data rendering spine
continues to reject no-value tokens and malformed command fragments after the obsolete conditionals
and var disappear.

Dropping the var without migration would break an unreported external user of `awfInvokeCmd` on
upgrade. The user accepts that risk based on the confirmed adopter set; retaining compatibility for
a hypothetical consumer would contradict the configuration-surface reduction this decision makes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `runnerEnabled` fixed to true and repair only the bare commands | Preserves unreachable template branches and misrepresents an unconditional output as a choice. |
| Keep `awfInvokeCmd` for this repository | Keeps repo-local development semantics in the public configuration surface despite an existing convention-part boundary. |
| Translate `awfInvokeCmd` to a part during upgrade | No adopter uses the key, so migration machinery would protect no real state. |
| Put awf on every agent environment's PATH | Bypasses the repository wrapper and can execute a stale or unpinned binary. |

## Status history

- 2026-08-12: Proposed
