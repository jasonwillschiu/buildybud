package cdn

import (
	"bytes"
	"context"
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
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jasonwillschiu/buildybud/internal/config"
	"gopkg.in/yaml.v2"
)

const (
	cloudflarePurgeEndpoint = "https://api.cloudflare.com/client/v4/zones/%s/purge_cache"
	cloudflareBatchSize     = 100
	cloudflareTimeout       = 15 * time.Second
)

type PlanResult struct {
	Paths   []string `json:"paths"`
	URLs    []string `json:"urls"`
	Reasons []string `json:"reasons,omitempty"`
}

type runtimeConfig struct {
	gitRoot        string
	gitPrefix      string
	baseURL        *url.URL
	hosts          []string
	staticPaths    []string
	tagPrefix      string
	redirectsFile  string
	redirectsTable string
	content        []config.CDNContentConfig
}

type purgeConfig struct {
	apiToken string
	zoneID   string
	dryRun   bool
	client   *http.Client
	endpoint string
	stdout   io.Writer
}

type contentMeta struct {
	slug  string
	tags  []string
	draft bool
}

type cloudflareResponse struct {
	Success bool              `json:"success"`
	Errors  []cloudflareError `json:"errors"`
}

type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type stringSet map[string]struct{}

func RootHelp() string {
	return `CDN commands:
  buildybud cdn plan --from-ref <ref> [--to-ref HEAD]
  buildybud cdn purge <path-or-url>...
  buildybud cdn plan-and-purge --from-ref <ref> [--to-ref HEAD]

Configuration:
  Enable and configure Cloudflare route planning in buildybud.toml under [cdn].
  Non-secret --base-url and --purge-hosts flags override TOML values.

Environment:
  BB_APP_BASE_URL / APP_BASE_URL           Optional base URL override
  BB_CDN_PURGE_HOSTS / CDN_PURGE_HOSTS     Optional comma-separated host override
  BB_CF_API_TOKEN / CF_API_TOKEN           Required for purge unless --dry-run
  BB_CF_ZONE_ID / CF_ZONE_ID               Required for purge unless --dry-run
`
}

func Plan(args []string) error {
	fs := newFlagSet("buildybud cdn plan")
	fromRef := fs.String("from-ref", "", "Git ref for the currently deployed revision")
	toRef := fs.String("to-ref", "HEAD", "Git ref for the target revision")
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud.toml")
	baseURL := fs.String("base-url", "", "Override cdn.base_url")
	purgeHosts := fs.String("purge-hosts", "", "Override cdn.hosts with comma-separated hostnames")
	verbose := fs.Bool("verbose", false, "Include route-planning reasons")
	help, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	if strings.TrimSpace(*fromRef) == "" {
		return errors.New("--from-ref is required")
	}

	runtime, err := loadRuntimeConfig(*configPath, *baseURL, *purgeHosts)
	if err != nil {
		return err
	}
	result, err := buildPlan(*fromRef, *toRef, runtime, *verbose)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, result)
}

func Purge(args []string) error {
	fs := newFlagSet("buildybud cdn purge")
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud.toml")
	baseURL := fs.String("base-url", "", "Override cdn.base_url")
	purgeHosts := fs.String("purge-hosts", "", "Override cdn.hosts with comma-separated hostnames")
	apiToken := fs.String("cf-api-token", envValue("BB_CF_API_TOKEN", "CF_API_TOKEN"), "Cloudflare API token")
	zoneID := fs.String("cf-zone-id", envValue("BB_CF_ZONE_ID", "CF_ZONE_ID"), "Cloudflare zone ID")
	dryRun := fs.Bool("dry-run", false, "Print URLs without calling Cloudflare")
	help, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	if fs.NArg() == 0 {
		return errors.New("purge requires at least one path or absolute URL argument")
	}

	runtime, err := loadRuntimeConfig(*configPath, *baseURL, *purgeHosts)
	if err != nil {
		return err
	}
	urls, err := buildPurgeURLs(fs.Args(), runtime)
	if err != nil {
		return err
	}
	return purgeCloudflareURLs(context.Background(), urls, purgeConfig{
		apiToken: *apiToken,
		zoneID:   *zoneID,
		dryRun:   *dryRun,
		stdout:   os.Stdout,
	})
}

