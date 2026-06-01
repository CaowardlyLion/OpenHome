package definitions

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

type cappedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(content []byte) (int, error) {
	original := len(content)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(content) > remaining {
			content = content[:remaining]
		}
		_, _ = b.buffer.Write(content)
	}
	if original > remaining {
		b.truncated = true
	}
	return original, nil
}

func RunCommand() tools.Definition {
	return tools.Definition{
		Name: "runCommand", Advanced: true, Description: "Run one binary directly from the workspace after permission when required. No shell pipes or redirection.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"command", "reason"},
			"properties": map[string]any{
				"command": map[string]any{"type": "string"}, "args": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"reason": map[string]any{"type": "string"}, "timeoutSeconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 600},
			},
		},
		Execute: func(ctx context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			command, err := stringArg(args, "command")
			if err != nil {
				return nil, err
			}
			commandArgs, err := stringSliceArg(args, "args")
			if err != nil {
				return nil, err
			}
			reason, err := stringArg(args, "reason")
			if err != nil {
				return nil, err
			}
			timeoutSeconds, err := intArg(args, "timeoutSeconds", 30)
			if err != nil {
				return nil, err
			}
			if timeoutSeconds < 1 || timeoutSeconds > 600 {
				return nil, fmt.Errorf("timeoutSeconds must be between 1 and 600")
			}
			if toolContext.Permissions == nil || toolContext.Permissions.AuthorizeCommand(ctx, command, commandArgs, reason, timeoutSeconds) == permissions.Deny {
				return denied("runCommand"), nil
			}
			commandCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
			defer cancel()
			process := exec.CommandContext(commandCtx, command, commandArgs...)
			process.Dir = toolContext.Workspace.Root
			stdout := &cappedBuffer{limit: 1 << 20}
			stderr := &cappedBuffer{limit: 1 << 20}
			process.Stdout, process.Stderr = stdout, stderr
			runErr := process.Run()
			exitCode := 0
			if runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else if commandCtx.Err() == nil {
					return nil, runErr
				}
			}
			return map[string]any{
				"command": command, "args": commandArgs, "exitCode": exitCode,
				"stdout": stdout.buffer.String(), "stderr": stderr.buffer.String(),
				"timedOut":        commandCtx.Err() == context.DeadlineExceeded,
				"stdoutTruncated": stdout.truncated, "stderrTruncated": stderr.truncated,
			}, nil
		},
	}
}
