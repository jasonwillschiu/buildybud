package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHashesAndUpdatesManifest(t *testing.T) {
	tmp := t.TempDir()
	assetsRoot := filepath.Join(tmp, "assets")
	inputDir := filepath.Join(assetsRoot, "css")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(inputDir, "app.css")
	if err := os.WriteFile(inputPath, []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(assetsRoot, "manifest.json")
	err := Run(Options{
		InputPath:    inputPath,
		LogicalPath:  "css/app.css",
		ManifestPath: manifestPath,
		AssetsRoot:   assetsRoot,
		HashLength:   8,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(inputPath); !os.IsNotExist(err) {
		t.Fatalf("expected unhashed input removed, got err=%v", err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := map[string]string{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	hashedRel, ok := manifest["css/app.css"]
	if !ok || hashedRel == "" {
		t.Fatalf("expected css/app.css entry in manifest")
	}
	if _, err := os.Stat(filepath.Join(assetsRoot, hashedRel)); err != nil {
		t.Fatalf("expected hashed file to exist: %v", err)
	}
}
