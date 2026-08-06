package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// commandWiringErrs initializes a fresh tree with the given config - first
// adoption is exempt from the command-wiring validation, so the init must
// succeed even for a config sync would refuse - then returns the SyncReport
// and Check errors for that same config.
func commandWiringErrs(t *testing.T, configYAML string) (syncErr, checkErr error) {
	t.Helper()
	root := scaffold(t, configYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatalf("init must stay exempt from command-wiring validation, got: %v", err)
	}
	_, _, _, syncErr = p.SyncReport(testContext(t))
	_, checkErr = p.Check(testContext(t))
	return syncErr, checkErr
}

// invariant: config/configuration:sidecar-data-defaults-control (TestCatalogListSidecarValidation)
func TestCatalogListSidecarValidation(t *testing.T) {
	base := "prefix: example\nintegrationBranch: main\nskills: [tdd]\ndocs: [glossary]\n"
	for _, tc := range []struct {
		name, path, sidecar, want string
	}{
		{"valid true and empty list", "skills/tdd.yaml", "dataDefaults:\n  testSurfaces: true\ndata:\n  testSurfaces: []\n", ""},
		{"valid false without authored list", "skills/tdd.yaml", "dataDefaults:\n  testSurfaces: false\n", ""},
		{"unknown suppression key", "skills/tdd.yaml", "dataDefaults:\n  missing: false\n", "skills/tdd.yaml dataDefaults.missing"},
		{"non-boolean suppression value", "skills/tdd.yaml", "dataDefaults:\n  testSurfaces: wrong\n", "cannot unmarshal"},
		{"catalog list null", "skills/tdd.yaml", "data:\n  testSurfaces:\n", "skills/tdd.yaml data.testSurfaces"},
		{"catalog list scalar", "skills/tdd.yaml", "data:\n  testSurfaces: wrong\n", "skills/tdd.yaml data.testSurfaces"},
		{"differently keyed specialized glossary value", "docs/glossary.yaml", "dataDefaults:\n  terms: false\n", "docs/glossary.yaml dataDefaults.terms"},
		{"specialized glossary catalog key", "docs/glossary.yaml", "dataDefaults:\n  standardTerms: false\n", "docs/glossary.yaml dataDefaults.standardTerms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFiles(t, base, map[string]string{tc.path: tc.sidecar})
			_, err := Open(testContext(t), root)
			if tc.want == "" && err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("Open error = %v, want containing %q", err, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		name, configYAML, path, sidecar, want string
	}{
		{"agents-doc singleton", "prefix: example\nintegrationBranch: main\n", "agents-doc.yaml", "dataDefaults:\n  any: false\n", "agents-doc.yaml dataDefaults.any"},
		{"plain singleton", "prefix: example\nintegrationBranch: main\n", "adr-readme.yaml", "dataDefaults:\n  any: false\n", "adr-readme.yaml dataDefaults.any"},
		{"local-only artifact", "prefix: example\nintegrationBranch: main\nskills: [custom]\n", "skills/custom.yaml", "local: true\ndataDefaults:\n  testSurfaces: false\n", "local-only artifact"},
		{"domain sidecar", "prefix: example\nintegrationBranch: main\ndomains: [config]\n", "domains/config.yaml", "dataDefaults:\n  any: false\n", "domain sidecar is paths-only"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFiles(t, tc.configYAML, map[string]string{tc.path: tc.sidecar})
			_, err := Open(testContext(t), root)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Open error = %v, want containing %q", err, tc.want)
			}
		})
	}

	if got := catalogData(nil, "unknown", "artifact"); got != nil {
		t.Fatalf("unknown catalog data = %v, want nil", got)
	}
	if err := validateCatalogListData("skills/example.yaml", config.Sidecar{DataDefaults: map[string]bool{"scalar": false}}, map[string]any{"scalar": "default"}); err == nil || !strings.Contains(err.Error(), "dataDefaults.scalar") {
		t.Fatalf("catalog non-list suppression error = %v", err)
	}
}

// Sync and check refuse an enabled hooks singleton whose rendered payloads
// could not resolve their commands: a missing project gate command, and - with
// the runner singleton disabled - a missing hook-referenced awf-verb var,
// checked in a fixed order that names the exact var to set. A resolvable
// wiring (gateCmd plus either the runner or the three vars) and a
// hooks-disabled config both stay valid, and first-adoption init never runs
// the rule.
// invariant: config/validation:hooks-commands-resolvable (TestValidateCommandWiring)
func TestValidateCommandWiring(t *testing.T) {
	fixtures := []struct {
		name, config, want string
	}{
		{
			"gateCmd unset",
			"prefix: example\nintegrationBranch: main\nhooks:\n  enabled: true\nrunner:\n  enabled: true\n",
			"hooks.enabled requires vars.gateCmd: the rendered hook payloads run the project gate; set vars.gateCmd in .awf/config.yaml",
		},
		{
			"runner disabled, checkCmd first",
			"prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\nhooks:\n  enabled: true\n",
			"hooks.enabled without the runner singleton requires vars.checkCmd: set it in .awf/config.yaml or enable the runner (awf enable runner)",
		},
		{
			"runner disabled, commitGateCmd second",
			"prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n  checkCmd: make check\nhooks:\n  enabled: true\nrunner:\n  enabled: false\n",
			"hooks.enabled without the runner singleton requires vars.commitGateCmd: set it in .awf/config.yaml or enable the runner (awf enable runner)",
		},

		{
			"runner satisfies the awf-verb vars",
			"prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\nhooks:\n  enabled: true\nrunner:\n  enabled: true\n", "",
		},
		{
			"explicit vars satisfy a runner-less config",
			"prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n  checkCmd: make check\n  commitGateCmd: make commit-gate\nhooks:\n  enabled: true\n", "",
		},
		{
			"hooks disabled needs nothing",
			"prefix: example\nintegrationBranch: main\nhooks:\n  enabled: false\n", "",
		},
		{
			"hooks absent needs nothing",
			"prefix: example\nintegrationBranch: main\n", "",
		},
	}
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			syncErr, checkErr := commandWiringErrs(t, tc.config)
			if tc.want == "" {
				if syncErr != nil || checkErr != nil {
					t.Errorf("errors = %v, %v; want none", syncErr, checkErr)
				}
				return
			}
			if syncErr == nil || syncErr.Error() != tc.want || checkErr == nil || checkErr.Error() != tc.want {
				t.Errorf("errors = %v, %v; want %q", syncErr, checkErr, tc.want)
			}
		})
	}
}
