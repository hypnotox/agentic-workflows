package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/telemetry"
)

func runMetrics(c *cmdCtx) error {
	switch c.sub {
	case "":
		return runMetricsQuery(c)
	case "export":
		return runMetricsExport(c)
	case "protocol":
		if !c.inv.bools["--json"] {
			return &usageErr{"usage: awf metrics protocol --json"}
		}
		result := struct {
			SchemaVersion    int                       `json:"schemaVersion"`
			Protocol         telemetry.ProtocolVersion `json:"protocol"`
			CompatibleMajor  uint16                    `json:"compatibleMajor"`
			DescriptorSHA256 string                    `json:"descriptorSha256"`
			AWFVersion       string                    `json:"awfVersion"`
			ProjectVersion   string                    `json:"projectVersion"`
		}{1, telemetry.ProtocolVersion{Major: 1, Minor: 0}, 1, telemetry.DescriptorSHA256(), awfVersion(), project.Version}
		return writeMetricsJSON(c.stdout, result)
	case "lifecycle":
		return runMetricsLifecycle(c)
	case "retain":
		return runMetricsRetain(c)
	case "purge":
		return runMetricsPurge(c)
	default:
		return &usageErr{fmt.Sprintf("awf metrics: unknown subcommand %q", c.sub)}
	}
}

var telemetryNow = func() time.Time { return time.Now().UTC() }
var telemetryStorageLstat = os.Lstat

func parseTelemetrySelector(inv invocation) (telemetry.Selector, error) {
	selector := telemetry.Selector{}
	for flag, destination := range map[string]**string{
		"--effort":  &selector.EffortID,
		"--session": &selector.SessionID,
		"--phase":   &selector.Phase,
	} {
		if value, present := inv.values[flag]; present {
			copy := value
			*destination = &copy
		}
	}
	for flag, destination := range map[string]**time.Time{"--since": &selector.Since, "--until": &selector.Until} {
		if value, present := inv.values[flag]; present {
			parsed, err := telemetry.ParseSelectorTime(value)
			if err != nil {
				return telemetry.Selector{}, &usageErr{fmt.Sprintf("%s: %v", flag, err)}
			}
			*destination = &parsed
		}
	}
	if err := telemetry.ValidateSelector(selector); err != nil {
		return telemetry.Selector{}, &usageErr{err.Error()}
	}
	return selector, nil
}

func runMetricsQuery(c *cmdCtx) error {
	selector, err := parseTelemetrySelector(c.inv)
	if err != nil {
		return err
	}
	reads, cfg, err := readTelemetryQueryInputs(c.root)
	if err != nil {
		return boundedTelemetryError(c.root, err)
	}
	result, err := telemetry.AggregateMetrics(reads, selector, telemetry.MetricsOptions{
		GeneratedAt: telemetryNow(),
		Retention: telemetry.RetentionPolicy{
			MaxCompletedEffortAgeDays: cfg.WorkflowTelemetry.Retention.MaxCompletedEffortAgeDays,
			MaxCompletedEffortCount:   cfg.WorkflowTelemetry.Retention.MaxCompletedEffortCount,
		},
	})
	if err != nil { // coverage-ignore: parsing validated the selector used by aggregation
		return boundedTelemetryError(c.root, err)
	}
	if c.inv.bools["--json"] {
		return writeMetricsJSON(c.stdout, result)
	}
	return telemetry.RenderMetricsHuman(c.stdout, result)
}

func runMetricsExport(c *cmdCtx) error {
	format := c.inv.values["--format"]
	if format != "json" && format != "jsonl" {
		return &usageErr{"usage: awf metrics export [selectors] --format <json|jsonl>"}
	}
	selector, err := parseTelemetrySelector(c.inv)
	if err != nil {
		return err
	}
	reads, cfg, err := readTelemetryQueryInputs(c.root)
	if err != nil {
		return boundedTelemetryError(c.root, err)
	}
	if format == "json" {
		result, aggregateErr := telemetry.AggregateMetrics(reads, selector, telemetry.MetricsOptions{
			GeneratedAt: telemetryNow(),
			Retention: telemetry.RetentionPolicy{
				MaxCompletedEffortAgeDays: cfg.WorkflowTelemetry.Retention.MaxCompletedEffortAgeDays,
				MaxCompletedEffortCount:   cfg.WorkflowTelemetry.Retention.MaxCompletedEffortCount,
			},
		})
		if aggregateErr != nil { // coverage-ignore: parsing validated the selector used by aggregation
			return boundedTelemetryError(c.root, aggregateErr)
		}
		return writeMetricsJSON(c.stdout, result)
	}
	events, err := telemetry.SelectNormalizedEvents(reads, selector)
	if err != nil {
		return boundedTelemetryError(c.root, err)
	}
	var output bytes.Buffer
	for _, event := range events {
		output.Write(event)
		output.WriteByte('\n')
	}
	_, err = io.Copy(c.stdout, &output)
	return err
}

func readTelemetryQueryInputs(root string) ([]telemetry.EffortRead, *config.Config, error) {
	cfg, err := config.Load(filepath.Join(root, config.DirName))
	if err != nil {
		return nil, nil, err
	}
	metricsPath := filepath.Join(root, config.DirName, "metrics")
	if _, err := telemetryStorageLstat(metricsPath); errors.Is(err, os.ErrNotExist) {
		return []telemetry.EffortRead{}, cfg, nil
	} else if err != nil {
		return nil, nil, fmt.Errorf("inspect telemetry storage: %w", err)
	}
	ledger, err := telemetry.OpenLedger(root)
	if err != nil {
		return nil, nil, err
	}
	reads, err := ledger.ReadAllEfforts()
	if err != nil {
		return nil, nil, err
	}
	return reads, cfg, nil
}

