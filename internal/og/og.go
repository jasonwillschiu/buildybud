package og

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/fogleman/gg"
	"gopkg.in/yaml.v2"
)

const (
	routeManifestVersion = 1
	blogManifestVersion  = 1
	rendererVersion      = "buildybud-og-v1"
)

type routesFile struct {
	Routes []routeEntry `toml:"route"`
}

type routeEntry struct {
	Path      string `toml:"path"`
	ImageName string `toml:"image_name"`
	Title     string `toml:"title"`
	Subtitle  string `toml:"subtitle"`
}

type requestedCard struct {
	Name       string
	OutputPath string
	Card       Card
	SourcePath string
	Custom     bool
}

type generatedManifest struct {
	Version int                   `json:"version"`
	Cards   map[string]cachedCard `json:"cards,omitempty"`
}

type cachedCard struct {
	Hash string `json:"hash"`
}

type renderStats struct {
	Processed int
	Cached    int
	Removed   int
}

type Stats struct {
	Route renderStats
	Blog  renderStats
}

func Run(ctx context.Context, opts Options) (Stats, error) {
	if err := validateOptions(opts); err != nil {
		return Stats{}, err
	}
	assets := Assets{BackgroundPath: opts.BackgroundPath, LogoPath: opts.LogoPath, FontPath: opts.FontPath}
	assetHash, err := hashAssets(assets, opts.SiteName, opts.SiteURL)
	if err != nil {
		return Stats{}, err
	}
	routeCards, err := requestedRouteCards(opts)
	if err != nil {
		return Stats{}, err
	}
	blogCards, err := requestedBlogCards(ctx, opts)
	if err != nil {
		return Stats{}, err
	}

	routeStats, err := renderCards(opts, assets, assetHash, opts.RouteManifest, routeManifestVersion, routeCards)
	if err != nil {
		return Stats{}, err
	}
	blogStats, err := renderCards(opts, assets, assetHash, opts.BlogManifest, blogManifestVersion, blogCards)
	if err != nil {
		return Stats{}, err
	}
	fmt.Printf("OG route images: Generated=%d Cached=%d Removed=%d Output=%s\n", routeStats.Processed, routeStats.Cached, routeStats.Removed, filepath.ToSlash(opts.RouteOutputDir))
	fmt.Printf("OG blog images: Generated=%d Cached=%d Removed=%d Output=%s\n", blogStats.Processed, blogStats.Cached, blogStats.Removed, filepath.ToSlash(opts.BlogImageDir))
	return Stats{Route: routeStats, Blog: blogStats}, nil
}

func validateOptions(opts Options) error {
	required := map[string]string{
		"site_url":           opts.SiteURL,
		"background":         opts.BackgroundPath,
		"logo":               opts.LogoPath,
		"font":               opts.FontPath,
		"routes_file":        opts.RoutesFile,
		"route_output_dir":   opts.RouteOutputDir,
		"route_manifest":     opts.RouteManifest,
		"blog_dir":           opts.BlogDir,
		"blog_image_dir":     opts.BlogImageDir,
		"raw_blog_image_dir": opts.RawBlogImageDir,
		"blog_manifest":      opts.BlogManifest,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("og %s is required", name)
		}
	}
	return nil
}

func requestedRouteCards(opts Options) ([]requestedCard, error) {
	var routes routesFile
	if _, err := toml.DecodeFile(opts.RoutesFile, &routes); err != nil {
		return nil, fmt.Errorf("read route OG file: %w", err)
	}
	cards := make([]requestedCard, 0, len(routes.Routes))
	seen := make(map[string]string, len(routes.Routes))
	for _, route := range routes.Routes {
		name := strings.TrimSpace(route.ImageName)
		if name == "" || strings.TrimSpace(route.Title) == "" {
			return nil, fmt.Errorf("route %q needs image_name and title", route.Path)
		}
		if previous, ok := seen[name]; ok {
			return nil, fmt.Errorf("routes %q and %q share image_name %q", previous, route.Path, name)
		}
		seen[name] = route.Path
		cards = append(cards, requestedCard{
			Name:       name,
			OutputPath: filepath.Join(opts.RouteOutputDir, name+".png"),
			Card:       Card{Title: route.Title, Subtitle: route.Subtitle},
		})
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Name < cards[j].Name })
	return cards, nil
}

