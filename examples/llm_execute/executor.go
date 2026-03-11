package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const maxOutputLen = 4096

// RunCommand executes a shell command and returns stdout, stderr, exit code, and any error.
func RunCommand(ctx context.Context, command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = truncateOutput(outBuf.String())
	stderr = truncateOutput(errBuf.String())

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return stdout, stderr, -1, fmt.Errorf("command execution failed: %w", runErr)
		}
	}

	return stdout, stderr, exitCode, nil
}

func truncateOutput(s string) string {
	if len(s) > maxOutputLen {
		return s[:maxOutputLen] + "\n... (output truncated)"
	}
	return s
}
