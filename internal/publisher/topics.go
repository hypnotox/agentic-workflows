package publisher

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/templates"
	"gopkg.in/yaml.v3"
)

func generateTopicDocs(p renderInputs, corpus topic.Corpus) (files []RenderedFile, deps map[string][]string, err error) {
	deps = map[string][]string{}
	topicTemplate, err := fs.ReadFile(templates.FS, topicTID)
	if err != nil {
		return nil, nil, err
	}
	indexTemplate, err := fs.ReadFile(templates.FS, topicIndexTID)
	if err != nil {
		return nil, nil, err
	}
	currentPaths, err := p.read.Paths("")
	if err != nil {
		return nil, nil, err
	}
	base := config.DocsDir + "/topics"
	for _, discovered := range corpus.All() {
		t, _ := corpus.ByTopicID(discovered.ID.String())
		var referenceProjection []string
		for _, parsed := range t.Claims {
			claim, _ := corpus.ByClaimID(parsed.ID)
			referenceProjection = append(referenceProjection, claim.ID+"<"+strings.Join(corpus.Incoming(claim.ID), ",")+">"+strings.Join(corpus.Outgoing(claim.ID), ","))
		}
		model := topic.BuildTopicModel(t, corpus.DomainPaths[t.ID.Domain], corpus.Markers, currentPaths)
		content, err := topic.RenderTopic(topicTID, topicTemplate, model)
		if err != nil {
			return nil, nil, fmt.Errorf("render topic %s: %w", t.ID.String(), err)
		}
		metadataPath, partPath := relSlash(p.root(), t.MetadataPath), relSlash(p.root(), t.PartPath)
		marker, templateInputs, err := templateSourceRootMarker(p, topicTID)
		if err != nil {
			return nil, nil, err
		}
		content = injectSourceMarker(injectBanner(marker+content, topicTID), []string{metadataPath, partPath})
		cfgHash, err := topicHash(p.root(), projectTreeReader(p), model, t.MetadataPath, t.PartPath)
		if err != nil {
			return nil, nil, err
		}
		path := base + "/" + t.ID.Domain + "/" + t.ID.Slug + ".md"
		cfgHash = templateSourceConfigHash(cfgHash, templateSourceRoot(p))
		observed := normalizeOutputInputs(append([]OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}, {Path: "templates/" + topicTID, Role: ArtifactTemplate}, {Path: metadataPath, Role: ArtifactTopicMetadata}, {Path: partPath, Role: ArtifactClaimPart}}, templateInputs...))
		files = append(files, RenderedFile{Path: path, Content: content, TemplateID: topicTID, TemplateHash: manifest.Hash(topicTemplate), ConfigHash: cfgHash, Policy: declaredPolicy("topics", false), Declarer: "topic:" + t.ID.String(), DeclarerProjection: t.ID.String() + "\x00" + strings.Join(referenceProjection, "\x00"), Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, ConsumedInputs: observed, ObservedTemplateID: topicTID})
		deps[path] = []string{metadataPath, partPath}
	}
	for _, domain := range slices.Sorted(slices.Values(p.cfg.Domains)) {
		topics := corpus.ForDomain(domain)
		if len(topics) == 0 {
			continue
		}
		model := topic.BuildIndexModel(domain, topics)
		content, err := topic.RenderIndex(topicIndexTID, indexTemplate, model)
		if err != nil {
			return nil, nil, fmt.Errorf("render topic index %s: %w", domain, err)
		}
		marker, templateInputs, err := templateSourceRootMarker(p, topicIndexTID)
		if err != nil {
			return nil, nil, err
		}
		content = injectSourceMarker(injectBanner(marker+content, topicIndexTID), []string{
			config.DirName + "/topics/metadata/" + domain + "/*.yaml",
			config.DirName + "/topics/parts/" + domain + "/*/current-state.md",
		})
		enc, _ := yaml.Marshal(model)
		path := base + "/" + domain + "/index.md"
		observed := []OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}, {Path: "templates/" + topicIndexTID, Role: ArtifactTemplate}}
		for _, t := range topics {
			metadataPath, partPath := relSlash(p.root(), t.MetadataPath), relSlash(p.root(), t.PartPath)
			deps[path] = append(deps[path], metadataPath, partPath)
			observed = append(observed, OutputInput{Path: metadataPath, Role: ArtifactTopicMetadata}, OutputInput{Path: partPath, Role: ArtifactClaimPart})
		}
		files = append(files, RenderedFile{Path: path, Content: content, TemplateID: topicIndexTID, TemplateHash: manifest.Hash(indexTemplate), ConfigHash: templateSourceConfigHash(manifest.Hash(enc), templateSourceRoot(p)), Policy: declaredPolicy("topics", false), Declarer: "topic-index:" + domain, DeclarerProjection: domain, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, ConsumedInputs: normalizeOutputInputs(append(observed, templateInputs...)), ObservedTemplateID: topicIndexTID})
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
		if !ok {
			return "", fmt.Errorf("read topic hash input %s", rel)
		}
		inputs[rel] = manifest.Hash(b)
	}
	proj["inputs"] = inputs
	enc, err := yaml.Marshal(proj)
	if err != nil {
		return "", err
	}
	return manifest.Hash(enc), nil
}
func relSlash(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(r)
}
