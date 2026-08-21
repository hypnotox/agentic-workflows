package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestDomainCurrentStateFilesystemFailures(t *testing.T) {
	open := func(t *testing.T) (*config.Config, string) {
		t.Helper()
		root := scaffoldProject(t)
		_, p, _, err := openProjectOperation(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		return p, root
	}

	t.Run("existing", func(t *testing.T) {
		p, root := open(t)
		path := filepath.Join(root, ".awf", "domains", "parts", "payments", "current-state.md")
		testsupport.WriteFile(t, path, "authored\n")
		if err := scaffoldDomainCurrentState(p, "payments"); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != "authored\n" {
			t.Fatalf("existing part = %q, %v", got, err)
		}
	})
	t.Run("stat", func(t *testing.T) {
		p, root := open(t)
		dir := filepath.Join(root, ".awf", "domains", "parts", "payments")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("current-state.md", filepath.Join(dir, "current-state.md")); err != nil {
			t.Fatal(err)
		}
		if err := scaffoldDomainCurrentState(p, "payments"); err == nil {
			t.Fatal("symlink loop stat error was not propagated")
		}
	})
	t.Run("mkdir", func(t *testing.T) {
		p, root := open(t)
		parts := filepath.Join(root, ".awf", "domains", "parts")
		testsupport.WriteFile(t, parts, "collision\n")
		if err := scaffoldDomainCurrentState(p, "payments"); err == nil {
			t.Fatal("directory collision was not propagated")
		}
	})
	t.Run("write", func(t *testing.T) {
		p, root := open(t)
		dir := filepath.Join(root, ".awf", "domains", "parts", "payments")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing", "part.md"), filepath.Join(dir, "current-state.md")); err != nil {
			t.Fatal(err)
		}
		if err := scaffoldDomainCurrentState(p, "payments"); err == nil {
			t.Fatal("part write error was not propagated")
		}
	})
}

func TestDomainLifecyclePropagatesDependencies(t *testing.T) {
	failure := errors.New("injected failure")
	newCases := []struct {
		name   string
		mutate func(*domainDependencies)
	}{
		{"open", func(d *domainDependencies) {
			d.open = func(context.Context, string) (*config.Config, error) { return nil, failure }
		}},
		{"edit", func(d *domainDependencies) {
			d.edit = func([]byte, string, string, bool) ([]byte, error) { return nil, failure }
		}},
		{"write", func(d *domainDependencies) { d.write = func(string, []byte, os.FileMode) error { return failure } }},
		{"scaffold", func(d *domainDependencies) { d.scaffold = func(*config.Config, string) error { return failure } }},
		{"sync", func(d *domainDependencies) {
			d.synchronize = func(context.Context, string, io.Writer) error { return failure }
		}},
	}
	for _, test := range newCases {
		t.Run("new "+test.name, func(t *testing.T) {
			dependencies := productionDomainDependencies()
			test.mutate(&dependencies)
			if err := runNewDomainWith(testContext(t), scaffoldProject(t), "payments", io.Discard, dependencies); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want injected failure", err)
			}
		})
	}
	removeCases := newCases[:3]
	removeCases = append(removeCases, newCases[4])
	for _, test := range removeCases {
		t.Run("remove "+test.name, func(t *testing.T) {
			root := scaffoldProject(t)
			if err := runNewDomain(testContext(t), root, "payments", io.Discard); err != nil {
				t.Fatal(err)
			}
			dependencies := productionDomainDependencies()
			test.mutate(&dependencies)
			if err := runRemoveDomainWith(testContext(t), root, "payments", io.Discard, dependencies); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want injected failure", err)
			}
		})
	}
}

func TestHasSidecarOrParts(t *testing.T) {
	root := t.TempDir()
	if found, err := hasDomainSidecarOrParts(root, "payments"); err != nil || found {
		t.Fatalf("absent authored domain = %t, %v", found, err)
	}
	sidecar := filepath.Join(root, ".awf", "domains", "payments.yaml")
	testsupport.WriteFile(t, sidecar, "paths: []\n")
	if found, err := hasDomainSidecarOrParts(root, "payments"); err != nil || !found {
		t.Fatalf("sidecar = %t, %v", found, err)
	}
	if err := os.Remove(sidecar); err != nil {
		t.Fatal(err)
	}
	parts := filepath.Join(root, ".awf", "domains", "parts", "payments")
	if err := os.MkdirAll(parts, 0o755); err != nil {
		t.Fatal(err)
	}
	if found, err := hasDomainSidecarOrParts(root, "payments"); err != nil || !found {
		t.Fatalf("parts = %t, %v", found, err)
	}
	if err := os.RemoveAll(parts); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("payments.yaml", sidecar); err != nil {
		t.Fatal(err)
	}
	if found, err := hasDomainSidecarOrParts(root, "payments"); err == nil || found || !strings.Contains(err.Error(), "payments.yaml") {
		t.Fatalf("stat failure = %t, %v", found, err)
	}
}