func requestedBlogCards(_ context.Context, opts Options) ([]requestedCard, error) {
	paths, err := filepath.Glob(filepath.Join(opts.BlogDir, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	cards := make([]requestedCard, 0, len(paths))
	seen := make(map[string]string, len(paths))
	for _, path := range paths {
		meta, err := readFrontMatter(path)
		if err != nil {
			return nil, err
		}
		title := strings.TrimSpace(meta["title"])
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		subtitle := strings.TrimSpace(meta["excerpt"])
		if subtitle == "" {
			subtitle = strings.TrimSpace(meta["description"])
		}
		heroImage := strings.TrimSpace(meta["hero_image"])
		source := strings.TrimSpace(meta["hero_image_source"])
		if heroImage == "" {
			if source == "" {
				continue
			}
			return nil, fmt.Errorf("blog post %q needs hero_image when hero_image_source is set", path)
		}
		switch source {
		case "custom":
			if isAbsoluteURL(heroImage) {
				continue
			}
			outputPath, err := cleanBlogHeroPath(heroImage, opts.RepoRoot, opts.BlogImageDir)
			if err != nil {
				return nil, fmt.Errorf("blog post %q has invalid custom hero_image: %w", path, err)
			}
			sourcePath := customHeroRawPath(outputPath, opts.BlogImageDir, opts.RawBlogImageDir)
			if _, err := os.Stat(sourcePath); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("blog post %q has custom hero_image %q but missing raw source %q", path, heroImage, sourcePath)
				}
				return nil, fmt.Errorf("blog post %q raw custom hero source %q is not readable: %w", path, sourcePath, err)
			}
			if previous, ok := seen[outputPath]; ok {
				return nil, fmt.Errorf("blog posts %q and %q share hero image %q", previous, path, outputPath)
			}
			seen[outputPath] = path
			cards = append(cards, requestedCard{
				Name:       strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath)),
				OutputPath: outputPath,
				SourcePath: sourcePath,
				Custom:     true,
			})
		case "generated":
			if isAbsoluteURL(heroImage) {
				return nil, fmt.Errorf("blog post %q has generated hero_image_source but absolute hero_image %q", path, heroImage)
			}
			outputPath, err := cleanBlogHeroPath(heroImage, opts.RepoRoot, opts.BlogImageDir)
			if err != nil {
				return nil, fmt.Errorf("blog post %q has invalid hero_image: %w", path, err)
			}
			if previous, ok := seen[outputPath]; ok {
				return nil, fmt.Errorf("blog posts %q and %q share hero image %q", previous, path, outputPath)
			}
			seen[outputPath] = path
			cards = append(cards, requestedCard{
				Name:       strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath)),
				OutputPath: outputPath,
				Card:       Card{Title: title, Subtitle: subtitle},
			})
		case "":
			return nil, fmt.Errorf("blog post %q needs hero_image_source generated or custom", path)
		default:
			return nil, fmt.Errorf("blog post %q has invalid hero_image_source %q", path, source)
		}
	}
	return cards, nil
}

func readFrontMatter(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read blog post %q: %w", path, err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return map[string]string{}, nil
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("blog post %q has unterminated front matter", path)
	}
	raw := rest[:end]
	values := map[string]any{}
	if err := yaml.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("parse blog front matter %q: %w", path, err)
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		if value == nil {
			continue
		}
		out[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	return out, nil
}

