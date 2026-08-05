package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func TestRunVersion(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "version"}, &out, &errb); code != 0 {
		t.Fatalf("version exited %d: %s", code, errb.String())
	}
	want := "version: " + project.Version + "\n"
	if out.String() != want || errb.Len() != 0 {
		t.Errorf("version streams stdout=%q stderr=%q, want stdout=%q", out.String(), errb.String(), want)
	}
}

// TestReleaseConsumerParsesOnlyLabeledVersionContract runs the parser embedded
// in the release workflow against controlled version-command streams. This
// rejects disconnected parser text and permissive positional parsing.
func TestReleaseConsumerParsesOnlyLabeledVersionContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(body), "\n")
	var script []string
	inStep := false
	for _, line := range lines {
		if line == "      - name: Verify tag matches project.Version" {
			inStep = true
			continue
		}
		if inStep && strings.HasPrefix(line, "      - name: ") {
			break
		}
		if inStep && strings.HasPrefix(line, "          ") {
			script = append(script, strings.TrimPrefix(line, "          "))
		}
	}
	if len(script) == 0 {
		t.Fatal("release parser step not found")
	}
	bin := t.TempDir()
	goPath := filepath.Join(bin, "go")
	if err := os.WriteFile(goPath, []byte("#!/bin/sh\nprintf '%s' \"$AWF_VERSION_FIXTURE\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, fixture string
		valid         bool
	}{
		{"labeled", "version: 1.2.3\n", true},
		{"missing", "", false},
		{"duplicate", "version: 1.2.3\nversion: 1.2.3\n", false},
		{"malformed", "version: 1.2.3 trailing\n", false},
		{"legacy-unlabeled", "1.2.3\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", strings.Join(script, "\n"))
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GITHUB_REF_NAME=v1.2.3", "AWF_VERSION_FIXTURE="+tc.fixture)
			err := cmd.Run()
			if (err == nil) != tc.valid {
				t.Fatalf("release parser fixture %q: err=%v", tc.fixture, err)
			}
		})
	}
}

func TestWriteVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"version only", nil, false, "version: " + project.Version + "\n"},
		{"with provenance", &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9-pre"}}, true, "version: " + project.Version + " (v9.9.9-pre)\n"},
		{"line-broken provenance normalized", &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9-pre\ninjected"}}, true, "version: " + project.Version + " (v9.9.9-pre injected)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeVersion(&out, tc.info, tc.ok); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != tc.want {
				t.Errorf("writeVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

type failingVersionWriter struct{ err error }

func (w failingVersionWriter) Write([]byte) (int, error) { return 0, w.err }

func TestVersionRenderFailureUsesCommandBoundary(t *testing.T) {
	cause := errors.New("version destination failed")
	var stderr bytes.Buffer
	if code := run([]string{"awf", "version"}, failingVersionWriter{err: cause}, &stderr); code == 0 {
		t.Fatal("version render failure exited zero")
	}
	if got, want := stderr.String(), "condition: awf: write presentation: version destination failed\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestAwfVersionSingleAuthority(t *testing.T) {
	// invariant: tooling/cli:single-version-authority (TestAwfVersionSingleAuthority)
	if got := awfVersion(); got != project.Version {
		t.Errorf("awfVersion() = %q, want project.Version %q", got, project.Version)
	}
}

func TestVersionLine(t *testing.T) {
	if got, want := versionLine(nil, false), project.Version; got != want {
		t.Errorf("versionLine(no build info) = %q, want %q", got, want)
	}
	if got, want := versionLine(&debug.BuildInfo{}, true), project.Version; got != want {
		t.Errorf("versionLine(empty provenance) = %q, want %q", got, want)
	}
	info := debug.BuildInfo{Main: debug.Module{Version: "v9.9.9-pre"}}
	if got, want := versionLine(&info, true), project.Version+" (v9.9.9-pre)"; got != want {
		t.Errorf("versionLine(with provenance) = %q, want %q", got, want)
	}
}

func TestFormatProvenance(t *testing.T) {
	long := "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"empty", debug.BuildInfo{}, ""},
		{"devel skipped", debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ""},
		{"const echo skipped", debug.BuildInfo{Main: debug.Module{Version: "v" + project.Version}}, ""},
		{"pseudo version kept", debug.BuildInfo{Main: debug.Module{Version: "v9.9.9-pre"}}, "v9.9.9-pre"},
		{"revision truncated", debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: long}},
		}, "rev 0123456789ab"},
		{"short revision kept", debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
		}, "rev abc123"},
		{"both joined", debug.BuildInfo{
			Main:     debug.Module{Version: "v9.9.9-pre"},
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
		}, "v9.9.9-pre, rev abc123"},
		{"whitespace normalized", debug.BuildInfo{
			Main:     debug.Module{Version: "v9.9.9-pre\ninjected"},
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc\t123"}},
		}, "v9.9.9-pre injected, rev abc 123"},
		{"empty revision skipped", debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: ""}},
		}, ""},
	}
	for _, c := range cases {
		if got := formatProvenance(&c.info); got != c.want {
			t.Errorf("%s: formatProvenance() = %q, want %q", c.name, got, c.want)
		}
	}
}
