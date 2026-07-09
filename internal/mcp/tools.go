package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/noviopenworks/candle/internal/registry"
	"github.com/noviopenworks/candle/internal/store"
)

// ErrNotFound is returned when a repo, symbol, or file cannot be resolved.
// Tool methods wrap it via notFound(reason) so the agent sees *why* a result
// was empty; errors.Is(err, ErrNotFound) still holds for callers.
var ErrNotFound = errors.New("not found")

// notFoundError wraps ErrNotFound with a specific reason.
type notFoundError struct{ reason string }

func (e *notFoundError) Error() string        { return "not found: " + e.reason }
func (e *notFoundError) Is(target error) bool { return target == ErrNotFound }

// notFound returns a not-found error carrying reason.
func notFound(reason string) error {
	return &notFoundError{reason: reason}
}

// repoNotFound is the common reason for an unresolved repo argument.
func repoNotFound(repo string) error {
	return notFound(fmt.Sprintf("repo %q not indexed", repo))
}

// Tools holds the pure tool implementations over the store.
type Tools struct {
	s              *store.Store
	reg            *registry.Registry
	sourceHydrator *sourceHydrator
}

// NewTools builds an unscoped tool set.
func NewTools(s *store.Store) *Tools {
	return NewToolsScoped(s, nil)
}

// NewToolsScoped builds a tool set limited to the given index ids (nil = all).
func NewToolsScoped(s *store.Store, allowed map[int64]bool) *Tools {
	return &Tools{s: s, reg: registry.NewScoped(s, allowed), sourceHydrator: newSourceHydrator()}
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
		return SymbolExplanation{}, repoNotFound(repo)
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
			return SymbolExplanation{}, notFound(fmt.Sprintf("symbol %q not found in %s", symbol, repo))
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
		return nil, repoNotFound(repo)
	}
	return t.s.NodesByFile(ri.IndexID, file)
}

// SourceFileContextResult is the hydrated get_file_context response: the file's
// symbols paired with the fetched source content for that file.
type SourceFileContextResult struct {
	File          string          `json:"file"`
	Symbols       []store.NodeRow `json:"symbols"`
	SourceContent SourceContent   `json:"source_content"`
}

// GetFileContextArgs are the arguments to GetFileContextWithSource. SourceContent
// is optional: nil or mode "off" preserves the existing metadata-only shape.
type GetFileContextArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	File          string                `json:"file" jsonschema:"source file path"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}

// GetFileContextWithSource is the source-aware get_file_context. It resolves
// symbols via the existing GetFileContext, then optionally hydrates the file's
// source content:
//
//   - nil opts or mode "off" -> returns []store.NodeRow (preserves default shape).
//   - present opts with empty mode -> treated as auto; hydrates because
//     get_file_context is an explicit file-context request.
//   - mode "auto"/"snippet"/"full" -> hydrates via ReadSourceContent's file
//     branch (defaults to full-file content).
//
// When hydration runs the return type is SourceFileContextResult.
func (t *Tools) GetFileContextWithSource(args GetFileContextArgs) (any, error) {
	symbols, err := t.GetFileContext(args.Repo, args.File)
	if err != nil {
		return nil, err
	}
	if args.SourceContent == nil {
		return symbols, nil
	}
	if strings.ToLower(strings.TrimSpace(args.SourceContent.Mode)) == sourceContentModeOff {
		return symbols, nil
	}
	source, err := t.ReadSourceContent(ReadSourceContentArgs{
		Repo:          args.Repo,
		File:          args.File,
		SourceContent: args.SourceContent,
	})
	if err != nil {
		return nil, err
	}
	return SourceFileContextResult{File: args.File, Symbols: symbols, SourceContent: source}, nil
}

// QueryRepo implements query_repo: structural node lookup by label.
func (t *Tools) QueryRepo(repo, name string) ([]store.NodeRow, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repoNotFound(repo)
	}
	return t.s.NodesByLabel(ri.IndexID, name)
}

// SourceNodeResult pairs a query_repo result node with its optional fetched
// source content. SourceContent.Status is always set; the other fields are
// populated only when hydration succeeded.
type SourceNodeResult struct {
	Node          store.NodeRow `json:"node"`
	SourceContent SourceContent `json:"source_content"`
}

// QueryRepoArgs are the arguments to QueryRepoWithSource. SourceContent is
// optional: nil or mode "off" preserves the existing metadata-only shape.
type QueryRepoArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	Name          string                `json:"name" jsonschema:"symbol label to look up"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}

