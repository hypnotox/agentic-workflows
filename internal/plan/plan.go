// Package plan parses plan files under docs/plans and scaffolds new plans from
// the rendered plans template (awf new plan). Unlike internal/adr it is not
// coupled to sequential numbering - plans are date-prefixed.
package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

// ValidStatuses are the two plan lifecycle states (ADR-0097): mutable while
// Proposed, frozen at Implemented.
var ValidStatuses = map[string]bool{"Proposed": true, "Implemented": true}

// FilenameRe matches a plan filename (YYYY-MM-DD-slug.md); it excludes
// template.md and README.md just as adr.FilenameRe's numeric form does.
var FilenameRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-.+\.md$`)

// ADRLink is one `adrs:` frontmatter entry. A plan links a decision record by
// its number or, before integration numbers it, by the retained slug of a
// pending record, so the link stays valid across numbering without the plan
// being rewritten (ADR-0202 item 14). Exactly one field is set.
type ADRLink struct {
	Number int
	Slug   string
}

// adrLinkNumberRe matches the digits-only spelling of a numeric entry. The raw
// scalar is matched rather than the resolved YAML tag because yaml.v3 resolves
// a zero-padded spelling to !!int or !!float depending on whether its digits
// are octal-valid - `0186` is !!float and `0153` is !!int - so a tag switch
// would read the eight plans spelling their link that way as slugs. Matching
// the raw digits also drops the octal reading `0153` used to get.
var adrLinkNumberRe = regexp.MustCompile(`^\d+$`)

// adrLinkMaxNumber is the largest number an entry may name: the corpus identity
// key is the four-digit number, so a wider value has no record to resolve to
// and would be handed to the slug index instead.
const adrLinkMaxNumber = 9999

// UnmarshalYAML reads one `adrs:` entry: a digits-only scalar is a number, and
// any other non-empty string scalar is a slug. A slug is not validated against
// the slug grammar here - an entry that names no record in the corpus fails
// link validation with a scoped finding (ADR-0202 item 14) rather than taking
// the whole check down. The number case is matched first, so an entirely
// numeric slug would be read as a number; the ADR scaffold refuses an all-digit
// slug for exactly that reason. Any other node names itself in the error.
func (l *ADRLink) UnmarshalYAML(node *yaml.Node) error {
	switch {
	case node.Kind != yaml.ScalarNode:
		return fmt.Errorf("plan: adrs entry must be an ADR number or slug, got yaml %s", node.Tag)
	case adrLinkNumberRe.MatchString(node.Value):
		n, err := strconv.Atoi(node.Value)
		if err != nil || n < 1 || n > adrLinkMaxNumber {
			return fmt.Errorf("plan: adrs entry %q is not a usable ADR number (1 to %d)", node.Value, adrLinkMaxNumber)
		}
		l.Number = n
	case node.Tag == "!!str" && node.Value != "":
		l.Slug = node.Value
	default:
		return fmt.Errorf("plan: adrs entry %q is neither an ADR number nor a slug (yaml %s)", node.Value, node.Tag)
	}
	return nil
}

// Identity returns the entry's corpus identity key: the four-digit number for a
// numeric entry, and the retained slug for a slug entry. It is the key
// adr.Corpus.ByIdentity resolves, so one lookup covers both spellings.
func (l ADRLink) Identity() string {
	if l.Slug != "" {
		return l.Slug
	}
	return fmt.Sprintf("%04d", l.Number)
}

// Plan is a parsed plan record. HasFrontmatter is false for the grandfathered
// pre-convention corpus (ADR-0098), which the checks skip.
type Plan struct {
	Filename       string
	Path           string
	Date           string
	ADRs           []ADRLink
	Status         string
	Format         string
	HasFrontmatter bool
	// Source retains the authored bytes. plan-v1 projections render only from
	// the parsed model and these retained sections; legacy callers need not use it.
	Source              []byte
	Preamble            string
	Title               string
	Goal                string
	ArchitectureSummary string
	Phases              []Phase
	DefinitionOfDone    string
	Notes               string
	// CommitSubjects are the planned commit subjects a plan marks with ```commit
	// fences (ADR-0111): the first non-empty line of each fenced block whose info
	// string's first token is `commit` and which carries no `awf-ignore` opt-out.
	CommitSubjects []string
}

type planFrontmatter struct {
	Format string    `yaml:"format"`
	Date   string    `yaml:"date"`
	ADRs   []ADRLink `yaml:"adrs"`
	Status string    `yaml:"status"`
}

