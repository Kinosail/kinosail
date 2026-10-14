# Production image librsvg security repair

Hosted image scans at `660bc08747da33aa640ef169a4c50fbfc02a74d1` reject `CVE-2026-96889` with HIGH severity.
The affected package is `librsvg2-2`, installed as `2.60.0+dfsg-1`.
[Debian's advisory](https://security-tracker.debian.org/tracker/CVE-2026-96889) identifies `2.60.0+dfsg-1+deb13u1` as fixed for trixie.
The advisory concerns a use-after-free when parsing crafted SVG content.
This finding does not establish exploitation or application-level reachability.

## Retained failing evidence

Run: [38493862973](https://github.com/Kinosail/kinosail/actions/runs/38493862973).

| Image | Job | Private log SHA-256 |
| --- | --- | --- |
| Player amd64 | `115625127597` | `d10cbbc5b835c267c9463ff072c51d6f02dfdba184b522a5f78bf32a41659a16` |
| Subtitles arm64 | `115625127533` | `7bd707026c99b15455e0ba7091df77af8a7de50e9b404ca3414256492fc2a36b` |

Both scanner tables identify the same installed and fixed versions.
Build logs show cached package-install layers: Player step 22 and Subtitles steps 22 and 27.
An unchanged cached layer does not rerun its existing `apt-get update` and `apt-get upgrade` commands.
Subtitles amd64 also fails its scan; that log has not yet been independently classified.

## Correction and acceptance

Both production Containerfiles now require the fixed minimum version after installing FFmpeg and its dependencies.
Changing that package-install layer invalidates its old cache entry and refreshes package metadata.
The version comparison fails the build if the fixed package is unavailable or an older version remains.
The security scan remains mandatory, with unchanged severity thresholds and no exception for this finding.

The existing hosted scan failure is the pre-change regression evidence.
No implementation-mirroring unit test is added for the package-version line.
POSIX shell syntax checks pass for every RUN command in both Containerfiles.
Source caps and whitespace checks pass. Local container execution remains held.

Acceptance requires new amd64 and arm64 builds, successful vulnerability scans, and affected container journeys for both apps.
Exact-main image publication, deployed versions, and physical playback remain separate facts.
The corrected images have not yet been built or scanned.
