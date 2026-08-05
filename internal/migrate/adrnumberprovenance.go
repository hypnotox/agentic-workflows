package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

var (
	// appliedSequenceRe matches the retired segment inside an Applied event
	// line; stripping it restores the sequence-free grammar (ADR-0191 item 9).
	appliedSequenceRe = regexp.MustCompile(`(?m)^(- \d{4}-\d{2}-\d{2}: Applied; )state-sequence: [1-9][0-9]*; (operations: [^\n]*)$`)
	// statusSequenceRe matches the retired segment inside a status event's
	// metadata tail, where it sits after the digest and before any rationale.
	statusSequenceRe = regexp.MustCompile(`(?m)^(- \d{4}-\d{2}-\d{2}: (?:Proposed|Accepted|Implementing|Implemented|Abandoned)[^\n]*?); state-sequence: [1-9][0-9]*((?:; rationale: [^\n]*)?)$`)
	// residualSequenceRe detects any state-sequence text the two rewrites left
	// behind, which means a malformed line this migration must not guess at. A
	// terminal rationale is free prose and is cut before the scan, so a
	// rationale that merely mentions the term never aborts the migration.
	residualSequenceRe = regexp.MustCompile(`(?m)^[^\n]*state-sequence[^\n]*$`)
	// revisedByLineRe matches one canonical Revised-by provenance line in a
	// topic part.
	revisedByLineRe = regexp.MustCompile(`(?m)^Revised-by: (ADR-[0-9]{4}(?:, ADR-[0-9]{4})*)$`)
	// statusHistoryHeading bounds the surgery: only event lines inside the
	// Status history section are rewritten, never a quoted example in a body.
	statusHistoryHeading = "\n## Status history\n"
)

// applyADRNumberProvenance ports a corpus off the repository-global
// state-sequence namespace (ADR-0191): every `state-sequence: <n>` segment is
// stripped from status-history event lines, and every topic-part Revised-by
// list is canonicalized to duplicate-free ascending ADR number, which
// deliberately reorders any historical application-order inversion. Edits are
// raw-byte string surgery scoped to the Status history section and the
// Revised-by lines, so digest-covered content survives byte-identical. A
// state-sequence line the two rewrites cannot consume is a hard stop naming
// the file, and a second run finds nothing to rewrite.
func applyADRNumberProvenance(root string, out *Changes) error {
	if _, err := os.Stat(config.ConfigPath(root)); os.IsNotExist(err) {
		return nil // no config: nothing to migrate (idempotent re-run safe)
	}
	cfg, err := loadForMigration(root)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, cfg.DocsDir, "decisions")
	if _, statErr := os.Stat(dir); statErr == nil {
		corpus, err := adr.LoadCorpus(dir)
		if err != nil {
			return err
		}
		for _, a := range corpus.All() {
			b, err := corpus.Raw(a.Number)
			if err != nil { // coverage-ignore: LoadCorpus above already read this exact path
				return err
			}
			raw := string(b)
			at := strings.Index(raw, statusHistoryHeading)
			if at < 0 {
				continue // ungoverned or legacy record without the section
			}
			head, history := raw[:at], raw[at:]
			rewritten := appliedSequenceRe.ReplaceAllString(history, "$1$2")
			rewritten = statusSequenceRe.ReplaceAllString(rewritten, "$1$2")
			if loc := residualSequenceRe.FindString(cutRationales(rewritten)); loc != "" {
				return fmt.Errorf("adr-number-provenance: %s: cannot rewrite status-history line %q; fix it by hand and re-run", a.Filename, loc)
			}
			if rewritten == history {
				continue
			}
			if err := os.WriteFile(a.Path, []byte(head+rewritten), 0o644); err != nil { // coverage-ignore: the path was just read successfully
				return err
			}
			fmt.Fprintf(out, "adr-number-provenance: %s: stripped state-sequence segment(s)\n", a.Filename)
		}
	}

	parts, err := filepath.Glob(filepath.Join(root, ".awf", "topics", "parts", "*", "*", "current-state.md"))
	if err != nil { // coverage-ignore: the pattern is constant and well-formed
		return err
	}
	sort.Strings(parts)
	for _, path := range parts {
		b, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: Glob just listed this exact path
			return err
		}
		raw := string(b)
		rewritten := revisedByLineRe.ReplaceAllStringFunc(raw, canonicalRevisedByLine)
		if rewritten == raw {
			continue
		}
		if err := os.WriteFile(path, []byte(rewritten), 0o644); err != nil { // coverage-ignore: the path was just read successfully
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(out, "adr-number-provenance: %s: Revised-by canonicalized to ascending ADR number\n", filepath.ToSlash(rel))
	}
	return nil
}

// cutRationales removes every line's terminal `; rationale: ...` free prose so
// the residual scan only sees structural metadata.
func cutRationales(history string) string {
	lines := strings.Split(history, "\n")
	for i, line := range lines {
		if at := strings.Index(line, "; rationale: "); at >= 0 {
			lines[i] = line[:at]
		}
	}
	return strings.Join(lines, "\n")
}

// canonicalRevisedByLine rewrites one matched Revised-by line to duplicate-free
// ascending ADR-number order.
func canonicalRevisedByLine(line string) string {
	nums := []int{}
	seen := map[int]bool{}
	for _, entry := range strings.Split(strings.TrimPrefix(line, "Revised-by: "), ", ") {
		n, _ := strconv.Atoi(strings.TrimPrefix(entry, "ADR-")) // the line regex admits only 4-digit entries
		if !seen[n] {
			seen[n] = true
			nums = append(nums, n)
		}
	}
	sort.Ints(nums)
	entries := make([]string, 0, len(nums))
	for _, n := range nums {
		entries = append(entries, fmt.Sprintf("ADR-%04d", n))
	}
	return "Revised-by: " + strings.Join(entries, ", ")
}
