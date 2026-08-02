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

func TestLoadResolvesRepoRootRelativeToConfigFile(t *testing.T) {
	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "apps", "web")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(appDir, "buildybud.toml")
	content := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets/embed/assets"
manifest_path = "assets/embed/assets/manifest.json"
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(appDir, "assets", "embed", "assets")
	if got := cfg.RepoPath(cfg.Paths.AssetsRoot); got != want {
		t.Fatalf("RepoPath() = %q, want %q", got, want)
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
		`minify = true`,
		`scan_template_dirs = ["ui", "feature"]`,
		`inline_dirs = ["ui", "feature"]`,
		`prefix = "/blog"`,
		`prefix = "/projects"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated config missing %q:\n%s", want, text)
		}
	}
}

func TestDiscoverDisablesJavaScriptWhenRepoHasNoInputs(t *testing.T) {
	cfg, err := Discover(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JS.Enabled || cfg.JS.Audit.Enabled {
		t.Fatalf("JS config = %#v, want build and audit disabled", cfg.JS)
	}
}

func TestLoadSupportsVerbatimJavaScriptAndAuditGroups(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	content := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[js]
enabled = true
minify = false
out_dir = "assets/js"
hash_length = 8

[js.audit]
enabled = true
inline_dirs = ["ui"]
max_inline_raw_bytes = 2048

[[js.audit.group]]
name = "global"
assets = ["js/app.js"]
gzip_budget_bytes = 24576
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JS.Minify {
		t.Fatal("js.minify = true, want false")
	}
	if len(cfg.JS.Audit.Groups) != 1 || cfg.JS.Audit.Groups[0].GzipBudgetBytes != 24576 {
		t.Fatalf("audit groups = %#v", cfg.JS.Audit.Groups)
	}
}

func TestInitDefaultConfigPathUsesRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, "assets", "src", "js"), 0o755); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	if err := Init(repoRoot, DefaultPath, false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, DefaultPath)); err != nil {
		t.Fatalf("expected config under repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, DefaultPath)); !os.IsNotExist(err) {
		t.Fatalf("expected no config in caller cwd, got err=%v", err)
	}
}
