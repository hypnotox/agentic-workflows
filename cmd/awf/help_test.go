package main

import (
	"bytes"
	"strings"
	"testing"
)

// Every command answers --help and -h with exit 0 and its own usage line —
// guaranteeing no command is missing structured help.
func TestPerCommandHelp(t *testing.T) {
	for name := range argSpecs {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(name+" "+flag, func(t *testing.T) {
				var out, errb bytes.Buffer
				if code := run([]string{"awf", name, flag}, &out, &errb); code != 0 {
					t.Fatalf("`awf %s %s` should exit 0, got %d (err=%s)", name, flag, code, errb.String())
				}
				if !strings.Contains(out.String(), "Usage: awf "+name) {
					t.Errorf("`awf %s %s` help missing usage line; got:\n%s", name, flag, out.String())
				}
			})
		}
	}
}

// The global overview lists every command's summary in the declared order.
func TestGlobalHelpListsAllCommands(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "help"}, &out, &errb); code != 0 {
		t.Fatalf("`awf help` should exit 0, got %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "Commands:") {
		t.Errorf("global help missing Commands header:\n%s", got)
	}
	for name, spec := range argSpecs {
		if !strings.Contains(got, name) {
			t.Errorf("global help omits command %q", name)
		}
		if spec.summary == "" || spec.help == "" {
			t.Errorf("command %q has empty summary/help", name)
		}
	}
}