func renderCards(opts Options, assets Assets, assetHash string, manifestPath string, version int, cards []requestedCard) (renderStats, error) {
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return renderStats{}, fmt.Errorf("create manifest directory: %w", err)
	}
	previous, err := readManifest(manifestPath)
	if err != nil {
		return renderStats{}, err
	}
	current := generatedManifest{Version: version, Cards: make(map[string]cachedCard, len(cards))}
	generatedPaths := make([]string, 0, len(cards))
	stats := renderStats{}
	for _, requested := range cards {
		path := filepath.Clean(requested.OutputPath)
		key := manifestKey(opts.RepoRoot, path)
		hash := hashCard(assetHash, requested)
		current.Cards[key] = cachedCard{Hash: hash}
		generatedPaths = append(generatedPaths, key)
		if previous.Version == version && previous.Cards[key].Hash == hash {
			if _, err := os.Stat(path); err == nil {
				stats.Cached++
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return renderStats{}, fmt.Errorf("create output directory for %s: %w", requested.Name, err)
		}
		var img image.Image
		var err error
		if requested.Custom {
			img, err = renderCustomHero(assets, opts.SiteName, opts.SiteURL, requested.SourcePath)
		} else {
			img, err = Render(assets, requested.Card)
		}
		if err != nil {
			return renderStats{}, fmt.Errorf("render %s: %w", requested.Name, err)
		}
		if err := writePNG(path, img); err != nil {
			return renderStats{}, fmt.Errorf("write %s: %w", requested.Name, err)
		}
		stats.Processed++
	}
	removed, err := removeStaleGeneratedFiles(opts.RepoRoot, previous, generatedPaths)
	if err != nil {
		return renderStats{}, err
	}
	stats.Removed = removed
	if err := writeManifest(manifestPath, current); err != nil {
		return renderStats{}, err
	}
	return stats, nil
}

func hashAssets(assets Assets, siteName string, siteURL string) (string, error) {
	hash := sha256.New()
	_, _ = hash.Write([]byte(rendererVersion))
	_, _ = hash.Write([]byte(siteName))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(siteURL))
	for _, path := range []string{assets.BackgroundPath, assets.FontPath, assets.LogoPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("hash OG asset %q: %w", path, err)
		}
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(data)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func hashCard(assetHash string, requested requestedCard) string {
	hash := sha256.New()
	sourceHash := ""
	if requested.SourcePath != "" {
		if data, err := os.ReadFile(requested.SourcePath); err == nil {
			sum := sha256.Sum256(data)
			sourceHash = fmt.Sprintf("%x", sum[:])
		}
	}
	for _, value := range []string{assetHash, requested.Name, requested.OutputPath, requested.Card.Title, requested.Card.Subtitle, requested.SourcePath, sourceHash, fmt.Sprintf("%t", requested.Custom)} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func renderCustomHero(assets Assets, siteName string, siteURL string, sourcePath string) (image.Image, error) {
	source, err := gg.LoadImage(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("load custom hero %q: %w", sourcePath, err)
	}
	logo, err := gg.LoadImage(assets.LogoPath)
	if err != nil {
		return nil, fmt.Errorf("load OG logo %q: %w", assets.LogoPath, err)
	}
	bounds := source.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("custom hero %q has empty bounds", sourcePath)
	}
	dc := gg.NewContext(w, h)
	dc.DrawImage(source, bounds.Min.X, bounds.Min.Y)
	drawCustomBrand(dc, logo, assets.FontPath, siteName, float64(w))
	drawCustomURL(dc, assets.FontPath, siteURL, float64(w), float64(h))
	return dc.Image(), nil
}

func drawCustomBrand(dc *gg.Context, logo image.Image, fontPath string, siteName string, w float64) {
	pad := math.Round(w * 0.045)
	logoSize := math.Round(w * 0.055)
	if logoSize < 56 {
		logoSize = 56
	}
	if logoSize > 92 {
		logoSize = 92
	}
	badgeH := logoSize + 26
	badgeW := logoSize + math.Round(w*0.205)
	drawSoftBadge(dc, pad-16, pad-16, badgeW, badgeH)
	bounds := logo.Bounds()
	lw := float64(bounds.Dx())
	lh := float64(bounds.Dy())
	if lw > 0 && lh > 0 {
		scale := math.Min(logoSize/lw, logoSize/lh)
		dc.Push()
		dc.Translate(pad, pad)
		dc.Scale(scale, scale)
		dc.DrawImage(logo, bounds.Min.X, bounds.Min.Y)
		dc.Pop()
	}
	fontSize := math.Round(w * 0.021)
	if fontSize < 22 {
		fontSize = 22
	}
	if fontSize > 34 {
		fontSize = 34
	}
	if err := dc.LoadFontFace(fontPath, fontSize); err != nil {
		return
	}
	if strings.TrimSpace(siteName) == "" {
		siteName = "SITE"
	}
	dc.SetColor(color.RGBA{R: 22, G: 47, B: 72, A: 255})
	dc.DrawStringAnchored(strings.ToUpper(siteName), pad+logoSize+18, pad+logoSize*0.56, 0, 0.5)
}

