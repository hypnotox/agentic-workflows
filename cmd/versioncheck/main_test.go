package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name  string
		check func() error
		code  int
		out   string
		err   string
	}{
		{"valid", func() error { return nil }, 0, "versioncheck: version authority valid\n", ""},
		{"invalid", func() error { return errors.New("bad version") }, 1, "", "versioncheck: bad version\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if got := run(&out, &errb, tc.check); got != tc.code {
				t.Fatalf("code = %d, want %d", got, tc.code)
			}
			if got := out.String(); got != tc.out {
				t.Errorf("stdout = %q, want %q", got, tc.out)
			}
			if got := errb.String(); got != tc.err {
				t.Errorf("stderr = %q, want %q", got, tc.err)
			}
		})
	}
}
