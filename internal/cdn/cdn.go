package cdn

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	bunnyPurgeEndpoint      = "https://api.bunny.net/purge"
	cloudflarePurgeEndpoint = "https://api.cloudflare.com/client/v4/zones/%s/purge_cache"
)

type Provider string

const (
	ProviderBunny      Provider = "bunny"
	ProviderCloudflare Provider = "cloudflare"
)

type contentKind string

const (
	kindBlog    contentKind = "blog"
	kindProject contentKind = "project"
	kindLegal   contentKind = "legal"
)

type contentMeta struct {
	Kind   contentKind
	Path   string
	Slug   string
	Tags   []string
	Draft  bool
	Exists bool
}

type diffEntry struct {
	Status  string
	Path    string
	OldPath string
}

type planner struct {
	fromRef string
	toRef   string
	verbose bool
}

type PlanResult struct {
	Paths   []string `json:"paths"`
	URLs    []string `json:"urls"`
	Reasons []string `json:"reasons"`
}

type stringSet map[string]struct{}

type purgeConfig struct {
	Provider         Provider
	BaseURL          string
	PurgeHosts       string
	BunnyAPIKey      string
	CloudflareAPITok string
	CloudflareZoneID string
	DryRun           bool
}

func RootHelp() string {
	return `CDN commands:
  buildybud cdn plan --from-ref <ref> [--to-ref HEAD] [--provider bunny|cloudflare]
  buildybud cdn purge [--provider bunny|cloudflare] <path-or-url>...
  buildybud cdn plan-and-purge --from-ref <ref> [--provider bunny|cloudflare]

Environment:
  APP_BASE_URL         Required for plan and for purge when using relative paths
  CDN_PROVIDER         Optional default provider: bunny or cloudflare
  CDN_PURGE_HOSTS      Optional comma-separated hostnames to purge
  BUNNY_API_KEY        Required when provider=bunny
  CF_API_TOKEN         Required when provider=cloudflare
  CF_ZONE_ID           Required when provider=cloudflare
`
}

func Plan(args []string) error {
	fs := flag.NewFlagSet("buildybud cdn plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildybud cdn plan --from-ref <ref> [flags]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, RootHelp())
	}
	fromRef := fs.String("from-ref", "", "Git ref representing the currently deployed revision")
	toRef := fs.String("to-ref", "HEAD", "Git ref representing the target revision")
	baseURL := fs.String("base-url", envOrDefault("APP_BASE_URL", ""), "Canonical site base URL")
	purgeHosts := fs.String("purge-hosts", envOrDefault("CDN_PURGE_HOSTS", ""), "Comma-separated hostnames to purge")
	provider := fs.String("provider", string(defaultProvider()), "CDN provider: bunny or cloudflare")
	verbose := fs.Bool("verbose", false, "Print route planning reasons")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*fromRef) == "" {
		return errors.New("--from-ref is required")
	}
	if _, err := parseProvider(*provider); err != nil {
		return err
	}

	result, err := buildPlan(*fromRef, *toRef, *baseURL, *purgeHosts, *verbose)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func Purge(args []string) error {
	fs := flag.NewFlagSet("buildybud cdn purge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildybud cdn purge [flags] <path-or-url>...")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, RootHelp())
	}
	provider := fs.String("provider", string(defaultProvider()), "CDN provider: bunny or cloudflare")
	baseURL := fs.String("base-url", envOrDefault("APP_BASE_URL", ""), "Canonical site base URL")
	purgeHosts := fs.String("purge-hosts", envOrDefault("CDN_PURGE_HOSTS", ""), "Comma-separated hostnames to purge")
	bunnyAPIKey := fs.String("bunny-api-key", envOrDefault("BUNNY_API_KEY", ""), "Bunny API key")
	cloudflareToken := fs.String("cf-api-token", envOrDefault("CF_API_TOKEN", ""), "Cloudflare API token")
	cloudflareZoneID := fs.String("cf-zone-id", envOrDefault("CF_ZONE_ID", ""), "Cloudflare zone ID")
	dryRun := fs.Bool("dry-run", false, "Print URLs without calling the CDN provider")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	parsedProvider, err := parseProvider(*provider)
	if err != nil {
		return err
	}
	cfg := purgeConfig{
		Provider:         parsedProvider,
		BaseURL:          *baseURL,
		PurgeHosts:       *purgeHosts,
		BunnyAPIKey:      *bunnyAPIKey,
		CloudflareAPITok: *cloudflareToken,
		CloudflareZoneID: *cloudflareZoneID,
		DryRun:           *dryRun,
	}

	if fs.NArg() == 0 {
		return errors.New("purge requires at least one path or absolute URL argument")
	}

	paths := make([]string, 0, fs.NArg())
	absoluteURLs := make([]string, 0, fs.NArg())
	for _, arg := range fs.Args() {
		if isAbsoluteURL(arg) {
			absoluteURLs = append(absoluteURLs, strings.TrimSpace(arg))
			continue
		}
		paths = append(paths, normalizePath(cfg.BaseURL, arg))
	}
	urls, err := buildPurgeURLs(paths, absoluteURLs, cfg.BaseURL, cfg.PurgeHosts)
	if err != nil {
		return err
	}
	return purgeURLs(urls, cfg)
}

