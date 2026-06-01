package definitions

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

type SearchProvider interface {
	Search(context.Context, *http.Client, string, int) ([]map[string]string, error)
}

type DuckDuckGoHTML struct {
	Endpoint string
}

func WebSearch(provider SearchProvider) tools.Definition {
	return tools.Definition{
		Name: "webSearch", Advanced: true, Description: "Search the web after user permission. Returns titles, URLs, and snippets.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"query", "reason"},
			"properties": map[string]any{
				"query": map[string]any{"type": "string"}, "reason": map[string]any{"type": "string"},
				"count": map[string]any{"type": "integer", "minimum": 1, "maximum": 10},
			},
		},
		Execute: func(ctx context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			query, err := stringArg(args, "query")
			if err != nil {
				return nil, err
			}
			reason, err := stringArg(args, "reason")
			if err != nil {
				return nil, err
			}
			count, err := intArg(args, "count", 5)
			if err != nil || count < 1 || count > 10 {
				return nil, fmt.Errorf("count must be between 1 and 10")
			}
			if toolContext.Permissions == nil || toolContext.Permissions.Authorize(ctx, permissions.Request{
				Tool: "webSearch", Reason: reason, Target: query, SimilarKey: query,
			}) == permissions.Deny {
				return denied("webSearch"), nil
			}
			return provider.Search(ctx, toolContext.HTTPClient, query, count)
		},
	}
}

func (provider DuckDuckGoHTML) Search(ctx context.Context, client *http.Client, query string, count int) ([]map[string]string, error) {
	endpoint := provider.Endpoint
	if endpoint == "" {
		endpoint = "https://html.duckduckgo.com/html/"
	}
	requestURL := endpoint + "?q=" + url.QueryEscape(query)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	content, _, err := readLimited(response.Body, 2<<20)
	if err != nil {
		return nil, err
	}
	return parseDuckDuckGo(string(content), count), nil
}

var resultPattern = regexp.MustCompile(`(?s)<a[^>]+class="[^"]*result__a[^"]*"[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?<a[^>]+class="[^"]*result__snippet[^"]*"[^>]*>(.*?)</a>`)
var tagsPattern = regexp.MustCompile(`<[^>]+>`)

func parseDuckDuckGo(content string, count int) []map[string]string {
	var results []map[string]string
	for _, match := range resultPattern.FindAllStringSubmatch(content, count) {
		target := html.UnescapeString(match[1])
		if parsed, err := url.Parse(target); err == nil && parsed.Query().Get("uddg") != "" {
			target = parsed.Query().Get("uddg")
		}
		results = append(results, map[string]string{
			"title": cleanHTML(match[2]), "url": target, "snippet": cleanHTML(match[3]),
		})
	}
	return results
}

func cleanHTML(content string) string {
	return strings.TrimSpace(html.UnescapeString(tagsPattern.ReplaceAllString(content, "")))
}
