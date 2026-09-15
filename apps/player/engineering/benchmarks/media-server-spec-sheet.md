# Media server comparison

Status: Kinosail, Jellyfin, and Plex have completed local Podman benchmark passes.

Date: 2026-08-30

## Executive summary

Kinosail provides a single-container deployment with an embedded database, a versioned API, direct media reads, and optional compatibility playback paths. Its image is larger than Plex's but smaller than Jellyfin's in this test.

Jellyfin provides the broadest open media-server baseline in this comparison. Its official guidance recommends at least 8 GB of system memory for an average deployment and a GPU for practical video transcoding.

Plex has the smallest current memory sample in this local snapshot, but its container has many more processes. Plex hardware-accelerated streaming requires Plex Pass for ordinary server installations. Plex software transcoding remains available without that feature.

The HTTP results are directional, not a final product ranking. Kinosail used local HTTPS, while Jellyfin and Plex used local HTTP. All three containers were unconstrained by explicit CPU and memory limits. Transcoding, GPU acceleration, and real-client playback require a separate matrix.

## Product specification

| Dimension | Kinosail | Jellyfin | Plex |
| --- | --- | --- | --- |
| Product role | Private self-hosted media server | Free software media system | Personal media server |
| Source and license | Source-available; PolyForm Perimeter License 1.0.1 | Jellyfin server repository: GPL-2.0 | Proprietary server and client ecosystem |
| Server runtime | Go server, HTMX web app, embedded SQLite, FFmpeg | .NET server, database and FFmpeg-based media pipeline | Plex Media Server, platform packages or container |
| Deployment tested | One Kinosail container | Official Jellyfin container | Official Plex container |
| Database sidecar | Not required | Not required for this test | Not required for this test |
| Library fixture | Same read-only fixture | Same read-only fixture | Same read-only fixture |
| Authentication tested | Protected local API token | Jellyfin user token | Protected local Plex server token |
| API style tested | Versioned JSON API | Jellyfin REST API | Plex XML API |
| Direct media path | `/media/{id}` with byte ranges | `/Videos/{id}/stream` with byte ranges | Authenticated media-part URL from Plex XML |
| Playback planning | `/api/v1/items/{id}/playback` | `/Items/{id}/PlaybackInfo` | `/library/metadata/{ratingKey}?checkFiles=1` metadata negotiation |
| Video transcoding | Software or supported Linux hardware paths | Software or hardware acceleration on supported hardware | Software available; hardware acceleration generally requires Plex Pass |
| Hardware paths | QSV, NVENC/NVDEC, VA-API, RKMPP in Linux images; native VideoToolbox/AMF outside Linux | Intel, NVIDIA, AMD, Apple, and Rockchip support depends on host and drivers | Intel Quick Sync, NVIDIA, AMD support depends on platform and drivers |
| Remote media boundary | Direct Viewer-to-owner-Server media; no Kinosail relay | Operator-managed networking | Plex account and remote-access ecosystem |
| Native mobile/client scope | Bundled web/PWA plus tested Jellyfin-compatible flows | Broad official and community clients | Broad official client ecosystem |
| License or subscription effect on hardware transcode | No product subscription required by Kinosail | No premium license required by Jellyfin | Plex Pass required for ordinary hardware-accelerated streaming |

