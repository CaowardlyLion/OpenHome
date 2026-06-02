package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CaowardlyLion/OpenHome/internal/config"
	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
	"github.com/CaowardlyLion/OpenHome/internal/skills"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
	"github.com/CaowardlyLion/OpenHome/internal/tools/definitions"
)

type structuredReply struct {
	name  string
	value any
}

type fakeProvider struct {
	structured []structuredReply
	completes  []openai.Message
	streams    []string
	requests   []request
}

type request struct {
	kind    string
	name    string
	history []openai.Message
	tools   []openai.Tool
	prompt  string
}

func (f *fakeProvider) Structured(_ context.Context, _ string, history []openai.Message, prompt, name string, _ map[string]any, out any) error {
	f.requests = append(f.requests, request{kind: "structured", name: name, history: history, prompt: prompt})
	reply := f.structured[0]
	f.structured = f.structured[1:]
	if reply.name != name {
		panic("structured reply mismatch: got " + name + ", want " + reply.name)
	}
	content, _ := json.Marshal(reply.value)
	return json.Unmarshal(content, out)
}

func (f *fakeProvider) Complete(_ context.Context, _ string, messages []openai.Message, tools []openai.Tool) (openai.Message, error) {
	f.requests = append(f.requests, request{kind: "complete", history: messages, tools: tools})
	reply := f.completes[0]
	f.completes = f.completes[1:]
	return reply, nil
}

func (f *fakeProvider) Stream(_ context.Context, _ string, history []openai.Message, prompt string, onToken func(string)) (string, error) {
	f.requests = append(f.requests, request{kind: "stream", history: history, prompt: prompt})
	reply := f.streams[0]
	f.streams = f.streams[1:]
	onToken(reply)
	return reply, nil
}

func fixture(t *testing.T, provider *fakeProvider) (*Orchestrator, config.Config) {
	t.Helper()
	root := t.TempDir()
	cfg := config.Config{WorkspaceDir: filepath.Join(root, "workspace"), RunsDir: filepath.Join(root, "runs"), MaxToolRounds: 6}
	registry, err := tools.NewRegistry(cfg.WorkspaceDir, nil, definitions.All())
	if err != nil {
		t.Fatal(err)
	}
	catalog := skills.Catalog{Skills: []skills.Skill{
		{Name: "grocery-list", Description: "Groceries", AllowedTools: []string{"readFile", "writeFile"}, Content: "Update groceries."},
		{Name: "task-planning", Description: "Tasks", AllowedTools: []string{"writeFile"}, Content: "Write tasks."},
	}, LibrarianContext: "grocery-list\ntask-planning"}
	return NewOrchestrator(provider, catalog, cfg, registry, nil), cfg
}

func call(id, name, arguments string) openai.ToolCall {
	return openai.ToolCall{ID: id, Type: "function", Function: openai.FunctionCall{Name: name, Arguments: arguments}}
}

func TestDirectAnswerStreamsAndSynthesizesCompletion(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Answer greeting.", Lane: DirectAnswer, Verification: VerifyNone, Reason: "question"}},
			{"completion_report", CompletionReport{Summary: "Answered greeting."}},
		},
		streams: []string{"Hello"},
	}
	orchestrator, _ := fixture(t, provider)
	result, err := orchestrator.Handle(context.Background(), "Say hello")
	if err != nil || result.Answer != "Hello" {
		t.Fatalf("result = %#v, %v", result, err)
	}
	if len(provider.requests) != 3 || provider.requests[2].name != "completion_report" {
		t.Fatalf("requests = %#v", provider.requests)
	}
}

func TestCompletionPromptSaysHiddenTraceWasNotShownToUser(t *testing.T) {
	prompt := CompletionPrompt("suggest a recipe")
	for _, expected := range []string{
		"The user has not seen prior executor replies, tool calls, tool outputs, or trace messages.",
		"includes that content directly",
		"include the actual useful content",
		"Every factual URL, headline, quote, and current claim must be grounded",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q: %s", expected, prompt)
		}
	}
}

