package project

import (
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type ArtifactRole string

const (
	ArtifactConfig         ArtifactRole = "config"
	ArtifactLock           ArtifactRole = "lock"
	ArtifactManifest       ArtifactRole = "manifest"
	ArtifactTemplate       ArtifactRole = "template"
	ArtifactConventionPart ArtifactRole = "convention-part"
	ArtifactAuthoredData   ArtifactRole = "authored-data"
	ArtifactTopicMetadata  ArtifactRole = "topic-metadata"
	ArtifactClaimPart      ArtifactRole = "claim-part"
	ArtifactDecisionRecord ArtifactRole = "decision-record"
	ArtifactManagedOutput  ArtifactRole = "managed-output"
)

type ArtifactLink struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}
type ArtifactSnapshot struct {
	InManifest bool `json:"inManifest"`
	Drifted    bool `json:"drifted"`
}
type ArtifactRecord struct {
	Role       ArtifactRole      `json:"role"`
	Identity   string            `json:"identity"`
	Sources    []ArtifactLink    `json:"sources"`
	Outputs    []ArtifactLink    `json:"outputs"`
	Navigation []ArtifactLink    `json:"navigation"`
	Snapshot   *ArtifactSnapshot `json:"snapshot,omitempty"`
}

type artifactAuthorities struct {
	Layout Layout
	ADRs   adr.Corpus
}

