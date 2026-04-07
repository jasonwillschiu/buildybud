package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const DefaultPath = "buildybud.toml"

type Config struct {
	SchemaVersion int    `toml:"schema_version"`
	ModulePath    string `toml:"module_path"`
	Strict        bool   `toml:"strict"`

	Paths      PathsConfig      `toml:"paths"`
	JS         JSConfig         `toml:"js"`
	Manifest   ManifestConfig   `toml:"manifest"`
	Images     ImagesConfig     `toml:"images"`
	TempluiMap TempluiMapConfig `toml:"templui_map"`
}

type PathsConfig struct {
	RepoRoot     string `toml:"repo_root"`
	AssetsRoot   string `toml:"assets_root"`
	ManifestPath string `toml:"manifest_path"`
}

type JSConfig struct {
	OutDir              string    `toml:"out_dir"`
	HashLength          int       `toml:"hash_length"`
	SrcDirs             []string  `toml:"src_dirs"`
	CopyDirs            []string  `toml:"copy_dirs"`
	ScanTemplateDirs    []string  `toml:"scan_template_dirs"`
	TempluiComponentDir string    `toml:"templui_component_dir"`
	Dependencies        DepConfig `toml:"dependencies"`
}

type DepConfig map[string][]string

type ManifestConfig struct {
	HashLength   int  `toml:"hash_length"`
	CleanupStale bool `toml:"cleanup_stale"`
}

type ImagesConfig struct {
	ConfigPath string `toml:"config_path"`
}

type TempluiMapConfig struct {
	Mode                   string        `toml:"mode"`
	Out                    string        `toml:"out"`
	ComponentDir           string        `toml:"component_dir"`
	DefaultComponents      []string      `toml:"default_components"`
	LongestPrefixMatch     bool          `toml:"longest_prefix_match"`
	FailOnMissingComponent bool          `toml:"fail_on_missing_component"`
	Rules                  []TempluiRule `toml:"rule"`
	Suggest                SuggestConfig `toml:"suggest"`
}

type TempluiRule struct {
	Prefix     string   `toml:"prefix"`
	Components []string `toml:"components"`
}

type SuggestConfig struct {
	Enabled    bool     `toml:"enabled"`
	ScanRouter string   `toml:"scan_router"`
	ScanDirs   []string `toml:"scan_dirs"`
}

