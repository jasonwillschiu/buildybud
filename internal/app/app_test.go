package app

import (
	"bytes"
	"encoding/json"
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
	if !strings.Contains(string(data), "BB_APP_BASE_URL=") {
		t.Fatalf(".env.example missing BB_APP_BASE_URL:\n%s", string(data))
	}
}

func TestRunInitDryRunDoesNotWriteFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "assets", "src", "js"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo-root", tmp, "--dry-run"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "[paths]") {
		t.Fatalf("dry run did not print config:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "buildybud.toml")); !os.IsNotExist(err) {
		t.Fatalf("expected no generated config, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".env.example")); !os.IsNotExist(err) {
		t.Fatalf("expected no generated env example, got err=%v", err)
	}
}

func TestRunInitSkipsEnvExampleWhenConfigOutsideRepo(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	outsideConfig := filepath.Join(tmp, "generated.toml")
	if err := os.MkdirAll(filepath.Join(repoRoot, "assets", "src", "js"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo-root", repoRoot, "--config", outsideConfig}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(outsideConfig); err != nil {
		t.Fatalf("expected generated external config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".env.example")); !os.IsNotExist(err) {
		t.Fatalf("expected no env example for external config, got err=%v", err)
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
	if strings.Contains(content, "\nAPP_BASE_URL=") || strings.HasPrefix(content, "APP_BASE_URL=") {
		t.Fatalf("expected legacy APP_BASE_URL to be removed:\n%s", content)
	}
	if strings.Count(content, "BB_APP_BASE_URL=") != 1 {
		t.Fatalf("expected BB_APP_BASE_URL once:\n%s", content)
	}
	if !strings.Contains(content, "BB_CDN_PROVIDER=") {
		t.Fatalf("expected appended vars in .env.example:\n%s", content)
	}
}

func TestRunJSAuditSupportsAuditOnlyProjects(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "vendor.js"), []byte("window.vendor = true;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	configText := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[js]
enabled = false
minify = false
out_dir = "assets/js"
hash_length = 8

[js.audit]
enabled = true
inline_dirs = []
max_inline_raw_bytes = 0

[[js.audit.group]]
name = "vendor"
files = ["vendor.js"]
gzip_budget_bytes = 1024
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(configText)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"js", "audit", "--config", cfgPath}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "vendor total:") || !strings.Contains(stdout.String(), "vendor.js") {
		t.Fatalf("audit output:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "assets")); !os.IsNotExist(err) {
		t.Fatalf("audit-only command created build output: %v", err)
	}
}

func TestRunJSBuildsVerbatimSourcesAndRunsAudit(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "// preserved\nwindow.app = true;\n"
	if err := os.WriteFile(filepath.Join(srcDir, "app.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	configText := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[js]
enabled = true
minify = false
out_dir = "assets/js"
hash_length = 8
src_dirs = ["src"]
copy_dirs = []
scan_template_dirs = []
templui_component_dir = ""

[js.audit]
enabled = true
inline_dirs = []
max_inline_raw_bytes = 0
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(configText)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"js", "--config", cfgPath}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "all total:") || !strings.Contains(stdout.String(), "js/app.js") {
		t.Fatalf("build audit output:\n%s", stdout.String())
	}
	manifestData, err := os.ReadFile(filepath.Join(tmp, "assets", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := map[string]string{}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	built, err := os.ReadFile(filepath.Join(tmp, "assets", filepath.FromSlash(manifest["js/app.js"])))
	if err != nil {
		t.Fatal(err)
	}
	if string(built) != source {
		t.Fatalf("verbatim source changed:\n%s", string(built))
	}
}

func TestRunJSSkipsDisabledCapability(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "buildybud.toml")
	configText := `
schema_version = 1
strict = true

[paths]
repo_root = "."
assets_root = "assets"
manifest_path = "assets/manifest.json"

[js]
enabled = false
minify = true
out_dir = "assets/js"
hash_length = 8

[js.audit]
enabled = false
`
	if err := os.WriteFile(cfgPath, []byte(strings.TrimSpace(configText)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"js", "--config", cfgPath}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "assets")); !os.IsNotExist(err) {
		t.Fatalf("disabled JS command created output: %v", err)
	}
}
