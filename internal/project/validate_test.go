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
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatalf("init must stay exempt from command-wiring validation, got: %v", err)
	}
	_, _, _, syncErr = p.SyncReport()
	_, checkErr = p.Check()
	return syncErr, checkErr
}

// Sync and check refuse an enabled hooks singleton whose rendered payloads
// could not resolve their commands: a missing project gate command, and - with
// the runner singleton disabled - a missing hook-referenced awf-verb var,
// checked in a fixed order that names the exact var to set. A resolvable
// wiring (gateCmd plus either the runner or the three vars) and a
// hooks-disabled config both stay valid, and first-adoption init never runs
// the rule.
// invariant: config/validation:hooks-commands-resolvable
func TestValidateCommandWiring(t *testing.T) {
	fire := []struct {
		name, config, want string
	}{
		{
			"gateCmd unset",
			"prefix: example\nhooks:\n  enabled: true\nrunner:\n  enabled: true\n",
			"hooks.enabled requires vars.gateCmd: the rendered hook payloads run the project gate; set vars.gateCmd in .awf/config.yaml",
		},
		{
			"runner disabled, checkCmd first",
			"prefix: example\nvars:\n  gateCmd: make gate\nhooks:\n  enabled: true\n",
			"hooks.enabled without the runner singleton requires vars.checkCmd: set it in .awf/config.yaml or enable the runner (awf enable runner)",
		},
		{
			"runner disabled, commitGateCmd second",
			"prefix: example\nvars:\n  gateCmd: make gate\n  checkCmd: make check\nhooks:\n  enabled: true\nrunner:\n  enabled: false\n",
			"hooks.enabled without the runner singleton requires vars.commitGateCmd: set it in .awf/config.yaml or enable the runner (awf enable runner)",
		},
		{
			"runner disabled, proseGateCmd third",
			"prefix: example\nvars:\n  gateCmd: make gate\n  checkCmd: make check\n  commitGateCmd: make commit-gate\nhooks:\n  enabled: true\n",
			"hooks.enabled without the runner singleton requires vars.proseGateCmd: set it in .awf/config.yaml or enable the runner (awf enable runner)",
		},
	}
	for _, tc := range fire {
		t.Run(tc.name, func(t *testing.T) {
			syncErr, checkErr := commandWiringErrs(t, tc.config)
			if syncErr == nil || syncErr.Error() != tc.want {
				t.Errorf("sync error = %v, want %q", syncErr, tc.want)
			}
			if checkErr == nil || checkErr.Error() != tc.want {
				t.Errorf("check error = %v, want %q", checkErr, tc.want)
			}
		})
	}

	valid := []struct{ name, config string }{
		{
			"runner satisfies the awf-verb vars",
			"prefix: example\nvars:\n  gateCmd: make gate\nhooks:\n  enabled: true\nrunner:\n  enabled: true\n",
		},
		{
			"explicit vars satisfy a runner-less config",
			"prefix: example\nvars:\n  gateCmd: make gate\n  checkCmd: make check\n  commitGateCmd: make commit-gate\n  proseGateCmd: make prose-gate\nhooks:\n  enabled: true\n",
		},
		{
			"hooks disabled needs nothing",
			"prefix: example\nhooks:\n  enabled: false\n",
		},
		{
			"hooks absent needs nothing",
			"prefix: example\n",
		},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			syncErr, checkErr := commandWiringErrs(t, tc.config)
			if syncErr != nil {
				t.Errorf("sync error = %v, want none", syncErr)
			}
			if checkErr != nil {
				t.Errorf("check error = %v, want none", checkErr)
			}
		})
	}
}
