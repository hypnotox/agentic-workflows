package project

import (
	"testing"
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
