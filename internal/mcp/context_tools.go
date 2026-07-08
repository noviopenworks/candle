package mcp

import (
	"fmt"
	"strings"

	"github.com/noviopenworks/candle/internal/store"
)

// GetContextArgs is the pure-tool input for get_context. Repo is required.
// Topic is optional: empty means repo catalog, non-empty means focused lookup.
type GetContextArgs struct {
	Repo             string `json:"repo" jsonschema:"repo identity (org/name)"`
	Topic            string `json:"topic,omitempty" jsonschema:"optional symbol/API/schema/library topic"`
	Mode             string `json:"mode,omitempty" jsonschema:"optional: overview|code|api|proto|library|all"`
	Depth            int    `json:"depth,omitempty" jsonschema:"optional graph depth; v1 supports 1"`
	IncludeResources bool   `json:"include_resources,omitempty" jsonschema:"include resource URI hints"`
}

// RepoSummary is the typed repo identity exposed by get_context.
type RepoSummary struct {
	Repo      string `json:"repo"`
	Commit    string `json:"commit"`
	Branch    string `json:"branch"`
	NodeCount int    `json:"node_count"`
}

// ContextResult is the get_context response: repo catalog plus optional topic matches.
type ContextResult struct {
	Repo               RepoSummary         `json:"repo"`
	Topic              string              `json:"topic,omitempty"`
	Mode               string              `json:"mode"`
	Capabilities       ContextCapabilities `json:"capabilities"`
	Matches            ContextMatches      `json:"matches,omitempty"`
	SuggestedNextCalls []ToolHint          `json:"suggested_next_calls"`
	ResourceSchemes    []ResourceScheme    `json:"resource_schemes,omitempty"`
	Resources          []string            `json:"resources,omitempty"`
	Limitations        []string            `json:"limitations"`
}

// ContextCapabilities groups per-surface availability for a repo.
type ContextCapabilities struct {
	CodeGraph        CapabilitySummary `json:"code_graph"`
	OpenAPI          CapabilitySummary `json:"openapi"`
	Protobuf         CapabilitySummary `json:"protobuf"`
	PrivateLibraries CapabilitySummary `json:"private_libraries"`
}

// CapabilitySummary describes one surface and the precise tools that serve it.
type CapabilitySummary struct {
	Available bool     `json:"available"`
	Count     int      `json:"count"`
	Tools     []string `json:"tools"`
}

// ContextMatches groups topic-search results across surfaces.
type ContextMatches struct {
	CodeSymbols      []CodeContext    `json:"code_symbols,omitempty"`
	Endpoints        any              `json:"endpoints,omitempty"`
	Schemas          []SchemaInfo     `json:"schemas,omitempty"`
	RPCs             []RPCExplanation `json:"rpcs,omitempty"`
	PrivateLibraries any              `json:"private_libraries,omitempty"`
}

// CodeContext is a matched code node with its one-hop callers and callees.
type CodeContext struct {
	Node     store.NodeRow   `json:"node"`
	Callers  []store.EdgeRow `json:"callers"`
	Callees  []store.EdgeRow `json:"callees"`
	Resource string          `json:"resource,omitempty"`
}

// ToolHint suggests a precise follow-up tool call.
type ToolHint struct {
	Tool   string         `json:"tool"`
	Reason string         `json:"reason"`
	Args   map[string]any `json:"args,omitempty"`
}

// ResourceScheme documents a resource URI scheme.
type ResourceScheme struct {
	Scheme      string `json:"scheme"`
	Description string `json:"description"`
}

// GetContext implements get_context as a repo-scoped retrieval facade.
func (t *Tools) GetContext(args GetContextArgs) (ContextResult, error) {
	ri, ok, err := t.reg.Resolve(args.Repo)
	if err != nil {
		return ContextResult{}, err
	}
	if !ok {
		return ContextResult{}, repoNotFound(args.Repo)
	}
	mode := normalizeContextMode(args.Mode)
	nodeCount := t.nodeCount(ri.IndexID)
	out := ContextResult{
		Repo: RepoSummary{
			Repo:      ri.Repo,
			Commit:    ri.Commit,
			Branch:    ri.Branch,
			NodeCount: nodeCount,
		},
		Topic:              args.Topic,
		Mode:               mode,
		Limitations:        contextLimitations(),
		Capabilities:       t.contextCapabilities(ri.IndexID, nodeCount),
		SuggestedNextCalls: overviewHints(ri.Repo),
		ResourceSchemes:    contextResourceSchemes(),
	}
	// Overview mode returns the catalog only and suppresses topic matches.
	if strings.TrimSpace(args.Topic) != "" && mode != "overview" {
		matches, resources, hints := t.contextMatches(ri.IndexID, ri.Repo, ri.Commit, args.Topic, mode, args.IncludeResources)
		out.Matches = matches
		out.Resources = resources
		out.SuggestedNextCalls = append(hints, out.SuggestedNextCalls...)
	}
	return out, nil
}

