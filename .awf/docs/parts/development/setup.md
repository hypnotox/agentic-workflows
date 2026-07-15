## Setup

A working checkout needs Go 1.26+ (see `go.mod`) and nothing else: no services,
environment variables, or credentials. Clone the repo and run `./x test` to confirm the
toolchain. Developer tools (`golangci-lint`, `deadcode`, `gremlins`) are pinned in
`go.mod`'s `tool` block and run through `go tool`, so they resolve on first use; nothing
is installed by hand. To activate the repo's git hooks, wire them once per clone with
`git config core.hooksPath .githooks`.
