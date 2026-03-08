package envfile

import (
	"bytes"
	"os"
	"strings"
)

type exampleVar struct {
	Key     string
	Comment string
}

var exampleVars = []exampleVar{
	{
		Key:     "APP_BASE_URL",
		Comment: "Required for CDN planning and for purging relative paths. BB_APP_BASE_URL is also supported.",
	},
	{
		Key:     "CDN_PROVIDER",
		Comment: "Optional default CDN provider: bunny or cloudflare. BB_CDN_PROVIDER is also supported.",
	},
	{
		Key:     "CDN_PURGE_HOSTS",
		Comment: "Optional comma-separated extra hostnames to purge. BB_CDN_PURGE_HOSTS is also supported.",
	},
	{
		Key:     "BUNNY_API_KEY",
		Comment: "Required when CDN_PROVIDER=bunny. BB_BUNNY_API_KEY is also supported.",
	},
	{
		Key:     "CF_API_TOKEN",
		Comment: "Required when CDN_PROVIDER=cloudflare. BB_CF_API_TOKEN is also supported.",
	},
	{
		Key:     "CF_ZONE_ID",
		Comment: "Required when CDN_PROVIDER=cloudflare. BB_CF_ZONE_ID is also supported.",
	},
}

func AppendExample(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	seen := parseExampleKeys(existing)
	var appendBuf bytes.Buffer
	for _, spec := range exampleVars {
		if seen[spec.Key] {
			continue
		}
		appendBuf.WriteString("# ")
		appendBuf.WriteString(spec.Comment)
		appendBuf.WriteString("\n")
		appendBuf.WriteString(spec.Key)
		appendBuf.WriteString("=\n\n")
	}
	if appendBuf.Len() == 0 {
		if os.IsNotExist(err) {
			return os.WriteFile(path, nil, 0o644)
		}
		return nil
	}

	var out bytes.Buffer
	if len(existing) > 0 {
		out.Write(existing)
		if existing[len(existing)-1] != '\n' {
			out.WriteByte('\n')
		}
		out.WriteByte('\n')
	}
	out.Write(appendBuf.Bytes())
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func parseExampleKeys(data []byte) map[string]bool {
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key != "" {
			seen[key] = true
		}
	}
	return seen
}
