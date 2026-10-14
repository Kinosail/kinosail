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
Subtitles amd64 independently confirms the same finding. Its private log has SHA-256 `ebb4904accfa3a0e9e8bf586b8dab90381a0e6de7f59264e13ac78dff7d84264`.

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
The correction is verified below; protected delivery and exact-main publication remain pending.

## Corrected image acceptance

At `d1f6dcf90f8b2f707a2866cee54c112067a653a4`, both apps pass production image builds, scans, and container journeys on amd64 and arm64.
Each retained build log confirms installation of `librsvg2-2` version `2.60.0+dfsg-1+deb13u1`.
Each system artifact verifies ten source pins, two checksum entries, unchanged clean source, and a successful command.
Image revision labels, command identity, and native runner architecture also verify.

| App | Architecture | Artifact | ZIP SHA-256 |
| --- | --- | --- | --- |
| Player | amd64 | `11863199262` | `832f40363a21941146427bde9c67e34b2b52b86358b5b1e42dfb4b7fd7a215f3` |
| Player | arm64 | `11862928457` | `f3cff50a5c0f4405875c3073d67754ac636a983363efad370742605286e4cf7f` |
| Subtitles | amd64 | `11864010198` | `2141f4ea2eb7cbad1dbe64a73d83ed85a533ca27d415968600060c11a41c7472` |
| Subtitles | arm64 | `11863463430` | `7ac798e12786ed17f1f53c55b2ffe223cbcacdf479a7edffa5bbaef07a65a9c2` |

These receipts establish hosted container acceptance. They do not establish publication, deployment, or physical playback.
