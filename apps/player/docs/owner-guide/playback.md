---
title: Configure playback
description: Choose direct-first playback, transcoding policy, hardware acceleration, and automatic skip behavior.
section: Own the Server
---

# Configure playback

Set playback policy in **Settings → Playback**. Start with **Automatic**. It uses Direct playback when the device supports the source and falls back to remuxing, audio conversion, or HLS transcoding when needed.

## Select a playback mode

Choose **Default playback**:

- **Automatic** is the recommended direct-first choice.
- **Direct** never transcodes. It is useful when every target device supports the source.
- **Compatibility** favors a broadly supported format and can use more Server capacity.

Select whether **Subtitles** are **On** or **Off**. Enable **Play the next Episode automatically** when you want episode autoplay. Save the form after you change a value.

## Configure automatic skip

Select any of these options in **Playback**:

- **Skip intros automatically**;
- **Skip recaps automatically**;
- **Skip commercials automatically**;
- **Skip outros automatically**; and
- **Skip credits automatically**.

Kinosail applies detected markers to the playback timeline. Analyze markers first from **Settings → Playback segment analysis**. A marker can be absent or incorrect when the source does not provide enough evidence.

## Select video conversion policy

In **Settings → Video conversion**, choose **Conversion preference**:

- **Automatic (Recommended)** balances interactive playback and output quality.
- **Prefer speed** reduces waiting and uses less complex processing.
- **Prefer quality** favors output quality and can use more capacity.

Choose a **Video format**. **Automatic per device (Recommended)** tries newer formats when the device and Server support them. Otherwise, Kinosail uses H.264.

Choose **Speed up with**. **Automatic (Recommended)** shows the selected processor or graphics path below the control. The other choices depend on usable Server hardware.

Select **Improve HDR colors on non-HDR screens** when a target device cannot display high dynamic range (HDR). Save the setting.

Run **Run quick check** under **Quick compatibility check**. The check makes a small test video. It does not use Library Content. A passed check does not certify every file or playback device.

Open **Video compatibility** for detected formats, available conversion methods, and focused setup guidance.


## Control Viewer capacity

In **Settings → Profiles**, clear **Allow transcoding** for a Viewer who must not consume transcoder capacity. A Viewer without permission receives the policy denial. Direct playback still depends on the device and source.

Clear the cache from **Settings → Automatic maintenance → Clear transcode cache** when prepared files are stale. Kinosail can recreate this cache; it is not a recovery backup.

## Source of truth

Sources: `internal/server/settings_http.go`, `internal/server/transcoder_support_html.go`, `internal/server/playback_timeline.go`, `README.md`, and `compose.release.yaml`.
