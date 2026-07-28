package git

import "testing"

func TestParseWorktreeStatus(t *testing.T) {
	tracked, untracked, err := parseWorktreeStatus([]byte("\x00? loose.txt\x001 tracked.txt\x00u conflicted.txt\x002 renamed.txt\x00old.txt\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if tracked != 3 || untracked != 1 {
		t.Fatalf("counts = (%d, %d), want (3, 1)", tracked, untracked)
	}
}

func TestParseWorktreeStatusRejectsMalformedRecords(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "unterminated", body: "? loose.txt"},
		{name: "rename missing original", body: "2 renamed.txt\x00"},
		{name: "unknown type", body: "! ignored.txt\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseWorktreeStatus([]byte(tc.body)); err == nil {
				t.Fatal("parseWorktreeStatus unexpectedly succeeded")
			}
		})
	}
}

func TestWorktreeChangeCountsRejectsNonRepository(t *testing.T) {
	if _, _, err := WorktreeChangeCounts(t.TempDir()); err == nil {
		t.Fatal("WorktreeChangeCounts unexpectedly succeeded outside a repository")
	}
}
