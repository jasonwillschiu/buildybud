package doctor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jasonwillschiu/buildybud/internal/config"
	"github.com/jasonwillschiu/buildybud/internal/templuimap"
)

func Run(cfg *config.Config) error {
	checks := []struct {
		name string
		fn   func() error
	}{
		{name: "config", fn: cfg.Validate},
		{name: "paths", fn: func() error { return checkPaths(cfg) }},
		{name: "vips", fn: checkVips},
		{name: "templui-map", fn: func() error { return templuimap.Check(cfg) }},
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
		cfg.RepoPath(cfg.JS.TempluiComponentDir),
	}
	for _, p := range requiredDirs {
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			return fmt.Errorf("missing directory: %s", p)
		}
	}
	if _, err := os.Stat(cfg.RepoPath(cfg.Images.ConfigPath)); err != nil {
		return fmt.Errorf("missing images config: %s", cfg.Images.ConfigPath)
	}
	return nil
}

func checkVips() error {
	_, err := exec.LookPath("vips")
	if err != nil {
		return fmt.Errorf("vips CLI not found in PATH")
	}
	return nil
}
