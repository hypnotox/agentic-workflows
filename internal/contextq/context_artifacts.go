package contextq

import (
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
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
	Role                         project.ArtifactRole
	Identity                     string
	Sources, Outputs, Navigation []artifactLink
	Snapshot                     *artifactSnapshot
}

type artifactAuthorities struct {
	Layout project.Layout
	ADRs   adr.Corpus
}

func artifactRecords(path string, declarations []project.OutputDeclaration, authorities artifactAuthorities) []artifactRecord {
	records := []artifactRecord{}
	add := func(role project.ArtifactRole, identity string, sources, outputs []artifactLink) {
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
			if declaration.Path == path && !declaration.Reservation {
				return []artifactLink{{Path: path, Label: label}}
			}
		}
		return []artifactLink{}
	}
	switch {
	case path == ".awf/config.yaml":
		add(project.ArtifactConfig, "project-config", nil, declarationOutputs(path, declarations))
	case path == ".awf/awf.lock":
		add(project.ArtifactLock, "project-lock", nil, nil)
		add(project.ArtifactManifest, "output-manifest", nil, nil)
	case strings.HasPrefix(path, ".awf/topics/metadata/") && strings.HasSuffix(path, ".yaml"):
		add(project.ArtifactTopicMetadata, strings.TrimSuffix(strings.TrimPrefix(path, ".awf/topics/metadata/"), ".yaml"), nil, declarationOutputs(path, declarations))
	case strings.HasPrefix(path, ".awf/topics/parts/") && strings.HasSuffix(path, "/current-state.md"):
		add(project.ArtifactClaimPart, strings.TrimSuffix(strings.TrimPrefix(path, ".awf/topics/parts/"), "/current-state.md"), nil, declarationOutputs(path, declarations))
	case strings.HasPrefix(path, strings.TrimRight(authorities.Layout.ADRDir, "/")+"/"):
		base := strings.TrimPrefix(path, strings.TrimRight(authorities.Layout.ADRDir, "/")+"/")
		if record, ok := authorities.ADRs.ByIdentity(adr.FileIdentity(base)); ok && record.Filename == base {
			add(project.ArtifactDecisionRecord, record.Identity(), nil, declarationOutputs(path, declarations))
		}
	}
	for _, d := range declarations {
		if d.Path == path && !d.Reservation {
			sources := make([]artifactLink, 0, len(d.Inputs))
			outputs := []artifactLink{}
			for _, in := range d.Inputs {
				sources = append(sources, artifactLink{Path: in.Path, Label: artifactSourceLabel(in.Role)})
				if in.Path == path && in.Role == project.ArtifactManagedOutput {
					outputs = append(outputs, artifactLink{Path: d.Path, Label: "managed output"})
				}
			}
			identity := d.TemplateID
			if identity == "" {
				identity = strings.Join(d.Declarers, ",")
			}
			add(project.ArtifactManagedOutput, identity, sources, outputs)
		}
		if !d.Reservation {
			for _, in := range d.Inputs {
				if in.Path == path && !canonicalArtifactInputRole(in.Role) && in.Role != project.ArtifactManagedOutput {
					identity := path
					if in.Role == project.ArtifactTemplate {
						identity = strings.TrimPrefix(path, "templates/")
					}
					add(in.Role, identity, nil, []artifactLink{{Path: d.Path, Label: "managed output"}})
				}
			}
		}
	}
	for i := range records {
		switch records[i].Role {
		case project.ArtifactConfig:
			records[i].Navigation = linkIfDeclared(configReference, "configuration reference")
		case project.ArtifactLock:
			records[i].Navigation = append([]artifactLink{{Path: ".awf/config.yaml", Label: "project config"}}, linkIfDeclared(configReference, "configuration reference")...)
		case project.ArtifactManifest:
			records[i].Navigation = linkIfDeclared(configReference, "configuration reference")
		case project.ArtifactTemplate, project.ArtifactConventionPart, project.ArtifactAuthoredData, project.ArtifactProtocolDescriptor:
			records[i].Navigation = cloneArtifactLinks(records[i].Outputs)
		case project.ArtifactTopicMetadata, project.ArtifactClaimPart:
			id := records[i].Identity
			domain := strings.SplitN(id, "/", 2)[0]
			records[i].Navigation = append(linkIfDeclared(authorities.Layout.DocsDir+"/topics/"+id+".md", "topic document"), linkIfDeclared(authorities.Layout.DomainsDir+"/"+domain+".md", "domain document")...)
		case project.ArtifactDecisionRecord:
			records[i].Navigation = linkIfDeclared(authorities.Layout.IndexMd, "decision index")
		case project.ArtifactManagedOutput:
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
	roleOrder := map[project.ArtifactRole]int{project.ArtifactConfig: 0, project.ArtifactLock: 1, project.ArtifactManifest: 2, project.ArtifactTemplate: 3, project.ArtifactProtocolDescriptor: 4, project.ArtifactConventionPart: 5, project.ArtifactAuthoredData: 6, project.ArtifactTopicMetadata: 7, project.ArtifactClaimPart: 8, project.ArtifactDecisionRecord: 9, project.ArtifactManagedOutput: 10}
	slices.SortFunc(records, func(a, b artifactRecord) int {
		if a.Role != b.Role {
			return roleOrder[a.Role] - roleOrder[b.Role]
		}
		return strings.Compare(a.Identity, b.Identity)
	})
	return records
}

func artifactSourceLabel(role project.ArtifactRole) string {
	switch role {
	case project.ArtifactConfig:
		return "project config"
	case project.ArtifactTemplate:
		return "render template"
	case project.ArtifactProtocolDescriptor:
		return "protocol descriptor"
	case project.ArtifactConventionPart:
		return "convention part"
	case project.ArtifactAuthoredData:
		return "authored data"
	case project.ArtifactTopicMetadata:
		return "topic metadata"
	case project.ArtifactClaimPart:
		return "claim part"
	case project.ArtifactDecisionRecord:
		return "decision record"
	case project.ArtifactManagedOutput:
		return "in-place managed output"
	default:
		return string(role)
	}
}

func canonicalArtifactInputRole(role project.ArtifactRole) bool {
	return role == project.ArtifactConfig || role == project.ArtifactTopicMetadata || role == project.ArtifactClaimPart || role == project.ArtifactDecisionRecord
}

func declarationOutputs(path string, declarations []project.OutputDeclaration) []artifactLink {
	out := []artifactLink{}
	for _, d := range declarations {
		if d.Reservation {
			continue
		}
		for _, in := range d.Inputs {
			if in.Path == path {
				out = append(out, artifactLink{Path: d.Path, Label: "managed output"})
			}
		}
	}
	return mergeArtifactLinks(nil, out)
}
func applyArtifactSnapshots(records []artifactRecord, path string, tree *snapshot.Tree, lock *manifest.Lock) {
	for i := range records {
		if records[i].Role != project.ArtifactManagedOutput {
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
