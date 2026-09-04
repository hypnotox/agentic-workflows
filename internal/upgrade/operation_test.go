package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func seedAuthority(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(config.LockPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: test\nintegrationBranch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: "0.45.0", SchemaVersion: 50, Files: map[string]manifest.Entry{"AGENTS.md": {}}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
}

func runTestUpgrade(t *testing.T, root string, sync Sync, gate Gate, migration Migration) (OperationOutcome, error) {
	t.Helper()
	return Run(context.Background(), root, sync, gate,
		func(string) (bool, error) { return true, nil },
		func() (int, int) { return 50, 53 },
		func(string) (string, int, error) { return "gate", 50, nil },
		migration, func() string { return "config schema already current" })
}

func TestRunSequencesMigrationThenSyncThenGate(t *testing.T) {
	root := t.TempDir()
	seedAuthority(t, root)
	var calls []string
	outcome, err := runTestUpgrade(t, root,
		func(context.Context, string) (presentation.Mutation, error) {
			calls = append(calls, "sync")
			return presentation.Mutation{Status: "synced"}, nil
		},
		func(context.Context, string) error { calls = append(calls, "gate"); return nil },
		func(context.Context, string) (MigrationResult, error) {
			calls = append(calls, "migrate")
			return MigrationResult{Planned: []string{"50 to 53"}, Applied: []string{"50 to 53"}, Changes: []string{"updated authored sources"}, Touched: []string{".awf/source"}}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "migrate,sync,gate" {
		t.Fatalf("calls=%v", calls)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, outcome.Document); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "applied migrations") || !strings.Contains(rendered.String(), "50 to 53") {
		t.Fatalf("output=%q", rendered.String())
	}
}

func TestRunRefusesLegacyMarkerBeforeMutatingDispatch(t *testing.T) {
	root := t.TempDir()
	seedAuthority(t, root)
	journal := filepath.Join(root, filepath.FromSlash(legacyUpgradeMarker))
	if err := os.WriteFile(journal, []byte("opaque legacy bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err := runTestUpgrade(t, root,
		func(context.Context, string) (presentation.Mutation, error) {
			called = true
			return presentation.Mutation{}, nil
		},
		func(context.Context, string) error { called = true; return nil },
		func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil },
	)
	if err == nil || called {
		t.Fatalf("err=%v called=%t", err, called)
	}
	for _, want := range []string{legacyUpgradeMarker, "last journal-capable", "Git", "remove", "rerun `awf upgrade`"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error=%q missing %q", err, want)
		}
	}
	got, readErr := os.ReadFile(journal)
	if readErr != nil || string(got) != "opaque legacy bytes" {
		t.Fatalf("legacy file changed: %q %v", got, readErr)
	}
}

func TestSyncAndGateFailuresReportVisiblePathsAndSafeRemedies(t *testing.T) {
	for _, tc := range []struct {
		name             string
		syncErr, gateErr error
	}{
		{name: "sync", syncErr: errors.New("sync failed")},
		{name: "gate", gateErr: errors.New("gate failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			seedAuthority(t, root)
			syncMutation := presentation.Mutation{Changes: []presentation.MutationChange{{Label: "outputs", Values: []presentation.Value{mustProse(t, "changed AGENTS.md")}}}}
			_, err := runTestUpgrade(t, root,
				func(context.Context, string) (presentation.Mutation, error) { return syncMutation, tc.syncErr },
				func(context.Context, string) error { return tc.gateErr },
				func(context.Context, string) (MigrationResult, error) {
					return MigrationResult{Planned: []string{"bridge"}, Applied: []string{"bridge"}, Changes: []string{"changed source"}, Touched: []string{".awf/touched"}, Pending: []string{".awf/pending"}}, nil
				},
			)
			if err == nil {
				t.Fatal("failure accepted")
			}
			var diagnosed interface {
				Diagnostic() (presentation.Diagnostic, error)
			}
			if !errors.As(err, &diagnosed) {
				t.Fatalf("error=%T", err)
			}
			diagnostic, diagnosticErr := diagnosed.Diagnostic()
			if diagnosticErr != nil {
				t.Fatal(diagnosticErr)
			}
			var out strings.Builder
			doc, docErr := diagnostic.Document()
			if docErr != nil {
				t.Fatal(docErr)
			}
			if err := presentation.Render(&out, doc); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			for _, want := range []string{".awf/touched", ".awf/pending", "changed AGENTS.md", "git status --short", "git diff", "restore desired paths with Git if wanted", "rerun awf upgrade"} {
				if !strings.Contains(text, want) {
					t.Fatalf("diagnostic=%q missing %q", text, want)
				}
			}
			for _, forbidden := range []string{"git clean", "upgrade --recover", "rollback"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("diagnostic=%q contains %q", text, forbidden)
				}
			}
		})
	}
}

func TestMigrationFailurePreservesTouchedAndPending(t *testing.T) {
	root := t.TempDir()
	seedAuthority(t, root)
	cause := errors.New("apply failed")
	_, err := runTestUpgrade(t, root, nil, nil, func(context.Context, string) (MigrationResult, error) {
		return MigrationResult{Planned: []string{"bridge"}, Touched: []string{"first"}, Pending: []string{"second"}}, cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("err=%v", err)
	}
	var failure upgradeFailure
	if !errors.As(err, &failure) || len(failure.migration.Touched) != 1 || len(failure.migration.Pending) != 1 {
		t.Fatalf("failure=%#v", failure)
	}
}

func mustProse(t *testing.T, text string) presentation.Value {
	t.Helper()
	value, err := presentation.Prose(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
