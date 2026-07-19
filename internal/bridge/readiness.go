package bridge

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type Finding struct {
	Code, Path, Detail string
}
type Adjudication struct {
	Key, Disposition, Destination, Origin, Backing string
	Approved                                       bool
}
type Report struct {
	Ready                  bool
	Findings               []Finding
	InvariantAdjudications []Adjudication
	PlannedMutations       []Mutation
}

var canonicalKeyRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func Check(root string) Report {
	r := Report{}
	add := func(code, path string, err error) {
		r.Findings = append(r.Findings, Finding{Code: code, Path: filepath.ToSlash(path), Detail: err.Error()})
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		add("config-conversion", ".awf/config.yaml", err)
		return finish(r)
	}
	if err := cfg.Validate(); err != nil {
		add("config-conversion", ".awf/config.yaml", err)
	}
	_, convertErr := config.ConvertInvariantsToCurrentState(cfg.Source())
	if convertErr != nil {
		add("config-conversion", ".awf/config.yaml", convertErr)
	}
	if cfg.CurrentState != nil && cfg.CurrentState.TopicCoverage != "error" {
		add("coverage-severity", ".awf/config.yaml", fmt.Errorf("currentState.topicCoverage must be error; got %q", cfg.CurrentState.TopicCoverage))
	}
	for _, d := range cfg.Domains {
		if !canonicalKeyRE.MatchString(d) {
			add("domain-key", ".awf/domains/"+d+".yaml", fmt.Errorf("domain key %q is not canonical kebab-case", d))
		}
	}
	corpus, corpusErr := adr.LoadCorpus(filepath.Join(root, cfg.DocsDir, "decisions"))
	if corpusErr != nil {
		add("invariant-inventory", cfg.DocsDir+"/decisions", corpusErr)
		return finish(r)
	}
	for _, a := range corpus.All() {
		if a.IsInflight() {
			add("inflight-adr", rel(root, a.Path), fmt.Errorf("ADR-%s remains %s", a.Number, a.Status))
		}
	}
	inventory, inventoryErr := BuildInventory(corpus)
	if inventoryErr != nil {
		code := "invariant-inventory"
		if strings.Contains(inventoryErr.Error(), "Migration history") || strings.Contains(inventoryErr.Error(), "migration history") {
			code = "migration-history"
		}
		add(code, cfg.DocsDir+"/decisions", inventoryErr)
	}
	approvals, approvalErr := LoadApprovals(root)
	if approvalErr != nil {
		add("invariant-approval", ApprovalPath, approvalErr)
	}
	rawTopics, rawTopicErr := loadRawTopics(root)
	if rawTopicErr != nil {
		add("topic-corpus", topicErrorPath(rawTopicErr), rawTopicErr)
	}
	var mappings []Mapping
	if inventoryErr == nil && rawTopicErr == nil {
		mappings, err = DeriveMappings(inventory, rawTopics)
		if err != nil {
			add("claim-mapping", mappingErrorPath(root, inventory, err), err)
		}
	}
	if approvalErr == nil && inventoryErr == nil {
		approved, err := ApplyApprovals(inventory, mappings, approvals)
		if err != nil {
			add("invariant-approval", ApprovalPath, err)
		} else {
			mappings = approved
		}
	}
	buildAdjudications(&r, inventory, mappings)
	if inventoryErr != nil || convertErr != nil || rawTopicErr != nil {
		return finish(r)
	}
	mutations, normalizeErr := PlanNormalization(root, cfg, corpus, inventory, mappings)
	if normalizeErr != nil {
		add(classifyNormalize(normalizeErr), normalizePath(normalizeErr), normalizeErr)
		return finish(r)
	}
	r.PlannedMutations = mutations
	if err := ValidateLegacySnapshot(root, corpus, inventory, mappings, mutations); err != nil {
		add("invariant-inventory", cfg.DocsDir+"/decisions", err)
	}
	tmp, err := copyPreparedTree(root)
	if err != nil { // coverage-ignore: copy errors require the filesystem races and temporary-directory faults excluded in copyPreparedTree
		add("output-plan", ".", err)
		return finish(r)
	}
	defer os.RemoveAll(tmp)
	for _, mutation := range mutations {
		if err := applyMutation(tmp, mutation); err != nil { // coverage-ignore: operations target the writable temporary tree with prevalidated paths
			add("output-plan", mutation.Path, err)
			return finish(r)
		}
	}
	prepared, err := project.Open(tmp)
	if err != nil {
		add("topic-corpus", ".awf/config.yaml", err)
		return finish(r)
	}
	preparedTopics, err := prepared.Topics()
	if err != nil {
		code := "topic-corpus"
		if strings.Contains(err.Error(), "marker") || strings.Contains(err.Error(), "proof") {
			code = "marker-mapping"
		}
		add(code, topicErrorPath(err), err)
	} else {
		checkCoverage(&r, root, tmp, prepared.Cfg, preparedTopics)
	}
	terminal, err := prepared.BridgeProjection(true)
	if err != nil { // coverage-ignore: prepared.Topics and the same OutputPlan inputs succeeded immediately above
		add("output-plan", ".", err)
		return finish(r)
	}
	legacyExpected := map[string]bool{strings.TrimRight(prepared.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md": true}
	for _, domain := range prepared.Cfg.Domains {
		legacyExpected[strings.TrimRight(prepared.Cfg.DocsDir, "/")+"/domains/"+domain+".md"] = true
	}
	for _, output := range terminal {
		if output.Deletion {
			if !legacyExpected[output.Path] { // coverage-ignore: BridgeProjection builds deletions from this same config-derived set
				add("legacy-output", output.Path, errors.New("unexpected terminal deletion"))
			}
			delete(legacyExpected, output.Path)
			if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(output.Path))); statErr == nil {
				m, mErr := newMutation(root, output.Path, nil, false, 0)
				if mErr == nil {
					r.PlannedMutations = append(r.PlannedMutations, m)
				}
			}
			continue
		}
		if output.Reservation {
			continue
		}
		outputPath := output.Path
		before, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(outputPath)))
		mode := fileMode(root, outputPath)
		if os.IsNotExist(readErr) || readErr == nil && (!bytes.Equal(before, output.Bytes) || mode != output.Mode) {
			m, mErr := newMutation(root, output.Path, output.Bytes, true, output.Mode)
			if mErr != nil { // coverage-ignore: projection just read or planned this same root-relative path; failure requires a concurrent filesystem race
				add("output-plan", output.Path, mErr)
			} else {
				r.PlannedMutations = append(r.PlannedMutations, m)
			}
		} else if readErr != nil { // coverage-ignore: only a permission or IO fault on a planned output reaches this branch
			add("output-plan", output.Path, readErr)
		}
	}
	for path := range legacyExpected { // coverage-ignore: BridgeProjection constructs deletions from the same config-derived legacy set
		add("legacy-output", path, errors.New("terminal projection does not reserve legacy deletion"))
	}
	r.PlannedMutations = mergeMutations(nil, r.PlannedMutations)
	slices.SortFunc(r.PlannedMutations, func(a, b Mutation) int { return strings.Compare(a.Path, b.Path) })
	return finish(r)
}

