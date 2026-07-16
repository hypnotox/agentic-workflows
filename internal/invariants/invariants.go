// Package invariants checks that each Implemented ADR's `invariant: <slug>`
// declaration is backed by a proof `<marker> invariant: <slug>` comment in a
// configured source file. With `invariants.testGlobs` set, a proof marker backs
// a slug only in a test file; absent testGlobs it falls back to source-glob
// scope (ADR-0105). An `unbacked-invariant: <slug>` declaration is a reasoned
// contract exempt from the proof requirement but carrying a `Verify:` note; a
// `touches-invariant: <slug>` marker is advisory context, never backing. The
// comment marker and the files scanned are language-configurable via the
// project's invariants config; nothing here assumes Go.
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
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// Status classifies an invariant finding.
type Status string

const (
	Unbacked  Status = "unbacked"  // declared backed, but no proof marker in backing scope
	Unchecked Status = "unchecked" // no invariant sources configured (and not disabled)
	// UnbackedHasProof: an `unbacked-invariant:` declaration for which a proof
	// marker exists in backing scope (ADR-0105 unbacked-refuses-proof).
	UnbackedHasProof Status = "unbacked-has-proof"
	// MissingVerify: an `unbacked-invariant:` declaration lacking a `Verify:` note
	// (ADR-0105 unbacked-requires-verify-note).
	MissingVerify Status = "missing-verify"
)

// Class is an invariant's declared backing class (ADR-0105): backed invariants
// require a proof marker; unbacked ones are reasoned contracts carrying a
// `Verify:` note.
type Class string

const (
	ClassBacked   Class = "backed"
	ClassUnbacked Class = "unbacked"
)

// Decl is a declared invariant slug's declaring ADR, its backing class, and -
// for an unbacked declaration - the `Verify:` guidance text its bullet carries
// (empty when absent). Check treats an empty Verify on an unbacked declaration as
// a MissingVerify finding; ContextFor surfaces the text as the site note.
type Decl struct {
	ADR    string // filename of the declaring ADR
	Class  Class
	Verify string // Verify: guidance text; only meaningful for ClassUnbacked
}

// Note is a non-failing advisory from the invariant scan (ADR-0105 item 5):
// a proof/touches marker naming a slug no Implemented ADR declares, or a bare
// `touches-invariant:` marker carrying no note. Notes ride the `awf check`
// `note:` channel; they never feed the failure count.
type Note struct {
	Slug string
	Text string
}

// Line renders the note as a single human-readable line for the `note:` channel.
func (n Note) Line() string { return n.Text }

// Finding is an Implemented-ADR invariant slug whose backing declaration is not
// satisfied.
type Finding struct {
	Slug   string
	ADR    string // filename of the declaring ADR
	Status Status
}

// Detail is a human, language-neutral remedy line for the finding.
func (f Finding) Detail() string {
	switch f.Status {
	case Unchecked:
		return "unchecked: configure invariants.sources or set invariants.disabled: true"
	case UnbackedHasProof:
		return "declared unbacked but a proof `invariant: " + f.Slug + "` marker exists: reclassify as backed or remove the marker"
	case MissingVerify:
		return "declared unbacked without a `Verify:` note: add manual-verification guidance"
	default: // Unbacked
		return "unbacked: declared backed but no proof `invariant: " + f.Slug + "` marker in backing scope"
	}
}

// Line renders the finding as a single human-readable line (no leading
// indent/column), shared by `awf check` and `awf invariants`.
func (f Finding) Line() string {
	return fmt.Sprintf("%s: invariant %q %s", f.ADR, f.Slug, f.Detail())
}

