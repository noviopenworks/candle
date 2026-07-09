---
change: hydrate-github-source-content
design-doc: docs/superpowers/specs/2026-07-09-hydrate-github-source-content-design.md
base-ref: 7bae63b570201af99393f47cbb65e4d353ec452c
---

# Hydrate GitHub Source Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement opt-in GitHub source content hydration for Graphify-backed MCP tools and add a direct `read_source_content` tool.

**Reference Design:** `docs/superpowers/specs/2026-07-09-hydrate-github-source-content-design.md`.

**Architecture:** Keep default MCP responses metadata-only. Extend query-facing store rows to expose existing provenance, add a shared SDK-free source hydrator in `internal/mcp`, then branch tool responses only when `source_content` is present. Register one new direct source-read MCP tool in `internal/mcp/server.go`.

**Tech Stack:** Go, SQLite, `net/http`, `httptest`, `github.com/modelcontextprotocol/go-sdk/mcp`, existing candle store/registry/MCP packages.

## Global Constraints

- Preserve metadata-only defaults for `get_context`, `query_repo`, `explain_symbol`, and `get_file_context` when `source_content` is omitted or set to mode `off`.
- Do not persist fetched source content in SQLite.
- Do not add GitHub auth, token configuration, or a non-GitHub provider abstraction.
- Treat source fetch failures as structured `SourceContent` statuses, not whole-tool errors, when graph metadata is available.
- Use deterministic hydration limits: 64 KiB default maximum content, 20-line default snippet radius, 5 default hydrated candidates, and 5 second default HTTP timeout.
- Use tests with fake clients or `httptest.Server`; no test should depend on external GitHub availability.
- Do not add a new MCP resource URI in this change.

---

## File Map

- Modify `internal/store/query.go`: add provenance fields to `store.NodeRow`, `scanNodes`, and `nodeCols`.
- Modify `internal/store/query_test.go`: prove provenance fields are returned by node query helpers.
- Create `internal/mcp/source_content.go`: define source hydration options, result envelopes, hydrator defaults, URL normalization, fetching, text detection, snippet extraction, and direct read helpers.
- Create `internal/mcp/source_content_test.go`: unit tests for URL normalization, fetch limits, text detection, snippets, truncation, missing provenance, unsupported URLs, and direct reads.
- Modify `internal/mcp/tools.go`: add hydratable pure methods for `query_repo`, `explain_symbol`, and `get_file_context`; keep existing metadata-only method signatures intact for current callers.
- Modify `internal/mcp/context_tools.go`: add `source_content` to `GetContextArgs` and optional `source_content` to code-symbol matches.
- Modify `internal/mcp/tools_test.go`: add tests proving default metadata-only output and opt-in wrapper output for `query_repo`, `explain_symbol`, `get_file_context`, and direct reads.
- Modify `internal/mcp/context_tools_test.go`: add tests for default `get_context` output and opt-in code-symbol hydration.
- Modify `internal/mcp/server.go`: add tool args fields, register `read_source_content`, and marshal default or hydrated response shapes correctly.
- Modify `internal/mcp/e2e_surface_test.go`: update advertised tool-surface expectations and add a direct source-read smoke assertion with deterministic fixture provenance.
- Modify `internal/mcp/e2e_test.go`: update comments that describe the advertised tool count if needed.
- Modify `docs/tools.md`: document `source_content`, hydrated response envelopes, limits, `source_url` expectations, and `read_source_content`.
- Modify `openspec/changes/hydrate-github-source-content/tasks.md`: mark tasks complete after implementation and verification.

## Interfaces To Add

Use these names consistently across tasks.

```go
type SourceContentOptions struct {
	Mode          string `json:"mode,omitempty"`
	MaxBytes      int    `json:"max_bytes,omitempty"`
	LineRadius    int    `json:"line_radius,omitempty"`
	MaxCandidates int    `json:"max_candidates,omitempty"`
}

type SourceContent struct {
	Status     string `json:"status"`
	Mode       string `json:"mode,omitempty"`
	SourceFile string `json:"source_file,omitempty"`
	SourceURL  string `json:"source_url,omitempty"`
	StartLine  int    `json:"start_line,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	Content    string `json:"content,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type SourceNodeResult struct {
	Node          store.NodeRow `json:"node"`
	SourceContent SourceContent `json:"source_content"`
}

type SourceSymbolExplanation struct {
	Explanation   SymbolExplanation `json:"explanation"`
	SourceContent SourceContent     `json:"source_content"`
}

type SourceFileContextResult struct {
	File          string          `json:"file"`
	Symbols       []store.NodeRow `json:"symbols"`
	SourceContent SourceContent   `json:"source_content"`
}

type ReadSourceContentArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	NodeID        string                `json:"node_id,omitempty" jsonschema:"graph node id to read"`
	File          string                `json:"file,omitempty" jsonschema:"source file path to read"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}
```

Hydratable pure methods should be additive so existing tests and callers of current pure methods still compile.

```go
func (t *Tools) QueryRepoWithSource(args QueryRepoArgs) (any, error)
func (t *Tools) ExplainSymbolWithSource(args ExplainSymbolArgs) (any, error)
func (t *Tools) GetFileContextWithSource(args GetFileContextArgs) (any, error)
func (t *Tools) ReadSourceContent(args ReadSourceContentArgs) (SourceContent, error)
```

Define `QueryRepoArgs`, `ExplainSymbolArgs`, and `GetFileContextArgs` in `internal/mcp/tools.go` or `internal/mcp/source_content.go` and reuse them from `server.go`.

## Task 1: Expose Stored Node Provenance

**OpenSpec coverage:** 1.1.

**Files:**
- Modify `internal/store/query.go`
- Modify `internal/store/query_test.go`

