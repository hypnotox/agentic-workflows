package render_test

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

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
