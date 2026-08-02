package js

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSupportsMinifiedAndVerbatimSources(t *testing.T) {
	source := []byte("// keep this comment\nfunction add(a, b) { return a + b; }\n")
	for _, tc := range []struct {
		name   string
		minify bool
	}{
		{name: "minified", minify: true},
		{name: "verbatim", minify: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			srcDir := filepath.Join(root, "src")
			assetsRoot := filepath.Join(root, "assets")
			outDir := filepath.Join(assetsRoot, "js")
			manifestPath := filepath.Join(assetsRoot, "manifest.json")
			if err := os.MkdirAll(srcDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(srcDir, "app.js"), source, 0o644); err != nil {
				t.Fatal(err)
			}

			if err := Run(Options{
				OutDir:       outDir,
				AssetsRoot:   assetsRoot,
				ManifestPath: manifestPath,
				HashLength:   8,
				Minify:       tc.minify,
				SrcDirs:      []string{srcDir},
			}); err != nil {
				t.Fatalf("Run: %v", err)
			}
			manifest, err := readManifest(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			built, err := os.ReadFile(filepath.Join(assetsRoot, filepath.FromSlash(manifest["js/app.js"])))
			if err != nil {
				t.Fatal(err)
			}
			if tc.minify {
				if strings.Contains(string(built), "keep this comment") || len(built) >= len(source) {
					t.Fatalf("expected minified output, got %q", string(built))
				}
			} else if string(built) != string(source) {
				t.Fatalf("verbatim output changed:\n%s", string(built))
			}
		})
	}
}

func TestRunAllowsNoJavaScriptInputs(t *testing.T) {
	root := t.TempDir()
	assetsRoot := filepath.Join(root, "assets")
	manifestPath := filepath.Join(assetsRoot, "manifest.json")
	if err := Run(Options{
		OutDir:       filepath.Join(assetsRoot, "js"),
		AssetsRoot:   assetsRoot,
		ManifestPath: manifestPath,
		HashLength:   8,
		Minify:       true,
	}); err != nil {
		t.Fatalf("Run with no inputs: %v", err)
	}
	manifest, err := readManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) != 0 {
		t.Fatalf("manifest = %#v, want no JavaScript assets", manifest)
	}
}
