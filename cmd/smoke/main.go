package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/CaowardlyLion/OpenHome/internal/agents"
	"github.com/CaowardlyLion/OpenHome/internal/config"
	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := openai.New(cfg.OpenAIBaseURL, cfg.OpenAIModel, cfg.OpenAIAPIKey)
	if err := client.Models(ctx); err != nil {
		fail(err)
	}
	var route agents.RouteDecision
	if err := client.Structured(ctx, agents.MainSystem, nil, agents.RoutingPrompt("What is 2 + 2?"), "route_decision", agents.RouteSchema(), &route); err != nil {
		fail(err)
	}
	message, err := client.Complete(ctx, agents.MainSystem, []openai.Message{{Role: "user", Content: `Call writeFile with {"path":"smoke.md","content":"ok"}.`}}, []openai.Tool{{
		Type: "function", Function: openai.FunctionDefinition{Name: "writeFile", Description: "Write a file.", Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"path", "content"},
			"properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}},
		}},
	}})
	if err != nil {
		fail(err)
	}
	if len(message.ToolCalls) == 0 {
		fail(fmt.Errorf("model did not return a native tool call"))
	}
	fmt.Printf("oMLX smoke passed: route=%s native_tool=%s\n", route.Lane, message.ToolCalls[0].Function.Name)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "smoke:", err)
	os.Exit(1)
}
