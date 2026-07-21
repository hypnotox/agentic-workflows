// Package migrate ports a project's awf config across schema generations. It is
// the sole reader of the legacy single-file .claude/awf.yaml (ADR-0010
// inv: legacy-read-isolation, the named exemption to ADR-0009 inv: config-root)
// and is imported by nothing on the render/sync/check load path. It reads the
// compile-time catalog (internal/catalog) for the ADR-0081 close-enabled-set
// migration - a leaf import that keeps this package off the render path.
//
// Output convention: a migration that mutates the tree prints one line per
// performed operation to its out writer, prefixed with its registry Name
// (`<name>: <op>`), so an upgrade's config changes are readable from the
// command output rather than git archaeology; a no-op run prints nothing.
package migrate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// A Migration ports a project from the generation below To up to To.
type Migration struct {
	To              int
	Name            string
	Apply           func(root string, out io.Writer) error
	OwnsSchemaStamp bool
}

// registry is ordered ascending by To; current schema = last To.
var registry = []Migration{
	{To: 1, Name: "tree-layout", Apply: applyTreeLayout},
	{To: 2, Name: "drop-replacewith", Apply: applyDropReplaceWith},
	{To: 3, Name: "awf-dir-relocation", Apply: applyAwfRelocation},
	{To: 4, Name: "drop-hooks", Apply: applyDropHooks},
	{To: 5, Name: "enable-bootstrap", Apply: applyEnableBootstrap},
	{To: 6, Name: "singleton-standard-docs", Apply: applySingletonStandardDocs},
	{To: 7, Name: "anchored-globs", Apply: applyAnchoredGlobs},
	{To: 8, Name: "close-enabled-set", Apply: applyCloseEnabledSet},
	{To: 9, Name: "pitfalls-data", Apply: applyPitfallsData},
	{To: 10, Name: "retirement-tokens", Apply: applyRetirementTokens},
	{To: 11, Name: "drop-audit-base", Apply: applyDropAuditBase},
	{To: 12, Name: "supersession-keys", Apply: applySupersessionKeys},
	{To: 13, Name: "exploring-skill-closure", Apply: applyCloseEnabledSet},
	{To: 14, Name: "current-state-topic-substrate", Apply: applyCurrentStateTopicSubstrate},
	{To: 15, Name: "adr-format-v2-cutoff", Apply: applyADRFormatV2Cutoff, OwnsSchemaStamp: true},
}

// applyCurrentStateTopicSubstrate ports schema 13 -> 14: the invariants->current-state
// cutover retires the top-level `invariants` config block. The current-state
// topic corpus is authored, not migration-generated, so this migration performs
// no topic synthesis; it only removes the schema field the current strict
// config.Config no longer accepts, which would otherwise hard-fail the new binary
// on the migrated tree. Mirroring applyDropAuditBase, the removal is announced so
// deleting a value an adopter set stays readable from command output. The edit
// routes through config.RemoveKey so config.yaml serialization stays owned by
// internal/config (ADR-0026); the key is top-level, so RemoveKey applies.
func applyCurrentStateTopicSubstrate(root string, w io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		out, err := config.RemoveKey(src, "invariants")
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(out, src) {
			fmt.Fprintln(w, "current-state-topic-substrate: removed the retired top-level invariants block")
		}
		return out, nil
	})
}

// Current is the current schema generation (the highest registered To).
func Current() int { return registry[len(registry)-1].To }

// Generation reports the project's schema generation. Detection is by layout:
// a .awf/ tree reports its lock's SchemaVersion (or Current() when no lock yet -
// fresh init / just-upgraded); a pre-relocation .claude/awf/ tree reports its
// lock's schema, or 1 when no lock - such a tree is the tree-layout port's
// output (the port deletes the legacy lock), so every later migration up to and
// including the To:3 relocation must still apply; the legacy single file
// reports 0; nothing present reports Current(). A present-but-unreadable lock
// in either lock-bearing layout is a hard error, never a sentinel generation
// (ADR-0076 Decision 2, narrowing ADR-0016 Decision 6's presence keying).
func Generation(root string) (int, error) {
	newTree := config.ConfigPath(root)
	oldTree := filepath.Join(root, ".claude", "awf", "config.yaml")
	legacy := filepath.Join(root, ".claude", "awf.yaml")
	if _, err := os.Stat(newTree); err == nil {
		l, found, err := manifest.LoadOptional(config.LockPath(root))
		if err != nil {
			return 0, err
		}
		if !found {
			return Current(), nil
		}
		return l.SchemaVersion, nil
	}
	if _, err := os.Stat(oldTree); err == nil {
		l, found, err := manifest.LoadOptional(filepath.Join(root, ".claude", "awf", "awf.lock"))
		if err != nil {
			return 0, err
		}
		if !found {
			return 1, nil
		}
		return l.SchemaVersion, nil
	}
	if _, err := os.Stat(legacy); err == nil {
		return 0, nil
	}
	return Current(), nil
}

