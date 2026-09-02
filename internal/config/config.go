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

	baseDir string

	Paths      PathsConfig      `toml:"paths"`
	CSS        CSSConfig        `toml:"css"`
	JS         JSConfig         `toml:"js"`
	Manifest   ManifestConfig   `toml:"manifest"`
	OG         OGConfig         `toml:"og"`
	Images     ImagesConfig     `toml:"images"`
	Favicon    FaviconConfig    `toml:"favicon"`
	TempluiMap TempluiMapConfig `toml:"templui_map"`
}

type PathsConfig struct {
	RepoRoot     string `toml:"repo_root"`
	AssetsRoot   string `toml:"assets_root"`
	ManifestPath string `toml:"manifest_path"`
}

type CSSConfig struct {
	Enabled bool           `toml:"enabled"`
	Mode    string         `toml:"mode"`
	Command string         `toml:"command"`
	Input   string         `toml:"input"`
	Output  string         `toml:"output"`
	Content []string       `toml:"content"`
	Minify  bool           `toml:"minify"`
	Bundles []CSSBundle    `toml:"bundle"`
	Audit   CSSAuditConfig `toml:"audit"`
}

type CSSBundle struct {
	Name   string `toml:"name"`
	Input  string `toml:"input"`
	Output string `toml:"output"`
}

type CSSAuditConfig struct {
	Enabled bool            `toml:"enabled"`
	Groups  []CSSAuditGroup `toml:"group"`
}

type CSSAuditGroup struct {
	Name            string   `toml:"name"`
	Bundles         []string `toml:"bundles"`
	GzipBudgetBytes int      `toml:"gzip_budget_bytes"`
}

type JSConfig struct {
	Enabled             bool          `toml:"enabled"`
	Minify              bool          `toml:"minify"`
	OutDir              string        `toml:"out_dir"`
	HashLength          int           `toml:"hash_length"`
	SrcDirs             []string      `toml:"src_dirs"`
	CopyDirs            []string      `toml:"copy_dirs"`
	ScanTemplateDirs    []string      `toml:"scan_template_dirs"`
	TempluiComponentDir string        `toml:"templui_component_dir"`
	Dependencies        DepConfig     `toml:"dependencies"`
	Audit               JSAuditConfig `toml:"audit"`
}

type DepConfig map[string][]string

type JSAuditConfig struct {
	Enabled           bool                 `toml:"enabled"`
	InlineDirs        []string             `toml:"inline_dirs"`
	MaxInlineRawBytes int                  `toml:"max_inline_raw_bytes"`
	Groups            []JSAuditGroupConfig `toml:"group"`
}

type JSAuditGroupConfig struct {
	Name            string   `toml:"name"`
	Assets          []string `toml:"assets"`
	Files           []string `toml:"files"`
	GzipBudgetBytes int      `toml:"gzip_budget_bytes"`
}

type ManifestConfig struct {
	HashLength   int  `toml:"hash_length"`
	CleanupStale bool `toml:"cleanup_stale"`
}

type OGConfig struct {
	Enabled         bool   `toml:"enabled"`
	SiteName        string `toml:"site_name"`
	SiteURL         string `toml:"site_url"`
	Background      string `toml:"background"`
	Logo            string `toml:"logo"`
	Font            string `toml:"font"`
	RoutesFile      string `toml:"routes_file"`
	RouteOutputDir  string `toml:"route_output_dir"`
	RouteManifest   string `toml:"route_manifest"`
	BlogDir         string `toml:"blog_dir"`
	BlogImageDir    string `toml:"blog_image_dir"`
	RawBlogImageDir string `toml:"raw_blog_image_dir"`
	BlogManifest    string `toml:"blog_manifest"`
}

type ImagesConfig struct {
	Enabled    bool           `toml:"enabled"`
	ConfigPath string         `toml:"config_path"`
	SourceDir  string         `toml:"source_dir"`
	OutputDir  string         `toml:"output_dir"`
	Formats    []string       `toml:"formats"`
	Sizes      []int          `toml:"sizes"`
	Quality    map[string]int `toml:"quality"`
}

type FaviconConfig struct {
	Enabled   bool   `toml:"enabled"`
	SourceDir string `toml:"source_dir"`
	OutDir    string `toml:"out_dir"`
	Clean     bool   `toml:"clean"`
}

