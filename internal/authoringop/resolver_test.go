package authoringop

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func resolverFixture(t *testing.T) (*project.ProjectState, *config.Config) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [tooling]\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
	state, cfg, err := loader.OpenForOperation(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return state, cfg
}

func TestResolvePartUsesKindCatalogConfigurationAndLayouts(t *testing.T) {
	state, cfg := resolverFixture(t)
	cases := []struct {
		kind, name, part string
		local            bool
	}{
		{"skill", "tdd", catalog.Standard.Skills["tdd"].Sections[0], false},
		{"agent", "implementer", catalog.Standard.Agents["implementer"].Sections[0], false},
		{"doc", "architecture", catalog.Standard.Docs["architecture"].Sections[0], false},
		{"domain", "tooling", catalog.Standard.DomainDoc.Sections[0], false},
		{"doc", "runbooks/incident", "body", true},
	}
	for _, tc := range cases {
		t.Run(tc.kind+"/"+tc.name, func(t *testing.T) {
			target, err := ResolvePart(state, cfg, tc.kind, tc.name, tc.part)
			if err != nil {
				t.Fatal(err)
			}
			if target.Local != tc.local || target.SourcePath == "" {
				t.Fatalf("target = %#v", target)
			}
		})
	}
	for _, tc := range []struct{ kind, name, part string }{
		{"bogus", "x", "x"}, {"skill", "absent", "x"}, {"domain", "absent", catalog.Standard.DomainDoc.Sections[0]},
		{"doc", "architecture", "absent"}, {"doc", "runbooks/incident", "title"},
	} {
		if _, err := ResolvePart(state, cfg, tc.kind, tc.name, tc.part); err == nil {
			t.Errorf("accepted invalid target %#v", tc)
		}
	}
	if !slices.Contains(project.Kinds(), "doc") {
		t.Fatal("closed kind authority omitted doc")
	}
}
