package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
)

// manager owns the active cron scheduler. It loads the crontab on start, polls
// the file's modification time, and rebuilds the scheduler whenever the file
// changes. All methods are called from a single goroutine (Run's loop), so the
// struct needs no internal locking.
type manager struct {
	path     string
	logger   *log.Logger
	reload   time.Duration  // how often to poll the crontab's mtime
	grace    time.Duration  // how long to wait for running jobs on shutdown
	location *time.Location // timezone used to interpret cron expressions

	active  *cron.Cron
	lastMod int64  // mtime (unix nano) of the crontab last successfully applied
	lastErr string // last stat/read error logged, to suppress duplicate spam
}

// Run loads the crontab, starts scheduling, and blocks until ctx is cancelled.
// On cancellation it stops the scheduler and waits up to grace for in-flight
// jobs to finish before returning.
func (m *manager) Run(ctx context.Context) {
	m.checkAndReload()

	ticker := time.NewTicker(m.reload)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdown()
			return
		case <-ticker.C:
			m.checkAndReload()
		}
	}
}

// checkAndReload rebuilds the scheduler if the crontab file changed since the
// last successful load. Errors are logged but never fatal: a transient read
// failure leaves the previously loaded schedule running.
func (m *manager) checkAndReload() {
	info, err := os.Stat(m.path)
	if err != nil {
		m.logErrOnce(fmt.Sprintf("stat crontab %s: %v", m.path, err))
		return
	}

	mod := info.ModTime().UnixNano()
	if mod == m.lastMod && m.active != nil {
		return
	}

	entries, err := parseFile(m.path, m.logger)
	if err != nil {
		m.logErrOnce(fmt.Sprintf("read crontab %s: %v (keeping current schedule)", m.path, err))
		// Mark this mtime as seen so we don't re-read the same bad file every
		// tick; the next save (new mtime) will trigger another attempt.
		m.lastMod = mod
		return
	}
	m.lastErr = ""

	next := cron.New(cron.WithLocation(m.location))
	registered := 0
	for _, e := range entries {
		e := e // capture for the closure
		if _, err := next.AddFunc(e.Spec, func() { runCommand(m.logger, e.Command) }); err != nil {
			m.logger.Printf("skip invalid schedule %q (%s): %v", e.Spec, e.Command, err)
			continue
		}
		registered++
	}

	next.Start()
	if m.active != nil {
		// Stop scheduling new runs of the old set; jobs already running are
		// left to finish on their own.
		m.active.Stop()
	}
	m.active = next
	m.lastMod = mod
	m.logger.Printf("loaded %d job(s) from %s", registered, m.path)
}

// shutdown stops the scheduler and waits for running jobs, bounded by grace.
func (m *manager) shutdown() {
	m.logger.Printf("shutdown requested, stopping scheduler")
	if m.active == nil {
		return
	}
	stopped := m.active.Stop()
	select {
	case <-stopped.Done():
		m.logger.Printf("all running jobs finished")
	case <-time.After(m.grace):
		m.logger.Printf("grace period (%s) expired, exiting with jobs still running", m.grace)
	}
}

// logErrOnce logs msg only when it differs from the previous error, so a
// persistent condition (e.g. a deleted crontab) does not flood the log on every
// poll tick.
func (m *manager) logErrOnce(msg string) {
	if msg == m.lastErr {
		return
	}
	m.lastErr = msg
	m.logger.Print(msg)
}
