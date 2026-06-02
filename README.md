# OpenHome

OpenHome is a local Go TUI agent for everyday tasks. It uses an OpenAI-compatible
Chat Completions endpoint, Markdown skills, and sandboxed workspace tools.

Developer documentation: [docs/README.md](docs/README.md)

## Start

```bash
make dev
go run ./cmd/openhome
go run ./cmd/openhome run "add watermelon to grocery list"
```

Defaults:

```text
OPENAI_BASE_URL=http://10.10.30.80:8000/v1
OPENAI_MODEL=gemma-4-e2b-it-bf16
OPENAI_API_KEY=
OPENHOME_SKILLS_DIR=./skills
OPENHOME_WORKSPACE=./workspace
OPENHOME_MAX_TOOL_ROUNDS=8
OPENHOME_MAX_STEP_ATTEMPTS=3
OPENHOME_PERMISSION_MODE=default
OPENHOME_SEARCH_ENDPOINT=https://www.bing.com/search?format=rss
OPENHOME_CLOAKBROWSER_PYTHON=.venv/bin/python
```

`OPENHOME_MAX_STEP_ATTEMPTS` remains accepted for compatibility with existing
launch scripts. The Go runtime uses one optional final verifier instead of
per-step retries.

Type `/new` to clear chat context. Type `/verbose` to show or hide live execution
details. Type `/permissions` to choose advanced-tool approval mode. Type `/exit` to leave the TUI. Press `Ctrl+C` during a task to cancel
its context while preserving the JSONL event log under `.openhome/runs/`.

OpenHome routes requests through `direct_answer`, `simple_task`, or
`planned_task`. Task execution uses native OpenAI tool calls. Final verification
runs only when the router or executor requests it.

Advanced tools can read external files, run commands, fetch compact website text,
download files, search the web, navigate rendered pages with CloakBrowser, and
send explicitly requested email through configured SMTP. Risky operations pause
for explicit approval.

Optional CloakBrowser support uses a Python bridge while Go remains the runtime:

```bash
.venv/bin/python -m pip install cloakbrowser
.venv/bin/cloakbrowser install
```

## Verify

```bash
make test
make smoke
make build
```

## Layout

```text
cmd/openhome/   TUI binary and headless run command
cmd/smoke/      live oMLX compatibility smoke check
internal/
  agents/       prompts and three-lane orchestration
  app/          composition root
  commands/     declarative TUI commands
  config/       environment loading
  logging/      append-only JSONL logs
  permissions/  advanced-tool approval policy
  providers/    OpenAI-compatible client
  session/      native message history
  skills/       Markdown skill loading and validation
  tools/        declarative tools, registry, workspace sandbox
  tui/          Bubble Tea application
skills/         preserved Markdown playbooks
workspace/      preserved local artifacts
```

The previous TypeScript implementation is retained locally under
`.legacy-typescript/` as a migration backup.
