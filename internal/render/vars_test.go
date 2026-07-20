package render_test

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

func TestReferencedDataKeys(t *testing.T) {
	src := "{{ .data.a }} {{ with .data.b }}{{ .data.a.c }}{{ end }}"
	got := render.ReferencedDataKeys(src)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReferencesBareData(t *testing.T) {
	for src, want := range map[string]bool{
		"{{ range .data }}{{ . }}{{ end }}": true,
		"{{ index .data \"k\" }}":           true,
		"{{ .data }}":                       true,
		"{{ .data.a }}":                     false,
		"{{ .database }}":                   false,
	} {
		if got := render.ReferencesBareData(src); got != want {
			t.Errorf("ReferencesBareData(%q) = %v, want %v", src, got, want)
		}
	}
}

func TestReferencesBareVars(t *testing.T) {
	for src, want := range map[string]bool{
		"{{ range .vars }}{{ . }}{{ end }}": true,
		"{{ .vars.gateCmd }}":               false,
		"{{ .variant }}":                    false,
	} {
		if got := render.ReferencesBareVars(src); got != want {
			t.Errorf("ReferencesBareVars(%q) = %v, want %v", src, got, want)
		}
	}
}

func TestPlaceholderVarRefs(t *testing.T) {
	body := "run {{=awf:gateCmd}} then {{=awf:checkCmd}} and {{=awf:gateCmd}}"
	got := render.PlaceholderVarRefs(body)
	want := []string{"checkCmd", "gateCmd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := render.PlaceholderVarRefs("uses {{=awf:prefix}} only"); len(got) != 0 {
		t.Fatalf("non-var placeholders must not count, got %v", got)
	}
	if got := render.PlaceholderVarRefs(`documents \{{=awf:gateCmd}} literally`); len(got) != 0 {
		t.Fatalf("an ADR-0058-escaped token renders literally and reads no var, got %v", got)
	}
}

func TestReferencedVars(t *testing.T) {
	src := "{{ .vars.gateCmd }} {{ if .vars.adrDir }} {{ .vars.gateCmd }}"
	got := render.ReferencedVars(src)
	want := []string{"adrDir", "gateCmd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReferencedVarsEmpty(t *testing.T) {
	got := render.ReferencedVars("no vars here")
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestReferencedVarsDeduplicated(t *testing.T) {
	src := "{{ .vars.foo }} {{ .vars.foo }} {{ .vars.bar }}"
	got := render.ReferencedVars(src)
	want := []string{"bar", "foo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReferencesScopes(t *testing.T) {
	if !render.ReferencesScopes("x {{ with .commitScopes }}y{{ end }} z") {
		t.Error("expected a .commitScopes action to be detected")
	}
	if render.ReferencesScopes("prose mentioning .commitScopes outside an action") {
		t.Error("a non-action mention must not match")
	}
}
