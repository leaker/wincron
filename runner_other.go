//go:build !windows

package main

// shellCommand runs the command line through /bin/sh on non-Windows platforms.
// This keeps the tool buildable and testable on macOS/Linux during development;
// the production target is Windows, which uses cmd (see runner_windows.go).
func shellCommand(command string) (name string, args []string) {
	return "sh", []string{"-c", command}
}
