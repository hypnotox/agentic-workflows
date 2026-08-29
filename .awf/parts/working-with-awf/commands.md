{{=awf:sectionDefault}}

Author one declared convention part by semantic identity with `./awf edit <kind> <name> <part> --content <text>` or `./awf edit <kind> <name> <part> --stdin`. Exactly one input mode is required, and an explicitly empty `--content` value remains an authored override. The closed kinds are `doc`, `skill`, `agent`, and `domain`; names and parts come from the selected catalog and project configuration rather than filesystem paths.

Use `./awf reset <kind> <name> <part>` to remove a convention-part override and restore its inherited default. A configured local document is addressed as `doc <name> body`; edit replaces only its body and reset restores the empty body while preserving its declaration and awf-owned shell. Both commands validate the complete candidate project before changing the source, then render and update the lock. If a later source or publication step fails, follow the reported residue-first recovery actions and rerun `./awf render`; no rollback is implied.

Owner-managed installations support the current awf release and one previous release. The supported
floor advances only after every managed repository pin has been upgraded to remain at or above it;
older installed releases are unsupported. Use `bash .awf/upgrade.sh` from the repository root to
upgrade the pinned binary and project together. Live project authority starts at schema 46. A below-floor or retired layout must be recovered with a release that supports it before adopting the supported `.awf/` control pair; this release does not mutate retired layouts.
