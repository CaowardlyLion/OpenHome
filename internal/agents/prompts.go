package agents

import (
	"encoding/json"
	"fmt"

	"github.com/CaowardlyLion/OpenHome/internal/skills"
)

const MainSystem = `You are OpenHome, a local assistant for everyday tasks.
Skills are instruction playbooks. Tools are executable operations.
Use only observed tool results when describing workspace state.
When tools are available, use standard native function calls.
Before saying you lack information, inspect the selected skill and use its allowed tools when they may answer the request.
Only ask the user for missing information after skill instructions and available tool evidence cannot supply it.`

const LibrarianSystem = `You are the OpenHome skill librarian. Select exactly one known skill from the catalog.
A skill is an instruction playbook, not an executable tool.
Always return a non-empty skillName exactly matching one catalog skill name.`

func RoutingPrompt(message string) string {
	return fmt.Sprintf(`Summarize the user's intended outcome and route it into one lane.
- intent must be a concise user-facing outcome sentence. Never set intent to a lane label such as "simple_task".
- direct_answer: answer-only questions and conversation that can be answered from general knowledge or existing chat history.
- simple_task: one narrow outcome. This includes reading or answering from local workspace state, such as showing a grocery list,
  and narrow mutations such as adding one grocery item. Do not create a plan.
- planned_task: multiple meaningful outcomes or capabilities.
If an answer may depend on private, local, workspace, household, preference, pantry, grocery, meal-plan, or task-list state,
never choose direct_answer. Choose simple_task so the librarian can select a skill and the executor can inspect tools.
Choose final verification only when complexity, mutation risk, or uncertain evidence warrants it. Otherwise choose none.

Message:
%s`, message)
}

func PlanningPrompt(task string) string {
	return fmt.Sprintf(`Create a compact outcome-oriented plan for this task.
Each step must describe a meaningful user outcome. Never make separate steps for reading, writing, tool use, or verification;
those are internal executor details. Include explicit success criteria.

Task:
%s`, task)
}

func SkillSelectionPrompt(catalog, task string, step Step, reason string) string {
	extra := ""
	if reason != "" {
		extra = "\nPrevious skill could not handle the execution unit: " + reason
	}
	return fmt.Sprintf("Catalog:\n%s\n\nTask: %s\nCurrent execution unit: %s%s\nReturn one exact non-empty catalog skillName.", catalog, task, jsonText(step), extra)
}

func ExecutionPrompt(task string, step Step, skill skills.Skill) string {
	return fmt.Sprintf(`Execute the current outcome using the selected skill.
Task: %s
Outcome: %s
Selected skill: %s
Allowed workspace tools: %v
Skill instructions:
%s

Use native function calls as needed. Use exact argument key "path", not "file_path".
Before saying information is missing or asking the user for more detail, inspect the skill instructions and use allowed tools
when they may provide the answer. Ask for more information only after available skill tools cannot answer the request.
Do not repeat a tool call when its successful result already appears in the conversation.
When the outcome is complete, reply with a brief completion sentence and no tool call.
Call request_skill_reselection only if this skill cannot handle the outcome.
Call request_verification if observed evidence makes final verification prudent.`, task, jsonText(step), skill.Name, skill.AllowedTools, skill.Content)
}

func VerificationPrompt(task string) string {
	return fmt.Sprintf(`Run the single final verification for this completed task. Pass only when observed evidence supports the intended outcome.
Task: %s`, task)
}

func CompletionPrompt(task string) string {
	return fmt.Sprintf(`Write a concise user-facing completion report.
State the resulting answer or completed outcome. Include useful observed details. Do not mention internal lanes, skills, tools,
planning, verification, or implementation machinery. Do not invent facts.

Task: %s`, task)
}

func jsonText(value any) string {
	content, _ := json.Marshal(value)
	return string(content)
}
