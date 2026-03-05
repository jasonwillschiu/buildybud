package images

import (
	"encoding/json"
	"os"
)

type ImageManifest struct {
	Images map[string]*ProcessedImage `json:"images"`
}

type ProcessedImage struct {
	OriginalPath string
	Variants     map[string]map[int]string
	Hash         string
}

type Config struct {
	SourceDir string         `json:"sourceDir"`
	OutputDir string         `json:"outputDir"`
	Formats   []string       `json:"formats"`
	Sizes     []int          `json:"sizes"`
	Quality   map[string]int `json:"quality"`
}

func NewImageManifest() *ImageManifest {
	return &ImageManifest{Images: make(map[string]*ProcessedImage)}
}

func (im *ImageManifest) AddImage(processed *ProcessedImage) {
	base := processed.OriginalPath
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' || base[i] == '\\' {
			base = base[i+1:]
			break
		}
	}
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '.' {
			base = base[:i]
			break
		}
	}
	im.Images[base] = processed
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
