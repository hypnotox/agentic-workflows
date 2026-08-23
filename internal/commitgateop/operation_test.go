package commitgateop

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestRunAppliesMessagePolicyBeforeAuthorization(t *testing.T) {
	cfg := &config.Config{Profile: catalog.ProfileCore}
	for _, raw := range [][]byte{
		[]byte("feat: cite\n\n.awf/efforts/example/memory.md\n"),
		[]byte("not a conventional subject\n"),
	} {
		result, err := RunWithDependencies(context.Background(), t.TempDir(), cfg, nil, raw, Dependencies{})
		if err == nil || !result.HasDocument {
			t.Fatalf("RunWithDependencies(%q) = %#v, %v", raw, result, err)
		}
	}
	result, err := RunWithDependencies(context.Background(), t.TempDir(), cfg, nil, []byte("Merge branch 'x'\n"), Dependencies{})
	if err != nil || result.HasDocument {
		t.Fatalf("exempt core result = %#v, %v", result, err)
	}
}

func TestRunWithDependenciesPreservesAuthorizationOutcomes(t *testing.T) {
	failure := errors.New("injected failure")
	base := Dependencies{
		Authorize: func(context.Context, string, *git.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error) {
			return currentstatecoord.CommitAuthorizationResult{}, nil
		},
		Diagnostic:         AuthorizationDiagnostic,
		DiagnosticDocument: func(d presentation.Diagnostic) (presentation.Document, error) { return d.Document() },
	}
	cfg := &config.Config{Profile: catalog.ProfileFull}
	t.Run("diagnostic failures", func(t *testing.T) {
		for _, dependencies := range []Dependencies{
			{Authorize: base.Authorize, Diagnostic: func(currentstatecoord.CommitAuthorizationResult) (presentation.Diagnostic, error) {
				return presentation.Diagnostic{}, failure
			}, DiagnosticDocument: base.DiagnosticDocument},
			{Authorize: base.Authorize, Diagnostic: base.Diagnostic, DiagnosticDocument: func(presentation.Diagnostic) (presentation.Document, error) { return presentation.Document{}, failure }},
		} {
			dependencies.Authorize = func(context.Context, string, *git.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error) {
				return currentstatecoord.CommitAuthorizationResult{Category: "operation", Condition: "refused"}, nil
			}
			_, err := RunWithDependencies(context.Background(), t.TempDir(), cfg, nil, []byte("Merge x\n"), dependencies)
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
		}
	})
	for _, tc := range []struct {
		name   string
		result currentstatecoord.CommitAuthorizationResult
		err    error
		want   string
	}{
		{"next actions", currentstatecoord.CommitAuthorizationResult{Category: "operation", Condition: "refused", NextActions: []string{"fix it"}}, nil, "refused"},
		{"syntax error", currentstatecoord.CommitAuthorizationResult{Category: "operation", Condition: "refused"}, &commitmsg.SyntaxError{Line: 2, Reason: "bad"}, "refused"},
		{"other error", currentstatecoord.CommitAuthorizationResult{}, failure, "stale merge authorization:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dependencies := base
			dependencies.Authorize = func(context.Context, string, *git.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error) {
				return tc.result, tc.err
			}
			result, err := RunWithDependencies(context.Background(), t.TempDir(), cfg, nil, []byte("Merge x\n"), dependencies)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("result=%#v error=%v", result, err)
			}
			if tc.result.Category != "" && !result.HasDocument {
				t.Fatalf("diagnostic missing: %#v", result)
			}
		})
	}
}

func TestAuthorizationDiagnosticReportsActions(t *testing.T) {
	diagnostic, err := AuthorizationDiagnostic(currentstatecoord.CommitAuthorizationResult{Category: "operation", Condition: "refused", NextActions: []string{"correct it"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := diagnostic.Document(); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthorizationDiagnostic(currentstatecoord.CommitAuthorizationResult{NextActions: []string{""}}); err == nil {
		t.Fatal("empty action accepted")
	}
}
