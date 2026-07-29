package effort

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProtocol2StaticStateAndPublicObject(t *testing.T) {
	// invariant: tooling/effort-management:effort-record-authority
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

func TestSlugDerivationAndValidation(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{title: "Ship Protocol 2", want: "ship-protocol-2"},
		{title: "  Alpha---BETA___42  ", want: "alpha-beta-42"},
		{title: "a🙂界b", want: "a-b"},
		{title: "one\t\n two", want: "one-two"},
		{title: "123", want: "123"},
	}
	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			got, err := deriveSlug(test.title)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("deriveSlug(%q) = %q, want %q", test.title, got, test.want)
			}
			if err := validateSlug(got); err != nil {
				t.Fatalf("derived slug is invalid: %v", err)
			}
		})
	}

	for _, title := range []string{
		"界🙂",
		strings.Repeat("a", 64),
		string([]byte{0xff}),
	} {
		t.Run("reject-title", func(t *testing.T) {
			_, err := deriveSlug(title)
			if err == nil || !strings.Contains(err.Error(), "shorter outcome title with ASCII words or digits") {
				t.Fatalf("error = %v, want actionable ASCII-title repair", err)
			}
		})
	}

	for _, slug := range []string{"", "A", "a_b", "a/b", ".", "..", "a-", "-a", strings.Repeat("a", 64)} {
		t.Run("reject-slug-"+slug, func(t *testing.T) {
			if err := validateSlug(slug); err == nil {
				t.Fatalf("validateSlug(%q) succeeded", slug)
			}
		})
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
		"schema":    func(r *persistedRecord) { r.SchemaVersion = 1 },
		"uuid":      func(r *persistedRecord) { r.ID = "not-a-uuid" },
		"slug":      func(r *persistedRecord) { r.Slug = "other-slug" },
		"title":     func(r *persistedRecord) { r.Title = " " },
		"createdAt": func(r *persistedRecord) { r.CreatedAt = time.Time{} },
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
