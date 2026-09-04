AWF documents the external Pi dependency boundary without declaring a runtime capability or implementation.

## Claims

### `invariant: pi-runtime-dependency-separation`

The AWF binary remains functional offline and does not inspect global harness installations. Operators install `agentic-skills` for generic skills and roles and install `pi-tools` separately for Pi role delegation.
Backing: unbacked
Verify: Run fresh initialization, render, check, and upgrade with network access unavailable and neither global dependency present; inspect the resulting output plan and production dependency graph for external installation or probing behavior.
