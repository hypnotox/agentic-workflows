package adr

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// digestSections is the ordered, canonical section set the content-sha256
// covers (ADR-0135 item 6): everything except frontmatter and Status history.
var digestSections = []string{"Context", "Decision", "State changes", "Consequences", "Alternatives Considered"}

// ContentDigest computes the current-state-v1 content-sha256 over the five
// canonical sections in fixed order, excluding frontmatter and Status history.
// Each section is serialized as its heading line followed by its body with
// trailing whitespace stripped, so cosmetic trailing-blank-line noise does not
// change the digest while any substantive edit does. Each stamped history event
// records this value as of its own append; the latest stamp must always match
// it, and a terminal status freezes it permanently (ADR-0186). awf both computes
// and re-verifies it, so this canonical form is the single source of truth.
func ContentDigest(sections map[string]string) string {
	var b strings.Builder
	for _, name := range digestSections {
		b.WriteString("## ")
		b.WriteString(name)
		b.WriteByte('\n')
		b.WriteString(strings.TrimRight(sections[name], " \t\r\n"))
		b.WriteByte('\n')
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(b.String())))
}
