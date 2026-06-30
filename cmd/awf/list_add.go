package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func unknownKind(kind string) error {
	return &usageErr{fmt.Sprintf("unknown kind %q (want: skill, agent, doc, domain, target, bootstrap)", kind)}
}

// addRemoveBootstrap enables or disables the self-pinning bootstrap singleton in
// the config (ADR-0040). It is the bespoke path (bootstrap is not a kindDescriptor —
// it has no catalog pool / sections / plural enable array, so it stays out of the
// single dispatch table that inv: kind-dispatch-single-table guards): a nested
// bootstrap.enabled scalar, written via config.SetMappingScalar.
func addRemoveBootstrap(root string, add bool, stdout io.Writer) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	enabled := p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled
	if add && enabled {
		return errors.New("bootstrap already enabled")
	}
	if !add && !enabled {
		return errors.New("bootstrap is not enabled")
	}
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
		return err
	}
	updated, err := config.SetMappingScalar(b, "bootstrap", "enabled", add)
	if err != nil { // coverage-ignore: config.Load already parsed this config, so SetMappingScalar's parse/mapping checks cannot fail here
		return err
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault root bypasses
		return err
	}
	return runSync(root, stdout)
}

// addRemoveTarget enables or disables an adapter in the config targets array. It
// is the bespoke path (targets is not a kindDescriptor — ADR-0037): it validates
// against the known-adapter set and writes the full resolved list, since the
// targets array carries a Load default that an absent on-disk key would drop.
// invariant: target-cli
func addRemoveTarget(root, name string, add bool, stdout io.Writer) error {
	if !slices.Contains(project.KnownTargets(), name) {
		return fmt.Errorf("%q is not a known target (known: %s)", name, strings.Join(project.KnownTargets(), ", "))
	}
	p, err := project.Open(root)
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
		return fmt.Errorf("cannot remove the last target %q (a project must render to at least one)", name)
	}
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
		return err
	}
	updated, err := config.SetArray(b, "targets", desired)
	if err != nil { // coverage-ignore: config.Load already parsed this config, so SetArray's parse/mapping checks cannot fail here
		return err
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault root bypasses
		return err
	}
	return runSync(root, stdout)
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

func runAdd(root, kind, name string, stdout io.Writer) error {
	if kind == "target" {
		return addRemoveTarget(root, name, true, stdout)
	}
	if kind == "bootstrap" {
		return addRemoveBootstrap(root, true, stdout)
	}
	key, ok := project.PluralKind(kind)
	if !ok {
		return unknownKind(kind)
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
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
	if err := rewriteConfig(root, key, name, true); err != nil { // coverage-ignore: rewriteConfig only errors on an unreachable SetArrayMember/write failure (the already-enabled guard and config.Load preclude it)
		return err
	}
	// Doc-gated skill: warn when its required doc is not enabled, since it would
	// otherwise render nothing (inv: doc-gated-skill-suppressed).
	if kind == "skill" {
		if req := p.Cat.Skills[name].RequiresDoc; req != "" && !slices.Contains(p.Cfg.Docs, req) {
			fmt.Fprintf(stdout, "note: skill %q requires the %q doc, which is not enabled — it will not render until you run `awf add doc %s`\n", name, req, req)
		}
	}
	return runSync(root, stdout)
}

func runRemove(root, kind, name string, stdout io.Writer) error {
	if kind == "target" {
		return addRemoveTarget(root, name, false, stdout)
	}
	if kind == "bootstrap" {
		return addRemoveBootstrap(root, false, stdout)
	}
	key, ok := project.PluralKind(kind)
	if !ok {
		return unknownKind(kind)
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if !slices.Contains(enabledNames(p.Cfg, kind), name) {
		return fmt.Errorf("%s %q is not enabled", kind, name)
	}
	if err := rewriteConfig(root, key, name, false); err != nil { // coverage-ignore: rewriteConfig only errors on an unreachable SetArrayMember/write failure (the not-enabled guard and config.Load preclude it)
		return err
	}
	if hasSidecarOrParts(root, key, name) {
		fmt.Fprintf(stdout, "note: %s %q still has a sidecar or convention parts under .awf/ — now orphaned (awf check will flag them); delete them or re-add to keep them\n", kind, name)
	}
	return runSync(root, stdout)
}

// rewriteConfig edits the enable array for key in .awf/config.yaml (adding or
// removing name) and writes it back.
func rewriteConfig(root, key, name string, add bool) error {
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
		return err
	}
	updated, err := config.SetArrayMember(b, key, name, add)
	if err != nil { // coverage-ignore: callers guard add-present / remove-absent before this, and config.Load already rejected a malformed config, so SetArrayMember cannot error here
		return err
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault that root bypasses
		return err
	}
	return nil
}

// hasSidecarOrParts reports whether an orphaned sidecar (<key>/<name>.yaml) or
// convention-parts dir (<key>/parts/<name>) for the target exists under .awf/.
func hasSidecarOrParts(root, key, name string) bool {
	awf := filepath.Join(root, ".awf")
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

func runList(root, kindFilter string, stdout io.Writer) error {
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if kindFilter == "target" {
		fmt.Fprintln(stdout, "targets:")
		for _, n := range project.KnownTargets() {
			state := "available"
			if slices.Contains(p.Cfg.Targets, n) {
				state = "enabled"
			}
			fmt.Fprintf(stdout, "  %-28s %s\n", n, state)
		}
		return nil
	}
	if kindFilter == "bootstrap" {
		state := "available"
		if p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled {
			state = "enabled"
		}
		fmt.Fprintln(stdout, "bootstrap:")
		fmt.Fprintf(stdout, "  %-28s %s\n", "awf-bootstrap.sh", state)
		return nil
	}
	kinds := project.Kinds()
	if kindFilter != "" {
		if _, ok := project.PluralKind(kindFilter); !ok {
			return unknownKind(kindFilter)
		}
		kinds = []string{kindFilter}
	}
	for _, kind := range kinds {
		pl, _ := project.PluralKind(kind)
		fmt.Fprintf(stdout, "%s:\n", pl)
		pool, catalogBacked := catalogNames(p.Cat, kind)
		if !catalogBacked { // domains: configured set only
			for _, n := range slices.Sorted(slices.Values(p.Cfg.Domains)) {
				fmt.Fprintf(stdout, "  %-28s %s\n", n, "configured")
			}
			continue
		}
		for _, n := range pool {
			fmt.Fprintf(stdout, "  %-28s %s\n", n, artifactState(p, kind, n))
		}
	}
	return nil
}

// artifactState returns the display state of a catalog-backed artifact: "available"
// when not enabled, else "local"/"tuned"/"enabled" from its sidecar.
func artifactState(p *project.Project, kind, name string) string {
	if !slices.Contains(enabledNames(p.Cfg, kind), name) {
		return "available"
	}
	// project.Open pre-validated every enabled sidecar, so a read here cannot fail.
	pl, _ := project.PluralKind(kind)
	sc, _ := p.Cfg.Sidecar(pl, name)
	switch {
	case sc.Local:
		return "local"
	case sc.Data != nil || sc.Sections != nil:
		return "tuned"
	}
	return "enabled"
}
