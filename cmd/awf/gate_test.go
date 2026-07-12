package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// gateFixture writes a .awf/ tree with a minimal config.yaml and a hand-written
// awf.lock carrying the given awfVersion and schemaVersion, returning the root.
// A negative schema means "write no lock at all".
func gateFixture(t *testing.T, awfVersion string, schema int) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: ex\n")
	if schema >= 0 {
		l := &manifest.Lock{AWFVersion: awfVersion, SchemaVersion: schema, Files: map[string]manifest.Entry{}}
		if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
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
	// A lock far above any real release is unambiguously newer than the test
	// binary (project.Version), so this stays correct across version bumps.
	root := gateFixture(t, "99.0.0", migrate.Current())
	err := gate(root)
	if err == nil {
		t.Fatal("expected gate error on behind version")
	}
	if !strings.Contains(err.Error(), "is behind this project (rendered by 99.0.0)") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGateAtOrAheadVersionPermitted(t *testing.T) {
	// project.Version is the equal boundary; "0.0.1" is below any real release.
	for _, v := range []string{project.Version, "0.0.1"} {
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

// TestNewGatesInHandler confirms runNew (GatedInHandler) surfaces the gate error
// itself — after name validation, not via the driver — on an ahead-schema project.
func TestNewGatesInHandler(t *testing.T) {
	root := gateFixture(t, "0.4.0", migrate.Current()+1)
	var out bytes.Buffer
	if err := runNew(root, "adr", []string{"x"}, &out); err == nil {
		t.Error("runNew: expected gate error on ahead schema")
	}
	if err := runNew(root, "plan", []string{"x"}, &out); err == nil {
		t.Error("runNew plan: expected gate error on ahead schema")
	}
	if err := runNew(root, "skill", []string{"x", "desc"}, &out); err == nil {
		t.Error("runNew skill: expected gate error on ahead schema")
	}
	if err := runNew(root, "doc", []string{"x", "desc"}, &out); err == nil {
		t.Error("runNew doc: expected gate error on ahead schema")
	}
}

// TestInitAndUpgradeGateBehindVersion pins that init and upgrade re-assert the
// binary-version gate their chained sync used to provide (removed from runSync in
// the parse-once refactor): a tree whose lock awfVersion is newer than the binary
// refuses rather than silently re-stamping a downgraded version.
func TestInitAndUpgradeGateBehindVersion(t *testing.T) {
	for _, cmd := range []string{"init", "upgrade"} {
		t.Run(cmd, func(t *testing.T) {
			root := scaffoldProject(t)
			lockPath := filepath.Join(root, ".awf", "awf.lock")
			l, err := manifest.Load(lockPath)
			if err != nil {
				t.Fatal(err)
			}
			l.AWFVersion = "99.0.0" // rendered by a newer awf → binary is behind
			if err := l.Save(lockPath); err != nil {
				t.Fatal(err)
			}
			var out, errb bytes.Buffer
			if code := runAt(t, root, []string{"awf", cmd}, &out, &errb); code != 1 {
				t.Fatalf("%s: expected exit 1 on a version-behind lock, got %d (%s)", cmd, code, errb.String())
			}
			if all := out.String() + errb.String(); !strings.Contains(all, "update your pinned awf") {
				t.Errorf("%s: expected the version-gate message, got: %s", cmd, all)
			}
			l2, err := manifest.Load(lockPath)
			if err != nil {
				t.Fatal(err)
			}
			if l2.AWFVersion != "99.0.0" {
				t.Errorf("%s: lock awfVersion re-stamped to %q despite the gate", cmd, l2.AWFVersion)
			}
		})
	}
}

// TestDriverGatesGatedCommands confirms the driver refuses every Gated command
// before its handler on an ahead-schema project. For enable/disable this also
// pins the gate-before-config-write guarantee: the handler never runs, so no
// half-mutated config is stranded.
func TestDriverGatesGatedCommands(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		args []string
	}{
		{"sync", []string{"awf", "sync"}},
		{"check", []string{"awf", "check"}},
		{"invariants", []string{"awf", "invariants"}},
		{"audit", []string{"awf", "audit"}},
		{"list", []string{"awf", "list"}},
		{"enable", []string{"awf", "enable", "skill", "tdd"}},
		{"disable", []string{"awf", "disable", "skill", "tdd"}},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			root := gateFixture(t, "0.4.0", migrate.Current()+1)
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			var out, errb bytes.Buffer
			if code := run(tc.args, &out, &errb); code != 1 {
				t.Errorf("%s: expected exit 1 on ahead schema, got %d (%s)", tc.cmd, code, errb.String())
			}
		})
	}
}
