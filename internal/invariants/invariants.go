// Package invariants checks that each Implemented ADR's `inv: <slug>` invariant
// tags are backed by a `// invariant: <slug>` comment in the project's Go source.
package invariants

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agentic-workflows/internal/adr"
)

// Finding is an Implemented-ADR invariant slug with no backing test.
type Finding struct {
	Slug string
	ADR  string // filename of the declaring ADR
}

var (
	tagRe  = regexp.MustCompile("`inv:\\s*([a-z0-9-]+)`")
	testRe = regexp.MustCompile(`//\s*invariant:\s*([a-z0-9-]+)`)
)

// Check returns a Finding for each Implemented-ADR invariant slug under
// decisionsDir lacking a `// invariant: <slug>` comment in any *.go file under
// root. A slug declared by two ADRs is an error.
func Check(decisionsDir, root string) ([]Finding, error) {
	adrs, err := adr.ParseDir(decisionsDir)
	if err != nil {
		return nil, err
	}
	required := map[string]string{} // slug -> declaring ADR filename
	for _, a := range adrs {
		if a.Status != "Implemented" {
			continue
		}
		for _, m := range tagRe.FindAllStringSubmatch(a.Sections["Invariants"], -1) {
			slug := m[1]
			if prev, ok := required[slug]; ok {
				return nil, fmt.Errorf("duplicate inv slug %q (in %s and %s)", slug, prev, a.Filename)
			}
			required[slug] = a.Filename
		}
	}
	present, err := scanTags(root)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for slug, file := range required {
		if !present[slug] {
			findings = append(findings, Finding{Slug: slug, ADR: file})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Slug < findings[j].Slug })
	return findings, nil
}

// scanTags collects every slug named by a `// invariant: <slug>` comment in a
// *.go file under root (skipping .git/vendor/node_modules).
func scanTags(root string) (map[string]bool, error) {
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
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range testRe.FindAllStringSubmatch(string(data), -1) {
			present[m[1]] = true
		}
		return nil
	})
	return present, err
}
