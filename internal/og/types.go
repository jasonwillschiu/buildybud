package og

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/fogleman/gg"
)

const (
	cardWidth  = 1200
	cardHeight = 630

	textLeft         = 76.0
	textColumnRight  = 600.0
	textMaxWidth     = textColumnRight - textLeft
	maxTitleLines    = 3
	maxSubtitleLines = 3
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
	SiteName       string
	SiteURL        string
}

func Render(assets Assets, card Card) (image.Image, error) {
	title := normalizeCardText(card.Title)
	if title == "" {
		return nil, fmt.Errorf("OG card title is required")
	}
	background, err := gg.LoadImage(assets.BackgroundPath)
	if err != nil {
		return nil, fmt.Errorf("load background: %w", err)
	}
	logo, err := gg.LoadImage(assets.LogoPath)
	if err != nil {
		return nil, fmt.Errorf("load logo: %w", err)
	}

	dc := gg.NewContext(cardWidth, cardHeight)
	drawCover(dc, background)
	drawWash(dc)
	if err := drawBrand(dc, logo, assets.FontPath, assets.SiteName); err != nil {
		return nil, err
	}
	if err := drawCardText(dc, assets.FontPath, title, card.Subtitle); err != nil {
		return nil, err
	}
	if err := drawFooterURL(dc, assets.FontPath, assets.SiteURL); err != nil {
		return nil, err
	}
	return dc.Image(), nil
}

func drawCover(dc *gg.Context, background image.Image) {
	bounds := background.Bounds()
	bw := float64(bounds.Dx())
	bh := float64(bounds.Dy())
	if bw <= 0 || bh <= 0 {
		return
	}
	scale := math.Max(float64(cardWidth)/bw, float64(cardHeight)/bh)
	scaledW := bw * scale
	scaledH := bh * scale
	dc.Push()
	dc.Translate((float64(cardWidth)-scaledW)/2, (float64(cardHeight)-scaledH)/2)
	dc.Scale(scale, scale)
	dc.DrawImage(background, bounds.Min.X, bounds.Min.Y)
	dc.Pop()
}

func drawWash(dc *gg.Context) {
	left := gg.NewLinearGradient(0, 0, 780, 0)
	left.AddColorStop(0, color.NRGBA{R: 250, G: 252, B: 255, A: 252})
	left.AddColorStop(0.62, color.NRGBA{R: 250, G: 252, B: 255, A: 232})
	left.AddColorStop(1, color.NRGBA{R: 250, G: 252, B: 255, A: 10})
	dc.SetFillStyle(left)
	dc.DrawRectangle(0, 0, cardWidth, cardHeight)
	dc.Fill()

	bottom := gg.NewLinearGradient(0, 400, 0, cardHeight)
	bottom.AddColorStop(0, color.NRGBA{R: 248, G: 251, B: 255, A: 0})
	bottom.AddColorStop(1, color.NRGBA{R: 236, G: 245, B: 255, A: 210})
	dc.SetFillStyle(bottom)
	dc.DrawRectangle(0, 360, cardWidth, cardHeight-360)
	dc.Fill()
}

func drawBrand(dc *gg.Context, logo image.Image, fontPath string, siteName string) error {
	bounds := logo.Bounds()
	lw := float64(bounds.Dx())
	lh := float64(bounds.Dy())
	if lw > 0 && lh > 0 {
		const logoSize = 62.0
		scale := math.Min(logoSize/lw, logoSize/lh)
		dc.Push()
		dc.Translate(textLeft, 57)
		dc.Scale(scale, scale)
		dc.DrawImage(logo, bounds.Min.X, bounds.Min.Y)
		dc.Pop()
	}

	if err := dc.LoadFontFace(fontPath, 26); err != nil {
		return fmt.Errorf("load OG brand font %q: %w", fontPath, err)
	}
	dc.SetColor(color.RGBA{R: 22, G: 47, B: 72, A: 255})
	dc.DrawString(strings.ToUpper(siteName), 154, 98)
	return nil
}

func drawCardText(dc *gg.Context, fontPath string, title string, subtitle string) error {
	titleLines, titleSize, err := fitLines(dc, fontPath, title, 68, 52, textMaxWidth, maxTitleLines)
	if err != nil {
		return err
	}
	dc.SetColor(color.RGBA{R: 22, G: 47, B: 72, A: 255})
	titleY := 214.0
	titleLineHeight := titleSize * 1.12
	for i, line := range titleLines {
		dc.DrawString(line, textLeft, titleY+float64(i)*titleLineHeight)
	}

	subtitle = normalizeCardText(subtitle)
	if subtitle == "" {
		return nil
	}
	subtitleY := titleY + float64(len(titleLines))*titleLineHeight + 26
	subtitleLines, subtitleSize, err := fitLines(dc, fontPath, subtitle, 29, 24, textMaxWidth, maxSubtitleLines)
	if err != nil {
		return err
	}
	dc.SetColor(color.RGBA{R: 67, G: 86, B: 104, A: 255})
	for i, line := range subtitleLines {
		dc.DrawString(line, textLeft, subtitleY+float64(i)*subtitleSize*1.34)
	}
	return nil
}

func drawFooterURL(dc *gg.Context, fontPath string, siteURL string) error {
	if strings.TrimSpace(siteURL) == "" {
		return nil
	}
	if err := dc.LoadFontFace(fontPath, 20); err != nil {
		return fmt.Errorf("load OG footer font %q: %w", fontPath, err)
	}
	dc.SetColor(color.RGBA{R: 45, G: 91, B: 132, A: 255})
	dc.DrawString(siteURL, textLeft, 578)
	return nil
}

func fitLines(dc *gg.Context, fontPath, text string, startSize, minSize, maxWidth float64, maxLines int) ([]string, float64, error) {
	for size := startSize; size >= minSize; size -= 2 {
		if err := dc.LoadFontFace(fontPath, size); err != nil {
			return nil, 0, fmt.Errorf("load OG font %q: %w", fontPath, err)
		}
		lines := dc.WordWrap(text, maxWidth)
		if len(lines) <= maxLines {
			return lines, size, nil
		}
	}

	if err := dc.LoadFontFace(fontPath, minSize); err != nil {
		return nil, 0, fmt.Errorf("load OG font %q: %w", fontPath, err)
	}
	lines := dc.WordWrap(text, maxWidth)
	if len(lines) <= maxLines {
		return lines, minSize, nil
	}
	lines = lines[:maxLines]
	lines[maxLines-1] = truncateLine(dc, lines[maxLines-1], maxWidth)
	return lines, minSize, nil
}

func truncateLine(dc *gg.Context, value string, maxWidth float64) string {
	runes := []rune(strings.TrimSpace(value))
	for len(runes) > 0 {
		candidate := strings.TrimSpace(string(runes)) + "..."
		if width, _ := dc.MeasureString(candidate); width <= maxWidth {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "..."
}

func normalizeCardText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
