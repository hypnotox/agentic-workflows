Pi publishes AWF-specific skills while generic skills and role delegation remain operator-installed dependencies.

## Claims

### `invariant: pi-native-awf-skills`

The fixed Pi target renders exactly `awf-effort`, `awf-topics`, `awf-decisions`, and `awf-maintenance` under `.pi/skills/`, independent of the configured project prefix. AWF renders no Pi agents, subagent adapter, model router, role policy, or preference store.
Backing: test

### `invariant: pi-external-role-boundary`

Generic Pi skills and roles come from a globally installed `agentic-skills` package. Role delegation additionally requires a separately installed compatible `pi-tools`; AWF does not install, vendor, update, configure, or probe either dependency.
Backing: unbacked
Verify: Inspect the standard catalog and Pi target declarations, confirm only the four fixed AWF skills remain, and search production code for network, installation, package-probing, role-registration, routing, policy, profile, preference, and runtime-capability behavior tied to either external dependency.
