# Player parity with best-in-class players — 2026-10-31

Goal (from the Owner): the Kinosail player clients must be as good as or better than the best-in-class third-party video players on Apple platforms, and support the major media servers. This note compares Kinosail today with that bar and orders the work. Scope (Owner, 2026-10-31): iPhone, iPad and Apple TV only. "Focus on iOS and Apple TV for now. No Mac apps etc." iPad is part of iOS. Other platforms are listed under [Later, not planned](#later-not-planned).

Status key: **Have** (shipped and evidenced), **Partial**, **Missing**, **Verify** (code suggests support, but no evidence yet). Evidence paths are relative to the repository root.

## Business model and ranking

The Owner's model, 2026-10-31: "We make money on the player and not the server." The Kinosail Server stays free. The player apps are the product.

So the matrix is ranked by what sells a player to someone who does **not** run a Kinosail Server. That person already has Plex, Jellyfin or Emby, or a folder of files on a NAS. For them, first-class Plex, Jellyfin and Emby connections and network shares (SMB, NFS, WebDAV) with good metadata are the core product, not an extra. Picture and sound fidelity, subtitles and polish then decide whether they keep paying. App Store readiness is a separate track that runs in parallel, because nothing sells until the apps are in the store.

Pricing, App Store Connect records, agreements and any spending are the Owner's decisions. They are listed under [Decisions for the Owner](#decisions-for-the-owner) with a recommendation each.

## How Kinosail plays media today

Kinosail is server-assisted. The Server probes each file, the client reports its capabilities, and the plan is direct play first, then a lossless remux ("copied HLS", fMP4), then a transcode (`packages/playback/plan.go`, `packages/playback/hls_source.go`). The Apple clients use AVFoundation/AVKit only; they have no software decoder (`apps/player/apps/native/IMPLEMENTATION.md`). Android uses Media3 ExoPlayer without a software decoder extension (`apps/player/apps/android/app/build.gradle.kts`).

Best-in-class players decode on the device with a software decoder when the platform decoder cannot. Kinosail reaches the same files through the Server. The gap is therefore smaller for files on a Kinosail Server and largest for sources without a Kinosail Server (Plex/Jellyfin/Emby direct, network shares).

This matters more under the business model. A Plex, Jellyfin or Emby server can remux or transcode for the app, so those sources work today. A network share has no server at all: AVFoundation cannot open MKV (93% of the library's video files), DTS or TrueHD audio, or PGS and ASS subtitles. **Network shares therefore need an on-device playback engine.** The measured list under D1 shows that 75% of files need only an on-device demuxer, with no third-party decoder.

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
| Offline downloads from any source | iPhone/iPad downloads from a Kinosail Server only; the Server converts when the format needs it | Partial | Add downloads from Plex, Jellyfin and Emby (their download or transcode-to-file APIs), shares and cloud drives; see R3 on guideline 5.2.3 |
| Bookmarks, casting from these sources | Kinosail Server only | Missing | `IMPLEMENTATION.md` Other servers row |
| Subtitle search and sidecars for these sources | Embedded and server sidecars only | Partial | No online search; Kinosail Subtitles needs a Kinosail Server |
| Local playback: files on the device, the Files app, iCloud Drive, USB and SSD drives on iPhone/iPad | None: the app declares no document types and has no file importer | Missing | `Configuration/iOS-Info.plist`; needs document types, `LSSupportsOpeningDocumentsInPlace`, security-scoped bookmarks and the on-device engine (D1) for MKV |
| "Open in Kinosail" from the Share Sheet and AirDrop | The app opens only its own links (`onOpenURL` for Quick Connect and app links) | Missing | Same document-type work as local playback |
| Metadata matching for loose files | None | Missing | Same matcher as shares; needs M4 |
| SMB shares, with automatic discovery on the local network | None | Missing | Discovery today finds only Jellyfin/Emby (port 8096) and Plex (32400): `SourceDiscovery.swift`. Add Bonjour `_smb._tcp` discovery. |
| NFS and WebDAV shares | None | Missing | Needs a share client, a scanner, a local index, metadata matching and the on-device engine (D1) |
| DLNA/UPnP media servers as a source | None; the Server can cast to DLNA renderers, which is the opposite direction | Missing | `Sources/Features/Casting/PlayOnTVScreen.swift` |
| FTP and SFTP | None | Missing | Lower demand than SMB; same scanner |
| Cloud drives (Google Drive, Dropbox, OneDrive, Box, pCloud) | None | Missing (later phase) | Each provider needs a registered app and has review rules of its own; Google's Drive read scopes are restricted and need a security assessment (R2, M10) |
| Metadata for shares and local files (posters, plots, cast, episode names) | None | Missing | Needs a metadata provider and its commercial terms (R2, M4) |
| Trakt scrobbling and sync | None | Missing | Common reason people pick a player |
| Kinosail Server | Native | Have | The free path; see M2 for what the app charges here |

### P0b — A library with no server at all

A buyer with only a NAS or files on a drive still expects a full library: artwork, seasons, Continue watching and Up Next.

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| On-device library built from shares and local files | None; the on-device library combines Plex, Jellyfin and Emby only | Missing | Extend `Sources/DeviceLibrary` with a file-source catalog, a background scanner and the metadata matcher |
| Watch state, progress, My List synced across the user's own devices | Kinosail Server or the remote server stores progress; iCloud Keychain syncs only account sign-ins | Missing | CloudKit private database: no Kinosail account and no backend for Kinosail to run. Data stays in the user's iCloud, so the app's privacy label can stay "Data Not Collected" (confirm against Apple's definition before release). |
| Up Next and Continue watching without a server | Server-backed only | Missing | Built from the CloudKit state above |

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
| tvOS match frame rate and dynamic range | Applies the stream's display criteria when Match Content is on and waits for the TV to switch before playback | Have (simulator; hardware check pending) | #820, `Sources/Platform/DisplayMatching.swift`. Works for every source, because it runs in the app. Needs an Apple TV hardware check. |
| Dolby Digital (AC-3), Dolby Digital Plus (E-AC-3) incl. Atmos (JOC) | Passed through | Verify | `packages/playback/http.go` lists ac3/eac3 |
| TrueHD, DTS, DTS-HD MA, DTS:X | E-AC-3 5.1 at 640 kb/s when the client reports six or more channels; stereo AAC otherwise | Have (Kinosail Server, deployed) | #819, `packages/playback/audio_conversion.go`; a real DTS-HD MA 7.1 file converted with the Nox Player image's FFmpeg. 7.1 becomes 5.1 (E-AC-3 encoder limit). Plex/Jellyfin/Emby sources still get AAC: the app must ask those servers for E-AC-3. |
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

### P6 — Accessibility (baseline for every release)

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| VoiceOver on every screen | Labels on many controls (36 files set `accessibilityLabel`) | Partial | Needs a full VoiceOver pass on iPhone, iPad and Apple TV, recorded in `VERIFICATION.md` |
| Audio-description tracks | Not recognized | Missing | Mark tracks with the "describes video" characteristic and prefer them when the system Audio Descriptions setting is on |
| Caption style from system settings | AVKit tracks follow the system style; Kinosail's own caption label uses a fixed style | Partial | `PlayerPresentation.swift` (tvOS label); read `MACaptionAppearance` for size, color and background |
| Dynamic Type | Supported in many views (23 files) | Partial | Check the largest sizes on iPhone and iPad |
| Reduce Motion | Respected in launch, focus and transitions (20 files) | Have | `KinoMotion.swift`, `LaunchReveal.swift` |

### P7 — Apple ecosystem polish

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| Siri, Shortcuts, App Intents | "Search library" and "Play title" intents | Partial | `Sources/Platform/LibraryIntents.swift`; add Continue watching, Next episode, play from a source, and App Shortcuts phrases |
| SharePlay, tied to Watch Rooms | Watch Rooms exist in the web Player; no Group Activities in the apps | Missing | Group Activities for synced playback across people's own devices |
| Widgets and Live Activities for what is playing | None | Missing | WidgetKit and ActivityKit on iPhone/iPad; a Continue watching widget |
| iPhone as an Apple TV remote and keyboard | The Watch app is a remote; the Server has remote players | Partial | Add an iPhone remote with text entry for search |
| tvOS multi-user profiles | One app session for the Apple TV | Missing | Run as the current Apple TV user so each person has their own sources and progress |
| Apple TV app "Up Next" integration | None | Not available (likely) | Apple's TV app integration is a partner program for video providers with their own catalog; a player for personal media is unlikely to qualify (M11) |

### P8 — iPad

| Capability | Kinosail today | Status | Evidence / note |
| --- | --- | --- | --- |
| iPad multitasking and Stage Manager | iPad builds from the iOS target | Verify | Check Split View, Slide Over, Stage Manager window sizes and external displays |

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

## Paid-app risks for the Owner

These are flags, not solutions. Engineering has not assumed an answer. None of this is legal advice; counsel and each provider must confirm before release.

### R1 — Codec licensing

Formats split into three groups on Apple platforms:

| Group | Formats | What it means for a paid app |
| --- | --- | --- |
| Decoded by Apple's system decoders (AVFoundation) | H.264, HEVC (hardware), AV1 (devices with hardware AV1 decode only), AAC and HE-AAC, MP3, ALAC, FLAC, Opus, LPCM, AC-3 (Dolby Digital), E-AC-3 (Dolby Digital Plus), HDR10, HLG, Dolby Vision profiles 5 and 8 | The decoders ship in the operating system. The app calls them through AVFoundation, which is how Kinosail plays today. No separate codec license is expected for this use; counsel should confirm. |
| Passthrough only | Dolby Atmos carried in E-AC-3 (JOC): Apple TV sends it to the receiver as Dolby Digital Plus | Handled by the system. Apple TV cannot pass TrueHD or DTS bitstreams at all, so passthrough is not an option for them (D2). |
| Needs a license decision if the app decodes it in software | DTS family (DTS, DTS-HD MA, DTS:X); Dolby TrueHD (and Atmos in TrueHD); VC-1; MPEG-2 video; HEVC and AV1 on devices without hardware decode; any other format added with the on-device engine (D1) | Patent pools or the format owner may require a license for a decoder in a paid app. Patent status differs by country and some older patents have expired, so counsel must check each format. Using the names "Dolby", "Dolby Vision", "Atmos" or "DTS" in the app or store listing is a trademark question too. |

Today these files avoid in-app decoding because a server converts them: the Kinosail Server converts DTS and TrueHD to E-AC-3 or AAC with its bundled FFmpeg, and Plex, Jellyfin and Emby transcode on their own servers. The same counsel review should cover the decoders in the free Server image.

### R2 — Metadata provider terms

| Provider | Published terms (checked 2026-10-31) | Effect |
| --- | --- | --- |
| TMDB | The API terms say charging users a fee for an app is commercial use, and commercial use "is only permitted under a separate written agreement between You and TMDB". Attribution with the TMDB logo and a non-endorsement notice is required. | A paid Kinosail app needs a written TMDB agreement before it calls TMDB from the device. |
| TheTVDB | The API "is available for both commercial projects and to individual developers"; commercial pricing is by contact with sales. Attribution with a link to TheTVDB.com is required. | Needs a commercial license; price unknown. |
| fanart.tv | Not checked in this pass | Confirm their terms for a paid app before use. |

Note: today Plex, Jellyfin and Emby supply their own metadata to the app, so these terms matter only for shares, local files and cloud drives.

### R3 — App Review for apps that play user media from third-party servers

- **2.1(a), demo access:** reviewers need "demo account info (and turn on your back-end service!)" when an app has a login; a built-in demo mode needs Apple's prior approval. Kinosail needs a demo server (M6).
- **5.2.2, third-party services:** an app that "uses, accesses, monetizes access to, or displays content from a third-party service" must be permitted to by that service's terms, and "authorization must be provided upon request". This applies to Plex, Jellyfin, Emby, Trakt, metadata providers and cloud drives. Plex's API terms need a specific check, because the app charges money and signs in through plex.tv.
- **5.2.3, downloading:** apps must not "save, convert, or download media from third-party sources … without explicit authorization from those sources". Offline downloads from Plex, Jellyfin, Emby and cloud drives need that authorization, or clear terms that allow them. Downloads from the user's own Kinosail Server and own shares are the safer case.
- **4.2.7 (remote desktop clients)** does not apply: Kinosail is a media player, not a mirror of the host.
- Screenshots and the demo must use public-domain or Owner-made media only.

## Decisions for the Owner

Each item needs the Owner. Engineering will not set prices, create App Store Connect records, sign agreements, spend money or change licensing.

| # | Decision | Recommendation |
| --- | --- | --- |
| M1 | Purchase model | Free download with a full-featured trial, then either a one-time unlock or a yearly subscription. Offer both if support load allows: a subscription funds ongoing source and codec work, and a lifetime unlock answers buyers who dislike subscriptions. Family Sharing on. Prices are the Owner's call. |
| M2 | What stays free in the app | Keep Kinosail Server playback free in the apps, so the README promise for current households stays true and the free Server keeps working as the funnel. Charge for Plex, Jellyfin, Emby, network shares and pro playback features. Alternative: one price for every source, with the web Player as the free path. |
| M3 | App Store Connect record, bundle IDs, Paid Apps agreement, tax and banking | Owner action. One app record with iOS and tvOS (universal purchase) under `com.kinosail.player`. |
| M4 | Metadata provider for network shares | Pick a provider whose terms allow use in a paid app, and confirm any commercial license or fee before work starts. Show the provider's attribution in the app. |
| M5 | D1 engine stage 2: DTS and TrueHD decode on the device | Stage 1 (demux, repackage, bitmap subtitles) needs no decision and covers 75% of files plus subtitles. Stage 2 affects 17.3% of files. Recommendation: an LGPL-only FFmpeg build as a dynamic framework limited to the DTS and TrueHD decoders, after counsel reviews the patent position (M12). Until then these files play through a server, or through a native alternate audio track when the file has one. |
| M6 | Demo access for App Review and screenshots | A small, internet-reachable demo server with public-domain films only, separate from the household Server. Opening anything to the internet is a security change and needs the Owner. |
| M7 | Terms for app binaries and app source | Use Apple's standard App Store licence for the binaries. Decide whether the native app source stays public under PolyForm Perimeter (see F2). |
| M8 | README and site wording | Approve the wording changes in F1 and F6 before launch. |
| M9 | Launch languages | English plus the 17 catalog languages only after the catalog is complete; otherwise English, German, French, Spanish, Japanese and Portuguese (Brazil) first. |
| M10 | Cloud drive app registrations | Later phase. Each provider needs a developer app registration in the Owner's name; Google's restricted Drive scopes need a paid yearly security assessment. Start with Dropbox and OneDrive, decide on Google Drive after measuring demand. |
| M11 | Apple TV app integration | Ask Apple only if the Owner wants to try; a personal-media player is unlikely to qualify. No engineering work until then. |
| M12 | Codec and Plex terms review (R1, R3) | Engage counsel before the first paid release that bundles the on-device engine, and before shipping downloads from third-party servers. |

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

- **D1, on-device playback engine.** Best-in-class players play files on the device that the platform cannot open. Kinosail is source-available under PolyForm Perimeter, so any FFmpeg use must be an LGPL-only build shipped as a dynamic framework with notices and relinking materials; GPL builds are not compatible. The measured list below shows that most of the engine needs no third-party decoder at all. The decoder part is the Owner's decision (M5).
- **D2, Apple TV lossless audio.** Apple TV cannot pass TrueHD or DTS bitstreams to a receiver. Kinosail converts to multichannel E-AC-3 instead of AAC. This is a platform limit, not a choice.

### Measured list for D1 (Nox library, 2026-10-31)

A random sample of 600 of the 4,084 movie and TV files, probed read-only with ffprobe inside the Player container. Only codec and container names were kept, no file names. Each file is counted once, by its default audio track.

| Group | Files | Share | What serverless playback needs |
| --- | --- | --- | --- |
| A. MP4/MOV with H.264 or HEVC and native audio | 23 | 3.8% | Nothing: AVFoundation plays it as-is |
| B. MKV or AVI with H.264 or HEVC and native audio (AAC, AC-3, E-AC-3, FLAC, Opus) | 426 | 71.0% | An on-device demuxer that repackages to fragmented MP4 for AVFoundation. No decoding, so no codec license question. |
| C. H.264 or HEVC with DTS (126) or TrueHD (17) default audio | 143 | 23.8% | An audio decoder plus an E-AC-3 or LPCM encode on the device. 39 of these files also have a native alternate audio track, which can play instead. The other 104 (17.3%) need the decoder. |
| D. Video Apple cannot decode on Apple TV 4K: AV1 (5), VC-1, MPEG-2, MPEG-4 Part 2 (1 each) | 8 | 1.3% | A software video decoder. AV1 plays in hardware on recent iPhones. |
| Files with PGS, VobSub or DVB bitmap subtitles (any group) | 198 | 33.0% | A bitmap subtitle renderer. PGS is a simple image format, not a codec with patent pools. |
| Files with ASS/SSA subtitles | 10 | 1.7% | A styled subtitle renderer (text works today as plain WebVTT) |

Engine stages that follow from this:

1. **Stage 1, no third-party decoders:** Matroska/AVI demuxer, fragmented-MP4 repackager into the existing loopback player path, and a PGS/VobSub renderer. Covers groups A and B (75%), the 39 group C files with a native alternate track, and bitmap subtitles. It can be written in Swift without FFmpeg, which avoids the LGPL question for this stage.
2. **Stage 2, audio decoders (needs M5):** DTS and TrueHD decode for the remaining 17.3%. This is the licensing and patent decision for the Owner (R1).
3. **Stage 3, video decoders (optional):** AV1, VC-1 and MPEG-2 software decode for 1.3%. Recommendation: skip, and point these files to a server.

## Phase plan

Ranked by what sells the player to a buyer without a Kinosail Server. Work that helps every user (frame-rate and HDR match, surround sound) is interleaved with the source work instead of finishing first. Each step ships as reviewable pull requests with tests and recorded verification.

| Step | Work | Who it sells to | Status or dependency |
| --- | --- | --- | --- |
| 1 | Match frame rate and dynamic range on Apple TV | Everyone, every source | Done (#820); hardware check pending |
| 2 | E-AC-3 5.1 instead of stereo for DTS-HD and TrueHD on a Kinosail Server | Kinosail Server users | Done (#819), deployed to Nox |
| 3 | Plex, Jellyfin and Emby playback quality: ask these servers for E-AC-3 surround instead of AAC, and for MKV remux instead of a transcode | Plex/Jellyfin/Emby users (largest group) | Next |
| 4 | Plex, Jellyfin and Emby library completeness: music, audiobooks, photos, collections, people; per-show audio and subtitle memory | Plex/Jellyfin/Emby users | After 3 |
| 5 | Engine stage 1: on-device MKV demux and repackage, PGS/VobSub renderer | Serverless users; also removes remote remuxes for Plex/Jellyfin/Emby | No Owner decision needed |
| 6 | Local playback: Files app, iCloud Drive, USB/SSD drives, "Open in Kinosail" from the Share Sheet and AirDrop | Serverless users | Needs 5 |
| 7 | Serverless library: on-device index with artwork, seasons, Continue watching, Up Next; CloudKit private-database sync | Serverless users | Metadata needs M4 |
| 8 | SMB with automatic discovery, then WebDAV and NFS | NAS owners | Needs 5 and 7 |
| 9 | Offline downloads from Plex, Jellyfin, Emby and shares | Everyone on iPhone/iPad | Plex/Jellyfin/Emby downloads need the R3 check (M12) |
| 10 | Engine stage 2: DTS and TrueHD decode on the device | Serverless users with DTS/TrueHD files (17.3% of files) | Needs M5 |
| 11 | Subtitles: styled ASS, offset, size/color/background/position, automatic forced tracks, in-app online search | Everyone | Bitmap rendering arrives in 5 |
| 12 | Dolby Vision P5/P8 and P7 fallback; stream quality cap | Owners of HDR libraries | No HDR or Dolby Vision files in the Nox sample, so lower |
| 13 | Trakt; DLNA/UPnP servers; FTP and SFTP | Enthusiasts | After 8 |
| 14 | Apple ecosystem: App Intents and Shortcuts, widgets and Live Activities, SharePlay with Watch Rooms, iPhone as Apple TV remote and keyboard, tvOS multi-user | Everyone | Any time after 4 |
| 15 | Player experience: next-episode countdown, chapters, audio delay, video sleep timer, direct-play thumbnails | Everyone | Any time |
| 16 | iPad: Split View, Slide Over, Stage Manager, external displays | iPad users | Any time |
| 17 | Cloud drives: Dropbox and OneDrive, then Box and pCloud, then Google Drive | Serverless users | Needs M10 |
| 18 | Measurement: time-to-first-frame, memory budgets, real-file playback matrix | Quality | Any time |

**Runs in parallel with every step:**

- **App Store readiness.** Localization and the native notices file now; StoreKit 2 entitlements, paywall and trial behind a build flag next; MetricKit reporting; then TestFlight once M3 and M6 are done.
- **Accessibility (P6).** A VoiceOver, Dynamic Type and caption-style pass recorded for each release; audio-description tracks with step 4.

Skipped on purpose: lossless TrueHD/DTS passthrough on Apple TV (platform limit, D2); Google Cast on Apple clients (no native sender SDK, already documented); Apple TV app integration until M11; engine stage 3 (1.3% of files).

## Later, not planned

Out of scope for this program by the Owner's decision of 2026-10-31. Kept here so the matrix shows them; no work is scheduled.

- Mac app (native or Catalyst).
- Apple Vision Pro.
- Android phone and Android TV parity with this matrix.
- Web Player parity with this matrix.
