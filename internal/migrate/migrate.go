// Package migrate ports a project's awf config across schema generations. It is
// the sole reader of the legacy single-file .claude/awf.yaml (ADR-0010
// inv: legacy-read-isolation, the named exemption to ADR-0009 inv: config-root)
// and is imported by nothing on the render/sync/check load path. It reads the
// compile-time catalog (internal/catalog) for the ADR-0081 close-enabled-set
// migration - a leaf import that keeps this package off the render path.
//
// Migrations collect ordered typed Change values for every performed operation.
// The command owner presents only terminal results; a no-op run collects no
// changes.
package migrate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// A Migration ports a project from the generation below To up to To.
type Migration struct {
	To              int
	Name            string
	Apply           func(ctx context.Context, root string, out *Changes) error
	OwnsSchemaStamp bool
}

// registry is ordered ascending by To; current schema = last To.
var registry = []Migration{
	{To: 1, Name: "tree-layout", Apply: treeOnly(applyTreeLayout)},
	{To: 2, Name: "drop-replacewith", Apply: treeOnly(applyDropReplaceWith)},
	{To: 3, Name: "awf-dir-relocation", Apply: treeOnly(applyAwfRelocation)},
	{To: 4, Name: "drop-hooks", Apply: treeOnly(applyDropHooks)},
	{To: 5, Name: "enable-bootstrap", Apply: treeOnly(applyEnableBootstrap)},
	{To: 6, Name: "singleton-standard-docs", Apply: treeOnly(applySingletonStandardDocs)},
	{To: 7, Name: "anchored-globs", Apply: treeOnly(applyAnchoredGlobs)},
	{To: 8, Name: "close-enabled-set", Apply: treeOnly(applyCloseEnabledSet)},
	{To: 9, Name: "pitfalls-data", Apply: treeOnly(applyPitfallsData)},
	{To: 10, Name: "retirement-tokens", Apply: treeOnly(applyRetirementTokens)},
	{To: 11, Name: "drop-audit-base", Apply: treeOnly(applyDropAuditBase)},
	{To: 12, Name: "supersession-keys", Apply: treeOnly(applySupersessionKeys)},
	{To: 13, Name: "exploring-skill-closure", Apply: treeOnly(applyCloseEnabledSet)},
	{To: 14, Name: "current-state-topic-substrate", Apply: treeOnly(applyCurrentStateTopicSubstrate)},
	{To: 16, Name: "topic-claim-budget", Apply: treeOnly(applyTopicClaimBudget)},
	{To: 17, Name: "workflow-telemetry", Apply: treeOnly(applyWorkflowTelemetry)},
	{To: 18, Name: "enable-runner", Apply: treeOnly(applyEnableRunner)},
	{To: 19, Name: "rename-retired-commands", Apply: treeOnly(applyRenameRetiredCommands)},
	{To: 20, Name: "drop-workflow-telemetry", Apply: treeOnly(applyDropWorkflowTelemetry)},
	{To: 21, Name: "remove-workflow-residents", Apply: applyRemoveWorkflowResidents},
	{To: 22, Name: "unified-effort-residents", Apply: applyUnifiedEffortResidents, OwnsSchemaStamp: true},
	{To: 23, Name: "implementer-agent-closure", Apply: treeOnly(applyCloseEnabledSet)},
	// Generation 24 pairs exploring->explorer and brainstorming->grounding-checker
	// (ADR-0179); closing an enabled set over a new structural edge is what
	// applyCloseEnabledSet already does, so an adopter who enables either skill
	// gains its paired agent on upgrade instead of failing at project open.
	{To: 24, Name: "explorer-grounding-closure", Apply: treeOnly(applyCloseEnabledSet)},
	{To: 25, Name: "drop-severity-settings", Apply: treeOnly(applyDropSeveritySettings)},
	{To: 26, Name: "orienting-skill-backfill", Apply: treeOnly(applyOrientingSkillBackfill)},
	{To: 27, Name: "adr-number-provenance", Apply: treeOnly(applyADRNumberProvenance)},
	// ADR-0194 retires the topic claim-count advisory, so the key it configured
	// is removed rather than tolerated: config.yaml is strict-parsed and a
	// survivor would hard-fail the new binary.
	{To: 28, Name: "drop-max-claims-per-topic", Apply: treeOnly(applyDropMaxClaimsPerTopic)},
	{To: integrationBranchGeneration, Name: "integration-branch-explicit", Apply: treeOnly(applyIntegrationBranch)},
	{To: intrinsicADRFormatGeneration, Name: "intrinsic-adr-format", Apply: applyIntrinsicADRFormat, OwnsSchemaStamp: true},
	{To: retargetCheckCommandsGeneration, Name: "retarget-check-commands", Apply: treeOnly(applyRetargetCheckCommands)},
	{To: 33, Name: "decision-item-slugs", Apply: applyDecisionItemSlugs},
	{To: 34, Name: "commit-policy", Apply: treeOnly(applyCommitPolicy)},
	{To: layerCatalogListsGeneration, Name: "layer-catalog-lists", Apply: treeOnly(applyLayerCatalogLists)},
	{To: structuralHeadingsGeneration, Name: "structural-headings", Apply: treeOnly(applyStructuralHeadings)},
	{To: 37, Name: "grounding-skill-backfill", Apply: treeOnly(applyGroundingSkillBackfill)},
	{To: 38, Name: "drop-gate-audit-settings", Apply: treeOnly(applyDropGateAuditSettings)},
	{To: 39, Name: "drop-selection", Apply: treeOnly(applyDropSelection)},
	{To: 40, Name: "retire-plan-resync-selection", Apply: treeOnly(applyRetirePlanResync)},
	{To: globalTopicPathOwnershipGeneration, Name: "global-topic-path-ownership", Apply: treeOnly(applyGlobalTopicPathOwnership)},
	{To: effortArchiveGeneration, Name: "effort-archive-root", Apply: treeOnly(applyEffortArchiveRoot)},
	{To: pitfallCorpusGeneration, Name: "pitfall-corpus", Apply: treeOnly(applyPitfallCorpus)},
	{To: templateSourceGeneration, Name: "template-source-root", Apply: treeOnly(applyTemplateSourceRoot)},
	{To: localDocsGeneration, Name: "local-docs", Apply: treeOnly(applyLocalDocs)},
}

