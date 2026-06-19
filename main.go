package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	// Embed the timezone database so the -tz flag resolves named zones (e.g.
	// Asia/Shanghai) on Windows hosts, which have no system zoneinfo.
	_ "time/tzdata"
)

func main() {
	var (
		logPath    string
		reload     time.Duration
		grace      time.Duration
		tz         string
		maxSizeMB  int
		maxBackups int
		maxAgeDays int
		compress   bool
	)
	flag.StringVar(&logPath, "log", "", "log file path (default: <exe dir>/wincron.log)")
	flag.DurationVar(&reload, "reload", 15*time.Second, "crontab reload poll interval")
	flag.DurationVar(&grace, "grace", 30*time.Second, "max wait for running jobs on shutdown")
	flag.StringVar(&tz, "tz", "Local", "timezone for cron expressions (Local, UTC, Asia/Shanghai, ...)")
	flag.IntVar(&maxSizeMB, "log-max-size", 10, "rotate the log after it exceeds this many MB")
	flag.IntVar(&maxBackups, "log-max-backups", 5, "number of rotated log files to keep")
	flag.IntVar(&maxAgeDays, "log-max-age", 30, "max age in days for rotated log files")
	flag.BoolVar(&compress, "log-compress", true, "gzip rotated log files")
	flag.Usage = usage
	flag.Parse()

	baseDir := exeDir()
	crontabPath := flag.Arg(0)
	if crontabPath == "" {
		crontabPath = filepath.Join(baseDir, "crontab.txt")
	}
	if logPath == "" {
		logPath = filepath.Join(baseDir, "wincron.log")
	}

	logger, logCloser := newLogger(logConfig{
		path:       logPath,
		maxSizeMB:  maxSizeMB,
		maxBackups: maxBackups,
		maxAgeDays: maxAgeDays,
		compress:   compress,
	})
	defer logCloser.Close()

	loc, err := loadLocation(tz)
	if err != nil {
		logger.Printf("invalid -tz %q: %v (falling back to Local)", tz, err)
		loc = time.Local
	}

	logger.Printf("wincron starting (%s) crontab=%s log=%s tz=%s reload=%s",
		runtime.Version(), crontabPath, logPath, loc, reload)

	// nssm stops a service by sending Ctrl-C (a console control event), which
	// arrives as os.Interrupt; SIGTERM is included for non-Windows hosts.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mgr := &manager{
		path:     crontabPath,
		logger:   logger,
		reload:   reload,
		grace:    grace,
		location: loc,
	}
	mgr.Run(ctx)
	logger.Printf("wincron stopped")
}

// exeDir returns the directory containing the running executable, falling back
// to the current working directory if it cannot be determined. Defaulting the
// crontab and log next to the binary keeps the tool predictable when nssm
// launches it with an unexpected working directory.
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		if wd, werr := os.Getwd(); werr == nil {
			return wd
		}
		return "."
	}
	return filepath.Dir(exe)
}

// loadLocation resolves a timezone name. "Local" (the default) and an empty
// string map to the host's local time, matching the original tool's behaviour.
func loadLocation(tz string) (*time.Location, error) {
	if tz == "" || tz == "Local" {
		return time.Local, nil
	}
	if tz == "UTC" {
		return time.UTC, nil
	}
	return time.LoadLocation(tz)
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(),
		"wincron - a cron-style scheduler for Windows services (run under nssm)\n\n"+
			"Usage:\n  wincron [flags] [crontab-file]\n\n"+
			"If crontab-file is omitted, <exe dir>/crontab.txt is used.\n\n"+
			"Flags:\n")
	flag.PrintDefaults()
}
