package effort

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMemoryMetadataReadersAreDeliberatelySeparate(t *testing.T) {
	t.Parallel()
	canonical := []byte("---\neffort: sample\nphase: \"  padded  \"\nnext: next!\nupdated: \"2026-08-02T12:00:00.123456789Z\"\n---\nbody\r\n")
	metadata, body, err := readMemoryMetadata(canonical, "sample")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Phase != "  padded  " || string(body) != "body\r\n" {
		t.Fatalf("metadata=%#v body=%q", metadata, body)
	}
	// Identity users deliberately remain available despite invalid mutable data.
	invalidMutable := []byte("---\neffort: sample\nphase: \"\"\nnext: \"\"\nupdated: nope\n---\nbody")
	if err := readMemoryIdentity(invalidMutable, "sample"); err != nil {
		t.Fatalf("identity reader rejected mutable damage: %v", err)
	}
	if _, _, err := readMemoryMetadata(invalidMutable, "sample"); err == nil {
		t.Fatal("metadata reader accepted invalid mutable fields")
	}
	if err := readMemoryIdentity([]byte("Effort: sample\nanything"), "sample"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryMetadataRejectsClosedSchemaHazards(t *testing.T) {
	t.Parallel()
	cases := []string{
		"---\neffort: sample\neffort: sample\nphase: a\nnext: b\nupdated: \"2026-08-02T12:00:00Z\"\n---\n",
		"---\neffort: sample\nphase: a\nnext: b\nupdated: \"2026-08-02T12:00:00Z\"\nextra: no\n---\n",
		"---\neffort: [sample]\nphase: a\nnext: b\nupdated: \"2026-08-02T12:00:00Z\"\n---\n",
	}
	for _, raw := range cases {
		if _, _, err := readMemoryMetadata([]byte(raw), "sample"); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if _, _, err := readMemoryMetadata([]byte(strings.Repeat("x", maxMemoryBytes+1)), "sample"); err == nil {
		t.Fatal("accepted oversized memory")
	}
}

func TestUpdateMemoryRejectsOversizedResidentWithoutChangingBytes(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "oversized-update", Title: "Oversized update"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("oversized-update")
	before := []byte(strings.Repeat("x", maxMemoryBytes+1))
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	phase := "replacement"
	if err := service.UpdateMemory("oversized-update", MemoryUpdate{Phase: &phase}); err == nil {
		t.Fatal("accepted oversized memory resident")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("oversized update changed bytes: %v", err)
	}
}

func TestUpdateMemoryMigratesLegacyAndPreservesBody(t *testing.T) {
	t.Parallel()
	root := initEffortRepo(t)
	now := time.Date(2026, 8, 2, 12, 0, 0, 123456789, time.UTC)
	service := openTestService(t, root, func(deps *Dependencies) { deps.Clock = func() time.Time { return now } })
	if _, err := service.New(testContext(t), NewInput{Slug: "migration", Title: "Migration"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "efforts", "migration", "memory.md")
	body := "## Brief\r\n\r\nbody bytes stay exact\r\n"
	legacy := "Effort: migration\nPhase: old\nNext: old next\nUpdated: Not yet updated.\n\n" + body
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	phase := "  punctuation: [] {} #  "
	if err := service.UpdateMemory("migration", MemoryUpdate{Phase: &phase}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	metadata, gotBody, err := readMemoryMetadata(raw, "migration")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Phase != phase || metadata.Next != "old next" || metadata.Updated != now.Format(time.RFC3339Nano) {
		t.Fatalf("metadata=%#v", metadata)
	}
	if string(gotBody) != body {
		t.Fatalf("body changed: %q", gotBody)
	}
	if !strings.HasPrefix(string(raw), "---\n") {
		t.Fatalf("not migrated: %q", raw)
	}
}

func TestUpdateMemoryRepairsOnlySuppliedInvalidMutableFields(t *testing.T) {
	t.Parallel()
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "repair", Title: "Repair"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "efforts", "repair", "memory.md")
	if err := os.WriteFile(path, []byte("---\neffort: repair\nphase: \"\"\nnext: \"\"\nupdated: nope\n---\nbody"), 0o600); err != nil {
		t.Fatal(err)
	}
	phase := "fixed"
	err := service.UpdateMemory("repair", MemoryUpdate{Phase: &phase})
	var invalid *InvalidMemoryError
	if !errors.As(err, &invalid) || !strings.Contains(invalid.NextAction, "--next <replacement-next>") || !strings.Contains(invalid.NextAction, "--phase <replacement-phase>") {
		t.Fatalf("error=%v", err)
	}
	next := "fixed next"
	if err := service.UpdateMemory("repair", MemoryUpdate{Phase: &phase, Next: &next}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMemoryRepairsNonScalarMutableField(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "non-scalar", Title: "Non scalar"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "efforts", "non-scalar", "memory.md")
	raw := "---\neffort: non-scalar\nphase: [not, scalar]\nnext: valid\nupdated: \"2026-08-02T12:00:00Z\"\n---\nbody"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	phase := "repaired"
	if err := service.UpdateMemory("non-scalar", MemoryUpdate{Phase: &phase}); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryMetadataGrammarAndErrorContracts(t *testing.T) {
	t.Parallel()
	valid := "effort: sample\nphase: ok\nnext: next\nupdated: \"2026-08-02T12:00:00Z\"\n"
	// Each form is independently forbidden by the closed codec; identity-only
	// callers retain only the deliberately documented mutable-field tolerance.
	for name, raw := range map[string]string{
		"no-frontmatter": "no header", "wrong-identity": "---\n" + strings.Replace(valid, "sample", "other", 1) + "---\n",
		"tag":                "---\n!x effort: sample\nphase: ok\nnext: next\nupdated: x\n---\n",
		"alias":              "---\neffort: &x sample\nphase: *x\nnext: next\nupdated: x\n---\n",
		"multiple-documents": "---\n" + valid + "...\n" + valid + "---\n",
		"missing-field":      "---\neffort: sample\nphase: ok\nnext: next\n---\n",
		"non-scalar-mutable": "---\neffort: sample\nphase: [x]\nnext: next\nupdated: x\n---\n",
		"blank":              "---\neffort: sample\nphase: \" \"\nnext: next\nupdated: x\n---\n",
		"newline":            "---\neffort: sample\nphase: |\n  two\n  lines\nnext: next\nupdated: x\n---\n",
		"legacy-boundary":    "Effort: sample\nPhase: ok\nNext: next\nUpdated: x\nbody",
		"legacy-order":       "Phase: ok\nEffort: sample\nNext: next\nUpdated: x\n\nbody",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := readMemoryMetadata([]byte(raw), "sample"); err == nil {
				t.Fatal("accepted malformed metadata")
			}
		})
	}
	for _, value := range []string{"", " \t", "a\nb", "a\rb", strings.Repeat("a", 501)} {
		if validateMemoryMutable(value) == nil {
			t.Fatalf("mutable value accepted: %q", value)
		}
	}
	for _, value := range []string{"Not yet updated.", "2026-08-02T12:00:00+00:00", "nope"} {
		if validateUpdated(value, false) == nil {
			t.Fatalf("canonical updated accepted: %q", value)
		}
	}
	if validateUpdated(notYetUpdated, true) != nil || formatMemoryTime(time.Date(2026, 8, 2, 12, 0, 0, 0, time.FixedZone("x", 3600))) != "2026-08-02T11:00:00Z" {
		t.Fatal("updated normalization")
	}
	if _, err := encodeMemory(MemoryMetadata{Effort: "sample", Phase: "[]", Next: "#", Updated: "2026-08-02T12:00:00Z"}, []byte("body")); err != nil {
		t.Fatal(err)
	}
	err := &InvalidMemoryError{Slug: "sample", NextAction: "repair", Err: errors.New("broken")}
	if !errors.Is(err, err.Err) || !strings.Contains(err.Error(), "changed bytes: no") {
		t.Fatalf("invalid error = %v", err)
	}
}

func TestUpdateMemoryRejectsEveryUnsafeRepairBoundary(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "unsafe-update", Title: "Unsafe update"}); err != nil {
		t.Fatal(err)
	}
	value := "ok"
	for _, update := range []MemoryUpdate{{}, {Phase: ptr("\n")}, {Next: ptr(" ")}} {
		if err := service.UpdateMemory("unsafe-update", update); err == nil {
			t.Fatalf("accepted update %#v", update)
		}
	}
	path := filepath.Join(root, ".awf", "efforts", "unsafe-update", "memory.md")
	for _, raw := range []string{
		"---\neffort: other\nphase: x\nnext: y\nupdated: x\n---\nbody",
		"not a recognizable memory",
		"---\neffort: unsafe-update\nphase: \"\"\nnext: y\nupdated: x\n---\nbody",
	} {
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		err := service.UpdateMemory("unsafe-update", MemoryUpdate{Next: &value})
		var invalid *InvalidMemoryError
		if !errors.As(err, &invalid) {
			t.Fatalf("unsafe repair %q = %v", raw, err)
		}
	}
	if err := os.WriteFile(path, []byte("---\neffort: unsafe-update\nphase: \"\"\nnext: \"\"\nupdated: x\n---\nbody"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateMemory("unsafe-update", MemoryUpdate{Phase: &value, Next: &value}); err != nil {
		t.Fatal(err)
	}
}

func ptr(s string) *string { return &s }

func TestMemoryCodecIdentityAndYAMLEdgeGrammar(t *testing.T) {
	if err := readMemoryIdentity([]byte("---\neffort: sample\nphase: [broken]\n---\n"), "sample"); err != nil {
		t.Fatalf("identity should ignore mutable metadata: %v", err)
	}
	if err := readMemoryIdentity([]byte("Effort: other\n"), "sample"); err == nil {
		t.Fatal("legacy mismatch accepted")
	}
	if _, _, err := readMemoryMetadata([]byte("---\neffort: sample\nphase: ok\nnext: ok\nupdated: \"2026-08-02T12:00:00Z\"\n---\n\xff"), "sample"); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	if got, err := encodeMemory(MemoryMetadata{Effort: "sample", Phase: " leading: #[]", Next: "quote \\\"", Updated: "2026-08-02T12:00:00Z"}, []byte("body")); err != nil || !strings.HasPrefix(string(got), "---\n") || !strings.Contains(string(got), "effort:") {
		t.Fatalf("canonical encoding = %q, %v", got, err)
	}
}

func TestMemoryMetadataExactParserHazards(t *testing.T) {
	for name, raw := range map[string][]byte{
		"identity-too-large":    []byte(strings.Repeat("x", maxMemoryBytes+1)),
		"identity-invalid-utf8": []byte("\xff"),
		"identity-bad-yaml":     []byte("---\n[\n---\n"),
		"identity-other":        []byte("---\neffort: other\n---\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := readMemoryIdentity(raw, "sample"); err == nil {
				t.Fatal("unsafe identity accepted")
			}
		})
	}
	for name, raw := range map[string][]byte{
		"legacy-identity":   []byte("Effort: other\nPhase: a\nNext: b\nUpdated: Not yet updated.\n\nbody"),
		"legacy-phase":      []byte("Effort: sample\nPhase: \nNext: b\nUpdated: Not yet updated.\n\nbody"),
		"legacy-next":       []byte("Effort: sample\nPhase: a\nNext: \nUpdated: Not yet updated.\n\nbody"),
		"legacy-updated":    []byte("Effort: sample\nPhase: a\nNext: b\nUpdated: bad\n\nbody"),
		"legacy-five-lines": []byte("Effort: sample\nPhase: a\nNext: b\nUpdated: x\nExtra: y\n\nbody"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := readMemoryMetadata(raw, "sample"); err == nil {
				t.Fatal("unsafe legacy accepted")
			}
		})
	}
	if _, err := yamlIdentity([]byte("phase: ok")); err == nil {
		t.Fatal("identity without effort accepted")
	}
	if _, err := yamlMapping([]byte("[")); err == nil {
		t.Fatal("malformed mapping accepted")
	}
	if _, err := yamlMapping([]byte("a: b\n---\nc: d\n")); err == nil {
		t.Fatal("multiple documents accepted")
	}
	if _, err := yamlMapping([]byte("- item\n")); err == nil {
		t.Fatal("sequence mapping accepted")
	}
	for name, raw := range map[string][]byte{
		"yaml-parse":         []byte("["),
		"yaml-sequence":      []byte("- sample"),
		"identity-duplicate": []byte("effort: sample\neffort: sample"),
		"identity-tag":       []byte("effort: !tag sample"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := yamlIdentity(raw); err == nil {
				t.Fatal("unsafe identity YAML accepted")
			}
		})
	}
	if _, err := encodeMemory(MemoryMetadata{Effort: "sample", Phase: "ok", Next: "next", Updated: "2026-08-02T12:00:00Z"}, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "oversized")
	if err := os.WriteFile(path, []byte("12"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegularNoFollowBounded(path, 1); err == nil {
		t.Fatal("over-bound read accepted")
	}
}

func TestUpdateMemoryPreRenameFaultPreservesBytes(t *testing.T) {
	for _, stage := range []string{"memory-update.write", "memory-update.fsync", "memory-update.rename"} {
		t.Run(stage, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(deps *Dependencies) {
				deps.Fault = func(got string) error {
					if got == stage {
						return errors.New("stop")
					}
					return nil
				}
			})
			if _, err := service.New(testContext(t), NewInput{Slug: "atomic", Title: "Atomic"}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, ".awf", "efforts", "atomic", "memory.md")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			phase := "changed"
			if err := service.UpdateMemory("atomic", MemoryUpdate{Phase: &phase}); err == nil {
				t.Fatal("fault did not stop update")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("pre-rename failure changed memory")
			}
		})
	}
}