// treeOnly adapts a migration that only rewrites the config tree to the
// registry's context-taking shape. Only a migration that reaches Git needs the
// context, so wrapping the rest keeps that distinction visible in the registry
// itself rather than hiding it behind an unused parameter in every migration.
func treeOnly(apply func(root string, out *Changes) error) func(context.Context, string, *Changes) error {
	return func(_ context.Context, root string, out *Changes) error { return apply(root, out) }
}

// applyCurrentStateTopicSubstrate ports schema 13 -> 14: the invariants->current-state
// cutover retires the top-level `invariants` config block. The current-state
// topic corpus is authored, not migration-generated, so this migration performs
// no topic synthesis; it only removes the schema field the current strict
// config.Config no longer accepts, which would otherwise hard-fail the new binary
// on the migrated tree. Mirroring applyDropAuditBase, the removal collects a
// change fact so terminal-owner presentation can name an adopter-set value. The edit
// routes through config.RemoveKey so config.yaml serialization stays owned by
// internal/config (ADR-0026); the key is top-level, so RemoveKey applies.
func applyCurrentStateTopicSubstrate(root string, w *Changes) error {
	return editConfig(root, w, func(src []byte, planned *Changes) ([]byte, error) {
		out, err := config.RemoveKey(src, "invariants")
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(out, src) {
			planned.Add("current-state-topic-substrate: removed the retired top-level invariants block")
		}
		return out, nil
	})
}

const effortArchiveGeneration = 42
const templateSourceGeneration = 44
const localDocsGeneration = 45

// applyEffortArchiveRoot creates a schema boundary for the archive marker.
// Ordinary upgrade sync owns publication of the governed output and lock.
func applyEffortArchiveRoot(_ string, _ *Changes) error { return nil }

// applyTemplateSourceRoot deliberately performs no byte rewrite: the optional
// repository fact is absent-compatible and only a schema boundary is needed.
func applyTemplateSourceRoot(_ string, _ *Changes) error { return nil }

// applyLocalDocs is no-byte: an absent optional list declares no local docs.
func applyLocalDocs(_ string, _ *Changes) error { return nil }

// Current is the current schema generation (the highest registered To).
func Current() int { return registry[len(registry)-1].To }

// retiredKeyRemovals is where each retired config key lives, so the
// port-forward can strip it. An empty parent means a top-level key.
var retiredKeyRemovals = []struct{ parent, key string }{
	{"", "workflowTelemetry"},
	{"currentState", "topicCoverage"},
	{"currentState", "topicFanout"},
	{"currentState", "maxClaimsPerTopic"},
	{"", "hooks"}, {"", "runner"},
	{"proseGate", "enabled"}, {"memoryCite", "enabled"},
	{"audit", "allowedTypes"}, {"audit", "subjectMaxLength"}, {"audit", "diffThreshold"},
	{"audit", "dependencyManifests"}, {"audit", "domainDocStaleness"}, {"audit", "domainCodeStaleness"},
	{"audit", "undocumentedDomain"}, {"audit", "plainPunctuation"}, {"audit", "uncommittedChanges"},
	{"currentState", "maxTopicsPerPath"},
	{"", "skills"}, {"", "agents"}, {"", "docs"}, {"", "targets"}, {"", "docsDir"},
}

