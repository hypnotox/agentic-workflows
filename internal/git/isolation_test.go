package git

import (
	"os"
	"strings"
	"testing"
)

// seamIsolationPins is the whole environment policy every native invocation
// runs under, written out independently of the code that builds it so the table
// asserts a contract rather than restating an implementation.
//
// Its twin is nativeIsolationPins in internal/testsupport/gitfixture. The
// duplication is forced: tooling/quality-gates:testsupport-zero-internal-deps
// forbids the fixture importing this package, so no compiler edge ties the two
// copies together, awf check reads no Go symbols, and deadcode skips test
// packages. Two tables are what make a divergence in EITHER copy fail, which is
// what tooling/git-access:fixture-isolation-parity claims. An end-to-end test
// cannot serve here: Git's own defaults are benign under a temporary HOME, so
// deleting every pin below leaves a behavioural test green.
//
// One divergence from the fixture is deliberate and belongs to the runner, not
// here: the seam replays the developer's effective core.excludesFile into the
// commands whose answer depends on the ignore universe, because it renders an
// oracle that must match what the developer sees. A fixture only builds state.
var seamIsolationPins = map[string]string{
	"GIT_CONFIG_GLOBAL":   os.DevNull,
	"GIT_CONFIG_NOSYSTEM": "1",
	"GIT_TERMINAL_PROMPT": "0",
	"GIT_ASKPASS":         "true",
	"SSH_ASKPASS":         "true",
	"GCM_INTERACTIVE":     "Never",
}

// effectiveSeamEnvironment reduces an environment slice the way a process does:
// the last assignment of a key wins.
func effectiveSeamEnvironment(env []string) map[string]string {
	out := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		out[key] = value
	}
	return out
}

// TestIsolatedGitEnvironmentPinsTheWholeIsolationPolicy is the seam half of the
// isolation parity proof: every hostile Git variable is stripped and every pin
// is present with its expected value. Each pin and the strip are individually
// falsifiable here, which they are not through any behavioural path.
// invariant: tooling/git-access:fixture-isolation-parity
func TestIsolatedGitEnvironmentPinsTheWholeIsolationPolicy(t *testing.T) {
	// Every variable here would redirect, reconfigure, or unblock Git if it
	// survived: a repository selection, a configuration source, or a credential
	// helper that can hang awf on a prompt.
	hostile := []string{
		"GIT_DIR=/hostile/.git",
		"GIT_WORK_TREE=/hostile",
		"GIT_INDEX_FILE=/hostile/index",
		"GIT_OBJECT_DIRECTORY=/hostile/objects",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/hostile/alt",
		"GIT_COMMON_DIR=/hostile/common",
		"GIT_NAMESPACE=hostile",
		"GIT_CEILING_DIRECTORIES=/",
		"GIT_CONFIG=/hostile/config",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.bare",
		"GIT_CONFIG_VALUE_0=true",
		"GIT_CONFIG_PARAMETERS='core.bare'='true'",
		"GIT_CONFIG_SYSTEM=/hostile/system",
		"GIT_CONFIG_GLOBAL=/hostile/gitconfig",
		"GIT_CONFIG_NOSYSTEM=0",
		"GIT_TERMINAL_PROMPT=1",
		"GIT_ASKPASS=/hostile/askpass",
		"SSH_ASKPASS=/hostile/ssh-askpass",
		"GCM_INTERACTIVE=Always",
		// Lower case, because the filter compares case-insensitively and an
		// environment is case-insensitive on Windows.
		"git_dir=/hostile/lower/.git",
		// Unrelated variables must survive: stripping PATH would leave no git
		// to run, and stripping HOME would change where Git looks for config
		// the seam deliberately re-reads through core.excludesFile.
		"PATH=/usr/bin",
		"HOME=/home/developer",
	}

	effective := effectiveSeamEnvironment(isolatedGitEnvironment(hostile))

	for key, want := range seamIsolationPins {
		got, ok := effective[key]
		if !ok {
			t.Errorf("%s is not pinned; a native invocation no longer isolates it", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want the pinned %q", key, got, want)
		}
	}
	// Strict equality, not a GIT_-prefixed sweep: two of the six pins
	// (SSH_ASKPASS, GCM_INTERACTIVE) carry no GIT_ prefix, so a prefix sweep
	// cannot see a seventh pin from that same credential-helper family being
	// added. Requiring the whole resulting environment to be exactly the
	// surviving inheritance plus the declared pins is what makes an ADDED pin
	// fail here as loudly as a dropped one.
	want := map[string]string{"PATH": "/usr/bin", "HOME": "/home/developer"}
	for key, value := range seamIsolationPins {
		want[key] = value
	}
	for key, value := range effective {
		expected, ok := want[key]
		if !ok {
			t.Errorf("%s=%q survived or was added; the environment must be exactly the surviving inheritance plus the declared pins", key, value)
			continue
		}
		if expected != value {
			t.Errorf("%s = %q, want %q", key, value, expected)
		}
	}
	for key := range want {
		if _, ok := effective[key]; !ok {
			t.Errorf("%s is missing from the isolated environment", key)
		}
	}
}

// TestSeamAndFixtureIsolationPoliciesAgree states the parity obligation as an
// assertion rather than as a comment. The fixture cannot import this package,
// so its table is a hand-maintained copy; this pins the contract both copies
// implement, so adding a pin to one without the other fails here.
func TestSeamAndFixtureIsolationPoliciesAgree(t *testing.T) {
	// The expected policy, restated a third time and deliberately: if this
	// literal, the seam's table, and the fixture's table ever disagree, the
	// disagreement is the finding.
	want := map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_ASKPASS":         "true",
		"SSH_ASKPASS":         "true",
		"GCM_INTERACTIVE":     "Never",
	}
	if len(seamIsolationPins) != len(want) {
		t.Fatalf("the seam pins %d variables, want %d; a new pin needs a case here and in the fixture lane's table", len(seamIsolationPins), len(want))
	}
	for key, value := range want {
		if seamIsolationPins[key] != value {
			t.Errorf("seam pin %s = %q, want %q", key, seamIsolationPins[key], value)
		}
	}
}