**Interfaces:**
- Consumes: existing `nodes.source_url`, `nodes.captured_at`, `nodes.author`, `nodes.contributor` columns from `internal/store/schema.go`.
- Produces: `store.NodeRow.SourceURL`, `store.NodeRow.CapturedAt`, `store.NodeRow.Author`, `store.NodeRow.Contributor` for all node query helpers.

- [x] **Step 1: Write the failing store provenance test**

Add this test to `internal/store/query_test.go`.

```go
func TestNodeRowsIncludeStoredProvenance(t *testing.T) {
	s, idA, _ := seed(t)
	defer s.Close()
	mustExec(t, s, `UPDATE nodes SET source_url=?, captured_at=?, author=?, contributor=? WHERE index_id=? AND node_id=?`,
		"https://github.com/org/svc-a/blob/a1/h.go", "2026-07-09T12:00:00Z", "Ada", "Grace", idA, "n1")

	ns, err := s.NodesByLabel(idA, "ReserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 {
		t.Fatalf("expected one node, got %+v", ns)
	}
	got := ns[0]
	if got.SourceURL != "https://github.com/org/svc-a/blob/a1/h.go" || got.CapturedAt != "2026-07-09T12:00:00Z" || got.Author != "Ada" || got.Contributor != "Grace" {
		t.Fatalf("provenance not scanned: %+v", got)
	}

	byID, ok, err := s.NodeByID(idA, "n1")
	if err != nil || !ok {
		t.Fatalf("NodeByID: ok=%v err=%v", ok, err)
	}
	if byID.SourceURL != got.SourceURL {
		t.Fatalf("NodeByID SourceURL=%q, want %q", byID.SourceURL, got.SourceURL)
	}

	byFile, err := s.NodesByFile(idA, "h.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(byFile) != 1 || byFile[0].Contributor != "Grace" {
		t.Fatalf("NodesByFile provenance mismatch: %+v", byFile)
	}
}
```

- [x] **Step 2: Run the focused failing test**

Run: `go test ./internal/store -run TestNodeRowsIncludeStoredProvenance -count=1`

Expected: FAIL with compile errors that `store.NodeRow` has no `SourceURL`, `CapturedAt`, `Author`, or `Contributor` fields.

- [x] **Step 3: Extend `NodeRow`, `scanNodes`, and `nodeCols`**

In `internal/store/query.go`, keep existing field names unchanged and append the provenance fields without JSON tags so existing `NodeRow` JSON casing remains stable.

```go
type NodeRow struct {
	IndexID        int64
	NodeID         string
	Label          string
	FileType       string
	SourceFile     string
	SourceLocation string
	SourceURL      string
	CapturedAt     string
	Author         string
	Contributor    string
}
```

Update the scan call and column list.

```go
if err := rows.Scan(&n.IndexID, &n.NodeID, &n.Label, &n.FileType, &n.SourceFile, &n.SourceLocation, &n.SourceURL, &n.CapturedAt, &n.Author, &n.Contributor); err != nil {
	return nil, err
}
```

```go
const nodeCols = `index_id, node_id, COALESCE(label,''), COALESCE(file_type,''), COALESCE(source_file,''), COALESCE(source_location,''), COALESCE(source_url,''), COALESCE(captured_at,''), COALESCE(author,''), COALESCE(contributor,'')`
```

- [x] **Step 4: Verify store provenance passes**

Run: `go test ./internal/store -run 'TestNodeRowsIncludeStoredProvenance|TestNodesByLabel|TestNeighbors|TestNodesByLabelAcrossIndexes' -count=1`

Expected: PASS.

## Task 2: Add Source Hydration Core

**OpenSpec coverage:** 1.2, 1.3, 1.4, 3.1, 3.3.

**Files:**
- Create `internal/mcp/source_content.go`
- Create `internal/mcp/source_content_test.go`

**Interfaces:**
- Consumes: `store.NodeRow` provenance fields from Task 1 and `registry.RepoInfo`.
- Produces: source modes, `SourceContentOptions`, `SourceContent`, wrapper result structs, URL normalization, bounded fetch, text detection, snippet extraction, and status-only failures.

- [x] **Step 1: Write URL normalization tests**

Create `internal/mcp/source_content_test.go` with these tests first.

```go
package mcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/noviopenworks/candle/internal/registry"
	"github.com/noviopenworks/candle/internal/store"
)

func TestNormalizeGitHubSourceURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "raw passthrough",
			in:   "https://raw.githubusercontent.com/org/repo/abc/internal/server.go",
			want: "https://raw.githubusercontent.com/org/repo/abc/internal/server.go",
		},
		{
			name: "github blob converts to raw",
			in:   "https://github.com/org/repo/blob/abc/internal/server.go",
			want: "https://raw.githubusercontent.com/org/repo/abc/internal/server.go",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeGitHubSourceURL(tc.in)
			if !ok {
				t.Fatalf("expected %q to normalize", tc.in)
			}
			if got != tc.want {
				t.Fatalf("normalize=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeGitHubSourceURLRejectsUnsupportedHost(t *testing.T) {
	if got, ok := normalizeGitHubSourceURL("https://git.acme.local/org/repo/blob/main/a.go"); ok || got != "" {
		t.Fatalf("unsupported host normalized to %q", got)
	}
}
```

- [x] **Step 2: Write fetch, snippet, and status tests**

Append these tests to `internal/mcp/source_content_test.go`. Use a fake `RoundTripper` so hydration can use GitHub URLs without reaching the network.

