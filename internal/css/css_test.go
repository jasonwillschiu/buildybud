package css

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunInvokesCommandAndWritesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(binDir, "fake-tailwind")
	script := `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    shift
    mkdir -p "$(dirname "$1")"
    printf 'body{color:red}' > "$1"
    exit 0
  fi
  shift
done
exit 1
`
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out := filepath.Join(tmp, "assets", "app.css")
	if err := Run(Options{
		Command: "fake-tailwind",
		Input:   filepath.Join(tmp, "input.css"),
		Output:  out,
		Content: []string{"ui/**/*.templ"},
		Minify:  true,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "body{color:red}" {
		t.Fatalf("output = %q", string(data))
	}
}

func TestRunNativeMinifiesCSS(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.Join(tmp, "site.css")
	output := filepath.Join(tmp, "assets", "site.css")
	if err := os.WriteFile(input, []byte("/* source */\nbody { color: red; margin: 0 1rem; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(Options{Command: NativeCommand, Input: input, Output: output, Minify: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "body{color:red;margin:0 1rem}"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunNativeCopiesCSSWhenMinifyDisabled(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.Join(tmp, "site.css")
	output := filepath.Join(tmp, "assets", "site.css")
	source := "body { color: red; }\n"
	if err := os.WriteFile(input, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(Options{Command: NativeCommand, Input: input, Output: output}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != source {
		t.Fatalf("output = %q, want verbatim source", string(data))
	}
}
