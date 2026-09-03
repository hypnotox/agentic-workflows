{{=awf:sectionDefault}}

Author one declared convention part by semantic identity with `./awf edit <kind> <name> <part> --content <text>` or `./awf edit <kind> <name> <part> --stdin`. Exactly one input mode is required, and an explicitly empty `--content` value remains an authored override. The closed kinds are `doc`, `skill`, and `domain`; names and parts come from the selected catalog and project configuration rather than filesystem paths.

Use `./awf reset <kind> <name> <part>` to remove a convention-part override and restore its inherited default. A configured local document is addressed as `doc <name> body`; edit replaces only its body and reset restores the empty body while preserving its declaration and awf-owned shell. Both commands validate the complete candidate project before changing the source, then render and update the lock. If a later source or publication step fails, follow the reported residue-first recovery actions and rerun `./awf render`; no rollback is implied.

Use `./awf edit sidecar <kind> <name> <field>` for one leaf-only dotted sidecar field and exactly one of `--value`, `--json-value`, `--add`, `--add-json`, `--remove`, or `--remove-json`. Scalar modes author strings; JSON modes accept one complete JSON value and preserve structured values. Add and remove operate on the authored list only, are structurally idempotent, and retain order. `./awf reset sidecar <kind> <name> <field>` removes the authored leaf and cleans empty parents and a final empty sidecar. Fields include `data.<key>`, valid `dataDefaults.<key>` controls, declared `sections.<section>.drop`, and domain `paths`; unsupported or intermediate fields refuse.

Owner-managed installations support the current awf release and one previous release. The supported
floor advances only after every managed repository pin has been upgraded to remain at or above it;
older installed releases are unsupported. Bootstrap is disabled in this repository, so `./awf` runs
this checkout from source. After updating the source, run `./awf upgrade` to perform the supported
config migration. Live project authority starts at schema 50. A below-floor or retired layout must first use AWF 0.44 to reach schema 50 before adopting the supported `.awf/` control pair; this release does not mutate retired layouts.
