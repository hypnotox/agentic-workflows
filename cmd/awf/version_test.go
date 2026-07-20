package main

import (
	"bytes"
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
	if !strings.Contains(out.String(), "awf ") || !strings.Contains(out.String(), project.Version) {
		t.Errorf("version output = %q, want it to contain %q", out.String(), project.Version)
	}
}

func TestAwfVersionSingleAuthority(t *testing.T) {
	// invariant: tooling/cli:single-version-authority
	if got := awfVersion(); got != project.Version {
		t.Errorf("awfVersion() = %q, want project.Version %q", got, project.Version)
	}
}

func TestVersionLine(t *testing.T) {
	if got, want := versionLine(nil, false), "awf "+project.Version; got != want {
		t.Errorf("versionLine(no build info) = %q, want %q", got, want)
	}
	if got, want := versionLine(&debug.BuildInfo{}, true), "awf "+project.Version; got != want {
		t.Errorf("versionLine(empty provenance) = %q, want %q", got, want)
	}
	info := debug.BuildInfo{Main: debug.Module{Version: "v9.9.9-pre"}}
	if got, want := versionLine(&info, true), "awf "+project.Version+" (v9.9.9-pre)"; got != want {
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
