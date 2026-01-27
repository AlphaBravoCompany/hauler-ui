package hauler

import (
	"context"
	"testing"
	"time"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMajor string
		wantMinor string
		wantPatch string
		wantFull  string
	}{
		{
			name:      "standard version with v prefix",
			input:     "hauler v1.2.3",
			wantMajor: "1",
			wantMinor: "2",
			wantPatch: "3",
			wantFull:  "v1.2.3",
		},
		{
			name:      "version without v prefix",
			input:     "hauler 1.2.3",
			wantMajor: "1",
			wantMinor: "2",
			wantPatch: "3",
			wantFull:  "v1.2.3",
		},
		{
			name:      "version with capital H",
			input:     "Hauler v1.2.3",
			wantMajor: "1",
			wantMinor: "2",
			wantPatch: "3",
			wantFull:  "v1.2.3",
		},
		{
			name:     "just version",
			input:    "v1.2.3",
			wantMajor: "1",
			wantMinor: "2",
			wantPatch: "3",
			wantFull:  "v1.2.3",
		},
		{
			name:     "unparseable version",
			input:    "unknown",
			wantMajor: "",
			wantMinor: "",
			wantPatch: "",
			wantFull:  "unknown",
		},
		{
			name:     "dev version",
			input:    "hauler dev",
			wantMajor: "",
			wantMinor: "",
			wantPatch: "",
			wantFull:  "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVersion(tt.input)
			if got.Major != tt.wantMajor {
				t.Errorf("parseVersion().Major = %v, want %v", got.Major, tt.wantMajor)
			}
			if got.Minor != tt.wantMinor {
				t.Errorf("parseVersion().Minor = %v, want %v", got.Minor, tt.wantMinor)
			}
			if got.Patch != tt.wantPatch {
				t.Errorf("parseVersion().Patch = %v, want %v", got.Patch, tt.wantPatch)
			}
			if got.Full != tt.wantFull {
				t.Errorf("parseVersion().Full = %v, want %v", got.Full, tt.wantFull)
			}
		})
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantFlag string
	}{
		{
			name: "standard flags",
			input: `Flags:
  -c, --config string   Path to config file
  -d, --debug           Enable debug mode
  -v, --verbose         Verbose output`,
			wantLen:  3,
			wantFlag: "config string",
		},
		{
			name: "flags with equals",
			input: `  --output=string   Output format (json|text)`,
			wantLen:  1,
			wantFlag: "output=string",
		},
		{
			name:     "no flags",
			input:    "Some text without flags",
			wantLen:  0,
			wantFlag: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlags(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("parseFlags() len = %v, want %v", len(got), tt.wantLen)
			}
			if tt.wantFlag != "" && len(got) > 0 {
				found := false
				for _, f := range got {
					if f.Name == tt.wantFlag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("parseFlags() did not find flag %v", tt.wantFlag)
				}
			}
		})
	}
}

func TestExtractSubcommands(t *testing.T) {
	d := New("hauler")

	tests := []struct {
		name         string
		input        string
		wantLen      int
		wantSubcmd   string
		wantDesc     string
	}{
		{
			name: "standard subcommands",
			input: `Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  store       Store operations
  version     Print version info`,
			wantLen:    4,
			wantSubcmd: "store",
			wantDesc:   "Store operations",
		},
		{
			name: "commands with aliases",
			input: `Commands:
  store, s    Store operations
  pull        Pull images`,
			wantLen:    2,
			wantSubcmd: "store",
			wantDesc:   "Store operations",
		},
		{
			name: "no subcommands",
			input: `No commands available`,
			wantLen:    0,
			wantSubcmd: "",
			wantDesc:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.extractSubcommands(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("extractSubcommands() len = %v, want %v", len(got), tt.wantLen)
			}
			if tt.wantSubcmd != "" {
				found := false
				for _, sc := range got {
					if sc.Name == tt.wantSubcmd {
						found = true
						if sc.Description != tt.wantDesc {
							t.Errorf("extractSubcommands()[%s].Description = %v, want %v", sc.Name, sc.Description, tt.wantDesc)
						}
						break
					}
				}
				if !found {
					t.Errorf("extractSubcommands() did not find subcommand %v", tt.wantSubcmd)
				}
			}
		})
	}
}

func TestDetectorCache(t *testing.T) {
	d := New("echo")
	d.cacheTTL = 100 * time.Millisecond

	ctx := context.Background()

	// First call should populate cache
	caps1, err := d.Get(ctx)
	if err != nil {
		t.Fatalf("First Get() failed: %v", err)
	}

	// Second call within TTL should return cached
	caps2, err := d.Get(ctx)
	if err != nil {
		t.Fatalf("Second Get() failed: %v", err)
	}

	if caps1.LastRefresh != caps2.LastRefresh {
		t.Error("Cached capabilities should have same LastRefresh time")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call should refresh
	caps3, err := d.Get(ctx)
	if err != nil {
		t.Fatalf("Third Get() failed: %v", err)
	}

	if caps3.LastRefresh.Equal(caps2.LastRefresh) {
		t.Error("Capabilities should have been refreshed after TTL")
	}
}

func TestRefresh(t *testing.T) {
	d := New("echo")
	d.cacheTTL = 1 * time.Hour

	ctx := context.Background()

	// Initial get
	caps1, err := d.Get(ctx)
	if err != nil {
		t.Fatalf("First Get() failed: %v", err)
	}

	// Wait a bit and force refresh
	time.Sleep(50 * time.Millisecond)

	caps2, err := d.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh() failed: %v", err)
	}

	if !caps2.LastRefresh.After(caps1.LastRefresh) {
		t.Error("Refresh() should update LastRefresh time")
	}
}