var (
	// declRe matches an invariant DECLARATION leading a markdown list item
	// (optionally indented): a backed `invariant: <slug>` or an unbacked
	// `unbacked-invariant: <slug>` token. Group 1 is the optional `unbacked-`
	// prefix, group 2 the slug. Only backticks and spaces may sit between the bullet
	// and the token, so both the single-backtick form (`- `+"`invariant: x`") and
	// the double-backtick form ADR-0007 uses to render literal backticks
	// (`- `+"``  `invariant: x`  ``") are recognised, while a mid-prose
	// cross-reference to another ADR's slug is not (it does not lead a list item)
	// - which would otherwise phantom-duplicate that slug.
	declRe = regexp.MustCompile("(?m)^[ \\t]*[-*][ \\t]+[`\\t ]*(unbacked-)?invariant:\\s*([a-z0-9-]+)")
	// verifyRe locates the `Verify:` marker an unbacked declaration must carry; the
	// guidance text is the (whitespace-normalised) bullet remainder after it, so a
	// note wrapped across continuation lines is captured whole (surfaced by ContextFor).
	verifyRe = regexp.MustCompile(`(?i)\bVerify:`)
	// itemStartRe matches a markdown list-item lead - the boundary a wrapped bullet
	// runs until (used to group a declaration bullet with its continuation lines).
	itemStartRe = regexp.MustCompile(`^[ \t]*[-*][ \t]+`)
	// slugRe matches a proof `invariant: <slug>` marker after a source marker.
	slugRe = regexp.MustCompile(`^\s*invariant:\s*([a-z0-9-]+)`)
	// touchesRe matches an advisory `touches-invariant: <slug>[ note]` marker;
	// group 2 is everything after the slug (the trimmed note).
	touchesRe = regexp.MustCompile(`^\s*touches-invariant:\s*([a-z0-9-]+)(.*)$`)
)

// DeclaringADRs returns the slug → declaring-ADR map for adrs: every invariant
// slug declared (in the Invariants section) by an Implemented ADR, carrying its
// backing class (backed `invariant:` / unbacked `unbacked-invariant:`) and, for
// unbacked declarations, the `Verify:` guidance text the bullet carries.
// Token retirements (ADR-0120) are applied. It refuses two Implemented ADRs
// declaring the same slug (duplicate) and a retirement of a slug no ADR
// declares (dangling). Check and ContextFor (ADR-0104 Tier 1) share it.
func DeclaringADRs(adrs []adr.ADR) (map[string]Decl, error) {
	required := map[string]Decl{} // slug -> declaring ADR + class
	for _, a := range adrs {
		if a.Status != "Implemented" {
			continue
		}
		for _, bullet := range invariantBullets(a.Sections["Invariants"]) {
			m := declRe.FindStringSubmatch(bullet)
			if m == nil {
				continue
			}
			slug := m[2]
			if prev, ok := required[slug]; ok {
				return nil, fmt.Errorf("duplicate inv slug %q (in %s and %s)", slug, prev.ADR, a.Filename)
			}
			class := ClassBacked
			if m[1] == "unbacked-" {
				class = ClassUnbacked
			}
			// The `Verify:` note is scanned over the whole bullet - declaration line
			// plus wrapped continuation lines - so a note spanning lines is captured
			// whole. An empty note (bare `Verify:`) leaves verify == "", which Check
			// treats as missing.
			verify := ""
			if loc := verifyRe.FindStringIndex(bullet); loc != nil {
				verify = strings.Trim(strings.Join(strings.Fields(bullet[loc[1]:]), " "), " *")
			}
			required[slug] = Decl{ADR: a.Filename, Class: class, Verify: verify}
		}
	}
	// Token retirements (ADR-0120 item 6): a `supersedes-invariant:` token
	// carried by an Implemented ADR drops its slug from owed backing. Dangling
	// detection scans every ADR's declarations - a slug declared only by a
	// non-Implemented ADR is not owed, so retiring it is a no-op, not an error.
	// touches-invariant: token-retirement-implemented-only - the status test below; proof in invariants_test.go
	// touches-invariant: token-retirement-dangling-errors - the declaredAnywhere refusal; proof in invariants_test.go
	declaredAnywhere := map[string]bool{}
	for _, a := range adrs {
		for _, slug := range DeclaredSlugs(a) {
			declaredAnywhere[slug] = true
		}
	}
	for _, a := range adrs {
		if a.Status != "Implemented" {
			continue
		}
		for _, r := range a.Refs {
			if r.Slug == "" {
				continue
			}
			if !declaredAnywhere[r.Slug] {
				return nil, fmt.Errorf("dangling retirement: ADR %s supersedes invariant %q, which no ADR declares", a.Filename, r.Slug)
			}
			delete(required, r.Slug)
		}
	}
	return required, nil
}

