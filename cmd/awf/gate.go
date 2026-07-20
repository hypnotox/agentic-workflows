package main

import (
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"golang.org/x/mod/semver"
)

// normalizeSemver keeps the command-local call sites readable while delegating
// lock syntax to the manifest package's shared authority normalizer.
func normalizeSemver(s string) (string, bool) { return manifest.NormalizeSemver(s) }

// gate refuses to operate against a config the running binary cannot correctly
// interpret. It runs before project.Open. On the schema axis: "gate" (config
// behind binary) → "run awf upgrade"; "ahead" (config ahead of binary) → "update
// your pinned awf" (ADR-0039); "autobump" proceeds and the subsequent sync stamps
// the current schema. On the release-version axis: after the schema check it loads
// .awf/awf.lock and compares lock.AWFVersion vs awfVersion() - a lock semver-newer
// than the binary (binary behind) errors; a binary at-or-ahead is the permitted
// pre-upgrade state. The version sub-check is skipped (never errors) on an absent,
// unparseable, empty, or non-normalizable version, mirroring Generation's no-lock
// tolerance.
// touches-state: tooling/cli:version-compat-gate - binary-vs-config version gate; proof in gate_test.go
func gate(root string) error {
	state, gen, err := migrate.GateState(root)
	if err != nil {
		return err
	}
	if err := gateGeneration(state, gen); err != nil {
		return err
	}
	lockV, binV, ok, err := lockVsBinary(root)
	if err != nil {
		// Reachable without any race: a corrupt lock beside no config layout -
		// Generation stats no config file and never loads the lock, so this
		// version sub-check is the first reader to hit it (ADR-0076).
		return err
	}
	return gateLockVersion(lockV, binV, ok)
}

// gateStaged applies the normal schema and release-version classifications to
// the index lock. It never consults a divergent working lock.
func gateStaged(root string) error {
	lock, err := stagedLock(root)
	if err != nil {
		return err
	}
	gen := lock.SchemaVersion
	if err := gateGeneration(migrate.GateStateForGeneration(gen), gen); err != nil {
		return err
	}
	lockV, binV, ok := lockVsBinaryLock(lock)
	return gateLockVersion(lockV, binV, ok)
}

func gateGeneration(state string, gen int) error {
	switch state {
	case "gate":
		return fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade", gen, migrate.Current())
	case "ahead":
		return schemaAheadError(gen)
	}
	return nil
}

func gateLockVersion(lockV, binV string, ok bool) error {
	if !ok {
		return nil
	}
	if semver.Compare(lockV, binV) > 0 {
		return fmt.Errorf("awf %s is behind this project (rendered by %s); update your pinned awf",
			awfVersion(), strings.TrimPrefix(lockV, "v"))
	}
	return nil
}

func stagedLock(root string) (*manifest.Lock, error) {
	tree, err := snapshot.IndexTree(root)
	if err != nil {
		return nil, err
	}
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, fmt.Errorf("no staged %s/awf.lock", config.DirName)
	}
	lock, err := manifest.Parse(file.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse staged lock: %w", err)
	}
	return lock, nil
}

// schemaAheadError is the single "config schema ahead of this binary" message,
// shared by gate() and runUpgrade so the guidance cannot drift between the two
// entry points that classify the ahead state.
func schemaAheadError(gen int) error {
	return fmt.Errorf("awf %s is behind this project's config (schema generation %d > %d); update your pinned awf",
		awfVersion(), gen, migrate.Current())
}

// lockVsBinary returns the normalized lock awfVersion and binary version for the
// release-version sub-check. The surviving skip set (ok=false, nil error): an
// absent lock; an absent or empty awfVersion field; an awfVersion failing semver
// normalization - all still skip rather than error. A present-but-unparseable
// lock now errors upstream via the LoadOptional choke point (ADR-0076 partially
// supersedes ADR-0039 Decision 5).
func lockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	l, found, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil {
		return "", "", false, err
	}
	if !found {
		return "", "", false, nil
	}
	lockV, binV, ok = lockVsBinaryLock(l)
	return lockV, binV, ok, nil
}

func lockVsBinaryLock(l *manifest.Lock) (lockV, binV string, ok bool) {
	if l == nil || l.AWFVersion == "" {
		return "", "", false
	}
	lockV, lok := normalizeSemver(l.AWFVersion)
	binV, bok := normalizeSemver(awfVersion())
	if !lok || !bok {
		return "", "", false
	}
	return lockV, binV, true
}
