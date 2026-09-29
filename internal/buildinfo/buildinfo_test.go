package buildinfo_test

import (
	"github.com/winlex/spindle/internal/buildinfo"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	info := buildinfo.New()

	if info.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
	if info.StartedAt.IsZero() {
		t.Error("StartedAt is zero - New must always fill the time")
	}
	if !strings.HasPrefix(info.GoVersion, "go1.") {
		t.Errorf("unexpected GoVersion: %q", info.GoVersion)
	}
}

func TestInfoString(t *testing.T) {
	ts := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		info buildinfo.Info
		want []string
	}{
		{name: "dev build", info: buildinfo.New(), want: []string{"spindle-api", "go1."}},
		{
			name: "all fields set",
			info: buildinfo.Info{Name: "api", Version: "v1.2.3", GoVersion: "go1.27.1", StartedAt: ts},
			want: []string{"api v1.2.3", "go1.27.1", "2026-09-28T12:00:00Z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("String() = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}
