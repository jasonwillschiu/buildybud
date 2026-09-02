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

func TestLoadSupportsInlineImageConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	content := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "public/assets"
manifest_path = "public/assets/manifest.json"

[images]
enabled = true
source_dir = "content/images"
output_dir = "public/assets/images"
formats = ["jpeg", "avif"]
sizes = [480, 768, 1200]
quality = { jpeg = 80, avif = 60 }
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Images.ConfigPath != "" || cfg.Images.SourceDir != "content/images" || cfg.Images.Sizes[0] != 480 {
		t.Fatalf("images config = %#v", cfg.Images)
	}
	if cfg.Images.Quality["avif"] != 60 {
		t.Fatalf("images quality = %#v", cfg.Images.Quality)
	}
}

func TestLoadSupportsCloudflareCDNConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	content := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[cdn]
enabled = true
base_url = "https://example.com"
hosts = ["example.com", "www.example.com"]
static_paths = ["/", "/about", "/search?q=go"]
tag_prefix = "/tags"
redirects_file = "AppConstants.toml"
redirects_table = "redirects"

[[cdn.content]]
source_dir = "content/blog"
route_prefix = "/blog"
include_tags = true
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CDN.Enabled || cfg.CDN.BaseURL != "https://example.com" || len(cfg.CDN.Content) != 1 {
		t.Fatalf("cdn config = %#v", cfg.CDN)
	}
	if got := cfg.CDN.Content[0]; got.SourceDir != "content/blog" || got.RoutePrefix != "/blog" || !got.IncludeTags {
		t.Fatalf("cdn content config = %#v", got)
	}
}

func TestLoadRejectsInvalidCloudflareCDNConfig(t *testing.T) {
	tests := []struct {
		name   string
		cdn    string
		needle string
	}{
		{name: "relative base URL", cdn: `base_url = "/local"\nhosts = ["example.com"]`, needle: "absolute http or https"},
		{name: "host with scheme", cdn: `base_url = "https://example.com"\nhosts = ["https://example.com"]`, needle: "invalid cdn host"},
		{name: "base host omitted", cdn: `base_url = "https://example.com"\nhosts = ["www.example.com"]`, needle: "must be listed"},
		{name: "base URL path", cdn: `base_url = "https://example.com/app"\nhosts = ["example.com"]`, needle: "must not contain a path"},
		{name: "relative static path", cdn: `base_url = "https://example.com"\nhosts = ["example.com"]\nstatic_paths = ["about"]`, needle: "absolute URL paths"},
		{name: "partial redirects config", cdn: `base_url = "https://example.com"\nhosts = ["example.com"]\nredirects_file = "AppConstants.toml"`, needle: "must be set together"},
		{name: "no route sources", cdn: `base_url = "https://example.com"\nhosts = ["example.com"]`, needle: "at least one"},
		{name: "escaping content source", cdn: `base_url = "https://example.com"\nhosts = ["example.com"]\n[[cdn.content]]\nsource_dir = "../content"\nroute_prefix = "/blog"`, needle: "repo-relative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			cfgPath := filepath.Join(tmp, "buildybud.toml")
			content := "schema_version = 1\nstrict = true\n[paths]\nrepo_root = \".\"\nassets_root = \"assets\"\nmanifest_path = \"assets/manifest.json\"\n[cdn]\nenabled = true\n" + strings.ReplaceAll(tt.cdn, `\n`, "\n") + "\n"
			if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(cfgPath)
			if err == nil || !strings.Contains(err.Error(), tt.needle) {
				t.Fatalf("expected %q error, got %v", tt.needle, err)
			}
		})
	}
}

func TestLoadRejectsMixedImageConfigModes(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	content := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[images]
enabled = true
config_path = "imageopt.json"
sizes = [480, 768]
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected mixed image mode error, got %v", err)
	}
}

func TestLoadRejectsUnsafeInlineImageOutput(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	content := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[images]
enabled = true
source_dir = "images"
output_dir = "."
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "must be a child") {
		t.Fatalf("expected unsafe output error, got %v", err)
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
		`config_path = "tools/imageopt/config.json"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated config missing %q:\n%s", want, text)
		}
	}
}

func TestInitGeneratesInlineImageConfigWithoutLegacyJSON(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{"assets/embed/assets", "assets/src/images"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfgPath := filepath.Join(tmp, "buildybud.toml")
	if err := Init(tmp, cfgPath, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`source_dir = "assets/src/images"`,
		`output_dir = "assets/embed/assets/images"`,
		`formats = ["jpeg", "avif"]`,
		`sizes = [480, 768, 1200]`,
		`quality = { avif = 60, jpeg = 80 }`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "config_path") {
		t.Fatalf("generated inline config contains config_path:\n%s", text)
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

func TestLoadSupportsNamedCSSBundlesAndAuditGroups(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	content := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[css]
enabled = true
mode = "bundles"
command = "tailwindcss"
minify = true

[[css.bundle]]
name = "core"
input = "css/core.css"
output = "assets/css/core.css"

[[css.bundle]]
name = "home"
input = "css/home.css"
output = "assets/css/home.css"

[css.audit]
enabled = true

[[css.audit.group]]
name = "home-initial"
bundles = ["core", "home"]
gzip_budget_bytes = 24576
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CSS.Mode != "bundles" || len(cfg.CSS.Bundles) != 2 {
		t.Fatalf("CSS config = %#v", cfg.CSS)
	}
	if len(cfg.CSS.Audit.Groups) != 1 || cfg.CSS.Audit.Groups[0].Bundles[1] != "home" {
		t.Fatalf("CSS audit groups = %#v", cfg.CSS.Audit.Groups)
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
