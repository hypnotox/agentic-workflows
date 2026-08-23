package main

import (
	"bytes"
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

	t.Run("loader errors", func(t *testing.T) {
		for _, operation := range []struct {
			name string
			run  func(string) error
		}{
			{name: "add", run: func(root string) error { return runNewDomain(ctx, root, "payments", io.Discard) }},
			{name: "remove", run: func(root string) error { return runRemoveDomain(ctx, root, "payments", io.Discard) }},
		} {
			t.Run(operation.name, func(t *testing.T) {
				root := scaffoldProject(t)
				if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
					t.Fatal(err)
				}
				testsupport.WriteFile(t, filepath.Join(root, ".git"), "not a gitdir pointer\n")
				if err := operation.run(root); err == nil {
					t.Fatal("malformed repository accepted")
				}
			})
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

	t.Run("remove without orphan", func(t *testing.T) {
		root := scaffoldProject(t)
		if err := runNewDomain(ctx, root, "payments", io.Discard); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(root, ".awf", "domains", "parts", "payments")); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := runRemoveDomain(ctx, root, "payments", &out); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "orphaned") {
			t.Fatalf("unexpected orphan note: %q", out.String())
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
