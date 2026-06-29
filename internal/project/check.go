package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/refs"
)

// localOutPath returns the conventional output path awf would render a local
// skill/agent to (the same formulas RenderAll uses); "" for neutral kinds.
func (p *Project) localOutPath(kind, name string) string {
	if d, ok := descriptorByPlural(kind); ok && d.outPath != nil {
		return d.outPath(p.Target, p.Cfg.Prefix, name)
	}
	return ""
}

// checkLocalFrontmatter validates the on-disk frontmatter of every declared local
// skill/agent at its conventional output path. fail wraps a path+error into the
// caller's accumulator (a hard error for Sync, a drift entry for Check).
func (p *Project) checkLocalFrontmatter(fail func(path string, err error)) error {
	for _, kv := range []struct {
		kind  string
		names []string
	}{{"skills", p.Cfg.Skills}, {"agents", p.Cfg.Agents}} {
		d, _ := descriptorByPlural(kv.kind)
		for _, name := range kv.names {
			sc, err := p.Cfg.Sidecar(kv.kind, name)
			if err != nil {
				return err
			}
			if !sc.Local {
				continue
			}
			rel := p.localOutPath(kv.kind, name)
			b, err := os.ReadFile(filepath.Join(p.Root, rel))
			if err != nil {
				fail(rel, fmt.Errorf("local %s file absent", d.Singular))
				continue
			}
			if err := validateFrontmatter(b); err != nil {
				fail(rel, err)
			}
		}
	}
	return nil
}

// orphans reports sidecar and convention-part files whose artifact is not in the
// matching enable list, plus convention-part files of an enabled artifact whose
// section is not catalog-declared (inv: drift-source-set; ADR-0011 section-orphan-flagged).
func (p *Project) orphans() ([]manifest.Drift, error) {
	enabled := map[string]map[string]bool{
		"skills":  sliceSet(p.Cfg.Skills),
		"agents":  sliceSet(p.Cfg.Agents),
		"docs":    sliceSet(p.Cfg.Docs),
		"domains": sliceSet(p.Cfg.Domains),
	}
	var drift []manifest.Drift
	for _, kind := range []string{"skills", "agents", "docs", "domains"} {
		base := filepath.Join(p.Root, ".awf", kind)
		// Sidecars: <kind>/<name>.yaml.
		entries, err := os.ReadDir(base)
		if errors.Is(err, os.ErrNotExist) {
			continue // kind branch absent → nothing to orphan
		} else if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			if !enabled[kind][name] {
				drift = append(drift, manifest.Drift{
					Path: filepath.Join(".awf", kind, e.Name()),
					Kind: "orphaned", Detail: "sidecar for an artifact not in the enable list",
				})
			}
		}
		// Parts: <kind>/parts/<target>/<section>.md.
		partsDir := filepath.Join(base, "parts")
		targets, err := os.ReadDir(partsDir)
		if errors.Is(err, os.ErrNotExist) {
			continue // no parts dir for this kind → nothing to orphan
		} else if err != nil {
			return nil, err
		}
		for _, t := range targets {
			if !t.IsDir() {
				continue
			}
			if !enabled[kind][t.Name()] {
				drift = append(drift, manifest.Drift{
					Path: filepath.Join(".awf", kind, "parts", t.Name()),
					Kind: "orphaned", Detail: "convention parts for an artifact not in the enable list",
				})
				continue
			}
			// Enabled target: flag part files whose section is not catalog-declared.
			declared := sliceSet(p.declaredSections(kind, t.Name()))
			sections, err := os.ReadDir(filepath.Join(partsDir, t.Name()))
			if err != nil { // coverage-ignore: os.ReadDir on an enabled target's existing parts directory fails only on a permission fault (a no-op as root)
				continue
			}
			for _, sf := range sections {
				if sf.IsDir() || !strings.HasSuffix(sf.Name(), ".md") {
					continue
				}
				if section := strings.TrimSuffix(sf.Name(), ".md"); !declared[section] {
					drift = append(drift, manifest.Drift{
						Path: filepath.Join(".awf", kind, "parts", t.Name(), sf.Name()),
						Kind: "orphaned", Detail: "convention part for a section not in the target's declared set",
					})
				}
			}
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return drift, nil
}

// declaredSections returns the catalog-declared section names for a target.
func (p *Project) declaredSections(kind, name string) []string {
	if d, ok := descriptorByPlural(kind); ok && d.sections != nil {
		s, _ := d.sections(p.Cat, name)
		return s
	}
	return nil
}

// isManagedMarkdown reports whether a RenderAll template id is awf-managed rendered
// markdown subject to the dead-reference scan (ADR-0020 Decision 3): everything
// RenderAll produces except the CLAUDE.md bridge.
func isManagedMarkdown(tid string) bool {
	return tid != "claude/CLAUDE.md.tmpl"
}

func (p *Project) Check() ([]manifest.Drift, error) {
	lock, err := manifest.Load(p.lockPath())
	if err != nil {
		return nil, fmt.Errorf("no lock (run awf sync): %w", err)
	}
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	rendered := map[string]RenderedFile{}
	for _, f := range files {
		rendered[f.Path] = f
	}
	lay := p.layout()
	activeMdRel := lay.ActiveMd
	domainsPrefix := lay.DomainsDir + "/"

	var drift []manifest.Drift
	drift = append(drift, p.checkLockedFiles(lock, rendered, activeMdRel, domainsPrefix)...)
	// Local skills/agents are not rendered, so their hand-authored frontmatter is
	// validated directly on disk.
	if err := p.checkLocalFrontmatter(func(path string, e error) {
		drift = append(drift, manifest.Drift{Path: path, Kind: "invalid-frontmatter", Detail: e.Error()})
	}); err != nil { // coverage-ignore: checkLocalFrontmatter only errors on a malformed local-target sidecar, which RenderAll above already surfaces earlier in Check
		return nil, err
	}
	// Orphan sidecars/parts (second clause of inv: drift-source-set).
	od, err := p.orphans()
	if err != nil {
		return nil, err
	}
	drift = append(drift, od...)

	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	drift = append(drift, p.checkActiveMD(activeMdRel, amd)...)

	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — the ACTIVE.md regenerate above parses the same decisions dir and fails first on a malformed ADR
		return nil, err
	}
	drift = append(drift, p.checkDomainDocs(lock, domainsPrefix, dds)...)

	drift = append(drift, p.checkDeadRefs(files, amd, dds)...)
	return drift, nil
}

