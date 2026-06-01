# Architecture

## Source Layout

```text
cmd/openhome/   TUI binary and headless run command
cmd/smoke/      live oMLX compatibility smoke check
internal/
  agents/       prompts, routing, execution, and completion
  app/          composition root
  commands/     TUI command registry
  config/       environment loading
  logging/      append-only run logger
  permissions/  approval modes, session rules, shell regex policy
  providers/    OpenAI-compatible Chat Completions client
  session/      native message history
  skills/       Markdown skill catalog loader
  tools/        registry, workspace sandbox, tool definitions
  tui/          Bubble Tea application
```

## Request Lifecycle

1. Route request into `direct_answer`, `simple_task`, or `planned_task`.
2. Print intent and lane before action.
3. Stream direct answers immediately.
4. Skip planning for narrow simple tasks.
5. Build outcome-oriented steps for planned tasks.
6. Select one skill for the first execution unit and reuse it.
7. Execute with native OpenAI `tool_calls` and `tool` messages. Advanced calls pause for permission when policy requires it.
8. Reselect only after `request_skill_reselection`.
9. Escalate optional final verification with `request_verification`.
10. Produce a completion report and retain native messages in session history.

Router, librarian, planner, verifier, and completion calls use JSON schema.
Executor calls use the standard Chat Completions `tools` field.
