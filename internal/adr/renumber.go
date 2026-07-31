package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RenumberPending renames the pending record `<slug>.md` under dir to
// `NNNN-<slug>.md` and rewrites its `# ADR-<slug>:` heading to `# ADR-NNNN:`.
// Nothing else moves: the retained `slug:` frontmatter key, every body byte, and
// every Status history event survive verbatim, which is what makes numbering
// digest-safe and history-free (ADR-0194 item 9).
//
// It is the rewrite seam the numbering command performs its rename through, and
// the only reader and writer of a decision record's path outside this package's
// corpus construction seams. Keeping it here is what lets the command live in
// internal/project without joining the enumerated raw-bytes accessors
// (adr-system/adr-lifecycle:corpus-raw-access-enumerated).
func RenumberPending(dir, slug string, number int) error {
	from := filepath.Join(dir, slug+".md")
	data, err := os.ReadFile(from)
	if err != nil {
		return fmt.Errorf("adr: read pending record %s: %w", slug, err)
	}
	numbered := fmt.Sprintf("%04d", number)
	oldHeading, newHeading := "# ADR-"+slug+":", "# ADR-"+numbered+":"
	lines := strings.SplitAfter(string(data), "\n")
	rewritten := false
	for i, line := range lines {
		if !strings.HasPrefix(line, oldHeading) {
			continue
		}
		lines[i] = newHeading + strings.TrimPrefix(line, oldHeading)
		rewritten = true
		break
	}
	if !rewritten {
		return fmt.Errorf("adr: pending record %s has no %q heading to renumber", slug, oldHeading)
	}
	to := filepath.Join(dir, numbered+"-"+slug+".md")
	if err := os.WriteFile(to, []byte(strings.Join(lines, "")), 0o644); err != nil { // coverage-ignore: the numbered path sits beside the file just read, so a write fails only on a permission fault a test cannot trigger
		return err
	}
	return os.Remove(from)
}
