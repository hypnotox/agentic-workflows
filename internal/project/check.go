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

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/refs"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// AdvisoryNotes returns the non-failing render advisories in print order - the
// ADR-0045 unset-var notes, the ADR-0070 stub notes, then the ADR-0083 part-
// marker notes - computed from one RenderAll pass plus the domain-doc
// generation, which renders outside it.
func (p *Project) AdvisoryNotes() ([]string, error) {
	p.beginInvocation()
	files, err := p.RenderAll()
	if err != nil { // coverage-ignore: AdvisoryNotes callers exercise render failures through RenderAll directly
		return nil, err
	}
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: OutputPlan's RenderAll has already generated domains
		return nil, err
	}
	all := slices.Concat(files, dds)
	// The generated config reference joins the stub/marker scan: its intro
	// part is authored like any other and can carry residue (ADR-0088).
	if cref, ok, err := p.generateConfigReference(all); err != nil { // coverage-ignore: OutputPlan's RenderAll has already generated the config reference
		return nil, err
	} else if ok {
		all = append(all, *cref)
	}
	notes := append(p.unsetVarNotes(files), stubNotes(all)...)
	notes = append(notes, markerNotes(all)...)
	th, err := p.tagHealthNotes()
	if err != nil { // coverage-ignore: advisory read errors are covered by direct helper tests
		return nil, err
	}
	notes = append(notes, th...)
	pcs, err := p.planCommitScopeNotes()
	if err != nil { // coverage-ignore: advisory read errors are covered by direct helper tests
		return nil, err
	}
	notes = append(notes, pcs...)
	sn, err := p.supersessionNotes()
	if err != nil { // coverage-ignore: advisory read errors are covered by direct helper tests
		return nil, err
	}
	return append(notes, sn...), nil
}

// tagFrequencyThreshold is the share of tag-bearing artifacts above which a tag
// is flagged as coarsening toward domain scale (ADR-0109 item 4). Advisory only;
// a documented constant, deliberately not a config key in this slice.
const tagFrequencyThreshold = 0.25

// tagHealthNotes returns advisory (non-failing) notes about the tag vocabulary's
// health: a frequency note for any tag carried by more than tagFrequencyThreshold
// of the tag-bearing artifacts (the coarsening the exact tag≠domain gate cannot
// express), and a coverage note for any ADR or pitfall carrying zero tags (the
// under-tagging backstop). Inert under an empty/absent vocabulary, so an
// un-curated adopter - and the example - stays note-free.
// invariant: tag-frequency-note
// invariant: tag-coverage-note
func (p *Project) tagHealthNotes() ([]string, error) {
	if len(p.Cfg.Tags) == 0 {
		return nil, nil
	}
	corpus, err := p.Corpus()
	if err != nil {
		return nil, err
	}
	adrs := corpus.All()
	pf, err := p.pitfallTagEntries()
	if err != nil {
		return nil, err
	}
	type artifact struct {
		label string
		tags  []string
	}
	var arts []artifact
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
	for _, a := range adrs {
		arts = append(arts, artifact{label: rel + "/" + a.Filename, tags: a.Tags})
	}
	for _, e := range pf {
		arts = append(arts, artifact{label: e.Title, tags: e.Tags})
	}

	var notes []string
	tagged := 0
	freq := map[string]int{}
	for _, art := range arts {
		if len(art.tags) == 0 {
			notes = append(notes, art.label+" carries no tags: add a narrow topic tag")
			continue
		}
		// Count only vocabulary members - both the numerator and the denominator.
		// The invariant speaks of "vocabulary tags" and "artifacts carrying at
		// least one vocabulary tag", and a non-member tag is already a hard
		// checkTagVocabulary failure, so it must not skew the coarsening signal.
		var vocab []string
		for _, t := range art.tags {
			if _, ok := p.Cfg.Tags[t]; ok {
				vocab = append(vocab, t)
			}
		}
		if len(vocab) == 0 {
			continue
		}
		tagged++
		for _, t := range vocab {
			freq[t]++
		}
	}
	// Empty-denominator guard: no tag-bearing artifacts, no frequency to compute.
	if tagged > 0 {
		for _, t := range slices.Sorted(maps.Keys(freq)) {
			if float64(freq[t]) > tagFrequencyThreshold*float64(tagged) {
				notes = append(notes, fmt.Sprintf("tag %q is on %d/%d tagged artifacts (>%.0f%%): coarsening toward domain scale", t, freq[t], tagged, tagFrequencyThreshold*100))
			}
		}
	}
	return notes, nil
}

