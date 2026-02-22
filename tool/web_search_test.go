package tool

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testDDGHTML = `<html><body>
<div id="links">
 <div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
   <a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2Fdoc%2F&amp;rut=abc123">Go Documentation</a>
  </h2>
  <div class="result__snippet">The Go programming language documentation and tutorials.</div>
 </div>
 <div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
   <a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fpkg.go.dev&amp;rut=def456">Go Packages</a>
  </h2>
  <div class="result__snippet">Discover &amp; learn about Go packages and modules.</div>
 </div>
 <div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
   <a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev&amp;rut=ghi789">The Go Programming Language</a>
  </h2>
  <div class="result__snippet">Build simple, secure, scalable systems with Go.</div>
 </div>
</div></body></html>`

const testDDGNoResults = `<html><body>
<div id="links">
 <div class="no-results">No results found</div>
</div></body></html>`

func TestWebSearchExecute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testDDGHTML)
	}))
	defer srv.Close()

	orig := ddgEndpoint
	ddgEndpoint = srv.URL
	defer func() { ddgEndpoint = orig }()

	tool := NewWebSearch()
	ctx := Context{Ctx: context.Background()}
	r := tool.Execute(ctx, `{"query":"golang"}`)

	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(r.Output, "Go Documentation") {
		t.Errorf("expected title 'Go Documentation' in output:\n%s", r.Output)
	}
	if !strings.Contains(r.Output, "https://golang.org/doc/") {
		t.Errorf("expected URL 'https://golang.org/doc/' in output:\n%s", r.Output)
	}
	if !strings.Contains(r.Output, "Go programming language documentation") {
		t.Errorf("expected snippet text in output:\n%s", r.Output)
	}
	if !strings.Contains(r.Output, "1.") || !strings.Contains(r.Output, "2.") {
		t.Errorf("expected numbered results in output:\n%s", r.Output)
	}
	if !strings.Contains(r.Title, "3 results") {
		t.Errorf("expected '3 results' in title, got: %s", r.Title)
	}
}

func TestWebSearchMaxResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testDDGHTML)
	}))
	defer srv.Close()

	orig := ddgEndpoint
	ddgEndpoint = srv.URL
	defer func() { ddgEndpoint = orig }()

	tool := NewWebSearch()
	ctx := Context{Ctx: context.Background()}
	r := tool.Execute(ctx, `{"query":"golang","max_results":1}`)

	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if strings.Contains(r.Output, "2.") {
		t.Errorf("expected only 1 result, but found '2.' in output:\n%s", r.Output)
	}
	if !strings.Contains(r.Title, "1 results") {
		t.Errorf("expected '1 results' in title, got: %s", r.Title)
	}
}

func TestWebSearchNoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testDDGNoResults)
	}))
	defer srv.Close()

	orig := ddgEndpoint
	ddgEndpoint = srv.URL
	defer func() { ddgEndpoint = orig }()

	tool := NewWebSearch()
	ctx := Context{Ctx: context.Background()}
	r := tool.Execute(ctx, `{"query":"xyznonexistent"}`)

	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(r.Output, "No results found") {
		t.Errorf("expected 'No results found' in output, got: %s", r.Output)
	}
	if !strings.Contains(r.Title, "0 results") {
		t.Errorf("expected '0 results' in title, got: %s", r.Title)
	}
}

func TestWebSearchEmptyQuery(t *testing.T) {
	tool := NewWebSearch()
	ctx := Context{Ctx: context.Background()}
	r := tool.Execute(ctx, `{"query":""}`)

	if r.Error == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(r.Error.Error(), "query is required") {
		t.Errorf("expected 'query is required' error, got: %v", r.Error)
	}
}

func TestWebSearchBadJSON(t *testing.T) {
	tool := NewWebSearch()
	ctx := Context{Ctx: context.Background()}
	r := tool.Execute(ctx, `{invalid}`)

	if r.Error == nil {
		t.Fatal("expected error for bad JSON")
	}
	if !strings.Contains(r.Error.Error(), "parse args") {
		t.Errorf("expected 'parse args' error, got: %v", r.Error)
	}
}

func TestWebSearchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := ddgEndpoint
	ddgEndpoint = srv.URL
	defer func() { ddgEndpoint = orig }()

	tool := NewWebSearch()
	ctx := Context{Ctx: context.Background()}
	r := tool.Execute(ctx, `{"query":"test"}`)

	if r.Error == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(r.Error.Error(), "HTTP 500") {
		t.Errorf("expected 'HTTP 500' in error, got: %v", r.Error)
	}
}

func TestWebSearchTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testDDGHTML)
	}))
	defer srv.Close()

	orig := ddgEndpoint
	ddgEndpoint = srv.URL
	defer func() { ddgEndpoint = orig }()

	tool := NewWebSearch()
	r := tool.Execute(Context{Ctx: ctx}, `{"query":"test"}`)

	if r.Error == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestWebSearchToolMetadata(t *testing.T) {
	tool := NewWebSearch()
	if tool.Name() != "web_search" {
		t.Errorf("expected name 'web_search', got %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
	params := tool.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map in parameters")
	}
	if _, ok := props["query"]; !ok {
		t.Error("expected 'query' in properties")
	}
	req, ok := params["required"].([]string)
	if !ok {
		t.Fatal("expected required array in parameters")
	}
	if len(req) != 1 || req[0] != "query" {
		t.Errorf("expected required=['query'], got %v", req)
	}
}

// --- Unit tests for parsing helpers ---

func TestParseResults(t *testing.T) {
	results := parseResults(testDDGHTML, 10)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Title != "Go Documentation" {
		t.Errorf("result[0] title = %q, want 'Go Documentation'", results[0].Title)
	}
	if results[0].URL != "https://golang.org/doc/" {
		t.Errorf("result[0] URL = %q, want 'https://golang.org/doc/'", results[0].URL)
	}
	if results[1].Title != "Go Packages" {
		t.Errorf("result[1] title = %q, want 'Go Packages'", results[1].Title)
	}
	if results[1].URL != "https://pkg.go.dev" {
		t.Errorf("result[1] URL = %q, want 'https://pkg.go.dev'", results[1].URL)
	}
	// Check entity decoding in snippet
	if !strings.Contains(results[1].Snippet, "Discover & learn") {
		t.Errorf("result[1] snippet should decode &amp; to &, got: %q", results[1].Snippet)
	}
}

func TestParseResultsCapped(t *testing.T) {
	results := parseResults(testDDGHTML, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (capped), got %d", len(results))
	}
}

func TestExtractURLUddg(t *testing.T) {
	raw := "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpage&rut=abc"
	got := extractURL(raw)
	if got != "https://example.com/page" {
		t.Errorf("extractURL(%q) = %q, want 'https://example.com/page'", raw, got)
	}
}

func TestExtractURLDirect(t *testing.T) {
	raw := "https://example.com/direct"
	got := extractURL(raw)
	if got != "https://example.com/direct" {
		t.Errorf("extractURL(%q) = %q, want %q", raw, got, raw)
	}
}

func TestExtractURLProtocolRelative(t *testing.T) {
	raw := "//example.com/path"
	got := extractURL(raw)
	if got != "https://example.com/path" {
		t.Errorf("extractURL(%q) = %q, want 'https://example.com/path'", raw, got)
	}
}

func TestExtractURLInvalid(t *testing.T) {
	for _, raw := range []string{"", "javascript:void(0)", "/relative/path"} {
		got := extractURL(raw)
		if got != "" {
			t.Errorf("extractURL(%q) = %q, want empty string", raw, got)
		}
	}
}

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"<b>bold</b> text", "bold text"},
		{"hello &amp; world", "hello & world"},
		{"  lots   of   space  ", "lots of space"},
		{"<a href=\"x\">link</a> &lt;tag&gt;", "link <tag>"},
		{"no tags", "no tags"},
	}
	for _, tt := range tests {
		got := cleanHTML(tt.in)
		if got != tt.want {
			t.Errorf("cleanHTML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"&amp;", "&"},
		{"&lt;script&gt;", "<script>"},
		{"&quot;hello&quot;", `"hello"`},
		{"&#39;apos&#39;", "'apos'"},
		{"&nbsp;space", " space"},
		{"no entities", "no entities"},
	}
	for _, tt := range tests {
		got := decodeHTMLEntities(tt.in)
		if got != tt.want {
			t.Errorf("decodeHTMLEntities(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTruncateQuery(t *testing.T) {
	short := "golang tutorials"
	if truncateQuery(short) != short {
		t.Errorf("short query should not be truncated")
	}

	long := "this is a really long search query that exceeds forty characters"
	got := truncateQuery(long)
	if len(got) > 40 {
		t.Errorf("truncated query should be <= 40 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated query should end with '...', got %q", got)
	}
}
