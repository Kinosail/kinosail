# Reliable automatic discovery of newly added media

Research snapshot: 2026-08-22. This note covers local and bind-mounted Library Content visible to the single Kinosail Server container. It does not add a hosted file index or sidecar, or assume that filesystem notifications are a durable event log.

## Decision

Use a **hybrid watcher plus reconciliation** design:

1. Keep a recursive directory watcher as the low-latency hint path.
2. Coalesce hints by affected directory, wait for writes to settle, and reconcile the smallest safe subtree.
3. Run a full reconciliation at startup and on a safety schedule.
4. Treat queue overflow, watch-limit exhaustion, unmounts, and unsupported filesystems as recoverable degraded states that force reconciliation and remain visible to the owner.
5. Serialize every watcher, scheduled, API, and web-triggered scan through one application operation and atomically publish only a complete successful result.

This gives local files near-real-time discovery without pretending that inotify is complete. It also gives NFS, SMB, FUSE, container-desktop mounts, missed events, and server downtime a correctness path through polling.

## How Kinosail sync works now

Kinosail currently performs a complete recursive scan when `libraryIndex` is constructed, then repeats a complete scan on the configured interval. The container default is ten minutes. The owner-facing **Sync** action and both web/API task routes also invoke the same full refresh. ([index construction and scheduler](../../packages/catalog/index.go), [composition and web Sync route](../../apps/player/internal/server/server.go), [container default](../../apps/player/cmd/kinosail/main.go), [versioned admin API](../../apps/player/internal/server/api_admin.go))

The scanner walks every configured root, recognizes supported media extensions, reads local metadata/artwork/subtitle sidecars, and deliberately skips symbolic links. A successful scan replaces the in-memory item list. ([scanner](../../packages/library/library.go))

What is missing is a realtime hint path. New media therefore appears only at startup, after a manual Sync, or at the next periodic scan. The present API already reports item count, last scan, schedule, and a scan-error boolean; that is the narrowest place to add watcher/reconciliation status. ([admin settings response](../../apps/player/internal/server/api_admin.go), [settings diagnostics](../../apps/player/internal/server/settings_http.go))

## Why watcher-only is not reliable

Linux inotify is a bounded, non-recursive notification queue, not a source of truth:

- watches must be added to every directory; a newly created or moved-in directory can already contain files before its watch is installed;
- identical unread events may be coalesced, so events cannot be counted as changes;
- excess queued events are dropped and only an `IN_Q_OVERFLOW` marker remains;
- a pathname may have been renamed or deleted before the consumer processes its event;
- remote changes on network filesystems are not reported and require polling;
- a filesystem mounted on top of an already watched directory generates no mount event and changes immediately beneath the new mount are not reported; and
- rename pairs are ordered but are not atomically enqueued or guaranteed to both be observable.

