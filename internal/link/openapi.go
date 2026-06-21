package link

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// Op is the subset of an HTTP operation the linker needs.
type Op struct {
	Method      string
	Path        string
	OperationID string
}

// MatchOpenAPI returns handler impl links for ops within a single index. Each
// name candidate becomes a link; ambiguous matches keep their tier rather than
// being dropped or collapsed, mirroring MatchRPCs. root is the absolute source
// root used to resolve node source files for AST analysis; an empty root
// disables AST and falls back to the string-scan heuristic.
func MatchOpenAPI(s *store.Store, indexID int64, ops []Op, root string) ([]store.HTTPOpImplLink, error) {
	routeRegistered, err := hasRouteRegistration(s, indexID)
	if err != nil {
		return nil, err
	}
	var out []store.HTTPOpImplLink
	for _, op := range ops {
		for _, name := range handlerNameCandidates(op.OperationID) {
			nodes, err := s.NodesByLabel(indexID, name)
			if err != nil {
				return nil, err
			}
			for _, n := range nodes {
				conf, reason := scoreHTTP(root, n, name, routeRegistered)
				out = append(out, store.HTTPOpImplLink{
					Method: op.Method, Path: op.Path, NodeID: n.NodeID,
					Confidence: conf, MatchReason: reason,
				})
			}
		}
	}
	return out, nil
}

// handlerNameCandidates derives handler-name candidates from operationId only:
// the operationId verbatim and its PascalCase (exported) form, deduped. An empty
// operationId yields no candidates → the op contributes no link.
func handlerNameCandidates(operationID string) []string {
	if operationID == "" {
		return nil
	}
	out := []string{operationID}
	if title := titleFirst(operationID); title != operationID {
		out = append(out, title)
	}
	return out
}

// titleFirst upper-cases the first rune of s (ASCII), e.g. "reserveProduct" ->
// "ReserveProduct". It does not touch the remainder, matching Go exported-method
// naming where only the leading rune differs from a camelCase operationId.
func titleFirst(s string) string {
	if s == "" {
		return s
	}
	c := s[0]
	if c >= 'a' && c <= 'z' {
		return string(c-('a'-'A')) + s[1:]
	}
	return s
}

// hasRouteRegistration is a coarse, existence-based signal analogous to
// hasServiceRegistration: it reports whether the repo contains any HTTP
// route-registration infrastructure node. It is computed once per MatchOpenAPI
// call, not per op. Precise path→handler binding is deferred.
func hasRouteRegistration(s *store.Store, indexID int64) (bool, error) {
	for _, label := range []string{"HandleFunc", "Handle", "NewServeMux", "NewRouter", "registerRoutes"} {
		nodes, err := s.NodesByLabel(indexID, label)
		if err != nil {
			return false, err
		}
		if len(nodes) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// scoreHTTP maps the available signals to a confidence tier and reason string,
// mirroring score() for RPCs:
//   - name alone                     → LOW  "name"
//   - + route-registration presence  → MEDIUM "name+route"
//   - + AST-confirmed handler shape   → HIGH "...+ast" (root available)
//   - + string-scan confirms shape    → HIGH "...+signature" (root absent)
//
// AST is authoritative: when the source is readable but the declaration is not
// a handler, the candidate keeps its name/route tier and is never promoted.
func scoreHTTP(root string, n store.NodeRow, name string, routeRegistered bool) (float64, string) {
	reason := "name"
	conf := confLow
	if routeRegistered {
		reason = "name+route"
		conf = confMedium
	}

	matched, ok := astHTTPHandlerMatch(root, n.SourceFile, name)
	if ok {
		if matched {
			reason += "+ast"
			conf = confHigh
		}
		return conf, reason
	}

	if httpSignatureScan(n.SourceFile, name) {
		reason += "+signature"
		conf = confHigh
	}
	return conf, reason
}

// astHTTPHandlerMatch parses the node's source under root and reports whether a
// method named name is an HTTP handler. ok=false means the source was
// unavailable (caller falls back to the string scan); ok=true with matched=false
// means the source parsed but no such handler declaration exists.
func astHTTPHandlerMatch(root, sourceFile, name string) (matched bool, ok bool) {
	path, src, ok := readSourceUnderRoot(root, sourceFile)
	if !ok {
		return false, false
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return false, false
	}
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Name == nil || fn.Name.Name != name {
			continue
		}
		if classifyHTTPHandler(fn) {
			return true, true
		}
	}
	return false, true
}

// classifyHTTPHandler reports whether fn has the HTTP handler shape: exactly two
// params flattening to [http.ResponseWriter, *http.Request] and no results.
func classifyHTTPHandler(fn *ast.FuncDecl) bool {
	params := fieldTypes(fn.Type.Params)
	if len(params) != 2 {
		return false
	}
	if fn.Type.Results != nil && len(fn.Type.Results.List) != 0 {
		return false
	}
	if !isSelector(params[0], "http", "ResponseWriter") {
		return false
	}
	star, ok := params[1].(*ast.StarExpr)
	if !ok {
		return false
	}
	return isSelector(star.X, "http", "Request")
}

// isSelector reports whether expr is the selector pkg.sel (e.g. http.Request).
func isSelector(expr ast.Expr, pkg, sel string) bool {
	s, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == pkg && s.Sel.Name == sel
}

// httpSignatureScan is the legacy fallback used only when AST is unavailable
// (no root). It reads the node's source_file directly and looks for a func
// declaration of name whose params mention http.ResponseWriter and *http.Request.
// Unreadable or unsafe paths simply do not match. Sibling of signatureMatches.
func httpSignatureScan(sourceFile, name string) bool {
	if sourceFile == "" {
		return false
	}
	// #nosec G304 -- legacy fallback intentionally reads graph source_file when no
	// repo root was configured; unreadable paths simply do not match.
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "func") || !strings.Contains(line, name+"(") {
			continue
		}
		params := line
		if i := strings.Index(line, name+"("); i >= 0 {
			params = line[i+len(name):]
		}
		if strings.Contains(params, "http.ResponseWriter") &&
			strings.Contains(params, "*http.Request") {
			return true
		}
	}
	return false
}