// checkLockedFiles compares each lock entry (except the separately-checked
// generated ACTIVE.md / domain docs) against the freshly-rendered output and the
// on-disk file: orphaned, stale, missing, hand-edited, or invalid-frontmatter.
func (p *Project) checkLockedFiles(lock *manifest.Lock, rendered map[string]RenderedFile, activeMdRel, domainsPrefix string) []manifest.Drift {
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		if path == activeMdRel || strings.HasPrefix(path, domainsPrefix) {
			continue // generated artifacts — checked separately
		}
		e := lock.Files[path]
		rf, ok := rendered[path]
		if !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
			continue
		}
		if rf.TemplateHash != e.TemplateHash || rf.ConfigHash != e.ConfigHash {
			// stale takes precedence: a re-sync overwrites any hand-edit, so it
			// is the actionable signal — one drift entry per path.
			drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "template or config changed; run awf sync"})
			continue
		}
		onDisk, err := os.ReadFile(filepath.Join(p.Root, path))
		if err != nil {
			drift = append(drift, manifest.Drift{Path: path, Kind: "missing", Detail: "file absent; run awf sync"})
			continue
		}
		if manifest.Hash(onDisk) != e.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from lock; run awf sync to discard the edit, or move it into a .awf convention part to keep it"})
			continue
		}
		// In-sync skill/agent files must still carry valid frontmatter (subordinate
		// to the hash kinds above — a re-sync is the fix for those).
		if isSkillOrAgent(rf.TemplateID) {
			if err := validateFrontmatter(onDisk); err != nil {
				drift = append(drift, manifest.Drift{Path: path, Kind: "invalid-frontmatter", Detail: err.Error()})
			}
		}
	}
	return drift
}

// checkActiveMD regenerates the ADR index and compares it on disk. ACTIVE.md is
// generated from ADR frontmatter, not a template, so its staleness cannot be
// detected by the template/config hash comparison in checkLockedFiles.
func (p *Project) checkActiveMD(activeMdRel string, amd RenderedFile) []manifest.Drift {
	if onDisk, err := os.ReadFile(filepath.Join(p.Root, activeMdRel)); err != nil {
		return []manifest.Drift{{Path: activeMdRel, Kind: "missing", Detail: "ADR index absent; run awf sync"}}
	} else if manifest.Hash(onDisk) != manifest.Hash([]byte(amd.Content)) {
		return []manifest.Drift{{Path: activeMdRel, Kind: "stale", Detail: "ADR index out of date; run awf sync"}}
	}
	return nil
}

// checkDomainDocs compares each regenerated domain doc on disk and flags a lock
// entry no longer produced (domain removed). Like ACTIVE.md, domain docs are
// generated from ADR frontmatter + convention parts, so the lock hash cannot
// detect their staleness.
func (p *Project) checkDomainDocs(lock *manifest.Lock, domainsPrefix string, dds []RenderedFile) []manifest.Drift {
	var drift []manifest.Drift
	produced := map[string]bool{}
	for _, dd := range dds {
		produced[dd.Path] = true
		onDisk, err := os.ReadFile(filepath.Join(p.Root, dd.Path))
		if err != nil {
			drift = append(drift, manifest.Drift{Path: dd.Path, Kind: "missing", Detail: "domain doc absent; run awf sync"})
		} else if manifest.Hash(onDisk) != manifest.Hash([]byte(dd.Content)) {
			drift = append(drift, manifest.Drift{Path: dd.Path, Kind: "stale", Detail: "domain doc out of date; run awf sync"})
		}
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		if strings.HasPrefix(path, domainsPrefix) && !produced[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "domain removed; run awf sync"})
		}
	}
	return drift
}

// checkDeadRefs runs the dead-reference scan (inv: dead-reference-gated): every
// awf-managed rendered markdown file's inline links must resolve file-relative on
// disk. The generated ACTIVE.md and domain docs are in scope; the CLAUDE.md bridge
// is not.
func (p *Project) checkDeadRefs(files []RenderedFile, amd RenderedFile, dds []RenderedFile) []manifest.Drift {
	scan := make([]RenderedFile, 0, len(files)+1+len(dds))
	for _, f := range files {
		if isManagedMarkdown(f.TemplateID) {
			scan = append(scan, f)
		}
	}
	scan = append(scan, amd)
	scan = append(scan, dds...)
	var drift []manifest.Drift
	for _, f := range scan {
		base := filepath.Dir(f.Path)
		for _, target := range refs.Links(f.Content) {
			resolved := filepath.Join(p.Root, base, target)
			if _, err := os.Stat(resolved); err != nil {
				drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-reference", Detail: target})
			}
		}
	}
	return drift
}
