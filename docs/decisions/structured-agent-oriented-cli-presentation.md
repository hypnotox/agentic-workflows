---
format: current-state-v4
slug: structured-agent-oriented-cli-presentation
status: Proposed
date: 2026-08-04
---
# ADR-structured-agent-oriented-cli-presentation: Structured Agent-Oriented CLI Presentation


## Context

Awf's eighteen top-level commands expose one interface but do not share a presentation model.
Command handlers and result-owning packages print a mixture of padded tables, Markdown-like
headings, prose sentences, bare paths, semicolon-compressed facts, and command-specific labels.
The same concepts appear under unrelated spellings such as `note:`, `plan:`, `awf render:`, and
`check repo`. High-volume findings from audit and checks align for a terminal rather than form
stable records, while low-volume mutations compress several facts onto one line. Agents can read
these forms, but cannot rely on one hierarchy, field boundary, grouping rule, or error envelope.

ADR-0195 made each result-model-owning package responsible for its human rendering and left the
command binary argument parsing, renderer selection, and exit mapping. That decision corrected
representation leakage, but deliberately left untouched command-side renderers and gave every
owner freedom to invent syntax. A repository-wide conversion now needs one syntax authority
without moving domain meaning back into the command or into a universal result map.

Several current contracts make this more than cosmetic cleanup. Repository checks execute a
capability plan whose steps write eagerly while failures are accumulated. Upgrade migrations
receive an `io.Writer` and print while mutating. The command boundary sometimes prints findings
to stdout, then returns a prose error that `main` prints again to stderr. Interactive init prompts
must be flushed before input and therefore cannot use whole-command buffering. The repository
runner recognizes exact `note: ` prefixes, and release and bootstrap consumers parse version
output. These producers and consumers must move with the public contract rather than become
unrecorded compatibility exceptions.

Some outputs are not presentations. `read plan` and changelog selections intentionally return
source-like payload bytes. Effort activity and init descriptor output are machine protocols, and
context has an exact spill notice. Conversely, optional JSON modes for effort new, list, and show
and for topic duplicate ordinary query results without serving a required protocol. The current
actionable-outcome rule also flattens multi-step recovery onto one numbered line, contrary to the
chosen readable list form, and the audit severity claim reserves `warn` even though grouped output
needs the readable category `warnings`.

The desired interface is readable before it is terse: labeled blocks for ordinary results,
semantic categories for collections, and one stable record per line when volume is high. It must
remain deterministic enough for agent and script consumption, use minimal formatting, and avoid
both terminal tables and a JSON-first contract.

## Decision

1. `decision: readable-text-contract` Ordinary awf command output uses one deterministic readable-text contract. A scalar is `label: value`, indentation is two spaces, and a section is a lowercase label followed by nested nodes. Output uses no alignment padding, Markdown headings or tables, decorative framing, or semicolon-compressed records. Colon separates a label from its value. A compact record separates its ordered fields with ASCII ` | `. A collection groups records under semantic category labels and gives every entry in one category the same declared field schema. Categories and records have deterministic order, and each high-volume record occupies one line. Root Fields form an optional leading block and every root Section follows that block; a root Field after any root Section is invalid. Consecutive root Fields have no blank line, a Section-first document has no leading blank line, exactly one blank line separates a leading Field block from the first Section, exactly one blank line separates root Sections, nested nodes have no blank lines, and a complete document ends with exactly one newline. Labels contain lowercase ASCII words or digits separated by one space or hyphen. Empty values, lists, records, and sections are invalid, and explicit emptiness renders as a scalar such as `efforts: none`. The domain severity rank tokens remain `error` and `warn`, while grouped presentation category labels use the readable plurals `errors` and `warnings`.

2. `decision: closed-presentation-tree` One package owns a closed, bounded presentation node tree and its sole renderer. The root Document admits Field and Section nodes. A Section admits Field, Section, List, RecordGroup, and Steps nodes; RecordGroup admits only fixed-arity Record leaves under an unprinted schema. A List's section label names the common plural entity and its normalized single-line leaves render bare at the next indentation level, without a repeated item label. Steps render under their section as `step 1: value`, `step 2: value`, and so on. Section nesting is limited to three levels, and the tree exposes no raw-text node. Constructors validate labels, shapes, record arity, normalization, and escaping. Scalar, list, step, and record values reject CR and LF. A prose constructor trims and collapses Unicode whitespace to one ASCII space; a literal constructor preserves meaningful horizontal spaces. Compact record fields replace `\` with `\\` and `|` with `\|`; other nodes need no delimiter escaping. Rendering validates the complete tree into a buffer before one destination write, so invalid presentation cannot leak partial bytes. The package exists to enforce the one text contract, not to support speculative JSON, Markdown, color, or alternate-renderer visitors.