func TestRemoveDomainOrphanAndCleanCompletion(t *testing.T) {
	failureWriterRoot := scaffoldProject(t)
	if err := runNewDomain(testContext(t), failureWriterRoot, "payments", io.Discard); err != nil {
		t.Fatal(err)
	}
	dependencies := productionDomainDependencies()
	dependencies.synchronize = func(context.Context, string, io.Writer) error { return nil }
	if err := runRemoveDomainWith(testContext(t), failureWriterRoot, "payments", errorWriter{}, dependencies); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("orphan note error = %v", err)
	}

	statFailureRoot := scaffoldProject(t)
	if err := runNewDomain(testContext(t), statFailureRoot, "payments", io.Discard); err != nil {
		t.Fatal(err)
	}
	dependencies.authored = func(string, string) (bool, error) { return false, errors.New("injected inspection failure") }
	if err := runRemoveDomainWith(testContext(t), statFailureRoot, "payments", io.Discard, dependencies); err == nil || !strings.Contains(err.Error(), "injected inspection failure") {
		t.Fatalf("orphan inspection error = %v", err)
	}
	dependencies.authored = hasDomainSidecarOrParts

	cleanRoot := scaffoldProject(t)
	cfgPath := config.ConfigPath(cleanRoot)
	src, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := config.SetArrayMember(src, "domains", "clean", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runRemoveDomainWith(testContext(t), cleanRoot, "clean", io.Discard, dependencies); err != nil {
		t.Fatalf("clean removal = %v", err)
	}
}

func TestRetainedNewUsageAndProjectErrors(t *testing.T) {
	if err := runNew(testContext(t), t.TempDir(), "domain", nil, io.Discard); err == nil {
		t.Fatal("domain without name accepted")
	}
	if err := newPlan(testContext(t), t.TempDir(), []string{"title"}, io.Discard); err == nil {
		t.Fatal("plan without project accepted")
	}
}

