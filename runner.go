package main

import (
	"bytes"
	"errors"
	"log"
	"os/exec"
	"strings"
	"time"
)

// runCommand executes a single crontab command through the platform shell,
// waiting for it to finish so the exit status can be reported. robfig/cron runs
// every job invocation in its own goroutine, so blocking here does not stall the
// scheduler, and overlapping runs of a slow command are allowed (matching the
// fire-and-forget behaviour of the original Python tool).
//
// On success the exit code (0) and duration are logged. On failure the specific
// reason is logged: for a command that ran but exited non-zero, the exit code
// plus captured stderr; for a command that never started (not found, permission
// denied, ...), the underlying OS error.
func runCommand(logger *log.Logger, command string) {
	name, args := shellCommand(command)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Printf("run: %s", command)
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			logger.Printf("FAILED (exit %d, %s): %s | stderr: %s",
				exitErr.ExitCode(), elapsed, command, oneLine(stderr.String()))
		} else {
			// The process could not be started at all.
			logger.Printf("ERROR starting (%s): %s | %v", elapsed, command, err)
		}
		return
	}

	logger.Printf("OK (exit 0, %s): %s", elapsed, command)
	if out := oneLine(stdout.String()); out != "" {
		logger.Printf("output: %s", out)
	}
}

// oneLine trims surrounding whitespace and collapses embedded newlines so a
// multi-line stderr/stdout capture stays on a single, greppable log line.
func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
	return replacer.Replace(s)
}
