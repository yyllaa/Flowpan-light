// plugin-scan reports statically visible host calls, including SDK wrappers.
// Dynamic dispatch still needs workflow tests; this is not an execution proof.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type report struct {
	Tools        []string `json:"tools"`
	DynamicCalls []string `json:"dynamic_calls"`
	UnknownTools []string `json:"unknown_tools"`
}

func stringValue(e ast.Expr, constants map[string]string) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, _ := strconv.Unquote(v.Value)
			return s
		}
	case *ast.Ident:
		return constants[v.Name]
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			a, b := stringValue(v.X, constants), stringValue(v.Y, constants)
			if a != "" && b != "" {
				return a + b
			}
		}
	}
	return ""
}

func sorted(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for v := range set {
		values = append(values, v)
	}
	sort.Strings(values)
	return values
}

func files(dir string, recursive bool) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != dir && (!recursive || d.Name() == "vendor" || d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

func scan(dir, sdk string, known map[string]bool) (report, error) {
	fset := token.NewFileSet()
	parse := func(dir string, recursive bool) ([]*ast.File, error) {
		paths, err := files(dir, recursive)
		if err != nil {
			return nil, err
		}
		var result []*ast.File
		for _, path := range paths {
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil, err
			}
			result = append(result, f)
		}
		return result, nil
	}
	sdkFiles, err := parse(sdk, false)
	if err != nil {
		return report{}, err
	}
	functions := map[string]map[string]bool{}
	edges := map[string][]string{}
	for _, f := range sdkFiles {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			name := fn.Name.Name
			functions[name] = map[string]bool{}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok {
					edges[name] = append(edges[name], id.Name)
					for _, arg := range call.Args {
						if v := stringValue(arg, nil); known[v] {
							functions[name][v] = true
						}
					}
				}
				return true
			})
		}
	}
	for changed := true; changed; {
		changed = false
		for name, calls := range edges {
			for _, callee := range calls {
				for tool := range functions[callee] {
					if !functions[name][tool] {
						functions[name][tool], changed = true, true
					}
				}
			}
		}
	}
	pluginFiles, err := parse(dir, true)
	if err != nil {
		return report{}, err
	}
	constants := map[string]string{}
	for _, f := range pluginFiles {
		for _, decl := range f.Decls {
			if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.CONST {
				for _, spec := range d.Specs {
					v := spec.(*ast.ValueSpec)
					for i, name := range v.Names {
						if i < len(v.Values) {
							constants[name.Name] = stringValue(v.Values[i], constants)
						}
					}
				}
			}
		}
	}
	tools, dynamic, unknown := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, f := range pluginFiles {
		aliases := map[string]bool{}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasSuffix(path, "/sdk/pluginwasm") {
				name := "pluginwasm"
				if imp.Name != nil {
					name = imp.Name.Name
				}
				aliases[name] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, sdkCall := "", false
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
				if id, ok := fn.X.(*ast.Ident); ok {
					sdkCall = aliases[id.Name]
				}
			case *ast.Ident:
				name, sdkCall = fn.Name, aliases["."]
			}
			if sdkCall {
				for tool := range functions[name] {
					tools[tool] = true
				}
			}
			if (name == "Call" || name == "call") && len(call.Args) > 0 {
				tool := stringValue(call.Args[0], constants)
				if known[tool] {
					tools[tool] = true
				} else if sdkCall && tool != "" {
					unknown[tool] = true
				} else if tool == "" {
					pos := fset.Position(call.Pos())
					rel, _ := filepath.Rel(dir, pos.Filename)
					dynamic[fmt.Sprintf("%s:%d", filepath.ToSlash(rel), pos.Line)] = true
				}
			}
			return true
		})
	}
	return report{sorted(tools), sorted(dynamic), sorted(unknown)}, nil
}

func main() {
	dir := flag.String("plugin", "", "plugin source directory")
	sdk := flag.String("sdk", "sdk/pluginwasm", "SDK source directory")
	catalog := flag.String("catalog", "plugins/host-baseline.json", "pinned host capability snapshot")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "-plugin is required")
		os.Exit(2)
	}
	var baseline struct {
		Capabilities struct {
			Tools []struct{ Name string } `json:"tools"`
		} `json:"capabilities"`
	}
	b, err := os.ReadFile(*catalog)
	if err == nil {
		err = json.Unmarshal(b, &baseline)
	}
	known := map[string]bool{}
	for _, tool := range baseline.Capabilities.Tools {
		known[tool.Name] = true
	}
	if err == nil && len(known) == 0 {
		err = fmt.Errorf("empty host tool catalog")
	}
	var result report
	if err == nil {
		result, err = scan(*dir, *sdk, known)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
