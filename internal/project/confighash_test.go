package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestCommitPolicyConsumerConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	eff := mustDeriveSkills(t, p)
	consumerBefore, err := p.artifactConfigHash("{{ with .commitPolicy }}{{ .grandfatheredThrough }}{{ end }}", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	unrelatedBefore, err := p.artifactConfigHash("plain", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg.CommitPolicy = &config.CommitPolicyConfig{GrandfatheredThrough: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	consumerAfter, err := p.artifactConfigHash("{{ with .commitPolicy }}{{ .grandfatheredThrough }}{{ end }}", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	unrelatedAfter, err := p.artifactConfigHash("plain", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	if consumerBefore == consumerAfter || unrelatedBefore != unrelatedAfter {
		t.Fatalf("consumer hashes %q/%q and unrelated hashes %q/%q", consumerBefore, consumerAfter, unrelatedBefore, unrelatedAfter)
	}
	part := filepath.Join(root, ".awf", "parts", "scope.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("{{=awf:commitScopes}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.artifactConfigHash("plain", config.Sidecar{}, []string{part}, eff); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("<!-- awf:comment no close\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.artifactConfigHash("plain", config.Sidecar{}, []string{part}, eff); err == nil {
		t.Fatal("malformed authoring comment did not fail config hashing")
	}
}

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
