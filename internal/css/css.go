package css

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Options struct {
	Command string
	Input   string
	Output  string
	Content []string
	Minify  bool
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
