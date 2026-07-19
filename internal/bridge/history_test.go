package bridge

import (
	"strings"
	"testing"
)

func TestParseMigrationHistory(t *testing.T) {
	effective := map[string]Retirement{"ADR-0001#old": {}}
	valid := "## Context\n\n```md\n## Migration history\n- nope\n```\n\n## Migration history\n\n- 2026-07-19: retired invariant `ADR-0001#old`; basis: encoded\n- 2026-07-20: retired invariant `ADR-0002#gone`; basis: migration; rationale: reviewed removal\n"
	got, err := ParseMigrationHistory([]byte(valid), "0003", effective)
	if err != nil || len(got) != 2 || got[1].Rationale != "reviewed removal" {
		t.Fatalf("valid: %#v %v", got, err)
	}
	for _, src := range []string{
		"## Migration history\n\n- 2026-99-01: retired invariant `ADR-0001#old`; basis: encoded\n",
		"## Migration history\n\n- 2026-07-19: retired invariant `ADR-0009#x`; basis: encoded\n",
		"## Migration history\n\n- 2026-07-19: retired invariant `ADR-0001#old`; basis: other\n",
		"## Migration history\n\n- 2026-07-19: retired invariant `ADR-0001#old`; basis: migration; rationale:  \n",
		"## Migration history\n\ntext\n",
		"## Migration history\n\n## Other\n\n## Migration history\n",
	} {
		if _, err := ParseMigrationHistory([]byte(src), "0001", effective); err == nil {
			t.Errorf("accepted invalid: %q", src)
		}
	}
	if got, err := ParseMigrationHistory([]byte("## Context\nnone\n"), "0001", effective); err != nil || got != nil {
		t.Fatalf("absent: %#v %v", got, err)
	}
}

func TestMigrationHistoryInsertOffset(t *testing.T) {
	for _, tc := range []struct {
		src    string
		exists bool
		tail   string
	}{
		{"body", false, ""}, {"## Migration history\n\nold\n", true, ""}, {"## Migration history\n\nold\n## Consequences\nrest\n", true, "## Consequences"},
	} {
		off, exists := migrationHistoryInsertOffset([]byte(tc.src))
		if exists != tc.exists {
			t.Errorf("%q exists=%v", tc.src, exists)
		}
		if tc.tail != "" && !strings.HasPrefix(tc.src[off:], tc.tail) {
			t.Errorf("offset %d in %q", off, tc.src)
		}
	}
}
