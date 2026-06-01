package commands

import "fmt"

type Action string

const (
	NewSession  Action = "new_session"
	Verbose     Action = "verbose"
	Permissions Action = "permissions"
	Exit        Action = "exit"
)

type Command struct {
	Name        string
	Description string
	Action      Action
}

type Registry struct {
	commands map[string]Command
	order    []Command
}

func New(items []Command) (*Registry, error) {
	registry := &Registry{commands: map[string]Command{}}
	for _, item := range items {
		if _, exists := registry.commands[item.Name]; exists {
			return nil, fmt.Errorf("duplicate CLI command: %s", item.Name)
		}
		registry.commands[item.Name] = item
		registry.order = append(registry.order, item)
	}
	return registry, nil
}

func Default() *Registry {
	registry, _ := New([]Command{
		{Name: "/new", Description: "Clear chat context and start a new session.", Action: NewSession},
		{Name: "/verbose", Description: "Show or hide execution details.", Action: Verbose},
		{Name: "/permissions", Description: "Choose advanced-tool permission mode.", Action: Permissions},
		{Name: "/exit", Description: "Quit OpenHome.", Action: Exit},
	})
	return registry
}

func (r *Registry) Lookup(input string) (Command, bool) {
	command, ok := r.commands[input]
	return command, ok
}

func (r *Registry) Describe() string {
	result := ""
	for index, command := range r.order {
		if index > 0 {
			result += "\n"
		}
		result += command.Name + ": " + command.Description
	}
	return result
}
