package migrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"gopkg.in/yaml.v3"
)

const pitfallCorpusGeneration = 43

type legacyPitfallSidecar struct {
	Data struct {
		Pitfalls []legacyPitfallEntry `yaml:"pitfalls"`
	} `yaml:"data"`
}
type legacyPitfallEntry struct {
	Title   string   `yaml:"title"`
	Domains []string `yaml:"domains"`
	Tags    []string `yaml:"tags"`
	Related []int    `yaml:"related"`
	Body    string   `yaml:"body"`
}
type plannedPitfallLeaf struct {
	entry pitfall.Entry
	bytes []byte
	path  string
}

type pitfallCorpusOperation struct {
	create        func(string, []byte) error
	writeSidecar  func(string, []byte) error
	removeSidecar func(string) error
}

func productionPitfallCorpusOperation() pitfallCorpusOperation {
	return pitfallCorpusOperation{
		create: func(path string, data []byte) error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				return err
			}
			if _, err = f.Write(data); err != nil { // coverage-ignore: a newly opened regular file cannot deterministically fail a complete small write in the test environment
				_ = f.Close()
				return err
			}
			return f.Close()
		},
		writeSidecar:  func(path string, data []byte) error { return os.WriteFile(path, data, 0o644) },
		removeSidecar: os.Remove,
	}
}

func applyPitfallCorpus(root string, out *Changes) error {
	return applyPitfallCorpusWith(root, out, productionPitfallCorpusOperation())
}

func applyPitfallCorpusWith(root string, out *Changes, operation pitfallCorpusOperation) error {
	sidecarPath := filepath.Join(config.RootDir(root), "docs", "pitfalls.yaml")
	raw, err := os.ReadFile(sidecarPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var sidecar legacyPitfallSidecar
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(false)
	if err := dec.Decode(&sidecar); err != nil {
		return fmt.Errorf("pitfall-corpus: parse %s: %w", sidecarPath, err)
	}
	if len(sidecar.Data.Pitfalls) == 0 {
		return nil
	}

	used := map[string]bool{}
	plans := make([]plannedPitfallLeaf, 0, len(sidecar.Data.Pitfalls))
	for i, old := range sidecar.Data.Pitfalls {
		title := strings.TrimSpace(old.Title)
		slug, err := pitfall.AllocateSlug(title, used)
		if err != nil {
			return fmt.Errorf("pitfall-corpus: entry %d %q: %w", i, title, err)
		}
		used[slug] = true
		e := pitfall.Entry{Slug: slug, SourcePath: pitfall.SourceDir + "/" + slug + ".md", Title: title, Domains: old.Domains, Tags: old.Tags, Related: old.Related, Body: strings.TrimRight(old.Body, "\n")}
		serialized, err := pitfall.Serialize(e)
		if err != nil {
			return fmt.Errorf("pitfall-corpus: entry %d %q: %w", i, title, err)
		}
		e.Source = serialized
		for _, link := range pitfall.RelativeLinks(e) {
			return fmt.Errorf("pitfall-corpus: entry %q contains relative link %q; replace it with a repository-root absolute or external target and retry awf upgrade", title, link.Destination)
		}
		plans = append(plans, plannedPitfallLeaf{entry: e, bytes: serialized, path: filepath.Join(root, filepath.FromSlash(e.SourcePath))})
	}
	files := make([]pitfall.SourceFile, 0, len(plans))
	for _, plan := range plans {
		files = append(files, pitfall.SourceFile{Path: plan.entry.SourcePath, Bytes: plan.bytes, Regular: true})
	}
	if _, err := pitfall.Load(files); err != nil {
		return fmt.Errorf("pitfall-corpus: preflight: %w", err)
	}

	remainder, err := config.RemoveMappingKey(raw, "data", "pitfalls")
	if err != nil { // coverage-ignore: the same bytes were decoded as a mapping immediately above
		return fmt.Errorf("pitfall-corpus: preflight sidecar remainder: %w", err)
	}
	for _, plan := range plans {
		existing, err := os.ReadFile(plan.path)
		if err == nil && !bytes.Equal(existing, plan.bytes) {
			return fmt.Errorf("pitfall-corpus: destination %s conflicts with migrated entry %q", plan.entry.SourcePath, plan.entry.Title)
		}
		if err != nil && !os.IsNotExist(err) { // coverage-ignore: requires a filesystem permission or IO fault after successful sidecar read
			return fmt.Errorf("pitfall-corpus: inspect %s: %w", plan.entry.SourcePath, err)
		}
	}
	for _, plan := range plans {
		if existing, err := os.ReadFile(plan.path); err == nil && bytes.Equal(existing, plan.bytes) {
			continue
		}
		if err := operation.create(plan.path, plan.bytes); err != nil {
			return fmt.Errorf("pitfall-corpus: create %s: %w", plan.entry.SourcePath, err)
		}
		out.Add("pitfall-corpus: created " + plan.entry.SourcePath)
	}
	emptyRemainder, err := pitfallSidecarRemainderEmpty(remainder)
	if err != nil { // coverage-ignore: RemoveMappingKey already produced valid YAML from decoded input
		return fmt.Errorf("pitfall-corpus: validate sidecar remainder: %w", err)
	}
	if emptyRemainder {
		if err := operation.removeSidecar(sidecarPath); err != nil { // coverage-ignore: injected-operation failures are covered at the create boundary; production removal failure requires an IO race
			return fmt.Errorf("pitfall-corpus: retire sidecar: %w", err)
		}
		out.Add("pitfall-corpus: removed empty .awf/docs/pitfalls.yaml")
	} else {
		if err := operation.writeSidecar(sidecarPath, remainder); err != nil { // coverage-ignore: production write failure requires an IO fault after successful preflight
			return fmt.Errorf("pitfall-corpus: retain sections-only sidecar: %w", err)
		}
		out.Add("pitfall-corpus: retained sections-only .awf/docs/pitfalls.yaml")
	}
	return nil
}

func pitfallSidecarRemainderEmpty(raw []byte) (bool, error) {
	var value map[string]any
	if err := yaml.Unmarshal(raw, &value); err != nil {
		return false, err
	}
	if data, ok := value["data"].(map[string]any); ok && len(data) == 0 { // coverage-ignore: config.RemoveMappingKey currently removes an emptied data mapping entirely
		delete(value, "data")
	}
	return len(value) == 0, nil
}