// contextMatches searches the requested surfaces for topic and returns matches,
// resource URI hints, and follow-up tool hints.
func (t *Tools) contextMatches(indexID int64, repo, commit, topic, mode string, includeResources bool) (ContextMatches, []string, []ToolHint) {
	var matches ContextMatches
	var resources []string
	var hints []ToolHint
	include := func(surface string) bool { return mode == "all" || mode == surface }

	if include("code") {
		nodes, _ := t.s.NodesByLabel(indexID, topic)
		for _, n := range nodes {
			callers, _ := t.s.Callers(indexID, n.NodeID)
			callees, _ := t.s.Callees(indexID, n.NodeID)
			cc := CodeContext{Node: n, Callers: callers, Callees: callees}
			if includeResources {
				cc.Resource = graphNodeResource(repo, commit, n.NodeID)
				resources = append(resources, cc.Resource)
			}
			matches.CodeSymbols = append(matches.CodeSymbols, cc)
		}
		if len(matches.CodeSymbols) > 0 {
			hints = append(hints, ToolHint{Tool: "explain_symbol", Reason: "code symbol matched topic", Args: map[string]any{"repo": repo, "symbol": topic}})
		}
	}

	if include("api") {
		ops, _ := t.s.FindOperations(indexID, topic)
		if len(ops) > 0 {
			matches.Endpoints = ops
			hints = append(hints, ToolHint{Tool: "find_endpoint", Reason: "HTTP operation matched topic", Args: map[string]any{"repo": repo, "query": topic}})
			if includeResources {
				for _, op := range ops {
					if op.OperationID != "" {
						resources = append(resources, fmt.Sprintf("openapi://%s/commit/%s/operation/%s", repo, commitOrLatest(commit), op.OperationID))
					}
				}
			}
		}
	}

	if include("api") || include("proto") {
		schemas, _ := t.FindSchema(repo, topic)
		matches.Schemas = append(matches.Schemas, schemas...)
		if len(schemas) > 0 {
			hints = append(hints, ToolHint{Tool: "find_schema", Reason: "schema/message matched topic", Args: map[string]any{"repo": repo, "query": topic}})
		}
	}

	if include("proto") {
		rpcs, _ := t.s.FindRPCs(indexID, topic, "")
		for _, rpc := range rpcs {
			expl, err := t.ExplainRPC(repo, rpc.Service, rpc.Name)
			if err == nil {
				matches.RPCs = append(matches.RPCs, expl)
				if includeResources {
					resources = append(resources, fmt.Sprintf("proto://%s/commit/%s/rpc/%s/%s/%s", repo, commitOrLatest(commit), rpcPackage(rpc.FullName, rpc.Service, rpc.Name), rpc.Service, rpc.Name))
				}
			}
		}
		if len(matches.RPCs) > 0 {
			hints = append(hints, ToolHint{Tool: "find_rpc", Reason: "RPC matched topic", Args: map[string]any{"repo": repo, "query": topic}})
		}
	}

	if include("library") {
		libs, _ := t.FindPrivateLibrary(repo, topic)
		if len(libs) > 0 {
			matches.PrivateLibraries = libs
			hints = append(hints, ToolHint{Tool: "find_private_library", Reason: "private library matched topic", Args: map[string]any{"repo": repo, "query": topic}})
			if includeResources {
				for _, lib := range libs {
					resources = append(resources, "lib://"+lib.ModulePath)
				}
			}
		}
	}

	return matches, resources, hints
}

// normalizeContextMode keeps overview distinct from all: unknown/empty -> all.
func normalizeContextMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "overview", "all", "code", "api", "proto", "library":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "all"
	}
}