// unsetVarNotes reports, per rendered artifact, the vars its assembled template
// references whose key is present in config with an empty or null value - the
// non-failing render-completeness advisory (ADR-0045 item 4, narrowed by
// ADR-0087: an absent key is the deliberate, git-auditable decline and produces
// no note; deleting the key is the acknowledgement). One line per artifact with
// at least one hit, sorted. Duplicates collapse by the note itself: adapter
// duplicates produce identical notes, while base-shared artifacts (project-local
// skills all render from one base template id) each report their own vars under
// a path-derived label (see localLabel).
func (p *Project) unsetVarNotes(files []RenderedFile) []string {
	seen := map[string]bool{}
	var notes []string
	for _, f := range files {
		var unset []string
		for _, r := range render.ReferencedVars(f.assembled) {
			// invariant: absent-var-acknowledged
			if v, ok := p.Cfg.Vars[r]; ok && (v == nil || v == "") {
				unset = append(unset, r)
			}
		}
		if len(unset) == 0 {
			continue
		}
		label := artifactLabel(f.TemplateID)
		if f.TemplateID == baseSkillTID || f.TemplateID == baseAgentTID {
			label = localLabel(f.TemplateID, f.Path)
		}
		note := fmt.Sprintf("%s references unset vars: %s; set a value, or delete the key to accept the generic prose",
			label, strings.Join(unset, ", "))
		if seen[note] {
			continue
		}
		seen[note] = true
		notes = append(notes, note)
	}
	sort.Strings(notes)
	return notes
}

// stubNotes reports, per rendered artifact, its unauthored stub content -
// stub-attributed sections still at default and awf:stub-marked parts. One line
// per output path: artifacts sharing a template id (local artifacts, the domain
// docs) each report independently, and a multi-target project prints one line
// per target path by design (ADR-0070).
// touches-invariant: stub-notes-path-keyed - per-output-path stub note; proof in notes_test.go
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
		notes = append(notes, fmt.Sprintf("%s has unauthored stub content: %s",
			f.Path, strings.Join(clauses, "; ")))
	}
	sort.Strings(notes)
	return notes
}

// markerNotes reports each convention part whose raw body carries a whole-line
// section-marker residue - the ADR-0083 advisory. Keyed by the part path, a
// deliberate deviation from stubNotes' output-path keying (the actionable file
// is the part itself), and deduplicated: multi-target rendering consumes the
// same part once per target and must not repeat its note.
func markerNotes(files []RenderedFile) []string {
	seen := map[string]bool{}
	var notes []string
	for _, f := range files {
		for _, part := range f.markerParts {
			if seen[part] {
				continue
			}
			seen[part] = true
			notes = append(notes, fmt.Sprintf("part %s contains a marker-shaped line: section markers have no effect inside convention parts; fence the example to silence this note", part))
		}
	}
	sort.Strings(notes)
	return notes
}

// unusedVarDrift reports each non-empty vars: key referenced by no rendered
// artifact - neither a .vars.X reference in any assembled source (RenderAll
// output and the generated domain docs, passed concatenated) nor a
// gateCmd/checkCmd part placeholder (ADR-0086 Decision 3). Empty values are
// exempt: they are the ADR-0022 seeded open-to-do state, which the unset-var
// note owns nudging (ADR-0087 - presence, not emptiness, is that note's
// trigger; this exemption keeps the seed-all-vars scaffold legal). A bare
// .vars reference conservatively consumes every key.
// invariant: unused-var-drift
func (p *Project) unusedVarDrift(files []RenderedFile) []manifest.Drift {
	used := map[string]bool{}
	for _, f := range files {
		if render.ReferencesBareVars(f.assembled) {
			return nil
		}
		for _, r := range render.ReferencedVars(f.assembled) {
			used[r] = true
		}
		for _, r := range f.partVarRefs {
			used[r] = true
		}
	}
	var drift []manifest.Drift
	for _, k := range slices.Sorted(maps.Keys(p.Cfg.Vars)) {
		if v := p.Cfg.Vars[k]; v == nil || v == "" || used[k] {
			continue
		}
		drift = append(drift, manifest.Drift{
			Path: config.DirName + "/config.yaml", Kind: "unused-var",
			Detail: fmt.Sprintf("var %q is set but referenced by no rendered artifact; delete it from vars: or enable an artifact that consumes it", k),
		})
	}
	return drift
}

