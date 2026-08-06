package css

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type AuditGroup struct {
	Name            string
	Bundles         []string
	GzipBudgetBytes int
}

type AuditBundle struct {
	Name      string
	RawBytes  int64
	GzipBytes int64
}

type AuditGroupResult struct {
	Name      string
	RawBytes  int64
	GzipBytes int64
	Bundles   []AuditBundle
}

func Audit(bundlePaths map[string]string, groups []AuditGroup, writer io.Writer) ([]AuditGroupResult, error) {
	if writer == nil {
		writer = io.Discard
	}
	if len(groups) == 0 {
		names := make([]string, 0, len(bundlePaths))
		for name := range bundlePaths {
			names = append(names, name)
		}
		sort.Strings(names)
		groups = []AuditGroup{{Name: "all", Bundles: names}}
	}
	results := make([]AuditGroupResult, 0, len(groups))
	for _, group := range groups {
		result := AuditGroupResult{Name: group.Name}
		bundleNames := append([]string(nil), group.Bundles...)
		sort.Strings(bundleNames)
		for _, bundleName := range bundleNames {
			path, ok := bundlePaths[bundleName]
			if !ok {
				return nil, fmt.Errorf("CSS audit group %q is missing bundle %q", group.Name, bundleName)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read CSS audit bundle %s: %w", path, err)
			}
			var compressed bytes.Buffer
			zipper, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
			if err != nil {
				return nil, err
			}
			if _, err := zipper.Write(data); err != nil {
				return nil, err
			}
			if err := zipper.Close(); err != nil {
				return nil, err
			}
			bundle := AuditBundle{Name: bundleName, RawBytes: int64(len(data)), GzipBytes: int64(compressed.Len())}
			result.Bundles = append(result.Bundles, bundle)
			result.RawBytes += bundle.RawBytes
			result.GzipBytes += bundle.GzipBytes
		}
		results = append(results, result)
	}

	_, _ = fmt.Fprintf(writer, "%-18s %10s %10s  %s\n", "scope", "raw", "gzip", "bundle")
	for index, result := range results {
		for _, bundle := range result.Bundles {
			_, _ = fmt.Fprintf(writer, "%-18s %10d %10d  %s\n", result.Name, bundle.RawBytes, bundle.GzipBytes, bundle.Name)
		}
		budget := groups[index].GzipBudgetBytes
		if budget > 0 {
			_, _ = fmt.Fprintf(writer, "%s total: %d bytes gzip (budget %d)\n", result.Name, result.GzipBytes, budget)
		} else {
			_, _ = fmt.Fprintf(writer, "%s total: %d bytes gzip\n", result.Name, result.GzipBytes)
		}
	}
	for index, group := range groups {
		if group.GzipBudgetBytes > 0 && results[index].GzipBytes > int64(group.GzipBudgetBytes) {
			return results, fmt.Errorf("CSS audit group %q is %d bytes gzip, over budget %d", group.Name, results[index].GzipBytes, group.GzipBudgetBytes)
		}
	}
	return results, nil
}

func ManifestBundlePaths(manifestPath, assetsRoot string, logicalPaths map[string]string) (map[string]string, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read CSS manifest: %w", err)
	}
	manifest := map[string]string{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse CSS manifest: %w", err)
	}
	paths := make(map[string]string, len(logicalPaths))
	for name, logical := range logicalPaths {
		hashed, ok := manifest[logical]
		if !ok || hashed == "" {
			return nil, fmt.Errorf("CSS bundle %q is missing manifest asset %q", name, logical)
		}
		paths[name] = filepath.Join(assetsRoot, filepath.FromSlash(hashed))
	}
	return paths, nil
}
