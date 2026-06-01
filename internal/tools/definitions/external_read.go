package definitions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func ReadExternalFile() tools.Definition {
	return tools.Definition{
		Name: "readExternalFile", Advanced: true, Description: "Read a UTF-8 file outside the workspace after user permission.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"path", "reason"},
			"properties": map[string]any{"path": map[string]any{"type": "string"}, "reason": map[string]any{"type": "string"}},
		},
		Execute: func(ctx context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			name, err := stringArg(args, "path")
			if err != nil {
				return nil, err
			}
			reason, err := stringArg(args, "reason")
			if err != nil {
				return nil, err
			}
			if !filepath.IsAbs(name) {
				return nil, fmt.Errorf("readExternalFile requires an absolute path")
			}
			canonical, err := filepath.EvalSymlinks(name)
			if err != nil {
				return nil, err
			}
			request := permissions.Request{Tool: "readExternalFile", Reason: reason, Target: canonical, SimilarKey: canonical}
			if toolContext.Permissions == nil || toolContext.Permissions.Authorize(ctx, request) == permissions.Deny {
				return denied("readExternalFile"), nil
			}
			file, err := os.Open(canonical)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			content, truncated, err := readLimited(file, 2<<20)
			if err != nil {
				return nil, err
			}
			return map[string]any{"path": canonical, "content": string(content), "truncated": truncated}, nil
		},
	}
}

func denied(tool string) map[string]any {
	return map[string]any{"denied": true, "reason": "User denied permission for " + tool + "."}
}
