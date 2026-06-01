package definitions

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func SearchFiles() tools.Definition {
	return tools.Definition{
		Name: "searchFiles", Description: "Search workspace text files for matching lines.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"query"},
			"properties": map[string]any{"query": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}},
		},
		Execute: func(workspace tools.Workspace, args map[string]any) (any, error) {
			query, err := stringArg(args, "query")
			if err != nil {
				return nil, err
			}
			name := "."
			if value, ok := args["path"]; ok {
				name, _ = value.(string)
			}
			root, err := workspace.ResolveRead(name)
			if err != nil {
				return nil, err
			}
			return search(root, workspace.Root, strings.ToLower(query))
		},
	}
}

func search(root, workspaceRoot, query string) ([]map[string]any, error) {
	var matches []map[string]any
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			if strings.Contains(strings.ToLower(text), query) {
				relative, _ := filepath.Rel(workspaceRoot, name)
				matches = append(matches, map[string]any{"path": relative, "line": line, "text": text})
			}
		}
		return scanner.Err()
	})
	return matches, err
}
