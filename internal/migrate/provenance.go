package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

const retireClaimProvenanceMetadataName = "retire-claim-provenance-metadata"

const currentStatePartsDir = ".awf/topics/parts"

var (
	retiredClaimHeading = regexp.MustCompile("^### `(?:rule|invariant): [a-z0-9]+(?:-[a-z0-9]+)*`$")
	retiredADRIdentity  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	retiredDigits       = regexp.MustCompile(`^[0-9]+$`)
)

// retireClaimProvenanceMetadata is deliberately a frozen, migration-owned
// reader for the retired claim layout. It does not use the live topic parser:
// the live parser is allowed to move on with the new schema.
func retireClaimProvenanceMetadata(_ context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
	paths, err := tree.Paths(currentStatePartsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("enumerate current-state parts: %w", err)
	}
	var mutations []FileMutation
	for _, sourcePath := range paths {
		if path.Base(sourcePath) != "current-state.md" {
			continue
		}
		source, mode, err := tree.Read(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", sourcePath, err)
		}
		updated, changed, err := removeRetiredClaimProvenance(source)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", sourcePath, err)
		}
		if changed {
			mutations = append(mutations, FileMutation{Path: sourcePath, Content: updated, Mode: mode})
			changes.Add("removed claim provenance from " + sourcePath)
		}
	}
	return mutations, nil
}

type sourceLine struct {
	raw  []byte
	text string
}

