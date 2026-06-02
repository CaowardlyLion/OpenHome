package definitions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func DownloadFile() tools.Definition {
	return tools.Definition{
		Name: "downloadFile", Advanced: true, Effect: tools.EffectMutate, Description: "Download an HTTP(S) URL into the workspace after user permission. Maximum size: 50 MB.",
		Parameters: urlReasonSchema(true),
		Execute: func(ctx context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			rawURL, err := stringArg(args, "url")
			if err != nil {
				return nil, err
			}
			reason, err := stringArg(args, "reason")
			if err != nil {
				return nil, err
			}
			name, err := stringArg(args, "path")
			if err != nil {
				return nil, err
			}
			response, deniedResult, err := getApproved(ctx, toolContext, "downloadFile", rawURL, reason)
			if err != nil || deniedResult != nil {
				return deniedResult, err
			}
			defer response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				return nil, fmt.Errorf("download failed: HTTP %d", response.StatusCode)
			}
			content, truncated, err := readLimited(response.Body, 50<<20)
			if err != nil {
				return nil, err
			}
			if truncated {
				return nil, fmt.Errorf("download exceeds 50 MB limit")
			}
			file, err := toolContext.Workspace.ResolveWrite(name)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(file, content, 0o644); err != nil {
				return nil, err
			}
			relative, _ := filepath.Rel(toolContext.Workspace.Root, file)
			return map[string]any{"path": relative, "bytes": len(content), "contentType": response.Header.Get("Content-Type"), "url": response.Request.URL.String()}, nil
		},
	}
}