func TestWebSearchSkillRequiresOpenedSourceBeforeCompletion(t *testing.T) {
	if !requiresOpenedWebSource("web-search", map[string]bool{"webSearch": true}) {
		t.Fatal("web-search completed from snippets without opening source")
	}
	if requiresOpenedWebSource("web-search", map[string]bool{"webSearch": true, "fetchURL": true}) {
		t.Fatal("fetchURL did not satisfy opened-source requirement")
	}
	if requiresOpenedWebSource("web-search", map[string]bool{"webSearch": true, "browserInteract": true}) {
		t.Fatal("browserInteract did not satisfy opened-source requirement")
	}
	if requiresOpenedWebSource("grocery-list", map[string]bool{"webSearch": true}) {
		t.Fatal("non-web skill unexpectedly requires opened source")
	}
}

func TestExecutionPromptFollowsSkillReselectionToolGuidance(t *testing.T) {
	prompt := ExecutionPrompt("research site", Step{ID: "1", Goal: "Research site"}, skills.Skill{Name: "web-search"})
	if !strings.Contains(prompt, "When a tool result recommends request_skill_reselection") {
		t.Fatalf("prompt = %q", prompt)
	}
	if !strings.Contains(prompt, "navigationLinks destination matching the user's requested") {
		t.Fatalf("prompt = %q", prompt)
	}
	if !strings.Contains(prompt, "observed evidence should be checked") {
		t.Fatalf("prompt = %q", prompt)
	}
}

func TestRequestedNavigationDestinationMatchesRequestedSection(t *testing.T) {
	result := map[string]any{
		"url": "https://example.com/",
		"navigationLinks": []any{
			map[string]any{"text": "World", "url": "https://example.com/world/"},
			map[string]any{"text": "Technology", "url": "https://example.com/technology/"},
		},
	}
	if destination := requestedNavigationDestination("show technology headlines", result); destination != "https://example.com/technology/" {
		t.Fatalf("destination = %q", destination)
	}
	result["url"] = "https://example.com/technology/"
	if destination := requestedNavigationDestination("show technology headlines", result); destination != "" {
		t.Fatalf("destination = %q", destination)
	}
}

func TestActiveContextDropsOldestMessagesWhenBudgetExceeded(t *testing.T) {
	recent := openai.Message{Role: "assistant", Content: "recent"}
	content, _ := json.Marshal([]openai.Message{recent})
	orchestrator := &Orchestrator{config: config.Config{MaxContextBytes: len(content) + 1}}
	active := orchestrator.active(
		[]openai.Message{{Role: "user", Content: "old request that should drop"}},
		[]openai.Message{recent},
	)
	if len(active) != 1 || active[0].Content != recent.Content {
		t.Fatalf("active = %#v", active)
	}
}

func TestRoutingPromptRequestsOperationalRationale(t *testing.T) {
	prompt := RoutingPrompt("could you add milk?")
	for _, expected := range []string{"concise operational rationale", "locate the appropriate list and update it", "Do not mention internal lanes"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q: %s", expected, prompt)
		}
	}
}

func TestSimpleTaskUsesNativeToolsWithoutPlan(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Add spinach.", Lane: SimpleTask, Verification: VerifyNone, Reason: "narrow"}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "fit"}},
			{"completion_report", CompletionReport{Summary: "Grocery list updated.", Details: []string{"Added spinach."}}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "writeFile", `{"path":"groceries.md","content":"- spinach"}`)}},
			{Role: "assistant", Content: "Added spinach."},
		},
	}
	orchestrator, cfg := fixture(t, provider)
	result, err := orchestrator.Handle(context.Background(), "add spinach")
	if err != nil {
		t.Fatal(err)
	}
	if result.Answer != "Grocery list updated.\n- Added spinach." {
		t.Fatalf("answer = %q", result.Answer)
	}
	content, _ := os.ReadFile(filepath.Join(cfg.WorkspaceDir, "groceries.md"))
	if string(content) != "- spinach" {
		t.Fatalf("content = %q", content)
	}
	for _, request := range provider.requests {
		if request.name == "task_plan" {
			t.Fatal("simple task unexpectedly planned")
		}
	}
	complete := provider.requests[2]
	if !hasTool(complete.tools, "writeFile") || !hasTool(complete.tools, "request_skill_reselection") || !hasTool(complete.tools, "request_verification") {
		t.Fatalf("tools = %#v", complete.tools)
	}
	if !hasTool(complete.tools, "runCommand") || !hasTool(complete.tools, "webSearch") || !hasTool(complete.tools, "readExternalFile") {
		t.Fatalf("advanced tools not exposed globally: %#v", complete.tools)
	}
}

