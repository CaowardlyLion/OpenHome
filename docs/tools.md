# Tools

Tools are executable operations exposed through native OpenAI function calls.
All registered tools are available to executor. Skills remain workflow guidance.

## Workspace Tools

- `listFiles`: list workspace files and directories.
- `readFile`: read a UTF-8 workspace file.
- `searchFiles`: search matching lines in workspace text files.
- `writeFile`: write a UTF-8 workspace file.

Workspace tools are automatic and sandboxed. They reject absolute paths,
traversal, parent symlink escapes, and writes through symlinked output files.

## Advanced Tools

- `readExternalFile`: read approved absolute local path, capped at 2 MB.
- `runCommand`: run one binary directly from workspace cwd. No shell pipes or redirection.
- `fetchURL`: fetch approved HTTP(S), extract readable text, and return at most 32 KB of context. Raw HTML, scripts, styles, and large page bodies are not inserted into model context.
- `downloadFile`: save approved HTTP(S) response inside workspace, capped at 50 MB.
- `webSearch`: search through provider-neutral adapter. Default is Bing RSS.
- `browserInteract`: open rendered pages through CloakBrowser, optionally click or type with CSS selectors, capture workspace PNG screenshots, and return compact visible text and links. Always prompts.
- `sendEmail`: send explicitly requested email through configured SMTP. Always prompts.

Advanced tools require agent-supplied reason. Prompt choices: allow once,
always allow similar for current process, or deny.

## CloakBrowser

`browserInteract` keeps Go as the orchestrator and invokes
`scripts/cloakbrowser_bridge.py`. Install the Python package separately:

```bash
.venv/bin/python -m pip install cloakbrowser
.venv/bin/cloakbrowser install
```

CloakBrowser requires its Chromium binary. The OpenHome bridge
uses a persistent profile under `.openhome/cloakbrowser-profile` and returns at
most 24 KB of visible page text plus 30 links. Screenshots are written only to
workspace-relative `.png` paths after Go workspace sandbox validation.
Obstructing overlays may be hidden for capture with `screenshotHideSelectors`
without clicking acceptance or changing site state.

## Email

`sendEmail` requires `OPENHOME_SMTP_FROM` and `OPENHOME_SMTP_ADDR`.
`OPENHOME_SMTP_USER` and `OPENHOME_SMTP_PASSWORD` are optional for SMTP servers
that do not require authentication. Email sending always pauses for approval.

## Shell Policy

`runCommand` defaults to 30-second timeout and caps stdout and stderr at 1 MB
each. Agent may request up to 600 seconds; values above 30 seconds always prompt.

Default mode automatically permits workspace-safe inspection commands such as
`ls`, `pwd`, `find`, `cat`, `head`, `tail`, `wc`, and `rg`. Host paths, risky
flags, and write-like commands prompt. User regex allow and deny files live
under `.openhome/`; deny rules always win.
