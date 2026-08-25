{{=awf:sectionDefault}}

Owner-managed installations support the current awf release and one previous release. The supported
floor advances only after every managed repository pin has been upgraded to remain at or above it;
older installed releases are unsupported. Use `bash .awf/upgrade.sh` from the repository root to
upgrade the pinned binary and project together. Live project authority starts at schema 46. A below-floor or retired layout must be recovered with a release that supports it before adopting the supported `.awf/` control pair; this release does not mutate retired layouts.