func TestWorkspaceQuestionSelectsSkillAndReadsFile(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Show current grocery list.", Lane: SimpleTask, Verification: VerifyNone, Reason: "workspace state"}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "reads grocery list"}},
			{"final_verification", Verification{Status: "passed", Reason: "read file supports answer"}},
			{"completion_report", CompletionReport{Summary: "Current grocery list:", Details: []string{"milk", "spinach"}}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "readFile", `{"path":"grocery_list.md"}`)}},
			{Role: "assistant", Content: "The grocery list contains milk and spinach."},
		},
	}
	orchestrator, cfg := fixture(t, provider)
	if err := os.MkdirAll(cfg.WorkspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.WorkspaceDir, "grocery_list.md"), []byte("- milk\n- spinach"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.Handle(context.Background(), "what is in my grocery list?")
	if err != nil {
		t.Fatal(err)
	}
	if result.Answer != "Current grocery list:\n- milk\n- spinach" {
		t.Fatalf("answer = %q", result.Answer)
	}
	for _, request := range provider.requests {
		if request.kind == "stream" || request.name == "task_plan" {
			t.Fatalf("workspace question used wrong path: %#v", request)
		}
	}
}

func TestLibrarianRetriesEmptySkillChoice(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Show current grocery list.", Lane: SimpleTask, Verification: VerifyNone, Reason: "workspace state"}},
			{"skill_choice", SkillChoice{}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "corrected"}},
			{"completion_report", CompletionReport{Summary: "List read."}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "writeFile", `{"path":"list.md","content":"read"}`)}},
			{Role: "assistant", Content: "List read."},
		},
	}
	orchestrator, _ := fixture(t, provider)
	if _, err := orchestrator.Handle(context.Background(), "what is in my grocery list?"); err != nil {
		t.Fatal(err)
	}
	skillChoices := 0
	for _, request := range provider.requests {
		if request.name == "skill_choice" {
			skillChoices++
		}
	}
	if skillChoices != 2 {
		t.Fatalf("skill choices = %d", skillChoices)
	}
}

func TestLibrarianAcceptsNoApplicableSkill(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Write a generic note.", Lane: SimpleTask, Verification: VerifyNone, Reason: "narrow"}},
			{"skill_choice", SkillChoice{SkillName: "none", Reason: "No catalog skill applies."}},
			{"completion_report", CompletionReport{Summary: "Note written."}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "writeFile", `{"path":"note.md","content":"hello"}`)}},
			{Role: "assistant", Content: "Note written."},
		},
	}
	orchestrator, cfg := fixture(t, provider)
	if _, err := orchestrator.Handle(context.Background(), "write a generic note"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(cfg.WorkspaceDir, "note.md"))
	if err != nil || string(content) != "hello" {
		t.Fatalf("content = %q, %v", content, err)
	}
	skillChoices := 0
	for _, request := range provider.requests {
		if request.name == "skill_choice" {
			skillChoices++
		}
	}
	if skillChoices != 1 {
		t.Fatalf("skill choices = %d", skillChoices)
	}
	if !containsText(provider.requests[2].history, "Selected skill: none") {
		t.Fatalf("executor history = %#v", provider.requests[2].history)
	}
}

