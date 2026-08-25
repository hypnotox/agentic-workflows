// Package migrate owns supported live schema upgrades. Historical schema
// decoding belongs to internal/audit and retired layouts are recognized only
// for refusal, never parsed or relocated here.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// Migration is one ordered upgrade step for a supported live generation.
type Migration struct {
	To    int
	Name  string
	Apply func(context.Context, string, *Changes) error
}

// LiveSchemaFloor is the oldest source generation this binary can operate on.
const LiveSchemaFloor = 46

// registry intentionally begins at the floor. The no-op floor entry is the one
// explicit seam where a later supported schema migration is appended.
var registry = []Migration{{To: LiveSchemaFloor, Name: "supported-schema-46"}}

func Current() int                { return registry[len(registry)-1].To }
func LiveSchemaRange() (int, int) { return LiveSchemaFloor, Current() }

// RetiredLayoutError identifies a removed layout without decoding its config.
type RetiredLayoutError struct{ Layout string }

func (e *RetiredLayoutError) Error() string {
	return fmt.Sprintf("retired project layout %s is unsupported at live floor %d", e.Layout, LiveSchemaFloor)
}
func (e *RetiredLayoutError) Is(target error) bool {
	return target == manifest.ErrUnsupportedLiveSource
}

func currentConfigPresent(root string) bool {
	_, err := os.Stat(config.ConfigPath(root))
	return err == nil
}
func retiredLayout(root string) string {
	for _, layout := range []struct{ path, name string }{
		{filepath.Join(root, ".claude", "awf.yaml"), ".claude/awf.yaml"},
		{filepath.Join(root, ".claude", "awf"), ".claude/awf/"},
	} {
		if _, err := os.Stat(layout.path); err == nil {
			return layout.name
		}
	}
	return ""
}

// Generation reads only a current-layout lock schema. Retired layouts receive a
// typed refusal before their authority representation can be decoded.
func Generation(root string) (int, error) {
	if layout := retiredLayout(root); layout != "" && !currentConfigPresent(root) {
		return 0, &RetiredLayoutError{Layout: layout}
	}
	if !currentConfigPresent(root) {
		return Current(), nil
	}
	generation, found, err := manifest.LoadSchemaOptional(config.LockPath(root))
	if err != nil {
		return 0, err
	}
	if !found {
		return Current(), nil
	}
	return generation, nil
}

// ProjectPresent recognizes current control files and retired layouts without
// interpreting their content.
func ProjectPresent(root string) bool {
	for _, path := range []string{config.ConfigPath(root), config.LockPath(root), filepath.Join(root, ".claude", "awf.yaml"), filepath.Join(root, ".claude", "awf")} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func ProjectPresentFromFiles(has func(string) bool) bool {
	return has(config.DirName+"/config.yaml") || has(config.DirName+"/awf.lock") || has(".claude/awf.yaml") || has(".claude/awf/config.yaml") || has(".claude/awf/awf.lock")
}

// AuthorityLockPath is always the current control lock. Retired layouts have
// no live authority lock.
func AuthorityLockPath(root string) string { return config.LockPath(root) }
func registryTos() []int {
	tos := make([]int, len(registry))
	for i, m := range registry {
		tos[i] = m.To
	}
	return tos
}
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
func GateStateForGeneration(gen int) string { return gateStateFor(gen, Current(), registryTos()) }

type UpgradeRequiredError struct{ Generation, Current int }

func (e *UpgradeRequiredError) Error() string {
	return fmt.Sprintf("schema %d requires migration to schema %d", e.Generation, e.Current)
}
func CheckLiveGeneration(gen int) error {
	if err := manifest.ValidateLive(&manifest.Lock{SchemaVersion: gen}, LiveSchemaFloor, Current()); err != nil {
		return err
	}
	if GateStateForGeneration(gen) == "gate" {
		return &UpgradeRequiredError{Generation: gen, Current: Current()}
	}
	return nil
}
func CheckLive(root string) (int, error) {
	gen, err := Generation(root)
	if err != nil {
		return 0, err
	}
	return gen, CheckLiveGeneration(gen)
}
func GateState(root string) (string, int, error) {
	gen, err := Generation(root)
	if err != nil {
		return "", 0, err
	}
	return GateStateForGeneration(gen), gen, nil
}

// Upgrade executes only registered supported migrations. It never decodes or
// writes a below-floor source.
func Upgrade(ctx context.Context, root string) ([]string, []Change, error) {
	from, err := Generation(root)
	if err != nil {
		return nil, nil, err
	}
	if err := manifest.ValidateLive(&manifest.Lock{SchemaVersion: from}, LiveSchemaFloor, Current()); err != nil {
		return nil, nil, err
	}
	changes := &Changes{}
	var applied []string
	for _, m := range registry {
		if m.To <= from {
			continue
		}
		if err := m.Apply(ctx, root, changes); err != nil {
			return applied, changes.Items(), fmt.Errorf("migration %q (to %d): %w", m.Name, m.To, err)
		}
		applied = append(applied, m.Name)
	}
	return applied, changes.Items(), nil
}

// IsRetiredLayout makes presentation boundaries distinguish layout refusal
// from a below-floor current lock without retaining a layout reader.
func IsRetiredLayout(err error) bool {
	var retired *RetiredLayoutError
	return errors.As(err, &retired)
}