func Default() *Config {
	return &Config{
		SchemaVersion: 1,
		Strict:        true,
		Paths: PathsConfig{
			RepoRoot:     ".",
			AssetsRoot:   "assets/embed/assets",
			ManifestPath: "assets/embed/assets/manifest.json",
		},
		JS: JSConfig{
			OutDir:              "assets/embed/assets/js",
			HashLength:          8,
			SrcDirs:             []string{"assets/src/js"},
			CopyDirs:            []string{"assets/src/templui/assets/js"},
			ScanTemplateDirs:    []string{"ui", "feature"},
			TempluiComponentDir: "assets/src/templui/assets/js",
			Dependencies: DepConfig{
				"sheet":      {"dialog"},
				"dropdown":   {"popover"},
				"selectbox":  {"input", "popover"},
				"datepicker": {"input", "popover"},
				"timepicker": {"input", "popover"},
				"tagsinput":  {"popover"},
			},
		},
		Manifest: ManifestConfig{
			HashLength:   8,
			CleanupStale: true,
		},
		Images: ImagesConfig{ConfigPath: "tools/imageopt/config.json"},
		TempluiMap: TempluiMapConfig{
			Mode:                   "declarative",
			Out:                    "core/templui/generated_routes.go",
			ComponentDir:           "assets/src/templui/assets/js",
			DefaultComponents:      []string{"dialog"},
			LongestPrefixMatch:     true,
			FailOnMissingComponent: true,
			Suggest: SuggestConfig{
				Enabled:    true,
				ScanRouter: "core/router/router.go",
				ScanDirs:   []string{"ui", "feature"},
			},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			absPath, absErr := filepath.Abs(path)
			if absErr != nil {
				absPath = path
			}
			return nil, fmt.Errorf("missing %s at %s; run `buildybud init` in the repo root or pass --config <path>", DefaultPath, absPath)
		}
		return nil, err
	}

	meta, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Strict {
		if undecoded := meta.Undecoded(); len(undecoded) > 0 {
			keys := make([]string, 0, len(undecoded))
			for _, key := range undecoded {
				keys = append(keys, key.String())
			}
			sort.Strings(keys)
			return nil, fmt.Errorf("unknown config keys: %s", strings.Join(keys, ", "))
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", c.SchemaVersion)
	}
	if c.Manifest.HashLength < 1 || c.Manifest.HashLength > 64 {
		return fmt.Errorf("manifest.hash_length must be 1..64")
	}
	if c.JS.HashLength < 1 || c.JS.HashLength > 64 {
		return fmt.Errorf("js.hash_length must be 1..64")
	}
	if c.Paths.RepoRoot == "" {
		return errors.New("paths.repo_root is required")
	}
	if c.Paths.AssetsRoot == "" {
		return errors.New("paths.assets_root is required")
	}
	if c.Paths.ManifestPath == "" {
		return errors.New("paths.manifest_path is required")
	}
	if c.TempluiMap.Out == "" {
		return errors.New("templui_map.out is required")
	}
	if c.TempluiMap.ComponentDir == "" {
		return errors.New("templui_map.component_dir is required")
	}
	seen := map[string]bool{}
	for _, rule := range c.TempluiMap.Rules {
		if !strings.HasPrefix(rule.Prefix, "/") {
			return fmt.Errorf("templui_map.rule prefix must start with '/': %s", rule.Prefix)
		}
		if seen[rule.Prefix] {
			return fmt.Errorf("duplicate templui_map.rule prefix: %s", rule.Prefix)
		}
		seen[rule.Prefix] = true
	}
	return nil
}

func (c *Config) RepoPath(rel string) string {
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	return filepath.Clean(filepath.Join(c.Paths.RepoRoot, rel))
}

func Init(repoRoot, path string, force bool) error {
	if repoRoot == "" {
		repoRoot = "."
	}
	if path == "" {
		path = filepath.Join(repoRoot, DefaultPath)
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists at %s; rerun with --force to overwrite it", DefaultPath, path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	cfg, err := Discover(repoRoot)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, cfg.MarshalTOML(), 0o644); err != nil {
		return err
	}
	return nil
}

func Discover(repoRoot string) (*Config, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	cfg := Default()
	cfg.Paths.RepoRoot = "."
	cfg.ModulePath = detectModulePath(root)
	cfg.Paths.AssetsRoot = detectExisting(root, "assets/embed/assets", "assets")
	cfg.Paths.ManifestPath = filepath.ToSlash(filepath.Join(cfg.Paths.AssetsRoot, "manifest.json"))
	cfg.JS.OutDir = filepath.ToSlash(filepath.Join(cfg.Paths.AssetsRoot, "js"))
	cfg.JS.SrcDirs = existingDirs(root, "assets/src/js")
	cfg.JS.CopyDirs = existingDirs(root, "assets/src/templui/assets/js")
	cfg.JS.ScanTemplateDirs = existingDirs(root, "ui", "feature")
	cfg.JS.TempluiComponentDir = firstOrDefault(cfg.JS.CopyDirs, cfg.JS.TempluiComponentDir)
	cfg.Images.ConfigPath = detectExisting(root, "tools/imageopt/config.json", "imageopt/config.json")
	cfg.TempluiMap.ComponentDir = cfg.JS.TempluiComponentDir
	cfg.TempluiMap.Out = detectExisting(root, "core/templui/generated_routes.go", "internal/templui/generated_routes.go")
	cfg.TempluiMap.Suggest.ScanRouter = detectExisting(root, "core/router/router.go", "internal/router/router.go")
	cfg.TempluiMap.Suggest.ScanDirs = append([]string(nil), cfg.JS.ScanTemplateDirs...)
	cfg.TempluiMap.Rules = discoverTempluiRules(root)
	return cfg, nil
}

func (c *Config) MarshalTOML() []byte {
	var buf bytes.Buffer
	writeKV := func(key, value string) {
		fmt.Fprintf(&buf, "%s = %q\n", key, value)
	}
	writeBool := func(key string, value bool) {
		fmt.Fprintf(&buf, "%s = %t\n", key, value)
	}
	writeInt := func(key string, value int) {
		fmt.Fprintf(&buf, "%s = %d\n", key, value)
	}
	writeList := func(key string, values []string) {
		quoted := make([]string, 0, len(values))
		for _, value := range values {
			quoted = append(quoted, fmt.Sprintf("%q", filepath.ToSlash(value)))
		}
		fmt.Fprintf(&buf, "%s = [%s]\n", key, strings.Join(quoted, ", "))
	}

	writeInt("schema_version", c.SchemaVersion)
	writeKV("module_path", c.ModulePath)
	writeBool("strict", c.Strict)
	buf.WriteString("\n[paths]\n")
	writeKV("repo_root", c.Paths.RepoRoot)
	writeKV("assets_root", filepath.ToSlash(c.Paths.AssetsRoot))
	writeKV("manifest_path", filepath.ToSlash(c.Paths.ManifestPath))

	buf.WriteString("\n[js]\n")
	writeKV("out_dir", filepath.ToSlash(c.JS.OutDir))
	writeInt("hash_length", c.JS.HashLength)
	writeList("src_dirs", c.JS.SrcDirs)
	writeList("copy_dirs", c.JS.CopyDirs)
	writeList("scan_template_dirs", c.JS.ScanTemplateDirs)
	writeKV("templui_component_dir", filepath.ToSlash(c.JS.TempluiComponentDir))

	buf.WriteString("\n[js.dependencies]\n")
	depNames := make([]string, 0, len(c.JS.Dependencies))
	for name := range c.JS.Dependencies {
		depNames = append(depNames, name)
	}
	sort.Strings(depNames)
	for _, name := range depNames {
		writeList(name, c.JS.Dependencies[name])
	}

	buf.WriteString("\n[manifest]\n")
	writeInt("hash_length", c.Manifest.HashLength)
	writeBool("cleanup_stale", c.Manifest.CleanupStale)

	buf.WriteString("\n[images]\n")
	writeKV("config_path", filepath.ToSlash(c.Images.ConfigPath))

	buf.WriteString("\n[templui_map]\n")
	writeKV("mode", c.TempluiMap.Mode)
	writeKV("out", filepath.ToSlash(c.TempluiMap.Out))
	writeKV("component_dir", filepath.ToSlash(c.TempluiMap.ComponentDir))
	writeList("default_components", c.TempluiMap.DefaultComponents)
	writeBool("longest_prefix_match", c.TempluiMap.LongestPrefixMatch)
	writeBool("fail_on_missing_component", c.TempluiMap.FailOnMissingComponent)
	for _, rule := range c.TempluiMap.Rules {
		buf.WriteString("\n[[templui_map.rule]]\n")
		writeKV("prefix", rule.Prefix)
		writeList("components", rule.Components)
	}

	buf.WriteString("\n[templui_map.suggest]\n")
	writeBool("enabled", c.TempluiMap.Suggest.Enabled)
	writeKV("scan_router", filepath.ToSlash(c.TempluiMap.Suggest.ScanRouter))
	writeList("scan_dirs", c.TempluiMap.Suggest.ScanDirs)
	return buf.Bytes()
}

func detectModulePath(repoRoot string) string {
	goModPath := filepath.Join(repoRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				module := strings.TrimSpace(strings.TrimPrefix(line, "module "))
				parts := strings.Split(module, "/")
				return parts[len(parts)-1]
			}
		}
	}
	return filepath.Base(repoRoot)
}

func detectExisting(repoRoot string, candidates ...string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(candidate))); err == nil {
			return candidate
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func existingDirs(repoRoot string, candidates ...string) []string {
	found := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(candidate)))
		if err == nil && info.IsDir() {
			found = append(found, candidate)
		}
	}
	return found
}

func firstOrDefault(values []string, fallback string) string {
	if len(values) > 0 {
		return values[0]
	}
	return fallback
}

func discoverTempluiRules(repoRoot string) []TempluiRule {
	rules := []TempluiRule{{Prefix: "/", Components: []string{"dialog"}}}
	if dirExists(filepath.Join(repoRoot, "content", "blog")) {
		rules = append(rules, TempluiRule{Prefix: "/blog", Components: []string{"dialog", "popover"}})
	}
	if dirExists(filepath.Join(repoRoot, "content", "projects")) {
		rules = append(rules, TempluiRule{Prefix: "/projects", Components: []string{"dialog", "selectbox"}})
	}
	return rules
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
