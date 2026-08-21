package publisher

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func TestNewRejectsMissingCompositionDependencies(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	cases := []struct {
		name string
		new  func()
	}{
		{"state", func() { New(nil, cfg, memoryProjectReader{}, project.Version) }},
		{"config", func() { New(state.OutputState(), nil, memoryProjectReader{}, project.Version) }},
		{"reader", func() { New(state.OutputState(), cfg, nil, project.Version) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("New accepted a missing composition dependency")
				}
			}()
			tc.new()
		})
	}
}

func TestPublisherRenderResidentMarkerPropagatesPlanningFailure(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	lower := lowerWithTargets(state.OutputState(), append(state.Targets(), Target{Outputs: []TargetOutput{{TemplateID: "missing/live-template.tmpl"}}}))
	publisher := New(lower, cfg, NewFilesystemReader(state.Root()), project.Version)
	if _, err := publisher.RenderResidentMarker("effort-archive"); err == nil {
		t.Fatal("RenderResidentMarker hid planning failure")
	}
}
