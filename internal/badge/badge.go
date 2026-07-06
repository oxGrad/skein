// Package badge runs the caveman plugin's statusline hook script, if the
// plugin is enabled in settings.json.
package badge

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// pluginKey is the enabledPlugins key Claude Code uses for the caveman
// plugin in settings.json.
const pluginKey = "caveman@caveman"

// GlobPattern locates caveman-statusline.sh under a plugin cache dir. It's
// versioned by a hash directory, hence the wildcard.
func GlobPattern(claudeHome string) string {
	return filepath.Join(claudeHome, "plugins", "cache", "caveman", "caveman", "*", "hooks", "caveman-statusline.sh")
}

// Enabled reports whether settings.json marks the caveman plugin as enabled.
// A missing settings file, missing key, or malformed JSON all count as
// disabled, since the plugin cache and hook flag files can outlive an
// uninstall or disable and shouldn't cause the badge to reappear.
func Enabled(claudeHome string) bool {
	raw, err := os.ReadFile(filepath.Join(claudeHome, "settings.json"))
	if err != nil {
		return false
	}
	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return false
	}
	return settings.EnabledPlugins[pluginKey]
}

// FindScript returns the first script matching the glob pattern, or "" if
// none exists. Errors from glob matching (only possible for a malformed
// pattern) are treated as no match.
func FindScript(claudeHome string) string {
	matches, err := filepath.Glob(GlobPattern(claudeHome))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// Runner executes a script and returns its trimmed stdout.
type Runner func(script string) (string, error)

// ExecRunner runs a script with bash and captures stdout.
func ExecRunner(script string) (string, error) {
	out, err := exec.Command("bash", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Render finds and runs the caveman statusline hook, returning "" if the
// plugin isn't enabled in settings.json, the script is absent, or it fails
// (matching the shell script's silent-skip behavior).
func Render(claudeHome string, run Runner) string {
	if !Enabled(claudeHome) {
		return ""
	}
	script := FindScript(claudeHome)
	if script == "" {
		return ""
	}
	out, err := run(script)
	if err != nil {
		return ""
	}
	return out
}
