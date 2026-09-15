# Popular open Plex and Jellyfin feature requests

Research snapshot: **2026-08-22 (America/Denver)**.

This note answers a narrow question: what do users repeatedly and popularly ask Plex and Jellyfin to add or finish? It measures demand from first-party request surfaces, not Reddit, blogs, app-store reviews, or general social media.

## Bottom line

The clearest cross-product demand is not another basic movie grid. Users repeatedly ask for:

1. **First-class books and audiobooks**, including proper library types, progress, metadata, and mobile use.
2. **Serious music and playlist behavior**, especially smart playlists, rich tags, gapless playback, cue sheets, and multi-artist metadata.
3. **Reliable group watching on every client**, with simple invitations and no regression when a client UI is replaced.
4. **Better organization and personal state**, including watchlists, watched history, control over Continue Watching, folders, and nested collections.
5. **Account and household controls**, especially SSO, 2FA, profile switching, local authentication resilience, and owner-to-viewer messaging.
6. **Predictable playback and remote streaming**, including sane quality defaults, efficient transcoding, and clearer audio/subtitle controls.

The strongest platform-specific signal is Jellyfin's unfinished offline experience. Three separate current requests cover complete Android offline mode, broader offline sync, and transcoded downloads. Plex's most distinctive very-high-vote asks are a reader for comics/books/PDFs, restoring Watch Together in the new experience, a true audiobook library, and richer playlists.

## Method

