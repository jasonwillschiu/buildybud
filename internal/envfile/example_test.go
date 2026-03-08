package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendExampleCreatesFileWithRequiredVars(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.example")

	if err := AppendExample(path); err != nil {
		t.Fatalf("append example: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"# Required for buildybud CDN planning and for purging relative paths.",
		"BB_APP_BASE_URL=",
		"BB_CDN_PROVIDER=",
		"BB_BUNNY_API_KEY=",
		"BB_CF_API_TOKEN=",
		"BB_CF_ZONE_ID=",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf(".env.example missing %q:\n%s", want, content)
		}
	}
	for _, unwanted := range []string{
		"\nAPP_BASE_URL=",
		"\nCDN_PROVIDER=",
		"\nBUNNY_API_KEY=",
		"\nCF_API_TOKEN=",
		"\nCF_ZONE_ID=",
	} {
		if strings.Contains(content, unwanted) {
			t.Fatalf(".env.example should not contain legacy key %q:\n%s", unwanted, content)
		}
	}
}

func TestAppendExampleAppendsMissingVarsWithoutDuplicatingExistingOnes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.example")
	seed := "# existing\nBB_APP_BASE_URL=https://example.com"
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := AppendExample(path); err != nil {
		t.Fatalf("append example: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	content := string(data)
	if strings.Count(content, "BB_APP_BASE_URL=") != 1 {
		t.Fatalf("expected BB_APP_BASE_URL once:\n%s", content)
	}
	if !strings.HasPrefix(content, seed+"\n\n") {
		t.Fatalf("expected appended content after existing file:\n%s", content)
	}
	if !strings.Contains(content, "BB_CDN_PROVIDER=") {
		t.Fatalf("expected missing vars to be appended:\n%s", content)
	}
}

func TestAppendExampleMigratesLegacyKeyToPrefixedKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.example")
	seed := "# Required for CDN planning and for purging relative paths. BB_APP_BASE_URL is also supported.\nAPP_BASE_URL=https://example.com\n"
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := AppendExample(path); err != nil {
		t.Fatalf("append example: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "\nAPP_BASE_URL=") || strings.HasPrefix(content, "APP_BASE_URL=") {
		t.Fatalf("expected legacy APP_BASE_URL to be removed:\n%s", content)
	}
	if !strings.Contains(content, "BB_APP_BASE_URL=") {
		t.Fatalf("expected BB_APP_BASE_URL to be appended:\n%s", content)
	}
}
