package main

import (
	"bytes"
	"testing"
)

func TestCommandOutputBoundary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command exit = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("usage output streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
