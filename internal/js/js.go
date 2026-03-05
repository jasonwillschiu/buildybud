package js

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	minify "github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

type Options struct {
	OutDir              string
	AssetsRoot          string
	ManifestPath        string
	HashLength          int
	SrcDirs             []string
	CopyDirs            []string
	ScanTemplateDirs    []string
	TempluiComponentDir string
	Dependencies        map[string][]string
}

type asset struct {
	rel     string
	logical string
	src     string
}

func Run(opts Options) error {
	assets, err := collectAssets(opts.SrcDirs)
	if err != nil {
		return err
	}
	if opts.HashLength < 1 || opts.HashLength > 64 {
		return fmt.Errorf("invalid hash length %d", opts.HashLength)
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.MkdirAll(opts.AssetsRoot, 0o755); err != nil {
		return fmt.Errorf("ensure assets root: %w", err)
	}

	manifest, err := readManifest(opts.ManifestPath)
	if err != nil {
		return err
	}

	processed := make(map[string]string, len(assets))
	m := minify.New()
	m.AddFunc("text/javascript", js.Minify)

	if len(assets) > 0 {
		keys := make([]string, 0, len(assets))
		for rel := range assets {
			keys = append(keys, rel)
		}
		sort.Strings(keys)

		for _, rel := range keys {
			item := assets[rel]
			content, err := os.ReadFile(item.src)
			if err != nil {
				return fmt.Errorf("read %s: %w", item.src, err)
			}

			var buf bytes.Buffer
			if err := m.Minify("text/javascript", &buf, bytes.NewReader(content)); err != nil {
				return fmt.Errorf("minify %s: %w", item.src, err)
			}
			minified := buf.Bytes()

			hashedRel, err := writeHashed(opts.OutDir, opts.AssetsRoot, item.rel, minified, opts.HashLength)
			if err != nil {
				return fmt.Errorf("hash %s: %w", item.src, err)
			}

			plainPath := filepath.Join(opts.OutDir, filepath.FromSlash(item.rel))
			if err := os.Remove(plainPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove plain asset %s: %w", plainPath, err)
			}

			if previous, ok := manifest[item.logical]; ok && previous != hashedRel {
				previousPath := filepath.Join(opts.AssetsRoot, filepath.FromSlash(previous))
				_ = os.Remove(previousPath)
			}

			processed[item.logical] = hashedRel
			manifest[item.logical] = hashedRel
		}
	}

	for logical, hashed := range manifest {
		if !strings.HasPrefix(logical, "js/") {
			continue
		}
		if _, ok := processed[logical]; !ok {
			hashedPath := filepath.Join(opts.AssetsRoot, filepath.FromSlash(hashed))
			_ = os.Remove(hashedPath)
			delete(manifest, logical)
		}
	}

	var usedComponents map[string]bool
	if len(opts.ScanTemplateDirs) > 0 {
		usedComponents, err = scanTempluiUsage(opts.ScanTemplateDirs, opts.TempluiComponentDir, opts.Dependencies)
		if err != nil {
			return fmt.Errorf("scan templates: %w", err)
		}
		fmt.Printf("Detected templui components in use: %v\n", sortedKeys(usedComponents))
	}

	if err := copyVerbatim(opts.CopyDirs, opts.OutDir, usedComponents); err != nil {
		return err
	}

	return writeManifest(opts.ManifestPath, manifest)
}

func collectAssets(srcDirs []string) (map[string]asset, error) {
	assets := map[string]asset{}
	for _, dir := range srcDirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source %s is not a directory", dir)
		}

		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".js" {
				return nil
			}

			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if rel == "" {
				return nil
			}

			logical := filepath.ToSlash(filepath.Join("js", rel))
			if existing, ok := assets[rel]; ok {
				return fmt.Errorf("duplicate asset %q (existing: %s, new: %s)", rel, existing.src, path)
			}

			assets[rel] = asset{rel: rel, logical: logical, src: path}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, err)
		}
	}
	return assets, nil
}

func writeHashed(outDir, assetsRoot, rel string, content []byte, hashLength int) (string, error) {
	sum := sha256.Sum256(content)
	hashHex := hex.EncodeToString(sum[:])
	if hashLength > len(hashHex) {
		return "", fmt.Errorf("hash length %d exceeds digest length", hashLength)
	}

	ext := filepath.Ext(rel)
	base := strings.TrimSuffix(filepath.Base(rel), ext)
	hashedName := fmt.Sprintf("%s.%s%s", base, hashHex[:hashLength], ext)

	relDir := filepath.Dir(rel)
	hashedRel := filepath.ToSlash(filepath.Join(relDir, hashedName))
	hashedPath := filepath.Join(outDir, filepath.FromSlash(hashedRel))

	if err := os.MkdirAll(filepath.Dir(hashedPath), 0o755); err != nil {
		return "", fmt.Errorf("ensure hashed dir: %w", err)
	}
	if err := os.WriteFile(hashedPath, content, 0o644); err != nil {
		return "", fmt.Errorf("write hashed asset: %w", err)
	}

	absHashedPath, err := filepath.Abs(hashedPath)
	if err != nil {
		return "", fmt.Errorf("resolve hashed abs path: %w", err)
	}
	absAssetsRoot, err := filepath.Abs(assetsRoot)
	if err != nil {
		return "", fmt.Errorf("resolve assets root: %w", err)
	}
	relToRoot, err := filepath.Rel(absAssetsRoot, absHashedPath)
	if err != nil {
		return "", fmt.Errorf("rel path from assets root: %w", err)
	}

	return filepath.ToSlash(relToRoot), nil
}

