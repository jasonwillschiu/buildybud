package images

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneDeletedSourceOutputsRemovesVariantsAndCache(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "assets", "src", "images")
	outputDir := filepath.Join(root, "assets", "embed", "assets", "images")
	liveSource := filepath.Join(sourceDir, "live.png")
	liveVariant := filepath.Join(outputDir, "live-1200x.livehash.avif")
	staleVariant := filepath.Join(outputDir, "stale-1200x.stalehash.avif")
	writeTestFile(t, liveSource, "live")
	writeTestFile(t, liveVariant, "live-avif")
	writeTestFile(t, staleVariant, "stale-avif")

	previous := &ImageManifest{Images: map[string]*ProcessedImage{
		"live": {
			OriginalPath: liveSource,
			Variants: map[string]map[int]string{
				"avif": {1200: liveVariant},
			},
			Hash: "livehash",
		},
		"stale": {
			OriginalPath: filepath.Join(sourceDir, "stale.png"),
			Variants: map[string]map[int]string{
				"avif": {1200: staleVariant},
			},
			Hash: "stalehash",
		},
	}}
	cache := &ImageCache{Entries: map[string]CacheEntry{
		filepath.ToSlash(liveVariant): {
			SourceHash: "livehash",
			OutputPath: filepath.ToSlash(liveVariant),
			Timestamp:  1,
		},
		filepath.ToSlash(staleVariant): {
			SourceHash: "stalehash",
			OutputPath: filepath.ToSlash(staleVariant),
			Timestamp:  1,
		},
	}}

	removedFiles, removedCacheEntries, err := pruneDeletedSourceOutputs(root, sourceDir, outputDir, previous, cache, false)
	if err != nil {
		t.Fatal(err)
	}
	if removedFiles != 1 || removedCacheEntries != 1 {
		t.Fatalf("removed files/cache = %d/%d, want 1/1", removedFiles, removedCacheEntries)
	}
	if _, err := os.Stat(staleVariant); !os.IsNotExist(err) {
		t.Fatalf("stale variant still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(liveVariant); err != nil {
		t.Fatalf("live variant removed: %v", err)
	}
	if _, ok := cache.Entries[filepath.ToSlash(staleVariant)]; ok {
		t.Fatalf("stale cache entry kept: %#v", cache.Entries)
	}
	if _, ok := cache.Entries[filepath.ToSlash(liveVariant)]; !ok {
		t.Fatalf("live cache entry removed: %#v", cache.Entries)
	}
}

func TestPruneDeletedSourceOutputsIgnoresPathsOutsideConfiguredDirs(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "assets", "src", "images")
	outputDir := filepath.Join(root, "assets", "embed", "assets", "images")
	outsideOutput := filepath.Join(root, "elsewhere", "stale.avif")
	writeTestFile(t, outsideOutput, "do-not-touch")

	previous := &ImageManifest{Images: map[string]*ProcessedImage{
		"old-config": {
			OriginalPath: filepath.Join(root, "old-assets", "src", "images", "gone.png"),
			Variants: map[string]map[int]string{
				"avif": {1200: outsideOutput},
			},
			Hash: "oldhash",
		},
	}}

	removedFiles, removedCacheEntries, err := pruneDeletedSourceOutputs(root, sourceDir, outputDir, previous, &ImageCache{Entries: map[string]CacheEntry{}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if removedFiles != 0 || removedCacheEntries != 0 {
		t.Fatalf("removed files/cache = %d/%d, want 0/0", removedFiles, removedCacheEntries)
	}
	if _, err := os.Stat(outsideOutput); err != nil {
		t.Fatalf("outside output removed: %v", err)
	}
}

func TestPruneStaleOutputsRemovesDeletedSourceVariantsAfterFreshManifest(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "assets", "embed", "assets", "images")
	liveVariant := filepath.Join(outputDir, "live-1200x.livehash.avif")
	staleVariant := filepath.Join(outputDir, "stale-1200x.stalehash.avif")
	writeTestFile(t, liveVariant, "live-avif")
	writeTestFile(t, staleVariant, "stale-avif")
	writeTestFile(t, filepath.Join(outputDir, "manifest.json"), "{}")
	writeTestFile(t, filepath.Join(outputDir, "cache.json"), "{}")

	manifest := &ImageManifest{Images: map[string]*ProcessedImage{
		"live": {
			OriginalPath: filepath.Join(root, "assets", "src", "images", "live.png"),
			Variants: map[string]map[int]string{
				"avif": {1200: liveVariant},
			},
			Hash: "livehash",
		},
	}}
	cache := &ImageCache{Entries: map[string]CacheEntry{
		filepath.ToSlash(liveVariant): {
			SourceHash: "livehash",
			OutputPath: filepath.ToSlash(liveVariant),
			Timestamp:  1,
		},
		filepath.ToSlash(staleVariant): {
			SourceHash: "stalehash",
			OutputPath: filepath.ToSlash(staleVariant),
			Timestamp:  1,
		},
	}}

	removedFiles, removedCacheEntries, err := pruneStaleOutputs(root, outputDir, manifest, cache, false)
	if err != nil {
		t.Fatal(err)
	}
	if removedFiles != 1 || removedCacheEntries != 1 {
		t.Fatalf("removed files/cache = %d/%d, want 1/1", removedFiles, removedCacheEntries)
	}
	if _, err := os.Stat(staleVariant); !os.IsNotExist(err) {
		t.Fatalf("stale variant still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(liveVariant); err != nil {
		t.Fatalf("live variant removed: %v", err)
	}
}

func TestCachedImagePathsStayRelativeToRepoRoot(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "assets", "src", "images", "photo.png")
	writeTestFile(t, sourcePath, "source")
	sourceHash, err := fileShortMD5(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	outputRecord := filepath.ToSlash(filepath.Join("assets", "embed", "assets", "images", "photo-480x."+sourceHash+".avif"))
	outputPath := filepath.Join(root, filepath.FromSlash(outputRecord))
	writeTestFile(t, outputPath, "variant")

	cfg, err := newRunConfig(&Config{
		SourceDir: "assets/src/images",
		OutputDir: "assets/embed/assets/images",
		Formats:   []string{"avif"},
		Sizes:     []int{480},
		Quality:   map[string]int{"avif": 60},
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	cache := &ImageCache{Entries: map[string]CacheEntry{
		outputRecord: {SourceHash: sourceHash, OutputPath: outputRecord, Timestamp: 1},
	}}

	processed, cached, err := processOneWithCache(sourcePath, cfg, cache, sourceHash, false)
	if err != nil {
		t.Fatal(err)
	}
	if !cached {
		t.Fatal("expected cached image")
	}
	if processed.OriginalPath != "assets/src/images/photo.png" {
		t.Fatalf("original path = %q", processed.OriginalPath)
	}
	if got := processed.Variants["avif"][480]; got != outputRecord {
		t.Fatalf("variant path = %q, want %q", got, outputRecord)
	}
}

func writeTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