func PlanAndPurge(args []string) error {
	fs := flag.NewFlagSet("buildybud cdn plan-and-purge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: buildybud cdn plan-and-purge --from-ref <ref> [flags]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, RootHelp())
	}
	fromRef := fs.String("from-ref", "", "Git ref representing the currently deployed revision")
	toRef := fs.String("to-ref", "HEAD", "Git ref representing the target revision")
	baseURL := fs.String("base-url", envOrDefault("APP_BASE_URL", ""), "Canonical site base URL")
	purgeHosts := fs.String("purge-hosts", envOrDefault("CDN_PURGE_HOSTS", ""), "Comma-separated hostnames to purge")
	provider := fs.String("provider", string(defaultProvider()), "CDN provider: bunny or cloudflare")
	bunnyAPIKey := fs.String("bunny-api-key", envOrDefault("BUNNY_API_KEY", ""), "Bunny API key")
	cloudflareToken := fs.String("cf-api-token", envOrDefault("CF_API_TOKEN", ""), "Cloudflare API token")
	cloudflareZoneID := fs.String("cf-zone-id", envOrDefault("CF_ZONE_ID", ""), "Cloudflare zone ID")
	dryRun := fs.Bool("dry-run", false, "Print URLs without calling the CDN provider")
	verbose := fs.Bool("verbose", false, "Print route planning reasons")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*fromRef) == "" {
		return errors.New("--from-ref is required")
	}

	parsedProvider, err := parseProvider(*provider)
	if err != nil {
		return err
	}
	cfg := purgeConfig{
		Provider:         parsedProvider,
		BaseURL:          *baseURL,
		PurgeHosts:       *purgeHosts,
		BunnyAPIKey:      *bunnyAPIKey,
		CloudflareAPITok: *cloudflareToken,
		CloudflareZoneID: *cloudflareZoneID,
		DryRun:           *dryRun,
	}

	result, err := buildPlan(*fromRef, *toRef, cfg.BaseURL, cfg.PurgeHosts, *verbose)
	if err != nil {
		return err
	}
	if *verbose {
		for _, reason := range result.Reasons {
			fmt.Fprintln(os.Stderr, reason)
		}
	}
	return purgeURLs(result.URLs, cfg)
}