- **Plex:** queried the official [open Feature Suggestions list sorted by votes](https://forums.plex.tv/c/feature-suggestions/8/l/latest?order=votes&status=open). Plex says votes are a planning signal, not a promise, and closes implemented or rejected requests; each user can vote once per request ([forum rules](https://forums.plex.tv/t/please-read-before-submitting-a-feature-suggestion/273976)). “Posts” below is total thread posts, including the opening post. “Last activity” is the forum's `last_posted_at` value.
- **Jellyfin:** queried the official [Jellyfin Feature Requests board](https://features.jellyfin.org/) using its “Most Wanted” order. Jellyfin's own [forum sticky](https://forum.jellyfin.org/t-please-read-before-posting-please-send-new-feature-requests-to-fider) directs new requests to that board. The board says votes show support but do not guarantee work in its volunteer “do-ocracy.” “Comments” and statuses are the board's own current fields.
- **Recurrence:** checked the official Jellyfin feature-request forum and project-owned GitHub issues/discussions where they materially corroborated a theme. These signals are supporting evidence, not added to board vote totals.
- **Ranking:** vote totals are ranked only within a product. Plex and Jellyfin counts are not comparable because the communities and voting systems differ. Counts from related requests are shown separately and never summed because voter overlap is unknown.
- **Freshness:** headline items had to be open, planned, or started on 2026-08-22. Partially shipped requests are described as requests to *finish* the experience. Clearly completed, duplicate, declined, or obsolete requests were excluded.

## Shared demand, ranked conservatively

### 1. First-class books and audiobooks

This is the strongest exact cross-product overlap.

- Plex: [PLEXREADER: Comics, Books, PDFs](https://forums.plex.tv/t/plexreader-comics-books-pdfs/26684) — **3,177 votes**, 1,064 posts, opened 2013-01-08, last activity 2026-06-13, open.
- Plex: [Support for audiobooks](https://forums.plex.tv/t/support-for-audiobooks/27518) — **2,258 votes**, 1,154 posts, opened 2013-01-20, last activity 2026-04-04, open. The request calls for a separate audiobook library and remembered position.
- Jellyfin: [Audiobook support](https://features.jellyfin.org/posts/243/audiobook-support) — **852 votes**, 39 comments, opened 2019-08-11, latest comment 2026-05-27, **started**. Jellyfin's staff response says basic support exists in the Books library, so the live request is for a complete audiobook experience, not the mere ability to index a file.
- Jellyfin: [Calibre Library Access Plugin](https://features.jellyfin.org/posts/219/calibre-library-access-plugin) — **165 votes**, 6 comments, open.

**Product reading:** a polished audiobook slice is the safer common opportunity; comics/PDF reading is an additional Plex-heavy signal.

### 2. Serious playlists and music-library behavior

- Plex: [Better Playlists](https://forums.plex.tv/t/better-playlists/73590) — **1,425 votes**, 565 posts, opened 2014-08-13, last activity 2026-08-05, open.
- Plex: [Robust music-tag support](https://forums.plex.tv/t/tag-support-for-robust-music-library-organization/106326) — **862 votes**, 252 posts, last activity 2026-06-19, open.
- Plex: [Better multi-artist album/track support](https://forums.plex.tv/t/better-support-for-albums-and-tracks-with-multiple-artists/116658) — **514 votes**, 312 posts, last activity 2026-07-18, open.
- Plex: [CUE support for FLAC](https://forums.plex.tv/t/cue-support-for-flac-files/96352) — **504 votes**, 210 posts, last activity 2026-08-18, open.
- Jellyfin: [Gapless playback](https://features.jellyfin.org/posts/181/gapless-playback) — **642 votes**, 34 comments, opened 2019-07-29, latest comment 2026-03-31, open. A project-owned [2026 implementation discussion](https://github.com/jellyfin/jellyfin-web/discussions/7568) was still active on 2026-08-17.
- Jellyfin: [Dynamic/smart playlists](https://features.jellyfin.org/posts/49/add-support-for-dynamic-smart-playlists) — **585 votes**, 40 comments, latest comment 2026-02-16, planned.
- Jellyfin: [Podcast support](https://features.jellyfin.org/posts/48/podcast-support) — **560 votes**, 20 comments, latest comment 2026-07-11, open.
- Jellyfin: [Cue-sheet support](https://features.jellyfin.org/posts/1190/cuesheet-support) — **167 votes**, 48 comments, open.

**Product reading:** “music support” is too broad. The repeated asks are concrete: smart lists, reliable long-play/gapless behavior, rich tags, and correct multi-artist/cue semantics.

### 3. Group watching that works across clients

- Plex: [Add Watch Together to New Plex Experience](https://forums.plex.tv/t/add-watch-together-to-new-plex-experience/906941) — **2,848 votes**, 341 posts, opened 2025-02-25, last activity 2026-08-16, open. This is a restoration/parity request after the new experience replaced an older client experience.
- Plex: [Tandem Playback to several clients](https://forums.plex.tv/t/tandem-playback-to-several-clients/38777) — **421 votes**, 443 posts, last activity 2026-07-15, open.
- Jellyfin: [SyncPlay invite-to-watch link](https://features.jellyfin.org/posts/971/syncplay-invite-to-watch-link) — **261 votes**, 2 comments, open.
- Jellyfin: [Watch together with multiple accounts](https://features.jellyfin.org/posts/389/watch-together-watch-with-multiples-accounts) — **164 votes**, 11 comments, open.
- Jellyfin forum recurrence: [SyncPlay / watch together on Android TV](https://forum.jellyfin.org/t-syncplay-watch-together-android-tv) had **34 replies**, **40,884 views**, and activity on 2026-08-17.

**Product reading:** synchronized playback itself is partly shipped in both ecosystems; the unmet demand is universal client availability, invitations, and dependable household/remote UX.

### 4. Better library organization and personal state

- Plex: [Nested Collections](https://forums.plex.tv/t/nested-collections/354043) — **514 votes**, 175 posts, last activity 2026-07-08, open.
- Jellyfin: [Remove an item from Continue Watching](https://features.jellyfin.org/posts/517/add-an-option-to-remove-an-item-from-continue-watching) — **1,706 votes**, 51 comments, opened 2020-03-26, latest comment 2026-08-02, planned.
- Jellyfin: [Netflix-like watchlist](https://features.jellyfin.org/posts/576/watchlist-like-netflix) — **1,276 votes**, 52 comments, latest comment 2026-08-09, planned. A project-owned [implementation pull request](https://github.com/jellyfin/jellyfin/pull/17504) remained open on 2026-08-22, so it is active work, not shipped behavior.
- Jellyfin: [Watched History](https://features.jellyfin.org/posts/633/watched-history) — **821 votes**, 21 comments, latest comment 2026-07-16, planned.
- Jellyfin: [Folder view for libraries](https://features.jellyfin.org/posts/224/add-a-folder-view-to-libraries) — **437 votes**, 134 comments, latest comment 2026-02-11, open.
- Jellyfin: [Nested collections](https://features.jellyfin.org/posts/4039/support-nested-collections-collections-within-collections) — **36 votes**, 10 comments, opened 2026-07-16, open. It is too new to call “very popular,” but it confirms current exact recurrence with Plex.

**Product reading:** Jellyfin demand is especially strong for controlling home-screen state; hierarchy/folder demand is shared but less dominant.

### 5. Voice assistants and smart-home control

- Plex: [Google Home integration](https://forums.plex.tv/t/google-home-integration/237823) — **2,461 votes**, 346 posts, opened 2018-06-05, open. Its last activity was 2023-12-15, so this is a huge historic-open signal rather than a currently fast-moving thread.
- Jellyfin: [Google Assistant and Alexa](https://features.jellyfin.org/posts/275/support-google-assistant-and-alexa) — **243 votes**, 12 comments, open.

**Product reading:** exact recurrence exists, but current activity is weaker than the top four themes.

### 6. Account, household, and server-owner controls

- Plex: [Send server messages](https://forums.plex.tv/t/send-server-messages/23773) — **1,196 votes**, 633 posts, opened 2012-11-24, last activity 2026-02-05, open.
- Plex: [Built-in local authentication for plex.tv outages](https://forums.plex.tv/t/feature-request-built-in-local-authentication-server-prevent-plex-tv-outage/111339) — **369 votes**, 94 posts, last activity 2026-08-12, open.
- Jellyfin: [OIDC/OAuth SSO](https://features.jellyfin.org/posts/230/support-for-oidc-oauth-sso) — **1,165 votes**, 43 comments, latest comment 2026-08-05, planned. A separate project-owned [2026 native-SSO discussion](https://github.com/orgs/jellyfin/discussions/16470) had **91 reactions**, 11 comments, and activity on 2026-08-22.
- Jellyfin: [Two-factor authentication](https://features.jellyfin.org/posts/26/add-support-for-two-factor-authentication-2fa) — **1,086 votes**, 83 comments, latest comment 2026-05-24, planned.
- Jellyfin: [Multiple-account switching](https://features.jellyfin.org/posts/2348/multiple-account-switch) — **150 votes**, 14 comments, started and only partly available across clients. The project-owned [multi-user profile-switching discussion](https://github.com/jellyfin/jellyfin-web/discussions/7059) remains open.

**Product reading:** the products start from different identity models, so this is a shared problem area, not one shared implementation. Owners want local resilience, stronger authentication, easy household switching, and a communication path to viewers.

### 7. Predictable remote playback, transcoding, and track controls

- Plex: [Default all clients to maximum internet streaming quality](https://forums.plex.tv/t/default-all-clients-to-max-internet-streaming/440641) — **1,287 votes**, 1,726 posts, opened 2019-08-02, last activity 2026-06-01, open.
- Plex: [AMD VCN hardware encoding](https://forums.plex.tv/t/feature-request-add-support-for-amds-video-core-next-encoding/226861) — **849 votes**, 194 posts, last activity 2026-02-23, open.
- Plex: [Correctly identify Atmos, DTS:X, and DTS-HD MA](https://forums.plex.tv/t/correctly-identify-display-dolby-atmos-dts-x-and-dts-hd-ma-audio-tracks-in-plex-clients/209431) — **700 votes**, 181 posts, last activity 2026-05-10, open.
- Plex: [IPv6 support for myPlex](https://forums.plex.tv/t/ipv6-support-for-myplex/36520) — **549 votes**, 444 posts, last activity 2026-07-16, open.
- Jellyfin: [Allow global defaults](https://features.jellyfin.org/posts/469/allow-global-defaults) — **255 votes**, 33 comments, open.
- Jellyfin: [Improve subtitle appearance/options](https://features.jellyfin.org/posts/161/improve-subtitle-appearance-options) — **186 votes**, 30 comments, open.
- Jellyfin: [Filter by audio-stream language](https://features.jellyfin.org/posts/39/filter-videos-by-audio-stream-language) — **171 votes**, 19 comments, open.

**Product reading:** exact feature requests differ, but the repeated outcome is consistent: avoid unnecessary transcoding, make quality policy owner-controllable, and expose track capabilities clearly.

### 8. Better live-TV and lean-back organization

- Plex: [PseudoTV](https://forums.plex.tv/t/pseudotv/99226) — **620 votes**, 235 posts, last activity 2026-07-31, open; it asks for scheduled channels made from an owner's library.
- Plex: [Multiple EPG sources for multiple tuners](https://forums.plex.tv/t/multiple-epg-sources-for-those-with-multiple-tuners/192844) — **611 votes**, 279 posts, last activity 2025-08-24, open.
- Jellyfin: [IPTV channel groupings](https://features.jellyfin.org/posts/186/iptv-channel-groupings) — **375 votes**, 107 comments, latest comment 2026-06-27, planned.

**Product reading:** Live TV is a narrower audience than core playback, but within that audience channel grouping, guide-source control, and a “turn it on and watch” mode recur strongly.

## Biggest platform-specific open requests

### Plex-only or strongly Plex-skewed

| Rank | Request | Popularity and freshness | Current interpretation |
| ---: | --- | --- | --- |
| 1 | [PLEXREADER](https://forums.plex.tv/t/plexreader-comics-books-pdfs/26684) | 3,177 votes; 1,064 posts; active 2026-06-13 | Dedicated comics/books/PDF server and client experience |
| 2 | [Watch Together in the new experience](https://forums.plex.tv/t/add-watch-together-to-new-plex-experience/906941) | 2,848 votes; 341 posts; active 2026-08-16 | Restore a removed/absent client capability |
| 3 | [Audiobook library](https://forums.plex.tv/t/support-for-audiobooks/27518) | 2,258 votes; 1,154 posts; active 2026-04-04 | Separate type, progress, metadata, and mobile behavior |
| 4 | [Better Playlists](https://forums.plex.tv/t/better-playlists/73590) | 1,425 votes; 565 posts; active 2026-08-05 | Sorting, bulk editing, imports, watchlist-like behavior |
| 5 | [Max internet quality by default](https://forums.plex.tv/t/default-all-clients-to-max-internet-streaming/440641) | 1,287 votes; 1,726 posts; active 2026-06-01 | Stop low client defaults from forcing avoidable transcodes |
| 6 | [Server messages](https://forums.plex.tv/t/send-server-messages/23773) | 1,196 votes; 633 posts; active 2026-02-05 | Owner-to-viewer maintenance/status notices |
| 7 | [Robust music tags](https://forums.plex.tv/t/tag-support-for-robust-music-library-organization/106326) | 862 votes; 252 posts; active 2026-06-19 | First-class ID3/Vorbis semantics |
| 8 | [AMD VCN encoding](https://forums.plex.tv/t/feature-request-add-support-for-amds-video-core-next-encoding/226861) | 849 votes; 194 posts; active 2026-02-23 | Broader hardware-transcode support |
| 9 | [Atmos/DTS metadata labels](https://forums.plex.tv/t/correctly-identify-display-dolby-atmos-dts-x-and-dts-hd-ma-audio-tracks-in-plex-clients/209431) | 700 votes; 181 posts; active 2026-05-10 | Accurate audio capability display in clients |
| 10 | [HEIC/HEIF photo support](https://forums.plex.tv/t/heic-heif-support-in-photo-libraries-new-photo-format-for-iphones/195514) | 632 votes; 230 posts; active 2026-03-23 | Modern iPhone photo-library ingestion/display |

[PseudoTV](https://forums.plex.tv/t/pseudotv/99226) is also notable at **620 votes**, 235 posts, and activity on 2026-07-31: users want scheduled “channels” made from their own library.

### Jellyfin-only or strongly Jellyfin-skewed

| Rank | Request | Popularity and freshness | Board status / current interpretation |
| ---: | --- | --- | --- |
| 1 | [Complete Android offline mode](https://features.jellyfin.org/posts/218/support-offline-mode-on-android-mobile) | 1,804 votes; 81 comments; latest 2026-02-23 | Started; original-file download exists, but managed in-app offline playback remains incomplete |
| 2 | [Remove from Continue Watching](https://features.jellyfin.org/posts/517/add-an-option-to-remove-an-item-from-continue-watching) | 1,706 votes; 51 comments; latest 2026-08-02 | Planned |
| 3 | [Watchlist](https://features.jellyfin.org/posts/576/watchlist-like-netflix) | 1,276 votes; 52 comments; latest 2026-08-09 | Planned; implementation PR still open |
| 4 | [OIDC/OAuth SSO](https://features.jellyfin.org/posts/230/support-for-oidc-oauth-sso) | 1,165 votes; 43 comments; latest 2026-08-05 | Planned and independently active on project GitHub |
| 5 | [Two-factor authentication](https://features.jellyfin.org/posts/26/add-support-for-two-factor-authentication-2fa) | 1,086 votes; 83 comments; latest 2026-05-24 | Planned |
| 6 | [Full audiobook experience](https://features.jellyfin.org/posts/243/audiobook-support) | 852 votes; 39 comments; latest 2026-05-27 | Started; basic Books-library support exists |
| 7 | [Watched History](https://features.jellyfin.org/posts/633/watched-history) | 821 votes; 21 comments; latest 2026-07-16 | Planned |
| 8 | [Offline Sync](https://features.jellyfin.org/posts/1341/offline-sync-feature) | 807 votes; 36 comments; latest 2025-12-06 | Planned; broader than the Android-only request |
| 9 | [External/MySQL database backend](https://features.jellyfin.org/posts/315/mysql-server-back-end) | 653 votes; 52 comments; latest 2026-07-28 | Started; staff says EF Core groundwork landed and PostgreSQL may precede MySQL |
| 10 | [Gapless playback](https://features.jellyfin.org/posts/181/gapless-playback) | 642 votes; 34 comments; latest 2026-03-31 | Open and under renewed project discussion |
| 11 | [Smart playlists](https://features.jellyfin.org/posts/49/add-support-for-dynamic-smart-playlists) | 585 votes; 40 comments; latest 2026-02-16 | Planned |
| 12 | [Temporary direct-file links](https://features.jellyfin.org/posts/72/temporary-direct-file-sharing-links) | 575 votes; 41 comments; latest 2025-02-27 | Open |
| 13 | [Podcast support](https://features.jellyfin.org/posts/48/podcast-support) | 560 votes; 20 comments; latest 2026-07-11 | Open |
| 14 | [Download transcoded files](https://features.jellyfin.org/posts/57/support-download-of-transcoded-files) | 510 votes; 42 comments; latest 2026-03-18 | Planned; a key part of a storage-efficient offline experience |
| 15 | [Folder view](https://features.jellyfin.org/posts/224/add-a-folder-view-to-libraries) | 437 votes; 134 comments; latest 2026-02-11 | Open |

## Important exclusions and caveats

- **Do not sum related requests.** Jellyfin's 1,804 Android-offline, 807 offline-sync, and 510 transcoded-download votes establish recurrence, not 3,121 unique voters.
- **Partially implemented is not absent.** Jellyfin can download original files and has basic audiobook support; both boards still mark the fuller requests “started.” Plex already had Watch Together in an older experience; the 2,848-vote request is specifically for the new experience.
- **Status fields may lag reality.** Jellyfin's 1,171-vote lazy-loading request is marked “started,” but its staff response dates to 2020 and the latest comment is 2025-08-08. It was left out of the main platform list because the remaining scope is unclear.
- **High demand does not imply architectural fit.** Jellyfin's [Pre-transcoding](https://features.jellyfin.org/posts/570/pre-transcoding) has 977 votes and remains open, but a 2025 staff response says it is not planned for core and may be a plugin/external-tool concern. It is demand evidence, not a safe core-product recommendation.
- **Blocked client-store requests are weak product signals.** Jellyfin's VIDA OS and Samsung/Tizen requests were not ranked because platform access/store publication—not merely server capability—controls delivery, and some client support already exists or is in testing.
- **Old-but-open needs caution.** Plex's Google Home request is the third-most-voted open suggestion, but it had no activity after 2023-12-15. The count proves historic demand; it does not prove current momentum.
- **Views and posts are not votes.** They help show sustained discussion but may include the same people repeatedly. Votes are still self-selected and not representative market research.
- **This is a demand map, not a roadmap.** Both projects explicitly warn that votes do not guarantee implementation. Kinosail should use these requests as discovery input, then test the smallest coherent workflows against its one-container, API-driven, local-first product boundary.
