package effort

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestNewEffortMutationRejectsMissingIdentity(t *testing.T) {
	mutation := presentation.Mutation{Status: "completed"}
	if _, err := (Record{}).NewEffortMutation(mutation); err == nil {
		t.Fatal("missing effort identity accepted")
	}
}

func TestPartialFinishErrorDiagnostic(t *testing.T) {
	refusal := &managedTopologyError{message: "legacy message", actions: []RecoveryAction{{Text: "run `awf effort worktree remove demo`"}, {Text: "retry `awf effort finish demo`"}}}
	refusalDiagnostic, refusalErr := refusal.Diagnostic()
	if refusalErr != nil || refusalDiagnostic.Condition != "managed topology remains" || refusalDiagnostic.State != "topology" || len(refusalDiagnostic.Changed) != 2 {
		t.Fatalf("topology diagnostic=%#v err=%v", refusalDiagnostic, refusalErr)
	}
	refusalDocument, refusalDocumentErr := refusalDiagnostic.Document()
	if refusalDocumentErr != nil {
		t.Fatal(refusalDocumentErr)
	}
	var refusalOut bytes.Buffer
	if renderErr := presentation.Render(&refusalOut, refusalDocument); renderErr != nil {
		t.Fatal(renderErr)
	}
	const refusalWant = "condition: managed topology remains\nstate: topology\n\ndiagnostic:\n  changed:\n    active resident: no\n    managed topology: no\n  steps:\n    step 1: run `awf effort worktree remove demo`\n    step 2: retry `awf effort finish demo`\n"
	if refusalOut.String() != refusalWant {
		t.Fatalf("topology diagnostic = %q, want %q", refusalOut.String(), refusalWant)
	}
	err := &PartialFinishError{Result: FinishResult{State: FinishStateArchived, Reserved: true, Archived: true}, Cause: errors.New("disk fault"), Actions: []RecoveryAction{{Text: "retry `awf effort finish path/with  spaces`"}}}
	if err.Error() != "disk fault" {
		t.Fatalf("partial finish error = %q", err.Error())
	}
	diagnostic, diagnosticErr := err.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	document, documentErr := diagnostic.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	var out bytes.Buffer
	if renderErr := presentation.Render(&out, document); renderErr != nil {
		t.Fatal(renderErr)
	}
	const want = "condition: effort finish was interrupted\nstate: operation\ncause: disk fault\n\ndiagnostic:\n  changed:\n    active resident: no\n    finishing reservation: no\n    archived resident: yes\n    archive parent sync available: no\n    archive parent synced: no\n    efforts parent sync available: no\n    efforts parent synced: no\n  steps:\n    step 1: retry `awf effort finish path/with  spaces`\n"
	if out.String() != want {
		t.Fatalf("diagnostic = %q, want %q", out.String(), want)
	}
}

func TestFinishPresentationRejectsInvalidLiteralInputs(t *testing.T) {
	valid, err := (&PartialFinishError{Result: FinishResult{ArchivePath: ".awf/effort-archive/id-demo"}, Cause: errors.New("failure")}).Diagnostic()
	if err != nil || len(valid.Changed) != 8 {
		t.Fatalf("valid archive diagnostic=%#v err=%v", valid, err)
	}
	if _, err := (&PartialFinishError{Result: FinishResult{ArchivePath: "bad\npath"}, Cause: errors.New("failure")}).Diagnostic(); err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("partial archive path error = %v", err)
	}
	if _, err := (FinishResult{ArchivePath: ".awf/effort-archive/id-demo"}).FinishMutation(" \t\n"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("finish slug error = %v", err)
	}
	if _, err := (FinishResult{ArchivePath: "bad\npath"}).FinishMutation("demo"); err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("finish archive path error = %v", err)
	}
}

func TestRecoveryActionDiagnosticsRejectLineBreaks(t *testing.T) {
	if _, err := (&managedTopologyError{actions: []RecoveryAction{{Text: "retry\nnow"}}}).Diagnostic(); err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("topology action line break diagnostic error = %v", err)
	}
	if _, err := (&PartialFinishError{Actions: []RecoveryAction{{Text: "retry\nnow"}}}).Diagnostic(); err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("partial finish action line break diagnostic error = %v", err)
	}
}

