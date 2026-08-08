package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jasonwillschiu/buildybud/internal/cdn"
	"github.com/jasonwillschiu/buildybud/internal/changelog"
	"github.com/jasonwillschiu/buildybud/internal/config"
	"github.com/jasonwillschiu/buildybud/internal/css"
	"github.com/jasonwillschiu/buildybud/internal/doctor"
	"github.com/jasonwillschiu/buildybud/internal/envfile"
	"github.com/jasonwillschiu/buildybud/internal/favicon"
	"github.com/jasonwillschiu/buildybud/internal/images"
	"github.com/jasonwillschiu/buildybud/internal/js"
	"github.com/jasonwillschiu/buildybud/internal/manifest"
	"github.com/jasonwillschiu/buildybud/internal/og"
	"github.com/jasonwillschiu/buildybud/internal/templuimap"
)

const (
	ExitOK      = 0
	ExitGeneral = 1
	ExitUsage   = 2
	toolName    = "buildybud"
)

var ToolVersion = "v0.0.0"

type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func Run(args []string, stdout, stderr io.Writer) int {
	if err := run(args, stdout, stderr); err != nil {
		if _, ok := err.(*usageError); ok {
			_, _ = fmt.Fprintln(stderr, err)
			_, _ = fmt.Fprintln(stderr)
			printRootUsage(stderr)
			return ExitUsage
		}
		if pe := new(changelog.ParseError); errors.As(err, &pe) {
			_, _ = fmt.Fprintln(stderr, "Error:", err)
			_, _ = fmt.Fprintf(stderr, "Expected format example in %s: %s\n", pe.Path, changelog.ExpectedFormat)
			return ExitGeneral
		}
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return ExitGeneral
	}
	return ExitOK
}

func run(args []string, stdout, stderr io.Writer) error {
	if err := envfile.LoadIfPresent(".env"); err != nil {
		return fmt.Errorf("failed to load .env: %w", err)
	}
	if len(args) > 0 {
		switch args[0] {
		case "-h", "-help", "--help":
			printRootUsage(stdout)
			return nil
		case "-version", "--version":
			if len(args) > 1 {
				return &usageError{msg: "--version does not accept additional arguments"}
			}
			_, _ = fmt.Fprintf(stdout, "%s version %s\n", toolName, ToolVersion)
			return nil
		case "version":
			return runRepoVersion(args[1:], stdout, stderr)
		case "manifest":
			return runManifest(args[1:], stderr)
		case "css":
			return runCSS(args[1:], stdout, stderr)
		case "js":
			return runJS(args[1:], stdout, stderr)
		case "images":
			return runImages(args[1:], stderr)
		case "og":
			return runOG(args[1:], stderr)
		case "favicon":
			return runFavicon(args[1:], stderr)
		case "assets":
			return runAssets(args[1:], stdout, stderr)
		case "init":
			return runInit(args[1:], stdout, stderr)
		case "doctor":
			return runDoctor(args[1:], stderr)
		case "cdn":
			return runCDN(args[1:], stderr)
		case "templui-map":
			return runTempluiMap(args[1:], stderr)
		default:
			return &usageError{msg: fmt.Sprintf("unknown command: %s", args[0])}
		}
	}
	return &usageError{msg: "missing command"}
}

func runInit(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoRoot := fs.String("repo-root", ".", "Repo root to scan for build paths")
	configPath := fs.String("config", config.DefaultPath, "Path for the generated buildybud TOML config")
	force := fs.Bool("force", false, "Overwrite an existing config file")
	dryRun := fs.Bool("dry-run", false, "Print the generated config without writing files")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	if fs.NArg() != 0 {
		return &usageError{msg: "init does not accept positional arguments"}
	}
	if *dryRun {
		cfg, err := config.Discover(*repoRoot)
		if err != nil {
			return err
		}
		_, _ = stdout.Write(cfg.MarshalTOML())
		return nil
	}

	resolvedConfigPath := resolveInitConfigPath(*repoRoot, *configPath)
	if err := config.Init(*repoRoot, *configPath, *force); err != nil {
		return err
	}
	absPath, err := filepath.Abs(resolvedConfigPath)
	if err != nil {
		absPath = resolvedConfigPath
	}
	_, _ = fmt.Fprintf(stdout, "Generated %s\n", absPath)
	if configPathIsInsideRepo(*repoRoot, resolvedConfigPath) {
		envExamplePath := filepath.Join(*repoRoot, ".env.example")
		if err := envfile.AppendExample(envExamplePath); err != nil {
			return err
		}
		if absEnvPath, err := filepath.Abs(envExamplePath); err == nil {
			_, _ = fmt.Fprintf(stdout, "Updated %s\n", absEnvPath)
		} else {
			_, _ = fmt.Fprintf(stdout, "Updated %s\n", envExamplePath)
		}
	}
	return nil
}

