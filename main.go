package main

import (
	_ "embed"
	"os"
	"strings"

	"github.com/jasonwillschiu/buildybud/internal/app"
	"github.com/jasonwillschiu/buildybud/internal/changelog"
)

//go:embed changelog.md
var embeddedChangelog string

func init() {
	entry, err := changelog.ParseLatestContent(embeddedChangelog, "embedded/changelog.md")
	if err != nil {
		return
	}
	version := entry.Version
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	app.ToolVersion = version
}

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
