// Package invariants checks that each Implemented ADR's `inv: <slug>` invariant
// tag is backed by a `<marker> invariant: <slug>` comment in a configured source
// file. The comment marker and the files scanned are language-configurable via
// the project's invariants config; nothing here assumes Go.
package invariants

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// Status classifies an invariant finding.
type Status string

const (
	Unbacked  Status = "unbacked"  // configured, but no backing tag found
	Unchecked Status = "unchecked" // no invariant sources configured (and not disabled)
)

// Finding is an Implemented-ADR invariant slug that is not satisfied.
type Finding struct {
	Slug   string
	ADR    string // filename of the declaring ADR
	Status Status
}

// Detail is a human, language-neutral remedy line for the finding.
func (f Finding) Detail() string {
	if f.Status == Unchecked {
		return "unchecked — configure invariants.sources or set invariants.disabled: true"
	}
	return "unbacked — add a `<marker> invariant: " + f.Slug + "` comment in a configured source file"
}

// Line renders the finding as a single human-readable line (no leading
// indent/column), shared by `awf check` and `awf invariants`.
func (f Finding) Line() string {
	return fmt.Sprintf("%s — invariant %q %s", f.ADR, f.Slug, f.Detail())
}

var (
	// declRe matches an invariant DECLARATION: an `inv: <slug>` tag that leads a
	// markdown list item (optionally indented). Only backticks and spaces may sit
	// between the bullet and the tag, so both the single-backtick form
	// (`- `+"`inv: x`") and the double-backtick form ADR-0007 uses to render
	// literal backticks (`- `+"``  `inv: x`  ``") are recognised, while a
	// mid-prose cross-reference to another ADR's slug is not (it does not lead a
	// list item) — which would otherwise phantom-duplicate that slug.
	declRe = regexp.MustCompile("(?m)^[ \\t]*[-*][ \\t]+[`\\t ]*inv:\\s*([a-z0-9-]+)")
	slugRe = regexp.MustCompile(`^\s*invariant:\s*([a-z0-9-]+)`)
)

// Check returns a Finding per unsatisfied Implemented-ADR invariant slug.
// No required slugs → nil. cfg disabled → nil. cfg nil or source-less → every
// required slug is Unchecked. Otherwise unbacked slugs are Unbacked.
func Check(decisionsDir, root string, cfg *config.InvariantConfig) ([]Finding, error) {
	adrs, err := adr.ParseDir(decisionsDir)
	if err != nil {
		return nil, err
	}
	required := map[string]string{} // slug -> declaring ADR filename
	for _, a := range adrs {
		if a.Status != "Implemented" {
			continue
		}
		for _, m := range declRe.FindAllStringSubmatch(a.Sections["Invariants"], -1) {
			slug := m[1]
			if prev, ok := required[slug]; ok {
				return nil, fmt.Errorf("duplicate inv slug %q (in %s and %s)", slug, prev, a.Filename)
			}
			required[slug] = a.Filename
		}
	}
	// Retirements: an Implemented ADR may retire an inv slug it removed the
	// backing for (ADR-0031). A retired slug is dropped from required; a slug
	// retired but declared by no Implemented ADR is a dangling retirement.
	for _, a := range adrs {
		if a.Status != "Implemented" {
			continue
		}
		for _, slug := range a.RetiresInvariants {
			if _, ok := required[slug]; !ok {
				return nil, fmt.Errorf("dangling retirement: ADR %s retires %q, which no Implemented ADR declares as an inv slug", a.Filename, slug)
			}
			delete(required, slug)
		}
	}
	if len(required) == 0 {
		return nil, nil
	}
	if cfg != nil && cfg.Disabled {
		return nil, nil
	}

	mk := func(status Status) []Finding {
		out := make([]Finding, 0, len(required))
		for slug, file := range required {
			out = append(out, Finding{Slug: slug, ADR: file, Status: status})
		}
		sortBySlug(out)
		return out
	}

	if cfg == nil || len(cfg.Sources) == 0 {
		return mk(Unchecked), nil
	}

	present, err := scanTags(root, cfg.Sources)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for slug, file := range required {
		if !present[slug] {
			findings = append(findings, Finding{Slug: slug, ADR: file, Status: Unbacked})
		}
	}
	sortBySlug(findings)
	return findings, nil
}

// scanTags collects slugs backed by a `<marker> invariant: <slug>` comment in a
// file whose basename matches one of a source's globs (skipping
// .git/vendor/node_modules). The marker is matched literally; whitespace between
// the marker and `invariant:` is tolerated.
// sortBySlug orders findings by slug, the stable output order of both scans.
func sortBySlug(f []Finding) {
	sort.Slice(f, func(i, j int) bool { return f[i].Slug < f[j].Slug })
}

func scanTags(root string, sources []config.InvariantSource) (map[string]bool, error) {
	present := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		base := filepath.Base(path)
		var markers []string
		for _, src := range sources {
			for _, g := range src.Globs {
				if ok, _ := filepath.Match(g, base); ok {
					markers = append(markers, src.Marker)
					break
				}
			}
		}
		if len(markers) == 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			for _, marker := range markers {
				if idx := strings.Index(line, marker); idx >= 0 {
					if m := slugRe.FindStringSubmatch(line[idx+len(marker):]); m != nil {
						present[m[1]] = true
					}
				}
			}
		}
		return nil
	})
	return present, err
}
