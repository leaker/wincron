# wincron

A small, single-binary cron scheduler for Windows, designed to be run as a
service via [nssm](https://nssm.cc/). It is a Go reimplementation of the classic
Python `crontab.py`, using **standard cron semantics** (the `robfig/cron`
parser) plus automatic crontab hot-reload, per-job exit-code logging, and
size-based log rotation.

It builds and runs on macOS/Linux too (commands run through `/bin/sh`), which
makes local development and testing easy; the production target is Windows
(commands run through `cmd /C`).

## Features

- **Standard cron syntax** — 5-field expressions (`*/5 * * * *`), descriptors
  (`@daily`, `@hourly`, …), and intervals (`@every 90s`). Weekday `0 = Sunday`.
- **Hot reload** — edit `crontab.txt` and the schedule reloads automatically; no
  restart required.
- **Exit-code & failure logging** — each run logs its exit code and duration.
  On failure it logs the *specific reason*: the captured `stderr` for a non-zero
  exit, or the OS error if the command could not start.
- **Log rotation** — logs go to both stdout (captured by nssm) and a rotating
  file (`lumberjack`), so long-running services never fill the disk.
- **Graceful shutdown** — handles the Ctrl-C that nssm sends on stop, waiting
  (bounded) for in-flight jobs to finish.

## Install

**Scoop (Windows):**

```
scoop bucket add leaker https://github.com/leaker/scoop-bucket
scoop install wincron
```

`scoop update wincron` picks up new releases. Check the installed version with
`wincron -version`.

Or grab `wincron-<version>-windows-amd64.zip` from the
[Releases](https://github.com/leaker/wincron/releases) page and extract
`wincron.exe`. To build from source, see [Build](#build) below.

After installing via Scoop, register the real binary with nssm (point at the
versioned path under `apps`, not the shim, so the service survives `scoop
update` cleanly):

```bat
nssm install wincron "%USERPROFILE%\scoop\apps\wincron\current\wincron.exe"
nssm set wincron AppDirectory "C:\wincron"
nssm set wincron AppParameters "C:\wincron\crontab.txt"
```

(`...\current\...` is a junction Scoop repoints on each update, so this path
keeps working across upgrades.)

## Crontab format

```
# minute hour day-of-month month day-of-week  command
*/5 * * * * echo tick >> C:\logs\tick.txt
30 2 * * * C:\scripts\backup.bat
0 9 * * MON-FRI C:\scripts\report.exe --daily
@hourly C:\scripts\sync.cmd
@every 90s C:\scripts\heartbeat.exe
```

See [`crontab.example.txt`](crontab.example.txt) for the full reference. Lines
starting with `#` and blank lines are ignored. The command is everything after
the schedule and runs through `cmd /C` on Windows.

## Build

Build for the current platform:

```sh
go build -o wincron .
```

Cross-compile for Windows from macOS/Linux:

```sh
# 64-bit
GOOS=windows GOARCH=amd64 go build -o wincron.exe .
# 32-bit
GOOS=windows GOARCH=386 go build -o wincron.exe .
```

## Usage

```
wincron [flags] [crontab-file]
```

If `crontab-file` is omitted, `crontab.txt` next to the executable is used.

| Flag | Default | Description |
|------|---------|-------------|
| `-log` | `<exe dir>/wincron.log` | log file path |
| `-reload` | `15s` | crontab reload poll interval |
| `-grace` | `30s` | max wait for running jobs on shutdown |
| `-tz` | `Local` | timezone (`Local`, `UTC`, `Asia/Shanghai`, …) |
| `-log-max-size` | `10` | rotate the log after this many MB |
| `-log-max-backups` | `5` | rotated log files to keep |
| `-log-max-age` | `30` | max age (days) for rotated logs |
| `-log-compress` | `true` | gzip rotated logs |

Run directly to test:

```sh
wincron -reload 2s crontab.txt
```

## Run as a Windows service with nssm

1. Place `wincron.exe` and `crontab.txt` in a folder, e.g. `C:\wincron\`.

2. Install the service (run as Administrator):

   ```bat
   nssm install wincron "C:\wincron\wincron.exe"
   nssm set wincron AppDirectory "C:\wincron"
   nssm set wincron AppParameters "C:\wincron\crontab.txt"
   nssm set wincron Start SERVICE_AUTO_START
   nssm set wincron AppStopMethodConsole 30000
   ```

   - `AppDirectory` sets the working directory so relative paths in your
     commands resolve predictably.
   - `AppStopMethodConsole` tells nssm to send Ctrl-C on stop and wait up to 30s
     for a graceful exit (wincron handles this and finishes running jobs).

   nssm already writes the binary's stdout to its own log if you configure
   `AppStdout`/`AppStderr`; wincron *also* keeps its own rotating
   `wincron.log`, so log redirection in nssm is optional.

3. Start / stop / inspect:

   ```bat
   nssm start wincron
   nssm status wincron
   nssm stop wincron
   nssm remove wincron confirm
   ```

After it's running, you can edit `C:\wincron\crontab.txt` at any time — wincron
detects the change and reloads the schedule within the `-reload` interval.

## Notes & differences from the original Python `crontab.py`

- **Standard semantics.** The original used non-standard weekday numbering
  (`0 = Monday`) and idiosyncratic `@`-keyword mappings; wincron follows
  standard cron (`0 = Sunday`, standard `@daily`/`@weekly`/… meanings).
- **No `$crontab.bin` / pid file.** The cron parser handles descriptors
  directly, and nssm owns the service lifecycle, so those intermediate files
  are gone.
- **Overlapping runs are allowed.** Like the original's `start`-based spawning,
  a job may start again before a previous slow run finishes (each invocation
  runs in its own goroutine).
