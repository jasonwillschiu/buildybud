package favicon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCopiesFaviconTreeAndCleansOutput(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "favicon.ico"), []byte("ico"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "icon.svg"), []byte("svg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "stale.ico"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(Options{SourceDir: src, OutDir: dst, Clean: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, rel := range []string{"favicon.ico", filepath.Join("nested", "icon.svg")} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Fatalf("missing copied file %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.ico")); !os.IsNotExist(err) {
		t.Fatalf("expected stale file removed, got err=%v", err)
	}
}
