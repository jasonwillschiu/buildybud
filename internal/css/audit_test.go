package css

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditReportsCombinedBundleSizeAndEnforcesBudget(t *testing.T) {
	tmp := t.TempDir()
	core := filepath.Join(tmp, "core.css")
	home := filepath.Join(tmp, "home.css")
	if err := os.WriteFile(core, []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(home, []byte(".home{display:block}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	results, err := Audit(map[string]string{"core": core, "home": home}, []AuditGroup{{
		Name:            "home-initial",
		Bundles:         []string{"core", "home"},
		GzipBudgetBytes: 1024,
	}}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].Bundles) != 2 || results[0].GzipBytes == 0 {
		t.Fatalf("results = %#v", results)
	}
	if !strings.Contains(output.String(), "home-initial total:") {
		t.Fatalf("audit output:\n%s", output.String())
	}

	_, err = Audit(map[string]string{"core": core}, []AuditGroup{{
		Name:            "too-small",
		Bundles:         []string{"core"},
		GzipBudgetBytes: 1,
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "over budget") {
		t.Fatalf("expected budget error, got %v", err)
	}
}
