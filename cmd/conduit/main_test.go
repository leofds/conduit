package main

import (
	"testing"
)

func TestParseFlagsSupportsLongAndShortWriteDefaultsAliases(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantWrite     bool
		wantResetHost string
	}{
		{
			name:      "long form",
			args:      []string{"--write-defaults"},
			wantWrite: true,
		},
		{
			name:      "single dash long form",
			args:      []string{"-write-defaults"},
			wantWrite: true,
		},
		{
			name:      "short alias",
			args:      []string{"-W"},
			wantWrite: true,
		},
		{
			name:          "reset host remains supported",
			args:          []string{"--reset-known-host", "myserver"},
			wantResetHost: "myserver",
		},
		{
			name:          "short reset alias remains supported",
			args:          []string{"-R", "myserver"},
			wantResetHost: "myserver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetHost, writeDefaults, _, err := parseFlags(tt.args)
			if err != nil {
				t.Fatalf("parseFlags() error = %v", err)
			}
			if resetHost != tt.wantResetHost {
				t.Fatalf("parseFlags() resetHost = %q, want %q", resetHost, tt.wantResetHost)
			}
			if writeDefaults != tt.wantWrite {
				t.Fatalf("parseFlags() writeDefaults = %v, want %v", writeDefaults, tt.wantWrite)
			}
		})
	}
}
