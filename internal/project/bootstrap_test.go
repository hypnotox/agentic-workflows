package project

import (
	"strings"
	"testing"
)

// bootstrapFile renders a project with the given config and returns the
// awf-bootstrap.sh RenderedFile, or nil if none was produced.
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
	for i := range out {
		if out[i].Path == "awf-bootstrap.sh" {
			return &out[i]
		}
	}
	return nil
}

// invariant: bootstrap-pin
func TestBootstrapPinsRenderingVersion(t *testing.T) {
	rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	if rf == nil {
		t.Fatal("expected awf-bootstrap.sh to render when enabled")
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
		t.Fatal("expected awf-bootstrap.sh to render when enabled")
	}
	verify := strings.Index(rf.Content, "sha256sum -c")
	install := strings.Index(rf.Content, "install -m 0755")
	if verify < 0 {
		t.Fatalf("bootstrap missing sha256sum -c step:\n%s", rf.Content)
	}
	if install < 0 {
		t.Fatalf("bootstrap missing install step:\n%s", rf.Content)
	}
	if verify >= install {
		t.Errorf("checksum verify (index %d) must precede install (index %d)", verify, install)
	}
}

func TestBootstrapNotRenderedWhenDisabled(t *testing.T) {
	if rf := bootstrapFile(t, "prefix: example\n"); rf != nil {
		t.Errorf("expected no awf-bootstrap.sh when bootstrap absent, got %q", rf.Path)
	}
	if rf := bootstrapFile(t, "prefix: example\nbootstrap:\n  enabled: false\n"); rf != nil {
		t.Errorf("expected no awf-bootstrap.sh when bootstrap disabled, got %q", rf.Path)
	}
}
