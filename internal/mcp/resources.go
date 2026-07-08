package mcp

import (
	"encoding/json"

	"github.com/noviopenworks/candle/internal/store"
)

// mustJSON marshals v to indented JSON, returning an error message string on
// failure (never panics) so it is safe to embed in tool results.
func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(b)
}

// RepoResource returns the JSON snapshot summary for repo://org/name.
func (t *Tools) RepoResource(repo string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(ri, "", "  ")
	return string(b), err
}

// GraphNodeResource returns the JSON for a node behind
// graph://org/name/commit/<sha>/node/<node_id>.
func (t *Tools) GraphNodeResource(repo, nodeID string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	node, found, err := t.s.NodeByID(ri.IndexID, nodeID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(node, "", "  ")
	return string(b), err
}

// OperationResource returns the JSON for an operation behind
// openapi://org/name/commit/<sha>/operation/<operationId>.
func (t *Tools) OperationResource(repo, operationID string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	op, found, err := t.s.OperationByID(ri.IndexID, operationID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(op, "", "  ")
	return string(b), err
}

// SchemaResource returns the JSON for a schema behind
// openapi://org/name/commit/<sha>/schema/<name>, filtered to an exact name match.
func (t *Tools) SchemaResource(repo, name string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	matches, err := t.s.FindSchemas(ri.IndexID, name)
	if err != nil {
		return "", err
	}
	for _, sc := range matches {
		if sc.Name == name {
			b, err := json.MarshalIndent(sc, "", "  ")
			return string(b), err
		}
	}
	return "", ErrNotFound
}

// ProtoFileResource returns JSON for proto://…/file/<path>.
func (t *Tools) ProtoFileResource(repo, path string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	files, err := t.s.ListProtoFiles(ri.IndexID)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.Path == path {
			return mustJSON(f), nil
		}
	}
	return "", ErrNotFound
}

// ProtoRPCResource returns JSON for proto://…/rpc/<pkg>/<service>/<rpc>.
func (t *Tools) ProtoRPCResource(repo, pkg, service, rpc string) (string, error) {
	out, err := t.ExplainRPC(repo, service, rpc)
	if err != nil {
		return "", err
	}
	return mustJSON(out), nil
}

// ProtoServiceResource returns JSON for proto://…/service/<pkg>/<service>.
func (t *Tools) ProtoServiceResource(repo, pkg, service string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	rpcs, err := t.s.FindRPCs(ri.IndexID, "", "")
	if err != nil {
		return "", err
	}
	var matched []store.ProtoRPCResult
	for _, r := range rpcs {
		if r.Service == service {
			matched = append(matched, r)
		}
	}
	if len(matched) == 0 {
		return "", ErrNotFound
	}
	return mustJSON(map[string]any{"service": service, "package": pkg, "rpcs": matched}), nil
}

// ProtoMessageResource returns JSON for proto://…/message/<pkg>/<message>.
func (t *Tools) ProtoMessageResource(repo, pkg, message string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	msgs, err := t.s.FindMessages(ri.IndexID, message)
	if err != nil {
		return "", err
	}
	full := pkg + "." + message
	for _, m := range msgs {
		if m.Name == message || m.FullName == full || m.FullName == message {
			return mustJSON(m), nil
		}
	}
	return "", ErrNotFound
}

// LibraryResource returns JSON for lib://<module-path> (provider library + exports).
func (t *Tools) LibraryResource(modulePath string) (string, error) {
	lib, ok, err := t.s.PrivateLibraryByModule(modulePath)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	return mustJSON(lib), nil
}

// LibraryPackageResource returns JSON for lib://<module-path>/package/<pkg>.
func (t *Tools) LibraryPackageResource(modulePath, pkg string) (string, error) {
	lib, ok, err := t.s.PrivateLibraryByModule(modulePath)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	var exps []store.PrivateExport
	for _, e := range lib.Exports {
		if e.PackagePath == pkg {
			exps = append(exps, e)
		}
	}
	if len(exps) == 0 {
		return "", ErrNotFound
	}
	return mustJSON(map[string]any{"module_path": modulePath, "package": pkg, "exports": exps}), nil
}

// LibrarySymbolResource returns JSON for lib://<module-path>/symbol/<symbol>.
func (t *Tools) LibrarySymbolResource(modulePath, symbol string) (string, error) {
	lib, ok, err := t.s.PrivateLibraryByModule(modulePath)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	for _, e := range lib.Exports {
		if e.Symbol == symbol {
			return mustJSON(e), nil
		}
	}
	return "", ErrNotFound
}

// SpecResource returns the JSON for a spec behind
// openapi://org/name/commit/<sha>/spec/<path>: the spec metadata plus its operations.
func (t *Tools) SpecResource(repo, path string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	specs, err := t.s.ListAPISpecs(ri.IndexID)
	if err != nil {
		return "", err
	}
	for _, sp := range specs {
		if sp.Path != path {
			continue
		}
		ops, err := t.s.FindOperations(ri.IndexID, "")
		if err != nil {
			return "", err
		}
		var specOps []store.HTTPOperation
		for _, op := range ops {
			if op.SpecPath == sp.Path {
				specOps = append(specOps, op)
			}
		}
		out := struct {
			Spec       store.APISpecRow      `json:"spec"`
			Operations []store.HTTPOperation `json:"operations"`
		}{Spec: sp, Operations: specOps}
		b, err := json.MarshalIndent(out, "", "  ")
		return string(b), err
	}
	return "", ErrNotFound
}
