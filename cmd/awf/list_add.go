package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func unknownKind(kind string) error {
	return &usageErr{fmt.Sprintf("unknown kind %q (want: skill, agent, doc, domain, target, bootstrap)", kind)}
}

// enableDisableSingleton enables or disables a nested <key>.enabled singleton toggle
// in the config - the bootstrap (ADR-0040), the git-hook payloads (ADR-0048), or the
// awf wrapper runner (ADR-0156). It is the bespoke path (singletons are not
// kindDescriptors - no catalog pool / sections / plural enable array, so they stay
// out of the single dispatch table that inv: kind-dispatch-single-table guards): a
// nested <key>.enabled scalar, written via config.SetMappingScalar.
func enableDisableSingleton(ctx context.Context, root, key string, add bool, stdout io.Writer) error {
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	enabled := key == "bootstrap" && p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled
	if add && enabled {
		return fmt.Errorf("%s already enabled", key)
	}
	if !add && !enabled {
		return fmt.Errorf("%s is not enabled", key)
	}
	cfgPath := config.ConfigPath(root)
	updated, err := config.SetMappingScalar(p.Cfg.Source(), key, "enabled", add)
	if err != nil { // coverage-ignore: config.Load already parsed this config, so SetMappingScalar's parse/mapping checks cannot fail here
		return err
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault root bypasses
		return err
	}
	return runSync(ctx, root, stdout)
}

