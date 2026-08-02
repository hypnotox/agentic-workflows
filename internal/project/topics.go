package project

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/templates"
	"gopkg.in/yaml.v3"
)

// QueryTopic assembles one read-only topic or claim projection from one
// intrinsically routed working snapshot. Active state and operation history therefore
// cannot come from different worktree universes.
func (p *Project) QueryTopic(ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	ws, err := p.workingCurrentState(ctx)
	if err != nil {
		return topic.QueryResult{}, err
	}
	findings := currentstate.Check(ws.Loaded.ADRs, ws.Loaded.Topics.All())
	if len(findings) > 0 {
		messages := make([]string, len(findings))
		for i, finding := range findings {
			messages[i] = finding.Message
		}
		return topic.QueryResult{}, fmt.Errorf("current-state validation failed: %s", strings.Join(messages, "; "))
	}
	return topic.Query(ws.Loaded.Topics, ws.Loaded.Corpus, selector, opts, safelyMatchablePaths(ws.Tree))
}

func (p *Project) generateTopicDocs(ctx context.Context, corpus topic.Corpus) (files []RenderedFile, deps map[string][]string, err error) {
	deps = map[string][]string{}
	topicTemplate, err := fs.ReadFile(templates.FS, topicTID)
	if err != nil { // coverage-ignore: the topic template is compile-time embedded
		return nil, nil, err
	}
	indexTemplate, err := fs.ReadFile(templates.FS, topicIndexTID)
	if err != nil { // coverage-ignore: the topic index template is compile-time embedded
		return nil, nil, err
	}
	var currentPaths []string
	if p.read != nil {
		currentPaths, err = p.read.Paths("")
		if err != nil { // coverage-ignore: the only injected production reader is snapshotTreeReader, whose in-memory enumeration cannot fail
			return nil, nil, err
		}
	} else if workingTree, snapErr := p.workingTree(ctx); snapErr == nil {
		currentPaths = safelyMatchablePaths(workingTree)
	} else {
		// Init and isolated renderer tests can render before a Git repository
		// exists; use the same canonical filesystem paths in that pre-adoption case.
		currentPaths, err = filesystemProjectReader{root: p.Root}.Paths("")
		if err != nil {
			return nil, nil, err
		}
	}
	base := strings.TrimRight(p.Cfg.DocsDir, "/") + "/topics"
	for _, discovered := range corpus.All() {
		t, _ := corpus.ByTopicID(discovered.ID.String())
		var referenceProjection []string
		for _, parsed := range t.Claims {
			claim, _ := corpus.ByClaimID(parsed.ID)
			referenceProjection = append(referenceProjection, claim.ID+"<"+strings.Join(corpus.Incoming(claim.ID), ",")+">"+strings.Join(corpus.Outgoing(claim.ID), ","))
		}
		model := topic.BuildTopicModel(t, corpus.DomainPaths[t.ID.Domain], corpus.Markers, currentPaths)
		content, err := topic.RenderTopic(topicTID, topicTemplate, model)
		if err != nil { // coverage-ignore: ParsePart already validated authoring comments and the typed model is always executable
			return nil, nil, fmt.Errorf("render topic %s: %w", t.ID.String(), err)
		}
		content = injectBanner(content, topicTID)
		cfgHash, err := topicHash(p.Root, p.projectTreeReader(), model, t.MetadataPath, t.PartPath)
		if err != nil { // coverage-ignore: topic loading just read both inputs; failure requires a concurrent filesystem race
			return nil, nil, err
		}
		path := base + "/" + t.ID.Domain + "/" + t.ID.Slug + ".md"
		metadataPath, partPath := relSlash(p.Root, t.MetadataPath), relSlash(p.Root, t.PartPath)
		observed := normalizeOutputInputs([]OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}, {Path: "templates/" + topicTID, Role: ArtifactTemplate}, {Path: metadataPath, Role: ArtifactTopicMetadata}, {Path: partPath, Role: ArtifactClaimPart}})
		files = append(files, RenderedFile{Path: path, Content: content, TemplateID: topicTID, TemplateHash: manifest.Hash(topicTemplate), ConfigHash: cfgHash, Policy: declaredPolicy("topics", false), Declarer: "topic:" + t.ID.String(), DeclarerProjection: t.ID.String() + "\x00" + strings.Join(referenceProjection, "\x00"), Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, ConsumedInputs: observed, ObservedTemplateID: topicTID})
		deps[path] = []string{metadataPath, partPath}
	}
	for _, domain := range slices.Sorted(slices.Values(p.Cfg.Domains)) {
		topics := corpus.ForDomain(domain)
		if len(topics) == 0 {
			continue
		}
		model := topic.BuildIndexModel(domain, topics)
		content, err := topic.RenderIndex(topicIndexTID, indexTemplate, model)
		if err != nil { // coverage-ignore: the embedded index template and typed model are always executable
			return nil, nil, fmt.Errorf("render topic index %s: %w", domain, err)
		}
		content = injectBanner(content, topicIndexTID)
		enc, _ := yaml.Marshal(model)
		path := base + "/" + domain + "/index.md"
		observed := []OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}, {Path: "templates/" + topicIndexTID, Role: ArtifactTemplate}}
		for _, t := range topics {
			metadataPath, partPath := relSlash(p.Root, t.MetadataPath), relSlash(p.Root, t.PartPath)
			deps[path] = append(deps[path], metadataPath, partPath)
			observed = append(observed, OutputInput{Path: metadataPath, Role: ArtifactTopicMetadata}, OutputInput{Path: partPath, Role: ArtifactClaimPart})
		}
		files = append(files, RenderedFile{Path: path, Content: content, TemplateID: topicIndexTID, TemplateHash: manifest.Hash(indexTemplate), ConfigHash: manifest.Hash(enc), Policy: declaredPolicy("topics", false), Declarer: "topic-index:" + domain, DeclarerProjection: domain, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, ConsumedInputs: normalizeOutputInputs(observed), ObservedTemplateID: topicIndexTID})
	}
	return files, deps, nil
}
func topicHash(root string, read ProjectTreeReader, model topic.TopicRenderModel, paths ...string) (string, error) {
	proj := map[string]any{"model": model}
	inputs := map[string]string{}
	for _, path := range paths {
		rel := relSlash(root, path)
		b, ok, err := read.ReadFile(rel)
		if err != nil {
			return "", err
		}
		if !ok { // coverage-ignore: topic loading just read both inputs from the same project-tree reader
			return "", fmt.Errorf("read topic hash input %s", rel)
		}
		inputs[rel] = manifest.Hash(b)
	}
	proj["inputs"] = inputs
	enc, err := yaml.Marshal(proj)
	if err != nil { // coverage-ignore: the projection contains only strings, slices, and typed topic models
		return "", err
	}
	return manifest.Hash(enc), nil
}
func relSlash(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil { // coverage-ignore: every topic input is discovered beneath the project root
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(r)
}

// safelyMatchablePaths returns every scannable snapshot path: the universe a
// selector may be matched against. Symlinks and deletions are excluded because
// matching a selector against them would attribute authority to a path that
// carries no content.
func safelyMatchablePaths(tree *snapshot.Tree) []string {
	out := []string{}
	for _, f := range tree.List() {
		if f.Scannable() {
			out = append(out, f.Path)
		}
	}
	return out
}
