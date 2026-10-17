---
title: Troubleshoot Subtitles
description: Start with the visible symptom and preserve your data.
section: Fix a problem
last_reviewed: 2026-09-15
---

# Troubleshoot Subtitles

| Symptom | First check |
| --- | --- |
| Server will not start | Read container logs; check the media path, port, configuration, and mounted secret-file paths. |
| No videos appear | Check the container's `/media` mapping, selected relative folders, permissions, and scan status. |
| No provider results | Confirm credentials, language, episode/release identity, and provider quota. A configured provider may have no match. |
| Subtitle cannot be saved | Check write permission for the container account and whether an existing sidecar is protected. |
| Subtitle language/timing is wrong | Confirm the selected language and release; inspect the result in a player before accepting a replacement. |
| A setting is read-only | Change its managing environment or YAML value, then restart if required. |

From `apps/subtitles/`, inspect `docker compose ps` and `docker compose logs --tail 100 kinosail` (or Podman equivalents). Keep credentials, private titles/paths, and backup files out of reports.

Server logs use `debug`, `info`, `warn`, and `error`; the default is `info`. An Owner can temporarily set `logging.level` to `debug` in **Settings → Configuration** without restarting. Repeat the problem, then restore `info`. Match a browser response's `X-Request-ID` to the Server log's `request_id`, status, and route pattern. Remove secrets and private details before sharing a log excerpt.

- [Installation and startup]({{ '/troubleshooting/install-and-startup/' | relative_url }})
- [Scanning and matching]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }})
- [Subtitle timing and saves]({{ '/troubleshooting/playback/' | relative_url }})
- [Sign-in and access]({{ '/troubleshooting/sign-in-and-access/' | relative_url }})

## Collect performance diagnostics

Use this section when the Server is slow, uses too much memory, or stops under load.

The Owner metrics page reports Go runtime values. Look for `kinosail_go_heap_live_bytes`, `kinosail_go_memory_total_bytes`, `kinosail_go_memory_limit_bytes`, `kinosail_go_goroutines`, `kinosail_go_gc_pause_seconds`, and `kinosail_go_sched_latency_seconds`. The pause and latency values are totals since the Server started.

At startup, the Server reads the container memory limit. It sets the Go memory limit to 40% of that value, because whisper.cpp, Tesseract, and FFmpeg share the same limit. The public HTTPS gateway uses 80%. The log line `Go memory limit configured` shows the value. To choose your own value, set the standard `GOMEMLIMIT` variable, for example `GOMEMLIMIT=1GiB`.

For deeper analysis, turn on the profiling socket. Profiling is off by default. It never opens a network port.

1. Add `KINOSAIL_PROFILING: "1"` to the `environment` list of the `kinosail` service in a Compose override file. Compose does not forward variables that a Compose file does not declare.
2. Restart the Server. The log shows `profiling socket listening`.
3. Find the socket at `profiling/pprof.sock` in the data directory. Only the Server account can open it. If that path is longer than 103 characters, the Server uses a private temporary folder instead. The `socket` field of the `profiling socket listening` log line shows that path inside the container.
4. On the host, as an account that can open the socket, save a profile. For a rootless Podman volume, run the command inside `podman unshare`.

   ```sh
   curl --unix-socket /path/to/data/profiling/pprof.sock --output heap.pprof http://localhost/debug/pprof/heap
   curl --unix-socket /path/to/data/profiling/pprof.sock --output cpu.pprof 'http://localhost/debug/pprof/profile?seconds=30'
   curl --unix-socket /path/to/data/profiling/pprof.sock --output flight.trace http://localhost/debug/pprof/flight-recorder
   ```

5. Open a profile with `go tool pprof heap.pprof`. Open a trace with `go tool trace flight.trace`.
6. Remove the variable and restart the Server when you are done.

The flight recorder keeps about the last 30 seconds of runtime activity. Profiles can contain file paths and titles. Review them before you share them.

Do not delete the state volume to fix sign-in or startup. Restore deliberately from a known backup; see [recovery]({{ '/owner-guide/backups-and-updates/' | relative_url }}).
