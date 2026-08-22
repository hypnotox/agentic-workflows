package generatedcheck

import (
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// AdditionalInput supplies the immutable operation facts needed by generated
// output conformance checks that are not represented in an output plan.
type AdditionalInput struct {
	Root, ResidentRoot string
	Config             *config.Config
	Catalog            *catalog.Catalog
	Topics             []topic.Topic
	Paths              func(string) ([]string, error)
}

// Additional checks the closed configuration tree and generated-output
// vocabulary. Its input is the operation's loaded facts and prepared plan.
func Additional(input AdditionalInput, plan outputplan.Plan) (checkresult.Result, error) {
	files := plan.Outputs()
	sweep, err := sweep(input, files)
	if err != nil { // coverage-ignore: prepared operation inputs have already opened the config tree
		return checkresult.Result{}, err
	}
	findings := make([]checkresult.Finding, 0, len(sweep))
	for _, d := range sweep {
		findings = append(findings, errorFinding(d.Kind, d.Path, d.Detail))
	}
	infos := make([]checkresult.Information, 0)
	for _, d := range unusedVars(input.Config, files) {
		infos = append(infos, information(d.Kind, d.Path, d.Detail))
	}
	data, err := unusedData(input.Config, input.Catalog, files)
	if err != nil { // coverage-ignore: Loader validated every enabled sidecar before check composition
		return checkresult.Result{}, err
	}
	for _, d := range data {
		infos = append(infos, information(d.Kind, d.Path, d.Detail))
	}
	return result(findings, infos)
}

func unusedVars(cfg *config.Config, files []outputplan.Output) []manifest.Drift {
	used := map[string]bool{}
	for _, file := range files {
		if render.ReferencesBareVars(file.Assembled()) {
			return nil
		}
		for _, r := range render.ReferencedVars(file.Assembled()) {
			used[r] = true
		}
		for _, r := range file.PartVarRefs() {
			used[r] = true
		}
	}
	var drift []manifest.Drift
	for _, key := range slices.Sorted(maps.Keys(cfg.Vars)) {
		if value := cfg.Vars[key]; value == nil || value == "" || used[key] {
			continue
		}
		drift = append(drift, manifest.Drift{Path: config.DirName + "/config.yaml", Kind: "unused-var", Detail: fmt.Sprintf("var %q is set but referenced by no rendered artifact; delete it from vars: or enable an artifact that consumes it", key)})
	}
	return drift
}

func unusedData(cfg *config.Config, cat *catalog.Catalog, files []outputplan.Output) ([]manifest.Drift, error) {
	type refset struct {
		keys map[string]bool
		bare bool
	}
	refs := map[string]*refset{}
	for _, file := range files {
		key := file.Kind() + "\x00" + file.Artifact()
		rs := refs[key]
		if rs == nil {
			rs = &refset{keys: map[string]bool{}}
			refs[key] = rs
		}
		for _, key := range render.ReferencedDataKeys(file.Assembled()) {
			rs.keys[key] = true
		}
		rs.bare = rs.bare || render.ReferencesBareData(file.Assembled())
	}
	var drift []manifest.Drift
	check := func(kind, name, path string) error {
		sidecar, err := cfg.Sidecar(kind, name)
		if err != nil { // coverage-ignore: sidecars were parsed during output-plan preparation
			return err
		}
		if len(sidecar.Data) == 0 {
			return nil
		}
		rs := refs[kind+"\x00"+name]
		if rs != nil && rs.bare {
			return nil
		}
		var unused []string
		for _, key := range slices.Sorted(maps.Keys(sidecar.Data)) {
			if rs == nil || !rs.keys[key] {
				unused = append(unused, key)
			}
		}
		if len(unused) != 0 {
			drift = append(drift, manifest.Drift{Path: path, Kind: "unused-data", Detail: "data keys referenced by no rendered section: " + strings.Join(unused, ", ") + "; a key referenced only inside a dropped section counts as unused; remove the key or the drop"})
		}
		return nil
	}
	for _, kind := range []string{"skills", "agents", "docs"} {
		for _, name := range names(cat, kind) {
			if err := check(kind, name, config.DirName+"/"+kind+"/"+name+".yaml"); err != nil { // coverage-ignore: see checked prepared-sidecar error above
				return nil, err
			}
		}
	}
	for _, kind := range catalog.SingletonKindsFor(cat) {
		if err := check(kind, "", config.DirName+"/"+kind+".yaml"); err != nil { // coverage-ignore: see checked prepared-sidecar error above
			return nil, err
		}
	}
	return drift, nil
}

type claimed struct {
	files, dirs map[string]bool
	artifacts   map[string]map[string]bool
	singletons  map[string]bool
}

func (m claimed) directory(path string) bool {
	if m.dirs[path] {
		return true
	}
	prefix := path + "/"
	for file := range m.files {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

func sweep(input AdditionalInput, files []outputplan.Output) ([]manifest.Drift, error) {
	m := claimed{files: map[string]bool{config.DirName + "/config.yaml": true, config.DirName + "/awf.lock": true, config.DirName + "/current-state-upgrade.journal": true}, dirs: map[string]bool{config.DirName: true, config.DirName + "/parts": true, config.DirName + "/memory": true}, artifacts: map[string]map[string]bool{}, singletons: map[string]bool{}}
	for _, name := range resident.RootNames() {
		m.dirs[config.DirName+"/"+name] = true
	}
	for _, file := range files {
		if strings.HasPrefix(file.Path(), config.DirName+"/") {
			m.files[file.Path()] = true
		}
	}
	for _, kind := range []string{"skills", "agents", "docs", "domains"} {
		m.dirs[config.DirName+"/"+kind], m.dirs[config.DirName+"/"+kind+"/parts"] = true, true
		m.artifacts[kind] = map[string]bool{}
		for _, name := range names(input.Catalog, kind) {
			m.artifacts[kind][name] = true
			m.files[config.DirName+"/"+kind+"/"+name+".yaml"] = true
			m.dirs[config.DirName+"/"+kind+"/parts/"+name] = true
			for _, section := range sections(input.Catalog, kind, name) {
				m.files[config.DirName+"/"+kind+"/parts/"+name+"/"+section+".md"] = true
			}
		}
		if kind == "domains" {
			for _, name := range input.Config.Domains {
				m.artifacts[kind][name] = true
				m.files[config.DirName+"/domains/"+name+".yaml"] = true
				m.dirs[config.DirName+"/domains/parts/"+name] = true
				for _, section := range sections(input.Catalog, kind, name) {
					m.files[config.DirName+"/domains/parts/"+name+"/"+section+".md"] = true
				}
			}
		}
	}
	for _, kind := range catalog.SingletonKindsFor(input.Catalog) {
		m.files[config.DirName+"/"+kind+".yaml"] = true
		m.singletons[kind] = true
		m.dirs[config.DirName+"/parts/"+kind] = true
		for _, section := range input.Catalog.Docs[kind].Sections {
			m.files[config.DirName+"/parts/"+kind+"/"+section+".md"] = true
		}
	}
	m.dirs[config.DirName+"/docs/pitfalls"] = true
	for _, path := range mustPaths(input.Paths, config.DirName+"/docs/pitfalls/") {
		m.files[path] = true
	}
	m.dirs[config.DirName+"/topics"], m.dirs[config.DirName+"/topics/metadata"], m.dirs[config.DirName+"/topics/parts"] = true, true, true
	for _, domain := range input.Config.Domains {
		m.dirs[config.DirName+"/topics/metadata/"+domain], m.dirs[config.DirName+"/topics/parts/"+domain] = true, true
	}
	for _, item := range input.Topics {
		domain, slug := item.ID.Domain, item.ID.Slug
		m.dirs[config.DirName+"/topics/metadata/"+domain], m.dirs[config.DirName+"/topics/parts/"+domain+"/"+slug] = true, true
		m.files[config.DirName+"/topics/metadata/"+domain+"/"+slug+".yaml"], m.files[config.DirName+"/topics/parts/"+domain+"/"+slug+"/current-state.md"] = true, true
	}
	m.dirs[config.DirName+"/runner/parts"] = true
	for _, section := range []string{"runner-body"} {
		m.files[config.DirName+"/runner/parts/"+section+".md"] = true
	}
	var drift []manifest.Drift
	err := filepath.WalkDir(filepath.Join(input.Root, config.DirName), func(path string, entry fs.DirEntry, err error) error {
		if err != nil { // coverage-ignore: WalkDir errors only on a filesystem permission fault after preparation
			return err
		}
		rel, err := filepath.Rel(input.Root, path)
		if err != nil { // coverage-ignore: WalkDir supplies paths rooted beneath input.Root
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == config.DirName {
			return nil
		}
		if entry.IsDir() {
			if resident.IsResidentPath(rel) && input.Root == input.ResidentRoot {
				return filepath.SkipDir
			}
			if m.directory(rel) {
				return nil
			}
			drift = append(drift, m.classify(rel, true))
			return filepath.SkipDir
		}
		if !m.files[rel] {
			drift = append(drift, m.classify(rel, false))
		}
		return nil
	})
	if err != nil { // coverage-ignore: callback errors only preserve the preceding impossible filesystem faults
		return nil, err
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return drift, nil
}

func mustPaths(pathsFunc func(string) ([]string, error), prefix string) []string {
	if pathsFunc == nil { // coverage-ignore: composition always supplies the prepared operation reader
		return nil
	}
	paths, err := pathsFunc(prefix)
	if err != nil { // coverage-ignore: pitfall corpus preparation already enumerated this same prefix
		return nil
	}
	return paths
}
func names(cat *catalog.Catalog, kind string) []string {
	if kind == "domains" {
		return nil
	}
	switch kind {
	case "skills":
		return slices.Sorted(maps.Keys(cat.Skills))
	case "agents":
		return slices.Sorted(maps.Keys(cat.Agents))
	case "docs":
		return slices.Sorted(maps.Keys(cat.Docs))
	}
	return nil // coverage-ignore: callers use the closed artifact-kind set
}
func sections(cat *catalog.Catalog, kind, name string) []string {
	switch kind {
	case "skills":
		return cat.Skills[name].Sections
	case "agents":
		return cat.Agents[name].Sections
	case "docs":
		return cat.Docs[name].Sections
	case "domains":
		return cat.DomainDoc.Sections
	}
	return nil // coverage-ignore: callers use the closed artifact-kind set
}

var backup = regexp.MustCompile(`\.awf-bak(\.\d+)?$`)

func (m claimed) classify(rel string, dir bool) manifest.Drift {
	d := manifest.Drift{Path: rel, Kind: "orphaned"}
	segs := strings.Split(rel, "/")
	switch {
	case !dir && backup.MatchString(rel):
		d.Detail = "stale awf-bak backup: review and delete"
	case len(segs) == 3 && segs[1] == "parts" && dir && !m.singletons[segs[2]]:
		d.Detail = "convention parts for an unknown singleton kind"
	case len(segs) == 4 && segs[1] == "parts" && !dir && strings.HasSuffix(segs[3], ".md"):
		d.Detail = "convention part for a section not in the singleton's declared set"
	case len(segs) == 3 && !dir && strings.HasSuffix(segs[2], ".yaml") && m.artifacts[segs[1]] != nil:
		d.Detail = "sidecar for an artifact not in the catalog"
	case len(segs) == 4 && segs[2] == "parts" && dir && m.artifacts[segs[1]] != nil && !m.artifacts[segs[1]][segs[3]]:
		d.Detail = "convention parts for an artifact not in the catalog"
	case len(segs) == 5 && segs[2] == "parts" && !dir && strings.HasSuffix(segs[4], ".md") && m.artifacts[segs[1]] != nil && m.artifacts[segs[1]][segs[3]]:
		d.Detail = "convention part for a section not in the target's declared set"
	default:
		d.Detail = "unclaimed file or directory: not part of the .awf config tree; delete it or move it out"
	}
	return d
}
