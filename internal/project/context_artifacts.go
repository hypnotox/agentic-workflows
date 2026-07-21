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

func artifactRecords(path string, declarations []OutputDeclaration, decisionsDir string, adrs adr.Corpus) []ArtifactRecord {
	records := []ArtifactRecord{}
	add := func(role ArtifactRole, identity string, sources, outputs []ArtifactLink) {
		records = append(records, ArtifactRecord{Role: role, Identity: identity, Sources: nonNilLinks(sources), Outputs: nonNilLinks(outputs), Navigation: []ArtifactLink{}})
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
	case strings.HasPrefix(path, strings.TrimRight(decisionsDir, "/")+"/"):
		base := strings.TrimPrefix(path, strings.TrimRight(decisionsDir, "/")+"/")
		if match := adr.FilenameRe.FindStringSubmatch(base); match != nil {
			if record, ok := adrs.ByNumber(match[1]); ok && record.Filename == base {
				add(ArtifactDecisionRecord, path, nil, declarationOutputs(path, declarations))
			}
		}
	}
	for _, d := range declarations {
		if d.Path == path && !d.Reservation {
			sources := make([]ArtifactLink, 0, len(d.Inputs))
			for _, in := range d.Inputs {
				sources = append(sources, ArtifactLink{Path: in.Path, Label: string(in.Role)})
			}
			add(ArtifactManagedOutput, d.TemplateID, sources, nil)
		}
		if !d.Reservation {
			for _, in := range d.Inputs {
				if in.Path == path && !canonicalArtifactInputRole(in.Role) {
					add(in.Role, d.TemplateID, nil, []ArtifactLink{{Path: d.Path, Label: "managed output"}})
				}
			}
		}
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
func canonicalArtifactInputRole(role ArtifactRole) bool {
	return role == ArtifactConfig || role == ArtifactTopicMetadata || role == ArtifactClaimPart || role == ArtifactDecisionRecord
}

func declarationOutputs(path string, declarations []OutputDeclaration) []ArtifactLink {
	out := []ArtifactLink{}
	for _, d := range declarations {
		for _, in := range d.Inputs {
			if in.Path == path {
				out = append(out, ArtifactLink{Path: d.Path, Label: "managed output"})
			}
		}
	}
	return out
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

func nonNilLinks(in []ArtifactLink) []ArtifactLink {
	if in == nil {
		return []ArtifactLink{}
	}
	return in
}
