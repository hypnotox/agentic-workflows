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
// it, and a terminal status freezes it permanently (ADR-0188). awf both computes
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

// CanonicalDigest answers this record's content-sha256 from its own parsed
// sections, so a caller outside this package never reads them itself. The value
// is computed rather than taken from the latest stamped history event: a stamp
// can sit on an Amended event in the middle of a history whose trailing Applied
// events carry none, and a record whose latest stamp disagrees with the computed
// value does not parse, so the two never differ for a record that exists.
//
// The frontmatter and the `# ADR-NNNN:` heading are outside every covered
// section, which is what leaves this value fixed across a renumber and makes it
// the key a transition pairs a slugless record on.
func (a ADR) CanonicalDigest() string { return ContentDigest(a.Sections) }