// unusedDataDrift reports, per enabled artifact, the sidecar data: keys its
// assembled sources reference nowhere, unioned across enabled targets
// (ADR-0086 Decision 4). Domains are excluded - their sidecars are rejected
// as paths-only at open. A local: true sidecar renders nothing, so every
// key reports. A key referenced only inside a dropped section counts as
// unused: the drop makes it configuration that does nothing.
// invariant: unused-data-drift
func (p *Project) unusedDataDrift(files []RenderedFile) ([]manifest.Drift, error) {
	type refset struct {
		keys map[string]bool
		bare bool
	}
	refs := map[string]*refset{}
	for _, f := range files {
		key := f.kind + "\x00" + f.artifact
		rs := refs[key]
		if rs == nil {
			rs = &refset{keys: map[string]bool{}}
			refs[key] = rs
		}
		for _, k := range render.ReferencedDataKeys(f.assembled) {
			rs.keys[k] = true
		}
		rs.bare = rs.bare || render.ReferencesBareData(f.assembled)
	}
	var drift []manifest.Drift
	check := func(kind, name, sidecarRel string) error {
		sc, err := p.Cfg.Sidecar(kind, name)
		if err != nil { // coverage-ignore: this sidecar was already read by RenderAll (or validation) in the same Check pass
			return err
		}
		if len(sc.Data) == 0 {
			return nil
		}
		rs := refs[kind+"\x00"+name]
		if rs != nil && rs.bare {
			return nil
		}
		var unused []string
		for _, k := range slices.Sorted(maps.Keys(sc.Data)) {
			if rs == nil || !rs.keys[k] {
				unused = append(unused, k)
			}
		}
		if len(unused) == 0 {
			return nil
		}
		detail := "data keys referenced by no rendered section: " + strings.Join(unused, ", ") + "; a key referenced only inside a dropped section counts as unused; remove the key or the drop"
		if sc.Local {
			detail = "local: true renders nothing, so no data key is consumed; remove the data block: " + strings.Join(unused, ", ")
		}
		drift = append(drift, manifest.Drift{Path: sidecarRel, Kind: "unused-data", Detail: detail})
		return nil
	}
	for _, d := range kindDescriptors {
		if d.Plural == "domains" {
			continue
		}
		for _, name := range d.enable(p.Cfg) {
			if err := check(d.Plural, name, config.DirName+"/"+d.Plural+"/"+name+".yaml"); err != nil { // coverage-ignore: see check's coverage-ignore
				return nil, err
			}
		}
	}
	for _, kind := range catalog.SingletonKinds() {
		if err := check(kind, "", config.DirName+"/"+kind+".yaml"); err != nil { // coverage-ignore: see check's coverage-ignore
			return nil, err
		}
	}
	return drift, nil
}

// artifactLabel derives a human label from a template id: catalog kinds get
// "<kind> <name>" ("skill tdd", "agent code-reviewer", "doc testing"), hook
// payloads their script ("hooks pre-commit" - ADR-0048); the singletons read
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

// localLabel labels a base-shared project-local artifact (ADR-0068) by its
// output path: every local skill/agent renders from the same base template id,
// so the template-derived label ("skill _base") cannot say which artifact a
// note is about. The name keeps its on-disk form (skill directories keep the
// "<prefix>-" prefix); adapter duplicates share one path-derived name, so note
// dedup still collapses them.
func localLabel(tid, path string) string {
	if tid == baseSkillTID {
		return "skill " + filepath.Base(filepath.Dir(path))
	}
	return "agent " + strings.TrimSuffix(filepath.Base(path), ".md")
}

// validateArtifact validates an artifact using its declared encoder, never a
// filename suffix. This keeps policy routing independent of path spelling.
func validateArtifact(content []byte, encoder AgentDialect) error {
	if encoder == TOMLAgentDialect {
		return validateTOMLAgent(content)
	}
	return validateFrontmatter(content)
}

