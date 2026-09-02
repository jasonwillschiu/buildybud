package cdn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jasonwillschiu/buildybud/internal/config"
)

func TestBuildPlanUnionsPublicRoutesAtBothCommits(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	gitRun(t, repo, "config", "user.email", "buildybud@example.com")
	gitRun(t, repo, "config", "user.name", "Buildybud Test")

	writeTestFile(t, repo, "content/blog/old.md", `---
slug: old-post
tags: [Go]
---
Old.
`)
	writeTestFile(t, repo, "content/blog/draft.md", `---
slug: secret
tags: [Hidden]
draft: true
---
Draft.
`)
	writeTestFile(t, repo, "AppConstants.toml", `[redirects]
"/legacy" = "/blog/vanished?from=legacy"
`)
	oldCommit := gitCommit(t, repo, "old routes")

	if err := os.Rename(filepath.Join(repo, "content/blog/old.md"), filepath.Join(repo, "content/blog/new.md")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, repo, "content/blog/new.md", `---
slug: new-post
tags: [Web, "C#"]
---
New.
`)
	writeTestFile(t, repo, "AppConstants.toml", `[redirects]
"/retired" = "/blog/vanished?from=retired"
`)
	newCommit := gitCommit(t, repo, "new routes")

	baseURL, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	runtime := runtimeConfig{
		gitRoot:        repo,
		baseURL:        baseURL,
		hosts:          []string{"example.com", "www.example.com"},
		staticPaths:    []string{"/", "/blog"},
		tagPrefix:      "/tags",
		redirectsFile:  "AppConstants.toml",
		redirectsTable: "redirects",
		content: []config.CDNContentConfig{{
			SourceDir:   "content/blog",
			RoutePrefix: "/blog",
			IncludeTags: true,
		}},
	}

	result, err := buildPlan(oldCommit, newCommit, runtime, true)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"/",
		"/blog",
		"/blog/new-post",
		"/blog/old-post",
		"/blog/vanished?from=legacy",
		"/blog/vanished?from=retired",
		"/legacy",
		"/retired",
		"/tags/c%23",
		"/tags/go",
		"/tags/web",
	}
	if !reflect.DeepEqual(result.Paths, wantPaths) {
		t.Fatalf("paths = %#v, want %#v", result.Paths, wantPaths)
	}
	if len(result.URLs) != len(wantPaths)*2 {
		t.Fatalf("got %d URLs, want %d: %#v", len(result.URLs), len(wantPaths)*2, result.URLs)
	}
	if contains(result.Paths, "/blog/secret") || contains(result.Paths, "/tags/hidden") {
		t.Fatalf("draft routes leaked into plan: %#v", result.Paths)
	}
	if !contains(result.URLs, "https://www.example.com/blog/vanished?from=legacy") {
		t.Fatalf("redirect query string was not preserved: %#v", result.URLs)
	}
	if len(result.Reasons) != 1 || !strings.Contains(result.Reasons[0], oldCommit) || !strings.Contains(result.Reasons[0], newCommit) {
		t.Fatalf("reasons = %#v", result.Reasons)
	}
}

func TestBuildPurgeURLsRejectsUnknownAbsoluteHost(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com")
	_, err := buildPurgeURLs([]string{"https://attacker.example/private"}, runtimeConfig{
		baseURL: baseURL,
		hosts:   []string{"example.com"},
	})
	if err == nil || !strings.Contains(err.Error(), "not listed in cdn.hosts") {
		t.Fatalf("expected host allowlist error, got %v", err)
	}
}

func TestBuildPurgeURLsExpandsRelativePathsAcrossConfiguredHosts(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com")
	got, err := buildPurgeURLs([]string{"/blog/post?preview=1", "https://example.com/about#ignored"}, runtimeConfig{
		baseURL: baseURL,
		hosts:   []string{"example.com", "www.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"https://example.com/about",
		"https://example.com/blog/post?preview=1",
		"https://www.example.com/blog/post?preview=1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("URLs = %#v, want %#v", got, want)
	}
}

func TestPurgeCloudflareURLsBatchesAtOneHundred(t *testing.T) {
	var mu sync.Mutex
	var batchSizes []int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s", request.Method)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization = %q", got)
		}
		var payload struct {
			Files []string `json:"files"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		mu.Lock()
		batchSizes = append(batchSizes, len(payload.Files))
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"success":true,"errors":[]}`)
	}))
	defer server.Close()

	urls := make([]string, 205)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://example.com/page/%03d", i)
	}
	err := purgeCloudflareURLs(context.Background(), urls, purgeConfig{
		apiToken: "secret",
		zoneID:   "zone",
		client:   server.Client(),
		endpoint: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{100, 100, 5}; !reflect.DeepEqual(batchSizes, want) {
		t.Fatalf("batch sizes = %#v, want %#v", batchSizes, want)
	}
}

func TestPurgeCloudflareURLsReturnsTypedAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprint(writer, `{"success":false,"errors":[{"code":10000,"message":"authentication error"}]}`)
	}))
	defer server.Close()

	err := purgeCloudflareURLs(context.Background(), []string{"https://example.com"}, purgeConfig{
		apiToken: "wrong",
		zoneID:   "zone",
		client:   server.Client(),
		endpoint: server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "10000: authentication error") {
		t.Fatalf("expected typed Cloudflare error, got %v", err)
	}
}

func TestPurgeCloudflareURLsDryRunNeedsNoCredentials(t *testing.T) {
	var output bytes.Buffer
	err := purgeCloudflareURLs(context.Background(), []string{"https://example.com/about"}, purgeConfig{
		dryRun: true,
		stdout: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "https://example.com/about\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestPurgeCloudflareURLsMissingCredentialsMentionsDotEnv(t *testing.T) {
	err := purgeCloudflareURLs(context.Background(), []string{"https://example.com"}, purgeConfig{})
	if err == nil || !strings.Contains(err.Error(), ".env") {
		t.Fatalf("expected .env hint, got %v", err)
	}
}

func gitCommit(t *testing.T, repo, message string) string {
	t.Helper()
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", message)
	return strings.TrimSpace(gitRun(t, repo, "rev-parse", "HEAD"))
}

func gitRun(t *testing.T, repo string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
