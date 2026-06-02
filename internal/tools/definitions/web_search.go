package definitions

import (
	"context"
	"encoding/xml"
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

type BingRSS struct {
	Endpoint string
}

func WebSearch(provider SearchProvider) tools.Definition {
	return tools.Definition{
		Name: "webSearch", Advanced: true, Description: "Search the web. Default permission mode allows searches automatically. Returns titles, URLs, and snippets.",
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
	requestURL, err := searchURL(endpoint, query)
	if err != nil {
		return nil, err
	}
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
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search provider returned %s", response.Status)
	}
	results := parseDuckDuckGo(string(content), count)
	if len(results) == 0 {
		return nil, fmt.Errorf("search provider returned no parseable results")
	}
	return results, nil
}

var resultPattern = regexp.MustCompile(`(?s)<a[^>]+class="[^"]*result__a[^"]*"[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?<a[^>]+class="[^"]*result__snippet[^"]*"[^>]*>(.*?)</a>`)
var tagsPattern = regexp.MustCompile(`<[^>]+>`)

func (provider BingRSS) Search(ctx context.Context, client *http.Client, query string, count int) ([]map[string]string, error) {
	endpoint := provider.Endpoint
	if endpoint == "" {
		endpoint = "https://www.bing.com/search?format=rss"
	}
	requestURL, err := searchURL(endpoint, query)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search provider returned %s", response.Status)
	}
	content, _, err := readLimited(response.Body, 2<<20)
	if err != nil {
		return nil, err
	}
	var feed struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(content, &feed); err != nil {
		return nil, fmt.Errorf("decode search provider response: %w", err)
	}
	results := make([]map[string]string, 0, min(count, len(feed.Items)))
	for _, item := range feed.Items {
		if len(results) == count {
			break
		}
		results = append(results, map[string]string{
			"title": cleanHTML(item.Title), "url": strings.TrimSpace(item.Link), "snippet": cleanHTML(item.Description),
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("search provider returned no results")
	}
	return results, nil
}

func searchURL(endpoint, query string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid search endpoint: %w", err)
	}
	values := parsed.Query()
	values.Set("q", query)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

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