// localOutPaths returns the conventional output paths for a local artifact.
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

// declaredSections returns the catalog-declared section names for a target.
func (p *Project) declaredSections(kind, name string) []string {
	if d, ok := descriptorByPlural(kind); ok && d.sections != nil {
		s, _ := d.sections(p.Cat, name)
		return s
	}
	return nil
}

func (p *Project) Check() ([]manifest.Drift, error) {
	p.beginInvocation()
	lock, found, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("no lock (run awf sync)")
	}
	op, err := p.OutputPlan()
	if err != nil {
		return nil, err
	}
	files := op.writeFiles()
	rendered := map[string]RenderedFile{}
	for _, f := range files {
		rendered[f.Path] = f
	}
	var drift []manifest.Drift
	drift = append(drift, p.checkLockedFiles(lock, rendered)...)
	// Local reservations are validated from their declared node policy.
	p.localReservations(op, func(path string, e error) {
		drift = append(drift, manifest.Drift{Path: path, Kind: "invalid-frontmatter", Detail: e.Error()})
	})
	// Closed-tree sweep: orphans, strays, backups (ADR-0086 Decision 1).
	od, err := p.sweepConfigTree(files)
	if err != nil { // coverage-ignore: the sweep errors only on faults RenderAll above would have surfaced first (see its coverage-ignores)
		return nil, err
	}
	drift = append(drift, od...)

	// Generated and ordinary outputs are already exactly the plan write nodes;
	// do not regenerate a second, duplicate node set in Check.
	drift = append(drift, p.unusedVarDrift(files)...)
	ud, err := p.unusedDataDrift(files)
	if err != nil { // coverage-ignore: unusedDataDrift re-reads sidecars RenderAll already read
		return nil, err
	}
	drift = append(drift, ud...)

	drift = append(drift, p.checkDeadRefs(files)...)
	drift = append(drift, p.checkDeadSkillRefs(files, p.effSkills)...)

	planDrift, err := p.checkPlans()
	if err != nil {
		return nil, err
	}
	drift = append(drift, planDrift...)
	pitfallDrift, err := p.checkPitfalls()
	if err != nil { // coverage-ignore: every error checkPitfalls returns is pre-empted by an earlier Check() step (RenderAll's transform reads data.pitfalls; checkPlans parses the decisions dir), so this wiring branch is unreachable
		return nil, err
	}
	drift = append(drift, pitfallDrift...)
	tagDrift, err := p.checkTagVocabulary()
	if err != nil { // coverage-ignore: checkTagVocabulary's reads are pre-empted by earlier Check() steps (checkPlans ParseDir, RenderAll's pitfalls transform)
		return nil, err
	}
	drift = append(drift, tagDrift...)
	relDrift, err := p.checkADRRelatedLinks()
	if err != nil { // coverage-ignore: adr.ParseDir here is pre-empted by checkPlans
		return nil, err
	}
	drift = append(drift, relDrift...)
	supDrift, err := p.checkSupersessionAll()
	if err != nil { // coverage-ignore: adr.ParseDir here is pre-empted by checkPlans
		return nil, err
	}
	drift = append(drift, supDrift...)
	citeDrift, err := p.checkCitations()
	if err != nil { // coverage-ignore: adr.ParseDir here is pre-empted by checkPlans
		return nil, err
	}
	drift = append(drift, citeDrift...)
	return drift, nil
}