func finish(r Report) Report {
	slices.SortFunc(r.Findings, func(a, b Finding) int {
		if a.Code != b.Code {
			return strings.Compare(a.Code, b.Code)
		}
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Detail, b.Detail)
	})
	r.Ready = len(r.Findings) == 0
	return r
}

func buildAdjudications(r *Report, inventory Inventory, mappings []Mapping) {
	byKey := map[string]Mapping{}
	for _, m := range mappings {
		byKey[m.Key] = m
	}
	for _, entry := range inventory.Entries {
		a := Adjudication{Key: string(entry.Key), Origin: "ADR-" + entry.Declarer, Backing: entry.Backing}
		if entry.Active {
			a.Disposition = "live"
			m := byKey[a.Key]
			a.Destination, a.Approved = m.Destination, m.Approved
		} else {
			a.Disposition = "retired"
			// Approved when an effective token (Carrier set, encoded now or by planned
			// normalization) or an authored migration/encoded ledger entry establishes it.
			a.Approved = entry.Carrier != "" || entry.History != nil
		}
		r.InvariantAdjudications = append(r.InvariantAdjudications, a)
	}
}

func loadRawTopics(root string) ([]topic.Topic, error) {
	metadataRoot := filepath.Join(root, ".awf", "topics", "metadata")
	partsRoot := filepath.Join(root, ".awf", "topics", "parts")
	var out []topic.Topic
	err := filepath.WalkDir(partsRoot, func(path string, de fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil { // coverage-ignore: after the optional missing root, WalkDir errors require a permission or concurrent filesystem fault
			return err
		}
		if de.IsDir() || de.Name() != "current-state.md" {
			return nil
		}
		relPath, _ := filepath.Rel(partsRoot, path)
		seg := strings.Split(filepath.ToSlash(relPath), "/")
		if len(seg) != 3 {
			return fmt.Errorf("invalid topic part path %s", path)
		}
		id := topic.TopicID{Domain: seg[0], Slug: seg[1]}
		metadataPath := filepath.Join(metadataRoot, id.Domain, id.Slug+".yaml")
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return err
		}
		_, metadata, err := topic.ParseMetadata(metadataRoot, metadataPath, metadataBytes)
		if err != nil {
			return err
		}
		partBytes, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: WalkDir just returned this part path; failure requires a concurrent filesystem race
			return err
		}
		parsed, err := topic.ParsePart(id, path, partBytes)
		if err != nil {
			return err
		}
		parsed.Metadata, parsed.MetadataPath = metadata, metadataPath
		out = append(out, parsed)
		return nil
	})
	slices.SortFunc(out, func(a, b topic.Topic) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	return out, err
}

