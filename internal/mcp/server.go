package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vend-ai/intel-mcp/internal/store"
	"github.com/vend-ai/intel-mcp/internal/version"
)

// ToolNames lists every base tool the server advertises, in registration order.
// Exported so tests (and callers) can assert the advertised surface without
// reaching into the SDK.
var ToolNames = []string{
	"list_repos",
	"resolve_repo",
	"query_repo",
	"explain_symbol",
	"get_file_context",
	"list_apis",
	"find_endpoint",
	"explain_endpoint",
	"find_schema",
	"find_rpc",
	"explain_rpc",
}

// NewServer builds the MCP server backed by the store, registering all base
// tools and the repo:// / graph:// resource templates. All SDK types are
// confined to this file; the pure Tools/resource methods are untouched.
func NewServer(s *store.Store) *mcpsdk.Server {
	tools := NewTools(s)
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "intel-mcp",
		Version: version.String(),
	}, nil)

	registerListRepos(srv, tools)
	registerResolveRepo(srv, tools)
	registerQueryRepo(srv, tools)
	registerExplainSymbol(srv, tools)
	registerGetFileContext(srv, tools)
	registerListAPIs(srv, tools)
	registerFindEndpoint(srv, tools)
	registerExplainEndpoint(srv, tools)
	registerFindSchema(srv, tools)
	registerFindRPC(srv, tools)
	registerExplainRPC(srv, tools)
	registerResources(srv, tools)

	return srv
}

// Serve runs the MCP stdio server backed by the store until ctx is cancelled.
func Serve(ctx context.Context, s *store.Store) error {
	return NewServer(s).Run(ctx, &mcpsdk.StdioTransport{})
}

// textResult wraps a JSON/text payload in the SDK's tool-result content type.
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

// ---- tool registrations -------------------------------------------------

type emptyArgs struct{}

func registerListRepos(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_repos",
		Description: "List all indexed repository snapshots with node counts.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ emptyArgs) (*mcpsdk.CallToolResult, any, error) {
		repos, err := tools.ListRepos()
		if err != nil {
			return nil, nil, err
		}
		return textResult(mustJSON(repos)), nil, nil
	})
}

type resolveRepoArgs struct {
	Query string `json:"query" jsonschema:"repo identity (org/name) or fuzzy substring"`
}

func registerResolveRepo(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "resolve_repo",
		Description: "Resolve a repo query to a snapshot: exact match first, else fuzzy candidates.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args resolveRepoArgs) (*mcpsdk.CallToolResult, any, error) {
		best, candidates, err := tools.ResolveRepo(args.Query)
		if err != nil {
			return nil, nil, err
		}
		out := map[string]any{"best": best, "candidates": candidates}
		return textResult(mustJSON(out)), nil, nil
	})
}

type queryRepoArgs struct {
	Repo string `json:"repo" jsonschema:"repo identity (org/name)"`
	Name string `json:"name" jsonschema:"symbol label to look up"`
}

func registerQueryRepo(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "query_repo",
		Description: "Structural node lookup in a repo by symbol label.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args queryRepoArgs) (*mcpsdk.CallToolResult, any, error) {
		nodes, err := tools.QueryRepo(args.Repo, args.Name)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(nodes)), nil, nil
	})
}

type explainSymbolArgs struct {
	Repo   string `json:"repo" jsonschema:"repo identity (org/name)"`
	Symbol string `json:"symbol" jsonschema:"node id or label to explain"`
}

func registerExplainSymbol(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "explain_symbol",
		Description: "Explain a symbol: its node plus callers and callees.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args explainSymbolArgs) (*mcpsdk.CallToolResult, any, error) {
		out, err := tools.ExplainSymbol(args.Repo, args.Symbol)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(out)), nil, nil
	})
}

type getFileContextArgs struct {
	Repo string `json:"repo" jsonschema:"repo identity (org/name)"`
	File string `json:"file" jsonschema:"source file path"`
}

func registerGetFileContext(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_file_context",
		Description: "List the symbols defined in a given source file.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args getFileContextArgs) (*mcpsdk.CallToolResult, any, error) {
		syms, err := tools.GetFileContext(args.Repo, args.File)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(syms)), nil, nil
	})
}

type listAPIsArgs struct {
	Repo string `json:"repo" jsonschema:"repo identity (org/name)"`
}