func TestExecutorRetriesUnsupportedMissingInformationClaim(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Show current grocery list.", Lane: SimpleTask, Verification: VerifyNone, Reason: "workspace state"}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "reads list"}},
			{"final_verification", Verification{Status: "passed", Reason: "read file supports answer"}},
			{"completion_report", CompletionReport{Summary: "Current grocery list:", Details: []string{"milk"}}},
		},
		completes: []openai.Message{
			{Role: "assistant", Content: "No grocery list found."},
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "readFile", `{"path":"grocery_list.md"}`)}},
			{Role: "assistant", Content: "The grocery list contains milk."},
		},
	}
	orchestrator, cfg := fixture(t, provider)
	if err := os.MkdirAll(cfg.WorkspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.WorkspaceDir, "grocery_list.md"), []byte("- milk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Handle(context.Background(), "what is in my grocery list?"); err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) < 4 || !containsText(provider.requests[3].history, "No observed tool evidence exists yet") {
		t.Fatalf("requests = %#v", provider.requests)
	}
}

func TestSkillChoiceAcceptsCommonModelKeyVariants(t *testing.T) {
	for _, content := range []string{
		`{"skillName":"grocery-list","reason":"camel"}`,
		`{"skill_name":"grocery-list","reason":"snake"}`,
		`{"name":"grocery-list","reason":"short"}`,
		`{"skill_choice":{"skillName":"grocery-list","reason":"wrapped"}}`,
	} {
		var choice SkillChoice
		if err := json.Unmarshal([]byte(content), &choice); err != nil {
			t.Fatal(err)
		}
		if choice.SkillName != "grocery-list" {
			t.Fatalf("content = %s, choice = %#v", content, choice)
		}
	}
}

func TestPlannedTaskReusesSkillAndRunsFinalVerification(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Create and confirm groceries.", Lane: PlannedTask, Verification: VerifyFinal, Reason: "multiple outcomes"}},
			{"task_plan", Plan{Summary: "Groceries", Steps: []Step{{ID: "1", Goal: "Create list.", SuccessCriteria: "exists"}, {ID: "2", Goal: "Confirm list.", SuccessCriteria: "confirmed"}}}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "fit"}},
			{"final_verification", Verification{Status: "passed", Reason: "observed"}},
			{"completion_report", CompletionReport{Summary: "Groceries complete."}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "writeFile", `{"path":"groceries.md","content":"- milk"}`)}},
			{Role: "assistant", Content: "Created."},
			{Role: "assistant", Content: "Confirmed from prior evidence."},
		},
	}
	orchestrator, _ := fixture(t, provider)
	if _, err := orchestrator.Handle(context.Background(), "create and confirm groceries"); err != nil {
		t.Fatal(err)
	}
	skillChoices := 0
	for _, request := range provider.requests {
		if request.name == "skill_choice" {
			skillChoices++
		}
	}
	if skillChoices != 1 {
		t.Fatalf("skill selections = %d", skillChoices)
	}
}

func TestExecutorCanEscalateVerificationAndReselectSkill(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Write tasks.", Lane: SimpleTask, Verification: VerifyNone, Reason: "narrow"}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "initial"}},
			{"skill_choice", SkillChoice{SkillName: "task-planning", Reason: "reselected"}},
			{"final_verification", Verification{Status: "passed", Reason: "observed"}},
			{"completion_report", CompletionReport{Summary: "Tasks written."}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "listFiles", `{}`)}},
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("2", "request_skill_reselection", `{"reason":"Need task planning."}`)}},
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("3", "request_verification", `{"reason":"Confirm mutation."}`), call("4", "writeFile", `{"path":"tasks.md","content":"- done"}`)}},
			{Role: "assistant", Content: "Tasks written."},
		},
	}
	orchestrator, _ := fixture(t, provider)
	if _, err := orchestrator.Handle(context.Background(), "write tasks"); err != nil {
		t.Fatal(err)
	}
}

func TestPrematureSkillReselectionIsRejectedUntilEvidenceExists(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Inspect files.", Lane: SimpleTask, Verification: VerifyNone, Reason: "narrow"}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "initial"}},
			{"final_verification", Verification{Status: "passed", Reason: "listFiles observed workspace"}},
			{"completion_report", CompletionReport{Summary: "Files inspected."}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "request_skill_reselection", `{"reason":"Different skill maybe."}`)}},
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("2", "listFiles", `{}`)}},
			{Role: "assistant", Content: "Files inspected."},
		},
	}
	orchestrator, _ := fixture(t, provider)
	if _, err := orchestrator.Handle(context.Background(), "inspect files"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, request := range provider.requests {
		if containsText(request.history, "Skill reselection is premature") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("requests = %#v", provider.requests)
	}
}

