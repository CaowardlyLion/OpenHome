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
- `fetchURL`: fetch approved HTTP(S) text, capped at 2 MB.
- `downloadFile`: save approved HTTP(S) response inside workspace, capped at 50 MB.
- `webSearch`: search through provider-neutral adapter. Default is DuckDuckGo HTML.

Advanced tools require agent-supplied reason. Prompt choices: allow once,
always allow similar for current process, or deny.

## Shell Policy

`runCommand` defaults to 30-second timeout and caps stdout and stderr at 1 MB
each. Agent may request up to 600 seconds; values above 30 seconds always prompt.

Default mode automatically permits workspace-safe inspection commands such as
`ls`, `pwd`, `find`, `cat`, `head`, `tail`, `wc`, and `rg`. Host paths, risky
flags, and write-like commands prompt. User regex allow and deny files live
under `.openhome/`; deny rules always win.
