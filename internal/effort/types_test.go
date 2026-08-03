package effort

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

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
	// Many multi-byte runes collapse to one hyphen, so the derived slug is tiny
	// while the title is far past the bound.
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
