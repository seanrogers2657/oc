package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ddgEndpoint is the DuckDuckGo HTML search URL. Package-level var for test injection.
var ddgEndpoint = "https://html.duckduckgo.com/html/"

// WebSearchTool searches the web via DuckDuckGo.
type WebSearchTool struct{}

// NewWebSearch creates a new web search tool.
func NewWebSearch() *WebSearchTool { return &WebSearchTool{} }

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web using DuckDuckGo. Returns titles, URLs, and snippets for matching results. Use this to find current information, documentation, or answers to questions."
}

func (t *WebSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default 10, max 30)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx Context, argsJSON string) Result {
	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{Error: fmt.Errorf("parse args: %w", err)}
	}
	if args.Query == "" {
		return Result{Error: fmt.Errorf("query is required")}
	}

	maxResults := 10
	if args.MaxResults > 0 {
		maxResults = args.MaxResults
	}
	if maxResults > 30 {
		maxResults = 30
	}

	searchCtx := ctx.Ctx
	if searchCtx == nil {
		searchCtx = context.Background()
	}
	searchCtx, cancel := context.WithTimeout(searchCtx, 30*time.Second)
	defer cancel()

	body, err := fetchDDG(searchCtx, args.Query)
	if err != nil {
		return Result{Error: fmt.Errorf("search failed: %w", err)}
	}

	results := parseResults(body, maxResults)

	if len(results) == 0 {
		return Result{
			Output: "No results found for: " + args.Query,
			Title:  fmt.Sprintf("Search %q (0 results)", truncateQuery(args.Query)),
		}
	}

	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet)
	}

	return Result{
		Output: b.String(),
		Title:  fmt.Sprintf("Search %q (%d results)", truncateQuery(args.Query), len(results)),
	}
}

// searchResult holds a single parsed search result.
type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func fetchDDG(ctx context.Context, query string) (string, error) {
	form := url.Values{"q": {query}}
	req, err := http.NewRequestWithContext(ctx, "POST", ddgEndpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Cap at 1MB to prevent runaway responses.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

var (
	resultLinkRe = regexp.MustCompile(`(?s)<a[^>]+class="result__a"[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	snippetRe    = regexp.MustCompile(`(?s)class="result__snippet"[^>]*>(.*?)</(?:a|td|div)>`)
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	uddgRe       = regexp.MustCompile(`[?&]uddg=([^&]+)`)
)

func parseResults(html string, max int) []searchResult {
	linkMatches := resultLinkRe.FindAllStringSubmatch(html, -1)
	snippetMatches := snippetRe.FindAllStringSubmatch(html, -1)

	var results []searchResult
	for i, m := range linkMatches {
		if len(results) >= max {
			break
		}

		rawHref := m[1]
		rawTitle := m[2]

		actualURL := extractURL(rawHref)
		if actualURL == "" {
			continue
		}

		title := cleanHTML(rawTitle)
		if title == "" {
			continue
		}

		snippet := ""
		if i < len(snippetMatches) && len(snippetMatches[i]) > 1 {
			snippet = cleanHTML(snippetMatches[i][1])
		}

		results = append(results, searchResult{
			Title:   title,
			URL:     actualURL,
			Snippet: snippet,
		})
	}
	return results
}

func extractURL(rawHref string) string {
	// DDG wraps URLs: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=...
	if m := uddgRe.FindStringSubmatch(rawHref); len(m) > 1 {
		decoded, err := url.QueryUnescape(m[1])
		if err == nil && (strings.HasPrefix(decoded, "http://") || strings.HasPrefix(decoded, "https://")) {
			return decoded
		}
	}
	if strings.HasPrefix(rawHref, "http://") || strings.HasPrefix(rawHref, "https://") {
		return rawHref
	}
	if strings.HasPrefix(rawHref, "//") {
		return "https:" + rawHref
	}
	return ""
}

func cleanHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = decodeHTMLEntities(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func decodeHTMLEntities(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&#x27;", "'",
		"&nbsp;", " ",
		"&#38;", "&",
	)
	return replacer.Replace(s)
}

func truncateQuery(q string) string {
	q = strings.TrimSpace(q)
	if len(q) > 40 {
		return q[:37] + "..."
	}
	return q
}
