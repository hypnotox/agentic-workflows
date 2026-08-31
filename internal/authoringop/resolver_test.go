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

func resolverFixture(t *testing.T) (*project.Session, *config.Config) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [tooling]\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
	state, err := loader.Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return state, state.Config()
}

// invariant: tooling/cli:semantic-artifact-authoring (TestResolveSidecarUsesSemanticCapabilitiesAndOwnedLayouts)
func TestResolveSidecarUsesSemanticCapabilitiesAndOwnedLayouts(t *testing.T) {
	state, cfg := resolverFixture(t)
	cases := []struct {
		kind, name, field, path string
	}{
		{"skill", "using-awf", "data.custom", ".awf/skills/using-awf.yaml"},
		{"agent", "implementer", "dataDefaults.tools", ".awf/agents/implementer.yaml"},
		{"doc", "architecture", "data.custom", ".awf/docs/architecture.yaml"},
		{"doc", "working-with-awf", "data.custom", ".awf/working-with-awf.yaml"},
		{"doc", "glossary", "data.custom", ".awf/docs/glossary.yaml"},
		{"domain", "tooling", "paths", ".awf/domains/tooling.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.kind+"/"+tc.name+"/"+tc.field, func(t *testing.T) {
			target, err := ResolveSidecar(state, cfg, tc.kind, tc.name, tc.field)
			if err != nil {
				t.Fatal(err)
			}
			if target.SourcePath != tc.path || target.Local {
				t.Fatalf("target = %#v, want path %q", target, tc.path)
			}
		})
	}
	section := catalog.Standard.Skills["using-awf"].Sections[0]
	for _, tc := range []struct{ kind, name, field string }{
		{"bogus", "x", "data.key"},
		{"skill", "absent", "data.key"},
		{"domain", "absent", "paths"},
		{"domain", "tooling", "data.key"},
		{"skill", "using-awf", "data"},
		{"skill", "using-awf", "sections.absent.drop"},
		{"skill", "using-awf", "sections." + section},
		{"doc", "runbooks/incident", "data.key"},
	} {
		if _, err := ResolveSidecar(state, cfg, tc.kind, tc.name, tc.field); err == nil {
			t.Errorf("accepted invalid sidecar target %#v", tc)
		}
	}
}

func TestResolveTargetsRejectMissingAuthoringAuthority(t *testing.T) {
	state, cfg := resolverFixture(t)
	for _, tc := range []struct {
		name  string
		state *project.Session
		cfg   *config.Config
	}{
		{name: "nil state", cfg: cfg},
		{name: "missing state", state: &project.Session{}, cfg: cfg},
		{name: "nil config", state: state},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, resolve := range []func(*project.Session, *config.Config) error{
				func(state *project.Session, cfg *config.Config) error {
					_, err := project.ResolveSidecarTarget(state, cfg, "skill", "using-awf", "data.custom")
					return err
				},
				func(state *project.Session, cfg *config.Config) error {
					_, err := project.ResolveAuthoringTarget(state, cfg, "skill", "using-awf", "identity")
					return err
				},
			} {
				if err := resolve(tc.state, tc.cfg); err == nil || err.Error() != "project: missing authoring authority" {
					t.Errorf("error = %v", err)
				}
			}
		})
	}
}

func TestResolvePartUsesKindCatalogConfigurationAndLayouts(t *testing.T) {
	state, cfg := resolverFixture(t)
	cases := []struct {
		kind, name, part string
		local            bool
	}{
		{"skill", "using-awf", catalog.Standard.Skills["using-awf"].Sections[0], false},
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