type TempluiMapConfig struct {
	Enabled                bool          `toml:"enabled"`
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
		CSS: CSSConfig{
			Enabled: true,
			Mode:    "single",
			Command: "tailwindcss",
			Input:   "assets/src/css/input.css",
			Output:  "assets/embed/assets/css/app.css",
			Content: []string{
				"feature/**/*.templ",
				"feature/**/*.go",
				"ui/**/*.templ",
				"assets/src/templui/**/*.templ",
			},
			Minify: true,
		},
		JS: JSConfig{
			Enabled:             true,
			Minify:              true,
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
			Audit: JSAuditConfig{
				Enabled:    true,
				InlineDirs: []string{"ui", "feature"},
			},
		},
		Manifest: ManifestConfig{
			HashLength:   8,
			CleanupStale: true,
		},
		OG: OGConfig{
			Enabled:         false,
			SiteName:        "",
			SiteURL:         "",
			Background:      "assets/src/images/og/background.png",
			Logo:            "assets/src/favicon/web-app-manifest-192x192.png",
			Font:            "assets/src/fonts/og/OpenSans-Bold.ttf",
			RoutesFile:      "content/seo/routes.toml",
			RouteOutputDir:  "assets/src/images/og",
			RouteManifest:   "assets/src/images/og/generated-manifest.json",
			BlogDir:         "content/blog",
			BlogImageDir:    "assets/src/images/blog",
			RawBlogImageDir: "assets/src/raw-images/blog",
			BlogManifest:    "assets/src/images/blog/generated-manifest.json",
		},
		Images: ImagesConfig{Enabled: true},
		Favicon: FaviconConfig{
			Enabled:   true,
			SourceDir: "assets/src/favicon",
			OutDir:    "assets/embed/assets/favicon",
			Clean:     true,
		},
		TempluiMap: TempluiMapConfig{
			Enabled:                true,
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

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing %s at %s; run `buildybud init` in the repo root or pass --config <path>", DefaultPath, absPath)
		}
		return nil, err
	}

	cfg.baseDir = filepath.Dir(absPath)
	meta, err := toml.DecodeFile(absPath, cfg)
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
	if c.JS.Enabled && c.JS.OutDir == "" {
		return errors.New("js.out_dir is required when js.enabled is true")
	}
	if c.JS.Audit.MaxInlineRawBytes < 0 {
		return errors.New("js.audit.max_inline_raw_bytes must be zero or greater")
	}
	auditGroups := map[string]bool{}
	for _, group := range c.JS.Audit.Groups {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			return errors.New("js.audit.group name is required")
		}
		if auditGroups[name] {
			return fmt.Errorf("duplicate js.audit.group name: %s", name)
		}
		auditGroups[name] = true
		if len(group.Assets) == 0 && len(group.Files) == 0 {
			return fmt.Errorf("js.audit.group %q requires assets or files", name)
		}
		if group.GzipBudgetBytes < 0 {
			return fmt.Errorf("js.audit.group %q gzip_budget_bytes must be zero or greater", name)
		}
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
	if c.CSS.Enabled {
		if c.CSS.Command == "" {
			return errors.New("css.command is required when css.enabled is true")
		}
		mode := strings.TrimSpace(c.CSS.Mode)
		if mode == "" {
			mode = "single"
			c.CSS.Mode = mode
		}
		switch mode {
		case "single":
			if c.CSS.Input == "" {
				return errors.New("css.input is required in single mode")
			}
			if c.CSS.Output == "" {
				return errors.New("css.output is required in single mode")
			}
		case "bundles":
			if len(c.CSS.Bundles) == 0 {
				return errors.New("css.bundle is required in bundles mode")
			}
			bundleNames := map[string]bool{}
			bundleOutputs := map[string]bool{}
			for _, bundle := range c.CSS.Bundles {
				name := strings.TrimSpace(bundle.Name)
				if name == "" {
					return errors.New("css.bundle name is required")
				}
				if bundleNames[name] {
					return fmt.Errorf("duplicate css.bundle name: %s", name)
				}
				bundleNames[name] = true
				if strings.TrimSpace(bundle.Input) == "" || strings.TrimSpace(bundle.Output) == "" {
					return fmt.Errorf("css.bundle %q requires input and output", name)
				}
				output := filepath.Clean(c.RepoPath(bundle.Output))
				if bundleOutputs[output] {
					return fmt.Errorf("duplicate css.bundle output: %s", bundle.Output)
				}
				bundleOutputs[output] = true
				assetsRoot := filepath.Clean(c.RepoPath(c.Paths.AssetsRoot))
				rel, err := filepath.Rel(assetsRoot, output)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return fmt.Errorf("css.bundle %q output must be inside paths.assets_root", name)
				}
			}
			groupNames := map[string]bool{}
			for _, group := range c.CSS.Audit.Groups {
				name := strings.TrimSpace(group.Name)
				if name == "" {
					return errors.New("css.audit.group name is required")
				}
				if groupNames[name] {
					return fmt.Errorf("duplicate css.audit.group name: %s", name)
				}
				groupNames[name] = true
				if len(group.Bundles) == 0 {
					return fmt.Errorf("css.audit.group %q requires bundles", name)
				}
				if group.GzipBudgetBytes < 0 {
					return fmt.Errorf("css.audit.group %q gzip_budget_bytes must be zero or greater", name)
				}
				for _, bundleName := range group.Bundles {
					if !bundleNames[bundleName] {
						return fmt.Errorf("css.audit.group %q references unknown bundle %q", name, bundleName)
					}
				}
			}
		default:
			return fmt.Errorf("css.mode must be single or bundles, got %q", c.CSS.Mode)
		}
	}
	if c.Favicon.Enabled {
		if c.Favicon.SourceDir == "" {
			return errors.New("favicon.source_dir is required when favicon.enabled is true")
		}
		if c.Favicon.OutDir == "" {
			return errors.New("favicon.out_dir is required when favicon.enabled is true")
		}
	}
	if c.Images.Enabled {
		if c.Images.ConfigPath != "" && c.Images.hasInlineValues() {
			return errors.New("images.config_path cannot be combined with inline image settings")
		}
		if c.Images.ConfigPath == "" {
			c.Images.applyDefaults(c.Paths.AssetsRoot)
			if strings.TrimSpace(c.Images.SourceDir) == "" {
				return errors.New("images.source_dir is required when images.enabled is true")
			}
			if strings.TrimSpace(c.Images.OutputDir) == "" {
				return errors.New("images.output_dir is required when images.enabled is true")
			}
			if len(c.Images.Formats) == 0 {
				return errors.New("images.formats is required when images.enabled is true")
			}
			seenFormats := map[string]bool{}
			for i, format := range c.Images.Formats {
				format = strings.ToLower(strings.TrimSpace(format))
				if format == "" {
					return errors.New("images.formats entries cannot be empty")
				}
				if seenFormats[format] {
					return fmt.Errorf("duplicate images format: %s", format)
				}
				seenFormats[format] = true
				c.Images.Formats[i] = format
			}
			if len(c.Images.Sizes) == 0 {
				return errors.New("images.sizes is required when images.enabled is true")
			}
			lastSize := 0
			for _, size := range c.Images.Sizes {
				if size <= lastSize {
					return errors.New("images.sizes must contain unique positive widths in ascending order")
				}
				lastSize = size
			}
			for format, quality := range c.Images.Quality {
				if quality < 1 || quality > 100 {
					return fmt.Errorf("images.quality.%s must be 1..100", format)
				}
			}
			assetsRoot := filepath.Clean(c.RepoPath(c.Paths.AssetsRoot))
			outputDir := filepath.Clean(c.RepoPath(c.Images.OutputDir))
			rel, err := filepath.Rel(assetsRoot, outputDir)
			if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return errors.New("images.output_dir must be a child of paths.assets_root")
			}
		}
	}
	if c.OG.Enabled {
		if c.OG.SiteURL == "" {
			return errors.New("og.site_url is required when og.enabled is true")
		}
		for name, value := range map[string]string{
			"og.background":         c.OG.Background,
			"og.logo":               c.OG.Logo,
			"og.font":               c.OG.Font,
			"og.routes_file":        c.OG.RoutesFile,
			"og.route_output_dir":   c.OG.RouteOutputDir,
			"og.route_manifest":     c.OG.RouteManifest,
			"og.blog_dir":           c.OG.BlogDir,
			"og.blog_image_dir":     c.OG.BlogImageDir,
			"og.raw_blog_image_dir": c.OG.RawBlogImageDir,
			"og.blog_manifest":      c.OG.BlogManifest,
		} {
			if value == "" {
				return fmt.Errorf("%s is required when og.enabled is true", name)
			}
		}
	}
	if c.TempluiMap.Enabled && c.TempluiMap.Out == "" {
		return errors.New("templui_map.out is required")
	}
	if c.TempluiMap.Enabled && c.TempluiMap.ComponentDir == "" {
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
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	root := c.Paths.RepoRoot
	if root == "" {
		root = "."
	}
	if !filepath.IsAbs(root) {
		base := c.baseDir
		if base == "" {
			base = "."
		}
		root = filepath.Join(base, root)
	}
	return filepath.Clean(filepath.Join(root, rel))
}