Product facts are grounded in the Kinosail [README](../../README.md), [license](../../LICENSE), [Jellyfin installation documentation](https://jellyfin.org/docs/general/installation/), [Jellyfin hardware guide](https://jellyfin.org/docs/general/administration/hardware-selection/), [Jellyfin repository](https://github.com/jellyfin/jellyfin), [Plex requirements](https://support.plex.tv/articles/200375666-plex-media-server-requirements/), and [Plex hardware-acceleration documentation](https://support.plex.tv/articles/115002178853-using-hardware-accelerated-streaming/).

## Test environment

| Field | Value |
| --- | --- |
| Host | macOS, arm64, Apple M1 Pro, 10 CPU cores, 32 GiB RAM |
| Container engine | Podman |
| Network | Local loopback ports |
| Media fixture | Four synthetic 640x360 H.264/AAC MP4 movies, about 196 KiB each, plus artwork/subtitles |
| Media mount | Same read-only host directory in every server |
| CPU limits | None; `NanoCpus=0`, `CpuQuota=0` |
| Memory limits | None; `Memory=0` |
| GPU | Not passed through; no hardware-transcode claim |
| Nox | Not used |

## Tested builds and footprint

These values are container measurements. The current CPU percentage is an instantaneous Podman sample, not a normalized benchmark score.

| Measurement | Kinosail | Jellyfin | Plex |
| --- | ---: | ---: | ---: |
| Server version | 1.0.0 | 10.11.11 | 1.43.3.10896-cb3ebc72d |
| Image root filesystem | 487.4 MB | 816.6 MB | 365.3 MB |
| Writable container layer | 25.5 KiB | 26.8 KiB | 34.5 KiB |
| Current memory sample | 380.9 MB | 213.9 MB | 115.0 MB |
| Current memory percentage | 4.60% | 2.58% | 1.39% |
| Current CPU sample | 24.81% | 14.67% | 5.28% |
| Current process count | 20 | 17 | 61 |

The current samples were taken after the benchmark containers had the shared fixture mounted. They are instantaneous snapshots, not peak values. The harness samples CPU, memory, and process count during sustained workloads and records those profiles in the JSON output.

The strongest warm sustained resource profiles at concurrency 8 were:

| Workload | Server | CPU p95 / max | Memory p95 / max | PIDs max |
| --- | --- | ---: | ---: | ---: |
| Library browse | Kinosail | 63.63% / 63.69% | 373.4 / 373.6 MB | 12 |
| Library browse | Jellyfin | 14.97% / 15.06% | 238.1 / 239.0 MB | 30 |
| Playback plan | Kinosail | 57.32% / 57.34% | 377.5 / 377.5 MB | 18 |
| Playback plan | Jellyfin | 31.87% / 31.97% | 224.4 / 224.4 MB | 27 |
| 1 MiB media range | Kinosail | 57.02% / 57.07% | 380.2 / 381.0 MB | 20 |
| 1 MiB media range | Jellyfin | 38.41% / 38.49% | 224.5 / 224.5 MB | 26 |
| 1 MiB media range | Plex | 79.22% / 79.24% | 111.4 / 111.5 MB | 84 |
| Playback plan | Plex | 85.31% / 85.41% | 98.7 / 98.8 MB | 84 |

CPU percentage is the Podman container statistic. It is not a host-normalized score. Kinosail's current memory profile includes the fresh protected test instance and its embedded services.

## Why the Plex image is smaller

The Plex image is smaller in this measurement because image size measures the bytes packaged into the container, not where the product performs media work. The local Podman image history shows these image sizes and history records:

| Image | Image size | Non-zero history records | History entries |
| --- | ---: | ---: | ---: |
| Plex | 365.2 MB | 6 | 23 |
| Kinosail | 487.4 MB | 6 | 19 |
| Jellyfin | 816.6 MB | 10 | 40 |

The measured Plex image uses a Debian base, the proprietary Plex Media Server package, and the s6 process supervisor. Jellyfin's official packaging combines a self-contained .NET server, the web client, FFmpeg, and hardware-support libraries. Kinosail's test image includes its Go server plus FFmpeg and the test-instance support package. These packaging choices explain more of the image-size difference than cloud execution.

The synthetic test data is not included in the Kinosail image measurement. Every server received the same host fixture through a read-only `/media` bind mount. Configuration, cache, backups, and Plex transcode data were separate Podman volumes. The Kinosail `localhost/kinosail:test-instance` image measured 487.4 MB; its base `localhost/kinosail:dev` measured 486.0 MB, and the test-only `busybox` HTTP fixture helper added about 1.42 MB. The remaining size comes from the production runtime: Debian, the Go server, the bundled Jellyfin FFmpeg build, media libraries, and hardware-video support packages.

Plex is still a local media server. Plex's own documentation says the local web app is bundled with the server and that hosted Plex Web communication can occur directly between the browser and the server. Plex also documents internet requirements for account authentication, metadata and artwork, dynamic components, and remote access. A relay is a fallback path when the app cannot connect directly; it is not the normal local loopback path used here. See Plex's [local web app guidance](https://support.plex.tv/articles/200288666-opening-plex-web-app/), [internet requirements](https://support.plex.tv/articles/200484903-internet-and-network-requirements/), and [relay documentation](https://support.plex.tv/articles/216766168-accessing-a-server-through-relay/).

Therefore, the Plex result means: smaller packaged image, lower current memory, and a larger process count in this snapshot. It does not mean cloud-hosted media, lower transcoding cost, or less network dependence. Those require authenticated direct-play, remux, audio-transcode, video-transcode, and offline or disconnected tests.

## Benchmark procedure

The benchmark uses [`scripts/benchmark-media-servers.sh`](../../scripts/benchmark-media-servers.sh), which invokes [`media_server_benchmark.py`](../../scripts/media_server_benchmark.py).

The completed Kinosail/Jellyfin run used:

```text
--iterations 2
--duration 2
--concurrency 1,4,8
--cache-passes cold,warm
--startup-restarts 1
--range-bytes 65536,1048576,4194304
```

For each server, the harness:

1. Authenticates with a protected local credential.
2. Discovers one common movie and its server-specific identifier.
3. Records server version, image digest, image size, root filesystem size, writable layer size, and configured limits.
4. Measures health, library browse, search, item detail, artwork, and playback-plan requests.
5. Measures direct media range reads at 64 KiB, 1 MiB, and 4 MiB.
6. Runs each workload at concurrency 1, 4, and 8.
7. Runs a first post-discovery pass and a warmed pass.
8. Runs sustained workers for two seconds per case.
9. Records p50, p95, p99, mean, minimum, maximum, time to first byte, response bytes, request rate, byte rate, status codes, and errors.
10. Samples Podman CPU, memory, memory percentage, process count, network I/O, and block I/O during sustained cases.
11. Restarts each supplied benchmark container once and waits for its discovered library endpoint to become ready.

The cache labels are workload warmup policies. They do not purge the operating-system page cache or database caches. The startup restart is the stronger cold-process measurement.

## Completed request results

All completed Kinosail, Jellyfin, and Plex samples were successful after readiness and authentication. The latest valid warm sustained cases at concurrency 8 were:

| Workload | Kinosail p95 / req/s | Jellyfin p95 / req/s | Plex p95 / req/s |
| --- | ---: | ---: | ---: |
| Library browse | 6.6 ms / 1,641.1 | 124.2 ms / 92.8 | 138.4 ms / 126.7 |
| Search | 7.1 ms / 1,537.1 | 83.9 ms / 130.3 | 173.9 ms / 67.5 |
| Playback plan | 9.0 ms / 1,199.1 | 43.0 ms / 285.1 | 78.9 ms / 173.2 |
| 1 MiB direct range | 12.4 ms / 803.4 | 36.4 ms / 332.0 | 47.6 ms / 312.2 |

Warm sustained direct-read throughput at concurrency 8 was approximately:

| Range | Kinosail | Jellyfin | Plex |
| --- | ---: | ---: | ---: |
| 64 KiB | 60.7 MB/s | 23.3 MB/s | 26.6 MB/s |
| 1 MiB | 157.1 MB/s | 64.9 MB/s | 61.1 MB/s |
| 4 MiB | 115.4 MB/s | 65.2 MB/s | 51.2 MB/s |

The fixture files are about 196 KiB, so the 1 MiB and 4 MiB requests return the same smaller file body. These rates measure the observed response path, not a sustained large-file disk limit.

Startup readiness after one container restart:

| Server | Library-ready time |
| --- | ---: |
| Kinosail | 3.35 s |
| Jellyfin | 19.17 s (earlier aligned run) |
| Plex | 6.71 s |

Raw results: `/tmp/media-server-kinosail-comprehensive-v2.json`, `/tmp/media-server-jellyfin-footprint.json`, and `/tmp/media-server-plex-comprehensive-v2.json`. The Jellyfin resource pass used the same fixture and warm matrix but did not repeat startup after its authentication token expired. Plex completed 24,228 successful requests out of 24,228 attempts.

## Interpretation

Kinosail showed lower control-plane latency and higher local request throughput in this fixture. This result is consistent with the smaller application surface exercised and the embedded deployment model, but it is not proof of performance for large libraries, remote networks, or transcoding.

Jellyfin showed higher library-browse cost in this small container, while Plex showed higher search cost than Kinosail and lower direct-read throughput than Kinosail. Its image was smaller than Kinosail's, but its measured process count was higher. Jellyfin's official hardware guidance makes clear that CPU, GPU, drivers, tone mapping, codec, and concurrent streams can dominate transcoding behavior.

Plex is now included in the control-plane and direct-range ranking. Its server was claimed, its local `/media/Movies` library contained five files, and its authenticated discovery and workloads all succeeded. Transcoding and real-client playback remain separate verification boundaries.

## Required follow-up for a complete three-way result

1. Add matched H.264, HEVC, subtitle, audio-transcode, and video-transcode fixtures.
2. Run the three servers with identical explicit CPU and memory limits.
3. Run real browser clients and record startup to second moving frame, rebuffering, dropped frames, and seek latency.

Do not combine direct play, remux, audio transcode, and video transcode into one score.
