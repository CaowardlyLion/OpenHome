package definitions

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

var errRedirectDenied = errors.New("user denied redirect")

func FetchURL() tools.Definition {
	return tools.Definition{
		Name: "fetchURL", Advanced: true, Description: "Fetch text from an HTTP(S) URL after user permission.",
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
			content, truncated, err := readLimited(response.Body, 2<<20)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"status": response.StatusCode, "url": response.Request.URL.String(), "headers": selectedHeaders(response.Header),
				"body": string(content), "truncated": truncated,
			}, nil
		},
	}
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
		if toolContext.Permissions.Authorize(ctx, permissions.Request{Tool: tool, Reason: reason + " (redirect)", Target: target, SimilarKey: target}) == permissions.Deny {
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
