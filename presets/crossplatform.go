package presets

import (
	"regexp"
	"runtime"
)

// CrossPlatformCommand represents a command translation across different operating systems.
type CrossPlatformCommand struct {
	// Regex patterns to capture the command and its arguments.
	Patterns []*regexp.Regexp
	// OS-specific translations. Use $1, $2, etc., to place captured regex groups.
	Translations map[string]string
}

var crossPlatformCommands []CrossPlatformCommand

func init() {
	// --- Community contributions start here ---
	// Add cross-platform command mappings below. Use regex capture groups for arguments.
	crossPlatformCommands = []CrossPlatformCommand{
		{
			// Example: Clear screen across platforms
			Patterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^clear$`),
				regexp.MustCompile(`(?i)^cls$`),
			},
			Translations: map[string]string{
				"windows": "cls",
				"linux":   "clear",
				"darwin":  "clear",
			},
		},
		{
			// Example: Force delete a file/directory mapping arguments
			// Captures the target path in $1
			Patterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^rm\s+-rf\s+(.+)$`),
				regexp.MustCompile(`(?i)^rmdir\s+/s\s+/q\s+(.+)$`),
			},
			Translations: map[string]string{
				"windows": "rmdir /s /q $1", // Simple mapping for Windows CMD
				"linux":   "rm -rf $1",
				"darwin":  "rm -rf $1",
			},
		},
		{
			// Create a directory
			Patterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^(?:mkdir|md)\s+(.+)$`),
			},
			Translations: map[string]string{
				"windows": "mkdir $1",
				"linux":   "mkdir -p $1",
				"darwin":  "mkdir -p $1",
			},
		},
		{
			// Copy a file
			Patterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^(?:cp|copy)\s+([^\s]+)\s+([^\s]+)$`),
			},
			Translations: map[string]string{
				"windows": "copy $1 $2",
				"linux":   "cp $1 $2",
				"darwin":  "cp $1 $2",
			},
		},
		{
			// Move or rename a file
			Patterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^(?:mv|move|ren)\s+([^\s]+)\s+([^\s]+)$`),
			},
			Translations: map[string]string{
				"windows": "move $1 $2",
				"linux":   "mv $1 $2",
				"darwin":  "mv $1 $2",
			},
		},
		{
			// Display file content
			Patterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^(?:cat|type)\s+(.+)$`),
			},
			Translations: map[string]string{
				"windows": "type $1",
				"linux":   "cat $1",
				"darwin":  "cat $1",
			},
		},
		{
			// Show network interface configuration
			Patterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^ipconfig$`),
				regexp.MustCompile(`(?i)^ifconfig$`),
			},
			Translations: map[string]string{
				"windows": "ipconfig",
				"linux":   "ip addr", // `ifconfig` is often deprecated in favor of `ip`
				"darwin":  "ifconfig",
			},
		},
	}
}

// CheckForCrossPlatform iterates through mappings and returns a translated native command.
func CheckForCrossPlatform(input string) (string, bool) {
	currentOS := runtime.GOOS

	for _, cmd := range crossPlatformCommands {
		for _, pattern := range cmd.Patterns {
			if pattern.MatchString(input) {
				if translation, ok := cmd.Translations[currentOS]; ok {
					// Use ReplaceAllString to preserve regex capture groups (like arguments)
					resolved := pattern.ReplaceAllString(input, translation)
					return resolved, true
				}
				break
			}
		}
	}
	return "", false
}