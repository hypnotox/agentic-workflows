package telemetry

import (
	"encoding/json"
	"fmt"
	"sort"
)

// SelectNormalizedEvents returns selected, validated events in canonical
// effort/session/stream-sequence order. Corrupt raw records are never returned.
func SelectNormalizedEvents(reads []EffortRead, selector Selector) ([]json.RawMessage, error) {
	if err := ValidateSelector(selector); err != nil {
		return nil, err
	}
	ordered := append([]EffortRead(nil), reads...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Metadata.EffortID < ordered[j].Metadata.EffortID })
	result := []json.RawMessage{}
	for _, read := range ordered {
		if issue, fatal := fatalExportIntegrity(read.Integrity); fatal {
			return nil, fmt.Errorf("cannot export effort %s: ledger integrity %s in %s", read.Metadata.EffortID, issue.Code, issue.Scope)
		}
		_, selected, err := selectEffortEvents(read, selector)
		if err != nil { // coverage-ignore: the selector was validated above
			return nil, err
		}
		records := append([]LedgerRecord(nil), read.Records...)
		sort.SliceStable(records, func(i, j int) bool {
			if records[i].SessionID != records[j].SessionID {
				return records[i].SessionID < records[j].SessionID
			}
			return records[i].Line < records[j].Line
		})
		seen := map[string]bool{}
		for _, record := range records {
			if record.Event == nil || !selected[record.Event.EventID] || seen[record.Event.EventID] {
				continue
			}
			normalized := eventToRaw(*record.Event)
			validated, validateErr := ValidateEvent(normalized)
			if validateErr != nil { // coverage-ignore: reader records retain only descriptor-valid events
				return nil, fmt.Errorf("normalize event %s: %w", record.Event.EventID, validateErr)
			}
			result = append(result, eventToRaw(validated))
			seen[record.Event.EventID] = true
		}
	}
	return result, nil
}

func fatalExportIntegrity(issues []IntegrityIssue) (IntegrityIssue, bool) {
	for _, issue := range issues {
		switch issue.Code {
		case "malformed-complete-line", "unsupported-protocol", "conflicting-duplicate", "unsafe-stream-entry", "unsafe-stream-identifier", "unsafe-stream-path", "stream-identity-mismatch", "misplaced-creation-event", "creation-metadata-mismatch", "duplicate-creation-event", "missing-or-duplicate-creation-event":
			return issue, true
		}
	}
	return IntegrityIssue{}, false
}