type telemetryCommandError struct {
	message string
	cause   error
}

func (e *telemetryCommandError) Error() string { return e.message }
func (e *telemetryCommandError) Unwrap() error { return e.cause }

func boundedTelemetryError(root string, err error) error {
	message := strings.ReplaceAll(err.Error(), "\n", " ")
	if absolute, absoluteErr := filepath.Abs(root); absoluteErr == nil && absolute != string(filepath.Separator) {
		message = strings.ReplaceAll(message, filepath.Clean(absolute), "<project>")
	}
	const maximum = 512
	if len(message) > maximum {
		message = message[:maximum-3] + "..."
	}
	return &telemetryCommandError{message: message, cause: err}
}

func runMetricsLifecycle(c *cmdCtx) error {
	requestPath := c.inv.values["--request"]
	if requestPath == "" {
		return &usageErr{"usage: awf metrics lifecycle --request <FILE|-> [--json]"}
	}
	var raw []byte
	var err error
	if requestPath == "-" {
		raw, err = io.ReadAll(c.stdin)
	} else {
		raw, err = os.ReadFile(requestPath)
	}
	if err != nil {
		return fmt.Errorf("read lifecycle request: %w", err)
	}
	request, err := telemetry.DecodeLifecycleRequest(raw)
	if err != nil {
		return fmt.Errorf("decode lifecycle request: %w", err)
	}
	ledger, err := telemetry.NewLedger(c.root)
	if err != nil {
		return err
	}
	result, err := ledger.ApplyLifecycle(context.Background(), request)
	if err != nil {
		return err
	}
	if c.inv.bools["--json"] {
		output := struct {
			SchemaVersion int    `json:"schemaVersion"`
			EventID       string `json:"eventId"`
			EffortID      string `json:"effortId"`
			SessionID     string `json:"sessionId"`
			TrajectoryID  string `json:"trajectoryId,omitempty"`
			Idempotent    bool   `json:"idempotent"`
		}{1, result.Event.EventID, result.Event.EffortID, result.Event.SessionID, result.Event.TrajectoryID, result.Idempotent}
		return writeMetricsJSON(c.stdout, output)
	}
	fmt.Fprintf(c.stdout, "recorded %s for effort %s session %s", result.Event.EventID, result.Event.EffortID, result.Event.SessionID)
	if result.Event.TrajectoryID != "" {
		fmt.Fprintf(c.stdout, " trajectory %s", result.Event.TrajectoryID)
	}
	fmt.Fprintln(c.stdout)
	return nil
}

type metricsMaintenanceResult struct {
	SchemaVersion int      `json:"schemaVersion"`
	DryRun        bool     `json:"dryRun"`
	Candidates    []string `json:"candidates"`
	Pruned        []string `json:"pruned"`
	Recovered     []string `json:"recovered"`
}

func runMetricsRetain(c *cmdCtx) error {
	ledger, recovered, err := telemetryLedgerWithRecovery(c.root)
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(c.root, config.DirName))
	if err != nil {
		return err
	}
	dryRun := c.inv.bools["--dry-run"]
	retained, err := ledger.Retain(context.Background(), telemetry.RetentionPolicy{
		MaxCompletedEffortAgeDays: cfg.WorkflowTelemetry.Retention.MaxCompletedEffortAgeDays,
		MaxCompletedEffortCount:   cfg.WorkflowTelemetry.Retention.MaxCompletedEffortCount,
	}, dryRun)
	if err != nil {
		return err
	}
	result := metricsMaintenanceResult{1, dryRun, retained.Candidates, retained.Pruned, recovered}
	if c.inv.bools["--json"] {
		return writeMetricsJSON(c.stdout, result)
	}
	fmt.Fprintf(c.stdout, "retention candidates %d, pruned %d, recovered %d\n", len(result.Candidates), len(result.Pruned), len(result.Recovered))
	return nil
}

func runMetricsPurge(c *cmdCtx) error {
	effortID := c.inv.values["--effort"]
	if effortID == "" || !c.inv.bools["--confirm"] {
		return &usageErr{"usage: awf metrics purge --effort <ID> --confirm [--json]"}
	}
	ledger, recovered, err := telemetryLedgerWithRecovery(c.root)
	if err != nil {
		return err
	}
	purged, err := ledger.Purge(context.Background(), effortID, true)
	if err != nil {
		return err
	}
	result := metricsMaintenanceResult{1, false, purged.Candidates, purged.Pruned, recovered}
	if c.inv.bools["--json"] {
		return writeMetricsJSON(c.stdout, result)
	}
	fmt.Fprintf(c.stdout, "purged effort %s\n", effortID)
	return nil
}

var recoverTelemetryLedger = func(ledger *telemetry.Ledger) (telemetry.RecoveryReport, error) {
	return ledger.Recover()
}

func telemetryLedgerWithRecovery(root string) (*telemetry.Ledger, []string, error) {
	ledger, err := telemetry.NewLedger(root)
	if err != nil {
		return nil, nil, err
	}
	recovery, err := recoverTelemetryLedger(ledger)
	if err != nil {
		return nil, nil, err
	}
	if len(recovery.Ambiguous) > 0 {
		return nil, nil, errors.New("telemetry recovery found ambiguous resident state")
	}
	sort.Strings(recovery.Recovered)
	return ledger, recovery.Recovered, nil
}

func writeMetricsJSON(out io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil { // coverage-ignore: fixed telemetry result structs contain only JSON-safe values
		return err
	}
	encoded = append(encoded, '\n')
	_, err = out.Write(encoded)
	return err
}
