package main

import (
	"bytes"
	"errors"
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
	ctx := testContext(t)
	_ = ctx
	nonRepo := t.TempDir()
	if err := gateStaged(ctx, nonRepo); err == nil {
		t.Fatal("gateStaged accepted a non-repository")
	}
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	if _, err := stagedLock(ctx, dir); err == nil || !strings.Contains(err.Error(), "no staged .awf/awf.lock") {
		t.Fatalf("missing staged lock error = %v", err)
	}
	gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": "{not json"})
	if err := gateStaged(ctx, dir); err == nil || !strings.Contains(err.Error(), "parse staged lock") {
		t.Fatalf("gateStaged malformed lock error = %v", err)
	}
}

func TestGateCorruptLockError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gateFixture(t, "0.4.0", migrate.Current())
	if err := os.WriteFile(config.LockPath(root), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gate(ctx, root); err == nil {
		t.Fatal("expected gate lock error")
	}
}

// invariant: tooling/cli:version-compat-gate (TestGateAheadSchemaErrors)
func TestGateAheadSchemaErrors(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gateFixture(t, "0.4.0", migrate.Current()+1)
	err := gate(ctx, root)
	if err == nil {
		t.Fatal("expected gate error on ahead schema")
	}
	if !strings.Contains(err.Error(), "update your pinned awf") || !strings.Contains(err.Error(), "ahead of live schema") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGateRefusalPresentation(t *testing.T) {
	required := &migrate.UpgradeRequiredError{Generation: migrate.LiveSchemaFloor, Current: migrate.LiveSchemaFloor + 1}
	if got := presentGateRefusal(required); !strings.Contains(got.Error(), "run awf upgrade") {
		t.Fatalf("supported migration refusal = %v", got)
	}

	other := errors.New("other gate failure")
	if got := presentLiveSourceRefusal(other); !errors.Is(got, other) {
		t.Fatalf("unclassified refusal = %v, want source identity", got)
	}
}

func TestGateBehindVersionErrors(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A lock far above any real release is unambiguously newer than the test
	// binary (project.Version), so this stays correct across version bumps.
	root := gateFixture(t, "99.0.0", migrate.Current())
	err := gate(ctx, root)
	// invariant: tooling/cli:version-compat-gate (TestGateBehindVersionErrors)
	if err == nil {
		t.Fatal("expected gate error on behind version")
	}
	if !strings.Contains(err.Error(), "is behind this project (rendered by 99.0.0)") {
		t.Errorf("unexpected error: %v", err)
	}
}

// invariant: tooling/cli:version-compat-gate (TestGateAtOrAheadVersionPermitted)
func TestGateAtOrAheadVersionPermitted(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// project.Version is the equal boundary; "0.0.1" is below any real release.
	for _, v := range []string{project.Version, "0.0.1"} {
		root := gateFixture(t, v, migrate.Current())
		if err := gate(ctx, root); err != nil {
			t.Errorf("gate(ctx, %s) = %v, want nil", v, err)
		}
	}
}

func TestGateSkipNoLock(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gateFixture(t, "", -1)
	if err := gate(ctx, root); err != nil {
		t.Errorf("gate (no lock) = %v, want nil", err)
	}
}

func TestGateSkipEmptyVersion(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gateFixture(t, "", migrate.Current())
	if err := gate(ctx, root); err != nil {
		t.Errorf("gate (empty version) = %v, want nil", err)
	}
}

func TestGateSkipUnparseableVersion(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gateFixture(t, "garbage", migrate.Current())
	if err := gate(ctx, root); err != nil {
		t.Errorf("gate (unparseable version) = %v, want nil", err)
	}
}

func TestNormalizeSemver(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
// invariant: tooling/cli:pitfall-scaffold (TestNewGatesInHandler)
func TestNewGatesInHandler(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gateFixture(t, "0.4.0", migrate.Current()+1)
	var out bytes.Buffer
	// invariant: tooling/cli:adr-new-version-gated (TestNewGatesInHandler)
	if err := runNew(ctx, root, "adr", []string{"x"}, &out); err == nil {
		t.Error("runNew: expected gate error on ahead schema")
	}
	if err := runNew(ctx, root, "plan", []string{"x"}, &out); err == nil {
		t.Error("runNew plan: expected gate error on ahead schema")
	}
	if err := runNew(ctx, root, "topic", []string{"rendering", "x"}, &out); err == nil {
		t.Error("runNew topic: expected gate error on ahead schema")
	}
	testsupport.WriteFile(t, filepath.Join(root, ".awf/docs/pitfalls/bad.md"), "malformed source")
	if err := runNew(ctx, root, "pitfall", []string{"x"}, &out); err == nil || !strings.Contains(err.Error(), "update your pinned awf") || strings.Contains(err.Error(), "pitfall source") {
		t.Errorf("runNew pitfall must gate before corpus reads, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf/docs/pitfalls/x.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("runNew pitfall wrote before gate refusal: %v", err)
	}
	if err := runNew(ctx, root, "topic", []string{"rendering"}, &out); err == nil || !strings.Contains(err.Error(), "usage: awf new topic") {
		t.Errorf("runNew topic must validate usage before gating, got %v", err)
	}
	if err := runNew(ctx, root, "pitfall", nil, &out); err == nil || !strings.Contains(err.Error(), "usage: awf new pitfall") {
		t.Errorf("runNew pitfall must validate usage before gating, got %v", err)
	}
	if err := runNew(ctx, root, "skill", []string{"x", "desc"}, &out); err == nil {
		t.Error("runNew skill: expected gate error on ahead schema")
	}
	if err := runNew(ctx, root, "doc", []string{"x", "desc"}, &out); err == nil {
		t.Error("runNew doc: expected gate error on ahead schema")
	}
}

// TestInitAndUpgradeGateBehindVersion pins that init and upgrade re-assert the
// binary-version gate their chained sync used to provide (removed from runSync in
// the parse-once refactor): a tree whose lock awfVersion is newer than the binary
// refuses rather than silently re-stamping a downgraded version.
func TestInitAndUpgradeGateBehindVersion(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	"render":                    {"awf", "render"},
	"check":                     {"awf", "check"},
	"check repo":                {"awf", "check", "repo"},
	"check staged":              {"awf", "check", "staged"},
	"check commit-policy":       {"awf", "check", "commit-policy", "HEAD"},
	"check staged state":        {"awf", "check", "staged", "state"},
	"check staged drift":        {"awf", "check", "staged", "drift"},
	"check repo drift":          {"awf", "check", "repo", "drift"},
	"check repo state":          {"awf", "check", "repo", "state"},
	"check repo prose":          {"awf", "check", "repo", "prose"},
	"check repo memory":         {"awf", "check", "repo", "memory"},
	"check staged commit":       {"awf", "check", "staged", "commit"},
	"read":                      {"awf", "read"},
	"read plan":                 {"awf", "read", "plan", "2026-08-02-plan", "1"},
	"audit":                     {"awf", "audit", "HEAD"},
	"effort":                    {"awf", "effort", "list"},
	"effort new":                {"awf", "effort", "new", "--slug", "gate-probe", "gate probe outcome"},
	"effort list":               {"awf", "effort", "list"},
	"effort show":               {"awf", "effort", "show", "gate-probe"},
	"effort finish":             {"awf", "effort", "finish", "gate-probe"},
	"effort worktree":           {"awf", "effort", "worktree", "add", "gate-probe"},
	"effort integrate":          {"awf", "effort", "integrate", "gate-probe"},
	"effort memory":             {"awf", "effort", "memory", "update", "gate-probe", "--phase", "phase"},
	"effort memory read":        {"awf", "effort", "memory", "read", "gate-probe"},
	"effort memory edit":        {"awf", "effort", "memory", "edit", "gate-probe"},
	"effort memory update":      {"awf", "effort", "memory", "update", "gate-probe", "--phase", "phase"},
	"effort activity":           {"awf", "effort", "activity", "attach", "gate-probe", "--owner", "00000000-0000-4000-8000-000000000001", "--json"},
	"effort activity attach":    {"awf", "effort", "activity", "attach", "gate-probe", "--owner", "00000000-0000-4000-8000-000000000001", "--json"},
	"effort activity heartbeat": {"awf", "effort", "activity", "heartbeat", "gate-probe", "--owner", "00000000-0000-4000-8000-000000000001", "--json"},
	"effort activity detach":    {"awf", "effort", "activity", "detach", "gate-probe", "--owner", "00000000-0000-4000-8000-000000000001", "--json"},
	"adr":                       {"awf", "adr", "number"},
	"adr number":                {"awf", "adr", "number"},
	"list":                      {"awf", "list"},
	"config":                    {"awf", "config"},
	"context":                   {"awf", "context", "README.md"},
	"topic":                     {"awf", "topic", "rendering/doc-outputs"},
	"new":                       {"awf", "new", "plan", "Gate probe"},
	"new adr":                   {"awf", "new", "adr", "Gate probe"},
	"new plan":                  {"awf", "new", "plan", "Gate probe"},
	"new pitfall":               {"awf", "new", "pitfall", "Gate probe"},
	"new doc":                   {"awf", "new", "doc", "runbooks/gate-probe", "Gate probe"},
	"new topic":                 {"awf", "new", "topic", "rendering", "gate-probe"},
	"new domain":                {"awf", "new", "domain", "gate-probe"},
	"remove":                    {"awf", "remove", "domain", "gate-probe"},
	"remove domain":             {"awf", "remove", "domain", "gate-probe"},
}

// gatedCommandKeys derives, from clispec alone, the key of every command whose
// top-level family is not Ungated.
func gatedCommandKeys() []string {
	var keys []string
	var add func(clispec.Command, string, bool)
	add = func(c clispec.Command, path string, gated bool) {
		gated = gated && c.Gating != clispec.Ungated
		if gated {
			keys = append(keys, path)
		}
		for _, child := range c.Children {
			add(child, path+" "+child.Name, gated)
		}
	}
	for _, c := range clispec.Commands {
		add(c, c.Name, c.Gating != clispec.Ungated)
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
			if strings.HasPrefix(key, "check staged") {
				gitfixture.Add(t, gitfixture.At(root), ".awf/awf.lock")
			}
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

func TestGateRejectsStaleSchema(t *testing.T) {
	ctx := testContext(t)
	// A legacy single-file layout (.claude/awf.yaml, no tree config) reports
	// generation 0, which is below the live floor rather than an executable
	// migration route for this binary.
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gate(ctx, root); !errors.Is(err, manifest.ErrUnsupportedLiveSource) || strings.Contains(err.Error(), "run awf upgrade") {
		t.Fatalf("stale schema refusal = %v, want unsupported live source without an impossible upgrade direction", err)
	}
	// render is Gated: command-state admission must preserve that refusal rather
	// than route the missing retired-layout lock through bridge authority.
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "render"}, &out, &errb); code != 1 {
		t.Errorf("expected the driver to fail render on stale schema, got %d", code)
	}
	if got := errb.String(); !strings.Contains(got, "below live floor") || strings.Contains(got, "attest") || strings.Contains(got, "run awf upgrade") {
		t.Fatalf("driver stale-schema refusal = %q", got)
	}
}

func TestCommandStateGuardAdmitsOnlyCompleteLiveAuthority(t *testing.T) {
	t.Run("below-floor bridge is refused before authority dispatch", func(t *testing.T) {
		root := scaffoldProject(t)
		attestLock(t, root)
		lock, err := manifest.Load(config.LockPath(root))
		if err != nil {
			t.Fatal(err)
		}
		lock.SchemaVersion = migrate.LiveSchemaFloor - 1
		if err := lock.Save(config.LockPath(root)); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if code := runAt(t, root, []string{"awf", "render"}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if got := stderr.String(); !strings.Contains(got, "below live floor") || strings.Contains(got, "consume") {
			t.Fatalf("below-floor bridge refusal = %q", got)
		}
	})

	t.Run("config-only authority is partial, not pre-tracking", func(t *testing.T) {
		root := scaffoldProject(t)
		if err := os.Remove(config.LockPath(root)); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if code := runAt(t, root, []string{"awf", "render"}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if got := stderr.String(); !strings.Contains(got, "partial authority") || strings.Contains(got, "pre-tracking") {
			t.Fatalf("config-only refusal = %q", got)
		}
	})

	t.Run("retired working layout is refused even at current schema", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".claude/awf/config.yaml"), "prefix: test\n")
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
		if err := lock.Save(filepath.Join(root, ".claude/awf/awf.lock")); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if code := runAt(t, root, []string{"awf", "render"}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if got := stderr.String(); !strings.Contains(got, "retired project layout is unsupported") || strings.Contains(got, "attest") {
			t.Fatalf("retired-layout refusal = %q", got)
		}
	})

	t.Run("staged retired authority is refused before staged loading", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Stage(t, repo, map[string]string{
			".claude/awf/config.yaml": "prefix: test\n",
			".claude/awf/awf.lock":    `{"awfVersion":"0.39.2","schemaVersion":46,"files":{}}` + "\n",
		})
		var stdout, stderr bytes.Buffer
		if code := runAt(t, repo.Root(), []string{"awf", "check", "staged"}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if got := stderr.String(); !strings.Contains(got, "retired project authority is below live floor") || strings.Contains(got, "attest") {
			t.Fatalf("staged retired-authority refusal = %q", got)
		}
	})
}