func splitSourceLines(source []byte) []sourceLine {
	var lines []sourceLine
	for len(source) != 0 {
		i := bytes.IndexByte(source, '\n')
		if i < 0 {
			lines = append(lines, sourceLine{raw: source, text: strings.TrimSuffix(string(source), "\r")})
			break
		}
		raw := source[:i+1]
		lines = append(lines, sourceLine{raw: raw, text: strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")})
		source = source[i+1:]
	}
	return lines
}

// removeRetiredClaimProvenance removes only complete Origin, Revised-by pairs
// from the old canonical metadata tail. Its line slices retain every untouched
// byte, including line endings and an absent final newline.
func removeRetiredClaimProvenance(source []byte) ([]byte, bool, error) {
	lines := splitSourceLines(source)
	claimsAt := -1
	fenced, commented := byte(0), false
	for i := range lines {
		ignored, nextFence, nextComment, err := scannerIgnored(lines[i].text, fenced, commented)
		if err != nil {
			return nil, false, fmt.Errorf("line %d: %w", i+1, err)
		}
		if !ignored && lines[i].text == "## Claims" {
			if claimsAt >= 0 {
				return nil, false, errors.New("topic part must contain exactly one ## Claims section")
			}
			claimsAt = i
		}
		fenced, commented = nextFence, nextComment
	}
	if fenced != 0 || commented {
		return nil, false, errors.New("unterminated fenced code or authoring comment")
	}
	if claimsAt < 0 {
		return nil, false, errors.New("missing ## Claims section")
	}

	var headings []int
	fenced, commented = byte(0), false
	for i := claimsAt + 1; i < len(lines); i++ {
		ignored, nextFence, nextComment, err := scannerIgnored(lines[i].text, fenced, commented)
		if err != nil {
			return nil, false, fmt.Errorf("line %d: %w", i+1, err)
		}
		if !ignored {
			if strings.HasPrefix(lines[i].text, "## ") {
				return nil, false, errors.New("## Claims must be the final level-two section")
			}
			if retiredClaimHeading.MatchString(lines[i].text) {
				headings = append(headings, i)
			} else if strings.HasPrefix(lines[i].text, "### ") {
				return nil, false, fmt.Errorf("invalid claim heading at line %d", i+1)
			}
		}
		fenced, commented = nextFence, nextComment
	}
	if fenced != 0 || commented {
		return nil, false, errors.New("unterminated fenced code or authoring comment")
	}

	remove := make(map[int]bool)
	for n, start := range headings {
		end := len(lines)
		if n+1 < len(headings) {
			end = headings[n+1]
		}
		if err := locateRetiredProvenance(lines, start+1, end, remove); err != nil {
			return nil, false, fmt.Errorf("claim at line %d: %w", start+1, err)
		}
	}
	if len(remove) == 0 {
		return append([]byte(nil), source...), false, nil
	}
	out := make([]byte, 0, len(source))
	for i, line := range lines {
		if !remove[i] {
			out = append(out, line.raw...)
		}
	}
	if len(source) > 0 && source[len(source)-1] != '\n' && remove[len(lines)-1] {
		out = bytes.TrimSuffix(out, []byte{'\n'})
		out = bytes.TrimSuffix(out, []byte{'\r'})
	}
	return out, true, nil
}

// scannerIgnored reports whether a line is comment/fence content and advances
// the minimal Markdown state needed to keep lookalike metadata out of scope.
func scannerIgnored(line string, fenced byte, commented bool) (bool, byte, bool, error) {
	if commented {
		if strings.Contains(line, "-->") {
			return true, fenced, false, nil
		}
		return true, fenced, true, nil
	}
	if strings.Contains(line, "<!--") {
		if !strings.Contains(line[strings.Index(line, "<!--")+4:], "-->") {
			return true, fenced, true, nil
		}
		return true, fenced, false, nil
	}
	marker := fenceMarker(line)
	if fenced != 0 {
		if marker == fenced {
			return true, 0, false, nil
		}
		return true, fenced, false, nil
	}
	if marker != 0 {
		return true, marker, false, nil
	}
	if strings.Contains(line, "-->") {
		return false, 0, false, errors.New("unexpected authoring comment close")
	}
	return false, 0, false, nil
}

func fenceMarker(line string) byte {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return 0
	}
	if (trimmed[0] != '`' && trimmed[0] != '~') || trimmed[1] != trimmed[0] || trimmed[2] != trimmed[0] {
		return 0
	}
	return trimmed[0]
}

func locateRetiredProvenance(lines []sourceLine, start, end int, remove map[int]bool) error {
	// A raw comment is semantically stripped by the old parser, so it is
	// transparent when finding the metadata tail but is never itself removed.
	semantic := make([]int, 0, end-start)
	lastFence := -1
	fenced, commented := byte(0), false
	for i := start; i < end; i++ {
		wasFenced := fenced != 0
		ignored, nextFence, nextComment, err := scannerIgnored(lines[i].text, fenced, commented)
		if err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
		if !ignored {
			semantic = append(semantic, i)
		} else if wasFenced || nextFence != 0 {
			// Fenced material is not metadata, but it still means preceding
			// metadata is not the claim's trailing metadata tail.
			lastFence = i
		}
		fenced, commented = nextFence, nextComment
	}
	if fenced != 0 || commented {
		return errors.New("unterminated fenced code or authoring comment")
	}
	for len(semantic) > 0 && strings.TrimSpace(lines[semantic[len(semantic)-1]].text) == "" {
		semantic = semantic[:len(semantic)-1]
	}
	metaStart := len(semantic)
	for metaStart > 0 && metadataKind(lines[semantic[metaStart-1]].text) != "" {
		metaStart--
	}
	for _, index := range semantic[:metaStart] {
		if reservedMetadata(lines[index].text) {
			return fmt.Errorf("malformed or out-of-order metadata at line %d", index+1)
		}
	}
	meta := semantic[metaStart:]
	origin, revised := -1, -1
	lastKind := ""
	for _, index := range meta {
		kind := metadataKind(lines[index].text)
		if kind == "" || metadataOrder(kind) <= metadataOrder(lastKind) {
			return fmt.Errorf("malformed metadata at line %d", index+1)
		}
		lastKind = kind
		if kind == "origin" {
			if !validRetiredADR(strings.TrimPrefix(lines[index].text, "Origin: ")) {
				return fmt.Errorf("invalid Origin metadata at line %d", index+1)
			}
			origin = index
		}
		if kind == "revised" {
			if !validRetiredADRList(strings.TrimPrefix(lines[index].text, "Revised-by: ")) {
				return fmt.Errorf("invalid Revised-by metadata at line %d", index+1)
			}
			revised = index
		}
	}
	if revised >= 0 && origin < 0 {
		return errors.New("Revised-by requires Origin")
	}
	if origin >= 0 && origin < lastFence {
		return errors.New("Origin is not trailing metadata")
	}
	if origin >= 0 {
		remove[origin] = true
		if revised >= 0 {
			remove[revised] = true
		}
	}
	return nil
}

func validRetiredADR(value string) bool {
	if !strings.HasPrefix(value, "ADR-") {
		return false
	}
	identity := strings.TrimPrefix(value, "ADR-")
	if identity == "" || !retiredADRIdentity.MatchString(identity) {
		return false
	}
	return !retiredDigits.MatchString(identity) || len(identity) == 4
}

func validRetiredADRList(value string) bool {
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if !validRetiredADR(item) || seen[item] {
			return false
		}
		seen[item] = true
	}
	return true
}

func metadataKind(line string) string {
	for prefix, kind := range map[string]string{"Summary: ": "summary", "Origin: ": "origin", "Revised-by: ": "revised", "References: ": "references", "Backing: ": "backing", "Verify: ": "verify"} {
		if strings.HasPrefix(line, prefix) {
			return kind
		}
	}
	return ""
}
func reservedMetadata(line string) bool {
	for _, name := range []string{"Summary", "Origin", "Revised-by", "References", "Backing", "Verify"} {
		if strings.HasPrefix(line, name+":") || strings.HasPrefix(line, name+" ") {
			return true
		}
	}
	return false
}
func metadataOrder(kind string) int {
	switch kind {
	case "summary":
		return 1
	case "origin":
		return 2
	case "revised":
		return 3
	case "references":
		return 4
	case "backing":
		return 5
	case "verify":
		return 6
	}
	return 0
}