func registerListAPIs(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_apis",
		Description: "List the API contracts indexed for a repo (kind-discriminated).",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args listAPIsArgs) (*mcpsdk.CallToolResult, any, error) {
		apis, err := tools.ListAPIs(args.Repo)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(apis)), nil, nil
	})
}

type findEndpointArgs struct {
	Repo  string `json:"repo" jsonschema:"repo identity (org/name)"`
	Query string `json:"query" jsonschema:"lexical match: NL / path / method / operationId"`
}

func registerFindEndpoint(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "find_endpoint",
		Description: "Find HTTP operations in a repo by lexical match on path/method/operationId/summary.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args findEndpointArgs) (*mcpsdk.CallToolResult, any, error) {
		ops, err := tools.FindEndpoint(args.Repo, args.Query)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(ops)), nil, nil
	})
}

type explainEndpointArgs struct {
	Repo   string `json:"repo" jsonschema:"repo identity (org/name)"`
	Method string `json:"method" jsonschema:"HTTP method (GET, POST, ...)"`
	Path   string `json:"path" jsonschema:"endpoint path template"`
}

func registerExplainEndpoint(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "explain_endpoint",
		Description: "Explain an HTTP endpoint's contract: operationId, summary, request/response schemas, security, tags.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args explainEndpointArgs) (*mcpsdk.CallToolResult, any, error) {
		op, err := tools.ExplainEndpoint(args.Repo, args.Method, args.Path)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(op)), nil, nil
	})
}

type findSchemaArgs struct {
	Repo  string `json:"repo" jsonschema:"repo identity (org/name)"`
	Query string `json:"query" jsonschema:"schema name substring"`
}

func registerFindSchema(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "find_schema",
		Description: "Find OpenAPI component schemas in a repo by name substring.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args findSchemaArgs) (*mcpsdk.CallToolResult, any, error) {
		schemas, err := tools.FindSchema(args.Repo, args.Query)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(schemas)), nil, nil
	})
}

type findRPCArgs struct {
	Repo       string `json:"repo" jsonschema:"repo identity (org/name)"`
	Query      string `json:"query" jsonschema:"lexical match: NL / service / rpc / full name"`
	StreamKind string `json:"stream_kind" jsonschema:"optional filter: unary|server_stream|client_stream|bidi"`
}

func registerFindRPC(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "find_rpc",
		Description: "Find gRPC RPCs in a repo by lexical match, optionally filtered by stream_kind.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args findRPCArgs) (*mcpsdk.CallToolResult, any, error) {
		rpcs, err := tools.FindRPC(args.Repo, args.Query, args.StreamKind)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(rpcs)), nil, nil
	})
}

type explainRPCArgs struct {
	Repo    string `json:"repo" jsonschema:"repo identity (org/name)"`
	Service string `json:"service" jsonschema:"gRPC service name"`
	RPC     string `json:"rpc" jsonschema:"RPC method name"`
}

func registerExplainRPC(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "explain_rpc",
		Description: "Explain a gRPC RPC: proto facts, messages, same-repo implementation, one-hop calls.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args explainRPCArgs) (*mcpsdk.CallToolResult, any, error) {
		out, err := tools.ExplainRPC(args.Repo, args.Service, args.RPC)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(out)), nil, nil
	})
}

// toolErr maps a not-found into a tool-level error result (IsError) rather than
// a protocol error, so unknown repos/symbols degrade gracefully.
func toolErr(err error) (*mcpsdk.CallToolResult, any, error) {
	if err == ErrNotFound {
		return &mcpsdk.CallToolResult{
			IsError: true,
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: ErrNotFound.Error()}},
		}, nil, nil
	}
	return nil, nil, err
}

// ---- resource registrations ---------------------------------------------

