package config

import (
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
				"sheet":     {"dialog"},
				"dropdown":  {"popover"},
				"selectbox": {"popover"},
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
			return nil, fmt.Errorf("config not found: %s", path)
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
