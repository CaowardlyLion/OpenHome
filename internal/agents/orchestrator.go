package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/CaowardlyLion/OpenHome/internal/config"
	runlog "github.com/CaowardlyLion/OpenHome/internal/logging"
	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
	"github.com/CaowardlyLion/OpenHome/internal/session"
	"github.com/CaowardlyLion/OpenHome/internal/skills"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

type Provider interface {
	Structured(context.Context, string, []openai.Message, string, string, map[string]any, any) error
	Complete(context.Context, string, []openai.Message, []openai.Tool) (openai.Message, error)
	Stream(context.Context, string, []openai.Message, string, func(string)) (string, error)
}

type Result struct {
	Answer  string
	LogPath string
}

type Orchestrator struct {
	client  Provider
	catalog skills.Catalog
	config  config.Config
	tools   *tools.Registry
	status  func(StatusEvent)
	session *session.Session
}

const noSkillName = "none"

func NewOrchestrator(client Provider, catalog skills.Catalog, cfg config.Config, registry *tools.Registry, status func(StatusEvent)) *Orchestrator {
	if status == nil {
		status = func(StatusEvent) {}
	}
	return &Orchestrator{client: client, catalog: catalog, config: cfg, tools: registry, status: status, session: session.New(cfg.MaxContextBytes)}
}

func (o *Orchestrator) NewSession() {
	o.session.Clear()
}

func (o *Orchestrator) Handle(ctx context.Context, userMessage string) (result Result, err error) {
	logger := runlog.NewRunLogger(o.config.RunsDir)
	if manager := o.tools.Permissions(); manager != nil {
		manager.SetAudit(func(event string, data any) { _ = logger.Append(event, data) })
		defer manager.SetAudit(nil)
	}
	result.LogPath = logger.FilePath
	_ = logger.Append("user_message", map[string]any{"content": userMessage})
	history := o.session.History()
	run := []openai.Message{{Role: "user", Content: userMessage}}
	defer func() {
		if err != nil {
			event := "failed"
			if errors.Is(ctx.Err(), context.Canceled) {
				event = "cancelled"
			}
			_ = logger.Append(event, map[string]any{"error": err.Error()})
			if len(run) > 1 {
				o.session.Append(run...)
			}
		}
	}()

	o.emit("thinking", "Understanding request")
	var route RouteDecision
	if err = o.client.Structured(ctx, MainSystem, history, RoutingPrompt(userMessage), "route_decision", RouteSchema(), &route); err != nil {
		return result, err
	}
	if err = validateRoute(route); err != nil {
		return result, err
	}
	if route.Intent == string(route.Lane) {
		route.Intent = strings.TrimSpace(userMessage)
	}
	o.emit("intent", route.Intent)
	o.emit("lane", string(route.Lane))
	_ = logger.Append("intent", map[string]any{"summary": route.Intent})
	_ = logger.Append("route", route)
	run = appendEvent(run, "intent", map[string]any{"summary": route.Intent})
	run = appendEvent(run, "route", route)

	if route.Lane == DirectAnswer {
		o.emit("answering", "Streaming answer")
		var answer string
		answer, err = o.client.Stream(ctx, MainSystem, history, userMessage, func(token string) {
			o.emit("token", token)
		})
		if err != nil {
			return result, err
		}
		run = append(run, openai.Message{Role: "assistant", Content: answer})
		report, reportErr := o.complete(ctx, userMessage, history, run, logger)
		if reportErr != nil {
			return result, reportErr
		}
		run = appendEvent(run, "completion_report", report)
		o.session.Append(run...)
		_ = logger.Append("completed", map[string]any{"answer": answer, "report": report})
		result.Answer = answer
		return result, nil
	}

	var steps []Step
	if route.Lane == SimpleTask {
		steps = []Step{{ID: "1", Goal: route.Intent, SuccessCriteria: "The requested narrow outcome is complete."}}
	} else {
		o.emit("planning", "Creating outcome plan")
		var plan Plan
		if err = o.client.Structured(ctx, MainSystem, o.active(history, run), PlanningPrompt(userMessage), "task_plan", PlanSchema(), &plan); err != nil {
			return result, err
		}
		if len(plan.Steps) == 0 {
			return result, fmt.Errorf("planner returned no steps")
		}
		steps = plan.Steps
		_ = logger.Append("plan", plan)
		run = appendEvent(run, "plan", plan)
		o.emit("plan", FormatPlan(plan))
	}

	policy := route.Verification
	var current *skills.Skill
	for index, step := range steps {
		o.emit("step", fmt.Sprintf("Step %d/%d: %s", index+1, len(steps), step.Goal))
		if current == nil {
			current, run, err = o.selectSkill(ctx, history, run, logger, userMessage, step, "")
			if err != nil {
				return result, err
			}
		} else {
			o.emit("skill", "Reusing skill: "+current.Name)
			_ = logger.Append("skill_reused", map[string]any{"stepId": step.ID, "skillName": current.Name})
			run = appendEvent(run, "skill_reused", map[string]any{"stepId": step.ID, "skillName": current.Name})
		}
		for {
			var reselectReason string
			run, policy, reselectReason, err = o.execute(ctx, history, run, logger, userMessage, step, *current, policy)
			if err != nil {
				return result, err
			}
			if reselectReason == "" {
				break
			}
			o.emit("skill", "Reselecting skill: "+reselectReason)
			current, run, err = o.selectSkill(ctx, history, run, logger, userMessage, step, reselectReason)
			if err != nil {
				return result, err
			}
		}
	}

	if policy == VerifyFinal {
		o.emit("verification", "Running final verification")
		var verification Verification
		if err = o.client.Structured(ctx, MainSystem, o.active(history, run), VerificationPrompt(userMessage), "final_verification", VerificationSchema(), &verification); err != nil {
			return result, err
		}
		_ = logger.Append("verification", verification)
		run = appendEvent(run, "verification", verification)
		if verification.Status != "passed" {
			return result, fmt.Errorf("final verification failed: %s", verification.Reason)
		}
	} else {
		o.emit("verification", "Skipped final verification")
		_ = logger.Append("verification_skipped", map[string]any{"policy": policy})
		run = appendEvent(run, "verification_skipped", map[string]any{"policy": policy})
	}

	report, err := o.complete(ctx, userMessage, history, run, logger)
	if err != nil {
		return result, err
	}
	answer := FormatCompletion(report)
	run = appendEvent(run, "completion_report", report)
	run = append(run, openai.Message{Role: "assistant", Content: answer})
	o.session.Append(run...)
	_ = logger.Append("completed", map[string]any{"answer": answer})
	o.emit("report", answer)
	result.Answer = answer
	return result, nil
}

