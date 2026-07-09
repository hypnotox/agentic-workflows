package project

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/templates"
)

// residueADRRe matches a concrete awf ADR citation — `ADR-` followed by four
// digits. The `ADR-NNNN` authoring placeholder never matches.
var residueADRRe = regexp.MustCompile(`ADR-[0-9]{4}`)

// identityExempt lists the template files whose repo-identity literal is a
// reference to awf-the-product, not residue: the bootstrap's download source
// and the agent guide's awf-home link. Entries fail when stale; extending the
// list is a successor-ADR act (ADR-0082 Decision 2).
// invariant: residue-exemptions-pinned
var identityExempt = map[string]bool{
	"bootstrap/awf-bootstrap.sh.tmpl": true,
	"agents-doc/AGENTS.md.tmpl":       true,
}

// identityLiterals are the banned repo-identity tokens.
var identityLiterals = []string{"hypnotox", "agentic-workflows"}

// TestTemplateSourceResidue scans every embedded template source — all
// branches of every conditional, which no render-based sweep can cover — and
// fails on a concrete awf ADR citation or on a repo-identity literal outside
// the explicit exemption list (ADR-0082).
// invariant: template-source-residue
func TestTemplateSourceResidue(t *testing.T) {
	if len(identityExempt) != 2 ||
		!identityExempt["bootstrap/awf-bootstrap.sh.tmpl"] ||
		!identityExempt["agents-doc/AGENTS.md.tmpl"] {
		t.Error("identity-exemption list must name exactly the bootstrap and agents-doc templates — extending it requires a successor ADR (ADR-0082)")
	}
	used := map[string]bool{}
	err := fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		src := string(b)
		if m := residueADRRe.FindString(src); m != "" {
			t.Errorf("%s cites %s — decision rationale lives in the decisions directory, never in shipped templates (ADR-0082)", path, m)
		}
		for _, lit := range identityLiterals {
			if !strings.Contains(src, lit) {
				continue
			}
			if identityExempt[path] {
				used[path] = true
			} else {
				t.Errorf("%s carries repo-identity literal %q outside the exemption list (ADR-0082)", path, lit)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range identityExempt {
		if !used[path] {
			t.Errorf("stale identity exemption %q — the template no longer carries a repo-identity literal; remove the entry via a successor ADR", path)
		}
	}
}
