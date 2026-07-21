package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
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
// cutoff-aware working snapshot. Active state and v1 operation history therefore
// cannot come from different worktree universes.
func (p *Project) QueryTopic(selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	p.beginInvocation()
	ws, err := p.workingCurrentState()
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
	return topic.Query(ws.Loaded.Topics, adr.NewCorpus(ws.Loaded.ADRs), selector, opts, safelyMatchablePaths(ws.Tree))
}

func (p *Project) Topics() (topic.Corpus, error) {
	if p.topics != nil {
		return *p.topics, nil
	}
	adrs, err := p.Corpus()
	if err != nil {
		return topic.Corpus{}, err
	}
	c, err := topic.LoadCorpus(p.Root, p.Cfg, adrs)
	if err != nil {
		return topic.Corpus{}, err
	}
	p.topics = &c
	return c, nil
}

func (p *Project) generateTopicDocs() (files []RenderedFile, deps map[string][]string, err error) {
	corpus, err := p.Topics()
	if err != nil {
		return nil, nil, err
	}
	deps = map[string][]string{}
	topicTemplate, err := fs.ReadFile(templates.FS, "topics/topic.md.tmpl")
	if err != nil { // coverage-ignore: the topic template is compile-time embedded
		return nil, nil, err
	}
	indexTemplate, err := fs.ReadFile(templates.FS, "topics/index.md.tmpl")
	if err != nil { // coverage-ignore: the topic index template is compile-time embedded
		return nil, nil, err
	}
	var currentPaths []string
	if workingTree, snapErr := snapshot.WorkingTree(p.Root); snapErr == nil {
		currentPaths = safelyMatchablePaths(workingTree)
	} else {
		// Init and isolated renderer tests can render before a Git repository
		// exists; use the same canonical filesystem paths in that pre-adoption case.
		currentPaths = filesystemProjectReader{root: p.Root}.Paths("")
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
		content, err := topic.RenderTopic(model)
		if err != nil { // coverage-ignore: ParsePart already validated authoring comments and the typed model is always executable
			return nil, nil, fmt.Errorf("render topic %s: %w", t.ID.String(), err)
		}
		content = injectBanner(content, "topics/topic.md.tmpl")
		cfgHash, err := topicHash(p.Root, model, t.MetadataPath, t.PartPath)
		if err != nil { // coverage-ignore: topic loading just read both inputs; failure requires a concurrent filesystem race
			return nil, nil, err
		}
		path := base + "/" + t.ID.Domain + "/" + t.ID.Slug + ".md"
		metadataPath, partPath := relSlash(p.Root, t.MetadataPath), relSlash(p.Root, t.PartPath)
		observed := normalizeOutputInputs([]OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}, {Path: "templates/topics/topic.md.tmpl", Role: ArtifactTemplate}, {Path: metadataPath, Role: ArtifactTopicMetadata}, {Path: partPath, Role: ArtifactClaimPart}})
		files = append(files, RenderedFile{Path: path, Content: content, TemplateID: "topics/topic.md.tmpl", TemplateHash: manifest.Hash(topicTemplate), ConfigHash: cfgHash, Policy: declaredPolicy("topics", false), Declarer: "topic:" + t.ID.String(), DeclarerProjection: t.ID.String() + "\x00" + strings.Join(referenceProjection, "\x00"), Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, ConsumedInputs: observed, ObservedTemplateID: "topics/topic.md.tmpl"})
		deps[path] = []string{metadataPath, partPath}
	}
	for _, domain := range slices.Sorted(slices.Values(p.Cfg.Domains)) {
		topics := corpus.ForDomain(domain)
		if len(topics) == 0 {
			continue
		}
		model := topic.BuildIndexModel(domain, topics)
		content, err := topic.RenderIndex(model)
		if err != nil { // coverage-ignore: the embedded index template and typed model are always executable
			return nil, nil, fmt.Errorf("render topic index %s: %w", domain, err)
		}
		content = injectBanner(content, "topics/index.md.tmpl")
		enc, _ := yaml.Marshal(model)
		path := base + "/" + domain + "/index.md"
		observed := []OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}, {Path: "templates/topics/index.md.tmpl", Role: ArtifactTemplate}}
		for _, t := range topics {
			metadataPath, partPath := relSlash(p.Root, t.MetadataPath), relSlash(p.Root, t.PartPath)
			deps[path] = append(deps[path], metadataPath, partPath)
			observed = append(observed, OutputInput{Path: metadataPath, Role: ArtifactTopicMetadata}, OutputInput{Path: partPath, Role: ArtifactClaimPart})
		}
		files = append(files, RenderedFile{Path: path, Content: content, TemplateID: "topics/index.md.tmpl", TemplateHash: manifest.Hash(indexTemplate), ConfigHash: manifest.Hash(enc), Policy: declaredPolicy("topics", false), Declarer: "topic-index:" + domain, DeclarerProjection: domain, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, ConsumedInputs: normalizeOutputInputs(observed), ObservedTemplateID: "topics/index.md.tmpl"})
	}
	return files, deps, nil
}
func topicHash(root string, model topic.TopicRenderModel, paths ...string) (string, error) {
	proj := map[string]any{"model": model}
	inputs := map[string]string{}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: topic loading just read both inputs; failure requires a concurrent filesystem race
			return "", err
		}
		inputs[relSlash(root, path)] = manifest.Hash(b)
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
