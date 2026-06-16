package godep

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// extractUsages walks dir's .go files and records references to exported symbols
// of private modules (imports + selector expressions), with file and line.
func extractUsages(dir string, deps []Dependency) ([]Usage, []string) {
	var privateMods []Dependency
	for _, d := range deps {
		if d.IsPrivate {
			privateMods = append(privateMods, d)
		}
	}
	if len(privateMods) == 0 {
		return nil, nil
	}
	var usages []Usage
	var warns []string
	_ = filepath.WalkDir(dir, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if de.IsDir() {
			if de.Name() == "vendor" || (strings.HasPrefix(de.Name(), ".") && path != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(de.Name(), ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			warns = append(warns, path+": "+perr.Error())
			return nil
		}
		usages = append(usages, fileUsages(fset, f, path, privateMods)...)
		return nil
	})
	return usages, warns
}

func fileUsages(fset *token.FileSet, f *ast.File, path string, privateMods []Dependency) []Usage {
	// alias -> (module, version, packagePath, import line) for private imports.
	type imp struct {
		module, version, pkg string
		line                 int
	}
	aliases := map[string]imp{}
	for _, spec := range f.Imports {
		pkgPath := strings.Trim(spec.Path.Value, `"`)
		dep, ok := longestPrefixModule(pkgPath, privateMods)
		if !ok {
			continue
		}
		alias := pkgPath[strings.LastIndex(pkgPath, "/")+1:]
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		aliases[alias] = imp{module: dep.ModulePath, version: dep.Version, pkg: pkgPath, line: fset.Position(spec.Pos()).Line}
	}
	if len(aliases) == 0 {
		return nil
	}
	var out []Usage
	used := map[string]bool{} // aliases that produced at least one selector usage
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		im, ok := aliases[id.Name]
		if !ok {
			return true
		}
		used[id.Name] = true
		out = append(out, Usage{
			ModulePath:  im.module,
			Version:     im.version,
			PackagePath: im.pkg,
			Symbol:      sel.Sel.Name,
			File:        path,
			Line:        fset.Position(sel.Sel.Pos()).Line,
		})
		return true
	})
	// Spec scenario: a private import referencing NO exported symbol (including
	// a blank/side-effect import `import _ "..."`) is still recorded as an
	// import of the module, with an empty Symbol.
	for alias, im := range aliases {
		if used[alias] {
			continue
		}
		out = append(out, Usage{
			ModulePath:  im.module,
			Version:     im.version,
			PackagePath: im.pkg,
			Symbol:      "",
			File:        path,
			Line:        im.line,
		})
	}
	return out
}

// longestPrefixModule returns the private dep whose module path is the longest
// prefix of importPath (so a package under a module resolves to that module).
func longestPrefixModule(importPath string, mods []Dependency) (Dependency, bool) {
	var best Dependency
	found := false
	for _, m := range mods {
		if importPath == m.ModulePath || strings.HasPrefix(importPath, m.ModulePath+"/") {
			if !found || len(m.ModulePath) > len(best.ModulePath) {
				best, found = m, true
			}
		}
	}
	return best, found
}