func (t *Tools) contextCapabilities(indexID int64, nodeCount int) ContextCapabilities {
	apis, _ := t.s.ListAPISpecs(indexID)
	protos, _ := t.s.ListProtoFiles(indexID)
	libs, _ := t.s.FindPrivateLibraries(indexID, "")
	deps, _ := t.s.FindPrivateDeps(indexID, "")
	return ContextCapabilities{
		CodeGraph:        CapabilitySummary{Available: true, Count: nodeCount, Tools: []string{"query_repo", "explain_symbol", "get_file_context"}},
		OpenAPI:          CapabilitySummary{Available: len(apis) > 0, Count: len(apis), Tools: []string{"list_apis", "find_endpoint", "explain_endpoint", "find_schema"}},
		Protobuf:         CapabilitySummary{Available: len(protos) > 0, Count: len(protos), Tools: []string{"list_apis", "find_rpc", "explain_rpc", "find_schema"}},
		PrivateLibraries: CapabilitySummary{Available: len(libs)+len(deps) > 0, Count: len(libs) + len(deps), Tools: []string{"find_private_library", "find_library_consumers"}},
	}
}

func (t *Tools) nodeCount(indexID int64) int {
	rows, err := t.s.DB.Query(`SELECT COUNT(*) FROM nodes WHERE index_id=?`, indexID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		_ = rows.Scan(&n)
	}
	return n
}

func overviewHints(repo string) []ToolHint {
	return []ToolHint{
		{Tool: "get_context", Reason: "retrieve focused context by topic", Args: map[string]any{"repo": repo, "topic": "<symbol endpoint rpc schema or library>"}},
		{Tool: "explain_symbol", Reason: "walk codegraph callers and callees", Args: map[string]any{"repo": repo, "symbol": "<symbol>"}},
		{Tool: "find_endpoint", Reason: "search HTTP endpoints", Args: map[string]any{"repo": repo, "query": "<path operationId or summary>"}},
		{Tool: "find_rpc", Reason: "search protobuf RPCs", Args: map[string]any{"repo": repo, "query": "<service or rpc>"}},
		{Tool: "find_private_library", Reason: "search internal Go modules", Args: map[string]any{"repo": repo, "query": "<module or purpose>"}},
	}
}

func contextResourceSchemes() []ResourceScheme {
	return []ResourceScheme{
		{Scheme: "repo://org/repo", Description: "repo snapshot summary"},
		{Scheme: "graph://org/repo/commit/<sha>/node/<nodeID>", Description: "code graph node"},
		{Scheme: "openapi://org/repo/commit/<sha>/operation/<operationId>", Description: "OpenAPI operation"},
		{Scheme: "openapi://org/repo/commit/<sha>/schema/<schemaName>", Description: "OpenAPI schema"},
		{Scheme: "proto://org/repo/commit/<sha>/rpc/<package>/<service>/<rpc>", Description: "protobuf RPC"},
		{Scheme: "proto://org/repo/commit/<sha>/message/<package>/<message>", Description: "protobuf message"},
		{Scheme: "lib://<module-path>", Description: "private library"},
	}
}

func contextLimitations() []string {
	return []string{
		"OpenAPI/HTTP handler linking is name-based: path→handler binding is coarse (route-registration presence), not router-precise.",
		"consumed_by is heuristic: it lists repos with a node labelled like the RPC (gRPC client calls are not indexed); providers are excluded.",
		"Cross-repo private library consumer aggregation is available via explain_private_library, not find_library_consumers.",
		"explain_symbol is one-hop; use call_path for multi-hop traversal (up to 5 hops).",
	}
}

func graphNodeResource(repo, commit, nodeID string) string {
	return fmt.Sprintf("graph://%s/commit/%s/node/%s", repo, commitOrLatest(commit), nodeID)
}

func commitOrLatest(commit string) string {
	if commit == "" {
		return "latest"
	}
	return commit
}

// rpcPackage derives the proto package from an RPC full name
// ("pkg.Service.Rpc" -> "pkg"); ProtoRPC has no Package field.
func rpcPackage(fullName, service, name string) string {
	pkg := strings.TrimSuffix(fullName, "."+service+"."+name)
	if pkg == fullName {
		// Fallback: strip the last two dotted segments.
		parts := strings.Split(fullName, ".")
		if len(parts) > 2 {
			return strings.Join(parts[:len(parts)-2], ".")
		}
	}
	return pkg
}