func registerResources(srv *mcpsdk.Server, tools *Tools) {
	srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "repo",
		Description: "Repo snapshot summary as JSON.",
		MIMEType:    "application/json",
		URITemplate: "repo://{org}/{name}",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		uri := req.Params.URI
		repo := strings.TrimPrefix(uri, "repo://")
		body, err := tools.RepoResource(repo)
		if err != nil {
			if err == ErrNotFound {
				return nil, mcpsdk.ResourceNotFoundError(uri)
			}
			return nil, err
		}
		return resourceText(uri, body), nil
	})

	srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "graph-node",
		Description: "A single graph node as JSON, commit-pinned.",
		MIMEType:    "application/json",
		URITemplate: "graph://{org}/{name}/commit/{sha}/node/{nodeID}",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		repo, nodeID, err := parseGraphNodeURI(req.Params.URI)
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		body, err := tools.GraphNodeResource(repo, nodeID)
		if err != nil {
			if err == ErrNotFound {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
			return nil, err
		}
		return resourceText(req.Params.URI, body), nil
	})

	srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "openapi",
		Description: "An OpenAPI artifact (operation/schema/spec) as JSON, commit-pinned.",
		MIMEType:    "application/json",
		URITemplate: "openapi://{org}/{name}/commit/{sha}/{kind}/{ref}",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		repo, kind, ref, err := parseOpenAPIURI(req.Params.URI)
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		var body string
		switch kind {
		case "operation":
			body, err = tools.OperationResource(repo, ref)
		case "schema":
			body, err = tools.SchemaResource(repo, ref)
		case "spec":
			body, err = tools.SpecResource(repo, ref)
		default:
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		if err != nil {
			if err == ErrNotFound {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
			return nil, err
		}
		return resourceText(req.Params.URI, body), nil
	})

	srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "proto",
		Description: "A protobuf artifact (file/service/rpc/message) as JSON, commit-pinned.",
		MIMEType:    "application/json",
		URITemplate: "proto://{org}/{name}/commit/{sha}/{kind}/{ref}",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		repo, kind, ref, err := parseProtoURI(req.Params.URI)
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		var body string
		switch kind {
		case "file":
			body, err = tools.ProtoFileResource(repo, ref)
		case "service":
			if p := strings.SplitN(ref, "/", 2); len(p) == 2 {
				body, err = tools.ProtoServiceResource(repo, p[0], p[1])
			} else {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
		case "rpc":
			if p := strings.SplitN(ref, "/", 3); len(p) == 3 {
				body, err = tools.ProtoRPCResource(repo, p[0], p[1], p[2])
			} else {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
		case "message":
			if p := strings.SplitN(ref, "/", 2); len(p) == 2 {
				body, err = tools.ProtoMessageResource(repo, p[0], p[1])
			} else {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
		default:
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		if err != nil {
			if err == ErrNotFound {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
			return nil, err
		}
		return resourceText(req.Params.URI, body), nil
	})
}

func resourceText(uri, body string) *mcpsdk.ReadResourceResult {
	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     body,
		}},
	}
}

// parseGraphNodeURI parses graph://org/name/commit/<sha>/node/<node_id> into
// the repo identity (org/name) and node id. The commit segment is accepted but
// resolution is by snapshot (commit-pinning is reflected in the URI/resource).
func parseGraphNodeURI(uri string) (repo, nodeID string, err error) {
	rest := strings.TrimPrefix(uri, "graph://")
	parts := strings.Split(rest, "/")
	// org / name / commit / <sha> / node / <node_id>
	if len(parts) != 6 || parts[2] != "commit" || parts[4] != "node" {
		return "", "", fmt.Errorf("malformed graph node uri %q", uri)
	}
	return parts[0] + "/" + parts[1], parts[5], nil
}

// parseOpenAPIURI parses openapi://org/name/commit/<sha>/<kind>/<ref> into the
// repo identity (org/name), the kind segment (operation|schema|spec), and the
// trailing ref. The ref may itself contain slashes (e.g. spec paths), so all
// segments after the kind are rejoined.
func parseOpenAPIURI(uri string) (repo, kind, ref string, err error) {
	rest := strings.TrimPrefix(uri, "openapi://")
	parts := strings.Split(rest, "/")
	// org / name / commit / <sha> / <kind> / <ref...>
	if len(parts) < 6 || parts[2] != "commit" {
		return "", "", "", fmt.Errorf("malformed openapi uri %q", uri)
	}
	return parts[0] + "/" + parts[1], parts[4], strings.Join(parts[5:], "/"), nil
}

// parseProtoURI parses proto://org/name/commit/<sha>/<kind>/<ref...>.
func parseProtoURI(uri string) (repo, kind, ref string, err error) {
	rest := strings.TrimPrefix(uri, "proto://")
	parts := strings.Split(rest, "/")
	if len(parts) < 6 || parts[2] != "commit" {
		return "", "", "", fmt.Errorf("malformed proto uri %q", uri)
	}
	return parts[0] + "/" + parts[1], parts[4], strings.Join(parts[5:], "/"), nil
}