// DeclaredSlugs returns the invariant slugs a's Invariants section declares
// (backed and unbacked alike), in declaration order. Status-independent: the
// ref-validity check and the retirement migration resolve slug anchors against
// any ADR's declarations, not just Implemented ones (ADR-0120 item 2).
func DeclaredSlugs(a adr.ADR) []string {
	var slugs []string
	for _, bullet := range invariantBullets(a.Sections["Invariants"]) {
		if m := declRe.FindStringSubmatch(bullet); m != nil {
			slugs = append(slugs, m[2])
		}
	}
	return slugs
}

// invariantBullets splits an Invariants section into markdown list items, each
// joined with its wrapped continuation lines. A bullet starts at a list-item lead
// and runs until the next list-item lead, a blank line, or the section end - so a
// declaration's `Verify:` note is scanned over the whole bullet, not just its
// first physical line.
func invariantBullets(section string) []string {
	var bullets []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			bullets = append(bullets, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(section, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			flush()
		case itemStartRe.MatchString(line):
			flush()
			cur = []string{line}
		case len(cur) > 0:
			cur = append(cur, line)
		}
	}
	flush()
	return bullets
}

// Check returns the hard Findings and advisory Notes for a project's invariants.
// No required slugs → nil. cfg disabled → nil. cfg nil or source-less → every
// required slug is Unchecked. Otherwise, per the ADR-0105 model: a backed slug
// with no proof marker in backing scope is Unbacked; an unbacked slug with a
// proof marker in scope is UnbackedHasProof; an unbacked slug whose declaration
// lacks a `Verify:` note is MissingVerify. Advisory notes cover a marker naming
// an undeclared slug (dangling) and a bare `touches-invariant:` marker.
func Check(decisionsDir, root string, cfg *config.InvariantConfig) ([]Finding, []Note, error) {
	adrs, err := adr.ParseDir(decisionsDir)
	if err != nil {
		return nil, nil, err
	}
	required, err := DeclaringADRs(adrs)
	if err != nil {
		return nil, nil, err
	}
	if len(required) == 0 {
		return nil, nil, nil
	}
	if cfg != nil && cfg.Disabled {
		return nil, nil, nil
	}

	if cfg == nil || len(cfg.Sources) == 0 {
		out := make([]Finding, 0, len(required))
		for slug, d := range required {
			out = append(out, Finding{Slug: slug, ADR: d.ADR, Status: Unchecked})
		}
		sortFindings(out)
		return out, nil, nil
	}

	scan, err := scanTags(root, cfg)
	if err != nil {
		return nil, nil, err
	}

	var findings []Finding
	for slug, d := range required {
		switch d.Class {
		case ClassUnbacked:
			// ADR-0105 unbacked-refuses-proof (marker added by the migration plan
			// when ADR-0105 flips to Implemented).
			if scan.proofInScope[slug] {
				findings = append(findings, Finding{Slug: slug, ADR: d.ADR, Status: UnbackedHasProof})
			}
			// ADR-0105 unbacked-requires-verify-note.
			if d.Verify == "" {
				findings = append(findings, Finding{Slug: slug, ADR: d.ADR, Status: MissingVerify})
			}
		default: // ClassBacked
			// ADR-0105 backed-requires-proof.
			if !scan.proofInScope[slug] {
				findings = append(findings, Finding{Slug: slug, ADR: d.ADR, Status: Unbacked})
			}
		}
	}
	sortFindings(findings)

	notes := advisoryNotes(required, scan)
	return findings, notes, nil
}

// advisoryNotes derives the non-failing notes: a proof or touches marker naming
// a slug no Implemented ADR declares (dangling-marker), and a bare
// `touches-invariant:` marker carrying no note (bare-touches). Each dangling
// slug and each bare-touches slug is noted once, in slug order.
func advisoryNotes(required map[string]Decl, scan scanResult) []Note {
	danglingSeen := map[string]bool{}
	bareSeen := map[string]bool{}
	var notes []Note
	addDangling := func(slug string) {
		if _, ok := required[slug]; ok || danglingSeen[slug] {
			return
		}
		danglingSeen[slug] = true
		notes = append(notes, Note{Slug: slug, Text: fmt.Sprintf("invariant marker %q names a slug no Implemented ADR declares", slug)})
	}
	for slug := range scan.proofAny {
		addDangling(slug)
	}
	for _, tm := range scan.touches {
		if _, ok := required[tm.Slug]; !ok {
			addDangling(tm.Slug)
			continue
		}
		// ADR-0105 bare-touches-note.
		if tm.Note == "" && !bareSeen[tm.Slug] {
			bareSeen[tm.Slug] = true
			notes = append(notes, Note{Slug: tm.Slug, Text: fmt.Sprintf("touches-invariant marker for %q carries no note", tm.Slug)})
		}
	}
	// Each slug yields at most one note (a slug is either declared - bare-touches,
	// deduped - or undeclared - dangling, deduped), so slug order is total.
	sort.Slice(notes, func(i, j int) bool { return notes[i].Slug < notes[j].Slug })
	return notes
}

