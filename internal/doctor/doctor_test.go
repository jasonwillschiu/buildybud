package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonwillschiu/buildybud/internal/config"
)

func TestRunAcceptsNativeCSSWithoutExternalCommand(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "assets", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "assets", "src", "site.css"), []byte("body { color: red; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(tmp, "buildybud.toml")
	configSource := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[css]
enabled = true
mode = "single"
command = "native"
input = "assets/src/site.css"
output = "assets/site.css"
minify = true

[js]
enabled = false

[images]
enabled = false

[favicon]
enabled = false

[templui_map]
enabled = false
`
	if err := os.WriteFile(configPath, []byte(configSource), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
