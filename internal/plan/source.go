package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

// Source is one plan document supplied by a filesystem or immutable snapshot.
// Filename identifies the plan, Path is its diagnostic display path, and Bytes
// are the complete authored source.
type Source struct {
	Filename string
	Path     string
	Bytes    []byte
}

// ParseSources parses one already-confined source set in its supplied order.
// It retains valid siblings when independent documents carry diagnostics.
func ParseSources(sources []Source) ([]Plan, error) {
	var plans []Plan
	var diagnostics []*Diagnostic
	for _, source := range sources {
		base := source.Filename
		if !FilenameRe.MatchString(base) {
			continue
		}
		if source.Path == "\x00escape" {
			diagnostics = append(diagnostics, &Diagnostic{Category: "path", Path: base, Detail: "plan path escapes plans directory"})
			continue
		}
		format, present, formatErr := frontmatterFormat(source.Bytes)
		if formatErr != nil {
			var diagnostic *Diagnostic
			if errors.As(formatErr, &diagnostic) {
				diagnostic.Path = base
				diagnostics = append(diagnostics, diagnostic)
				continue
			}
			return nil, formatErr // coverage-ignore: frontmatterFormat constructs every non-nil error as *Diagnostic
		}
		var fm planFrontmatter
		body, found, err := frontmatter.Parse(source.Bytes, &fm)
		if err != nil {
			diagnostics = append(diagnostics, &Diagnostic{Category: "frontmatter", Path: base, Detail: err.Error()})
			continue
		}
		pl := Plan{Filename: base, Path: source.Path, Date: fm.Date, ADRs: fm.ADRs, Status: fm.Status, Format: fm.Format, HasFrontmatter: found, Source: source.Bytes, CommitSubjects: commitSubjects(string(source.Bytes))}
		if found && present {
			switch format {
			case "plan-v1":
				err = parsePlanV1(base, string(source.Bytes), string(body), &pl)
			case "plan-v2":
				err = parsePlanV2(base, string(source.Bytes), string(body), &pl)
			default:
				err = structuralError(base, "frontmatter", "format must be exactly plan-v1 or plan-v2")
			}
			if err != nil {
				var diagnostic *Diagnostic
				if errors.As(err, &diagnostic) {
					diagnostics = append(diagnostics, diagnostic)
					continue
				}
				return nil, err // coverage-ignore: plan-v1 and plan-v2 parsers construct every returned error as *Diagnostic
			}
			pl.TerminalReconciliation, err = ParseTerminalReconciliation(pl.Notes)
			if err != nil {
				diagnostics = append(diagnostics, &Diagnostic{Category: "terminal-reconciliation", Path: base, Detail: err.Error()})
				continue
			}
		}
		plans = append(plans, pl)
	}
	if len(diagnostics) > 0 {
		return plans, &DiagnosticsError{Diagnostics: diagnostics}
	}
	return plans, nil
}

// parseDirSources confines filesystem plan paths before supplying them to the
// shared parser.
func parseDirSources(dir string) ([]Source, error) {
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve plans directory %s: %w", dir, err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	out := make([]Source, 0, len(matches))
	for _, path := range matches {
		base := filepath.Base(path)
		if !FilenameRe.MatchString(base) {
			continue
		}
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", base, err)
		}
		rel, err := filepath.Rel(resolvedDir, resolvedPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			out = append(out, Source{Filename: base, Path: "\x00escape"})
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		out = append(out, Source{Filename: base, Path: path, Bytes: data})
	}
	return out, nil
}
