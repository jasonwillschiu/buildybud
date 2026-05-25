package favicon

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type Options struct {
	SourceDir string
	OutDir    string
	Clean     bool
}

func Run(opts Options) error {
	if opts.SourceDir == "" || opts.OutDir == "" {
		return fmt.Errorf("favicon source and output directories are required")
	}
	info, err := os.Stat(opts.SourceDir)
	if err != nil {
		return fmt.Errorf("stat favicon source dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("favicon source is not a directory: %s", opts.SourceDir)
	}
	if opts.Clean {
		if err := os.RemoveAll(opts.OutDir); err != nil {
			return fmt.Errorf("clean favicon output dir: %w", err)
		}
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return fmt.Errorf("ensure favicon output dir: %w", err)
	}

	return filepath.WalkDir(opts.SourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(opts.SourceDir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		dst := filepath.Join(opts.OutDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(path, dst)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open favicon source %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat favicon source %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("ensure favicon destination dir: %w", err)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("open favicon destination %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy favicon %s to %s: %w", src, dst, err)
	}
	return nil
}
