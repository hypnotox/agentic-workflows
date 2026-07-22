package project

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/templates"
)

// residueADRRe matches a concrete awf ADR citation - `ADR-` followed by four
// digits. The `ADR-NNNN` authoring placeholder never matches.
var residueADRRe = regexp.MustCompile(`ADR-[0-9]{4}`)

// identityExempt lists the template files whose repo-identity literal is a
// reference to awf-the-product, not residue: the bootstrap unit's download
// sources (installer and upgrade porcelain) and the agent guide's awf-home
// link. Entries fail when stale; extending the list is a successor-ADR act
// (ADR-0082 Decision 2, extended by ADR-0085 Decision 5, pinned at three by ADR-0131).
var identityExempt = map[string]bool{
	"bootstrap/awf-bootstrap.sh.tmpl": true,
	"bootstrap/awf-upgrade.sh.tmpl":   true,
	"agents-doc/AGENTS.md.tmpl":       true,
}

// identityLiterals are the banned repo-identity tokens.
var identityLiterals = []string{"hypnotox", "agentic-workflows"}

// TestTemplateSourceResidue scans every embedded template source - all
// branches of every conditional, which no render-based sweep can cover - and
// fails on a concrete awf ADR citation or on a repo-identity literal outside
// the explicit exemption list (ADR-0082).
// invariant: rendering/templates:template-source-residue
func TestTemplateSourceResidue(t *testing.T) {
	// The marker sits on the assertion rather than on the var it guards, so the
	// proof site contains the check that proves it (ADR-0131 Task 3.3).
	// invariant: rendering/sync-and-drift:residue-exemptions-pinned-three
	if len(identityExempt) != 3 ||
		!identityExempt["bootstrap/awf-bootstrap.sh.tmpl"] ||
		!identityExempt["bootstrap/awf-upgrade.sh.tmpl"] ||
		!identityExempt["agents-doc/AGENTS.md.tmpl"] {
		t.Error("identity-exemption list must name exactly the bootstrap, upgrade, and agents-doc templates - extending it requires a successor ADR (ADR-0082, pinned at three by ADR-0131)")
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
			t.Errorf("%s cites %s - decision rationale lives in the decisions directory, never in shipped templates (ADR-0082)", path, m)
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
			t.Errorf("stale identity exemption %q - the template no longer carries a repo-identity literal; remove the entry via a successor ADR", path)
		}
	}
}

// collectStrings walks an any-typed catalog Data value and appends every
// string it holds (map values and list elements, recursively) to out. Any
// other composite type is reported instead of silently dropped - a Data entry
// written as []string or map[string]string would otherwise escape the scan
// with no signal, recreating the unscanned-surface class this guard closes.
func collectStrings(t *testing.T, site string, v any, out *[]string) {
	t.Helper()
	switch x := v.(type) {
	case nil, bool, int, float64:
		// Non-prose scalars: nothing to scan.
	case string:
		*out = append(*out, x)
	case map[string]any:
		for _, mv := range x {
			collectStrings(t, site, mv, out)
		}
	case []any:
		for _, e := range x {
			collectStrings(t, site, e, out)
		}
	default:
		t.Errorf("%s carries a Data value of unexpected type %T - use map[string]any/[]any/scalars so the residue scan sees every string", site, v)
	}
}

// TestCatalogDataResidue extends the ADR-0082 residue rule to the catalog's
// shipped prose: every Data string of every skill, agent, doc, and the domain
// doc, each doc's Title/Desc, and every var descriptor's Description, Default,
// and Options render into adopter artifacts or adopter-facing prompts exactly
// like template source does, so a concrete awf ADR citation or a repo-identity
// literal there is the same leak the templates scan catches (a citation
// resolves to nothing - or to the adopter's own unrelated ADR - in every
// corpus but awf's). Descriptors are scanned here directly rather than
// deferred to configspec-description-residue, which reads VarEntries and so
// skips the routing-target descriptors and never sees Default/Options. No
// exemptions: unlike the bootstrap templates, no catalog string references
// awf-the-product. An ordinary gate test, deliberately slug-free: ADR-0082
// owns the principle and this is its enforcement catching up with the
// catalog's move to Go (ADR-0060 postdates its scan scope).
func TestCatalogDataResidue(t *testing.T) {
	cat := catalog.Standard
	check := func(site string, strs []string) {
		t.Helper()
		for _, s := range strs {
			if m := residueADRRe.FindString(s); m != "" {
				t.Errorf("%s cites %s - decision rationale lives in the decisions directory, never in shipped catalog prose (ADR-0082)", site, m)
			}
			for _, lit := range identityLiterals {
				if strings.Contains(s, lit) {
					t.Errorf("%s carries repo-identity literal %q (ADR-0082)", site, lit)
				}
			}
		}
	}
	for name, spec := range cat.Skills {
		var strs []string
		collectStrings(t, "skill "+name, spec.Data, &strs)
		check("skill "+name, strs)
	}
	for name, spec := range cat.Agents {
		var strs []string
		collectStrings(t, "agent "+name, spec.Data, &strs)
		check("agent "+name, strs)
	}
	var strs []string
	collectStrings(t, "domainDoc", cat.DomainDoc.Data, &strs)
	check("domainDoc", strs)
	for name, d := range cat.Docs {
		var strs []string
		collectStrings(t, "doc "+name, d.Data, &strs)
		strs = append(strs, d.Title, d.Desc)
		check("doc "+name, strs)
	}
	for _, v := range cat.Vars {
		check("var "+v.Key, append([]string{v.Description, v.Default}, v.Options...))
	}
}

