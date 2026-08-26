package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	if err := (&manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 46, Files: map[string]manifest.Entry{"prior": {}}}).Save(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: test\nprofile: full\nintegrationBranch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func setOperationLockSchema(t *testing.T, root string, schema int) {
	t.Helper()
	lock, found, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil || !found {
		t.Fatalf("load operation lock: found=%t err=%v", found, err)
	}
	lock.SchemaVersion = schema
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
}

func testLiveSchemaRange() (int, int) { return 46, 46 }

func runOperation(t *testing.T, root string, sync Sync, gate Gate, migration Migration, schema SchemaGate) (OperationOutcome, error) {
	t.Helper()
	return Run(testContext(t), root, sync, gate, func(string) (bool, error) { return true, nil }, testLiveSchemaRange, schema, migration, func() string { return "current schema" })
}

func TestRecoverOperationRoutesJournalOutcomeAndFailure(t *testing.T) {
	presenceFailure := errors.New("inspect project presence")
	if _, err := RecoverOperation(t.TempDir(), func(string) (bool, error) { return false, presenceFailure }); !errors.Is(err, presenceFailure) {
		t.Fatalf("project presence error = %v, want %v", err, presenceFailure)
	}
	if _, err := RecoverOperation(t.TempDir(), func(string) (bool, error) { return false, nil }); err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("absent project error = %v", err)
	}
	root := t.TempDir()
	if _, err := RecoverOperation(root, func(string) (bool, error) { return true, nil }); err == nil {
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
	outcome, err := RecoverOperation(root, func(string) (bool, error) { return true, nil })
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
	presenceFailure := errors.New("inspect project presence")
	if _, err := Run(context.Background(), t.TempDir(), nil, nil, func(string) (bool, error) { return false, presenceFailure }, testLiveSchemaRange, nil, nil, nil); !errors.Is(err, presenceFailure) {
		t.Fatalf("project presence error = %v, want %v", err, presenceFailure)
	}
	_, err := Run(context.Background(), t.TempDir(), nil, nil, func(string) (bool, error) { return false, nil }, testLiveSchemaRange, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("absent project error = %v", err)
	}
	root := t.TempDir()
	_, err = Run(context.Background(), root, nil, nil, func(string) (bool, error) { return true, nil }, testLiveSchemaRange, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("absent authority error = %v", err)
	}
	testsupport.WriteFile(t, config.LockPath(root), "{")
	_, err = Run(context.Background(), root, nil, nil, func(string) (bool, error) { return true, nil }, testLiveSchemaRange, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("corrupt authority error = %v", err)
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
	testsupport.WriteFile(t, path, `{"awfVersion":"0.19.0","schemaVersion":46,"initializedWithVersion":"bad","files":{"prior":{}}}`)
	_, err := Run(testContext(t), root, nil, nil, func(string) (bool, error) { return true, nil }, testLiveSchemaRange, nil, nil, nil)
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

func TestRunRejectsBelowFloorBeforeAuthorityValidation(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, config.ConfigPath(root), "prefix: test\n")
	testsupport.WriteFile(t, config.LockPath(root), `{"awfVersion":"0.39.2","schemaVersion":45,"files":{},"bridgeAttestation":{"version":1,"adrFormatV1From":1,"legacyADRGaps":null}}`)

	called := false
	_, err := Run(testContext(t), root, nil, nil, func(string) (bool, error) { return true, nil }, testLiveSchemaRange,
		func(string) (string, int, error) { called = true; return "ok", 45, nil },
		func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, nil)
	if !errors.Is(err, manifest.ErrUnsupportedLiveSource) || called {
		t.Fatalf("Run() error = %v, dispatch = %t, want schema-first refusal", err, called)
	}
	if strings.Contains(err.Error(), "invalid lock authority") {
		t.Fatalf("Run() interpreted below-floor authority: %v", err)
	}
}

func TestRunRefusesUnsupportedLiveAuthorityBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		schema     int
		rangeFn    LiveSchemaRange
	}{
		{"below floor", "below live floor 46", 45, testLiveSchemaRange},
		{"ahead of range", "ahead of live schema 46", 47, testLiveSchemaRange},
		{"caller supplied bounds", "below live floor 47", 46, func() (int, int) { return 47, 48 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			operationLock(t, root)
			setOperationLockSchema(t, root, tc.schema)
			called := false
			_, err := Run(testContext(t), root, nil, nil, func(string) (bool, error) { return true, nil }, tc.rangeFn,
				func(string) (string, int, error) { called = true; return "ok", tc.schema, nil },
				func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) || called {
				t.Fatalf("error=%v dispatch=%t, want %q before authority dispatch", err, called, tc.want)
			}
		})
	}
}

func TestRunRefusesPartialCurrentAuthorityBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name           string
		writeAuthority func(t *testing.T, root string)
		want           manifest.PartialAuthorityError
	}{
		{
			name: "config only",
			writeAuthority: func(t *testing.T, root string) {
				t.Helper()
				testsupport.WriteFile(t, config.ConfigPath(root), "prefix: test\n")
			},
			want: manifest.PartialAuthorityError{Config: true, Lock: false},
		},
		{
			name: "lock only",
			writeAuthority: func(t *testing.T, root string) {
				t.Helper()
				operationLock(t, root)
				if err := os.Remove(config.ConfigPath(root)); err != nil {
					t.Fatal(err)
				}
			},
			want: manifest.PartialAuthorityError{Config: false, Lock: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.writeAuthority(t, root)
			called := false
			_, err := Run(testContext(t), root, nil, nil, func(string) (bool, error) { return true, nil }, testLiveSchemaRange,
				func(string) (string, int, error) { called = true; return "ok", 46, nil },
				func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, nil)
			var partial *manifest.PartialAuthorityError
			if !errors.As(err, &partial) || *partial != tc.want || called {
				t.Fatalf("error=%v partial=%#v dispatch=%t, want %#v before dispatch", err, partial, called, tc.want)
			}
		})
	}
}

