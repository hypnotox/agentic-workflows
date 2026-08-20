package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

func TestDataDefaultsConfigurationChangesConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	eff := mustDeriveSkills(t, p)
	without, err := artifactConfigHash(renderInputsForTest(p), "plain", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	withTrue, err := artifactConfigHash(renderInputsForTest(p), "plain", config.Sidecar{DataDefaults: map[string]bool{"items": true}}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	withFalse, err := artifactConfigHash(renderInputsForTest(p), "plain", config.Sidecar{DataDefaults: map[string]bool{"items": false}}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	if without == withTrue || withTrue == withFalse || without == withFalse {
		t.Fatalf("dataDefaults configuration presence/value not represented in hashes: %q %q %q", without, withTrue, withFalse)
	}
}

func TestCommitPolicyConsumerConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	eff := mustDeriveSkills(t, p)
	consumerBefore, err := artifactConfigHash(renderInputsForTest(p), "{{ with .commitPolicy }}{{ .GrandfatheredThrough }}{{ end }}", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	unrelatedBefore, err := artifactConfigHash(renderInputsForTest(p), "plain prose mentioning .commitPolicy", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	commentBefore, err := artifactConfigHash(renderInputsForTest(p), "{{/* .commitPolicy */}}", config.Sidecar{}, nil, eff)
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
		got, err := artifactConfigHash(renderInputsForTest(p), "{{ with .commitPolicy }}{{ .GrandfatheredThrough }}{{ end }}", config.Sidecar{}, nil, eff)
		if err != nil {
			t.Fatal(err)
		}
		if got == previous {
			t.Fatalf("policy mutation %d did not change consumer hash %q", i, got)
		}
		previous = got
	}
	unrelatedAfter, err := artifactConfigHash(renderInputsForTest(p), "plain prose mentioning .commitPolicy", config.Sidecar{}, nil, eff)
	if err != nil {
		t.Fatal(err)
	}
	commentAfter, err := artifactConfigHash(renderInputsForTest(p), "{{/* .commitPolicy */}}", config.Sidecar{}, nil, eff)
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
	if _, err := artifactConfigHash(renderInputsForTest(p), "plain", config.Sidecar{}, []string{part}, eff); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("<!-- awf:comment no close\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := artifactConfigHash(renderInputsForTest(p), "plain", config.Sidecar{}, []string{part}, eff); err == nil {
		t.Fatal("malformed authoring comment did not fail config hashing")
	}
}

// invariant: config/configuration:template-source-root (TestTemplateSourceRootChangesOnlyActivatedMarkdownConfigHash)
func TestTemplateSourceRootChangesOnlyActivatedMarkdownConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	const tid = "adr-readme/README.md.tmpl"
	src, err := templates.FS.ReadFile(tid)
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := render.ExpandIncludesSource(string(src), tid, templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range expanded.Spans {
		if span.Source == "" {
			continue
		}
		file := filepath.Join(root, "templates", filepath.FromSlash(span.Source))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(span.Text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sc := config.Sidecar{}
	plain, err := renderTarget(renderInputsForTest(p), "adr-readme", "", tid, projectCatalog(renderInputsForTest(p)).Docs["adr-readme"].Sections, sc, projectData(renderInputsForTest(p), sc, map[string]bool{}), "out.md", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg.Render = &config.RenderConfig{TemplateSourceRoot: "templates"}
	active, err := renderTarget(renderInputsForTest(p), "adr-readme", "", tid, projectCatalog(renderInputsForTest(p)).Docs["adr-readme"].Sections, sc, projectData(renderInputsForTest(p), sc, map[string]bool{}), "out.md", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if plain.ConfigHash == active.ConfigHash || plain.TemplateHash != active.TemplateHash {
		t.Fatalf("root projection hashes plain=%#v active=%#v", plain, active)
	}
	nativeBefore, err := renderTarget(renderInputsForTest(p), "hooks", "", "hooks/pre-commit.sh.tmpl", nil, sc, projectData(renderInputsForTest(p), sc, map[string]bool{}), "hook", map[string]bool{}, &renderOutputOptions{encoder: PlainAgentDialect})
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg.Render = nil
	nativeAbsent, err := renderTarget(renderInputsForTest(p), "hooks", "", "hooks/pre-commit.sh.tmpl", nil, sc, projectData(renderInputsForTest(p), sc, map[string]bool{}), "hook", map[string]bool{}, &renderOutputOptions{encoder: PlainAgentDialect})
	if err != nil {
		t.Fatal(err)
	}
	if nativeBefore.ConfigHash != nativeAbsent.ConfigHash || nativeBefore.TemplateHash != nativeAbsent.TemplateHash {
		t.Fatalf("native encoder projected root: before=%#v absent=%#v", nativeBefore, nativeAbsent)
	}
}

func TestRetiredTelemetryTemplateValuesDoNotAffectConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := artifactConfigHash(renderInputsForTest(p), "{{ .telemetryWidgetEnabled }} {{ .telemetryWidgetShowCost }}", config.Sidecar{}, nil, mustDeriveSkills(t, p))
	if err != nil {
		t.Fatal(err)
	}
	after, err := artifactConfigHash(renderInputsForTest(p), "{{ .telemetryWidgetEnabled }} {{ .telemetryWidgetShowCost }}", config.Sidecar{}, nil, mustDeriveSkills(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("retired telemetry data changed config hash: %q != %q", before, after)
	}
}

// invariant: rendering/sync-and-drift:profile-config-hash (TestProfileChangesConfigHash)
func TestProfileChangesConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: core\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	core, err := artifactConfigHash(renderInputsForTest(p), "plain", config.Sidecar{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg.Profile = "full"
	full, err := artifactConfigHash(renderInputsForTest(p), "plain", config.Sidecar{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if core == full {
		t.Fatalf("profile did not participate in config hash: %q", core)
	}
}
