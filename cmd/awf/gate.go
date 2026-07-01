package main

import (
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"golang.org/x/mod/semver"
)

// normalizeSemver returns s in the single-leading-v form x/mod/semver requires.
// awfVersion() already returns the v-form for `go install` builds, so a naive
// prefix would yield "vv0.4.0" and fail semver.IsValid; trimming any existing v
// first makes the normalization idempotent (ADR-0039 Decision 3).
func normalizeSemver(s string) (string, bool) {
	v := "v" + strings.TrimPrefix(s, "v")
	if !semver.IsValid(v) {
		return "", false
	}
	return v, true
}

// gate refuses to operate against a config the running binary cannot correctly
// interpret. It runs before project.Open. On the schema axis: "gate" (config
// behind binary) → "run awf upgrade"; "ahead" (config ahead of binary) → "update
// your pinned awf" (ADR-0039); "autobump" proceeds and the subsequent sync stamps
// the current schema. On the release-version axis: after the schema check it loads
// .awf/awf.lock and compares lock.AWFVersion vs awfVersion() — a lock semver-newer
// than the binary (binary behind) errors; a binary at-or-ahead is the permitted
// pre-upgrade state. The version sub-check is skipped (never errors) on an absent,
// unparseable, empty, or non-normalizable version, mirroring Generation's no-lock
// tolerance.
// invariant: version-compat-gate
func gate(root string) error {
	switch migrate.GateState(root) {
	case "gate":
		return fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade",
			migrate.Generation(root), migrate.Current())
	case "ahead":
		return fmt.Errorf("awf %s is behind this project's config (schema generation %d > %d); update your pinned awf",
			awfVersion(), migrate.Generation(root), migrate.Current())
	}
	lockV, binV, ok := lockVsBinary(root)
	if !ok {
		return nil // version sub-check not computable; schema check already applied
	}
	if semver.Compare(lockV, binV) > 0 {
		return fmt.Errorf("awf %s is behind this project (rendered by %s); update your pinned awf",
			awfVersion(), strings.TrimPrefix(lockV, "v"))
	}
	return nil
}

// lockVsBinary returns the normalized lock awfVersion and binary version for the
// release-version sub-check, with ok=false whenever the comparison cannot be
// computed (no/unloadable lock, empty AWFVersion, or a version that fails semver
// normalization) so the caller skips rather than errors (ADR-0039 Decision 5).
func lockVsBinary(root string) (lockV, binV string, ok bool) {
	l, err := manifest.Load(config.LockPath(root))
	if err != nil || l.AWFVersion == "" {
		return "", "", false
	}
	lockV, lok := normalizeSemver(l.AWFVersion)
	binV, bok := normalizeSemver(awfVersion())
	if !lok || !bok {
		return "", "", false
	}
	return lockV, binV, true
}
