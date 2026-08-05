
A working checkout needs Go 1.26+ (see `go.mod`), native Git, and Docker. No host Node, npm,
`node_modules`, services, environment variables, or model credentials are required. Clone the
repo and run `./x test` to confirm the Go toolchain; `./x gate` creates the Pi-extension test
container on first use. Developer tools (`golangci-lint`, `deadcode`, `gremlins`) are pinned in
`go.mod`'s `tool` block and run through `go tool`, so they resolve on first use; nothing
is installed by hand. Activate the repo's worktree-aware git-hook stubs once per clone with
`git config core.hooksPath .githooks`. The effective identity must be `Josua Müller <hypnotox@pm.me>` from global configuration, with `commit.gpgSign=true`, `gpg.format=ssh`, and the approved `user.signingKey`; do not add repository- or worktree-local identity overrides. Preview any intended ref history with `./awf check commit-policy <revision-or-range>...` before moving it.
