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
// project.Version and lock awfVersion values are the no-v form, but historical
// locks may carry either; trimming any existing v first makes the
// normalization idempotent (ADR-0039 Decision 3, single-sourced by ADR-0049).
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
	state, gen, err := migrate.GateState(root)
	if err != nil {
		return err
	}
	switch state {
	case "gate":
		return fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade",
			gen, migrate.Current())
	case "ahead":
		return fmt.Errorf("awf %s is behind this project's config (schema generation %d > %d); update your pinned awf",
			awfVersion(), gen, migrate.Current())
	}
	lockV, binV, ok, err := lockVsBinary(root)
	if err != nil {
		// Reachable without any race: a corrupt lock beside no config layout —
		// Generation stats no config file and never loads the lock, so this
		// version sub-check is the first reader to hit it (ADR-0076).
		return err
	}
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
// release-version sub-check. The surviving skip set (ok=false, nil error): an
// absent lock; an absent or empty awfVersion field; an awfVersion failing semver
// normalization — all still skip rather than error. A present-but-unparseable
// lock now errors upstream via the LoadOptional choke point (ADR-0076 partially
// supersedes ADR-0039 Decision 5).
func lockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	l, found, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil {
		return "", "", false, err
	}
	if !found || l.AWFVersion == "" {
		return "", "", false, nil
	}
	lockV, lok := normalizeSemver(l.AWFVersion)
	binV, bok := normalizeSemver(awfVersion())
	if !lok || !bok {
		return "", "", false, nil
	}
	return lockV, binV, true, nil
}