func runRepoVersion(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	changelogPath := fs.String("changelog", "changelog.md", "Path to changelog file")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	if fs.NArg() != 0 {
		return &usageError{msg: "version does not accept positional arguments"}
	}
	entry, err := changelog.ParseLatest(*changelogPath)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, entry.Version)
	return nil
}

func runManifest(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud manifest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	inputPath := fs.String("input", "", "Path to built asset that should be hashed")
	logicalPath := fs.String("logical", "", "Logical asset path (for example css/app.css)")
	manifestPath := fs.String("manifest", "", "Manifest JSON path override")
	assetsRoot := fs.String("assets-root", "", "Assets root directory override")
	hashLength := fs.Int("hash-length", 0, "Hash prefix length override")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *manifestPath == "" {
		*manifestPath = cfg.Paths.ManifestPath
	}
	if *assetsRoot == "" {
		*assetsRoot = cfg.Paths.AssetsRoot
	}
	if *hashLength == 0 {
		*hashLength = cfg.Manifest.HashLength
	}

	return manifest.Run(manifest.Options{
		InputPath:    cfg.RepoPath(*inputPath),
		LogicalPath:  *logicalPath,
		ManifestPath: cfg.RepoPath(*manifestPath),
		AssetsRoot:   cfg.RepoPath(*assetsRoot),
		HashLength:   *hashLength,
	})
}