func TestPartialFinishDiagnosticReportsEveryAxis(t *testing.T) {
	for _, test := range []struct {
		name   string
		result FinishResult
		want   string
	}{
		{"active", FinishResult{State: FinishStateActive}, "active resident: yes\n    finishing reservation: no\n    archived resident: no\n    archive parent sync available: no\n    archive parent synced: no\n    efforts parent sync available: no\n    efforts parent synced: no"},
		{"reserved", FinishResult{State: FinishStateReserved, Reserved: true}, "active resident: no\n    finishing reservation: yes\n    archived resident: no\n    archive parent sync available: no\n    archive parent synced: no\n    efforts parent sync available: no\n    efforts parent synced: no"},
		{"archived", FinishResult{State: FinishStateArchived, Archived: true}, "active resident: no\n    finishing reservation: no\n    archived resident: yes\n    archive parent sync available: no\n    archive parent synced: no\n    efforts parent sync available: no\n    efforts parent synced: no"},
		{"archived after reservation", FinishResult{State: FinishStateArchived, Reserved: true, Archived: true}, "active resident: no\n    finishing reservation: no\n    archived resident: yes\n    archive parent sync available: no\n    archive parent synced: no\n    efforts parent sync available: no\n    efforts parent synced: no"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostic, err := (&PartialFinishError{Result: test.result, Cause: errors.New("mechanism failed"), Actions: []RecoveryAction{{Text: "retry"}}}).Diagnostic()
			if err != nil {
				t.Fatal(err)
			}
			document, err := diagnostic.Document()
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := presentation.Render(&out, document); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), test.want) {
				t.Fatalf("diagnostic=%q missing axes=%q", out.String(), test.want)
			}
			result := test.result
			result.ArchivePath = ".awf/effort-archive/id-demo"
			mutation, err := result.FinishMutation("demo")
			if err != nil {
				t.Fatal(err)
			}
			gotChanges := 0
			for _, change := range mutation.Changes {
				gotChanges += len(change.Values)
			}
			wantChanges := 2 // unavailable parent syncs are reported as platform limits
			if test.result.Reserved {
				wantChanges++
			}
			if test.result.Archived {
				wantChanges++
			}
			if gotChanges != wantChanges {
				t.Fatalf("finish changes=%d, want %d", gotChanges, wantChanges)
			}
		})
	}
}

func TestProtocol2StaticStateAndPublicObject(t *testing.T) {
	// invariant: tooling/effort-management:effort-record-authority (TestProtocol2StaticStateAndPublicObject)
	created := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	record := Record{
		SchemaVersion: SchemaVersion,
		ID:            "018f47a0-7b3d-4c52-8f1a-123456789abc",
		Slug:          "ship-protocol-2",
		Title:         "Ship protocol 2",
		CreatedAt:     created,
		MemoryPath:    ".awf/efforts/ship-protocol-2/memory.md",
	}

	staticRaw, err := json.Marshal(persisted(record))
	if err != nil {
		t.Fatal(err)
	}
	wantStatic := `{"schemaVersion":2,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"ship-protocol-2","title":"Ship protocol 2","createdAt":"2026-07-29T12:00:00Z"}`
	if string(staticRaw) != wantStatic {
		t.Fatalf("static state = %s\nwant         = %s", staticRaw, wantStatic)
	}
	if strings.Contains(string(staticRaw), "memoryPath") {
		t.Fatalf("static state exposed public path: %s", staticRaw)
	}

	publicRaw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	wantPublic := `{"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"ship-protocol-2","title":"Ship protocol 2","createdAt":"2026-07-29T12:00:00Z","memoryPath":".awf/efforts/ship-protocol-2/memory.md"}`
	if string(publicRaw) != wantPublic {
		t.Fatalf("public effort = %s\nwant          = %s", publicRaw, wantPublic)
	}
}

