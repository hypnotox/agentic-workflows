package bridge

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type MigrationHistoryEntry struct {
	Date, Key, Basis, Rationale string
	ADR                         string
}

var encodedHistoryRE = regexp.MustCompile("^- ([0-9]{4}-[0-9]{2}-[0-9]{2}): retired invariant `((?:ADR-[0-9]{4})#[a-z0-9]+(?:-[a-z0-9]+)*)`; basis: encoded$")
var migrationHistoryRE = regexp.MustCompile("^- ([0-9]{4}-[0-9]{2}-[0-9]{2}): retired invariant `((?:ADR-[0-9]{4})#[a-z0-9]+(?:-[a-z0-9]+)*)`; basis: migration; rationale: (.+)$")

// ParseMigrationHistory accepts one optional exact section outside Markdown fences.
func ParseMigrationHistory(data []byte, adrNumber string, effective map[string]Retirement) ([]MigrationHistoryEntry, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	sections, in, fence, fenceLen := 0, false, byte(0), 0
	var entries []MigrationHistoryEntry
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if marker, n, ok := historyFence(trim); ok {
			if fence == 0 {
				fence, fenceLen = marker, n
			} else if marker == fence && n >= fenceLen && strings.Trim(trim[n:], " \t") == "" {
				fence, fenceLen = 0, 0
			}
			continue
		}
		if fence != 0 {
			continue
		}
		if line == "## Migration history" {
			sections++
			if sections > 1 {
				return nil, fmt.Errorf("ADR-%s has more than one Migration history section", adrNumber)
			}
			in = true
			continue
		}
		if in && strings.HasPrefix(line, "## ") {
			in = false
		}
		if !in || trim == "" {
			continue
		}
		var e MigrationHistoryEntry
		if m := encodedHistoryRE.FindStringSubmatch(line); m != nil {
			e = MigrationHistoryEntry{Date: m[1], Key: m[2], Basis: "encoded", ADR: adrNumber}
			if _, ok := effective[e.Key]; !ok {
				return nil, fmt.Errorf("encoded migration history for %s has no effective retirement token", e.Key)
			}
		} else if m := migrationHistoryRE.FindStringSubmatch(line); m != nil {
			e = MigrationHistoryEntry{Date: m[1], Key: m[2], Basis: "migration", Rationale: strings.TrimSpace(m[3]), ADR: adrNumber}
			if e.Rationale == "" {
				return nil, fmt.Errorf("migration history for %s requires rationale", e.Key)
			}
		} else {
			return nil, fmt.Errorf("ADR-%s has malformed Migration history entry %q", adrNumber, line)
		}
		if _, err := time.Parse("2006-01-02", e.Date); err != nil {
			return nil, fmt.Errorf("migration history for %s has malformed date %q", e.Key, e.Date)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func historyFence(line string) (byte, int, bool) {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return 0, 0, false
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	return line[0], n, n >= 3
}
