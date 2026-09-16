# Plex, Emby, Jellyfin, and Kinosail missing-feature request report

Research snapshot: **2026-08-26 (America/Denver)**.

The request said “Jellyfish.” This report uses the product name **Jellyfin**.

## Executive summary

This report now focuses on the highest-demand **open or incomplete requests** for Plex, Emby, and Jellyfin. It does not list ordinary features that those products already provide.

Kinosail already covers several of these requests, including passkeys, two-factor authentication, smart playlists, Watch Rooms, audiobooks, server notifications, API access, backups, and direct-first playback. The largest Kinosail gaps are native clients and casting, full offline clients, comics/PDF reading, richer music metadata, broad Trakt and smart-home integrations, bulk metadata editing, and a safe extension model.

## Method and limits

- Each product has a separate 20-item table. These are request-level items, not broad feature areas.
- Plex is ordered from its official open Feature Suggestions board. Jellyfin is ordered from its official Feature Requests board and corroborating project issues. Emby is ordered from the official forum's reactions, replies, and recurring request threads.
- Plex, Emby, and Jellyfin use different demand systems. Counts are therefore comparable only within one product.
- Emby has no comparable numeric vote board. Its official forum says starter-post likes act as votes, so rows without a visible count are marked as such.
- “Missing” includes absent capability, incomplete client coverage, or a request marked started/planned/in-progress but not finished.
- Competitor status means the server or first-party product documents the capability. It does not mean every client supports every variant.
- Kinosail status uses the committed tree at `7385caf` (`HEAD`). Uncommitted local work was excluded from shipped status. Repository paths below are evidence pointers, not claims of physical-device certification.

Legend: **✅ core**, **◐ partial or conditional**, **— not a core capability**.

## Requested missing or incomplete features

Status uses **✅ present**, **◐ partial**, and **— missing**. A request can remain listed when the product has a limited version but the official request asks for a complete experience.

### Plex — 20 open requests

