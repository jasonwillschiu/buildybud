package js

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditReportsManifestFilesVendorFilesAndInlineScripts(t *testing.T) {
	root := t.TempDir()
	assetsRoot := filepath.Join(root, "assets")
	jsDir := filepath.Join(assetsRoot, "js")
	templateDir := filepath.Join(root, "ui")
	if err := os.MkdirAll(jsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(jsDir, "app.12345678.js"), "function app(){return 1}\n")
	vendorPath := filepath.Join(jsDir, "vendor.js")
	writeTestFile(t, vendorPath, "window.vendor=true;\n")
	manifestPath := filepath.Join(assetsRoot, "manifest.json")
	manifestData, err := json.Marshal(map[string]string{"js/app.js": "js/app.12345678.js"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(templateDir, "page.templ"), `<script src="/external.js"></script>
<script type="application/ld+json">{"name":"ignored"}</script>
<script>window.inline = true;</script>`)

	var output bytes.Buffer
	result, err := Audit(AuditOptions{
		AssetsRoot:        assetsRoot,
		ManifestPath:      manifestPath,
		InlineDirs:        []string{templateDir},
		MaxInlineRawBytes: 128,
		Groups: []AuditGroup{{
			Name:            "global",
			Assets:          []string{"js/app.js"},
			Files:           []string{vendorPath},
			GzipBudgetBytes: 1024,
		}},
		Writer: &output,
	})
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.Groups) != 1 || len(result.Groups[0].Assets) != 2 {
		t.Fatalf("groups = %#v", result.Groups)
	}
	if len(result.InlineFiles) != 1 || result.InlineFiles[0].RawBytes != len("window.inline = true;") {
		t.Fatalf("inline files = %#v", result.InlineFiles)
	}
	for _, want := range []string{"global", "js/app.js", "vendor.js", "inline JavaScript by template"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("audit output missing %q:\n%s", want, output.String())
		}
	}
}

func TestAuditEnforcesGroupAndInlineBudgets(t *testing.T) {
	root := t.TempDir()
	assetsRoot := filepath.Join(root, "assets")
	jsDir := filepath.Join(assetsRoot, "js")
	templateDir := filepath.Join(root, "ui")
	if err := os.MkdirAll(jsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(jsDir, "app.hash.js"), strings.Repeat("window.x=true;", 20))
	manifestPath := filepath.Join(assetsRoot, "manifest.json")
	writeTestFile(t, manifestPath, `{"js/app.js":"js/app.hash.js"}`)
	writeTestFile(t, filepath.Join(templateDir, "page.templ"), `<script>window.inline = "too large";</script>`)

	_, err := Audit(AuditOptions{
		AssetsRoot:   assetsRoot,
		ManifestPath: manifestPath,
		Groups: []AuditGroup{{
			Name:            "global",
			Assets:          []string{"js/app.js"},
			GzipBudgetBytes: 1,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "over budget") {
		t.Fatalf("expected gzip budget error, got %v", err)
	}

	_, err = Audit(AuditOptions{
		AssetsRoot:        assetsRoot,
		ManifestPath:      manifestPath,
		InlineDirs:        []string{templateDir},
		MaxInlineRawBytes: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "inline JavaScript") {
		t.Fatalf("expected inline budget error, got %v", err)
	}
}

func TestAuditAllowsProjectsWithNoJavaScript(t *testing.T) {
	root := t.TempDir()
	var output bytes.Buffer
	result, err := Audit(AuditOptions{
		AssetsRoot:   filepath.Join(root, "assets"),
		ManifestPath: filepath.Join(root, "assets", "manifest.json"),
		InlineDirs:   []string{filepath.Join(root, "missing")},
		Writer:       &output,
	})
	if err != nil {
		t.Fatalf("Audit empty project: %v", err)
	}
	if len(result.Groups) != 1 || result.Groups[0].GzipBytes != 0 || len(result.InlineFiles) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(output.String(), "all total: 0 bytes gzip") || !strings.Contains(output.String(), "none") {
		t.Fatalf("unexpected empty audit output:\n%s", output.String())
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
