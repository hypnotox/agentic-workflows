package project

import (
	"slices"
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

// invariant: bootstrap-env-override
func TestBootstrapEnvOverrideDefaultsToRenderingVersion(t *testing.T) {
	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/bootstrap.sh to render when enabled")
	}
	// Default-expansion form: a pre-set AWF_VERSION wins; absent one, the
	// rendering binary's version resolves (replaces the retired bootstrap-pin
	// literal-assignment form).
	want := `AWF_VERSION="${AWF_VERSION:-` + Version + `}"`
	if !strings.Contains(rf.Content, want) {
		t.Errorf("bootstrap missing env-override pin %q:\n%s", want, rf.Content)
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
	for _, cfg := range []string{"prefix: example\n", "prefix: example\nbootstrap:\n  enabled: false\n"} {
		if rf := bootstrapFile(t, cfg); rf != nil {
			t.Errorf("expected no bootstrap script when bootstrap off, got %q", rf.Path)
		}
		if rf := upgradeFile(t, cfg); rf != nil {
			t.Errorf("expected no upgrade script when bootstrap off, got %q", rf.Path)
		}
	}
}

// upgradeFile renders a project with the given config and returns the
// .awf/upgrade.sh RenderedFile, or nil if none was produced.
func upgradeFile(t *testing.T, configYAML string) *RenderedFile {
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
	for i := range out {
		if out[i].Path == ".awf/upgrade.sh" {
			return &out[i]
		}
	}
	return nil
}

// invariant: bootstrap-two-files
func TestBootstrapSingletonRendersBothScripts(t *testing.T) {
	root := scaffold(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	// Exactly two files render under the singleton — both directions: the
	// pair is present, and no third file joins the unit unnoticed.
	var unit []string
	for _, rf := range out {
		if strings.HasPrefix(rf.TemplateID, "bootstrap/") {
			unit = append(unit, rf.Path)
		}
	}
	slices.Sort(unit)
	if want := []string{".awf/bootstrap.sh", ".awf/upgrade.sh"}; !slices.Equal(unit, want) {
		t.Errorf("bootstrap unit renders %v, want exactly %v", unit, want)
	}
}

// invariant: upgrade-exec-final
func TestUpgradeScriptExecFinal(t *testing.T) {
	rf := upgradeFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/upgrade.sh to render when enabled")
	}
	// The exec must be the script's final statement: `awf upgrade` rewrites
	// this script truncate-in-place while it runs, and replacing the shell
	// process before the rewrite is what makes self-modification unreachable.
	last := ""
	for _, line := range strings.Split(rf.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		last = trimmed
	}
	if last != `exec "$binary" upgrade` {
		t.Errorf("final statement = %q, want the exec handoff", last)
	}
}

// invariant: upgrade-delegates-fetch
func TestUpgradeScriptDelegatesFetch(t *testing.T) {
	rf := upgradeFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected .awf/upgrade.sh to render when enabled")
	}
	if !strings.Contains(rf.Content, `AWF_VERSION="$target" bash .awf/bootstrap.sh`) {
		t.Errorf("upgrade script must fetch via the bootstrap with AWF_VERSION set:\n%s", rf.Content)
	}
	// The latest-tag redirect probe is the script's only direct network call:
	// no release-asset download and no checksum invocation of its own.
	if n := strings.Count(rf.Content, "curl "); n != 1 {
		t.Errorf("upgrade script has %d curl invocations, want exactly the latest-tag probe", n)
	}
	if !strings.Contains(rf.Content, "releases/latest") {
		t.Errorf("upgrade script's curl must target releases/latest:\n%s", rf.Content)
	}
	for _, banned := range []string{"releases/download", "sha256sum", "shasum"} {
		if strings.Contains(rf.Content, banned) {
			t.Errorf("upgrade script must not carry its own fetch/verify logic, found %q", banned)
		}
	}
	// An empty target must fail loudly before the fetch: the bootstrap's
	// default expansion would otherwise silently re-fetch the current pin.
	if !strings.Contains(rf.Content, `[ -n "${target}" ] ||`) {
		t.Errorf("upgrade script missing the empty-target guard:\n%s", rf.Content)
	}
}
