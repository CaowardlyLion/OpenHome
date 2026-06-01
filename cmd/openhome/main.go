package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/CaowardlyLion/OpenHome/internal/agents"
	"github.com/CaowardlyLion/OpenHome/internal/app"
	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		runHeadless(strings.Join(os.Args[2:], " "))
		return
	}
	var program *tea.Program
	prompter := permissions.NewChannelPrompter()
	application, err := app.New(func(event agents.StatusEvent) {
		if program != nil {
			program.Send(tui.StatusMsg(event))
		}
	}, prompter.Prompt)
	if err != nil {
		fail(err)
	}
	program = tea.NewProgram(tui.New(application.Orchestrator, application.Permissions, prompter))
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
	}, terminalPrompt)
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

func terminalPrompt(ctx context.Context, request permissions.Request) permissions.Decision {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return permissions.Deny
	}
	fmt.Printf("\nPermission required: %s\nReason: %s\nTarget: %s\n%s\n[1] Allow once  [2] Always allow similar  [3] Deny\n> ",
		request.Tool, request.Reason, request.Target, request.Details)
	answer := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer <- strings.TrimSpace(line)
	}()
	select {
	case value := <-answer:
		switch value {
		case "1", "a", "allow":
			return permissions.AllowOnce
		case "2", "s", "always":
			return permissions.AllowSimilar
		default:
			return permissions.Deny
		}
	case <-ctx.Done():
		return permissions.Deny
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "openhome:", err)
	os.Exit(1)
}
