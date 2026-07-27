{{=awf:sectionDefault}}

For managed `awf context` calls, start bare, request only the named facets required by the active lens, and never prescribe `--full`. When the command returns a valid spill notice, consume the complete packet, verify its declared byte length, and best-effort delete its temporary file after successful or failed use.

Command specifics, metrics and lifecycle contracts, and upgrade behaviour: see docs/working-with-awf.md.
