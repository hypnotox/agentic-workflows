package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// historicalConfig is the migration-local view of fields retired from the live
// schema. Frozen migrations use the shape that existed at their generation;
// it must not extend config.Config with retired fields.
type historicalConfig struct {
	Skills            []string `yaml:"skills"`
	Agents            []string `yaml:"agents"`
	Docs              []string `yaml:"docs"`
	DocsDir           string   `yaml:"docsDir"`
	IntegrationBranch string   `yaml:"integrationBranch"`
	root              string
}

type historicalSidecar struct {
	Local bool `yaml:"local"`
}

func loadHistoricalConfig(root string, src []byte) (*historicalConfig, error) {
	var cfg historicalConfig
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(false)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse historical config: %w", err)
	}
	if cfg.DocsDir == "" {
		cfg.DocsDir = "docs"
	}
	cfg.root = root
	return &cfg, nil
}

func (c *historicalConfig) Sidecar(kind, name string) (historicalSidecar, error) {
	path := filepath.Join(c.root, ".awf", kind, name+".yaml")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return historicalSidecar{}, nil
	}
	if err != nil {
		return historicalSidecar{}, fmt.Errorf("read historical sidecar %s: %w", path, err)
	}
	var sidecar historicalSidecar
	if err := yaml.Unmarshal(b, &sidecar); err != nil {
		return historicalSidecar{}, fmt.Errorf("parse historical sidecar %s: %w", path, err)
	}
	return sidecar, nil
}
