package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
)

type Context struct {
	Workspace   Workspace
	Permissions *permissions.Manager
	HTTPClient  *http.Client
}

type Definition struct {
	Name        string
	Description string
	Parameters  map[string]any
	Advanced    bool
	Execute     func(context.Context, Context, map[string]any) (any, error)
}

type Execution struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	Result any            `json:"result"`
}

type Registry struct {
	context     Context
	definitions map[string]Definition
	names       []string
}

func NewRegistry(workspaceDir string, permissionManager *permissions.Manager, definitions []Definition) (*Registry, error) {
	registry := &Registry{context: Context{Workspace: Workspace{Root: workspaceDir}, Permissions: permissionManager, HTTPClient: http.DefaultClient}, definitions: map[string]Definition{}}
	for _, definition := range definitions {
		if _, ok := registry.definitions[definition.Name]; ok {
			return nil, fmt.Errorf("duplicate tool name: %s", definition.Name)
		}
		registry.definitions[definition.Name] = definition
		registry.names = append(registry.names, definition.Name)
	}
	return registry, nil
}

func (r *Registry) Names() []string {
	return append([]string(nil), r.names...)
}

func (r *Registry) Permissions() *permissions.Manager {
	return r.context.Permissions
}

func (r *Registry) OpenAITools(allowed []string) ([]openai.Tool, error) {
	result := make([]openai.Tool, 0, len(allowed))
	for _, name := range allowed {
		definition, ok := r.definitions[name]
		if !ok {
			return nil, fmt.Errorf("tool %s is unavailable", name)
		}
		result = append(result, openai.Tool{Type: "function", Function: openai.FunctionDefinition{
			Name: definition.Name, Description: definition.Description, Parameters: definition.Parameters,
		}})
	}
	return result, nil
}

func (r *Registry) Execute(ctx context.Context, name, arguments string) (Execution, error) {
	definition, ok := r.definitions[name]
	if !ok {
		return Execution{}, fmt.Errorf("tool %s is unavailable", name)
	}
	args := map[string]any{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return Execution{}, fmt.Errorf("decode %s arguments: %w", name, err)
		}
	}
	if _, ok := args["path"]; !ok {
		if alias, ok := args["file_path"]; ok {
			args["path"] = alias
		}
	}
	result, err := definition.Execute(ctx, r.context, args)
	if err != nil {
		return Execution{}, err
	}
	return Execution{Tool: name, Args: args, Result: result}, nil
}

func (r *Registry) AllOpenAITools() ([]openai.Tool, error) {
	return r.OpenAITools(r.Names())
}