func (o *Orchestrator) selectSkill(ctx context.Context, history, run []openai.Message, logger *runlog.RunLogger, task string, step Step, reason string) (*skills.Skill, []openai.Message, error) {
	prompt := SkillSelectionPrompt(o.catalog.LibrarianContext, task, step, reason)
	var choice SkillChoice
	var skill skills.Skill
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		choice = SkillChoice{}
		if err = o.client.Structured(ctx, LibrarianSystem, o.active(history, run), prompt, "skill_choice", SkillChoiceSchema(), &choice); err != nil {
			return nil, run, err
		}
		if choice.SkillName == noSkillName {
			skill = skills.Skill{Name: noSkillName, Content: "No specialized skill applies. Use the registered tools and general reasoning needed for the outcome."}
			err = nil
		} else {
			skill, err = o.catalog.Find(choice.SkillName)
		}
		if err == nil {
			break
		}
		if attempt == 2 {
			return nil, run, err
		}
		prompt += "\nPrevious response used an empty or unknown skillName. Return exactly one catalog skillName, or \"none\" when no catalog skill applies."
	}
	event := map[string]any{"stepId": step.ID, "skillName": choice.SkillName, "reason": choice.Reason}
	_ = logger.Append("skill_selected", event)
	run = appendEvent(run, "skill_selected", event)
	o.emit("skill", "Using skill: "+skill.Name)
	return &skill, run, nil
}

