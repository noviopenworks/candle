// Package mcp source hydration core: GitHub source URL normalization, bounded
// fetch, text detection, snippet extraction, and structured per-source status
// envelopes shared by the source-aware MCP tools.
package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/noviopenworks/candle/internal/registry"
	"github.com/noviopenworks/candle/internal/store"
)

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

// SourceContentOptions is the optional source_content argument attached to MCP
// tool calls. nil or mode "off" preserves existing metadata-only behavior.
type SourceContentOptions struct {
	Mode          string `json:"mode,omitempty"` // off|auto|snippet|full
	MaxBytes      int    `json:"max_bytes,omitempty"`
	LineRadius    int    `json:"line_radius,omitempty"`
	MaxCandidates int    `json:"max_candidates,omitempty"`
}

// SourceContent is the per-source status envelope returned by hydration and by
// hydrated wrapper results. Status is always set; other fields are optional.
type SourceContent struct {
	Status     string `json:"status"` // fetched|skipped|unsupported|error
	Mode       string `json:"mode,omitempty"`
	SourceFile string `json:"source_file,omitempty"`
	SourceURL  string `json:"source_url,omitempty"`
	StartLine  int    `json:"start_line,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	Content    string `json:"content,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// sourceContentRequest is the normalized, internal form of
// SourceContentOptions used by the hydrator.
type sourceContentRequest struct {
	mode          string
	maxBytes      int
	lineRadius    int
	maxCandidates int
}

// sourceContentRequestFromOptions applies caller-supplied options on top of the
// per-tool default mode. A nil opts yields defaults verbatim.
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

// normalizeGitHubSourceURL accepts raw.githubusercontent.com URLs as-is and
// converts github.com blob URLs to raw URLs. Any other host or malformed path
// returns ("", false).
func normalizeGitHubSourceURL(raw string) (string, bool) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", false
	}
	const rawHost = "raw.githubusercontent.com"
	const rawPrefix = "https://" + rawHost + "/"
	if strings.HasPrefix(u, rawPrefix) {
		return u, true
	}
	const blobPrefix = "https://github.com/"
	if !strings.HasPrefix(u, blobPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(u, blobPrefix)
	// Path shape: <org>/<repo>/blob/<ref>/<path...>. Reject shallow paths.
	parts := strings.SplitN(rest, "/", 5)
	if len(parts) < 5 || parts[2] != "blob" {
		return "", false
	}
	org, repo, ref, path := parts[0], parts[1], parts[3], parts[4]
	if org == "" || repo == "" || ref == "" || path == "" {
		return "", false
	}
	return rawPrefix + org + "/" + repo + "/" + ref + "/" + path, true
}

// isSupportedSourceURL reports whether raw looks like a fetchable GitHub URL.
func isSupportedSourceURL(raw string) bool {
	_, ok := normalizeGitHubSourceURL(raw)
	return ok
}

// rawURLForNode prefers n.SourceURL when supported; otherwise it builds a raw
// URL from repo identity (commit takes precedence over branch) and source file.
func rawURLForNode(ri registry.RepoInfo, n store.NodeRow) (string, bool) {
	if n.SourceURL != "" {
		if u, ok := normalizeGitHubSourceURL(n.SourceURL); ok {
			return u, true
		}
	}
	if ri.Repo == "" || n.SourceFile == "" {
		return "", false
	}
	ref := ri.Commit
	if ref == "" {
		ref = ri.Branch
	}
	if ref == "" {
		return "", false
	}
	return "https://raw.githubusercontent.com/" + ri.Repo + "/" + ref + "/" + n.SourceFile, true
}

// parseSourceLocation returns the first positive integer encoded in a source
// location string such as L10, L10-L12, "line 10", or "10:4". Named to avoid
// colliding with the existing parseSourceLine helper in library_explain.go.
func parseSourceLocation(sourceLocation string) (int, bool) {
	digits := ""
	for _, r := range sourceLocation {
		if r >= '0' && r <= '9' {
			digits += string(r)
		} else if digits != "" {
			break
		}
	}
	if digits == "" {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// snippetWindow returns whole lines only, centered on centerLine with radius
// lines on each side, clamped to file bounds. One trailing newline is trimmed.
func snippetWindow(content string, centerLine, radius int) (snippet string, startLine int, endLine int) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || centerLine <= 0 {
		return content, 1, len(lines)
	}
	start := centerLine - radius
	if start < 1 {
		start = 1
	}
	end := centerLine + radius
	if end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		start = len(lines)
	}
	// lines is 0-indexed; line N lives at lines[N-1].
	picked := lines[start-1 : end]
	out := strings.TrimSuffix(strings.Join(picked, "\n"), "\n")
	return out, start, end
}