// sortFindings orders findings by slug, then status, for deterministic output
// (an unbacked slug can raise both UnbackedHasProof and MissingVerify).
func sortFindings(f []Finding) {
	sort.Slice(f, func(i, j int) bool {
		if f[i].Slug != f[j].Slug {
			return f[i].Slug < f[j].Slug
		}
		return f[i].Status < f[j].Status
	})
}

// touchMark is a `touches-invariant:` marker occurrence: its slug and the
// trimmed free-form note after the slug (empty when none).
type touchMark struct {
	Slug string
	Note string
}

// scanResult is the aggregate of a source-tree scan: proof slugs seen in backing
// scope (proofInScope), proof slugs seen anywhere a source glob matches
// (proofAny, for dangling detection), and every touches marker (touches).
type scanResult struct {
	proofInScope map[string]bool
	proofAny     map[string]bool
	touches      []touchMark
}

// scanTags scans files whose slash-separated repo-relative path matches one of a
// source's anchored globs (ADR-0077; skipping .git/vendor/node_modules and
// nested checkouts). A proof `<marker> invariant: <slug>` comment counts toward
// backing scope when its file matches a `cfg.TestGlobs` pattern, or when
// TestGlobs is empty (source-only fallback, ADR-0105); it is always recorded in
// proofAny. A `<marker> touches-invariant: <slug>[ note]` comment is recorded as
// an advisory touch. The marker is matched literally; whitespace between the
// marker and the token is tolerated.
func scanTags(root string, cfg *config.InvariantConfig) (scanResult, error) {
	res := scanResult{proofInScope: map[string]bool{}, proofAny: map[string]bool{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return fs.SkipDir
			}
			// A subdirectory with its own .git entry - a directory in a
			// primary clone, a gitdir-pointer file in a linked worktree or
			// submodule - is another repository's working tree; a subdirectory
			// carrying its own .awf tree - a nested adopter (e.g. an embedded
			// example project) - is another awf project. Either way its markers
			// back its own ADRs and must not back this project's invariants.
			if path != root {
				if _, lerr := os.Lstat(filepath.Join(path, ".git")); lerr == nil {
					return fs.SkipDir
				}
				if _, lerr := os.Lstat(filepath.Join(path, ".awf")); lerr == nil {
					return fs.SkipDir
				}
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil { // coverage-ignore: WalkDir yields paths under root, so Rel cannot fail
			return rerr
		}
		relSlash := filepath.ToSlash(rel)
		var markers []string
		for _, src := range cfg.Sources {
			for _, g := range src.Globs {
				if pathglob.Match(g, relSlash) {
					markers = append(markers, src.Marker)
					break
				}
			}
		}
		if len(markers) == 0 {
			return nil
		}
		// A proof marker backs a slug only in a test file (matching testGlobs),
		// or in any source file when testGlobs is empty (the ADR-0008 fallback).
		inScope := len(cfg.TestGlobs) == 0 || matchesAnyGlob(cfg.TestGlobs, relSlash)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			// Only a marker opening its line (after indentation) counts: a
			// mid-line match may sit inside a string literal - e.g. a test
			// fixture's source-code string - and must not count.
			trimmed := strings.TrimLeft(line, " \t")
			for _, marker := range markers {
				if !strings.HasPrefix(trimmed, marker) {
					continue
				}
				rest := trimmed[len(marker):]
				if m := slugRe.FindStringSubmatch(rest); m != nil {
					res.proofAny[m[1]] = true
					if inScope {
						res.proofInScope[m[1]] = true
					}
				} else if m := touchesRe.FindStringSubmatch(rest); m != nil {
					res.touches = append(res.touches, touchMark{Slug: m[1], Note: strings.TrimSpace(m[2])})
				}
			}
		}
		return nil
	})
	return res, err
}

