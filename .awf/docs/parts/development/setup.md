| Requirement | Notes |
|---|---|
| Go 1.27+ | See `go.mod`. |
| Native Git | Required at runtime and in tests. |

AWF development requires no Node runtime, services, model credentials, or globally installed harness packages. Operators install `agentic-skills` globally for generic skills and roles. Pi operators install `pi-tools` separately for role delegation. Neither package is an AWF binary or test dependency, and AWF does not pin, install, update, or probe them.

```sh
git clone <repo>
cd agentic-workflows
./x test
git config core.hooksPath .githooks
```

Tools in `go.mod`'s `tool` block resolve through `go tool`; do not install them manually. The effective global identity is `Josua Müller <hypnotox@pm.me>` with `commit.gpgSign=true`, `gpg.format=ssh`, and the approved `user.signingKey`. Do not set repository- or worktree-local identity overrides. Before moving intended history, preview it with `./awf check commit-policy <revision-or-range>...`.