func PlanAndPurge(args []string) error {
	fs := newFlagSet("buildybud cdn plan-and-purge")
	fromRef := fs.String("from-ref", "", "Git ref for the currently deployed revision")
	toRef := fs.String("to-ref", "HEAD", "Git ref for the target revision")
	configPath := fs.String("config", config.DefaultPath, "Path to buildybud.toml")
	baseURL := fs.String("base-url", "", "Override cdn.base_url")
	purgeHosts := fs.String("purge-hosts", "", "Override cdn.hosts with comma-separated hostnames")
	apiToken := fs.String("cf-api-token", envValue("BB_CF_API_TOKEN", "CF_API_TOKEN"), "Cloudflare API token")
	zoneID := fs.String("cf-zone-id", envValue("BB_CF_ZONE_ID", "CF_ZONE_ID"), "Cloudflare zone ID")
	dryRun := fs.Bool("dry-run", false, "Print URLs without calling Cloudflare")
	verbose := fs.Bool("verbose", false, "Include route-planning reasons")
	help, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	if strings.TrimSpace(*fromRef) == "" {
		return errors.New("--from-ref is required")
	}

	runtime, err := loadRuntimeConfig(*configPath, *baseURL, *purgeHosts)
	if err != nil {
		return err
	}
	result, err := buildPlan(*fromRef, *toRef, runtime, *verbose)
	if err != nil {
		return err
	}
	if err := writeJSON(os.Stdout, result); err != nil {
		return err
	}
	return purgeCloudflareURLs(context.Background(), result.URLs, purgeConfig{
		apiToken: *apiToken,
		zoneID:   *zoneID,
		dryRun:   *dryRun,
		stdout:   os.Stdout,
	})
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", name)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, RootHelp())
	}
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func loadRuntimeConfig(configPath, baseOverride, hostsOverride string) (runtimeConfig, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return runtimeConfig{}, err
	}
	if !cfg.CDN.Enabled {
		return runtimeConfig{}, errors.New("cdn is disabled; set cdn.enabled = true in buildybud.toml")
	}

	baseValue := firstValue(baseOverride, os.Getenv("BB_APP_BASE_URL"), os.Getenv("APP_BASE_URL"), cfg.CDN.BaseURL)
	baseURL, err := parseBaseURL(baseValue)
	if err != nil {
		return runtimeConfig{}, err
	}
	hosts := append([]string(nil), cfg.CDN.Hosts...)
	if override := firstValue(hostsOverride, os.Getenv("BB_CDN_PURGE_HOSTS"), os.Getenv("CDN_PURGE_HOSTS")); override != "" {
		hosts = splitHosts(override)
	}
	hosts, err = validateHosts(hosts, baseURL.Host)
	if err != nil {
		return runtimeConfig{}, err
	}

	repoRoot := cfg.RepoPath(".")
	gitRoot, err := gitOutputAt(repoRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("find git root: %w", err)
	}
	gitRoot = strings.TrimSpace(gitRoot)
	prefix, err := filepath.Rel(gitRoot, repoRoot)
	if err != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(filepath.Separator)) {
		return runtimeConfig{}, fmt.Errorf("configured repo root %s is outside git root %s", repoRoot, gitRoot)
	}
	if prefix == "." {
		prefix = ""
	}

	return runtimeConfig{
		gitRoot:        gitRoot,
		gitPrefix:      filepath.ToSlash(prefix),
		baseURL:        baseURL,
		hosts:          hosts,
		staticPaths:    append([]string(nil), cfg.CDN.StaticPaths...),
		tagPrefix:      cfg.CDN.TagPrefix,
		redirectsFile:  cfg.CDN.RedirectsFile,
		redirectsTable: cfg.CDN.RedirectsTable,
		content:        append([]config.CDNContentConfig(nil), cfg.CDN.Content...),
	}, nil
}

