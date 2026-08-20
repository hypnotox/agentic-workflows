Configuration keys and placeholder semantics are in the [configuration reference](config-reference.md). A convention part replaces its section body; `awf:edit` names its source and `sectionDefault` extends a default.

Local documents are declared in `localDocs` or with `./awf new doc <name> <description> [--title <title>]`. It creates `docs/<name>.md` (`api-v2` becomes `Api V2` without `--title`). Edit only its body between `awf:edit-in-place` and `awf:end`; render and check after editing. Removal or uninstall preserves a present body as `.awf-bak`.

For generated guidance, `awf:edit` names the owning convention part and `awf:source` names reader authority. Use `./awf new pitfall "<Title>"` for an authored pitfall source, then render; never edit its generated index or leaf.

Declared local documents appear in the rendered `AGENTS.md` document map. Ordinary `awf check` validates Markdown links and skill references in their preserved inline bodies; it does not make them catalog documents or widen staged drift.
