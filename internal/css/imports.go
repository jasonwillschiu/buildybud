package css

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// inlineImports reads a CSS entry and replaces every top-level @import of a
// local file with that file's contents, the way a bundler does, so a native
// build can still be split into partials. Conditions carried by the import
// keep their meaning: `layer(name)` wraps the content in `@layer name {}`,
// `supports(...)` in `@supports (...) {}`, and a media list in `@media {}`.
//
// Remote imports (`https:`, `//`) cannot be inlined; they are kept and hoisted
// to the top of the output, where an @import is still valid. Imports that
// resolve to neither a local file nor a remote URL — a package name such as
// `tailwindcss`, or a root-relative `/path` — are errors, because shipping them
// unresolved would break against the fingerprinted output.
//
// Relative url() references inside an inlined file are rebased onto the entry
// file's directory, so `url(images/a.png)` in a partial still names the same
// file after it has been moved into the entry.
func inlineImports(entry string) ([]byte, error) {
	abs, err := filepath.Abs(entry)
	if err != nil {
		return nil, fmt.Errorf("resolve css input: %w", err)
	}
	inliner := &importInliner{rootDir: filepath.Dir(abs)}
	body, err := inliner.file(abs, nil)
	if err != nil {
		return nil, err
	}
	if len(inliner.remote) == 0 {
		return []byte(body), nil
	}
	return []byte(strings.Join(inliner.remote, "\n") + "\n" + body), nil
}

type importInliner struct {
	rootDir string
	remote  []string
}

func (in *importInliner) file(name string, chain []string) (string, error) {
	for _, seen := range chain {
		if seen == name {
			return "", fmt.Errorf("css @import cycle: %s", strings.Join(append(chain, name), " -> "))
		}
	}
	source, err := os.ReadFile(name)
	if err != nil {
		if len(chain) == 0 {
			return "", fmt.Errorf("read css input: %w", err)
		}
		return "", fmt.Errorf("read css @import %s (from %s): %w", name, chain[len(chain)-1], err)
	}
	text := string(source)
	if len(chain) > 0 {
		text = stripCharset(text)
		text = in.rebaseURLs(text, filepath.Dir(name))
	}
	chain = append(chain, name)

	var out strings.Builder
	cursor := 0
	for _, stmt := range topLevelImports(text) {
		out.WriteString(text[cursor:stmt.start])
		replacement, err := in.statement(stmt, name, chain)
		if err != nil {
			return "", err
		}
		out.WriteString(replacement)
		cursor = stmt.end
	}
	out.WriteString(text[cursor:])
	return out.String(), nil
}

func (in *importInliner) statement(stmt importStatement, from string, chain []string) (string, error) {
	target, conditions, err := parseImport(stmt.body)
	if err != nil {
		return "", fmt.Errorf("%s: %w", from, err)
	}
	if isRemoteURL(target) {
		in.remote = append(in.remote, "@import "+strings.TrimSpace(stmt.body)+";")
		return "", nil
	}
	if strings.HasPrefix(target, "/") || !looksLikePath(target) {
		return "", fmt.Errorf("%s: css @import %q cannot be resolved by the native processor; use a path relative to the importing file", from, target)
	}
	resolved := filepath.Join(filepath.Dir(from), filepath.FromSlash(target))
	content, err := in.file(resolved, chain)
	if err != nil {
		return "", err
	}
	return wrapConditions(content, conditions), nil
}

type importConditions struct {
	layer    bool
	layerArg string
	supports string
	media    string
}

func wrapConditions(content string, c importConditions) string {
	if c.layer {
		if c.layerArg != "" {
			content = "@layer " + c.layerArg + " {\n" + content + "\n}"
		} else {
			content = "@layer {\n" + content + "\n}"
		}
	}
	if c.supports != "" {
		content = "@supports (" + c.supports + ") {\n" + content + "\n}"
	}
	if c.media != "" {
		content = "@media " + c.media + " {\n" + content + "\n}"
	}
	return content
}

type importStatement struct {
	start int // offset of "@import"
	end   int // offset just past the closing ';'
	body  string
}

// topLevelImports finds @import statements outside every block, skipping
// comments and strings, so `/* @import x */` or `content: "@import"` is left
// alone.
func topLevelImports(text string) []importStatement {
	var stmts []importStatement
	depth := 0
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == '/' && i+1 < len(text) && text[i+1] == '*':
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return stmts
			}
			i += end + 4
		case c == '"' || c == '\'':
			i = skipString(text, i)
		case c == '{':
			depth++
			i++
		case c == '}':
			if depth > 0 {
				depth--
			}
			i++
		case c == '@' && depth == 0 && hasKeyword(text[i:], "@import"):
			end := statementEnd(text, i)
			stmts = append(stmts, importStatement{
				start: i,
				end:   end,
				body:  strings.TrimSuffix(text[i+len("@import"):end], ";"),
			})
			i = end
		default:
			i++
		}
	}
	return stmts
}

func hasKeyword(text, keyword string) bool {
	if !strings.HasPrefix(strings.ToLower(text), keyword) {
		return false
	}
	if len(text) == len(keyword) {
		return true
	}
	next := text[len(keyword)]
	return next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '\'' || next == '"' || next == 'u' || next == 'U'
}

