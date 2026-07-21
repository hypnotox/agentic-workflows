The cmd packages and their spec helpers implement the awf command surfaces and their dispatch. The claims below capture the current CLI behaviour contracts.

## Claims

### `invariant: init-unborn-head-supported`

Working-state assembly uses an empty committed baseline only when HEAD is specifically unborn, allowing init and check to consume eligible working files while every other repository, reference, and object error remains a failure.
Origin: ADR-0139
Backing: test

### `invariant: add-applies-closure-plan`

Adding an artifact enables its full missing forward requirement closure in a single config rewrite and prints one provenance plan line per enabled node, naming which requirement pulled each dependency in.
Origin: ADR-0081
Backing: test

### `invariant: add-skill-pairs-agent`

Enabling a reviewing skill whose required reviewer agent is not yet enabled adds that agent to the agents array in the same config rewrite.
Origin: ADR-0050
Backing: test

### `invariant: adr-new-version-gated`

awf new adr runs the binary-version compatibility gate before it reads or writes any project file.
Origin: ADR-0042
Backing: test

### `invariant: audit-empty-range-announced`

When the resolved audit range contains zero commits, `awf audit` prints a distinct notice naming the range and stating that no history rule was evaluated, never the bare clean line, and still exits zero with zero findings.
Origin: ADR-0127
Backing: test

### `invariant: audit-reports-evaluated-scope`

Every `awf audit` run prints the resolved range and its commit count on its verdict, whether clean, warning, or error, so no verdict is emitted without the scope that produced it.
Origin: ADR-0127
Backing: test

### `invariant: audit-requires-explicit-range`

`awf audit` invoked with no positional range argument exits non-zero without evaluating any rule, and its refusal message names both the bare-base and the a..b accepted forms.
Origin: ADR-0127
Backing: test

### `invariant: audit-scopes-descriptor-routed`

A non-empty commit-scopes init answer is written to `audit.allowedScopes` (comma-split and trimmed), while an empty answer writes no audit block at all, leaving scopes accept-any.
Origin: ADR-0051
Backing: test

### `invariant: audit-warn-exit-zero`

The awf audit command returns success with a zero exit status when all of its findings are Warning severity, and returns a non-zero exit status when any single finding is Error severity.
Origin: ADR-0017
Backing: test

### `invariant: cli-command-spec-single-source`

The top-level usage line, awf help overview and order, generated gated-command list, and managed-runner forwarding dispositions all derive from the clispec command table, with no parallel command-order, gated-command, or runner-availability list.
Origin: ADR-0094
Revised-by: ADR-0144
Backing: test

### `invariant: cli-config-kinds`

The enable and disable commands operate on exactly four kinds, skill, agent, doc, and domain, each mapping to its plural enable array in the config. The three catalog-backed kinds are validated against the catalog pool, while the freeform domain kind is validated through the config path-safety rule.
Origin: ADR-0024
Backing: test

### `invariant: completeness-advisory-nonfailing`

The unset-variable advisory notes that `awf check` prints for under-configured artifacts are informational only and never change the command's exit code.
Origin: ADR-0045
Backing: test

### `invariant: config-command-static-fallback`

Run outside an adopted project, where no config file is present, the config command prints the static catalog-wide reference labeled as a static, not-inside-a-project reference and returns success instead of refusing. The static output lists catalog keys, vars, sidecar fields, and consumers but carries no live project state.
Origin: ADR-0088
Backing: test

### `invariant: context-default-excludes-history`

Normal path context returns active current-state claims without expanding Implemented ADRs or historical plans.
Origin: ADR-0134
Backing: unbacked
Verify: On a fixture with claim provenance, ADR tags and relations, and linked plans, bounded default context differs from explicit awf topic <claim-id> --history output.

### `invariant: context-output-parity`

The human-readable and --json renderings of `awf context` report the same underlying context set for the same inputs, because both are produced from one assembled context struct.
Origin: ADR-0092
Backing: test

### `invariant: context-read-only`

The `awf context` command only reads committed state: file modification times and the lock bytes are byte-identical before and after every branch of the command, so it never writes to disk or mutates config or the lock.
Origin: ADR-0092
Backing: test

### `invariant: context-static-fallback`

Run outside an adopted tree, where no config file is present, `awf context` degrades to a static empty answer and succeeds rather than refusing, mirroring `awf config`.
Origin: ADR-0092
Backing: test

### `invariant: describe-read-only`

awf init --describe prints the var descriptor set as JSON to stdout and creates no files under the target root.
Origin: ADR-0029
Backing: test

### `invariant: explicit-answers-win`

A value supplied to awf init via --set or --answers is written verbatim into the scaffolded config and suppresses any prompt for that key, regardless of whether stdin is a terminal.
Origin: ADR-0029
Backing: test

### `invariant: gated-commands-generated`

The gated-command list rendered into the managed docs is generated from the clispec command table (exactly the top-level commands whose gating classification is not ungated) through one generator feeding both the render placeholder and the agent-guide value, with no hand-maintained enumeration in either doc.
Origin: ADR-0094
Backing: test

