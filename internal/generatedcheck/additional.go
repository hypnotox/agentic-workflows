package generatedcheck

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

const awfDir = ".awf"

// Artifact identifies one configured artifact and its declared sections.
type Artifact struct {
	Kind, Name string
	Sections   []string
}

// SidecarData is the prepared data projection for one semantic artifact identity.
type SidecarData struct {
	Kind, Name, Path string
	Data             map[string]any
}

// TreeEntry is one path in the selected operation tree.
type TreeEntry struct {
	Path      string
	Directory bool
}

// AdditionalInput supplies closed, immutable semantic projections prepared by Publisher.
type AdditionalInput struct {
	Vars         map[string]any
	Domains      []string
	Artifacts    []Artifact
	Singletons   []Artifact
	Sidecars     []SidecarData
	PitfallPaths []string
	Topics       []Topic
	Entries      []TreeEntry
	ResidentRoot bool
}

// Topic is the path identity needed to claim generated topic sources.
type Topic struct{ Domain, Slug string }

func Additional(input AdditionalInput, plan outputplan.Plan) (checkresult.Result, error) {
	files := plan.Outputs()
	sweep := sweep(input, files)
	findings := make([]checkresult.Finding, 0, len(sweep))
	for _, d := range sweep {
		findings = append(findings, errorFinding(d.Kind, d.Path, d.Detail))
	}
	infos := make([]checkresult.Information, 0)
	for _, d := range unusedVars(input.Vars, files) {
		infos = append(infos, information(d.Kind, d.Path, d.Detail))
	}
	for _, d := range unusedData(input.Sidecars, files) {
		infos = append(infos, information(d.Kind, d.Path, d.Detail))
	}
	return result(findings, infos)
}

func unusedVars(vars map[string]any, files []outputplan.Output) []manifest.Drift {
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
	for _, key := range slices.Sorted(maps.Keys(vars)) {
		if value := vars[key]; value == nil || value == "" || used[key] {
			continue
		}
		drift = append(drift, manifest.Drift{Path: awfDir + "/config.yaml", Kind: "unused-var", Detail: fmt.Sprintf("var %q is set but referenced by no rendered artifact; delete it from vars: or enable an artifact that consumes it", key)})
	}
	return drift
}

func unusedData(sidecars []SidecarData, files []outputplan.Output) []manifest.Drift {
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
	for _, sidecar := range sidecars {
		if len(sidecar.Data) == 0 {
			continue
		}
		rs := refs[sidecar.Kind+"\x00"+sidecar.Name]
		if rs != nil && rs.bare {
			continue
		}
		var unused []string
		for _, key := range slices.Sorted(maps.Keys(sidecar.Data)) {
			if rs == nil || !rs.keys[key] {
				unused = append(unused, key)
			}
		}
		if len(unused) != 0 {
			drift = append(drift, manifest.Drift{Path: sidecar.Path, Kind: "unused-data", Detail: "data keys referenced by no rendered section: " + strings.Join(unused, ", ") + "; a key referenced only inside a dropped section counts as unused; remove the key or the drop"})
		}
	}
	return drift
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

func sweep(input AdditionalInput, files []outputplan.Output) []manifest.Drift {
	m := claimed{files: map[string]bool{awfDir + "/config.yaml": true, awfDir + "/awf.lock": true, awfDir + "/current-state-upgrade.journal": true}, dirs: map[string]bool{awfDir: true, awfDir + "/parts": true, awfDir + "/memory": true}, artifacts: map[string]map[string]bool{}, singletons: map[string]bool{}}
	for _, name := range resident.RootNames() {
		m.dirs[awfDir+"/"+name] = true
	}
	for _, file := range files {
		if strings.HasPrefix(file.Path(), awfDir+"/") {
			m.files[file.Path()] = true
		}
	}
	for _, kind := range []string{"skills", "docs", "domains"} {
		m.dirs[awfDir+"/"+kind], m.dirs[awfDir+"/"+kind+"/parts"] = true, true
		m.artifacts[kind] = map[string]bool{}
	}
	for _, artifact := range input.Artifacts {
		m.artifacts[artifact.Kind][artifact.Name] = true
		m.files[awfDir+"/"+artifact.Kind+"/"+artifact.Name+".yaml"] = true
		m.dirs[awfDir+"/"+artifact.Kind+"/parts/"+artifact.Name] = true
		for _, section := range artifact.Sections {
			m.files[awfDir+"/"+artifact.Kind+"/parts/"+artifact.Name+"/"+section+".md"] = true
		}
	}
	for _, domain := range input.Domains {
		m.artifacts["domains"][domain] = true
		m.files[awfDir+"/domains/"+domain+".yaml"] = true
		m.dirs[awfDir+"/domains/parts/"+domain] = true
	}
	for _, singleton := range input.Singletons {
		m.files[awfDir+"/"+singleton.Kind+".yaml"] = true
		m.singletons[singleton.Kind] = true
		m.dirs[awfDir+"/parts/"+singleton.Kind] = true
		for _, section := range singleton.Sections {
			m.files[awfDir+"/parts/"+singleton.Kind+"/"+section+".md"] = true
		}
	}
	m.dirs[awfDir+"/docs/pitfalls"] = true
	for _, path := range input.PitfallPaths {
		m.files[path] = true
	}
	m.dirs[awfDir+"/topics"], m.dirs[awfDir+"/topics/metadata"], m.dirs[awfDir+"/topics/parts"] = true, true, true
	for _, domain := range input.Domains {
		m.dirs[awfDir+"/topics/metadata/"+domain], m.dirs[awfDir+"/topics/parts/"+domain] = true, true
	}
	for _, item := range input.Topics {
		m.dirs[awfDir+"/topics/metadata/"+item.Domain], m.dirs[awfDir+"/topics/parts/"+item.Domain+"/"+item.Slug] = true, true
		m.files[awfDir+"/topics/metadata/"+item.Domain+"/"+item.Slug+".yaml"], m.files[awfDir+"/topics/parts/"+item.Domain+"/"+item.Slug+"/current-state.md"] = true, true
	}
	m.dirs[awfDir+"/runner/parts"] = true
	m.files[awfDir+"/runner/parts/runner-body.md"] = true
	var drift []manifest.Drift
	skipped := map[string]bool{}
	for _, entry := range input.Entries {
		rel := entry.Path
		ignored := false
		for prefix := range skipped {
			if strings.HasPrefix(rel, prefix+"/") {
				ignored = true
				break
			}
		}
		if ignored {
			continue
		}
		if rel == awfDir || (resident.IsResidentPath(rel) && input.ResidentRoot) {
			continue
		}
		if entry.Directory {
			if m.directory(rel) {
				continue
			}
			drift = append(drift, m.classify(rel, true))
			skipped[rel] = true
			continue
		}
		if !m.files[rel] {
			drift = append(drift, m.classify(rel, false))
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return drift
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
