package envfile

import (
	"bytes"
	"os"
	"strings"
)

type exampleVar struct {
	Key           string
	LegacyKey     string
	LegacyComment string
	Comment       string
}

var exampleVars = []exampleVar{
	{
		Key:           "BB_APP_BASE_URL",
		LegacyKey:     "APP_BASE_URL",
		LegacyComment: "Required for CDN planning and for purging relative paths. BB_APP_BASE_URL is also supported.",
		Comment:       "Required for buildybud CDN planning and for purging relative paths.",
	},
	{
		Key:           "BB_CDN_PROVIDER",
		LegacyKey:     "CDN_PROVIDER",
		LegacyComment: "Optional default CDN provider: bunny or cloudflare. BB_CDN_PROVIDER is also supported.",
		Comment:       "Optional default buildybud CDN provider: bunny or cloudflare.",
	},
	{
		Key:           "BB_CDN_PURGE_HOSTS",
		LegacyKey:     "CDN_PURGE_HOSTS",
		LegacyComment: "Optional comma-separated extra hostnames to purge. BB_CDN_PURGE_HOSTS is also supported.",
		Comment:       "Optional comma-separated extra hostnames for buildybud CDN purges.",
	},
	{
		Key:           "BB_BUNNY_API_KEY",
		LegacyKey:     "BUNNY_API_KEY",
		LegacyComment: "Required when CDN_PROVIDER=bunny. BB_BUNNY_API_KEY is also supported.",
		Comment:       "Required when BB_CDN_PROVIDER=bunny.",
	},
	{
		Key:           "BB_CF_API_TOKEN",
		LegacyKey:     "CF_API_TOKEN",
		LegacyComment: "Required when CDN_PROVIDER=cloudflare. BB_CF_API_TOKEN is also supported.",
		Comment:       "Required when BB_CDN_PROVIDER=cloudflare.",
	},
	{
		Key:           "BB_CF_ZONE_ID",
		LegacyKey:     "CF_ZONE_ID",
		LegacyComment: "Required when CDN_PROVIDER=cloudflare. BB_CF_ZONE_ID is also supported.",
		Comment:       "Required when BB_CDN_PROVIDER=cloudflare.",
	},
}

func AppendExample(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	existing = normalizeExample(existing)
	seen := parseExampleKeys(existing)
	var appendBuf bytes.Buffer
	for _, spec := range exampleVars {
		if seen[spec.Key] || (spec.LegacyKey != "" && seen[spec.LegacyKey]) {
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

func normalizeExample(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	lines := strings.Split(string(data), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isLegacyManagedComment(trimmed) || isLegacyManagedKey(trimmed) {
			continue
		}
		filtered = append(filtered, line)
	}
	return []byte(strings.Join(filtered, "\n"))
}

func isLegacyManagedComment(line string) bool {
	if !strings.HasPrefix(line, "# ") {
		return false
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "# "))
	for _, spec := range exampleVars {
		if spec.LegacyComment == line {
			return true
		}
	}
	return false
}

func isLegacyManagedKey(line string) bool {
	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	key = strings.TrimSpace(key)
	for _, spec := range exampleVars {
		if spec.LegacyKey == key {
			return true
		}
	}
	return false
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
