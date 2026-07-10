package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// invariant: inert-sidecar-field-rejected
func TestOpenRejectsInertSidecarFields(t *testing.T) {
	cases := []struct {
		name    string
		cfg     string
		files   map[string]string
		wantErr string
	}{
		{
			name:    "paths on a skill sidecar",
			cfg:     "prefix: example\nskills:\n  - tdd\nagents: []\n",
			files:   map[string]string{"skills/tdd.yaml": "paths:\n  - '**/*.go'\n"},
			wantErr: "paths: is read only from domain sidecars",
		},
		{
			name:    "paths on a local skill sidecar",
			cfg:     "prefix: example\nskills:\n  - tdd\nagents: []\n",
			files:   map[string]string{"skills/tdd.yaml": "local: true\npaths:\n  - '**/*.go'\n"},
			wantErr: "paths: is read only from domain sidecars",
		},
		{
			name:    "data on a domain sidecar",
			cfg:     "prefix: example\nskills: []\nagents: []\ndomains:\n  - config\n",
			files:   map[string]string{"domains/config.yaml": "data:\n  k: v\n"},
			wantErr: "a domain sidecar is paths-only",
		},
		{
			name:    "sections on a domain sidecar",
			cfg:     "prefix: example\nskills: []\nagents: []\ndomains:\n  - config\n",
			files:   map[string]string{"domains/config.yaml": "sections:\n  current-state:\n    drop: true\n"},
			wantErr: "a domain sidecar is paths-only",
		},
		{
			name:    "local on a domain sidecar",
			cfg:     "prefix: example\nskills: []\nagents: []\ndomains:\n  - config\n",
			files:   map[string]string{"domains/config.yaml": "local: true\n"},
			wantErr: "a domain sidecar is paths-only",
		},
		{
			name:    "paths on the agents-doc sidecar",
			cfg:     "prefix: example\nskills: []\nagents: []\n",
			files:   map[string]string{"agents-doc.yaml": "paths:\n  - '**/*.go'\n"},
			wantErr: ".awf/agents-doc.yaml",
		},
		{
			name:    "paths on a plain singleton sidecar",
			cfg:     "prefix: example\nskills: []\nagents: []\n",
			files:   map[string]string{"workflow.yaml": "paths:\n  - '**/*.go'\n"},
			wantErr: ".awf/workflow.yaml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFiles(t, tc.cfg, tc.files)
			_, err := Open(root)
			if err == nil {
				t.Fatalf("Open should reject the inert field")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestOpenPropagatesDomainSidecarReadError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains:\n  - config\n", nil)
	if err := os.MkdirAll(filepath.Join(root, ".awf", "domains", "config.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("want sidecar read error from Open, got %v", err)
	}
}

func TestOpenAcceptsPathsOnlyDomainSidecar(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains:\n  - config\n",
		map[string]string{"domains/config.yaml": "paths:\n  - internal/config/**\n"})
	if _, err := Open(root); err != nil {
		t.Fatalf("Open should accept a paths-only domain sidecar: %v", err)
	}
}