func readManifest(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if len(data) == 0 {
		return map[string]string{}, nil
	}

	manifest := map[string]string{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return manifest, nil
}

func writeManifest(path string, manifest map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure manifest dir: %w", err)
	}

	keys := make([]string, 0, len(manifest))
	for k := range manifest {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(map[string]string, len(manifest))
	for _, k := range keys {
		ordered[k] = manifest[k]
	}

	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func scanTempluiUsage(dirs []string, componentDir string, dependencies map[string][]string) (map[string]bool, error) {
	used := map[string]bool{}

	componentNames, err := getAvailableTempluiComponents(componentDir)
	if err != nil {
		return nil, fmt.Errorf("get available components: %w", err)
	}
	if len(componentNames) == 0 {
		return used, nil
	}

	detectors := buildTempluiDetectors(componentNames)

	var markUsed func(string)
	markUsed = func(name string) {
		if used[name] {
			return
		}
		used[name] = true
		for _, dep := range dependencies[name] {
			markUsed(dep)
		}
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("scan directory %s is not a directory", dir)
		}

		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".templ" {
				return nil
			}

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}

			for _, detector := range detectors {
				if used[detector.name] {
					continue
				}
				if detector.matches(content) {
					markUsed(detector.name)
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, err)
		}
	}

	return used, nil
}

type templuiDetector struct {
	name             string
	scriptPattern    *regexp.Regexp
	componentPattern *regexp.Regexp
	markers          [][]byte
}

func (t templuiDetector) matches(content []byte) bool {
	if t.scriptPattern.Match(content) || t.componentPattern.Match(content) {
		return true
	}
	for _, marker := range t.markers {
		if bytes.Contains(content, marker) {
			return true
		}
	}
	return false
}

func buildTempluiDetectors(componentNames []string) []templuiDetector {
	detectors := make([]templuiDetector, 0, len(componentNames))
	for _, name := range componentNames {
		detectors = append(detectors, templuiDetector{
			name:             name,
			scriptPattern:    regexp.MustCompile(fmt.Sprintf(`@%s\.Script\(\)`, regexp.QuoteMeta(name))),
			componentPattern: regexp.MustCompile(fmt.Sprintf(`@%s\.`, regexp.QuoteMeta(name))),
			markers:          templuiMarkersFor(name),
		})
	}
	return detectors
}

func templuiMarkersFor(component string) [][]byte {
	markers := [][]byte{[]byte("data-tui-" + component)}
	switch component {
	case "dialog":
		markers = append(markers, []byte("data-tui-sheet"))
	case "popover":
		markers = append(markers, []byte("data-tui-dropdown"), []byte("data-tui-selectbox"))
	case "dropdown", "selectbox":
		markers = append(markers, []byte("data-tui-popover"))
	}
	return markers
}

func getAvailableTempluiComponents(jsDir string) ([]string, error) {
	components := []string{}
	entries, err := os.ReadDir(jsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return components, nil
		}
		return nil, fmt.Errorf("read templui js dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		componentName := extractComponentName(entry.Name())
		if componentName != "" {
			components = append(components, componentName)
		}
	}
	sort.Strings(components)
	return components, nil
}

func extractComponentName(filename string) string {
	name := strings.TrimSuffix(filename, ".js")
	name = strings.TrimSuffix(name, ".min")
	return name
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func copyVerbatim(dirs []string, outDir string, usedComponents map[string]bool) error {
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("source %s is not a directory", dir)
		}

		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".js" {
				return nil
			}

			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if rel == "" {
				return nil
			}

			if usedComponents != nil {
				componentName := extractComponentName(filepath.Base(path))
				if componentName != "" && !usedComponents[componentName] {
					fmt.Printf("Skipping unused component JS: %s\n", rel)
					return nil
				}
			}

			destPath := filepath.Join(outDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return fmt.Errorf("ensure copy dir for %s: %w", destPath, err)
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read %s: %w", path, readErr)
			}
			if writeErr := os.WriteFile(destPath, data, 0o644); writeErr != nil {
				return fmt.Errorf("write %s: %w", destPath, writeErr)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("copy assets from %s: %w", dir, err)
		}
	}
	return nil
}