// ConfigForCurrentSchema applies the config-byte portions of registered
// migrations after from through the current generation. Snapshot consumers use
// it to compare a historical committed config with a current staged config
// without relaxing the current strict parser. Migrations that do not mutate
// config.yaml have no byte-level action here.
func ConfigForCurrentSchema(src []byte, from int) ([]byte, error) {
	if from > Current() {
		return nil, fmt.Errorf("config schema generation %d is ahead of current %d", from, Current())
	}
	out := src
	// A catalog selection that no longer exists must be stripped before the
	// current strict catalog consumer sees historical bytes. As with retired
	// keys below, this is unconditional because a later stamp does not prove a
	// concurrently introduced migration ran.
	out, _, err := removePlanResyncSelection(out)
	if err != nil {
		return nil, fmt.Errorf("port-forward removal of retired skill %q: %w", retiredPlanResyncSkill, err)
	}
	// A key whose struct field no longer exists is stripped unconditionally,
	// not when its own generation happens to fall inside the ported range. This
	// function exists so historical bytes PARSE under the current strict
	// decoder, and a stamped generation is not proof the removal ever ran: two
	// branches allocating generations concurrently can leave a tree stamped past
	// a removal it never applied, which is exactly how a config carrying a
	// retired key reaches a decoder that has no field for it. Removing an absent
	// key is a no-op, so doing this for every retired key is free.
	for _, retired := range retiredKeyRemovals {
		var err error
		if retired.parent == "" {
			out, err = config.RemoveKey(out, retired.key)
		} else {
			out, err = config.RemoveMappingKey(out, retired.parent, retired.key)
		}
		if err != nil { // coverage-ignore: the retired-skill editor above already parsed the same mapping bytes
			return nil, fmt.Errorf("port-forward removal of retired key %q: %w", retired.key, err)
		}
	}
	for _, migration := range registry {
		if migration.To <= from {
			continue
		}
		// Unlike the pure removals above, this case materializes a key the
		// committed bytes never had, and it must. integrationBranch is
		// required with no in-code default (ADR-0202 Decision 6), so a
		// historical config without it does not merely lack a value: it fails
		// Validate, and the whole before-side load a transition check depends
		// on aborts. Seeding exactly what the migration writes is the faithful
		// port-forward, which is why the question asked here is the migration's
		// own: a key carrying a value keeps it, and a key that is absent, null,
		// or empty is seeded, because the migration would have written all three.
		if migration.To == integrationBranchGeneration {
			valued, err := config.HasValue(out, "integrationBranch")
			if err != nil { // coverage-ignore: the retired-key removal pass above already parsed these same bytes as a mapping, so HasValue cannot fail here
				return nil, fmt.Errorf("migration %q (to %d): %w", migration.Name, migration.To, err)
			}
			if !valued {
				out, err = config.SetString(out, "integrationBranch", "main")
				if err != nil { // coverage-ignore: HasValue parsed these same bytes as a mapping one statement above, so SetString cannot fail here
					return nil, fmt.Errorf("migration %q (to %d): %w", migration.Name, migration.To, err)
				}
			}
		}
		if migration.To == retargetCheckCommandsGeneration {
			var err error
			out, _, err = retargetCheckCommandBytes(out)
			if err != nil { // coverage-ignore: prior forward-port editors and the helper's initial parse already validated this YAML
				return nil, fmt.Errorf("migration %q (to %d): %w", migration.Name, migration.To, err)
			}
		}
	}
	return out, nil
}

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
func stampLockSchemaWithSave(root string, save func(*manifest.Lock, string) error) (bool, error) {
	lockPath := config.LockPath(root)
	if !fileExists(lockPath) {
		return false, nil // no lock yet; the terminal sync stamps it
	}
	l, err := manifest.Load(lockPath)
	if err != nil { // coverage-ignore: reached only via Upgrade, whose upfront Generation now hard-errors on a corrupt lock (ADR-0076), so when this runs the lock loads cleanly
		return false, err
	}
	l.SchemaVersion = Current()
	if err := save(l, lockPath); err != nil {
		return false, err
	}
	return true, nil
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
// ascending To order, and returns applied names and ordered changes, including
// facts collected before a migration failure.
func Upgrade(ctx context.Context, root string) ([]string, []Change, error) {
	return upgradeWithStampSave(ctx, root, func(lock *manifest.Lock, path string) error { return lock.Save(path) })
}

func upgradeWithStampSave(ctx context.Context, root string, save func(*manifest.Lock, string) error) ([]string, []Change, error) {
	from, err := Generation(root)
	if err != nil {
		return nil, nil, err
	}
	changes := &Changes{}
	var applied []string
	var highestApplied Migration
	for _, m := range registry { // registry is already ascending by To
		if m.To <= from {
			continue
		}
		if err := m.Apply(ctx, root, changes); err != nil {
			return applied, changes.Items(), fmt.Errorf("migration %q (to %d): %w", m.Name, m.To, err)
		}
		applied = append(applied, m.Name)
		highestApplied = m
	}
	if len(applied) > 0 && !highestApplied.OwnsSchemaStamp {
		stamped, err := stampLockSchemaWithSave(root, save)
		if err != nil {
			return applied, changes.Items(), err
		}
		if stamped {
			changes.Add("schema-stamp: updated awf.lock schema version")
		}
	}
	return applied, changes.Items(), nil
}
