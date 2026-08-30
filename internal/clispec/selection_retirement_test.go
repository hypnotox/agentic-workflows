package clispec

import "testing"

func TestSelectionRetirementCommandGrammar(t *testing.T) {
	for _, retired := range []string{"enable", "disable", "target"} {
		if _, ok := Lookup(retired); ok {
			t.Fatalf("retired command %q is declared", retired)
		}
	}
	newCommand, ok := Lookup("new")
	if !ok {
		t.Fatal("new command is absent")
	}
	var kinds []string
	for _, child := range newCommand.Children {
		kinds = append(kinds, child.Name)
	}
	want := []string{"topic", "domain", "doc", "pitfall"}
	if len(kinds) != len(want) {
		t.Fatalf("new kinds = %v, want %v", kinds, want)
	}
	for i, kind := range want {
		if kinds[i] != kind {
			t.Fatalf("new kinds = %v, want %v", kinds, want)
		}
	}
	remove, ok := Lookup("remove")
	if !ok || len(remove.Children) != 1 || remove.Children[0].Name != "domain" {
		t.Fatalf("remove grammar = %#v", remove)
	}
}
