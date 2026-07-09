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

func TestHydrateSourceContentUnsupportedURL(t *testing.T) {
	h := testHydrator("package main\n", "text/plain")
	got := h.hydrateNode(context.Background(), registry.RepoInfo{}, store.NodeRow{
		NodeID:     "n1",
		SourceFile: "a.go",
		SourceURL:  "https://gitlab.com/org/repo/blob/main/a.go",
	}, sourceContentRequest{mode: sourceContentModeFull, maxBytes: 1024, lineRadius: 20, maxCandidates: 5})
	if got.Status != sourceContentStatusUnsupported || !strings.Contains(got.Reason, "unsupported source URL") {
		t.Fatalf("unsupported URL status mismatch: %+v", got)
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