func buildPlan(fromRef, toRef string, cfg runtimeConfig, verbose bool) (PlanResult, error) {
	fromCommit, err := resolveCommit(cfg.gitRoot, fromRef)
	if err != nil {
		return PlanResult{}, fmt.Errorf("resolve from ref %q: %w", fromRef, err)
	}
	toCommit, err := resolveCommit(cfg.gitRoot, toRef)
	if err != nil {
		return PlanResult{}, fmt.Errorf("resolve to ref %q: %w", toRef, err)
	}

	paths := make(stringSet)
	for _, commit := range []string{fromCommit, toCommit} {
		atRef, err := publicPathsAtRef(commit, cfg)
		if err != nil {
			return PlanResult{}, err
		}
		paths.add(atRef...)
	}
	pathList := paths.sorted()
	urls, err := buildURLs(pathList, cfg)
	if err != nil {
		return PlanResult{}, err
	}
	result := PlanResult{Paths: pathList, URLs: urls}
	if verbose {
		result.Reasons = []string{fmt.Sprintf("union of public routes at %s and %s", fromCommit, toCommit)}
	}
	return result, nil
}

func publicPathsAtRef(ref string, cfg runtimeConfig) ([]string, error) {
	paths := newStringSet(cfg.staticPaths...)
	for _, source := range cfg.content {
		root := cfg.gitPath(source.SourceDir)
		output, err := gitOutputAt(cfg.gitRoot, "ls-tree", "-r", "--name-only", ref, "--", root)
		if err != nil {
			return nil, fmt.Errorf("list %s at %s: %w", source.SourceDir, ref, err)
		}
		for _, file := range strings.Split(strings.TrimSpace(output), "\n") {
			file = strings.TrimSpace(file)
			if file == "" || strings.ToLower(filepath.Ext(file)) != ".md" {
				continue
			}
			data, err := gitShow(cfg.gitRoot, ref, file)
			if err != nil {
				return nil, err
			}
			meta, err := parseContentMeta(file, data)
			if err != nil {
				return nil, err
			}
			if meta.draft || meta.slug == "" {
				continue
			}
			paths.add(joinRoute(source.RoutePrefix, meta.slug))
			if source.IncludeTags {
				for _, tag := range meta.tags {
					paths.add(joinRoute(cfg.tagPrefix, tag))
				}
			}
		}
	}

	redirects, err := redirectsAtRef(ref, cfg)
	if err != nil {
		return nil, err
	}
	for source, target := range redirects {
		normalizedSource, err := normalizePublicValue(source, cfg)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect source %q: %w", source, err)
		}
		normalizedTarget, err := normalizePublicValue(target, cfg)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect target %q: %w", target, err)
		}
		paths.add(normalizedSource, normalizedTarget)
	}
	return paths.sorted(), nil
}

func redirectsAtRef(ref string, cfg runtimeConfig) (map[string]string, error) {
	if cfg.redirectsFile == "" {
		return nil, nil
	}
	data, err := gitShow(cfg.gitRoot, ref, cfg.gitPath(cfg.redirectsFile))
	if err != nil {
		return nil, fmt.Errorf("read redirects at %s: %w", ref, err)
	}
	var document map[string]any
	if _, err := toml.Decode(string(data), &document); err != nil {
		return nil, fmt.Errorf("parse redirects at %s: %w", ref, err)
	}
	var current any = document
	for _, part := range strings.Split(cfg.redirectsTable, ".") {
		table, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("redirects table %q is not a TOML table at %s", cfg.redirectsTable, ref)
		}
		current, ok = table[part]
		if !ok {
			return nil, fmt.Errorf("redirects table %q is missing at %s", cfg.redirectsTable, ref)
		}
	}
	table, ok := current.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("redirects table %q is not a TOML table at %s", cfg.redirectsTable, ref)
	}
	redirects := make(map[string]string, len(table))
	for source, rawTarget := range table {
		target, ok := rawTarget.(string)
		if !ok {
			return nil, fmt.Errorf("redirect %q at %s must have a string target", source, ref)
		}
		redirects[source] = target
	}
	return redirects, nil
}

func parseContentMeta(file string, data []byte) (contentMeta, error) {
	meta := contentMeta{slug: strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))}
	frontmatter, err := parseFrontmatter(data)
	if err != nil {
		return contentMeta{}, fmt.Errorf("parse frontmatter for %s: %w", file, err)
	}
	if slug := stringValue(frontmatter["slug"]); slug != "" {
		meta.slug = slug
	}
	meta.tags = normalizeTags(stringSlice(frontmatter["tags"]))
	meta.draft = boolValue(frontmatter["draft"])
	return meta, nil
}