// QueryRepoWithSource is the source-aware query_repo. It resolves nodes via the
// existing QueryRepo, then optionally hydrates source content:
//
//   - nil opts or mode "off" -> returns []store.NodeRow (preserves default shape).
//   - mode "auto"            -> hydrates when there is more than one node, or any
//     node lacks a parseable source location while still carrying fetchable
//     provenance; otherwise returns []store.NodeRow.
//   - mode "snippet"/"full"  -> hydrates up to req.maxCandidates nodes.
//
// When hydration runs every matched node is returned as a []SourceNodeResult;
// nodes past req.maxCandidates carry a "skipped" envelope rather than being
// dropped, so enabling source_content never hides structural matches. When
// hydration is off the return type stays []store.NodeRow so callers that opt
// out see no shape change.
func (t *Tools) QueryRepoWithSource(args QueryRepoArgs) (any, error) {
	nodes, err := t.QueryRepo(args.Repo, args.Name)
	if err != nil {
		return nil, err
	}
	req := sourceContentRequestFromOptions(args.SourceContent, sourceContentModeOff)
	if req.mode == sourceContentModeOff {
		return nodes, nil
	}
	ri, ok, err := t.reg.Resolve(args.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repoNotFound(args.Repo)
	}
	if req.mode == sourceContentModeAuto && !queryRepoShouldHydrateAuto(nodes, ri) {
		return nodes, nil
	}
	limit := req.maxCandidates
	out := make([]SourceNodeResult, 0, len(nodes))
	ctx := context.Background()
	for _, n := range nodes {
		res := SourceNodeResult{Node: n}
		if limit > 0 {
			res.SourceContent = t.sourceHydrator.hydrateNode(ctx, ri, n, req)
			limit--
		} else {
			res.SourceContent = SourceContent{Status: sourceContentStatusSkipped, SourceFile: n.SourceFile, Reason: "max_candidates hydration limit reached"}
		}
		out = append(out, res)
	}
	return out, nil
}

// queryRepoShouldHydrateAuto reports whether the auto trigger fires for a
// query_repo result set: when there is ambiguity (more than one node), or when
// any node with fetchable provenance lacks a parseable source location, the
// agent benefits from inline source content.
func queryRepoShouldHydrateAuto(nodes []store.NodeRow, ri registry.RepoInfo) bool {
	if len(nodes) > 1 {
		return true
	}
	for _, n := range nodes {
		if n.SourceLocation == "" && nodeHasFetchableSource(ri, n) {
			return true
		}
	}
	return false
}

// SourceSymbolExplanation wraps an explain_symbol result with optional source
// content for the resolved node.
type SourceSymbolExplanation struct {
	Explanation   SymbolExplanation `json:"explanation"`
	SourceContent SourceContent     `json:"source_content"`
}

// ExplainSymbolArgs are the arguments to ExplainSymbolWithSource. SourceContent
// is optional: nil or mode "off" preserves the existing metadata-only shape.
type ExplainSymbolArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	Symbol        string                `json:"symbol" jsonschema:"node id or label to explain"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}

// ExplainSymbolWithSource is the source-aware explain_symbol. It resolves the
// explanation via the existing ExplainSymbol, then optionally hydrates source
// content for the resolved node:
//
//   - nil opts or mode "off" -> returns SymbolExplanation (preserves default shape).
//   - mode "auto"            -> hydrates only when the resolved node has no
//     parseable source location AND fetchable provenance (otherwise the
//     metadata alone is enough, or hydration would only yield "skipped").
//   - mode "snippet"/"full"  -> hydrates the resolved node.
//
// When hydration runs the return type is SourceSymbolExplanation.
func (t *Tools) ExplainSymbolWithSource(args ExplainSymbolArgs) (any, error) {
	explanation, err := t.ExplainSymbol(args.Repo, args.Symbol)
	if err != nil {
		return nil, err
	}
	req := sourceContentRequestFromOptions(args.SourceContent, sourceContentModeOff)
	if req.mode == sourceContentModeOff {
		return explanation, nil
	}
	ri, ok, err := t.reg.Resolve(args.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repoNotFound(args.Repo)
	}
	if req.mode == sourceContentModeAuto {
		if _, hasLoc := parseSourceLocation(explanation.Node.SourceLocation); hasLoc {
			return explanation, nil
		}
		if !nodeHasFetchableSource(ri, explanation.Node) {
			return explanation, nil
		}
	}
	source := t.sourceHydrator.hydrateNode(context.Background(), ri, explanation.Node, req)
	return SourceSymbolExplanation{Explanation: explanation, SourceContent: source}, nil
}