| Rank | Requested feature | Demand signal | What Plex is missing or has not finished | Kinosail |
| ---: | --- | --- | --- | --- |
| 1 | [PLEXREADER: comics, books, PDFs](https://forums.plex.tv/t/plexreader-comics-books-pdfs/26684) | 1,063 replies; 83,021 views; top open item ([D14]) | Native reader and reading-position sync | ◐ EPUB reader; no comics/PDF reader |
| 2 | [Watch Together in the new experience](https://forums.plex.tv/t/add-watch-together-to-new-plex-experience/906941) | 342 replies; 21,071 views | Full new-client coverage | ✅ Watch Rooms; clients remain limited |
| 3 | [Google Home integration](https://forums.plex.tv/t/google-home-integration/237823) | 345 replies; 53,908 views | Native Assistant discovery and control | — |
| 4 | [Audiobook support](https://forums.plex.tv/t/support-for-audiobooks/27518) | 1,153 replies; 50,741 views | Complete audiobook library and progress model | ◐ Audiobooks and progress |
| 5 | [Better Playlists](https://forums.plex.tv/t/better-playlists/73590) | 564 replies; 18,748 views | Richer rules, sharing, editing, and imports | ◐ Smart playlists; no full import/share depth |
| 6 | [Max internet quality by default](https://forums.plex.tv/t/default-all-clients-to-max-internet-streaming/440641) | 1,725 replies; 69,508 views | Server-wide maximum-quality default | ✅ Policy-based quality controls |
| 7 | [Send server messages](https://forums.plex.tv/t/send-server-messages/23773) | 632 replies; 17,785 views | Built-in owner-to-viewer messages | ✅ Notifications; richer user messaging remains |
| 8 | [Robust music tags](https://forums.plex.tv/t/tag-support-for-robust-music-library-organization/106326) | 251 replies; 18,783 views | Flexible multi-tag music organization | ◐ Embedded tags; richer semantics remain |
| 9 | Transcode to HEVC/H.265 | Current open board item ([D14]) | More efficient remote transcode target | ◐ HLS/transcode paths; certification incomplete |
| 10 | Rename and reorder Live TV channels | Current open board item ([D14]) | Complete channel-management workflow | ◐ Channel management; richer ordering remains |
| 11 | Persistent automatic skip-intro playback | Current open board item ([D14]) | User-controlled persistent skip behavior | ◐ Markers and skip actions |
| 12 | Buffer remote content like YouTube | Current open board item ([D14]) | Predictive remote buffering | — |
| 13 | Custom skip-back duration | Current open board item ([D14]) | User-defined player skip interval | ◐ Player controls; custom interval remains |
| 14 | Show names for all audio tracks and subtitles | Current open board item ([D14]) | Consistent client track labels | ◐ Track metadata; client coverage incomplete |
| 15 | Hardware transcoding on Raspberry Pi | Current open board item ([D14]) | Broad ARM hardware support | ◐ Hardware backends; device proof incomplete |
| 16 | Chromecast Live TV | Current open board item ([D14]) | Reliable Live TV casting | — |
| 17 | Full Trakt integration | Current open board item ([D14]) | Complete two-way Trakt workflows | — |
| 18 | Local authentication during Plex outage | 91 replies on current board ([D14]) | Server-local login independent of Plex cloud | ✅ Local-first auth |
| 19 | Download entire library or playlist in Plexamp | Current open board item ([D14]) | Bulk music offline workflow | ◐ Server download jobs; no Plexamp client |
| 20 | Native NFO support | Current open board item ([D14]) | First-class NFO metadata ingestion | ✅ NFO import |

### Emby — 20 open or incomplete requests

Emby’s official forum asks users to vote by liking the first post. It does not provide a stable numeric leaderboard, so this is a high-signal shortlist based on reactions, replies, current forum sorting, and recurring requests.

| Rank | Requested feature | Demand signal | What Emby is missing or has not finished | Kinosail |
| ---: | --- | --- | --- | --- |
| 1 | [Smart Playlists/Views](https://emby.media/community/topic/36916-smart-playlistsviews/) | 174 reactions; 27 followers | Dynamic rule-based playlists/views | ✅ Smart playlists |
| 2 | [Native 2FA](https://emby.media/community/topic/55542-2-factor-authentication-2fa/) | 137 reactions | Server and account two-factor flow | ✅ TOTP and passkeys |
| 3 | [Trakt recommendations](https://emby.media/community/topic/45908-server-integrate-trakt-recommendations/) | 133 reactions | Trakt-powered recommendation rows | — |
| 4 | [TV-series extras folder types](https://emby.media/community/topic/55915-emby-server-theater-additional-extrasspecial-folder-types-for-tv-series-similar-to-movies/) | 113 reactions; in progress | Complete extras organization and display | ◐ Metadata/extras depth varies |
| 5 | [Reusable user groups](https://emby.media/community/topic/45623-user-groups/) | 112 reactions | Group-based permissions | ◐ Profile policies; no groups |
| 6 | [Multicast support](https://emby.media/community/topic/17805-multi-cast-support/) | 106 reactions | Multicast delivery option | — |
| 7 | [Recommend media to other users](https://emby.media/community/topic/24045-recommend-movies-or-shows-to-other-users/) | 101 reactions | Native user-to-user recommendations | — |
| 8 | [Bulk metadata editor](https://emby.media/community/topic/27732-bulk-meta-data-editor/) | 85 reactions; 146 replies | Complete bulk metadata workflow | ◐ Metadata editing; bulk depth remains |
| 9 | ReplayGain/R128 normalization | 40 reactions; recurring music request ([D16]) | Server/client loudness normalization | — |
| 10 | OpenID/SSO | 44 reactions; recurring request ([D16]) | Consistent SSO across clients | ✅ OIDC linking |
| 11 | More TVDB display orders | 53 reactions ([D16]) | Complete alternate episode ordering | ◐ Metadata ordering is limited |
| 12 | WebSocket keepalive | 27 reactions ([D16]) | Stable long-lived client connection | ◐ API/web events; coverage remains |
| 13 | Internal links for albums, playlists, collections | 3 reactions; 13 replies | Shareable native internal links | ◐ API links; user-facing sharing remains |
| 14 | HDR content filter | 2 reactions; recurring request | Direct HDR filtering | ◐ Media filters; HDR-specific filter remains |
| 15 | Passkey support | 3 reactions; active request | Passwordless account login | ✅ Passkeys |
| 16 | Separate multiple movie editions | 10 reactions; 35 replies | First-class edition presentation | ◐ Multiple versions; edition model remains |
| 17 | Sort collections by last item added | 14 reactions; 23 replies | Dynamic collection recency sort | ◐ Sorts exist; collection depth remains |
| 18 | Module or extension support | 5 reactions; 22 replies | Safe supported extension seam | — |
| 19 | Continue Listening for music | Recurring request; no stable count | Resume music queues across devices | ◐ Progress exists; queue handoff remains |
| 20 | Playlist of playlists | Recurring request; open forum thread | Nested playlists and grouped music works | ◐ Playlists; no playlist nesting |

### Jellyfin — 20 open or incomplete requests

| Rank | Requested feature | Demand signal | What Jellyfin is missing or has not finished | Kinosail |
| ---: | --- | --- | --- | --- |
| 1 | [Android offline mode](https://features.jellyfin.org/posts/218/support-offline-mode-on-android-mobile) | 1,804 votes; started | Managed in-app offline playback | ◐ Server download jobs; client certification remains |
| 2 | [Remove from Continue Watching](https://features.jellyfin.org/posts/517/add-an-option-to-remove-an-item-from-continue-watching) | 1,706 votes; planned | User-controlled home-screen cleanup | ✅ |
| 3 | [Netflix-like watchlist](https://features.jellyfin.org/posts/576/watchlist-like-netflix) | 1,276 votes; planned | First-class watchlist | ✅ My List |
| 4 | [OIDC/OAuth SSO](https://features.jellyfin.org/posts/230/support-for-oidc-oauth-sso) | 1,165 votes; planned | Consistent native-client SSO | ✅ OIDC linking |
| 5 | [Two-factor authentication](https://features.jellyfin.org/posts/26/add-support-for-two-factor-authentication-2fa) | 1,086 votes; planned | Native server-wide 2FA | ✅ TOTP |
| 6 | [Audiobook support](https://features.jellyfin.org/posts/243/audiobook-support) | 852 votes; started | Complete audiobook workflow | ◐ |
| 7 | [Watched History](https://features.jellyfin.org/posts/633/watched-history) | 821 votes; planned | Complete history view | ✅ |
| 8 | [Offline Sync](https://features.jellyfin.org/posts/1341/offline-sync-feature) | 807 votes; planned | Cross-client managed sync | ◐ |
| 9 | [External database backend](https://features.jellyfin.org/posts/315/mysql-server-back-end) | 653 votes; started | Supported external database options | — One-container SQLite boundary |
| 10 | [Gapless playback](https://features.jellyfin.org/posts/181/gapless-playback) | 642 votes; open | Reliable gapless music playback | ◐ Low-gap queue; client proof remains |
| 11 | [Dynamic/smart playlists](https://features.jellyfin.org/posts/49/add-support-for-dynamic-smart-playlists) | 585 votes; planned | Rule-based playlist generation | ✅ |
| 12 | [Temporary direct-file links](https://features.jellyfin.org/posts/72/temporary-direct-file-sharing-links) | 575 votes; open | Expiring direct share URLs | ◐ Authenticated downloads; link workflow remains |
| 13 | [Podcast support](https://features.jellyfin.org/posts/48/podcast-support) | 560 votes; open | Native podcast ingestion and playback | — |
| 14 | [Download transcoded files](https://features.jellyfin.org/posts/57/support-download-of-transcoded-files) | 510 votes; planned | Storage-efficient prepared downloads | ◐ Prepared downloads; client workflow remains |
| 15 | [Folder view](https://features.jellyfin.org/posts/224/add-a-folder-view-to-libraries) | 437 votes; open | Filesystem-oriented library view | ◐ Folder paths exist; view remains |
| 16 | [IPTV channel groupings](https://features.jellyfin.org/posts/186/iptv-channel-groupings) | 375 votes; planned | User-defined channel groups | ◐ Live TV; grouping depth remains |
| 17 | [SyncPlay invite link](https://features.jellyfin.org/posts/971/syncplay-invite-to-watch-link) | 261 votes; open | Simple shareable group-watch invitation | ✅ Watch Rooms; invite polish remains |
| 18 | [Google Assistant and Alexa](https://features.jellyfin.org/posts/275/support-google-assistant-and-alexa) | 243 votes; open | Native smart-home control | — |
| 19 | [Pre-transcoding](https://features.jellyfin.org/posts/570/pre-transcoding) | 977 votes; open; not planned for core | Scheduled prepared media outside plugins | ◐ Prepared downloads; no general pre-transcode queue |
| 20 | [Multi-user profile switching](https://github.com/jellyfin/jellyfin-web/issues/7447) | Open project issue | Fast household switching on shared devices | ✅ Profiles; client polish remains |

These lists identify demand and gaps. They do not claim that votes guarantee implementation, and they do not add related-request votes together.

## Recommended backlog

| Priority | Work | Why it matters |
| --- | --- | --- |
| P0 | Finish and certify offline clients. | It is the clearest cross-product demand gap and the main place where server-side readiness does not prove user success. |
| P0 | Expand native-client and casting strategy. | Server parity has limited value if a household cannot reach a TV or mobile device without a browser. |
| P1 | Improve music semantics and the book/audio reader. | These are repeated high-vote requests and fit Kinosail’s existing media model. |
| P1 | Add tuner discovery and richer DVR rules. | Current M3U/XMLTV coverage is useful but does not match the normal tuner-based setup path. |
| P1 | Strengthen Watch Rooms and cross-device state. | Kinosail has the server seam; the remaining work is invitations, reconnection, client reach, and conflict rules. |
| P2 | Add nested collections, richer list import/export, and folder views. | These improve organization without changing the one-container architecture. |
| P2 | Design a narrow, sandboxed extension seam. | Integrations are valuable, but arbitrary plugins would weaken security and the supported deployment boundary. |

## Sources

### First-party product documentation

- **P1** — [Plex: What is Plex?](https://support.plex.tv/articles/200288286-what-is-plex/)
- **P2** — [Plex: Library overview](https://support.plex.tv/articles/201282253-overview/)
- **P3** — [Plex: Local media assets](https://support.plex.tv/articles/200220677-local-media-assets-movies/)
- **P4** — [Plex Support index](https://support.plex.tv/articles/)
- **P5** — [Plex: Lists](https://support.plex.tv/articles/lists/)
- **P6** — [Plex: Sync watch state and ratings](https://support.plex.tv/articles/sync-watch-state-and-ratings/)
- **P7** — [Plex Home and parental controls](https://support.plex.tv/articles/203815766-what-is-plex-home/)
- **P8** — [Plex: Remote streaming](https://support.plex.tv/articles/200289506-remote-access/)
- **P9** — [Plex: Relay](https://support.plex.tv/articles/216766168-accessing-a-server-through-relay/)
- **P10** — [Plex: Direct Play, Direct Stream, and transcoding](https://support.plex.tv/articles/200430303-streaming-overview/)
- **P11** — [Plex: HDR to SDR tone mapping](https://support.plex.tv/articles/hdr-to-sdr-tone-mapping/)
- **P12** — [Plex: Subtitle support](https://support.plex.tv/articles/categories/your-media/using-subtitles/)
- **P13** — [Plex: Credits detection](https://support.plex.tv/articles/credits-detection/)
- **P14** — [Plex: Downloads overview](https://support.plex.tv/articles/downloads-overview/)
- **P15** — [Plex: Downloads for mobile and desktop](https://support.plex.tv/articles/download-ios-android/)
- **P16** — [Plex: Scanners, including photos](https://support.plex.tv/articles/200241548-scanners/)
- **P17** — [Plex: Live TV and DVR FAQ](https://support.plex.tv/articles/226463767-frequently-asked-questions-dvr-live-tv/)
- **P18** — [Plex: Watch Together](https://support.plex.tv/articles/watch-together/)
- **P19** — [Plex: Supported devices and apps](https://support.plex.tv/articles/200288286-what-is-plex/)
- **P20** — [Plex: Webhooks](https://support.plex.tv/articles/115002267687-webhooks/)

- **E1** — [Emby: Quick Start](https://emby.media/support/articles/Quick-Start.html)
- **E2** — [Emby Premiere Feature Matrix](https://emby.media/support/articles/Premiere-Feature-Matrix.html)
- **E3** — [Emby: Users](https://support.emby.media/support/articles/Users.html)
- **E4** — [Emby: Metadata Manager](https://support.emby.media/support/articles/Metadata-manager.html)
- **E5** — [Emby: Playlist management](https://support.emby.media/support/articles/Playlist-Manual-Migration.html)
- **E6** — [Emby: Parental Controls](https://emby.media/support/articles/Parental-Controls.html)
- **E7** — [Emby: Plugins](https://emby.media/support/articles/Plugins.html)
- **E8** — [Emby: Intro Skip and Cinema Intros](https://support.emby.media/support/articles/Intro-Skip.html)
- **E9** — [Emby: Downloads and Sync](https://support.emby.media/support/articles/Sync.html)
- **E10** — [Emby: Folder Sync](https://support.emby.media/support/articles/Folder-Sync.html)
- **E11** — [Emby: Live TV setup](https://support.emby.media/support/articles/Live-TV.html)
- **E12** — [Emby: DVR settings](https://emby.media/support/articles/DVR-Settings.html)
- **E13** — [Emby: Watch-together request](https://emby.media/community/topic/52954-feature-request-watching-together/)
- **E14** — [Emby: Backup and restore](https://support.emby.media/support/articles/Backup.html)
- **E15** — [Emby: Notifications](https://support.emby.media/support/articles/Notifications.html)

- **J1** — [Jellyfin: Product overview](https://jellyfin.org/)
- **J2** — [Jellyfin: Libraries](https://jellyfin.org/docs/general/server/libraries/)
- **J3** — [Jellyfin: Users](https://jellyfin.org/docs/general/server/users/)
- **J4** — [Jellyfin: Clients](https://jellyfin.org/docs/general/clients/)
- **J5** — [Jellyfin: Managing users](https://jellyfin.org/docs/general/server/users/adding-managing-users/)
- **J6** — [Jellyfin: Networking](https://jellyfin.org/docs/general/post-install/networking/)
- **J7** — [Jellyfin: Plugins](https://jellyfin.org/docs/general/server/plugins/)
- **J8** — [Jellyfin: Transcoding](https://jellyfin.org/docs/general/post-install/transcoding/)
- **J9** — [Jellyfin: Codec support](https://jellyfin.org/docs/general/clients/codec-support/)
- **J10** — [Jellyfin: Media segments](https://jellyfin.org/docs/general/server/metadata/media-segments/)
- **J11** — [Jellyfin: Downloads in user management](https://jellyfin.org/docs/general/server/users/adding-managing-users/)
- **J12** — [Jellyfin: Live TV setup](https://jellyfin.org/docs/general/server/live-tv/setup-guide/)
- **J13** — [Jellyfin: TVHeadend and tuner integration](https://jellyfin.org/docs/general/server/plugins/tvheadend/)
- **J14** — [Jellyfin: Live TV post-processing](https://jellyfin.org/docs/general/server/live-tv/post-process/)
- **J15** — [Jellyfin: SyncPlay API](https://kotlin-sdk.jellyfin.org/dokka/jellyfin-api/org.jellyfin.sdk.api.operations/-sync-play-api/index.html)
- **J16** — [Jellyfin: DLNA](https://jellyfin.org/docs/general/post-install/networking/dlna/)
- **J17** — [Jellyfin: Backup and restore](https://jellyfin.org/docs/general/administration/backup-and-restore/)
- **J18** — [Jellyfin: Monitoring and administration docs](https://jellyfin.org/docs/)

### Official demand signals

- **D1** — [Plex: Better Playlists](https://forums.plex.tv/t/better-playlists/73590)
- **D2** — [Jellyfin: Dynamic/smart playlists](https://features.jellyfin.org/posts/49/add-support-for-dynamic-smart-playlists)
- **D3** — [Jellyfin: Remove an item from Continue Watching](https://features.jellyfin.org/posts/517/add-an-option-to-remove-an-item-from-continue-watching)
- **D4** — [Jellyfin: Netflix-like watchlist](https://features.jellyfin.org/posts/576/watchlist-like-netflix)
- **D5** — [Jellyfin: Watched History](https://features.jellyfin.org/posts/633/watched-history)
- **D6** — [Jellyfin: OIDC/OAuth SSO and 2FA requests](https://features.jellyfin.org/posts/230/support-for-oidc-oauth-sso)
- **D7** — [Jellyfin: Most Wanted feature board](https://features.jellyfin.org/)
- **D8** — [Jellyfin: Android offline mode](https://features.jellyfin.org/posts/218/support-offline-mode-on-android-mobile)
- **D9** — [Jellyfin: Gapless playback](https://features.jellyfin.org/posts/181/gapless-playback)
- **D10** — [Plex PLEXREADER and audiobook requests](https://forums.plex.tv/t/plexreader-comics-books-pdfs/26684)
- **D11** — [Plex: HEIC/HEIF support request](https://forums.plex.tv/t/heic-heif-support-in-photo-libraries-new-photo-format-for-iphones/195514)
- **D12** — [Jellyfin: IPTV channel groupings](https://features.jellyfin.org/posts/186/iptv-channel-groupings)
- **D13** — [Jellyfin: SyncPlay invite link](https://features.jellyfin.org/posts/971/syncplay-invite-to-watch-link)
- **D14** — [Plex: open Feature Suggestions sorted by votes](https://forums.plex.tv/c/feature-suggestions/8/l/latest?order=votes&status=open)
- **D15** — [Plex: feature-request voting rules](https://forums.plex.tv/t/please-read-before-submitting-a-feature-suggestion/273976/)
- **D16** — [Emby: Feature Requests sorted by reactions](https://emby.media/community/forum/98-feature-requests/?filter=topics_without_best_answers&sortby=forums_topics_reactions.topic_reactions&sortdirection=desc)
- **D17** — [Emby: feature-request voting guidance](https://emby.media/community/forum/98-feature-requests/)