// enableDisableTarget enables or disables an adapter in the config targets array. It
// is the bespoke path (targets is not a kindDescriptor - ADR-0037): it validates
// against the known-adapter set and writes the full resolved list, since the
// targets array carries a Load default that an absent on-disk key would drop.
func enableDisableTarget(ctx context.Context, root, name string, add bool, stdout io.Writer) error {
	if !slices.Contains(project.KnownTargets(), name) {
		return fmt.Errorf("%q is not a known target (known: %s)", name, strings.Join(project.KnownTargets(), ", "))
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	enabled := slices.Contains(p.Cfg.Targets, name)
	if add && enabled {
		return fmt.Errorf("target %q already enabled", name)
	}
	if !add && !enabled {
		return fmt.Errorf("target %q is not enabled", name)
	}
	var desired []string
	if add {
		desired = append(slices.Clone(p.Cfg.Targets), name)
	} else {
		desired = slices.DeleteFunc(slices.Clone(p.Cfg.Targets), func(s string) bool { return s == name })
	}
	if len(desired) == 0 {
		return fmt.Errorf("cannot disable the last target %q (a project must render to at least one)", name)
	}
	cfgPath := config.ConfigPath(root)
	updated, err := config.SetArray(p.Cfg.Source(), "targets", desired)
	if err != nil { // coverage-ignore: config.Load already parsed this config, so SetArray's parse/mapping checks cannot fail here
		return err
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault root bypasses
		return err
	}
	return runSync(ctx, root, stdout)
}

// enabledNames returns the config enable array for a singular CLI kind; the
// descriptor table in internal/project is the single source (ADR-0027).
func enabledNames(cfg *config.Config, kind string) []string {
	names, _ := project.EnabledNames(cfg, kind)
	return names
}

// catalogNames returns the catalog pool for a catalog-backed kind; the second
// result is false for `domain`, which is freeform (no catalog pool).
func catalogNames(cat *catalog.Catalog, kind string) ([]string, bool) {
	return project.CatalogNames(cat, kind)
}

// isGraphKind reports whether kind is one the ADR-0081 resolver plans over.
func isGraphKind(kind string) bool { return project.IsGraphKind(kind) }

// checkGraphFlags rejects a graph-only flag on a non-graph kind, so no flag
// is ever silently ignored (ADR-0081).
func checkGraphFlags(kind string, dryRun, withDependents bool) error {
	if isGraphKind(kind) || (!dryRun && !withDependents) {
		return nil
	}
	return &usageErr{fmt.Sprintf("graph flags (--dry-run, --with-dependents) apply to skill, agent, and doc only, not %q", kind)}
}

// printPlan renders one model-owned homogeneous plan collection.
func printPlan(stdout io.Writer, plan []project.PlanOp) error {
	document, err := project.PlanDocument(plan)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

// planEdits converts a resolver plan into enable-array edits (every op in a
// plan shares one add/remove direction).
func planEdits(plan []project.PlanOp) []enableEdit {
	edits := make([]enableEdit, 0, len(plan))
	for _, op := range plan {
		pl, _ := project.PluralKind(op.Node.Kind)
		edits = append(edits, enableEdit{key: pl, name: op.Node.Name})
	}
	return edits
}

// direction is the enable/disable axis of the shared toggle helper.
type direction int

const (
	enableDir direction = iota
	disableDir
)

// toggleFlags bundles the graph-plan flags shared by enable and disable
// (--with-dependents is disable-only, always false for enable).
type toggleFlags struct {
	dryRun         bool
	withDependents bool
}

// runEnable and runDisable are the two directions of the same spine: gate (by
// the driver) → checkGraphFlags → the target/bootstrap/hooks bespoke arms →
// PluralKind lookup → Open → per-direction validation → per-direction graph plan
// → rewrite → per-direction post-notes → sync. toggle holds the spine; the two
// entry points select the direction.
func runEnable(ctx context.Context, root, kind, name string, dryRun bool, stdout io.Writer) error {
	return toggle(ctx, root, kind, name, enableDir, toggleFlags{dryRun: dryRun}, stdout)
}

func toggle(ctx context.Context, root, kind, name string, dir direction, flags toggleFlags, stdout io.Writer) error {
	return toggleWithProjectLoader(ctx, root, kind, name, dir, flags, stdout, newProjectLoader)
}

func toggleWithProjectLoader(ctx context.Context, root, kind, name string, dir direction, flags toggleFlags, stdout io.Writer, loadProject func(string) (*project.Loader, error)) error {
	add := dir == enableDir
	if err := checkGraphFlags(kind, flags.dryRun, flags.withDependents); err != nil {
		return err
	}
	if kind == "target" {
		return enableDisableTarget(ctx, root, name, add, stdout)
	}
	if kind == "bootstrap" {
		return enableDisableSingleton(ctx, root, kind, add, stdout)
	}
	key, ok := project.PluralKind(kind)
	if !ok {
		return unknownKind(kind)
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	if add {
		if pool, catalogBacked := catalogNames(p.Cat, kind); catalogBacked {
			if !slices.Contains(pool, name) {
				return fmt.Errorf("%q is not a catalog %s (run: awf list %s)", name, kind, kind)
			}
		} else if err := config.ValidateDomainName(name); err != nil {
			return err
		}
		if slices.Contains(enabledNames(p.Cfg, kind), name) {
			return fmt.Errorf("%s %q already enabled", kind, name)
		}
	} else if !slices.Contains(enabledNames(p.Cfg, kind), name) {
		return fmt.Errorf("%s %q is not enabled", kind, name)
	}
	edits := []enableEdit{{key: key, name: name}}
	var plan []project.PlanOp
	if isGraphKind(kind) {
		if add {
			// Closure plan (ADR-0081 Decision 4): enabling an artifact enables its
			// full missing forward closure - skills, agents, and docs - in one
			// config rewrite, printed as a plan. Generalizes the ADR-0050 pairing
			// and subsumes the ADR-0013 doc advisory note.
			// touches-state: tooling/init-and-enablement:add-skill-pairs-agent - closure plan enables the required agent; proof in list_add_test.go
			plan = p.ResolveEnable(kind, name)
		} else {
			// Dependent-refusing removal (ADR-0081 Decision 5): the plan is the
			// target plus its enabled transitive dependents, printed before any
			// change; a longer plan refuses upfront - BEFORE the config rewrite,
			// so no half-broken tree is stranded. Generalizes the ADR-0050 agent
			// guard (the reverse walk's length-1 case).
			plan = p.ResolveDisable(kind, name)
		}
		if err := printPlan(stdout, plan); err != nil { // coverage-ignore: resolver plans contain only validated catalog nodes and fixed provenance syntax
			return err
		}
		if flags.dryRun {
			return nil
		}
		if !add && len(plan) > 1 && !flags.withDependents {
			return fmt.Errorf("disabling %s %q also disables the %d artifacts above; re-run with --with-dependents to apply", kind, name, len(plan)-1)
		}
		edits = planEdits(plan)
	}
	loader, err := loadProject(root)
	if err != nil {
		return err
	}
	if err := rewriteConfig(root, p.Cfg.Source(), add, edits...); err != nil { // coverage-ignore: rewriteConfig only errors on an unreachable SetArrayMember/write failure (the enabled/not-enabled guards and config.Load preclude it)
		return err
	}
	if add && project.IsFreeformDomainKind(kind) {
		if err := scaffoldDomainCurrentState(p, name); err != nil { // coverage-ignore: scaffoldDomainCurrentState only errors on an unreachable filesystem fault a test cannot trigger
			return err
		}
	}
	syncResult, _, err := syncMutation(ctx, loader, root, nil)
	if err != nil {
		return err
	}
	if add {
		return renderSyncMutation(stdout, syncResult)
	}
	var notes []project.EnablementNote
	for _, op := range plan {
		pl, _ := project.PluralKind(op.Node.Kind)
		if hasSidecarOrParts(root, pl, op.Node.Name) {
			notes = append(notes, project.EnablementNote{Reason: project.EnablementNoteOrphanedAuthoredState, Kind: op.Node.Kind, Name: op.Node.Name})
		}
	}
	if project.IsFreeformDomainKind(kind) && hasSidecarOrParts(root, key, name) {
		notes = append(notes, project.EnablementNote{Reason: project.EnablementNoteOrphanedAuthoredState, Kind: kind, Name: name})
	}
	notes = append(notes, unrequiredAgentNotes(p, plan)...)
	if len(notes) > 0 {
		if err := printEnablementNotes(stdout, notes); err != nil { // coverage-ignore: typed notes are synthesized from validated artifact identities and known reasons
			return err
		}
	}
	return renderSyncMutation(stdout, syncResult)
}

func printEnablementNotes(stdout io.Writer, notes []project.EnablementNote) error {
	document, err := project.EnablementNotesDocument(notes)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

// domainCurrentStateStub is the starter content for a new domain's current-state
// convention part: a concrete writing prompt instead of a blank file. It names the
// doc-standard path in plain text, not a markdown link, since doc-standard is an
// optional catalog doc - a link would risk ADR-0020's dead-reference gate on a
// project that hasn't enabled it.
const domainCurrentStateStub = "Describe where the %q domain stands today: its current shape, load-bearing constraints, and what a newcomer must know before changing it. Refresh by hand when the position materially shifts. Follow `docs/doc-standard.md` for tone: terse, present tense, reference other docs rather than restate them.\n"

// scaffoldDomainCurrentState writes name's current-state convention part with a
// starter prompt, unless one already exists - idempotent, so it never clobbers
// hand-authored content from a prior enable or a manual file.
func scaffoldDomainCurrentState(p *project.Project, name string) error {
	path := p.Cfg.PartPath("domains", name, "current-state")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) { // coverage-ignore: fails only on a permission fault a test cannot trigger
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // coverage-ignore: parent is under the just-validated .awf dir; fails only on a permission fault a test cannot trigger
		return err
	}
	return os.WriteFile(path, fmt.Appendf(nil, domainCurrentStateStub, name), 0o644)
}

func runDisable(ctx context.Context, root, kind, name string, withDependents, dryRun bool, stdout io.Writer) error {
	return toggle(ctx, root, kind, name, disableDir, toggleFlags{dryRun: dryRun, withDependents: withDependents}, stdout)
}

// unrequiredAgentNotes returns, after a cascade, a note for each still-enabled
// agent that a removed skill required and no remaining enabled, non-local
// skill still requires - agents are legal standalone (ADR-0050 Decision
// item 3), so they stay enabled (ADR-0081). Restricting to agents a removed
// skill required keeps "no longer" honest (a pre-existing standalone agent is
// not this cascade's doing), and local-sidecar agents mirror the resolver's
// skip.
func unrequiredAgentNotes(p *project.Project, plan []project.PlanOp) []project.EnablementNote {
	if len(plan) < 2 {
		return nil
	}
	removed := map[catalog.Node]bool{}
	wasRequiredBy := map[string]bool{}
	for _, op := range plan {
		removed[op.Node] = true
		if project.IsSkillKind(op.Node.Kind) {
			if a := p.Cat.Skills[op.Node.Name].RequiresAgent; a != "" {
				wasRequiredBy[a] = true
			}
		}
	}
	var notes []project.EnablementNote
	for _, agent := range p.Cfg.Agents {
		if removed[catalog.Node{Kind: "agent", Name: agent}] || !wasRequiredBy[agent] {
			continue
		}
		if sc, err := p.Cfg.Sidecar("agents", agent); err != nil || sc.Local {
			continue
		}
		required := false
		for _, skill := range p.Cfg.Skills {
			if removed[catalog.Node{Kind: "skill", Name: skill}] {
				continue
			}
			if sc, err := p.Cfg.Sidecar("skills", skill); err != nil || sc.Local {
				continue
			}
			if p.Cat.Skills[skill].RequiresAgent == agent {
				required = true
				break
			}
		}
		if !required {
			notes = append(notes, project.EnablementNote{Reason: project.EnablementNoteAgentNoLongerRequired, Kind: "agent", Name: agent})
		}
	}
	return notes
}

// enableEdit is one enable-array edit for rewriteConfig: the config key and
// the member name to add or remove.
type enableEdit struct{ key, name string }

// rewriteConfig applies one or more enable-array edits to src (the config.yaml
// bytes project.Open already read) in a single modify-write, so a paired
// skill+agent add lands in the same config rewrite (ADR-0050).
func rewriteConfig(root string, src []byte, add bool, edits ...enableEdit) error {
	b := src
	for _, e := range edits {
		var err error
		if b, err = config.SetArrayMember(b, e.key, e.name, add); err != nil { // coverage-ignore: callers guard add-present / remove-absent before this, and config.Load already rejected a malformed config, so SetArrayMember cannot error here
			return err
		}
	}
	if err := os.WriteFile(config.ConfigPath(root), b, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault that root bypasses
		return err
	}
	return nil
}

// hasSidecarOrParts reports whether an orphaned sidecar (<key>/<name>.yaml) or
// convention-parts dir (<key>/parts/<name>) for the target exists under .awf/.
func hasSidecarOrParts(root, key, name string) bool {
	awf := config.RootDir(root)
	for _, p := range []string{
		filepath.Join(awf, key, name+".yaml"),
		filepath.Join(awf, key, "parts", name),
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// runList selects streams and renders the complete model-owned collection.
func runList(ctx context.Context, root, kindFilter string, stdout io.Writer) error {
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	document, err := p.ListDocument(kindFilter)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
