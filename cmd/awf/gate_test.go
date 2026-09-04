package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
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
		l := &manifest.Lock{AWFVersion: awfVersion, SchemaVersion: schema, Files: map[string]manifest.Entry{"prior": {}}}
		if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestMutationAdmissionWaitsForLeaseBeforeGuardReads(t *testing.T) {
	root := scaffoldProject(t)
	lockPath := config.LockPath(root)
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	held, err := filesystem.AcquireProjectLease(testContext(t), root, awfgit.ProjectResidentRoot(testContext(t), root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	var out, errout bytes.Buffer
	go func() { done <- runFrom(root, []string{"awf", "render"}, &out, &errout) }()
	select {
	case code := <-done:
		t.Fatalf("render read partial authority before lease release: exit=%d stderr=%q", code, errout.String())
	case <-time.After(50 * time.Millisecond):
	}
	if err := os.WriteFile(lockPath, lockBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("render after stable authority: exit=%d stderr=%q", code, errout.String())
	}
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

// TestInitAndUpgradeGateBehindVersion pins that init and upgrade apply the
// binary-version gate before sync. A tree whose lock awfVersion is newer than
// the binary refuses rather than silently re-stamping a downgraded version
// before any sync can rewrite its lock.
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

// TestDriverAppliesBothGatePlacements retains one dispatch-level case for each
// gate placement. Clispec owns the complete command inventory; owner and route
// tests cover every member without rebuilding a project for each metadata row.
func TestDriverAppliesBothGatePlacements(t *testing.T) {
	root := aheadSchemaProject(t)
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "driver", args: []string{"awf", "render"}},
		{name: "handler", args: []string{"awf", "new", "pitfall", "Gate probe"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runAt(t, root, test.args, &stdout, &stderr); code != 1 {
				t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if all := stdout.String() + stderr.String(); !strings.Contains(all, "update your pinned awf") {
				t.Fatalf("gate diagnostic = %q", all)
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
	if got := errb.String(); !strings.Contains(got, "retired project layout") || strings.Contains(got, "attest") || strings.Contains(got, "run awf upgrade") {
		t.Fatalf("driver stale-schema refusal = %q", got)
	}
}

func TestValidateCurrentAuthorityHandlesConcurrentLockDisappearance(t *testing.T) {
	err := validateCurrentAuthority(false, true, true)
	var partial *manifest.PartialAuthorityError
	if !errors.As(err, &partial) || partial.Config != true || partial.Lock != false {
		t.Fatalf("validateCurrentAuthority() error = %#v, want config-only partial authority", err)
	}
}

func TestProjectGuardStateRejectsControlSymlinksWithoutFollowingThem(t *testing.T) {
	for _, rel := range []string{"config.yaml", "awf.lock"} {
		t.Run(rel, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
				t.Fatal(err)
			}
			if rel == "config.yaml" {
				if err := os.Symlink(rel, config.ConfigPath(root)); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(config.LockPath(root), []byte(`{"awfVersion":"0.39.2","schemaVersion":49,"files":{}}`), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: test\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(rel, config.LockPath(root)); err != nil {
					t.Fatal(err)
				}
			}
			_, _, _, _, _, loadErr, err := projectGuardState(testContext(t), root, false)
			if err != nil || loadErr == nil {
				t.Fatalf("projectGuardState() errors = load %v, operation %v; want safe authority refusal", loadErr, err)
			}
			if rel == "awf.lock" && !errors.Is(loadErr, filesystem.ErrIdentityChanged) {
				t.Fatalf("lock load error = %v, want no-follow identity refusal", loadErr)
			}
		})
	}
}

func TestCommandStateGuardAdmitsOnlyCompleteLiveAuthority(t *testing.T) {
	t.Run("below-floor lock is refused before authority dispatch", func(t *testing.T) {
		root := scaffoldProject(t)
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

	t.Run("working below-floor authority refuses before malformed bridge interpretation", func(t *testing.T) {
		root := scaffoldProject(t)
		testsupport.WriteFile(t, config.LockPath(root), `{"awfVersion":"0.39.2","schemaVersion":49,"files":{},"bridgeAttestation":{"version":1,"adrFormatV1From":1,"legacyADRGaps":null}}`)
		var stdout, stderr bytes.Buffer
		if code := runAt(t, root, []string{"awf", "render"}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if got := stderr.String(); !strings.Contains(got, "below live floor") || strings.Contains(got, "invalid lock authority") {
			t.Fatalf("working below-floor refusal = %q", got)
		}
	})

	t.Run("staged below-floor authority refuses before malformed bridge interpretation", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Stage(t, repo, map[string]string{
			".awf/config.yaml": "prefix: test\n",
			".awf/awf.lock":    `{"awfVersion":"0.39.2","schemaVersion":49,"files":{},"bridgeAttestation":{"version":1,"adrFormatV1From":1,"legacyADRGaps":null}}` + "\n",
		})
		var stdout, stderr bytes.Buffer
		if code := runAt(t, repo.Root(), []string{"awf", "check", "staged"}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if got := stderr.String(); !strings.Contains(got, "below live floor") || strings.Contains(got, "invalid lock authority") {
			t.Fatalf("staged below-floor refusal = %q", got)
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
		if got := stderr.String(); !strings.Contains(got, "retired project layout") || strings.Contains(got, "attest") {
			t.Fatalf("retired-layout refusal = %q", got)
		}
	})

	t.Run("staged retired authority is refused before staged loading", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Stage(t, repo, map[string]string{
			".claude/awf/config.yaml": "prefix: test\n",
			".claude/awf/awf.lock":    `{"awfVersion":"0.39.2","schemaVersion":49,"files":{}}` + "\n",
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
