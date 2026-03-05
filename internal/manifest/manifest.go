package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Options struct {
	InputPath    string
	LogicalPath  string
	ManifestPath string
	AssetsRoot   string
	HashLength   int
}

func Run(opts Options) error {
	if opts.InputPath == "" || opts.LogicalPath == "" {
		return fmt.Errorf("--input and --logical are required")
	}
	if opts.HashLength < 1 || opts.HashLength > 64 {
		return fmt.Errorf("invalid hash length %d", opts.HashLength)
	}

	content, err := os.ReadFile(opts.InputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	sum := sha256.Sum256(content)
	hashHex := hex.EncodeToString(sum[:])

	ext := filepath.Ext(opts.InputPath)
	base := strings.TrimSuffix(filepath.Base(opts.InputPath), ext)
	hashedName := fmt.Sprintf("%s.%s%s", base, hashHex[:opts.HashLength], ext)
	hashedRelPath := filepath.ToSlash(filepath.Join(filepath.Dir(relToAssets(opts.AssetsRoot, opts.InputPath)), hashedName))
	hashedAbsPath := filepath.Join(opts.AssetsRoot, filepath.FromSlash(hashedRelPath))

	if err := os.MkdirAll(filepath.Dir(hashedAbsPath), 0o755); err != nil {
		return fmt.Errorf("ensure hashed dir: %w", err)
	}
	if err := os.Remove(hashedAbsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing hashed file: %w", err)
	}
	if err := os.WriteFile(hashedAbsPath, content, 0o644); err != nil {
		return fmt.Errorf("write hashed asset: %w", err)
	}
	if err := os.Remove(opts.InputPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unhashed asset: %w", err)
	}

	manifest, err := readManifest(opts.ManifestPath)
	if err != nil {
		return err
	}

	if previous, ok := manifest[opts.LogicalPath]; ok && previous != hashedRelPath {
		previousAbs := filepath.Join(opts.AssetsRoot, filepath.FromSlash(previous))
		_ = os.Remove(previousAbs)
	}
	manifest[opts.LogicalPath] = hashedRelPath

	if err := writeManifest(opts.ManifestPath, manifest); err != nil {
		return err
	}

	return nil
}

func relToAssets(assetsRoot, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	assetsAbs, err := filepath.Abs(assetsRoot)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(assetsAbs, abs)
	if err != nil {
		return path
	}
	return rel
}

func readManifest(path string) (map[string]string, error) {
	manifest := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if len(data) == 0 {
		return manifest, nil
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return manifest, nil
}

func writeManifest(path string, manifest map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure manifest dir: %w", err)
	}

	keys := make([]string, 0, len(manifest))
	for k := range manifest {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(map[string]string, len(manifest))
	for _, k := range keys {
		ordered[k] = manifest[k]
	}

	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
