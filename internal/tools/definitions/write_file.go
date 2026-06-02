package definitions

import (
	"context"
	"os"
	"path/filepath"

	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func WriteFile() tools.Definition {
	return tools.Definition{
		Name: "writeFile", Effect: tools.EffectMutate, Description: "Write a UTF-8 text file inside the workspace.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"path", "content"},
			"properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}},
		},
		Execute: func(_ context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			workspace := toolContext.Workspace
			name, err := stringArg(args, "path")
			if err != nil {
				return nil, err
			}
			content, err := stringArg(args, "content")
			if err != nil {
				return nil, err
			}
			file, err := workspace.ResolveWrite(name)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
				return nil, err
			}
			relative, _ := filepath.Rel(workspace.Root, file)
			return map[string]any{"path": relative, "written": true}, nil
		},
	}
}
