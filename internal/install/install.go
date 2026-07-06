// Package install patches Claude Code's settings.json to point statusLine
// at the compiled skein binary.
package install

import (
	"encoding/json"
	"fmt"
	"os"
)

func PatchStatusLine(settings []byte, binPath string) ([]byte, error) {
	doc := map[string]any{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &doc); err != nil {
			return nil, fmt.Errorf("parse settings.json: %w", err)
		}
	}

	doc["statusLine"] = map[string]any{
		"type":    "command",
		"command": binPath,
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// Apply reads settings.json from path, patches its statusLine to binPath,
// and writes it back. If the file doesn't exist, a new one is created.
func Apply(path, binPath string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	patched, err := PatchStatusLine(data, binPath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, patched, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