// ParseDir scans dir for plan files (YYYY-MM-DD-*.md) and parses each. Files
// without frontmatter parse to a Plan with HasFrontmatter false.
func ParseDir(dir string) ([]Plan, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	var plans []Plan
	var diagnostics []*Diagnostic
	for _, path := range matches {
		base := filepath.Base(path)
		if !FilenameRe.MatchString(base) {
			continue // skip template.md, README.md, and any non-plan file
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		format, present, formatErr := frontmatterFormat(data)
		if formatErr != nil {
			var diagnostic *Diagnostic
			if errors.As(formatErr, &diagnostic) {
				diagnostic.Path = base
				diagnostics = append(diagnostics, diagnostic)
				continue
			}
			return nil, formatErr // coverage-ignore: frontmatterFormat returns only typed Diagnostic errors
		}
		var fm planFrontmatter
		body, found, err := frontmatter.Parse(data, &fm)
		if err != nil {
			diagnostics = append(diagnostics, &Diagnostic{Category: "frontmatter", Path: base, Detail: err.Error()})
			continue
		}
		pl := Plan{
			Filename: base, Path: path, Date: fm.Date, ADRs: fm.ADRs,
			Status: fm.Status, Format: fm.Format, HasFrontmatter: found,
			Source: data, CommitSubjects: commitSubjects(string(data)),
		}
		if found && present {
			if format != "plan-v1" {
				diagnostics = append(diagnostics, &Diagnostic{Category: "frontmatter", Path: base, Detail: "format must be exactly plan-v1"})
				continue
			}
			if err := parsePlanV1(base, string(data), string(body), &pl); err != nil {
				var diagnostic *Diagnostic
				if errors.As(err, &diagnostic) {
					diagnostics = append(diagnostics, diagnostic)
					continue
				}
				return nil, err // coverage-ignore: parsePlanV1 returns only typed Diagnostic errors
			}
		}
		plans = append(plans, pl)
	}
	if len(diagnostics) > 0 {
		return plans, &DiagnosticsError{Diagnostics: diagnostics}
	}
	return plans, nil
}

// frontmatterFormat inspects the YAML node before decoding into planFrontmatter
// so marker absence is the only legacy route and duplicate or non-scalar format
// declarations retain typed frontmatter error identity.
func frontmatterFormat(data []byte) (string, bool, error) {
	yamlBlock, _, found := frontmatter.Split(data)
	if !found {
		return "", false, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBlock, &doc); err != nil {
		return "", false, structuralError("", "frontmatter", err.Error())
	}
	if len(doc.Content) == 0 {
		return "", false, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return "", false, structuralError("", "frontmatter", "frontmatter must be a mapping")
	}
	var value string
	present := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, node := root.Content[i], root.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Value != "format" {
			continue
		}
		if present {
			return "", true, structuralError("", "frontmatter", "duplicate format")
		}
		present = true
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" || strings.TrimSpace(node.Value) == "" {
			return "", true, structuralError("", "frontmatter", "format must be a nonempty string")
		}
		value = node.Value
	}
	return value, present, nil
}

// commitSubjects returns the planned commit subjects a plan marks with ```commit
// fences (ADR-0111): for every ``` fenced block whose info string's first
// whitespace-delimited token is `commit` and which carries no `awf-ignore` opt-out
// token, the block's first non-empty line. An empty/whitespace-only block yields
// nothing. Every line beginning with ``` toggles the fenced state. Only ``` is a
// fence delimiter here - ADR-0111 deliberately drops `~~~` to avoid an uncovered
// branch - so, as accepted best-effort edges over well-formed plan markdown, a
// ```commit nested inside a `~~~` block is still read, and an unclosed ```commit
// fence at end-of-file (subject appended only on its closing ```) yields no subject.
func commitSubjects(content string) []string {
	var subjects []string
	inFence := false
	checked := false // the open fence is a checkable ```commit block
	var first string // first non-empty line inside the open block
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "```") {
			if inFence && checked && first == "" && trimmed != "" {
				first = trimmed
			}
			continue
		}
		if inFence {
			if checked && first != "" {
				subjects = append(subjects, first)
			}
			inFence, checked, first = false, false, ""
			continue
		}
		inFence = true
		checked = isCommitInfo(trimmed[3:])
	}
	return subjects
}

// isCommitInfo reports whether a fence info string marks a checkable planned-commit
// block: its first token is `commit` and no token is the `awf-ignore` opt-out.
func isCommitInfo(info string) bool {
	fields := strings.Fields(info)
	if len(fields) == 0 || fields[0] != "commit" {
		return false
	}
	for _, f := range fields[1:] {
		if f == "awf-ignore" {
			return false
		}
	}
	return true
}

// now returns the current time; overridden in tests (mirrors adr.now).
var now = time.Now

var markerLineRe = regexp.MustCompile(`(?m)^<!-- (GENERATED by awf|awf:edit).*-->\n`)
var slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) (string, error) {
	s := slugNonAlnumRe.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "", fmt.Errorf("plan: title %q has no usable characters for a filename", title)
	}
	return s, nil
}

func replaceOnce(s, old, replacement string) (string, error) {
	if !strings.Contains(s, old) {
		return "", fmt.Errorf("plan: template missing expected %q", old)
	}
	return strings.Replace(s, old, replacement, 1), nil
}

// NewFile scaffolds a new plan under dir from the rendered plans template
// (dir/template.md): today's date filled, marker comments stripped, named
// YYYY-MM-DD-slug.md. No sequential number is allocated. Refuses to overwrite.
// touches-state: adr-system/plan-artifacts:plan-new-unnumbered - unnumbered dated plan scaffold; proof in plan_test.go
func NewFile(dir, title string) (string, error) {
	title = strings.TrimSpace(title)
	slug, err := slugify(title)
	if err != nil {
		return "", err
	}
	date := now().Format("2006-01-02")
	path := filepath.Join(dir, date+"-"+slug+".md")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("plan: %s already exists", path)
	} else if !os.IsNotExist(err) { // coverage-ignore: Stat fails here only on a permission fault a test cannot trigger
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "template.md"))
	if err != nil {
		return "", fmt.Errorf("plan: read template: %w", err)
	}
	content := markerLineRe.ReplaceAllString(string(raw), "")
	content, err = replaceOnce(content, "date: YYYY-MM-DD", "date: "+date)
	if err != nil {
		return "", err
	}
	content, err = replaceOnce(content, "# Plan: Title", "# Plan: "+title)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { // coverage-ignore: post-Stat write; fails only on a permission fault a test cannot trigger
		return "", err
	}
	return path, nil
}
