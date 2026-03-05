package templuimap

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jasonwillschiu/buildybud/internal/config"
)

type routeTarget struct {
	prefix    string
	importDir string
}

type detector struct {
	name             string
	scriptPattern    *regexp.Regexp
	componentPattern *regexp.Regexp
	markers          [][]byte
	addAs            string
}

func Generate(cfg *config.Config) error {
	if err := Check(cfg); err != nil {
		return err
	}

	componentSet, err := availableSet(cfg)
	if err != nil {
		return err
	}

	routeMap := map[string]map[string]bool{}
	for _, rule := range cfg.TempluiMap.Rules {
		set := map[string]bool{}
		for _, name := range cfg.TempluiMap.DefaultComponents {
			set[name] = true
		}
		for _, name := range rule.Components {
			set[name] = true
		}
		expandDependencies(set, cfg.JS.Dependencies)

		for name := range set {
			if cfg.TempluiMap.FailOnMissingComponent && !componentSet[name] {
				return fmt.Errorf("templui_map.rule %s references missing component %q", rule.Prefix, name)
			}
		}
		routeMap[rule.Prefix] = set
	}

	return writeOutput(cfg.RepoPath(cfg.TempluiMap.Out), routeMap)
}

func Check(cfg *config.Config) error {
	componentSet, err := availableSet(cfg)
	if err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, rule := range cfg.TempluiMap.Rules {
		if !strings.HasPrefix(rule.Prefix, "/") {
			return fmt.Errorf("invalid templui rule prefix: %s", rule.Prefix)
		}
		if seen[rule.Prefix] {
			return fmt.Errorf("duplicate templui rule prefix: %s", rule.Prefix)
		}
		seen[rule.Prefix] = true
		for _, c := range append(append([]string{}, cfg.TempluiMap.DefaultComponents...), rule.Components...) {
			if cfg.TempluiMap.FailOnMissingComponent && !componentSet[c] {
				return fmt.Errorf("templui component %q does not exist in %s", c, cfg.TempluiMap.ComponentDir)
			}
		}
	}
	return nil
}

func Suggest(cfg *config.Config) error {
	if !cfg.TempluiMap.Suggest.Enabled {
		fmt.Println("templui-map suggest is disabled in config")
		return nil
	}

	componentNames, err := getAvailableTempluiComponents(cfg.RepoPath(cfg.TempluiMap.ComponentDir))
	if err != nil {
		return err
	}
	if len(componentNames) == 0 {
		return fmt.Errorf("no templui components found in %s", cfg.TempluiMap.ComponentDir)
	}
	targets, err := discoverRoutes(cfg.RepoPath(cfg.TempluiMap.Suggest.ScanRouter), cfg.ModulePath)
	if err != nil {
		return err
	}
	detectors := buildDetectors(componentNames)

	baseComponents := map[string]bool{}
	for _, scanDir := range cfg.TempluiMap.Suggest.ScanDirs {
		s := scanDirForComponents(cfg.RepoPath(scanDir), detectors)
		for k := range s {
			baseComponents[k] = true
		}
	}
	expandDependencies(baseComponents, cfg.JS.Dependencies)
	if len(baseComponents) == 0 {
		baseComponents["dialog"] = true
	}

	routes := map[string][]string{}
	for _, target := range targets {
		viewDir := filepath.Join(cfg.Paths.RepoRoot, target.importDir, "view")
		components := scanDirForComponents(viewDir, detectors)
		expandDependencies(components, cfg.JS.Dependencies)
		for name := range baseComponents {
			components[name] = true
		}
		names := make([]string, 0, len(components))
		for name := range components {
			names = append(names, name)
		}
		sort.Strings(names)
		routes[target.prefix] = names
	}

	prefixes := make([]string, 0, len(routes))
	for p := range routes {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	fmt.Println("Suggested templui_map rules:")
	for _, p := range prefixes {
		if len(routes[p]) == 0 {
			continue
		}
		fmt.Printf("[[templui_map.rule]]\n")
		fmt.Printf("prefix = %q\n", p)
		fmt.Printf("components = [")
		for i, c := range routes[p] {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%q", c)
		}
		fmt.Println("]")
		fmt.Println()
	}
	return nil
}

func availableSet(cfg *config.Config) (map[string]bool, error) {
	names, err := getAvailableTempluiComponents(cfg.RepoPath(cfg.TempluiMap.ComponentDir))
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return set, nil
}

func expandDependencies(set map[string]bool, deps map[string][]string) {
	changed := true
	for changed {
		changed = false
		for name := range set {
			for _, dep := range deps[name] {
				if !set[dep] {
					set[dep] = true
					changed = true
				}
			}
		}
	}
}

func discoverRoutes(routerPath, modulePath string) ([]routeTarget, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, routerPath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse router: %w", err)
	}

	imports := map[string]string{}
	for _, spec := range file.Imports {
		pathVal := strings.Trim(spec.Path.Value, "\"")
		base := filepath.Base(pathVal)
		ident := base
		if spec.Name != nil && spec.Name.Name != "." && spec.Name.Name != "_" {
			ident = spec.Name.Name
		}
		imports[ident] = pathVal
	}

	var targets []routeTarget
	var inspectFunc func(node ast.Node, prefix string)
	inspectFunc = func(node ast.Node, prefix string) {
		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Route" && len(call.Args) >= 2 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					p, _ := strconv.Unquote(lit.Value)
					if fn, ok := call.Args[1].(*ast.FuncLit); ok {
						inspectFunc(fn.Body, joinPrefix(prefix, p))
						return false
					}
				}
			}

			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "RegisterRoutes" {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if importPath, ok := imports[ident.Name]; ok {
						targets = append(targets, routeTarget{prefix: prefix, importDir: importPathToDir(importPath, modulePath)})
					}
				}
			}
			return true
		})
	}

	inspectFunc(file, "/")
	return targets, nil
}

