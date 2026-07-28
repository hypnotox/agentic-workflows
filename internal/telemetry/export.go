package telemetry

import (
	"encoding/json"
	"sort"
)

func Export(reads ReadSet, s Selector) ([][]byte, error) {
	if err := ValidateSelector(s); err != nil {
		return nil, err
	}
	out := [][]byte{}
	sessions := append([]SessionRead(nil), reads.Sessions...)
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].SessionID < sessions[j].SessionID })
	for _, session := range sessions {
		if s.SessionID != nil && *s.SessionID != session.SessionID {
			continue
		}
		if s.EffortID != nil && reads.Assignments[session.SessionID] != *s.EffortID {
			continue
		}
		for index, raw := range session.Records {
			// Headers have no observation time and remain part of a selected stream.
			if index > 0 {
				var observation Observation
				if json.Unmarshal(raw, &observation) != nil || !selectObservation(observation, s) {
					continue
				}
			}
			out = append(out, wrap("session-v1", raw))
		}
	}
	legacy := append([]LegacyEffortRead(nil), reads.Legacy...)
	sort.Slice(legacy, func(i, j int) bool { return legacy[i].EffortID < legacy[j].EffortID })
	for _, read := range legacy {
		if s.EffortID != nil && *s.EffortID != read.EffortID {
			continue
		}
		entries, identified := read.Entries, len(read.Entries) != 0
		if !identified {
			for _, raw := range read.Records {
				entries = append(entries, LegacyRecord{Source: "legacy-protocol-2", Raw: raw})
			}
		}
		for _, entry := range entries {
			if identified && s.SessionID != nil && entry.SessionID != *s.SessionID {
				continue
			}
			if !selectLegacy(entry.Raw, s) {
				continue
			}
			source := entry.Source
			if source == "" {
				source = legacySource(entry.Raw)
			}
			out = append(out, wrap(source, entry.Raw))
		}
	}
	return out, nil
}
func wrap(source string, raw json.RawMessage) []byte {
	var v any
	_ = json.Unmarshal(raw, &v)
	return mustJSON(map[string]any{"source": source, "record": v})
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
