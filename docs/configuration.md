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

Each request gets an append-only JSONL log under `.openhome/runs/`. Logs include
routes, plans, skill selection, native assistant messages, tool results,
rejections, verification, cancellation, and completion.

```bash
make dev
make test
make smoke
make build
```
