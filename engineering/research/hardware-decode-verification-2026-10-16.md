# Hardware decode verification and on-device HDR pipelines

Date: 2026-10-16. Branch: `claude/hardware-transcode-decode`. Pull request: #641.

This change widens hardware transcoding in Player's shared packages. It does **not** establish behavior on any real GPU. No NVIDIA, Intel, AMD, Rockchip, or Apple device was available in the session. Every new FFmpeg recipe is gated behind the existing startup smoke check, so a device uses a recipe only after that recipe produced and decoded a playable HLS clip on that device.

## Problem

| Area | Before | After |
| --- | --- | --- |
| Hardware decode | Only 8-bit H.264 sources decoded on the GPU. HEVC, 10-bit, AV1, and VP9 sources decoded on the CPU. | Startup checks decode synthetic H.264, HEVC, HEVC 10-bit, AV1, AV1 10-bit, VP9, and VP9 10-bit clips per device. A source decodes on the device only when its format passed. |
| HDR tone mapping | CPU decode, then `hwupload`, then `tonemap_cuda` / `tonemap_vaapi` / `vpp_qsv`. | Devices that decode HEVC 10-bit also verify `-hwaccel` decode straight into the hardware tone-map filter. 10-bit HEVC HDR sources then stay on the device end to end. |
| HDR output | CPU decode always. `scale_cuda` requested `yuv420p10le`, which the filter does not support. | Verified devices decode on the GPU for HEVC HDR output. CUDA requests `p010le`. |
| VideoToolbox | No decode path. `-q:v 22` on a 1–100 scale. | `-hwaccel videotoolbox` with `scale_vt`. CRF 19/22/26 map to quality 75/65/55. |
| NVENC and QSV | Fixed `-preset p4`; no QSV preset. | p2/p4/p6 and veryfast/medium follow the conversion preference. |
| Recovery | A failed hardware tone map disabled hardware tone mapping. | A hardware-decoded tone map first retries with CPU decode on the same device, then CPU tone mapping, then another device. A decode failure removes only the failed input format. |

## Verification run in this session

Commands ran from the repository root unless noted. Local FFmpeg: 6.1.1 (Ubuntu), software encoders only.

```sh
cd packages && go test ./transcodepolicy ./transcodehardware ./playback ./downloads
cd packages && golangci-lint run ./transcodepolicy/... ./transcodehardware/...   # v2.14.0 built with go1.27.2: 0 issues
make max-loc        # only the frozen legacy allowance remains
make comments-check # OK
```

`TestRealFFmpegProducesEveryDecodeCheckSource` builds every synthetic decode source with the local FFmpeg and probes it with FFprobe. It confirms the codec, pixel format (`yuv420p` or `yuv420p10le`), and `smpte2084` transfer for the HDR variants. The AV1 sources use the first available encoder from `libsvtav1`, `libaom-av1`, and `librav1e`.

Isolated tests cover the new FFmpeg argument shapes, the check setup rules that reject unverifiable combinations, verification ordering within the startup budget, selection against recorded evidence, and the recovery ladder. They protect failures that populated E2E journeys cannot see without a GPU: the only hardware-specific paths in CI run against a fake FFmpeg that accepts every recipe.

## Not verified

- Real decode or encode on any device, including whether a driver accepts a specific 10-bit input, `scale_vt` on macOS, or `tonemap_vaapi` on a given Intel generation. The startup check decides this per deployment.
- Startup check duration on slow devices. The synthetic sources add up to six decode checks per device inside the existing 30-second budget, ordered HEVC 10-bit, HEVC, HDR output, then AV1 10-bit, AV1, VP9, VP9 10-bit. A device that runs out of budget keeps the inputs it already verified.
- Container build, Nox deployment, browser journeys, and the Player server suite result at the time of writing; see the pull request for the final record.
