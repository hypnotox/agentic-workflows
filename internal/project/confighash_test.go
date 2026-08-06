package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestDataDefaultsConfigurationChangesConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	eff := mustDeriveSkills(t, p)
	without, err := p.artifactConfigHash("plain", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	withTrue, err := p.artifactConfigHash("plain", config.Sidecar{DataDefaults: map[string]bool{"items": true}}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	withFalse, err := p.artifactConfigHash("plain", config.Sidecar{DataDefaults: map[string]bool{"items": false}}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	if without == withTrue || withTrue == withFalse || without == withFalse {
		t.Fatalf("dataDefaults configuration presence/value not represented in hashes: %q %q %q", without, withTrue, withFalse)
	}
}

func TestCommitPolicyConsumerConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	eff := mustDeriveSkills(t, p)
	consumerBefore, err := p.artifactConfigHash("{{ with .commitPolicy }}{{ .GrandfatheredThrough }}{{ end }}", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	unrelatedBefore, err := p.artifactConfigHash("plain prose mentioning .commitPolicy", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	commentBefore, err := p.artifactConfigHash("{{/* .commitPolicy */}}", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	policies := []*config.CommitPolicyConfig{
		{GrandfatheredThrough: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{GrandfatheredThrough: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AllowedIdentities: []config.CommitPolicyIdentity{{Name: "Ada", Email: "ada@example.test"}}},
		{GrandfatheredThrough: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", AllowedIdentities: []config.CommitPolicyIdentity{{Name: "Ada", Email: "ada@example.test"}}, RequireSignedCommits: true, AllowedSigners: []config.CommitPolicySigner{{Principal: "ada@example.test", Key: "ssh-ed25519 key"}}},
	}
	previous := consumerBefore
	for i, policy := range policies {
		p.Cfg.CommitPolicy = policy
		got, err := p.artifactConfigHash("{{ with .commitPolicy }}{{ .GrandfatheredThrough }}{{ end }}", config.Sidecar{}, nil, eff)
		if err != nil {
			t.Fatal(err)
		}
		if got == previous {
			t.Fatalf("policy mutation %d did not change consumer hash %q", i, got)
		}
		previous = got
	}
	unrelatedAfter, err := p.artifactConfigHash("plain prose mentioning .commitPolicy", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	commentAfter, err := p.artifactConfigHash("{{/* .commitPolicy */}}", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	if unrelatedBefore != unrelatedAfter || commentBefore != commentAfter {
		t.Fatalf("non-consumer hashes changed: prose %q/%q comment %q/%q", unrelatedBefore, unrelatedAfter, commentBefore, commentAfter)
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