func buildPlan(fromRef, toRef, baseURL, purgeHosts string, verbose bool) (*PlanResult, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("APP_BASE_URL or --base-url is required")
	}

	p := planner{fromRef: fromRef, toRef: toRef, verbose: verbose}
	diff, err := p.diffEntries()
	if err != nil {
		return nil, err
	}

	paths := newStringSet("/sitemap.xml")
	reasons := make([]string, 0)
	fullSite := false

	for _, entry := range diff {
		switch {
		case entry.Path == "AppConstants.toml" || entry.Path == "core/router/router.go":
			fullSite = true
			reasons = append(reasons, fmt.Sprintf("full site purge: route/config change in %s", entry.Path))
		case isSharedSiteFile(entry.Path) || isSharedSiteFile(entry.OldPath):
			fullSite = true
			path := entry.Path
			if path == "" {
				path = entry.OldPath
			}
			reasons = append(reasons, fmt.Sprintf("full site purge: shared rendering change in %s", path))
		case isBlogContentPath(entry.Path) || isBlogContentPath(entry.OldPath):
			if err := p.expandContentChange(paths, &reasons, entry, kindBlog, "/blog"); err != nil {
				return nil, err
			}
		case isProjectContentPath(entry.Path) || isProjectContentPath(entry.OldPath):
			if err := p.expandContentChange(paths, &reasons, entry, kindProject, "/projects"); err != nil {
				return nil, err
			}
		case isLegalContentPath(entry.Path) || isLegalContentPath(entry.OldPath):
			if err := p.expandLegalChange(paths, &reasons, entry); err != nil {
				return nil, err
			}
		case isFeatureHomeFile(entry.Path):
			paths.add("/")
			reasons = append(reasons, fmt.Sprintf("purge /: home feature changed in %s", entry.Path))
		case isFeatureAboutFile(entry.Path):
			paths.add("/about")
			reasons = append(reasons, fmt.Sprintf("purge /about: about feature changed in %s", entry.Path))
		case isFeatureBlogFile(entry.Path):
			if err := p.expandAllBlogRoutes(paths, &reasons, entry.Path); err != nil {
				return nil, err
			}
		case isFeaturePortfolioFile(entry.Path):
			if err := p.expandAllProjectRoutes(paths, &reasons, entry.Path); err != nil {
				return nil, err
			}
		case isFeatureTagsFile(entry.Path):
			if err := p.expandAllTagRoutes(paths, &reasons, entry.Path); err != nil {
				return nil, err
			}
		case isFeatureLegalFile(entry.Path):
			paths.add("/privacy", "/terms", "/sitemap.xml")
			reasons = append(reasons, fmt.Sprintf("purge legal routes: legal feature changed in %s", entry.Path))
		}
	}

	if fullSite {
		if err := p.expandAllRoutes(paths); err != nil {
			return nil, err
		}
	}

	if paths.len() == 1 && paths.has("/sitemap.xml") {
		reasons = append(reasons, "only /sitemap.xml scheduled; no relevant public-route changes detected")
	}

	sortedPaths := paths.sorted()
	urls, err := buildURLs(sortedPaths, baseURL, purgeHosts)
	if err != nil {
		return nil, err
	}
	return &PlanResult{Paths: sortedPaths, URLs: urls, Reasons: reasons}, nil
}

