# Subtitles audit (2026-10-30)

Revision inspected: `a9311482`. Scope: `apps/subtitles` server, web app, and docs. The Owner asked for a cleanup, UI polish, better defaults, and bug fixes.

Each finding has a status: **fixed** (with the pull request), **planned** (with the work item), or **open**.

## How the audit was done

- Three read-only code reviews: architecture and duplication, correctness, and UI with defaults. Each finding lists a path and line.
- Live checks on Nox (`kinosail-subtitles-dev`): logs, provider health, and stored configuration.
- Each fixed bug first gets a failing regression test. The fix follows.

## Architecture

- `internal/server` is one flat package: 211 production files, 18,127 lines.
- 113 files share a name with a Player file. About 4,845 of their 6,330 lines are the same. 37 files are byte-identical. These are auth, profiles, sessions, the settings shell, locale, and configuration. **Planned:** [Stage 3](../player-feature-removal-plan.md#stage-3-shared-code) moves them to `packages/`.
- Player-only settings fields still load in Subtitles: playback, transcoding, Jellyfin, Home Assistant, and DLNA. **Planned** with the stored-settings migration.
- Providers have no shared abstraction:
  - SubDL is built into the `subtitleProvider` god object.
  - OpenSubtitles and SubSource each have their own HTTP, health, download, and archive code.
  - Scoring rules differ between providers.
  - The provider list is hard-coded in at least 7 places.
  - **Planned:** one `subtitleSource` interface with one HTTP client.
- Five call sites repeat the sidecar install steps by hand: keep the original, back up, write, record history, roll back. The rollback errors are discarded. **Planned:** one install transaction.
- Dead code: five empty files, test-only aliases, and variadic "optional" dependencies that only tests omit. **Planned.**
- Provider and sidecar code writes no logs, so provider failures are invisible. **Planned** with the provider work.

## Correctness bugs

| # | Bug | Status |
|---|---|---|
| 1 | An untagged `Movie.srt` counted as every preferred language. Automation never fetched the second language, and the manual view disagreed with it. | **Fixed** (this PR) |
| 2 | With the SDH preference and a standard `Movie.en.srt` present, automation retried every 15 minutes and recorded no backoff. | **Fixed** (this PR): an occupied sidecar records an `exists` backoff. The SDH naming is planned with the defaults work. |
| 3 | Legacy codepage subtitles (cp1250 Czech, cp1256 Farsi) are rejected before language-aware decoding runs. SubSource rejects every file that is not UTF-8. | Planned |
| 4 | New sidecars are created with mode 0600, and upgrades replace a 0644 file with a 0600 one. Media servers running as another user cannot read them. | Planned |
| 5 | A 404 or 5xx on one download link blocks the whole provider, so the remaining candidates are skipped. | Planned |
| 6 | A partial provider outage is recorded as "no result", which gives a 6-hour to 7-day backoff instead of minutes. | Planned |
| 7 | Candidates are cut to 3 before the SDH preference and the SubDL quota check run. | Planned |
| 8 | SubDL episode searches send the episode's year and IDs instead of the show's. | Planned |
| 9 | An OpenSubtitles response with more than 50 results is rejected completely. | Planned (needs an API check) |
| 10 | Exclusive writes need hard links. They fail on SMB, exFAT, and some FUSE mounts, and leftover temporary files are never cleaned up. | Planned |
| 11 | Sidecars the household created can be replaced automatically. CONTEXT.md says they never are. | Planned with the defaults: never replace by default |
| 12 | SubDL has no forced-subtitle filter. | Planned |
| 13 | SubSource episode names raise the release match enough to skip sync validation. | Planned |
| 14 | Full language names such as `Movie.English.srt` were not recognized, so a duplicate `Movie.en.srt` was added. | **Fixed** (this PR) |
| 15 | The OpenSubtitles 406 download quota gets a 1-minute backoff, and a stale token is not cleared. | Planned |

## UI and defaults

- The CSS is Player's stylesheet plus a patch with dead rules. Five rules conflict, colors are hard-coded, and breakpoints differ from Player's. Planned.
- The dashboard does not show running work, the next automation run, provider errors as problems, or subtitles installed with unverified timing. Planned.
- Defaults that differ from the Owner's target:
  - Languages are English only and ignore the UI language.
  - The SDH preference is strict, with no fallback.
  - There is no provider order, and no provider works without a key.
  - Sidecars the household created can be replaced.
  - Planned.
- Docs drift: the picker default, "restart when prompted", first-setup steps, and doc file names left over from Player. Planned.

## Live state on Nox (2026-10-30)

- No warnings or errors in the last 72 hours of logs.
- OpenSubtitles, SubDL, and SubSource all report recent successes.
- The cache held 125 MB, with no leftover OCR work folders.
