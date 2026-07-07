package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/refs"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// AdvisoryNotes returns the non-failing render advisories in print order — the
// ADR-0045 unset-var notes, then the ADR-0070 stub notes — computed from one
// RenderAll pass plus the domain-doc generation, which renders outside it.
func (p *Project) AdvisoryNotes() ([]string, error) {
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	dds, err := p.generateDomainDocs()
	if err != nil {
		return nil, err
	}
	return append(p.unsetVarNotes(files), stubNotes(append(files, dds...))...), nil
}

// unsetVarNotes reports, per rendered artifact, the vars its assembled template
// references that are unset (missing or empty) in config — the non-failing
// render-completeness advisory (ADR-0045 item 4). One line per artifact with at
// least one hit, sorted; adapter duplicates are collapsed by template id.
func (p *Project) unsetVarNotes(files []RenderedFile) []string {
	seen := map[string]bool{}
	var notes []string
	for _, f := range files {
		if seen[f.TemplateID] {
			continue
		}
		seen[f.TemplateID] = true
		var unset []string
		for _, r := range render.ReferencedVars(f.assembled) {
			if v := p.Cfg.Vars[r]; v == nil || v == "" {
				unset = append(unset, r)
			}
		}
		if len(unset) == 0 {
			continue
		}
		notes = append(notes, fmt.Sprintf("%s references unset vars: %s",
			artifactLabel(f.TemplateID), strings.Join(unset, ", ")))
	}
	sort.Strings(notes)
	return notes
}

// stubNotes reports, per rendered artifact, its unauthored stub content —
// stub-attributed sections still at default and awf:stub-marked parts. One line
// per output path: artifacts sharing a template id (local artifacts, the domain
// docs) each report independently, and a multi-target project prints one line
// per target path by design (ADR-0070).
// invariant: stub-notes-path-keyed
func stubNotes(files []RenderedFile) []string {
	var notes []string
	for _, f := range files {
		if len(f.stubDefaults) == 0 && len(f.stubParts) == 0 {
			continue
		}
		var clauses []string
		if len(f.stubDefaults) > 0 {
			clauses = append(clauses, "sections at stub default: "+strings.Join(f.stubDefaults, ", "))
		}
		if len(f.stubParts) > 0 {
			clauses = append(clauses, "stub-marked parts: "+strings.Join(f.stubParts, ", "))
		}
		notes = append(notes, fmt.Sprintf("%s has unauthored stub content — %s",
			f.Path, strings.Join(clauses, "; ")))
	}
	sort.Strings(notes)
	return notes
}

// artifactLabel derives a human label from a template id: catalog kinds get
// "<kind> <name>" ("skill tdd", "agent code-reviewer", "doc testing"), hook
// payloads their script ("hooks pre-commit" — ADR-0048); the singletons read
// as their kind ("agents-doc").
func artifactLabel(tid string) string {
	segs := strings.Split(tid, "/")
	switch segs[0] {
	case "skills", "agents", "docs":
		name := segs[1]
		if segs[0] != "skills" {
			name = strings.TrimSuffix(name, ".md.tmpl")
		}
		return strings.TrimSuffix(segs[0], "s") + " " + name
	case "hooks":
		return "hooks " + strings.TrimSuffix(segs[1], ".sh.tmpl")
	default:
		return segs[0]
	}
}

// localOutPaths returns the conventional output paths awf would render a local
// skill/agent to — one per enabled target (the same formulas RenderAll uses); nil
// for neutral kinds. A local artifact must exist at every target's path (ADR-0037),
// so no target carries an unchecked hand-authored file.
func (p *Project) localOutPaths(kind, name string) []string {
	d, ok := descriptorByPlural(kind)
	if !ok || d.outPath == nil {
		return nil
	}
	paths := make([]string, 0, len(p.Targets))
	for _, t := range p.Targets {
		paths = append(paths, d.outPath(t, p.Cfg.Prefix, name))
	}
	return paths
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
			// A local artifact must be present and valid at every enabled target's path.
			for _, rel := range p.localOutPaths(kv.kind, name) {
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
	}
	return nil
}

// localTargetPaths returns the on-disk output paths of every declared local
// skill/agent across all enabled targets. RenderAll does not produce these
// (local artifacts are hand-authored), so Sync's prune must treat them as wanted;
// otherwise converting a skill from managed to local deletes its file.
func (p *Project) localTargetPaths() ([]string, error) {
	var paths []string
	for _, kv := range []struct {
		kind  string
		names []string
	}{{"skills", p.Cfg.Skills}, {"agents", p.Cfg.Agents}} {
		for _, name := range kv.names {
			sc, err := p.Cfg.Sidecar(kv.kind, name)
			if err != nil {
				return nil, err
			}
			if !sc.Local {
				continue
			}
			paths = append(paths, p.localOutPaths(kv.kind, name)...)
		}
	}
	return paths, nil
}

