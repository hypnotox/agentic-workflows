// Package migrate ports a project's awf config across schema generations. It is
// the sole reader of the legacy single-file .claude/awf.yaml (ADR-0010
// inv: legacy-read-isolation, the named exemption to ADR-0009 inv: config-root)
// and is imported by nothing on the render/sync/check load path.
package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// A Migration ports a project from the generation below To up to To.
type Migration struct {
	To    int
	Name  string
	Apply func(root string) error
}

// registry is ordered ascending by To; current schema = last To.
var registry = []Migration{
	{To: 1, Name: "tree-layout", Apply: applyTreeLayout},
	{To: 2, Name: "drop-replacewith", Apply: applyDropReplaceWith},
}

// Current is the current schema generation (the highest registered To).
func Current() int { return registry[len(registry)-1].To }

// Generation reports the project's schema generation: 0 if the legacy single-file
// layout is present (.claude/awf.yaml exists and .claude/awf/config.yaml does not),
// else the lock's SchemaVersion. A tree project with no lock yet (a fresh awf init
// or a just-upgraded project before its terminal sync) reports Current() — it is
// on the current layout and must not gate; the next sync stamps the lock.
func Generation(root string) int {
	legacy := filepath.Join(root, ".claude", "awf.yaml")
	tree := filepath.Join(root, ".claude", "awf", "config.yaml")
	_, legacyErr := os.Stat(legacy)
	_, treeErr := os.Stat(tree)
	if legacyErr == nil && os.IsNotExist(treeErr) {
		return 0
	}
	l, err := manifest.Load(filepath.Join(root, ".claude", "awf", "awf.lock"))
	if err != nil {
		return Current()
	}
	return l.SchemaVersion
}

// stampLockSchema sets an existing tree lock's SchemaVersion to Current(). A
// missing lock (e.g. just after the legacy tree-layout port, before the first
// sync) is a no-op — Generation's no-lock branch already reports Current().
func stampLockSchema(root string) error {
	lockPath := filepath.Join(root, ".claude", "awf", "awf.lock")
	if !fileExists(lockPath) {
		return nil // no lock yet; the terminal sync stamps it
	}
	l, err := manifest.Load(lockPath)
	if err != nil { // coverage-ignore: reached only via Upgrade, which calls this after a migration applied, i.e. Generation < Current; Generation returns Current() on an unloadable lock, so when this runs the lock just loaded cleanly
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

// gateStateFor is the pure classifier (extracted for testability): "ok" when gen is
// at/above current; "gate" when at least one To lands in the open interval
// (gen, current]; "autobump" otherwise.
func gateStateFor(gen, current int, tos []int) string {
	if gen >= current {
		return "ok"
	}
	for _, to := range tos {
		if to > gen && to <= current {
			return "gate"
		}
	}
	return "autobump"
}

// GateState classifies a project: "ok" | "gate" | "autobump".
func GateState(root string) string {
	return gateStateFor(Generation(root), Current(), registryTos())
}

// Upgrade applies every registered migration with To > Generation(root), in
// ascending To order, and returns the applied migration names. Idempotent: at the
// current generation it applies nothing and returns an empty slice, nil error.
// After applying any migration it restamps an existing tree lock to Current() so
// Generation reflects the new state and the terminal sync's schema gate passes
// (a tree→tree upgrade keeps its lock, unlike the legacy 0→1 port which drops it).
func Upgrade(root string) ([]string, error) {
	from := Generation(root)
	var applied []string
	for _, m := range registry { // registry is already ascending by To
		if m.To <= from {
			continue
		}
		if err := m.Apply(root); err != nil {
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
