package topic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// ScaffoldFile is one repository-relative authored input produced for a topic.
type ScaffoldFile struct {
	Path    string
	Content []byte
}

type scaffoldMetadata struct {
	Title   string   `yaml:"title"`
	Summary string   `yaml:"summary"`
	Paths   []string `yaml:"paths"`
}

// ScaffoldFiles validates and allocates the paired authored inputs for a topic.
// It inspects both trees but performs no writes.
func ScaffoldFiles(root string, cfg *config.Config, domain, title string) ([]ScaffoldFile, error) {
	if !kebabRE.MatchString(domain) {
		return nil, fmt.Errorf("topic domain %q must use lowercase kebab-case", domain)
	}
	if !slices.Contains(cfg.Domains, domain) {
		return nil, fmt.Errorf("topic domain %q is not configured", domain)
	}
	title = strings.TrimSpace(title)
	slug := slugNonAlnumRE.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return nil, fmt.Errorf("topic title %q has no usable characters for a slug", title)
	}
	if slug == "index" {
		return nil, errors.New("topic slug \"index\" is reserved for the generated domain index")
	}

	var metadataPath, partPath string
	for suffix := 1; ; suffix++ {
		candidate := slug
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", slug, suffix)
		}
		metadataPath = filepath.ToSlash(filepath.Join(config.DirName, "topics", "metadata", domain, candidate+".yaml"))
		partPath = filepath.ToSlash(filepath.Join(config.DirName, "topics", "parts", domain, candidate, "current-state.md"))
		metadataExists, err := scaffoldPathExists(filepath.Join(root, filepath.FromSlash(metadataPath)))
		if err != nil {
			return nil, err
		}
		partExists, err := scaffoldPathExists(filepath.Join(root, filepath.FromSlash(partPath)))
		if err != nil {
			return nil, err
		}
		if metadataExists != partExists {
			return nil, fmt.Errorf("topic %s/%s has an orphaned scaffold half; restore or remove it before scaffolding", domain, candidate)
		}
		if !metadataExists {
			break
		}
	}

	metadata, err := yaml.Marshal(scaffoldMetadata{
		Title:   title,
		Summary: "Current project contracts for this topic.",
		Paths:   []string{"replace/with/project/path/**"},
	})
	if err != nil { // coverage-ignore: the fixed string-only metadata struct always marshals
		return nil, err
	}
	metadata = append(metadata, []byte("# EDIT: replace the path placeholder above with anchored project paths.\n")...)
	part := []byte("<!-- awf:comment Replace the placeholder prose below, edit metadata paths, and add reviewed claims manually. -->\n" +
		"Current project contracts for this topic are documented here.\n\n" +
		"## Claims\n")
	return []ScaffoldFile{{Path: metadataPath, Content: metadata}, {Path: partPath, Content: part}}, nil
}

var (
	slugNonAlnumRE = regexp.MustCompile(`[^a-z0-9]+`)
	scaffoldStat   = os.Stat
)

func scaffoldPathExists(path string) (bool, error) {
	_, err := scaffoldStat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect topic scaffold path %q: %w", filepath.ToSlash(path), err)
}
