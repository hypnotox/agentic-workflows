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
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func operationLock(t *testing.T, root string) string {
	t.Helper()
	path := config.LockPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 14, Files: map[string]manifest.Entry{}}).Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func runOperation(t *testing.T, root string, sync Sync, gate Gate, migration Migration, schema SchemaGate) (OperationOutcome, error) {
	t.Helper()
	return Run(testContext(t), root, sync, gate, func(string) bool { return true }, config.LockPath, schema, func(n int) error { return errors.New("schema ahead " + string(rune('0'+n))) }, migration, func() string { return "current schema" })
}

func TestRecoverOperationRoutesJournalOutcomeAndFailure(t *testing.T) {
	if _, err := RecoverOperation(t.TempDir(), func(string) bool { return false }); err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("absent project error = %v", err)
	}
	root := t.TempDir()
	if _, err := RecoverOperation(root, func(string) bool { return true }); err == nil {
		t.Fatal("missing journal accepted")
	} else {
		var failure journalFailure
		if !errors.As(err, &failure) || failure.Unwrap() == nil {
			t.Fatalf("error = %T, want journal failure", err)
		}
	}
	writeRawJournal(t, root, lockJournal(phaseApplying))
	mustWrite(t, filepath.Join(root, "a.txt"), []byte("new"))
	mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))
	outcome, err := RecoverOperation(root, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, outcome.Document); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "status: upgrade recovered") {
		t.Fatalf("presentation = %q", rendered.String())
	}
}

func TestRunRejectsAbsentProjectAndAuthority(t *testing.T) {
	_, err := Run(context.Background(), t.TempDir(), nil, nil, func(string) bool { return false }, config.LockPath, nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("absent project error = %v", err)
	}
	root := t.TempDir()
	_, err = Run(context.Background(), root, nil, nil, func(string) bool { return true }, config.LockPath, nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "bridge release") {
		t.Fatalf("absent authority error = %v", err)
	}
	testsupport.WriteFile(t, config.LockPath(root), "{")
	_, err = Run(context.Background(), root, nil, nil, func(string) bool { return true }, config.LockPath, nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("corrupt authority error = %v", err)
	}
}

func TestRunRoutesBridgeAuthorityToFinalCutover(t *testing.T) {
	root, head, digest := sealedRepo(t)
	finalLock(t, root, sealedAtt(head, digest))
	called := false
	outcome, err := Run(testContext(t), root, func(context.Context, string) (presentation.Mutation, error) {
		called = true
		return presentation.Mutation{}, nil
	}, func(context.Context, string) error { called = true; return nil }, func(string) bool { return true }, config.LockPath, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("bridge cutover ran ordinary migration dependencies")
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, outcome.Document); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "status: upgrade completed") {
		t.Fatalf("presentation = %q", rendered.String())
	}
}

func TestRunSequencingAndCurrentSchemaPresentation(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	var calls []string
	outcome, err := runOperation(t, root,
		func(context.Context, string) (presentation.Mutation, error) {
			calls = append(calls, "sync")
			return presentation.Mutation{Status: "synced"}, nil
		},
		func(context.Context, string) error { calls = append(calls, "gate"); return nil },
		func(context.Context, string) (MigrationResult, error) {
			calls = append(calls, "migrate")
			return MigrationResult{Applied: []string{"first"}, Changes: []string{"changed config"}}, nil
		},
		func(string) (string, int, error) { calls = append(calls, "schema"); return "ok", 14, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(calls, ","); got != "schema,migrate,gate,sync" {
		t.Fatalf("calls = %q", got)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, outcome.Document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"applied migrations:\n      first", "migration changes:\n      changed config\n      current schema"} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("presentation = %q, want %q", rendered.String(), want)
		}
	}
}

func TestRunPropagatesAuthorityAndSchemaFailures(t *testing.T) {
	root := t.TempDir()
	path := operationLock(t, root)
	testsupport.WriteFile(t, path, `{"awfVersion":"0.19.0","initializedWithVersion":"bad","files":{}}`)
	_, err := Run(testContext(t), root, nil, nil, func(string) bool { return true }, config.LockPath, nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid lock authority") {
		t.Fatalf("authority error = %v", err)
	}
	operationLock(t, root)
	failure := errors.New("schema state failed")
	_, err = runOperation(t, root, nil, nil, nil, func(string) (string, int, error) { return "", 0, failure })
	if !errors.Is(err, failure) {
		t.Fatalf("schema error = %v", err)
	}
}

func TestRunRejectsSchemaAheadBeforeMigration(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	called := false
	_, err := runOperation(t, root, nil, nil, func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, func(string) (string, int, error) { return "ahead", 7, nil })
	if err == nil || !strings.Contains(err.Error(), "schema ahead 7") || called {
		t.Fatalf("err = %v migrate called = %t", err, called)
	}
}

