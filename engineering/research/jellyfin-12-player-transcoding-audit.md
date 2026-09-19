# Jellyfin 12 player and transcoding audit

Date: 2026-09-19. This is a source comparison, not client or GPU certification.

## Release baseline

[Server 12.1](https://github.com/jellyfin/jellyfin/releases/tag/v12.1) is the latest stable release, following [12.0](https://jellyfin.org/posts/jellyfin-release-12.0/). [Jellyfin FFmpeg 8.1.2-5](https://github.com/jellyfin/jellyfin-ffmpeg/releases/tag/v8.1.2-5) is the current media runtime. Release asset SHA-256 digests were retrieved from GitHub's release API for both Trixie architectures.

## Changes made

- Player and Subtitles container pins move from Jellyfin FFmpeg 7.1.4-3 to 8.1.2-5 with architecture-specific checksums; third-party notices match. Native installations use their installed runtime and are not upgraded by this change.
- The shared playback planner now evaluates the official `Width`, `Height`, and `VideoRotation` profile properties. Previously optional conditions could be ignored and required conditions could reject otherwise usable playback. Rotation restrictions select video conversion, whose output has zero rotation, while retaining conversion permission checks.
- Rotation values are bounded canonical integers in quarter turns, including negative rotation; malformed conditions are rejected before media inspection/session creation. Focused regression tests cover these boundaries.

The official property names are defined in [ProfileConditionValue](https://github.com/jellyfin/jellyfin/blob/v12.1/MediaBrowser.Model/Dlna/ProfileConditionValue.cs). Existing Kinosail VideoWidth/VideoHeight handling remains for compatibility.

## Compatibility assessment

| Surface | Source evidence | Remaining boundary |
| --- | --- | --- |
| API discovery | `packages/jellyfincompat/entry.go` advertises 12.0.0 | This identifies the implemented API baseline, not full Jellyfin server parity. |
| Login and Quick Connect | Shared compatibility routes support current login and POST initiation; older aliases remain | Current released Swiftfin, Android/Android TV, Roku, webOS and Tizen applications need authenticated playback acceptance. |
| Playback negotiation | `packages/playback/jellyfin.go`, `client_constraints.go` and `packages/jellyfincompat/playback_http.go` parse device profiles and project playback URLs | All client profile combinations and subtitle/audio selection are not certified. |
| Seeking and progress | Shared media delivery, scoped play sessions and progress routes exist | Fresh device login, seek, resume, download and reconnect are not tested in this audit. |

Jellyfin removing legacy routes from its own server does not require Kinosail to remove working aliases. Do not increase the advertised version merely to suppress client warnings.

## Transcoding assessment

| Capability | Kinosail source status | Parity assessment |
| --- | --- | --- |
| H.264, HEVC, AV1, VP9 encoding | `packages/transcodepolicy/codecs.go` maps software and applicable hardware encoders | Available paths, not verified device performance. |
| Intel, NVIDIA, AMD, Apple, Rockchip | Backend definitions and codec-specific verification exist | Runtime/device/driver access still required. |
| V4L2 and Windows Media Foundation | Additional backend definitions exist | Broader catalog does not prove superior working coverage. |
| HDR10/HLG preservation | HEVC 10-bit output and color signaling paths exist | Potential capability beyond Jellyfin's documented SDR conversion; real HDR samples and display verification remain required. |
| GPU tone mapping | Dedicated CUDA, QSV, VA-API, Vulkan/libplacebo, OpenCL, VideoToolbox and D3D11 paths now have device/codec/HDR/frame-path smoke-check selection | Implemented; real GPU execution and comparative performance remain unverified. |
| HLG BT.2446 Method B | Existing zscale/Hable pipeline | Gap: no explicit equivalent algorithm selection. |
| Dolby Vision Profile 5 | Planner rejects conversion without a compatible base layer | Gap: safe rejection remains; no claim of P5 tone mapping or compliant P5 HLS variant delivery. Parsing dvh1 initialization alone is insufficient. |
| Audio output | Conversion emits stereo AAC | Gap against Jellyfin's broader codec/channel output options. |
| Image subtitles and styled text | Image burn-in and text delivery paths exist | Client-rendered image subtitles during remux, MKS embedding and style-preserving subtitle conversion parity are not established. |
| Rotated video profiles | New VideoRotation handling uses existing conversion path | Source gap fixed; Android TV/device playback pending. |
| HLS audio/video synchronization | Video conversion currently also encodes audio | Jellyfin's specific video-transcode/audio-copy fix does not directly match that path; actual timing still requires media fixtures. |

See [Jellyfin transcoding documentation](https://jellyfin.org/docs/general/post-install/transcoding/) for its HDR and hardware support boundaries. Server 12.1 also fixes Vulkan color-range handling and transcode throttling; these are not evidence that Kinosail has equivalent Vulkan or throttling implementations.

## Verification and conclusion

The repository's `.gates-disabled` policy prohibits test, build, container and acceptance suites unless explicitly enabled. Regression tests were added but have not been run. Runtime, deployed revision, real-client, GPU, audio synchronization and display evidence remain unverified. The enabled source-file cap is checked separately during delivery.

Do not claim all current Jellyfin players are certified or that Kinosail meets or exceeds all transcoding capabilities. The release update closes the runtime pin and profile-property gaps. The follow-up adds hardware tone-mapping selection and recovery. Dolby Vision P5, richer audio/subtitle output, and real-client/GPU acceptance remain outstanding.

## Hardware tone-mapping follow-up

Both applications use the shared policy. HDR10 and HLG are checked independently for each encoder/device and for GPU-resident versus CPU-filter delivery. HDR10+ uses its HDR10 base; supported Dolby Vision base layers retain the existing HDR10/HLG conversion boundary. Profile 5 conversion remains rejected.

CUDA/QSV/VA-API can retain SDR frames on the GPU through scaling and encoding when no CPU filters are needed. Subtitle burn-in, interlacing, rotation, anamorphic sources and server-side cuts use the separately checked download/software-filter path. OpenCL/Vulkan derive from the selected DRM device; Rockchip records its render-device identity. Mesa Vulkan drivers are included in both containers. Native installation driver availability still depends on the host.

Smoke checks create actual ten-bit HDR pixels, check the encoded SDR format, decode it, and reject unchanged/flat luminance output. This detects no-op conversion but does not certify perceptual quality. Native QSV/VA-API tone mapping is excluded for HLG. Optional tone-map checks have a separate 30-second budget after existing checks, preserving codec discovery; this can add up to 30 seconds to startup probing. Missing or unfinished evidence keeps software tone mapping.

Recovery first retains the hardware encoder while disabling hardware tone mapping, then uses the existing bounded encoder fallback. A failed tone-mapping operation does not erase unrelated encode/HDR evidence. Cache policy advances and seek identity includes the tone mapper to avoid mixing processing implementations within one presentation.

Regression coverage was added for device/codec/HDR isolation, source base layers, subtitle filter order, malformed/conflicting check requests without side effects, no-op pixel rejection, optional probe ordering, recovery and seek identity. These tests were not executed under the disabled-gates policy. Independent source review and the enabled line cap passed; FFmpeg execution, container builds, physical GPU color and performance comparison remain unverified.

Protocol/filter references: [Jellyfin encoding paths](https://github.com/jellyfin/jellyfin/blob/v12.1/MediaBrowser.Controller/MediaEncoding/EncodingHelper.cs), [Jellyfin FFmpeg patches](https://github.com/jellyfin/jellyfin-ffmpeg/tree/v8.1.2-5/debian/patches), [Vulkan device derivation](https://github.com/jellyfin/jellyfin-ffmpeg/blob/v8.1.2-5/libavutil/hwcontext_vulkan.c). The implementations use these filter interfaces; no Jellyfin server source is incorporated.
