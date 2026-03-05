package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsUnknownKeysInStrictMode(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	content := `
schema_version = 1
strict = true
unknown_key = "nope"

[paths]
repo_root = "."
assets_root = "assets/embed/assets"
manifest_path = "assets/embed/assets/manifest.json"
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "unknown config keys") {
		t.Fatalf("expected unknown config key error, got %v", err)
	}
}
