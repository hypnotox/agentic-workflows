// Package testperformance owns qualification records for verification-performance evidence.
package testperformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Version is the supported qualification-record schema version.
const Version = 1

// Record is the versioned, canonical qualification evidence for test performance.
type Record struct {
	Version      int           `json:"version"`
	Workloads    []Workload    `json:"workloads"`
	Environments []Environment `json:"environments"`
	SampleMethod SampleMethod  `json:"sample_method"`
	Baselines    []Baseline    `json:"baselines"`
	Budgets      []Budget      `json:"budgets"`
	Observations []Observation `json:"observations"`
}

// Workload identifies one independently qualified verification workload.
type Workload struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Mutation    string `json:"mutation"`
	Description string `json:"description"`
}

// Environment identifies one local or hosted measurement machine completely.
type Environment struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	CPU          string `json:"cpu"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Filesystem   string `json:"filesystem"`
	Memory       string `json:"memory"`
	RunnerImage  string `json:"runner_image"`
	Toolchain    string `json:"toolchain"`
}

// SampleMethod describes reproducible cache and sampling treatment.
type SampleMethod struct {
	CachePreparation []string `json:"cache_preparation"`
	WarmSamples      int      `json:"warm_samples"`
	ColdSamples      int      `json:"cold_samples"`
	Aggregation      string   `json:"aggregation"`
	WallTime         string   `json:"wall_time"`
}

// Baseline records landed evidence for one like-for-like workload.
type Baseline struct {
	Workload    string      `json:"workload"`
	Environment string      `json:"environment"`
	Seconds     float64     `json:"seconds"`
	Components  []Component `json:"components"`
}

// Budget records threshold and component targets for one workload.
type Budget struct {
	Workload              string      `json:"workload"`
	Environment           string      `json:"environment"`
	Qualification         string      `json:"qualification"`
	MaximumSeconds        float64     `json:"maximum_seconds"`
	StrongerTargetSeconds float64     `json:"stronger_target_seconds"`
	ComponentMaximums     []Component `json:"component_maximums"`
}

// Component keeps the stage, package, and test timing identity intact.
type Component struct {
	Stage   string  `json:"stage"`
	Package string  `json:"package"`
	Test    string  `json:"test"`
	Seconds float64 `json:"seconds"`
}

// Observation records one measured sample and its component evidence.
type Observation struct {
	Workload    string      `json:"workload"`
	Environment Environment `json:"environment"`
	Cache       string      `json:"cache"`
	Sample      int         `json:"sample"`
	Seconds     float64     `json:"seconds"`
	Components  []Component `json:"components"`
}

// Aggregate summarizes same-workload, same-environment observations.
type Aggregate struct {
	Workload    string      `json:"workload"`
	Environment string      `json:"environment"`
	Cache       string      `json:"cache"`
	Samples     int         `json:"samples"`
	Seconds     float64     `json:"seconds"`
	Components  []Component `json:"components"`
}

// Evaluation classifies aggregate evidence against a declared budget.
type Evaluation struct {
	Workload             string   `json:"workload"`
	Environment          string   `json:"environment"`
	WallTime             string   `json:"wall_time"`
	MaximumSeconds       float64  `json:"maximum_seconds"`
	ObservedSeconds      float64  `json:"observed_seconds"`
	MeetsMaximum         bool     `json:"meets_maximum"`
	ComponentRegressions []string `json:"component_regressions,omitempty"`
}

// Report is the shared input for human and machine renderings.
type Report struct {
	RecordVersion int           `json:"record_version"`
	Workloads     []Workload    `json:"workloads"`
	Environments  []Environment `json:"environments"`
	SampleMethod  SampleMethod  `json:"sample_method"`
	Baselines     []Baseline    `json:"baselines"`
	Budgets       []Budget      `json:"budgets"`
	Observations  []Observation `json:"observations"`
	Aggregates    []Aggregate   `json:"aggregates"`
	Evaluations   []Evaluation  `json:"evaluations"`
}

// Load reads and strictly validates one qualification record.
func Load(path string) (Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("read qualification record: %w", err)
	}
	return Parse(data)
}

// Parse strictly decodes and validates canonical-record data.
func Parse(data []byte) (Record, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return Record{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("decode qualification record: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Record{}, errors.New("decode qualification record: multiple JSON values")
	}
	if err := Validate(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// Canonical renders a validated record in its stable repository representation.
func Canonical(record Record) ([]byte, error) {
	if err := Validate(record); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

type workloadContract struct {
	kind     string
	mutation string
}

var requiredWorkloads = map[string]workloadContract{
	"fast-gate":            {kind: "component", mutation: "excluded"},
	"common-feedback":      {kind: "common-local", mutation: "excluded"},
	"ordinary-full":        {kind: "terminal-local", mutation: "empty-universe"},
	"selected-mutation":    {kind: "exceptional-local", mutation: "range-selected"},
	"hosted-critical-path": {kind: "hosted", mutation: "separately-timed"},
}

// Validate checks the complete qualification contract without comparing unlike environments.
func Validate(r Record) error {
	if r.Version != Version {
		return fmt.Errorf("version must be %d", Version)
	}
	if len(r.Workloads) == 0 || len(r.Environments) == 0 || len(r.Baselines) == 0 || len(r.Budgets) == 0 {
		return errors.New("workloads, environments, baselines, and budgets are required")
	}
	if len(r.Workloads) != len(requiredWorkloads) {
		return fmt.Errorf("workloads must contain exactly the schema-v%d required set", Version)
	}
	workloads := map[string]bool{}
	for _, w := range r.Workloads {
		contract, required := requiredWorkloads[w.ID]
		if !required || w.Description == "" {
			return fmt.Errorf("workload %q is not a complete schema-v%d workload", w.ID, Version)
		}
		if w.Kind != contract.kind || w.Mutation != contract.mutation {
			return fmt.Errorf("workload %q classification must be kind %q and mutation %q", w.ID, contract.kind, contract.mutation)
		}
		if workloads[w.ID] {
			return fmt.Errorf("duplicate workload %q", w.ID)
		}
		workloads[w.ID] = true
	}
	for id := range requiredWorkloads {
		if !workloads[id] {
			return fmt.Errorf("required workload %q is missing", id)
		}
	}
	environments := map[string]Environment{}
	for _, e := range r.Environments {
		if e.ID == "" || (e.Kind != "local" && e.Kind != "hosted") || e.CPU == "" || e.OS == "" || e.Architecture == "" || e.Filesystem == "" || e.Memory == "" || e.RunnerImage == "" || e.Toolchain == "" {
			return fmt.Errorf("environment %q requires full identity", e.ID)
		}
		if _, ok := environments[e.ID]; ok {
			return fmt.Errorf("duplicate environment %q", e.ID)
		}
		environments[e.ID] = e
	}
	if len(r.SampleMethod.CachePreparation) == 0 || r.SampleMethod.WarmSamples < 1 || r.SampleMethod.ColdSamples < 1 || r.SampleMethod.Aggregation != "median" || r.SampleMethod.WallTime != "evidence-not-correctness" {
		return errors.New("sample_method requires cache preparation, positive warm and cold samples, median aggregation, and evidence-not-correctness wall time")
	}
	baselines := map[string]Baseline{}
	for _, b := range r.Baselines {
		if err := identity(b.Workload, b.Environment, workloads, environments); err != nil {
			return err
		}
		if b.Seconds < 0 {
			return errors.New("baseline seconds must not be negative")
		}
		if err := components(b.Components); err != nil {
			return err
		}
		key := b.Workload + "\x00" + b.Environment
		if _, ok := baselines[key]; ok {
			return fmt.Errorf("duplicate baseline %q", key)
		}
		baselines[key] = b
	}
	budgets := map[string]bool{}
	for _, b := range r.Budgets {
		if err := identity(b.Workload, b.Environment, workloads, environments); err != nil {
			return err
		}
		key := b.Workload + "\x00" + b.Environment
		if budgets[key] {
			return fmt.Errorf("duplicate budget %q", key)
		}
		switch b.Qualification {
		case "minimum":
			if _, ok := baselines[key]; !ok {
				return fmt.Errorf("minimum budget %q has no like-for-like baseline", key)
			}
			if b.MaximumSeconds <= 0 || b.StrongerTargetSeconds <= 0 || b.StrongerTargetSeconds > b.MaximumSeconds {
				return fmt.Errorf("budget %q has invalid targets", key)
			}
		case "target":
			if b.MaximumSeconds <= 0 || b.StrongerTargetSeconds <= 0 || b.StrongerTargetSeconds > b.MaximumSeconds {
				return fmt.Errorf("budget %q has invalid targets", key)
			}
		case "unqualified":
			if b.MaximumSeconds != 0 || b.StrongerTargetSeconds != 0 || len(b.ComponentMaximums) != 0 {
				return fmt.Errorf("unqualified budget %q must not invent a threshold", key)
			}
		default:
			return fmt.Errorf("budget %q has invalid qualification %q", key, b.Qualification)
		}
		if err := components(b.ComponentMaximums); err != nil {
			return err
		}
		budgets[key] = true
	}
	observationSamples := map[string]bool{}
	for _, o := range r.Observations {
		reference, ok := environments[o.Environment.ID]
		if !ok {
			return fmt.Errorf("unknown environment %q", o.Environment.ID)
		}
		if err := identity(o.Workload, o.Environment.ID, workloads, environments); err != nil {
			return err
		}
		if err := MatchEnvironment(reference, o.Environment); err != nil {
			return err
		}
		if o.Cache != "warm" && o.Cache != "cold" {
			return fmt.Errorf("observation cache %q is invalid", o.Cache)
		}
		if o.Sample < 1 || o.Seconds < 0 {
			return errors.New("observation requires positive sample and non-negative seconds")
		}
		sampleKey := fmt.Sprintf("%s\x00%s\x00%s\x00%d", o.Workload, o.Environment.ID, o.Cache, o.Sample)
		if observationSamples[sampleKey] {
			return fmt.Errorf("duplicate observation sample %q", sampleKey)
		}
		observationSamples[sampleKey] = true
		if err := components(o.Components); err != nil {
			return err
		}
	}
	return nil
}
func identity(w, e string, ws map[string]bool, es map[string]Environment) error {
	if !ws[w] {
		return fmt.Errorf("unknown workload %q", w)
	}
	if _, ok := es[e]; !ok {
		return fmt.Errorf("unknown environment %q", e)
	}
	return nil
}
func components(cs []Component) error {
	seen := map[string]bool{}
	for _, c := range cs {
		if c.Stage == "" || c.Package == "" || c.Test == "" || c.Seconds < 0 {
			return errors.New("component requires stage, package, test, and non-negative seconds")
		}
		key := componentName(c)
		if seen[key] {
			return fmt.Errorf("duplicate component %q", key)
		}
		seen[key] = true
	}
	return nil
}

// Aggregates produces stable median timing evidence while retaining every component identity.
func Aggregates(r Record) []Aggregate {
	groups := map[string][]Observation{}
	for _, o := range r.Observations {
		key := o.Workload + "\x00" + o.Environment.ID + "\x00" + o.Cache
		groups[key] = append(groups[key], o)
	}
	keys := sortedKeys(groups)
	out := make([]Aggregate, 0, len(keys))
	for _, key := range keys {
		os := groups[key]
		sort.Slice(os, func(i, j int) bool { return os[i].Seconds < os[j].Seconds })
		parts := strings.Split(key, "\x00")
		a := Aggregate{Workload: parts[0], Environment: parts[1], Cache: parts[2], Samples: len(os), Seconds: median(os)}
		for _, o := range os {
			a.Components = append(a.Components, o.Components...)
		}
		out = append(out, a)
	}
	return out
}
func median(os []Observation) float64 {
	n := len(os)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return os[n/2].Seconds
	}
	return (os[n/2-1].Seconds + os[n/2].Seconds) / 2
}
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Evaluate deterministically classifies timing evidence and component budget regressions.
func Evaluate(r Record, aggregates []Aggregate) []Evaluation {
	budgets := map[string]Budget{}
	for _, b := range r.Budgets {
		budgets[b.Workload+"\x00"+b.Environment] = b
	}
	out := []Evaluation{}
	for _, a := range aggregates {
		b, ok := budgets[a.Workload+"\x00"+a.Environment]
		if !ok || b.Qualification == "unqualified" {
			continue
		}
		e := Evaluation{Workload: a.Workload, Environment: a.Environment, WallTime: r.SampleMethod.WallTime, MaximumSeconds: b.MaximumSeconds, ObservedSeconds: a.Seconds, MeetsMaximum: a.Seconds <= b.MaximumSeconds}
		for _, limit := range b.ComponentMaximums {
			var samples []float64
			for _, seen := range a.Components {
				if sameComponent(limit, seen) {
					samples = append(samples, seen.Seconds)
				}
			}
			if len(samples) != a.Samples {
				e.ComponentRegressions = append(e.ComponentRegressions, componentName(limit)+" (missing)")
			} else if medianValues(samples) > limit.Seconds {
				e.ComponentRegressions = append(e.ComponentRegressions, componentName(limit))
			}
		}
		sort.Strings(e.ComponentRegressions)
		out = append(out, e)
	}
	return out
}
func sameComponent(a, b Component) bool {
	return a.Stage == b.Stage && a.Package == b.Package && a.Test == b.Test
}
func componentName(c Component) string { return c.Stage + "/" + c.Package + "/" + c.Test }

func medianValues(values []float64) float64 {
	sort.Float64s(values)
	n := len(values)
	if n%2 == 1 {
		return values[n/2]
	}
	return (values[n/2-1] + values[n/2]) / 2
}

// HasComponentRegressions reports whether deterministic qualification failed.
func HasComponentRegressions(report Report) bool {
	for _, evaluation := range report.Evaluations {
		if len(evaluation.ComponentRegressions) != 0 {
			return true
		}
	}
	return false
}

// BuildReport creates human and machine output from one observation set.
func BuildReport(r Record) Report {
	a := Aggregates(r)
	return Report{
		RecordVersion: r.Version,
		Workloads:     r.Workloads,
		Environments:  r.Environments,
		SampleMethod:  r.SampleMethod,
		Baselines:     r.Baselines,
		Budgets:       r.Budgets,
		Observations:  r.Observations,
		Aggregates:    a,
		Evaluations:   Evaluate(r, a),
	}
}

// WriteHuman renders a report without changing its source record.
func WriteHuman(w io.Writer, report Report) {
	fmt.Fprintf(w, "test-performance qualification record v%d\n", report.RecordVersion)
	for _, workload := range report.Workloads {
		fmt.Fprintf(w, "workload %s: kind=%s mutation=%s\n", workload.ID, workload.Kind, workload.Mutation)
	}
	for _, environment := range report.Environments {
		fmt.Fprintf(w, "environment %s: kind=%s cpu=%s os=%s architecture=%s filesystem=%s memory=%s runner=%s toolchain=%s\n", environment.ID, environment.Kind, environment.CPU, environment.OS, environment.Architecture, environment.Filesystem, environment.Memory, environment.RunnerImage, environment.Toolchain)
	}
	fmt.Fprintf(w, "sample method: aggregation=%s warm=%d cold=%d wall=%s\n", report.SampleMethod.Aggregation, report.SampleMethod.WarmSamples, report.SampleMethod.ColdSamples, report.SampleMethod.WallTime)
	for _, preparation := range report.SampleMethod.CachePreparation {
		fmt.Fprintf(w, "  cache preparation: %s\n", preparation)
	}
	for _, baseline := range report.Baselines {
		fmt.Fprintf(w, "baseline %s on %s: %.3fs\n", baseline.Workload, baseline.Environment, baseline.Seconds)
	}
	for _, budget := range report.Budgets {
		fmt.Fprintf(w, "budget %s on %s: qualification=%s maximum=%.3fs stronger=%.3fs\n", budget.Workload, budget.Environment, budget.Qualification, budget.MaximumSeconds, budget.StrongerTargetSeconds)
	}
	for _, observation := range report.Observations {
		fmt.Fprintf(w, "observation %s on %s: cache=%s sample=%d seconds=%.3f\n", observation.Workload, observation.Environment.ID, observation.Cache, observation.Sample, observation.Seconds)
	}
	for _, aggregate := range report.Aggregates {
		fmt.Fprintf(w, "aggregate %s on %s: cache=%s samples=%d seconds=%.3f\n", aggregate.Workload, aggregate.Environment, aggregate.Cache, aggregate.Samples, aggregate.Seconds)
	}
	for _, e := range report.Evaluations {
		fmt.Fprintf(w, "%s on %s: %.3fs (budget %.3fs, meets=%t, evidence: %s)\n", e.Workload, e.Environment, e.ObservedSeconds, e.MaximumSeconds, e.MeetsMaximum, e.WallTime)
		for _, c := range e.ComponentRegressions {
			fmt.Fprintf(w, "  component regression: %s\n", c)
		}
	}
}
func rejectDuplicateKeys(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	return scanValue(d)
}
func scanValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("decode qualification record: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s := key.(string)
			if seen[s] {
				return fmt.Errorf("decode qualification record: duplicate key %q", s)
			}
			seen[s] = true
			if err := scanValue(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	case '[':
		for d.More() {
			if err := scanValue(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return nil
}

// MatchEnvironment refuses comparison unless every identity field matches.
func MatchEnvironment(reference, candidate Environment) error {
	if reference != candidate {
		return fmt.Errorf("environment mismatch: %s and %s are not like-for-like", reference.ID, candidate.ID)
	}
	return nil
}
