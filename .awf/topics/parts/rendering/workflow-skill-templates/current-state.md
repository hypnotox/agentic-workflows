AWF publishes a fixed repository-local skill layer over globally installed generic agentic capabilities.

## Claims

### `invariant: fixed-awf-skill-surface`

Both targets render exactly four prefix-independent AWF skills: `awf-effort`, `awf-topics`, `awf-decisions`, and `awf-maintenance`. Their bodies own only AWF effort lifecycle, current-state topics and proof, durable decision records, and generated-source maintenance. They direct general context, brainstorming, debugging, design, planning, implementation, and review behavior to the corresponding globally installed `agentic-*` skills instead of copying generic bodies.
Backing: test

### `invariant: repository-awf-invocation`

Every rendered agent-facing executable AWF instruction invokes the unconditional repository-root `./awf` wrapper. Product and CLI grammar, and bootstrap or wrapper PATH resolution, remain bare `awf` where they describe rather than invoke the repository command.
Backing: test

### `invariant: awf-skill-prose-tool-agnostic`

The four rendered AWF skill bodies contain no role-registration or runtime-specific subagent tool syntax. Harness-specific installation and capability guidance remains in runtime documentation rather than the skills.
Backing: test