3. `decision: standard-result-shapes` The presentation package owns standard presentation-level shapes for categorized reports, mutations, diagnostics, details, and collections, all of which lower into the closed tree. A report carries status, context, summary, and semantic record categories. A mutation carries status, identity, grouped changes, notes, and next actions. A diagnostic carries the observed condition, optional state category, safety-relevant changed axes, optional cause, and ordered next actions. These are representation types only: domain packages retain their typed results and decide the status meaning, labels, category names, record schemas, values, ordering, severity, and remedies. The first whole-capability consumers are the audit presentation for Report, project sync presentation for Mutation, commit-policy outcome presentation for Diagnostic, context query presentation for Detail, and config-reference presentation for Collection; a shape without a concrete consumer is removed rather than retained speculatively.

4. `decision: semantic-mapping-ownership` Presentation ownership means that the package owning a result model owns the semantic mapping from that model into the central presentation shapes; the central package alone owns syntax validation and text rendering. Mapping lives separately from business logic so operation tests assert typed results, presentation tests assert semantic labels and ordering, and the central package tests grammar. Command code keeps argument parsing, presentation-versus-bypass selection, stream choice, and exit mapping. Structured command help is model data rather than free-form pre-rendered bodies: the command specification owns usage forms, descriptions, details, positionals, options, examples, and related commands and maps them into the same tree.

5. `decision: typed-command-boundary` The command boundary distinguishes a produced failing report from a failure to produce a report. A complete report, including one whose status requires a nonzero process exit, writes to stdout without a duplicate stderr error. Usage and operational failures render one diagnostic to stderr. A partial mutation diagnostic states every safety-relevant changed axis before its recovery steps. The boundary carries typed presentation and exit information rather than using returned prose as both error identity and output. Checks and migrations collect typed step or change results before presentation; continuation policy remains domain-owned and does not authorize eager unstructured writes.

6. `decision: interactive-presentation` Interactive prompts are a validated presentation mode, not a raw-output exemption. The presentation package buffers and writes the prompt prelude under the ordinary tree grammar, then writes and flushes one validated `prompt:` tail without a newline before input is read. This is the only ordinary presentation mode allowed to write before a complete command result exists.

7. `decision: explicit-bypasses` Every output that bypasses the presentation tree is explicitly classified as a payload or machine protocol and tested byte-for-byte. Authored plan projections and changelog content are payloads. Effort activity JSON, init descriptor JSON, and the context spill notice are protocols. Bypasses never mix presentation text into their bytes. Version output is ordinary presentation and its in-repository consumers parse the labeled contract. Optional JSON modes for effort new, list, and show and for topic are removed; JSON remains only where the command contract is itself a machine protocol.

8. `decision: enforced-adoption` Every ordinary command surface, including help, prompts, successful results, advisories, reports, refusals, and partial outcomes, converts to the central presentation contract. A structural gate rejects direct formatted command output outside the explicit payload and protocol boundary. Exact presentation tests own public labels, schemas, grouping, order, escaping, stream selection, and exit behavior; long expected outputs are checked-in reviewable test data rather than silently regenerated snapshots. Every repository consumer of ordinary output validates the public labels and field boundaries it reads instead of relying on incidental whitespace positions.

9. `decision: package-and-authority-home` The new presentation package belongs to the code-design domain because it owns a repository-wide representation pattern rather than tooling command semantics. The global presentation-ownership topic owns the cross-cutting node grammar, closed standard result-shape set, and semantic-mapping boundary. The path-scoped presentation-package topic covers `internal/presentation/**` and owns its package-local dependency and representation-only boundary without duplicating the global syntax contract. The tooling CLI topic owns the public text grammar, typed command boundary, explicit bypass set, structured help contract, and command-specific JSON retirement. The context-query boundary continues to assign query and semantic mapping ownership to `internal/contextq`, while the presentation package owns syntax rendering. Outcome modeling continues to own diagnostic meaning and retry safety, with its flattened remedy rendering revised to use the central Steps shape.