// looksText reports whether contentType and a small byte scan suggest textual
// content suitable for inclusion in MCP responses.
func looksText(contentType string, b []byte) bool {
	if bytes.IndexByte(b, 0x00) >= 0 {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	switch ct {
	case "application/json",
		"application/xml",
		"application/x-yaml",
		"application/yaml":
		return true
	}
	return false
}

// fetchText reads at most maxBytes+1 bytes from rawURL via client, marking
// truncation when more than maxBytes bytes are available and capping content at
// maxBytes. Non-2xx responses produce an error mentioning the status code.
func fetchText(ctx context.Context, client *http.Client, rawURL string, maxBytes int) (content string, truncated bool, contentType string, err error) {
	if client == nil {
		return "", false, "", errors.New("source hydrator has no http client")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", false, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}
	limit := maxBytes + 1
	buf := make([]byte, limit)
	n, readErr := io.ReadFull(resp.Body, buf)
	switch {
	case readErr == nil:
		// Buffer filled: source has at least limit bytes, so truncate.
		return string(buf[:maxBytes]), true, resp.Header.Get("Content-Type"), nil
	case errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF):
		// Source exhausted before filling the buffer: no truncation possible.
		return string(buf[:n]), false, resp.Header.Get("Content-Type"), nil
	default:
		return "", false, "", readErr
	}
}

// sourceHydrator wraps an injectable http.Client for source content fetches.
type sourceHydrator struct {
	client *http.Client
}

// newSourceHydrator builds a source hydrator with the default timeout.
func newSourceHydrator() *sourceHydrator {
	return &sourceHydrator{client: &http.Client{Timeout: defaultSourceContentTimeout}}
}

// hydrateNode resolves a fetchable source URL for n, fetches it with bounds,
// and returns a structured SourceContent envelope. Every outcome (skipped,
// unsupported, error, fetched) becomes a status rather than a Go error.
func (h *sourceHydrator) hydrateNode(ctx context.Context, ri registry.RepoInfo, n store.NodeRow, req sourceContentRequest) SourceContent {
	if req.mode == sourceContentModeOff {
		return SourceContent{Status: sourceContentStatusSkipped, Reason: "source_content mode is off"}
	}
	if n.SourceURL != "" && !isSupportedSourceURL(n.SourceURL) {
		return SourceContent{Status: sourceContentStatusUnsupported, SourceFile: n.SourceFile, SourceURL: n.SourceURL, Reason: "unsupported source URL"}
	}
	rawURL, ok := rawURLForNode(ri, n)
	if !ok {
		return SourceContent{Status: sourceContentStatusSkipped, SourceFile: n.SourceFile, Reason: "missing source provenance"}
	}
	content, truncated, contentType, err := fetchText(ctx, h.client, rawURL, req.maxBytes)
	if err != nil {
		return SourceContent{Status: sourceContentStatusError, SourceFile: n.SourceFile, SourceURL: rawURL, Reason: err.Error()}
	}
	if !looksText(contentType, []byte(content)) {
		return SourceContent{Status: sourceContentStatusUnsupported, SourceFile: n.SourceFile, SourceURL: rawURL, Reason: "non-text source content"}
	}
	out := SourceContent{
		Status:     sourceContentStatusFetched,
		Mode:       req.mode,
		SourceFile: n.SourceFile,
		SourceURL:  rawURL,
		Truncated:  truncated,
	}
	if req.mode == sourceContentModeSnippet {
		center, ok := parseSourceLocation(n.SourceLocation)
		if !ok {
			out.Mode = sourceContentModeFull
			out.Content = content
			return out
		}
		snippet, startLine, endLine := snippetWindow(content, center, req.lineRadius)
		out.Content = snippet
		out.StartLine = startLine
		out.EndLine = endLine
		return out
	}
	out.Content = content
	return out
}

// ReadSourceContentArgs are the arguments to ReadSourceContent. Exactly one of
// NodeID or File must be set; SourceContent tunes the fetch (nil = defaults).
type ReadSourceContentArgs struct {
	Repo          string                `json:"repo" jsonschema:"repo identity (org/name)"`
	NodeID        string                `json:"node_id,omitempty" jsonschema:"graph node id to read source for"`
	File          string                `json:"file,omitempty" jsonschema:"source file path to read source for"`
	SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}

// ReadSourceContent implements read_source_content: a direct source-read over
// the code graph. Node reads default to snippet mode when the node carries a
// parseable source location, else full; file reads default to full mode and
// pick the first node with fetchable source provenance. Missing-file cases
// return a skipped status rather than an error so the agent can keep iterating.
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

// nodeHasFetchableSource reports whether ri plus node carry enough provenance
// for rawURLForNode to assemble a fetchable GitHub URL.
func nodeHasFetchableSource(ri registry.RepoInfo, node store.NodeRow) bool {
	_, ok := rawURLForNode(ri, node)
	return ok
}

// defaultDirectMode picks snippet mode when the node pins a source line, else
// full mode; used by ReadSourceContent for node-id reads.
func defaultDirectMode(node store.NodeRow) string {
	if _, ok := parseSourceLocation(node.SourceLocation); ok {
		return sourceContentModeSnippet
	}
	return sourceContentModeFull
}
