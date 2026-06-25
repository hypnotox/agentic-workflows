package main

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestSkillState(t *testing.T) {
	cases := []struct {
		name    string
		sc      config.Sidecar
		enabled bool
		want    string
	}{
		{
			name:    "available when not enabled",
			sc:      config.Sidecar{},
			enabled: false,
			want:    "available",
		},
		{
			name:    "enabled when present with no customization",
			sc:      config.Sidecar{},
			enabled: true,
			want:    "enabled",
		},
		{
			name:    "tuned when data set",
			sc:      config.Sidecar{Data: map[string]any{"key": "val"}},
			enabled: true,
			want:    "tuned",
		},
		{
			name:    "tuned when sections set",
			sc:      config.Sidecar{Sections: map[string]config.SectionOverride{"notes": {Drop: true}}},
			enabled: true,
			want:    "tuned",
		},
		{
			name:    "local when local flag set",
			sc:      config.Sidecar{Local: true},
			enabled: true,
			want:    "local",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := skillState(tc.sc, tc.enabled)
			if got != tc.want {
				t.Errorf("skillState(%+v, enabled=%v) = %q, want %q", tc.sc, tc.enabled, got, tc.want)
			}
		})
	}
}
