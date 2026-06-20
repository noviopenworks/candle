package godep

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// extractExports walks the module rooted at dir and collects exported top-level
// declarations per package, plus the module README and the package doc synopsis.
func extractExports(dir, modulePath string) (*Library, []string) {
	lib := &Library{ModulePath: modulePath}
	var warns []string
	// #nosec G304 -- module dirs are explicit user manifest inputs.
	if data, err := os.ReadFile(filepath.Join(dir, "README.md")); err == nil {
		lib.Readme = string(data)
	}
	pkgSeen := map[string]bool{}
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "vendor" || (strings.HasPrefix(d.Name(), ".") && path != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			warns = append(warns, path+": "+perr.Error())
			return nil
		}
		pkgPath := importPath(modulePath, dir, filepath.Dir(path))
		if !pkgSeen[pkgPath] && f.Doc != nil {
			lib.DocSynopsis = strings.TrimSpace((&doc.Package{}).Synopsis(f.Doc.Text()))
			pkgSeen[pkgPath] = true
		}
		if pkgPath != "" {
			has := false
			for _, p := range lib.Packages {
				if p == pkgPath {
					has = true
					break
				}
			}
			if !has {
				lib.Packages = append(lib.Packages, pkgPath)
			}
		}
		lib.Exports = append(lib.Exports, fileExports(f, pkgPath)...)
		return nil
	})
	if walkErr != nil {
		warns = append(warns, dir+": "+walkErr.Error())
	}
	return lib, warns
}

func importPath(modulePath, moduleDir, fileDir string) string {
	rel, err := filepath.Rel(moduleDir, fileDir)
	if err != nil || rel == "." {
		return modulePath
	}
	return modulePath + "/" + filepath.ToSlash(rel)
}

func fileExports(f *ast.File, pkgPath string) []Export {
	var out []Export
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil || !ast.IsExported(d.Name.Name) {
				continue // skip methods and unexported
			}
			kind := "func"
			if strings.HasPrefix(d.Name.Name, "New") && hasResults(d) {
				kind = "constructor"
			}
			out = append(out, Export{PackagePath: pkgPath, Symbol: d.Name.Name, Kind: kind, Doc: docText(d.Doc)})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !ast.IsExported(s.Name.Name) {
						continue
					}
					kind := "type"
					if _, ok := s.Type.(*ast.InterfaceType); ok {
						kind = "interface"
					}
					out = append(out, Export{PackagePath: pkgPath, Symbol: s.Name.Name, Kind: kind, Doc: docText(d.Doc)})
				case *ast.ValueSpec:
					vk := "var"
					if d.Tok.String() == "const" {
						vk = "const"
					}
					for _, n := range s.Names {
						if ast.IsExported(n.Name) {
							out = append(out, Export{PackagePath: pkgPath, Symbol: n.Name, Kind: vk, Doc: docText(d.Doc)})
						}
					}
				}
			}
		}
	}
	return out
}

func hasResults(d *ast.FuncDecl) bool { return d.Type.Results != nil && len(d.Type.Results.List) > 0 }

func docText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	return strings.TrimSpace(g.Text())
}
