# Tools

Tools are executable operations exposed through native OpenAI function calls.

## Registered Tools

- `listFiles`: list workspace files and directories.
- `readFile`: read a UTF-8 workspace file.
- `searchFiles`: search matching lines in workspace text files.
- `writeFile`: write a UTF-8 workspace file.

Definitions live under `internal/tools/definitions/`. Add a Go definition that
returns `tools.Definition`, then register it in `index.go`.

Filesystem tools reject absolute paths, traversal outside the workspace, parent
symlink escapes, and writes through symlinked output files. Repeated identical
successful calls in one outcome are rejected and returned to the executor as
corrective context.
