package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoggerWritesFile proves the rotating file writer (not just stdout)
// actually receives log output.
func TestLoggerWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wincron.log")

	logger := buildLogger(logConfig{path: path, maxSizeMB: 10, maxBackups: 3, maxAgeDays: 30}, io.Discard)
	logger.Printf("hello from the file writer")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("log file not written: %v", err)
	}
	if !strings.Contains(string(data), "hello from the file writer") {
		t.Fatalf("log file missing expected line, got: %q", string(data))
	}
}

// TestLoggerRotates proves size-based rotation actually produces backup files
// once the active log exceeds maxSizeMB.
func TestLoggerRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wincron.log")

	// maxSizeMB=1 → rotate after ~1MB. Each line below is ~100 bytes, so ~25k
	// lines comfortably crosses two rotations.
	logger := buildLogger(logConfig{path: path, maxSizeMB: 1, maxBackups: 5, maxAgeDays: 0, compress: false}, io.Discard)
	line := strings.Repeat("x", 90)
	for i := 0; i < 25000; i++ {
		logger.Println(line)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "wincron*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) < 2 {
		t.Fatalf("expected rotation to create backup files, found only: %v", matches)
	}
}