```go
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testHydrator(body string, contentType string) *sourceHydrator {
	return &sourceHydrator{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{contentType}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}}
}

func TestHydrateSourceContentSnippet(t *testing.T) {
	h := testHydrator("line1\nline2\nline3\nline4\nline5\n", "text/plain; charset=utf-8")
	got := h.hydrateNode(context.Background(), registry.RepoInfo{Repo: "org/repo", Commit: "abc", Branch: "main"}, store.NodeRow{
		NodeID: "n1", SourceFile: "internal/server.go", SourceLocation: "L3", SourceURL: "https://github.com/org/repo/blob/abc/internal/server.go",
	}, sourceContentRequest{mode: sourceContentModeSnippet, maxBytes: 1024, lineRadius: 1, maxCandidates: 5})
	if got.Status != sourceContentStatusFetched || got.Mode != sourceContentModeSnippet {
		t.Fatalf("status/mode mismatch: %+v", got)
	}
	if got.StartLine != 2 || got.EndLine != 4 {
		t.Fatalf("line range=%d-%d, want 2-4", got.StartLine, got.EndLine)
	}
	if got.Content != "line2\nline3\nline4" {
		t.Fatalf("content=%q", got.Content)
	}
}

func TestHydrateSourceContentFullTruncated(t *testing.T) {
	h := testHydrator("abcdef", "text/plain")
	got := h.hydrateNode(context.Background(), registry.RepoInfo{Repo: "org/repo", Commit: "abc"}, store.NodeRow{
		SourceFile: "a.go", SourceURL: "https://raw.githubusercontent.com/org/repo/abc/a.go",
	}, sourceContentRequest{mode: sourceContentModeFull, maxBytes: 3, lineRadius: 20, maxCandidates: 5})
	if got.Status != sourceContentStatusFetched || !got.Truncated || got.Content != "abc" {
		t.Fatalf("truncated fetch mismatch: %+v", got)
	}
}

func TestHydrateSourceContentRejectsNonText(t *testing.T) {
	h := testHydrator("\x00\x01\x02", "application/octet-stream")
	got := h.hydrateNode(context.Background(), registry.RepoInfo{Repo: "org/repo", Commit: "abc"}, store.NodeRow{
		SourceFile: "asset.bin", SourceURL: "https://raw.githubusercontent.com/org/repo/abc/asset.bin",
	}, sourceContentRequest{mode: sourceContentModeFull, maxBytes: 1024, lineRadius: 20, maxCandidates: 5})
	if got.Status != sourceContentStatusUnsupported || got.Content != "" || !strings.Contains(got.Reason, "non-text") {
		t.Fatalf("non-text status mismatch: %+v", got)
	}
}

func TestHydrateSourceContentMissingProvenance(t *testing.T) {
	h := testHydrator("package main\n", "text/plain")
	got := h.hydrateNode(context.Background(), registry.RepoInfo{Repo: "", Commit: ""}, store.NodeRow{}, sourceContentRequest{mode: sourceContentModeFull, maxBytes: 1024, lineRadius: 20, maxCandidates: 5})
	if got.Status != sourceContentStatusSkipped || !strings.Contains(got.Reason, "missing source provenance") {
		t.Fatalf("missing provenance status mismatch: %+v", got)
	}
}

func TestHydrateSourceContentFetchErrorIsStatus(t *testing.T) {
	h := &sourceHydrator{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})}}
	got := h.hydrateNode(context.Background(), registry.RepoInfo{Repo: "org/repo", Commit: "abc"}, store.NodeRow{
		SourceFile: "a.go", SourceURL: "https://raw.githubusercontent.com/org/repo/abc/a.go",
	}, sourceContentRequest{mode: sourceContentModeFull, maxBytes: 1024, lineRadius: 20, maxCandidates: 5})
	if got.Status != sourceContentStatusError || !strings.Contains(got.Reason, "network down") {
		t.Fatalf("fetch error status mismatch: %+v", got)
	}
}
```

- [x] **Step 3: Run the focused failing hydrator tests**

Run: `go test ./internal/mcp -run 'TestNormalizeGitHubSourceURL|TestHydrateSourceContent' -count=1`

Expected: FAIL with compile errors for missing source hydration types and functions.

- [x] **Step 4: Add the source hydration implementation**

Create `internal/mcp/source_content.go` with package `mcp`. Include these constants and unexported request type.

```go
const (
	sourceContentModeOff     = "off"
	sourceContentModeAuto    = "auto"
	sourceContentModeSnippet = "snippet"
	sourceContentModeFull    = "full"

	sourceContentStatusFetched     = "fetched"
	sourceContentStatusSkipped     = "skipped"
	sourceContentStatusUnsupported = "unsupported"
	sourceContentStatusError       = "error"

	defaultSourceContentMaxBytes      = 64 * 1024
	defaultSourceContentLineRadius    = 20
	defaultSourceContentMaxCandidates = 5
	defaultSourceContentTimeout       = 5 * time.Second
)

type sourceContentRequest struct {
	mode          string
	maxBytes      int
	lineRadius    int
	maxCandidates int
}
```

Use this normalization behavior.

```go
func sourceContentRequestFromOptions(opts *SourceContentOptions, defaultMode string) sourceContentRequest {
	req := sourceContentRequest{
		mode:          defaultMode,
		maxBytes:      defaultSourceContentMaxBytes,
		lineRadius:    defaultSourceContentLineRadius,
		maxCandidates: defaultSourceContentMaxCandidates,
	}
	if opts == nil {
		return req
	}
	if strings.TrimSpace(opts.Mode) != "" {
		req.mode = strings.ToLower(strings.TrimSpace(opts.Mode))
	}
	if req.mode == "" {
		req.mode = defaultMode
	}
	if opts.MaxBytes > 0 {
		req.maxBytes = opts.MaxBytes
	}
	if opts.LineRadius >= 0 {
		req.lineRadius = opts.LineRadius
	}
	if opts.MaxCandidates > 0 {
		req.maxCandidates = opts.MaxCandidates
	}
	return req
}
```

Implement these functions in the same file.

