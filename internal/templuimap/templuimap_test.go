package templuimap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonwillschiu/buildybud/internal/config"
)

func TestGenerateWritesRouteScripts(t *testing.T) {
	tmp := t.TempDir()
	componentDir := filepath.Join(tmp, "assets", "src", "templui", "assets", "js")
	if err := os.MkdirAll(componentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(componentDir, "dialog.min.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(componentDir, "popover.min.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Paths.RepoRoot = tmp
	cfg.TempluiMap.ComponentDir = filepath.ToSlash(filepath.Join("assets", "src", "templui", "assets", "js"))
	cfg.TempluiMap.Out = filepath.ToSlash(filepath.Join("core", "templui", "generated_routes.go"))
	cfg.TempluiMap.DefaultComponents = []string{"dialog"}
	cfg.JS.Dependencies = map[string][]string{"dropdown": {"popover"}}
	cfg.TempluiMap.Rules = []config.TempluiRule{{Prefix: "/", Components: []string{"dialog"}}}

	if err := Generate(cfg); err != nil {
		t.Fatalf("generate: %v", err)
	}

	out := filepath.Join(tmp, "core", "templui", "generated_routes.go")
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatalf("expected generated file to be non-empty")
	}
}
