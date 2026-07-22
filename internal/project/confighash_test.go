package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestDashboardWidgetLeavesAreTheOnlyTelemetryConfigHashInputs(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := "{{ .telemetryWidgetEnabled }} {{ .telemetryWidgetShowCost }}"
	before, err := p.artifactConfigHash(dashboard, config.Sidecar{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plainBefore, err := p.artifactConfigHash("plain", config.Sidecar{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg.WorkflowTelemetry.Widget.Enabled = !p.Cfg.WorkflowTelemetry.Widget.Enabled
	p.Cfg.WorkflowTelemetry.Widget.ShowCost = !p.Cfg.WorkflowTelemetry.Widget.ShowCost
	after, err := p.artifactConfigHash(dashboard, config.Sidecar{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plainAfter, err := p.artifactConfigHash("plain", config.Sidecar{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("widget leaves did not change dashboard config hash")
	}
	if plainBefore != plainAfter {
		t.Fatal("widget leaves changed an unrelated config hash")
	}
}