10. `decision: claim-backing` The four added invariants are mechanically backed. `closed-presentation-tree` is proven by `TestPresentationTreeContract`; `readable-text-output` by `TestOrdinaryCommandOutputUsesPresentation`; `typed-command-output-boundary` by `TestCommandOutputBoundary`; and `explicit-output-bypasses` by `TestExplicitOutputBypasses`. Each proof unit carries its matching current-state marker when the claim is applied.

## State changes

- update `code-design/presentation-ownership:model-owner-renders`
- add `code-design/presentation-package:presentation-package-boundary`
- add `code-design/presentation-ownership:closed-presentation-tree`
- update `tooling/context-and-topic:context-query-boundary`
- update `code-design/outcome-modeling:actionable-outcome-protocol`
- update `tooling/cli:cli-command-spec-single-source`
- update `tooling/cli:effort-command-contract`
- update `tooling/audit-commands:severity-single-spelling`
- add `tooling/cli:readable-text-output`
- add `tooling/cli:typed-command-output-boundary`
- add `tooling/cli:explicit-output-bypasses`

## Consequences

Every ordinary command gains one inspectable hierarchy and delimiter rule. High-volume reports can
lead with counts, group errors and warnings, and retain one compact record per finding without
turning terminal alignment into an API. Low-volume results remain readable labeled blocks. Agents
can consume the default interface without selecting a parallel machine mode, and human readers no
longer pay the cost of JSON or compressed key-value lines.

The node tree makes malformed shape, excessive nesting, inconsistent record arity, and partial
rendering mechanically rejectable. Central syntax tests replace repeated indentation and escaping
tests, while domain tests still protect the public semantic contract. Separating typed operation
results from their presentation reduces prose assertions in business-logic tests, but does not
reduce output compatibility coverage: exact text remains an intentional public contract at the
presentation and command boundaries.

The central package becomes a dependency of many model-owning packages. That dependency is
accepted because it contains representation vocabulary only; allowing packages to recreate the
grammar would defeat the decision. Its path-scoped package topic owns that local dependency and
representation boundary, while the global presentation topic remains the one syntax authority.
The package must remain closed and small. Adding a renderer,
raw node, unbounded nesting, or domain fields to it requires a later decision rather than local
convenience.

The migration is broad. Checks and upgrades must stop streaming prose, help specifications become
structured data, model owners gain semantic mapping files, and shell consumers move with changed
labels. Buffering complete presentations adds bounded memory use proportional to output size;
context already renders completely before applying its cap, and other command outputs are small
enough that deterministic atomic presentation is worth that cost. Interactive prompts retain the
necessary early flush through their single governed mode.

Removing optional JSON modes is a deliberate pre-1.0 compatibility break. Required machine
protocols remain isolated and exact. New machine consumers should prefer the deterministic text
contract unless they genuinely require a separately governed protocol; convenience alone does not
earn a second renderer.

A multi-phase implementation plan is required. The state claims apply with their matching code and
test transactions rather than becoming authority before the converted surfaces exist.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep package-owned ad hoc text and clean up each command locally | Improves examples but leaves syntax policy duplicated and unenforceable, so future commands drift again. |
| Introduce a universal command result model and renderer | Centralizes domain meaning, weakens model ownership, and forces unrelated operations into one generic map or hierarchy. |
| Make JSON or JSON Lines the default | Mechanically easy to parse but less readable, duplicates authored payload concerns, and contradicts the primary agent interface chosen for awf. |
| Keep readable text plus optional JSON everywhere | Doubles compatibility and test surface without a demonstrated protocol consumer. |
| Use padded tables or Markdown-like output | Optimizes terminal appearance rather than stable records and introduces alignment and decorative syntax with no semantic value. |
| Let prompts and migrations bypass central presentation | Preserves eager writes but leaves two of the most complicated output producers outside the rule the decision exists to establish. |
| Store raw strings in a generic node tree | Makes the tree nominal rather than enforcing; arbitrary text would recreate every current inconsistency behind one type. |

## Status history

- 2026-08-04: Proposed