```go
func normalizeGitHubSourceURL(raw string) (string, bool)
func isSupportedSourceURL(raw string) bool
func rawURLForNode(ri registry.RepoInfo, n store.NodeRow) (string, bool)
func parseSourceLine(sourceLocation string) (int, bool)
func snippetWindow(content string, centerLine, radius int) (snippet string, startLine int, endLine int)
func looksText(contentType string, b []byte) bool
func fetchText(ctx context.Context, client *http.Client, rawURL string, maxBytes int) (content string, truncated bool, contentType string, err error)
```

Implementation rules:

- `normalizeGitHubSourceURL` accepts raw URLs like `https://raw.githubusercontent.com/org/repo/abc/internal/server.go` as-is.
- `normalizeGitHubSourceURL` converts blob URLs like `https://github.com/org/repo/blob/abc/internal/server.go` to `https://raw.githubusercontent.com/org/repo/abc/internal/server.go`.
- `normalizeGitHubSourceURL` rejects every other host or malformed GitHub blob path by returning `"", false`.
- `rawURLForNode` prefers `n.SourceURL` when supported; otherwise builds a URL like `https://raw.githubusercontent.com/org/repo/abc/internal/server.go` when `ri.Repo`, a commit or branch ref, and `n.SourceFile` are present. Use commit before branch.
- `parseSourceLine` returns the first positive integer in strings like `L10`, `L10-L12`, `line 10`, or `10:4`.
- `snippetWindow` returns whole lines only, trims one trailing newline from the returned snippet, and clamps start/end to the file bounds.
- `fetchText` reads at most `maxBytes + 1` bytes, marks truncation when more than `maxBytes` bytes are available, closes the body, and treats non-2xx HTTP statuses as errors containing text such as `HTTP 404`.
- `looksText` accepts content types starting with `text/` and common textual application types such as `application/json`, `application/xml`, `application/x-yaml`, and `application/yaml`; it rejects bytes containing `0x00`.

Implement the hydrator around an injectable client.

```go
type sourceHydrator struct {
	client *http.Client
}

func newSourceHydrator() *sourceHydrator {
	return &sourceHydrator{client: &http.Client{Timeout: defaultSourceContentTimeout}}
}

func (h *sourceHydrator) hydrateNode(ctx context.Context, ri registry.RepoInfo, n store.NodeRow, req sourceContentRequest) SourceContent
```

`hydrateNode` status behavior:

- mode `off` returns `SourceContent{Status: "skipped", Reason: "source_content mode is off"}`.
- missing source URL/fallback provenance returns `Status: "skipped"` with reason `missing source provenance`.
- unsupported URL returns `Status: "unsupported"` with reason `unsupported source URL`.
- fetch errors return `Status: "error"` with the error text in `Reason`.
- non-text content returns `Status: "unsupported"`, omits `Content`, and includes reason `non-text source content`.
- snippet mode with no parseable source line returns full capped content with mode `full` only for direct reads; for enrichment calls return `Status: "skipped"` with reason `missing source location for snippet`.
- fetched content sets `Status: "fetched"`, `Mode`, `SourceFile`, `SourceURL`, optional line range, `Truncated`, and `Content`.

- [x] **Step 5: Verify hydrator unit tests pass**

Run: `go test ./internal/mcp -run 'TestNormalizeGitHubSourceURL|TestHydrateSourceContent' -count=1`

Expected: PASS.

## Task 3: Add Direct Source-Read Pure Tool

**OpenSpec coverage:** 2.5, 3.2 direct reads, 3.3 failure paths.

**Files:**
- Modify `internal/mcp/source_content.go`
- Modify `internal/mcp/source_content_test.go`
- Modify `internal/mcp/tools.go` only if `ReadSourceContent` is placed there instead of `source_content.go`

**Interfaces:**
- Consumes: `Tools.reg.Resolve`, `Store.NodeByID`, `Store.NodesByFile`, `sourceHydrator.hydrateNode`.
- Produces: `func (t *Tools) ReadSourceContent(args ReadSourceContentArgs) (SourceContent, error)`.

- [x] **Step 1: Write direct read tests by node and by file**

Append these tests to `internal/mcp/source_content_test.go`.

```go
func seedSourceContentTools(t *testing.T) (*Tools, int64) {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	id, err := s.UpsertIndex("org", "repo", "abc", "main", "/g")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location,source_url) VALUES(?,?,?,?,?,?,?)`,
		id, "n1", "ReserveProduct", "code", "internal/server.go", "L2", "https://raw.githubusercontent.com/org/repo/abc/internal/server.go"); err != nil {
		t.Fatal(err)
	}
	return NewTools(s), id
}

