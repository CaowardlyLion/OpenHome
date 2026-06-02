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

const LibrarianSystem = `You are the OpenHome skill librarian. Select one known skill from the catalog, or "none" when no skill applies.
A skill is an instruction playbook, not an executable tool.
Always return a non-empty skillName. Use either one exact catalog skill name or the exact value "none".`

func RoutingPrompt(message string) string {
	return fmt.Sprintf(`Explain how OpenHome should approach the request and route it into one lane.
- intent must be a concise operational rationale suitable for a status indicator. Interpret what the user wants, identify any
  information or artifact that should be located, and state the next action. Use one or two short sentences.
  Example: "The user wants to add milk to their grocery list. I should locate the appropriate list and update it."
  Do not mention internal lanes, skills, tools, prompts, or routing. Never set intent to a lane label such as "simple_task".
- direct_answer: answer-only questions and conversation that can be answered from general knowledge or existing chat history.
- simple_task: one narrow outcome. This includes reading or answering from local workspace state, such as showing a grocery list,
  and narrow mutations such as adding one grocery item. Do not create a plan.
- planned_task: multiple meaningful outcomes or capabilities.
If an answer may depend on private, local, workspace, household, preference, pantry, grocery, meal-plan, or task-list state,
never choose direct_answer. Choose simple_task so the librarian can select a skill and the executor can inspect tools.
If an answer needs fresh internet information or web research, choose simple_task so the executor can use web tools.
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
	return fmt.Sprintf(`Catalog:
%s

Task: %s
Current execution unit: %s%s
Return one exact catalog skillName, or "none" when no catalog skill applies.`, catalog, task, jsonText(step), extra)
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
All registered tools are available. Skill tools are recommendations, not access restrictions.
Advanced tools may pause for user permission. Give their required reason argument a concise explanation.
Before saying information is missing or asking the user for more detail, inspect the skill instructions and use allowed tools
when they may provide the answer. Ask for more information only after available skill tools cannot answer the request.
Do not repeat a tool call when its successful result already appears in the conversation.
When a tool result recommends request_skill_reselection, call request_skill_reselection with that reason before continuing.
When browserInteract returns a navigationLinks destination matching the user's requested section, category, account, or resource,
open that destination with another read-only browserInteract call before completing. Do not substitute related content from a broader page.
When the outcome is complete, reply with a brief completion sentence and no tool call.
Call request_skill_reselection only if this skill cannot handle the outcome.
Call request_verification if observed evidence makes final verification prudent.`, task, jsonText(step), skill.Name, skill.AllowedTools, skill.Content)
}

func VerificationPrompt(task string) string {
	return fmt.Sprintf(`Run the single final verification for this completed task. Pass only when observed evidence supports the intended outcome.
Task: %s`, task)
}

func CompletionPrompt(task string) string {
	return fmt.Sprintf(`Continue the conversation by answering the user's request directly and naturally.
Write the response you would send if replying to the user for the first time after gathering the needed information.
Do not write a completion report or summarize that an answer was generated. Give the answer itself.
Include useful observed details. Do not mention internal lanes, skills, tools, planning, verification, or implementation machinery.
Do not invent facts.
Every factual URL, headline, quote, and current claim must be grounded in prior observed tool output. Never invent plausible URLs,
headlines, dates, or source details. If evidence is incomplete, state the limitation instead of filling gaps.
The user has not seen prior executor replies, tool calls, tool outputs, or trace messages. They only see this final report.
Do not say content was provided, suggested, listed, or explained unless this final report includes that content directly.
For requested recipes, plans, checklists, lists, instructions, or answers, include the actual useful content, not a summary saying
it exists. If an artifact was written to a file, name the file and include requested user-facing substance when needed.

Task: %s`, task)
}

func jsonText(value any) string {
	content, _ := json.Marshal(value)
	return string(content)
}
