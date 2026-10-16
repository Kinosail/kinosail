---
title: Troubleshooting
description: Start from a Kinosail symptom and find the shortest safe diagnostic path.
section: Fix a problem
---

# Troubleshooting

Start with the visible symptom. Use the smallest diagnostic that can confirm the cause.

## Choose a symptom

- [Install and startup problems]({{ '/troubleshooting/install-and-startup/' | relative_url }}) covers containers, ports, HTTPS, health, storage, and startup.
- [Scanning and metadata problems]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }}) covers missing items, wrong matches, artwork, permissions, and providers.
- [Playback problems]({{ '/troubleshooting/playback/' | relative_url }}) covers unavailable playback, buffering, tracks, subtitles, seeking, and device differences.
- [Sign-in and access problems]({{ '/troubleshooting/sign-in-and-access/' | relative_url }}) covers passwords, passkeys, TOTP, profiles, Jellyfin apps, certificates, remote access, and shares.

## Use this safe order

1. Record the exact message, page, item title, device, and time.
2. Check whether the problem affects one item, one Profile, one device, or the whole Server.
3. Check **Settings → System** as an Owner. Review scan status, diagnostics, recent activity, sessions, and cache status.
4. Retry one time after a small, reversible change.
5. Collect a short log excerpt and remove secrets before sharing it.

Server logs use `debug`, `info`, `warn`, and `error`; the default is `info`. An Owner can temporarily set `logging.level` to `debug` in **Settings → Configuration** without restarting. Repeat the problem, then restore `info`. Match a native app's `request_id` or a browser response's `X-Request-ID` to the Server log's `request_id`. Keep the time, HTTP status, and route pattern; do not share raw URLs or tokens.

Do not paste passwords, passkeys, TOTP secrets, recovery codes, API keys, Media Share links, cookies, private hostnames, or full request URLs. Kinosail logs redact sensitive values, but review any excerpt yourself.

If an issue can affect data, stop before deleting a volume, library path, configuration file, or backup. Make a verified backup first.

## Collect performance diagnostics

Use this section when the Server is slow, uses too much memory, or stops under load.

The Owner metrics page reports Go runtime values. Look for `kinosail_go_heap_live_bytes`, `kinosail_go_memory_total_bytes`, `kinosail_go_memory_limit_bytes`, `kinosail_go_goroutines`, `kinosail_go_gc_pause_seconds`, and `kinosail_go_sched_latency_seconds`. The pause and latency values are totals since the Server started.

At startup, the Server reads the container memory limit. It sets the Go memory limit to 40% of that value, because FFmpeg and other tools share the same limit. The public HTTPS gateway uses 80%. The log line `Go memory limit configured` shows the value. To choose your own value, set the standard `GOMEMLIMIT` variable, for example `GOMEMLIMIT=1GiB`.

For deeper analysis, turn on the profiling socket. Profiling is off by default. It never opens a network port.

1. Add `KINOSAIL_PROFILING: "1"` to the `environment` list of the `kinosail` service in a Compose override file. Compose does not forward variables that a Compose file does not declare.
2. Restart the Server. The log shows `profiling socket listening`.
3. Find the socket at `profiling/pprof.sock` in the data directory. Only the Server account can open it.
4. On the host, as an account that can open the socket, save a profile. For a rootless Podman volume, run the command inside `podman unshare`.

   ```sh
   curl --unix-socket /path/to/data/profiling/pprof.sock --output heap.pprof http://localhost/debug/pprof/heap
   curl --unix-socket /path/to/data/profiling/pprof.sock --output cpu.pprof 'http://localhost/debug/pprof/profile?seconds=30'
   curl --unix-socket /path/to/data/profiling/pprof.sock --output flight.trace http://localhost/debug/pprof/flight-recorder
   ```

5. Open a profile with `go tool pprof heap.pprof`. Open a trace with `go tool trace flight.trace`.
6. Remove the variable and restart the Server when you are done.

The flight recorder keeps about the last 30 seconds of runtime activity. Save it right after a slow playback start. Profiles can contain file paths and titles. Review them before you share them.

Source of truth: current Server diagnostics and recovery paths.
