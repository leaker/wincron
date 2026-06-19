package main

import (
	"log"
	"os"
	"strings"
)

// Entry is a single scheduled job parsed from the crontab file. Spec is the
// schedule string handed to the cron parser (standard 5-field expression or an
// @descriptor); Command is the shell command line to execute when it fires.
type Entry struct {
	Spec    string
	Command string
}

// parseFile reads the crontab file and splits each non-comment, non-blank line
// into an Entry. Structural problems (a schedule with no command) are logged
// and the offending line is skipped. The cron expression itself is NOT
// validated here — that happens when the entry is registered with the cron
// scheduler, so robfig/cron remains the single source of truth for syntax.
func parseFile(path string, logger *log.Logger) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		spec, command, ok := splitLine(line)
		if !ok {
			logger.Printf("skip line %d (schedule without command): %q", lineNo, line)
			continue
		}
		entries = append(entries, Entry{Spec: spec, Command: command})
	}
	return entries, nil
}

// splitLine separates the schedule portion of a crontab line from the command.
// It understands three shapes:
//
//   - a standard 5-field expression followed by the command
//     ("0 9 * * 1 backup.bat");
//   - the "@every 5m command" interval descriptor, whose schedule spans the
//     first two tokens;
//   - a single-token descriptor such as "@daily command" or "@hourly command".
//
// ok is false when the line carries a schedule but no command to run.
func splitLine(line string) (spec, command string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", "", false
	}

	if strings.HasPrefix(fields[0], "@") {
		if strings.EqualFold(fields[0], "@every") {
			if len(fields) < 3 {
				return "", "", false
			}
			return fields[0] + " " + fields[1], strings.Join(fields[2:], " "), true
		}
		if len(fields) < 2 {
			return "", "", false
		}
		return fields[0], strings.Join(fields[1:], " "), true
	}

	// Standard expression: five schedule fields followed by the command.
	if len(fields) < 6 {
		return "", "", false
	}
	return strings.Join(fields[:5], " "), strings.Join(fields[5:], " "), true
}
