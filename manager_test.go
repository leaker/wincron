package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func newTestManager(path string, buf *bytes.Buffer) *manager {
	return &manager{
		path:     path,
		logger:   log.New(buf, "", 0),
		reload:   time.Hour,
		grace:    time.Second,
		location: time.UTC,
	}
}

func writeCrontab(t *testing.T, path, content string, mod time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

// Valid entries register; a cron-invalid expression and a schedule-without-command
// are both skipped (logged), and the rest still load.
func TestCheckAndReloadRegistersAndSkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab.txt")
	content := "0 0 * * 0 echo weekly\n" + // valid
		"@hourly echo hourly\n" + // valid
		"99 5 * * * echo bad-minute\n" + // cron-invalid (minute 99) -> skipped at AddFunc
		"@daily\n" // schedule without command -> skipped at parse

	var buf bytes.Buffer
	writeCrontab(t, path, content, time.Now())
	m := newTestManager(path, &buf)
	m.checkAndReload()
	defer m.active.Stop()

	if got := len(m.active.Entries()); got != 2 {
		t.Fatalf("registered %d jobs, want 2", got)
	}
	logs := buf.String()
	if !strings.Contains(logs, "skip invalid schedule") {
		t.Errorf("expected an invalid-schedule skip log, got:\n%s", logs)
	}
	if !strings.Contains(logs, "schedule without command") {
		t.Errorf("expected a schedule-without-command skip log, got:\n%s", logs)
	}
}

// "0 0 * * 0" must fire on Sunday — standard cron semantics (0=Sunday), NOT the
// original Python tool's non-standard 0=Monday. This is the headline design
// decision, so it gets an explicit guard.
func TestStandardWeekdaySemantics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab.txt")
	writeCrontab(t, path, "0 0 * * 0 echo sunday\n", time.Now())

	var buf bytes.Buffer
	m := newTestManager(path, &buf)
	m.checkAndReload()
	defer m.active.Stop()

	entries := m.active.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	// 2026-06-17 is a Wednesday; the next "0 0 * * 0" must be Sunday the 21st.
	ref := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	next := entries[0].Schedule.Next(ref)
	if next.Weekday() != time.Sunday || next.Day() != 21 {
		t.Fatalf("0 0 * * 0 next run = %s (%s); want Sunday 2026-06-21 (standard 0=Sunday)",
			next.Format("2006-01-02"), next.Weekday())
	}
}

// A reload reflects both added and removed jobs.
func TestReloadPicksUpAddAndRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab.txt")
	base := time.Now()

	var buf bytes.Buffer
	m := newTestManager(path, &buf)

	writeCrontab(t, path, "0 0 * * 0 echo a\n", base)
	m.checkAndReload()
	if got := len(m.active.Entries()); got != 1 {
		t.Fatalf("initial load: %d jobs, want 1", got)
	}

	writeCrontab(t, path, "0 0 * * 0 echo a\n0 1 * * 0 echo b\n@hourly echo c\n", base.Add(time.Minute))
	m.checkAndReload()
	if got := len(m.active.Entries()); got != 3 {
		t.Fatalf("after add: %d jobs, want 3", got)
	}

	writeCrontab(t, path, "@hourly echo c\n", base.Add(2*time.Minute))
	m.checkAndReload()
	defer m.active.Stop()
	if got := len(m.active.Entries()); got != 1 {
		t.Fatalf("after remove: %d jobs, want 1", got)
	}
}

// If the crontab disappears (stat fails) the previously loaded schedule keeps
// running rather than the process dying.
func TestMissingFileKeepsSchedule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab.txt")
	writeCrontab(t, path, "0 0 * * 0 echo a\n", time.Now())

	var buf bytes.Buffer
	m := newTestManager(path, &buf)
	m.checkAndReload()
	prev := m.active
	defer prev.Stop()

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	m.checkAndReload()

	if m.active != prev {
		t.Fatal("scheduler was replaced after a stat error")
	}
	if got := len(m.active.Entries()); got != 1 {
		t.Fatalf("schedule changed after stat error: %d jobs, want 1", got)
	}
	if !strings.Contains(buf.String(), "stat crontab") {
		t.Errorf("expected a stat-error log, got:\n%s", buf.String())
	}
}

// If the crontab can be stat'd but not read (here: the path becomes a
// directory), the previous schedule is retained and the error is logged.
//
// Skipped on Windows: this simulates the read error by replacing the file with
// a directory at the same path, which is unreliable there. Windows file-system
// tunneling restores the deleted file's timestamps to a same-named replacement
// created within ~15s, so the manager's mtime check sees "no change" and never
// reaches the read, making the test flaky. The read-error branch itself is
// OS-independent (covered here on Unix), and the realistic "crontab disappears"
// case is covered portably by TestMissingFileKeepsSchedule.
func TestReadErrorKeepsSchedule(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory-as-file read-error simulation is unreliable on Windows (file-system tunneling)")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab.txt")
	writeCrontab(t, path, "0 0 * * 0 echo a\n", time.Now())

	var buf bytes.Buffer
	m := newTestManager(path, &buf)
	m.checkAndReload()
	prev := m.active
	defer prev.Stop()

	// Replace the file with a directory at the same path: Stat succeeds, ReadFile fails.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	m.checkAndReload()

	if m.active != prev {
		t.Fatal("scheduler was replaced after a read error")
	}
	if !strings.Contains(buf.String(), "keeping current schedule") {
		t.Errorf("expected a keeping-current-schedule log, got:\n%s", buf.String())
	}
}
