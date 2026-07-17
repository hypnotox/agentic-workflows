package migrate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
)

var (
	// retiresLineRe matches the column-0 retires_invariants: frontmatter line,
	// newline included, so stripping it is one slice. The corpus form is a
	// single inline list; anything else fails the migration loudly.
	retiresLineRe = regexp.MustCompile(`(?m)^retires_invariants:[^\n]*\n`)
	// relatedLineRe captures the inline related: list a back-pointer inserts into.
	relatedLineRe = regexp.MustCompile(`(?m)^related: \[([^\n\]]*)\]`)
)

// applyRetirementTokens ports a corpus from `retires_invariants:` frontmatter
// to `supersedes-invariant:` tokens (ADR-0120 item 8): it strips the key from
// every ADR, appends a retirement-bookkeeping Decision item carrying one token
// per retired slug (appending never renumbers existing items), and inserts the
// carrier's number into each token target's `related:` back-pointer. Slugs are
// resolved against the pre-edit corpus via the shared declaration grammar
// (invariants.DeclaredSlugs); an unresolvable or multi-declared slug and a
// multi-line key fail loudly, naming the file. Edits are raw-byte string
// surgery, never a frontmatter re-serialization, so untouched lines survive
// byte-identical and meaning-preservation is checkable by diff. Idempotent: a
// corpus with no keys prints nothing.
// touches-invariant: upgrade-migrates-retirements - the migration itself; proof in retirementtokens_test.go
func applyRetirementTokens(root string, out io.Writer) error {
	if _, err := os.Stat(config.ConfigPath(root)); os.IsNotExist(err) {
		return nil // no config: nothing to migrate (idempotent re-run safe)
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return err
	}
	dir := filepath.Join(root, cfg.DocsDir, "decisions")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // no decisions dir: an adopter without the docs module
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		return err
	}
	// Pre-edit resolution state: every ADR's declared slugs (any status) and
	// the parsed related: lists, before any byte surgery.
	declarers := map[string][]string{}
	byNum := map[string]adr.ADR{}
	for _, a := range adrs {
		byNum[a.Number] = a
		for _, slug := range invariants.DeclaredSlugs(a) {
			declarers[slug] = append(declarers[slug], a.Number)
		}
	}

	edited := map[string][]byte{} // path -> pending content
	load := func(a adr.ADR) ([]byte, error) {
		if b, ok := edited[a.Path]; ok {
			return b, nil
		}
		return os.ReadFile(a.Path)
	}

	type edge struct{ target, carrier string }
	var edges []edge
	for _, a := range adrs {
		b, err := load(a)
		if err != nil { // coverage-ignore: ParseDir above already read this exact path
			return err
		}
		raw := string(b)
		// frontmatter.Parse succeeded in ParseDir, so the closing fence exists.
		// +4 keeps the newline before the fence in the search window: the key's
		// trailing newline IS that newline when it is the last frontmatter line.
		fmEnd := strings.Index(raw[3:], "\n---") + 3 + 1
		loc := retiresLineRe.FindStringIndex(raw[:fmEnd])
		if loc == nil {
			continue
		}
		line := strings.TrimSuffix(raw[loc[0]:loc[1]], "\n")
		value := strings.TrimSpace(strings.TrimPrefix(line, "retires_invariants:"))
		if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
			return fmt.Errorf("retirement-tokens: %s: retires_invariants: is not a single-line inline list", a.Filename)
		}
		removed := loc[1] - loc[0]
		raw = raw[:loc[0]] + raw[loc[1]:]
		fmt.Fprintf(out, "retirement-tokens: %s: stripped retires_invariants\n", a.Filename)

		// The slug list comes from the raw line, not the parsed struct: the
		// schema no longer carries the field (ADR-0120 item 7), and this
		// migration is exactly the reader that outlives it.
		var slugs []string
		if inner := strings.TrimSpace(value[1 : len(value)-1]); inner != "" {
			for _, s := range strings.Split(inner, ",") {
				slugs = append(slugs, strings.TrimSpace(s))
			}
		}
		if len(slugs) > 0 {
			tokens := make([]string, len(slugs))
			for i, slug := range slugs {
				ds := declarers[slug]
				if len(ds) == 0 {
					return fmt.Errorf("retirement-tokens: %s retires %q, declared by no ADR", a.Filename, slug)
				}
				if len(ds) > 1 {
					return fmt.Errorf("retirement-tokens: %s retires %q, declared by ADR-%s", a.Filename, slug, strings.Join(ds, " and ADR-"))
				}
				tokens[i] = "`supersedes-invariant: ADR-" + ds[0] + "#" + slug + "`"
				edges = append(edges, edge{target: ds[0], carrier: a.Number})
			}
			items := a.DecisionItems()
			n := 1
			if len(items) > 0 {
				n = items[len(items)-1] + 1
			}
			item := fmt.Sprintf("%d. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,\n   ADR-0120).** This ADR retires %s.\n", n, strings.Join(tokens, ", "))
			if a.DecisionEnd == 0 {
				return fmt.Errorf("retirement-tokens: %s: no Decision section to append the bookkeeping item to", a.Filename)
			}
			at := a.DecisionEnd - removed
			if at == len(raw) {
				raw += "\n" + item
			} else {
				raw = raw[:at] + item + "\n" + raw[at:]
			}
			fmt.Fprintf(out, "retirement-tokens: %s: appended Decision item %d (%s)\n", a.Filename, n, strings.Join(slugs, ", "))
		}
		edited[a.Path] = []byte(raw)
	}

	// Back-pointers: each token target's related: gains the carrier's number
	// when absent (bare int, existing order preserved, appended last).
	inserted := map[edge]bool{}
	for _, e := range edges {
		target := byNum[e.target]
		carrier, _ := strconv.Atoi(e.carrier) // a 4-digit numeral matched by FilenameRe
		if slices.Contains(target.Related, carrier) || inserted[e] {
			continue
		}
		inserted[e] = true
		b, err := load(target)
		if err != nil { // coverage-ignore: ParseDir above already read this exact path
			return err
		}
		raw := string(b)
		// Scope the scan to the frontmatter block, as the key-strip pass does: a
		// column-0 "related:" line in a body (a quoted frontmatter example) must
		// not be silently edited in place of the loud no-line failure.
		fmEnd := strings.Index(raw[3:], "\n---") + 3 + 1
		m := relatedLineRe.FindStringSubmatchIndex(raw[:fmEnd])
		if m == nil {
			return fmt.Errorf("retirement-tokens: %s: no related: line for the back-pointer to ADR-%s", target.Filename, e.carrier)
		}
		entry := strconv.Itoa(carrier)
		if existing := raw[m[2]:m[3]]; existing != "" {
			entry = existing + ", " + entry
		}
		raw = raw[:m[2]] + entry + raw[m[3]:]
		edited[target.Path] = []byte(raw)
		fmt.Fprintf(out, "retirement-tokens: %s: related: gains %d (back-pointer for ADR-%s)\n", target.Filename, carrier, e.carrier)
	}

	for path, b := range edited {
		if err := os.WriteFile(path, b, 0o644); err != nil { // coverage-ignore: writing back a file just read fails only on a permission fault that root bypasses (chmod fixtures are unportable and root-fragile)
			return err
		}
	}
	return nil
}
