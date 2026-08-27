package topic

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

const (
	originPrefix    = "Origin: "
	revisedByPrefix = "Revised-by: "
	partFileName    = "current-state.md"
)

// ProvenanceResult identifies every authored topic part whose replacement
// committed, in walk order.
type ProvenanceResult struct {
	Paths []string
}

// PartialProvenanceError retains the exact committed part paths when a later
// provenance observation or replacement fails.
type PartialProvenanceError struct {
	Result ProvenanceResult
	Cause  error
}

func (e *PartialProvenanceError) Error() string { return e.Cause.Error() }
func (e *PartialProvenanceError) Unwrap() error { return e.Cause }

// SubstituteProvenanceConfined rewrites authored claim parts through the
// caller-held selected-root handle. It touches only Origin and Revised-by
// values under .awf/topics/parts and canonicalizes touched lists. Numbering
// owns the transaction lease; topic retains metadata grammar and replacement
// policy.
func SubstituteProvenanceConfined(files *filesystem.Handle, renames map[string]string) (ProvenanceResult, error) {
	result := ProvenanceResult{}
	if len(renames) == 0 {
		return result, nil
	}
	partsRoot := filepath.ToSlash(filepath.Join(config.DirName, "topics", "parts"))
	if _, err := files.LinkInfo(partsRoot); errors.Is(err, os.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return result, err
	}
	err := files.Walk(partsRoot, func(path string, info fs.FileInfo) (bool, error) {
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("topic provenance path %q is a symlink", path)
		}
		if info.IsDir() || filepath.Base(path) != partFileName {
			return true, nil
		}
		expected, err := files.ExpectedIdentity(path)
		if err != nil {
			return false, err
		}
		if expected.Mode()&os.ModeSymlink != 0 || !expected.Mode().IsRegular() {
			_ = expected.Release()
			return false, fmt.Errorf("topic provenance path %q is not a regular file", path)
		}
		data, err := files.Read(path)
		if err != nil {
			_ = expected.Release()
			return false, err
		}
		body, changed := substituteProvenanceLines(string(data), renames)
		if !changed {
			_ = expected.Release()
			return true, nil
		}
		if err := files.ReplaceExpected(path, expected, []byte(body), expected.Mode().Perm()); err != nil {
			return false, err
		}
		result.Paths = append(result.Paths, path)
		return true, nil
	})
	if err != nil && len(result.Paths) != 0 {
		return result, &PartialProvenanceError{Result: result, Cause: err}
	}
	return result, err
}

// substituteProvenanceLines applies the substitution to one part's bytes,
// reporting whether anything moved. A line whose value does not parse as
// provenance is left alone: the corpus loader owns that diagnosis, and
// numbering must not turn a malformed part into a differently malformed one.
func substituteProvenanceLines(body string, renames map[string]string) (string, bool) {
	lines := strings.SplitAfter(body, "\n")
	changed := false
	for i, raw := range lines {
		trimmed := strings.TrimSuffix(raw, "\n")
		eol := raw[len(trimmed):]
		switch {
		case strings.HasPrefix(trimmed, originPrefix):
			ref, err := parseADRRef(strings.TrimSpace(strings.TrimPrefix(trimmed, originPrefix)))
			if err != nil {
				continue
			}
			number, renamed := renames[ref]
			if !renamed {
				continue
			}
			lines[i], changed = originPrefix+"ADR-"+number+eol, true
		case strings.HasPrefix(trimmed, revisedByPrefix):
			refs, err := parseADRList(strings.TrimSpace(strings.TrimPrefix(trimmed, revisedByPrefix)))
			if err != nil {
				continue
			}
			touched := false
			for j, ref := range refs {
				if number, renamed := renames[ref]; renamed {
					refs[j], touched = number, true
				}
			}
			if !touched {
				continue
			}
			lines[i], changed = revisedByPrefix+strings.Join(canonicalADRRefs(refs), ", ")+eol, true
		}
	}
	return strings.Join(lines, ""), changed
}

// canonicalADRRefs renders a provenance list in the duplicate-free ascending
// order ADR-0191 requires: a substituted entry that lands numerically below an
// entry already in the list is re-sorted into place, not appended after it.
func canonicalADRRefs(refs []string) []string {
	unique := make([]string, 0, len(refs))
	for _, ref := range refs {
		if !slices.Contains(unique, ref) {
			unique = append(unique, ref)
		}
	}
	slices.SortStableFunc(unique, func(a, b string) int {
		return adr.IdentityOrder(a) - adr.IdentityOrder(b)
	})
	for i, ref := range unique {
		unique[i] = "ADR-" + ref
	}
	return unique
}
