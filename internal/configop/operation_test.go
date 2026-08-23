package configop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func testLoader(string) (*project.Loader, error) {
	return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot), nil
}

func renderDocument(t *testing.T, document presentation.Document) string {
	t.Helper()
	var out bytes.Buffer
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestRunStaticReferenceAndErrors(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	called := false
	document, err := Run(ctx, root, "gateCmd", func(string) (*project.Loader, error) {
		called = true
		return nil, errors.New("unexpected loader")
	}, func(context.Context, string) error {
		called = true
		return errors.New("unexpected gate")
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("static reference invoked live dependencies")
	}
	if got := renderDocument(t, document); !strings.Contains(got, "config reference static (not inside an awf project)") {
		t.Fatalf("static reference = %q", got)
	}
	if _, err := Run(ctx, root, "not-a-key", testLoader, func(context.Context, string) error { return nil }); err == nil {
		t.Fatal("unknown static key accepted")
	}

	faultRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(faultRoot, ".awf"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, faultRoot, "", testLoader, func(context.Context, string) error { return nil }); err == nil {
		t.Fatal("non-absence stat fault accepted")
	}
}

func TestRunLiveReferenceAndDependencyFailures(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\n")
	gated := false
	document, err := Run(ctx, root, "gateCmd", testLoader, func(context.Context, string) error {
		gated = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !gated {
		t.Fatal("live reference skipped compatibility gate")
	}
	if got := renderDocument(t, document); !strings.Contains(got, "config reference live") {
		t.Fatalf("live reference = %q", got)
	}

	wantGate := errors.New("gate failed")
	if _, err := Run(ctx, root, "", testLoader, func(context.Context, string) error { return wantGate }); !errors.Is(err, wantGate) {
		t.Fatalf("gate error = %v", err)
	}
	wantLoad := errors.New("load failed")
	if _, err := Run(ctx, root, "", func(string) (*project.Loader, error) { return nil, wantLoad }, func(context.Context, string) error { return nil }); !errors.Is(err, wantLoad) {
		t.Fatalf("loader construction error = %v", err)
	}

	invalidRoot := t.TempDir()
	testsupport.WriteAwfConfig(t, invalidRoot, "prefix: \"\"\nintegrationBranch: main\n")
	if _, err := Run(ctx, invalidRoot, "", testLoader, func(context.Context, string) error { return nil }); err == nil {
		t.Fatal("project open error not propagated")
	}
}

func TestRunLivePublisherFailure(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\n")
	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills", "parts", "tdd", "surfaces.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, root, "", testLoader, func(context.Context, string) error { return nil }); err == nil {
		t.Fatal("Publisher build failure not propagated")
	}
}
