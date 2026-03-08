package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpShowsQuickStartAndVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Quick start:",
		"buildybud init",
		"--version, -version",
		"buildybud version v0.0.0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunVersionFlagPrintsToolVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "buildybud version v0.0.0" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunInitGeneratesConfig(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "assets", "src", "js"), 0o755); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "buildybud.toml")); err != nil {
		t.Fatalf("missing generated config: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".env.example"))
	if err != nil {
		t.Fatalf("missing .env.example: %v", err)
	}
	if !strings.Contains(string(data), "APP_BASE_URL=") {
		t.Fatalf(".env.example missing APP_BASE_URL:\n%s", string(data))
	}
}

func TestRunInitForceAppendsEnvExample(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "buildybud.toml"), []byte("schema_version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env.example"), []byte("APP_BASE_URL=https://example.com"), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--force"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".env.example"))
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	content := string(data)
	if strings.Count(content, "APP_BASE_URL=") != 1 {
		t.Fatalf("expected APP_BASE_URL once:\n%s", content)
	}
	if !strings.Contains(content, "CDN_PROVIDER=") {
		t.Fatalf("expected appended vars in .env.example:\n%s", content)
	}
}
