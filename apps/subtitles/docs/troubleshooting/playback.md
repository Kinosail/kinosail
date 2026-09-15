---
title: Playback problems
description: Diagnose unavailable playback, buffering, transcoding, seeking, tracks, subtitles, and device differences.
section: Fix a problem
---

# Playback problems

Use this page when a title does not start, stops, buffers, seeks poorly, or has the wrong track or subtitles.

## Playback is unavailable

Check the item in this order:

1. Confirm that the item still appears for your Profile.
2. Try **Automatic** playback.
3. If Automatic fails, try **Compatibility**.
4. Try **Direct** only when the device supports the source.
5. Check whether the Owner allows transcoding for your Profile.

**Direct** uses the original media path and does not transcode. **Automatic** tries Direct first and can fall back to remuxing, audio conversion, or HLS transcoding. **Compatibility** requests a converted HLS stream.

A Viewer without transcoding permission cannot use a stream that requires conversion. An Owner can enable transcoding for that Profile or choose a device that supports Direct playback.

## Video buffers or stops

Check whether the issue follows the device, title, or network:

- Try the same title on the Server’s local network.
- Try another title with a smaller or known-compatible source.
- In an adaptive player, choose a lower **Quality** level.
- Close other playback and heavy background tasks.
- Check the Server’s available storage, CPU, and hardware status in **Settings → System**.

Automatic transcoding uses Server capacity. Hardware acceleration is available only when the host and container expose a supported backend. A failed hardware path can fall back to software for prepared downloads, but a live stream still depends on current capacity.

Do not increase CPU limits or add a second Kinosail container as a first response. Record the item, mode, quality, device, and time.

## Direct works on one device but not another

This is expected when browsers and devices support different containers, codecs, audio tracks, or subtitle formats. Keep **Automatic** for mixed households. Use **Compatibility** for the device that cannot play the source.

The scanner’s media type is not a device certification. Kinosail supports common formats, but tested client flows do not certify every physical device.

## Seeking is inaccurate or returns to the old position

Wait until the player shows its duration before seeking. Use the player seek bar or the 10-second controls. If playback starts at an old position, the player is restoring your Profile’s saved progress. Seek after metadata loads, then allow the player to save the new position.

For a title that is stuck at an incorrect position, mark it unwatched and start again. This changes only your Profile state.

## Audio or subtitles are wrong

Open **Audio tracks** when the source has more than one audio track. Choose a subtitle in **CC** or **Playback settings**. Choose **Off** to hide subtitles.

Embedded and matching `.srt` or `.vtt` sidecar subtitles can appear in the player. If a subtitle needs conversion or the browser cannot display it, use **Compatibility**. If the track is absent, an Owner can use **Find** when the subtitle provider is configured.

## Intro or credits skipping is wrong

Skipping depends on named chapters, analyzed playback segments, and the Owner’s selected auto-skip types. A marker can be **recap**, **intro**, **commercial**, **outro**, or **credits**. An Owner can review or edit markers from the item’s **Manage media** controls.

If a marker is wrong, disable that type in Owner playback settings or edit the marker. Do not treat ordinary chapter names as proof of an intro or credits boundary.

## Offline preparation fails

Open **Offline downloads** and read the job state. A failed job shows **Needs attention** and a **Try again** action. Retry once after checking storage and transcoding capacity.

**Ready offline** means the Server created the file and verified its integrity. If a previously ready job becomes invalid, Kinosail marks it failed. Remove that job and prepare it again. Prepared files are private to the Viewer Profile.

## Collect playback evidence

Record the title, Profile role, device and browser, connection type, selected mode, quality, audio and subtitle choices, and exact message. An Owner can review **Settings → System** and the diagnostics API. Collect only a short relevant log excerpt:

```sh
podman compose logs --tail 100 kinosail
```

Review the excerpt before sharing it. Never include playback tokens, cookies, API keys, claim links, or private media paths.

Source of truth: player, playback plan, HLS, and download handlers.