func (p planner) diffEntries() ([]diffEntry, error) {
	output, err := gitOutput("diff", "--name-status", "--find-renames", p.fromRef, p.toRef)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	entries := make([]diffEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("unexpected git diff output: %q", line)
		}
		entry := diffEntry{Status: fields[0]}
		switch {
		case strings.HasPrefix(fields[0], "R") || strings.HasPrefix(fields[0], "C"):
			if len(fields) < 3 {
				return nil, fmt.Errorf("unexpected rename diff output: %q", line)
			}
			entry.OldPath = fields[1]
			entry.Path = fields[2]
		default:
			entry.Path = fields[1]
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (p planner) expandContentChange(paths stringSet, reasons *[]string, entry diffEntry, kind contentKind, prefix string) error {
	oldMeta, err := p.metaForChange(p.fromRef, entry.OldPath, entry.Path, kind)
	if err != nil {
		return err
	}
	newMeta, err := p.metaForChange(p.toRef, entry.Path, entry.OldPath, kind)
	if err != nil {
		return err
	}

	paths.add("/", prefix, "/tags", "/sitemap.xml")
	for _, meta := range []contentMeta{oldMeta, newMeta} {
		if !meta.Exists || meta.Draft || meta.Slug == "" {
			continue
		}
		paths.add(prefix + "/" + meta.Slug)
		for _, tag := range meta.Tags {
			paths.add("/tags/" + tag)
		}
	}

	changedPath := entry.Path
	if changedPath == "" {
		changedPath = entry.OldPath
	}
	*reasons = append(*reasons, fmt.Sprintf("purge %s routes: content changed in %s", prefix, changedPath))
	return nil
}

func (p planner) expandLegalChange(paths stringSet, reasons *[]string, entry diffEntry) error {
	oldMeta, err := p.metaForChange(p.fromRef, entry.OldPath, entry.Path, kindLegal)
	if err != nil {
		return err
	}
	newMeta, err := p.metaForChange(p.toRef, entry.Path, entry.OldPath, kindLegal)
	if err != nil {
		return err
	}

	for _, meta := range []contentMeta{oldMeta, newMeta} {
		if !meta.Exists || meta.Draft || meta.Slug == "" {
			continue
		}
		paths.add("/" + meta.Slug)
	}
	paths.add("/sitemap.xml")
	changedPath := entry.Path
	if changedPath == "" {
		changedPath = entry.OldPath
	}
	*reasons = append(*reasons, fmt.Sprintf("purge legal routes: content changed in %s", changedPath))
	return nil
}

func (p planner) expandAllBlogRoutes(paths stringSet, reasons *[]string, source string) error {
	paths.add("/", "/blog", "/tags", "/sitemap.xml")
	items, err := p.allContentAtRef(p.toRef, kindBlog)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Draft || item.Slug == "" {
			continue
		}
		paths.add("/blog/" + item.Slug)
		for _, tag := range item.Tags {
			paths.add("/tags/" + tag)
		}
	}
	*reasons = append(*reasons, fmt.Sprintf("purge all blog routes: blog feature changed in %s", source))
	return nil
}

func (p planner) expandAllProjectRoutes(paths stringSet, reasons *[]string, source string) error {
	paths.add("/", "/projects", "/tags", "/sitemap.xml")
	items, err := p.allContentAtRef(p.toRef, kindProject)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Draft || item.Slug == "" {
			continue
		}
		paths.add("/projects/" + item.Slug)
		for _, tag := range item.Tags {
			paths.add("/tags/" + tag)
		}
	}
	*reasons = append(*reasons, fmt.Sprintf("purge all project routes: portfolio feature changed in %s", source))
	return nil
}

func (p planner) expandAllTagRoutes(paths stringSet, reasons *[]string, source string) error {
	paths.add("/tags", "/sitemap.xml")
	for _, ref := range []string{p.fromRef, p.toRef} {
		tags, err := p.allTagsAtRef(ref)
		if err != nil {
			return err
		}
		for tag := range tags {
			paths.add("/tags/" + tag)
		}
	}
	*reasons = append(*reasons, fmt.Sprintf("purge all tag routes: tags feature changed in %s", source))
	return nil
}

func (p planner) expandAllRoutes(paths stringSet) error {
	paths.add("/", "/about", "/blog", "/projects", "/tags", "/privacy", "/terms", "/sitemap.xml", "/robots.txt")
	for _, ref := range []string{p.fromRef, p.toRef} {
		for _, kind := range []contentKind{kindBlog, kindProject, kindLegal} {
			items, err := p.allContentAtRef(ref, kind)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.Draft || item.Slug == "" {
					continue
				}
				switch kind {
				case kindBlog:
					paths.add("/blog/" + item.Slug)
				case kindProject:
					paths.add("/projects/" + item.Slug)
				case kindLegal:
					paths.add("/" + item.Slug)
				}
				for _, tag := range item.Tags {
					paths.add("/tags/" + tag)
				}
			}
		}
	}
	return nil
}

