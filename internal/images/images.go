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
	Config     *Config
	RepoRoot   string
	Verbose    bool
}

type runConfig struct {
	repoRoot  string
	sourceDir string
	outputDir string
	formats   []string
	sizes     []int
	quality   map[string]int
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

	cfg := opts.Config
	if cfg == nil {
		loaded, err := LoadConfig(opts.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg = loaded
	}
	resolved, err := newRunConfig(cfg, opts.RepoRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve config: %w", err)
	}
	if opts.Verbose {
		fmt.Printf("Loaded config: %#v\n", *cfg)
	}

	files, err := findImageFiles(resolved.sourceDir)
	if err != nil {
		return fmt.Errorf("failed to enumerate images: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No source images found in", cfg.SourceDir, "- cleaning stale generated images")
	}
	sort.Strings(files)

	if err := os.MkdirAll(resolved.outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	manifest := NewImageManifest()
	manifestPath := filepath.Join(resolved.outputDir, "manifest.json")

	cachePath := filepath.Join(resolved.outputDir, "cache.json")
	cache := loadImageCache(cachePath)
	previousManifest := loadImageManifest(manifestPath)
	if opts.Verbose {
		fmt.Printf("Loaded cache with %d entries\n", len(cache.Entries))
	}

	deletedSourceFiles, deletedSourceCacheEntries, err := pruneDeletedSourceOutputs(resolved.repoRoot, resolved.sourceDir, resolved.outputDir, previousManifest, cache, opts.Verbose)
	if err != nil {
		return fmt.Errorf("failed to prune deleted source image outputs: %w", err)
	}

	var processedCount, failedCount, cachedCount int
	for _, file := range files {
		sourceHash, err := fileShortMD5(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error computing hash for %s: %v\n", file, err)
			failedCount++
			continue
		}
		proc, cached, err := processOneWithCache(file, resolved, cache, sourceHash, opts.Verbose)
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

	removedFiles, removedCacheEntries, err := pruneStaleOutputs(resolved.repoRoot, resolved.outputDir, manifest, cache, opts.Verbose)
	if err != nil {
		return fmt.Errorf("failed to prune stale image outputs: %w", err)
	}
	removedFiles += deletedSourceFiles
	removedCacheEntries += deletedSourceCacheEntries
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

func processOne(src string, cfg *runConfig, verbose bool) (*ProcessedImage, error) {
	shortHash, err := fileShortMD5(src)
	if err != nil {
		return nil, fmt.Errorf("hash: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	processed := &ProcessedImage{
		OriginalPath: cfg.recordPath(src),
		Variants:     make(map[string]map[int]string),
		Hash:         shortHash,
	}

	var outputs []vipsOutput
	for _, fmtName := range cfg.formats {
		if _, ok := processed.Variants[fmtName]; !ok {
			processed.Variants[fmtName] = make(map[int]string)
		}
		for _, w := range cfg.sizes {
			out := filepath.Join(cfg.outputDir, fmt.Sprintf("%s-%dx.%s.%s", base, w, shortHash, fmtName))
			outputs = append(outputs, vipsOutput{path: out, width: w, format: fmtName, quality: cfg.quality[fmtName]})
			processed.Variants[fmtName][w] = cfg.recordPath(out)
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

func loadImageManifest(path string) *ImageManifest {
	manifest := NewImageManifest()
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest
	}
	if err := json.Unmarshal(data, manifest); err != nil {
		return NewImageManifest()
	}
	if manifest.Images == nil {
		manifest.Images = make(map[string]*ProcessedImage)
	}
	return manifest
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

func processOneWithCache(src string, cfg *runConfig, cache *ImageCache, sourceHash string, verbose bool) (*ProcessedImage, bool, error) {
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))

	type expectedPath struct {
		record string
		disk   string
	}
	var expectedPaths []expectedPath
	for _, fmtName := range cfg.formats {
		for _, w := range cfg.sizes {
			diskPath := filepath.Join(cfg.outputDir, fmt.Sprintf("%s-%dx.%s.%s", base, w, sourceHash, fmtName))
			expectedPaths = append(expectedPaths, expectedPath{record: cfg.recordPath(diskPath), disk: diskPath})
		}
	}

	allCachedAndValid := true
	for _, outPath := range expectedPaths {
		entry, exists := cache.Entries[outPath.record]
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
		if _, err := os.Stat(outPath.disk); err != nil {
			allCachedAndValid = false
			break
		}
	}

	if allCachedAndValid {
		processed := &ProcessedImage{OriginalPath: cfg.recordPath(src), Variants: make(map[string]map[int]string), Hash: sourceHash}
		for _, fmtName := range cfg.formats {
			processed.Variants[fmtName] = make(map[int]string)
			for _, w := range cfg.sizes {
				outPath := filepath.Join(cfg.outputDir, fmt.Sprintf("%s-%dx.%s.%s", base, w, sourceHash, fmtName))
				processed.Variants[fmtName][w] = cfg.recordPath(outPath)
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

func pruneDeletedSourceOutputs(repoRoot, sourceDir, outputDir string, previous *ImageManifest, cache *ImageCache, verbose bool) (int, int, error) {
	if previous == nil || len(previous.Images) == 0 {
		return 0, 0, nil
	}

	removedFiles := 0
	removedCacheEntries := 0
	for _, processed := range previous.Images {
		if processed == nil || strings.TrimSpace(processed.OriginalPath) == "" {
			continue
		}
		sourcePath := resolveRecordPath(repoRoot, processed.OriginalPath)
		if !isPathUnder(sourcePath, sourceDir) {
			continue
		}
		if _, err := os.Stat(sourcePath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return 0, 0, fmt.Errorf("stat source image %s: %w", filepath.ToSlash(sourcePath), err)
		}

		for _, outputRecord := range processed.VariantPaths() {
			outputPath := resolveRecordPath(repoRoot, outputRecord)
			if !isPathUnder(outputPath, outputDir) {
				continue
			}
			if err := os.Remove(outputPath); err == nil {
				removedFiles++
				if verbose {
					fmt.Printf("Removed deleted-source image output: %s\n", filepath.ToSlash(outputPath))
				}
			} else if !os.IsNotExist(err) {
				return 0, 0, fmt.Errorf("remove deleted-source image output %s: %w", filepath.ToSlash(outputPath), err)
			}
			removed := deleteCacheEntry(cache, outputRecord, outputPath)
			removedCacheEntries += removed
			if verbose && removed > 0 {
				fmt.Printf("Removed stale cache entry: %s\n", filepath.ToSlash(outputRecord))
			}
		}
	}
	return removedFiles, removedCacheEntries, nil
}

func pruneStaleOutputs(repoRoot, outputDir string, manifest *ImageManifest, cache *ImageCache, verbose bool) (int, int, error) {
	keep := make(map[string]struct{})
	for _, processed := range manifest.Images {
		if processed == nil {
			continue
		}
		for _, outputPath := range processed.VariantPaths() {
			keep[filepath.ToSlash(resolveRecordPath(repoRoot, outputPath))] = struct{}{}
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
		normalized := filepath.ToSlash(resolveRecordPath(repoRoot, outputPath))
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

func deleteCacheEntry(cache *ImageCache, paths ...string) int {
	if cache == nil || cache.Entries == nil {
		return 0
	}
	removed := 0
	for _, path := range paths {
		for _, key := range []string{filepath.ToSlash(path), path} {
			if _, ok := cache.Entries[key]; !ok {
				continue
			}
			delete(cache.Entries, key)
			removed++
		}
	}
	return removed
}

func newRunConfig(cfg *Config, repoRoot string) (*runConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if repoRoot == "" {
		repoRoot = "."
	}
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	return &runConfig{
		repoRoot:  absRoot,
		sourceDir: resolveRecordPath(absRoot, cfg.SourceDir),
		outputDir: resolveRecordPath(absRoot, cfg.OutputDir),
		formats:   append([]string(nil), cfg.Formats...),
		sizes:     append([]int(nil), cfg.Sizes...),
		quality:   cloneQuality(cfg.Quality),
	}, nil
}

func (c *runConfig) recordPath(path string) string {
	rel, err := filepath.Rel(c.repoRoot, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func resolveRecordPath(repoRoot, path string) string {
	if filepath.IsAbs(path) || repoRoot == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(repoRoot, filepath.FromSlash(path)))
}

func cloneQuality(quality map[string]int) map[string]int {
	cloned := make(map[string]int, len(quality))
	for format, value := range quality {
		cloned[format] = value
	}
	return cloned
}

func isPathUnder(path, dir string) bool {
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
