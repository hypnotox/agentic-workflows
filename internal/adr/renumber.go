package adr

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"strings"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

// PartialRenumberError retains the independently observable destination and
// source states when the destination is published but source retirement fails.
// Retrying source retirement is safe because the destination is complete.
type PartialRenumberError struct {
	Destination          string
	Source               string
	DestinationPublished bool
	SourceRetired        bool
	Cause                error
	Recovery             []string
}

func (e *PartialRenumberError) Error() string { return e.Cause.Error() }
func (e *PartialRenumberError) Unwrap() error { return e.Cause }

// RenumberPendingConfined renames the pending record `<slug>.md` under dir to
// `NNNN-<slug>.md` and rewrites its `# ADR-<slug>:` heading to `# ADR-NNNN:`.
// Nothing else moves: the retained `slug:` frontmatter key, every body byte, and
// every Status history event survive verbatim, which is what makes numbering
// digest-safe and history-free (ADR-0202 item 9). The caller supplies the
// selected-root handle and retains the project lease for the whole transaction.
//
// It is the rewrite seam the numbering command performs its rename through, and
// the only reader and writer of a decision record's path outside this package's
// corpus construction seams. Keeping it here is what lets the command live in
// internal/project without joining the enumerated raw-bytes accessors
// (adr-system/adr-lifecycle:corpus-raw-access-enumerated). dir is a root-relative
// decisions directory.
func RenumberPendingConfined(files *filesystem.Handle, dir, slug string, number int) error {
	return renumberPending(files, dir, slug, number, files.RemoveExpected)
}

func renumberPending(files *filesystem.Handle, dir, slug string, number int, retire func(string, fs.FileInfo) error) error {
	from := filepath.ToSlash(filepath.Join(dir, slug+".md"))
	data, err := files.Read(from)
	if err != nil {
		return fmt.Errorf("adr: read pending record %s: %w", slug, err)
	}
	info, err := files.LinkInfo(from)
	if err != nil {
		return fmt.Errorf("adr: stat pending record %s: %w", slug, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("adr: pending record %s is a symlink", slug)
	}
	numbered := fmt.Sprintf("%04d", number)
	oldHeading, newHeading := "# ADR-"+slug+":", "# ADR-"+numbered+":"
	lines := strings.SplitAfter(string(data), "\n")
	rewritten := false
	for i, line := range lines {
		if strings.HasPrefix(line, oldHeading) {
			lines[i] = newHeading + strings.TrimPrefix(line, oldHeading)
			rewritten = true
			break
		}
	}
	if !rewritten {
		return fmt.Errorf("adr: pending record %s has no %q heading to renumber", slug, oldHeading)
	}
	to := filepath.ToSlash(filepath.Join(dir, numbered+"-"+slug+".md"))
	if err := files.Publish(to, []byte(strings.Join(lines, "")), info.Mode().Perm()); err != nil {
		return fmt.Errorf("adr: publish numbered record %s: %w", slug, err)
	}
	if err := retire(from, info); err != nil {
		return &PartialRenumberError{Destination: to, Source: from, DestinationPublished: true, SourceRetired: false, Cause: fmt.Errorf("retire pending ADR source: %w", err), Recovery: []string{"verify the numbered destination", "remove only the retained pending source", "rerun publication"}}
	}
	return nil
}
