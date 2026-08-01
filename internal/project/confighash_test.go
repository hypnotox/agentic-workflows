package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestRetiredTelemetryTemplateValuesDoNotAffectConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := p.artifactConfigHash("{{ .telemetryWidgetEnabled }} {{ .telemetryWidgetShowCost }}", config.Sidecar{}, nil, mustDeriveSkills(t, p))
	if err != nil {
		t.Fatal(err)
	}
	after, err := p.artifactConfigHash("{{ .telemetryWidgetEnabled }} {{ .telemetryWidgetShowCost }}", config.Sidecar{}, nil, mustDeriveSkills(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("retired telemetry data changed config hash: %q != %q", before, after)
	}
}