func (p planner) metaForChange(ref, primaryPath, fallbackPath string, kind contentKind) (contentMeta, error) {
	path := primaryPath
	if path == "" {
		path = fallbackPath
	}
	if path == "" {
		return contentMeta{Kind: kind}, nil
	}
	blob, err := gitShow(ref, path)
	if err != nil {
		if isMissingGitObject(err) {
			return contentMeta{Kind: kind, Path: path}, nil
		}
		return contentMeta{}, err
	}
	meta, err := parseContentMeta(path, []byte(blob), kind)
	if err != nil {
		return contentMeta{}, err
	}
	meta.Exists = true
	return meta, nil
}

func (p planner) allContentAtRef(ref string, kind contentKind) ([]contentMeta, error) {
	root := kindRoot(kind)
	output, err := gitOutput("ls-tree", "-r", "--name-only", ref, "--", root)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	items := make([]contentMeta, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || filepath.Ext(line) != ".md" {
			continue
		}
		blob, err := gitShow(ref, line)
		if err != nil {
			return nil, err
		}
		meta, err := parseContentMeta(line, []byte(blob), kind)
		if err != nil {
			return nil, err
		}
		meta.Exists = true
		items = append(items, meta)
	}
	return items, nil
}

func (p planner) allTagsAtRef(ref string) (stringSet, error) {
	tags := make(stringSet)
	for _, kind := range []contentKind{kindBlog, kindProject} {
		items, err := p.allContentAtRef(ref, kind)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.Draft {
				continue
			}
			for _, tag := range item.Tags {
				tags.add(tag)
			}
		}
	}
	return tags, nil
}

func parseContentMeta(path string, data []byte, kind contentKind) (contentMeta, error) {
	meta := contentMeta{Kind: kind, Path: path, Slug: slugFromPath(path)}
	fm, err := parseFrontmatter(data)
	if err != nil {
		return contentMeta{}, fmt.Errorf("parse frontmatter for %s: %w", path, err)
	}
	if slug := stringValue(fm["slug"]); slug != "" {
		meta.Slug = slug
	}
	meta.Tags = normalizeTags(sliceValue(fm["tags"]))
	meta.Draft = boolValue(fm["draft"])
	return meta, nil
}

func parseFrontmatter(data []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if !bytes.HasPrefix(trimmed, []byte("---\n")) && !bytes.HasPrefix(trimmed, []byte("---\r\n")) {
		return map[string]any{}, nil
	}

	lines := strings.Split(string(trimmed), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]any{}, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, errors.New("missing frontmatter terminator")
	}

	var meta map[string]any
	body := strings.Join(lines[1:end], "\n")
	if err := yaml.Unmarshal([]byte(body), &meta); err != nil {
		return nil, err
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return meta, nil
}

func buildURLs(paths []string, baseURL, purgeHosts string) ([]string, error) {
	canonical, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if canonical.Scheme == "" || canonical.Host == "" {
		return nil, fmt.Errorf("APP_BASE_URL or --base-url must include scheme and host when purging relative paths: %q", baseURL)
	}

	hosts := resolvePurgeHosts(canonical, purgeHosts)
	urls := make(stringSet)
	for _, path := range paths {
		for _, host := range hosts {
			u := *canonical
			u.Host = host
			u.Path = path
			u.RawPath = ""
			u.RawQuery = ""
			u.Fragment = ""
			urls.add(u.String())
		}
	}
	return urls.sorted(), nil
}

func buildPurgeURLs(paths, absoluteURLs []string, baseURL, purgeHosts string) ([]string, error) {
	urls := newStringSet(absoluteURLs...)
	if len(paths) == 0 {
		return urls.sorted(), nil
	}
	resolved, err := buildURLs(paths, baseURL, purgeHosts)
	if err != nil {
		return nil, err
	}
	urls.add(resolved...)
	return urls.sorted(), nil
}

