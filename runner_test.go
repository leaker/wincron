package main

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// A successful command logs exit 0 and its captured stdout.
func TestRunCommandSuccess(t *testing.T) {
	var buf bytes.Buffer
	runCommand(log.New(&buf, "", 0), "echo wincron-ok")

	logs := buf.String()
	if !strings.Contains(logs, "OK (exit 0") {
		t.Errorf("missing success log, got:\n%s", logs)
	}
	if !strings.Contains(logs, "output: wincron-ok") {
		t.Errorf("missing captured stdout, got:\n%s", logs)
	}
}

// A failing command logs both the exit code and the specific reason (stderr).
func TestRunCommandFailureRecordsExitAndReason(t *testing.T) {
	var buf bytes.Buffer
	// Write to stderr, then exit non-zero — works under both sh and cmd.
	runCommand(log.New(&buf, "", 0), "echo boom 1>&2 && exit 7")

	logs := buf.String()
	if !strings.Contains(logs, "FAILED (exit 7") {
		t.Errorf("missing exit-7 failure log, got:\n%s", logs)
	}
	if !strings.Contains(logs, "boom") {
		t.Errorf("failure reason (stderr) not captured, got:\n%s", logs)
	}
}
