package project

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/configspec"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

const crefYAML = `prefix: example
integrationBranch: main
vars:
  gateCmd: make gate
  checkCmd:
`

func TestConfigReferenceRowsPropagatesInjectedTemplateReadError(t *testing.T) {
	root := scaffold(t, crefYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	selected := p.catalog()
	selected.Skills["missing-template"] = catalog.SkillSpec{}
	state := *p
	state.selectedCat = catalog.NewView(selected)
	p = &state
	_, err = configReferenceRows(renderInputsForTest(p), nil)
	if err == nil || !strings.Contains(err.Error(), "skills/missing-template/SKILL.md.tmpl") {
		t.Fatalf("config reference template error = %v", err)
	}
}

// invariant: config/configspec-and-reference:live-state-projection-explicit (TestLiveStateAuthorityRejectsOmissionAndWrongClass)
func TestTemplateSourceRootCurrentValue(t *testing.T) {
	cfg := &config.Config{}
	p := testState(cfg)
	if got := currentValueResolvers(newRenderInputs(p, cfg, nil))["render.templateSourceRoot"](); got != "(none)" {
		t.Fatalf("absent root = %q", got)
	}
	cfg.Render = &config.RenderConfig{TemplateSourceRoot: "templates"}
	if got := currentValueResolvers(newRenderInputs(p, cfg, nil))["render.templateSourceRoot"](); got != "`templates`" {
		t.Fatalf("configured root = %q", got)
	}
}

func TestLiveStateAuthorityRejectsOmissionAndWrongClass(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	resolvers := currentValueResolvers(renderInputsForTest(p))
	if err := validateLiveStateAuthority(configspec.LiveStateClassifications(), resolvers); err != nil {
		t.Fatal(err)
	}

	omitted := configspec.LiveStateClassifications()
	delete(omitted, "prefix")
	if err := validateLiveStateAuthority(omitted, resolvers); err == nil || !strings.Contains(err.Error(), "has no classification") {
		t.Fatalf("omitted classification error = %v", err)
	}
	wrongStatic := configspec.LiveStateClassifications()
	wrongStatic["prefix"] = configspec.StaticNotApplicable
	if err := validateLiveStateAuthority(wrongStatic, resolvers); err == nil || !strings.Contains(err.Error(), "static live-state key") {
		t.Fatalf("wrong static classification error = %v", err)
	}
	omittedResolver := currentValueResolvers(renderInputsForTest(p))
	delete(omittedResolver, "tags")
	if err := validateLiveStateAuthority(configspec.LiveStateClassifications(), omittedResolver); err == nil || !strings.Contains(err.Error(), `live-state key "tags" has no resolver`) {
		t.Fatalf("omitted resolver error = %v", err)
	}
	staticResolver := currentValueResolvers(renderInputsForTest(p))
	staticResolver["commitPolicy.allowedIdentities[].name"] = func() string { return "wrong" }
	if err := validateLiveStateAuthority(configspec.LiveStateClassifications(), staticResolver); err == nil || !strings.Contains(err.Error(), `static live-state key "commitPolicy.allowedIdentities[].name" has a resolver`) {
		t.Fatalf("structural static resolver error = %v", err)
	}
	extra := currentValueResolvers(renderInputsForTest(p))
	extra["not.a.key"] = func() string { return "wrong" }
	if err := validateLiveStateAuthority(configspec.LiveStateClassifications(), extra); err == nil || !strings.Contains(err.Error(), `live-state resolver "not.a.key" has no classification`) {
		t.Fatalf("extra resolver error = %v", err)
	}
	unknown := configspec.LiveStateClassifications()
	unknown["tags"] = configspec.LiveStateClass(99)
	if err := validateLiveStateAuthority(unknown, resolvers); err == nil || !strings.Contains(err.Error(), "unknown class") {
		t.Fatalf("unknown classification error = %v", err)
	}
}

func syncedProject(t *testing.T, configYAML string, files map[string]string) (string, *ProjectState) {
	t.Helper()
	root := scaffoldFiles(t, configYAML, files)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	return root, p
}

func TestConfigReferencePresentationRejectsInvalidRows(t *testing.T) {
	for _, test := range []struct {
		name  string
		model ConfigReference
	}{
		{name: "config key", model: ConfigReference{ConfigKeys: []ConfigKeyRow{{}}}},
		{name: "var", model: ConfigReference{VarEntries: []VarRow{{}}}},
		{name: "sidecar field", model: ConfigReference{SidecarFields: []ConfigKeyRow{{}}}},
		{name: "data key", model: ConfigReference{DataKeys: []DataKeyRow{{}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ConfigReferencePresentation("", &test.model, "reference"); err == nil {
				t.Fatal("invalid reference row produced a presentation")
			}
		})
	}
	valid := ConfigReference{ConfigKeys: []ConfigKeyRow{{Path: "key", Type: "string", Default: "none", Description: "description", Availability: "always"}}}
	if _, err := ConfigReferencePresentation("", &valid, " \n\t"); err == nil {
		t.Fatal("empty normalized status accepted")
	}
}

// The generated reference renders per-project state: key rows with resolved
// defaults, the three-way var states, consumer/dormant hints, per-artifact
// data keys - and the document map cites it.

// A minimal project still renders coherent prose - no empty table skeletons,
// no unresolved tokens (the publication-safe degradation for generated docs).
func TestConfigReferenceEmptyStateDegrades(t *testing.T) {
	root, _ := syncedProject(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", nil)
	b, err := os.ReadFile(filepath.Join(root, "docs/config-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if strings.Contains(got, "<no value>") || strings.Contains(got, "|  |  |") {
		t.Errorf("empty-state reference degraded incoherently:\n%s", got)
	}
	if !strings.Contains(got, "1 keys, 1 set") || !strings.Contains(got, "accept any scope") {
		t.Errorf("empty-state reference missing live-state prose:\n%s", got)
	}
	// The vars section still lists every catalog var (all absent).
	if !strings.Contains(got, "`gateCmd`") || !strings.Contains(got, "absent, declined") {
		t.Errorf("empty-state reference lost the var catalog:\n%s", got)
	}
}

// TestConfigReferenceDerivedLiveValues pins each structurally live project
// value to its non-secret current summary while item-schema and sidecar fields
// remain static.
// invariant: config/configspec-and-reference:live-state-projection-explicit (TestConfigReferenceDerivedLiveValues)
func TestConfigReferenceDerivedLiveValues(t *testing.T) {
	assertValues := func(t *testing.T, configYAML string, want map[string]string) {
		t.Helper()
		root, p := syncedProject(t, configYAML, nil)
		model, err := configReferenceProject(p, testContext(t))
		if err != nil {
			t.Fatal(err)
		}
		rows := map[string]string{}
		for _, row := range model.ConfigKeys {
			rows[row.Path] = row.Current
		}
		for path, value := range want {
			if got := rows[path]; got != value {
				t.Errorf("%s current = %q, want %q", path, got, value)
			}
		}
		for path, class := range configspec.LiveStateClassifications() {
			if strings.HasPrefix(path, "sidecar.") {
				continue
			}
			if class == configspec.LiveStateProjection && rows[path] == "n/a" {
				t.Errorf("live project row %q rendered n/a", path)
			}
		}
		if got := rows["commitPolicy.allowedIdentities[].name"]; got != "n/a" {
			t.Errorf("item-schema row current = %q, want n/a", got)
		}
		for _, row := range model.SidecarFields {
			if row.Current != "" {
				t.Errorf("sidecar row %q current = %q, want no project value", row.Path, row.Current)
			}
		}
		b, err := os.ReadFile(filepath.Join(root, "docs/config-reference.md"))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(b), "\n")
		for path, value := range want {
			prefix := "| `" + path + "` | "
			var row string
			for _, line := range lines {
				if strings.HasPrefix(line, prefix) {
					row = line
					break
				}
			}
			if row == "" {
				t.Errorf("generated reference missing %q row", path)
				continue
			}
			columns := strings.Split(row, " | ")
			if len(columns) < 5 {
				t.Errorf("generated reference row %q has %d columns", path, len(columns))
				continue
			}
			if got := columns[3]; got != value {
				t.Errorf("generated reference %s current = %q, want %q", path, got, value)
			}
		}
	}

	absent := "prefix: example\nprofile: full\nintegrationBranch: main\n"
	assertValues(t, absent, map[string]string{
		"tags": "(none)", "contextIgnore": "(none)",
		"commitPolicy.grandfatheredThrough": "(none)",
		"commitPolicy.allowedIdentities":    "(none)",
		"commitPolicy.requireSignedCommits": "false (default)",
		"commitPolicy.allowedSigners":       "(none)",
	})

	presentFalsePolicy := absent + `commitPolicy:
  grandfatheredThrough: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  requireSignedCommits: false
`
	assertValues(t, presentFalsePolicy, map[string]string{
		"commitPolicy.grandfatheredThrough": "`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`",
		"commitPolicy.allowedIdentities":    "(none)",
		"commitPolicy.requireSignedCommits": "false",
		"commitPolicy.allowedSigners":       "(none)",
	})

	t.Run("non-nil empty grandfathered boundary", func(t *testing.T) {
		cfg := &config.Config{CommitPolicy: &config.CommitPolicyConfig{}}
		p := testState(cfg)
		if got := currentValueResolvers(newRenderInputs(p, cfg, nil))["commitPolicy.grandfatheredThrough"](); got != "(none)" {
			t.Errorf("empty grandfatheredThrough current = %q, want (none)", got)
		}
	})

	configured := absent + `tags:
  release: Release work.
  security: Security work.
contextIgnore: [docs/**, README.md]
commitPolicy:
  grandfatheredThrough: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  allowedIdentities:
    - name: Ada
      email: ada@example.test
  requireSignedCommits: true
  allowedSigners:
    - principal: ada@example.test
      key: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFSyHgjX4Y74rFN//IDMW2HBGkTMn5JF1Ls6VJr4pojt
`
	assertValues(t, configured, map[string]string{
		"tags": "2 tags", "contextIgnore": "2 patterns",
		"commitPolicy.grandfatheredThrough": "`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`",
		"commitPolicy.allowedIdentities":    "1 identities",
		"commitPolicy.requireSignedCommits": "true",
		"commitPolicy.allowedSigners":       "1 signers",
	})
}

func TestConfigReferenceListLayerStates(t *testing.T) {
	base := "prefix: example\nprofile: full\nintegrationBranch: main\n"
	for _, tc := range []struct {
		name, sidecar, want string
	}{
		{"catalog default", "", "catalog default"},
		{"explicit true is presence only", "dataDefaults:\n  testSurfaces: true\n", "catalog default; dataDefaults explicitly true"},
		{"layered project entries", "data:\n  testSurfaces:\n    - {name: Local, kind: unit, location: here}\n", "catalog default + project entries"},
		{"suppressed default", "dataDefaults:\n  testSurfaces: false\ndata:\n  testSurfaces: []\n", "explicitly suppressed default; project entries only"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string(nil)
			if tc.sidecar != "" {
				files = map[string]string{"skills/tdd.yaml": tc.sidecar}
			}
			_, p := syncedProject(t, base, files)
			model, err := configReferenceProject(p, testContext(t))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range model.DataKeys {
				if row.Artifact == "skill tdd" && row.Key == "testSurfaces" {
					found = true
					if !strings.Contains(row.State, tc.want) {
						t.Errorf("typed state = %q, want %q", row.State, tc.want)
					}
				}
			}
			if !found {
				t.Fatal("tdd data row missing")
			}
		})
	}

	_, glossaryProject := syncedProject(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    - {term: Local, meaning: ProjectState-specific term.}\n",
	})
	glossaryModel, err := configReferenceProject(glossaryProject, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	foundSpecialized := false
	for _, row := range glossaryModel.DataKeys {
		if row.Artifact == "doc glossary" && row.Key == "terms" {
			foundSpecialized = true
			if !strings.Contains(row.State, "project-only/specialized") {
				t.Errorf("specialized glossary state = %q", row.State)
			}
		}
	}
	if !foundSpecialized {
		t.Fatal("specialized glossary data row missing")
	}

	ref, err := StaticConfigReference()
	if err != nil {
		t.Fatal(err)
	}
	document, err := ConfigReferencePresentation("sidecar.data", &ref, "config reference")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"catalog-backed list", "null or a non-list value is invalid", "Project-only and specialized"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("shared CLI row missing %q:\n%s", want, out.String())
		}
	}
}

