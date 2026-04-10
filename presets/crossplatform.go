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