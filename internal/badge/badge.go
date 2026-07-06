// Package badge runs the caveman plugin's statusline hook script, if present.
package badge

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// GlobPattern locates caveman-statusline.sh under a plugin cache dir. It's
// versioned by a hash directory, hence the wildcard.
func GlobPattern(claudeHome string) string {
	return filepath.Join(claudeHome, "plugins", "cache", "caveman", "caveman", "*", "hooks", "caveman-statusline.sh")
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

// Render finds and runs the caveman statusline hook, returning "" if it's
// absent or fails (matching the shell script's silent-skip behavior).
func Render(claudeHome string, run Runner) string {
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
