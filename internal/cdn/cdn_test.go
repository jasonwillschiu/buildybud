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
