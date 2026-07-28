package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/telemetry"
)

var telemetryNow = func() time.Time { return time.Now().UTC() }

var metricsRead = func(c *cmdCtx) (telemetry.ReadSet, error) {
	return telemetry.Read(context.Background(), c.root)
}

var metricsAggregate = telemetry.Aggregate
var metricsExport = telemetry.Export

func runMetrics(c *cmdCtx) error {
	switch c.sub {
	case "":
		return runMetricsReport(c)
	case "doctor":
		return runMetricsDoctor(c)
	case "list":
		return runMetricsList(c)
	case "export":
		return runMetricsExport(c)
	default:
		return &usageErr{fmt.Sprintf("awf metrics: unknown subcommand %q", c.sub)}
	}
}
func parseTelemetrySelector(inv invocation) (telemetry.Selector, error) {
	s := telemetry.Selector{}
	for flag, target := range map[string]**string{"--effort": &s.EffortID, "--session": &s.SessionID} {
		if value, ok := inv.values[flag]; ok {
			v := value
			*target = &v
		}
	}
	for flag, target := range map[string]**time.Time{"--since": &s.Since, "--until": &s.Until} {
		if value, ok := inv.values[flag]; ok {
			v, err := telemetry.ParseSelectorTime(value)
			if err != nil {
				return s, &usageErr{fmt.Sprintf("%s: timestamp must be RFC3339: %v", flag, err)}
			}
			*target = &v
		}
	}
	if err := telemetry.ValidateSelector(s); err != nil {
		return s, &usageErr{err.Error()}
	}
	return s, nil
}
func runMetricsReport(c *cmdCtx) error {
	s, err := parseTelemetrySelector(c.inv)
	if err != nil {
		return err
	}
	reads, err := metricsRead(c)
	if err != nil {
		return boundedTelemetryError(err)
	}
	report, err := metricsAggregate(reads, s)
	if err != nil {
		return boundedTelemetryError(err)
	}
	if c.inv.bools["--json"] {
		return writeMetricsJSON(c.stdout, report)
	}
	return telemetry.RenderHuman(c.stdout, report)
}
func runMetricsDoctor(c *cmdCtx) error {
	s, err := parseTelemetrySelector(c.inv)
	if err != nil {
		return err
	}
	reads, err := metricsRead(c)
	if err != nil {
		return boundedTelemetryError(err)
	}
	findings := make([]telemetry.IntegrityFinding, 0, len(reads.Findings))
	for _, f := range reads.Findings {
		if s.SessionID != nil && f.SessionID != *s.SessionID {
			continue
		}
		findings = append(findings, f)
	}
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.SessionID != b.SessionID {
			return a.SessionID < b.SessionID
		}
		return a.Code < b.Code
	})
	result := telemetry.DoctorReport{SchemaVersion: telemetry.SchemaVersion, Selector: s, Findings: findings}
	if c.inv.bools["--json"] {
		return writeMetricsJSON(c.stdout, result)
	}
	return telemetry.RenderDoctorHuman(c.stdout, result)
}
func runMetricsList(c *cmdCtx) error {
	reads, err := metricsRead(c)
	if err != nil {
		return boundedTelemetryError(err)
	}
	type item struct {
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		State    string   `json:"state"`
		Sessions []string `json:"sessions"`
	}
	out := make([]item, 0, len(reads.Records))
	for _, r := range reads.Records {
		out = append(out, item{r.ID, r.Title, string(r.State), r.AssignedSessionIDs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if c.inv.bools["--json"] {
		return writeMetricsJSON(c.stdout, struct {
			SchemaVersion int    `json:"schemaVersion"`
			Efforts       []item `json:"efforts"`
		}{telemetry.SchemaVersion, out})
	}
	for _, r := range out {
		if _, err := fmt.Fprintf(c.stdout, "effort %s title=%q state=%s sessions=%s\n", r.ID, r.Title, r.State, strings.Join(r.Sessions, ",")); err != nil {
			return err
		}
	}
	return nil
}
func runMetricsExport(c *cmdCtx) error {
	format := c.inv.values["--format"]
	if format != "json" && format != "jsonl" {
		return &usageErr{"usage: awf metrics export [selectors] --format <json|jsonl>"}
	}
	s, err := parseTelemetrySelector(c.inv)
	if err != nil {
		return err
	}
	reads, err := metricsRead(c)
	if err != nil {
		return boundedTelemetryError(err)
	}
	if format == "json" {
		report, err := metricsAggregate(reads, s)
		if err != nil {
			return err
		}
		return writeMetricsJSON(c.stdout, report)
	}
	records, err := metricsExport(reads, s)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	for _, record := range records {
		b.Write(record)
		b.WriteByte('\n')
	}
	_, err = io.Copy(c.stdout, &b)
	return err
}

type telemetryCommandError struct {
	message string
	cause   error
}

func (e *telemetryCommandError) Error() string { return e.message }
func (e *telemetryCommandError) Unwrap() error { return e.cause }
func boundedTelemetryError(err error) error {
	message := strings.ReplaceAll(err.Error(), "\n", " ")
	const maximum = 512
	if len(message) > maximum {
		message = message[:maximum-3] + "..."
	}
	return &telemetryCommandError{message: message, cause: err}
}
func writeMetricsJSON(out io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = out.Write(append(raw, '\n'))
	return err
}