func parseFrontmatter(data []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	lines := strings.Split(string(trimmed), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
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
	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &frontmatter); err != nil {
		return nil, err
	}
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}
	return frontmatter, nil
}

func buildPurgeURLs(values []string, cfg runtimeConfig) ([]string, error) {
	paths := make([]string, 0, len(values))
	absolute := make(stringSet)
	allowed := newStringSet(cfg.hosts...)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("parse purge value %q: %w", value, err)
		}
		if parsed.IsAbs() {
			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				return nil, fmt.Errorf("purge URL %q must use http or https", value)
			}
			if parsed.User != nil {
				return nil, fmt.Errorf("purge URL %q must not contain user information", value)
			}
			if !allowed.has(strings.ToLower(parsed.Host)) {
				return nil, fmt.Errorf("purge URL host %q is not listed in cdn.hosts", parsed.Host)
			}
			parsed.Fragment = ""
			absolute.add(parsed.String())
			continue
		}
		if !strings.HasPrefix(value, "/") {
			return nil, fmt.Errorf("purge path %q must start with /", value)
		}
		paths = append(paths, value)
	}
	resolved, err := buildURLs(paths, cfg)
	if err != nil {
		return nil, err
	}
	absolute.add(resolved...)
	return absolute.sorted(), nil
}

func buildURLs(paths []string, cfg runtimeConfig) ([]string, error) {
	urls := make(stringSet)
	for _, route := range paths {
		parsed, err := url.Parse(route)
		if err != nil || parsed.IsAbs() || !strings.HasPrefix(parsed.Path, "/") {
			return nil, fmt.Errorf("invalid public path %q", route)
		}
		for _, host := range cfg.hosts {
			resolved := *cfg.baseURL
			resolved.Host = host
			resolved.Path = parsed.Path
			resolved.RawPath = parsed.RawPath
			resolved.RawQuery = parsed.RawQuery
			resolved.Fragment = ""
			urls.add(resolved.String())
		}
	}
	return urls.sorted(), nil
}

func purgeCloudflareURLs(ctx context.Context, urls []string, cfg purgeConfig) error {
	if len(urls) == 0 {
		return nil
	}
	stdout := cfg.stdout
	if stdout == nil {
		stdout = io.Discard
	}
	if cfg.dryRun {
		for _, value := range urls {
			fmt.Fprintln(stdout, value)
		}
		return nil
	}
	if strings.TrimSpace(cfg.apiToken) == "" || strings.TrimSpace(cfg.zoneID) == "" {
		return errors.New("Cloudflare purge requires BB_CF_API_TOKEN and BB_CF_ZONE_ID (legacy CF_API_TOKEN and CF_ZONE_ID are also supported); add them to .env or pass flags")
	}
	client := cfg.client
	if client == nil {
		client = &http.Client{Timeout: cloudflareTimeout}
	}
	endpoint := cfg.endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf(cloudflarePurgeEndpoint, url.PathEscape(cfg.zoneID))
	}
	for start := 0; start < len(urls); start += cloudflareBatchSize {
		end := start + cloudflareBatchSize
		if end > len(urls) {
			end = len(urls)
		}
		if err := purgeCloudflareBatch(ctx, client, endpoint, cfg.apiToken, urls[start:end]); err != nil {
			return fmt.Errorf("purge Cloudflare batch %d-%d: %w", start+1, end, err)
		}
	}
	return nil
}

func purgeCloudflareBatch(ctx context.Context, client *http.Client, endpoint, token string, urls []string) error {
	body, err := json.Marshal(struct {
		Files []string `json:"files"`
	}{Files: urls})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	var decoded cloudflareResponse
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &decoded); err != nil {
			return fmt.Errorf("decode response (HTTP %d): %w", response.StatusCode, err)
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !decoded.Success {
		messages := make([]string, 0, len(decoded.Errors))
		for _, item := range decoded.Errors {
			messages = append(messages, fmt.Sprintf("%d: %s", item.Code, item.Message))
		}
		if len(messages) == 0 {
			messages = append(messages, strings.TrimSpace(string(data)))
		}
		return fmt.Errorf("Cloudflare returned HTTP %d: %s", response.StatusCode, strings.Join(messages, "; "))
	}
	return nil
}

