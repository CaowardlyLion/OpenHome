package definitions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
	xhtml "golang.org/x/net/html"
)

var errRedirectDenied = errors.New("user denied redirect")

const fetchURLLinkLimit = 60

func FetchURL() tools.Definition {
	return tools.Definition{
		Name: "fetchURL", Advanced: true, Description: "Fetch compact readable text and bounded page links from an HTTP(S) URL. Default permission mode allows static fetches automatically. Use specific returned article links when citing sources. If the site rejects static fetching or returns a browser challenge, follow the result guidance and request skill reselection for browser-navigation.",
		Parameters: urlReasonSchema(false),
		Execute: func(ctx context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			rawURL, err := stringArg(args, "url")
			if err != nil {
				return nil, err
			}
			reason, err := stringArg(args, "reason")
			if err != nil {
				return nil, err
			}
			response, deniedResult, err := getApproved(ctx, toolContext, "fetchURL", rawURL, reason)
			if err != nil || deniedResult != nil {
				return deniedResult, err
			}
			defer response.Body.Close()
			content, responseTruncated, err := readLimited(response.Body, 2<<20)
			if err != nil {
				return nil, err
			}
			return fetchedResponseResult(response, content, responseTruncated), nil
		},
	}
}

func fetchedResponseResult(response *http.Response, content []byte, responseTruncated bool) map[string]any {
	text, contextTruncated := compactResponseText(content, response.Header.Get("Content-Type"))
	result := map[string]any{
		"status": response.StatusCode, "url": response.Request.URL.String(), "headers": selectedHeaders(response.Header),
	}
	if reason := browserNavigationFallbackReason(response.StatusCode, text); reason != "" {
		result["blocked"] = true
		result["reason"] = reason
		result["recommendedAction"] = "Call request_skill_reselection so the librarian can select browser-navigation, then use browserInteract."
		result["recommendedSkill"] = "browser-navigation"
		return result
	}
	result["text"] = text
	if links := extractResponseLinks(content, response.Request.URL); len(links) > 0 {
		result["links"] = links
	}
	result["truncated"] = responseTruncated || contextTruncated
	return result
}

func extractResponseLinks(content []byte, baseURL *url.URL) []map[string]string {
	if baseURL == nil {
		return nil
	}
	document, err := xhtml.Parse(bytes.NewReader(content))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	result := make([]map[string]string, 0, fetchURLLinkLimit)
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if len(result) == fetchURLLinkLimit {
			return
		}
		if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "a") {
			href := anchorHref(node)
			if href != "" {
				result = appendResolvedLink(result, seen, baseURL, href, nodeText(node))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	return result
}

func appendResolvedLink(result []map[string]string, seen map[string]bool, baseURL *url.URL, href, label string) []map[string]string {
	href = strings.TrimSpace(href)
	if href == "" {
		return result
	}
	target, err := url.Parse(href)
	if err != nil {
		return result
	}
	target = baseURL.ResolveReference(target)
	if (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return result
	}
	target.Fragment = ""
	resolved := target.String()
	if seen[resolved] {
		return result
	}
	text := strings.TrimSpace(spacePattern.ReplaceAllString(label, " "))
	if text == "" {
		return result
	}
	if len(text) > 240 {
		text = text[:240]
	}
	seen[resolved] = true
	return append(result, map[string]string{"text": text, "url": resolved})
}

func anchorHref(node *xhtml.Node) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "href") {
			return attr.Val
		}
	}
	return ""
}

func nodeText(node *xhtml.Node) string {
	var builder strings.Builder
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func browserNavigationFallbackReason(status int, text string) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusProxyAuthRequired, http.StatusTooManyRequests:
		return fmt.Sprintf("Static fetch returned HTTP %d. The site may require a rendered browser session, authentication, or interactive access.", status)
	}
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"verify you are human",
		"checking your browser",
		"enable javascript and cookies",
		"captcha",
		"access denied",
		"please complete the following challenge",
		"unfortunately, bots use",
	} {
		if strings.Contains(lower, marker) {
			return "Static fetch returned a browser challenge or access-block page."
		}
	}
	return ""
}

func urlReasonSchema(path bool) map[string]any {
	properties := map[string]any{"url": map[string]any{"type": "string"}, "reason": map[string]any{"type": "string"}}
	required := []string{"url", "reason"}
	if path {
		properties["path"] = map[string]any{"type": "string"}
		required = append(required, "path")
	}
	return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
}

func getApproved(ctx context.Context, toolContext tools.Context, tool, rawURL, reason string) (*http.Response, any, error) {
	parsed, err := parseHTTPURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	if toolContext.Permissions == nil || toolContext.Permissions.Authorize(ctx, permissions.Request{
		Tool: tool, Reason: reason, Target: parsed.String(), SimilarKey: parsed.String(),
	}) == permissions.Deny {
		return nil, denied(tool), nil
	}
	client := *toolContext.HTTPClient
	redirects := 0
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		redirects++
		if redirects > 5 {
			return fmt.Errorf("too many redirects")
		}
		target := request.URL.String()
		if toolContext.Permissions.Authorize(ctx, permissions.Request{Tool: tool, Reason: reason + " (redirect)", Target: target, SimilarKey: target, Elevated: true}) == permissions.Deny {
			return fmt.Errorf("%w to %s", errRedirectDenied, target)
		}
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, errRedirectDenied) {
			return nil, denied(tool), nil
		}
		return nil, nil, err
	}
	return response, nil, nil
}