func joinPrefix(parent, child string) string {
	if child == "" || child == "/" {
		if parent == "" {
			return "/"
		}
		return parent
	}
	if !strings.HasPrefix(child, "/") {
		child = "/" + child
	}
	if parent == "" || parent == "/" {
		return child
	}
	return strings.TrimRight(parent, "/") + child
}

func importPathToDir(importPath, modulePath string) string {
	if modulePath != "" {
		importPath = strings.TrimPrefix(importPath, modulePath+"/")
	}
	return filepath.ToSlash(importPath)
}

func getAvailableTempluiComponents(jsDir string) ([]string, error) {
	entries, err := os.ReadDir(jsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read templui js dir: %w", err)
	}

	var components []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".js")
		name = strings.TrimSuffix(name, ".min")
		if name != "" {
			components = append(components, name)
		}
	}
	sort.Strings(components)
	return components, nil
}

func buildDetectors(componentNames []string) []detector {
	var detectors []detector
	for _, name := range componentNames {
		detectors = append(detectors, detector{
			name:             name,
			scriptPattern:    regexp.MustCompile(fmt.Sprintf(`@%s\.Script\(\)`, regexp.QuoteMeta(name))),
			componentPattern: regexp.MustCompile(fmt.Sprintf(`@%s\.`, regexp.QuoteMeta(name))),
			markers:          [][]byte{[]byte("data-tui-" + name)},
		})
	}
	detectors = append(detectors, detector{name: "sheet", addAs: "dialog", markers: [][]byte{[]byte("data-tui-sheet")}})
	return detectors
}

func scanDirForComponents(dir string, detectors []detector) map[string]bool {
	components := map[string]bool{}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return components
	}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".templ" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, det := range detectors {
			if components[det.name] {
				continue
			}
			if (det.scriptPattern != nil && det.scriptPattern.Match(data)) || (det.componentPattern != nil && det.componentPattern.Match(data)) {
				markComponent(components, det)
				continue
			}
			for _, m := range det.markers {
				if bytes.Contains(data, m) {
					markComponent(components, det)
					break
				}
			}
		}
		return nil
	})
	return components
}

func markComponent(set map[string]bool, det detector) {
	name := det.name
	if det.addAs != "" {
		name = det.addAs
	}
	set[name] = true
}

func writeOutput(path string, routeMap map[string]map[string]bool) error {
	if len(routeMap) == 0 {
		return fmt.Errorf("no templui components configured for any routes")
	}

	prefixes := make([]string, 0, len(routeMap))
	for p := range routeMap {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	var buf bytes.Buffer
	buf.WriteString("// Code generated by buildybud templui-map; DO NOT EDIT.\n")
	buf.WriteString("package templui\n\n")
	buf.WriteString("import \"strings\"\n\n")
	buf.WriteString("var routeScripts = map[string][]string{\n")
	for _, p := range prefixes {
		names := make([]string, 0, len(routeMap[p]))
		for name := range routeMap[p] {
			names = append(names, name)
		}
		sort.Strings(names)
		buf.WriteString(fmt.Sprintf("\t%q: {", p))
		for i, name := range names {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("%q", name))
		}
		buf.WriteString("},\n")
	}
	buf.WriteString("}\n\n")
	buf.WriteString("// ScriptsForPath returns templui script component names for a given request path using longest-prefix match.\n")
	buf.WriteString("func ScriptsForPath(path string) []string {\n")
	buf.WriteString("\tbestPrefix := \"\"\n")
	buf.WriteString("\tfor prefix := range routeScripts {\n")
	buf.WriteString("\t\tif !strings.HasPrefix(path, prefix) {\n")
	buf.WriteString("\t\t\tcontinue\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif len(prefix) > len(bestPrefix) {\n")
	buf.WriteString("\t\t\tbestPrefix = prefix\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn routeScripts[bestPrefix]\n")
	buf.WriteString("}\n")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure output dir: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
