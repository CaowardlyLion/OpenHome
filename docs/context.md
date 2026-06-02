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

Session history and active executor messages use a bounded sliding window.
When retained message payload exceeds `OPENHOME_MAX_CONTEXT_BYTES`, OpenHome
drops the oldest messages first. Assistant tool-call messages remain grouped
with their tool outputs so trimming does not leave broken native tool sequences.
The newest message unit remains available even if it alone exceeds the budget.
