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
		for _, raw := range session.Records {
			out = append(out, wrap("session-v1", raw))
		}
	}
	legacy := append([]LegacyEffortRead(nil), reads.Legacy...)
	sort.Slice(legacy, func(i, j int) bool { return legacy[i].EffortID < legacy[j].EffortID })
	for _, read := range legacy {
		if s.EffortID != nil && *s.EffortID != read.EffortID {
			continue
		}
		for _, raw := range read.Records {
			out = append(out, wrap("legacy-protocol-2", raw))
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
