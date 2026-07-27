## Setup

A working checkout needs Go 1.26+ (see `go.mod`) and Docker. No host Node, npm,
`node_modules`, services, environment variables, or model credentials are required. Clone the
repo and run `./x test` to confirm the Go toolchain; `./x gate` creates the Pi-extension test
container on first use. Developer tools (`golangci-lint`, `deadcode`, `gremlins`) are pinned in
`go.mod`'s `tool` block and run through `go tool`, so they resolve on first use; nothing
is installed by hand. To activate the repo's git hooks, wire them once per clone with
`git config core.hooksPath .githooks`.