func TestRunPreservesCurrentAuthorityStatFailuresBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		writeLock bool
	}{
		{name: "loaded lock", writeLock: true},
		{name: "missing lock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.writeLock {
				operationLock(t, root)
				if err := os.Remove(config.ConfigPath(root)); err != nil {
					t.Fatal(err)
				}
			} else if err := os.MkdirAll(filepath.Dir(config.ConfigPath(root)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("config.yaml", config.ConfigPath(root)); err != nil {
				t.Fatal(err)
			}

			called := false
			_, err := Run(testContext(t), root,
				func(context.Context, string) (presentation.Mutation, error) {
					called = true
					return presentation.Mutation{}, nil
				},
				func(context.Context, string) error { called = true; return nil },
				func(string) (bool, error) { return true, nil }, testLiveSchemaRange,
				func(string) (string, int, error) { called = true; return "ok", 46, nil },
				func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, nil)
			if !errors.Is(err, syscall.ELOOP) || called {
				t.Fatalf("Run() error = %v, dispatch = %t, want config stat failure before dispatch", err, called)
			}
		})
	}
}

func TestRunPreservesInitialLockStatFailure(t *testing.T) {
	root := t.TempDir()
	lockPath := operationLock(t, root)
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("awf.lock", lockPath); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err := Run(testContext(t), root,
		func(context.Context, string) (presentation.Mutation, error) {
			called = true
			return presentation.Mutation{}, nil
		},
		func(context.Context, string) error { called = true; return nil },
		func(string) (bool, error) { return true, nil }, testLiveSchemaRange,
		func(string) (string, int, error) { called = true; return "ok", 46, nil },
		func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, nil)
	if !errors.Is(err, syscall.ELOOP) || called {
		t.Fatalf("Run() error = %v, dispatch = %t, want initial lock stat failure", err, called)
	}
}

func TestRunRevalidatesCurrentAuthorityAfterSchemaClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, lockPath string)
		check  func(t *testing.T, err error)
	}{
		{
			name: "lock disappeared",
			mutate: func(t *testing.T, lockPath string) {
				t.Helper()
				if err := os.Remove(lockPath); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, err error) {
				t.Helper()
				var partial *manifest.PartialAuthorityError
				if !errors.As(err, &partial) || !partial.Config || partial.Lock {
					t.Fatalf("Run() error = %#v, want config-only partial authority", err)
				}
			},
		},
		{
			name: "lock stat failure",
			mutate: func(t *testing.T, lockPath string) {
				t.Helper()
				if err := os.Remove(lockPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("awf.lock", lockPath); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, syscall.ELOOP) {
					t.Fatalf("Run() error = %v, want lock stat failure", err)
				}
			},
		},
		{
			name: "lock replaced",
			mutate: func(t *testing.T, lockPath string) {
				t.Helper()
				testsupport.WriteFile(t, lockPath, `{"awfVersion":"0.39.2","schemaVersion":45,"files":{},"bridgeAttestation":{"version":1}}`)
			},
			check: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
					t.Fatalf("Run() error = %v, want replacement live-source refusal", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			lockPath := operationLock(t, root)
			called := false
			_, err := Run(testContext(t), root,
				func(context.Context, string) (presentation.Mutation, error) {
					called = true
					return presentation.Mutation{}, nil
				},
				func(context.Context, string) error { called = true; return nil },
				func(string) (bool, error) { return true, nil }, testLiveSchemaRange,
				func(string) (string, int, error) {
					tc.mutate(t, lockPath)
					return "ok", 46, nil
				},
				func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, nil)
			tc.check(t, err)
			if called {
				t.Fatal("migration, gate, or sync ran after current authority changed")
			}
		})
	}
}

func TestRunRevalidatesConfigAfterSchemaClassification(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	called := false
	_, err := Run(testContext(t), root,
		func(context.Context, string) (presentation.Mutation, error) {
			called = true
			return presentation.Mutation{}, nil
		},
		func(context.Context, string) error { called = true; return nil },
		func(string) (bool, error) { return true, nil }, testLiveSchemaRange,
		func(string) (string, int, error) {
			if err := os.Remove(config.ConfigPath(root)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("config.yaml", config.ConfigPath(root)); err != nil {
				t.Fatal(err)
			}
			return "ok", 46, nil
		},
		func(context.Context, string) (MigrationResult, error) { called = true; return MigrationResult{}, nil }, nil)
	if !errors.Is(err, syscall.ELOOP) || called {
		t.Fatalf("Run() error = %v, dispatch = %t, want config stat failure after schema classification", err, called)
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
	if got := strings.Join(partial.applied, ","); got != "first" {
		t.Fatalf("applied = %q", got)
	}
	if got := strings.Join(partial.changes, ","); got != "changed config" {
		t.Fatalf("changes = %q", got)
	}
	diagnostic, diagnosticErr := partial.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	if len(diagnostic.Changed) != 3 || len(diagnostic.Steps) != 3 {
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
