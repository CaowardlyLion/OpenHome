# OpenHome Developer Guide

OpenHome is a local Go TUI agent for everyday tasks. It uses an OpenAI-compatible
Chat Completions endpoint, Markdown skills, declarative native tools, and three
routing lanes.

## Guides

- [Architecture](architecture.md): runtime components and request lifecycle.
- [Context](context.md): native OpenAI messages and sessions.
- [Skills](skills.md): skill tree format and adding playbooks.
- [Tools](tools.md): tool registry, sandbox, and adding operations.
- [CLI Commands](cli-commands.md): built-in TUI commands.
- [Configuration](configuration.md): environment variables and logs.

## Core Terms

- **Skill:** non-executable Markdown playbook with an allowed tool list.
- **Tool:** executable operation such as `readFile` or `writeFile`.
- **Librarian:** role that selects the first skill and reselects only on request.
- **Executor:** role that uses native tool calls until an outcome is complete.
- **Verifier:** role invoked once at task end only when requested.
