package currentstate

import (
	"bytes"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

// Qualification reports whether one older-format introduction is carried by
// an incoming parent with only sanctioned integration substitutions.
type Qualification struct {
	Introduction Introduction
	Qualified    bool
}

// QualifyIncoming qualifies every provisional result introduction against all
// incoming parents and returns results in identity order.
func QualifyIncoming(first, result Universe, incoming []Universe, current adr.Format) []Qualification {
	introductions := OlderIntroductions(first, result, current)
	out := make([]Qualification, 0, len(introductions))
	for _, introduction := range introductions {
		qualified := false
		resultRecord, ok := recordByIdentity(result.ADRs, introduction.Identity)
		if ok {
			for _, parent := range incoming {
				pairs := newPairing(parent.ADRs, result.ADRs)
				parentRecord, paired := pairs.before(resultRecord)
				if !paired || parentRecord.Format != resultRecord.Format {
					continue
				}
				if sourcesQualify(parentRecord, resultRecord, parent.Sources[parentRecord.Identity()], result.Sources[resultRecord.Identity()]) {
					qualified = true
					break
				}
			}
		}
		out = append(out, Qualification{Introduction: introduction, Qualified: qualified})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Introduction.Identity < out[j].Introduction.Identity })
	return out
}

func recordByIdentity(records []adr.ADR, identity string) (adr.ADR, bool) {
	for _, record := range records {
		if record.Identity() == identity {
			return record, true
		}
	}
	return adr.ADR{}, false // coverage-ignore: the identity is selected directly from the same result ADR slice
}

func sourcesQualify(parent, result adr.ADR, parentSource, resultSource []byte) bool {
	if parentSource == nil || resultSource == nil {
		return false
	}
	if bytes.Equal(parentSource, resultSource) {
		return true
	}
	oldIdentity, newIdentity := parent.Identity(), result.Identity()
	if oldIdentity == newIdentity {
		return false
	}
	lines := strings.SplitAfter(string(parentSource), "\n")
	for i, line := range lines {
		ending := ""
		body := line
		if strings.HasSuffix(body, "\n") {
			body, ending = strings.TrimSuffix(body, "\n"), "\n"
		}
		if strings.HasPrefix(body, "# ADR-"+oldIdentity+": ") {
			body = "# ADR-" + newIdentity + strings.TrimPrefix(body, "# ADR-"+oldIdentity)
		}
		if parent.IsGoverned() && (strings.HasPrefix(body, "Origin:") || strings.HasPrefix(body, "Revised-by:")) {
			body = strings.ReplaceAll(body, "ADR-"+oldIdentity, "ADR-"+newIdentity)
		}
		lines[i] = body + ending
	}
	return bytes.Equal([]byte(strings.Join(lines, "")), resultSource)
}
