package telemetry

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	DefaultEffortPageLimit = 10
	MaximumEffortPageLimit = 100
)

var (
	ErrSelectedEffortRequired     = errors.New("an effort selection is required")
	ErrSelectedEffortUnknown      = errors.New("selected effort is not resident")
	ErrSelectedEffortIncompatible = errors.New("selected effort requires an unsupported protocol")
	ErrSelectedEffortEmpty        = errors.New("selected effort has no events matching the selectors")
)

// EffortListRow is the bounded discovery projection. Incompatible efforts do
// not expose interpreted state and remain eligible only as pagination anchors.
type EffortListRow struct {
	EffortID      string     `json:"effortId"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastAppliedAt *time.Time `json:"lastAppliedAt,omitempty"`
	Incompatible  bool       `json:"incompatible,omitempty"`
	State         string     `json:"state,omitempty"`
	Route         string     `json:"route,omitempty"`
	Phase         string     `json:"phase,omitempty"`
	Outcome       string     `json:"outcome,omitempty"`
	Discovery     bool       `json:"discovery,omitempty"`
}

type EffortListPage struct {
	SchemaVersion int             `json:"schemaVersion"`
	Efforts       []EffortListRow `json:"efforts"`
	NextCursor    string          `json:"nextCursor,omitempty"`
}

type effortCursor struct {
	Version   int    `json:"v"`
	CreatedAt string `json:"createdAt"`
	EffortID  string `json:"effortId"`
}

// SelectEffort rejects broad, unknown, incompatible, and empty selections so
// report callers can never fall back to repository-wide aggregation.
func SelectEffort(reads []EffortRead, selector Selector) (EffortRead, error) {
	if err := ValidateSelector(selector); err != nil {
		return EffortRead{}, err
	}
	if selector.EffortID == nil || *selector.EffortID == "" {
		return EffortRead{}, ErrSelectedEffortRequired
	}
	for _, read := range reads {
		if read.Metadata.EffortID != *selector.EffortID {
			continue
		}
		if !effortProjectionCompatible(read) {
			return EffortRead{}, ErrSelectedEffortIncompatible
		}
		events, _, _ := selectEffortEvents(read, selector) // selector validation above makes event selection infallible
		if len(events) == 0 {
			return EffortRead{}, ErrSelectedEffortEmpty
		}
		return read, nil
	}
	return EffortRead{}, ErrSelectedEffortUnknown
}

func AggregateSelectedMetrics(reads []EffortRead, selector Selector, options MetricsOptions) (MetricsResult, error) {
	read, err := SelectEffort(reads, selector)
	if err != nil {
		return MetricsResult{}, err
	}
	return AggregateMetrics([]EffortRead{read}, selector, options)
}

func DiagnoseSelected(reads []EffortRead, selector Selector, options HeuristicOptions, generatedAt time.Time) (DoctorResult, error) {
	read, err := SelectEffort(reads, selector)
	if err != nil {
		return DoctorResult{}, err
	}
	return Diagnose([]EffortRead{read}, selector, options, generatedAt)
}

// ListEfforts returns the resident effort directory in descending immutable
// creation time, with byte-ascending effort IDs resolving ties.
func ListEfforts(reads []EffortRead, limit int, cursor string) (EffortListPage, error) {
	if limit < 1 || limit > MaximumEffortPageLimit {
		return EffortListPage{}, fmt.Errorf("limit must be between 1 and %d", MaximumEffortPageLimit)
	}
	rows := make([]EffortListRow, 0, len(reads))
	for _, read := range reads {
		created, err := time.Parse(time.RFC3339Nano, read.Metadata.CreatedAt)
		if err != nil {
			return EffortListPage{}, fmt.Errorf("invalid effort creation time for %s", read.Metadata.EffortID)
		}
		row := EffortListRow{EffortID: read.Metadata.EffortID, CreatedAt: created.UTC()}
		if !effortProjectionCompatible(read) {
			row.Incompatible = true
			rows = append(rows, row)
			continue
		}
		row.LastAppliedAt = lastAppliedAt(read)
		lifecycle := projectLifecycleFromRead(read)
		row.State, row.Route = string(lifecycle.State), string(lifecycle.Route)
		row.Phase = string(currentListPhase(lifecycle))
		if row.State == string(EffortCompleted) || row.State == string(EffortAbandoned) {
			row.Outcome = row.State
		}
		row.Discovery = row.Phase == "" && row.Outcome == ""
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.After(rows[j].CreatedAt)
		}
		return rows[i].EffortID < rows[j].EffortID
	})
	start := 0
	if cursor != "" {
		boundary, err := decodeEffortCursor(cursor)
		if err != nil {
			return EffortListPage{}, err
		}
		found := -1
		for i, row := range rows {
			if row.EffortID == boundary.EffortID && row.CreatedAt.Format(time.RFC3339Nano) == boundary.CreatedAt {
				found = i
				break
			}
		}
		if found < 0 {
			return EffortListPage{}, errors.New("cursor boundary is no longer resident")
		}
		start = found + 1
	}
	end := start + limit
	if end > len(rows) {
		end = len(rows)
	}
	page := EffortListPage{SchemaVersion: 1, Efforts: append([]EffortListRow(nil), rows[start:end]...)}
	if end < len(rows) {
		page.NextCursor = encodeEffortCursor(rows[end-1])
	}
	return page, nil
}

func currentListPhase(lifecycle LifecycleProjection) Phase {
	keys := make([]string, 0, len(lifecycle.OpenPhases))
	for key := range lifecycle.OpenPhases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) != 0 {
		return lifecycle.OpenPhases[keys[0]].Phase
	}
	return ""
}

func lastAppliedAt(read EffortRead) *time.Time {
	var latest *time.Time
	for _, record := range read.Records {
		if !record.Applied || record.Event == nil {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, record.Event.Timestamp)
		if err != nil {
			continue
		}
		at = at.UTC()
		if latest == nil || at.After(*latest) {
			copy := at
			latest = &copy
		}
	}
	return latest
}

func encodeEffortCursor(row EffortListRow) string {
	raw, _ := json.Marshal(effortCursor{Version: 1, CreatedAt: row.CreatedAt.Format(time.RFC3339Nano), EffortID: row.EffortID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeEffortCursor(value string) (effortCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return effortCursor{}, errors.New("invalid cursor")
	}
	var cursor effortCursor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || ensureJSONEOF(decoder) != nil || cursor.Version != 1 || cursor.EffortID == "" {
		return effortCursor{}, errors.New("invalid or unsupported cursor")
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt); err != nil || validatePathIdentifier("effortId", cursor.EffortID) != nil {
		return effortCursor{}, errors.New("invalid cursor")
	}
	canonical, _ := json.Marshal(cursor)
	if string(canonical) != string(raw) {
		return effortCursor{}, errors.New("invalid cursor")
	}
	return cursor, nil
}
