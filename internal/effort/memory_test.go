package effort

import (
	"strings"
	"testing"
)

func TestOwnedMemorySkeletonIsCoherentAndSlugged(t *testing.T) {
	raw := string(memorySkeleton("coherent-effort"))
	phrases := []string{
		"Effort: coherent-effort\n",
		"Phase: Not started.",
		"Next: Record the next concrete action.",
		"Updated: Not yet updated.",
		"## Brief",
		"durable artifacts",
		"## Decision log",
		"- D<n> <date> <phase> (user|autonomous)",
		"Record:",
		"## Observations",
		"when they happen",
		"## Handoff log",
		"No handoffs recorded.",
	}
	cursor := 0
	for _, phrase := range phrases {
		index := strings.Index(raw[cursor:], phrase)
		if index < 0 {
			t.Fatalf("memory skeleton missing ordered phrase %q:\n%s", phrase, raw)
		}
		cursor += index + len(phrase)
	}
}
