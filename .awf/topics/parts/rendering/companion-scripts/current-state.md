Rendered companion script contracts: the bootstrap and upgrade scripts, the command runner, and hook payload fallback behaviour.

## Claims

### `invariant: dashboard-development-runtime-commands`

The awf repository's editable runner project-verb region advertises `dashboard-awf-path` and `dashboard-awf-advance [commit]`: path prints only the resolved immutable launcher path to standard output with diagnostics on standard error, while advance reports the old commit, new commit, and launcher path after publish-before-compare-and-swap advancement. The generic rendered runner omits both repository-only commands, so adopters do not acquire source-layout assumptions.
Origin: ADR-0150
Backing: test

### `invariant: bootstrap-checksum`

The rendered `awf-bootstrap.sh` performs a SHA-256 checksum verification of the downloaded archive before it installs the binary, so the download is always integrity-checked ahead of use.
Origin: ADR-0148
Backing: test

### `invariant: bootstrap-env-override`

The rendered bootstrap script's version assignment is the default-expansion form AWF_VERSION set to the pattern that prefers a pre-set AWF_VERSION and otherwise expands to the rendering binary's version, so an environment override wins and, absent one, the script resolves exactly the version of the binary that rendered it.
Origin: ADR-0148
Backing: test

### `invariant: bootstrap-local-first`

The rendered bootstrap installer probes for an awf binary already on PATH before downloading anything. When a local binary reports exactly the pinned target version, the script uses it and exits before reaching any download step.
Origin: ADR-0148
Backing: test

### `invariant: bootstrap-stdout-path-only`

The rendered bootstrap installer writes only the resolved binary path to standard output. Every diagnostic line is a comment or is redirected to standard error, so nothing but the binary path reaches standard output.
Origin: ADR-0148
Backing: test

### `invariant: hook-payloads-fallback-safe`

With checkCmd, gateCmd, gateCmdFull, and commitGateCmd all unset, every rendered hook payload is a runnable script whose commands degrade to the generic awf forms, carrying no unresolved-value token.
Origin: ADR-0148
Backing: test

### `invariant: runner-example-adopted`

The bundled sundial example enables the runner singleton, and its rendered `awf` wrapper is drift-free, invariant-clean, and free of advisory notes; its project verbs live in a hand-written `./x` outside the render set, and its config carries no awf-verb command vars, so it dogfoods the rendered defaults.
Origin: ADR-0148
Revised-by: ADR-0156
Backing: test

### `invariant: runner-prune-backup`

A lock prune that removes a co-owned runner output (an outgoing lock entry whose template id is `runner/x.tmpl`) backs the file up through the standard backup path (`x.awf-bak`, collision-suffixed) instead of deleting it, and still records the path as pruned.
Origin: ADR-0156
Backing: test

### `invariant: runner-pure-forwarder`

With the runner singleton enabled, the rendered wrapper at the repo-root path `awf` contains no per-verb dispatch and no in-place-editable region: it resolves one awf invocation and execs it with all arguments forwarded verbatim.
Origin: ADR-0156
Backing: test

### `invariant: runner-render-publication-safe`

The runner template renders leak-free under empty data, producing no unresolved token and no stray section or marker residue, like every other awf template.
Origin: ADR-0148
Backing: test

### `invariant: runner-resolution-pinned-first`

With `vars.awfInvokeCmd` set, the rendered wrapper execs exactly that command; with it unset, the wrapper resolves the bootstrap-pinned binary when `.awf/bootstrap.sh` exists and falls back to the PATH `awf` otherwise.
Origin: ADR-0156
Backing: test

### `invariant: runner-singleton-toggle`

With the runner singleton enabled, `awf sync` renders exactly one wrapper file at the repo-root path `awf`; with it disabled or absent, it renders none. `awf init` scaffolding seeds `runner.enabled: true` and the enable-runner migration seeds an absent key to enabled on `awf upgrade`, respecting an explicit false.
Origin: ADR-0148
Revised-by: ADR-0156
Backing: test

### `invariant: upgrade-delegates-fetch`

The rendered `.awf/upgrade.sh` obtains the binary only by invoking `.awf/bootstrap.sh` with AWF_VERSION set; it performs no release-asset download and no checksum of its own, and its single direct network call is the latest-tag redirect probe against releases/latest.
Origin: ADR-0148
Backing: test

### `invariant: upgrade-exec-final`

The rendered `.awf/upgrade.sh` hands off with exec of the fetched binary running upgrade as its final statement, so the shell process is replaced before `awf upgrade` re-renders the script in place.
Origin: ADR-0148
Backing: test
