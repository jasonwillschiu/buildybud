package js

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type AuditOptions struct {
	RepoRoot          string
	AssetsRoot        string
	ManifestPath      string
	InlineDirs        []string
	MaxInlineRawBytes int
	Groups            []AuditGroup
	Writer            io.Writer
}

type AuditGroup struct {
	Name            string
	Assets          []string
	Files           []string
	GzipBudgetBytes int
}

type AuditResult struct {
	Groups      []AuditGroupResult
	InlineFiles []InlineResult
}

type AuditGroupResult struct {
	Name      string
	RawBytes  int64
	GzipBytes int64
	Assets    []AuditAssetResult
}

type AuditAssetResult struct {
	Name      string
	RawBytes  int64
	GzipBytes int64
}

type InlineResult struct {
	Path     string
	RawBytes int
}

var (
	inlineScriptPattern = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	scriptSrcPattern    = regexp.MustCompile(`(?i)\bsrc\s*=`)
	scriptJSONPattern   = regexp.MustCompile(`(?i)\btype\s*=\s*["'][^"']*json[^"']*["']`)
)

func Audit(opts AuditOptions) (AuditResult, error) {
	manifest, err := readManifest(opts.ManifestPath)
	if err != nil {
		return AuditResult{}, err
	}

	groups := append([]AuditGroup(nil), opts.Groups...)
	if len(groups) == 0 {
		assets := make([]string, 0, len(manifest))
		for logical := range manifest {
			if strings.HasPrefix(logical, "js/") {
				assets = append(assets, logical)
			}
		}
		sort.Strings(assets)
		groups = []AuditGroup{{Name: "all", Assets: assets}}
	}

	result := AuditResult{Groups: make([]AuditGroupResult, 0, len(groups))}
	for _, group := range groups {
		groupResult, measureErr := auditGroup(opts.RepoRoot, opts.AssetsRoot, manifest, group)
		if measureErr != nil {
			return AuditResult{}, measureErr
		}
		result.Groups = append(result.Groups, groupResult)
	}

	result.InlineFiles, err = scanInlineScripts(opts.RepoRoot, opts.InlineDirs)
	if err != nil {
		return AuditResult{}, err
	}

	writer := opts.Writer
	if writer == nil {
		writer = io.Discard
	}
	writeAudit(writer, result, groups, opts.MaxInlineRawBytes)

	for index, group := range groups {
		if group.GzipBudgetBytes > 0 && result.Groups[index].GzipBytes > int64(group.GzipBudgetBytes) {
			return result, fmt.Errorf("js audit group %q is %d bytes gzip, over budget %d", group.Name, result.Groups[index].GzipBytes, group.GzipBudgetBytes)
		}
	}
	if opts.MaxInlineRawBytes > 0 {
		for _, inline := range result.InlineFiles {
			if inline.RawBytes > opts.MaxInlineRawBytes {
				return result, fmt.Errorf("inline JavaScript in %s is %d raw bytes, over budget %d", inline.Path, inline.RawBytes, opts.MaxInlineRawBytes)
			}
		}
	}
	return result, nil
}

func auditGroup(repoRoot, assetsRoot string, manifest map[string]string, group AuditGroup) (AuditGroupResult, error) {
	result := AuditGroupResult{Name: group.Name}
	assets := append([]string(nil), group.Assets...)
	sort.Strings(assets)
	for _, logical := range assets {
		hashed, ok := manifest[logical]
		if !ok {
			return AuditGroupResult{}, fmt.Errorf("js audit group %q is missing manifest asset %q", group.Name, logical)
		}
		measured, err := measureAsset(logical, filepath.Join(assetsRoot, filepath.FromSlash(hashed)))
		if err != nil {
			return AuditGroupResult{}, err
		}
		result.Assets = append(result.Assets, measured)
		result.RawBytes += measured.RawBytes
		result.GzipBytes += measured.GzipBytes
	}

	files := append([]string(nil), group.Files...)
	sort.Strings(files)
	for _, path := range files {
		measured, err := measureAsset(displayPath(repoRoot, path), path)
		if err != nil {
			return AuditGroupResult{}, err
		}
		result.Assets = append(result.Assets, measured)
		result.RawBytes += measured.RawBytes
		result.GzipBytes += measured.GzipBytes
	}
	return result, nil
}

func measureAsset(name, path string) (AuditAssetResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AuditAssetResult{}, fmt.Errorf("read JavaScript audit asset %s: %w", path, err)
	}
	var compressed bytes.Buffer
	zipper, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return AuditAssetResult{}, err
	}
	if _, err := zipper.Write(data); err != nil {
		return AuditAssetResult{}, err
	}
	if err := zipper.Close(); err != nil {
		return AuditAssetResult{}, err
	}
	return AuditAssetResult{Name: name, RawBytes: int64(len(data)), GzipBytes: int64(compressed.Len())}, nil
}

func scanInlineScripts(repoRoot string, dirs []string) ([]InlineResult, error) {
	results := []InlineResult{}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat inline JavaScript directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("inline JavaScript path is not a directory: %s", dir)
		}
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !isTemplateFile(path) {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			total := 0
			for _, match := range inlineScriptPattern.FindAllSubmatch(data, -1) {
				attributes := match[1]
				if scriptSrcPattern.Match(attributes) || scriptJSONPattern.Match(attributes) {
					continue
				}
				total += len(match[2])
			}
			if total > 0 {
				results = append(results, InlineResult{Path: displayPath(repoRoot, path), RawBytes: total})
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("scan inline JavaScript in %s: %w", dir, err)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].RawBytes == results[j].RawBytes {
			return results[i].Path < results[j].Path
		}
		return results[i].RawBytes > results[j].RawBytes
	})
	return results, nil
}

func displayPath(repoRoot, path string) string {
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}

func isTemplateFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".templ", ".tmpl", ".gohtml":
		return true
	default:
		return false
	}
}

func writeAudit(writer io.Writer, result AuditResult, groups []AuditGroup, inlineBudget int) {
	_, _ = fmt.Fprintf(writer, "%-14s %10s %10s  %s\n", "scope", "raw", "gzip", "asset")
	for _, group := range result.Groups {
		for _, asset := range group.Assets {
			_, _ = fmt.Fprintf(writer, "%-14s %10d %10d  %s\n", group.Name, asset.RawBytes, asset.GzipBytes, asset.Name)
		}
	}
	for index, group := range result.Groups {
		budget := groups[index].GzipBudgetBytes
		if budget > 0 {
			_, _ = fmt.Fprintf(writer, "%s total: %d bytes gzip (budget %d)\n", group.Name, group.GzipBytes, budget)
		} else {
			_, _ = fmt.Fprintf(writer, "%s total: %d bytes gzip\n", group.Name, group.GzipBytes)
		}
	}
	_, _ = fmt.Fprintln(writer, "inline JavaScript by template (raw bytes):")
	if len(result.InlineFiles) == 0 {
		_, _ = fmt.Fprintln(writer, "none")
		return
	}
	for _, inline := range result.InlineFiles {
		if inlineBudget > 0 {
			_, _ = fmt.Fprintf(writer, "%10d  %s (budget %d)\n", inline.RawBytes, inline.Path, inlineBudget)
		} else {
			_, _ = fmt.Fprintf(writer, "%10d  %s\n", inline.RawBytes, inline.Path)
		}
	}
}