func (o *Orchestrator) execute(ctx context.Context, history, run []openai.Message, logger *runlog.RunLogger, task string, step Step, skill skills.Skill, policy VerificationPolicy) ([]openai.Message, VerificationPolicy, string, error) {
	nativeTools, err := o.tools.AllOpenAITools()
	if err != nil {
		return run, policy, "", err
	}
	nativeTools = append(nativeTools, controlTools()...)
	successful := map[string]bool{}
	successfulTools := map[string]bool{}
	visitedBrowserURLs := map[string]bool{}
	pendingNavigationURL := ""
	messages := o.active(history, run)
	observedWorkspace := hasWorkspaceToolEvidence(messages)
	messages = append(messages, openai.Message{Role: "user", Content: ExecutionPrompt(task, step, skill)})
	messages, _ = session.TrimMessages(messages, o.config.MaxContextBytes)
	for round := 1; round <= o.config.MaxToolRounds; round++ {
		assistant, err := o.client.Complete(ctx, MainSystem, messages, nativeTools)
		if err != nil {
			return run, policy, "", err
		}
		if assistant.Role == "" {
			assistant.Role = "assistant"
		}
		messages = append(messages, assistant)
		run = append(run, assistant)
		_ = logger.Append("native_assistant_message", map[string]any{"stepId": step.ID, "round": round, "message": assistant})
		if len(assistant.ToolCalls) == 0 {
			if pendingNavigationURL != "" {
				correction := openai.Message{Role: "user", Content: "Browser navigation is incomplete. The opened page exposed a navigation destination matching the user's requested resource. Open it with a read-only browserInteract call before answering: " + pendingNavigationURL}
				messages = append(messages, correction)
				run = append(run, correction)
				_ = logger.Append("execution_retry", map[string]any{"stepId": step.ID, "round": round, "reason": correction.Content})
				o.emit("retry", correction.Content)
				continue
			}
			if requiresOpenedWebSource(skill.Name, successfulTools) {
				correction := openai.Message{Role: "user", Content: "Web research is incomplete. Search snippets are discovery hints only. Refine the search if needed, then open relevant source pages with fetchURL or browserInteract before answering. Do not ask the user whether to continue."}
				messages = append(messages, correction)
				run = append(run, correction)
				_ = logger.Append("execution_retry", map[string]any{"stepId": step.ID, "round": round, "reason": correction.Content})
				o.emit("retry", correction.Content)
				continue
			}
			if !observedWorkspace {
				correction := openai.Message{Role: "user", Content: "No observed tool evidence exists yet. Use an allowed tool to inspect workspace state before completing the outcome or saying information is missing."}
				messages = append(messages, correction)
				run = append(run, correction)
				_ = logger.Append("execution_retry", map[string]any{"stepId": step.ID, "round": round, "reason": correction.Content})
				o.emit("retry", correction.Content)
				continue
			}
			return run, policy, "", nil
		}
		for _, call := range assistant.ToolCalls {
			o.emit("tool", call.Function.Name)
			if call.Function.Name == "request_skill_reselection" {
				reason := reasonArg(call.Function.Arguments)
				toolMessage := toolResult(call.ID, map[string]any{"accepted": true, "reason": reason})
				run = append(run, toolMessage)
				return run, policy, reason, nil
			}
			if call.Function.Name == "request_verification" {
				reason := reasonArg(call.Function.Arguments)
				policy = VerifyFinal
				o.emit("verification", "Final verification requested: "+reason)
				toolMessage := toolResult(call.ID, map[string]any{"accepted": true, "policy": policy})
				messages = append(messages, toolMessage)
				run = append(run, toolMessage)
				_ = logger.Append("verification_requested", map[string]any{"stepId": step.ID, "reason": reason})
				continue
			}
			key := call.Function.Name + "\x00" + call.Function.Arguments
			if successful[key] {
				rejection := map[string]any{"rejected": true, "error": "Duplicate tool request rejected; reuse the prior successful result."}
				toolMessage := toolResult(call.ID, rejection)
				messages = append(messages, toolMessage)
				run = append(run, toolMessage)
				_ = logger.Append("tool_rejected", map[string]any{"stepId": step.ID, "round": round, "request": call, "result": rejection})
				o.emit("retry", rejection["error"].(string))
				continue
			}
			execution, executeErr := o.tools.Execute(ctx, call.Function.Name, call.Function.Arguments)
			var output any = execution
			if executeErr != nil {
				output = map[string]any{"rejected": true, "error": executeErr.Error()}
				o.emit("retry", executeErr.Error())
				_ = logger.Append("tool_rejected", map[string]any{"stepId": step.ID, "round": round, "request": call, "result": output})
			} else {
				successful[key] = true
				successfulTools[call.Function.Name] = true
				observedWorkspace = true
				if call.Function.Name == "browserInteract" {
					if result, ok := execution.Result.(map[string]any); ok {
						visitedBrowserURLs[normalizedURL(stringValue(result["url"]))] = true
						if destination := requestedNavigationDestination(task, result); destination != "" && !visitedBrowserURLs[normalizedURL(destination)] {
							pendingNavigationURL = destination
						} else if visitedBrowserURLs[normalizedURL(pendingNavigationURL)] {
							pendingNavigationURL = ""
						}
					}
				}
				_ = logger.Append("tool_result", map[string]any{"stepId": step.ID, "round": round, "request": call, "result": execution})
			}
			toolMessage := toolResult(call.ID, output)
			messages = append(messages, toolMessage)
			run = append(run, toolMessage)
		}
		messages, _ = session.TrimMessages(messages, o.config.MaxContextBytes)
	}
	return run, policy, "", fmt.Errorf("outcome exceeded %d tool rounds: %s", o.config.MaxToolRounds, step.Goal)
}