// matchesAnyGlob reports whether relSlash matches any of the anchored globs.
func matchesAnyGlob(globs []string, relSlash string) bool {
	for _, g := range globs {
		if pathglob.Match(g, relSlash) {
			return true
		}
	}
	return false
}

// MarkerHit is an invariant slug found under a queried path: the marker kind(s)
// that surfaced it - a proof `invariant:` marker, a `touches-invariant:` marker,
// or both - and, for touches markers, the deduped, sorted, non-empty site notes.
// (ADR-0106: both marker kinds count as present under a path.)
type MarkerHit struct {
	Slug    string
	Proof   bool     // surfaced by a proof `invariant:` marker under the query
	Touches bool     // surfaced by a `touches-invariant:` marker under the query
	Notes   []string // touches-marker site notes (deduped, sorted, non-empty)
}

// MarkersUnder returns the slug-sorted MarkerHits for the invariant markers that
// lie in a file sitting under one of paths (a queried path P owns file F when
// F == P or F is prefixed by P+"/"). A file is scanned when it matches an
// `invariants.sources` glob or a `cfg.TestGlobs` glob (the ADR-0106 union scan),
// so a proof marker in a test file governing production code queried by its path
// still surfaces. Both the proof `invariant: <slug>` and the advisory
// `touches-invariant: <slug>[ note]` markers count as present. paths are
// slash-separated repo-relative paths. It reads only source files and writes
// nothing.
func MarkersUnder(root string, cfg *config.InvariantConfig, paths []string) ([]MarkerHit, error) {
	proof := map[string]bool{}
	touches := map[string]bool{}
	notes := map[string]map[string]bool{} // slug -> set of non-empty touches notes
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return fs.SkipDir
			}
			if path != root {
				if _, lerr := os.Lstat(filepath.Join(path, ".git")); lerr == nil {
					return fs.SkipDir
				}
				if _, lerr := os.Lstat(filepath.Join(path, ".awf")); lerr == nil {
					return fs.SkipDir
				}
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil { // coverage-ignore: WalkDir yields paths under root, so Rel cannot fail
			return rerr
		}
		relSlash := filepath.ToSlash(rel)
		if !underAny(relSlash, paths) {
			return nil
		}
		markers := markersFor(cfg, relSlash)
		if len(markers) == 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			for _, marker := range markers {
				if !strings.HasPrefix(trimmed, marker) {
					continue
				}
				rest := trimmed[len(marker):]
				if m := slugRe.FindStringSubmatch(rest); m != nil {
					proof[m[1]] = true
				} else if m := touchesRe.FindStringSubmatch(rest); m != nil {
					touches[m[1]] = true
					if note := strings.TrimSpace(m[2]); note != "" {
						if notes[m[1]] == nil {
							notes[m[1]] = map[string]bool{}
						}
						notes[m[1]][note] = true
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slugs := map[string]bool{}
	for s := range proof {
		slugs[s] = true
	}
	for s := range touches {
		slugs[s] = true
	}
	out := make([]MarkerHit, 0, len(slugs))
	for s := range slugs {
		var ns []string
		for n := range notes[s] {
			ns = append(ns, n)
		}
		sort.Strings(ns)
		out = append(out, MarkerHit{Slug: s, Proof: proof[s], Touches: touches[s], Notes: ns})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// markersFor returns the deduped comment markers to scan for in the file at
// relSlash: the marker of every source whose glob matches, plus - when the file
// matches a `cfg.TestGlobs` glob (the union scan) - every source marker, so a
// test file matched only by testGlobs is still scanned with the known markers.
func markersFor(cfg *config.InvariantConfig, relSlash string) []string {
	seen := map[string]bool{}
	var markers []string
	add := func(m string) {
		if !seen[m] {
			seen[m] = true
			markers = append(markers, m)
		}
	}
	for _, src := range cfg.Sources {
		if matchesAnyGlob(src.Globs, relSlash) {
			add(src.Marker)
		}
	}
	if matchesAnyGlob(cfg.TestGlobs, relSlash) {
		for _, src := range cfg.Sources {
			add(src.Marker)
		}
	}
	return markers
}

// underAny reports whether rel is one of paths or nested beneath one.
func underAny(rel string, paths []string) bool {
	for _, p := range paths {
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}
