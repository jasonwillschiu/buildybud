package css

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCSS(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func buildNative(t *testing.T, root, entry string, minify bool) string {
	t.Helper()
	output := filepath.Join(root, "out", "bundle.css")
	if err := Run(Options{Command: NativeCommand, Input: filepath.Join(root, filepath.FromSlash(entry)), Output: output, Minify: minify}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNativeInlinesNestedRelativeImports(t *testing.T) {
	root := t.TempDir()
	writeCSS(t, root, map[string]string{
		"entries/app.css":     "@import '../partials/core.css';\n@import url(\"../partials/page.css\");\n.app { color: red; }\n",
		"partials/core.css":   "@import './tokens.css';\n.core { margin: 0; }\n",
		"partials/tokens.css": ":root { --x: 1px; }\n",
		"partials/page.css":   ".page { padding: 0; }\n",
	})
	got := buildNative(t, root, "entries/app.css", true)
	want := ":root{--x:1px}.core{margin:0}.page{padding:0}.app{color:red}"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestNativeImportConditionsWrapTheContent(t *testing.T) {
	root := t.TempDir()
	writeCSS(t, root, map[string]string{
		"app.css": "@layer reset, ts;\n" +
			"@import 'reset.css' layer(reset);\n" +
			"@import 'anon.css' layer;\n" +
			"@import 'grid.css' supports(display: grid) screen and (min-width: 40rem);\n",
		"reset.css": "a { color: inherit; }\n",
		"anon.css":  "b { color: blue; }\n",
		"grid.css":  ".g { display: grid; }\n",
	})
	got := buildNative(t, root, "app.css", false)
	for _, want := range []string{
		"@layer reset {\na { color: inherit; }\n\n}",
		"@layer {\nb { color: blue; }\n\n}",
		"@media screen and (min-width: 40rem) {\n@supports (display: grid) {\n.g { display: grid; }\n\n}\n}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if !strings.HasPrefix(got, "@layer reset, ts;") {
		t.Fatalf("layer order statement moved:\n%s", got)
	}
}

func TestNativeHoistsRemoteImportsAndIgnoresCommentsAndStrings(t *testing.T) {
	root := t.TempDir()
	writeCSS(t, root, map[string]string{
		"app.css": "/* @import 'missing.css'; */\n" +
			"@import 'local.css';\n" +
			"@import url(https://fonts.googleapis.com/css2?family=Inter);\n" +
			".q::before { content: \"@import 'nope.css';\"; }\n" +
			"@media print { .p { color: black; } }\n",
		"local.css": ".l { color: green; }\n",
	})
	got := buildNative(t, root, "app.css", false)
	if !strings.HasPrefix(got, "@import url(https://fonts.googleapis.com/css2?family=Inter);\n") {
		t.Fatalf("remote import not hoisted:\n%s", got)
	}
	if !strings.Contains(got, ".l { color: green; }") || !strings.Contains(got, "/* @import 'missing.css'; */") || !strings.Contains(got, "content: \"@import 'nope.css';\"") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestNativeRebasesRelativeURLsInImportedFiles(t *testing.T) {
	root := t.TempDir()
	writeCSS(t, root, map[string]string{
		"entries/app.css":    "@import '../vendor/leaflet.css';\n.a { background: url(local.png); }\n",
		"vendor/leaflet.css": ".m { background: url(images/marker.png); }\n.v { behavior: url(#default#VML); }\n.d { background: url(\"data:image/png;base64,AA\"); }\n.r { background: url('/abs.png'); }\n.h { background: url(https://x.test/a.png); }\n.q { background: url( \"images/q.png\" ); }\n",
	})
	got := buildNative(t, root, "entries/app.css", false)
	for _, want := range []string{
		"url(../vendor/images/marker.png)",
		"url(#default#VML)",
		"url(\"data:image/png;base64,AA\")",
		"url('/abs.png')",
		"url(https://x.test/a.png)",
		"url(\"../vendor/images/q.png\")",
		".a { background: url(local.png); }",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestNativeImportErrors(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"cycle":         {"app.css": "@import 'a.css';", "a.css": "@import 'b.css';", "b.css": "@import 'a.css';"},
		"missing":       {"app.css": "@import 'gone.css';"},
		"package":       {"app.css": "@import 'tailwindcss';"},
		"root relative": {"app.css": "@import '/styles/a.css';"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeCSS(t, root, files)
			err := Run(Options{Command: NativeCommand, Input: filepath.Join(root, "app.css"), Output: filepath.Join(root, "out.css")})
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