func TestRunFailureRetainsMigrationAndSyncFacts(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	failure := errors.New("terminal sync failed")
	partialMutation := presentation.Mutation{Changes: []presentation.MutationChange{{Label: "outputs", Values: []presentation.Value{mustProse(t, "changed AGENTS.md")}}}}
	_, err := runOperation(t, root, func(context.Context, string) (presentation.Mutation, error) { return partialMutation, failure }, func(context.Context, string) error { return nil }, func(context.Context, string) (MigrationResult, error) {
		return MigrationResult{Applied: []string{"first"}, Changes: []string{"changed config"}}, nil
	}, func(string) (string, int, error) { return "behind", 13, nil })
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	var partial upgradeFailure
	if !errors.As(err, &partial) {
		t.Fatalf("error type = %T", err)
	}
	if got := strings.Join(partial.changes, ","); got != "changed config" {
		t.Fatalf("changes = %q", got)
	}
	diagnostic, diagnosticErr := partial.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	if len(diagnostic.Changed) != 2 || len(diagnostic.Steps) != 3 {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestRunFailureBeforeChangesUsesRetryDiagnostic(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	failure := errors.New("migration failed")
	_, err := runOperation(t, root, nil, nil, func(context.Context, string) (MigrationResult, error) { return MigrationResult{}, failure }, func(string) (string, int, error) { return "behind", 13, nil })
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	var partial upgradeFailure
	if !errors.As(err, &partial) {
		t.Fatalf("error type = %T", err)
	}
	diagnostic, diagnosticErr := partial.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	if len(diagnostic.Changed) != 0 || len(diagnostic.Steps) != 1 {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestRunGateFailureRetainsOrderedMigrationFacts(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	failure := errors.New("gate failed")
	_, err := runOperation(t, root, nil, func(context.Context, string) error { return failure }, func(context.Context, string) (MigrationResult, error) {
		return MigrationResult{Applied: []string{"first", "second"}, Changes: []string{"first change", "second change"}}, nil
	}, func(string) (string, int, error) { return "behind", 13, nil })
	var partial upgradeFailure
	if !errors.Is(err, failure) || !errors.As(err, &partial) {
		t.Fatalf("error = %v", err)
	}
	if got := strings.Join(partial.changes, ","); got != "first change,second change" {
		t.Fatalf("changes = %q", got)
	}
}

type specialDiagnosticError struct{}

func (specialDiagnosticError) Error() string { return "collision" }
func (specialDiagnosticError) UpgradeDiagnostic(changes []string) (presentation.Diagnostic, error) {
	return presentation.Diagnostic{Condition: strings.Join(changes, ","), State: "operation"}, nil
}

func TestRunRoutesSpecialMigrationDiagnostic(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	_, err := runOperation(t, root, nil, nil, func(context.Context, string) (MigrationResult, error) {
		return MigrationResult{Changes: []string{"earlier change"}}, specialDiagnosticError{}
	}, func(string) (string, int, error) { return "behind", 13, nil })
	var failure upgradeFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %T", err)
	}
	diagnostic, diagnosticErr := failure.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	if diagnostic.Condition != "earlier change" || diagnostic.Cause != "" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestRunMigrationAndPresentationRejectInvalidSemanticValues(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	_, err := runOperation(t, root, func(context.Context, string) (presentation.Mutation, error) { return presentation.Mutation{}, nil }, func(context.Context, string) error { return nil }, func(context.Context, string) (MigrationResult, error) {
		return MigrationResult{Applied: []string{"\n"}}, nil
	}, func(string) (string, int, error) { return "behind", 13, nil })
	if err == nil {
		t.Fatal("invalid migration name accepted")
	}
}

func TestRunRelocatedLockCompletionFailureRetainsCreatedLockAxis(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, ".claude", "awf.lock")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 2, Files: map[string]manifest.Entry{}}).Save(legacy); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("completion failed")
	calls := 0
	_, err := Run(testContext(t), root, nil, nil, func(string) bool { return true }, func(string) string { return legacy }, func(string) (string, int, error) { return "behind", 2, nil }, func(int) error { return nil }, func(context.Context, string) (MigrationResult, error) {
		calls++
		if calls == 1 {
			if err := os.MkdirAll(config.RootDir(root), 0o755); err != nil {
				t.Fatal(err)
			}
			return MigrationResult{Applied: []string{"relocate"}, Changes: []string{"moved config"}}, nil
		}
		return MigrationResult{}, failure
	}, func() string { return "current schema" })
	var partial upgradeFailure
	if !errors.Is(err, failure) || !errors.As(err, &partial) {
		t.Fatalf("error = %v", err)
	}
	if got := strings.Join(partial.changes, ","); got != "moved config,created and schema-stamped .awf/awf.lock" {
		t.Fatalf("changes = %q", got)
	}
}

func TestRunRelocatedLockReadFailureRetainsMigrationFacts(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, ".claude", "awf.lock")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 2, Files: map[string]manifest.Entry{}}).Save(legacy); err != nil {
		t.Fatal(err)
	}
	_, err := Run(testContext(t), root, nil, nil, func(string) bool { return true }, func(string) string { return legacy }, func(string) (string, int, error) { return "behind", 2, nil }, func(int) error { return nil }, func(context.Context, string) (MigrationResult, error) {
		testsupport.WriteFile(t, config.LockPath(root), "{")
		return MigrationResult{Applied: []string{"relocate"}, Changes: []string{"moved config"}}, nil
	}, func() string { return "current schema" })
	if err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("error = %v, want malformed replacement lock failure", err)
	}
	var partial upgradeFailure
	if !errors.As(err, &partial) || strings.Join(partial.changes, ",") != "moved config" {
		t.Fatalf("failure = %#v, want retained migration fact", partial)
	}
}

func TestRunRejectsInvalidTerminalMutation(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	_, err := runOperation(t, root, func(context.Context, string) (presentation.Mutation, error) {
		return presentation.Mutation{Status: "\n"}, nil
	}, func(context.Context, string) error { return nil }, func(context.Context, string) (MigrationResult, error) {
		return MigrationResult{Changes: []string{"migrated"}}, nil
	}, func(string) (string, int, error) { return "behind", 13, nil })
	var partial upgradeFailure
	if err == nil || !errors.As(err, &partial) {
		t.Fatalf("error = %v, want invalid terminal mutation wrapped as upgrade failure", err)
	}
	if got := strings.Join(partial.changes, ","); got != "migrated" {
		t.Fatalf("retained changes = %q", got)
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