func TestReadSourceContentByNode(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("line1\nline2\nline3\n", "text/plain")

	got, err := tools.ReadSourceContent(ReadSourceContentArgs{Repo: "org/repo", NodeID: "n1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != sourceContentStatusFetched || got.Mode != sourceContentModeSnippet || !strings.Contains(got.Content, "line2") {
		t.Fatalf("direct node read mismatch: %+v", got)
	}
}

func TestReadSourceContentByFile(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("package server\nfunc ReserveProduct() {}\n", "text/plain")

	got, err := tools.ReadSourceContent(ReadSourceContentArgs{Repo: "org/repo", File: "internal/server.go"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != sourceContentStatusFetched || got.Mode != sourceContentModeFull || !strings.Contains(got.Content, "ReserveProduct") {
		t.Fatalf("direct file read mismatch: %+v", got)
	}
}

func TestReadSourceContentRequiresNodeOrFile(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	_, err := tools.ReadSourceContent(ReadSourceContentArgs{Repo: "org/repo"})
	if err == nil || !strings.Contains(err.Error(), "node_id or file") {
		t.Fatalf("expected node_id/file validation error, got %v", err)
	}
}

func TestReadSourceContentFileWithoutResolvableNodeReturnsSkippedStatus(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	got, err := tools.ReadSourceContent(ReadSourceContentArgs{Repo: "org/repo", File: "missing.go"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != sourceContentStatusSkipped || !strings.Contains(got.Reason, "no indexed nodes for file") {
		t.Fatalf("missing file status mismatch: %+v", got)
	}
}
```

- [x] **Step 2: Run the focused failing direct-read tests**

Run: `go test ./internal/mcp -run 'TestReadSourceContent' -count=1`

Expected: FAIL with missing `Tools.sourceHydrator` or `ReadSourceContent` compile errors.

- [x] **Step 3: Add the hydrator dependency to `Tools`**

Modify `internal/mcp/tools.go`.

```go
type Tools struct {
	s              *store.Store
	reg            *registry.Registry
	sourceHydrator *sourceHydrator
}
```

Update `NewToolsScoped`.

```go
func NewToolsScoped(s *store.Store, allowed map[int64]bool) *Tools {
	return &Tools{s: s, reg: registry.NewScoped(s, allowed), sourceHydrator: newSourceHydrator()}
}
```

- [x] **Step 4: Implement `ReadSourceContent`**

Implement this behavior:

```go
func (t *Tools) ReadSourceContent(args ReadSourceContentArgs) (SourceContent, error) {
	ri, ok, err := t.reg.Resolve(args.Repo)
	if err != nil {
		return SourceContent{}, err
	}
	if !ok {
		return SourceContent{}, repoNotFound(args.Repo)
	}
	if strings.TrimSpace(args.NodeID) == "" && strings.TrimSpace(args.File) == "" {
		return SourceContent{}, fmt.Errorf("read_source_content requires node_id or file")
	}
	if strings.TrimSpace(args.NodeID) != "" {
		node, found, err := t.s.NodeByID(ri.IndexID, args.NodeID)
		if err != nil {
			return SourceContent{}, err
		}
		if !found {
			return SourceContent{}, notFound(fmt.Sprintf("node %q not found in %s", args.NodeID, args.Repo))
		}
		req := sourceContentRequestFromOptions(args.SourceContent, defaultDirectMode(node))
		return t.sourceHydrator.hydrateNode(context.Background(), ri, node, req), nil
	}

	nodes, err := t.s.NodesByFile(ri.IndexID, args.File)
	if err != nil {
		return SourceContent{}, err
	}
	if len(nodes) == 0 {
		return SourceContent{Status: sourceContentStatusSkipped, SourceFile: args.File, Reason: "no indexed nodes for file"}, nil
	}
	req := sourceContentRequestFromOptions(args.SourceContent, sourceContentModeFull)
	for _, node := range nodes {
		if nodeHasFetchableSource(ri, node) {
			return t.sourceHydrator.hydrateNode(context.Background(), ri, node, req), nil
		}
	}
	got := t.sourceHydrator.hydrateNode(context.Background(), ri, nodes[0], req)
	if got.SourceFile == "" {
		got.SourceFile = args.File
	}
	return got, nil
}
```

Use this helper for the file-read node selection loop.

```go
func nodeHasFetchableSource(ri registry.RepoInfo, node store.NodeRow) bool {
	_, ok := rawURLForNode(ri, node)
	return ok
}
```

This keeps selection deterministic: prefer the first node that can resolve a supported GitHub raw/blob URL or fallback raw URL.

Default direct modes:

```go
func defaultDirectMode(node store.NodeRow) string {
	if _, ok := parseSourceLine(node.SourceLocation); ok {
		return sourceContentModeSnippet
	}
	return sourceContentModeFull
}
```

- [x] **Step 5: Verify direct-read tests pass**

Run: `go test ./internal/mcp -run 'TestReadSourceContent|TestHydrateSourceContent|TestNormalizeGitHubSourceURL' -count=1`

Expected: PASS.

## Task 4: Integrate `query_repo` and `explain_symbol`

**OpenSpec coverage:** 2.1, 2.2, 3.2 default and opt-in behavior.

**Files:**
- Modify `internal/mcp/tools.go`
- Modify `internal/mcp/tools_test.go`

**Interfaces:**
- Consumes: `sourceContentRequestFromOptions`, `sourceHydrator.hydrateNode`, `SourceNodeResult`, `SourceSymbolExplanation`.
- Produces: `QueryRepoWithSource` and `ExplainSymbolWithSource`, while existing `QueryRepo(repo, name)` and `ExplainSymbol(repo, symbol)` remain metadata-only.

- [x] **Step 1: Write `query_repo` default and hydrated tests**

Append to `internal/mcp/tools_test.go`.

```go
func TestQueryRepoWithSourcePreservesDefaultShape(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	out, err := tools.QueryRepoWithSource(QueryRepoArgs{Repo: "org/repo", Name: "ReserveProduct"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.([]store.NodeRow); !ok {
		t.Fatalf("default query_repo shape = %T, want []store.NodeRow", out)
	}
}

func TestQueryRepoWithSourceHydratesExplicitSnippet(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("line1\nline2\nline3\n", "text/plain")
	out, err := tools.QueryRepoWithSource(QueryRepoArgs{
		Repo: "org/repo",
		Name: "ReserveProduct",
		SourceContent: &SourceContentOptions{Mode: sourceContentModeSnippet, LineRadius: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.([]SourceNodeResult)
	if !ok {
		t.Fatalf("hydrated query_repo shape = %T, want []SourceNodeResult", out)
	}
	if len(got) != 1 || got[0].Node.NodeID != "n1" || got[0].SourceContent.Status != sourceContentStatusFetched {
		t.Fatalf("hydrated query_repo mismatch: %+v", got)
	}
}
```

- [x] **Step 2: Write `explain_symbol` default and hydrated tests**

Append to `internal/mcp/tools_test.go`.

```go
func TestExplainSymbolWithSourcePreservesDefaultShape(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	out, err := tools.ExplainSymbolWithSource(ExplainSymbolArgs{Repo: "org/repo", Symbol: "ReserveProduct"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(SymbolExplanation); !ok {
		t.Fatalf("default explain_symbol shape = %T, want SymbolExplanation", out)
	}
}

func TestExplainSymbolWithSourceHydratesFullContent(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("package server\nfunc ReserveProduct() {}\n", "text/plain")
	out, err := tools.ExplainSymbolWithSource(ExplainSymbolArgs{
		Repo: "org/repo",
		Symbol: "ReserveProduct",
		SourceContent: &SourceContentOptions{Mode: sourceContentModeFull},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(SourceSymbolExplanation)
	if !ok {
		t.Fatalf("hydrated explain_symbol shape = %T, want SourceSymbolExplanation", out)
	}
	if got.Explanation.Node.NodeID != "n1" || !strings.Contains(got.SourceContent.Content, "ReserveProduct") {
		t.Fatalf("hydrated explain_symbol mismatch: %+v", got)
	}
}
```

- [x] **Step 3: Run focused failing tests**

Run: `go test ./internal/mcp -run 'TestQueryRepoWithSource|TestExplainSymbolWithSource' -count=1`

Expected: FAIL with missing `QueryRepoArgs`, `ExplainSymbolArgs`, or hydratable methods.

- [x] **Step 4: Add hydratable args and methods**

Add exported args, then keep existing pure methods untouched.

```go
type QueryRepoArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	Name          string                `json:"name" jsonschema:"symbol label to look up"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}

type ExplainSymbolArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	Symbol        string                `json:"symbol" jsonschema:"node id or label to explain"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}
```

Implement `QueryRepoWithSource`:

- Resolve nodes by calling existing `QueryRepo(args.Repo, args.Name)`.
- If `args.SourceContent == nil` or normalized mode is `off`, return `nodes` as `[]store.NodeRow`.
- For mode `auto`, hydrate only when `len(nodes) > 1` or at least one node has an empty `SourceLocation` and fetchable provenance.
- For mode `snippet` or `full`, hydrate up to `req.maxCandidates` nodes.
- For skipped auto hydration, return metadata-only `nodes` so default-like shape is preserved when auto has no trigger.

Implement `ExplainSymbolWithSource`:

- Resolve the explanation by calling existing `ExplainSymbol(args.Repo, args.Symbol)`.
- If `args.SourceContent == nil` or normalized mode is `off`, return `SymbolExplanation`.
- For mode `auto`, hydrate only when the resolved node has no parseable source location and fetchable provenance.
- For mode `snippet` or `full`, hydrate the resolved node.
- Return `SourceSymbolExplanation{Explanation: explanation, SourceContent: source}` when hydrating.

- [x] **Step 5: Verify focused tests pass**

Run: `go test ./internal/mcp -run 'TestQueryRepoWithSource|TestExplainSymbolWithSource' -count=1`

Expected: PASS.

## Task 5: Integrate `get_file_context` and `get_context`

**OpenSpec coverage:** 2.3, 2.4, 3.2 default and opt-in behavior.

**Files:**
- Modify `internal/mcp/tools.go`
- Modify `internal/mcp/context_tools.go`
- Modify `internal/mcp/tools_test.go`
- Modify `internal/mcp/context_tools_test.go`

**Interfaces:**
- Consumes: `GetFileContextWithSource`, `GetContextArgs.SourceContent`, optional `CodeContext.SourceContent`.
- Produces: `SourceFileContextResult` for hydrated file context and hydrated code-symbol matches in `ContextResult`.

- [x] **Step 1: Write `get_file_context` default and hydrated tests**

Append to `internal/mcp/tools_test.go`.

```go
func TestGetFileContextWithSourcePreservesDefaultShape(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	out, err := tools.GetFileContextWithSource(GetFileContextArgs{Repo: "org/repo", File: "internal/server.go"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.([]store.NodeRow); !ok {
		t.Fatalf("default get_file_context shape = %T, want []store.NodeRow", out)
	}
}

func TestGetFileContextWithSourceHydratesFile(t *testing.T) {
	tools, _ := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("package server\nfunc ReserveProduct() {}\n", "text/plain")
	out, err := tools.GetFileContextWithSource(GetFileContextArgs{
		Repo: "org/repo",
		File: "internal/server.go",
		SourceContent: &SourceContentOptions{},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(SourceFileContextResult)
	if !ok {
		t.Fatalf("hydrated get_file_context shape = %T, want SourceFileContextResult", out)
	}
	if got.File != "internal/server.go" || len(got.Symbols) != 1 || got.SourceContent.Status != sourceContentStatusFetched {
		t.Fatalf("hydrated get_file_context mismatch: %+v", got)
	}
}
```

- [x] **Step 2: Write `get_context` default and hydrated tests**

Append to `internal/mcp/context_tools_test.go`.

```go
func TestGetContextSourceContentPreservesDefaultCodeSymbolShape(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", Mode: "code"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matches.CodeSymbols) != 1 {
		t.Fatalf("expected one code symbol, got %+v", out.Matches.CodeSymbols)
	}
	if out.Matches.CodeSymbols[0].SourceContent != nil {
		t.Fatalf("default get_context included source content: %+v", out.Matches.CodeSymbols[0].SourceContent)
	}
}

func TestGetContextSourceContentHydratesCodeSymbols(t *testing.T) {
	tools := seedContextTools(t)
	tools.sourceHydrator = testHydrator("line1\nline2\nline3\nline4\n", "text/plain")
	_, err := tools.s.DB.Exec(`UPDATE nodes SET source_url=? WHERE node_id=?`, "https://raw.githubusercontent.com/org/inventory/abc123/internal/http/reservation_handler.go", "handler_reserve")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", Mode: "code", SourceContent: &SourceContentOptions{Mode: sourceContentModeSnippet, LineRadius: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matches.CodeSymbols) != 1 || out.Matches.CodeSymbols[0].SourceContent == nil {
		t.Fatalf("expected hydrated code symbol, got %+v", out.Matches.CodeSymbols)
	}
	if out.Matches.CodeSymbols[0].SourceContent.Status != sourceContentStatusFetched {
		t.Fatalf("source content mismatch: %+v", out.Matches.CodeSymbols[0].SourceContent)
	}
}
```

- [x] **Step 3: Run focused failing tests**

Run: `go test ./internal/mcp -run 'TestGetFileContextWithSource|TestGetContextSourceContent' -count=1`

Expected: FAIL with missing `GetFileContextArgs`, `GetFileContextWithSource`, or `CodeContext.SourceContent` compile errors.

- [x] **Step 4: Add `GetFileContextArgs` and hydratable file context method**

Define args.

```go
type GetFileContextArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	File          string                `json:"file" jsonschema:"source file path"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}
```

Implement `GetFileContextWithSource`:

- Resolve symbols by calling existing `GetFileContext(args.Repo, args.File)`.
- If `args.SourceContent == nil` or normalized mode is `off`, return `[]store.NodeRow`.
- If `source_content` is present with empty mode, treat it as automatic and hydrate because `get_file_context` is an explicit file-context request.
- Use `ReadSourceContent(ReadSourceContentArgs{Repo: args.Repo, File: args.File, SourceContent: args.SourceContent})` to hydrate full-file content by default.
- Return `SourceFileContextResult{File: args.File, Symbols: symbols, SourceContent: source}`.

- [x] **Step 5: Add `get_context` source content fields and logic**

Modify `GetContextArgs`.

```go
SourceContent *SourceContentOptions `json:"source_content,omitempty" jsonschema:"optional GitHub source hydration: off|auto|snippet|full"`
```

Modify `CodeContext`.

```go
SourceContent *SourceContent `json:"source_content,omitempty"`
```

Update `GetContext` and `contextMatches` so source options flow into code-symbol matching. The minimal signature change is:

```go
func (t *Tools) contextMatches(indexID int64, ri registry.RepoInfo, topic, mode string, includeResources bool, opts *SourceContentOptions) (ContextMatches, []string, []ToolHint)
```

Hydrate only code-symbol matches. Preserve API, proto, and library behavior. For `mode: auto`, hydrate when more than one code symbol matched or a matched code symbol has no parseable source location and fetchable provenance. For explicit `snippet` or `full`, hydrate up to `max_candidates` code-symbol matches.

- [x] **Step 6: Verify focused tests pass**

Run: `go test ./internal/mcp -run 'TestGetFileContextWithSource|TestGetContextSourceContent' -count=1`

Expected: PASS.

## Task 6: Wire MCP Server Registration and End-to-End Surface

**OpenSpec coverage:** 2.1 through 2.6 and e2e surface expectations.

**Files:**
- Modify `internal/mcp/server.go`
- Modify `internal/mcp/e2e_surface_test.go`
- Modify `internal/mcp/e2e_test.go` comments if they mention the old count

**Interfaces:**
- Consumes: pure hydratable methods and `ReadSourceContent`.
- Produces: advertised `read_source_content` MCP tool and server wrappers for optional `source_content` args.

- [x] **Step 1: Write server-surface expectations first**

Modify `internal/mcp/e2e_surface_test.go` fixture graph nodes to include deterministic `source_url` values such as `https://raw.githubusercontent.com/org/inventory/abc/internal/grpc/server.go`. Add a call assertion for `read_source_content` that expects a status envelope, not necessarily live fetched content, because e2e must not depend on GitHub network.

Use this smoke assertion near the other code-graph tool checks:

```go
sourceBody := call("read_source_content", map[string]any{"repo": "org/inventory", "node_id": "grpc_server_reserveproduct", "source_content": map[string]any{"mode": "snippet"}})
mustContain("read_source_content", sourceBody, "status", "source_url")
```

If the e2e fixture uses raw GitHub URLs that cannot be reached in CI, assert the structured status only. The acceptable statuses are `fetched`, `error`, `unsupported`, or `skipped`; the key requirement is that the tool call succeeds and returns a `source_content` envelope rather than a protocol error.

- [x] **Step 2: Run the focused failing surface test compile**

Run: `go test ./internal/mcp -run TestEndToEndToolSurface -count=1`

Expected: FAIL because `read_source_content` is not registered or not advertised yet.

- [x] **Step 3: Update `ToolNames` and registration order**

In `internal/mcp/server.go`, add `read_source_content` after `get_file_context` so it sits with code/source tools.

```go
var ToolNames = []string{
	"list_repos",
	"resolve_repo",
	"get_context",
	"query_repo",
	"explain_symbol",
	"get_file_context",
	"read_source_content",
	"call_path",
	"list_apis",
	"find_endpoint",
	"explain_endpoint",
	"find_schema",
	"find_rpc",
	"explain_rpc",
	"find_private_library",
	"find_library_consumers",
	"explain_private_library",
}
```

Call `registerReadSourceContent(srv, tools)` after `registerGetFileContext(srv, tools)`.

- [x] **Step 4: Replace server arg structs for hydrated tools**

Remove or stop using unexported `queryRepoArgs`, `explainSymbolArgs`, and `getFileContextArgs`. Reuse `QueryRepoArgs`, `ExplainSymbolArgs`, and `GetFileContextArgs` in registrations.

Update registrations:

```go
out, err := tools.QueryRepoWithSource(args)
```

```go
out, err := tools.ExplainSymbolWithSource(args)
```

```go
out, err := tools.GetFileContextWithSource(args)
```

Keep `registerGetContext` as-is except the decoded `GetContextArgs` now carries `SourceContent`.

- [x] **Step 5: Add `read_source_content` registration**

Add this registration in `internal/mcp/server.go`.

```go
func registerReadSourceContent(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "read_source_content",
		Description: "Read GitHub source content for an indexed graph node or source file using stored Graphify provenance.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args ReadSourceContentArgs) (*mcpsdk.CallToolResult, any, error) {
		out, err := tools.ReadSourceContent(args)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(out)), nil, nil
	})
}
```

If `ReadSourceContent` can return validation errors that are not `ErrNotFound`, let them be protocol errors. Repo and node not-found errors should still use `toolErr`.

- [x] **Step 6: Verify server-surface tests**

Run: `go test ./internal/mcp -run 'TestEndToEndToolSurface|TestEndToEndStdio' -count=1`

Expected: PASS. If network restrictions make the direct source read return `error` status, the test should still pass because it asserts the structured envelope rather than fetched body text.

## Task 7: Documentation and OpenSpec Task Closure

**OpenSpec coverage:** 3.4, 3.5.

**Files:**
- Modify `docs/tools.md`
- Modify `openspec/changes/hydrate-github-source-content/tasks.md`

**Interfaces:**
- Consumes: final tool args and response shapes from Tasks 2 through 6.
- Produces: accurate docs and checked OpenSpec task list.

- [x] **Step 1: Update tool count and list in docs**

In `docs/tools.md`, change the advertised count from 16 to 17 and add `read_source_content` after `get_file_context` in the registration-order list.

- [x] **Step 2: Document `source_content` options once**

Add a section under `## Repo & code-graph tools` before `get_context` or after `get_file_context` with this exact option table.

```markdown
### Source content hydration option

`get_context`, `query_repo`, `explain_symbol`, and `get_file_context` accept an optional `source_content` object. Omitting it preserves metadata-only behavior and does not fetch source text.

| Field | Type | Description |
|-----|------|-------------|
| `mode` | string | `off`, `auto`, `snippet`, or `full`; an empty object means `auto` for enrichment tools |
| `max_bytes` | number | maximum fetched bytes before truncation; default 65536 |
| `line_radius` | number | lines before and after the node source location for snippets; default 20 |
| `max_candidates` | number | maximum hydrated candidates for ambiguous results; default 5 |

The source-content envelope uses `status` values `fetched`, `skipped`, `unsupported`, and `error`. Fetch errors, unsupported URLs, missing provenance, non-text content, and truncation are reported inside this envelope instead of failing the whole tool call when graph metadata is available.
```

- [x] **Step 3: Update each affected tool doc**

For `get_context`, `query_repo`, `explain_symbol`, and `get_file_context`, add `source_content` to the arg table and show the hydrated response shape for that tool:

```json
{
  "node": {"NodeID": "reservation_service_reserveproduct", "Label": "ReserveProduct", "SourceFile": "internal/reservation/service.go", "SourceURL": "https://github.com/org/inventory-service/blob/abc123/internal/reservation/service.go"},
  "source_content": {
    "status": "fetched",
    "mode": "snippet",
    "source_file": "internal/reservation/service.go",
    "source_url": "https://raw.githubusercontent.com/org/inventory-service/abc123/internal/reservation/service.go",
    "start_line": 30,
    "end_line": 70,
    "content": "func (s *Service) ReserveProduct(ctx context.Context, req Request) Response {\n  return Response{}\n}"
  }
}
```

For `explain_symbol`, document that the response wraps the existing `SymbolExplanation` under `explanation` and adds `source_content`. For `get_file_context`, document that the response contains `file`, `symbols`, and `source_content`. For `get_context`, document that `matches.code_symbols[]` gains an optional `source_content` field.

- [x] **Step 4: Document `read_source_content`**

Add a new subsection after `get_file_context`.

```markdown
### `read_source_content`

Read GitHub source content directly for an indexed graph node or source file.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `node_id` | string | graph node id to read; provide this or `file` |
| `file` | string | source file path to read; provide this or `node_id` |
| `source_content` | object | optional source-content options; omitted means snippet for node reads with a source location and full capped content for file reads |

**Response** — `SourceContent` envelope.
```

- [x] **Step 5: Document Graphify provenance expectations and limits**

In the same docs section, state that candle prefers `nodes.source_url` from Graphify when it is a GitHub raw URL or convertible GitHub blob URL, and can fall back to repo identity plus commit/branch plus `source_file` when enough snapshot data exists. State that non-GitHub hosts return `unsupported` for this release.

- [x] **Step 6: Run docs and full tests**

Run: `go test ./...`

Expected: PASS.

- [x] **Step 7: Mark OpenSpec tasks complete**

After `go test ./...` passes, edit `openspec/changes/hydrate-github-source-content/tasks.md` and change every checkbox from `- [ ]` to `- [x]` for tasks 1.1 through 3.5.

## Final Verification Checklist

- [ ] Run `go test ./internal/store -count=1`.
- [ ] Run `go test ./internal/mcp -count=1`.
- [ ] Run `go test ./...`.
- [ ] Confirm default JSON shapes remain metadata-only when `source_content` is omitted by inspecting tests for `QueryRepoWithSource`, `ExplainSymbolWithSource`, `GetFileContextWithSource`, and `GetContext`.
- [ ] Confirm `docs/tools.md` says 17 tools and lists `read_source_content` exactly once.
- [ ] Confirm `openspec/changes/hydrate-github-source-content/tasks.md` is fully checked only after tests pass.

## Planning Risks

- `source_location` formats may vary beyond `L10`, `L10-L12`, `line 10`, and `10:4`; the first-integer parser is intentionally conservative and should return structured skipped status when it cannot find a line.
- Fallback raw GitHub URL construction assumes repo identity is `org/name`; private non-GitHub repo identities will return fetch `error` or unsupported status rather than fetched content.
- The direct source-read e2e assertion must not require live GitHub access; it should assert the structured envelope so CI remains deterministic.
- Adding provenance fields to `NodeRow` expands metadata-only JSON with new metadata fields, but does not include fetched source text and does not rename existing fields.