func artifactRecords(path string, declarations []OutputDeclaration, authorities artifactAuthorities) []ArtifactRecord {
	records := []ArtifactRecord{}
	add := func(role ArtifactRole, identity string, sources, outputs []ArtifactLink) {
		for i := range records {
			if records[i].Role == role && records[i].Identity == identity {
				records[i].Sources = mergeArtifactLinks(records[i].Sources, sources)
				records[i].Outputs = mergeArtifactLinks(records[i].Outputs, outputs)
				return
			}
		}
		records = append(records, ArtifactRecord{Role: role, Identity: identity, Sources: nonNilLinks(sources), Outputs: nonNilLinks(outputs), Navigation: []ArtifactLink{}})
	}
	configReference := authorities.Layout.DocsDir + "/config-reference.md"
	linkIfDeclared := func(path, label string) []ArtifactLink {
		for _, declaration := range declarations {
			if declaration.Path == path && !declaration.Reservation {
				return []ArtifactLink{{Path: path, Label: label}}
			}
		}
		return []ArtifactLink{}
	}
	switch {
	case path == ".awf/config.yaml":
		add(ArtifactConfig, "project-config", nil, declarationOutputs(path, declarations))
	case path == ".awf/awf.lock":
		add(ArtifactLock, "project-lock", nil, nil)
		add(ArtifactManifest, "output-manifest", nil, nil)
	case strings.HasPrefix(path, ".awf/topics/metadata/") && strings.HasSuffix(path, ".yaml"):
		add(ArtifactTopicMetadata, strings.TrimSuffix(strings.TrimPrefix(path, ".awf/topics/metadata/"), ".yaml"), nil, declarationOutputs(path, declarations))
	case strings.HasPrefix(path, ".awf/topics/parts/") && strings.HasSuffix(path, "/current-state.md"):
		add(ArtifactClaimPart, strings.TrimSuffix(strings.TrimPrefix(path, ".awf/topics/parts/"), "/current-state.md"), nil, declarationOutputs(path, declarations))
	case strings.HasPrefix(path, strings.TrimRight(authorities.Layout.ADRDir, "/")+"/"):
		base := strings.TrimPrefix(path, strings.TrimRight(authorities.Layout.ADRDir, "/")+"/")
		if match := adr.FilenameRe.FindStringSubmatch(base); match != nil {
			if record, ok := authorities.ADRs.ByNumber(match[1]); ok && record.Filename == base {
				add(ArtifactDecisionRecord, record.Number, nil, declarationOutputs(path, declarations))
			}
		}
	}
	for _, d := range declarations {
		if d.Path == path && !d.Reservation {
			sources := make([]ArtifactLink, 0, len(d.Inputs))
			outputs := []ArtifactLink{}
			for _, in := range d.Inputs {
				sources = append(sources, ArtifactLink{Path: in.Path, Label: artifactSourceLabel(in.Role)})
				if in.Path == path && in.Role == ArtifactManagedOutput {
					outputs = append(outputs, ArtifactLink{Path: d.Path, Label: "managed output"})
				}
			}
			identity := d.TemplateID
			if identity == "" {
				identity = strings.Join(d.Declarers, ",")
			}
			add(ArtifactManagedOutput, identity, sources, outputs)
		}
		if !d.Reservation {
			for _, in := range d.Inputs {
				if in.Path == path && !canonicalArtifactInputRole(in.Role) && in.Role != ArtifactManagedOutput {
					identity := path
					if in.Role == ArtifactTemplate {
						identity = strings.TrimPrefix(path, "templates/")
					}
					add(in.Role, identity, nil, []ArtifactLink{{Path: d.Path, Label: "managed output"}})
				}
			}
		}
	}
	for i := range records {
		switch records[i].Role {
		case ArtifactConfig:
			records[i].Navigation = linkIfDeclared(configReference, "configuration reference")
		case ArtifactLock:
			records[i].Navigation = append([]ArtifactLink{{Path: ".awf/config.yaml", Label: "project config"}}, linkIfDeclared(configReference, "configuration reference")...)
		case ArtifactManifest:
			records[i].Navigation = linkIfDeclared(configReference, "configuration reference")
		case ArtifactTemplate, ArtifactConventionPart, ArtifactAuthoredData:
			records[i].Navigation = cloneArtifactLinks(records[i].Outputs)
		case ArtifactTopicMetadata, ArtifactClaimPart:
			id := records[i].Identity
			domain := strings.SplitN(id, "/", 2)[0]
			records[i].Navigation = append(linkIfDeclared(authorities.Layout.DocsDir+"/topics/"+id+".md", "topic document"), linkIfDeclared(authorities.Layout.DomainsDir+"/"+domain+".md", "domain document")...)
		case ArtifactDecisionRecord:
			records[i].Navigation = linkIfDeclared(authorities.Layout.IndexMd, "decision index")
		case ArtifactManagedOutput:
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
	roleOrder := map[ArtifactRole]int{ArtifactConfig: 0, ArtifactLock: 1, ArtifactManifest: 2, ArtifactTemplate: 3, ArtifactConventionPart: 4, ArtifactAuthoredData: 5, ArtifactTopicMetadata: 6, ArtifactClaimPart: 7, ArtifactDecisionRecord: 8, ArtifactManagedOutput: 9}
	slices.SortFunc(records, func(a, b ArtifactRecord) int {
		if a.Role != b.Role {
			return roleOrder[a.Role] - roleOrder[b.Role]
		}
		return strings.Compare(a.Identity, b.Identity)
	})
	return records
}

func artifactSourceLabel(role ArtifactRole) string {
	switch role {
	case ArtifactConfig:
		return "project config"
	case ArtifactTemplate:
		return "render template"
	case ArtifactConventionPart:
		return "convention part"
	case ArtifactAuthoredData:
		return "authored data"
	case ArtifactTopicMetadata:
		return "topic metadata"
	case ArtifactClaimPart:
		return "claim part"
	case ArtifactDecisionRecord:
		return "decision record"
	case ArtifactManagedOutput:
		return "in-place managed output"
	default:
		return string(role)
	}
}

func canonicalArtifactInputRole(role ArtifactRole) bool {
	return role == ArtifactConfig || role == ArtifactTopicMetadata || role == ArtifactClaimPart || role == ArtifactDecisionRecord
}

func declarationOutputs(path string, declarations []OutputDeclaration) []ArtifactLink {
	out := []ArtifactLink{}
	for _, d := range declarations {
		if d.Reservation {
			continue
		}
		for _, in := range d.Inputs {
			if in.Path == path {
				out = append(out, ArtifactLink{Path: d.Path, Label: "managed output"})
			}
		}
	}
	return mergeArtifactLinks(nil, out)
}
func applyArtifactSnapshots(records []ArtifactRecord, path string, tree *snapshot.Tree, lock *manifest.Lock) {
	for i := range records {
		if records[i].Role != ArtifactManagedOutput {
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
		records[i].Snapshot = &ArtifactSnapshot{InManifest: inManifest, Drifted: drifted}
	}
}

func cloneArtifactLinks(in []ArtifactLink) []ArtifactLink { return append([]ArtifactLink{}, in...) }
func mergeArtifactLinks(a, b []ArtifactLink) []ArtifactLink {
	out := append(cloneArtifactLinks(a), b...)
	slices.SortFunc(out, func(a, b ArtifactLink) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Label, b.Label)
	})
	return slices.Compact(out)
}
func nonNilLinks(in []ArtifactLink) []ArtifactLink {
	if in == nil {
		return []ArtifactLink{}
	}
	return in
}