func Init(repoRoot, path string, force bool) error {
	if repoRoot == "" {
		repoRoot = "."
	}
	if path == "" {
		path = DefaultPath
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
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
	cfg.baseDir = root
	cfg.Paths.RepoRoot = "."
	cfg.ModulePath = detectModulePath(root)
	cfg.Paths.AssetsRoot = detectExisting(root, "assets/embed/assets", "assets")
	cfg.Paths.ManifestPath = filepath.ToSlash(filepath.Join(cfg.Paths.AssetsRoot, "manifest.json"))
	cfg.CSS.Input = detectExisting(root, "assets/src/css/input.css")
	cfg.CSS.Output = filepath.ToSlash(filepath.Join(cfg.Paths.AssetsRoot, "css", "app.css"))
	cfg.JS.OutDir = filepath.ToSlash(filepath.Join(cfg.Paths.AssetsRoot, "js"))
	cfg.JS.SrcDirs = existingDirs(root, "assets/src/js")
	cfg.JS.CopyDirs = existingDirs(root, "assets/src/templui/assets/js")
	cfg.JS.ScanTemplateDirs = existingDirs(root, "ui", "feature")
	cfg.JS.Enabled = len(cfg.JS.SrcDirs) > 0 || len(cfg.JS.CopyDirs) > 0
	cfg.JS.Audit.Enabled = cfg.JS.Enabled
	cfg.JS.Audit.InlineDirs = append([]string(nil), cfg.JS.ScanTemplateDirs...)
	cfg.JS.TempluiComponentDir = firstOrDefault(cfg.JS.CopyDirs, cfg.JS.TempluiComponentDir)
	cfg.Images.ConfigPath = firstExisting(root, "assets/src/images/imageopt.config.json", "tools/imageopt/config.json", "imageopt/config.json")
	if cfg.Images.ConfigPath == "" {
		cfg.Images.applyDefaults(cfg.Paths.AssetsRoot)
	}
	cfg.OG.RoutesFile = detectExisting(root, "content/seo/routes.toml")
	cfg.OG.BlogDir = detectExisting(root, "content/blog")
	cfg.Favicon.SourceDir = detectExisting(root, "assets/src/favicon")
	cfg.Favicon.OutDir = filepath.ToSlash(filepath.Join(cfg.Paths.AssetsRoot, "favicon"))
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
	writeIntList := func(key string, values []int) {
		parts := make([]string, 0, len(values))
		for _, value := range values {
			parts = append(parts, fmt.Sprintf("%d", value))
		}
		fmt.Fprintf(&buf, "%s = [%s]\n", key, strings.Join(parts, ", "))
	}
	writeIntMap := func(key string, values map[string]int) {
		names := make([]string, 0, len(values))
		for name := range values {
			names = append(names, name)
		}
		sort.Strings(names)
		parts := make([]string, 0, len(names))
		for _, name := range names {
			parts = append(parts, fmt.Sprintf("%s = %d", name, values[name]))
		}
		fmt.Fprintf(&buf, "%s = { %s }\n", key, strings.Join(parts, ", "))
	}

	writeInt("schema_version", c.SchemaVersion)
	writeKV("module_path", c.ModulePath)
	writeBool("strict", c.Strict)
	buf.WriteString("\n[paths]\n")
	writeKV("repo_root", c.Paths.RepoRoot)
	writeKV("assets_root", filepath.ToSlash(c.Paths.AssetsRoot))
	writeKV("manifest_path", filepath.ToSlash(c.Paths.ManifestPath))

	buf.WriteString("\n[css]\n")
	writeBool("enabled", c.CSS.Enabled)
	writeKV("mode", c.CSS.Mode)
	writeKV("command", c.CSS.Command)
	writeBool("minify", c.CSS.Minify)
	if c.CSS.Mode != "bundles" {
		writeKV("input", filepath.ToSlash(c.CSS.Input))
		writeKV("output", filepath.ToSlash(c.CSS.Output))
		writeList("content", c.CSS.Content)
	}
	for _, bundle := range c.CSS.Bundles {
		buf.WriteString("\n[[css.bundle]]\n")
		writeKV("name", bundle.Name)
		writeKV("input", filepath.ToSlash(bundle.Input))
		writeKV("output", filepath.ToSlash(bundle.Output))
	}
	if c.CSS.Mode == "bundles" || c.CSS.Audit.Enabled || len(c.CSS.Audit.Groups) > 0 {
		buf.WriteString("\n[css.audit]\n")
		writeBool("enabled", c.CSS.Audit.Enabled)
		for _, group := range c.CSS.Audit.Groups {
			buf.WriteString("\n[[css.audit.group]]\n")
			writeKV("name", group.Name)
			writeList("bundles", group.Bundles)
			writeInt("gzip_budget_bytes", group.GzipBudgetBytes)
		}
	}

	buf.WriteString("\n[js]\n")
	writeBool("enabled", c.JS.Enabled)
	writeBool("minify", c.JS.Minify)
	writeKV("out_dir", filepath.ToSlash(c.JS.OutDir))
	writeInt("hash_length", c.JS.HashLength)
	writeList("src_dirs", c.JS.SrcDirs)
	writeList("copy_dirs", c.JS.CopyDirs)
	writeList("scan_template_dirs", c.JS.ScanTemplateDirs)
	writeKV("templui_component_dir", filepath.ToSlash(c.JS.TempluiComponentDir))

	buf.WriteString("\n[js.audit]\n")
	writeBool("enabled", c.JS.Audit.Enabled)
	writeList("inline_dirs", c.JS.Audit.InlineDirs)
	writeInt("max_inline_raw_bytes", c.JS.Audit.MaxInlineRawBytes)
	for _, group := range c.JS.Audit.Groups {
		buf.WriteString("\n[[js.audit.group]]\n")
		writeKV("name", group.Name)
		writeList("assets", group.Assets)
		writeList("files", group.Files)
		writeInt("gzip_budget_bytes", group.GzipBudgetBytes)
	}

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

	buf.WriteString("\n[og]\n")
	writeBool("enabled", c.OG.Enabled)
	writeKV("site_name", c.OG.SiteName)
	writeKV("site_url", c.OG.SiteURL)
	writeKV("background", filepath.ToSlash(c.OG.Background))
	writeKV("logo", filepath.ToSlash(c.OG.Logo))
	writeKV("font", filepath.ToSlash(c.OG.Font))
	writeKV("routes_file", filepath.ToSlash(c.OG.RoutesFile))
	writeKV("route_output_dir", filepath.ToSlash(c.OG.RouteOutputDir))
	writeKV("route_manifest", filepath.ToSlash(c.OG.RouteManifest))
	writeKV("blog_dir", filepath.ToSlash(c.OG.BlogDir))
	writeKV("blog_image_dir", filepath.ToSlash(c.OG.BlogImageDir))
	writeKV("raw_blog_image_dir", filepath.ToSlash(c.OG.RawBlogImageDir))
	writeKV("blog_manifest", filepath.ToSlash(c.OG.BlogManifest))

	buf.WriteString("\n[images]\n")
	writeBool("enabled", c.Images.Enabled)
	if c.Images.ConfigPath != "" {
		writeKV("config_path", filepath.ToSlash(c.Images.ConfigPath))
	} else {
		writeKV("source_dir", filepath.ToSlash(c.Images.SourceDir))
		writeKV("output_dir", filepath.ToSlash(c.Images.OutputDir))
		writeList("formats", c.Images.Formats)
		writeIntList("sizes", c.Images.Sizes)
		writeIntMap("quality", c.Images.Quality)
	}

	buf.WriteString("\n[favicon]\n")
	writeBool("enabled", c.Favicon.Enabled)
	writeKV("source_dir", filepath.ToSlash(c.Favicon.SourceDir))
	writeKV("out_dir", filepath.ToSlash(c.Favicon.OutDir))
	writeBool("clean", c.Favicon.Clean)

	buf.WriteString("\n[templui_map]\n")
	writeBool("enabled", c.TempluiMap.Enabled)
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

func firstExisting(repoRoot string, candidates ...string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(candidate))); err == nil {
			return candidate
		}
	}
	return ""
}

func (c *ImagesConfig) hasInlineValues() bool {
	return c.SourceDir != "" || c.OutputDir != "" || len(c.Formats) > 0 || len(c.Sizes) > 0 || len(c.Quality) > 0
}

func (c *ImagesConfig) applyDefaults(assetsRoot string) {
	if c.SourceDir == "" {
		c.SourceDir = "assets/src/images"
	}
	if c.OutputDir == "" {
		c.OutputDir = filepath.ToSlash(filepath.Join(assetsRoot, "images"))
	}
	if len(c.Formats) == 0 {
		c.Formats = []string{"jpeg", "avif"}
	}
	if len(c.Sizes) == 0 {
		c.Sizes = []int{480, 768, 1200}
	}
	if c.Quality == nil {
		c.Quality = map[string]int{"jpeg": 80, "avif": 60}
	}
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
