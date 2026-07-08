package project

import (
	"strings"
	"testing"
)

// bootstrapFile renders a project with the given config and returns the
// .awf/bootstrap.sh RenderedFile, or nil if none was produced. It also asserts
// no output lands at the retired repo-root path (ADR-0047).
// invariant: bootstrap-config-tree-path
func bootstrapFile(t *testing.T, configYAML string) *RenderedFile {
	t.Helper()
	root := scaffold(t, configYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var found *RenderedFile
	for i := range out {
		if out[i].Path == "awf-bootstrap.sh" {
			t.Errorf("output at retired root path awf-bootstrap.sh (relocated by ADR-0047)")
		}
		if out[i].Path == ".awf/bootstrap.sh" {
			found = &out[i]
		}
	}
	return found
}

// invariant: bootstrap-pin
func TestBootstrapPinsRenderingVersion(t *testing.T) {
	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
	}
	want := `AWF_VERSION="` + Version + `"`
	if !strings.Contains(rf.Content, want) {
		t.Errorf("bootstrap missing pin %q:\n%s", want, rf.Content)
	}
	// The banner is a #-comment after the shebang, keeping the script executable.
	lines := strings.Split(rf.Content, "\n")
	if lines[0] != "#!/usr/bin/env bash" {
		t.Errorf("first line = %q, want the shebang", lines[0])
	}
	if !strings.HasPrefix(lines[1], "# ") || strings.HasPrefix(lines[1], "<!--") {
		t.Errorf("second line = %q, want a #-comment banner", lines[1])
	}
}

// invariant: bootstrap-checksum
func TestBootstrapVerifiesBeforeInstall(t *testing.T) {
	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
	}
	install := strings.Index(rf.Content, "install -m 0755")
	if install < 0 {
		t.Fatalf("bootstrap missing install step:\n%s", rf.Content)
	}
	for _, verify := range []string{"sha256sum -c - >&2", "shasum -a 256 -c - >&2"} {
		idx := strings.Index(rf.Content, verify)
		if idx < 0 {
			t.Fatalf("bootstrap missing checksum branch %q:\n%s", verify, rf.Content)
		}
		if idx >= install {
			t.Errorf("checksum verify %q (index %d) must precede install (index %d)", verify, idx, install)
		}
	}
}

// invariant: bootstrap-stdout-path-only
func TestBootstrapStdoutPathOnly(t *testing.T) {
	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
	}
	for _, line := range strings.Split(rf.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "echo ") {
			continue
		}
		if trimmed == `echo "$binary"` || strings.Contains(trimmed, ">&2") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		t.Errorf("stdout-polluting line in bootstrap: %q (only the binary path may print to stdout)", trimmed)
	}
}

// invariant: bootstrap-local-first
func TestBootstrapLocalFirstResolution(t *testing.T) {
	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
	}
	probe := strings.Index(rf.Content, "command -v awf")
	download := strings.Index(rf.Content, "curl ")
	if probe < 0 {
		t.Fatalf("bootstrap missing the local PATH probe:\n%s", rf.Content)
	}
	if download >= 0 && probe >= download {
		t.Errorf("local probe (index %d) must precede the download (index %d)", probe, download)
	}
	if !strings.Contains(rf.Content, `[ "${local_version}" = "${AWF_VERSION}" ]`) {
		t.Errorf("local probe must require an exact pinned-version match:\n%s", rf.Content)
	}
}

// TestBootstrapUnsupportedPlatformPointsAtManualInstall pins the pointer added
// for Windows/git-bash users: both unsupported-platform failures must name the
// manual-install path (README → Install) on their stderr line.
func TestBootstrapUnsupportedPlatformPointsAtManualInstall(t *testing.T) {
	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
	}
	for _, branch := range []string{"unsupported arch", "unsupported os"} {
		line := ""
		for _, ln := range strings.Split(rf.Content, "\n") {
			if strings.Contains(ln, branch) {
				line = ln
				break
			}
		}
		if line == "" {
			t.Errorf("bootstrap missing the %q branch", branch)
			continue
		}
		if !strings.Contains(line, "#install") {
			t.Errorf("%q failure must point at the manual-install path: %q", branch, line)
		}
	}
}

func TestBootstrapNotRenderedWhenDisabled(t *testing.T) {
	if rf := bootstrapFile(t, "prefix: example\n"); rf != nil {
		t.Errorf("expected no bootstrap script when bootstrap absent, got %q", rf.Path)
	}
	if rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: false\n"); rf != nil {
		t.Errorf("expected no bootstrap script when bootstrap disabled, got %q", rf.Path)
	}
}
