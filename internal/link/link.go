// Package link matches contract operations (currently proto RPCs) to their
// implementation symbols in a repo's code graph. It is intentionally generic so
// the OpenAPI handler linker can adopt it later.
package link

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// RPC is the subset of an RPC the linker needs.
type RPC struct {
	FullName   string
	Service    string
	Name       string
	StreamKind string
}

const (
	confHigh   = 0.9
	confMedium = 0.6
	confLow    = 0.3
)

// MatchRPCs returns impl links for rpcs within a single index. Each candidate is
// recorded; ambiguous matches keep their tier rather than being dropped or
// collapsed. root is the absolute source root used to resolve node source files
// for AST analysis; an empty root disables AST and falls back to the string-scan
// heuristic.
func MatchRPCs(s *store.Store, indexID int64, rpcs []RPC, root string) ([]store.RPCImplLink, error) {
	var out []store.RPCImplLink
	for _, r := range rpcs {
		nodes, err := s.NodesByLabel(indexID, r.Name)
		if err != nil {
			return nil, err
		}
		serviceRegistered, err := hasServiceRegistration(s, indexID, r.Service)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			conf, reason := score(root, n, r, serviceRegistered)
			out = append(out, store.RPCImplLink{
				RPCFullName: r.FullName, NodeID: n.NodeID, Confidence: conf, MatchReason: reason,
			})
		}
	}
	return out, nil
}

func hasServiceRegistration(s *store.Store, indexID int64, service string) (bool, error) {
	for _, label := range []string{"Register" + service + "Server", service + "Server"} {
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

// score maps the available signals to a confidence tier and a reason string.
//
// AST is authoritative when source is available:
//   - astSignatureMatch (true, true)  → HIGH, reason includes "ast".
//   - astSignatureMatch (false, true) → AST rejects the shape: do NOT grant HIGH,
//     keep the name/service tier with a heuristic-free reason.
//   - astSignatureMatch (_, false)    → source unavailable: fall back to the
//     legacy string-scan to decide the HIGH tier (no regression).
func score(root string, n store.NodeRow, r RPC, serviceRegistered bool) (float64, string) {
	reason := "name"
	conf := confLow
	if serviceRegistered {
		reason = "name+service"
		conf = confMedium
	}

	matched, ok := astSignatureMatch(root, n.SourceFile, r.Name, r.StreamKind)
	if ok {
		// AST is authoritative; only a positive match earns HIGH.
		if matched {
			reason += "+ast"
			conf = confHigh
		}
		return conf, reason
	}

	// Source unavailable: fall back to the legacy string-scan heuristic.
	if signatureMatches(n.SourceFile, r.Name, r.StreamKind) {
		reason += "+signature"
		conf = confHigh
	}
	return conf, reason
}

// astSignatureMatch parses filepath.Join(root, sourceFile) with go/parser and
// checks whether a method (FuncDecl with a receiver) named rpcName has a shape
// consistent with streamKind.
//
//   - ok=false when root is empty, the resolved file is unreadable, or the file
//     fails to parse — the caller should fall back to the string-scan heuristic.
//   - ok=true with matched reporting whether a method matching the requested
//     shape was found. A "unary" streamKind requires the unary shape
//     (context.Context first param, (*Resp, error) results); any other value
//     requires the streaming shape (last param type name ends in "Server", no
//     context.Context param, result is just error). matched=false, ok=true when
//     no matching FuncDecl exists.
func astSignatureMatch(root, sourceFile, rpcName, streamKind string) (matched bool, ok bool) {
	path, src, ok := readSourceUnderRoot(root, sourceFile)
	if !ok {
		return false, false
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return false, false
	}
	wantUnary := streamKind == "unary"
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Recv == nil || fn.Name == nil || fn.Name.Name != rpcName {
			continue
		}
		if classifyUnary(fn) {
			if wantUnary {
				return true, true
			}
			continue
		}
		if classifyStreaming(fn) {
			if !wantUnary {
				return true, true
			}
			continue
		}
	}
	return false, true
}

// classifyUnary reports whether fn has the unary server-impl shape:
// first param is context.Context and results are (*Resp, error).
func classifyUnary(fn *ast.FuncDecl) bool {
	params := fieldTypes(fn.Type.Params)
	results := fieldTypes(fn.Type.Results)
	if len(params) != 2 || len(results) != 2 {
		return false
	}
	if !isContextContext(params[0]) {
		return false
	}
	return isErrorType(results[1])
}

// classifyStreaming reports whether fn has the streaming server-impl shape:
// the last param's type name ends in "Server", there is no context.Context
// param, and the single result is error.
func classifyStreaming(fn *ast.FuncDecl) bool {
	params := fieldTypes(fn.Type.Params)
	results := fieldTypes(fn.Type.Results)
	if len(params) == 0 || len(results) != 1 {
		return false
	}
	for _, p := range params {
		if isContextContext(p) {
			return false
		}
	}
	if !strings.HasSuffix(typeName(params[len(params)-1]), "Server") {
		return false
	}
	return isErrorType(results[0])
}

// fieldTypes flattens a field list into one entry per parameter/result,
// accounting for grouped fields like (a, b T).
func fieldTypes(fl *ast.FieldList) []ast.Expr {
	if fl == nil {
		return nil
	}
	var out []ast.Expr
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, f.Type)
		}
	}
	return out
}

