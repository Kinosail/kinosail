# Backend hardware and performance source audit

Date: 2026-09-12. Starting revision: `01fa007d`.

This audit found and repaired backend defects in Player, Subtitles, and their shared packages. It does **not** establish flawless operation or measured performance improvements. The user explicitly kept `.gates-disabled` in place and requested audit and fixes from source. No tests, builds, benchmarks, FFmpeg checks, GPU checks, container checks, browser checks, or deployment checks were run. Fourteen regression test functions were added for a future authorized run.

UI assets, layouts, native client presentation, and interaction design were outside this change. Direct First decisions and negotiated codecs remain unchanged.

## Findings and changes

| Area | Source finding | Change |
| --- | --- | --- |
| Hardware startup | Optional codec checks ran before hardware H.264 decoding checks, allowing them to consume the shared startup deadline first. | Check the preferred H.264 decode path after H.264 encoding baselines and before optional codecs. The startup deadline remains bounded. |
| Hardware recovery | A failed hardware decode quarantined the entire device/codec, including its separately verified encoder. Recovery also received settings from before source-specific color and filter decisions. | Disable the failed decode evidence first; retry with software decoding and the verified encoder. Quarantine encoding if that attempt also fails. Build recovery candidates from actual source requirements. |
| HLS cache recovery | Effective backend/device selection formed part of cache identity. Quarantining a GPU changed the identity expected by subsequent requests while the existing job was still producing its fallback. Player also appended a recovery suffix that its seek path did not accept. | Key cached output by requested playback policy, source, and recipe. Keep that identity through bounded recovery. Match the complete policy marker rather than a prefix. |
| HLS seeks | Reusing a policy cache across backend changes must not mix new encoder output with existing initialization data. | Record Player's producing encoder, device, codec, and HDR mode in bounded private cache metadata. Reject resumed encoding if the identity differs or the metadata is missing/corrupt. |
| Media probing | Cold `Duration` calls bypassed the shared in-flight work and did not populate its memory/disk cache. A second caller could also miss a just-completed probe between lookup and locking. | Route duration through the existing facts operation and recheck the memory cache under the work-coordination lock. |
| Probe failures | A rejected FFprobe response became an empty result that `run` reported as successful, allowing invalid facts to persist. | Preserve parse success separately, bound stream/chapter counts, and keep rejected results out of both caches. Increment the probe cache schema so old rejected facts can be rebuilt. |
| Download capacity | Production cleared the generic reservation callback to avoid nested reservations. Original copies and compatible video copies then bypassed the governor; audio conversions used the GPU reservation callback despite not encoding video. | Acquire exactly one reservation inside the selected operation: device-aware for video encoding, generic for original/compatible copies and audio. Restore the generic callback in both apps. |
| Download validation/retry | Invalid qualities/profiles could reach media inspection. Reservation failures could enter the software retry branch. | Validate the job before inspection. Permit software retry only after an executed command fails and the lifecycle remains active. Clear the GPU device in software settings. |
| Catalog search | Unicode relevance ranking was recalculated for both items in each sort comparison. | Calculate each item's rank once for a search sort, preserving relevance, locale ordering, and ID tie-breaks. |

## Additional source review

These paths were inspected for ownership, bounding, caching, cancellation, and failure handling. No additional defect was established within this source review; this is not a runtime certification or an exhaustive security audit.

- **Detection and frame processing:** `packages/transcodehardware/{probe,backends,verification,recovery}.go`, `packages/transcodepolicy/{check,arguments,frames}.go`, and `packages/playback/{hls_source,hls_filters,video_geometry}.go`. Selection remains based on codec-specific smoke evidence. CPU filters retain explicit frame upload paths. HDR preservation requires separate evidence.
- **HLS publication and delivery:** Player/Subtitles HLS adapters, bounded playlist reads, generated MP4 initialization parsing, readiness, cache eviction, job cancellation, and workload reservations. Publication reads actual codec/color/dimension metadata. Recovery remains restricted before a presentation has been published.
- **Library/catalog:** `packages/catalog/{index,scan,browse,browse_apply,browse_sort,search}.go`. ID lookup uses an index, refreshes serialize work and atomically publish successful snapshots, and browse inputs/pages are bounded.
- **Subtitles/media delivery:** embedded subtitle validation and caching in `packages/mediaprobe/embedded.go`, source/track validation in shared playback, and authorized file delivery in `packages/playback/file_delivery.go`.
- **Persistence:** `packages/documentdb/{documentdb,config}.go`. Document names and sizes are bounded and related saves use transactions. Request cancellation and storage latency still need runtime evaluation.
- **Sessions/transport:** shared session-token handling, Dashboard host protection/login limiting, and server transport limits. These were sampled backend paths, not a complete authentication review.
- **Metadata:** provider HTTP deadlines, bounded JSON/images, outbound address checks, and artwork/metadata caches. No provider requests were made.
- **Installation:** Player/Subtitles device mapping logic and Player's container build inputs. Source includes DRM device/group mapping, NVIDIA CDI selection, and Rockchip mapping. No host passthrough or driver operation was verified.

## Tradeoffs and remaining evidence

- Existing probe and HLS caches will rebuild when needed. Initial requests after upgrade can therefore do extra work.
- Search ranking now uses one temporary map proportional to the selected result count. No latency or allocation benchmark was run.
- Hardware sessions remain conservatively limited per device. Startup checks have a finite budget, and hardware decoding evidence remains limited to the tested H.264 input. Other devices/codecs can remain unverified.
- A seek that requires changing the producing encoder fails before generating replacement segments. Restarting compatible playback may be required after a device/encoder change. Continuing a published presentation across such a change is not certified.
- Real SDR/HDR media, subtitles, interlacing, rotation, anamorphic input, long seeks, cancellation, concurrent playback/downloads, and GPU-to-software recovery still need runtime evidence on supported hardware.
- No claims are made about deployed revision, container health, TLS, physical devices, or measured speed. Source publication is a separate delivery fact.

## Verification to run only after gates are explicitly enabled

From `packages/`:

```sh
go test ./transcodehardware ./transcodepolicy ./playback ./mediaprobe ./downloads ./catalog ./workload
go test -race ./transcodehardware ./mediaprobe ./downloads ./workload
```

Run the affected Player and Subtitles server suites, then root `make max-loc` and each affected app's `make verify-changed`, serializing root checks. Exercise the real FFmpeg/GPU matrix and concurrent playback/download/seek flows separately. Formatting and source/diff inspection performed during this audit do not substitute for those checks.
