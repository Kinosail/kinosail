# Player parity with best-in-class players — 2026-10-31

Goal (from the Owner): the Kinosail player clients must be as good as or better than the best-in-class third-party video players on Apple platforms, and support the major media servers. This note compares Kinosail today with that bar and orders the work. Apple TV and iPhone/iPad come first because iOS and tvOS are the design standard; Android, Android TV and web follow.

Status key: **Have** (shipped and evidenced), **Partial**, **Missing**, **Verify** (code suggests support, but no evidence yet). Evidence paths are relative to the repository root.

## Business model and ranking

The Owner's model, 2026-10-31: "We make money on the player and not the server." The Kinosail Server stays free. The player apps are the product.

So the matrix is ranked by what sells a player to someone who does **not** run a Kinosail Server. That person already has Plex, Jellyfin or Emby, or a folder of files on a NAS. For them, first-class Plex, Jellyfin and Emby connections and network shares (SMB, NFS, WebDAV) with good metadata are the core product, not an extra. Picture and sound fidelity, subtitles and polish then decide whether they keep paying. App Store readiness is a separate track that runs in parallel, because nothing sells until the apps are in the store.

Pricing, App Store Connect records, agreements and any spending are the Owner's decisions. They are listed under [Decisions for the Owner](#decisions-for-the-owner) with a recommendation each.

## How Kinosail plays media today

Kinosail is server-assisted. The Server probes each file, the client reports its capabilities, and the plan is direct play first, then a lossless remux ("copied HLS", fMP4), then a transcode (`packages/playback/plan.go`, `packages/playback/hls_source.go`). The Apple clients use AVFoundation/AVKit only; they have no software decoder (`apps/player/apps/native/IMPLEMENTATION.md`). Android uses Media3 ExoPlayer without a software decoder extension (`apps/player/apps/android/app/build.gradle.kts`).

Best-in-class players decode on the device with a software decoder when the platform decoder cannot. Kinosail reaches the same files through the Server. The gap is therefore smaller for files on a Kinosail Server and largest for sources without a Kinosail Server (Plex/Jellyfin/Emby direct, network shares).

