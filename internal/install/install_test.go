package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchStatusLinePreservesOtherKeys(t *testing.T) {
	orig := []byte(`{
		"statusLine": {"type": "command", "command": "bash old.sh"},
		"theme": "dark-daltonized",
		"effortLevel": "medium"
	}`)

	out, err := PatchStatusLine(orig, "/usr/local/bin/skein")
	if err != nil {
		t.Fatalf("PatchStatusLine error: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	if doc["theme"] != "dark-daltonized" {
		t.Errorf("theme = %v, want preserved", doc["theme"])
	}
	if doc["effortLevel"] != "medium" {
		t.Errorf("effortLevel = %v, want preserved", doc["effortLevel"])
	}

	sl, ok := doc["statusLine"].(map[string]any)
	if !ok {
		t.Fatalf("statusLine not an object: %v", doc["statusLine"])
	}
	if sl["command"] != "/usr/local/bin/skein" {
		t.Errorf("statusLine.command = %v, want /usr/local/bin/skein", sl["command"])
	}
	if sl["type"] != "command" {
		t.Errorf("statusLine.type = %v, want command", sl["type"])
	}
}

func TestPatchStatusLineEmptyInput(t *testing.T) {
	out, err := PatchStatusLine(nil, "/bin/skein")
	if err != nil {
		t.Fatalf("PatchStatusLine error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	sl := doc["statusLine"].(map[string]any)
	if sl["command"] != "/bin/skein" {
		t.Errorf("statusLine.command = %v, want /bin/skein", sl["command"])
	}
}

func TestPatchStatusLineInvalidJSON(t *testing.T) {
	if _, err := PatchStatusLine([]byte("not json"), "/bin/skein"); err == nil {
		t.Error("expected error for invalid input JSON, got nil")
	}
}

func TestApplyCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := Apply(path, "/bin/skein"); err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	sl := doc["statusLine"].(map[string]any)
	if sl["command"] != "/bin/skein" {
		t.Errorf("statusLine.command = %v, want /bin/skein", sl["command"])
	}
}

func TestApplyPreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(`{"theme": "light", "statusLine": {"type": "command", "command": "old"}}`), 0o644)

	if err := Apply(path, "/bin/skein"); err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var doc map[string]any
	json.Unmarshal(data, &doc)
	if doc["theme"] != "light" {
		t.Errorf("theme = %v, want preserved as light", doc["theme"])
	}
	sl := doc["statusLine"].(map[string]any)
	if sl["command"] != "/bin/skein" {
		t.Errorf("statusLine.command = %v, want /bin/skein", sl["command"])
	}
}
