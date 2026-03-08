package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jasonwillschiu/buildybud/internal/cdn"
	"github.com/jasonwillschiu/buildybud/internal/changelog"
	"github.com/jasonwillschiu/buildybud/internal/config"
	"github.com/jasonwillschiu/buildybud/internal/doctor"
	"github.com/jasonwillschiu/buildybud/internal/envfile"
	"github.com/jasonwillschiu/buildybud/internal/images"
	"github.com/jasonwillschiu/buildybud/internal/js"
	"github.com/jasonwillschiu/buildybud/internal/manifest"
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
		case "js":
			return runJS(args[1:], stderr)
		case "images":
			return runImages(args[1:], stderr)
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
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &usageError{msg: err.Error()}
	}
	if fs.NArg() != 0 {
		return &usageError{msg: "init does not accept positional arguments"}
	}
	if err := config.Init(*repoRoot, *configPath, *force); err != nil {
		return err
	}
	envExamplePath := filepath.Join(*repoRoot, ".env.example")
	if err := envfile.AppendExample(envExamplePath); err != nil {
		return err
	}
	absPath, err := filepath.Abs(*configPath)
	if err != nil {
		absPath = *configPath
	}
	_, _ = fmt.Fprintf(stdout, "Generated %s\n", absPath)
	if absEnvPath, err := filepath.Abs(envExamplePath); err == nil {
		_, _ = fmt.Fprintf(stdout, "Updated %s\n", absEnvPath)
	} else {
		_, _ = fmt.Fprintf(stdout, "Updated %s\n", envExamplePath)
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

func runJS(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("buildybud js", flag.ContinueOnError)
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

	return js.Run(js.Options{
		OutDir:              cfg.RepoPath(cfg.JS.OutDir),
		AssetsRoot:          cfg.RepoPath(cfg.Paths.AssetsRoot),
		ManifestPath:        cfg.RepoPath(cfg.Paths.ManifestPath),
		HashLength:          cfg.JS.HashLength,
		SrcDirs:             rebase(cfg, cfg.JS.SrcDirs),
		CopyDirs:            rebase(cfg, cfg.JS.CopyDirs),
		ScanTemplateDirs:    rebase(cfg, cfg.JS.ScanTemplateDirs),
		TempluiComponentDir: cfg.RepoPath(cfg.JS.TempluiComponentDir),
		Dependencies:        cfg.JS.Dependencies,
	})
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

	return images.Run(images.Options{
		ConfigPath: cfg.RepoPath(cfg.Images.ConfigPath),
		Verbose:    *verbose,
	})
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
		return templuimap.Generate(cfg)
	case "check":
		return templuimap.Check(cfg)
	case "suggest":
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

func printRootUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, `%s version %s

Usage:
  %s <command> [flags]

Quick start:
  1. Install: go install github.com/jasonwillschiu/buildybud@%s
  2. In your repo root: buildybud init
  3. Check the generated %s and adjust repo-specific paths
  4. Run buildybud doctor
  5. Run a command such as buildybud js or buildybud manifest --input ... --logical ...

Commands:
  init               Scan the repo and generate a starter buildybud.toml
  manifest           Hash a built file and update manifest.json
  js                 Build and hash JS assets, copy templui JS
  images             Generate image variants via vips
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
  %s js --config ./buildybud.toml
  %s cdn purge --provider bunny /blog /projects/demo
`, toolName, ToolVersion, toolName, ToolVersion, config.DefaultPath, toolName, toolName, toolName, toolName)
}
