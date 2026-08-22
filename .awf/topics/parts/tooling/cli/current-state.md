The cmd packages and their spec helpers implement the awf command surfaces and their dispatch. Effort commands manage optional durable coordination, memory, and managed worktrees.

## Claims

### `invariant: cli-creation-and-inventory`

The CLI lists fixed catalog inventory and never selects individual catalog membership. Full creates authored ADRs, plans, topics, and domains; both governance footprints retain pitfalls and additive local documents, while the selected footprint controls rendered membership.
Origin: ADR-0254
Revised-by: ADR-0262, ADR-0272, ADR-0278, ADR-0292
Backing: test

### `invariant: pitfall-scaffold`

`awf new pitfall <title>` accepts exactly one complete title positional, gates before project reads or writes, loads the current authored corpus, refuses empty, reserved, or whitespace-and-case-equivalent duplicate titles, uses the shared ASCII slug allocator and canonical serializer, creates exactly one selected source path exclusively, and reports that repository-relative authored path through project-owned presentation. Occupied suffix gaps choose the first free candidate; a race at the selected path refuses without advancing, while an ordinary retry reloads and recomputes. The command never renders or mutates generated output, a sidecar, or another registry.
Origin: ADR-0262
Backing: test

### `invariant: domain-lifecycle-commands`

`awf new domain` validates and scaffolds a configured domain without clobbering authored parts; `awf remove domain` prunes rendered output and reports authored residue as orphaned.
Origin: ADR-0254
Backing: test

### `invariant: adr-new-version-gated`

awf new adr runs the binary-version compatibility gate before it reads or writes any project file.
Origin: ADR-0042
Backing: test

### `invariant: cli-command-spec-single-source`

The top-level usage line, awf help overview and order, structured command help model data (usage forms, descriptions, details, positionals, options, examples, and related commands), generated gated-command list, and bounded root README command block in top-level clispec order all derive from the clispec command table, with no parallel independent command-order membership decision and no parallel gated-command list.
Origin: ADR-0094
Revised-by: ADR-0144, ADR-0156, ADR-0234, ADR-0242
Backing: test

### `invariant: help-lists-group-children`

The awf help overview lists every group command's descendants at every depth beneath their parent with successively deeper indentation, so no command is reachable only by knowing to ask a parent for help.
Origin: ADR-0159
Revised-by: ADR-0210
Backing: test

### `invariant: completeness-advisory-nonfailing`

The unset-variable advisory notes that `awf check` prints for under-configured artifacts are informational only and never change the command's exit code.
Origin: ADR-0045
Backing: test

### `invariant: config-command-static-fallback`

Run outside an adopted project, where no config file is present, the config command prints the static catalog-wide reference labeled as a static, not-inside-a-project reference and returns success instead of refusing. The static output lists catalog keys, vars, sidecar fields, and consumers but carries no live project state.
Origin: ADR-0088
Backing: test

### `invariant: explicit-output-bypasses`

Only authored plan projections, selected changelog content, effort activity JSON, owner-scoped effort memory protocol JSON, init descriptor JSON, and the context spill notice bypass the presentation tree on successful output, each under byte-exact tests. Memory protocol JSON is a required protocol-1 machine envelope selected only by mutually required `--owner` and `--json`, while owner-free memory output remains ordinary presentation; its writer requires the complete newline-terminated envelope and treats a short write as failure. The exact `writeRendererFailure` terminal mechanism diagnostic is separate, is reachable only after presentation rendering fails, and is not an alternate successful renderer.
Origin: ADR-0234
Revised-by: ADR-0239
Backing: test

### `invariant: gated-commands-generated`

The gated-command list rendered into the managed docs is generated from the clispec command table through one generator feeding both the render placeholder and the agent-guide value, with no hand-maintained enumeration in either doc. It is the single projection of top-level commands whose gating classification is not ungated, with no group-child exclusion list.
Origin: ADR-0094
Revised-by: ADR-0159, ADR-0210
Backing: test

### `invariant: group-child-project-guard-exemption`

The current-state journal and attestation guard reads the deepest resolved command's exemption property, so `awf check staged commit` stays runnable in the states where its commit-msg hook must still function while the repo scan children remain guarded.
Origin: ADR-0159
Revised-by: ADR-0210
Backing: test

### `invariant: invariants-in-check`

