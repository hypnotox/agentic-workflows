package migrate

// Legacy compatibility helpers remain package-test-only until Phase 2 removes
// their implementations. Live callers cannot dispatch below schema 46.

import (
	"context"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// retiredKeyRemovals is where each retired config key lives, so the
// port-forward can strip it. An empty parent means a top-level key.
type retiredConfigKey struct{ parent, key string }

var retiredKeyRemovals = []retiredConfigKey{
	{"", "invariants"},
	{"audit", "baseBranch"},
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
		if migration.To == profileGeneration {
			valued, err := config.HasValue(out, "profile")
			if err != nil { // coverage-ignore: the current-schema forward-port has already parsed the historical config mapping
				return nil, fmt.Errorf("migration %q (to %d): %w", migration.Name, migration.To, err)
			}
			if !valued {
				out, err = config.SetString(out, "profile", "full")
				if err != nil { // coverage-ignore: HasValue parsed this unchanged mapping immediately above
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

func upgradeWithStampSave(ctx context.Context, root string, save func(*manifest.Lock, string) error) ([]string, []Change, error) {
	from, err := Generation(root)
	if err != nil {
		return nil, nil, err
	}
	return upgradeUnchecked(ctx, root, from, save)
}
