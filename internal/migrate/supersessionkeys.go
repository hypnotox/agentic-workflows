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
)

var (
	// supersedesLineRe and supersededByLineRe match the two column-0
	// frontmatter lines ADR-0128 item 1 removes, newline included so stripping
	// one is a single slice.
	supersedesLineRe   = regexp.MustCompile(`(?m)^supersedes:[^\n]*\n`)
	supersededByLineRe = regexp.MustCompile(`(?m)^superseded_by:[^\n]*\n`)
	// supersedesValueRe captures a supersedes: inline list's contents.
	supersedesValueRe = regexp.MustCompile(`(?m)^supersedes: \[([^\n\]]*)\]`)
	// itemTokenRe matches a pre-existing inline item token, which the
	// migration downgrades to the refinement relation.
	itemTokenRe = regexp.MustCompile("`supersedes: (ADR-[0-9]{4}#[1-9][0-9]*)`")
	// suffixedStatusRe matches the retired suffixed status form.
	suffixedStatusRe = regexp.MustCompile(`(?m)^status: Superseded by ADR-[0-9]{4}[^\n]*$`)
)

// applySupersessionKeys ports a corpus from frontmatter-encoded full
// supersedence to coverage-derived supersedence (ADR-0128 item 8).
//
// The order of the three passes is load-bearing. Every pre-existing item token
// is first downgraded to `refines:`, against the pre-append body: the
// mechanical rewrite is deliberately conservative, because the corpus is 22
// refinements to 13 genuine retirements and asserting less is the safe
// direction (an author promotes the handful that are real retirements back).
// Only then are the bookkeeping items appended, so the retirement tokens they
// carry survive - reversed, the rewrite would downgrade the very tokens the
// append had just written and deliver the legacy pairs straight into a
// coverage failure.
//
// Then both keys are stripped, each predecessor named by a `supersedes:` list
// gains one bookkeeping Decision item carrying a retirement token per anchor,
// the carrier's number is inserted into each predecessor's related: when
// absent, and each predecessor's suffixed status is rewritten to bare
// `Superseded`.
//
// Edits are raw-byte string surgery, never a frontmatter re-serialization, so
// untouched lines survive byte-identical and meaning-preservation is checkable
// by diff. Idempotency rests on the generation gate: appending an item is not
// naturally idempotent the way stripping a key is.
// touches-invariant: upgrade-migrates-supersession-keys - the migration itself; proof in supersessionkeys_test.go
func applySupersessionKeys(root string, out io.Writer) error {
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
	corpus, err := adr.LoadCorpus(dir)
	if err != nil {
		return err
	}
	adrs := corpus.All()

	edited := map[string][]byte{} // path -> pending content
	// shift tracks pass 1's net byte delta per file. The parsed DecisionEnd
	// offsets come from the pre-edit bytes, and the token rewrite shortens the
	// body by three bytes per token ("supersedes" -> "refines"), so pass 2 must
	// correct for it before appending at that offset. Without this the
	// bookkeeping item lands inside whatever heading follows the Decision
	// section - which for a token-dense ADR means corrupting `## Invariants`.
	shift := map[string]int{}
	load := func(a adr.ADR) ([]byte, error) {
		if b, ok := edited[a.Path]; ok {
			return b, nil
		}
		return corpus.Raw(a.Number)
	}

	// Pass 1: downgrade every pre-existing item token to the refinement
	// relation, against the pre-append body.
	for _, a := range adrs {
		b, err := load(a)
		if err != nil { // coverage-ignore: LoadCorpus above already read this exact path
			return err
		}
		raw := string(b)
		if a.DecisionEnd == 0 {
			continue // no Decision section: no tokens to rewrite
		}
		body := raw[a.DecisionStart:a.DecisionEnd]
		rewritten := itemTokenRe.ReplaceAllString(body, "`refines: $1`")
		if rewritten == body {
			continue
		}
		n := strings.Count(body, "`supersedes: ADR-") - strings.Count(rewritten, "`supersedes: ADR-")
		shift[a.Path] = len(rewritten) - len(body)
		edited[a.Path] = []byte(raw[:a.DecisionStart] + rewritten + raw[a.DecisionEnd:])
		fmt.Fprintf(out, "supersession-keys: %s: %d item token(s) downgraded to refines:\n", a.Filename, n)
	}

	type edge struct{ target, carrier string }
	var edges []edge

	// carriesBookkeeping records which ADRs will gain a bookkeeping Decision
	// item of their own. That item is a NEW anchor, so a chained supersession
	// (A supersedes B, B supersedes C) must have A retire it too: enumerating
	// B's anchors from the pre-migration parse alone would leave B one anchor
	// short of covered while its status was rewritten to Superseded, and the
	// very next awf check would fail on a corpus this migration just produced.
	carriesBookkeeping := map[string]bool{}
	for _, a := range adrs {
		b, err := load(a)
		if err != nil { // coverage-ignore: LoadCorpus above already read this exact path
			return err
		}
		raw := string(b)
		fmEnd := strings.Index(raw[3:], "\n---") + 3 + 1
		if m := supersedesValueRe.FindStringSubmatch(raw[:fmEnd]); m != nil && strings.TrimSpace(m[1]) != "" {
			carriesBookkeeping[a.Number] = true
		}
	}

	// pendingAnchors is a target's anchor set as it will stand AFTER the
	// migration: its parsed anchors plus the bookkeeping item it is about to
	// gain, if any.
	pendingAnchors := func(target adr.ADR) []adr.Anchor {
		anchors := corpus.Anchors(target.Number)
		if !carriesBookkeeping[target.Number] {
			return anchors
		}
		items := target.DecisionItems()
		next := 1
		if len(items) > 0 {
			next = items[len(items)-1] + 1
		}
		// Slot the pending item with the other items rather than after the
		// slugs, so the emitted token list reads in anchor order.
		out := make([]adr.Anchor, 0, len(anchors)+1)
		for _, a := range anchors {
			if a.Slug != "" && len(out) == len(items) {
				out = append(out, adr.Anchor{ADR: target.Number, Item: next})
			}
			out = append(out, a)
		}
		if len(out) == len(items) { // no slugs: the pending item goes last
			out = append(out, adr.Anchor{ADR: target.Number, Item: next})
		}
		return out
	}

	// Pass 2: strip both keys, and append a bookkeeping item for each
	// predecessor a supersedes: list named.
	for _, a := range adrs {
		b, err := load(a)
		if err != nil { // coverage-ignore: LoadCorpus above already read this exact path
			return err
		}
		raw := string(b)
		fmEnd := strings.Index(raw[3:], "\n---") + 3 + 1

		// Refuse rather than corrupt. supersedesLineRe strips the key line
		// unconditionally, but supersedesValueRe only recognises the inline
		// list form, so a block-style list would have its key stripped and its
		// orphaned `  - 2` entries left inside the frontmatter - invalid YAML,
		// a silently dropped supersession claim, and a corpus that no longer
		// parses. Any supersedes: line the value regex cannot read is a hard
		// stop naming the file.
		var predecessors []string
		if supersedesLineRe.MatchString(raw[:fmEnd]) && !supersedesValueRe.MatchString(raw[:fmEnd]) {
			return fmt.Errorf("supersession-keys: %s: supersedes: is not a single-line inline list; rewrite it as `supersedes: [N]` (or `[]`) and re-run", a.Filename)
		}
		if m := supersedesValueRe.FindStringSubmatch(raw[:fmEnd]); m != nil {
			if inner := strings.TrimSpace(m[1]); inner != "" {
				for _, s := range strings.Split(inner, ",") {
					n, err := strconv.Atoi(strings.TrimSpace(s))
					if err != nil {
						return fmt.Errorf("supersession-keys: %s: supersedes: entry %q is not an ADR number", a.Filename, strings.TrimSpace(s))
					}
					predecessors = append(predecessors, fmt.Sprintf("%04d", n))
				}
			}
		}

		before := len(raw)
		raw = stripKeyLine(raw, supersedesLineRe)
		raw = stripKeyLine(raw, supersededByLineRe)
		removed := before - len(raw)
		if removed > 0 {
			fmt.Fprintf(out, "supersession-keys: %s: stripped supersedes:/superseded_by:\n", a.Filename)
		}

		if len(predecessors) > 0 {
			var tokens []string
			for _, pred := range predecessors {
				target, ok := corpus.ByNumber(pred)
				if !ok {
					return fmt.Errorf("supersession-keys: %s supersedes ADR-%s, which does not exist", a.Filename, pred)
				}
				anchors := pendingAnchors(target)
				if len(anchors) == 0 {
					return fmt.Errorf("supersession-keys: %s supersedes ADR-%s, which has no Decision items to retire", a.Filename, pred)
				}
				for _, anchor := range anchors {
					// The kind is named by the key, never inferred from the
					// anchor shape (ADR-0120 item 1): a slug anchor takes
					// supersedes-invariant:, and emitting the item key for one
					// would write a token the grammar does not recognise.
					key := "supersedes: "
					if anchor.Slug != "" {
						key = "supersedes-invariant: "
					}
					tokens = append(tokens, "`"+key+anchor.String()+"`")
				}
				edges = append(edges, edge{target: pred, carrier: a.Number})
			}
			items := a.DecisionItems()
			n := 1
			if len(items) > 0 {
				n = items[len(items)-1] + 1
			}
			item := fmt.Sprintf("%d. **Supersedence bookkeeping (migrated from supersedes: by awf upgrade,\n   ADR-0128).** This ADR retires every anchor of ADR-%s: %s\n",
				n, strings.Join(predecessors, ", ADR-"), strings.Join(tokens, ", "))
			if a.DecisionEnd == 0 {
				return fmt.Errorf("supersession-keys: %s: no Decision section to append the bookkeeping item to", a.Filename)
			}
			at := a.DecisionEnd + shift[a.Path] - removed
			if at == len(raw) {
				raw += "\n" + item
			} else {
				raw = raw[:at] + item + "\n" + raw[at:]
			}
			fmt.Fprintf(out, "supersession-keys: %s: appended Decision item %d (retires ADR-%s)\n", a.Filename, n, strings.Join(predecessors, ", ADR-"))
		}
		edited[a.Path] = []byte(raw)
	}

	// Pass 3: back-pointers, then the predecessors' status rewrite. Both are
	// keyed on the same edge set, so a predecessor named by no carrier is left
	// entirely alone.
	inserted := map[edge]bool{}
	for _, e := range edges {
		target, _ := corpus.ByNumber(e.target) // pass 2 refused a missing target
		carrier, _ := strconv.Atoi(e.carrier)  // a 4-digit numeral matched by FilenameRe
		b, err := load(target)
		if err != nil { // coverage-ignore: LoadCorpus above already read this exact path
			return err
		}
		raw := string(b)
		fmEnd := strings.Index(raw[3:], "\n---") + 3 + 1

		// The related: back-pointer is genuinely needed: none of the three
		// legacy predecessors names its claimant today.
		if !slices.Contains(target.Related, carrier) && !inserted[e] {
			inserted[e] = true
			m := relatedLineRe.FindStringSubmatchIndex(raw[:fmEnd])
			if m == nil {
				return fmt.Errorf("supersession-keys: %s: no related: line for the back-pointer to ADR-%s", target.Filename, e.carrier)
			}
			entry := strconv.Itoa(carrier)
			if existing := raw[m[2]:m[3]]; existing != "" {
				entry = existing + ", " + entry
			}
			raw = raw[:m[2]] + entry + raw[m[3]:]
			fmt.Fprintf(out, "supersession-keys: %s: related: gains %d (back-pointer for ADR-%s)\n", target.Filename, carrier, e.carrier)
			fmEnd = strings.Index(raw[3:], "\n---") + 3 + 1
		}

		// Bare `Superseded`: coverage may split across successors, so a status
		// naming one of them would lie (ADR-0128 item 4). The claimants are
		// recoverable from the related: back-pointers written above.
		if loc := suffixedStatusRe.FindStringIndex(raw[:fmEnd]); loc != nil {
			raw = raw[:loc[0]] + "status: Superseded" + raw[loc[1]:]
			fmt.Fprintf(out, "supersession-keys: %s: status rewritten to bare Superseded\n", target.Filename)
		}
		edited[target.Path] = []byte(raw)
	}

	for path, content := range edited {
		if err := os.WriteFile(path, content, 0o644); err != nil { // coverage-ignore: the path was just read successfully
			return err
		}
	}
	return nil
}

// stripKeyLine removes the first match of re from the frontmatter block only,
// so a column-0 key inside a body (a quoted frontmatter example) is never
// silently edited.
func stripKeyLine(raw string, re *regexp.Regexp) string {
	fmEnd := strings.Index(raw[3:], "\n---") + 3 + 1
	loc := re.FindStringIndex(raw[:fmEnd])
	if loc == nil {
		return raw
	}
	return raw[:loc[0]] + raw[loc[1]:]
}