// AuthorityLockPath returns the lock belonging to the active config layout.
// It keeps all knowledge of retired layout paths inside the migration package.
func AuthorityLockPath(root string) string {
	current := config.LockPath(root)
	if fileExists(config.ConfigPath(root)) || fileExists(current) {
		return current
	}
	if fileExists(filepath.Join(root, ".claude", "awf.yaml")) {
		return filepath.Join(root, ".claude", "awf.lock")
	}
	if fileExists(filepath.Join(root, ".claude", "awf", "config.yaml")) {
		return filepath.Join(root, ".claude", "awf", "awf.lock")
	}
	return current
}

// ProjectPresent reports whether any awf config layout (current tree,
// pre-relocation tree, or legacy single file) exists under root - the
// distinction Generation cannot express, since "nothing present" reports
// Current() (ADR-0076 Decision 4).
func ProjectPresent(root string) bool {
	return ProjectPresentFromFiles(func(path string) bool {
		return fileExists(filepath.Join(root, filepath.FromSlash(path)))
	})
}

// ProjectPresentFromFiles reports project presence through a repository-relative
// file lookup. Snapshot consumers use it so current and legacy layout knowledge
// remains owned by the migration package rather than being duplicated.
func ProjectPresentFromFiles(has func(string) bool) bool {
	for _, path := range []string{
		config.DirName + "/config.yaml",
		".claude/awf/config.yaml",
		".claude/awf.yaml",
	} {
		if has(path) {
			return true
		}
	}
	return false
}

// stampLockSchema sets an existing tree lock's SchemaVersion to Current(). A
// missing lock (e.g. just after the legacy tree-layout port, before the first
// sync) is a no-op - Generation's no-lock branch already reports Current().
func stampLockSchema(root string) error { // coverage-ignore: schema 15 owns the stamp and is the highest registered migration, so every currently reachable applied set ends in an owning migration
	lockPath := config.LockPath(root)
	if !fileExists(lockPath) {
		return nil // no lock yet; the terminal sync stamps it
	}
	l, err := manifest.Load(lockPath)
	if err != nil { // coverage-ignore: reached only via Upgrade, whose upfront Generation now hard-errors on a corrupt lock (ADR-0076), so when this runs the lock loads cleanly
		return err
	}
	l.SchemaVersion = Current()
	return l.Save(lockPath)
}

// registryTos returns the To values of every registered migration.
func registryTos() []int {
	tos := make([]int, len(registry))
	for i, m := range registry {
		tos[i] = m.To
	}
	return tos
}

// gateStateFor is the pure classifier (extracted for testability): "ahead" when
// gen is strictly above current (the binary is behind the project - ADR-0039);
// "ok" when gen == current; "gate" when at least one To lands in the open interval
// (gen, current]; "autobump" otherwise.
func gateStateFor(gen, current int, tos []int) string {
	if gen > current {
		return "ahead"
	}
	if gen == current {
		return "ok"
	}
	for _, to := range tos {
		if to > gen && to <= current {
			return "gate"
		}
	}
	return "autobump"
}

// GateStateForGeneration classifies an already-loaded schema generation with
// the same migration-registry semantics as GateState. Snapshot-aware callers
// use it after loading a lock from their own universe instead of rereading the
// working tree.
func GateStateForGeneration(gen int) string {
	return gateStateFor(gen, Current(), registryTos())
}

// GateState classifies a project ("ok" | "gate" | "autobump" | "ahead") and
// returns the generation it classified, so callers need only one Generation
// call for both the state and their messages.
func GateState(root string) (string, int, error) {
	gen, err := Generation(root)
	if err != nil {
		return "", 0, err
	}
	return GateStateForGeneration(gen), gen, nil
}

// Upgrade applies every registered migration with To > Generation(root), in
// ascending To order, and returns the applied migration names. Idempotent: at the
// current generation it applies nothing and returns an empty slice, nil error.
// After applying any migration it restamps an existing tree lock to Current() so
// Generation reflects the new state and the terminal sync's schema gate passes
// (a tree→tree upgrade keeps its lock, unlike the legacy 0→1 port which drops it).
func Upgrade(root string, out io.Writer) ([]string, error) {
	from, err := Generation(root)
	if err != nil {
		return nil, err
	}
	var applied []string
	var highestApplied Migration
	for _, m := range registry { // registry is already ascending by To
		if m.To <= from {
			continue
		}
		if err := m.Apply(root, out); err != nil {
			return applied, fmt.Errorf("migration %q (to %d): %w", m.Name, m.To, err)
		}
		applied = append(applied, m.Name)
		highestApplied = m
	}
	if len(applied) > 0 && !highestApplied.OwnsSchemaStamp {
		if err := stampLockSchema(root); err != nil { // coverage-ignore: stampLockSchema only fails on a lock Save fault, unreachable in a writable tree
			return applied, err
		}
	}
	return applied, nil
}
