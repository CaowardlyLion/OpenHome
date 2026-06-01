package definitions

import (
	"context"
	"os"

	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func ListFiles() tools.Definition {
	return tools.Definition{
		Name: "listFiles", Description: "List files and directories in the workspace.", Parameters: pathSchema(false),
		Execute: func(_ context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			workspace := toolContext.Workspace
			name := "."
			if value, ok := args["path"]; ok {
				var valid bool
				name, valid = value.(string)
				if !valid {
					return nil, os.ErrInvalid
				}
			}
			dir, err := workspace.ResolveRead(name)
			if err != nil {
				return nil, err
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				return nil, err
			}
			result := make([]map[string]string, 0, len(entries))
			for _, entry := range entries {
				entryType := "file"
				if entry.IsDir() {
					entryType = "directory"
				}
				result = append(result, map[string]string{"name": entry.Name(), "type": entryType})
			}
			return result, nil
		},
	}
}
