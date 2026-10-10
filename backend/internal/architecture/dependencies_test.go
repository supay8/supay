package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These tests also protect developers running only go test. depguard enforces
// the same import boundaries in CI; named application contracts live in ports.
func TestCoreDependencyDirection(t *testing.T) {
	const module = "github.com/brandsrx/supay/internal/"
	for _, layer := range []string{"domain", "ports", "usecase", "delivery"} {
		err := filepath.WalkDir(filepath.Join("..", layer), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			source, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			for _, spec := range source.Imports {
				imported, _ := strconv.Unquote(spec.Path.Value)
				if layer == "delivery" {
					if strings.HasPrefix(imported, module+"adapters") {
						t.Errorf("%s imports %s", path, imported)
					}
					continue
				}
				if strings.HasPrefix(imported, "gorm.io/") || strings.HasPrefix(imported, "github.com/ron86i/go-siat") {
					t.Errorf("%s imports infrastructure %s", path, imported)
				}
				if strings.HasPrefix(imported, module) {
					dependency := strings.Split(strings.TrimPrefix(imported, module), "/")[0]
					allowed := dependency == "domain" || (layer != "domain" && dependency == "ports") || (layer == "usecase" && dependency == "usecase")
					if !allowed {
						t.Errorf("%s imports infrastructure %s", path, imported)
					}
				}
			}
			if layer == "domain" || layer == "usecase" {
				for _, decl := range source.Decls {
					if group, ok := decl.(*ast.GenDecl); ok {
						for _, spec := range group.Specs {
							if contract, ok := spec.(*ast.TypeSpec); ok {
								if _, ok = contract.Type.(*ast.InterfaceType); ok {
									t.Errorf("%s: move contract %s to ports", path, contract.Name.Name)
								}
							}
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestInternalPackagesDoNotTerminateProcess(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		aliases := map[string]string{}
		for _, spec := range source.Imports {
			imported, _ := strconv.Unquote(spec.Path.Value)
			alias := filepath.Base(imported)
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			aliases[alias] = imported
		}
		ast.Inspect(source, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			pkg := aliases[receiver.Name]
			name := selector.Sel.Name
			if (pkg == "log" && strings.HasPrefix(name, "Fatal")) || (pkg == "os" && name == "Exit") {
				t.Errorf("%s: %s.%s must propagate errors", path, pkg, name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
