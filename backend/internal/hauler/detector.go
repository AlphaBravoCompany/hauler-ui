package hauler

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// VersionInfo holds parsed version information
type VersionInfo struct {
	Major string
	Minor string
	Patch string
	Full  string
}

// Flag represents a command-line flag
type Flag struct {
	Name        string
	Short       string
	Description string
	Default     string
}

// Subcommand represents a hauler subcommand with its flags
type Subcommand struct {
	Name        string
	Description string
	Flags       []Flag
}

// Capabilities holds detected hauler capabilities
type Capabilities struct {
	Version     VersionInfo            `json:"version"`
	Subcommands []Subcommand           `json:"subcommands"`
	GlobalFlags []Flag                 `json:"globalFlags"`
	LastRefresh time.Time              `json:"lastRefresh"`
	RawHelp     map[string]string      `json:"rawHelp,omitempty"` // Store raw help output for debugging
}

// Detector handles hauler version and capabilities detection
type Detector struct {
	mu           sync.RWMutex
	cached       *Capabilities
	haulerBinary string
	cacheTTL     time.Duration
}

// New creates a new detector with the specified hauler binary path
func New(haulerBinary string) *Detector {
	return &Detector{
		haulerBinary: haulerBinary,
		cacheTTL:     5 * time.Minute,
	}
}

// Get retrieves capabilities, using cache if valid
func (d *Detector) Get(ctx context.Context) (*Capabilities, error) {
	d.mu.RLock()
	if d.cached != nil && time.Since(d.cached.LastRefresh) < d.cacheTTL {
		cached := d.cached
		d.mu.RUnlock()
		return cached, nil
	}
	d.mu.RUnlock()

	// Cache miss or expired, acquire write lock and refresh
	d.mu.Lock()
	defer d.mu.Unlock()

	// Double-check in case another goroutine refreshed while we waited
	if d.cached != nil && time.Since(d.cached.LastRefresh) < d.cacheTTL {
		return d.cached, nil
	}

	caps, err := d.detect(ctx)
	if err != nil {
		return nil, err
	}

	d.cached = caps
	return caps, nil
}

// Refresh forces a refresh of the capabilities cache
func (d *Detector) Refresh(ctx context.Context) (*Capabilities, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	caps, err := d.detect(ctx)
	if err != nil {
		return nil, err
	}

	d.cached = caps
	return caps, nil
}

// detect runs hauler commands to detect version and capabilities
func (d *Detector) detect(ctx context.Context) (*Capabilities, error) {
	// Get version
	version, err := d.getVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting version: %w", err)
	}

	// Get global help to discover subcommands
	globalHelp, globalFlags, err := d.parseHelp(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing global help: %w", err)
	}

	// Get subcommands from global help
	subcommands := d.extractSubcommands(globalHelp)

	// Get detailed help for each subcommand
	rawHelp := make(map[string]string)
	rawHelp["global"] = globalHelp

	for i := range subcommands {
		subHelp, flags, err := d.parseHelp(ctx, []string{subcommands[i].Name, "--help"})
		if err != nil {
			// Subcommand might not support --help, skip
			continue
		}
		subcommands[i].Flags = flags
		rawHelp[subcommands[i].Name] = subHelp
	}

	return &Capabilities{
		Version:     *version,
		Subcommands: subcommands,
		GlobalFlags: globalFlags,
		LastRefresh: time.Now(),
		RawHelp:     rawHelp,
	}, nil
}

// getVersion runs `hauler version` and parses the output
func (d *Detector) getVersion(ctx context.Context) (*VersionInfo, error) {
	cmd := exec.CommandContext(ctx, d.haulerBinary, "version")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s version: %w", d.haulerBinary, err)
	}

	output := strings.TrimSpace(out.String())
	return parseVersion(output), nil
}

// parseVersion parses version output like "hauler v1.2.3" or "v1.2.3"
func parseVersion(output string) *VersionInfo {
	// Remove "hauler" prefix if present
	output = strings.TrimPrefix(output, "hauler ")
	output = strings.TrimPrefix(output, "Hauler ")
	output = strings.TrimSpace(output)

	// Extract version string - looks for "v1.2.3" or "1.2.3"
	versionRe := regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)
	matches := versionRe.FindStringSubmatch(output)

	if len(matches) < 4 {
		// No version found, return the raw output
		return &VersionInfo{Full: output}
	}

	return &VersionInfo{
		Major: matches[1],
		Minor: matches[2],
		Patch: matches[3],
		Full:  "v" + matches[1] + "." + matches[2] + "." + matches[3],
	}
}

// parseHelp runs hauler with args and parses flags from the output
func (d *Detector) parseHelp(ctx context.Context, args []string) (string, []Flag, error) {
	cmdArgs := append([]string{}, args...)
	if len(args) == 0 {
		cmdArgs = []string{"--help"}
	}

	cmd := exec.CommandContext(ctx, d.haulerBinary, cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", nil, fmt.Errorf("running %s %v: %w", d.haulerBinary, cmdArgs, err)
	}

	output := out.String()
	flags := parseFlags(output)

	return output, flags, nil
}

