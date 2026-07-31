package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
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

func TestGateStagedLoadErrors(t *testing.T) {
	nonRepo := t.TempDir()
	if err := gateStaged(nonRepo); err == nil {
		t.Fatal("gateStaged accepted a non-repository")
	}
	if _, _, _, err := checkLockVsBinary(nonRepo, true); err == nil {
		t.Fatal("staged ahead-note loader accepted a non-repository")
	}
	repo, dir := gitfixture.InitRepo(t)
	if _, err := stagedLock(dir); err == nil || !strings.Contains(err.Error(), "no staged .awf/awf.lock") {
		t.Fatalf("missing staged lock error = %v", err)
	}
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/awf.lock": "{not json"})
	if err := gateStaged(dir); err == nil || !strings.Contains(err.Error(), "parse staged lock") {
		t.Fatalf("gateStaged malformed lock error = %v", err)
	}
}

func TestGateCorruptLockError(t *testing.T) {
	root := gateFixture(t, "0.4.0", migrate.Current())
	if err := os.WriteFile(config.LockPath(root), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gate(root); err == nil {
		t.Fatal("expected gate lock error")
	}
}

// invariant: tooling/cli:version-compat-gate
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
	// invariant: tooling/cli:version-compat-gate
	if err == nil {
		t.Fatal("expected gate error on behind version")
	}
	if !strings.Contains(err.Error(), "is behind this project (rendered by 99.0.0)") {
		t.Errorf("unexpected error: %v", err)
	}
}

// invariant: tooling/cli:version-compat-gate
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
// itself - after name validation, not via the driver - on an ahead-schema project.
func TestNewGatesInHandler(t *testing.T) {
	root := gateFixture(t, "0.4.0", migrate.Current()+1)
	var out bytes.Buffer
	// invariant: tooling/cli:adr-new-version-gated
	if err := runNew(root, "adr", []string{"x"}, &out); err == nil {
		t.Error("runNew: expected gate error on ahead schema")
	}
	if err := runNew(root, "plan", []string{"x"}, &out); err == nil {
		t.Error("runNew plan: expected gate error on ahead schema")
	}
	if err := runNew(root, "topic", []string{"rendering", "x"}, &out); err == nil {
		t.Error("runNew topic: expected gate error on ahead schema")
	}
	if err := runNew(root, "topic", []string{"rendering"}, &out); err == nil || !strings.Contains(err.Error(), "usage: awf new topic") {
		t.Errorf("runNew topic must validate usage before gating, got %v", err)
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

// gatedProbes maps every gated command to an invocation that reaches its gate.
// Group children are keyed "<parent> <child>". The set is checked against
// clispec below rather than trusted, so this table cannot silently fall behind
// the spec.
var gatedProbes = map[string][]string{
	"render":           {"awf", "render"},
	"check":            {"awf", "check"},
	"check drift":      {"awf", "check", "drift"},
	"check state":      {"awf", "check", "state"},
	"check invariants": {"awf", "check", "invariants"},
	"audit":            {"awf", "audit"},
	"effort":           {"awf", "effort", "list"},
	"effort new":       {"awf", "effort", "new", "gate probe outcome"},
	"effort list":      {"awf", "effort", "list"},
	"effort show":      {"awf", "effort", "show", "gate-probe"},
	"effort finish":    {"awf", "effort", "finish", "gate-probe"},
	"effort worktree":  {"awf", "effort", "worktree", "add", "gate-probe"},
	"effort integrate": {"awf", "effort", "integrate", "gate-probe"},
	"list":             {"awf", "list"},
	"config":           {"awf", "config"},
	"context":          {"awf", "context", "README.md"},
	"topic":            {"awf", "topic", "rendering/doc-outputs"},
	"new":              {"awf", "new", "plan", "Gate probe"},
	"new adr":          {"awf", "new", "adr", "Gate probe"},
	"new plan":         {"awf", "new", "plan", "Gate probe"},
	"new topic":        {"awf", "new", "topic", "rendering", "gate-probe"},
	"new skill":        {"awf", "new", "skill", "gate-probe", "desc"},
	"new agent":        {"awf", "new", "agent", "gate-probe", "desc"},
	"new doc":          {"awf", "new", "doc", "gate-probe", "desc"},
	"enable":           {"awf", "enable", "skill", "tdd"},
	"disable":          {"awf", "disable", "skill", "tdd"},
}

// gatedCommandKeys derives, from clispec alone, the key of every command whose
// resolved gating is not Ungated.
func gatedCommandKeys() []string {
	var keys []string
	for _, c := range clispec.Commands {
		if c.Gating != clispec.Ungated {
			keys = append(keys, c.Name)
		}
		for _, child := range c.Children {
			if clispec.ResolvedGating(c, child) != clispec.Ungated {
				keys = append(keys, c.Name+" "+child.Name)
			}
		}
	}
	return keys
}

// TestDriverGatesGatedCommands confirms every command clispec marks gated
// refuses on an ahead-schema project, and derives that set from clispec rather
// than restating it, so a newly added gated command fails here until it gets a
// probe. A Gated command refuses in the driver before its handler runs, which
// for enable and disable also pins the gate-before-config-write guarantee: the
// handler never runs, so no half-mutated config is stranded. A GatedInHandler
// command refuses from inside its handler, after the static-fallback check that
// lets config, context, and topic degrade outside an adopted tree. Both layers
// must exit 1 on an adopted-but-ahead tree, which is what this asserts; the
// per-handler tests above pin which layer does the refusing.
func TestDriverGatesGatedCommands(t *testing.T) {
	keys := gatedCommandKeys()
	for _, key := range keys {
		if _, ok := gatedProbes[key]; !ok {
			t.Errorf("gated command %q has no probe: every gated command must be proven to refuse on an ahead-schema project", key)
		}
	}
	if len(gatedProbes) != len(keys) {
		known := make(map[string]bool, len(keys))
		for _, key := range keys {
			known[key] = true
		}
		for key := range gatedProbes {
			if !known[key] {
				t.Errorf("probe %q is not a gated command in clispec", key)
			}
		}
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			root := aheadSchemaProject(t)
			var out, errb bytes.Buffer
			code := runAt(t, root, gatedProbes[key], &out, &errb)
			all := out.String() + errb.String()
			if code != 1 {
				t.Fatalf("%s: expected exit 1 on ahead schema, got %d (%s)", key, code, all)
			}
			// Exit 1 alone would also pass for an incidental failure unrelated to
			// the gate, so pin the gate's own message. Before this assertion the
			// fixture's lock carried pre-tracking authority, and every command
			// here refused at the authority guard without reaching the gate.
			if !strings.Contains(all, "update your pinned awf") {
				t.Errorf("%s: refused without the version-gate message, so the refusal was not the gate: %s", key, all)
			}
		})
	}
}

// aheadSchemaProject builds a fully initialized project (permanent lock
// authority, so the command-state guard passes) whose lock then claims a schema
// generation newer than this binary. That is the only shape in which a gated
// command reaches the binary-version gate.
func aheadSchemaProject(t *testing.T) string {
	t.Helper()
	root := scaffoldProject(t)
	lockPath := filepath.Join(root, ".awf", "awf.lock")
	l, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	l.SchemaVersion = migrate.Current() + 1
	if err := l.Save(lockPath); err != nil {
		t.Fatal(err)
	}
	return root
}
