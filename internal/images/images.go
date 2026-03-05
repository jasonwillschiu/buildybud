package images

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const variantHashLength = 8

type Options struct {
	ConfigPath string
	Verbose    bool
}

type vipsOutput struct {
	path    string
	width   int
	format  string
	quality int
}

type CacheEntry struct {
	SourceHash string `json:"sourceHash"`
	OutputPath string `json:"outputPath"`
	Timestamp  int64  `json:"timestamp"`
}

type ImageCache struct {
	Entries map[string]CacheEntry `json:"entries"`
}

func Run(opts Options) error {
	if _, err := exec.LookPath("vips"); err != nil {
		return fmt.Errorf("vips CLI not found in PATH. Please install libvips-tools / vips")
	}

	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if opts.Verbose {
		fmt.Printf("Loaded config: %#v\n", *cfg)
	}

	files, err := findImageFiles(cfg.SourceDir)
	if err != nil {
		return fmt.Errorf("failed to enumerate images: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No source images found in", cfg.SourceDir, "- cleaning stale generated images")
	}
	sort.Strings(files)

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	manifest := NewImageManifest()
	manifestPath := filepath.Join(cfg.OutputDir, "manifest.json")

	cachePath := filepath.Join(cfg.OutputDir, "cache.json")
	cache := loadImageCache(cachePath)
	if opts.Verbose {
		fmt.Printf("Loaded cache with %d entries\n", len(cache.Entries))
	}

	var processedCount, failedCount, cachedCount int
	for _, file := range files {
		sourceHash, err := fileShortMD5(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error computing hash for %s: %v\n", file, err)
			failedCount++
			continue
		}
		proc, cached, err := processOneWithCache(file, cfg, cache, sourceHash, opts.Verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", file, err)
			failedCount++
			continue
		}
		manifest.AddImage(proc)
		if cached {
			cachedCount++
		} else {
			processedCount++
		}
	}

	removedFiles, removedCacheEntries, err := pruneStaleOutputs(cfg.OutputDir, manifest, cache, opts.Verbose)
	if err != nil {
		return fmt.Errorf("failed to prune stale image outputs: %w", err)
	}
	if err := saveManifestAtomic(manifest, manifestPath); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}
	if err := saveImageCache(cache, cachePath); err != nil {
		return fmt.Errorf("failed to save cache: %w", err)
	}

	fmt.Printf("Image optimization complete. Processed=%d Cached=%d Failed=%d Removed=%d CacheRemoved=%d Manifest=%s\n",
		processedCount,
		cachedCount,
		failedCount,
		removedFiles,
		removedCacheEntries,
		filepath.ToSlash(manifestPath),
	)
	return nil
}

func processOne(src string, cfg *Config, verbose bool) (*ProcessedImage, error) {
	shortHash, err := fileShortMD5(src)
	if err != nil {
		return nil, fmt.Errorf("hash: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	processed := &ProcessedImage{
		OriginalPath: filepath.ToSlash(src),
		Variants:     make(map[string]map[int]string),
		Hash:         shortHash,
	}

	var outputs []vipsOutput
	for _, fmtName := range cfg.Formats {
		if _, ok := processed.Variants[fmtName]; !ok {
			processed.Variants[fmtName] = make(map[int]string)
		}
		for _, w := range cfg.Sizes {
			out := filepath.Join(cfg.OutputDir, fmt.Sprintf("%s-%dx.%s.%s", base, w, shortHash, fmtName))
			outputs = append(outputs, vipsOutput{path: out, width: w, format: fmtName, quality: cfg.Quality[fmtName]})
			processed.Variants[fmtName][w] = filepath.ToSlash(out)
		}
	}

	if err := runVipsCommands(src, outputs, verbose); err != nil {
		return nil, fmt.Errorf("vips processing: %w", err)
	}
	for _, output := range outputs {
		if err := ensureNonZero(output.path); err != nil {
			return nil, fmt.Errorf("output validation: %w", err)
		}
	}

	return processed, nil
}

func runVipsCommands(input string, outputs []vipsOutput, verbose bool) error {
	if len(outputs) == 0 {
		return fmt.Errorf("no outputs specified")
	}
	for _, output := range outputs {
		if err := os.MkdirAll(filepath.Dir(output.path), 0o755); err != nil {
			return err
		}
	}

	for _, output := range outputs {
		quality := output.quality
		if quality <= 0 {
			switch strings.ToLower(output.format) {
			case "jpeg", "jpg":
				quality = 80
			case "avif":
				quality = 60
			default:
				quality = 80
			}
		}

		outWithOpts := output.path
		switch strings.ToLower(output.format) {
		case "jpeg", "jpg":
			outWithOpts = fmt.Sprintf("%s[Q=%d,strip]", output.path, quality)
		case "avif":
			outWithOpts = fmt.Sprintf("%s[Q=%d,effort=4,strip]", output.path, quality)
		case "png":
			outWithOpts = fmt.Sprintf("%s[compression=9,strip]", output.path)
		}

		args := []string{"thumbnail", input, outWithOpts, fmt.Sprintf("%d", output.width)}
		if verbose {
			fmt.Println("vips", strings.Join(args, " "))
		}
		cmd := exec.Command("vips", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("vips command failed for %s: %w", output.path, err)
		}
	}
	return nil
}

func fileShortMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	hexsum := hex.EncodeToString(h.Sum(nil))
	if len(hexsum) > variantHashLength {
		return hexsum[:variantHashLength], nil
	}
	return hexsum, nil
}

