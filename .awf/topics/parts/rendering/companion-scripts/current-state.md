Rendered companion script contracts: the bootstrap and upgrade scripts, the command runner, and hook payload fallback behaviour. awf always renders five inert payloads: pre-commit, commit-msg, pre-merge-commit, reference-transaction, and pre-push. An adopter may preview commit provenance with `awf check commit-policy <revision-or-range>...` before configuring policy and wiring its own stubs; the payloads do not activate themselves. Worktree-aware stubs must resolve the invoking worktree before delegating, and local hooks remain a preflight rather than a substitute for the remote's final branch policy.

## Claims

### `invariant: bootstrap-checksum`

The rendered `.awf/bootstrap.sh` performs a SHA-256 checksum verification of the downloaded archive before it installs the binary, so the download is always integrity-checked ahead of use.
Backing: test

### `invariant: bootstrap-env-override`

The rendered bootstrap script's version assignment is the default-expansion form AWF_VERSION set to the pattern that prefers a pre-set AWF_VERSION and otherwise expands to the rendering binary's version, so an environment override wins and, absent one, the script resolves exactly the version of the binary that rendered it.
Backing: test

### `invariant: bootstrap-local-first`

The rendered bootstrap installer probes for an awf binary already on PATH before downloading anything. When a local binary reports exactly the pinned target version, the script uses it and exits before reaching any download step.
Backing: test

### `invariant: bootstrap-stdout-path-only`

The rendered bootstrap installer writes only the resolved binary path to standard output. Every diagnostic line is a comment or is redirected to standard error, so nothing but the binary path reaches standard output.
Backing: test

### `invariant: hook-payloads-fallback-safe`

With checkCmd and gateCmd unset, every rendered hook payload is a runnable script whose awf-verb commands resolve through the always-rendered `./awf` wrapper, carrying no inline resolution shim and no unresolved-value token; the pre-commit payload consumes only the configured aggregate check and project gate.
Backing: test

### `invariant: runner-pure-forwarder`

The always-rendered wrapper at the repo-root path `awf` contains no per-verb dispatch and no in-place-editable region: it resolves one awf invocation and execs it with all arguments forwarded verbatim.
Backing: test

### `invariant: runner-render-publication-safe`

The runner template renders leak-free under empty data, producing no unresolved token and no stray section or marker residue, like every other awf template.
Backing: test

### `invariant: runner-resolution-pinned-first`

The standard rendered wrapper resolves the bootstrap-pinned binary when `.awf/bootstrap.sh` exists and falls back to PATH `awf` otherwise. Repository-specific execution semantics replace the `runner-body` convention part; no invocation var participates in resolution.
Backing: test

### `invariant: runner-wrapper-rendered`

A full render emits exactly one wrapper file at the repo-root path `awf`.
Backing: test

### `invariant: upgrade-delegates-fetch`

The rendered `.awf/upgrade.sh` obtains the binary only by invoking `.awf/bootstrap.sh` with AWF_VERSION set; it performs no release-asset download and no checksum of its own, and its single direct network call is the latest-tag redirect probe against releases/latest.
Backing: test

### `invariant: upgrade-exec-final`

The rendered `.awf/upgrade.sh` hands off with exec of the fetched binary running upgrade as its final statement, so the shell process is replaced before `awf upgrade` re-renders the script in place.
Backing: test