func TestNativeMessagesRemainInFollowupAndNewSessionClearsThem(t *testing.T) {
	provider := &fakeProvider{
		structured: []structuredReply{
			{"route_decision", RouteDecision{Intent: "Add milk.", Lane: SimpleTask, Verification: VerifyNone, Reason: "narrow"}},
			{"skill_choice", SkillChoice{SkillName: "grocery-list", Reason: "fit"}},
			{"completion_report", CompletionReport{Summary: "Milk added."}},
			{"route_decision", RouteDecision{Intent: "Explain changes.", Lane: DirectAnswer, Verification: VerifyNone, Reason: "follow-up"}},
			{"completion_report", CompletionReport{Summary: "Explained."}},
			{"route_decision", RouteDecision{Intent: "Fresh.", Lane: DirectAnswer, Verification: VerifyNone, Reason: "fresh"}},
			{"completion_report", CompletionReport{Summary: "Fresh."}},
		},
		completes: []openai.Message{
			{Role: "assistant", ToolCalls: []openai.ToolCall{call("1", "writeFile", `{"path":"groceries.md","content":"- milk"}`)}},
			{Role: "assistant", Content: "Done."},
		},
		streams: []string{"Changed groceries.", "Fresh answer."},
	}
	orchestrator, _ := fixture(t, provider)
	if _, err := orchestrator.Handle(context.Background(), "add milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestrator.Handle(context.Background(), "what changed?"); err != nil {
		t.Fatal(err)
	}
	followup := findRequest(provider.requests, "stream", 0)
	if !containsRole(followup.history, "tool") || !containsText(followup.history, "[execution:completion_report]") || !containsText(followup.history, "[execution:skill_selected]") {
		t.Fatalf("follow-up history = %#v", followup.history)
	}
	orchestrator.NewSession()
	if _, err := orchestrator.Handle(context.Background(), "fresh"); err != nil {
		t.Fatal(err)
	}
	fresh := findRequest(provider.requests, "stream", 1)
	if len(fresh.history) != 0 {
		t.Fatalf("fresh history = %#v", fresh.history)
	}
}

type blockingProvider struct{}

func (blockingProvider) Structured(ctx context.Context, _ string, _ []openai.Message, _, _ string, _ map[string]any, _ any) error {
	<-ctx.Done()
	return ctx.Err()
}

func (blockingProvider) Complete(context.Context, string, []openai.Message, []openai.Tool) (openai.Message, error) {
	panic("unexpected Complete call")
}

func (blockingProvider) Stream(context.Context, string, []openai.Message, string, func(string)) (string, error) {
	panic("unexpected Stream call")
}

func TestCancellationIsLogged(t *testing.T) {
	orchestrator, cfg := fixture(t, &fakeProvider{})
	orchestrator.client = blockingProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	if _, err := orchestrator.Handle(ctx, "wait"); err == nil {
		t.Fatal("expected cancellation")
	}
	entries, err := os.ReadDir(cfg.RunsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("logs = %#v, %v", entries, err)
	}
	content, err := os.ReadFile(filepath.Join(cfg.RunsDir, entries[0].Name()))
	if err != nil || !strings.Contains(string(content), `"type":"cancelled"`) {
		t.Fatalf("log = %q, %v", content, err)
	}
}

func hasTool(tools []openai.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Function.Name == name {
			return true
		}
	}
	return false
}

func containsRole(messages []openai.Message, role string) bool {
	for _, message := range messages {
		if message.Role == role {
			return true
		}
	}
	return false
}

func containsText(messages []openai.Message, text string) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, text) {
			return true
		}
	}
	return false
}

func findRequest(requests []request, kind string, index int) request {
	for _, request := range requests {
		if request.kind == kind {
			if index == 0 {
				return request
			}
			index--
		}
	}
	return request{}
}
