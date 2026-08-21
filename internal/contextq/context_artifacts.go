package contextq

import (
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type artifactLink struct {
	Path, Label string
}
type artifactSnapshot struct {
	InManifest, Drifted bool
}
type artifactRecord struct {
	Role                         outputplan.ArtifactRole
	Identity                     string
	Sources, Outputs, Navigation []artifactLink
	Snapshot                     *artifactSnapshot
}

type artifactAuthorities struct {
	Layout project.Layout
	ADRs   adr.Corpus
}

func artifactRecords(path string, declarations []outputplan.Declaration, authorities artifactAuthorities) []artifactRecord {
	records := []artifactRecord{}
	add := func(role outputplan.ArtifactRole, identity string, sources, outputs []artifactLink) {
		for i := range records {
			if records[i].Role == role && records[i].Identity == identity {
				records[i].Sources = mergeArtifactLinks(records[i].Sources, sources)
				records[i].Outputs = mergeArtifactLinks(records[i].Outputs, outputs)
				return
			}
		}
		records = append(records, artifactRecord{Role: role, Identity: identity, Sources: nonNilLinks(sources), Outputs: nonNilLinks(outputs), Navigation: []artifactLink{}})
	}
	configReference := authorities.Layout.DocsDir + "/config-reference.md"
	linkIfDeclared := func(path, label string) []artifactLink {
		for _, declaration := range declarations {
			if declaration.Path() == path {
				return []artifactLink{{Path: path, Label: label}}
			}
		}
		return []artifactLink{}
	}
	switch {
	case path == ".awf/config.yaml":
		add(outputplan.ArtifactConfig, "project-config", nil, declarationOutputs(path, declarations))
	case path == ".awf/awf.lock":
		add(outputplan.ArtifactLock, "project-lock", nil, nil)
		add(outputplan.ArtifactManifest, "output-manifest", nil, nil)
	case strings.HasPrefix(path, ".awf/topics/metadata/") && strings.HasSuffix(path, ".yaml"):
		add(outputplan.ArtifactTopicMetadata, strings.TrimSuffix(strings.TrimPrefix(path, ".awf/topics/metadata/"), ".yaml"), nil, declarationOutputs(path, declarations))
	case strings.HasPrefix(path, ".awf/topics/parts/") && strings.HasSuffix(path, "/current-state.md"):
		add(outputplan.ArtifactClaimPart, strings.TrimSuffix(strings.TrimPrefix(path, ".awf/topics/parts/"), "/current-state.md"), nil, declarationOutputs(path, declarations))
	case strings.HasPrefix(path, strings.TrimRight(authorities.Layout.ADRDir, "/")+"/"):
		base := strings.TrimPrefix(path, strings.TrimRight(authorities.Layout.ADRDir, "/")+"/")
		if record, ok := authorities.ADRs.ByIdentity(adr.FileIdentity(base)); ok && record.Filename == base {
			add(outputplan.ArtifactDecisionRecord, record.Identity(), nil, declarationOutputs(path, declarations))
		}
	}
	for _, d := range declarations {
		if d.Path() == path {
			sources := make([]artifactLink, 0, len(d.Inputs()))
			outputs := []artifactLink{}
			for _, in := range d.Inputs() {
				sources = append(sources, artifactLink{Path: in.Path(), Label: artifactSourceLabel(in.Role())})
				if in.Path() == path && in.Role() == outputplan.ArtifactManagedOutput {
					outputs = append(outputs, artifactLink{Path: d.Path(), Label: "managed output"})
				}
			}
			identity := d.TemplateID()
			if identity == "" {
				identity = strings.Join(d.Declarers(), ",")
			}
			add(outputplan.ArtifactManagedOutput, identity, sources, outputs)
		}
		for _, in := range d.Inputs() {
			if in.Path() == path && !canonicalArtifactInputRole(in.Role()) && in.Role() != outputplan.ArtifactManagedOutput {
				identity := path
				if in.Role() == outputplan.ArtifactTemplate {
					identity = strings.TrimPrefix(path, "templates/")
				}
				add(in.Role(), identity, nil, []artifactLink{{Path: d.Path(), Label: "managed output"}})
			}
		}
	}
	for i := range records {
		switch records[i].Role {
		case outputplan.ArtifactConfig:
			records[i].Navigation = linkIfDeclared(configReference, "configuration reference")
		case outputplan.ArtifactLock:
			records[i].Navigation = append([]artifactLink{{Path: ".awf/config.yaml", Label: "project config"}}, linkIfDeclared(configReference, "configuration reference")...)
		case outputplan.ArtifactManifest:
			records[i].Navigation = linkIfDeclared(configReference, "configuration reference")
		case outputplan.ArtifactTemplate, outputplan.ArtifactConventionPart, outputplan.ArtifactAuthoredData, outputplan.ArtifactProtocolDescriptor:
			records[i].Navigation = cloneArtifactLinks(records[i].Outputs)
		case outputplan.ArtifactTopicMetadata, outputplan.ArtifactClaimPart:
			id := records[i].Identity
			domain := strings.SplitN(id, "/", 2)[0]
			records[i].Navigation = append(linkIfDeclared(authorities.Layout.DocsDir+"/topics/"+id+".md", "topic document"), linkIfDeclared(authorities.Layout.DomainsDir+"/"+domain+".md", "domain document")...)
		case outputplan.ArtifactDecisionRecord:
			records[i].Navigation = linkIfDeclared(authorities.Layout.IndexMd, "decision index")
		case outputplan.ArtifactManagedOutput:
			for _, source := range records[i].Sources {
				if source.Path != path && source.Label != "render template" {
					records[i].Navigation = append(records[i].Navigation, source)
				}
			}
		}
		records[i].Sources = mergeArtifactLinks(nil, records[i].Sources)
		records[i].Outputs = mergeArtifactLinks(nil, records[i].Outputs)
		records[i].Navigation = mergeArtifactLinks(nil, records[i].Navigation)
	}
	roleOrder := map[outputplan.ArtifactRole]int{outputplan.ArtifactConfig: 0, outputplan.ArtifactLock: 1, outputplan.ArtifactManifest: 2, outputplan.ArtifactTemplate: 3, outputplan.ArtifactProtocolDescriptor: 4, outputplan.ArtifactConventionPart: 5, outputplan.ArtifactAuthoredData: 6, outputplan.ArtifactTopicMetadata: 7, outputplan.ArtifactClaimPart: 8, outputplan.ArtifactDecisionRecord: 9, outputplan.ArtifactManagedOutput: 10}
	slices.SortFunc(records, func(a, b artifactRecord) int {
		if a.Role != b.Role {
			return roleOrder[a.Role] - roleOrder[b.Role]
		}
		return strings.Compare(a.Identity, b.Identity)
	})
	return records
}

func artifactSourceLabel(role outputplan.ArtifactRole) string {
	switch role {
	case outputplan.ArtifactConfig:
		return "project config"
	case outputplan.ArtifactTemplate:
		return "render template"
	case outputplan.ArtifactProtocolDescriptor:
		return "protocol descriptor"
	case outputplan.ArtifactConventionPart:
		return "convention part"
	case outputplan.ArtifactAuthoredData:
		return "authored data"
	case outputplan.ArtifactTopicMetadata:
		return "topic metadata"
	case outputplan.ArtifactClaimPart:
		return "claim part"
	case outputplan.ArtifactDecisionRecord:
		return "decision record"
	case outputplan.ArtifactManagedOutput:
		return "in-place managed output"
	default:
		return string(role)
	}
}

func canonicalArtifactInputRole(role outputplan.ArtifactRole) bool {
	return role == outputplan.ArtifactConfig || role == outputplan.ArtifactTopicMetadata || role == outputplan.ArtifactClaimPart || role == outputplan.ArtifactDecisionRecord
}

func declarationOutputs(path string, declarations []outputplan.Declaration) []artifactLink {
	out := []artifactLink{}
	for _, d := range declarations {
		for _, in := range d.Inputs() {
			if in.Path() == path {
				out = append(out, artifactLink{Path: d.Path(), Label: "managed output"})
			}
		}
	}
	return mergeArtifactLinks(nil, out)
}
func applyArtifactSnapshots(records []artifactRecord, path string, tree *snapshot.Tree, lock *manifest.Lock) {
	for i := range records {
		if records[i].Role != outputplan.ArtifactManagedOutput {
			continue
		}
		entry, inManifest := manifest.Entry{}, false
		if lock != nil {
			entry, inManifest = lock.Files[path]
		}
		drifted := false
		if inManifest {
			if f, ok := tree.Lookup(path); !ok || !f.Scannable() || manifest.Hash(f.Bytes) != entry.OutputHash {
				drifted = true
			}
		}
		records[i].Snapshot = &artifactSnapshot{InManifest: inManifest, Drifted: drifted}
	}
}

func cloneArtifactLinks(in []artifactLink) []artifactLink { return append([]artifactLink{}, in...) }
func mergeArtifactLinks(a, b []artifactLink) []artifactLink {
	out := append(cloneArtifactLinks(a), b...)
	slices.SortFunc(out, func(a, b artifactLink) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Label, b.Label)
	})
	return slices.Compact(out)
}
func nonNilLinks(in []artifactLink) []artifactLink {
	if in == nil {
		return []artifactLink{}
	}
	return in
}
