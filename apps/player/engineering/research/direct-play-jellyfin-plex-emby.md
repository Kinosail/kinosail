# Direct Play behavior in Jellyfin, Plex, and Emby

Research snapshot: **2026-08-28 (America/Denver)**.

## Executive summary

Jellyfin, Plex, and Emby all prefer Direct Play. None uses “always send the original file” as its normal automatic rule.

Their common model is:

1. The client describes its playback capabilities.
2. The server compares the selected media, tracks, quality policy, and network limit with those capabilities.
3. The server chooses the smallest required transformation.
4. The product reports the actual playback method and its reason.

Plex also documents a `Force Direct Play` mode on some clients. It tries the original despite predicted incompatibility, then falls back after failure. This is the closest first-party match for a “95% Direct Play” preference.

The best Kinosail model is therefore not blind Direct Play. It is a persistent **Direct First** policy with no local bandwidth limit, exact capability matching, minimal transforms, and error-only fallback. It must retain **Direct Play only** as a strict override.

## Method and limits

This report uses only official product documentation and official repositories. Source links use fixed revisions where code behavior matters.

- Jellyfin Server revision: `fbb0f1afbcd52a15d6e56742bba0f338a5ced88f`.
- Jellyfin Web revision: `aa2bdf9512d5b8215c127d47a15cfb20a97d2055`.
- Emby Web Components revision: `69877ad3319a7b422d0e1f8289dc5ca234c4040d`.

Jellyfin publishes its complete server and web-client source. Plex Media Server and current Emby Server internals are proprietary. Their exact server scoring and all client-specific retry rules are not verifiable. Emby Web Components is public, but it does not prove identical behavior in every Emby client.

## Comparison

| Decision area | Jellyfin | Plex | Emby |
| --- | --- | --- | --- |
| Normal default | Enables Direct Play, Direct Stream, and transcoding. The capability result selects one. | `Auto` tries Direct Play for compatible media, then Direct Stream or transcode. | Apps automatically select a method. Current guidance says they avoid transcoding when possible. |
| Capability input | Detailed device profile with container, codec, profile, level, range, channels, subtitles, and bitrate. | The app tells the server its capabilities. Public docs name container, codec, bitrate, and resolution. | Device profiles describe direct-play formats and container or codec limits. |
| Container mismatch | Remux. Audio and video stay unchanged. | Direct Stream. Audio and video stay unchanged. | Direct Stream or transmux. Video stays unchanged. |
| Audio mismatch | Direct Stream. Audio changes; video stays unchanged. | Partially-transcoded Direct Stream. Audio changes; video stays unchanged. | Direct Stream can convert audio while leaving video unchanged. |
| Subtitle mismatch | External or remuxed subtitles can preserve video. Burn-in converts video. | Unsupported selected subtitles are burned into video. | Unsupported selected subtitles can require video conversion. |
| Bitrate policy | Client limit applies to selection. Remote server limits apply only outside the local network. | Separate local and remote quality settings. Server remote limits do not apply to local playback. | Every app has a maximum streaming bitrate. `Auto` is recommended for most users. |
| Failure fallback | Web retries from a playback error. Its local-file retry can force video conversion immediately. | Roku `Force` retries with Direct Stream or transcode after failure. Other client details are unpublished. | Web Components retries after a playback error and disables Direct Play and Direct Stream. |
| User control | In-player quality changes restart at the same position. Administrators can deny conversion or remuxing. | Many apps expose Direct Play and Direct Stream settings. Some expose `Force Direct Play`. | Apps expose maximum bitrate. Some platforms support direct file paths. |
| Visibility | Player `Playback Data` and server dashboard show actual method and reasons. | Player playback information and server dashboard show direct or converted tracks. | `Stats for Nerds` shows Direct Play or transcoding. Public strings include exact transcode reasons. |

## Jellyfin

### Verified behavior

