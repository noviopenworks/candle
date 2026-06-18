package mcp

import (
	"errors"

	"github.com/noviopenworks/candlegraph/internal/registry"
	"github.com/noviopenworks/candlegraph/internal/store"
)

// ErrNotFound is returned when a repo, symbol, or file cannot be resolved.
var ErrNotFound = errors.New("not found")

// Tools holds the pure tool implementations over the store.
type Tools struct {
	s   *store.Store
	reg *registry.Registry
}

// NewTools builds an unscoped tool set.
func NewTools(s *store.Store) *Tools {
	return NewToolsScoped(s, nil)
}

// NewToolsScoped builds a tool set limited to the given index ids (nil = all).
func NewToolsScoped(s *store.Store, allowed map[int64]bool) *Tools {
	return &Tools{s: s, reg: registry.NewScoped(s, allowed)}
}

// ListRepos implements the list_repos tool.
func (t *Tools) ListRepos() ([]registry.RepoInfo, error) {
	return t.reg.List()
}

// ResolveRepo implements resolve_repo: exact first, else fuzzy candidates.
func (t *Tools) ResolveRepo(query string) (best *registry.RepoInfo, candidates []registry.RepoInfo, err error) {
	if ri, ok, e := t.reg.Resolve(query); e != nil {
		return nil, nil, e
	} else if ok {
		return &ri, nil, nil
	}
	m, e := t.reg.Match(query)
	if e != nil {
		return nil, nil, e
	}
	if len(m) == 0 {
		return nil, nil, nil
	}
	return &m[0], m, nil
}

// SymbolExplanation is the explain_symbol result.
type SymbolExplanation struct {
	Node    store.NodeRow
	Callers []store.EdgeRow
	Callees []store.EdgeRow
}

// ExplainSymbol implements explain_symbol. symbol may be a node id or a label.
func (t *Tools) ExplainSymbol(repo, symbol string) (SymbolExplanation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return SymbolExplanation{}, err
	}
	if !ok {
		return SymbolExplanation{}, ErrNotFound
	}
	node, found, err := t.s.NodeByID(ri.IndexID, symbol)
	if err != nil {
		return SymbolExplanation{}, err
	}
	if !found {
		byLabel, err := t.s.NodesByLabel(ri.IndexID, symbol)
		if err != nil {
			return SymbolExplanation{}, err
		}
		if len(byLabel) == 0 {
			return SymbolExplanation{}, ErrNotFound
		}
		node = byLabel[0]
	}
	callers, err := t.s.Callers(ri.IndexID, node.NodeID)
	if err != nil {
		return SymbolExplanation{}, err
	}
	callees, err := t.s.Callees(ri.IndexID, node.NodeID)
	if err != nil {
		return SymbolExplanation{}, err
	}
	return SymbolExplanation{Node: node, Callers: callers, Callees: callees}, nil
}

// GetFileContext implements get_file_context.
func (t *Tools) GetFileContext(repo, file string) ([]store.NodeRow, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.NodesByFile(ri.IndexID, file)
}

// QueryRepo implements query_repo: structural node lookup by label.
func (t *Tools) QueryRepo(repo, name string) ([]store.NodeRow, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.NodesByLabel(ri.IndexID, name)
}