func purgeURLs(urls []string, cfg purgeConfig) error {
	parsedProvider, err := parseProvider(string(cfg.Provider))
	if err != nil {
		return err
	}
	cfg.Provider = parsedProvider
	if cfg.DryRun {
		for _, target := range urls {
			fmt.Println(target)
		}
		return nil
	}

	switch cfg.Provider {
	case ProviderBunny:
		if strings.TrimSpace(cfg.BunnyAPIKey) == "" {
			return errors.New("BUNNY_API_KEY or --bunny-api-key is required when --provider=bunny")
		}
		return purgeBunnyURLs(urls, cfg.BunnyAPIKey)
	case ProviderCloudflare:
		if strings.TrimSpace(cfg.CloudflareAPITok) == "" {
			return errors.New("CF_API_TOKEN or --cf-api-token is required when --provider=cloudflare")
		}
		if strings.TrimSpace(cfg.CloudflareZoneID) == "" {
			return errors.New("CF_ZONE_ID or --cf-zone-id is required when --provider=cloudflare")
		}
		return purgeCloudflareURLs(urls, cfg.CloudflareAPITok, cfg.CloudflareZoneID)
	default:
		return fmt.Errorf("unsupported CDN provider: %s", cfg.Provider)
	}
}

func purgeBunnyURLs(urls []string, bunnyAPIKey string) error {
	client := &http.Client{}
	for _, target := range urls {
		endpoint := bunnyPurgeEndpoint + "?url=" + url.QueryEscape(target) + "&async=false"
		req, err := http.NewRequest(http.MethodPost, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("AccessKey", bunnyAPIKey)

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("purge %s: %w", target, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("purge %s: bunny returned %s: %s", target, resp.Status, strings.TrimSpace(string(body)))
		}
		fmt.Printf("Purged %s via Bunny\n", target)
	}
	return nil
}

func purgeCloudflareURLs(urls []string, token, zoneID string) error {
	payload := map[string][]string{"files": urls}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf(cloudflarePurgeEndpoint, zoneID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && bytes.Contains(respBody, []byte(`"success":true`)) {
		fmt.Printf("Purged %d URL(s) via Cloudflare\n", len(urls))
		return nil
	}
	return fmt.Errorf("cloudflare purge failed: %s", strings.TrimSpace(string(respBody)))
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	if stderr.Len() > 0 {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
		}
	}
	return stdout.String(), nil
}

func gitShow(ref, path string) (string, error) {
	return gitOutput("show", ref+":"+path)
}

func isMissingGitObject(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "exists on disk, but not in") ||
		(strings.Contains(msg, "path '") && strings.Contains(msg, "does not exist in")) ||
		strings.Contains(msg, "fatal: invalid object name")
}

func kindRoot(kind contentKind) string {
	switch kind {
	case kindBlog:
		return "content/blog"
	case kindProject:
		return "content/projects"
	case kindLegal:
		return "content/legal"
	default:
		return "content"
	}
}

func isBlogContentPath(path string) bool {
	return strings.HasPrefix(path, "content/blog/") && filepath.Ext(path) == ".md"
}

func isProjectContentPath(path string) bool {
	return strings.HasPrefix(path, "content/projects/") && filepath.Ext(path) == ".md"
}

func isLegalContentPath(path string) bool {
	return strings.HasPrefix(path, "content/legal/") && filepath.Ext(path) == ".md"
}

func isFeatureHomeFile(path string) bool {
	return strings.HasPrefix(path, "feature/home/")
}

func isFeatureAboutFile(path string) bool {
	return strings.HasPrefix(path, "feature/about/")
}

func isFeatureBlogFile(path string) bool {
	return strings.HasPrefix(path, "feature/blog/")
}

