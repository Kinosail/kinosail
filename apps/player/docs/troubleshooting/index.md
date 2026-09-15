---
title: Troubleshooting
description: Start from a Kinosail symptom and find the shortest safe diagnostic path.
section: Fix a problem
---

# Troubleshooting

Start with the visible symptom. Use the smallest diagnostic that can confirm the cause.

## Choose a symptom

- [Install and startup problems]({{ '/troubleshooting/install-and-startup/' | relative_url }}) covers containers, ports, HTTPS, health, storage, and startup.
- [Scanning and metadata problems]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }}) covers missing items, wrong matches, artwork, permissions, and providers.
- [Playback problems]({{ '/troubleshooting/playback/' | relative_url }}) covers unavailable playback, buffering, tracks, subtitles, seeking, and device differences.
- [Sign-in and access problems]({{ '/troubleshooting/sign-in-and-access/' | relative_url }}) covers passwords, passkeys, TOTP, profiles, Jellyfin apps, certificates, remote access, and shares.

## Use this safe order

1. Record the exact message, page, item title, device, and time.
2. Check whether the problem affects one item, one Profile, one device, or the whole Server.
3. Check **Settings → System** as an Owner. Review scan status, diagnostics, recent activity, sessions, and cache status.
4. Retry one time after a small, reversible change.
5. Collect a short log excerpt and remove secrets before sharing it.

Do not paste passwords, passkeys, TOTP secrets, recovery codes, API keys, Media Share links, cookies, private hostnames, or full request URLs. Kinosail logs redact sensitive values, but review any excerpt yourself.

If an issue can affect data, stop before deleting a volume, library path, configuration file, or backup. Make a verified backup first.

Source of truth: current Server diagnostics and recovery paths.
