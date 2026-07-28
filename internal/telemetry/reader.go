package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

var readEffortList = func(svc *effort.Service) ([]effort.Record, error) { return svc.List() }
var readEffortAssignments = func(svc *effort.Service) ([]effort.Assignment, error) { return svc.Assignments("") }
var inspectLegacyDirectory = inspectDirectory

// Read loads new streams and legacy ledgers without mutating either. New stream
// attribution is intentionally joined from the current assignment map.
func Read(ctx context.Context, invokingRoot string) (ReadSet, error) {
	p, err := resolvePaths(ctx, invokingRoot)
	if err != nil {
		return ReadSet{}, err
	}
	svc, err := effort.Open(ctx, invokingRoot, effort.Options{})
	if err != nil {
		return ReadSet{}, err
	}
	records, err := readEffortList(svc)
	if err != nil {
		return ReadSet{}, err
	}
	out := ReadSet{Records: map[string]effort.Record{}, Assignments: map[string]string{}, Sessions: []SessionRead{}, Legacy: []LegacyEffortRead{}, Findings: []IntegrityFinding{}}
	for _, r := range records {
		out.Records[r.ID] = r
	}
	assignments, err := readEffortAssignments(svc)
	if err != nil {
		return ReadSet{}, err
	}
	for _, a := range assignments {
		out.Assignments[a.SessionID] = a.EffortID
	}
	if entries, err := os.ReadDir(p.sessions); err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			id := strings.TrimSuffix(entry.Name(), ".jsonl")
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") || !Identifier(id) {
				out.Findings = append(out.Findings, finding("session-v1", id, "unsafe-stream-entry"))
				continue
			}
			stream := readSession(filepath.Join(p.sessions, entry.Name()), id)
			out.Sessions = append(out.Sessions, stream)
			out.Findings = append(out.Findings, stream.Findings...)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ReadSet{}, fmt.Errorf("read sessions: %w", err)
	}
	legacy, findings, err := readLegacy(p.efforts)
	if err != nil {
		return ReadSet{}, err
	}
	out.Legacy = legacy
	out.Findings = append(out.Findings, findings...)
	sort.Slice(out.Sessions, func(i, j int) bool { return out.Sessions[i].SessionID < out.Sessions[j].SessionID })
	sortFindings(out.Findings)
	return out, nil
}
func finding(source, session, code string) IntegrityFinding {
	return IntegrityFinding{Source: source, SessionID: session, Code: code}
}
func readSession(path, id string) SessionRead {
	out := SessionRead{SessionID: id, Observations: []Observation{}, Records: []json.RawMessage{}, Findings: []IntegrityFinding{}}
	if _, err := inspectRegular(path); err != nil {
		out.Findings = append(out.Findings, finding("session-v1", id, "unsafe-stream-path"))
		return out
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		out.Findings = append(out.Findings, finding("session-v1", id, "read-failure"))
		return out
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		out.Findings = append(out.Findings, finding("session-v1", id, "missing-final-lf"))
		return out
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) == 0 || len(lines[0]) == 0 {
		out.Findings = append(out.Findings, finding("session-v1", id, "missing-header"))
		return out
	}
	h, err := ValidateHeader(lines[0])
	if err != nil {
		out.Findings = append(out.Findings, finding("session-v1", id, "invalid-header"))
		return out
	}
	out.Header = h
	if h.SessionID != id {
		out.Findings = append(out.Findings, finding("session-v1", id, "header-identity-mismatch"))
		return out
	}
	out.Records = append(out.Records, append(json.RawMessage(nil), lines[0]...))
	seen := map[string]bool{}
	for _, line := range lines[1:] {
		if len(line) == 0 {
			out.Findings = append(out.Findings, finding("session-v1", id, "malformed-record"))
			continue
		}
		o, e := ValidateObservation(line)
		if e != nil {
			out.Findings = append(out.Findings, finding("session-v1", id, "malformed-record"))
			continue
		}
		if seen[o.ObservationID] {
			out.Findings = append(out.Findings, finding("session-v1", id, "duplicate-observation-id"))
			continue
		}
		seen[o.ObservationID] = true
		out.Observations = append(out.Observations, o)
		out.Records = append(out.Records, append(json.RawMessage(nil), line...))
	}
	sort.Slice(out.Observations, func(i, j int) bool {
		if !out.Observations[i].Timestamp.Equal(out.Observations[j].Timestamp) {
			return out.Observations[i].Timestamp.Before(out.Observations[j].Timestamp)
		}
		return out.Observations[i].ObservationID < out.Observations[j].ObservationID
	})
	sortFindings(out.Findings)
	return out
}
func readLegacy(root string) ([]LegacyEffortRead, []IntegrityFinding, error) {
	out := []LegacyEffortRead{}
	findings := []IntegrityFinding{}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return out, findings, nil
	}
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || !Identifier(id) {
			findings = append(findings, finding("legacy", id, "unsafe-legacy-entry"))
			continue
		}
		dir := filepath.Join(root, id)
		if _, err := inspectLegacyDirectory(dir); err != nil {
			findings = append(findings, finding("legacy", id, "unsafe-legacy-path"))
			continue
		}
		read := LegacyEffortRead{EffortID: id, Records: []json.RawMessage{}, Entries: []LegacyRecord{}, Findings: []IntegrityFinding{}}
		metadata := filepath.Join(dir, "effort.json")
		if b, e := os.ReadFile(metadata); e == nil {
			raw := append(json.RawMessage(nil), bytes.TrimSuffix(b, []byte{'\n'})...)
			read.Records = append(read.Records, raw)
			read.Entries = append(read.Entries, LegacyRecord{Source: "legacy-protocol-2", Raw: raw})
		}
		sessions := filepath.Join(dir, "sessions")
		if sessionEntries, e := os.ReadDir(sessions); e == nil {
			sort.Slice(sessionEntries, func(i, j int) bool { return sessionEntries[i].Name() < sessionEntries[j].Name() })
			for _, se := range sessionEntries {
				sid := strings.TrimSuffix(se.Name(), ".jsonl")
				if se.IsDir() || !strings.HasSuffix(se.Name(), ".jsonl") || !Identifier(sid) {
					read.Findings = append(read.Findings, finding("legacy-protocol-1", sid, "unsafe-legacy-stream"))
					continue
				}
				b, e := os.ReadFile(filepath.Join(sessions, se.Name()))
				if e != nil {
					read.Findings = append(read.Findings, finding("legacy-protocol-1", sid, "read-failure"))
					continue
				}
				for _, line := range bytes.Split(bytes.TrimSuffix(b, []byte{'\n'}), []byte{'\n'}) {
					if len(line) > 0 {
						raw := append(json.RawMessage(nil), line...)
						read.Records = append(read.Records, raw)
						read.Entries = append(read.Entries, LegacyRecord{Source: legacySource(raw), SessionID: sid, Raw: raw})
					}
				}
			}
		}
		out = append(out, read)
		findings = append(findings, read.Findings...)
	}
	return out, findings, nil
}
func legacySource(raw json.RawMessage) string {
	var record struct {
		Version struct {
			Major int `json:"major"`
		} `json:"version"`
	}
	if json.Unmarshal(raw, &record) == nil && record.Version.Major == 1 {
		return "legacy-protocol-1"
	}
	return "legacy-protocol-2"
}

func sortFindings(values []IntegrityFinding) {
	sort.Slice(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.SessionID != b.SessionID {
			return a.SessionID < b.SessionID
		}
		return a.Code < b.Code
	})
}