// checkLockedFiles compares each lock entry (except the separately-checked
// regeneration-checked artifacts - the generated ACTIVE.md / domain docs / config
// reference) against the freshly-rendered output and the on-disk file: orphaned,
// stale, missing, hand-edited, or invalid-frontmatter. The reverse direction is
// checked too: a rendered path with no lock entry - an artifact enabled since the
// last sync - is flagged unsynced rather than silently skipped.
func (p *Project) checkLockedFiles(lock *manifest.Lock, rendered map[string]RenderedFile) []manifest.Drift {
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(rendered)) {
		if _, ok := lock.Files[path]; !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "unsynced", Detail: "enabled but not in lock; run awf sync"})
		}
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		e := lock.Files[path]
		rf, ok := rendered[path]
		if rf.Policy.Regenerate {
			// Every regeneration-checked path is a planned rendered node. Compare
			// it once here rather than reconstructing generated outputs elsewhere.
			if !ok { // coverage-ignore: full Check first builds the complete planned node set; only a direct malformed lock/map call can omit a regeneration node.
				drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
				continue
			}
			onDisk, err := os.ReadFile(filepath.Join(p.Root, path))
			if err != nil {
				drift = append(drift, manifest.Drift{Path: path, Kind: "missing", Detail: "file absent; run awf sync"})
				continue
			}
			if manifest.Hash(onDisk) != manifest.Hash([]byte(rf.Content)) {
				if rf.TemplateID == "" {
					drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "generated output out of date; run awf sync"})
				} else {
					// touches-invariant: in-place-tamper-drift - awf-region/structure edit drifts, in-place edit does not; proof in check_test.go
					drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from the regenerated file; run awf sync to restore awf-owned regions"})
				}
			}
			continue
		}
		if !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
			continue
		}
		if rf.TemplateHash != e.TemplateHash || rf.ConfigHash != e.ConfigHash {
			// stale takes precedence: a re-sync overwrites any hand-edit, so it
			// is the actionable signal - one drift entry per path.
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
		// to the hash kinds above - a re-sync is the fix for those).
		if rf.Policy.ValidateFrontmatter {
			if err := validateArtifact(onDisk, rf.Encoder); err != nil {
				drift = append(drift, manifest.Drift{Path: path, Kind: "invalid-frontmatter", Detail: err.Error()})
			}
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
func (p *Project) checkDeadSkillRefs(files []RenderedFile, effective map[string]bool) []manifest.Drift {
	scan := make([]RenderedFile, 0, len(files))
	for _, f := range files {
		if f.Policy.ScanSkillReferences {
			scan = append(scan, f)
		}
	}
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
// disk. Generated nodes use the same declared policy; bridges remain out of
// scope through theirs.
func (p *Project) checkDeadRefs(files []RenderedFile) []manifest.Drift {
	scan := make([]RenderedFile, 0, len(files))
	for _, f := range files {
		if f.Policy.ScanReferences {
			scan = append(scan, f)
		}
	}
	var drift []manifest.Drift
	for _, f := range scan {
		base := filepath.Dir(f.Path)
		for _, target := range refs.Links(f.Content) {
			// A leading-/ target is repo-root-relative; everything else resolves
			// file-relative. A target escaping the root is dead by definition -
			// a host path outside the repo must never validate it.
			resolved := filepath.Join(p.Root, base, target)
			if strings.HasPrefix(target, "/") {
				resolved = filepath.Join(p.Root, target)
			}
			if rel, err := filepath.Rel(p.Root, resolved); err != nil || (rel != "." && !filepath.IsLocal(rel)) {
				drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-reference", Detail: target})
				continue
			}
			if _, err := os.Stat(resolved); err != nil {
				drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-reference", Detail: target})
			}
		}
	}
	return drift
}

// checkPlans validates plan frontmatter, plan→ADR links, and planned commit
// subjects over docs/plans/, scanning the YYYY-MM-DD-*.md set only (excluding
// template.md and README.md). Frontmatter-less plans (the grandfathered corpus,
// ADR-0098) are skipped. A ```commit subject's length/type/shape violation is
// drift; an unknown scope is advisory (planCommitScopeNotes), not drift (ADR-0111).
// invariant: plan-frontmatter-validated
// invariant: plan-adr-link-resolved
// invariant: plan-commit-subject-length-checked
// invariant: plan-commit-subject-shape-checked
func (p *Project) checkPlans() ([]manifest.Drift, error) {
	plansDir := filepath.Join(p.Root, p.Cfg.DocsDir, "plans")
	plans, err := plan.ParseDir(plansDir)
	if err != nil {
		return nil, err
	}
	corpus, err := p.Corpus()
	if err != nil {
		return nil, err
	}
	aset := audit.Resolve(p.Cfg.Audit)
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "plans"))
	var drift []manifest.Drift
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		path := rel + "/" + pl.Filename
		if !plan.ValidStatuses[pl.Status] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "plan-frontmatter", Detail: fmt.Sprintf("status %q not in {Proposed, Implemented}", pl.Status)})
		}
		for _, n := range pl.ADRs {
			if !corpus.Has(fmt.Sprintf("%04d", n)) {
				drift = append(drift, manifest.Drift{Path: path, Kind: "plan-adr-link", Detail: fmt.Sprintf("ADR-%04d", n)})
			}
		}
		for _, sub := range pl.CommitSubjects {
			for _, f := range audit.CheckPlannedSubject(sub, aset) {
				if f.Severity == audit.Error {
					drift = append(drift, manifest.Drift{Path: path, Kind: "plan-commit-subject", Detail: f.Detail})
				}
			}
		}
	}
	return drift, nil
}