func runJS(args []string, stdout, stderr io.Writer) error {
	subcommand := "build"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcommand = strings.TrimSpace(args[0])
		args = args[1:]
	}
	if subcommand != "build" && subcommand != "audit" {
		return &usageError{msg: fmt.Sprintf("unknown js subcommand: %s", subcommand)}
	}
	fs := flag.NewFlagSet("buildybud js", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	if fs.NArg() != 0 {
		return &usageError{msg: "js does not accept positional arguments"}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if subcommand == "audit" {
		return runJSAuditWithConfig(cfg, stdout)
	}
	return runJSWithConfig(cfg, stdout)
}

func runJSWithConfig(cfg *config.Config, stdout io.Writer) error {
	if !cfg.JS.Enabled {
		return nil
	}
	if err := js.Run(js.Options{
		OutDir:              cfg.RepoPath(cfg.JS.OutDir),
		AssetsRoot:          cfg.RepoPath(cfg.Paths.AssetsRoot),
		ManifestPath:        cfg.RepoPath(cfg.Paths.ManifestPath),
		HashLength:          cfg.JS.HashLength,
		Minify:              cfg.JS.Minify,
		SrcDirs:             rebase(cfg, cfg.JS.SrcDirs),
		CopyDirs:            rebase(cfg, cfg.JS.CopyDirs),
		ScanTemplateDirs:    rebase(cfg, cfg.JS.ScanTemplateDirs),
		TempluiComponentDir: cfg.RepoPath(cfg.JS.TempluiComponentDir),
		Dependencies:        cfg.JS.Dependencies,
	}); err != nil {
		return err
	}
	return runJSAuditWithConfig(cfg, stdout)
}

func runJSAuditWithConfig(cfg *config.Config, stdout io.Writer) error {
	if !cfg.JS.Audit.Enabled {
		return nil
	}
	groups := make([]js.AuditGroup, 0, len(cfg.JS.Audit.Groups))
	for _, group := range cfg.JS.Audit.Groups {
		groups = append(groups, js.AuditGroup{
			Name:            group.Name,
			Assets:          append([]string(nil), group.Assets...),
			Files:           rebase(cfg, group.Files),
			GzipBudgetBytes: group.GzipBudgetBytes,
		})
	}
	_, err := js.Audit(js.AuditOptions{
		RepoRoot:          cfg.RepoPath("."),
		AssetsRoot:        cfg.RepoPath(cfg.Paths.AssetsRoot),
		ManifestPath:      cfg.RepoPath(cfg.Paths.ManifestPath),
		InlineDirs:        rebase(cfg, cfg.JS.Audit.InlineDirs),
		MaxInlineRawBytes: cfg.JS.Audit.MaxInlineRawBytes,
		Groups:            groups,
		Writer:            stdout,
	})
	return err
}

func runCSS(args []string, stdout, stderr io.Writer) error {
	subcommand := "build"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcommand = strings.TrimSpace(args[0])
		args = args[1:]
	}
	if subcommand != "build" && subcommand != "audit" {
		return &usageError{msg: fmt.Sprintf("unknown css subcommand: %s", subcommand)}
	}
	fs := flag.NewFlagSet("buildybud css", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if subcommand == "audit" {
		return runCSSAuditWithConfig(cfg, stdout)
	}
	return runCSSWithConfig(cfg, stdout)
}

func runCSSWithConfig(cfg *config.Config, stdout io.Writer) error {
	if !cfg.CSS.Enabled {
		return nil
	}
	if cfg.CSS.Mode == "bundles" {
		bundles := make([]css.Bundle, 0, len(cfg.CSS.Bundles))
		logicalPaths := make(map[string]string, len(cfg.CSS.Bundles))
		for _, bundle := range cfg.CSS.Bundles {
			bundles = append(bundles, css.Bundle{Name: bundle.Name, Input: cfg.RepoPath(bundle.Input)})
			output := cfg.RepoPath(bundle.Output)
			logical, err := logicalAssetPath(cfg, output)
			if err != nil {
				return err
			}
			logicalPaths[bundle.Name] = logical
		}
		compiled, cleanup, err := css.CompileBundles(cfg.CSS.Command, cfg.CSS.Minify, bundles)
		if err != nil {
			return err
		}
		defer cleanup()

		compiledPaths := make(map[string]string, len(compiled))
		for _, bundle := range compiled {
			compiledPaths[bundle.Name] = bundle.Path
		}
		if cfg.CSS.Audit.Enabled {
			if _, err := css.Audit(compiledPaths, cssAuditGroups(cfg), stdout); err != nil {
				return err
			}
		}

		assets := make([]manifest.BatchAsset, 0, len(compiled))
		for _, bundle := range compiled {
			assets = append(assets, manifest.BatchAsset{
				InputPath:   bundle.Path,
				LogicalPath: logicalPaths[bundle.Name],
			})
		}
		return manifest.RunBatch(manifest.BatchOptions{
			Assets:       assets,
			ManifestPath: cfg.RepoPath(cfg.Paths.ManifestPath),
			AssetsRoot:   cfg.RepoPath(cfg.Paths.AssetsRoot),
			HashLength:   cfg.Manifest.HashLength,
			OwnedPrefix:  "css/",
			CleanupStale: cfg.Manifest.CleanupStale,
		})
	}
	output := cfg.RepoPath(cfg.CSS.Output)
	if err := css.Run(css.Options{
		Command: cfg.CSS.Command,
		Input:   cfg.RepoPath(cfg.CSS.Input),
		Output:  output,
		Content: rebase(cfg, cfg.CSS.Content),
		Minify:  cfg.CSS.Minify,
	}); err != nil {
		return err
	}
	logicalPath, err := logicalAssetPath(cfg, output)
	if err != nil {
		return err
	}
	return manifest.Run(manifest.Options{
		InputPath:    output,
		LogicalPath:  logicalPath,
		ManifestPath: cfg.RepoPath(cfg.Paths.ManifestPath),
		AssetsRoot:   cfg.RepoPath(cfg.Paths.AssetsRoot),
		HashLength:   cfg.Manifest.HashLength,
	})
}

func runCSSAuditWithConfig(cfg *config.Config, stdout io.Writer) error {
	if !cfg.CSS.Enabled || !cfg.CSS.Audit.Enabled {
		return nil
	}
	if cfg.CSS.Mode != "bundles" {
		return fmt.Errorf("css audit requires css.mode = bundles")
	}
	logicalPaths := make(map[string]string, len(cfg.CSS.Bundles))
	for _, bundle := range cfg.CSS.Bundles {
		logical, err := logicalAssetPath(cfg, cfg.RepoPath(bundle.Output))
		if err != nil {
			return err
		}
		logicalPaths[bundle.Name] = logical
	}
	bundlePaths, err := css.ManifestBundlePaths(
		cfg.RepoPath(cfg.Paths.ManifestPath),
		cfg.RepoPath(cfg.Paths.AssetsRoot),
		logicalPaths,
	)
	if err != nil {
		return err
	}
	_, err = css.Audit(bundlePaths, cssAuditGroups(cfg), stdout)
	return err
}

func cssAuditGroups(cfg *config.Config) []css.AuditGroup {
	groups := make([]css.AuditGroup, 0, len(cfg.CSS.Audit.Groups))
	for _, group := range cfg.CSS.Audit.Groups {
		groups = append(groups, css.AuditGroup{
			Name:            group.Name,
			Bundles:         append([]string(nil), group.Bundles...),
			GzipBudgetBytes: group.GzipBudgetBytes,
		})
	}
	return groups
}

func runImages(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud images", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	verbose := fs.Bool("v", false, "Verbose output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if !cfg.Images.Enabled {
		return nil
	}
	return runImagesWithConfig(cfg, *verbose)
}

func runImagesWithConfig(cfg *config.Config, verbose bool) error {
	if cfg.OG.Enabled {
		if err := runOGWithConfig(cfg); err != nil {
			return err
		}
	}
	if !cfg.Images.Enabled {
		return nil
	}

	return images.Run(images.Options{
		ConfigPath: cfg.RepoPath(cfg.Images.ConfigPath),
		Verbose:    verbose,
	})
}

func runOG(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud og", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	return runOGWithConfig(cfg)
}

func runOGWithConfig(cfg *config.Config) error {
	if !cfg.OG.Enabled {
		return nil
	}
	_, err := og.Run(context.Background(), og.Options{
		RepoRoot:        cfg.RepoPath(cfg.Paths.RepoRoot),
		SiteName:        cfg.OG.SiteName,
		SiteURL:         cfg.OG.SiteURL,
		BackgroundPath:  cfg.RepoPath(cfg.OG.Background),
		LogoPath:        cfg.RepoPath(cfg.OG.Logo),
		FontPath:        cfg.RepoPath(cfg.OG.Font),
		RoutesFile:      cfg.RepoPath(cfg.OG.RoutesFile),
		RouteOutputDir:  cfg.RepoPath(cfg.OG.RouteOutputDir),
		RouteManifest:   cfg.RepoPath(cfg.OG.RouteManifest),
		BlogDir:         cfg.RepoPath(cfg.OG.BlogDir),
		BlogImageDir:    cfg.RepoPath(cfg.OG.BlogImageDir),
		RawBlogImageDir: cfg.RepoPath(cfg.OG.RawBlogImageDir),
		BlogManifest:    cfg.RepoPath(cfg.OG.BlogManifest),
	})
	return err
}

func runFavicon(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud favicon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	return runFaviconWithConfig(cfg)
}

func runFaviconWithConfig(cfg *config.Config) error {
	if !cfg.Favicon.Enabled {
		return nil
	}
	return favicon.Run(favicon.Options{
		SourceDir: cfg.RepoPath(cfg.Favicon.SourceDir),
		OutDir:    cfg.RepoPath(cfg.Favicon.OutDir),
		Clean:     cfg.Favicon.Clean,
	})
}

func runAssets(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud assets", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	only := fs.String("only", "", "Comma-separated asset stages to run: css,js,og,images,favicon,templui-map")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	selected, err := parseAssetStageSet(*only)
	if err != nil {
		return err
	}
	shouldRun := func(name string) bool {
		return len(selected) == 0 || selected[name]
	}
	if shouldRun("css") {
		if err := runCSSWithConfig(cfg, stdout); err != nil {
			return err
		}
	}
	if shouldRun("js") {
		if cfg.JS.Enabled {
			if err := runJSWithConfig(cfg, stdout); err != nil {
				return err
			}
		} else if cfg.JS.Audit.Enabled {
			if err := runJSAuditWithConfig(cfg, stdout); err != nil {
				return err
			}
		}
	}
	if shouldRun("og") && (!shouldRun("images") || !cfg.Images.Enabled) {
		if err := runOGWithConfig(cfg); err != nil {
			return err
		}
	}
	if shouldRun("images") && cfg.Images.Enabled {
		if err := runImagesWithConfig(cfg, false); err != nil {
			return err
		}
	}
	if shouldRun("favicon") {
		if err := runFaviconWithConfig(cfg); err != nil {
			return err
		}
	}
	if shouldRun("templui-map") && cfg.TempluiMap.Enabled {
		if err := templuimap.Generate(cfg); err != nil {
			return err
		}
	}
	return nil
}

func runDoctor(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	return doctor.Run(cfg)
}

func runTempluiMap(args []string, stderr io.Writer) error {
	if len(args) == 0 {
		return &usageError{msg: "templui-map requires a subcommand: generate|check|suggest"}
	}
	subcommand := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("buildybud templui-map", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud TOML config")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	switch subcommand {
	case "generate":
		if !cfg.TempluiMap.Enabled {
			return nil
		}
		return templuimap.Generate(cfg)
	case "check":
		if !cfg.TempluiMap.Enabled {
			return nil
		}
		return templuimap.Check(cfg)
	case "suggest":
		if !cfg.TempluiMap.Enabled {
			return nil
		}
		return templuimap.Suggest(cfg)
	default:
		return &usageError{msg: fmt.Sprintf("unknown templui-map subcommand: %s", subcommand)}
	}
}

func runCDN(args []string, stderr io.Writer) error {
	if len(args) == 0 {
		return &usageError{msg: "cdn requires a subcommand: plan|purge|plan-and-purge\n\n" + cdn.RootHelp()}
	}
	subcommand := strings.TrimSpace(args[0])
	switch subcommand {
	case "-h", "-help", "--help":
		_, _ = io.WriteString(stderr, cdn.RootHelp())
		return nil
	}
	switch subcommand {
	case "plan":
		return cdn.Plan(args[1:])
	case "purge":
		return cdn.Purge(args[1:])
	case "plan-and-purge":
		return cdn.PlanAndPurge(args[1:])
	default:
		return &usageError{msg: fmt.Sprintf("unknown cdn subcommand: %s", subcommand)}
	}
}

func rebase(cfg *config.Config, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, cfg.RepoPath(p))
	}
	return out
}

