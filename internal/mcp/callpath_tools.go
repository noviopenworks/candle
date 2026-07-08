package mcp

import (
	"fmt"
	"strings"

	"github.com/noviopenworks/candle/internal/store"
)

// CallPathHop is one node in a call_path traversal tree. Via is the edge from
// the parent node (nil for the root); Children are the next hops up to the
// requested depth.
type CallPathHop struct {
	Node     store.NodeRow  `json:"node"`
	Via      *store.EdgeRow `json:"via,omitempty"`
	Children []CallPathHop  `json:"children,omitempty"`
}

const (
	callPathDefaultDepth = 1
	callPathMaxDepth     = 5
)

// CallPath implements call_path: multi-hop call traversal from a symbol, up to
// depth hops (default 1, max 5), in direction callees | callers | both. Cycles
// are cut by a per-path visited set so a diamond does not loop. symbol may be a
// node id or a label (first label match wins), mirroring explain_symbol.
func (t *Tools) CallPath(repo, symbol string, depth int, direction string) (CallPathHop, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return CallPathHop{}, err
	}
	if !ok {
		return CallPathHop{}, repoNotFound(repo)
	}
	node, found, err := t.s.NodeByID(ri.IndexID, symbol)
	if err != nil {
		return CallPathHop{}, err
	}
	if !found {
		byLabel, err := t.s.NodesByLabel(ri.IndexID, symbol)
		if err != nil {
			return CallPathHop{}, err
		}
		if len(byLabel) == 0 {
			return CallPathHop{}, notFound(fmt.Sprintf("symbol %q not found in %s", symbol, repo))
		}
		node = byLabel[0]
	}
	if depth <= 0 {
		depth = callPathDefaultDepth
	}
	if depth > callPathMaxDepth {
		depth = callPathMaxDepth
	}
	return t.buildHop(ri.IndexID, node, depth, normalizeDirection(direction), map[string]bool{}), nil
}

// buildHop recursively builds the traversal tree, cutting cycles on the current
// path via visited (added before recursing, removed after — backtrack).
func (t *Tools) buildHop(indexID int64, node store.NodeRow, depth int, direction string, visited map[string]bool) CallPathHop {
	hop := CallPathHop{Node: node}
	if depth <= 0 {
		return hop
	}
	visited[node.NodeID] = true
	defer delete(visited, node.NodeID)
	for _, nb := range t.neighbors(indexID, node.NodeID, direction) {
		if visited[nb.childID] {
			continue
		}
		child, found, err := t.s.NodeByID(indexID, nb.childID)
		if err != nil || !found {
			continue
		}
		ch := t.buildHop(indexID, child, depth-1, direction, visited)
		edge := nb.edge
		ch.Via = &edge
		hop.Children = append(hop.Children, ch)
	}
	return hop
}

// neighbor is one adjacent node reachable via edge, with the child node id (the
// end of the edge that is not the current node).
type neighbor struct {
	edge    store.EdgeRow
	childID string
}

// neighbors returns the adjacent nodes for direction. For callees the child is
// the edge target; for callers the child is the edge source.
func (t *Tools) neighbors(indexID int64, nodeID, direction string) []neighbor {
	var out []neighbor
	if direction == "callees" || direction == "both" {
		if es, err := t.s.Callees(indexID, nodeID); err == nil {
			for _, e := range es {
				out = append(out, neighbor{edge: e, childID: e.Target})
			}
		}
	}
	if direction == "callers" || direction == "both" {
		if es, err := t.s.Callers(indexID, nodeID); err == nil {
			for _, e := range es {
				out = append(out, neighbor{edge: e, childID: e.Source})
			}
		}
	}
	return out
}

func normalizeDirection(d string) string {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "callers":
		return "callers"
	case "both":
		return "both"
	default:
		return "callees"
	}
}
