//go:build windows

package main

// shellCommand runs the command line through the Windows command interpreter,
// so built-in commands, batch files (.bat/.cmd) and PATH lookups all work the
// same way they would from a cmd prompt.
func shellCommand(command string) (name string, args []string) {
	return "cmd", []string{"/C", command}
}
