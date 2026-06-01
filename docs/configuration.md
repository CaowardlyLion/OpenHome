# Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENAI_BASE_URL` | `http://10.10.30.80:8000/v1` | OpenAI-compatible API base URL. |
| `OPENAI_MODEL` | `gemma-4-e2b-it-bf16` | Model ID sent to API. |
| `OPENAI_API_KEY` | empty | Optional bearer token. |
| `OPENHOME_SKILLS_DIR` | `./skills` | Skill catalog directory. |
| `OPENHOME_WORKSPACE` | `./workspace` | Sandboxed task artifact directory. |
| `OPENHOME_MAX_TOOL_ROUNDS` | `8` | Maximum native tool rounds per outcome. |
| `OPENHOME_MAX_STEP_ATTEMPTS` | `3` | Deprecated compatibility setting from the per-step verifier runtime. |
| `OPENHOME_PERMISSION_MODE` | `default` | Startup advanced-tool policy: `ask`, `default`, or `allow`. |
| `OPENHOME_SEARCH_ENDPOINT` | Bing RSS | Search adapter endpoint for `webSearch`. |

Each request gets an append-only JSONL log under `.openhome/runs/`. Logs include
routes, plans, skill selection, native assistant messages, tool results,
permission requests and decisions, rejections, verification, cancellation, and
completion. Advanced-tool output bodies remain fully logged.

Shell policy files are created under `.openhome/` on first launch:

```text
.openhome/shell-allow.txt
.openhome/shell-deny.txt
```

Each non-comment line is a regular expression matched against normalized shell
command plus arguments. Deny rules override every mode. Files load at startup.

Headless `openhome run` reads `OPENHOME_PERMISSION_MODE`. Approval-required calls
prompt only when stdin is a terminal; non-interactive runs deny them.

```bash
make dev
make test
make smoke
make build
```
