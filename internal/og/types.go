package og

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/fogleman/gg"
)

const (
	cardWidth  = 1200
	cardHeight = 630
)

type Options struct {
	RepoRoot        string
	SiteName        string
	SiteURL         string
	BackgroundPath  string
	LogoPath        string
	FontPath        string
	RoutesFile      string
	RouteOutputDir  string
	RouteManifest   string
	BlogDir         string
	BlogImageDir    string
	RawBlogImageDir string
	BlogManifest    string
}

type Card struct {
	Title    string
	Subtitle string
}

type Assets struct {
	BackgroundPath string
	LogoPath       string
	FontPath       string
}

func Render(assets Assets, card Card) (image.Image, error) {
	background, err := gg.LoadImage(assets.BackgroundPath)
	if err != nil {
		return nil, fmt.Errorf("load background: %w", err)
	}
	logo, err := gg.LoadImage(assets.LogoPath)
	if err != nil {
		return nil, fmt.Errorf("load logo: %w", err)
	}

	dc := gg.NewContext(cardWidth, cardHeight)
	drawBackground(dc, background)
	drawLogo(dc, logo)
	drawTitle(dc, assets.FontPath, card.Title)
	drawSubtitle(dc, assets.FontPath, card.Subtitle)
	return dc.Image(), nil
}

func drawBackground(dc *gg.Context, background image.Image) {
	bounds := background.Bounds()
	bw := float64(bounds.Dx())
	bh := float64(bounds.Dy())
	if bw <= 0 || bh <= 0 {
		dc.SetColor(color.RGBA{R: 244, G: 248, B: 248, A: 255})
		dc.Clear()
		return
	}
	scale := math.Max(float64(cardWidth)/bw, float64(cardHeight)/bh)
	dc.Push()
	dc.Translate(float64(cardWidth)/2, float64(cardHeight)/2)
	dc.Scale(scale, scale)
	dc.DrawImage(background, int(-bw/2), int(-bh/2))
	dc.Pop()
	dc.SetColor(color.NRGBA{R: 255, G: 255, B: 255, A: 180})
	dc.DrawRectangle(0, 0, cardWidth, cardHeight)
	dc.Fill()
}

func drawLogo(dc *gg.Context, logo image.Image) {
	const logoSize = 76.0
	const x = 78.0
	const y = 64.0
	bounds := logo.Bounds()
	lw := float64(bounds.Dx())
	lh := float64(bounds.Dy())
	if lw <= 0 || lh <= 0 {
		return
	}
	scale := math.Min(logoSize/lw, logoSize/lh)
	dc.Push()
	dc.Translate(x, y)
	dc.Scale(scale, scale)
	dc.DrawImage(logo, bounds.Min.X, bounds.Min.Y)
	dc.Pop()
}

func drawTitle(dc *gg.Context, fontPath string, title string) {
	if err := dc.LoadFontFace(fontPath, 68); err != nil {
		return
	}
	dc.SetColor(color.RGBA{R: 22, G: 47, B: 72, A: 255})
	dc.DrawStringWrapped(title, 76, 230, 0, 0, 560, 1.12, gg.AlignLeft)
}

func drawSubtitle(dc *gg.Context, fontPath string, subtitle string) {
	if err := dc.LoadFontFace(fontPath, 34); err != nil {
		return
	}
	dc.SetColor(color.RGBA{R: 56, G: 80, B: 96, A: 255})
	dc.DrawStringWrapped(subtitle, 80, 430, 0, 0, 610, 1.25, gg.AlignLeft)
}
