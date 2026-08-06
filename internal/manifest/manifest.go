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

type BatchAsset struct {
	InputPath   string
	LogicalPath string
}

type BatchOptions struct {
	Assets       []BatchAsset
	ManifestPath string
	AssetsRoot   string
	HashLength   int
	OwnedPrefix  string
	CleanupStale bool
}

type preparedBatchAsset struct {
	logicalPath string
	hashedPath  string
	content     []byte
}

// RunBatch publishes a complete group of assets behind one manifest update.
// Compilation output is read and validated before any current asset or
// manifest entry changes.
func RunBatch(opts BatchOptions) error {
	if opts.HashLength < 1 || opts.HashLength > 64 {
		return fmt.Errorf("invalid hash length %d", opts.HashLength)
	}
	if len(opts.Assets) == 0 {
		return fmt.Errorf("batch requires at least one asset")
	}

	prepared := make([]preparedBatchAsset, 0, len(opts.Assets))
	logicalPaths := make(map[string]bool, len(opts.Assets))
	for _, asset := range opts.Assets {
		logical, err := cleanLogicalPath(asset.LogicalPath)
		if err != nil {
			return err
		}
		if logicalPaths[logical] {
			return fmt.Errorf("duplicate logical asset path %q", logical)
		}
		logicalPaths[logical] = true

		content, err := os.ReadFile(asset.InputPath)
		if err != nil {
			return fmt.Errorf("read batch input %s: %w", asset.InputPath, err)
		}
		sum := sha256.Sum256(content)
		hashHex := hex.EncodeToString(sum[:])
		ext := filepath.Ext(logical)
		base := strings.TrimSuffix(filepath.Base(logical), ext)
		hashedName := fmt.Sprintf("%s.%s%s", base, hashHex[:opts.HashLength], ext)
		hashedPath := filepath.ToSlash(filepath.Join(filepath.Dir(logical), hashedName))
		prepared = append(prepared, preparedBatchAsset{
			logicalPath: logical,
			hashedPath:  hashedPath,
			content:     content,
		})
	}

	current, err := readManifest(opts.ManifestPath)
	if err != nil {
		return err
	}
	next := make(map[string]string, len(current)+len(prepared))
	for logical, hashed := range current {
		next[logical] = hashed
	}
	if opts.CleanupStale && opts.OwnedPrefix != "" {
		for logical := range next {
			if strings.HasPrefix(logical, opts.OwnedPrefix) && !logicalPaths[logical] {
				delete(next, logical)
			}
		}
	}
	for _, asset := range prepared {
		next[asset.logicalPath] = asset.hashedPath
	}

	created := make([]string, 0, len(prepared))
	cleanupCreated := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
	}
	for _, asset := range prepared {
		target := filepath.Join(opts.AssetsRoot, filepath.FromSlash(asset.hashedPath))
		if existing, readErr := os.ReadFile(target); readErr == nil {
			if string(existing) != string(asset.content) {
				cleanupCreated()
				return fmt.Errorf("hashed asset collision at %s", target)
			}
			continue
		} else if !os.IsNotExist(readErr) {
			cleanupCreated()
			return fmt.Errorf("read existing hashed asset %s: %w", target, readErr)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			cleanupCreated()
			return fmt.Errorf("ensure hashed asset directory: %w", err)
		}
		tmp, err := os.CreateTemp(filepath.Dir(target), ".buildybud-css-*")
		if err != nil {
			cleanupCreated()
			return fmt.Errorf("create staged asset: %w", err)
		}
		tmpPath := tmp.Name()
		publishErr := tmp.Chmod(0o644)
		if publishErr == nil {
			_, publishErr = tmp.Write(asset.content)
		}
		closeErr := tmp.Close()
		if publishErr == nil {
			publishErr = closeErr
		}
		if publishErr == nil {
			publishErr = os.Rename(tmpPath, target)
		}
		if publishErr != nil {
			_ = os.Remove(tmpPath)
			cleanupCreated()
			return fmt.Errorf("publish hashed asset %s: %w", target, publishErr)
		}
		created = append(created, target)
	}

	if err := writeManifest(opts.ManifestPath, next); err != nil {
		cleanupCreated()
		return err
	}

	liveHashedPaths := make(map[string]bool, len(next))
	for _, hashed := range next {
		liveHashedPaths[hashed] = true
	}
	for _, hashed := range current {
		if hashed != "" && !liveHashedPaths[hashed] {
			_ = os.Remove(filepath.Join(opts.AssetsRoot, filepath.FromSlash(hashed)))
		}
	}
	return nil
}

func cleanLogicalPath(path string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if clean == "" || clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid logical asset path %q", path)
	}
	return clean, nil
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

	tmp, err := os.CreateTemp(filepath.Dir(path), ".buildybud-manifest-*")
	if err != nil {
		return fmt.Errorf("create staged manifest: %w", err)
	}
	tmpPath := tmp.Name()
	writeErr := tmp.Chmod(0o644)
	if writeErr == nil {
		_, writeErr = tmp.Write(data)
	}
	closeErr := tmp.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr == nil {
		writeErr = os.Rename(tmpPath, path)
	}
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write manifest: %w", writeErr)
	}
	return nil
}