func isFeaturePortfolioFile(path string) bool {
	return strings.HasPrefix(path, "feature/portfolio/")
}

func isFeatureTagsFile(path string) bool {
	return strings.HasPrefix(path, "feature/tags/")
}

func isFeatureLegalFile(path string) bool {
	return strings.HasPrefix(path, "feature/legal/")
}

func isSharedSiteFile(path string) bool {
	return strings.HasPrefix(path, "ui/") ||
		strings.HasPrefix(path, "core/content/") ||
		strings.HasPrefix(path, "core/markdown/") ||
		strings.HasPrefix(path, "core/config/") ||
		path == "cmd/server/main.go"
}

func stringValue(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func sliceValue(v any) []string {
	switch value := v.(type) {
	case []any:
		items := make([]string, 0, len(value))
		for _, item := range value {
			items = append(items, stringValue(item))
		}
		return items
	case []string:
		return value
	case nil:
		return nil
	default:
		return []string{stringValue(value)}
	}
}

func boolValue(v any) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		lower := strings.ToLower(strings.TrimSpace(value))
		return lower == "true" || lower == "yes" || lower == "1"
	default:
		return false
	}
}

func normalizeTags(tags []string) []string {
	set := make(stringSet)
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		set.add(normalized)
	}
	return set.sortedValues()
}

func slugFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func normalizePath(baseURL, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/"
	}
	if isAbsoluteURL(trimmed) {
		if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
			return parsed.Path
		}
	}
	if baseURL != "" {
		trimmed = strings.TrimPrefix(trimmed, strings.TrimRight(baseURL, "/"))
	}
	if baseURL == "" && !strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}

func resolvePurgeHosts(canonical *url.URL, purgeHosts string) []string {
	hosts := newStringSet(canonical.Host)
	for _, raw := range strings.Split(purgeHosts, ",") {
		host := strings.TrimSpace(raw)
		if host == "" {
			continue
		}
		hosts.add(host)
	}
	if hosts.len() == 1 {
		host := canonical.Hostname()
		switch {
		case strings.HasPrefix(host, "www."):
			hosts.add(strings.TrimPrefix(host, "www."))
		case countLabels(host) == 2:
			hosts.add("www." + host)
		}
	}
	return hosts.sortedHosts(canonical)
}

func countLabels(host string) int {
	if host == "" {
		return 0
	}
	return len(strings.Split(host, "."))
}

func isAbsoluteURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func defaultProvider() Provider {
	if provider, err := parseProvider(envOrDefault("CDN_PROVIDER", string(ProviderBunny))); err == nil {
		return provider
	}
	return ProviderBunny
}

func parseProvider(value string) (Provider, error) {
	switch Provider(strings.ToLower(strings.TrimSpace(value))) {
	case "", ProviderBunny:
		return ProviderBunny, nil
	case ProviderCloudflare:
		return ProviderCloudflare, nil
	default:
		return "", fmt.Errorf("invalid CDN provider %q; expected bunny or cloudflare", value)
	}
}

func newStringSet(values ...string) stringSet {
	set := make(stringSet)
	set.add(values...)
	return set
}

func (s stringSet) add(values ...string) {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		s[trimmed] = struct{}{}
	}
}

func (s stringSet) has(value string) bool {
	_, ok := s[value]
	return ok
}

func (s stringSet) len() int {
	return len(s)
}

func (s stringSet) sorted() []string {
	values := s.sortedValues()
	sort.Strings(values)
	return values
}

func (s stringSet) sortedValues() []string {
	values := make([]string, 0, len(s))
	for value := range s {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func (s stringSet) sortedHosts(canonical *url.URL) []string {
	values := s.sortedValues()
	sort.Slice(values, func(i, j int) bool {
		if values[i] == canonical.Host {
			return true
		}
		if values[j] == canonical.Host {
			return false
		}
		return values[i] < values[j]
	})
	return values
}
