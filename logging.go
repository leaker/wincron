package main

import (
	"io"
	"log"
	"os"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// logConfig controls the rotating log file. Sizes follow lumberjack semantics.
type logConfig struct {
	path       string // log file path
	maxSizeMB  int    // rotate after the file grows past this many megabytes
	maxBackups int    // number of rotated files to retain
	maxAgeDays int    // max age in days for a rotated file
	compress   bool   // gzip rotated files
}

// newLogger writes to both stdout (so nssm/console can capture it) and a
// size-rotated file.
func newLogger(cfg logConfig) *log.Logger {
	return buildLogger(cfg, os.Stdout)
}

// buildLogger is the testable core of newLogger: it sends output to the given
// console writer plus a size-rotated file. The returned logger is safe for
// concurrent use because the standard log.Logger serialises writes with its own
// mutex.
func buildLogger(cfg logConfig, console io.Writer) *log.Logger {
	rotator := &lumberjack.Logger{
		Filename:   cfg.path,
		MaxSize:    cfg.maxSizeMB,
		MaxBackups: cfg.maxBackups,
		MaxAge:     cfg.maxAgeDays,
		Compress:   cfg.compress,
	}
	w := io.MultiWriter(console, rotator)
	return log.New(w, "", log.LstdFlags|log.Lmsgprefix)
}