func drawCustomURL(dc *gg.Context, fontPath string, siteURL string, w, h float64) {
	fontSize := math.Round(w * 0.021)
	if fontSize < 22 {
		fontSize = 22
	}
	if fontSize > 34 {
		fontSize = 34
	}
	if err := dc.LoadFontFace(fontPath, fontSize); err != nil {
		return
	}
	textW, textH := dc.MeasureString(siteURL)
	pad := math.Round(w * 0.045)
	badgePadX := 20.0
	badgePadY := 13.0
	x := w - pad - textW - badgePadX*2
	y := h - pad - textH - badgePadY*2
	drawSoftBadge(dc, x, y, textW+badgePadX*2, textH+badgePadY*2)
	dc.SetColor(color.RGBA{R: 22, G: 47, B: 72, A: 255})
	dc.DrawString(siteURL, x+badgePadX, y+badgePadY+textH)
}

func drawSoftBadge(dc *gg.Context, x, y, w, h float64) {
	dc.SetColor(color.NRGBA{R: 255, G: 252, B: 244, A: 225})
	dc.DrawRoundedRectangle(x, y, w, h, 18)
	dc.Fill()
	dc.SetColor(color.NRGBA{R: 22, G: 47, B: 72, A: 34})
	dc.SetLineWidth(1.5)
	dc.DrawRoundedRectangle(x, y, w, h, 18)
	dc.Stroke()
}

func writePNG(path string, img image.Image) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".og-*.png")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := png.Encode(tmp, img); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func readManifest(path string) (generatedManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return generatedManifest{}, nil
		}
		return generatedManifest{}, fmt.Errorf("read previous generated manifest: %w", err)
	}
	var manifest generatedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return generatedManifest{}, fmt.Errorf("parse previous generated manifest: %w", err)
	}
	return manifest, nil
}

func writeManifest(path string, manifest generatedManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode generated manifest: %w", err)
	}
	data = append(data, '\n')
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		return nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write generated manifest: %w", err)
	}
	return nil
}

func removeStaleGeneratedFiles(repoRoot string, previous generatedManifest, current []string) (int, error) {
	keep := make(map[string]struct{}, len(current))
	for _, path := range current {
		keep[filepath.Clean(path)] = struct{}{}
	}
	removed := 0
	for name := range previous.Cards {
		path := filepath.Clean(name)
		if _, ok := keep[path]; ok {
			continue
		}
		if filepath.IsAbs(path) || filepath.Ext(path) != ".png" {
			return 0, fmt.Errorf("refuse unsafe generated filename %q", name)
		}
		removePath := path
		if repoRoot != "" {
			removePath = filepath.Join(repoRoot, path)
		}
		if err := os.Remove(removePath); err != nil && !os.IsNotExist(err) {
			return 0, fmt.Errorf("remove stale generated file %q: %w", name, err)
		}
		removed++
	}
	return removed, nil
}

func manifestKey(repoRoot string, path string) string {
	clean := filepath.Clean(path)
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, clean); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			clean = rel
		}
	}
	return filepath.ToSlash(clean)
}

func cleanBlogHeroPath(value string, repoRoot string, blogImageDir string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")))
	if !filepath.IsAbs(clean) && repoRoot != "" {
		clean = filepath.Join(repoRoot, clean)
	}
	if !isPathUnder(clean, blogImageDir) {
		return "", fmt.Errorf("must be under %s, got %q", filepath.ToSlash(blogImageDir), value)
	}
	if filepath.Ext(clean) != ".png" {
		return "", fmt.Errorf("must be a PNG source image, got %q", value)
	}
	return clean, nil
}

func customHeroRawPath(outputPath string, blogImageDir string, rawBlogImageDir string) string {
	rel, err := filepath.Rel(blogImageDir, outputPath)
	if err != nil {
		return outputPath
	}
	return filepath.Join(rawBlogImageDir, rel)
}

func isPathUnder(path, dir string) bool {
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func isAbsoluteURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}
