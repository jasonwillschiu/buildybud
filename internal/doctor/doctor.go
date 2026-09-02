package doctor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jasonwillschiu/buildybud/internal/config"
	"github.com/jasonwillschiu/buildybud/internal/css"
	"github.com/jasonwillschiu/buildybud/internal/templuimap"
)

func Run(cfg *config.Config) error {
	checks := []struct {
		name string
		fn   func() error
	}{
		{name: "config", fn: cfg.Validate},
		{name: "paths", fn: func() error { return checkPaths(cfg) }},
	}
	if cfg.CSS.Enabled && cfg.CSS.Command != css.NativeCommand {
		checks = append(checks, struct {
			name string
			fn   func() error
		}{name: "css", fn: func() error { return checkCommand(cfg.CSS.Command) }})
	}
	if cfg.Images.Enabled {
		checks = append(checks, struct {
			name string
			fn   func() error
		}{name: "vips", fn: func() error { return checkCommand("vips") }})
	}
	if cfg.TempluiMap.Enabled {
		checks = append(checks, struct {
			name string
			fn   func() error
		}{name: "templui-map", fn: func() error { return templuimap.Check(cfg) }})
	}

	for _, check := range checks {
		if err := check.fn(); err != nil {
			return fmt.Errorf("%s check failed: %w", check.name, err)
		}
		fmt.Printf("ok: %s\n", check.name)
	}
	fmt.Println("doctor: all checks passed")
	return nil
}

func checkPaths(cfg *config.Config) error {
	requiredDirs := []string{
		cfg.RepoPath(cfg.Paths.RepoRoot),
		cfg.RepoPath(cfg.Paths.AssetsRoot),
	}
	if cfg.JS.Enabled && cfg.JS.TempluiComponentDir != "" {
		requiredDirs = append(requiredDirs, cfg.RepoPath(cfg.JS.TempluiComponentDir))
	}
	if cfg.Favicon.Enabled {
		requiredDirs = append(requiredDirs, cfg.RepoPath(cfg.Favicon.SourceDir))
	}
	for _, p := range requiredDirs {
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			return fmt.Errorf("missing directory: %s", p)
		}
	}
	if cfg.CSS.Enabled {
		inputs := []string{cfg.CSS.Input}
		if cfg.CSS.Mode == "bundles" {
			inputs = inputs[:0]
			for _, bundle := range cfg.CSS.Bundles {
				inputs = append(inputs, bundle.Input)
			}
		}
		for _, input := range inputs {
			if _, err := os.Stat(cfg.RepoPath(input)); err != nil {
				return fmt.Errorf("missing css input: %s", input)
			}
		}
	}
	if cfg.Images.Enabled {
		if cfg.Images.ConfigPath != "" {
			if _, err := os.Stat(cfg.RepoPath(cfg.Images.ConfigPath)); err != nil {
				return fmt.Errorf("missing images config: %s", cfg.Images.ConfigPath)
			}
		} else if info, err := os.Stat(cfg.RepoPath(cfg.Images.SourceDir)); err != nil || !info.IsDir() {
			return fmt.Errorf("missing images source directory: %s", cfg.Images.SourceDir)
		}
	}
	if cfg.OG.Enabled {
		requiredFiles := map[string]string{
			"og background":  cfg.OG.Background,
			"og logo":        cfg.OG.Logo,
			"og font":        cfg.OG.Font,
			"og routes file": cfg.OG.RoutesFile,
		}
		for label, path := range requiredFiles {
			if _, err := os.Stat(cfg.RepoPath(path)); err != nil {
				return fmt.Errorf("missing %s: %s", label, path)
			}
		}
		for label, path := range map[string]string{
			"og blog dir":       cfg.OG.BlogDir,
			"og blog image dir": cfg.OG.BlogImageDir,
		} {
			if info, err := os.Stat(cfg.RepoPath(path)); err != nil || !info.IsDir() {
				return fmt.Errorf("missing %s: %s", label, path)
			}
		}
	}
	return nil
}

func checkCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command is required")
	}
	_, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("%s CLI not found in PATH", command)
	}
	return nil
}
