package main

import (
	"testing"

	"agentic-workflows/internal/config"
)

func TestSkillState(t *testing.T) {
	cases := []struct {
		name    string
		sc      config.SkillConfig
		enabled bool
		want    string
	}{
		{
			name:    "available when not enabled",
			sc:      config.SkillConfig{},
			enabled: false,
			want:    "available",
		},
		{
			name:    "enabled when present with no customization",
			sc:      config.SkillConfig{},
			enabled: true,
			want:    "enabled",
		},
		{
			name:    "tuned when data set",
			sc:      config.SkillConfig{Data: map[string]any{"key": "val"}},
			enabled: true,
			want:    "tuned",
		},
		{
			name:    "tuned when sections set",
			sc:      config.SkillConfig{Sections: map[string]config.SectionOverride{"notes": {Drop: true}}},
			enabled: true,
			want:    "tuned",
		},
		{
			name:    "local when local flag set",
			sc:      config.SkillConfig{Local: true},
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
