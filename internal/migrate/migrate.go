// Package migrate ports a project's awf config across schema generations. It is
// the sole reader of the legacy single-file .claude/awf.yaml (ADR-0010
// inv: legacy-read-isolation, the named exemption to ADR-0009 inv: config-root)
// and is imported by nothing on the render/sync/check load path. It reads the
// compile-time catalog (internal/catalog) for the ADR-0081 close-enabled-set
// migration — a leaf import that keeps this package off the render path.
package migrate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// A Migration ports a project from the generation below To up to To.
type Migration struct {
	To    int
	Name  string
	Apply func(root string, out io.Writer) error
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
}

// Current is the current schema generation (the highest registered To).
func Current() int { return registry[len(registry)-1].To }

// Generation reports the project's schema generation. Detection is by layout:
// a .awf/ tree reports its lock's SchemaVersion (or Current() when no lock yet —
// fresh init / just-upgraded); a pre-relocation .claude/awf/ tree reports its
// lock's schema, or 1 when no lock — such a tree is the tree-layout port's
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
		// invariant: corrupt-lock-refuses
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

// ProjectPresent reports whether any awf config layout (current tree,
// pre-relocation tree, or legacy single file) exists under root — the
// distinction Generation cannot express, since "nothing present" reports
// Current() (ADR-0076 Decision 4).
func ProjectPresent(root string) bool {
	for _, p := range []string{
		config.ConfigPath(root),
		filepath.Join(root, ".claude", "awf", "config.yaml"),
		filepath.Join(root, ".claude", "awf.yaml"),
	} {
		if fileExists(p) {
			return true
		}
	}
	return false
}

// stampLockSchema sets an existing tree lock's SchemaVersion to Current(). A
// missing lock (e.g. just after the legacy tree-layout port, before the first
// sync) is a no-op — Generation's no-lock branch already reports Current().
func stampLockSchema(root string) error {
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
// gen is strictly above current (the binary is behind the project — ADR-0039);
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

// GateState classifies a project ("ok" | "gate" | "autobump" | "ahead") and
// returns the generation it classified, so callers need only one Generation
// call for both the state and their messages.
func GateState(root string) (string, int, error) {
	gen, err := Generation(root)
	if err != nil {
		return "", 0, err
	}
	return gateStateFor(gen, Current(), registryTos()), gen, nil
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
	for _, m := range registry { // registry is already ascending by To
		if m.To <= from {
			continue
		}
		if err := m.Apply(root, out); err != nil {
			return applied, fmt.Errorf("migration %q (to %d): %w", m.Name, m.To, err)
		}
		applied = append(applied, m.Name)
	}
	if len(applied) > 0 {
		if err := stampLockSchema(root); err != nil { // coverage-ignore: stampLockSchema only fails on a lock Save fault, unreachable in a writable tree
			return applied, err
		}
	}
	return applied, nil
}