// planCommitScopeNotes returns advisory (non-failing) notes for a plan's ```commit
// subject naming a scope outside the configured allow-list. Unlike an over-length or
// mistyped subject (hard drift in checkPlans), an unknown scope is advisory: a plan
// may be the change that adds the scope (ADR-0111). Mirrors checkPlans' scan; a
// frontmatter-less plan is skipped.
// invariant: plan-commit-subject-scope-advisory
func (p *Project) planCommitScopeNotes() ([]string, error) {
	plans, err := plan.ParseDir(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"))
	if err != nil {
		return nil, err
	}
	aset := audit.Resolve(p.Cfg.Audit)
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "plans"))
	var notes []string
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		for _, sub := range pl.CommitSubjects {
			for _, f := range audit.CheckPlannedSubject(sub, aset) {
				if f.Severity == audit.Warning {
					notes = append(notes, fmt.Sprintf("%s/%s: planned commit %s", rel, pl.Filename, f.Detail))
				}
			}
		}
	}
	return notes, nil
}

// checkPitfalls validates the pitfalls sidecar when the doc is enabled: each entry's
// domains: must resolve to a configured domain, and each related: number to an
// existing ADR. Structural validation (title/body) is the transform's job; this
// resolves the links the transform cannot see. A disabled pitfalls doc, or a sidecar
// with no data.pitfalls, yields no drift.
// invariant: pitfall-domains-resolved
// invariant: pitfall-adr-link-resolved
func (p *Project) checkPitfalls() ([]manifest.Drift, error) {
	if !slices.Contains(p.Cfg.Docs, "pitfalls") {
		return nil, nil
	}
	sc, err := p.Cfg.Sidecar("docs", "pitfalls")
	if err != nil { // coverage-ignore: the pitfalls sidecar's YAML was already parsed and validated at Open, so this re-read cannot fail
		return nil, err
	}
	entries, err := pitfallEntries(sc.Data["pitfalls"])
	if err != nil {
		return nil, err
	}
	domains := map[string]bool{}
	for _, d := range p.Cfg.Domains {
		domains[d] = true
	}
	corpus, err := p.Corpus()
	if err != nil {
		return nil, err
	}
	var drift []manifest.Drift
	for _, e := range entries {
		for _, d := range e.Domains {
			if !domains[d] {
				drift = append(drift, manifest.Drift{Path: pitfallsSidecarPath, Kind: "pitfall-domain", Detail: fmt.Sprintf("%q: unknown domain %q", e.Title, d)})
			}
		}
		for _, n := range e.Related {
			if !corpus.Has(fmt.Sprintf("%04d", n)) {
				drift = append(drift, manifest.Drift{Path: pitfallsSidecarPath, Kind: "pitfall-adr-link", Detail: fmt.Sprintf("%q: ADR-%04d", e.Title, n)})
			}
		}
	}
	return drift, nil
}

