package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIfPresentLoadsUnsetVars(t *testing.T) {
	t.Setenv("APP_BASE_URL", "")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("APP_BASE_URL=https://example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("APP_BASE_URL"); err != nil {
		t.Fatal(err)
	}

	if err := LoadIfPresent(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := os.Getenv("APP_BASE_URL"); got != "https://example.com" {
		t.Fatalf("APP_BASE_URL = %q", got)
	}
}

func TestLoadIfPresentRejectsInvalidLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("NOT_VALID\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadIfPresent(path); err == nil {
		t.Fatal("expected error")
	}
}
