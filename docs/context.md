# Context And Sessions

OpenHome retains native OpenAI-style messages rather than flattening execution
history into one prompt.

```text
user: original request
assistant: [execution:route] ...
assistant: [execution:plan] ...
assistant: [execution:skill_selected] ...
assistant: tool_calls=[writeFile(...)]
tool: write result
assistant: [execution:verification] ...
assistant: [execution:completion_report] ...
assistant: user-facing completion report
```

Later outcomes and follow-up requests can reuse prior tool outputs. `/new`
clears in-memory context while leaving workspace files and JSONL logs intact.
