package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantSpec    string
		wantCommand string
		wantOK      bool
	}{
		{"standard", "*/5 * * * * echo hi", "*/5 * * * *", "echo hi", true},
		{"standard with args", "0 9 * * 1 backup.bat --full", "0 9 * * 1", "backup.bat --full", true},
		{"command keeps trailing fields", "0 0 1 1 * cmd a b c", "0 0 1 1 *", "cmd a b c", true},
		{"descriptor", "@daily run-report", "@daily", "run-report", true},
		{"every", "@every 30s ping host", "@every 30s", "ping host", true},
		{"five fields no command", "* * * * *", "", "", false},
		{"descriptor without command", "@hourly", "", "", false},
		{"every without command", "@every 10s", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, command, ok := splitLine(tt.line)
			if ok != tt.wantOK || spec != tt.wantSpec || command != tt.wantCommand {
				t.Errorf("splitLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.line, spec, command, ok, tt.wantSpec, tt.wantCommand, tt.wantOK)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	content := `# this is a comment

*/5 * * * * echo every-five
@daily  daily-job.sh

   # indented comment
@every 2s tick
* * * * badline-no-command
`
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := log.New(io.Discard, "", 0)
	entries, err := parseFile(path, logger)
	if err != nil {
		t.Fatalf("parseFile error: %v", err)
	}

	want := []Entry{
		{Spec: "*/5 * * * *", Command: "echo every-five"},
		{Spec: "@daily", Command: "daily-job.sh"},
		{Spec: "@every 2s", Command: "tick"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("parseFile entries =\n  %#v\nwant\n  %#v", entries, want)
	}
}

func TestParseFileMissingReturnsError(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	if _, err := parseFile(filepath.Join(t.TempDir(), "nope.txt"), logger); err == nil {
		t.Fatal("expected error for missing crontab file, got nil")
	}
}