### `invariant: init-collision-guard`

Before writing anything, `awf init` pre-flights every path it would create and, if any already exists, writes nothing and reports the offending paths; `awf init --force` backs up each colliding file to `<path>.awf-bak` and overwrites.
Origin: ADR-0016
Backing: test

### `invariant: init-force-backs-up`

Running init with --force copies every colliding non-managed file to <path>.awf-bak before any managed output overwrites it, and reports the backup on stdout.
Origin: ADR-0023
Backing: test

### `invariant: init-hooks-default-on`

The config scaffolded by awf init enables the hooks singleton by default.
Origin: ADR-0048
Backing: test

### `invariant: init-noninteractive-default`

awf init with a non-terminal stdin and no --set or --answers seeds every var empty and writes no invariants config, producing output byte-identical to the plain seed-empty scaffold.
Origin: ADR-0029
Backing: test

### `invariant: init-prompts-enabled-vars`

Interactive awf init prompts only for the vars referenced by the chosen enabled set's templates, while the seeded config still carries the full catalog var union as empty keys.
Origin: ADR-0086
Backing: test

### `invariant: init-set-closed`

The enabled set that init scaffolds, whether the curated default or a closure-completed trim, satisfies the requirement-closure rule: every enabled node's catalog requirements are also enabled.
Origin: ADR-0081
Backing: test

### `invariant: invariants-in-check`

Running `awf check` evaluates the current-state topic corpus and exits non-zero, printing the finding, whenever that evaluation reports an error-severity issue, and stays clean when it reports none.
Origin: ADR-0007
Backing: test

### `invariant: managed-runner-command-parity`

Every clispec command declared runner-forwarded appears in the generated adopter runner and this repository's source runner, while each excluded command carries a reason and appears in neither; forwarded names cannot collide with adopter project verbs.
Origin: ADR-0144
Backing: test

### `invariant: mutants-missing-report-errors`

The mutants command exits non-zero for a nonexistent report path and never prints no survived mutants for one, while a present-but-empty report file reports no survivors with exit 0.
Origin: ADR-0071
Backing: test

### `invariant: new-seeds-scaffold-vars`

awf new adds an empty vars entry for every variable referenced by the scaffolded template source that is absent from config, leaving already-present keys untouched and surfacing the editor's error on a malformed source.
Origin: ADR-0087
Backing: test

### `invariant: remove-agent-pairing-guard`

Disabling an agent refuses upfront, leaving the config file unchanged, while any enabled non-local skill still requires that agent.
Origin: ADR-0050
Backing: test

### `invariant: remove-refuses-dependents`

Without the cascade flag, remove refuses while enabled transitive dependents exist and prints the dependent plan; with --with-dependents it removes the full reverse closure in a single config rewrite.
Origin: ADR-0081
Backing: test

### `invariant: repoaudit-requires-explicit-range`

The repoaudit command invoked with no range argument exits non-zero with its usage line and evaluates no rule; a supplied bare base is also rejected because repoaudit does not opt into the parser's bare-base form.
Origin: ADR-0127
Backing: test

### `invariant: single-os-exit`

Within the cmd/awf package, os.Exit appears only in main.go's main function, whose body is the single os.Exit(run(...)) wrapper; no other production source in the package calls os.Exit and no fatal or fatalIf helpers exist.
Origin: ADR-0012
Backing: test

### `invariant: single-version-authority`

The command-line tool resolves its version from a single authoritative version constant. The version-reporting entry point returns exactly that constant, so there is one source of truth for the binary's version.
Origin: ADR-0049
Backing: test

### `invariant: stub-advisory-nonfailing`

Unreplaced stub sections and stub-marked convention parts never by themselves cause awf check or any other gated command to exit non-zero.
Origin: ADR-0070
Backing: test

### `invariant: target-cli`

The add, remove, and list target commands mutate and read the config targets array against the known-adapter set without routing through the kind/catalog/parts/orphan machinery, and enabling a target renders its output tree.
Origin: ADR-0037
Backing: test

### `invariant: uncovered-collapses-directories`

In the coverage report, a directory all of whose scanned tracked descendants are owned by no domain is reported as that single topmost directory with a trailing slash, never as its individual files.
Origin: ADR-0102
Backing: test

### `invariant: uncovered-output-parity`

The human-readable and JSON renderings of the coverage report present the same uncovered and unowned sets, because both are printed from one assembled result.
Origin: ADR-0102
Backing: test

### `invariant: upgrade-always-syncs`

`awf upgrade` runs a full sync on every successful invocation, including the zero-migrations case, where it reports that the config is already at the current schema and still re-renders every managed file.
Origin: ADR-0085
Backing: test

### `invariant: version-compat-gate`

Every gated command routes through gate(), which refuses to proceed when the running binary is behind the project on either axis: the config schema generation exceeds the binary's current generation, or the lock's awfVersion is semver-greater than the binary's version. A binary at or ahead of the project on both axes is permitted.
Origin: ADR-0039
Backing: test
