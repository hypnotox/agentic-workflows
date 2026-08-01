package gitfixture

import (
	"os"
	"strings"
	"testing"
)

// nativeIsolationPins is the whole policy the native lane guarantees, written
// out independently of the code that builds it so the table asserts a contract
// rather than restating an implementation.
//
// This exists because the lane's isolation is a deliberate DUPLICATE of the one
// in internal/git: tooling/quality-gates:testsupport-zero-internal-deps forbids
// gitfixture importing that package, so no compiler edge ties the two copies
// together, awf check reads no Go symbols, and deadcode skips test packages.
// Nothing but this table would notice the copies diverging.
//
// One divergence is deliberate and must NOT be added here: the seam replays the
// developer's core.excludesFile, because it renders a working-tree oracle whose
// ignore universe has to match reality. A fixture only builds state, so it is
// correct for it to be stricter.
var nativeIsolationPins = map[string]string{
	"GIT_CONFIG_GLOBAL":   os.DevNull,
	"GIT_CONFIG_NOSYSTEM": "1",
	"GIT_TERMINAL_PROMPT": "0",
	"GIT_ASKPASS":         "true",
	"SSH_ASKPASS":         "true",
	"GCM_INTERACTIVE":     "Never",
}

// effectiveEnvironment reduces an environment slice the way a process does:
// the last assignment of a key wins.
func effectiveEnvironment(env []string) map[string]string {
	out := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		out[key] = value
	}
	return out
}

// TestNativeEnvironmentPinsTheWholeIsolationPolicy is the proof carrier for the
// fixture lane's parity with the seam's isolation. It must fail if any single
// pin is dropped or if the strip stops covering a hostile variable, so it is
// mutation-verified one pin at a time rather than trusted for passing.
// invariant: tooling/git-access:fixture-isolation-parity (TestNativeEnvironmentPinsTheWholeIsolationPolicy)
func TestNativeEnvironmentPinsTheWholeIsolationPolicy(t *testing.T) {
	t.Parallel()
	// Every variable here would redirect, reconfigure, or unblock Git if it
	// survived: a repository selection, a configuration source, or a credential
	// helper that can hang a fixture on a prompt.
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
		// to run, and stripping HOME would change where a fixture writes.
		"PATH=/usr/bin",
		"HOME=/home/developer",
	}

	effective := effectiveEnvironment(nativeEnvironment(hostile))

	for key, want := range nativeIsolationPins {
		got, ok := effective[key]
		if !ok {
			t.Errorf("%s is not pinned; the lane no longer isolates it", key)
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
	for key, value := range nativeIsolationPins {
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
	if len(nativeIsolationPins) != 6 {
		t.Errorf("the pinned set has %d entries, want 6; a new pin needs a case here and in the seam's own isolation", len(nativeIsolationPins))
	}
}
