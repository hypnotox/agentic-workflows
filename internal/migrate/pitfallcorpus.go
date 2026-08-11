package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
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
	Title   string
	Domains []string
	Tags    []string
	Related []int
	Body    string
}

func (e *legacyPitfallEntry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("legacy pitfall entry must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("duplicate legacy pitfall key %q", key)
		}
		seen[key] = true
		switch key {
		case "title", "body":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return fmt.Errorf("legacy pitfall %s must be a string", key)
			}
			if key == "title" {
				e.Title = strings.TrimSpace(value.Value)
			} else {
				e.Body = strings.TrimRight(value.Value, "\n")
			}
		case "domains", "tags":
			values, err := decodeLegacyStrings(key, value)
			if err != nil {
				return err
			}
			if key == "domains" {
				e.Domains = values
			} else {
				e.Tags = values
			}
		case "related":
			values, err := decodeLegacyInts(key, value)
			if err != nil {
				return err
			}
			e.Related = values
		}
	}
	return nil
}

func decodeLegacyStrings(field string, node *yaml.Node) ([]string, error) {
	if node.Tag == "!!null" {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("legacy pitfall %s must be a list", field)
	}
	values := make([]string, 0, len(node.Content))
	for _, member := range node.Content {
		if member.Kind != yaml.ScalarNode || member.Tag != "!!str" || strings.TrimSpace(member.Value) == "" {
			return nil, fmt.Errorf("legacy pitfall %s entries must be non-empty strings", field)
		}
		values = append(values, strings.TrimSpace(member.Value))
	}
	return values, nil
}

func decodeLegacyInts(field string, node *yaml.Node) ([]int, error) {
	if node.Tag == "!!null" {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("legacy pitfall %s must be a list", field)
	}
	values := make([]int, 0, len(node.Content))
	for _, member := range node.Content {
		if member.Kind != yaml.ScalarNode || member.Tag != "!!int" {
			return nil, fmt.Errorf("legacy pitfall %s entries must be ADR numbers", field)
		}
		var value int
		if err := member.Decode(&value); err != nil {
			return nil, fmt.Errorf("legacy pitfall %s entries must be ADR numbers: %w", field, err)
		}
		values = append(values, value)
	}
	return values, nil
}

type plannedPitfallLeaf struct {
	entry  pitfall.Entry
	bytes  []byte
	path   string
	exists bool
}

type pitfallCorpusOperation struct {
	create        func(string, []byte) error
	writeSidecar  func(string, []byte, fs.FileMode) error
	removeSidecar func(string) error
}

func productionPitfallCorpusOperation() pitfallCorpusOperation {
	return pitfallCorpusOperation{
		create: func(path string, data []byte) error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return filepublication.Publish(path, data, 0o644)
		},
		writeSidecar:  manifest.WriteFileAtomicMode,
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
	info, err := os.Stat(sidecarPath)
	if err != nil { // coverage-ignore: requires the sidecar to disappear or fault after its successful read
		return err
	}
	var document yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(false)
	if err := dec.Decode(&document); err != nil {
		return fmt.Errorf("pitfall-corpus: parse %s: %w", sidecarPath, err)
	}
	var sidecar legacyPitfallSidecar
	if err := document.Decode(&sidecar); err != nil {
		return fmt.Errorf("pitfall-corpus: parse %s: %w", sidecarPath, err)
	}
	if !mappingPathPresent(&document, "data", "pitfalls") {
		return nil
	}

	used := map[string]bool{}
	plans := make([]plannedPitfallLeaf, 0, len(sidecar.Data.Pitfalls))
	for i, old := range sidecar.Data.Pitfalls {
		title := old.Title
		slug, err := pitfall.AllocateSlug(title, used)
		if err != nil {
			return fmt.Errorf("pitfall-corpus: entry %d %q: %w", i, title, err)
		}
		used[slug] = true
		e := pitfall.Entry{Slug: slug, SourcePath: pitfall.SourceDir + "/" + slug + ".md", Title: title, Domains: old.Domains, Tags: old.Tags, Related: old.Related, Body: old.Body}
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
	emptyRemainder, err := preflightPitfallSidecarRemainder(remainder)
	if err != nil {
		return fmt.Errorf("pitfall-corpus: preflight sidecar remainder: %w", err)
	}
	for i := range plans {
		plan := &plans[i]
		info, err := os.Lstat(plan.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil { // coverage-ignore: requires a filesystem permission or IO fault after successful sidecar read
			return fmt.Errorf("pitfall-corpus: inspect %s: %w", plan.entry.SourcePath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pitfall-corpus: destination %s is not a direct regular file", plan.entry.SourcePath)
		}
		existing, err := os.ReadFile(plan.path)
		if err != nil { // coverage-ignore: requires the regular destination to fault after successful Lstat
			return fmt.Errorf("pitfall-corpus: inspect %s: %w", plan.entry.SourcePath, err)
		}
		if !bytes.Equal(existing, plan.bytes) {
			return fmt.Errorf("pitfall-corpus: destination %s conflicts with migrated entry %q", plan.entry.SourcePath, plan.entry.Title)
		}
		plan.exists = true
	}
	for _, plan := range plans {
		if plan.exists {
			continue
		}
		if err := operation.create(plan.path, plan.bytes); err != nil {
			return fmt.Errorf("pitfall-corpus: create %s: %w", plan.entry.SourcePath, err)
		}
		out.Add("pitfall-corpus: created " + plan.entry.SourcePath)
	}
	if emptyRemainder {
		if err := operation.removeSidecar(sidecarPath); err != nil { // coverage-ignore: injected-operation failures are covered at the create boundary; production removal failure requires an IO race
			return fmt.Errorf("pitfall-corpus: retire sidecar: %w", err)
		}
		out.Add("pitfall-corpus: removed empty .awf/docs/pitfalls.yaml")
	} else {
		if err := operation.writeSidecar(sidecarPath, remainder, info.Mode().Perm()); err != nil {
			return fmt.Errorf("pitfall-corpus: retain sections-only sidecar: %w", err)
		}
		out.Add("pitfall-corpus: retained sections-only .awf/docs/pitfalls.yaml")
	}
	return nil
}

func mappingPathPresent(document *yaml.Node, keys ...string) bool {
	node := document
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		node = node.Content[0]
	}
	for _, key := range keys {
		if node.Kind != yaml.MappingNode {
			return false
		}
		var next *yaml.Node
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			return false
		}
		node = next
	}
	return true
}

func preflightPitfallSidecarRemainder(raw []byte) (bool, error) {
	var value map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &value); err != nil {
		return false, err
	}
	if len(value) == 0 {
		return true, nil
	}
	if len(value) != 1 {
		return false, errors.New("only sections configuration may remain")
	}
	if _, ok := value["sections"]; !ok {
		return false, errors.New("only sections configuration may remain")
	}
	var supported struct {
		Sections map[string]config.SectionOverride `yaml:"sections"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&supported); err != nil {
		return false, fmt.Errorf("invalid sections configuration: %w", err)
	}
	effective := false
	for name, override := range supported.Sections {
		if name != "prepend" && name != "append" {
			return false, fmt.Errorf("unsupported pitfall section %q", name)
		}
		if override.Drop {
			effective = true
		}
	}
	return !effective, nil
}
