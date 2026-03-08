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
		"# Required for CDN planning and for purging relative paths. BB_APP_BASE_URL is also supported.",
		"APP_BASE_URL=",
		"CDN_PROVIDER=",
		"BUNNY_API_KEY=",
		"CF_API_TOKEN=",
		"CF_ZONE_ID=",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf(".env.example missing %q:\n%s", want, content)
		}
	}
}

func TestAppendExampleAppendsMissingVarsWithoutDuplicatingExistingOnes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.example")
	seed := "# existing\nAPP_BASE_URL=https://example.com"
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
	if strings.Count(content, "APP_BASE_URL=") != 1 {
		t.Fatalf("expected APP_BASE_URL once:\n%s", content)
	}
	if !strings.HasPrefix(content, seed+"\n\n") {
		t.Fatalf("expected appended content after existing file:\n%s", content)
	}
	if !strings.Contains(content, "CDN_PROVIDER=") {
		t.Fatalf("expected missing vars to be appended:\n%s", content)
	}
}
