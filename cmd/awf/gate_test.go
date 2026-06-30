package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// gateFixture writes a .awf/ tree with a minimal config.yaml and a hand-written
// awf.lock carrying the given awfVersion and schemaVersion, returning the root.
// A negative schema means "write no lock at all".
func gateFixture(t *testing.T, awfVersion string, schema int) string {
	t.Helper()
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte("prefix: ex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if schema >= 0 {
		l := &manifest.Lock{AWFVersion: awfVersion, SchemaVersion: schema, Files: map[string]manifest.Entry{}}
		if err := l.Save(filepath.Join(awf, "awf.lock")); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGateAheadSchemaErrors(t *testing.T) {
	root := gateFixture(t, "0.4.0", migrate.Current()+1)
	err := gate(root)
	if err == nil {
		t.Fatal("expected gate error on ahead schema")
	}
	if !strings.Contains(err.Error(), "update your pinned awf") || !strings.Contains(err.Error(), "schema generation") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGateBehindVersionErrors(t *testing.T) {
	root := gateFixture(t, "0.5.0", migrate.Current())
	err := gate(root)
	if err == nil {
		t.Fatal("expected gate error on behind version")
	}
	if !strings.Contains(err.Error(), "is behind this project (rendered by 0.5.0)") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGateAtOrAheadVersionPermitted(t *testing.T) {
	for _, v := range []string{"0.4.0", "0.3.0"} {
		root := gateFixture(t, v, migrate.Current())
		if err := gate(root); err != nil {
			t.Errorf("gate(%s) = %v, want nil", v, err)
		}
	}
}

func TestGateSkipNoLock(t *testing.T) {
	root := gateFixture(t, "", -1)
	if err := gate(root); err != nil {
		t.Errorf("gate (no lock) = %v, want nil", err)
	}
}

func TestGateSkipEmptyVersion(t *testing.T) {
	root := gateFixture(t, "", migrate.Current())
	if err := gate(root); err != nil {
		t.Errorf("gate (empty version) = %v, want nil", err)
	}
}

func TestGateSkipUnparseableVersion(t *testing.T) {
	root := gateFixture(t, "garbage", migrate.Current())
	if err := gate(root); err != nil {
		t.Errorf("gate (unparseable version) = %v, want nil", err)
	}
}

func TestNormalizeSemver(t *testing.T) {
	for _, in := range []string{"v0.4.0", "0.4.0"} {
		got, ok := normalizeSemver(in)
		if !ok || got != "v0.4.0" {
			t.Errorf("normalizeSemver(%q) = (%q, %v), want (v0.4.0, true)", in, got, ok)
		}
	}
	if got, ok := normalizeSemver("vv0.4.0"); ok || got != "" {
		t.Errorf("normalizeSemver(vv0.4.0) = (%q, %v), want (\"\", false)", got, ok)
	}
}

// TestGatedCommandsRejectAheadSchema confirms runInvariants/runAudit/runList each
// surface the gate error on an ahead-schema project (the inserted gate guard).
func TestGatedCommandsRejectAheadSchema(t *testing.T) {
	root := gateFixture(t, "0.4.0", migrate.Current()+1)
	var out bytes.Buffer
	if err := runInvariants(root, &out); err == nil {
		t.Error("runInvariants: expected gate error on ahead schema")
	}
	if err := runAudit(root, "", &out); err == nil {
		t.Error("runAudit: expected gate error on ahead schema")
	}
	if err := runList(root, "", &out); err == nil {
		t.Error("runList: expected gate error on ahead schema")
	}
}
