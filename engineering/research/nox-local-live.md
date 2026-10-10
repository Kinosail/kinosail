# Direct Nox development updates

The owner requested direct updates from saved local source, without waiting for GitHub CI or publication.
This development path intentionally accepts uncommitted source. It does not change PR or publication checks.

## Failure cases identified before implementation

- A burst of saves must produce one update after the debounce interval.
- Changes during a build must prevent that snapshot from replacing the running service.
- A failed build, SSH command, or health check must preserve the last healthy deployment.
- Runtime changes must rebuild the complete image. Binary updates must use a fixed runtime base to avoid growing image layers.
- Concurrent watchers must not deploy competing snapshots.
- Deleted files must change the source fingerprint. Ignored credentials and source symlinks must not enter a snapshot.
- Shared source changes must update both apps. An app-only change must leave the other app alone.
- Logs must identify the app, snapshot, and failure class without printing source, credentials, or remote logs.
- The updater must restart at login and recover from temporary SSH failures.
- Reinstalling must tolerate the short delay between unloading and restarting the macOS agent.
- A private local umask must still produce a binary executable by the container's unprivileged user.
- The login agent must use the existing runtime permissions for source reads and local network access.

## Isolated coverage gap

The local watcher process tests cover save timing, exclusion of ignored credentials, deletion,
symlink rejection, and superseded builds. A normal live deployment cannot reliably reproduce
those races or deliberately break the shared Nox service. The existing remote deployment
tests protect health-check rollback. These checks are isolated evidence, not populated-server E2E proof.
The executable-mode regression protects the image payload in CI, which cannot access the owner's Nox host.

## Operation

Run `make nox-live` in the checkout you want to test. Press Control-C to stop it.
Run `make nox-live-install` to watch that checkout automatically after login.
Run `make nox-live-stop` to stop the installed watcher.

The watcher waits three seconds after the last save. It includes nonignored source files,
including new files, and excludes tests, ignored files, and dotfiles.
It watches Player, Subtitles, and shared packages in that checkout only.
Use `make nox-live-install` from another worktree to change the watched checkout.

The first update builds a complete ARM64 image on the Mac. Runtime input changes repeat this build.
Other updates compile the Go binary locally, transfer it over SSH, and assemble an image on Nox.
The binary includes embedded web assets. Nox checks container health and rolls back a failed update.
Each update can briefly interrupt playback or an active Subtitles operation.

The macOS login agent uses `/usr/bin/python3` for deployment and the installer's `python3`
for source capture. On the verified Mac, Homebrew Python could read Documents but could not
reach Nox as an agent. Apple's Python could reach Nox but could not read this Documents checkout.
The source helper and deployment process use those existing permissions. No broader disk grant was added.
[Apple describes the different privacy rules for agents and terminal tools](https://developer.apple.com/documentation/technotes/tn3179-understanding-local-network-privacy).

The source snapshot ID appears in the container revision label. The separate
`io.kinosail.source.commit` label identifies its base Git commit. These development snapshots
have not passed hosted CI, image scanning, signing, browser checks, or release promotion.
They do not update GHCR. The Mac must be awake and connected to Nox.

Private build logs and snapshot evidence stay under `~/Library/Caches/KinosailNoxLocal`.
Do not share raw logs. Inspect only a bounded safe projection when reporting status.

## Live verification

With the watcher running, use this manual test from the implementation checkout:

```sh
python3 scripts/tooling/test-nox-local-e2e.py --repo /path/to/watched/kinosail \
  --watcher-mode login-agent --output /private/path/to/evidence
```

The test saves temporary source canaries, verifies deployed snapshot IDs and binary versions,
and removes the canaries. It then restores the original source snapshot.
It tests failed-image rollback in a disposable Compose project with separate volumes and no host ports.
The JSON report records revisions, environment, timings, results, and private log checksums.
This test restarts the live apps. Run it when brief interruptions are acceptable.