// isContextContext reports whether expr is the selector context.Context.
func isContextContext(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "context" && sel.Sel.Name == "Context"
}

// isErrorType reports whether expr is the builtin error identifier.
func isErrorType(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "error"
}

// typeName returns the trailing identifier name of a type expression, looking
// through pointers and selectors (e.g. *pb.Foo_SyncServer -> "Foo_SyncServer").
func typeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return typeName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// signatureMatches best-effort reads the impl source and checks the method's
// parameter shape against the RPC stream_kind. Unreadable source returns false
// (the caller keeps the lower-confidence name/service match). This is the legacy
// fallback used only when AST analysis is unavailable.
// Precondition: sourceFile (the graph node's source_file) is read directly from
// the filesystem, so the HIGH-confidence signature tier only fires when that path
// is readable from the process's working directory; otherwise the match degrades
// to the name/service (MEDIUM) tier.
func signatureMatches(sourceFile, rpcName, streamKind string) bool {
	if sourceFile == "" {
		return false
	}
	// #nosec G304 -- legacy fallback intentionally reads graph source_file when no
	// repo root was configured; unreadable or unsafe paths simply do not match.
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, ")") || !strings.Contains(line, rpcName+"(") {
			continue
		}
		if !strings.Contains(line, "func") {
			continue
		}
		// Examine only the method's own parameter list, after the method name,
		// so the receiver (e.g. "(s *Server)") cannot be mistaken for a gRPC
		// stream-server parameter.
		params := line
		if i := strings.Index(line, rpcName+"("); i >= 0 {
			params = line[i+len(rpcName):]
		}
		// gRPC streaming methods take a generated stream type such as
		// "InventoryService_SyncServer" and have no context.Context parameter.
		streaming := strings.Contains(params, "Server)") || strings.Contains(params, "Server ")
		unary := strings.Contains(params, "context.Context") && !streaming
		switch streamKind {
		case "unary":
			return unary
		default:
			return streaming
		}
	}
	return false
}

// Export is the subset of a private-library export the linker needs. SourceHint
// is an optional path fragment (e.g. package dir) used to prefer a co-located node.
type Export struct {
	PackagePath string
	Symbol      string
	SourceHint  string
	NodeID      string // filled by MatchExports
}

// MatchExports links each export to a code node by exact symbol name within the
// index. When root is set and source is parseable, it prefers the candidate node
// whose AST declaration of the symbol is in the export's package. Otherwise it
// falls back to preferring a node whose source_file contains SourceHint. Unmatched
// exports keep an empty NodeID. Returns the exports with NodeID populated.
func MatchExports(s *store.Store, indexID int64, exports []Export, root string) []Export {
	out := make([]Export, len(exports))
	copy(out, exports)
	for i := range out {
		nodes, err := s.NodesByLabel(indexID, out[i].Symbol)
		if err != nil || len(nodes) == 0 {
			continue
		}
		pick := nodes[0].NodeID

		// Prefer an AST-confirmed declaration in the export's package.
		if root != "" {
			if id, found := astExportPick(root, nodes, out[i]); found {
				out[i].NodeID = id
				continue
			}
		}

		// Fallback: prefer a node whose source file contains the SourceHint.
		if out[i].SourceHint != "" {
			for _, n := range nodes {
				if strings.Contains(n.SourceFile, out[i].SourceHint) {
					pick = n.NodeID
					break
				}
			}
		}
		out[i].NodeID = pick
	}
	return out
}

// astExportPick parses each candidate node's source file and returns the node
// whose file declares the symbol and whose package matches the export's package
// (by trailing path segment). found=false when no candidate can be confirmed.
func astExportPick(root string, nodes []store.NodeRow, e Export) (string, bool) {
	wantPkg := lastSeg(e.PackagePath)
	for _, n := range nodes {
		if n.SourceFile == "" {
			continue
		}
		path, src, ok := readSourceUnderRoot(root, n.SourceFile)
		if !ok {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			continue
		}
		if !declaresSymbol(file, e.Symbol) {
			continue
		}
		// Match the package: prefer the parsed package name, else the directory.
		if wantPkg == "" || file.Name.Name == wantPkg || lastSeg(filepath.Dir(n.SourceFile)) == wantPkg {
			return n.NodeID, true
		}
	}
	return "", false
}

func readSourceUnderRoot(root, sourceFile string) (string, []byte, bool) {
	if root == "" || sourceFile == "" {
		return "", nil, false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, false
	}
	path := filepath.Join(absRoot, sourceFile)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
		return "", nil, false
	}
	// #nosec G304 -- absPath is verified to remain under absRoot above.
	src, err := os.ReadFile(absPath)
	if err != nil {
		return "", nil, false
	}
	return absPath, src, true
}

// declaresSymbol reports whether the parsed file declares a top-level func, type,
// var, or const named sym.
func declaresSymbol(file *ast.File, sym string) bool {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name != nil && d.Name.Name == sym {
				return true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					if sp.Name != nil && sp.Name.Name == sym {
						return true
					}
				case *ast.ValueSpec:
					for _, id := range sp.Names {
						if id.Name == sym {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// lastSeg returns the trailing path segment of p (after the last '/').
func lastSeg(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