// invariant: tooling/cli:cli-creation-and-inventory (TestRetainedDomainAndListCLIPaths)
func TestRetainedDomainAndListCLIPaths(t *testing.T) {
	ctx := testContext(t)

	t.Run("dispatch usage", func(t *testing.T) {
		root := scaffoldProject(t)
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		for _, args := range [][]string{{"awf", "new", "retired"}, {"awf", "new", "domain"}, {"awf", "remove"}, {"awf", "remove", "domain"}} {
			var out, errOut bytes.Buffer
			if code := run(args, &out, &errOut); code != 2 {
				t.Fatalf("%v exit = %d, stderr = %q", args, code, errOut.String())
			}
		}
	})

	t.Run("production dispatch creates every authored kind without render selection", func(t *testing.T) {
		root := scaffoldProject(t)
		if err := runNew(ctx, root, "domain", []string{"payments"}, io.Discard); err != nil {
			t.Fatalf("dispatch domain: %v", err)
		}
		configAfterDomain := mustReadCLIFile(t, config.ConfigPath(root))
		lockAfterDomain := mustReadCLIFile(t, filepath.Join(root, ".awf", "awf.lock"))
		pitfallsBefore := mustReadCLIFile(t, filepath.Join(root, "docs", "pitfalls.md"))

		if err := runNew(ctx, root, "adr", []string{"Dispatch", "ADR"}, io.Discard); err != nil {
			t.Fatalf("dispatch adr: %v", err)
		}
		if err := runNew(ctx, root, "plan", []string{"Dispatch", "Plan"}, io.Discard); err != nil {
			t.Fatalf("dispatch plan: %v", err)
		}
		if err := runNew(ctx, root, "topic", []string{"payments", "Dispatch", "Topic"}, io.Discard); err != nil {
			t.Fatalf("dispatch topic: %v", err)
		}
		if err := runNew(ctx, root, "pitfall", []string{"Dispatch Pitfall"}, io.Discard); err != nil {
			t.Fatalf("dispatch pitfall: %v", err)
		}

		for _, pattern := range []string{
			filepath.Join(root, "docs", "decisions", "*-dispatch-adr.md"),
			filepath.Join(root, "docs", "plans", "*-dispatch-plan.md"),
			filepath.Join(root, ".awf", "domains", "parts", "payments", "current-state.md"),
			filepath.Join(root, ".awf", "topics", "metadata", "payments", "dispatch-topic.yaml"),
			filepath.Join(root, ".awf", "docs", "pitfalls", "dispatch-pitfall.md"),
		} {
			matches, err := filepath.Glob(pattern)
			if err != nil || len(matches) != 1 {
				t.Fatalf("created path %q matches = %v, %v", pattern, matches, err)
			}
		}
		if got := mustReadCLIFile(t, config.ConfigPath(root)); got != configAfterDomain {
			t.Fatal("non-domain creation selected render membership in config")
		}
		if got := mustReadCLIFile(t, filepath.Join(root, ".awf", "awf.lock")); got != lockAfterDomain {
			t.Fatal("authored creation rendered or changed lock inventory")
		}
		if got := mustReadCLIFile(t, filepath.Join(root, "docs", "pitfalls.md")); got != pitfallsBefore {
			t.Fatal("pitfall dispatch rendered the generated index")
		}
		if _, err := os.Stat(filepath.Join(root, "docs", "topics", "payments", "dispatch-topic.md")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("topic dispatch rendered generated output: %v", err)
		}
	})

	t.Run("version gate", func(t *testing.T) {
		root := scaffoldProject(t)
		lockPath := filepath.Join(root, ".awf", "awf.lock")
		lock, err := manifest.Load(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		lock.AWFVersion = "99.0.0"
		if err := lock.Save(lockPath); err != nil {
			t.Fatal(err)
		}
		if err := runNewDomain(ctx, root, "payments", io.Discard); err == nil {
			t.Fatal("new domain bypassed the version gate")
		}
		if err := runRemoveDomain(ctx, root, "payments", io.Discard); err == nil {
			t.Fatal("remove domain bypassed the version gate")
		}
	})

	t.Run("remove absence and validation", func(t *testing.T) {
		root := scaffoldProject(t)
		if err := runRemoveDomain(ctx, root, "../bad", io.Discard); err == nil {
			t.Fatal("invalid domain accepted")
		}
		if err := runRemoveDomain(ctx, root, "payments", io.Discard); err == nil || !strings.Contains(err.Error(), "not configured") {
			t.Fatalf("absent domain error = %v", err)
		}
	})

	t.Run("orphan note writer", func(t *testing.T) {
		root := scaffoldProject(t)
		if err := runNewDomain(ctx, root, "payments", io.Discard); err != nil {
			t.Fatal(err)
		}
		if err := runRemoveDomain(ctx, root, "payments", errorWriter{}); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("orphan note writer error = %v", err)
		}
	})

	t.Run("inventory", func(t *testing.T) {
		root := scaffoldProject(t)
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "tdd.yaml"), "data:\n  testSurfaces: []\n")
		for _, kind := range []string{"", "target", "domain", "skill", "agent", "doc"} {
			var out bytes.Buffer
			if err := runList(ctx, root, kind, &out); err != nil {
				t.Fatalf("list %q: %v", kind, err)
			}
			if !strings.Contains(out.String(), "status: artifact inventory") {
				t.Fatalf("list %q = %q", kind, out.String())
			}
		}
		var targets bytes.Buffer
		if err := runList(ctx, root, "target", &targets); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"claude", "pi"} {
			if !strings.Contains(targets.String(), name) {
				t.Errorf("fixed target inventory missing %q: %q", name, targets.String())
			}
		}
		for _, retired := range []string{"enabled", "available", "local"} {
			if strings.Contains(strings.ToLower(targets.String()), retired) {
				t.Errorf("target inventory retains enablement vocabulary %q: %q", retired, targets.String())
			}
		}
		if err := runList(ctx, root, "bogus", io.Discard); err == nil {
			t.Fatal("unknown list kind accepted")
		}
		if err := runList(ctx, root, "skill", errorWriter{}); err == nil {
			t.Fatal("list writer error was not propagated")
		}
		bad := t.TempDir()
		testsupport.WriteFile(t, config.ConfigPath(bad), "not: [valid\n")
		if err := runList(ctx, bad, "", io.Discard); err == nil {
			t.Fatal("list project-open error was not propagated")
		}
	})
}