Jellyfin states that its goal is to Direct Play all media. Full Direct Play requires compatible container, video, audio, and subtitles. It remuxes container mismatches, converts unsupported audio, and converts video for unsupported video or subtitle burn-in ([codec support](https://jellyfin.org/docs/general/clients/codec-support/)).

The client sends transcoding profiles with codecs, resolution, bitrate, and other limits. The server selects the best output within those limits ([transcoding documentation](https://jellyfin.org/docs/general/post-install/transcoding/)). The API permits Direct Play, Direct Stream, transcoding, video copying, and audio copying by default. Those flags permit methods; they do not force a result ([API defaults](https://github.com/jellyfin/jellyfin/blob/fbb0f1afbcd52a15d6e56742bba0f338a5ced88f/Jellyfin.Api/Controllers/MediaInfoController.cs#L149-L165)).

Jellyfin Web builds a device profile from browser and platform rules plus `canPlayType()`. It sends explicit direct-play profiles rather than one generic “browser” capability ([profile construction](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/scripts/browserDeviceProfile.js#L505-L526), [direct-play profiles](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/scripts/browserDeviceProfile.js#L742-L789)). The source includes targeted exceptions where browser reports are unreliable, such as Firefox Matroska behavior and Xbox AV1 reports ([browser capability rules](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/scripts/browserDeviceProfile.js#L1-L280)).

The server checks media and profile constraints before playback. It records direct-play rejection reasons, then selects an output profile ([decision order](https://github.com/jellyfin/jellyfin/blob/fbb0f1afbcd52a15d6e56742bba0f338a5ced88f/MediaBrowser.Model/Dlna/StreamBuilder.cs#L708-L819), [capability checks](https://github.com/jellyfin/jellyfin/blob/fbb0f1afbcd52a15d6e56742bba0f338a5ced88f/MediaBrowser.Model/Dlna/StreamBuilder.cs#L1278-L1418)). It can select a compatible audio track before converting audio ([audio selection](https://github.com/jellyfin/jellyfin/blob/fbb0f1afbcd52a15d6e56742bba0f338a5ced88f/MediaBrowser.Model/Dlna/StreamBuilder.cs#L1352-L1364)).

Automatic bitrate testing keeps 30% headroom. A detected local connection gets at least 140 Mbps ([bitrate test](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/utils/bitrateTest.ts#L106-L129), [local floor](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/utils/bitrateTest.ts#L166-L195)). A source above an effective limit can still lose Direct Play. No limit means no bitrate restriction ([bitrate rejection](https://github.com/jellyfin/jellyfin/blob/fbb0f1afbcd52a15d6e56742bba0f338a5ced88f/MediaBrowser.Model/Dlna/StreamBuilder.cs#L1645-L1668)). Server remote limits apply only outside the configured local network ([remote limit](https://github.com/jellyfin/jellyfin/blob/fbb0f1afbcd52a15d6e56742bba0f338a5ced88f/Jellyfin.Api/Helpers/MediaInfoHelper.cs#L518-L540)).

Jellyfin Web retries only through its playback-error handler. No startup timer or buffering counter controls this path. The current local-file retry disables Direct Play and Direct Stream, which can jump directly to full video conversion ([retry code](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/components/playback/playbackmanager.js#L3398-L3447)).

The player exposes `Playback Data`. The dashboard shows the actual play method and transcode reasons ([player details](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/components/playerstats/playerstats.js#L377-L436), [dashboard status](https://github.com/jellyfin/jellyfin-web/blob/aa2bdf9512d5b8215c127d47a15cfb20a97d2055/src/apps/dashboard/features/devices/components/DeviceCard.tsx#L84-L115)). Administrators can deny video conversion, audio conversion, or remuxing per user ([user controls](https://jellyfin.org/docs/general/server/users/adding-managing-users/)). Jellyfin Web does not expose a persistent `Direct Play only` switch in its player.

### Gap

Jellyfin does not define a persistent “95% Direct Play” preference. Its local 140 Mbps floor is generous, but it remains a ceiling. Its first local playback-error retry can also be more aggressive than the requested Kinosail policy.

## Plex

### Verified behavior

Plex uses capability negotiation. The app tells the server what it can handle. Exact compatibility permits Direct Play. A container mismatch permits Direct Stream. An incompatible audio track can be converted without changing the video ([playback overview](https://support.plex.tv/articles/200430303-streaming-overview/), [method details](https://support.plex.tv/articles/200250387-streaming-media-direct-play-and-direct-stream/)).

Plex checks container, bitrate, codecs, and resolution for Direct Play. It says most users should keep Direct Play and Direct Stream enabled. An unsupported selected subtitle can require video burn-in and a full video transcode ([method details](https://support.plex.tv/articles/200250387-streaming-media-direct-play-and-direct-stream/)).

Plex separates local and remote quality. Higher quality increases the chance of Direct Play. A quality limit below the media bitrate causes conversion. Server remote bandwidth controls do not apply to local playback ([quality guidance](https://support.plex.tv/articles/201767273-how-do-i-choose-the-right-streaming-quality-in-an-app/), [bandwidth limits](https://support.plex.tv/articles/227715247-server-settings-bandwidth-and-transcoding-limits/)).

Automatic quality does not change a session already using original quality. It works only after the session starts converted. It also does not change a converted session back to original quality ([automatic quality](https://support.plex.tv/articles/115007570148-automatically-adjust-quality-when-streaming/)).

Plex for Roku documents two Direct Play policies. `Auto` Direct Plays compatible media. `Force` tries Direct Play regardless of predicted compatibility, then falls back to Direct Stream or transcode after failure ([Roku settings](https://support.plex.tv/articles/204275243-settings-plex-for-roku/)). Plex HTPC also exposes `Allow Direct Play`, `Allow Direct Stream`, and `Force Direct Play` ([HTPC settings](https://support.plex.tv/articles/htpc-settings/)).

The player can show source, quality, and whether the video and audio are direct or transcoded. The server dashboard shows stream details and conversion state ([iOS playback information](https://support.plex.tv/articles/205671108-media-playback/), [server dashboard](https://support.plex.tv/articles/200871837-status-and-dashboard/)).

### Gap

Plex does not publish current server decision code. `Force Direct Play` is documented for Roku and HTPC, but official sources do not prove identical controls or fallback rules in every Plex app.

## Emby

### Verified behavior

Emby Direct Play uses the source without modification. Direct Stream keeps the video unchanged but can change the container, audio, or subtitle track. Video conversion occurs for unsupported video, excessive bitrate, or subtitle burn-in ([playback methods](https://support.emby.media/support/articles/DirectPlay-Stream-Transcoding.html)).

Emby says its apps avoid transcoding when possible. The automatic selection considers network performance, media information, device capability, and configuration. Every app has a `Max streaming bitrate` setting. Current guidance recommends `Auto` for most users. A file above that setting is converted ([transcoding guide](https://support.emby.media/support/articles/Transcoding.html)).

Emby device profiles declare native formats. Container and codec constraints can override a nominal Direct Play match ([official profile strings](https://github.com/MediaBrowser/Emby/blob/master/MediaBrowser.WebDashboard/dashboard-ui/strings/en-US.json)). The older official playback guide orders selection as Direct Play, then Direct Stream under the user bitrate limit, then transcode ([playback guide](https://github.com/MediaBrowser/Emby/wiki/Playback-Guidelines)).

Current Emby Web Components retries a failed session when the source supports conversion. It disables Direct Play and Direct Stream, then requests conversion from the current position. The retry helper lists decode, unsupported-media, network, and server errors, but its condition does not filter by error type ([retry code](https://github.com/MediaBrowser/emby-webcomponents/blob/69877ad3319a7b422d0e1f8289dc5ca234c4040d/playback/playbackmanager.js#L3230-L3282)).

Emby exposes `Stats for Nerds`. Its public client strings include `Direct playing`, `Direct streaming`, `Transcoding`, `Reason for transcoding`, and specific causes such as container, codec, level, channels, bitrate, subtitles, and direct-play errors ([status strings](https://github.com/MediaBrowser/emby-webcomponents/blob/69877ad3319a7b422d0e1f8289dc5ca234c4040d/strings/en-US.json)).

### Gap

Current Emby Server decision code is not public. The older playback guide uses a narrower definition of Direct Play than current documentation. Public evidence does not establish one universal force-direct control across Emby clients.

## Kinosail design inference

The following recommendations are product inferences. They are not claims about competitor internals.

### 1. Make Direct First the persistent default

Use one saved policy for the profile and device:

- **Direct First**: Try the original unless a verified hard incompatibility exists. Use error-only fallback.
- **Direct Play only**: Always use the original. Never fall back.
- **Compatibility**: Preselect the smallest compatible transformation.

Direct First should be the default. It best matches the requested 95% behavior and Plex `Force Direct Play` semantics.

### 2. Remove local bandwidth as an automatic transcode trigger

A trusted local network should have no bitrate ceiling by default. Do not infer insufficient Wi-Fi from connection type, startup delay, or buffering. Let the user set an explicit local cap if required.

Keep remote bitrate limits separate. An explicit remote limit can justify conversion.

### 3. Use a complete capability contract

Match at least these properties before declaring a hard incompatibility:

- container and MIME type;
- video codec, profile, level, bit depth, color range, resolution, and frame rate;
- audio codec, channel count, sample rate, and selected track;
- subtitle format and delivery method;
- byte-range and seeking support; and
- the selected device and playback engine.

Do not use one codec hint as the complete decision. Browser reports need targeted exceptions and real-device regression fixtures.

### 4. Transform one layer at a time

Use this strict fallback order:

1. Direct Play.
2. Remux only.
3. Convert audio only.
4. Convert video only when required.

Keep text subtitles external when possible. Never convert video only because the container or audio is incompatible.

### 5. Fall back only after a real playback failure

Valid fallback triggers are a rejected source, unsupported-media error, or decode failure. A timeout, slow startup, buffering, or network interruption must not authorize transcoding.

Resume from the observed position. Preserve the selected tracks. Try the next smallest method instead of jumping to full video conversion.

### 6. Show the actual method, not the requested plan

Keep an always-visible player status with these exact states:

- `Direct Play`
- `Remux`
- `Transcoding audio`
- `Transcoding video`

Selecting the status should open the method control and one clear reason. Examples are `Container is not supported`, `Selected audio is not supported`, and `Direct Play failed with a decode error`.

Derive the state from the actual output streams. Do not label a request `Direct Play` before the original source starts.

### 7. Preserve direct user control

The method switch must work during playback. It must resume at the same position. A strict Direct Play selection must persist until the user changes it.

The player and owner dashboard should show the same actual method and reason. This makes incorrect selection observable without server-log access.

## Acceptance standard

Direct Play is correct only when all of these statements are true:

- A supported local source starts with no remux or encoder process.
- No local bitrate estimate silently changes that result.
- Slow start and buffering never trigger conversion.
- Container-only mismatch does not convert audio or video.
- Audio-only mismatch does not convert video.
- External text subtitles do not convert video.
- A real playback error advances only one fallback step.
- `Direct Play only` never falls back.
- The visible state matches the actual server process and output streams.
- The UI gives the exact decision reason and permits an immediate switch.

Physical browser, phone, television, receiver, high-bitrate Wi-Fi, subtitle, and audio-track fixtures remain required. Protocol tests alone cannot certify device playback.