// parseFlags extracts flag definitions from help text
func parseFlags(helpText string) []Flag {
	var flags []Flag

	// Common patterns for flags in help text:
	// -f, --flag    description
	// --flag string description
	// --flag        description
	// -f            description
	lines := strings.Split(helpText, "\n")

	// Pattern for: -f, --flag type description
	// or: -f, --flag description (no type)
	flagRe := regexp.MustCompile(`^\s*(-([a-zA-Z]),\s+)?--([a-zA-Z0-9-]+)(?:\s+([a-zA-Z]+))?\s+(.*)$`)
	equalsRe := regexp.MustCompile(`^\s*(-([a-zA-Z]),\s+)?--([a-zA-Z0-9-]+)=([^\s]+)\s+(.*)$`)

	for _, line := range lines {
		// Try equals format first (--flag=value)
		if matches := equalsRe.FindStringSubmatch(line); len(matches) >= 5 {
			flags = append(flags, Flag{
				Short:       matches[2],
				Name:        matches[3] + "=" + matches[4],
				Description: strings.TrimSpace(matches[5]),
			})
			continue
		}

		// Try standard format
		if matches := flagRe.FindStringSubmatch(line); len(matches) >= 6 {
			// Extract default value before trimming
			rawDescription := strings.TrimSpace(matches[5])
			var defaultValue string
			if idx := strings.Index(rawDescription, "(default"); idx > 0 {
				// Extract default value from format: (default $HOME/.hauler) or (default: "value")
				defaultPart := rawDescription[idx:]
				if re := regexp.MustCompile(`\(default[^:]*:\s*([^\)]+)\)`); re.MatchString(defaultPart) {
					defaultMatches := re.FindStringSubmatch(defaultPart)
					if len(defaultMatches) >= 2 {
						defaultValue = defaultMatches[1]
					}
				}
				// Trim description to remove the default part
				rawDescription = strings.TrimSpace(rawDescription[:idx])
			}

			flag := Flag{
				Short:       matches[2],
				Name:        matches[3],
				Description: rawDescription,
				Default:     defaultValue,
			}
			// If there's a type (group 4), append to name for clarity
			if len(matches) > 4 && matches[4] != "" {
				flag.Name = flag.Name + " " + matches[4]
			}
			flags = append(flags, flag)
		}
	}

	return flags
}

// extractSubcommands parses subcommands from global help text
func (d *Detector) extractSubcommands(helpText string) []Subcommand {
	var subcommands []Subcommand

	// Look for "Available Commands:" or similar section
	lines := strings.Split(helpText, "\n")
	inCommandsSection := false

	// Markers that indicate the commands section
	sectionMarkers := []string{
		"Available Commands:",
		"Commands:",
		"Subcommands:",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're entering the commands section
		for _, marker := range sectionMarkers {
			if strings.Contains(trimmed, marker) {
				inCommandsSection = true
				break
			}
		}

		// Exit the section on empty line or new section
		if inCommandsSection && trimmed == "" {
			break
		}

		// Exit if we hit a flag section
		if inCommandsSection && strings.HasPrefix(trimmed, "-") {
			break
		}

		// Parse subcommand lines like:
		// "  store    Store related operations"
		// "  store, s  Store related operations"
		if inCommandsSection {
			// Skip section header and flags
			if strings.HasPrefix(trimmed, "Available") ||
				strings.HasPrefix(trimmed, "Commands") ||
				strings.HasPrefix(trimmed, "Subcommands") ||
				strings.HasPrefix(trimmed, "-") {
				continue
			}

			// Parse command name and description
			// Split by whitespace but limit to 2 parts (name, description)
			firstSpace := strings.Index(trimmed, " ")
			var name, description string
			if firstSpace > 0 {
				name = trimmed[:firstSpace]
				description = strings.TrimSpace(trimmed[firstSpace+1:])
			} else {
				name = trimmed
			}

			if name != "" {
				// Handle aliases like "store, s" or "store, s (alias)"
				// Extract the primary name (first part before comma)
				if commaIdx := strings.Index(name, ","); commaIdx > 0 {
					name = strings.TrimSpace(name[:commaIdx])
				}

				// If description starts with something that looks like an alias (short word followed by spaces and more text),
				// try to skip it to get to the actual description
				// e.g., "s    Store operations" -> "Store operations"
				if description != "" {
					// Split by multiple spaces to find where the real description starts
					parts := strings.Split(description, "    ") // 4 spaces as separator
					if len(parts) > 1 {
						// First part is likely an alias, rest is description
						description = strings.TrimSpace(strings.Join(parts[1:], " "))
					}

					// Also check for (alias in description
					if idx := strings.Index(description, "(alias"); idx > 0 {
						description = strings.TrimSpace(description[:idx])
					}
				}

				if name != "" {
					subcommands = append(subcommands, Subcommand{
						Name:        name,
						Description: description,
					})
				}
			}
		}
	}

	return subcommands
}
