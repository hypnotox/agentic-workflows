package project

import (
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// PlanDocument maps one enablement resolver plan into a homogeneous collection.
func PlanDocument(plan []PlanOp) (presentation.Document, error) {
	entries := make([]string, 0, len(plan))
	for _, op := range plan {
		sign := "+"
		if !op.Enable {
			sign = "-"
		}
		entry := fmt.Sprintf("%s %s %s", sign, op.Node.Kind, op.Node.Name)
		if op.RequiredBy != "" {
			entry += " (required by " + op.RequiredBy + ")"
		}
		entries = append(entries, entry)
	}
	category, err := listCategory("plan operations", entries)
	if err != nil {
		return presentation.Document{}, err
	}
	return (presentation.Collection{Status: "enablement plan", Categories: []presentation.CollectionCategory{category}}).Document()
}

// EnablementNoteReason identifies one project-owned post-enablement advisory.
type EnablementNoteReason int

const (
	// EnablementNoteOrphanedAuthoredState reports authored sidecars or parts left behind by removal.
	EnablementNoteOrphanedAuthoredState EnablementNoteReason = iota
	// EnablementNoteAgentNoLongerRequired reports a retained agent with no remaining skill requirement.
	EnablementNoteAgentNoLongerRequired
)

// EnablementNote is one typed post-enablement advisory.
type EnablementNote struct {
	Reason EnablementNoteReason
	Kind   string
	Name   string
}

// EnablementNotesDocument maps typed post-mutation advisories into one collection.
func EnablementNotesDocument(notes []EnablementNote) (presentation.Document, error) {
	entries := make([]string, 0, len(notes))
	for _, note := range notes {
		switch note.Reason {
		case EnablementNoteOrphanedAuthoredState:
			entries = append(entries, fmt.Sprintf("%s %q still has a sidecar or convention parts under .awf/, now orphaned (awf check will flag them); delete them or re-enable to keep them", note.Kind, note.Name))
		case EnablementNoteAgentNoLongerRequired:
			entries = append(entries, fmt.Sprintf("agent %q is no longer required by any enabled skill; it stays enabled (remove it separately if unwanted)", note.Name))
		default:
			return presentation.Document{}, fmt.Errorf("unknown enablement note reason %d", note.Reason)
		}
	}
	category, err := listCategory("notes", entries)
	if err != nil { // coverage-ignore: typed note formatting quotes identities and emits fixed single-line prose
		return presentation.Document{}, err
	}
	return (presentation.Collection{Status: "enablement notes", Categories: []presentation.CollectionCategory{category}}).Document()
}

// ListDocument maps the project's enabled and available artifacts into one
// collection. Category order follows the project kind order and every entry is
// a bare value, rather than an independently rendered status document.
func (p *Project) ListDocument(kindFilter string) (presentation.Document, error) {
	kinds := Kinds()
	if kindFilter != "" {
		if kindFilter == "target" || kindFilter == "bootstrap" || kindFilter == "hooks" || kindFilter == "runner" {
			return listSpecialDocument(p, kindFilter)
		}
		if _, ok := PluralKind(kindFilter); !ok {
			return presentation.Document{}, fmt.Errorf("unknown kind %q", kindFilter)
		}
		kinds = []string{kindFilter}
	}
	categories := make([]presentation.CollectionCategory, 0, len(kinds)+3)
	for _, kind := range kinds {
		plural, _ := PluralKind(kind)
		pool, catalogBacked := CatalogNames(p.Cat, kind)
		entries := []string{}
		if !catalogBacked {
			names := slices.Sorted(slices.Values(p.Cfg.Domains))
			for _, name := range names {
				entries = append(entries, name+" (configured)")
			}
			if len(entries) == 0 {
				entries = append(entries, "none")
			}
		} else {
			for _, name := range pool {
				entries = append(entries, name+" ("+artifactState(p, kind, name)+")")
			}
		}
		category, err := listCategory(plural, entries)
		if err != nil { // coverage-ignore: Open validates configured names and every catalog/special entry is synthesized as a single-line literal
			return presentation.Document{}, err
		}
		categories = append(categories, category)
	}
	if kindFilter == "" {
		for _, special := range []string{"targets", "bootstrap", "hooks"} {
			category, err := specialListCategory(p, special)
			if err != nil { // coverage-ignore: the fixed special names synthesize only grammar-valid single-line entries
				return presentation.Document{}, err
			}
			categories = append(categories, category)
		}
	}
	return (presentation.Collection{Status: "available artifacts", Categories: categories}).Document()
}

func listSpecialDocument(p *Project, kind string) (presentation.Document, error) {
	category, err := specialListCategory(p, map[string]string{"target": "targets", "bootstrap": "bootstrap", "hooks": "hooks", "runner": "runner"}[kind])
	if err != nil { // coverage-ignore: callers admit only the four mapped special kinds, whose entries are fixed and grammar-valid
		return presentation.Document{}, err
	}
	return (presentation.Collection{Status: "available artifacts", Categories: []presentation.CollectionCategory{category}}).Document()
}

func specialListCategory(p *Project, kind string) (presentation.CollectionCategory, error) {
	entries := []string{}
	switch kind {
	case "targets":
		for _, name := range KnownTargets() {
			state := "available"
			if slices.Contains(p.Cfg.Targets, name) {
				state = "enabled"
			}
			entries = append(entries, name+" ("+state+")")
		}
	case "bootstrap":
		state := "available"
		if p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled {
			state = "enabled"
		}
		entries = []string{".awf/bootstrap.sh (" + state + ")", ".awf/upgrade.sh (" + state + ")"}
	case "hooks":
		state := "available"
		if p.Cfg.Hooks != nil && p.Cfg.Hooks.Enabled {
			state = "enabled"
		}
		for _, name := range HookNames() {
			entries = append(entries, ".awf/hooks/"+name+".sh ("+state+")")
		}
	case "runner":
		state := "available"
		if p.Cfg.Runner != nil && p.Cfg.Runner.Enabled {
			state = "enabled"
		}
		entries = []string{"awf (" + state + ")"}
	}
	return listCategory(kind, entries)
}

func listCategory(label string, entries []string) (presentation.CollectionCategory, error) {
	values := make([]presentation.Value, 0, len(entries))
	for _, entry := range entries {
		value, err := presentation.Literal(entry)
		if err != nil {
			return presentation.CollectionCategory{}, err
		}
		values = append(values, value)
	}
	return presentation.CollectionCategory{Label: label, Values: values}, nil
}

func artifactState(p *Project, kind, name string) string {
	enabled, _ := EnabledNames(p.Cfg, kind)
	if !slices.Contains(enabled, name) {
		return "available"
	}
	if standard, _ := CatalogNames(catalog.Standard, kind); !slices.Contains(standard, name) {
		return "local"
	}
	plural, _ := PluralKind(kind)
	sidecar, _ := p.Cfg.Sidecar(plural, name)
	switch {
	case sidecar.Local:
		return "local"
	case sidecar.Data != nil || sidecar.Sections != nil:
		return "tuned"
	default:
		return "enabled"
	}
}
