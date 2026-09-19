# Third-party notices

This file identifies third-party material distributed with Kinosail. The
listed license files control. Kinosail's license does not replace them.

| Component | Version | License or terms | Notice |
| --- | --- | --- | --- |
| hls.js | 1.7.1 | Apache License 2.0 | [`third_party/hls.js/LICENSE`](third_party/hls.js/LICENSE) |
| HTMX | 2.0.10 | Zero-Clause BSD | [`third_party/htmx/LICENSE`](third_party/htmx/LICENSE) |
| SecLists test fixture | Pinned repository revision | MIT | [`third_party/seclists.LICENSE`](third_party/seclists.LICENSE) |
| Jellyfin FFmpeg runtime | 8.1.2-5 | Upstream and Debian package terms | [Jellyfin FFmpeg](https://github.com/jellyfin/jellyfin-ffmpeg) |
| libarchive tools | Debian runtime package | BSD-2-Clause and Debian package terms | [libarchive](https://github.com/libarchive/libarchive) |

SecLists is used only by repository tests and is not included in the runtime
image. Jellyfin FFmpeg is installed in the runtime image from its signed,
checksum-verified upstream Debian package. The package's own notices remain
authoritative and are installed by the package manager.

The Debian base image and runtime packages retain their own upstream license
terms. A complete commercial distribution review must inventory those package
licenses and any later dependency changes.

libarchive tools are used for bounded CB7/7z and CBT/tar comic reads. RAR/CBR
support is not enabled.

Private management includes the following Go dependencies. Their license files
are included in the shared third-party notices directory in the container.

| Component | Version | License | Notice |
| --- | --- | --- | --- |
| wireguard-go | ecfc5a8d5446 (2026-05-22) | MIT | [LICENSE](../../packages/third_party/wireguard/LICENSE) |
| gVisor userspace network stack | 39ed1f5ac29c (2025-05-03) | Apache-2.0 | [LICENSE](../../packages/third_party/gvisor/LICENSE) |
| Google B-tree | 1.1.2 | Apache-2.0 | [LICENSE](../../packages/third_party/btree/LICENSE) |
| Wintun Go adapter (Windows builds) | 0fa3db229ce2 (2023-01-26) | MIT | [LICENSE](../../packages/third_party/wintun/LICENSE) |