func requestedNavigationDestination(task string, result map[string]any) string {
	current := normalizedURL(stringValue(result["url"]))
	task = strings.ToLower(task)
	var best string
	var bestLabel string
	links, _ := result["navigationLinks"].([]any)
	for _, value := range links {
		link, ok := value.(map[string]any)
		if !ok {
			continue
		}
		label := strings.TrimSpace(stringValue(link["text"]))
		target := strings.TrimSpace(stringValue(link["url"]))
		if len(label) < 4 || target == "" || normalizedURL(target) == current || !strings.Contains(task, strings.ToLower(label)) {
			continue
		}
		if len(label) > len(bestLabel) {
			bestLabel, best = label, target
		}
	}
	return best
}

func normalizedURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func requiresOpenedWebSource(skillName string, successfulTools map[string]bool) bool {
	return skillName == "web-search" && successfulTools["webSearch"] && !successfulTools["fetchURL"] && !successfulTools["browserInteract"]
}

func (o *Orchestrator) complete(ctx context.Context, task string, history, run []openai.Message, logger *runlog.RunLogger) (CompletionReport, error) {
	o.emit("finishing", "Preparing completion report")
	var report CompletionReport
	err := o.client.Structured(ctx, MainSystem, o.active(history, run), CompletionPrompt(task), "completion_report", CompletionSchema(), &report)
	if err == nil {
		_ = logger.Append("completion_report", report)
	}
	return report, err
}

func controlTools() []openai.Tool {
	parameters := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"reason"}, "properties": map[string]any{"reason": map[string]any{"type": "string"}}}
	return []openai.Tool{
		{Type: "function", Function: openai.FunctionDefinition{Name: "request_skill_reselection", Description: "Ask the librarian to select a different skill when the current playbook cannot handle this outcome.", Parameters: parameters}},
		{Type: "function", Function: openai.FunctionDefinition{Name: "request_verification", Description: "Escalate this task to one final verification pass when mutation risk or uncertain evidence warrants it.", Parameters: parameters}},
	}
}

func reasonArg(arguments string) string {
	var parsed struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal([]byte(arguments), &parsed) != nil || parsed.Reason == "" {
		return "Executor requested orchestration control."
	}
	return parsed.Reason
}

func toolResult(id string, value any) openai.Message {
	content, _ := json.Marshal(value)
	return openai.Message{Role: "tool", ToolCallID: id, Content: string(content)}
}

func hasWorkspaceToolEvidence(messages []openai.Message) bool {
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			if call.Function.Name != "request_skill_reselection" && call.Function.Name != "request_verification" {
				return true
			}
		}
	}
	return false
}

func (o *Orchestrator) active(history, run []openai.Message) []openai.Message {
	result := append([]openai.Message(nil), history...)
	result = append(result, run...)
	result, _ = session.TrimMessages(result, o.config.MaxContextBytes)
	return result
}

func appendEvent(messages []openai.Message, eventType string, data any) []openai.Message {
	content, _ := json.Marshal(data)
	return append(messages, openai.Message{Role: "assistant", Content: "[execution:" + eventType + "] " + string(content)})
}

func validateRoute(route RouteDecision) error {
	if route.Intent == "" {
		return fmt.Errorf("router returned empty intent")
	}
	if route.Lane != DirectAnswer && route.Lane != SimpleTask && route.Lane != PlannedTask {
		return fmt.Errorf("router returned invalid lane: %s", route.Lane)
	}
	if route.Verification != VerifyNone && route.Verification != VerifyFinal {
		return fmt.Errorf("router returned invalid verification policy: %s", route.Verification)
	}
	return nil
}

func FormatPlan(plan Plan) string {
	var lines []string
	lines = append(lines, plan.Summary)
	for index, step := range plan.Steps {
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, step.Goal))
	}
	return strings.Join(lines, "\n")
}

func FormatCompletion(report CompletionReport) string {
	lines := []string{report.Summary}
	for _, detail := range report.Details {
		lines = append(lines, "- "+detail)
	}
	return strings.Join(lines, "\n")
}

func (o *Orchestrator) emit(eventType, message string) {
	o.status(StatusEvent{Type: eventType, Message: message})
}