Under Full, `awf check` evaluates the current-state topic corpus and propagates error findings to a non-zero result. Core does not load or evaluate that governance corpus.
Origin: ADR-0007
Revised-by: ADR-0210, ADR-0278
Backing: test

### `invariant: check-universe-groups`

The check command groups repository drift, prose, and memory checks in both governance footprints; Full additionally aggregates current-state and plan-artifact checks. Staged likewise selects its Full governance checks, and outside Git the bare form reports staged unavailable.
Origin: ADR-0210
Revised-by: ADR-0217, ADR-0278, ADR-0292
Backing: test

### `invariant: plan-read-command`

The gated `awf read plan <plan> <P[.T]>` command resolves only an exact plan filename or stem under the configured plans directory and only canonical positive numeric phase or task selectors. Failures retain plan-owned selector identities and available exact values. Plan-v2 success writes the internal/plan-rendered ordered Applying then Context Decision and phase-owner outcome closure unchanged, with first-authored resolved-key deduplication, Applying precedence, task scope safety, whole-plan Definition-of-done exclusion, and preserved source; plan-v1 bytes remain unchanged. Blocking references fail while assignment notes remain non-blocking. It neither includes other phases nor mutates source.
Origin: ADR-0213
Revised-by: ADR-0217
Backing: test

### `invariant: readable-text-output`

Every ordinary awf command surface, including help, prompts, results, advisories, reports, refusals, and partial outcomes, uses the central deterministic readable-text presentation contract with stable labels, semantic grouping, ordering, escaping, stream selection, and newline behavior.
Origin: ADR-0234
Backing: test

### `invariant: repo-check-capability-plan`

The direct drift, state, prose, and memory repository checks and their aggregate select from one closed capability plan. One operation loads working config once, conditionally opens one Project from that prepared config, derives one complete CheckReport and one working CurrentStateReport when selected, and captures one shared stage-0 index whenever either always-on scanner is selected; scanner-only selections acquire no unrelated capability. Repository drift presents its dedicated non-failing generated-artifact tracking-unavailable information both directly and in the aggregate, while aggregate-only render advisories remain absent from direct drift. RepositoryChecker consumes the completed owner results without preparing inputs or selecting check policy, preserves source order within each severity, and presents deterministic `errors`, `warnings`, then `information` categories. The aggregate executes drift, state, prose, then memory, continues after action errors and returns the first, while any preparation failure executes no step; the working report, current-state, and index universes never substitute for one another.
Origin: ADR-0223
Revised-by: ADR-0234, ADR-0253, ADR-0277, ADR-0295, ADR-make-repository-check-results-owner-classified
Backing: test

### `invariant: check-severity-by-protected-property`

Each semantic check owner emits immutable results in which every ranked finding names its fixed Error or Warning rank and protected property. No consumer recovers classification from evidence kind, presentation category, or slice placement. Every `awf check` Error protects correctness, safety, authority, or reproducibility and makes the command exit nonzero. Style, readability, plan-detail, fan-out, and other heuristic findings use Warning and exit zero. Optional improvements, unused vocabulary, context suggestions, non-blocking compatibility notices, and successful operation notes remain unranked Information and exit zero. Direct and aggregate readable output visibly separates `errors`, `warnings`, and `information`; information is not a third finding rank.
Origin: ADR-0295
Revised-by: ADR-make-repository-check-results-owner-classified
Backing: test

### `invariant: single-os-exit`

Within the cmd/awf package, os.Exit appears only in main.go's main function, whose body is the single os.Exit(run(...)) wrapper; no other production source in the package calls os.Exit and no fatal or fatalIf helpers exist.
Origin: ADR-0012
Backing: test

### `invariant: single-version-authority`

The newline-terminated `internal/project/VERSION` file is the command-line tool's single version authority. The binary embeds it as `project.Version`; version reporting, lock stamping, bootstrap pinning, changelog checks, and schema compatibility consume that exact value, while build provenance remains display-only. The unconditional versioncheck gate rejects noncanonical file bytes, a divergent exposed value, invalid no-`v` SemVer, a missing current schema minimum, or a binary version below that minimum.
Origin: ADR-0049
Revised-by: ADR-0284
Backing: test

### `invariant: stub-advisory-nonfailing`

