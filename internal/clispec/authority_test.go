package clispec

import (
	"reflect"
	"testing"
)

func TestFocusedAuthorityCommandSpecs(t *testing.T) {
	read, ok := Lookup("read")
	if !ok || read.Gating != Gated {
		t.Fatalf("read spec = %#v, found=%v", read, ok)
	}
	topic, ok := read.Child("topic")
	if !ok || !topic.FullOnly || topic.MinPos != 1 || topic.MaxPos != 1 || !reflect.DeepEqual(topic.BoolFlags, []string{"--history", "--references", "--coverage"}) {
		t.Fatalf("read topic spec = %#v, found=%v", topic, ok)
	}
	adr, ok := read.Child("adr")
	if !ok || !adr.FullOnly || adr.MinPos != 1 || adr.MaxPos != 1 {
		t.Fatalf("read adr spec = %#v, found=%v", adr, ok)
	}
	resolve, ok := Lookup("resolve")
	if !ok || resolve.Gating != Gated {
		t.Fatalf("resolve spec = %#v, found=%v", resolve, ok)
	}
	resolved, ok := resolve.Child("topic")
	if !ok || !resolved.FullOnly || resolved.MinPos != 0 || resolved.MaxPos != -1 || !reflect.DeepEqual(resolved.BoolFlags, []string{"--uncovered"}) {
		t.Fatalf("resolve topic spec = %#v, found=%v", resolved, ok)
	}
	for _, c := range []Command{topic, adr, resolved} {
		if len(c.Help.Usage) == 0 || c.Help.Description == "" {
			t.Errorf("focused authority command lacks structured help: %#v", c)
		}
	}
}
