package app

import (
	"github.com/CaowardlyLion/OpenHome/internal/agents"
	"github.com/CaowardlyLion/OpenHome/internal/config"
	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
	"github.com/CaowardlyLion/OpenHome/internal/skills"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
	"github.com/CaowardlyLion/OpenHome/internal/tools/definitions"
)

type App struct {
	Config       config.Config
	Client       *openai.Client
	Orchestrator *agents.Orchestrator
}

func New(status func(agents.StatusEvent)) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	registry, err := tools.NewRegistry(cfg.WorkspaceDir, definitions.All())
	if err != nil {
		return nil, err
	}
	catalog, err := skills.Load(cfg.SkillsDir, registry.Names())
	if err != nil {
		return nil, err
	}
	client := openai.New(cfg.OpenAIBaseURL, cfg.OpenAIModel, cfg.OpenAIAPIKey)
	return &App{Config: cfg, Client: client, Orchestrator: agents.NewOrchestrator(client, catalog, cfg, registry, status)}, nil
}