// orphans reports sidecar and convention-part files whose artifact is not in the
// matching enable list, plus convention-part files of an enabled artifact whose
// section is not catalog-declared (inv: drift-source-set; ADR-0011 section-orphan-flagged).
func (p *Project) orphans() ([]manifest.Drift, error) {
	var drift []manifest.Drift
	for _, desc := range kindDescriptors {
		kind := desc.Plural
		enabledSet := sliceSet(desc.enable(p.Cfg))
		base := filepath.Join(config.RootDir(p.Root), kind)
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
			if !enabledSet[name] {
				drift = append(drift, manifest.Drift{
					Path: filepath.Join(config.DirName, kind, e.Name()),
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
			if !enabledSet[t.Name()] {
				drift = append(drift, manifest.Drift{
					Path: filepath.Join(config.DirName, kind, "parts", t.Name()),
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
						Path: filepath.Join(config.DirName, kind, "parts", t.Name(), sf.Name()),
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
// RenderAll produces except the CLAUDE.md bridge and the non-markdown render units
// (the bootstrap, the git-hook payloads, and the memory gitignore — ADR-0048,
// ADR-0069).
func isManagedMarkdown(tid string) bool {
	return tid != bridgeTID && tid != bootstrapTID && tid != memoryTID &&
		!strings.HasPrefix(tid, "hooks/")
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
	drift = append(drift, p.checkDeadSkillRefs(files, amd, dds, p.effSkills)...)
	return drift, nil
}

// checkLockedFiles compares each lock entry (except the separately-checked
// generated ACTIVE.md / domain docs) against the freshly-rendered output and the
// on-disk file: orphaned, stale, missing, hand-edited, or invalid-frontmatter.
// The reverse direction is checked too: a rendered path with no lock entry — an
// artifact enabled since the last sync — is flagged unsynced rather than
// silently skipped.
func (p *Project) checkLockedFiles(lock *manifest.Lock, rendered map[string]RenderedFile, activeMdRel, domainsPrefix string) []manifest.Drift {
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(rendered)) {
		if _, ok := lock.Files[path]; !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "unsynced", Detail: "enabled but not in lock; run awf sync"})
		}
	}
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
	return p.regenDrift(activeMdRel, amd.Content,
		"ADR index absent; run awf sync", "ADR index out of date; run awf sync")
}

// regenDrift compares a freshly-generated file's content against its on-disk copy:
// a missing file or a hash mismatch yields one drift entry with the given details.
func (p *Project) regenDrift(rel, content, missingDetail, staleDetail string) []manifest.Drift {
	onDisk, err := os.ReadFile(filepath.Join(p.Root, rel))
	if err != nil {
		return []manifest.Drift{{Path: rel, Kind: "missing", Detail: missingDetail}}
	}
	if manifest.Hash(onDisk) != manifest.Hash([]byte(content)) {
		return []manifest.Drift{{Path: rel, Kind: "stale", Detail: staleDetail}}
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
		drift = append(drift, p.regenDrift(dd.Path, dd.Content,
			"domain doc absent; run awf sync", "domain doc out of date; run awf sync")...)
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		if strings.HasPrefix(path, domainsPrefix) && !produced[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "domain removed; run awf sync"})
		}
	}
	return drift
}

// checkDeadSkillRefs scans managed rendered markdown for <prefix>-<name> tokens
// whose <name> is a catalog-known skill outside the effective rendered set
// (inv: skill-ref-dead-fails). Names matching no known skill are ignored
// (inv: skill-ref-unknown-ignored); fenced code blocks are skipped like the
// dead-link scan. Matching is whole-token (ADR-0046 item 3): the token must not
// start mid-word (no word-ish rune before the prefix) and the regex captures
// the maximal word run after it.
// invariant: skill-ref-dead-fails
// invariant: skill-ref-unknown-ignored
func (p *Project) checkDeadSkillRefs(files []RenderedFile, amd RenderedFile, dds []RenderedFile, effective map[string]bool) []manifest.Drift {
	scan := make([]RenderedFile, 0, len(files)+1+len(dds))
	for _, f := range files {
		if isManagedMarkdown(f.TemplateID) {
			scan = append(scan, f)
		}
	}
	scan = append(scan, amd)
	scan = append(scan, dds...)
	re := regexp.MustCompile(`(?:^|[^a-zA-Z0-9_-])` + regexp.QuoteMeta(p.Cfg.Prefix) + `-([a-z0-9]+(?:-[a-z0-9]+)*)`)
	var drift []manifest.Drift
	for _, f := range scan {
		seen := map[string]bool{}
		for _, m := range re.FindAllStringSubmatch(refs.WithoutFences(f.Content), -1) {
			name := m[1]
			if _, known := p.Cat.Skills[name]; !known || effective[name] || seen[name] {
				continue
			}
			seen[name] = true
			drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-skill-reference", Detail: p.Cfg.Prefix + "-" + name})
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