func statementEnd(text string, start int) int {
	depth := 0
	for i := start; i < len(text); {
		switch c := text[i]; {
		case c == '"' || c == '\'':
			i = skipString(text, i)
			continue
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case c == ';' && depth == 0:
			return i + 1
		}
		i++
	}
	return len(text)
}

func skipString(text string, start int) int {
	quote := text[start]
	for i := start + 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(text)
}

// parseImport splits an @import prelude into its URL and its conditions:
// `'a.css' layer(base) supports(display: grid) screen and (min-width: 40rem)`.
func parseImport(prelude string) (string, importConditions, error) {
	rest := strings.TrimSpace(prelude)
	var target string
	switch {
	case strings.HasPrefix(rest, "\"") || strings.HasPrefix(rest, "'"):
		end := skipString(rest, 0)
		target = rest[1 : end-1]
		rest = rest[end:]
	case strings.HasPrefix(strings.ToLower(rest), "url("):
		end := strings.Index(rest, ")")
		if end < 0 {
			return "", importConditions{}, fmt.Errorf("css @import has an unclosed url(): %q", prelude)
		}
		target = strings.Trim(strings.TrimSpace(rest[4:end]), "\"'")
		rest = rest[end+1:]
	default:
		return "", importConditions{}, fmt.Errorf("css @import needs a quoted path or url(): %q", prelude)
	}

	var c importConditions
	rest = strings.TrimSpace(rest)
	if lower := strings.ToLower(rest); strings.HasPrefix(lower, "layer(") {
		end := strings.Index(rest, ")")
		if end < 0 {
			return "", c, fmt.Errorf("css @import has an unclosed layer(): %q", prelude)
		}
		c.layer = true
		c.layerArg = strings.TrimSpace(rest[len("layer("):end])
		rest = strings.TrimSpace(rest[end+1:])
	} else if lower == "layer" || strings.HasPrefix(lower, "layer ") {
		c.layer = true
		rest = strings.TrimSpace(rest[len("layer"):])
	}
	if strings.HasPrefix(strings.ToLower(rest), "supports(") {
		closeAt := matchingParen(rest, len("supports"))
		if closeAt < 0 {
			return "", c, fmt.Errorf("css @import has an unclosed supports(): %q", prelude)
		}
		c.supports = strings.TrimSpace(rest[len("supports("):closeAt])
		rest = strings.TrimSpace(rest[closeAt+1:])
	}
	c.media = strings.TrimSpace(rest)
	return target, c, nil
}

func matchingParen(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isRemoteURL(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//")
}

// looksLikePath accepts what a stylesheet author writes for a sibling file
// (`./a.css`, `../partials/a.css`, `partials/a.css`) and rejects a bare
// package name, which only a bundler with module resolution could find.
func looksLikePath(target string) bool {
	return strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") || strings.HasSuffix(strings.ToLower(target), ".css")
}

func stripCharset(text string) string {
	trimmed := strings.TrimPrefix(text, "\uFEFF")
	if strings.HasPrefix(strings.ToLower(trimmed), "@charset") {
		if end := strings.Index(trimmed, ";"); end >= 0 {
			return trimmed[end+1:]
		}
	}
	return trimmed
}

// rebaseURLs rewrites relative url() references found in fromDir so they
// resolve the same from the entry file's directory.
func (in *importInliner) rebaseURLs(text, fromDir string) string {
	if fromDir == in.rootDir {
		return text
	}
	var out strings.Builder
	for {
		index := indexURLFunc(text)
		if index < 0 {
			out.WriteString(text)
			return out.String()
		}
		open := index + len("url(")
		close := strings.Index(text[open:], ")")
		if close < 0 {
			out.WriteString(text)
			return out.String()
		}
		close += open
		raw := strings.TrimSpace(text[open:close])
		quote := ""
		if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '\'') && raw[len(raw)-1] == raw[0] {
			quote = raw[:1]
			raw = raw[1 : len(raw)-1]
		}
		out.WriteString(text[:open])
		if rebased, ok := in.rebase(raw, fromDir); ok {
			out.WriteString(quote + rebased + quote)
		} else {
			out.WriteString(text[open:close])
		}
		text = text[close:]
	}
}

func indexURLFunc(text string) int {
	lower := strings.ToLower(text)
	for start := 0; ; {
		index := strings.Index(lower[start:], "url(")
		if index < 0 {
			return -1
		}
		index += start
		if index == 0 || !isIdentChar(lower[index-1]) {
			return index
		}
		start = index + len("url(")
	}
}

func isIdentChar(c byte) bool {
	return c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func (in *importInliner) rebase(ref, fromDir string) (string, bool) {
	lower := strings.ToLower(ref)
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "/") || strings.Contains(lower, ":") || strings.HasPrefix(lower, "var(") {
		return "", false
	}
	rel, err := filepath.Rel(in.rootDir, filepath.Join(fromDir, filepath.FromSlash(ref)))
	if err != nil {
		return "", false
	}
	rel = path.Clean(filepath.ToSlash(rel))
	if !strings.HasPrefix(rel, "../") {
		rel = "./" + rel
	}
	return rel, true
}
