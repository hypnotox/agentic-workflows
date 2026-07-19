package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseApprovals(t *testing.T) {
	absent, err := ParseApprovals(nil, false)
	if err != nil || absent.Present {
		t.Fatalf("absent: %#v %v", absent, err)
	}
	for _, src := range []string{
		"version: 1\ninvariantApprovals: []\n",
		"version: 1\ninvariantApprovals:\n  - key: ADR-0001#stable\n    destination: core/contracts:stable\n",
	} {
		got, err := ParseApprovals([]byte(src), true)
		if err != nil || !got.Present {
			t.Fatalf("valid %q: %#v %v", src, got, err)
		}
	}
	bad := []string{
		"[", "[x]\n", "1: x\nversion: 1\ninvariantApprovals: []\n", "version: 2\ninvariantApprovals: []\n", "version: '1'\ninvariantApprovals: []\n",
		"version: 1\n", "invariantApprovals: []\n", "version: 1\nextra: x\ninvariantApprovals: []\n",
		"version: 1\nversion: 1\ninvariantApprovals: []\n", "version: 1\ninvariantApprovals: {}\n",
		"version: 1\ninvariantApprovals:\n  - x\n", "version: 1\ninvariantApprovals:\n  - key: 1\n    destination: core/x:y\n",
		"version: 1\ninvariantApprovals:\n  - key: ''\n    destination: core/x:y\n", "version: 1\ninvariantApprovals:\n  - key: ADR-1#x\n    destination: core/x:y\n",
		"version: 1\ninvariantApprovals:\n  - key: ADR-0001#x\n    destination: bad\n", "version: 1\ninvariantApprovals:\n  - key: ADR-0001#x\n    key: ADR-0001#x\n    destination: core/x:y\n",
		"version: 1\ninvariantApprovals:\n  - key: ADR-0001#x\n    destination: core/x:y\n    extra: no\n", "version: 1\ninvariantApprovals:\n  - key: ADR-0001#x\n",
		"version: 1\ninvariantApprovals:\n  - key: ADR-0001#x\n    destination: core/x:y\n  - key: ADR-0001#x\n    destination: core/x:y\n",
		"version: 1\ninvariantApprovals: []\n---\nversion: 1\ninvariantApprovals: []\n",
	}
	for _, src := range bad {
		if _, err := ParseApprovals([]byte(src), true); err == nil {
			t.Errorf("accepted invalid schema:\n%s", src)
		}
	}
}

func TestLoadApprovalsReadError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ApprovalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApprovals(root); err == nil {
		t.Fatal("directory approval accepted")
	}
}

func TestApplyApprovals(t *testing.T) {
	live := Inventory{Entries: []LegacyInvariant{{Key: "ADR-0001#x", Active: true}}}
	m := []Mapping{{Key: "ADR-0001#x", Destination: "core/x:x"}}
	for _, tc := range []struct {
		name      string
		inv       Inventory
		approvals Approvals
		want      string
	}{
		{"absent", live, Approvals{}, "required"}, {"missing", live, Approvals{Present: true}, "missing"},
		{"unknown", live, Approvals{Present: true, Entries: []Approval{{"ADR-0002#x", "core/x:x"}}}, "unknown"},
		{"mismatch", live, Approvals{Present: true, Entries: []Approval{{"ADR-0001#x", "core/x:y"}}}, "derived"},
		{"duplicate", live, Approvals{Present: true, Entries: []Approval{{"ADR-0001#x", "core/x:x"}, {"ADR-0001#x", "core/x:x"}}}, "duplicate"},
		{"retired", Inventory{Entries: []LegacyInvariant{{Key: "ADR-0001#x", Active: false, History: &MigrationHistoryEntry{Basis: "migration"}}}}, Approvals{Present: true, Entries: []Approval{{"ADR-0001#x", "core/x:x"}}}, "forbids"},
		{"retired-no-history", Inventory{Entries: []LegacyInvariant{{Key: "ADR-0001#x", Active: false}}}, Approvals{Present: true}, "lacks"},
	} {
		_, err := ApplyApprovals(tc.inv, m, tc.approvals)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	got, err := ApplyApprovals(live, m, Approvals{Present: true, Entries: []Approval{{"ADR-0001#x", "core/x:x"}}})
	if err != nil || !got[0].Approved {
		t.Fatalf("valid: %#v %v", got, err)
	}
}