func copyPreparedTree(root string) (string, error) {
	tmp, err := os.MkdirTemp("", "awf-bridge-prepared-")
	if err != nil { // coverage-ignore: the test-controlled system temporary directory is writable
		return "", err
	}
	err = filepath.WalkDir(root, func(path string, de fs.DirEntry, walkErr error) error {
		if walkErr != nil { // coverage-ignore: source walk errors require a concurrent removal or permission fault
			return walkErr
		}
		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}
		if de.IsDir() {
			if de.Name() == ".git" {
				return filepath.SkipDir
			}
			if path != root {
				if _, e := os.Stat(filepath.Join(path, ".git")); e == nil {
					return filepath.SkipDir
				}
				if _, e := os.Stat(filepath.Join(path, config.DirName)); e == nil {
					return filepath.SkipDir
				}
			}
			return os.MkdirAll(filepath.Join(tmp, relPath), 0o755)
		}
		info, e := de.Info()
		if e != nil { // coverage-ignore: WalkDir just returned this entry; failure requires a concurrent filesystem race
			return e
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil { // coverage-ignore: WalkDir and Info just returned this regular file; failure requires a concurrent filesystem race
			return e
		}
		dst := filepath.Join(tmp, relPath)
		if e = os.MkdirAll(filepath.Dir(dst), 0o755); e != nil { // coverage-ignore: the temporary root was created writable above
			return e
		}
		return os.WriteFile(dst, b, info.Mode().Perm())
	})
	if err != nil { // coverage-ignore: only the filesystem race and temporary-directory faults excluded above reach this aggregation
		os.RemoveAll(tmp)
		return "", err
	}
	return tmp, nil
}
func applyMutation(root string, m Mutation) error {
	path := filepath.Join(root, filepath.FromSlash(m.Path))
	if !m.AfterPresent {
		return os.Remove(path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // coverage-ignore: mutations apply only inside the writable temporary tree
		return err
	}
	if err := os.WriteFile(path, m.After, os.FileMode(m.AfterMode)); err != nil { // coverage-ignore: parent creation in the writable temporary tree succeeded
		return err
	}
	return os.Chmod(path, os.FileMode(m.AfterMode))
}

func checkCoverage(r *Report, gitRoot, temp string, cfg *config.Config, corpus topic.Corpus) {
	paths, err := awfgit.WorkingPaths(gitRoot)
	if err != nil {
		r.Findings = append(r.Findings, Finding{"topic-coverage", ".", err.Error()})
		return
	}
	generated := map[string]bool{}
	if p, e := project.Open(temp); e == nil {
		if outputs, e := p.BridgeProjection(false); e == nil {
			for _, o := range outputs {
				generated[o.Path] = true
			}
		}
	}
	for _, path := range paths {
		if generated[path] || excludedPath(path, cfg.ContextIgnore) || !regularExists(temp, path) {
			continue
		}
		for _, domain := range cfg.Domains {
			sidecar, e := cfg.Sidecar("domains", domain)
			if e != nil { // coverage-ignore: project.Open loaded and validated this same sidecar before checkCoverage
				continue
			}
			if !anyGlob(sidecar.Paths, path) {
				continue
			}
			covered := false
			for _, t := range corpus.ForDomain(domain) {
				if t.Metadata.Applies != "global" && len(t.Claims) > 0 && anyGlob(t.Metadata.Paths, path) {
					covered = true
					break
				}
			}
			if !covered {
				r.Findings = append(r.Findings, Finding{"topic-coverage", path, "domain " + domain + " has no nonempty scoped topic"})
			}
		}
	}
}
func anyGlob(globs []string, path string) bool {
	for _, g := range globs {
		if pathglob.Match(g, path) {
			return true
		}
	}
	return false
}
func excludedPath(path string, globs []string) bool {
	if strings.HasPrefix(path, ".git/") {
		return true
	}
	return anyGlob(globs, path)
}
func regularExists(root, path string) bool {
	i, e := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	return e == nil && i.Mode().IsRegular()
}
func rel(root, path string) string { r, _ := filepath.Rel(root, path); return filepath.ToSlash(r) }
func topicErrorPath(err error) string {
	msg := err.Error()
	for _, token := range strings.Fields(msg) {
		token = strings.Trim(token, "\":")
		if strings.Contains(token, ".awf/topics/") {
			if i := strings.Index(token, ".awf/topics/"); i >= 0 {
				return filepath.ToSlash(token[i:])
			}
		}
	}
	return ".awf/topics"
}
func mappingErrorPath(root string, inventory Inventory, err error) string {
	for _, e := range inventory.Entries {
		if strings.Contains(err.Error(), string(e.Key)) {
			return rel(root, e.DeclarerPath)
		}
	}
	return "docs/decisions"
}
func classifyNormalize(err error) string {
	if strings.Contains(err.Error(), "marker") || strings.Contains(err.Error(), "touches") {
		return "marker-mapping"
	}
	if strings.Contains(err.Error(), "config") {
		return "config-conversion"
	}
	return "migration-history"
}
func normalizePath(err error) string {
	if strings.Contains(err.Error(), "config") {
		return ".awf/config.yaml"
	}
	if classifyNormalize(err) == "marker-mapping" {
		if path, _, ok := strings.Cut(err.Error(), ":"); ok && path != "" {
			return filepath.ToSlash(path)
		}
	}
	return "docs/decisions"
}
