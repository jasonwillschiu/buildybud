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

func TestLoadMissingConfigSuggestsInit(t *testing.T) {
	tmp := t.TempDir()
	_, err := Load(filepath.Join(tmp, "buildybud.toml"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "run `buildybud init`") {
		t.Fatalf("expected init hint, got %v", err)
	}
}

func TestInitGeneratesConfigFromRepoShape(t *testing.T) {
	tmp := t.TempDir()
	dirs := []string{
		"assets/embed/assets",
		"assets/src/js",
		"assets/src/templui/assets/js",
		"ui",
		"feature",
		"content/blog",
		"content/projects",
		"core/router",
		"tools/imageopt",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(tmp, "core/templui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module github.com/acme/example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "tools/imageopt/config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "buildybud.toml")
	if err := Init(tmp, cfgPath, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`module_path = "example"`,
		`assets_root = "assets/embed/assets"`,
		`scan_template_dirs = ["ui", "feature"]`,
		`prefix = "/blog"`,
		`prefix = "/projects"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated config missing %q:\n%s", want, text)
		}
	}
}