// bannedRunes are the seven typographic punctuation substitutes banned from
// emitted prose (ADR-0115). Each key is written as an escape so this file states
// the rule without typing the glyphs it bans. Notation (arrows, mathematical
// symbols, accented letters) is deliberately absent: this is a closed blocklist
// of substitutes for ASCII punctuation, never an ASCII-only allowlist.
var bannedRunes = map[rune]string{
	'\u2014': "em-dash (U+2014)",
	'\u2013': "en-dash (U+2013)",
	'\u2026': "ellipsis (U+2026)",
	'\u2018': "left single quote (U+2018)",
	'\u2019': "right single quote (U+2019)",
	'\u201c': "left double quote (U+201C)",
	'\u201d': "right double quote (U+201D)",
}

// scanEmbedded reports every banned rune in every file of an embedded FS and
// returns the number of files inspected, at most one report per rune per file.
func scanEmbedded(t *testing.T, label string, fsys fs.FS) int {
	t.Helper()
	seen := 0
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		seen++
		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		flagged := map[rune]bool{}
		for _, r := range string(b) {
			if name, bad := bannedRunes[r]; bad && !flagged[r] {
				flagged[r] = true
				t.Errorf("%s: %s contains the %s; emitted prose uses plain punctuation (ADR-0115)", label, path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return seen
}

// scanGoLiterals reports every banned rune in a string literal of every non-test
// Go file under dir and returns the number of files inspected. Comments are
// deliberately not inspected: gofmt rewrites a double-backtick pair into U+201C,
// so scanning them would pit this gate against gofmt (ADR-0115 Decision item 4).
func scanGoLiterals(t *testing.T, dir string) int {
	t.Helper()
	seen := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		seen++
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				val = lit.Value
			}
			flagged := map[rune]bool{}
			for _, r := range val {
				if name, bad := bannedRunes[r]; bad && !flagged[r] {
					flagged[r] = true
					t.Errorf("%s:%d: string literal contains the %s; emitted prose uses plain punctuation (ADR-0115)",
						path, fset.Position(lit.Pos()).Line, name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return seen
}

// TestEmittedProseNoTypographicSubstitutes scans the three surfaces awf ships:
// the embedded template FS, the embedded changelog FS, and every string literal
// in production Go under internal/ and cmd/. Each surface carries a seen-count
// guard, so a mis-anchored walk fails rather than passing vacuously. Adopter
// content, and this repository's authored ADR and plan bodies, are out of scope:
// the ban covers what awf ships, in awf's own voice (ADR-0115).
// invariant: tooling/quality-gates:emitted-prose-no-typographic-substitutes
func TestEmittedProseNoTypographicSubstitutes(t *testing.T) {
	if n := scanEmbedded(t, "templates", templates.FS); n < 40 {
		t.Fatalf("inspected only %d embedded template file(s); expected the whole tree - did the FS move?", n)
	}
	if n := scanEmbedded(t, "changelog", changelogfs.FS); n < 1 {
		t.Fatalf("inspected only %d embedded changelog file(s); expected CHANGELOG.md - did the embed move?", n)
	}
	goFiles := 0
	for _, dir := range []string{"../../internal", "../../cmd"} {
		goFiles += scanGoLiterals(t, dir)
	}
	if goFiles < 60 {
		t.Fatalf("inspected only %d production Go file(s) under internal/ and cmd/; expected the whole tree - did the anchor move?", goFiles)
	}
}
