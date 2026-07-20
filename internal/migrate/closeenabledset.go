package migrate

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyCloseEnabledSet ports a tree to the enforced dependency graph
// (ADR-0081 Decision 8), in two ordered steps: first every dormant doc-gated
// skill - enabled while its doc is disabled, the pre-schema-8 valid
// silent-suppression state - is dropped, preserving the adopter's observed
// rendered output (the drop skips `local:`-owned skills, symmetric with the
// validator: a local doc-gated skill renders today even without its doc);
// then an additive fixed point over all three Requires* edge kinds enables
// every remaining enabled artifact's missing skills, agents, and docs - so a
// dormant skill something still requires re-enters with its doc. Idempotent;
// every addition and drop is printed; the config write is atomic.
func applyCloseEnabledSet(root string, out io.Writer) error {
	return closeEnabledSet(root, catalog.Standard, out)
}

// closeEnabledSet is the catalog-parameterized seam behind applyCloseEnabledSet,
// so the demanded-dormant re-add interplay (unreachable in the shipped catalog -
// nothing requires a doc-gated skill today) stays testable synthetically.
func closeEnabledSet(root string, cat *catalog.Catalog, out io.Writer) error {
	if _, err := os.Stat(config.ConfigPath(root)); os.IsNotExist(err) {
		return nil // no config: nothing to close (idempotent re-run safe)
	}
	cfg, err := loadForMigration(root)
	if err != nil {
		return err
	}
	local := func(kind, name string) bool {
		sc, err := cfg.Sidecar(kind, name)
		return err == nil && sc.Local
	}

	enabled := map[catalog.Node]bool{}
	for _, s := range cfg.Skills {
		enabled[catalog.Node{Kind: "skill", Name: s}] = true
	}
	for _, a := range cfg.Agents {
		enabled[catalog.Node{Kind: "agent", Name: a}] = true
	}
	for _, d := range cfg.Docs {
		enabled[catalog.Node{Kind: "doc", Name: d}] = true
	}

	// Step 1: drop dormant non-local doc-gated skills.
	var drops []catalog.Node
	for _, s := range slices.Sorted(slices.Values(cfg.Skills)) {
		req := cat.Skills[s].RequiresDoc
		if req == "" || local("skills", s) || enabled[catalog.Node{Kind: "doc", Name: req}] {
			continue
		}
		n := catalog.Node{Kind: "skill", Name: s}
		delete(enabled, n)
		drops = append(drops, n)
		fmt.Fprintf(out, "close-enabled-set: dropped dormant skill %q (its %q doc is disabled)\n", s, req)
	}

	// Step 2: additive fixed point over the direct requirement edges of every
	// enabled, non-local skill and agent. Iteration is sorted so the printed
	// plan and the resulting enable arrays are deterministic.
	var adds []catalog.Node
	for changed := true; changed; {
		changed = false
		nodes := slices.SortedFunc(maps.Keys(enabled), func(a, b catalog.Node) int {
			if a.Kind != b.Kind {
				return strings.Compare(a.Kind, b.Kind)
			}
			return strings.Compare(a.Name, b.Name)
		})
		for _, n := range nodes {
			if n.Kind == "doc" || local(n.Kind+"s", n.Name) {
				continue
			}
			for _, r := range catalog.RequiresOf(cat, n) {
				if enabled[r] {
					continue
				}
				enabled[r] = true
				adds = append(adds, r)
				fmt.Fprintf(out, "close-enabled-set: enabled %s %q (required by %q)\n", r.Kind, r.Name, n.Name)
				changed = true
			}
		}
	}
	if len(drops) == 0 && len(adds) == 0 {
		return nil
	}
	return editConfig(root, func(src []byte) ([]byte, error) {
		b := src
		var err error
		for _, n := range drops {
			if b, err = config.SetArrayMember(b, n.Kind+"s", n.Name, false); err != nil { // coverage-ignore: config.Load already parsed this config, so SetArrayMember cannot error here
				return nil, err
			}
		}
		for _, n := range adds {
			if b, err = config.SetArrayMember(b, n.Kind+"s", n.Name, true); err != nil { // coverage-ignore: config.Load already parsed this config, so SetArrayMember cannot error here
				return nil, err
			}
		}
		return b, nil
	})
}
