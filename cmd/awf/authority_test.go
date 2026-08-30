package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRetiredADRCommandsAreRejected(t *testing.T) {
	for _, args := range [][]string{{"awf", "adr", "number"}, {"awf", "read", "adr", "0001"}, {"awf", "new", "adr", "Decision"}} {
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code != 2 || out.Len() != 0 {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q", args, code, out.String(), stderr.String())
		}
	}
}

func TestRunResolveTopicUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "resolve", "topic"}, &out, &errb); code != 2 {
		t.Fatalf("exit = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "awf resolve topic <path>...") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// invariant: tooling/authority-queries:authority-read-projections (TestReadTopicExposesOnlyReferencesAndCoverage)
func TestReadTopicExposesOnlyReferencesAndCoverage(t *testing.T) {
	root := topicCmdFixture(t)
	var out, stderr bytes.Buffer
	if code := runFrom(root, []string{"awf", "read", "topic", "schedule/contracts", "--history"}, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "unknown flag \"--history\"") {
		t.Fatalf("history flag exit=%d stdout=%q stderr=%q", code, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := runFrom(root, []string{"awf", "read", "topic", "schedule/contracts", "--references", "--coverage"}, &out, &stderr); code != 0 {
		t.Fatalf("retained flags exit=%d stderr=%q", code, stderr.String())
	}
}
