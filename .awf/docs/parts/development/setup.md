| Requirement | Notes |
|---|---|
| Go 1.26+ | See `go.mod`. |
| Native Git | Required at runtime and in tests. |
| Node v24.19.0 and npm | `.nvmrc` pins the Pi lane runtime. Local NVM selects it without downloading; run `nvm install v24.19.0` if absent. Hosted workflows set `AWF_PI_TEST_SKIP_NVM=1` after setup-node and the runner still rejects any runtime that is not the exact pin. |
| `hypnotox/pi-tools` for Pi use | Install independently at any protocol-v2-compatible revision; awf does not pin it. |

For adopter Pi sessions, compatibility means a successful protocol-v2 capability handshake and final awf profile registration. Missing, incompatible, late, or rejected negotiation reports an actionable prerequisite error and activates no awf fallback. The awf effort extension separately requires a compatible adopter-supplied Pi runtime.

The Pi lane requires the pinned host Node and npm, but no services, environment variables, or model credentials.

```sh
git clone <repo>
cd agentic-workflows
./x test
git config core.hooksPath .githooks
```

Tools in `go.mod`'s `tool` block resolve through `go tool`; do not install them manually. The effective global identity is `Josua Müller <hypnotox@pm.me>` with `commit.gpgSign=true`, `gpg.format=ssh`, and the approved `user.signingKey`. Do not set repository- or worktree-local identity overrides. Before moving intended history, preview it with `./awf check commit-policy <revision-or-range>...`.
