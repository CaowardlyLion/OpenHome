package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CaowardlyLion/OpenHome/internal/agents"
	"github.com/CaowardlyLion/OpenHome/internal/app"
	"github.com/CaowardlyLion/OpenHome/internal/tui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		runHeadless(strings.Join(os.Args[2:], " "))
		return
	}
	var program *tea.Program
	application, err := app.New(func(event agents.StatusEvent) {
		if program != nil {
			program.Send(tui.StatusMsg(event))
		}
	})
	if err != nil {
		fail(err)
	}
	program = tea.NewProgram(tui.New(application.Orchestrator))
	if _, err := program.Run(); err != nil {
		fail(err)
	}
}

func runHeadless(input string) {
	if strings.TrimSpace(input) == "" {
		fail(fmt.Errorf("usage: openhome run \"request\""))
	}
	application, err := app.New(func(event agents.StatusEvent) {
		if event.Type != "token" {
			fmt.Printf("[%s] %s\n", event.Type, event.Message)
		}
	})
	if err != nil {
		fail(err)
	}
	result, err := application.Orchestrator.Handle(context.Background(), input)
	if err != nil {
		fail(err)
	}
	fmt.Println(result.Answer)
	fmt.Printf("[log] %s\n", result.LogPath)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "openhome:", err)
	os.Exit(1)
}