// invariant: config/configspec-and-reference:config-reference-regen-drift (TestConfigReferenceRegenDrift)
func TestConfigReferenceRegenDrift(t *testing.T) {
	root, p := syncedProject(t, crefYAML, nil)
	path := filepath.Join(root, "docs/config-reference.md")
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertDrift := func(kind string) {
		t.Helper()
		drift, err := checkProject(p, testContext(t))
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range drift {
			if finding.Path == "docs/config-reference.md" && finding.Kind == kind {
				return
			}
		}
		t.Fatalf("config-reference drift = %v, want %s", drift, kind)
	}
	assertDrift("stale")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	assertDrift("missing")
}

// The reference template consumes only dedicated data keys - a bare .vars or
// .data range would neutralize the consumption checks project-wide - and no
// partial references .vars, keeping the raw-source dormancy scan sound.
// invariant: config/configspec-and-reference:config-reference-no-bare-vars (TestConfigReferenceNoBareVars)
func TestConfigReferenceNoBareVars(t *testing.T) {
	_, p := syncedProject(t, crefYAML, nil)
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	cref, ok, err := generateConfigReference(renderInputsForTest(p), files, mustDeriveSkills(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a generated reference")
	}
	if render.ReferencesBareVars(cref.assembled) || render.ReferencesBareData(cref.assembled) {
		t.Error("the reference template must not reference .vars or .data bare")
	}
	// Include-expansion guard: the dormancy scan reads raw template bytes, so
	// no partial may reference .vars, dotted or bare.
	entries, err := fs.ReadDir(templates.FS, "partials")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := fs.ReadFile(templates.FS, "partials/"+e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if render.ReferencesBareVars(string(b)) || len(render.ReferencedVars(string(b))) > 0 {
			t.Errorf("partial %s references .vars - the raw-source dormancy scan would go blind", e.Name())
		}
	}
}

// The config-reference sidecar is sections-only: data: and paths: refuse
// at open, unknown section names refuse, a declared-section drop renders.
// invariant: config/configspec-and-reference:config-reference-data-rejected (TestConfigReferenceSidecarRules)
func TestConfigReferenceSidecarRules(t *testing.T) {
	for _, tc := range []struct {
		name, sidecar, wantErr string
	}{
		{"data rejected", "data:\n  k: v\n", "data: and dataDefaults: have no effect"},
		{"data defaults rejected", "dataDefaults:\n  k: false\n", "data: and dataDefaults: have no effect"},
		{"paths rejected", "paths:\n  - '**/*.go'\n", "read only from domain sidecars"},
		{"unknown section rejected", "sections:\n  intr:\n    drop: true\n", "intr"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFiles(t, crefYAML, map[string]string{"config-reference.yaml": tc.sidecar})
			if _, err := Open(testContext(t), root); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Open = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}

	root, _ := syncedProject(t, crefYAML, map[string]string{
		"config-reference.yaml": "sections:\n  intro:\n    drop: true\n",
	})
	b, err := os.ReadFile(filepath.Join(root, "docs/config-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "# Configuration Reference") {
		t.Error("dropped intro still rendered")
	}
	if !strings.Contains(string(b), "## config.yaml keys") {
		t.Error("generated tables must survive an intro drop")
	}
}

// Explicit audit values render without the default marker; explicit-empty
// lists render their accept-any/rule-off prose; a local-from-birth reference
// (never synced, no lock entry) reports nothing.
// A part-read fault at the reference's intro (a directory where the part file
// may sit) surfaces from every generation call site - the reference renders
// outside renderAllBase, so these branches are reachable, not theoretical.
func TestConfigReferencePartReadFault(t *testing.T) {
	root, p := syncedProject(t, crefYAML, nil)
	if err := os.MkdirAll(filepath.Join(root, ".awf/parts/config-reference/intro.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := checkProject(p, testContext(t)); err == nil {
		t.Error("Check should surface the part-read fault")
	}
	if err := syncProject(p); err == nil {
		t.Error("Sync should surface the part-read fault")
	}
	if _, err := advisoryNotesProject(p, testContext(t)); err == nil {
		t.Error("AdvisoryNotes should surface the part-read fault")
	}
	if _, err := plannedOutputsProject(p, testContext(t)); err == nil {
		t.Error("PlannedOutputs should surface the part-read fault")
	}
}

// A malformed config-reference sidecar fails at open like any other sidecar.
func TestConfigReferenceSidecarParseError(t *testing.T) {
	root := scaffoldFiles(t, crefYAML, map[string]string{"config-reference.yaml": "data: [unclosed\n"})
	if _, err := Open(testContext(t), root); err == nil || !strings.Contains(err.Error(), "config-reference.yaml") {
		t.Errorf("Open = %v, want a parse error naming the sidecar", err)
	}
}

// An intro convention part replaces the prose; the generated tables are
// unmarked template body and stay beyond a part's reach.
func TestConfigReferenceIntroOverride(t *testing.T) {
	root, _ := syncedProject(t, crefYAML, map[string]string{
		"parts/config-reference/intro.md": "# My Config Guide\n",
	})
	b, err := os.ReadFile(filepath.Join(root, "docs/config-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, "# My Config Guide") || !strings.Contains(got, "# Configuration Reference") {
		t.Errorf("intro part did not replace the body while retaining the owned heading:\n%s", got)
	}
	if !strings.Contains(got, "## config.yaml keys") || !strings.Contains(got, "## Vars") {
		t.Errorf("generated tables lost under an intro override:\n%s", got)
	}
}
