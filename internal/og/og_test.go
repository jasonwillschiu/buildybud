package og

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestRunRendersDeclarativeRoutesAndBlogCards(t *testing.T) {
	root := t.TempDir()
	writeTestPNG(t, filepath.Join(root, "assets", "src", "images", "background.png"), 1200, 630)
	writeTestPNG(t, filepath.Join(root, "assets", "src", "favicon", "logo.png"), 96, 96)
	writeFile(t, filepath.Join(root, "assets", "src", "fonts", "og", "font.ttf"), string(goregular.TTF))
	writeFile(t, filepath.Join(root, "content", "seo", "routes.toml"), `
[[route]]
path = "/"
image_name = "og-home"
title = "Find schools"
subtitle = "Compare clearly"
`)
	writeFile(t, filepath.Join(root, "content", "blog", "post.md"), `---
title: Blog Card
excerpt: A useful explanation.
hero_image: assets/src/images/blog/post-hero.png
hero_image_source: generated
---

Body.
`)

	stats, err := Run(context.Background(), Options{
		RepoRoot:        root,
		SiteName:        "Example",
		SiteURL:         "example.com",
		BackgroundPath:  filepath.Join(root, "assets", "src", "images", "background.png"),
		LogoPath:        filepath.Join(root, "assets", "src", "favicon", "logo.png"),
		FontPath:        filepath.Join(root, "assets", "src", "fonts", "og", "font.ttf"),
		RoutesFile:      filepath.Join(root, "content", "seo", "routes.toml"),
		RouteOutputDir:  filepath.Join(root, "assets", "src", "images", "og"),
		RouteManifest:   filepath.Join(root, "assets", "src", "images", "og", "generated-manifest.json"),
		BlogDir:         filepath.Join(root, "content", "blog"),
		BlogImageDir:    filepath.Join(root, "assets", "src", "images", "blog"),
		RawBlogImageDir: filepath.Join(root, "assets", "src", "raw-images", "blog"),
		BlogManifest:    filepath.Join(root, "assets", "src", "images", "blog", "generated-manifest.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Route.Processed != 1 || stats.Blog.Processed != 1 {
		t.Fatalf("stats = %#v", stats)
	}
	for _, path := range []string{
		filepath.Join(root, "assets", "src", "images", "og", "og-home.png"),
		filepath.Join(root, "assets", "src", "images", "blog", "post-hero.png"),
		filepath.Join(root, "assets", "src", "images", "og", "generated-manifest.json"),
		filepath.Join(root, "assets", "src", "images", "blog", "generated-manifest.json"),
	} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("expected non-empty generated file %s: info=%v err=%v", path, info, err)
		}
	}

	again, err := Run(context.Background(), Options{
		RepoRoot:        root,
		SiteName:        "Example",
		SiteURL:         "example.com",
		BackgroundPath:  filepath.Join(root, "assets", "src", "images", "background.png"),
		LogoPath:        filepath.Join(root, "assets", "src", "favicon", "logo.png"),
		FontPath:        filepath.Join(root, "assets", "src", "fonts", "og", "font.ttf"),
		RoutesFile:      filepath.Join(root, "content", "seo", "routes.toml"),
		RouteOutputDir:  filepath.Join(root, "assets", "src", "images", "og"),
		RouteManifest:   filepath.Join(root, "assets", "src", "images", "og", "generated-manifest.json"),
		BlogDir:         filepath.Join(root, "content", "blog"),
		BlogImageDir:    filepath.Join(root, "assets", "src", "images", "blog"),
		RawBlogImageDir: filepath.Join(root, "assets", "src", "raw-images", "blog"),
		BlogManifest:    filepath.Join(root, "assets", "src", "images", "blog", "generated-manifest.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.Route.Cached != 1 || again.Blog.Cached != 1 {
		t.Fatalf("second run stats = %#v", again)
	}
}

func TestRequestedBlogCardsSupportsLocalCustomHero(t *testing.T) {
	root := t.TempDir()
	writeTestPNG(t, filepath.Join(root, "assets", "src", "raw-images", "blog", "custom.png"), 1200, 630)
	writeFile(t, filepath.Join(root, "content", "blog", "custom.md"), `---
title: Custom
hero_image: assets/src/images/blog/custom.png
hero_image_source: custom
---
`)

	cards, err := requestedBlogCards(context.Background(), Options{
		RepoRoot:        root,
		BlogDir:         filepath.Join(root, "content", "blog"),
		BlogImageDir:    filepath.Join(root, "assets", "src", "images", "blog"),
		RawBlogImageDir: filepath.Join(root, "assets", "src", "raw-images", "blog"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || !cards[0].Custom {
		t.Fatalf("cards = %#v", cards)
	}
	if cards[0].SourcePath != filepath.Join(root, "assets", "src", "raw-images", "blog", "custom.png") {
		t.Fatalf("SourcePath = %q", cards[0].SourcePath)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 245, B: 245, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
