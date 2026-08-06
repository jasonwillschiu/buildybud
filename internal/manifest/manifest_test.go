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

func TestRunBatchPublishesAllAssetsAndCleansOwnedNamespace(t *testing.T) {
	tmp := t.TempDir()
	assetsRoot := filepath.Join(tmp, "assets")
	inputDir := filepath.Join(tmp, "compiled")
	if err := os.MkdirAll(filepath.Join(assetsRoot, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldCSS := filepath.Join(assetsRoot, "css", "app.old.css")
	if err := os.WriteFile(oldCSS, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(assetsRoot, "manifest.json")
	initial := map[string]string{
		"css/app.css": "css/app.old.css",
		"js/app.js":   "js/app.hash.js",
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	coreInput := filepath.Join(inputDir, "core.css")
	homeInput := filepath.Join(inputDir, "home.css")
	if err := os.WriteFile(coreInput, []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homeInput, []byte(".home{display:block}"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = RunBatch(BatchOptions{
		Assets: []BatchAsset{
			{InputPath: coreInput, LogicalPath: "css/core.css"},
			{InputPath: homeInput, LogicalPath: "css/home.css"},
		},
		ManifestPath: manifestPath,
		AssetsRoot:   assetsRoot,
		HashLength:   8,
		OwnedPrefix:  "css/",
		CleanupStale: true,
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}

	data, err = os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, logical := range []string{"css/core.css", "css/home.css", "js/app.js"} {
		if got[logical] == "" {
			t.Fatalf("manifest missing %q: %#v", logical, got)
		}
	}
	if _, ok := got["css/app.css"]; ok {
		t.Fatalf("stale CSS manifest entry retained: %#v", got)
	}
	if _, err := os.Stat(oldCSS); !os.IsNotExist(err) {
		t.Fatalf("stale CSS hash still exists: %v", err)
	}
}

func TestRunBatchLeavesCurrentManifestUntouchedWhenInputIsMissing(t *testing.T) {
	tmp := t.TempDir()
	assetsRoot := filepath.Join(tmp, "assets")
	if err := os.MkdirAll(assetsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(assetsRoot, "manifest.json")
	original := []byte("{\"css/app.css\":\"css/app.old.css\"}\n")
	if err := os.WriteFile(manifestPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunBatch(BatchOptions{
		Assets:       []BatchAsset{{InputPath: filepath.Join(tmp, "missing.css"), LogicalPath: "css/core.css"}},
		ManifestPath: manifestPath,
		AssetsRoot:   assetsRoot,
		HashLength:   8,
		OwnedPrefix:  "css/",
		CleanupStale: true,
	})
	if err == nil {
		t.Fatal("expected missing input error")
	}
	got, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("manifest changed after failed batch:\n%s", got)
	}
}
