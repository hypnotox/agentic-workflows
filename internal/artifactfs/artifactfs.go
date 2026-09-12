// Package artifactfs creates author-owned change documents, ADRs, and topics.
package artifactfs

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/hypnotox/agentic-workflows/internal/projector"
)

//go:embed templates/*.md
var starterFiles embed.FS

var starterTemplates = template.Must(template.New("starters").Funcs(template.FuncMap{
	"quote": strconv.Quote,
}).ParseFS(starterFiles, "templates/*.md"))

type starterData struct {
	Name      string
	Selectors []string
}

// NewIntent creates a tracked change intent scaffold.
func NewIntent(root, slug string) (string, error) {
	return newChangeDocument(root, slug, "intent")
}

// NewSpec creates a tracked change specification scaffold.
func NewSpec(root, slug string) (string, error) {
	return newChangeDocument(root, slug, "spec")
}

// NewPlan creates a tracked implementation plan scaffold.
func NewPlan(root, slug string) (string, error) {
	if err := validateSlug("plan", slug); err != nil {
		return "", err
	}
	body, err := renderStarter("plan", starterData{Name: slug})
	if err != nil {
		return "", err
	}
	relative := filepath.Join("docs", "plans", slug+".md")
	return create(root, relative, "plan", slug, body)
}

func newChangeDocument(root, slug, kind string) (string, error) {
	if err := validateSlug(kind, slug); err != nil {
		return "", err
	}
	body, err := renderStarter(kind, starterData{Name: slug})
	if err != nil {
		return "", err
	}
	relative := filepath.Join("docs", "changes", slug, kind+".md")
	return create(root, relative, kind, slug, body)
}

// NewADR creates a tracked architecture decision record scaffold.
func NewADR(root, slug string) (string, error) {
	if err := validateSlug("decision", slug); err != nil {
		return "", err
	}
	body, err := renderStarter("adr", starterData{Name: slug})
	if err != nil {
		return "", err
	}
	relative := filepath.Join("docs", "decisions", slug+".md")
	return create(root, relative, "decision record", slug, body)
}

// NewTopic creates a path-routed topic source with the supplied selectors.
func NewTopic(root, id string, selectors []string) (string, error) {
	if err := validateTopicID(id); err != nil {
		return "", err
	}
	if _, _, err := projector.NormalizeTopicPatterns(selectors); err != nil {
		return "", err
	}
	body, err := renderStarter("topic", starterData{Name: id, Selectors: selectors})
	if err != nil {
		return "", err
	}

	relative := filepath.Join(filepath.FromSlash(projector.TopicsPath), filepath.FromSlash(id)+".md")
	return create(root, relative, "topic", id, body)
}

func create(root, relative, kind, name, body string) (string, error) {
	filename := filepath.Join(root, relative)
	if err := ensureDirectory(root, filepath.Dir(relative)); err != nil {
		return "", fmt.Errorf("create %s directory: %w", kind, err)
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%s %q already exists", kind, name)
		}
		return "", fmt.Errorf("create %s %q: %w", kind, name, err)
	}
	if _, err := io.WriteString(file, body); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write %s %q: %w", kind, name, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close %s %q: %w", kind, name, err)
	}
	return relative, nil
}

func ensureDirectory(root, relative string) error {
	current := root
	for _, component := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		if component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if mkdirErr := os.Mkdir(current, 0o755); mkdirErr == nil {
				continue
			} else if !os.IsExist(mkdirErr) {
				return fmt.Errorf("create %s: %w", filepath.ToSlash(relative), mkdirErr)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", filepath.ToSlash(relative), err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", filepath.ToSlash(relative))
		}
	}
	return nil
}

func validateSlug(kind, slug string) error {
	if slug == "" {
		return fmt.Errorf("invalid %s slug: use letters, numbers, hyphens, or underscores", kind)
	}
	for i, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || (i > 0 && (char == '-' || char == '_')) {
			continue
		}
		return fmt.Errorf("invalid %s slug %q: use letters, numbers, hyphens, or underscores", kind, slug)
	}
	return nil
}

func validateTopicID(id string) error {
	if id == "" || strings.HasSuffix(strings.ToLower(id), ".md") || strings.Contains(id, `\`) || path.IsAbs(id) || path.Clean(id) != id {
		return fmt.Errorf("invalid topic ID %q: use slash-separated letters, numbers, hyphens, or underscores without .md", id)
	}
	for _, component := range strings.Split(id, "/") {
		if err := validateSlug("topic ID", component); err != nil {
			return fmt.Errorf("invalid topic ID %q: use slash-separated letters, numbers, hyphens, or underscores without .md", id)
		}
	}
	return nil
}

func renderStarter(name string, data starterData) (string, error) {
	var body bytes.Buffer
	if err := starterTemplates.ExecuteTemplate(&body, name+".md", data); err != nil {
		return "", fmt.Errorf("render %s starter: %w", name, err)
	}
	return body.String(), nil
}