// checkTagVocabulary validates tag governance when the config tags: vocabulary
// is non-empty: every tag used by an ADR (frontmatter tags:) or a pitfall
// (tags:) must be a declared vocabulary member, and every member must declare a
// non-empty meaning. An empty or absent vocabulary is inert (tags are then
// free-form). A declared member no artifact uses is intentionally permitted,
// mirroring an unused configured domain under pitfall-domains-resolved.
// invariant: tag-vocabulary-governed
func (p *Project) checkTagVocabulary() ([]manifest.Drift, error) {
	if len(p.Cfg.Tags) == 0 {
		return nil, nil
	}
	cfgPath := config.DirName + "/config.yaml"
	domainName := map[string]bool{}
	for _, d := range p.Cfg.Domains {
		domainName[d] = true
	}
	var drift []manifest.Drift
	for _, tag := range slices.Sorted(maps.Keys(p.Cfg.Tags)) {
		if strings.TrimSpace(p.Cfg.Tags[tag]) == "" {
			drift = append(drift, manifest.Drift{Path: cfgPath, Kind: "tag-vocabulary", Detail: fmt.Sprintf("tag %q has an empty meaning", tag)})
		}
		// A tag must be finer than a domain (ADR-0109): a vocabulary member that
		// names a configured domain is the coarse-tag regression, gated exactly.
		// invariant: tag-not-domain-name
		if domainName[tag] {
			drift = append(drift, manifest.Drift{Path: cfgPath, Kind: "tag-domain-collision", Detail: fmt.Sprintf("tag %q equals a configured domain name: tags must be finer than domains", tag)})
		}
	}
	corpus, err := p.Corpus()
	if err != nil { // reachable via a direct checkTagVocabulary call over a malformed ADR; pre-empted only inside full Check()
		return nil, err
	}
	adrs := corpus.All()
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
	for _, a := range adrs {
		for _, tag := range a.Tags {
			if _, ok := p.Cfg.Tags[tag]; !ok {
				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-tag", Detail: fmt.Sprintf("ADR-%s: unknown tag %q", a.Number, tag)})
			}
		}
	}
	pf, err := p.pitfallTagEntries()
	if err != nil { // reachable via a direct checkTagVocabulary call over a malformed pitfalls sidecar; pitfallEntries validates shape
		return nil, err
	}
	for _, e := range pf {
		for _, tag := range e.Tags {
			if _, ok := p.Cfg.Tags[tag]; !ok {
				drift = append(drift, manifest.Drift{Path: pitfallsSidecarPath, Kind: "pitfall-tag", Detail: fmt.Sprintf("%q: unknown tag %q", e.Title, tag)})
			}
		}
	}
	return drift, nil
}

// pitfallTagEntries returns the pitfall entries when the pitfalls doc is
// enabled, else nil - factored so checkTagVocabulary reads tags without
// duplicating checkPitfalls' sidecar plumbing.
func (p *Project) pitfallTagEntries() ([]pitfallEntry, error) {
	if !slices.Contains(p.Cfg.Docs, "pitfalls") {
		return nil, nil
	}
	sc, err := p.Cfg.Sidecar("docs", "pitfalls")
	if err != nil { // coverage-ignore: the pitfalls sidecar's YAML was validated at Open, so this re-read cannot fail
		return nil, err
	}
	return pitfallEntries(sc.Data["pitfalls"])
}

// checkADRRelatedLinks fails an ADR whose related: names an ADR number with no
// matching file under the decisions dir - structurally identical to the
// pitfall/plan link checks. Unconditional (independent of the tag vocabulary).
// invariant: adr-related-link-resolved
func (p *Project) checkADRRelatedLinks() ([]manifest.Drift, error) {
	corpus, err := p.Corpus()
	if err != nil { // reachable via a direct checkADRRelatedLinks call over a malformed ADR; pre-empted only inside full Check()
		return nil, err
	}
	adrs := corpus.All()
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
	var drift []manifest.Drift
	for _, a := range adrs {
		for _, n := range a.Related {
			if !corpus.Has(fmt.Sprintf("%04d", n)) {
				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-related-link", Detail: fmt.Sprintf("ADR-%s: ADR-%04d", a.Number, n)})
			}
		}
		// Ordering is scanned separately from resolution so that stopping at
		// the first descent cannot also stop the dangling-link scan
		// (adr-related-ascending). `related:` ascends, so a back-pointer edge
		// has exactly one correct position and appending a low-numbered
		// carrier is visibly wrong. One finding per array: the whole array is
		// one authoring act to fix.
		for i := 1; i < len(a.Related); i++ {
			if a.Related[i] < a.Related[i-1] {
				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-related-order", Detail: fmt.Sprintf("ADR-%s: related: descends at %d after %d; the array is ascending", a.Number, a.Related[i], a.Related[i-1])})
				break
			}
		}
	}
	return drift, nil
}