func normalizePublicValue(value string, cfg runtimeConfig) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", errors.New("absolute URL must use http or https")
		}
		if parsed.User != nil {
			return "", errors.New("absolute URL must not contain user information")
		}
		if !newStringSet(cfg.hosts...).has(strings.ToLower(parsed.Host)) {
			return "", fmt.Errorf("host %q is not listed in cdn.hosts", parsed.Host)
		}
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		return "", errors.New("path must start with /")
	}
	if parsed.Fragment != "" {
		return "", errors.New("fragments cannot be purged")
	}
	result := parsed.EscapedPath()
	if result == "" {
		result = "/"
	}
	if parsed.RawQuery != "" {
		result += "?" + parsed.RawQuery
	}
	return result, nil
}

func parseBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("cdn.base_url must be an absolute http or https URL: %q", value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("cdn.base_url must use http or https")
	}
	if parsed.User != nil {
		return nil, errors.New("cdn.base_url must not contain user information")
	}
	if parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("cdn.base_url must not contain a path, query, or fragment")
	}
	parsed.Path = ""
	return parsed, nil
}

func validateHosts(values []string, baseHost string) ([]string, error) {
	hosts := make(stringSet)
	for _, value := range values {
		host := strings.ToLower(strings.TrimSpace(value))
		if host == "" || strings.ContainsAny(host, "/?#") {
			return nil, fmt.Errorf("invalid cdn host %q", value)
		}
		parsed, err := url.Parse("http://" + host)
		if err != nil || parsed.User != nil || parsed.Host != host || parsed.Hostname() == "" {
			return nil, fmt.Errorf("invalid cdn host %q", value)
		}
		hosts.add(host)
	}
	if len(hosts) == 0 {
		return nil, errors.New("cdn.hosts must contain at least one hostname")
	}
	if !hosts.has(strings.ToLower(baseHost)) {
		return nil, fmt.Errorf("cdn.base_url host %q must be listed in cdn.hosts", baseHost)
	}
	return hosts.sorted(), nil
}

func splitHosts(value string) []string {
	parts := strings.Split(value, ",")
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		if host := strings.TrimSpace(part); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func joinRoute(prefix, segment string) string {
	return strings.TrimRight(prefix, "/") + "/" + url.PathEscape(strings.Trim(segment, "/"))
}

func resolveCommit(gitRoot, ref string) (string, error) {
	output, err := gitOutputAt(gitRoot, "rev-parse", "--verify", ref+"^{commit}")
	return strings.TrimSpace(output), err
}

func gitShow(gitRoot, ref, file string) ([]byte, error) {
	output, err := gitOutputBytesAt(gitRoot, "show", ref+":"+filepath.ToSlash(file))
	if err != nil {
		return nil, fmt.Errorf("read %s at %s: %w", file, ref, err)
	}
	return output, nil
}

func gitOutputAt(dir string, args ...string) (string, error) {
	output, err := gitOutputBytesAt(dir, args...)
	return string(output), err
}

func gitOutputBytesAt(dir string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (cfg runtimeConfig) gitPath(value string) string {
	if cfg.gitPrefix == "" {
		return filepath.ToSlash(value)
	}
	return filepath.ToSlash(filepath.Join(cfg.gitPrefix, value))
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func envValue(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if stringValue, ok := value.(string); ok {
		return strings.TrimSpace(stringValue)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stringSlice(value any) []string {
	switch values := value.(type) {
	case []any:
		result := make([]string, 0, len(values))
		for _, item := range values {
			result = append(result, stringValue(item))
		}
		return result
	case []string:
		return values
	case nil:
		return nil
	default:
		return []string{stringValue(values)}
	}
}

func boolValue(value any) bool {
	switch value := value.(type) {
	case bool:
		return value
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		return normalized == "true" || normalized == "yes" || normalized == "1"
	default:
		return false
	}
}

func normalizeTags(tags []string) []string {
	result := make(stringSet)
	for _, tag := range tags {
		if normalized := strings.ToLower(strings.TrimSpace(tag)); normalized != "" {
			result.add(normalized)
		}
	}
	return result.sorted()
}

func newStringSet(values ...string) stringSet {
	set := make(stringSet)
	set.add(values...)
	return set
}

func (set stringSet) add(values ...string) {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
}

func (set stringSet) has(value string) bool {
	_, ok := set[value]
	return ok
}

func (set stringSet) sorted() []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
