package cdn

import (
	"strings"
	"testing"
)

func TestBuildPlanMissingBaseURLMentionsDotEnv(t *testing.T) {
	_, err := buildPlan("HEAD~1", "HEAD", "", "", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), ".env") {
		t.Fatalf("expected .env hint, got %v", err)
	}
}

func TestPurgeURLsMissingProviderCredentialsMentionsDotEnv(t *testing.T) {
	err := purgeURLs([]string{"https://example.com/blog"}, purgeConfig{Provider: ProviderBunny})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), ".env") {
		t.Fatalf("expected .env hint, got %v", err)
	}
}

func TestEnvOrDefaultPrefersPrefixedAlias(t *testing.T) {
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("BB_APP_BASE_URL", "https://example.com")

	if got := envOrDefault("APP_BASE_URL", "BB_APP_BASE_URL", ""); got != "https://example.com" {
		t.Fatalf("envOrDefault() = %q", got)
	}
}

func TestDefaultProviderReadsPrefixedAlias(t *testing.T) {
	t.Setenv("CDN_PROVIDER", "")
	t.Setenv("BB_CDN_PROVIDER", "cloudflare")

	if got := defaultProvider(); got != ProviderCloudflare {
		t.Fatalf("defaultProvider() = %q", got)
	}
}