Unreplaced stub sections and stub-marked convention parts never by themselves cause awf check or any other gated command to exit non-zero.
Origin: ADR-0070
Backing: test

### `invariant: terseness-advisory-nonfailing`

The glossary terseness findings that `awf check` prints for over-long term meanings use the Warning rank and never change the command's exit code.
Origin: ADR-0207
Revised-by: ADR-0295
Backing: test

### `invariant: typed-command-output-boundary`

The command boundary carries typed presentation and exit information: complete produced reports write once to stdout even when their status requires nonzero exit, usage and operational failures render one diagnostic to stderr, and renderer failures alone use the exact terminal mechanism fallback without duplicate output.
Origin: ADR-0234
Backing: test

### `invariant: upgrade-always-syncs`

`awf upgrade` runs a full sync on every successful invocation, including zero migrations, re-rendering every managed file in the selected governance footprint.
Origin: ADR-0085
Revised-by: ADR-0278, ADR-0292
Backing: test

### `invariant: version-compat-gate`

Every ordinary gated command routes through gate(), which refuses to proceed when the running binary is behind the project on either axis: the config schema generation exceeds the binary's current generation, or the lock's awfVersion is semver-greater than the binary's version. A binary at or ahead of the project on both axes is permitted.
Origin: ADR-0039
Revised-by: ADR-0150, ADR-0153, ADR-0162
Backing: test

### `invariant: effort-command-contract`

`awf effort` exposes schema-2 readable-text `new --slug <slug> <outcome-title> [--no-worktree] [--base <ref>]`, active-only `list`, active-only `show <slug>`, archival `finish <slug>`, `worktree add <slug> [--base <ref>]`, `worktree remove <slug>`, `integrate <slug>`, owner-free `memory read <slug> [--offset <positive-line>] [--limit <positive-lines>]`, `memory edit <slug>`, `memory update <slug> [--phase <text>] [--next <text>]`, protocol-v2 JSON-only `activity attach|heartbeat|detach`, and the corresponding owner-scoped protocol-1 memory forms with mutually required nonrepeatable `--owner <uuid> --json`. New requires the nonrepeatable explicit slug value flag around the one independent title positional through interspersed ordering and validates grammar before composition; other flags and subcommands retain their combinations. Memory edit alone reads one closed 1-through-128-edit JSON object from stdin under 16 MiB, with closed edit objects and one-MiB-bounded UTF-8 strings; malformed grammar, stdin, bounds, or pre-observation failures use nonzero exit, empty stdout, and one complete UTF-8-safe actionable stderr diagnostic bounded to 50 KiB. Owner-free memory successes and handled refusals use effort-owned semantic mappings through ordinary presentation; owner-scoped handled results exit zero through exactly one newline-terminated protocol-1 envelope bounded to one MiB, with only publication uncertainty carrying a cause and `changedMemory` true only after atomic replacement. Activity handled replies retain exact condition-specific newline-terminated envelopes and the sole `changedActivity` mutation axis. Finish validates topology, the active or reserved resident, the exact archive root, and the command-composed fully rendered marker before active mutation; it reports the primary-root-qualified archive destination on completed archival, using `.awf/effort-archive/<uuid>-<slug>` from the primary checkout and the absolute primary-root path from a linked checkout and reports typed reservation, move, destination-parent sync, source-parent sync, and exact inspection actions on partial outcomes. Readable new/show/list presentations preserve schema-2 active resident facts, worktree behavior, primary-root-qualified memory paths, and unrelated command availability. There is no archive inventory, selection, restore, prune, analysis, retention, force-delete, finish bypass, resolve, checkout, destination, CWD, role, receiving-checkout activity action, rename, standalone memory, lifecycle ledger, manual integration, authoritative assignment, or force command. Owner-scoped `memory edit` and `memory update` additionally accept nonrepeatable `--preview` only with `--owner` and `--json`; edit preview emits `previewed` with replacementCount and diff, update preview emits `previewed` with diff only, and neither carries memory. Normal update success retains memory and requires its authoritative diff; owner-free and non-preview mutation contracts are unchanged.
Origin: ADR-0164
Revised-by: ADR-0167, ADR-0175, ADR-0189, ADR-0218, ADR-0225, ADR-0226, ADR-0234, ADR-0239, ADR-0244, ADR-0259
Backing: test
