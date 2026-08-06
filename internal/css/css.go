package css

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Options struct {
	Command string
	Input   string
	Output  string
	Content []string
	Minify  bool
}

type Bundle struct {
	Name  string
	Input string
}

type CompiledBundle struct {
	Name string
	Path string
}

func CompileBundles(command string, minify bool, bundles []Bundle) ([]CompiledBundle, func(), error) {
	tmpDir, err := os.MkdirTemp("", "buildybud-css-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create CSS build directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	compiled := make([]CompiledBundle, 0, len(bundles))
	for index, bundle := range bundles {
		name := strings.TrimSpace(bundle.Name)
		output := filepath.Join(tmpDir, fmt.Sprintf("%03d-%s.css", index, name))
		if err := Run(Options{
			Command: command,
			Input:   bundle.Input,
			Output:  output,
			Minify:  minify,
		}); err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("compile CSS bundle %q: %w", name, err)
		}
		compiled = append(compiled, CompiledBundle{Name: name, Path: output})
	}
	return compiled, cleanup, nil
}

func Run(opts Options) error {
	if opts.Command == "" {
		return fmt.Errorf("css command is required")
	}
	if opts.Input == "" || opts.Output == "" {
		return fmt.Errorf("css input and output are required")
	}
	if _, err := exec.LookPath(opts.Command); err != nil {
		return fmt.Errorf("%s CLI not found in PATH", opts.Command)
	}
	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
		return fmt.Errorf("ensure css output dir: %w", err)
	}

	args := []string{"-i", opts.Input, "-o", opts.Output}
	if opts.Minify {
		args = append(args, "--minify")
	}
	for _, pattern := range opts.Content {
		args = append(args, "--content", pattern)
	}

	cmd := exec.Command(opts.Command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", opts.Command, err)
	}
	return nil
}