This matters more under the business model. A Plex, Jellyfin or Emby server can remux or transcode for the app, so those sources work today. A network share has no server at all: AVFoundation cannot open MKV (93% of the library's video files), DTS or TrueHD audio, or PGS and ASS subtitles. **Network shares therefore need an on-device playback engine, which makes decision D1 a blocker for the share track.**

## The real library (Nox, 2026-10-31)

A random sample of 400 of the 4,263 video files (3,958 MKV, 293 MP4, 12 AVI), probed read-only with ffprobe; only aggregate counts were kept:

| Measure | Share of sampled files |
| --- | --- |
| H.264 video | 90% (no HEVC, HDR or Dolby Vision in the sample) |
| 23.976 fps video | 80% (29.97 fps 7%) |
| DTS-HD audio | 38% |
| E-AC-3 audio | 30% |
| 5.1 or more audio channels | 55% |
| PGS bitmap subtitles | 38% |
| SRT subtitles | 39% |

This is one household's library. It is a fair picture of what a buyer with a NAS has: mostly MKV, lots of DTS-HD, lots of PGS.

## Capability matrix

Sections are in priority order for a buyer without a Kinosail Server.

### P0 — Sources for people without a Kinosail Server

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Setup without a Kinosail Server | Setup offers Kinosail, Jellyfin, Emby and Plex, finds servers on the local network, and syncs accounts through iCloud Keychain | Have | `Sources/Features/Setup/SetupScreen.swift`, `IMPLEMENTATION.md` Other servers row |
| Plex sign-in and connections | plex.tv code sign-in (web sheet on iPhone/iPad, QR code on Apple TV), connection ranking with fallback | Have | `Sources/DeviceLibrary/Sources/Plex*.swift` |
| Jellyfin and Emby sign-in | Password and Quick Connect; an iPhone approves a TV's Quick Connect by QR code | Have | `Sources/DeviceLibrary/Sources/Jellyfin*.swift` |
| Movies and shows from Plex, Jellyfin, Emby | One combined library with provider-ID matching, search, history, My List | Have | `Sources/DeviceLibrary/DeviceLibraryRouter.swift` |
| Playback from Plex, Jellyfin, Emby | Direct play when AVFoundation can; otherwise the remote server remuxes or transcodes; progress written back | Partial | Jellyfin direct-play profile lists MP4/M4V/MOV only, so every MKV is remuxed by the remote server. The profile asks for AAC, not E-AC-3 surround. |
| Music, audiobooks, books, photos from Plex, Jellyfin, Emby | Empty views | Missing | `DeviceLibraryRouter.swift` returns no items for these views |
| Collections, people pages, albums from these sources | Not supported | Missing | Router returns "unsupported" |
| Downloads, bookmarks, casting from these sources | Kinosail Server only | Missing | `IMPLEMENTATION.md` Other servers row |
| Subtitle search and sidecars for these sources | Embedded and server sidecars only | Partial | No online search; Kinosail Subtitles needs a Kinosail Server |
| SMB, NFS, WebDAV shares | None | Missing | Needs a share client, a scanner, a local index, metadata matching and an on-device playback engine (D1) |
| Metadata for shares (posters, plots, cast, episode names) | None | Missing | Needs a metadata provider and its commercial terms (decision M4) |
| Trakt scrobbling and sync | None | Missing | Common reason people pick a player |
| Kinosail Server | Native | Have | The free path; see M2 for what the app charges here |

### P1 — Playback compatibility and picture/sound fidelity

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Direct play of MP4/MOV/HLS with H.264, HEVC (incl. 10-bit), AV1 where the device decodes it | Direct first; capability hints from Apple APIs | Have | `packages/playback/plan.go`; `IMPLEMENTATION.md` Playback row |
| MKV, AVI, other containers | Lossless Server remux to fMP4 HLS when codecs are compatible | Have (Server) | `packages/playback/hls_source.go`. Missing on device for shares (D1). |
| VC-1, MPEG-2, VP8, and AV1 on devices without AV1 decode | Server transcode | Partial | Works with a Server; no on-device decoder. See decision D1. |
| Dolby Vision profile 5 and 8.x | Signalled as `dvh1`/`dvhe` in fMP4 HLS | Verify | `packages/isobmff/codecs.go`, `packages/playback/hls_source.go`; needs a real P5/P8 file on Apple TV 4K |
| Dolby Vision profile 7 (dual layer, FEL/MEL) | Base layer used; some conversions rejected ("this Dolby Vision conversion is unsupported") | Partial | `DolbyVisionBaseLayer`, `dolbyVisionNeedsConversion`; target: P7 → P8.1 RPU conversion, else HDR10 base layer |
| HDR10, HLG | Probed and passed through | Verify | `packages/mediaprobe/facts.go` |
| HDR10+ | Probed | Verify | 3 Server references; Apple TV shows HDR10+ only on supported TVs |
| tvOS match frame rate and dynamic range | Applies the stream's display criteria when Match Content is on and waits for the TV to switch before playback | Have (simulator, in review) | #820, `Sources/Platform/DisplayMatching.swift`. Works for every source, because it runs in the app. Needs an Apple TV hardware check. |
| Dolby Digital (AC-3), Dolby Digital Plus (E-AC-3) incl. Atmos (JOC) | Passed through | Verify | `packages/playback/http.go` lists ac3/eac3 |
| TrueHD, DTS, DTS-HD MA, DTS:X | E-AC-3 5.1 at 640 kb/s when the client reports six or more channels; stereo AAC otherwise | Have (Kinosail Server, merging) | #819, `packages/playback/audio_conversion.go`; a real DTS-HD MA 7.1 file converted with the Nox Player image's FFmpeg. 7.1 becomes 5.1 (E-AC-3 encoder limit). Plex/Jellyfin/Emby sources still get AAC: the app must ask those servers for E-AC-3. |
| Fast start | Prepare step, loopback capability, direct-first | Verify | Needs a measured start-time budget (see Quality) |
| Resume (incl. across devices) | Server progress with compare-and-set | Have | `IMPLEMENTATION.md` Progress row |
| Transcode only as last resort; bitrate choice | Direct first; quality choices for downloads | Partial | No user-visible stream quality cap on Apple TV |

### P2 — Subtitles

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Embedded text subtitles (SRT/WebVTT/mov_text) | Native tracks | Have | `IMPLEMENTATION.md` Subtitles row |
| External sidecar subtitles | Delivered as WebVTT | Have | `packages/playback/subtitle_delivery.go` |
| ASS/SSA with styling and positioning | Converted to plain WebVTT text | Partial | Styling lost; target: styled rendering on device |
| PGS, VobSub, DVB bitmap subtitles | OCR to WebVTT text | Partial | `packages/mediaprobe/image_subtitle_ocr.go`; target: show the original bitmaps, keep OCR as fallback. Shares need on-device rendering (D1). |
| Online subtitle search from the player | Not in the player (Kinosail Subtitles has it) | Missing | Wire the player to Kinosail Subtitles; a buyer without a Server needs an in-app provider |
| Size, color, background, position | System caption style only | Partial | Add in-app style controls |
| Subtitle offset (sync) during playback | None | Missing | No offset control in the app |
| Forced and SDH handling | Forced/SDH roles recognized | Partial | `library.SubtitleFileRole`; automatic forced selection needs verification |
| Memory of audio and subtitle choices | Per title | Partial | `PreferencesAPI.playbackPreferences(itemID:)`; target: per show |
| External captions in Picture in Picture | Not shown | Missing | Documented limit in `IMPLEMENTATION.md` |

### P3 — Player experience

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Chapters | Server returns chapters | Verify | Need chapter navigation in the transport bar |
| Skip intro, recap, credits | Markers, automatic skip setting | Have | `apps/player/apps/native/DESIGN.md` Video playback |
| Scrubbing thumbnails (trick play) | I-frame playlist when the stream has one | Partial | `EXT-X-I-FRAME` in one Server path; direct play has none |
| Playback speed | AVKit | Have | |
| Audio delay | None | Missing | |
| Sleep timer | Audio only | Partial | Add for video |
| Picture in Picture | iPhone/iPad | Have | |
| AirPlay | Route picker | Have | |
| Siri Remote gestures (click-pad skip, swipe scrub) | AVKit defaults | Verify | Needs a physical remote check |
| Next episode | Starts at once when credits skip; Next Episode action otherwise | Partial | Add a visible countdown with cancel |
| Up Next / Continue watching across devices | Server-backed | Have | |
| Top Shelf | Continue watching, My List, recently added | Have | |

### P4 — Library (Kinosail Server)

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Metadata, artwork, collections, filters, search | Server-backed | Have | `IMPLEMENTATION.md` Library row |
| Profiles and parental controls | Viewer Profiles with rating policy | Have | `apps/player/docs/user-guide/profiles.md` |
| Offline downloads | iPhone/iPad with verification | Have | Apple TV streams only, by design |

### P5 — Quality

| Measure | Kinosail today | Status | Note |
| --- | --- | --- | --- |
| Crash-free sessions | Not measured | Missing | Needs MetricKit collection and a target (≥ 99.8%); see App Store track |
| Time to first frame | Not measured | Missing | Target: direct play ≤ 1.5 s, remux ≤ 3 s on Apple TV HD |
| Memory and thermal on Apple TV HD | Not measured | Missing | Budget per playback mode |

## App Store readiness track

This track runs in parallel with the phases. Engineering can design and build every item below. Creating records, signing agreements, setting prices and paying for anything stay with the Owner.

| Item | Today | Work | Risk or note |
| --- | --- | --- | --- |
| Purchase architecture (StoreKit 2) | None | One `Entitlements` actor: `Product.products`, `Transaction.currentEntitlements` at launch, a `Transaction.updates` listener, **Restore Purchases** (`AppStore.sync()`), Family Sharing on every product, one universal purchase for iPhone, iPad and Apple TV. On-device verification by StoreKit; no Kinosail account or receipt server. A StoreKit configuration file and StoreKit Testing in Xcode for tests. Supports either model in M1 (trial plus unlock, or subscription). | No prices in code or docs. Paywall copy must state what stays free (M2). |
| Paywall and trial flow | None | Show the paywall after the first successful playback, not before setup. Trial state visible in Settings. Expiry never interrupts a playing title. | A trial of a one-time unlock uses a free, non-consumable "trial" product; a subscription uses an introductory offer. |
| Privacy manifest | `Resources/PrivacyInfo.xcprivacy`, `TopShelf/` and `Watch/` manifests; tracking off; required-reason APIs declared (UserDefaults, disk space, file timestamps) | Re-audit after StoreKit, shares and crash reporting land; keep "no tracking domains" | No third-party SDKs today, which keeps this simple |
| App privacy ("nutrition") labels | Not prepared | Target label: **Data Not Collected**. Holds as long as crash data stays with Apple and diagnostics leave the device only when the user shares them | If the app ever uploads diagnostics or metadata queries to a Kinosail service, the label changes |
| Crash and hang reporting without third-party tracking | None | MetricKit (`MXMetricManager`) subscriber for crash, hang and launch diagnostics, kept on device; a user-started "Share diagnostics" export in Settings; Xcode Organizer crash reports from users who opt in to Apple analytics | Also feeds the P5 crash-free target |
| Onboarding with no Kinosail Server | Setup already lists Jellyfin, Emby and Plex and scans the local network; the first sentence names Kinosail first | Lead with "Connect Plex, Jellyfin, Emby or a network share"; add shares when they ship; a short "What you need" screen; no dead end when nothing is found | App Review must be able to reach real content (next row) |
| App Review access | None | A reviewer demo: a small server with public-domain films and step-by-step notes in App Review Information | Guideline 2.1: a login-only app without demo access is rejected. Where the demo runs is decision M6 |
| App Review: third-party server sign-in | Plex code sign-in in a web sheet; Jellyfin/Emby password or Quick Connect | Keep sign-in in the provider's own flow; name the services in text only ("Works with Plex, Jellyfin and Emby"), no logos without permission; trademark notice in the description | Guideline 5.2.1 (intellectual property) |
| App Review: network shares and piracy concerns | — | Screenshots and the demo use public-domain or Owner-made media only; describe the app as a player for media the user owns | Guidelines 5.2.2 and 5.2.3; players for personal media are accepted, but posters of commercial films in screenshots cause rejections |
| App Review: codecs and licensing | Apple system decoders only | Any bundled decoder (D1) must be an LGPL-only build as a dynamic framework, with notices and relinking materials in the app | Software decoding of DTS, TrueHD, AC-3 or HEVC can carry patent licensing duties; needs legal review before shipping (M5) |
| App Review: payments | Supporter badges are display only; no purchase links in the app | Every unlock goes through In-App Purchase; keep Supporter screens free of links or prompts to pay outside the app | Guideline 3.1.1 |
| Local network permission | `NSLocalNetworkUsageDescription` and `NSBonjourServices` in the iOS and tvOS plists | Add SMB/NFS/WebDAV service types when shares ship; plain-language purpose strings | Discovery fails silently if the user declines; setup must explain how to allow it |
| Export compliance | `ITSAppUsesNonExemptEncryption` is false | Re-check when shares add SMB encryption | Standard HTTPS and SMB 3 encryption are normally exempt |
| Localization | String catalog has 74 entries in 17 languages; about 220 UI strings in code, about 190 of them not in the catalog | Move every user-facing string into `Localizable.xcstrings`; a CI check that fails on untranslated strings for launch languages; localized App Store metadata and screenshots | Launch languages are decision M9 |
| Third-party notices in the apps | No notices file in the native app | Add an in-app Acknowledgements screen and a `THIRD_PARTY_NOTICES.md` for the native app (needed at once if D1 adds FFmpeg) | Server notices (`apps/player/THIRD_PARTY_NOTICES.md`) do not cover the apps |
| TestFlight | Builds install on Owner devices through a signed-device helper | Archive and upload from the Mac with `xcodebuild -exportArchive`; internal testing first; external testing needs Beta App Review and the demo access above | Needs the App Store Connect record and agreements (M3) |
| Watch and Top Shelf | Built and signed with the app | Include in the archive; the watch app is a remote only | Guideline 4.2 (minimum functionality) is met because the watch app controls playback |

## Decisions for the Owner

Each item needs the Owner. Engineering will not set prices, create App Store Connect records, sign agreements, spend money or change licensing.

| # | Decision | Recommendation |
| --- | --- | --- |
| M1 | Purchase model | Free download with a full-featured trial, then either a one-time unlock or a yearly subscription. Offer both if support load allows: a subscription funds ongoing source and codec work, and a lifetime unlock answers buyers who dislike subscriptions. Family Sharing on. Prices are the Owner's call. |
| M2 | What stays free in the app | Keep Kinosail Server playback free in the apps, so the README promise for current households stays true and the free Server keeps working as the funnel. Charge for Plex, Jellyfin, Emby, network shares and pro playback features. Alternative: one price for every source, with the web Player as the free path. |
| M3 | App Store Connect record, bundle IDs, Paid Apps agreement, tax and banking | Owner action. One app record with iOS and tvOS (universal purchase) under `com.kinosail.player`. |
| M4 | Metadata provider for network shares | Pick a provider whose terms allow use in a paid app, and confirm any commercial license or fee before work starts. Show the provider's attribution in the app. |
| M5 | D1, on-device playback engine | Now blocking the share track. Recommendation: an LGPL-only FFmpeg build as a dynamic framework that demuxes MKV and decodes only what Apple cannot (DTS, TrueHD, VC-1, MPEG-2, PGS, styled ASS), with legal review of the patent position for DTS and TrueHD decode before the first release. |
| M6 | Demo access for App Review and screenshots | A small, internet-reachable demo server with public-domain films only, separate from the household Server. Opening anything to the internet is a security change and needs the Owner. |
| M7 | Terms for app binaries and app source | Use Apple's standard App Store licence for the binaries. Decide whether the native app source stays public under PolyForm Perimeter (see F2). |
| M8 | README and site wording | Approve the wording changes in F1 and F6 before launch. |
| M9 | Launch languages | English plus the 17 catalog languages only after the catalog is complete; otherwise English, German, French, Spanish, Japanese and Portuguese (Brazil) first. |

## License and documentation consistency

Checked against the model "free Server, paid apps, source under PolyForm Perimeter". Nothing was changed; these are flags for the Owner.

| # | Where | Finding | Conflict? |
| --- | --- | --- | --- |
| F1 | `README.md` "Why Kinosail": "**Everything is free.** The Server, the web Player, remote streaming, hardware transcoding, and skip intro have no paid tier." | True today. Once the apps are paid, "Everything is free" reads as a promise about the apps too. Suggest "The Server and the web Player are free." | Yes, at launch |
| F2 | `README.md` License: "You can read, build, and run it for free." PolyForm Perimeter allows any use that does not compete. | Anyone can build the paid apps from source and use them without paying, because personal use is a permitted purpose. The Perimeter licence does stop someone from publishing a rebuilt copy, because that would compete. Options: accept this (many buyers will not build from source), or put the native app source under different terms. | Not a licence conflict; a revenue risk |
| F3 | `LICENSING.md` | Covers source only. It does not say which terms apply to App Store binaries. Add one line once M7 is decided. | Gap |
| F4 | `apps/player/CONTEXT.md` Supporter Badge: "recognizes voluntary support without unlocking product capability" | Consistent only if the paid unlock is a separate concept. Add a domain term for the unlock, and keep badges from looking like a purchase in the app (guideline 3.1.1). | No, if kept separate |
| F5 | `README.md` "No App Store release yet"; `apps/player/apps/native/README.md` "They do not imply an App Store release is available" | Accurate now; update at launch. | No |
| F6 | `apps/player/apps/native/PRODUCT.md` purpose: "Provide native access to the household Kinosail Server" | Too narrow for the business model: the apps are for any household library (Kinosail, Plex, Jellyfin, Emby, shares). | Yes |
| F7 | `apps/player/THIRD_PARTY_NOTICES.md`: the Server image ships a GPL v3 FFmpeg build | Fine for the free Server. A GPL build must never go into the apps; D1 must use an LGPL-only build. | No, if kept apart |
| F8 | `apps/player/CLA.md`, `CCLA.md` | Allow the Owner to distribute under proprietary terms and sell paid products. | No |
| F9 | `PRODUCT.md`: "Kinosail Server is free to run on household hardware" | Consistent. | No |

## Decisions needed (engineering)

- **D1, on-device software decoding.** Best-in-class players bundle a software decoder (typically FFmpeg-based) for VC-1, MPEG-2, unusual audio and styled subtitles. Kinosail is source-available under PolyForm Perimeter. FFmpeg can be used only in an LGPL-only build, shipped as a dynamic framework with notices and relinking materials; GPL builds are not compatible. This is a licensing decision for the Owner (M5). Under the business model it is no longer optional: network shares cannot play most files without it.
- **D2, Apple TV lossless audio.** Apple TV cannot pass TrueHD or DTS bitstreams to a receiver. Kinosail converts to multichannel E-AC-3 instead of AAC. This is a platform limit, not a choice.

## Phase plan

Ranked by what sells the player to a buyer without a Kinosail Server. Each phase ships as reviewable pull requests with tests and recorded verification.

1. **Phase 1, picture and sound on Apple TV (in progress).** In review: match frame rate and dynamic range (#820, simulator-verified, works for all sources) and E-AC-3 5.1 for DTS-HD and TrueHD on a Kinosail Server (#819, in the merge queue; FFmpeg in the Nox Player image converted a real DTS-HD MA 7.1 file to E-AC-3 5.1). Next: ask Plex, Jellyfin and Emby for E-AC-3 surround in their device profiles; Dolby Vision P5/P8 and P7 fallback; a stream quality cap.
2. **Phase 2, Plex, Jellyfin and Emby as first-class sources.** MKV direct play through the remote server's remux without needless transcodes; music, audiobooks and photos; collections and people; downloads on iPhone/iPad; per-show audio and subtitle memory; Trakt.
3. **Phase 3, network shares.** SMB first (most NAS units), then WebDAV and NFS; a share browser, background scanner, local index and metadata matching (needs M4); on-device playback engine (needs M5).
4. **Phase 4, subtitles.** Original PGS bitmaps and styled ASS (on-device, shared with phase 3); offset; size, color, background and position; automatic forced tracks; in-app online search for buyers without Kinosail Subtitles.
5. **Phase 5, player experience.** Next-episode countdown with cancel; chapters in the transport bar; audio delay; video sleep timer; thumbnails for direct play.
6. **Phase 6, measurement.** Time-to-first-frame and memory budgets and a playback test matrix of real files (crash reporting moves to the App Store track).
7. **Phase 7, other clients.** Bring Android, Android TV and web to the same matrix.

**App Store readiness** runs alongside phases 1–4. Order: localization and the native notices file now; StoreKit 2 entitlements, paywall and trial behind a build flag next; MetricKit reporting; then TestFlight once M3 and M6 are done.

Skipped on purpose: lossless TrueHD/DTS passthrough on Apple TV (platform limit, D2); Google Cast on Apple clients (no native sender SDK, already documented). Phase 3 cannot ship playback until M4 and M5 are decided; the share browser and scanner can be built first.