func ensureNonZero(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("output not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("output is zero bytes: %s", filepath.ToSlash(path))
	}
	return nil
}

func findImageFiles(dir string) ([]string, error) {
	var files []string
	supported := map[string]bool{`.jpg`: true, `.jpeg`: true, `.png`: true, `.webp`: true, `.gif`: true, `.tif`: true, `.tiff`: true}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if supported[strings.ToLower(filepath.Ext(path))] {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func saveManifestAtomic(im *ImageManifest, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(im, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadImageCache(path string) *ImageCache {
	cache := &ImageCache{Entries: make(map[string]CacheEntry)}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	if err := json.Unmarshal(data, cache); err != nil {
		return &ImageCache{Entries: make(map[string]CacheEntry)}
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]CacheEntry)
	}
	return cache
}

func saveImageCache(cache *ImageCache, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func processOneWithCache(src string, cfg *Config, cache *ImageCache, sourceHash string, verbose bool) (*ProcessedImage, bool, error) {
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))

	var expectedPaths []string
	for _, fmtName := range cfg.Formats {
		for _, w := range cfg.Sizes {
			outPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("%s-%dx.%s.%s", base, w, sourceHash, fmtName))
			expectedPaths = append(expectedPaths, filepath.ToSlash(outPath))
		}
	}

	allCachedAndValid := true
	for _, outPath := range expectedPaths {
		entry, exists := cache.Entries[outPath]
		if !exists {
			allCachedAndValid = false
			break
		}
		if entry.SourceHash != sourceHash {
			if len(entry.SourceHash) < variantHashLength || entry.SourceHash[:variantHashLength] != sourceHash {
				allCachedAndValid = false
				break
			}
		}
		if _, err := os.Stat(outPath); err != nil {
			allCachedAndValid = false
			break
		}
	}

	if allCachedAndValid {
		processed := &ProcessedImage{OriginalPath: filepath.ToSlash(src), Variants: make(map[string]map[int]string), Hash: sourceHash}
		for _, fmtName := range cfg.Formats {
			processed.Variants[fmtName] = make(map[int]string)
			for _, w := range cfg.Sizes {
				outPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("%s-%dx.%s.%s", base, w, sourceHash, fmtName))
				processed.Variants[fmtName][w] = filepath.ToSlash(outPath)
			}
		}
		if verbose {
			fmt.Printf("Using cached: %s\n", src)
		}
		return processed, true, nil
	}

	processed, err := processOne(src, cfg, verbose)
	if err != nil {
		return nil, false, err
	}
	now := time.Now().Unix()
	for _, widthMap := range processed.Variants {
		for _, outPath := range widthMap {
			cache.Entries[outPath] = CacheEntry{SourceHash: sourceHash, OutputPath: outPath, Timestamp: now}
		}
	}
	return processed, false, nil
}

func pruneStaleOutputs(outputDir string, manifest *ImageManifest, cache *ImageCache, verbose bool) (int, int, error) {
	keep := make(map[string]struct{})
	for _, processed := range manifest.Images {
		for _, sizeMap := range processed.Variants {
			for _, outputPath := range sizeMap {
				keep[filepath.ToSlash(outputPath)] = struct{}{}
			}
		}
	}

	manifestPath := filepath.ToSlash(filepath.Join(outputDir, "manifest.json"))
	cachePath := filepath.ToSlash(filepath.Join(outputDir, "cache.json"))

	removedFiles := 0
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		normalized := filepath.ToSlash(path)
		if normalized == manifestPath || normalized == cachePath {
			return nil
		}
		if _, ok := keep[normalized]; ok {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		removedFiles++
		if verbose {
			fmt.Printf("Removed stale image output: %s\n", normalized)
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	removedCacheEntries := 0
	for outputPath := range cache.Entries {
		normalized := filepath.ToSlash(outputPath)
		if _, ok := keep[normalized]; !ok {
			delete(cache.Entries, outputPath)
			removedCacheEntries++
			if verbose {
				fmt.Printf("Removed stale cache entry: %s\n", normalized)
			}
		}
	}

	return removedFiles, removedCacheEntries, nil
}
