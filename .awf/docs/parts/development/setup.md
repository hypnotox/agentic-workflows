| Requirement | Notes |
|---|---|
| Go 1.26+ | See `go.mod`. |
| Native Git | Required at runtime and in tests. |
| Docker | `./x gate` builds the Pi-extension test container on first use. |

No host Node, npm, `node_modules`, services, environment variables, or model credentials are required.

```sh
git clone <repo>
cd agentic-workflows
./x test
git config core.hooksPath .githooks
```

Tools in `go.mod`'s `tool` block resolve through `go tool`; do not install them manually. The effective global identity is `Josua Müller <hypnotox@pm.me>` with `commit.gpgSign=true`, `gpg.format=ssh`, and the approved `user.signingKey`. Do not set repository- or worktree-local identity overrides. Before moving intended history, preview it with `./awf check commit-policy <revision-or-range>...`.