func TestNewSlugPolicy(t *testing.T) {
	for _, slug := range []string{"a", "short-slug", strings.Repeat("a", 32)} {
		t.Run("accept-"+slug, func(t *testing.T) {
			if err := validateNewSlug(testContext(t), acceptEveryRefName, slug); err != nil {
				t.Fatalf("validateNewSlug(%q) = %v", slug, err)
			}
		})
	}
	for _, slug := range []string{"", "A", "a_b", "a/b", ".", "..", "a-", "-a", "a--b", strings.Repeat("a", 33)} {
		t.Run("reject-"+slug, func(t *testing.T) {
			err := validateNewSlug(testContext(t), acceptEveryRefName, slug)
			if err == nil || !strings.Contains(err.Error(), "changed bytes: no") || !strings.Contains(err.Error(), "--slug") {
				t.Fatalf("validateNewSlug(%q) = %v, want actionable unchanged refusal", slug, err)
			}
		})
	}
}

func TestResidentSlugPolicy(t *testing.T) {
	for _, slug := range []string{"a", strings.Repeat("a", 33), strings.Repeat("a", 63)} {
		if err := validateSlug(slug); err != nil {
			t.Fatalf("resident slug %q rejected: %v", slug, err)
		}
	}
	for _, slug := range []string{"", "A", "a_b", "a/b", ".", "..", "a-", "-a", "a--b", strings.Repeat("a", 64)} {
		t.Run("reject-"+slug, func(t *testing.T) {
			if err := validateSlug(slug); err == nil {
				t.Fatalf("validateSlug(%q) succeeded", slug)
			}
		})
	}
}

func TestOutcomeTitleByteBound(t *testing.T) {
	// The title is persisted verbatim, so its bound is asserted on both sides.
	// A 160-byte title whose slug stays short proves the slug bound does not
	// stand in for the title bound.
	atBound := strings.Repeat("a", 160)
	if got, err := normalizeTitle(atBound); err != nil || got != atBound {
		t.Fatalf("160-byte title rejected: %q, %v", got, err)
	}
	if _, err := normalizeTitle(strings.Repeat("a", 161)); err == nil || !strings.Contains(err.Error(), "160") {
		t.Fatalf("161-byte title accepted or unclear: %v", err)
	}
	// Multi-byte title content is validated independently from the explicit
	// ASCII slug, so its own byte boundary must reject this value.
	overlong := strings.Repeat("界", 80) + " ok"
	if _, err := normalizeTitle(overlong); err == nil {
		t.Fatal("overlong multi-byte title accepted")
	}
}

func TestPersistedProtocol2Validation(t *testing.T) {
	created := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	valid := persistedRecord{
		SchemaVersion: SchemaVersion,
		ID:            "018f47a0-7b3d-4c52-8f1a-123456789abc",
		Slug:          "valid-slug",
		Title:         "Valid slug",
		CreatedAt:     created,
	}
	if err := validatePersisted(valid, "valid-slug"); err != nil {
		t.Fatal(err)
	}

	mutations := map[string]func(*persistedRecord){
		"schema":         func(r *persistedRecord) { r.SchemaVersion = 1 },
		"uuid":           func(r *persistedRecord) { r.ID = "not-a-uuid" },
		"slug":           func(r *persistedRecord) { r.Slug = "other-slug" },
		"title":          func(r *persistedRecord) { r.Title = " " },
		"overlong title": func(r *persistedRecord) { r.Title = strings.Repeat("a", 161) },
		"createdAt":      func(r *persistedRecord) { r.CreatedAt = time.Time{} },
		// The non-UTC clause is asserted separately from the zero-time clause so
		// deleting it fails a test rather than being masked by IsZero.
		"non-UTC createdAt": func(r *persistedRecord) {
			r.CreatedAt = created.In(time.FixedZone("CEST", 2*60*60))
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := validatePersisted(candidate, "valid-slug"); err == nil {
				t.Fatal("invalid state accepted")
			}
		})
	}
}

// acceptEveryRefName isolates new-slug validation from the branch-name probe,
// whose own refusals are proven where the probe is injected.
func acceptEveryRefName(context.Context, string) (bool, error) { return true, nil }
