package definitions

import (
	"context"
	"os"

	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func ReadFile() tools.Definition {
	return tools.Definition{
		Name: "readFile", Description: "Read a UTF-8 text file from the workspace.", Parameters: pathSchema(true),
		Execute: func(_ context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			workspace := toolContext.Workspace
			name, err := stringArg(args, "path")
			if err != nil {
				return nil, err
			}
			file, err := workspace.ResolveRead(name)
			if err != nil {
				return nil, err
			}
			content, err := os.ReadFile(file)
			return string(content), err
		},
	}
}
