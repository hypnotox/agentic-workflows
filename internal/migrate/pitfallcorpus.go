package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
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

type pitfallCorpusFilesystem interface {
	LinkInfo(string) (fs.FileInfo, error)
	Read(string) ([]byte, error)
	MkdirAll(string, fs.FileMode) error
	Publish(string, []byte, fs.FileMode) error
	Replace(string, []byte, fs.FileMode) error
	Remove(string) error
}

const pitfallSidecarPath = config.DirName + "/docs/pitfalls.yaml"
const pitfallDocsRoot = config.DirName + "/docs"

func applyPitfallCorpus(root string, out *Changes) (returnErr error) {
	tree, err := filesystem.Open(root)
	if err != nil {
		return fmt.Errorf("pitfall-corpus: open repository root: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, tree.Close()) }()
	return applyPitfallCorpusWith(out, tree)
}

func applyPitfallCorpusWith(out *Changes, operation pitfallCorpusFilesystem) error {
	info, err := operation.LinkInfo(pitfallSidecarPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("pitfall-corpus: inspect %s: %w", pitfallSidecarPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pitfall-corpus: sidecar %s is not a direct regular file", pitfallSidecarPath)
	}
	raw, err := operation.Read(pitfallSidecarPath)
	if err != nil {
		return fmt.Errorf("pitfall-corpus: read %s: %w", pitfallSidecarPath, err)
	}
	var document yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(false)
	if err := dec.Decode(&document); err != nil {
		return fmt.Errorf("pitfall-corpus: parse %s: %w", pitfallSidecarPath, err)
	}
	var sidecar legacyPitfallSidecar
	if err := document.Decode(&sidecar); err != nil {
		return fmt.Errorf("pitfall-corpus: parse %s: %w", pitfallSidecarPath, err)
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
		plans = append(plans, plannedPitfallLeaf{entry: e, bytes: serialized, path: e.SourcePath})
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
	docsInfo, err := operation.LinkInfo(pitfallDocsRoot)
	if err != nil {
		return fmt.Errorf("pitfall-corpus: inspect source root %s: %w", pitfallDocsRoot, err)
	}
	if !docsInfo.IsDir() {
		return fmt.Errorf("pitfall-corpus: source root %s is not a direct directory", pitfallDocsRoot)
	}
	pitfallsRootMissing := false
	pitfallsInfo, err := operation.LinkInfo(pitfall.SourceDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		pitfallsRootMissing = true
	case err != nil:
		return fmt.Errorf("pitfall-corpus: inspect destination root %s: %w", pitfall.SourceDir, err)
	case !pitfallsInfo.IsDir():
		return fmt.Errorf("pitfall-corpus: destination root %s is not a direct directory", pitfall.SourceDir)
	}
	missingLeaf := false
	for i := range plans {
		plan := &plans[i]
		destinationInfo, err := operation.LinkInfo(plan.path)
		if errors.Is(err, fs.ErrNotExist) {
			missingLeaf = true
			continue
		}
		if err != nil {
			return fmt.Errorf("pitfall-corpus: inspect destination %s: %w", plan.entry.SourcePath, err)
		}
		if !destinationInfo.Mode().IsRegular() {
			return fmt.Errorf("pitfall-corpus: destination %s is not a direct regular file", plan.entry.SourcePath)
		}
		existing, err := operation.Read(plan.path)
		if err != nil {
			return fmt.Errorf("pitfall-corpus: read destination %s: %w", plan.entry.SourcePath, err)
		}
		if !bytes.Equal(existing, plan.bytes) {
			return fmt.Errorf("pitfall-corpus: destination %s conflicts with migrated entry %q", plan.entry.SourcePath, plan.entry.Title)
		}
		plan.exists = true
	}
	if missingLeaf && pitfallsRootMissing {
		if err := operation.MkdirAll(pitfall.SourceDir, 0o755); err != nil {
			return fmt.Errorf("pitfall-corpus: create destination root %s: %w", pitfall.SourceDir, err)
		}
	}
	for _, plan := range plans {
		if plan.exists {
			continue
		}
		if err := operation.Publish(plan.path, plan.bytes, 0o644); err != nil {
			var committed *filepublication.CommittedCleanupError
			if errors.As(err, &committed) {
				out.Add("pitfall-corpus: created " + plan.entry.SourcePath)
				return fmt.Errorf("pitfall-corpus: destination %s is committed but publication cleanup residue %s remains; remove the residue and retry awf upgrade before retiring legacy authority: %w", plan.entry.SourcePath, committed.ResiduePath, err)
			}
			return fmt.Errorf("pitfall-corpus: create %s: %w", plan.entry.SourcePath, err)
		}
		out.Add("pitfall-corpus: created " + plan.entry.SourcePath)
	}
	if emptyRemainder {
		if err := operation.Remove(pitfallSidecarPath); err != nil {
			return fmt.Errorf("pitfall-corpus: retire sidecar %s: %w", pitfallSidecarPath, err)
		}
		out.Add("pitfall-corpus: removed empty .awf/docs/pitfalls.yaml")
	} else {
		if err := operation.Replace(pitfallSidecarPath, remainder, info.Mode().Perm()); err != nil {
			return fmt.Errorf("pitfall-corpus: retain sections-only sidecar %s: %w", pitfallSidecarPath, err)
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