func logicalAssetPath(cfg *config.Config, path string) (string, error) {
	rel, err := filepath.Rel(cfg.RepoPath(cfg.Paths.AssetsRoot), path)
	if err != nil {
		return "", fmt.Errorf("resolve logical asset path for %s: %w", path, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("asset output %s is outside assets root %s", path, cfg.RepoPath(cfg.Paths.AssetsRoot))
	}
	return filepath.ToSlash(rel), nil
}

func parseAssetStageSet(value string) (map[string]bool, error) {
	selected := map[string]bool{}
	if strings.TrimSpace(value) == "" {
		return selected, nil
	}
	allowed := map[string]bool{
		"css":         true,
		"js":          true,
		"og":          true,
		"images":      true,
		"favicon":     true,
		"templui-map": true,
		"templui_map": true,
		"templuimap":  true,
	}
	for _, raw := range strings.Split(value, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !allowed[name] {
			return nil, fmt.Errorf("unknown asset stage %q", name)
		}
		if name == "templui_map" || name == "templuimap" {
			name = "templui-map"
		}
		selected[name] = true
	}
	return selected, nil
}

func configPathIsInsideRepo(repoRoot, configPath string) bool {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return false
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRepo, absConfig)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func resolveInitConfigPath(repoRoot, configPath string) string {
	if configPath == "" {
		configPath = config.DefaultPath
	}
	if filepath.IsAbs(configPath) {
		return configPath
	}
	if repoRoot == "" {
		repoRoot = "."
	}
	return filepath.Join(repoRoot, configPath)
}

func printRootUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, `%s version %s

Usage:
  %s <command> [flags]

Quick start:
  1. Install: go install github.com/jasonwillschiu/buildybud@%s
  2. In your repo root: buildybud init
  3. Check the generated %s and adjust repo-specific paths
  4. Run buildybud doctor
  5. Run a command such as buildybud assets or buildybud manifest --input ... --logical ...

Commands:
  init               Scan the repo and generate a starter buildybud.toml
  assets             Build configured web assets (css, js, og/images, favicon, templui-map)
  css                Build or audit CSS bundles (css audit)
  manifest           Hash a built file and update manifest.json
  js                 Build JS assets or audit their size (js audit)
  og                 Generate Open Graph PNG sources
  images             Generate image variants via vips
  favicon            Copy favicon assets into embedded assets
  cdn                Plan and purge Bunny or Cloudflare CDN URLs
  templui-map        Manage templui route map (generate|check|suggest)
  doctor             Validate config, paths, and tooling
  version            Print latest changelog semver

Global Flags:
  --help, -h, -help  Show help
  --version, -version
                     Show installed CLI version

Examples:
  %s init
  %s doctor
  %s assets --config ./buildybud.toml
  %s css audit --config ./buildybud.toml
  %s js --config ./buildybud.toml
  %s js audit --config ./buildybud.toml
  %s og --config ./buildybud.toml
  %s cdn purge --provider bunny /blog /projects/demo
`, toolName, ToolVersion, toolName, ToolVersion, config.DefaultPath, toolName, toolName, toolName, toolName, toolName, toolName, toolName, toolName)
}