These are documented limitations of the kernel API, which explicitly advises robust applications to rebuild some or all cached state after overflow. [Linux `inotify(7)` queue limits and ordering](https://man7.org/linux/man-pages/man7/inotify.7.html#NOTES), [limitations, new-directory race, overflow, mounts, and renames](https://man7.org/linux/man-pages/man7/inotify.7.html#DESCRIPTION)

The upstream Go `fsnotify` package exposes overflow as `ErrEventOverflow`, does not recursively watch subdirectories, recommends watching directories rather than individual files because applications commonly replace files by rename, and warns that NFS/SMB/FUSE notifications are unavailable or filesystem-dependent. It also documents per-user Linux watch/instance limits and their `ENOSPC`/`EMFILE`-like failure modes. [fsnotify API](https://pkg.go.dev/github.com/fsnotify/fsnotify), [fsnotify FAQ and platform notes](https://github.com/fsnotify/fsnotify#faq)

Established media servers reach the same practical conclusion:

- Plex uses operating-system notifications to initiate scans and supports partial scans, but says network shares commonly do not work, advises periodic scanning as fallback, and suggests waiting until activity has finished. [Plex scanning versus refreshing](https://support.plex.tv/articles/200289306-scanning-vs-refreshing-a-library/), [Plex library settings](https://support.plex.tv/articles/200289526-library/)
- Jellyfin uses inotify for realtime monitoring on Linux, documents NFS/rclone limitations and host-level watch-limit tuning for containers, and has a configurable delay specifically because item creation is often non-atomic and spans several files/directories. [Jellyfin realtime-monitor troubleshooting](https://jellyfin.org/docs/general/administration/troubleshooting/#real-time-monitoring), [Jellyfin `LibraryMonitorDelay`](https://typescript-sdk.jellyfin.org/interfaces/generated-client.ServerConfiguration.html#librarymonitordelay)
- Emby describes realtime monitoring as available only on supported filesystems. [Emby library setup](https://support.emby.media/support/articles/Library-Setup.html#enable-real-time-monitoring)

The systems research supports coalescing and reconciliation rather than event-by-event indexing. A USENIX LISA paper reports that one file creation produces multiple notifications and temporary files create noise; its implementation pruned duplicates and flushed changes after an inactivity window. [Kang, Sharma, and Thanki, “RegColl,” LISA 2005](https://static.usenix.org/events/lisa05/tech/full_papers/kang/kang_html/index.html#Registry-and-Configuration-Change-Monitoring) A FAST 2014 study records that Dropbox used filesystem notifications for the fast path but recovered from notifier/app failure by comparing file size and modification time, with progress/state stored locally. [Zhang et al., “ViewBox,” FAST 2014](https://www.usenix.org/conference/fast14/technical-sessions/presentation/zhang) An IEEE Cluster 2019 paper independently identifies inotify's per-directory crawl/watch cost, non-recursion, overflow risk, and poor fit for distributed filesystems; its larger lesson is to select a detection mechanism appropriate to the storage backend. [Paul et al., “FSMonitor,” IEEE Cluster 2019](https://arnabkrpaul.github.io/publications/FSMonitor_cameraReady.pdf) These are analogous systems rather than media-library evaluations, but the reliability lesson directly applies: notifications narrow the work; observed filesystem state decides the index.

## Recommended lifecycle

### 1. Startup: establish coverage, then reconcile

For each configured library root:

1. Validate and canonicalize the root while preserving Kinosail's root-boundary rules.
2. Create one `fsnotify.Watcher` for the process and add the root first.
3. Walk directories, adding a watch for every real directory and never following symlinks.
4. Whenever a directory is created or moved in, add watches recursively and immediately enumerate that subtree. This closes the documented gap between directory creation and watch installation.
5. After initial watch registration, perform a full reconciliation while the watcher continues collecting hints. Process any accumulated dirty scopes after the baseline commits.

Starting the watcher before the baseline scan avoids the “scan, then start watching” blind window. The final full reconciliation catches changes that raced with recursive watch installation.

If watcher setup fails, the server should still start and complete the full scan. Mark that root `polling` or `degraded`, keep periodic reconciliation active, and retry watcher setup with capped backoff.

### 2. Events: hints become dirty scopes

Do not mutate the index directly from an event callback. Normalize the event path, reject paths outside a configured root, and add a scope to a deduplicating dirty set:

| Hint | Action |
| --- | --- |
| file create/write | Mark its containing directory dirty and enter the settle gate |
| file remove | Mark its containing directory dirty; no write-settle check is needed |
| rename/move | Mark both known parent directories dirty; do not depend on successfully pairing rename events |
| directory create/move-in | Add recursive watches, scan the new subtree immediately, then mark its parent dirty |
| directory remove/move-out | Remove known watches below it and mark the old parent dirty |
| `.nfo`, subtitle, lyrics, or artwork change | Mark the associated media directory dirty so the existing item is redecorated |
| pure chmod | Usually ignore; for a known indexed path, a cheap `lstat` can detect the Linux delayed-remove case |
| watcher error/unmount/overflow | Mark watcher degraded, schedule a full root reconciliation, then rebuild watches |

Collapse child scopes when an ancestor is already dirty. If the dirty set becomes large (for example, more than 100 directories or a meaningful fraction of a root), promote it to one full-root reconciliation. Exact thresholds should be measured, not treated as correctness rules.

### 3. Write completion: settle before indexing

`CREATE` means a directory entry exists, not that a multi-gigabyte copy is playable. Kinosail should use a short inactivity debounce followed by a readiness check:

- reset a per-scope quiet timer on relevant new events;
- after roughly 2–5 quiet seconds, sample each candidate's type, size, and nanosecond modification time;
- require the values to remain unchanged across a second sample;
- require the file to open for reading; and
- when media probing is required, retry transient probe failures instead of committing incomplete metadata.

A same-filesystem rename into its final pathname is an especially strong readiness hint because replacement of the destination is atomic, but it is still only a hint: cross-filesystem `mv` becomes copy-plus-delete and remote filesystems have weaker failure behavior. [Linux `rename(2)`](https://man7.org/linux/man-pages/man2/rename.2.html)

Do not advertise “write safely stored” based on close or inactivity. Linux notes that a successful close does not guarantee persistence to disk; Kinosail only needs a stable readable representation for indexing, not a durability guarantee that the producer did not request. [Linux `close(2)` caveat](https://man7.org/linux/man-pages/man2/close.2.html#CAVEATS)

Use a generous retry horizon with backoff for files that are still growing or temporarily unprobeable. Keep them out of the visible library until ready and expose a pending count; never publish a knowingly partial item just to meet a maximum latency.

### 4. Reconcile observed state, atomically

The reconciler—not the watcher—is authoritative. For a dirty scope it should:

1. enumerate the smallest safe subtree;
2. build fingerprints from relative path, kind, size, and high-resolution modification time (plus device/inode where useful locally);
3. run the existing media/sidecar scanner for added or changed candidates;
4. calculate additions, updates, removals, and derived group changes;
5. build a complete candidate snapshot; and
6. swap it into the live index only if the reconciliation succeeds.

Do not hash every media file on each scan; full-file hashing makes safety scans scale with media bytes rather than directory entries. Existing download snapshots can continue to own strong representation identity separately.

For the first implementation, “targeted” can mean one containing directory or logical show/album folder while the in-memory derived views are recomputed from the merged item slice. That reuses the current scanner and avoids introducing a database. A persistent fingerprint manifest under `/config` can be considered later if startup scans become measurably expensive.

All triggers must feed one serialized coordinator. If a scan is running, merge new dirty scopes and run another pass afterward. Manual full scan supersedes pending targeted scopes. This prevents a slower, older scan from overwriting a newer result and satisfies Kinosail's shared-operation rule for API and web adapters.

### 5. Reconciliation remains the safety net

Keep a full startup reconciliation and the current ten-minute periodic default for the first release. It is conservative but already configurable and is the correctness path for event loss and network mounts. Later measurements can justify a longer healthy-local interval and a shorter degraded/network interval.

“Off” should explicitly mean **periodic safety reconciliation off**, not “realtime watcher is guaranteed.” Manual Sync must remain available. On NFS, SMB, many FUSE mounts, Docker Desktop file sharing, or any root whose watcher cannot be established, the UI should recommend leaving periodic reconciliation enabled.

On overflow:

1. record `ErrEventOverflow` and set the root to `degraded` immediately;
2. coalesce any outstanding targeted work into a full-root reconciliation;
3. close/recreate the watcher and rebuild directory watches;
4. atomically commit the full result; and
5. return to `watching` only after both reconciliation and watcher rebuild succeed.

Increasing host `fs.inotify.max_user_watches`, `max_user_instances`, or `max_queued_events` may reduce failures but must never replace overflow recovery. Jellyfin correctly notes that containerized Linux servers share these limits with the host and they must be changed on the host. [Jellyfin host-level guidance](https://jellyfin.org/docs/general/administration/troubleshooting/#real-time-monitoring)

## Symlinks, mounts, and container behavior

Preserve the scanner's current policy: do not traverse media or directory symlinks. This avoids loops and root escapes, and keeps the watcher and scanner consistent. If media outside `/media` should be included, the owner should bind-mount it beneath `/media` and configure it as an explicit library root rather than create a symlink escape.

Kinosail's Compose file bind-mounts host media read-only at `/media`. A Linux bind mount exposes host files through ordinary container filesystem operations, so a supported local host filesystem can deliver inotify events inside the container. Docker also documents important boundaries: Docker Desktop interposes a Linux VM, recursive submount inclusion is configurable, and bind propagation defaults to `rprivate`; hot-added host submounts therefore must not be assumed to appear or generate notifications. [Docker bind-mount documentation](https://docs.docker.com/engine/storage/bind-mounts/)

Podman likewise defaults bind propagation to private and says ordinary bind mounts do not include source submounts; `rslave` permits one-way host-to-container mount propagation only when the source mount has compatible propagation. Prefer binding each intended media filesystem explicitly before server startup. Do not add Podman's `:U` option to a media tree automatically: it recursively changes source ownership. Surface permission/SELinux failures instead. [Podman bind mounts, ownership, and propagation](https://docs.podman.io/en/latest/markdown/podman-run.1.html#volume-v-source-volume-host-dir-container-dir-options)

Practical policy:

- mount all expected media before the Kinosail container starts;
- use the existing read-only media bind;
- enumerate directory watches again after every full reconciliation;
- never claim realtime support solely because `Watcher.Add` succeeded; and
- show degraded/polling status when delivery cannot be demonstrated or the backend is known unsupported.

No extra container, privileged mode, host PID namespace, or Docker socket is needed.

## API, web, diagnostics, and metrics

Extend the existing owner-only `GET /api/v1/settings` response rather than create a parallel control plane. At minimum expose:

```json
{
  "librarySync": {
    "state": "idle",
    "mode": "watching",
    "pendingPaths": 0,
    "watchedDirectories": 431,
    "lastEvent": "2026-08-22T20:11:04Z",
    "lastReconcile": "2026-08-22T20:11:09Z",
    "lastFullReconcile": "2026-08-22T20:00:00Z",
    "lastError": ""
  }
}
```

Use bounded enums:

- `state`: `starting`, `idle`, `settling`, `scanning`;
- `mode`: `watching`, `polling`, `degraded`.

Keep `POST /api/v1/tasks/scan` as the full manual reconciliation. The Settings page must render the same operation and status, with plain wording such as “Watching for changes,” “Waiting for file copy to finish,” or “Realtime detection unavailable; periodic scans are active.” Do not report the overall server unhealthy merely because realtime watching is degraded when periodic scans are succeeding, but do make the degraded mode visible.

Add safe diagnostics and metrics without paths:

- `kinosail_library_sync_state{mode="watching|polling|degraded"} 1`;
- `kinosail_library_pending_paths`;
- `kinosail_library_watched_directories`;
- `kinosail_library_reconciliations_total{kind="targeted|full",result="success|error"}`;
- `kinosail_library_watch_overflows_total`; and
- duration/last-success timestamps for targeted and full reconciliation.

Never include owner filesystem paths in API responses, metrics labels, or ordinary diagnostics.

## Recommended implementation sequence

1. Introduce a single-flight/coalescing scan coordinator around the existing index refresh operation. Route scheduled, API, web, and watcher work through it.
2. Add the current stable `github.com/fsnotify/fsnotify` release (v1.10.1 at this snapshot) and recursive real-directory watch registration inside the existing Server process; keep the scanner's symlink policy. [fsnotify releases](https://github.com/fsnotify/fsnotify/releases/tag/v1.10.1)
3. Add dirty-scope coalescing, settle/readiness checks, and targeted subtree reconciliation.
4. Add overflow/watch-failure recovery and retry, retaining the existing ten-minute full safety scan.
5. Extend the existing versioned settings API, Settings UI, diagnostics, and metrics with watcher/reconciliation status.

This order establishes concurrency correctness before adding a new trigger and keeps the supported installation at exactly one Kinosail Server container.

## Verification matrix

Focused automated coverage should prove:

- startup finds existing nested media and captures a file created during watcher registration;
- a chunk-written large file stays pending and appears only after stability;
- temp-file-plus-rename appears quickly;
- a pre-populated new directory is watched and reconciled without missing its children;
- rename within a directory, across watched directories, into a root, and out of a root leaves no duplicate/stale item;
- sidecar creation/update/removal refreshes the owning item;
- delete while a stream still has the file open is eventually reconciled;
- symlink files/directories and root escapes remain excluded;
- injected `ErrEventOverflow` forces full reconciliation and watcher rebuild;
- injected watch-limit/setup errors degrade to polling without preventing server startup;
- events arriving during a scan cause a follow-up pass;
- simultaneous manual, periodic, and watcher triggers cannot publish out of order;
- a failed targeted/full scan retains the last complete good snapshot and exposes the error;
- disabling periodic scans does not disable watcher-triggered reconciliation;
- versioned API and web adapters expose the same status and invoke the same manual operation; and
- diagnostics/metrics contain no media paths.

Run the Go suite with the race detector for the coordinator and add a Linux/Podman integration test that writes into the host side of the existing read-only `/media` bind mount. Separately test a no-event fake watcher to prove periodic recovery. Actual NFS/SMB/FUSE and Docker Desktop behavior remains an environment-specific verification boundary; the product guarantee there is reconciliation, not realtime notification.
